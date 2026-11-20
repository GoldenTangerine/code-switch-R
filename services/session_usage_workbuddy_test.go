/**
 * @name: WorkBuddy 会话用量同步测试
 * @Descripttion: 验证 WorkBuddy rawUsage 的新输入口径、增量读取、子代理归属与目录定位。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_workbuddy_test.go
 */

package services

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type workBuddyTestRow struct {
	ID          string
	Model       string
	Input       int
	Output      int
	CacheRead   int
	CacheCreate int
	Duration    float64
	SessionID   string
	Platform    string
	DataSource  string
}

func workBuddyTestLoadRows(t *testing.T, db *sql.DB) []workBuddyTestRow {
	t.Helper()
	rows, err := db.Query(`SELECT source_record_id, model, input_tokens, output_tokens, cache_read_tokens,
		cache_create_tokens, COALESCE(duration_sec, 0), session_id, platform, data_source
		FROM request_log WHERE data_source = ? ORDER BY session_id, input_tokens, source_record_id`, requestLogDataSourceWorkBuddySession)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []workBuddyTestRow
	for rows.Next() {
		var row workBuddyTestRow
		if err := rows.Scan(&row.ID, &row.Model, &row.Input, &row.Output, &row.CacheRead, &row.CacheCreate,
			&row.Duration, &row.SessionID, &row.Platform, &row.DataSource); err != nil {
			t.Fatal(err)
		}
		result = append(result, row)
	}
	return result
}

func workBuddyTestAssistant(id string, model string, prompt int, hit int, write int, completion int) string {
	data, _ := json.Marshal(map[string]any{
		"id": id, "parentId": "p", "timestamp": 1792764379226, "type": "message",
		"role": "assistant", "status": "completed", "sessionId": "s1", "cwd": "/w",
		"content": []any{map[string]any{"type": "output_text", "text": "ok"}},
		"providerData": map[string]any{"model": model, "requestModelId": "alias", "rawUsage": map[string]any{
			"prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": prompt + completion,
			"prompt_cache_hit_tokens": hit, "prompt_cache_miss_tokens": prompt - hit,
			"cache_read_input_tokens": 0, "cache_creation_input_tokens": write,
			"prompt_cache_write_tokens": 0, "completion_thinking_tokens": 3, "credit": 0.5, "cached_tokens": 0,
		}},
	})
	return string(data)
}

func workBuddyTestCall(id string, session string) string {
	data, _ := json.Marshal(map[string]any{
		"id": id, "timestamp": 1792764380000, "type": "function_call", "name": "Read", "callId": "c", "sessionId": session,
		"providerData": map[string]any{"model": "m", "messageId": "msg-" + id, "rawUsage": map[string]any{
			"prompt_tokens": 1000, "completion_tokens": 20, "prompt_cache_hit_tokens": 900, "prompt_cache_miss_tokens": 100,
		}},
	})
	return string(data)
}

const workBuddyTestUser = `{"id":"u1","timestamp":1792764357348,"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}],"sessionId":"s1","cwd":"/w"}`
const workBuddyTestReasoning = `{"id":"r1","timestamp":1792764379203,"type":"reasoning","rawContent":[{"type":"reasoning_text","text":"..."}],"providerData":{"model":"m"},"sessionId":"s1"}`

