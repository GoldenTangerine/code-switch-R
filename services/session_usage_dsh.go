/**
 * @name: DeepSeek Harness 会话用量同步
 * @Descripttion: 解析 DeepSeek Harness 本地会话记录并写入请求日志统计。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_dsh.go
 */

package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	modelpricing "codeswitch/resources/model-pricing"
)

// dshStepKeyNone turn / step 缺失时的占位值
const dshStepKeyNone = "none"

// dshSessionLogName 判断文件名是否为 DSH 会话日志：session.<版本>.jsonl 或 session.<版本>.jsonl.zstd
func dshSessionLogName(name string) (isLog bool, compressed bool) {
	if !strings.HasPrefix(name, "session.") {
		return false, false
	}
	switch {
	case strings.HasSuffix(name, ".jsonl.zstd"):
		return true, true
	case strings.HasSuffix(name, ".jsonl"):
		return true, false
	}
	return false, false
}

// dshSessionsRoot DSH 会话根目录：本机优先取 $DSH_HOME/sessions，默认 ~/.dsh/sessions；WSL 只认 home 下的默认位置
func dshSessionsRoot(home string, scope string) string {
	dataDir := filepath.Join(home, ".dsh")
	if scope == "native" {
		if override := os.Getenv("DSH_HOME"); override != "" {
			dataDir = override
		}
	}
	return filepath.Join(dataDir, "sessions")
}

// syncDeepSeekHarnessSessionUsage DeepSeek Harness 会话用量同步入口；home 为用户主目录
//   - 目录层级：<根>/<项目目录>/<会话目录>/session.<版本>.jsonl，会话目录名即会话 id
//   - 只支持未压缩的 .jsonl；.jsonl.zstd 需要 zstd 解码，当前不引入依赖，计入 Skipped
//   - assistant/message 事件的 usage 里 inputTokens 不含缓存，直接作为新增输入
func syncDeepSeekHarnessSessionUsage(ls *LogService, scope string, home string) (SessionSyncSourceResult, error) {
	result := SessionSyncSourceResult{Scope: scope, Platform: sessionPlatformDeepSeekHarness}
	root := dshSessionsRoot(home, scope)

	type sessionLog struct {
		path      string
		sessionID string
	}
	logs := make([]sessionLog, 0, 16)
	projects, err := os.ReadDir(root)
	if err != nil {
		return result, nil
	}
	for _, project := range projects {
		if !project.IsDir() {
			continue
		}
		sessions, err := os.ReadDir(filepath.Join(root, project.Name()))
		if err != nil {
			continue
		}
		for _, session := range sessions {
			if !session.IsDir() {
				continue
			}
			sessionDir := filepath.Join(root, project.Name(), session.Name())
			entries, err := os.ReadDir(sessionDir)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				isLog, compressed := dshSessionLogName(entry.Name())
				if !isLog || !entry.Type().IsRegular() {
					continue
				}
				if compressed {
					result.Skipped++
					continue
				}
				logs = append(logs, sessionLog{path: filepath.Join(sessionDir, entry.Name()), sessionID: session.Name()})
			}
		}
	}
	sort.Slice(logs, func(left int, right int) bool { return logs[left].path < logs[right].path })
	result.FilesScanned = len(logs)

	var failures []string
	for _, log := range logs {
		imported, skipped, err := dshSyncFile(ls, scope, log.path, log.sessionID)
		result.Imported += imported
		result.Skipped += skipped
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", log.path, err))
		}
	}
	if len(failures) > 0 {
		result.Error = strings.Join(failures, "; ")
	}
	return result, nil
}

