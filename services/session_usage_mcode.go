/**
 * @name: MiniMax Code 会话用量同步
 * @Descripttion: 只读解析 MiniMax Code 本地运行时库的 token 用量表，按自增 id 水位增量汇入请求日志统计。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_mcode.go
 */

package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// mcodeDBRelPath 运行时库相对数据目录的位置
	mcodeDBRelPath = "v2/sqlite/runtime-state.sqlite"
	// mcodeTimingSettleMs 消息行可能比用量行晚落盘：超过这个时间仍对不上就当作没有计时，不再回查
	mcodeTimingSettleMs int64 = 10 * 60 * 1000
)

// mcodeParserState 两条水位：UsageMark 为已导入的最大用量行 id；TimingMark 为计时已处理完（对上或已过期）的最大 id
type mcodeParserState struct {
	UsageMark  int64 `json:"usage_mark"`
	TimingMark int64 `json:"timing_mark"`
}

// mcodeUsageRow local_runtime_token_usage 的一行
type mcodeUsageRow struct {
	ID         int64
	SessionID  string
	TurnID     sql.NullString
	Model      sql.NullString
	TS         int64
	Input      int
	Output     int
	Reasoning  int
	CacheRead  int
	CacheWrite int
}

// mcodeDatabasePath MiniMax Code 运行时库路径：本机优先取 MINIMAX_DATA_DIR，其次 MAVIS_DATA_DIR，默认 ~/.minimax
func mcodeDatabasePath(home string, scope string) string {
	dataDir := filepath.Join(home, ".minimax")
	if scope == "native" {
		for _, key := range []string{"MINIMAX_DATA_DIR", "MAVIS_DATA_DIR"} {
			if value := strings.TrimSpace(os.Getenv(key)); value != "" {
				dataDir = value
				break
			}
		}
	}
	return filepath.Join(dataDir, filepath.FromSlash(mcodeDBRelPath))
}

// syncMiniMaxCodeSessionUsage MiniMax Code 会话用量同步入口；home 为用户主目录
//   - 用量存放在 MiniMax Code 自己的 SQLite（local_runtime_token_usage），只读打开，不改动它
//   - 文件级指纹：主库与 -wal 的最大 mtime + 合并尺寸，未变则整库跳过（新行先落在 WAL 里，只看主库会漏）
//   - 水位保存在游标的 ParserState 中，旧行被用户裁剪后不会被重新导入
//   - 用量行只有请求开始时间，耗时与首字来自同一轮对应 assistant 消息行里记录的时长
func syncMiniMaxCodeSessionUsage(ls *LogService, scope string, home string) (SessionSyncSourceResult, error) {
	result := SessionSyncSourceResult{Scope: scope, Platform: sessionPlatformMiniMaxCode}
	dbPath := mcodeDatabasePath(home, scope)
	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return result, err
	}

	fingerprintNS := fileModifiedNS(info)
	fingerprintSize := info.Size()
	if walInfo, walErr := os.Stat(dbPath + "-wal"); walErr == nil {
		if walNS := fileModifiedNS(walInfo); walNS > fingerprintNS {
			fingerprintNS = walNS
		}
		fingerprintSize += walInfo.Size()
	}

	cursor, err := loadSessionFileCursor(scope, sessionPlatformMiniMaxCode, dbPath)
	if err != nil {
		return result, err
	}
	if cursor.ParserState != "" && cursor.ModifiedNS == fingerprintNS && cursor.FileSize == fingerprintSize {
		// 库未变化：与其他来源一致，不计入跳过数
		return result, nil
	}
	var state mcodeParserState
	if cursor.ParserState != "" {
		if json.Unmarshal([]byte(cursor.ParserState), &state) != nil {
			state = mcodeParserState{}
		}
	}
	// 计时水位不可能超过用量水位；数据异常时两者对齐
	state.UsageMark = max(state.UsageMark, 0)
	state.TimingMark = max(state.TimingMark, 0)
	if state.TimingMark > state.UsageMark {
		state.TimingMark = state.UsageMark
	}
	result.FilesScanned = 1

	source, err := mcodeOpenReadOnly(dbPath)
	if err != nil {
		return result, err
	}
	defer source.Close()

	rows, err := mcodeQueryUsageRows(source, state.TimingMark)
	if err != nil {
		return result, err
	}

	history := newMcodeHistoryModels(source)
	timing := newMcodeTimingLookup(source)
	settledBefore := time.Now().UnixMilli() - mcodeTimingSettleMs
	settling := true
	timingMark := state.TimingMark
	usageMark := state.UsageMark

	fresh := make([]sessionUsageRecord, 0, len(rows))
	replay := make([]sessionUsageRecord, 0)
	for _, row := range rows {
		latencyMs, firstTokenMs, hasTiming := timing.lookup(row)

		record := newSessionUsageRecordForSource(requestLogDataSourceMiniMaxCodeSession)
		record.Model = mcodeResolveModel(history, row)
		record.SessionID = row.SessionID
		record.InputTokens = row.Input
		record.OutputTokens = row.Output
		record.ReasoningTokens = row.Reasoning
		record.CacheReadTokens = row.CacheRead
		record.CacheCreateTokens = row.CacheWrite
		record.CreatedAt = time.Unix(row.TS/1000, 0).UTC()
		if hasTiming {
			record.DurationSec = float64(latencyMs) / 1000.0
			record.FirstTokenSec = float64(firstTokenMs) / 1000.0
		}
		record.SourceRecordID = stableSessionRecordID(scope, sessionPlatformMiniMaxCode, fmt.Sprintf("mcode:%s:%d", row.SessionID, row.ID))

		if row.ID > state.UsageMark {
			fresh = append(fresh, record)
			usageMark = max(usageMark, row.ID)
		} else if hasTiming {
			// 已导入过的行只用来补计时；是否仍存在在下面统一核对，被裁剪的行不能复活
			replay = append(replay, record)
		}

		// 水位只推过已经对上、或已超过等待期的行
		settling = settling && (hasTiming || row.TS < settledBefore)
		if settling {
			timingMark = row.ID
		}
	}

	if len(replay) > 0 {
		existing, err := existingSessionUsageRecords(replay)
		if err != nil {
			return result, err
		}
		for _, record := range replay {
			if _, found := existing[record.SourceRecordID]; found {
				fresh = append(fresh, record)
			}
		}
	}

	imported, skipped, err := persistSessionUsageRecords(ls, fresh)
	if err != nil {
		return result, err
	}
	result.Imported = imported
	result.Skipped = skipped

	encoded, err := json.Marshal(mcodeParserState{UsageMark: usageMark, TimingMark: timingMark})
	if err != nil {
		return result, err
	}
	cursor.ParserState = string(encoded)
	cursor.ModifiedNS = fingerprintNS
	cursor.FileSize = fingerprintSize
	cursor.ByteOffset = 0
	cursor.LineOffset = 0
	if err := saveSessionFileCursor(cursor); err != nil {
		return result, err
	}
	return result, nil
}

