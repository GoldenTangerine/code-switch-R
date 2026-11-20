/**
 * @name: WorkBuddy 会话用量同步
 * @Descripttion: 增量解析 WorkBuddy 项目会话日志（含子代理）中的 rawUsage，按请求写入请求日志统计。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_workbuddy.go
 */

package services

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// workBuddyUsage 模型回复事件的用量口径。
// prompt_tokens 为 OpenAI 口径（已含缓存命中），这里拆成「新输入 / 缓存读 / 缓存写 / 输出」
type workBuddyUsage struct {
	Model      string
	FreshInput uint64
	Output     uint64
	CacheRead  uint64
	CacheWrite uint64
}

func (usage workBuddyUsage) isEmpty() bool {
	return usage.FreshInput == 0 && usage.Output == 0 && usage.CacheRead == 0 && usage.CacheWrite == 0
}

// workBuddyDataDir 解析 WorkBuddy 数据目录：native 作用域下 WORKBUDDY_CONFIG_DIR（去空白、非空）优先，否则 <home>/.workbuddy
func workBuddyDataDir(scope string, home string) string {
	if scope == "native" {
		if override := strings.TrimSpace(os.Getenv("WORKBUDDY_CONFIG_DIR")); override != "" {
			return override
		}
	}
	return filepath.Join(home, ".workbuddy")
}

// workBuddyUsageFile 一个待同步的日志文件；ParentSession 非空表示子代理文件
type workBuddyUsageFile struct {
	Path          string
	ParentSession string
}

// workBuddyCollectFiles 收集 projects/<cwd>/<sessionId>.jsonl 以及 <sessionId>/subagents/*.jsonl，
// 子代理的请求算在父会话名下
func workBuddyCollectFiles(projects string) []workBuddyUsageFile {
	var files []workBuddyUsageFile
	projectDirs, err := os.ReadDir(projects)
	if err != nil {
		return nil
	}
	isJSONL := func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && info.Mode().IsRegular() && filepath.Ext(path) == ".jsonl"
	}
	for _, project := range projectDirs {
		projectPath := filepath.Join(projects, project.Name())
		entries, err := os.ReadDir(projectPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			entryPath := filepath.Join(projectPath, entry.Name())
			if isJSONL(entryPath) {
				files = append(files, workBuddyUsageFile{Path: entryPath})
			}
		}
		for _, entry := range entries {
			subagents, err := os.ReadDir(filepath.Join(projectPath, entry.Name(), "subagents"))
			if err != nil {
				continue
			}
			for _, subagent := range subagents {
				path := filepath.Join(projectPath, entry.Name(), "subagents", subagent.Name())
				if isJSONL(path) {
					files = append(files, workBuddyUsageFile{Path: path, ParentSession: entry.Name()})
				}
			}
		}
	}
	return files
}

// syncWorkBuddySessionUsage WorkBuddy 会话用量同步入口；home 为用户主目录
//   - 数据目录：<home>/.workbuddy（native 作用域可由 WORKBUDDY_CONFIG_DIR 覆盖）
//   - 日志只追加，按字节游标增量读取；文件变短由游标逻辑回到开头重读，已入库请求按 source_record_id 去重
//   - 日志里没有请求耗时，不写计时
func syncWorkBuddySessionUsage(ls *LogService, scope string, home string) (SessionSyncSourceResult, error) {
	result := SessionSyncSourceResult{Scope: scope, Platform: sessionPlatformWorkBuddy}
	files := workBuddyCollectFiles(filepath.Join(workBuddyDataDir(scope, home), "projects"))
	result.FilesScanned = len(files)
	var fileErrors sessionFileErrors
	for _, file := range files {
		imported, skipped, err := syncWorkBuddyFile(ls, scope, file)
		if err != nil {
			// 单个文件失败不影响同一来源的其他文件，错误汇总后返回
			fileErrors.add(file.Path, err)
			continue
		}
		result.Imported += imported
		result.Skipped += skipped
	}
	return result, fileErrors.err()
}

