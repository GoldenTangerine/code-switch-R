/**
 * @name: Kimi Code 会话用量同步
 * @Descripttion: 解析 Kimi Code 本地 wire.jsonl 会话记录（含子代理），按消息写入请求日志统计。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_kimi.go
 */

package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// kimiWireFileName 每个 agent 目录下的事件流文件名
	kimiWireFileName = "wire.jsonl"
	// kimiMinLatencyMs / kimiMaxLatencyMs 请求耗时的合理区间，区间外视为无效计时
	kimiMinLatencyMs = 100
	kimiMaxLatencyMs = 60 * 60 * 1000
	// kimiMaxDurationMs 原生耗时字段的上限（1 小时）
	kimiMaxDurationMs = 3_600_000
)

// kimiRequestInfo 按 messageId 关联出的请求元数据（仅保留模型与计时，不留任何对话内容）
type kimiRequestInfo struct {
	Model        string
	LatencyMs    int64
	FirstTokenMs int64 // 0 表示没有
	RequestMs    int64 // 最后一次 llm.request 的发起时间，0 表示没有
}

// kimiStep 当前正在进行的一步（step.begin 到 step.end）
type kimiStep struct {
	uuid       string
	turnStep   string
	model      string
	requestMs  int64
	responseMs int64
	hasTools   bool
}

// kimiRequestIndex 把 llm.request / step 事件按 messageId 关联到已提交的消息上。
// 人类视图里的模型是合成的 agent-loop，真实模型要从 loop 请求里取。
type kimiRequestIndex struct {
	agent    string // 当前 wire 所属 agent，空表示 main
	step     *kimiStep
	messages map[string]*kimiRequestInfo
}

func newKimiRequestIndex(agent string) *kimiRequestIndex {
	if agent == "main" {
		agent = ""
	}
	return &kimiRequestIndex{agent: agent, messages: make(map[string]*kimiRequestInfo)}
}

// kimiTimestampMs 解析事件时间；小于 1e12 的按秒处理
func kimiTimestampMs(value any) int64 {
	number, ok := value.(float64)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number <= 0 {
		return 0
	}
	if number < 1e12 {
		number *= 1000
	}
	if number >= math.MaxInt64 {
		return 0
	}
	return int64(number)
}

// kimiModelName 过滤掉合成/空的模型名
func kimiModelName(value any) string {
	name, _ := value.(string)
	if strings.TrimSpace(name) == "" || name == "agent-loop" || name == "unknown" {
		return ""
	}
	return name
}

// kimiDurationMs 读取原生耗时字段，范围 0..=1 小时，其余视为无效
func kimiDurationMs(value any) (int64, bool) {
	number, ok := value.(float64)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 || number > kimiMaxDurationMs {
		return 0, false
	}
	return int64(math.Round(number)), true
}

// kimiEstimatedLatencyMs 由起止时间估算耗时，落在合理区间外返回 0
func kimiEstimatedLatencyMs(startMs int64, endMs int64) int64 {
	latency := endMs - startMs
	if latency < kimiMinLatencyMs || latency > kimiMaxLatencyMs {
		return 0
	}
	return latency
}

// observe 依次喂入 wire 里的每条记录
func (index *kimiRequestIndex) observe(record map[string]any) {
	own := index.agent
	if own == "" {
		own = "main"
	}
	if id, ok := record["agentId"].(string); ok && id != own {
		return
	}
	switch sessionStringValue(record["type"]) {
	case "agent.switched", "agent.turn.started":
		index.step = nil
	case "llm.request":
		if kind, ok := record["kind"].(string); ok && kind != "loop" {
			return
		}
		step := index.step
		if step == nil {
			return
		}
		if actual, ok := record["turnStep"].(string); ok && step.turnStep != "" && step.turnStep != actual {
			return
		}
		// 重试会替换失败的那次，原生计时属于最后一次
		step.model = kimiModelName(record["model"])
		step.requestMs = kimiTimestampMs(record["time"])
		step.responseMs = 0
		step.hasTools = false
	case "context.append_loop_event":
		event := mapValue(record["event"])
		switch sessionStringValue(event["type"]) {
		case "step.begin":
			uuid, ok := event["uuid"].(string)
			if !ok {
				index.step = nil
				return
			}
			step := &kimiStep{uuid: uuid}
			if turn, ok := event["turnId"].(string); ok {
				if number, ok := event["step"].(float64); ok && number >= 0 && number == math.Trunc(number) {
					step.turnStep = turn + "." + strconv.FormatInt(int64(number), 10)
				}
			}
			index.step = step
		case "content.part", "tool.call":
			step := index.step
			if step == nil {
				return
			}
			if stepUUID, _ := event["stepUuid"].(string); stepUUID != step.uuid {
				return
			}
			if sessionStringValue(event["type"]) == "tool.call" {
				step.hasTools = true
			} else if at := kimiTimestampMs(record["time"]); at > step.responseMs {
				// 这些是已完成的片段，并不是首字通知
				step.responseMs = at
			}
		case "step.end":
			index.observeStepEnd(record, event)
		}
	}
}

