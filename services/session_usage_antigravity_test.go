/**
 * @name: Antigravity 会话用量同步测试
 * @Descripttion: 验证 Antigravity 会话记录导入、模型继承、原生库计时解析与重复同步幂等。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_antigravity_test.go
 */

package services

import (
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
)

// encodeAntigravityDuration 编码 google.protobuf.Duration
func encodeAntigravityDuration(ms int64) []byte {
	var value []byte
	value = protowire.AppendTag(value, 1, protowire.VarintType)
	value = protowire.AppendVarint(value, uint64(ms/1000))
	value = protowire.AppendTag(value, 2, protowire.VarintType)
	value = protowire.AppendVarint(value, uint64(ms%1000*1_000_000))
	return value
}

// encodeAntigravityGeneration 构造 gen_metadata.data：字段 1 内含 19 模型、11/12 计时
func encodeAntigravityGeneration(model string, firstTokenMs int64, streamingMs int64) []byte {
	var response []byte
	if firstTokenMs > 0 {
		response = protowire.AppendTag(response, 11, protowire.BytesType)
		response = protowire.AppendBytes(response, encodeAntigravityDuration(firstTokenMs))
		response = protowire.AppendTag(response, 12, protowire.BytesType)
		response = protowire.AppendBytes(response, encodeAntigravityDuration(streamingMs))
	}
	response = protowire.AppendTag(response, 19, protowire.BytesType)
	response = protowire.AppendString(response, model)
	var data []byte
	data = protowire.AppendTag(data, 1, protowire.BytesType)
	return protowire.AppendBytes(data, response)
}

// encodeAntigravityStepMetadata 构造 steps.metadata：字段 20 → 3 为生成序号
func encodeAntigravityStepMetadata(generation uint64) []byte {
	var reference []byte
	if generation > 0 {
		reference = protowire.AppendTag(reference, 3, protowire.VarintType)
		reference = protowire.AppendVarint(reference, generation)
	}
	var metadata []byte
	metadata = protowire.AppendTag(metadata, 20, protowire.BytesType)
	return protowire.AppendBytes(metadata, reference)
}

