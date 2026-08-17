/**
 * @name: 会话用量同步测试
 * @Descripttion: 验证三类会话增量导入与持久化来源去重行为。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-17 17:01:12
 * @LastEditTime: 2026-07-17 17:01:12
 * @FilePath: services/session_usage_test.go
 */

package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
)

func prepareSessionUsageTest(t *testing.T) (*LogService, *sql.DB, string) {
	t.Helper()
	if GlobalDBQueue != nil || GlobalDBQueueLogs != nil {
		if err := ShutdownGlobalDBQueue(5 * time.Second); err != nil {
			t.Fatalf("ShutdownGlobalDBQueue: %v", err)
		}
		GlobalDBQueue = nil
		GlobalDBQueueLogs = nil
	}
	dbPath := filepath.Join(t.TempDir(), "app.db?cache=shared&mode=rwc")
	if err := xdb.Inits([]xdb.Config{{Name: "default", Driver: "sqlite", DSN: dbPath}}); err != nil {
		t.Fatalf("initialize test database: %v", err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("xdb.DB: %v", err)
	}
	if err := ensureRequestLogTableWithDB(db); err != nil {
		t.Fatalf("ensureRequestLogTableWithDB: %v", err)
	}
	if err := InitGlobalDBQueue(); err != nil {
		t.Fatalf("InitGlobalDBQueue: %v", err)
	}
	scope := fmt.Sprintf("test-session-usage:%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_ = ShutdownGlobalDBQueue(5 * time.Second)
		GlobalDBQueue = nil
		GlobalDBQueueLogs = nil
		_ = db.Close()
		// 恢复 TestMain 的全局测试库与队列，避免污染后续测试
		restoreGlobalTestDatabase(t)
	})
	return NewLogService(nil), db, scope
}