// observeStepEnd 一步结束：算出该消息的模型与计时
func (index *kimiRequestIndex) observeStepEnd(record map[string]any, event map[string]any) {
	eventUUID, _ := event["uuid"].(string)
	if index.step != nil && eventUUID != index.step.uuid {
		return
	}
	step := index.step
	index.step = nil
	if step == nil {
		step = &kimiStep{}
	}
	id, _ := event["messageId"].(string)
	if id == "" {
		return
	}
	firstMs, hasFirst := kimiDurationMs(event["llmFirstTokenLatencyMs"])
	// step.end 可能被工具执行拖后，优先用原生请求耗时（不含工具、钩子与客户端空闲）
	var latency int64
	if streamMs, hasStream := kimiDurationMs(event["llmStreamDurationMs"]); hasFirst && hasStream {
		latency = kimiEstimatedLatencyMs(0, firstMs+streamMs)
	}
	if latency == 0 && step.requestMs > 0 {
		endMs := step.responseMs
		if endMs == 0 && !step.hasTools {
			endMs = kimiTimestampMs(record["time"])
		}
		if endMs > 0 {
			latency = kimiEstimatedLatencyMs(step.requestMs, endMs)
		}
	}
	// 分叉历史可能重复某一步却没有原始请求事件，保留已有信息
	info := index.messages[id]
	if info == nil {
		info = &kimiRequestInfo{}
		index.messages[id] = info
	}
	if step.model != "" {
		info.Model = step.model
	}
	if step.requestMs > 0 {
		info.RequestMs = step.requestMs
	}
	if latency > 0 {
		info.LatencyMs = latency
		switch {
		case hasFirst && firstMs <= latency:
			info.FirstTokenMs = firstMs
		case info.FirstTokenMs > latency:
			info.FirstTokenMs = 0
		}
	}
}

// get 取 meta.messageId 对应的请求元数据
func (index *kimiRequestIndex) get(meta map[string]any) *kimiRequestInfo {
	id, _ := meta["messageId"].(string)
	if id == "" {
		return nil
	}
	return index.messages[id]
}

// model 优先取真实请求里的模型，其次用消息自带的非合成模型名
func (index *kimiRequestIndex) model(meta map[string]any) string {
	if info := index.get(meta); info != nil && info.Model != "" {
		return info.Model
	}
	return kimiModelName(mapValue(meta["model"])["model"])
}

// kimiPendingUsage 一条待入库的用量（模型与计时要等整个文件读完才能关联）
type kimiPendingUsage struct {
	meta      map[string]any
	key       string
	input     int
	output    int
	cacheRead int
	cacheNew  int
	committed int64
}

// kimiDataDir 解析 Kimi Code 数据目录：native 作用域下 KIMI_CODE_HOME 优先，否则 <home>/.kimi-code
func kimiDataDir(scope string, home string) string {
	if scope == "native" {
		if override := os.Getenv("KIMI_CODE_HOME"); override != "" {
			return override
		}
	}
	return filepath.Join(home, ".kimi-code")
}

// kimiWireFile 一个待同步的 wire.jsonl 及其所属会话/agent
type kimiWireFile struct {
	path    string
	agent   string
	session string
}

