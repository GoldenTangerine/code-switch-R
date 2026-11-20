/**
 * @name: Gemini 会话 JSONL 回放测试
 * @Descripttion: 验证新版 Gemini CLI JSONL 会话的覆盖、回退回放及与旧版 JSON 并存时的取舍。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_gemini_jsonl_test.go
 */

package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGeminiSessionUsageReplaysJSONLDocument(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	root := t.TempDir()
	chats := filepath.Join(root, "tmp", "project-hash", "chats")
	if err := os.MkdirAll(chats, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	geminiMessage := func(id string, output int) map[string]any {
		return map[string]any{
			"id": id, "type": "gemini", "model": "gemini-3-pro-preview", "timestamp": "2026-11-20T01:00:00Z",
			"tokens": map[string]any{"input": 100, "cached": 40, "output": output, "thoughts": 5},
		}
	}
	lines := []string{
		mustJSONLine(t, map[string]any{"sessionId": "gem-1", "projectHash": "project-hash"}),
		mustJSONLine(t, geminiMessage("m1", 10)),
		mustJSONLine(t, geminiMessage("m2", 20)),
		// 回退到 m2：m2 及之后被删除
		mustJSONLine(t, map[string]any{"$rewindTo": "m2"}),
		mustJSONLine(t, geminiMessage("m3", 30)),
		// 同 id 覆盖：m1 输出被改写
		mustJSONLine(t, geminiMessage("m1", 11)),
	}
	jsonlPath := filepath.Join(chats, "session-2026-11-20.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(stringsJoinLines(lines)), 0o644); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}
	// 同名旧版 .json 并存时应被忽略
	writeGeminiSessionFile(t, filepath.Join(chats, "session-2026-11-20.json"), "gem-1", []map[string]any{geminiMessage("legacy", 999)})

	result, err := syncGeminiSessionUsage(ls, scope, root)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.FilesScanned != 1 || result.Imported != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	rows, err := db.Query("SELECT output_tokens, input_tokens, cache_read_tokens FROM request_log ORDER BY output_tokens")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	outputs := make([]int, 0, 2)
	for rows.Next() {
		var output, input, cacheRead int
		if err := rows.Scan(&output, &input, &cacheRead); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if input != 60 || cacheRead != 40 {
			t.Fatalf("unexpected input split: %d %d", input, cacheRead)
		}
		outputs = append(outputs, output)
	}
	if len(outputs) != 2 || outputs[0] != 11 || outputs[1] != 30 {
		t.Fatalf("unexpected outputs: %v", outputs)
	}
}

func TestGeminiJSONLCheckpointDoesNotDuplicateMessages(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	root := t.TempDir()
	chats := filepath.Join(root, "tmp", "hash", "chats")
	if err := os.MkdirAll(chats, 0o755); err != nil {
		t.Fatal(err)
	}
	message := func(id string, output int) map[string]any {
		return map[string]any{"id": id, "type": "gemini", "model": "gemini-3-pro-preview", "timestamp": "2026-11-20T01:00:00Z",
			"tokens": map[string]any{"input": 10, "output": output}}
	}
	path := filepath.Join(chats, "session-a.jsonl")
	lines := []string{
		mustJSONLine(t, map[string]any{"sessionId": "gem-a"}),
		mustJSONLine(t, message("A", 1)), mustJSONLine(t, message("B", 2)), mustJSONLine(t, message("C", 3)),
	}
	if err := os.WriteFile(path, []byte(stringsJoinLines(lines)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := syncGeminiSessionUsage(ls, scope, root); err != nil {
		t.Fatal(err)
	}
	// $set 检查点把消息重排为 [B, C]，位置序号变化但不应重复导入
	lines = append(lines, mustJSONLine(t, map[string]any{"$set": map[string]any{"messages": []any{message("B", 2), message("C", 3)}}}))
	if err := os.WriteFile(path, []byte(stringsJoinLines(lines)), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(path, future, future)
	if _, err := syncGeminiSessionUsage(ls, scope, root); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM request_log").Scan(&count); err != nil || count != 3 {
		t.Fatalf("expected 3 rows, got %d (%v)", count, err)
	}
}
