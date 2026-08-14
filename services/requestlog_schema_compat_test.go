package services

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/daodao97/xgo/xdb"
)

func TestEnsureRequestLogTableWithDB_RepairsDanglingStatsTrigger(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	legacySQL := []string{
		`DROP TRIGGER IF EXISTS request_log_stats_hourly_ai`,
		`DROP TRIGGER IF EXISTS request_log_stats_daily_ai`,
		`DROP TABLE IF EXISTS request_log_stats_hourly`,
		`DROP TABLE IF EXISTS request_log_stats_daily`,
		`DROP TABLE IF EXISTS request_log`,
		`CREATE TABLE request_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			platform TEXT,
			model TEXT,
			provider TEXT,
			http_code INTEGER,
			input_tokens INTEGER,
			output_tokens INTEGER,
			cache_create_tokens INTEGER,
			cache_read_tokens INTEGER,
			reasoning_tokens INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE request_log_stats_hourly (
			bucket_start TEXT NOT NULL,
			platform TEXT NOT NULL DEFAULT '',
			provider_id TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			total_requests INTEGER NOT NULL DEFAULT 0,
			successful_requests INTEGER NOT NULL DEFAULT 0,
			failed_requests INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			reasoning_tokens INTEGER NOT NULL DEFAULT 0,
			cache_create_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			total_cost REAL NOT NULL DEFAULT 0,
			PRIMARY KEY (bucket_start, platform, provider_id)
		) WITHOUT ROWID`,
		`CREATE TRIGGER request_log_stats_hourly_ai
		AFTER INSERT ON request_log
		BEGIN
			INSERT INTO request_log_stats_hourly (
				bucket_start, platform, provider_id, provider,
				total_requests, successful_requests, failed_requests,
				input_tokens, output_tokens, reasoning_tokens, cache_create_tokens, cache_read_tokens,
				total_cost
			) VALUES (
				'1970-01-01 00:00:00', '', '', '',
				1, 1, 0,
				0, 0, 0, 0, 0,
				0
			);
		END;`,
		`DROP TABLE request_log_stats_hourly`,
	}
	for _, stmt := range legacySQL {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("构造旧库数据失败: %v, sql=%s", err, stmt)
		}
	}

	if err := ensureRequestLogTableWithDB(db); err != nil {
		t.Fatalf("ensureRequestLogTableWithDB 升级失败: %v", err)
	}

	assertRequestLogColumnExists(t, db, "provider_id")
	assertRequestLogColumnExists(t, db, "requested_model")
	assertRequestLogColumnExists(t, db, "mapped_model")
	assertRequestLogColumnExists(t, db, "model_mapping_pattern")
	assertRequestLogColumnExists(t, db, "model_mapping_target")
	assertRequestLogColumnExists(t, db, "model_override")
	assertRequestLogColumnExists(t, db, "model_route_captured")
	assertRequestLogColumnExists(t, db, "session_preferred_provider_id")
	assertRequestLogColumnExists(t, db, "session_preferred_provider")
	assertRequestLogColumnExists(t, db, "session_provider_route")
	assertRequestLogColumnExists(t, db, "reasoning_effort_source")
	assertRequestLogColumnExists(t, db, "stream_last_event")
	assertRequestLogColumnExists(t, db, "stream_terminal_event")
	assertRequestLogColumnExists(t, db, "stream_error_kind")
	assertRequestLogColumnExists(t, db, "error_message")
	assertRequestLogColumnExists(t, db, "request_outcome")
	assertRequestLogColumnExists(t, db, "outcome_reason")
	assertRequestLogColumnExists(t, db, "stream_compaction_requested")
	assertRequestLogColumnExists(t, db, "stream_compaction_observed")
	assertRequestLogColumnExists(t, db, "stream_bytes")
	assertRequestLogColumnExists(t, db, "upstream_protocol")
	assertRequestLogColumnExists(t, db, "user_agent")
	assertRequestLogColumnExists(t, db, "first_token_sec")
	assertRequestLogColumnExists(t, db, "proxy_prepare_ms")
	assertRequestLogColumnExists(t, db, "dns_ms")
	assertRequestLogColumnExists(t, db, "connect_ms")
	assertRequestLogColumnExists(t, db, "tls_ms")
	assertRequestLogColumnExists(t, db, "upstream_ttfb_ms")
	assertRequestLogColumnExists(t, db, "proxy_stream_delay_ms")
	assertRequestLogColumnExists(t, db, "connection_reused")
	assertRequestLogColumnExists(t, db, "group_multiplier")
	assertRequestLogColumnExists(t, db, "provider_per_call_output_set")
	assertRequestLogColumnExists(t, db, "request_body")
	assertRequestLogColumnExists(t, db, "response_body")
	assertRequestLogColumnExists(t, db, "request_body_truncated")
	assertRequestLogColumnExists(t, db, "response_body_truncated")
	assertRequestLogColumnExists(t, db, "payload_bytes")
	assertRequestLogColumnExists(t, db, "payload_captured")
	assertRequestLogColumnExists(t, db, "error_read_at")

	assertSQLiteObjectExists(t, db, "table", requestLogStatsHourlyTable)
	assertSQLiteObjectExists(t, db, "table", requestLogStatsDailyTable)
	assertSQLiteObjectExists(t, db, "trigger", "request_log_stats_hourly_ai")
	assertSQLiteObjectExists(t, db, "trigger", "request_log_stats_daily_ai")
	assertTableColumnExists(t, db, requestLogStatsHourlyTable, "excluded_requests")
	assertTableColumnExists(t, db, requestLogStatsDailyTable, "excluded_requests")

	if _, err := db.Exec(`
		INSERT INTO request_log (
			platform, model, provider, http_code, input_tokens, output_tokens, cache_read_tokens
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "codex", "gpt-5", "compat-provider", 200, 10, 5, 2); err != nil {
		t.Fatalf("插入 request_log 失败: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO request_log (
			platform, model, provider, http_code, request_outcome, outcome_reason
		) VALUES (?, ?, ?, ?, ?, ?)
	`, "codex", "gpt-5", "compat-provider", 499, requestOutcomeExcluded, requestOutcomeReasonClientAbort); err != nil {
		t.Fatalf("插入 excluded request_log 失败: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO request_log (
			platform, model, provider, http_code, request_outcome
		) VALUES (?, ?, ?, ?, ?), (?, ?, ?, ?, ?)
	`,
		"codex", "gpt-5", "compat-provider", 503, "unknown",
		"codex", "gpt-5", "compat-provider", 503, "  "+requestOutcomeSuccess+"  ",
	); err != nil {
		t.Fatalf("插入结果回退 request_log 失败: %v", err)
	}

	var total, successful, failed, excluded int64
	if err := db.QueryRow(`
		SELECT total_requests, successful_requests, failed_requests, excluded_requests
		FROM request_log_stats_daily
		WHERE provider_id = ?
	`, "compat-provider").Scan(&total, &successful, &failed, &excluded); err != nil {
		t.Fatalf("查询聚合结果失败: %v", err)
	}
	if total != 4 || successful != 2 || failed != 1 || excluded != 1 {
		t.Fatalf("聚合三态错误: total=%d successful=%d failed=%d excluded=%d", total, successful, failed, excluded)
	}
}

func assertTableColumnExists(t *testing.T, db *sql.DB, table string, column string) {
	t.Helper()
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", table)
	if err := db.QueryRow(query, column).Scan(&count); err != nil {
		t.Fatalf("查询 %s 字段失败: %v", table, err)
	}
	if count != 1 {
		t.Fatalf("%s 缺少字段 %s", table, column)
	}
}

func assertRequestLogColumnExists(t *testing.T, db *sql.DB, column string) {
	t.Helper()
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('request_log') WHERE name = ?`,
		column,
	).Scan(&count); err != nil {
		t.Fatalf("查询 request_log 字段失败: %v", err)
	}
	if count == 0 {
		t.Fatalf("request_log 缺少字段: %s", column)
	}
}

func assertSQLiteObjectExists(t *testing.T, db *sql.DB, objectType string, name string) {
	t.Helper()
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?`,
		objectType,
		name,
	).Scan(&count); err != nil {
		t.Fatalf("查询 sqlite_master 失败: %v", err)
	}
	if count == 0 {
		t.Fatalf("缺少 sqlite 对象: type=%s name=%s", objectType, name)
	}
}