// kimiCollectWireFiles 遍历 sessions/<workspace>/<session>/agents/<agent>/wire.jsonl，
// 主代理与每个子代理各有独立的 wire
func kimiCollectWireFiles(root string) []kimiWireFile {
	var files []kimiWireFile
	workspaces, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	for _, workspace := range workspaces {
		sessions, err := os.ReadDir(filepath.Join(root, workspace.Name()))
		if err != nil {
			continue
		}
		for _, session := range sessions {
			agentsDir := filepath.Join(root, workspace.Name(), session.Name(), "agents")
			agents, err := os.ReadDir(agentsDir)
			if err != nil {
				continue
			}
			for _, agent := range agents {
				path := filepath.Join(agentsDir, agent.Name(), kimiWireFileName)
				if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
					files = append(files, kimiWireFile{path: path, agent: agent.Name(), session: session.Name()})
				}
			}
		}
	}
	return files
}

// syncKimiSessionUsage Kimi Code 会话用量同步入口；home 为用户主目录
//   - 数据目录：<home>/.kimi-code（native 作用域可由 KIMI_CODE_HOME 覆盖）
//   - 请求的模型、耗时来自同一 wire 里的 llm.request / step 事件，需要完整上下文，
//     因此文件变化后整体重读，靠 source_record_id 的 upsert 保证幂等
func syncKimiSessionUsage(ls *LogService, scope string, home string) (SessionSyncSourceResult, error) {
	result := SessionSyncSourceResult{Scope: scope, Platform: sessionPlatformKimi}
	files := kimiCollectWireFiles(filepath.Join(kimiDataDir(scope, home), "sessions"))
	result.FilesScanned = len(files)
	var fileErrors sessionFileErrors
	for _, file := range files {
		imported, skipped, err := syncKimiWireFile(ls, scope, file)
		if err != nil {
			// 单个文件失败不影响同一来源的其他文件，错误汇总后返回
			fileErrors.add(file.path, err)
			continue
		}
		result.Imported += imported
		result.Skipped += skipped
	}
	return result, fileErrors.err()
}