// mcodeOpenReadOnly 以只读方式打开 MiniMax Code 的运行时库（独立连接，不占用主库连接池）
func mcodeOpenReadOnly(dbPath string) (*sql.DB, error) {
	path := filepath.ToSlash(dbPath)
	if len(path) > 1 && path[1] == ':' {
		path = "/" + path
	}
	uri := (&url.URL{Scheme: "file", Path: path}).String() + "?mode=ro"
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("打开 MiniMax Code 运行时库失败: %w", err)
	}
	return db, nil
}

// mcodeQueryUsageRows 读取 id 大于水位的用量行；旧版本没有 turn_id 列时回退为 NULL
func mcodeQueryUsageRows(source *sql.DB, afterID int64) ([]mcodeUsageRow, error) {
	const columns = `id, session_id, %s, model, ts, input_tokens, output_tokens,
		reasoning_tokens, cache_read_tokens, cache_write_tokens`
	query := func(turnColumn string) (*sql.Rows, error) {
		return source.Query(fmt.Sprintf("SELECT "+columns+" FROM local_runtime_token_usage WHERE id > ? ORDER BY id", turnColumn), afterID)
	}
	rows, err := query("turn_id")
	if err != nil {
		var fallbackErr error
		if rows, fallbackErr = query("NULL"); fallbackErr != nil {
			return nil, fmt.Errorf("查询 MiniMax Code 用量失败: %w", err)
		}
	}
	defer rows.Close()

	var result []mcodeUsageRow
	for rows.Next() {
		var row mcodeUsageRow
		var sessionID sql.NullString
		var ts, input, output, reasoning, cacheRead, cacheWrite sql.NullInt64
		if err := rows.Scan(&row.ID, &sessionID, &row.TurnID, &row.Model, &ts, &input, &output, &reasoning, &cacheRead, &cacheWrite); err != nil {
			return nil, err
		}
		row.SessionID = sessionID.String
		row.TS = ts.Int64
		row.Input = int(max(input.Int64, 0))
		row.Output = int(max(output.Int64, 0))
		row.Reasoning = int(max(reasoning.Int64, 0))
		row.CacheRead = int(max(cacheRead.Int64, 0))
		row.CacheWrite = int(max(cacheWrite.Int64, 0))
		result = append(result, row)
	}
	return result, rows.Err()
}

// mcodeResolveModel 用量行自带模型时去掉第一个 / 之前的命名空间（如 custom_provider:router/vendor/model -> vendor/model）；
// 缺失时回查同会话同时间戳的 assistant 历史消息，仍然找不到记为 unknown
func mcodeResolveModel(history *mcodeHistoryModels, row mcodeUsageRow) string {
	if native := strings.TrimSpace(row.Model.String); row.Model.Valid && native != "" {
		if _, model, found := strings.Cut(native, "/"); found {
			return model
		}
		return native
	}
	if model := history.lookup(row.SessionID, row.TS); model != "" {
		return model
	}
	return "unknown"
}