// syncWorkBuddyFile 同步单个会话日志
func syncWorkBuddyFile(ls *LogService, scope string, file workBuddyUsageFile) (int, int, error) {
	cursor, unchanged, err := prepareSessionFileCursor(scope, sessionPlatformWorkBuddy, file.Path)
	if err != nil || unchanged {
		return 0, 0, err
	}
	records := make([]sessionUsageRecord, 0, 16)
	known := make(map[string]bool, 16)
	duplicates := 0
	next, err := readJSONLLines(cursor, func(line []byte, _ int64) error {
		var value map[string]any
		if json.Unmarshal(line, &value) != nil {
			return nil
		}
		record, ok := workBuddyRecordFromEvent(scope, file.ParentSession, value)
		if !ok {
			return nil
		}
		// 同一事件可能被重复写入，只取第一条
		if known[record.SourceRecordID] {
			duplicates++
			return nil
		}
		known[record.SourceRecordID] = true
		records = append(records, record)
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	imported, skipped, err := persistSessionUsageRecords(ls, records)
	if err != nil {
		return 0, 0, err
	}
	if err := saveSessionFileCursor(next); err != nil {
		return 0, 0, err
	}
	return imported, skipped + duplicates, nil
}

// workBuddyRecordFromEvent 把带 rawUsage 的事件转成用量记录
//   - 一次请求只有一个事件带 rawUsage（role 为模型侧的 message 或最后一个 function_call），事件 id 即请求 id
//   - parentSession 非空时（子代理文件）会话归属父会话
func workBuddyRecordFromEvent(scope string, parentSession string, event map[string]any) (sessionUsageRecord, bool) {
	usage, ok := workBuddyAssistantUsage(event)
	if !ok || usage.isEmpty() {
		return sessionUsageRecord{}, false
	}
	id, _ := event["id"].(string)
	if id == "" {
		return sessionUsageRecord{}, false
	}
	timestamp, ok := event["timestamp"].(float64)
	if !ok || timestamp <= 0 || timestamp != math.Trunc(timestamp) {
		return sessionUsageRecord{}, false
	}
	sessionID := parentSession
	if sessionID == "" {
		sessionID, _ = event["sessionId"].(string)
	}
	if sessionID == "" {
		sessionID = "unknown"
	}

	record := newSessionUsageRecordForSource(requestLogDataSourceWorkBuddySession)
	record.SourceRecordID = stableSessionRecordID(scope, sessionPlatformWorkBuddy, "workbuddy:"+sessionID+":"+id)
	record.SessionID = sessionID
	record.Model = "unknown"
	if model := normalizeSessionModel(usage.Model); model != "" {
		record.Model = model
	}
	record.InputTokens = workBuddyClamp(usage.FreshInput)
	record.OutputTokens = workBuddyClamp(usage.Output)
	record.CacheReadTokens = workBuddyClamp(usage.CacheRead)
	record.CacheCreateTokens = workBuddyClamp(usage.CacheWrite)
	record.CreatedAt = time.UnixMilli(int64(timestamp)).UTC().Truncate(time.Second) // 库中只存到秒
	return record, true
}

// workBuddyCount 读取 map 里的非负整数计数
func workBuddyCount(source map[string]any, key string) uint64 {
	number, ok := source[key].(float64)
	if !ok || number < 0 || number != math.Trunc(number) || number > math.MaxInt64 {
		return 0
	}
	return uint64(number)
}

func workBuddyMax(values ...uint64) uint64 {
	var result uint64
	for _, value := range values {
		if value > result {
			result = value
		}
	}
	return result
}

func workBuddyClamp(value uint64) int {
	if value > math.MaxUint32 {
		return math.MaxUint32
	}
	return int(value)
}

// workBuddyStripCustomPrefix 自定义模型在 requestModelId 里记成 custom-local:<id>，
// 去掉前缀以便与价格表中的模型 ID 对上
func workBuddyStripCustomPrefix(model string) string {
	prefix, id, found := strings.Cut(model, ":")
	if found && strings.HasPrefix(prefix, "custom-") && strings.TrimSpace(id) != "" {
		return strings.TrimSpace(id)
	}
	return model
}

// workBuddyAssistantUsage 取模型产出事件的用量；用户消息或没有任何用量字段时返回 false
func workBuddyAssistantUsage(event map[string]any) (workBuddyUsage, bool) {
	role, _ := event["role"].(string)
	if role == "user" {
		return workBuddyUsage{}, false
	}
	data := mapValue(event["providerData"])
	var model string
	if raw, ok := data["model"]; ok && raw != nil {
		model, _ = raw.(string)
	} else {
		model, _ = data["requestModelId"].(string)
	}
	model = strings.TrimSpace(model)
	if model != "" {
		model = workBuddyStripCustomPrefix(model)
	}

	if raw := mapValue(data["rawUsage"]); raw != nil {
		cacheRead := workBuddyMax(
			workBuddyCount(raw, "prompt_cache_hit_tokens"),
			workBuddyCount(raw, "cache_read_input_tokens"),
			workBuddyCount(raw, "cached_tokens"),
			workBuddyCount(mapValue(raw["prompt_tokens_details"]), "cached_tokens"),
		)
		cacheWrite := workBuddyMax(workBuddyCount(raw, "cache_creation_input_tokens"), workBuddyCount(raw, "prompt_cache_write_tokens"))
		return workBuddyUsage{
			Model:      model,
			FreshInput: saturatingSub(saturatingSub(workBuddyCount(raw, "prompt_tokens"), cacheRead), cacheWrite),
			Output:     workBuddyCount(raw, "completion_tokens"),
			CacheRead:  cacheRead,
			CacheWrite: cacheWrite,
		}, true
	}

	// 没有 rawUsage 时退回 agents SDK 的 usage（输入同样含缓存），只认 assistant 消息
	if kind, _ := event["type"].(string); kind != "message" || role != "assistant" {
		return workBuddyUsage{}, false
	}
	sdkUsage := mapValue(data["usage"])
	if sdkUsage == nil {
		return workBuddyUsage{}, false
	}
	var cacheRead uint64
	if details, ok := sdkUsage["inputTokensDetails"].([]any); ok {
		for _, detail := range details {
			cacheRead += workBuddyCount(mapValue(detail), "cached_tokens")
		}
	}
	return workBuddyUsage{
		Model:      model,
		FreshInput: saturatingSub(workBuddyCount(sdkUsage, "inputTokens"), cacheRead),
		Output:     workBuddyCount(sdkUsage, "outputTokens"),
		CacheRead:  cacheRead,
	}, true
}
