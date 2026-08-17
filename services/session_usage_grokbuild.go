/**
 * @name: Grok 会话用量同步
 * @Descripttion: 只读解析 ~/.grok 会话目录下的 updates.jsonl，按 turn_completed 差量汇入 request_log
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 03:19:47
 * @LastEditTime: 2026-08-17 03:19:47
 * @FilePath: services/session_usage_grokbuild.go
 */

package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// grokSessionUpdatesFileName Grok 会话更新事件流文件名（位于会话根目录）
const grokSessionUpdatesFileName = "updates.jsonl"

// grokSessionMaxFileBytes 单文件采集上限（50MiB，防止误扫超大文件拖慢同步）
const grokSessionMaxFileBytes = 50 << 20

// grokSessionMaxDepth 相对 ~/.grok 的目录深度上限（防止符号链接等异常结构导致深递归）
const grokSessionMaxDepth = 16

// grokModelUsageCounters turn_completed.model_usage 中单模型的累计计数器（calls 为请求次数，非 token，不参与统计）
type grokModelUsageCounters struct {
	Input  uint64 `json:"input"`
	Output uint64 `json:"output"`
	Cached uint64 `json:"cached"`
}

// grokModelTurnState 单模型最近一次 turn_completed 的快照（累计值 + 定位信息）
type grokModelTurnState struct {
	Counters  grokModelUsageCounters
	CreatedAt time.Time
	Line      int64
	SessionID string
}

// syncGrokSessionUsage Grok 会话用量同步入口
//   - 仅扫描 ~/.grok 下的 updates.jsonl（config.toml 属于配置，与用量无关）
//   - 增量模式：游标记已处理行号，仅对新增 turn_completed 行按同模型累计值取差量
//   - 全量模式（首次同步或文件重写导致游标失效）：按每个模型最后一条 turn_completed 的累计值覆盖，
//     并先清理该文件旧的差量记录，避免新旧口径叠加重复计数
func syncGrokSessionUsage(ls *LogService, scope string, root string) (SessionSyncSourceResult, error) {
	result := SessionSyncSourceResult{Scope: scope, Platform: "grokbuild"}
	files := collectGrokSessionFiles(root)
	result.FilesScanned = len(files)
	for _, path := range files {
		imported, skipped, err := syncGrokSessionFile(ls, scope, root, path)
		if err != nil {
			return result, err
		}
		result.Imported += imported
		result.Skipped += skipped
	}
	return result, nil
}

// collectGrokSessionFiles 递归收集会话根中的 updates.jsonl（限制单文件 ≤50MiB、目录深度 ≤16，跳过符号链接）
func collectGrokSessionFiles(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil
	}

	files := make([]string, 0, 8)
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		depth := grokSessionPathDepth(rel)
		if entry.IsDir() {
			if depth >= grokSessionMaxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if depth > grokSessionMaxDepth {
			return nil
		}
		if !strings.EqualFold(entry.Name(), grokSessionUpdatesFileName) {
			return nil
		}
		if entryInfo, infoErr := entry.Info(); infoErr == nil && entryInfo.Size() > grokSessionMaxFileBytes {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files
}

// grokSessionPathDepth 计算相对路径深度（root 自身为 0）
func grokSessionPathDepth(rel string) int {
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return 0
	}
	return strings.Count(rel, "/") + 1
}

