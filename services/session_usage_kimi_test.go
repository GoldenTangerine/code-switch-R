/**
 * @name: Kimi Code 会话用量同步测试
 * @Descripttion: 验证 Kimi Code wire 记录的模型/计时关联、子代理、分叉去重与目录定位。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_kimi_test.go
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

type kimiTestRow struct {
	ID          string
	Model       string
	Input       int
	Output      int
	CacheRead   int
	CacheCreate int
	Duration    float64
	FirstToken  float64
	SessionID   string
	CreatedAt   string
	Platform    string
	DataSource  string
}

func kimiTestLoadRows(t *testing.T, db *sql.DB) []kimiTestRow {
	t.Helper()
	rows, err := db.Query(`SELECT source_record_id, model, input_tokens, output_tokens, cache_read_tokens,
		cache_create_tokens, COALESCE(duration_sec, 0), COALESCE(first_token_sec, 0), session_id, created_at,
		platform, data_source
		FROM request_log WHERE data_source = ? ORDER BY created_at, output_tokens`, requestLogDataSourceKimiSession)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []kimiTestRow
	for rows.Next() {
		var row kimiTestRow
		if err := rows.Scan(&row.ID, &row.Model, &row.Input, &row.Output, &row.CacheRead, &row.CacheCreate,
			&row.Duration, &row.FirstToken, &row.SessionID, &row.CreatedAt, &row.Platform, &row.DataSource); err != nil {
			t.Fatal(err)
		}
		result = append(result, row)
	}
	return result
}

func kimiTestTime(seconds int64) string {
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

func kimiTestMessage(id string, timeMs int64, output int) map[string]any {
	return map[string]any{"type": "agent.message.appended", "time": timeMs, "message": map[string]any{
		"message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": "Done"}}},
		"meta": map[string]any{"messageId": id, "model": map[string]any{"provider": "agent-loop", "model": "agent-loop"},
			"usage": map[string]any{"inputOther": 10, "output": output, "inputCacheRead": 30, "inputCacheCreation": 40}},
	}}
}

func kimiTestLoopEvent(timeMs int64, event map[string]any) map[string]any {
	return map[string]any{"type": "context.append_loop_event", "time": timeMs, "event": event}
}

// kimiTestRequest 构造一次完整请求：step.begin -> llm.request -> content.part -> 消息 -> tool.call -> step.end
func kimiTestRequest(id string, model string, start int64, first int64, stream int64, output int) []map[string]any {
	return []map[string]any{
		kimiTestLoopEvent(start-10, map[string]any{"type": "step.begin", "uuid": id, "turnId": "0", "step": 1}),
		{"type": "llm.request", "time": start, "agentId": "main", "kind": "loop", "turnStep": "0.1", "provider": "openai", "model": model},
		kimiTestLoopEvent(start+first+stream, map[string]any{"type": "content.part", "stepUuid": id, "part": map[string]any{"type": "text", "text": "Done"}}),
		kimiTestMessage(id, start+first+stream, output),
		kimiTestLoopEvent(start+first+stream+100_000, map[string]any{"type": "tool.call", "stepUuid": id}),
		kimiTestLoopEvent(start+first+stream+200_000, map[string]any{"type": "step.end", "uuid": id, "messageId": id,
			"llmFirstTokenLatencyMs": first, "llmStreamDurationMs": stream}),
	}
}

func kimiTestWrite(t *testing.T, path string, records []map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var builder strings.Builder
	for _, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		builder.Write(data)
		builder.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

// kimiTestBump 推进文件修改时间，强制下一轮同步重新读取
func kimiTestBump(t *testing.T, path string) {
	t.Helper()
	future := time.Now().Add(time.Duration(time.Now().UnixNano()%1000+1) * time.Hour)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
}

func kimiTestWirePath(home string, session string, agent string) string {
	return filepath.Join(home, ".kimi-code", "sessions", "wd_project", session, "agents", agent, "wire.jsonl")
}

func TestKimiSessionUsageResolvesModelsAndTiming(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	path := kimiTestWirePath(home, "session_one", "main")
	records := kimiTestRequest("one", "model-one", 1_700_000_000_250, 3_500, 6_500, 900)
	records = append(records, kimiTestRequest("two", "model-two", 1_700_000_300_000, 1_500, 18_500, 600)...)
	kimiTestWrite(t, path, records)

	result, err := syncKimiSessionUsage(service, scope, home)
	if err != nil || result.Imported != 2 || result.FilesScanned != 1 || result.Platform != sessionPlatformKimi {
		t.Fatalf("first sync = %+v, %v", result, err)
	}
	rows := kimiTestLoadRows(t, db)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	want := []kimiTestRow{
		{Model: "model-one", Duration: 10, FirstToken: 3.5, Output: 900, CreatedAt: kimiTestTime(1_700_000_010)},
		{Model: "model-two", Duration: 20, FirstToken: 1.5, Output: 600, CreatedAt: kimiTestTime(1_700_000_320)},
	}
	for index, row := range rows {
		expected := want[index]
		if row.Platform != "kimi" || row.DataSource != "kimi_session" || row.SessionID != "session_one" {
			t.Fatalf("row meta = %+v", row)
		}
		if row.Model != expected.Model || row.Duration != expected.Duration || row.FirstToken != expected.FirstToken || row.Output != expected.Output {
			t.Fatalf("row %d = %+v, want %+v", index, row, expected)
		}
		if row.Input != 10 || row.CacheRead != 30 || row.CacheCreate != 40 {
			t.Fatalf("tokens = %+v", row)
		}
		// 记账时间 = 请求发起 + 耗时（不是提交时间）
		if row.CreatedAt != expected.CreatedAt {
			t.Fatalf("created_at = %s, want %s", row.CreatedAt, expected.CreatedAt)
		}
	}

	// 文件未变：游标命中；强制重读：全部识别为已存在，没有新增
	if again, err := syncKimiSessionUsage(service, scope, home); err != nil || again.Imported != 0 {
		t.Fatalf("unchanged sync = %+v, %v", again, err)
	}
	kimiTestBump(t, path)
	if again, err := syncKimiSessionUsage(service, scope, home); err != nil || again.Imported != 0 || again.Skipped != 2 {
		t.Fatalf("forced resync = %+v, %v", again, err)
	}
	if len(kimiTestLoadRows(t, db)) != 2 {
		t.Fatal("resync duplicated rows")
	}
}

func TestKimiSessionUsageEnrichesWhenStepEndArrivesLater(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	path := kimiTestWirePath(home, "session_a", "main")
	records := kimiTestRequest("live", "real-model", 1_700_000_000_000, 2_000, 8_000, 500)
	kimiTestWrite(t, path, records[:4])
	if result, err := syncKimiSessionUsage(service, scope, home); err != nil || result.Imported != 1 {
		t.Fatalf("partial sync = %+v, %v", result, err)
	}
	rows := kimiTestLoadRows(t, db)
	if len(rows) != 1 || rows[0].Model != "unknown" || rows[0].Duration != 0 {
		t.Fatalf("partial rows = %+v", rows)
	}

	kimiTestWrite(t, path, records)
	kimiTestBump(t, path)
	if result, err := syncKimiSessionUsage(service, scope, home); err != nil || result.Imported != 1 {
		t.Fatalf("complete sync = %+v, %v", result, err)
	}
	rows = kimiTestLoadRows(t, db)
	if len(rows) != 1 || rows[0].Model != "real-model" || rows[0].Duration != 10 || rows[0].FirstToken != 2 {
		t.Fatalf("enriched rows = %+v", rows)
	}
	resolved := rows[0]

	// 分叉里只有截断的副本：不能抹掉已解析的模型、计时与会话归属
	forkPath := kimiTestWirePath(home, "session_fork", "main")
	kimiTestWrite(t, forkPath, records[:4])
	if result, err := syncKimiSessionUsage(service, scope, home); err != nil || result.Imported != 0 {
		t.Fatalf("fork sync = %+v, %v", result, err)
	}
	rows = kimiTestLoadRows(t, db)
	if len(rows) != 1 || rows[0] != resolved {
		t.Fatalf("fork changed row: %+v, want %+v", rows, resolved)
	}
}

func TestKimiSessionUsageDatesBatchCommittedMessagesByRequest(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	commit := int64(1_700_000_900_000)
	var records []map[string]any
	for _, item := range []struct {
		id    string
		start int64
	}{{"one", 1_700_000_000_000}, {"two", 1_700_000_300_000}} {
		step := kimiTestRequest(item.id, "model", item.start, 1_000, 2_000, 10)
		// 去掉请求中途的人类视图消息，改为一轮结束后统一提交
		step = append(step[:3], step[4:]...)
		records = append(records, step...)
	}
	records = append(records, kimiTestMessage("one", commit, 10), kimiTestMessage("two", commit, 11))
	kimiTestWrite(t, kimiTestWirePath(home, "s", "main"), records)

	if result, err := syncKimiSessionUsage(service, scope, home); err != nil || result.Imported != 2 {
		t.Fatalf("sync = %+v, %v", result, err)
	}
	rows := kimiTestLoadRows(t, db)
	if len(rows) != 2 || rows[0].CreatedAt != kimiTestTime(1_700_000_003) || rows[1].CreatedAt != kimiTestTime(1_700_000_303) {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestKimiSessionUsageImportsSubagentWiresOnce(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	main := kimiTestRequest("main-step", "main-model", 1_700_000_000_000, 1_000, 1_000, 50)
	main = append(main, map[string]any{"type": "subagent.spawned", "subagentId": "agent-0", "time": 1_700_000_001_000})
	kimiTestWrite(t, kimiTestWirePath(home, "session_one", "main"), main)

	step := func(id string, model string, start int64, output int) []map[string]any {
		usage := map[string]any{"inputOther": 10, "output": output, "inputCacheRead": 30, "inputCacheCreation": 0}
		return []map[string]any{
			kimiTestLoopEvent(start-10, map[string]any{"type": "step.begin", "uuid": id, "turnId": "0", "step": 1}),
			{"type": "llm.request", "time": start, "agentId": "agent-0", "kind": "loop", "turnStep": "0.1", "provider": "openai", "model": model},
			{"type": "usage.record", "time": start + 900, "agentId": "agent-0", "model": "kimi-code/" + model, "usage": usage, "usageScope": "turn"},
			kimiTestLoopEvent(start+1_000, map[string]any{"type": "step.end", "uuid": id, "turnId": "0", "step": 1, "usage": usage,
				"messageId": id, "llmFirstTokenLatencyMs": 200, "llmStreamDurationMs": 800}),
		}
	}
	child := []map[string]any{{"type": "agent.message.appended", "time": 1_700_000_001_000, "message": map[string]any{
		"message": map[string]any{"role": "user", "content": []any{}}, "meta": map[string]any{}}}}
	child = append(child, step("child-1", "child-model", 1_700_000_002_000, 70)...)
	child = append(child, step("child-2", "child-model", 1_700_000_004_000, 90)...)
	// 归属其他 agent 的请求不能泄漏到本 wire
	child = append(child, map[string]any{"type": "llm.request", "agentId": "agent-1", "kind": "loop", "model": "other-model"})
	childPath := kimiTestWirePath(home, "session_one", "agent-0")
	kimiTestWrite(t, childPath, child)

	result, err := syncKimiSessionUsage(service, scope, home)
	if err != nil || result.FilesScanned != 2 || result.Imported != 3 {
		t.Fatalf("sync = %+v, %v", result, err)
	}
	rows := kimiTestLoadRows(t, db)
	if len(rows) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	expected := []struct {
		model    string
		output   int
		duration float64
	}{{"main-model", 50, 2}, {"child-model", 70, 1}, {"child-model", 90, 1}}
	for index, row := range rows {
		if row.Model != expected[index].model || row.Output != expected[index].output ||
			row.Duration != expected[index].duration || row.SessionID != "session_one" {
			t.Fatalf("row %d = %+v", index, row)
		}
	}
	if again, err := syncKimiSessionUsage(service, scope, home); err != nil || again.Imported != 0 {
		t.Fatalf("unchanged sync = %+v, %v", again, err)
	}
	kimiTestBump(t, childPath)
	if again, err := syncKimiSessionUsage(service, scope, home); err != nil || again.Imported != 0 {
		t.Fatalf("forced resync = %+v, %v", again, err)
	}
	if len(kimiTestLoadRows(t, db)) != 3 {
		t.Fatal("rescan duplicated rows")
	}
}

func TestKimiSessionUsageForkKeepsFirstImportAndDeduplicatesMessages(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	message := kimiTestMessage("request-1", 1_700_000_000_000, 20)
	meta := message["message"].(map[string]any)["meta"].(map[string]any)
	meta["model"] = map[string]any{"provider": "custom", "model": "vendor/model"}
	// 同一消息在两种事件类型下各出现一次，只能记一条
	human := map[string]any{"type": "human.agent.message.appended", "time": 1_700_000_000_000, "message": message["message"]}
	kimiTestWrite(t, kimiTestWirePath(home, "s1", "main"), []map[string]any{message, human})
	if result, err := syncKimiSessionUsage(service, scope, home); err != nil || result.Imported != 1 {
		t.Fatalf("first sync = %+v, %v", result, err)
	}
	kimiTestWrite(t, kimiTestWirePath(home, "fork", "main"), []map[string]any{message})
	if result, err := syncKimiSessionUsage(service, scope, home); err != nil || result.Imported != 0 {
		t.Fatalf("fork sync = %+v, %v", result, err)
	}
	rows := kimiTestLoadRows(t, db)
	if len(rows) != 1 || rows[0].SessionID != "s1" || rows[0].Model != "model" ||
		rows[0].Input != 10 || rows[0].Output != 20 || rows[0].CacheRead != 30 || rows[0].CacheCreate != 40 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestKimiSessionUsageSkipsIncompleteTail(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	path := kimiTestWirePath(home, "s", "main")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"type":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if result, err := syncKimiSessionUsage(service, scope, home); err != nil || result.Imported != 0 || result.FilesScanned != 1 {
		t.Fatalf("sync = %+v, %v", result, err)
	}
	if len(kimiTestLoadRows(t, db)) != 0 {
		t.Fatal("incomplete tail imported")
	}
}

func TestKimiSessionUsageDataDirResolution(t *testing.T) {
	service, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	override := t.TempDir()
	t.Setenv("KIMI_CODE_HOME", override)
	kimiTestWrite(t, filepath.Join(override, "sessions", "wd", "env_session", "agents", "main", "wire.jsonl"),
		[]map[string]any{kimiTestMessage("env-msg", 1_700_000_000_000, 5)})

	// 非 native 作用域（如 WSL）不应读取宿主机的环境变量覆盖
	wslResult, err := syncKimiSessionUsage(service, "wsl:Ubuntu", home)
	if err != nil || wslResult.FilesScanned != 0 || wslResult.Imported != 0 {
		t.Fatalf("wsl sync = %+v, %v", wslResult, err)
	}
	nativeResult, err := syncKimiSessionUsage(service, "native", home)
	if err != nil || nativeResult.FilesScanned != 1 || nativeResult.Imported != 1 {
		t.Fatalf("native sync = %+v, %v", nativeResult, err)
	}
	rows := kimiTestLoadRows(t, db)
	if len(rows) != 1 || rows[0].SessionID != "env_session" {
		t.Fatalf("rows = %+v", rows)
	}
	// 缺失目录不是错误
	t.Setenv("KIMI_CODE_HOME", "")
	if result, err := syncKimiSessionUsage(service, scope, t.TempDir()); err != nil || result.FilesScanned != 0 {
		t.Fatalf("missing dir = %+v, %v", result, err)
	}
}
