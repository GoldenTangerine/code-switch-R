package services

import (
	"strings"
	"testing"
)

func TestParseEventPayloadSupportsChunkedSSEWithoutSpaceAfterDataPrefix(t *testing.T) {
	reqLog := &ReqeustLog{IsStream: true}
	var remainder strings.Builder

	chunk1 := "event: message_start\ndata:{\"message\":{\"usage\":{\"input_tokens\":128"
	parseEventPayload(chunk1, ClaudeCodeParseTokenUsageFromResponse, reqLog, &remainder)

	if reqLog.InputTokens != 0 || reqLog.OutputTokens != 0 {
		t.Fatalf("chunk1 不应产生完整 token 统计，当前 input=%d output=%d", reqLog.InputTokens, reqLog.OutputTokens)
	}
	if remainder.Len() == 0 {
		t.Fatalf("chunk1 后应保留未完成的 SSE 数据")
	}

	chunk2 := ",\"output_tokens\":9,\"cache_read_input_tokens\":4}}}\n\n"
	parseEventPayload(chunk2, ClaudeCodeParseTokenUsageFromResponse, reqLog, &remainder)

	if reqLog.InputTokens != 128 {
		t.Fatalf("InputTokens = %d, 期望 128", reqLog.InputTokens)
	}
	if reqLog.OutputTokens != 9 {
		t.Fatalf("OutputTokens = %d, 期望 9", reqLog.OutputTokens)
	}
	if reqLog.CacheReadTokens != 4 {
		t.Fatalf("CacheReadTokens = %d, 期望 4", reqLog.CacheReadTokens)
	}
	if remainder.Len() != 0 {
		t.Fatalf("chunk2 完成后不应残留未解析数据，剩余长度=%d", remainder.Len())
	}
}

func TestParseEventPayloadHandlesLineByLineSSEWithoutTrailingNewline(t *testing.T) {
	reqLog := &ReqeustLog{IsStream: true}
	var remainder strings.Builder

	parseEventPayload("event: message_start", ClaudeCodeParseTokenUsageFromResponse, reqLog, &remainder)
	if remainder.Len() != 0 {
		t.Fatalf("event 行应被立即消费，remainder=%q", remainder.String())
	}

	parseEventPayload(
		"data: {\"message\":{\"usage\":{\"input_tokens\":77,\"cache_creation_input_tokens\":5}}}",
		ClaudeCodeParseTokenUsageFromResponse,
		reqLog,
		&remainder,
	)
	if reqLog.InputTokens != 77 {
		t.Fatalf("InputTokens = %d, 期望 77", reqLog.InputTokens)
	}
	if reqLog.CacheCreateTokens != 5 {
		t.Fatalf("CacheCreateTokens = %d, 期望 5", reqLog.CacheCreateTokens)
	}
	if remainder.Len() != 0 {
		t.Fatalf("data 行解析后不应残留数据，remainder=%q", remainder.String())
	}
}

func TestReqeustLogHookParsesNonStreamClaudeUsageWithPromptCompletionTokens(t *testing.T) {
	reqLog := &ReqeustLog{IsStream: false}
	hook := ReqeustLogHook(nil, "claude", reqLog)

	chunk1 := []byte("{\"type\":\"message\",\"usage\":{\"prompt_tokens\":321")
	_, _ = hook(chunk1)

	if reqLog.InputTokens != 0 || reqLog.OutputTokens != 0 {
		t.Fatalf("非完整 JSON 不应提前解析，当前 input=%d output=%d", reqLog.InputTokens, reqLog.OutputTokens)
	}

	chunk2 := []byte(",\"completion_tokens\":45,\"input_tokens_details\":{\"cached_tokens\":7}}}")
	_, _ = hook(chunk2)

	if reqLog.InputTokens != 321 {
		t.Fatalf("InputTokens = %d, 期望 321", reqLog.InputTokens)
	}
	if reqLog.OutputTokens != 45 {
		t.Fatalf("OutputTokens = %d, 期望 45", reqLog.OutputTokens)
	}
	if reqLog.CacheReadTokens != 7 {
		t.Fatalf("CacheReadTokens = %d, 期望 7", reqLog.CacheReadTokens)
	}
}

