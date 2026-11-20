/**
 * @name: Copilot CLI 会话用量同步测试
 * @Descripttion: 验证 Copilot CLI 用量事件的新输入口径、计时、去重与数据目录定位。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_copilot_test.go
 */

package services

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type copilotTestRow struct {
	ID          string
	Model       string
	Input       int
	Output      int
	CacheRead   int
	CacheCreate int
	Duration    float64
	FirstToken  float64
	SessionID   string
	Platform    string
	DataSource  string
	Provider    string
}

func copilotTestLoadRows(t *testing.T, db *sql.DB) []copilotTestRow {
	t.Helper()
	rows, err := db.Query(`SELECT source_record_id, model, input_tokens, output_tokens, cache_read_tokens,
		cache_create_tokens, COALESCE(duration_sec, 0), COALESCE(first_token_sec, 0), session_id,
		platform, data_source, provider_id
		FROM request_log WHERE data_source = ? ORDER BY session_id, source_record_id`, requestLogDataSourceCopilotSession)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []copilotTestRow
	for rows.Next() {
		var row copilotTestRow
		if err := rows.Scan(&row.ID, &row.Model, &row.Input, &row.Output, &row.CacheRead, &row.CacheCreate,
			&row.Duration, &row.FirstToken, &row.SessionID, &row.Platform, &row.DataSource, &row.Provider); err != nil {
			t.Fatal(err)
		}
		result = append(result, row)
	}
	return result
}

func copilotTestUsageEvent(id string, withDetails bool) map[string]any {
	usage := map[string]any{
		"model": "gpt-5-mini", "inputTokens": 25301, "outputTokens": 85,
		"cacheReadTokens": 7680, "cacheWriteTokens": 0, "duration": 2988, "timeToFirstTokenMs": 2906.8,
	}
	if withDetails {
		usage["copilotUsage"] = map[string]any{"tokenDetails": []any{
			map[string]any{"tokenType": "input", "tokenCount": 17621},
			map[string]any{"tokenType": "cache_read", "tokenCount": 7680},
			map[string]any{"tokenType": "cache_write", "tokenCount": 0},
			map[string]any{"tokenType": "output", "tokenCount": 85},
		}}
	}
	return map[string]any{"type": "session.usage_record", "id": id, "timestamp": "2026-10-10T09:08:03.054Z", "data": map[string]any{"usage": usage}}
}

func copilotTestWrite(t *testing.T, dataDir string, session string, events []map[string]any, tail string) string {
	t.Helper()
	path := filepath.Join(dataDir, "session-state", session, "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var builder strings.Builder
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		builder.Write(data)
		builder.WriteByte('\n')
	}
	builder.WriteString(tail)
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Duration(time.Now().UnixNano()%1000+1) * time.Hour)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCopilotSessionUsageImportsFreshInputOnce(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	dataDir := filepath.Join(home, ".copilot")
	copilotTestWrite(t, dataDir, "session-1", []map[string]any{
		{"type": "user.message", "timestamp": "2026-10-10T09:08:00.000Z", "data": map[string]any{"content": "hi"}},
		copilotTestUsageEvent("u1", true),
		copilotTestUsageEvent("u2", false),
	}, "")

	result, err := syncCopilotSessionUsage(service, scope, home)
	if err != nil || result.Imported != 2 || result.FilesScanned != 1 || result.Platform != sessionPlatformCopilot {
		t.Fatalf("first sync = %+v, %v", result, err)
	}
	rows := copilotTestLoadRows(t, db)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	for _, row := range rows {
		// 有 tokenDetails 与由 inputTokens 扣减缓存，得到同一个新输入
		if row.Input != 17621 || row.CacheRead != 7680 || row.Output != 85 || row.CacheCreate != 0 ||
			row.Duration != 2.988 || row.FirstToken != 2.907 || row.Model != "gpt-5-mini" ||
			row.SessionID != "session-1" || row.Platform != "copilot" || row.DataSource != "copilot_session" ||
			row.Provider != requestLogProviderIDCopilotSession {
			t.Fatalf("row = %+v", row)
		}
	}

	// 文件未变直接跳过；重写后追加 u3，已存在的两条不重复导入
	if again, err := syncCopilotSessionUsage(service, scope, home); err != nil || again.Imported != 0 {
		t.Fatalf("unchanged sync = %+v, %v", again, err)
	}
	copilotTestWrite(t, dataDir, "session-1", []map[string]any{
		copilotTestUsageEvent("u1", true), copilotTestUsageEvent("u2", false), copilotTestUsageEvent("u3", true),
	}, "")
	again, err := syncCopilotSessionUsage(service, scope, home)
	if err != nil || again.Imported != 1 || again.Skipped != 2 {
		t.Fatalf("rewrite sync = %+v, %v", again, err)
	}
	if len(copilotTestLoadRows(t, db)) != 3 {
		t.Fatal("rewrite duplicated rows")
	}
}

