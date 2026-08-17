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

	sessionUsageDedupWindowSeconds = 10 * 60
)

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
	sessionSQL := fmt.Sprintf(
		"%s IN ('%s', '%s', '%s', '%s', '%s')",
		sourceSQL,
		requestLogDataSourceClaudeSession,
		requestLogDataSourceCodexSession,
		requestLogDataSourceGeminiSession,
		requestLogDataSourceOpenCodeSession,
		requestLogDataSourceGrokSession,
	)

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
	switch strings.ToLower(strings.TrimSpace(source)) {
	case requestLogDataSourceClaudeSession, requestLogDataSourceCodexSession, requestLogDataSourceGeminiSession, requestLogDataSourceOpenCodeSession, requestLogDataSourceGrokSession:
		return true
	default:
		return false
	}
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
