/**
 * @name: 会话来源注册表测试
 * @Descripttion: 验证会话来源注册表的可见性、删除后不复活的回执机制及历史计时回填只更新已有行。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/requestlog_source_registry_test.go
 */

package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEverySessionSourceIsVisibleInSessionModeOnly(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	createdAt := "2026-11-20 08:00:00"
	for index, def := range sessionUsageSourceDefs {
		if _, err := db.Exec(`
			INSERT INTO request_log (
				platform, model, provider_id, provider, http_code, input_tokens, output_tokens,
				data_source, source_record_id, dedup_core, created_at
			) VALUES (?, 'model-x', ?, ?, 200, ?, 1, ?, ?, ?, ?)
		`, def.Platform, def.ProviderID, def.ProviderName, 100+index, def.DataSource,
			stableSessionRecordID(scope, def.Platform, "row"), buildRequestLogDedupCore(def.Platform, 100+index, 1, 0), createdAt); err != nil {
			t.Fatalf("insert %s: %v", def.DataSource, err)
		}
		if !isSessionRequestLogSource(def.DataSource) {
			t.Fatalf("%s should be recognized as a session source", def.DataSource)
		}
	}

	start, end := "2026-11-20 00:00:00", "2026-11-21 00:00:00"
	session, err := service.ListRequestLogsPageV3("", "", "", "session", 100, 0, start, end)
	if err != nil {
		t.Fatalf("session mode: %v", err)
	}
	if int(session.Total) != len(sessionUsageSourceDefs) {
		t.Fatalf("session mode should list every source, got %d of %d", session.Total, len(sessionUsageSourceDefs))
	}
	proxy, err := service.ListRequestLogsPageV3("", "", "", "proxy", 100, 0, start, end)
	if err != nil || proxy.Total != 0 {
		t.Fatalf("proxy mode should hide session rows, got %d (%v)", proxy.Total, err)
	}
	filtered, err := service.ListRequestLogsPageV3(sessionPlatformAntigravity, "", "", "all", 100, 0, start, end)
	if err != nil || filtered.Total != 1 || filtered.Items[0].DataSource != requestLogDataSourceAntigravitySession {
		t.Fatalf("antigravity platform filter failed: %+v (%v)", filtered, err)
	}
}

func TestDeletedSessionRowsAreNotResurrectedByResync(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	root := filepath.Join(home, ".gemini", "antigravity-cli")
	rows := []map[string]any{
		{"step_index": 1, "type": "PLANNER_RESPONSE", "status": "DONE", "created_at": "2026-11-20T02:33:05Z",
			"model": "gemini-4-neon-high", "input_tokens": 100, "output_tokens": 10},
		{"step_index": 2, "type": "PLANNER_RESPONSE", "status": "DONE", "created_at": "2026-11-20T02:33:15Z",
			"model": "gemini-4-neon-high", "input_tokens": 200, "output_tokens": 20},
	}
	path := writeAntigravityTranscript(t, root, "sess-a", rows)
	if result, err := syncAntigravitySessionUsage(ls, scope, home); err != nil || result.Imported != 2 {
		t.Fatalf("first sync: %+v %v", result, err)
	}
	// 用户在日志页删除了第 1 步
	if _, err := db.Exec("DELETE FROM request_log WHERE input_tokens = 100"); err != nil {
		t.Fatal(err)
	}
	// 会话继续：文件变化触发整份重读
	rows = append(rows, map[string]any{"step_index": 3, "type": "PLANNER_RESPONSE", "status": "DONE",
		"created_at": "2026-11-20T02:33:25Z", "model": "gemini-4-neon-high", "input_tokens": 300, "output_tokens": 30})
	writeAntigravityTranscript(t, root, "sess-a", rows)
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(path, future, future)
	if result, err := syncAntigravitySessionUsage(ls, scope, home); err != nil || result.Imported != 1 {
		t.Fatalf("resync should only import the new step: %+v %v", result, err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM request_log WHERE input_tokens = 100").Scan(&count); err != nil || count != 0 {
		t.Fatalf("deleted row was resurrected: %d %v", count, err)
	}

	// 选择「清空并重新导入会话」时回执随之清空，可以完整重建
	if err := resetSessionUsageSyncState(); err != nil {
		t.Fatal(err)
	}
	if result, err := syncAntigravitySessionUsage(ls, scope, home); err != nil || result.Imported != 1 {
		t.Fatalf("reimport should restore the deleted row: %+v %v", result, err)
	}
}

func TestTimingBackfillOnlyUpdatesRowsImportedBeforeUpgrade(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	path := filepath.Join(t.TempDir(), "projects", "demo", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	lines := []string{
		mustJSONLine(t, map[string]any{"type": "user", "uuid": "u1", "timestamp": "2026-11-20T01:00:00Z"}),
		claudeAssistantLine(t, "m1", "u1", "msg_1", "2026-11-20T01:00:10Z", 0, 800, "end_turn", nil),
		mustJSONLine(t, map[string]any{"type": "user", "uuid": "u2", "parentUuid": "m1", "timestamp": "2026-11-20T01:01:00Z"}),
		claudeAssistantLine(t, "m2", "u2", "msg_2", "2026-11-20T01:01:20Z", 0, 900, "end_turn", nil),
	}
	if err := os.WriteFile(path, []byte(stringsJoinLines(lines)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := syncClaudeSessionFile(ls, scope, path); err != nil {
		t.Fatal(err)
	}
	// 模拟升级前状态：无计时、无回执，且用户删除过 msg_1
	if _, err := db.Exec("UPDATE request_log SET duration_sec = 0, first_token_sec = 0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM session_usage_receipt"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM request_log WHERE source_record_id = ?", stableSessionRecordID(scope, "claude", "msg_1")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE session_log_sync SET backfill_line_offset = line_offset,
		byte_offset = 0, line_offset = 0, modified_ns = 0, file_size = 0, parser_state = ''`); err != nil {
		t.Fatal(err)
	}

	// 会话继续追加 msg_3
	lines = append(lines,
		mustJSONLine(t, map[string]any{"type": "user", "uuid": "u3", "parentUuid": "m2", "timestamp": "2026-11-20T01:02:00Z"}),
		claudeAssistantLine(t, "m3", "u3", "msg_3", "2026-11-20T01:02:30Z", 0, 700, "end_turn", nil),
	)
	if err := os.WriteFile(path, []byte(stringsJoinLines(lines)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := syncClaudeSessionFile(ls, scope, path); err != nil {
		t.Fatal(err)
	}
	durationOf := func(messageID string) (float64, bool) {
		var duration float64
		err := db.QueryRow("SELECT duration_sec FROM request_log WHERE source_record_id = ?", stableSessionRecordID(scope, "claude", messageID)).Scan(&duration)
		return duration, err == nil
	}
	if _, exists := durationOf("msg_1"); exists {
		t.Fatal("deleted history row must not be re-imported during backfill")
	}
	if duration, exists := durationOf("msg_2"); !exists || duration != 20 {
		t.Fatalf("existing row should get backfilled timing, got %v %v", duration, exists)
	}
	if duration, exists := durationOf("msg_3"); !exists || duration != 30 {
		t.Fatalf("new row should be imported with timing, got %v %v", duration, exists)
	}
}
