/**
 * @name: Copilot CLI 会话用量同步
 * @Descripttion: 解析 GitHub Copilot CLI 的 session.usage_record 事件，按请求写入请求日志统计。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_copilot.go
 */

package services

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// copilotEventsFileName 每个会话目录下的事件流文件名
const copilotEventsFileName = "events.jsonl"

// copilotDataDir 解析 Copilot CLI 数据目录：native 作用域下 COPILOT_HOME 优先，否则 <home>/.copilot
func copilotDataDir(scope string, home string) string {
	if scope == "native" {
		if override := os.Getenv("COPILOT_HOME"); override != "" {
			return override
		}
	}
	return filepath.Join(home, ".copilot")
}

// syncCopilotSessionUsage Copilot CLI 会话用量同步入口；home 为用户主目录
//   - 数据位于 <数据目录>/session-state/<会话 ID>/events.jsonl
//   - 每条 session.usage_record 对应一次模型调用，计数精确；旧版 CLI 不写该事件，不做估算
//   - 文件变化后整体重读，靠 source_record_id 的 upsert 保证幂等
func syncCopilotSessionUsage(ls *LogService, scope string, home string) (SessionSyncSourceResult, error) {
	result := SessionSyncSourceResult{Scope: scope, Platform: sessionPlatformCopilot}
	root := filepath.Join(copilotDataDir(scope, home), "session-state")
	sessions, err := os.ReadDir(root)
	if err != nil {
		// 目录不存在视为未使用 Copilot CLI
		return result, nil
	}
	var fileErrors sessionFileErrors
	for _, session := range sessions {
		path := filepath.Join(root, session.Name(), copilotEventsFileName)
		if info, statErr := os.Stat(path); statErr != nil || !info.Mode().IsRegular() {
			continue
		}
		result.FilesScanned++
		imported, skipped, err := syncCopilotEventsFile(ls, scope, path, session.Name())
		if err != nil {
			// 单个文件失败不影响同一来源的其他文件，错误汇总后返回
			fileErrors.add(path, err)
			continue
		}
		result.Imported += imported
		result.Skipped += skipped
	}
	return result, fileErrors.err()
}