// syncGrokSessionFile 同步单个 updates.jsonl 文件的用量
func syncGrokSessionFile(ls *LogService, scope string, root string, path string) (int, int, error) {
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return 0, 0, err
	}
	relPath = filepath.ToSlash(relPath)

	cursor, unchanged, err := prepareSessionFileCursor(scope, "grokbuild", path)
	if err != nil || unchanged {
		return 0, 0, err
	}

	// ParserState 为空说明是首次同步或文件重写后的全量解析
	fullParse := strings.TrimSpace(cursor.ParserState) == ""
	state := make(map[string]grokModelTurnState, 4)
	if !fullParse {
		var counters map[string]grokModelUsageCounters
		if json.Unmarshal([]byte(cursor.ParserState), &counters) == nil && counters != nil {
			for model, counter := range counters {
				state[model] = grokModelTurnState{Counters: counter}
			}
		} else {
			fullParse = true
		}
	}
	if fullParse {
		// 全量覆盖前清理该文件历史差量记录，避免旧口径叠加
		if err := deleteGrokSessionFileRecords(scope, relPath); err != nil {
			return 0, 0, err
		}
	}

	fileModTime := time.Time{}
	if info := mustFileInfo(path); info != nil {
		fileModTime = info.ModTime()
	}
	sessionID := grokSessionIDFromRelPath(relPath)

	records := make([]sessionUsageRecord, 0, 8)
	next, err := readJSONLLines(cursor, func(line []byte, lineNumber int64) error {
		var value map[string]any
		if json.Unmarshal(line, &value) != nil || sessionStringValue(value["type"]) != "turn_completed" {
			return nil
		}
		createdAt := parseSessionTimestamp(firstString(value, "timestamp", "time"))
		if createdAt.IsZero() {
			createdAt = fileModTime
		}
		lineSessionID := firstString(value, "session_id", "sessionId")
		if lineSessionID == "" {
			lineSessionID = sessionID
		}

		usage := mapValue(value["model_usage"])
		if len(usage) > 0 {
			for rawModel, rawCounters := range usage {
				counterMap := mapValue(rawCounters)
				current := grokModelUsageCounters{
					Input:  uint64(max(intValue(counterMap["input"]), 0)),
					Output: uint64(max(intValue(counterMap["output"]), 0)),
					Cached: uint64(max(intValue(counterMap["cached"]), 0)),
				}
				model := normalizeSessionModel(sessionStringValue(rawModel))
				if model == "" {
					model = "unknown"
				}
				records = applyGrokTurnState(records, state, scope, relPath, model, current, createdAt, lineNumber, lineSessionID, fullParse)
			}
			return nil
		}

		// model_usage 缺失时回落事件顶层的 input/output 计数
		current := grokModelUsageCounters{
			Input:  uint64(max(intValue(value["input"]), 0)),
			Output: uint64(max(intValue(value["output"]), 0)),
		}
		model := normalizeSessionModel(firstString(value, "model"))
		if model == "" {
			model = "unknown"
		}
		records = applyGrokTurnState(records, state, scope, relPath, model, current, createdAt, lineNumber, lineSessionID, fullParse)
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	if fullParse {
		records = append(records, buildGrokFullParseRecords(state, scope, relPath)...)
	}

	imported, skipped, err := persistSessionUsageRecords(ls, records)
	if err != nil {
		return 0, 0, err
	}

	countersJSON, _ := json.Marshal(grokStateCounters(state))
	next.ParserState = string(countersJSON)
	if err := saveSessionFileCursor(next); err != nil {
		return 0, 0, err
	}
	return imported, skipped, nil
}

// applyGrokTurnState 推进单模型的累计快照：
//   - 全量模式只保留最后一条 turn_completed（累计值 + 行号 + 时间），供结束后统一生成覆盖记录
//   - 增量模式计算与上一条同模型累计值的差量并即时生成记录
func applyGrokTurnState(
	records []sessionUsageRecord,
	state map[string]grokModelTurnState,
	scope string,
	relPath string,
	model string,
	current grokModelUsageCounters,
	createdAt time.Time,
	lineNumber int64,
	sessionID string,
	fullParse bool,
) []sessionUsageRecord {
	previous := state[model]
	state[model] = grokModelTurnState{Counters: current, CreatedAt: createdAt, Line: lineNumber, SessionID: sessionID}
	if fullParse {
		return records
	}
	delta := grokModelUsageCounters{
		Input:  saturatingSub(current.Input, previous.Counters.Input),
		Output: saturatingSub(current.Output, previous.Counters.Output),
		Cached: saturatingSub(current.Cached, previous.Counters.Cached),
	}
	if delta.Input+delta.Output+delta.Cached == 0 {
		return records
	}
	return append(records, sessionUsageRecord{
		Platform:        "grokbuild",
		Model:           model,
		ProviderID:      requestLogProviderIDGrokSession,
		Provider:        requestLogProviderGrokSession,
		DataSource:      requestLogDataSourceGrokSession,
		SourceRecordID:  buildGrokSessionRecordID(scope, relPath, lineNumber, model),
		SessionID:       sessionID,
		InputTokens:     int(delta.Input),
		OutputTokens:    int(delta.Output),
		CacheReadTokens: int(delta.Cached),
		CreatedAt:       createdAt,
	})
}

// buildGrokFullParseRecords 全量模式收尾：每个模型按最后一条 turn_completed 的累计值生成一条覆盖记录
func buildGrokFullParseRecords(state map[string]grokModelTurnState, scope string, relPath string) []sessionUsageRecord {
	type grokModelTurnEntry struct {
		Model string
		Turn  grokModelTurnState
	}
	turns := make([]grokModelTurnEntry, 0, len(state))
	for model, turn := range state {
		turns = append(turns, grokModelTurnEntry{Model: model, Turn: turn})
	}
	sort.Slice(turns, func(i, j int) bool {
		return turns[i].Turn.Line < turns[j].Turn.Line
	})

	records := make([]sessionUsageRecord, 0, len(turns))
	for _, entry := range turns {
		turn := entry.Turn
		if turn.Counters.Input+turn.Counters.Output+turn.Counters.Cached == 0 {
			continue
		}
		records = append(records, sessionUsageRecord{
			Platform:        "grokbuild",
			Model:           entry.Model,
			ProviderID:      requestLogProviderIDGrokSession,
			Provider:        requestLogProviderGrokSession,
			DataSource:      requestLogDataSourceGrokSession,
			SourceRecordID:  buildGrokSessionRecordID(scope, relPath, turn.Line, entry.Model),
			SessionID:       turn.SessionID,
			InputTokens:     int(turn.Counters.Input),
			OutputTokens:    int(turn.Counters.Output),
			CacheReadTokens: int(turn.Counters.Cached),
			CreatedAt:       turn.CreatedAt,
		})
	}
	return records
}

// grokStateCounters 提取游标可持久化的累计计数器（不含时间等易变字段）
func grokStateCounters(state map[string]grokModelTurnState) map[string]grokModelUsageCounters {
	counters := make(map[string]grokModelUsageCounters, len(state))
	for model, turn := range state {
		counters[model] = turn.Counters
	}
	return counters
}

// buildGrokSessionRecordID 生成稳定去重键：grok_session:{scope}:{相对路径}:{行号}[:{模型}]
// 同一行可能携带多个模型的计数，需追加模型名消歧；
// scope 参与键构成（对齐 stableSessionRecordID 的隔离模式），避免不同 scope 的同路径记录互相覆盖
func buildGrokSessionRecordID(scope string, relPath string, lineNumber int64, model string) string {
	id := "grok_session:" + scope + ":" + relPath + ":" + strconv.FormatInt(lineNumber, 10)
	if strings.TrimSpace(model) != "" {
		id += ":" + model
	}
	return id
}

// grokSessionIDFromRelPath 以文件所在目录名作为会话标识兜底（会话根目录通常含唯一 ID）
func grokSessionIDFromRelPath(relPath string) string {
	dir := filepath.Dir(filepath.FromSlash(relPath))
	if dir == "." || dir == "" {
		return ""
	}
	return filepath.Base(dir)
}

// deleteGrokSessionFileRecords 清理单个文件的全部 Grok 会话记录（全量覆盖前调用）
// LIKE 前缀与 buildGrokSessionRecordID 一致携带 scope，只清理当前 scope 的记录
// request_log 上的 AFTER DELETE 触发器会级联清理 session_usage_dedup 匹配关系
func deleteGrokSessionFileRecords(scope string, relPath string) error {
	if GlobalDBQueue == nil {
		return fmt.Errorf("database write queue is not initialized")
	}
	pattern := escapeGrokLikePattern("grok_session:"+scope+":"+relPath+":") + "%"
	return GlobalDBQueue.Exec(`
		DELETE FROM request_log
		WHERE data_source = ? AND source_record_id LIKE ? ESCAPE '\'
	`, requestLogDataSourceGrokSession, pattern)
}

// escapeGrokLikePattern 转义 LIKE 通配符（\、%、_），配合 ESCAPE '\' 使用
func escapeGrokLikePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}
