/**
 * @name: 请求日志数据来源
 * @Descripttion: 定义代理与会话日志来源、持久化去重结构及查询谓词。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-17 17:01:12
 * @LastEditTime: 2026-07-17 17:01:12
 * @FilePath: services/requestlog_source.go
 */

package services

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/daodao97/xgo/xdb"
)

type LogDataSourceMode string

const (
	LogDataSourceModeProxy   LogDataSourceMode = "proxy"
	LogDataSourceModeSession LogDataSourceMode = "session"
	LogDataSourceModeAll     LogDataSourceMode = "all"

	requestLogDataSourceProxy           = "proxy"
	requestLogDataSourceClaudeSession   = "session_log"
	requestLogDataSourceCodexSession    = "codex_session"
	requestLogDataSourceGeminiSession   = "gemini_session"
	requestLogDataSourceOpenCodeSession = "opencode_session"
	requestLogDataSourceGrokSession     = "grok_session"

	requestLogProviderIDClaudeSession   = "_session"
	requestLogProviderIDCodexSession    = "_codex_session"
	requestLogProviderIDGeminiSession   = "_gemini_session"
	requestLogProviderIDOpenCodeSession = "_opencode_session"
	requestLogProviderIDGrokSession     = "_grok_session"

	requestLogProviderClaudeSession   = "Claude 会话"
	requestLogProviderCodexSession    = "Codex 会话"
	requestLogProviderGeminiSession   = "Gemini 会话"
	requestLogProviderOpenCodeSession = "OpenCode 会话"
	requestLogProviderGrokSession     = "Grok 会话"

	requestLogDataSourceAntigravitySession     = "agy_session"
	requestLogDataSourceKimiSession            = "kimi_session"
	requestLogDataSourceCopilotSession         = "copilot_session"
	requestLogDataSourceWorkBuddySession       = "workbuddy_session"
	requestLogDataSourceDeepSeekHarnessSession = "dsh_session"
	requestLogDataSourceMiniMaxCodeSession     = "mcode_session"
	requestLogDataSourceOhMyPiSession          = "omp_session"
	requestLogDataSourcePiSession              = "pi_session"

	requestLogProviderIDAntigravitySession     = "_agy_session"
	requestLogProviderIDKimiSession            = "_kimi_session"
	requestLogProviderIDCopilotSession         = "_copilot_session"
	requestLogProviderIDWorkBuddySession       = "_workbuddy_session"
	requestLogProviderIDDeepSeekHarnessSession = "_dsh_session"
	requestLogProviderIDMiniMaxCodeSession     = "_mcode_session"
	requestLogProviderIDOhMyPiSession          = "_omp_session"
	requestLogProviderIDPiSession              = "_pi_session"

	requestLogProviderAntigravitySession     = "Antigravity 会话"
	requestLogProviderKimiSession            = "Kimi 会话"
	requestLogProviderCopilotSession         = "Copilot 会话"
	requestLogProviderWorkBuddySession       = "WorkBuddy 会话"
	requestLogProviderDeepSeekHarnessSession = "DeepSeek Harness 会话"
	requestLogProviderMiniMaxCodeSession     = "MiniMax Code 会话"
	requestLogProviderOhMyPiSession          = "Oh My Pi 会话"
	requestLogProviderPiSession              = "Pi 会话"

	// 会话来源对应的 request_log.platform（与首页供应商 Tab id 保持一致）
	sessionPlatformAntigravity     = "antigravity"
	sessionPlatformKimi            = "kimi"
	sessionPlatformCopilot         = "copilot"
	sessionPlatformWorkBuddy       = "workbuddy"
	sessionPlatformDeepSeekHarness = "deepseek-harness"
	sessionPlatformMiniMaxCode     = "minimax-code"
	sessionPlatformOhMyPi          = "omp"
	sessionPlatformPi              = "pi"

	sessionUsageDedupWindowSeconds = 10 * 60
)

// sessionUsageSourceDef 描述一种本地会话用量来源在 request_log 中的固定标识
type sessionUsageSourceDef struct {
	Platform     string
	DataSource   string
	ProviderID   string
	ProviderName string
}