func TestReqeustLogHookStreamRequestFallsBackToRawJSONUsage(t *testing.T) {
	reqLog := &ReqeustLog{IsStream: true}
	hook := ReqeustLogHook(nil, "claude", reqLog)

	_, _ = hook([]byte(`{"usage":{"input_tokens":66,"output_tokens":8}}`))

	if reqLog.InputTokens != 66 {
		t.Fatalf("InputTokens = %d, 期望 66", reqLog.InputTokens)
	}
	if reqLog.OutputTokens != 8 {
		t.Fatalf("OutputTokens = %d, 期望 8", reqLog.OutputTokens)
	}
}

func TestClaudeCodeParseTokenUsageFromResponsePrefersStandardTokenKeys(t *testing.T) {
	reqLog := &ReqeustLog{}
	data := `{"usage":{"input_tokens":100,"prompt_tokens":50,"output_tokens":40,"completion_tokens":20}}`

	ClaudeCodeParseTokenUsageFromResponse(data, reqLog)

	if reqLog.InputTokens != 100 {
		t.Fatalf("InputTokens = %d, 期望 100（优先 input_tokens）", reqLog.InputTokens)
	}
	if reqLog.OutputTokens != 40 {
		t.Fatalf("OutputTokens = %d, 期望 40（优先 output_tokens）", reqLog.OutputTokens)
	}
}

func TestClaudeCodeParseTokenUsageFromResponseStreamUsesMaxInsteadOfSum(t *testing.T) {
	reqLog := &ReqeustLog{IsStream: true}
	data := `{"message":{"usage":{"input_tokens":128,"output_tokens":9,"cache_read_input_tokens":4}}}`

	ClaudeCodeParseTokenUsageFromResponse(data, reqLog)
	ClaudeCodeParseTokenUsageFromResponse(data, reqLog)

	if reqLog.InputTokens != 128 {
		t.Fatalf("InputTokens = %d, 期望 128（重复解析不应累加）", reqLog.InputTokens)
	}
	if reqLog.OutputTokens != 9 {
		t.Fatalf("OutputTokens = %d, 期望 9（重复解析不应累加）", reqLog.OutputTokens)
	}
	if reqLog.CacheReadTokens != 4 {
		t.Fatalf("CacheReadTokens = %d, 期望 4（重复解析不应累加）", reqLog.CacheReadTokens)
	}
}

func TestClaudeCodeParseTokenUsageFromResponseParsesCacheCreateSplit(t *testing.T) {
	reqLog := &ReqeustLog{}
	data := `{"usage":{"cache_creation_input_tokens":30,"cache_creation":{"ephemeral_5m_input_tokens":10,"ephemeral_1h_input_tokens":20}}}`

	ClaudeCodeParseTokenUsageFromResponse(data, reqLog)

	if reqLog.CacheCreateTokens != 30 {
		t.Fatalf("CacheCreateTokens = %d, 期望 30", reqLog.CacheCreateTokens)
	}
	if reqLog.Ephemeral5mTokens != 10 {
		t.Fatalf("Ephemeral5mTokens = %d, 期望 10", reqLog.Ephemeral5mTokens)
	}
	if reqLog.Ephemeral1hTokens != 20 {
		t.Fatalf("Ephemeral1hTokens = %d, 期望 20", reqLog.Ephemeral1hTokens)
	}
}

func TestClaudeCodeParseTokenUsageFromResponseDerivesCacheCreateTotalFromSplit(t *testing.T) {
	reqLog := &ReqeustLog{}
	data := `{"usage":{"cache_creation":{"ephemeral_5m_input_tokens":6,"ephemeral_1h_input_tokens":4}}}`

	ClaudeCodeParseTokenUsageFromResponse(data, reqLog)

	if reqLog.CacheCreateTokens != 10 {
		t.Fatalf("CacheCreateTokens = %d, 期望 10（由 5m/1h 明细推导）", reqLog.CacheCreateTokens)
	}
	if reqLog.Ephemeral5mTokens != 6 {
		t.Fatalf("Ephemeral5mTokens = %d, 期望 6", reqLog.Ephemeral5mTokens)
	}
	if reqLog.Ephemeral1hTokens != 4 {
		t.Fatalf("Ephemeral1hTokens = %d, 期望 4", reqLog.Ephemeral1hTokens)
	}
}