// mcodeTimingLookup 按（会话、轮次、输入 / 输出 token）对上同一轮的 assistant 消息行，读取请求时长
//   - request_duration_ms：整段请求时长；decode_duration_ms：首字之后的解码时长，首字耗时 = 两者之差
//   - 旧版本没有消息行表或字段时 stmt 为空，耗时按 0（无计时）处理
type mcodeTimingLookup struct {
	stmt *sql.Stmt
}

func newMcodeTimingLookup(source *sql.DB) *mcodeTimingLookup {
	stmt, err := source.Prepare(`
		SELECT json_extract(data_json, '$.usage.request_duration_ms'),
		       json_extract(data_json, '$.usage.decode_duration_ms')
		FROM local_runtime_message_rows
		WHERE session_id = ?1 AND role = 'assistant' AND (?2 IS NULL OR turn_id = ?2)
		  AND created_at_ms >= ?3
		  AND json_extract(data_json, '$.usage.input_tokens') = ?4
		  AND json_extract(data_json, '$.usage.output_tokens') = ?5
		ORDER BY created_at_ms LIMIT 1`)
	if err != nil {
		return &mcodeTimingLookup{}
	}
	return &mcodeTimingLookup{stmt: stmt}
}

// lookup 返回 (耗时 ms, 首字 ms, 是否有计时)。缺请求时长即无计时；解码时长不合理时只保留耗时
func (lookup *mcodeTimingLookup) lookup(row mcodeUsageRow) (int64, int64, bool) {
	if lookup.stmt == nil {
		return 0, 0, false
	}
	var request, decode sql.NullFloat64
	err := lookup.stmt.QueryRow(row.SessionID, row.TurnID, row.TS, row.Input, row.Output).Scan(&request, &decode)
	if err != nil || !request.Valid || math.IsNaN(request.Float64) || math.IsInf(request.Float64, 0) || request.Float64 <= 0 {
		return 0, 0, false
	}
	latencyMs := int64(math.Round(request.Float64))
	firstTokenMs := int64(0)
	if decode.Valid && !math.IsNaN(decode.Float64) && decode.Float64 >= 0 && decode.Float64 <= request.Float64 {
		firstTokenMs = int64(math.Round(request.Float64 - decode.Float64))
	}
	return latencyMs, firstTokenMs, true
}

// mcodeHistoryModels 回查缺失的模型名：用量行的 ts 取自产生它的 assistant 消息的 timestamp，而 pi 历史里的消息自带 model。
// 新版历史在 local_runtime_pi_history_rows，按列匹配；旧版整段 JSON 存在 local_runtime_messages.pi_history_json，
// 每个会话只解析一次。哪张表不存在就跳过哪张
type mcodeHistoryModels struct {
	rows            *sql.Stmt
	legacy          *sql.Stmt
	legacyBySession map[string]map[int64]string
}

func newMcodeHistoryModels(source *sql.DB) *mcodeHistoryModels {
	history := &mcodeHistoryModels{legacyBySession: make(map[string]map[int64]string)}
	history.rows, _ = source.Prepare(`
		SELECT json_extract(data_json, '$.model') FROM local_runtime_pi_history_rows
		WHERE session_id = ? AND created_at_ms = ? AND role = 'assistant'`)
	history.legacy, _ = source.Prepare(`
		SELECT json_extract(message.value, '$.timestamp'),
		       json_extract(message.value, '$.model')
		FROM local_runtime_messages, json_each(pi_history_json) AS message
		WHERE session_id = ? AND json_extract(message.value, '$.role') = 'assistant'`)
	return history
}

func (history *mcodeHistoryModels) lookup(sessionID string, ts int64) string {
	if model := history.lookupRows(sessionID, ts); model != "" {
		return model
	}
	return history.lookupLegacy(sessionID, ts)
}

func (history *mcodeHistoryModels) lookupRows(sessionID string, ts int64) string {
	if history.rows == nil {
		return ""
	}
	rows, err := history.rows.Query(sessionID, ts)
	if err != nil {
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var model sql.NullString
		if rows.Scan(&model) == nil && strings.TrimSpace(model.String) != "" {
			return model.String
		}
	}
	return ""
}

func (history *mcodeHistoryModels) lookupLegacy(sessionID string, ts int64) string {
	models, loaded := history.legacyBySession[sessionID]
	if !loaded {
		models = make(map[int64]string)
		if history.legacy != nil {
			if rows, err := history.legacy.Query(sessionID); err == nil {
				for rows.Next() {
					var timestamp sql.NullFloat64
					var model sql.NullString
					if rows.Scan(&timestamp, &model) != nil || !timestamp.Valid || math.IsNaN(timestamp.Float64) || math.IsInf(timestamp.Float64, 0) {
						continue
					}
					if strings.TrimSpace(model.String) == "" {
						continue
					}
					if _, exists := models[int64(timestamp.Float64)]; !exists {
						models[int64(timestamp.Float64)] = model.String
					}
				}
				_ = rows.Close()
			}
		}
		history.legacyBySession[sessionID] = models
	}
	return models[ts]
}