func TestClaudeSessionUsageIncrementalSync(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	path := filepath.Join(t.TempDir(), "claude.jsonl")
	sessionID := scope + ":claude"
	first := fmt.Sprintf(`{"type":"assistant","message":{"id":"msg-1","model":"claude-sonnet-4","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":3,"cache_creation_input_tokens":2}},"timestamp":"2026-07-17T08:00:00Z","sessionId":%q}`, sessionID) + "\n"
	if err := os.WriteFile(path, []byte(first), 0o600); err != nil {
		t.Fatal(err)
	}

	imported, skipped, err := syncClaudeSessionFile(service, scope, path)
	if err != nil || imported != 1 || skipped != 0 {
		t.Fatalf("first sync = (%d, %d, %v)", imported, skipped, err)
	}
	imported, skipped, err = syncClaudeSessionFile(service, scope, path)
	if err != nil || imported != 0 || skipped != 0 {
		t.Fatalf("unchanged sync = (%d, %d, %v)", imported, skipped, err)
	}

	second := fmt.Sprintf(`{"type":"assistant","message":{"id":"msg-2","model":"claude-sonnet-4","usage":{"input_tokens":12,"output_tokens":6,"cache_read_input_tokens":4,"cache_creation_input_tokens":1}},"timestamp":"2026-07-17T08:01:00Z","sessionId":%q}`, sessionID) + "\n"
	final := fmt.Sprintf(`{"type":"assistant","message":{"id":"msg-2","model":"claude-sonnet-4","stop_reason":"end_turn","usage":{"input_tokens":12,"output_tokens":8,"cache_read_input_tokens":4,"cache_creation_input_tokens":1}},"timestamp":"2026-07-17T08:01:01Z","sessionId":%q}`, sessionID) + "\n"
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(second + final); err != nil {
		file.Close()
		t.Fatal(err)
	}
	_ = file.Close()
	imported, skipped, err = syncClaudeSessionFile(service, scope, path)
	if err != nil || imported != 1 || skipped != 0 {
		t.Fatalf("append sync = (%d, %d, %v)", imported, skipped, err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM request_log WHERE data_source = ? AND session_id = ?", requestLogDataSourceClaudeSession, sessionID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("claude session rows = %d, want 2", count)
	}
	var outputTokens int
	if err := db.QueryRow("SELECT SUM(output_tokens) FROM request_log WHERE data_source = ? AND session_id = ?", requestLogDataSourceClaudeSession, sessionID).Scan(&outputTokens); err != nil {
		t.Fatal(err)
	}
	if outputTokens != 13 {
		t.Fatalf("claude output tokens = %d, want 13", outputTokens)
	}
}

func TestClaudeSessionUsageUpdatesSnapshotAcrossSyncs(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	path := filepath.Join(t.TempDir(), "claude.jsonl")
	sessionID := scope + ":claude-update"
	first := fmt.Sprintf(`{"type":"assistant","message":{"id":"msg-update","model":"claude-sonnet-4","usage":{"input_tokens":10,"output_tokens":1}},"timestamp":"2026-07-17T08:00:00Z","sessionId":%q}`, sessionID) + "\n"
	if err := os.WriteFile(path, []byte(first), 0o600); err != nil {
		t.Fatal(err)
	}
	if imported, skipped, err := syncClaudeSessionFile(service, scope, path); err != nil || imported != 1 || skipped != 0 {
		t.Fatalf("first sync = (%d, %d, %v)", imported, skipped, err)
	}

	final := fmt.Sprintf(`{"type":"assistant","message":{"id":"msg-update","model":"claude-sonnet-4","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":8}},"timestamp":"2026-07-17T08:00:05Z","sessionId":%q}`, sessionID) + "\n"
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(final); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	_ = file.Close()
	if imported, skipped, err := syncClaudeSessionFile(service, scope, path); err != nil || imported != 1 || skipped != 0 {
		t.Fatalf("final sync = (%d, %d, %v)", imported, skipped, err)
	}

	var count int
	var output int
	if err := db.QueryRow(`
		SELECT COUNT(*), COALESCE(MAX(output_tokens), 0)
		FROM request_log WHERE data_source = ? AND session_id = ?
	`, requestLogDataSourceClaudeSession, sessionID).Scan(&count, &output); err != nil {
		t.Fatal(err)
	}
	if count != 1 || output != 8 {
		t.Fatalf("claude updated row = (%d, %d), want (1, 8)", count, output)
	}
}

func TestNormalizeSessionModelMatchesCodexSessionNaming(t *testing.T) {
	tests := map[string]string{
		"openai/GPT-5.4-2026-03-05": "gpt-5.4",
		"openai/gpt-5.4-20260305":   "gpt-5.4",
		"claude-opus-4-6-20260206":  "claude-opus-4-6",
		"gpt-5.2-codex":             "gpt-5.2-codex",
	}
	for input, want := range tests {
		if got := normalizeSessionModel(input); got != want {
			t.Fatalf("normalizeSessionModel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCodexSessionUsagePersistsCumulativeParserState(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	path := filepath.Join(t.TempDir(), "codex.jsonl")
	sessionID := scope + ":codex"
	lines := []string{
		fmt.Sprintf(`{"type":"session_meta","payload":{"id":%q,"session_id":%q}}`, sessionID, sessionID),
		`{"type":"turn_context","payload":{"model":"openai/gpt-5-codex"}}`,
		`{"type":"event_msg","timestamp":"2026-07-17T08:00:00Z","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":20,"output_tokens":10}}}}`,
	}
	if err := os.WriteFile(path, []byte(stringsJoinLines(lines)), 0o600); err != nil {
		t.Fatal(err)
	}
	if imported, _, err := syncCodexSessionFile(service, scope, path); err != nil || imported != 1 {
		t.Fatalf("first codex sync = (%d, %v)", imported, err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.WriteString(`{"type":"event_msg","timestamp":"2026-07-17T08:01:00Z","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":150,"cached_input_tokens":30,"output_tokens":20}}}}` + "\n")
	_ = file.Close()
	if imported, _, err := syncCodexSessionFile(service, scope, path); err != nil || imported != 1 {
		t.Fatalf("second codex sync = (%d, %v)", imported, err)
	}

	rows, err := db.Query(`
		SELECT input_tokens, cache_read_tokens, output_tokens
		FROM request_log WHERE data_source = ? AND session_id = ? ORDER BY created_at
	`, requestLogDataSourceCodexSession, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := [][3]int{}
	for rows.Next() {
		var values [3]int
		if err := rows.Scan(&values[0], &values[1], &values[2]); err != nil {
			t.Fatal(err)
		}
		got = append(got, values)
	}
	want := [][3]int{{80, 20, 10}, {40, 10, 10}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("codex tokens = %v, want %v", got, want)
	}
}

func TestCodexArchivedSessionInheritsCursor(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "sessions", "2026", "07", "17")
	archivedDir := filepath.Join(root, "archived_sessions")
	if err := os.MkdirAll(sessionsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(archivedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sessionsDir, "rollout-test.jsonl")
	lines := strings.Join([]string{
		`{"type":"session_meta","payload":{"id":"archive-thread","session_id":"archive-thread"}}`,
		`{"type":"turn_context","payload":{"model":"gpt-5.4"}}`,
		`{"type":"event_msg","timestamp":"2026-07-17T08:00:00Z","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":20,"output_tokens":10}}}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	if imported, skipped, err := syncCodexSessionFile(service, scope, path); err != nil || imported != 1 || skipped != 0 {
		t.Fatalf("initial sync = (%d, %d, %v)", imported, skipped, err)
	}

	archivedPath := filepath.Join(archivedDir, filepath.Base(path))
	if err := os.Rename(path, archivedPath); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(archivedPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"type":"event_msg","timestamp":"2026-07-17T08:01:00Z","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":150,"cached_input_tokens":30,"output_tokens":20}}}}` + "\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	_ = file.Close()
	if imported, skipped, err := syncCodexSessionFile(service, scope, archivedPath); err != nil || imported != 1 || skipped != 0 {
		t.Fatalf("archived sync = (%d, %d, %v)", imported, skipped, err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM request_log WHERE data_source = ? AND session_id = ?", requestLogDataSourceCodexSession, "archive-thread").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("codex archived rows = %d, want 2", count)
	}
}

func TestGeminiSessionUsageOnlyImportsAppendedMessages(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	path := filepath.Join(t.TempDir(), "session-test.json")
	sessionID := scope + ":gemini"
	writeGeminiSessionFile(t, path, sessionID, []map[string]any{
		{"type": "gemini", "id": "m1", "model": "gemini-2.5-pro", "timestamp": "2026-07-17T08:00:00Z", "tokens": map[string]any{"input": 100, "cached": 20, "output": 10, "thoughts": 5}},
	})
	if imported, _, err := syncGeminiSessionFile(service, scope, path); err != nil || imported != 1 {
		t.Fatalf("first gemini sync = (%d, %v)", imported, err)
	}
	writeGeminiSessionFile(t, path, sessionID, []map[string]any{
		{"type": "gemini", "id": "m1", "model": "gemini-2.5-pro", "timestamp": "2026-07-17T08:00:00Z", "tokens": map[string]any{"input": 100, "cached": 20, "output": 10, "thoughts": 5}},
		{"type": "gemini", "id": "m2", "model": "gemini-2.5-pro", "timestamp": "2026-07-17T08:01:00Z", "tokens": map[string]any{"input": 50, "cached": 10, "output": 6, "thoughts": 4}},
	})
	if imported, _, err := syncGeminiSessionFile(service, scope, path); err != nil || imported != 1 {
		t.Fatalf("second gemini sync = (%d, %v)", imported, err)
	}

	var input int
	var output int
	var reasoning int
	if err := db.QueryRow(`
		SELECT SUM(input_tokens), SUM(output_tokens), SUM(reasoning_tokens) FROM request_log
		WHERE data_source = ? AND session_id = ?
	`, requestLogDataSourceGeminiSession, sessionID).Scan(&input, &output, &reasoning); err != nil {
		t.Fatal(err)
	}
	if input != 120 || output != 16 || reasoning != 9 {
		t.Fatalf("gemini totals = (%d, %d, %d), want (120, 16, 9)", input, output, reasoning)
	}
}

func TestGeminiSessionUsageImportsCompletedZeroTokenMessage(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	path := filepath.Join(t.TempDir(), "session-test.json")
	sessionID := scope + ":gemini-update"
	writeGeminiSessionFile(t, path, sessionID, []map[string]any{
		{"type": "gemini", "id": "m1", "model": "gemini-2.5-pro", "timestamp": "2026-07-17T08:00:00Z", "tokens": map[string]any{}},
	})
	if imported, skipped, err := syncGeminiSessionFile(service, scope, path); err != nil || imported != 0 || skipped != 0 {
		t.Fatalf("zero-token sync = (%d, %d, %v)", imported, skipped, err)
	}

	writeGeminiSessionFile(t, path, sessionID, []map[string]any{
		{"type": "gemini", "id": "m1", "model": "gemini-2.5-pro", "timestamp": "2026-07-17T08:00:00Z", "tokens": map[string]any{"input": 100, "cached": 20, "output": 10, "thoughts": 5}},
	})
	if imported, skipped, err := syncGeminiSessionFile(service, scope, path); err != nil || imported != 1 || skipped != 0 {
		t.Fatalf("completed sync = (%d, %d, %v)", imported, skipped, err)
	}

	var input int
	var output int
	var reasoning int
	var cacheRead int
	var dedupCore string
	if err := db.QueryRow(`
		SELECT input_tokens, output_tokens, reasoning_tokens, cache_read_tokens, dedup_core
		FROM request_log WHERE data_source = ? AND session_id = ?
	`, requestLogDataSourceGeminiSession, sessionID).Scan(&input, &output, &reasoning, &cacheRead, &dedupCore); err != nil {
		t.Fatal(err)
	}
	if input != 80 || output != 10 || reasoning != 5 || cacheRead != 20 {
		t.Fatalf("gemini completed row = (%d, %d, %d, %d)", input, output, reasoning, cacheRead)
	}
	if dedupCore != buildRequestLogDedupCore("gemini", 80, 10, 20) {
		t.Fatalf("gemini dedup core = %q", dedupCore)
	}
}

func TestSessionUsageDedupIsPersistentOneToOneAndRestoresAfterDelete(t *testing.T) {
	_, db, scope := prepareSessionUsageTest(t)
	createdAt := "2026-07-17 08:00:00"
	inputTokens := int(time.Now().UnixNano()%1_000_000) + 1_000_000
	outputTokens := inputTokens + 1
	cacheReadTokens := inputTokens + 2
	providerID := scope + ":provider"
	sessionIDValue := scope + ":dedup"
	core := buildRequestLogDedupCore("codex", inputTokens, outputTokens, cacheReadTokens)
	proxyResult, err := db.Exec(`
		INSERT INTO request_log (
			platform, model, provider_id, provider, http_code, input_tokens, output_tokens,
			cache_read_tokens, data_source, dedup_core, created_at
		) VALUES ('codex', 'gpt-5-codex', ?, 'Provider 1', 200, ?, ?, ?, 'proxy', ?, ?)
	`, providerID, inputTokens, outputTokens, cacheReadTokens, core, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	proxyID, _ := proxyResult.LastInsertId()
	sessionResult, err := db.Exec(`
		INSERT INTO request_log (
			platform, model, provider_id, provider, http_code, input_tokens, output_tokens,
			cache_read_tokens, data_source, source_record_id, session_id, dedup_core, created_at
		) VALUES ('codex', 'gpt-5-codex', '_codex_session', 'Codex 会话', 200, ?, ?, ?, 'codex_session', ?, ?, ?, ?)
	`, inputTokens, outputTokens, cacheReadTokens, stableSessionRecordID(scope, "codex", "dedup"), sessionIDValue, core, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	sessionID, _ := sessionResult.LastInsertId()
	if _, err := db.Exec(`UPDATE session_usage_dedup_state SET last_proxy_log_id = ?, last_session_log_id = ? WHERE id = 1`, proxyID-1, sessionID-1); err != nil {
		t.Fatal(err)
	}
	created, err := reconcileSessionUsageDedup()
	if err != nil || created != 1 {
		t.Fatalf("reconcile = (%d, %v)", created, err)
	}
	var matchedProxyID int64
	if err := db.QueryRow("SELECT proxy_log_id FROM session_usage_dedup WHERE session_log_id = ?", sessionID).Scan(&matchedProxyID); err != nil {
		t.Fatal(err)
	}
	if matchedProxyID != proxyID {
		t.Fatalf("matched proxy = %d, want %d", matchedProxyID, proxyID)
	}

	allPage, err := NewLogService(nil).ListRequestLogsPageV3("codex", "", "", "all", 20, 0, "2026-07-17 00:00:00", "2026-07-18 00:00:00")
	if err != nil {
		t.Fatal(err)
	}
	if allPage.Total < 1 {
		t.Fatalf("all total = %d", allPage.Total)
	}
	for _, item := range allPage.Items {
		if item.ID == sessionID {
			t.Fatalf("matched session row remained visible in all mode")
		}
	}
	if _, err := db.Exec("DELETE FROM request_log WHERE id = ?", proxyID); err != nil {
		t.Fatal(err)
	}
	var matches int
	if err := db.QueryRow("SELECT COUNT(*) FROM session_usage_dedup WHERE session_log_id = ?", sessionID).Scan(&matches); err != nil {
		t.Fatal(err)
	}
	if matches != 0 {
		t.Fatalf("dedup match remains after proxy delete")
	}
}

func TestLogSourceModesAndProxyOnlyOperationalStats(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	createdAt := "2026-07-17 08:00:00"
	core := buildRequestLogDedupCore("codex", 80, 10, 20)
	proxyResult, err := db.Exec(`
		INSERT INTO request_log (
			platform, model, provider_id, provider, http_code, input_tokens, output_tokens,
			cache_read_tokens, total_cost, data_source, dedup_core, created_at
		) VALUES ('codex', 'gpt-5-codex', ?, 'Provider 1', 200, 80, 10, 20, 1, 'proxy', ?, ?)
	`, scope+":provider", core, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	proxyID, _ := proxyResult.LastInsertId()
	matchedSessionResult, err := db.Exec(`
		INSERT INTO request_log (
			platform, model, provider_id, provider, http_code, input_tokens, output_tokens,
			cache_read_tokens, total_cost, data_source, source_record_id, session_id, dedup_core, created_at
		) VALUES ('codex', 'gpt-5-codex', '_codex_session', 'Codex 会话', 200, 80, 10, 20, 2, 'codex_session', ?, ?, ?, ?)
	`, stableSessionRecordID(scope, "codex", "matched"), scope+":matched", core, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	matchedSessionID, _ := matchedSessionResult.LastInsertId()
	if _, err := db.Exec(`
		INSERT INTO request_log (
			platform, model, provider_id, provider, http_code, input_tokens, output_tokens,
			cache_read_tokens, total_cost, data_source, source_record_id, session_id, dedup_core, created_at
		) VALUES ('codex', 'gpt-5-codex', '_codex_session', 'Codex 会话', 200, 40, 5, 10, 3, 'codex_session', ?, ?, ?, '2026-07-17 08:01:00')
	`, stableSessionRecordID(scope, "codex", "session-only"), scope+":session-only", buildRequestLogDedupCore("codex", 40, 5, 10)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE session_usage_dedup_state SET last_proxy_log_id = ?, last_session_log_id = ? WHERE id = 1`, proxyID-1, matchedSessionID-1); err != nil {
		t.Fatal(err)
	}
	if created, err := reconcileSessionUsageDedup(); err != nil || created != 1 {
		t.Fatalf("reconcile = (%d, %v)", created, err)
	}

	for mode, want := range map[string]int64{"proxy": 1, "session": 2, "all": 2} {
		summary, err := service.SummaryRangeV3("codex", "", "", mode, "2026-07-17 00:00:00", "2026-07-18 00:00:00")
		if err != nil {
			t.Fatal(err)
		}
		if summary.TotalRequests != want {
			t.Fatalf("%s total requests = %d, want %d", mode, summary.TotalRequests, want)
		}
	}
	cost, err := service.CostSince("2026-07-17 00:00:00", "codex")
	if err != nil {
		t.Fatal(err)
	}
	if cost != 1 {
		t.Fatalf("proxy operational cost = %v, want 1", cost)
	}
	var aggregatedRequests int
	if err := db.QueryRow("SELECT COALESCE(SUM(total_requests), 0) FROM request_log_stats_daily").Scan(&aggregatedRequests); err != nil {
		t.Fatal(err)
	}
	if aggregatedRequests != 1 {
		t.Fatalf("proxy aggregate requests = %d, want 1", aggregatedRequests)
	}
}

func stringsJoinLines(lines []string) string {
	result := ""
	for _, line := range lines {
		result += line + "\n"
	}
	return result
}

func writeGeminiSessionFile(t *testing.T, path string, sessionID string, messages []map[string]any) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"sessionId": sessionID, "messages": messages})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