func TestCopilotSessionUsageCacheWriteAndSequenceIdentity(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	event := copilotTestUsageEvent("", false)
	usage := event["data"].(map[string]any)["usage"].(map[string]any)
	usage["inputTokens"] = 1000
	usage["cacheReadTokens"] = 600
	usage["cacheWriteTokens"] = 100
	usage["accounting"] = map[string]any{"sequence": 7}
	delete(usage, "model")
	delete(usage, "duration")
	delete(usage, "timeToFirstTokenMs")
	noIdentity := copilotTestUsageEvent("", false)
	copilotTestWrite(t, filepath.Join(home, ".copilot"), "s", []map[string]any{event, noIdentity}, "")

	if result, err := syncCopilotSessionUsage(service, scope, home); err != nil || result.Imported != 1 {
		t.Fatalf("sync = %+v, %v", result, err)
	}
	rows := copilotTestLoadRows(t, db)
	if len(rows) != 1 || rows[0].Input != 300 || rows[0].CacheRead != 600 || rows[0].CacheCreate != 100 ||
		rows[0].Model != "unknown" || rows[0].Duration != 0 || rows[0].FirstToken != 0 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].ID != stableSessionRecordID(scope, sessionPlatformCopilot, "copilot:s:seq7") {
		t.Fatalf("source_record_id = %s", rows[0].ID)
	}
}

func TestCopilotSessionUsageIgnoresPartialTailAndEmptyRecords(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	zero := map[string]any{"type": "session.usage_record", "id": "zero", "timestamp": "2026-10-10T09:08:03.054Z",
		"data": map[string]any{"usage": map[string]any{"model": "m"}}}
	copilotTestWrite(t, filepath.Join(home, ".copilot"), "s", []map[string]any{zero}, `{"type":"session.usage_record","id":"partial",`)
	if result, err := syncCopilotSessionUsage(service, scope, home); err != nil || result.Imported != 0 {
		t.Fatalf("sync = %+v, %v", result, err)
	}
	if len(copilotTestLoadRows(t, db)) != 0 {
		t.Fatal("empty or partial record imported")
	}
}

func TestCopilotSessionUsageDataDirResolution(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	override := t.TempDir()
	t.Setenv("COPILOT_HOME", override)
	copilotTestWrite(t, override, "env-session", []map[string]any{copilotTestUsageEvent("e1", true)}, "")

	// 非 native 作用域不读取宿主机环境变量
	wslResult, err := syncCopilotSessionUsage(service, "wsl:Ubuntu", home)
	if err != nil || wslResult.FilesScanned != 0 || wslResult.Imported != 0 {
		t.Fatalf("wsl sync = %+v, %v", wslResult, err)
	}
	nativeResult, err := syncCopilotSessionUsage(service, "native", home)
	if err != nil || nativeResult.FilesScanned != 1 || nativeResult.Imported != 1 {
		t.Fatalf("native sync = %+v, %v", nativeResult, err)
	}
	if rows := copilotTestLoadRows(t, db); len(rows) != 1 || rows[0].SessionID != "env-session" {
		t.Fatalf("rows = %+v", rows)
	}
	t.Setenv("COPILOT_HOME", "")
	if result, err := syncCopilotSessionUsage(service, scope, t.TempDir()); err != nil || result.FilesScanned != 0 {
		t.Fatalf("missing dir = %+v, %v", result, err)
	}
}