// dshSyncFile 增量导入单个会话日志；ParserState 保存尚未配对的 step/start 时间，供下一轮估算耗时
func dshSyncFile(ls *LogService, scope string, path string, sessionID string) (int, int, error) {
	cursor, unchanged, err := prepareSessionFileCursor(scope, sessionPlatformDeepSeekHarness, path)
	if err != nil || unchanged {
		return 0, 0, err
	}

	stepStarts := make(map[string]int64)
	if cursor.ParserState != "" && cursor.ByteOffset > 0 {
		if json.Unmarshal([]byte(cursor.ParserState), &stepStarts) != nil || stepStarts == nil {
			// 状态损坏：回到文件开头重扫，来源 ID 稳定，不会重复入库
			stepStarts = make(map[string]int64)
			cursor.ByteOffset = 0
			cursor.LineOffset = 0
		}
	} else {
		cursor.ByteOffset = 0
		cursor.LineOffset = 0
	}

	records := make([]sessionUsageRecord, 0, 16)
	seen := make(map[string]struct{}, 16)
	next, err := readJSONLLines(cursor, func(line []byte, _ int64) error {
		entry := piFamilyDecodeObject(line)
		if entry == nil {
			return nil
		}
		switch piFamilyString(entry["type"]) {
		case "step/start":
			data := mapValue(entry["data"])
			if startMs, ok := piFamilyInt64(entry["time"]); ok {
				stepStarts[dshStepKey(data)] = startMs
			}
		case "assistant/message":
			record, ok := dshParseAssistantMessage(scope, entry, sessionID, stepStarts)
			if !ok {
				return nil
			}
			delete(stepStarts, dshStepKey(mapValue(entry["data"])))
			// 同一次模型调用在分叉会话里会被原样抄进新日志，只记一次
			if _, exists := seen[record.SourceRecordID]; exists {
				return nil
			}
			seen[record.SourceRecordID] = struct{}{}
			records = append(records, record)
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	records, duplicated, err := piFamilyDropCrossSessionDuplicates(records)
	if err != nil {
		return 0, 0, err
	}
	pricing := ls.resolvePricingSnapshot()
	for index := range records {
		records[index].Model = dshResolvePricingModel(pricing, records[index].Platform, records[index].Model)
	}
	imported, skipped, err := persistSessionUsageRecords(ls, records)
	if err != nil {
		return 0, 0, err
	}
	skipped += duplicated
	state, err := json.Marshal(stepStarts)
	if err != nil {
		return 0, 0, err
	}
	next.ParserState = string(state)
	if err := saveSessionFileCursor(next); err != nil {
		return 0, 0, err
	}
	return imported, skipped, nil
}

// dshStepKey 由 data.turn / data.step 生成配对键
func dshStepKey(data map[string]any) string {
	part := func(value any) string {
		number, ok := value.(interface{ Int64() (int64, error) })
		if !ok {
			return dshStepKeyNone
		}
		parsed, err := number.Int64()
		if err != nil || parsed < 0 {
			return dshStepKeyNone
		}
		return fmt.Sprintf("%d", parsed)
	}
	return part(data["turn"]) + ":" + part(data["step"])
}

// dshParseAssistantMessage 解析带用量的 assistant/message 事件
//   - 模型：data.message.source.model，缺失记 unknown；带 `<路由>:` 前缀（如 global:deepseek-v4.1-flash）时去掉前缀，保证按真实模型名计价
//   - 耗时：step/start 到 assistant/message 的时间差；首字：data.stream 中最早的 time / time0 与 step/start 之差
//   - 来源 ID 按 message id 去重，没有 message id 时退化为整行内容的指纹
func dshParseAssistantMessage(scope string, entry map[string]any, sessionID string, stepStarts map[string]int64) (sessionUsageRecord, bool) {
	data := mapValue(entry["data"])
	usage := mapValue(data["usage"])
	if data == nil || usage == nil {
		return sessionUsageRecord{}, false
	}
	input := piFamilyTokenCount(usage["inputTokens"])
	output := piFamilyTokenCount(usage["outputTokens"])
	cacheRead := piFamilyTokenCount(usage["cacheReadTokens"])
	cacheWrite := piFamilyTokenCount(usage["cacheWriteTokens"])
	if input == 0 && output == 0 && cacheRead == 0 && cacheWrite == 0 {
		return sessionUsageRecord{}, false
	}
	createdMs, ok := piFamilyInt64(entry["time"])
	if !ok {
		return sessionUsageRecord{}, false
	}

	message := mapValue(data["message"])
	messageID := piFamilyString(message["id"])
	if messageID == "" {
		encoded, _ := json.Marshal(entry)
		digest := sha256.Sum256(encoded)
		messageID = hex.EncodeToString(digest[:])
	}
	model := piFamilyString(mapValue(message["source"])["model"])
	if model == "" {
		model = piFamilyUnknownModel
	}

	latencyMs := int64(0)
	firstTokenMs := int64(0)
	if start, found := stepStarts[dshStepKey(data)]; found {
		latencyMs = piFamilyPlausibleLatency(start, createdMs)
		if firstOutput, hasStream := dshFirstOutputTime(data["stream"]); hasStream && latencyMs > 0 {
			if first := firstOutput - start; first > 0 && first <= latencyMs {
				firstTokenMs = first
			}
		}
	}

	record := newSessionUsageRecordForSource(requestLogDataSourceDeepSeekHarnessSession)
	record.Model = model
	record.SessionID = sessionID
	record.InputTokens = input
	record.OutputTokens = output
	record.CacheReadTokens = cacheRead
	record.CacheCreateTokens = cacheWrite
	record.CreatedAt = time.Unix(createdMs/1000, 0).UTC()
	record.DurationSec = float64(latencyMs) / 1000.0
	record.FirstTokenSec = float64(firstTokenMs) / 1000.0
	record.SourceRecordID = stableSessionRecordID(scope, sessionPlatformDeepSeekHarness, "dsh:"+messageID)
	return record, true
}

// dshFirstOutputTime data.stream 中最早的 time / time0（time0 是打包块的起始时间）
func dshFirstOutputTime(stream any) (int64, bool) {
	items, ok := stream.([]any)
	if !ok {
		return 0, false
	}
	earliest := int64(math.MaxInt64)
	found := false
	for _, item := range items {
		chunk := mapValue(item)
		for _, key := range []string{"time", "time0"} {
			if value, ok := piFamilyInt64(chunk[key]); ok && value < earliest {
				earliest = value
				found = true
			}
		}
	}
	return earliest, found
}

// dshResolvePricingModel DSH 的模型 id 可能带「<路由>:<模型>」前缀（如 global:deepseek-v4.1-flash）。
// 与 cc-switch 一致：原样能查到价格时保持不变；查不到才去掉路由前缀，
// 避免把 deepseek-r1:free 这类本身带冒号的模型名截成 free
func dshResolvePricingModel(pricing *modelpricing.Service, platform string, model string) string {
	_, stripped, found := strings.Cut(model, ":")
	if !found || stripped == "" {
		return model
	}
	probe := ReqeustLog{
		Platform:       platform,
		Model:          model,
		RequestedModel: model,
		ResponseModel:  model,
		InputTokens:    1,
		OutputTokens:   1,
	}
	applyLogPricing(pricing, &probe)
	if probe.HasPricing {
		return model
	}
	return stripped
}