func writeAntigravityTranscript(t *testing.T, root string, sessionID string, rows []map[string]any) string {
	t.Helper()
	path := filepath.Join(root, "brain", sessionID, antigravityTranscriptRelPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		encoded, err := json.Marshal(row)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		lines = append(lines, string(encoded))
	}
	if err := os.WriteFile(path, []byte(stringsJoinLines(lines)), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	return path
}

func writeAntigravityNativeDB(t *testing.T, root string, sessionID string, steps map[int64]uint64, generations map[uint64][]byte) {
	t.Helper()
	path := filepath.Join(root, "conversations", sessionID+".db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open native db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE steps (idx INTEGER PRIMARY KEY, step_type INTEGER, metadata BLOB);
		CREATE TABLE gen_metadata (idx INTEGER PRIMARY KEY, data BLOB);`); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	for step, generation := range steps {
		if _, err := db.Exec("INSERT INTO steps VALUES (?, 15, ?)", step, encodeAntigravityStepMetadata(generation)); err != nil {
			t.Fatalf("insert step: %v", err)
		}
	}
	for generation, data := range generations {
		if _, err := db.Exec("INSERT INTO gen_metadata VALUES (?, ?)", generation, data); err != nil {
			t.Fatalf("insert generation: %v", err)
		}
	}
}

func TestAntigravitySessionUsageImportsTimingAndModels(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	root := filepath.Join(home, ".gemini", "antigravity-cli")
	sessionID := "11111111-2222-3333-4444-555555555555"
	writeAntigravityTranscript(t, root, sessionID, []map[string]any{
		{"step_index": 0, "type": "USER_INPUT", "status": "DONE", "created_at": "2026-11-20T02:33:01Z",
			"content": "The user changed setting `Model Selection` from None to Gemini 3.8 Flash (High). No need to comment."},
		// 第 1 步无 model 字段：继承 Model Selection；原生库给出真实计时
		{"step_index": 1, "type": "PLANNER_RESPONSE", "status": "DONE", "created_at": "2026-11-20T02:33:05Z",
			"input_tokens": 9897, "cache_read_tokens": 0, "output_tokens": 607},
		// 第 2 步先写未完成，再被重写为完成：以最后一行为准
		{"step_index": 2, "type": "PLANNER_RESPONSE", "status": "RUNNING", "created_at": "2026-11-20T02:33:10Z"},
		{"step_index": 2, "type": "PLANNER_RESPONSE", "status": "DONE", "created_at": "2026-11-20T02:33:10Z",
			"model": "gemini-4-neon-high", "input_tokens": 2314, "cache_read_tokens": 25600, "cache_write_tokens": 12, "output_tokens": 5395},
		// 编排模式：显示名只代表模式，实际模型取原生库
		{"step_index": 3, "type": "PLANNER_RESPONSE", "status": "DONE", "created_at": "2026-11-20T02:33:20Z",
			"model": "gemini-pro-orchestrator-neon", "input_tokens": 10, "output_tokens": 20},
		// 未完成与零 Token 的步骤不导入
		{"step_index": 4, "type": "PLANNER_RESPONSE", "status": "RUNNING", "input_tokens": 99},
		{"step_index": 5, "type": "PLANNER_RESPONSE", "status": "DONE", "input_tokens": 0, "output_tokens": 0},
	})
	writeAntigravityNativeDB(t, root, sessionID,
		map[int64]uint64{1: 0, 2: 1, 3: 2},
		map[uint64][]byte{
			0: encodeAntigravityGeneration("gemini-3.8-flash", 1200, 3800),
			1: encodeAntigravityGeneration("gemini-4-neon", 7200, 64800),
			2: encodeAntigravityGeneration("gemini-pro-orchestrator-neon", 0, 0),
		},
	)

	result, err := syncAntigravitySessionUsage(ls, scope, home)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.FilesScanned != 1 || result.Imported != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}

	rows, err := db.Query(`SELECT platform, data_source, provider_id, model, input_tokens, output_tokens,
		cache_read_tokens, cache_create_tokens, duration_sec, first_token_sec, session_id
		FROM request_log ORDER BY created_at`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	type row struct {
		platform, source, providerID, model   string
		input, output, cacheRead, cacheCreate int
		durationSec, firstTokenSec            float64
		sessionID                             string
	}
	got := make([]row, 0, 3)
	for rows.Next() {
		var value row
		if err := rows.Scan(&value.platform, &value.source, &value.providerID, &value.model, &value.input, &value.output,
			&value.cacheRead, &value.cacheCreate, &value.durationSec, &value.firstTokenSec, &value.sessionID); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, value)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(got))
	}
	first := got[0]
	if first.platform != sessionPlatformAntigravity || first.source != requestLogDataSourceAntigravitySession ||
		first.providerID != requestLogProviderIDAntigravitySession || first.sessionID != sessionID {
		t.Fatalf("unexpected identity: %+v", first)
	}
	if first.model != "gemini-3.8-flash-high" || first.input != 9897 || first.output != 607 {
		t.Fatalf("unexpected first row: %+v", first)
	}
	if math.Abs(first.durationSec-5.0) > 0.001 || math.Abs(first.firstTokenSec-1.2) > 0.001 {
		t.Fatalf("unexpected first timing: %+v", first)
	}
	second := got[1]
	if second.model != "gemini-4-neon-high" || second.cacheRead != 25600 || second.cacheCreate != 12 || second.output != 5395 {
		t.Fatalf("unexpected second row: %+v", second)
	}
	if math.Abs(second.durationSec-72.0) > 0.001 || math.Abs(second.firstTokenSec-7.2) > 0.001 {
		t.Fatalf("unexpected second timing: %+v", second)
	}
	third := got[2]
	if third.model != "gemini-4-neon" || third.durationSec != 0 || third.firstTokenSec != 0 {
		t.Fatalf("orchestrator row should resolve native model without timing: %+v", third)
	}

	again, err := syncAntigravitySessionUsage(ls, scope, home)
	if err != nil {
		t.Fatalf("resync: %v", err)
	}
	if again.Imported != 0 {
		t.Fatalf("unchanged resync should import nothing: %+v", again)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM request_log").Scan(&count); err != nil || count != 3 {
		t.Fatalf("expected 3 rows after resync, got %d (%v)", count, err)
	}
}

func TestAntigravitySessionUsageWithoutNativeDB(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	root := filepath.Join(home, ".gemini", "antigravity")
	writeAntigravityTranscript(t, root, "ide-session", []map[string]any{
		{"step_index": 1, "type": "PLANNER_RESPONSE", "status": "DONE", "created_at": "2026-11-20T02:33:05Z",
			"model": "Gemini 4 Neon (High)", "input_tokens": 100, "output_tokens": 50},
	})
	result, err := syncAntigravitySessionUsage(ls, scope, home)
	if err != nil || result.Imported != 1 {
		t.Fatalf("sync: %+v %v", result, err)
	}
	var model string
	var durationSec float64
	if err := db.QueryRow("SELECT model, duration_sec FROM request_log").Scan(&model, &durationSec); err != nil {
		t.Fatalf("query: %v", err)
	}
	if model != "gemini-4-neon-high" || durationSec != 0 {
		t.Fatalf("unexpected row: %s %v", model, durationSec)
	}
}

func TestAntigravityModelHelpers(t *testing.T) {
	cases := map[string]string{
		"Gemini 3.8 Flash (High)": "gemini-3.8-flash-high",
		"gemini-4-neon-high":      "gemini-4-neon-high",
		"Gemini 4 Neon":           "gemini-4-neon",
	}
	for input, want := range cases {
		if got := antigravityModelID(input); got != want {
			t.Fatalf("antigravityModelID(%q) = %q, want %q", input, got, want)
		}
	}
	content := "The user changed setting `Model Selection` from Gemini 3.8 Flash to Gemini 4 Neon (High). No need to comment."
	if got := antigravityModelFromSettingsChange(content); got != "Gemini 4 Neon (High)" {
		t.Fatalf("unexpected selection: %q", got)
	}
	if got := antigravityModelFromSettingsChange("changed setting `Model Selection` from X to None."); got != "" {
		t.Fatalf("none should be ignored, got %q", got)
	}
}

func TestAntigravityKeepsResolvedModelWhenNativeDBDisappears(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	// 目录名带 # 与 %，验证原生库路径按 URI 转义后可以打开
	root := filepath.Join(home, ".gemini", "antigravity-cli")
	sessionID := "sess-hash"
	rows := []map[string]any{
		{"step_index": 1, "type": "PLANNER_RESPONSE", "status": "DONE", "created_at": "2026-11-20T02:33:05Z",
			"model": "gemini-pro-orchestrator-neon", "input_tokens": 10, "output_tokens": 20},
	}
	transcript := writeAntigravityTranscript(t, root, sessionID, rows)
	writeAntigravityNativeDB(t, root, sessionID, map[int64]uint64{1: 1},
		map[uint64][]byte{1: encodeAntigravityGeneration("gemini-pro-orchestrator-neon", 1000, 2000)})
	if _, err := syncAntigravitySessionUsage(ls, scope, home); err != nil {
		t.Fatal(err)
	}
	var model string
	if err := db.QueryRow("SELECT model FROM request_log").Scan(&model); err != nil || model != "gemini-4-neon" {
		t.Fatalf("expected native model, got %q (%v)", model, err)
	}

	if err := os.Remove(filepath.Join(root, "conversations", sessionID+".db")); err != nil {
		t.Fatal(err)
	}
	rows = append(rows, map[string]any{"step_index": 2, "type": "USER_INPUT", "status": "DONE", "content": "继续"})
	writeAntigravityTranscript(t, root, sessionID, rows)
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(transcript, future, future)
	if _, err := syncAntigravitySessionUsage(ls, scope, home); err != nil {
		t.Fatal(err)
	}
	var durationSec float64
	if err := db.QueryRow("SELECT model, duration_sec FROM request_log").Scan(&model, &durationSec); err != nil {
		t.Fatal(err)
	}
	if model != "gemini-4-neon" || math.Abs(durationSec-3) > 0.001 {
		t.Fatalf("resolved model and timing should be kept, got %q %v", model, durationSec)
	}
}

func TestAntigravityOpensNativeDBUnderSpecialCharacterPath(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := filepath.Join(t.TempDir(), "用户 #1 %41")
	root := filepath.Join(home, ".gemini", "antigravity-cli")
	writeAntigravityTranscript(t, root, "sess-x", []map[string]any{
		{"step_index": 1, "type": "PLANNER_RESPONSE", "status": "DONE", "created_at": "2026-11-20T02:33:05Z",
			"model": "gemini-4-neon-high", "input_tokens": 10, "output_tokens": 20},
	})
	writeAntigravityNativeDB(t, root, "sess-x", map[int64]uint64{1: 1},
		map[uint64][]byte{1: encodeAntigravityGeneration("gemini-4-neon", 1500, 2500)})
	if _, err := syncAntigravitySessionUsage(ls, scope, home); err != nil {
		t.Fatal(err)
	}
	var durationSec float64
	if err := db.QueryRow("SELECT duration_sec FROM request_log").Scan(&durationSec); err != nil || math.Abs(durationSec-4) > 0.001 {
		t.Fatalf("expected timing from native db, got %v (%v)", durationSec, err)
	}
}