// sessionUsageSourceDefs 全部会话来源注册表；来源谓词与判定函数都从这里生成，新增来源只改这里
var sessionUsageSourceDefs = []sessionUsageSourceDef{
	{Platform: "claude", DataSource: requestLogDataSourceClaudeSession, ProviderID: requestLogProviderIDClaudeSession, ProviderName: requestLogProviderClaudeSession},
	{Platform: "codex", DataSource: requestLogDataSourceCodexSession, ProviderID: requestLogProviderIDCodexSession, ProviderName: requestLogProviderCodexSession},
	{Platform: "gemini", DataSource: requestLogDataSourceGeminiSession, ProviderID: requestLogProviderIDGeminiSession, ProviderName: requestLogProviderGeminiSession},
	{Platform: "opencode", DataSource: requestLogDataSourceOpenCodeSession, ProviderID: requestLogProviderIDOpenCodeSession, ProviderName: requestLogProviderOpenCodeSession},
	{Platform: "grokbuild", DataSource: requestLogDataSourceGrokSession, ProviderID: requestLogProviderIDGrokSession, ProviderName: requestLogProviderGrokSession},
	{Platform: sessionPlatformAntigravity, DataSource: requestLogDataSourceAntigravitySession, ProviderID: requestLogProviderIDAntigravitySession, ProviderName: requestLogProviderAntigravitySession},
	{Platform: sessionPlatformKimi, DataSource: requestLogDataSourceKimiSession, ProviderID: requestLogProviderIDKimiSession, ProviderName: requestLogProviderKimiSession},
	{Platform: sessionPlatformCopilot, DataSource: requestLogDataSourceCopilotSession, ProviderID: requestLogProviderIDCopilotSession, ProviderName: requestLogProviderCopilotSession},
	{Platform: sessionPlatformWorkBuddy, DataSource: requestLogDataSourceWorkBuddySession, ProviderID: requestLogProviderIDWorkBuddySession, ProviderName: requestLogProviderWorkBuddySession},
	{Platform: sessionPlatformDeepSeekHarness, DataSource: requestLogDataSourceDeepSeekHarnessSession, ProviderID: requestLogProviderIDDeepSeekHarnessSession, ProviderName: requestLogProviderDeepSeekHarnessSession},
	{Platform: sessionPlatformMiniMaxCode, DataSource: requestLogDataSourceMiniMaxCodeSession, ProviderID: requestLogProviderIDMiniMaxCodeSession, ProviderName: requestLogProviderMiniMaxCodeSession},
	{Platform: sessionPlatformOhMyPi, DataSource: requestLogDataSourceOhMyPiSession, ProviderID: requestLogProviderIDOhMyPiSession, ProviderName: requestLogProviderOhMyPiSession},
	{Platform: sessionPlatformPi, DataSource: requestLogDataSourcePiSession, ProviderID: requestLogProviderIDPiSession, ProviderName: requestLogProviderPiSession},
}

// sessionUsageSourceByDataSource 按 data_source 查找来源定义
func sessionUsageSourceByDataSource(dataSource string) (sessionUsageSourceDef, bool) {
	normalized := strings.ToLower(strings.TrimSpace(dataSource))
	for _, def := range sessionUsageSourceDefs {
		if def.DataSource == normalized {
			return def, true
		}
	}
	return sessionUsageSourceDef{}, false
}

// newSessionUsageRecordForSource 生成带固定来源标识的会话用量记录骨架
func newSessionUsageRecordForSource(dataSource string) sessionUsageRecord {
	def, _ := sessionUsageSourceByDataSource(dataSource)
	return sessionUsageRecord{
		Platform:   def.Platform,
		ProviderID: def.ProviderID,
		Provider:   def.ProviderName,
		DataSource: def.DataSource,
	}
}