// syncCopilotEventsFile 同步单个会话的 events.jsonl
func syncCopilotEventsFile(ls *LogService, scope string, path string, sessionID string) (int, int, error) {
	cursor, unchanged, err := prepareSessionFileCursor(scope, sessionPlatformCopilot, path)
	if err != nil || unchanged {
		return 0, 0, err
	}
	// Copilot 可能整体重写 events.jsonl（与 cc-switch 一致始终从头读）；
	// 已入库的请求按 source_record_id 识别，用户删除过的由回执阻止复活
	cursor.ByteOffset = 0
	cursor.LineOffset = 0

	records := make([]sessionUsageRecord, 0, 16)
	known := make(map[string]bool, 16)
	duplicates := 0
	next, err := readJSONLLines(cursor, func(line []byte, _ int64) error {
		var event map[string]any
		if json.Unmarshal(line, &event) != nil {
			return nil
		}
		record, ok := copilotRecordFromEvent(scope, sessionID, event)
		if !ok {
			return nil
		}
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
	next.ParserState = ""
	if err := saveSessionFileCursor(next); err != nil {
		return 0, 0, err
	}
	return imported, skipped + duplicates, nil
}

// copilotCount 读取非负整数计数；缺失、负数或非整数按 0
func copilotCount(value any) uint64 {
	number, ok := value.(float64)
	if !ok || math.IsNaN(number) || number < 0 || number != math.Trunc(number) || number > math.MaxInt64 {
		return 0
	}
	return uint64(number)
}

// copilotTokenDetail 取 copilotUsage.tokenDetails 里指定类型的计数
func copilotTokenDetail(usage map[string]any, kind string) (uint64, bool) {
	details, _ := mapValue(usage["copilotUsage"])["tokenDetails"].([]any)
	for _, item := range details {
		detail := mapValue(item)
		if sessionStringValue(detail["tokenType"]) != kind {
			continue
		}
		number, ok := detail["tokenCount"].(float64)
		if !ok || number < 0 || number != math.Trunc(number) {
			return 0, false
		}
		return uint64(number), true
	}
	return 0, false
}

// copilotRecordFromEvent 把 session.usage_record 事件转成用量记录
//   - inputTokens 含缓存读，因此新输入优先取 tokenDetails.input，
//     缺失时用 inputTokens - cacheReadTokens - cacheWriteTokens
//   - 计数全为 0、缺少 id/sequence 或时间戳的事件不导入
func copilotRecordFromEvent(scope string, sessionID string, event map[string]any) (sessionUsageRecord, bool) {
	if sessionStringValue(event["type"]) != "session.usage_record" {
		return sessionUsageRecord{}, false
	}
	usage := mapValue(mapValue(event["data"])["usage"])
	if usage == nil {
		return sessionUsageRecord{}, false
	}
	pick := func(kind string, field string) uint64 {
		if value, ok := copilotTokenDetail(usage, kind); ok {
			return value
		}
		return copilotCount(usage[field])
	}
	cacheRead := pick("cache_read", "cacheReadTokens")
	cacheWrite := pick("cache_write", "cacheWriteTokens")
	input, ok := copilotTokenDetail(usage, "input")
	if !ok {
		input = saturatingSub(saturatingSub(copilotCount(usage["inputTokens"]), cacheRead), cacheWrite)
	}
	output := pick("output", "outputTokens")
	if input == 0 && output == 0 && cacheRead == 0 && cacheWrite == 0 {
		return sessionUsageRecord{}, false
	}

	identity, _ := event["id"].(string)
	if identity == "" {
		sequence, hasSequence := mapValue(usage["accounting"])["sequence"].(float64)
		if !hasSequence || sequence < 0 || sequence != math.Trunc(sequence) {
			return sessionUsageRecord{}, false
		}
		identity = "seq" + strconv.FormatUint(uint64(sequence), 10)
	}
	rawTime, _ := event["timestamp"].(string)
	createdAt := parseSessionTimestamp(rawTime)
	if createdAt.IsZero() {
		return sessionUsageRecord{}, false
	}

	record := newSessionUsageRecordForSource(requestLogDataSourceCopilotSession)
	// 去重键不含供应商，避免归属规则变化导致旧记录重复导入
	record.SourceRecordID = stableSessionRecordID(scope, sessionPlatformCopilot, "copilot:"+sessionID+":"+identity)
	record.SessionID = sessionID
	record.Model = "unknown"
	if model := normalizeSessionModel(sessionStringValue(usage["model"])); model != "" {
		record.Model = model
	}
	record.InputTokens = copilotClampTokens(input)
	record.OutputTokens = copilotClampTokens(output)
	record.CacheReadTokens = copilotClampTokens(cacheRead)
	record.CacheCreateTokens = copilotClampTokens(cacheWrite)
	record.CreatedAt = createdAt.Truncate(time.Second) // 库中只存到秒，保留毫秒会导致每轮都判为已变更
	// duration 为整数毫秒；首字耗时取四舍五入后的正数
	if durationMs, ok := usage["duration"].(float64); ok && durationMs > 0 && durationMs == math.Trunc(durationMs) {
		record.DurationSec = durationMs / 1000.0
	}
	if firstMs, ok := usage["timeToFirstTokenMs"].(float64); ok && firstMs > 0 {
		record.FirstTokenSec = math.Round(firstMs) / 1000.0
	}
	return record, true
}

// copilotClampTokens 截断到 uint32 上限
func copilotClampTokens(value uint64) int {
	if value > math.MaxUint32 {
		return math.MaxUint32
	}
	return int(value)
}