func workBuddyTestWrite(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var content string
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func workBuddyTestAppend(t *testing.T, path string, text string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

func TestWorkBuddySessionUsageImportsAssistantMessagesIncrementally(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	path := filepath.Join(home, ".workbuddy", "projects", "Users-me-proj", "s1.jsonl")
	workBuddyTestWrite(t, path, workBuddyTestUser, workBuddyTestReasoning, workBuddyTestAssistant("a1", "glm-test", 1200, 1000, 0, 34))

	result, err := syncWorkBuddySessionUsage(service, scope, home)
	if err != nil || result.Imported != 1 || result.FilesScanned != 1 || result.Platform != sessionPlatformWorkBuddy {
		t.Fatalf("first sync = %+v, %v", result, err)
	}
	rows := workBuddyTestLoadRows(t, db)
	want := workBuddyTestRow{
		ID:    stableSessionRecordID(scope, sessionPlatformWorkBuddy, "workbuddy:s1:a1"),
		Model: "glm-test", Input: 200, Output: 34, CacheRead: 1000, CacheCreate: 0, Duration: 0,
		SessionID: "s1", Platform: "workbuddy", DataSource: "workbuddy_session",
	}
	if len(rows) != 1 || rows[0] != want {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}

	// 未变化直接跳过；追加一个完整请求和一个没写完的行，只导入完整的
	if again, err := syncWorkBuddySessionUsage(service, scope, home); err != nil || again.Imported != 0 {
		t.Fatalf("unchanged sync = %+v, %v", again, err)
	}
	workBuddyTestAppend(t, path, workBuddyTestAssistant("a2", "glm-test", 500, 0, 100, 10)+"\n"+`{"id":"a3",`)
	if again, err := syncWorkBuddySessionUsage(service, scope, home); err != nil || again.Imported != 1 {
		t.Fatalf("append sync = %+v, %v", again, err)
	}
	rows = workBuddyTestLoadRows(t, db)
	if len(rows) != 2 || rows[1].Input != 400 || rows[1].CacheRead != 0 || rows[1].CacheCreate != 100 || rows[1].Output != 10 {
		t.Fatalf("rows after append = %+v", rows)
	}
}

func TestWorkBuddySessionUsageDeduplicatesEventsAcrossRewrites(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	path := filepath.Join(home, ".workbuddy", "projects", "proj", "s1.jsonl")
	line := workBuddyTestAssistant("a1", "m", 300, 0, 0, 5)
	workBuddyTestWrite(t, path, line, line)

	result, err := syncWorkBuddySessionUsage(service, scope, home)
	if err != nil || result.Imported != 1 || result.Skipped != 1 {
		t.Fatalf("first sync = %+v, %v", result, err)
	}
	// 文件被改短重写：游标回到开头重读，已入库的请求不会重复
	workBuddyTestWrite(t, path, line)
	again, err := syncWorkBuddySessionUsage(service, scope, home)
	if err != nil || again.Imported != 0 {
		t.Fatalf("rewrite sync = %+v, %v", again, err)
	}
	if len(workBuddyTestLoadRows(t, db)) != 1 {
		t.Fatal("rewrite duplicated rows")
	}
}

func TestWorkBuddySessionUsageCountsToolCallsAndSubagents(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	path := filepath.Join(home, ".workbuddy", "projects", "proj", "s1.jsonl")
	// 同一请求里先出现、不带 rawUsage 的 function_call 不算
	emptyCall := `{"id":"c0","timestamp":1792764380000,"type":"function_call","name":"Glob","sessionId":"s1","providerData":{"model":"m"}}`
	workBuddyTestWrite(t, path, workBuddyTestAssistant("a1", "m", 300, 0, 0, 5), emptyCall, workBuddyTestCall("c1", "s1"))
	workBuddyTestWrite(t, filepath.Join(home, ".workbuddy", "projects", "proj", "s1", "subagents", "agent-x.jsonl"),
		workBuddyTestCall("c2", "agent-own-id"))

	result, err := syncWorkBuddySessionUsage(service, scope, home)
	if err != nil || result.Imported != 3 || result.FilesScanned != 2 {
		t.Fatalf("sync = %+v, %v", result, err)
	}
	rows := workBuddyTestLoadRows(t, db)
	if len(rows) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	// 子代理的请求记在父会话 s1 名下，不用子代理自己的 sessionId
	byID := map[string]workBuddyTestRow{}
	for _, row := range rows {
		byID[row.ID] = row
		if row.SessionID != "s1" {
			t.Fatalf("session_id = %s", row.SessionID)
		}
	}
	for _, id := range []string{"c1", "c2"} {
		row, ok := byID[stableSessionRecordID(scope, sessionPlatformWorkBuddy, "workbuddy:s1:"+id)]
		if !ok || row.Input != 100 || row.CacheRead != 900 || row.Output != 20 {
			t.Fatalf("call %s row = %+v (found=%v)", id, row, ok)
		}
	}
	if again, err := syncWorkBuddySessionUsage(service, scope, home); err != nil || again.Imported != 0 {
		t.Fatalf("resync = %+v, %v", again, err)
	}
}

func TestWorkBuddySessionUsageFallsBackToSDKUsageAndStripsCustomPrefix(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	path := filepath.Join(home, ".workbuddy", "projects", "proj", "s2.jsonl")
	sdk := `{"id":"m1","timestamp":1792764379226,"type":"message","role":"assistant","sessionId":"s2","providerData":{"requestModelId":"custom-local:my-model","usage":{"inputTokens":500,"outputTokens":40,"inputTokensDetails":[{"cached_tokens":200}]}}}`
	workBuddyTestWrite(t, path, sdk)
	if result, err := syncWorkBuddySessionUsage(service, scope, home); err != nil || result.Imported != 1 {
		t.Fatalf("sync = %+v, %v", result, err)
	}
	rows := workBuddyTestLoadRows(t, db)
	if len(rows) != 1 || rows[0].Model != "my-model" || rows[0].Input != 300 || rows[0].CacheRead != 200 || rows[0].Output != 40 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestWorkBuddySessionUsageDataDirResolution(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	override := t.TempDir()
	t.Setenv("WORKBUDDY_CONFIG_DIR", " "+override+" ")
	workBuddyTestWrite(t, filepath.Join(override, "projects", "p", "env.jsonl"), workBuddyTestAssistant("e1", "m", 100, 0, 0, 1))

	// 非 native 作用域不读取宿主机环境变量
	wslResult, err := syncWorkBuddySessionUsage(service, "wsl:Ubuntu", home)
	if err != nil || wslResult.FilesScanned != 0 || wslResult.Imported != 0 {
		t.Fatalf("wsl sync = %+v, %v", wslResult, err)
	}
	nativeResult, err := syncWorkBuddySessionUsage(service, "native", home)
	if err != nil || nativeResult.FilesScanned != 1 || nativeResult.Imported != 1 {
		t.Fatalf("native sync = %+v, %v", nativeResult, err)
	}
	if rows := workBuddyTestLoadRows(t, db); len(rows) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	// 空白覆盖值被忽略，缺失目录不是错误
	t.Setenv("WORKBUDDY_CONFIG_DIR", "  ")
	if result, err := syncWorkBuddySessionUsage(service, scope, t.TempDir()); err != nil || result.FilesScanned != 0 || result.Imported != 0 {
		t.Fatalf("missing dir = %+v, %v", result, err)
	}
}