// sessionDataSourceSQLList 生成会话来源 IN 列表（全部为内部常量，可安全拼接）
func sessionDataSourceSQLList() string {
	quoted := make([]string, 0, len(sessionUsageSourceDefs))
	for _, def := range sessionUsageSourceDefs {
		quoted = append(quoted, "'"+def.DataSource+"'")
	}
	return strings.Join(quoted, ", ")
}

func normalizeLogDataSourceMode(value string) LogDataSourceMode {
	switch LogDataSourceMode(strings.ToLower(strings.TrimSpace(value))) {
	case LogDataSourceModeSession:
		return LogDataSourceModeSession
	case LogDataSourceModeAll:
		return LogDataSourceModeAll
	default:
		return LogDataSourceModeProxy
	}
}

func requestLogDataSourceSQL(alias string) string {
	prefix := ""
	if trimmed := strings.TrimSpace(alias); trimmed != "" {
		prefix = trimmed + "."
	}
	return fmt.Sprintf("COALESCE(NULLIF(TRIM(%sdata_source), ''), '%s')", prefix, requestLogDataSourceProxy)
}

func requestLogSourceWhereClause(mode LogDataSourceMode, alias string) string {
	mode = normalizeLogDataSourceMode(string(mode))
	sourceSQL := requestLogDataSourceSQL(alias)
	prefix := ""
	if trimmed := strings.TrimSpace(alias); trimmed != "" {
		prefix = trimmed + "."
	}
	sessionSQL := fmt.Sprintf("%s IN (%s)", sourceSQL, sessionDataSourceSQLList())

	switch mode {
	case LogDataSourceModeSession:
		return sessionSQL
	case LogDataSourceModeAll:
		return fmt.Sprintf(
			"(%s = '%s' OR (%s AND NOT EXISTS (SELECT 1 FROM session_usage_dedup source_dedup WHERE source_dedup.session_log_id = %sid)))",
			sourceSQL,
			requestLogDataSourceProxy,
			sessionSQL,
			prefix,
		)
	default:
		return fmt.Sprintf("%s = '%s'", sourceSQL, requestLogDataSourceProxy)
	}
}

func requestLogSourceFilterOption(mode LogDataSourceMode) xdb.Option {
	return xdb.WhereRaw(requestLogSourceWhereClause(mode, "request_log"))
}

func buildRequestLogDedupCore(platform string, inputTokens int, outputTokens int, cacheReadTokens int) string {
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(platform)),
		strconv.Itoa(max(inputTokens, 0)),
		strconv.Itoa(max(outputTokens, 0)),
		strconv.Itoa(max(cacheReadTokens, 0)),
	}, "|")
}

func isSessionRequestLogSource(source string) bool {
	_, ok := sessionUsageSourceByDataSource(source)
	return ok
}