// syncKimiWireFile 同步单个 wire.jsonl
func syncKimiWireFile(ls *LogService, scope string, file kimiWireFile) (int, int, error) {
	cursor, unchanged, err := prepareSessionFileCursor(scope, sessionPlatformKimi, file.path)
	if err != nil || unchanged {
		return 0, 0, err
	}
	// 关联请求信息需要完整上下文，始终从头读
	cursor.ByteOffset = 0
	cursor.LineOffset = 0

	isMain := file.agent == "main"
	index := newKimiRequestIndex(file.agent)
	pending := make([]kimiPendingUsage, 0, 16)
	next, err := readJSONLLines(cursor, func(line []byte, _ int64) error {
		record := kimiParseLine(line)
		if record == nil {
			return nil
		}
		index.observe(record)

		var meta, usage map[string]any
		if isMain {
			recordType := sessionStringValue(record["type"])
			if recordType != "agent.message.appended" && recordType != "human.agent.message.appended" {
				return nil
			}
			message := mapValue(mapValue(record["message"])["message"])
			if sessionStringValue(message["role"]) != "assistant" {
				return nil
			}
			meta = mapValue(mapValue(record["message"])["meta"])
			usage = mapValue(meta["usage"])
		} else {
			// 子代理的回复不会进入人类视图；每个完成的 step 自带用量，
			// 父级的 subagent.completed 汇总不导入，因此不会重复计数
			event := mapValue(record["event"])
			if sessionStringValue(record["type"]) != "context.append_loop_event" || sessionStringValue(event["type"]) != "step.end" {
				return nil
			}
			usage = mapValue(event["usage"])
			meta = map[string]any{
				"messageId": event["messageId"],
				"model":     map[string]any{"provider": "subagent"},
			}
		}
		if usage == nil || kimiTimestampMs(record["time"]) == 0 {
			return nil
		}
		item := kimiPendingUsage{
			meta:      meta,
			input:     kimiTokenCount(usage["inputOther"]),
			output:    kimiTokenCount(usage["output"]),
			cacheRead: kimiTokenCount(usage["inputCacheRead"]),
			cacheNew:  kimiTokenCount(usage["inputCacheCreation"]),
			committed: kimiTimestampMs(record["time"]),
		}
		if item.input == 0 && item.output == 0 && item.cacheRead == 0 && item.cacheNew == 0 {
			return nil
		}
		identity, _ := meta["messageId"].(string)
		if identity == "" {
			sum := sha256.Sum256(line)
			identity = hex.EncodeToString(sum[:])
		}
		provider := sessionStringValue(mapValue(meta["model"])["provider"])
		if provider == "" {
			provider = "unknown"
		}
		item.key = "kimi:" + provider + ":" + identity
		pending = append(pending, item)
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	records := make([]sessionUsageRecord, 0, len(pending))
	known := make(map[string]bool, len(pending))
	duplicates := 0
	hasRequestStart := make(map[string]bool, len(pending))
	for _, item := range pending {
		recordID := stableSessionRecordID(scope, sessionPlatformKimi, item.key)
		// 同一消息可能在多个事件里重复出现，只取第一条
		if known[recordID] {
			duplicates++
			continue
		}
		known[recordID] = true

		record := newSessionUsageRecordForSource(requestLogDataSourceKimiSession)
		record.SourceRecordID = recordID
		record.SessionID = file.session
		record.Model = "unknown"
		if model := normalizeSessionModel(index.model(item.meta)); model != "" {
			record.Model = model
		}
		record.InputTokens = item.input
		record.OutputTokens = item.output
		record.CacheReadTokens = item.cacheRead
		record.CacheCreateTokens = item.cacheNew
		committed := item.committed
		if info := index.get(item.meta); info != nil {
			record.DurationSec = float64(info.LatencyMs) / 1000.0
			record.FirstTokenSec = float64(info.FirstTokenMs) / 1000.0
			if info.RequestMs > 0 {
				// Kimi 一轮的消息统一提交，记录时间都是提交时刻；
				// 改按模型调用完成的时间记账，与代理记录请求的时机一致
				if completed := info.RequestMs + info.LatencyMs; completed < committed {
					committed = completed
				}
				hasRequestStart[recordID] = true
			}
		}
		record.CreatedAt = time.UnixMilli(committed).UTC().Truncate(time.Second) // 库中只存到秒，避免重读时反复判为已变更
		records = append(records, record)
	}

	records, err = kimiKeepFirstImport(records, hasRequestStart)
	if err != nil {
		return 0, 0, err
	}
	imported, skipped, err := persistSessionUsageRecords(ls, records)
	if err != nil {
		return 0, 0, err
	}
	next.ParserState = ""
	if err := saveSessionFileCursor(next); err != nil {
		return 0, 0, err
	}
	return imported, skipped + duplicates, nil
}

// kimiKeepFirstImport 已入库的消息以首次导入为准：分叉/截断的副本不应覆盖
// 已解析出的模型、会话归属与记账时间（缺少请求事件时只补计时，不改时间）
func kimiKeepFirstImport(records []sessionUsageRecord, hasRequestStart map[string]bool) ([]sessionUsageRecord, error) {
	existing, err := existingSessionUsageRecords(records)
	if err != nil {
		return nil, err
	}
	for index := range records {
		stored, found := existing[records[index].SourceRecordID]
		if !found {
			continue
		}
		if stored.SessionID != "" {
			records[index].SessionID = stored.SessionID
		}
		if records[index].Model == "unknown" && stored.Model != "" {
			records[index].Model = stored.Model
		}
		if !hasRequestStart[records[index].SourceRecordID] {
			records[index].CreatedAt = time.Time{}
		}
	}
	return records, nil
}

// kimiTokenCount 读取 token 计数，负数与非数字按 0，超出 uint32 的截断
func kimiTokenCount(value any) int {
	number, ok := value.(float64)
	if !ok || math.IsNaN(number) || number < 0 {
		return 0
	}
	if number > math.MaxUint32 {
		number = math.MaxUint32
	}
	return int(number)
}

// kimiParseLine 解析一行 JSON；失败或不是对象返回 nil
func kimiParseLine(line []byte) map[string]any {
	var value map[string]any
	if err := json.Unmarshal(line, &value); err != nil {
		return nil
	}
	return value
}