func TestClaudeCodeParseTokenUsageFromResponseStreamKeepsCacheCreateSplitConsistent(t *testing.T) {
	reqLog := &ReqeustLog{IsStream: true, Platform: "claude"}

	// 首包仅包含 cache_create 总量，尚无 5m/1h 明细。
	ClaudeCodeParseTokenUsageFromResponse(`{"usage":{"cache_creation_input_tokens":30}}`, reqLog)
	// 后续包补充 1h 明细，若按字段独立取 max 容易出现 five+one > total。
	ClaudeCodeParseTokenUsageFromResponse(
		`{"usage":{"cache_creation_input_tokens":30,"cache_creation":{"ephemeral_1h_input_tokens":20}}}`,
		reqLog,
	)
	normalizeRequestLogCacheCreateTokens(reqLog)

	if reqLog.CacheCreateTokens != 30 {
		t.Fatalf("CacheCreateTokens = %d, 期望 30（流式合并后不应膨胀）", reqLog.CacheCreateTokens)
	}
	if reqLog.Ephemeral1hTokens != 20 {
		t.Fatalf("Ephemeral1hTokens = %d, 期望 20", reqLog.Ephemeral1hTokens)
	}
	if reqLog.Ephemeral5mTokens != 0 {
		t.Fatalf("Ephemeral5mTokens = %d, 期望 0（未显式提供 5m 明细）", reqLog.Ephemeral5mTokens)
	}
	if reqLog.Ephemeral5mTokens+reqLog.Ephemeral1hTokens > reqLog.CacheCreateTokens {
		t.Fatalf(
			"split 明细非法：five(%d)+one(%d) > total(%d)",
			reqLog.Ephemeral5mTokens,
			reqLog.Ephemeral1hTokens,
			reqLog.CacheCreateTokens,
		)
	}
}

func TestCodexParseTokenUsageFromResponseStreamUsesMaxInsteadOfSum(t *testing.T) {
	reqLog := &ReqeustLog{IsStream: true}
	data := `{"response":{"usage":{"input_tokens":1110,"output_tokens":266,"input_tokens_details":{"cached_tokens":1097},"output_tokens_details":{"reasoning_tokens":104}}}}`

	CodexParseTokenUsageFromResponse(data, reqLog)
	CodexParseTokenUsageFromResponse(data, reqLog)

	if reqLog.InputTokens != 1110 {
		t.Fatalf("InputTokens = %d, 期望 1110（重复解析不应累加）", reqLog.InputTokens)
	}
	if reqLog.OutputTokens != 266 {
		t.Fatalf("OutputTokens = %d, 期望 266（重复解析不应累加）", reqLog.OutputTokens)
	}
	if reqLog.CacheReadTokens != 1097 {
		t.Fatalf("CacheReadTokens = %d, 期望 1097（重复解析不应累加）", reqLog.CacheReadTokens)
	}
	if reqLog.ReasoningTokens != 104 {
		t.Fatalf("ReasoningTokens = %d, 期望 104（重复解析不应累加）", reqLog.ReasoningTokens)
	}
}

func TestNormalizeRequestLogInputTokensSubtractsCacheTokens(t *testing.T) {
	reqLog := &ReqeustLog{
		Platform:          "codex",
		InputTokens:       111_040,
		CacheReadTokens:   109_700,
		CacheCreateTokens: 0,
	}

	normalizeRequestLogInputTokens(reqLog)

	if reqLog.InputTokens != 1_340 {
		t.Fatalf("InputTokens = %d, 期望 1340（111040 - 109700）", reqLog.InputTokens)
	}
}

func TestNormalizeRequestLogInputTokensDoesNothingForClaude(t *testing.T) {
	reqLog := &ReqeustLog{
		Platform:          "claude",
		InputTokens:       5_000,
		CacheReadTokens:   100,
		CacheCreateTokens: 50,
	}

	normalizeRequestLogInputTokens(reqLog)

	if reqLog.InputTokens != 5_000 {
		t.Fatalf("Claude InputTokens = %d, 期望保持 5000（不应对 cache tokens 做减法）", reqLog.InputTokens)
	}
}