func ensureRequestLogSourceStorageWithDB(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS session_log_sync (
			sync_key TEXT PRIMARY KEY,
			source_scope TEXT NOT NULL DEFAULT 'native',
			platform TEXT NOT NULL,
			file_path TEXT NOT NULL,
			modified_ns INTEGER NOT NULL DEFAULT 0,
			file_size INTEGER NOT NULL DEFAULT 0,
			byte_offset INTEGER NOT NULL DEFAULT 0,
			line_offset INTEGER NOT NULL DEFAULT 0,
			parser_state TEXT NOT NULL DEFAULT '',
			last_synced_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS session_usage_dedup (
			session_log_id INTEGER NOT NULL PRIMARY KEY,
			proxy_log_id INTEGER NOT NULL UNIQUE,
			matched_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS session_usage_dedup_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			last_proxy_log_id INTEGER NOT NULL DEFAULT 0,
			last_session_log_id INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`INSERT OR IGNORE INTO session_usage_dedup_state (id) VALUES (1)`,
		// 会话用量永久回执：会话行一旦入库即登记，用户删除明细后不会因会话文件重读而复活
		`CREATE TABLE IF NOT EXISTS session_usage_receipt (
			source_record_id TEXT NOT NULL PRIMARY KEY,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_request_log_source_record_id
			ON request_log(source_record_id) WHERE source_record_id <> ''`,
		`CREATE INDEX IF NOT EXISTS idx_request_log_source_dedup_created
			ON request_log(data_source, dedup_core, created_at, id)`,
		`CREATE INDEX IF NOT EXISTS idx_request_log_source_platform_created
			ON request_log(data_source, platform, created_at)`,
		`CREATE TRIGGER IF NOT EXISTS request_log_session_dedup_ad
			AFTER DELETE ON request_log
			BEGIN
				DELETE FROM session_usage_dedup
				WHERE session_log_id = OLD.id OR proxy_log_id = OLD.id;
			END`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}

	const migrationKey = "request_log_source_v1_backfill"
	if err := ensureSchemaMigrationsTable(db); err != nil {
		return err
	}
	if err := ensureSessionLogSyncBackfillColumn(db); err != nil {
		return err
	}
	if err := backfillSessionUsageReceipts(db); err != nil {
		return err
	}
	if err := resetSessionCursorsForTimingBackfill(db); err != nil {
		return err
	}
	applied, err := isSchemaMigrationApplied(db, migrationKey)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	if _, err := db.Exec(`
		UPDATE request_log
		SET data_source = 'proxy'
		WHERE TRIM(COALESCE(data_source, '')) = ''
	`); err != nil {
		return err
	}
	if _, err := db.Exec(`
		UPDATE request_log
		SET dedup_core = LOWER(TRIM(COALESCE(platform, ''))) || '|' ||
			MAX(COALESCE(input_tokens, 0), 0) || '|' ||
			MAX(COALESCE(output_tokens, 0), 0) || '|' ||
			MAX(COALESCE(cache_read_tokens, 0), 0)
		WHERE TRIM(COALESCE(dedup_core, '')) = ''
	`); err != nil {
		return err
	}
	return markSchemaMigrationApplied(db, migrationKey, "1")
}

// ensureSessionLogSyncBackfillColumn 为会话游标补充 backfill_line_offset 列：
// 该行号之前的记录已在升级前导入过，重新解析时只允许更新已有行，不再新建
func ensureSessionLogSyncBackfillColumn(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(session_log_sync)")
	if err != nil {
		return err
	}
	exists := false
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if name == "backfill_line_offset" {
			exists = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = db.Exec("ALTER TABLE session_log_sync ADD COLUMN backfill_line_offset INTEGER NOT NULL DEFAULT 0")
	return err
}

// backfillSessionUsageReceipts 一次性为已存在的会话行补登回执
func backfillSessionUsageReceipts(db *sql.DB) error {
	const migrationKey = "session_usage_receipt_v1_backfill"
	applied, err := isSchemaMigrationApplied(db, migrationKey)
	if err != nil || applied {
		return err
	}
	if _, err := db.Exec(fmt.Sprintf(`
		INSERT OR IGNORE INTO session_usage_receipt (source_record_id)
		SELECT source_record_id FROM request_log
		WHERE TRIM(COALESCE(source_record_id, '')) <> '' AND %s IN (%s)
	`, requestLogDataSourceSQL("request_log"), sessionDataSourceSQLList())); err != nil {
		return err
	}
	return markSchemaMigrationApplied(db, migrationKey, "1")
}

// resetSessionCursorsForTimingBackfill 一次性让 Claude/Codex 会话从头重新解析以补写估算的耗时与首字。
// 原游标行号记入 backfill_line_offset：此前已导入的部分只更新已有行（补计时），
// 用户此前删除的历史明细不会被重新导入
func resetSessionCursorsForTimingBackfill(db *sql.DB) error {
	const migrationKey = "session_usage_timing_v1_cursor_reset"
	applied, err := isSchemaMigrationApplied(db, migrationKey)
	if err != nil || applied {
		return err
	}
	if _, err := db.Exec(`
		UPDATE session_log_sync
		SET backfill_line_offset = MAX(backfill_line_offset, line_offset),
			byte_offset = 0, line_offset = 0, modified_ns = 0, file_size = 0, parser_state = ''
		WHERE platform IN ('claude', 'codex')
	`); err != nil {
		return err
	}
	return markSchemaMigrationApplied(db, migrationKey, "1")
}
