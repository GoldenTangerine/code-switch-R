/**
 * @name: 会话用量耗时估算测试
 * @Descripttion: 验证 Claude 父链起点、思考块首字与 Codex 请求边界的耗时估算规则。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_timing_test.go
 */

package services

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func mustJSONLine(t *testing.T, value map[string]any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(encoded)
}

func claudeAssistantLine(t *testing.T, uuid, parent, messageID, timestamp string, blockIndex int, output int, stopReason any, extra map[string]any) string {
	message := map[string]any{
		"id":          messageID,
		"model":       "claude-opus-4-8",
		"stop_reason": stopReason,
		"usage": map[string]any{
			"input_tokens": 10, "output_tokens": output,
			"cache_read_input_tokens": 0, "cache_creation_input_tokens": 0,
		},
	}
	value := map[string]any{
		"type": "assistant", "uuid": uuid, "parentUuid": parent, "sessionId": "session-t",
		"timestamp": timestamp, "apiBlockIndex": blockIndex, "message": message,
	}
	for key, item := range extra {
		value[key] = item
	}
	return mustJSONLine(t, value)
}

func TestClaudeSessionUsageEstimatesLatencyAndThinkingFirstToken(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	path := filepath.Join(t.TempDir(), "projects", "demo", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	lines := []string{
		mustJSONLine(t, map[string]any{"type": "user", "uuid": "u1", "timestamp": "2026-11-20T01:00:00.000Z"}),
		// attachment 行时间戳不可靠，起点要越过它
		mustJSONLine(t, map[string]any{"type": "attachment", "uuid": "a1", "parentUuid": "u1", "timestamp": "2026-11-20T01:00:03.000Z"}),
	}
	thinking := claudeAssistantLine(t, "m1-b0", "a1", "msg_1", "2026-11-20T01:00:04.000Z", 0, 900, nil, map[string]any{"thinkingDurationMs": 2000})
	var thinkingValue map[string]any
	_ = json.Unmarshal([]byte(thinking), &thinkingValue)
	mapValue(thinkingValue["message"])["content"] = []any{map[string]any{"type": "thinking", "thinking": ""}}
	lines = append(lines,
		mustJSONLine(t, thinkingValue),
		claudeAssistantLine(t, "m1-b1", "m1-b0", "msg_1", "2026-11-20T01:00:10.000Z", 1, 900, "end_turn", nil),
		// 没有 stop_reason 的回复不估算
		mustJSONLine(t, map[string]any{"type": "user", "uuid": "u2", "parentUuid": "m1-b1", "timestamp": "2026-11-20T01:01:00.000Z"}),
		claudeAssistantLine(t, "m2-b0", "u2", "msg_2", "2026-11-20T01:01:05.000Z", 0, 300, nil, nil),
	)
	if err := os.WriteFile(path, []byte(stringsJoinLines(lines)), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := syncClaudeSessionFile(ls, scope, path); err != nil {
		t.Fatalf("sync: %v", err)
	}

	readTiming := func(messageID string) (float64, float64) {
		var durationSec, firstTokenSec float64
		recordID := stableSessionRecordID(scope, "claude", messageID)
		if err := db.QueryRow("SELECT duration_sec, first_token_sec FROM request_log WHERE source_record_id = ?", recordID).
			Scan(&durationSec, &firstTokenSec); err != nil {
			t.Fatalf("query %s: %v", messageID, err)
		}
		return durationSec, firstTokenSec
	}
	durationSec, firstTokenSec := readTiming("msg_1")
	// 起点 01:00:00，结束 01:00:10；思考块 01:00:04 写完、持续 2s → 01:00:02 开始输出
	if math.Abs(durationSec-10) > 0.001 || math.Abs(firstTokenSec-2) > 0.001 {
		t.Fatalf("unexpected msg_1 timing: %v %v", durationSec, firstTokenSec)
	}
	if durationSec, firstTokenSec := readTiming("msg_2"); durationSec != 0 || firstTokenSec != 0 {
		t.Fatalf("reply without stop_reason should not be timed: %v %v", durationSec, firstTokenSec)
	}
}

func TestClaudeSessionTimingSeedsChainBeforeCursor(t *testing.T) {
	tracker := newClaudeSessionTimingTracker()
	path := filepath.Join(t.TempDir(), "seed.jsonl")
	head := mustJSONLine(t, map[string]any{"type": "user", "uuid": "u1", "timestamp": "2026-11-20T01:00:00Z"}) + "\n"
	if err := os.WriteFile(path, []byte(head), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	tracker.SeedBefore(path, int64(len(head)))
	var reply map[string]any
	_ = json.Unmarshal([]byte(claudeAssistantLine(t, "m1", "u1", "msg_1", "2026-11-20T01:00:30Z", 0, 500, "end_turn", nil)), &reply)
	tracker.Observe(reply)
	if durationSec, _ := tracker.Estimate("msg_1", true); math.Abs(durationSec-30) > 0.001 {
		t.Fatalf("expected 30s from seeded parent, got %v", durationSec)
	}
}

func TestCodexRequestTimerUsesBoundariesAndItemStart(t *testing.T) {
	line := func(timestamp string, kind string, payload map[string]any) map[string]any {
		return map[string]any{"timestamp": timestamp, "type": kind, "payload": payload}
	}
	timer := codexRequestTimer{}
	itemStartMs, _ := sessionTimestampMs("2026-11-20T01:00:03.000Z")
	timer.ObserveLine(line("2026-11-20T01:00:00.000Z", "event_msg", map[string]any{"type": "task_started"}))
	timer.ObserveLine(line("2026-11-20T01:00:01.000Z", "response_item", map[string]any{"type": "message", "role": "user"}))
	timer.ObserveLine(line("2026-11-20T01:00:05.000Z", "response_item", map[string]any{"type": "reasoning"}))
	timer.ObserveLine(line("2026-11-20T01:00:05.000Z", "event_msg", map[string]any{
		"type": "item_completed", "item": map[string]any{"type": "Reasoning"}, "started_at_ms": float64(itemStartMs),
	}))
	timer.ObserveLine(line("2026-11-20T01:00:09.000Z", "response_item", map[string]any{"type": "message", "role": "assistant"}))
	endMs, _ := sessionTimestampMs("2026-11-20T01:00:09.050Z")
	durationSec, firstTokenSec := timer.Finish(endMs, nil)
	if math.Abs(durationSec-8.05) > 0.001 {
		t.Fatalf("unexpected duration: %v", durationSec)
	}
	// 起点为用户消息 01:00:01，思考项 01:00:03 开始生成
	if math.Abs(firstTokenSec-2) > 0.001 {
		t.Fatalf("unexpected first token: %v", firstTokenSec)
	}

	// 下一次请求在工具结果之后开始；token_count 紧跟工具结果写出时改用最后一个输出项作为结束
	timer.ObserveLine(line("2026-11-20T01:00:20.000Z", "response_item", map[string]any{"type": "function_call_output"}))
	timer.ObserveLine(line("2026-11-20T01:00:23.000Z", "response_item", map[string]any{"type": "function_call"}))
	timer.ObserveLine(line("2026-11-20T01:00:30.000Z", "response_item", map[string]any{"type": "function_call_output"}))
	tokenMs, _ := sessionTimestampMs("2026-11-20T01:00:30.020Z")
	if durationSec, _ := timer.Finish(tokenMs, nil); math.Abs(durationSec-3) > 0.001 {
		t.Fatalf("expected 3s ending at last model output, got %v", durationSec)
	}
}

func TestSessionUsageResyncOnlyBackfillsTimingWithoutRepricing(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	path := filepath.Join(t.TempDir(), "projects", "demo", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	lines := []string{
		mustJSONLine(t, map[string]any{"type": "user", "uuid": "u1", "timestamp": "2026-11-20T01:00:00.250Z"}),
		claudeAssistantLine(t, "m1", "u1", "msg_1", "2026-11-20T01:00:12.750Z", 0, 800, "end_turn", nil),
	}
	if err := os.WriteFile(path, []byte(stringsJoinLines(lines)), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := syncClaudeSessionFile(ls, scope, path); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	recordID := stableSessionRecordID(scope, "claude", "msg_1")
	var rowID int64
	if err := db.QueryRow("SELECT id FROM request_log WHERE source_record_id = ?", recordID).Scan(&rowID); err != nil {
		t.Fatalf("query id: %v", err)
	}
	// 模拟升级前的历史行：没有计时、成本按旧价格记账、已与代理行配对
	if _, err := db.Exec("UPDATE request_log SET duration_sec = 0, first_token_sec = 0, total_cost = 1.23 WHERE id = ?", rowID); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	if _, err := db.Exec("INSERT INTO session_usage_dedup (session_log_id, proxy_log_id) VALUES (?, ?)", rowID, 999999); err != nil {
		t.Fatalf("seed dedup: %v", err)
	}
	if _, err := db.Exec("DELETE FROM session_log_sync"); err != nil {
		t.Fatalf("reset cursor: %v", err)
	}

	imported, skipped, err := syncClaudeSessionFile(ls, scope, path)
	if err != nil {
		t.Fatalf("resync: %v", err)
	}
	if imported != 1 || skipped != 0 {
		t.Fatalf("expected one timing backfill, got imported=%d skipped=%d", imported, skipped)
	}
	var durationSec, totalCost float64
	if err := db.QueryRow("SELECT duration_sec, total_cost FROM request_log WHERE id = ?", rowID).Scan(&durationSec, &totalCost); err != nil {
		t.Fatalf("query row: %v", err)
	}
	if math.Abs(durationSec-12.5) > 0.001 || totalCost != 1.23 {
		t.Fatalf("expected timing backfill only, got duration=%v cost=%v", durationSec, totalCost)
	}
	var dedupCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM session_usage_dedup WHERE session_log_id = ?", rowID).Scan(&dedupCount); err != nil || dedupCount != 1 {
		t.Fatalf("dedup match should be kept, got %d (%v)", dedupCount, err)
	}

	// 第三次同步：内容与计时均未变化，应全部跳过
	if _, err := db.Exec("DELETE FROM session_log_sync"); err != nil {
		t.Fatalf("reset cursor: %v", err)
	}
	if imported, skipped, err := syncClaudeSessionFile(ls, scope, path); err != nil || imported != 0 || skipped != 1 {
		t.Fatalf("expected unchanged resync to skip, got imported=%d skipped=%d err=%v", imported, skipped, err)
	}
}
