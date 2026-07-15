package services

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestParseEventPayloadSupportsChunkedSSEWithoutSpaceAfterDataPrefix(t *testing.T) {
	reqLog := &ReqeustLog{IsStream: true}
	var remainder strings.Builder

	chunk1 := "event: message_start\ndata:{\"message\":{\"usage\":{\"input_tokens\":128"
	parseEventPayload(chunk1, ClaudeCodeParseTokenUsageFromResponse, reqLog, &remainder, "claude")

	if reqLog.InputTokens != 0 || reqLog.OutputTokens != 0 {
		t.Fatalf("chunk1 不应产生完整 token 统计，当前 input=%d output=%d", reqLog.InputTokens, reqLog.OutputTokens)
	}
	if remainder.Len() == 0 {
		t.Fatalf("chunk1 后应保留未完成的 SSE 数据")
	}

	chunk2 := ",\"output_tokens\":9,\"cache_read_input_tokens\":4}}}\n\n"
	parseEventPayload(chunk2, ClaudeCodeParseTokenUsageFromResponse, reqLog, &remainder, "claude")

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

func TestResponsesStreamDiagnosticsCaptureRemoteCompactionLifecycle(t *testing.T) {
	reqLog := &ReqeustLog{IsStream: true}
	hook := ReqeustLogHook(nil, "codex", reqLog)

	_, _ = hook([]byte("data: {\"type\":\"response.created\"}\n"))
	_, _ = hook([]byte("data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"context_compaction\",\"encrypted_content\":\"opaque\"}}\n"))
	_, _ = hook([]byte("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n"))

	if reqLog.StreamLastEvent != "response.completed" || reqLog.StreamTerminalEvent != "response.completed" {
		t.Fatalf("流生命周期记录错误: %#v", reqLog)
	}
	if !reqLog.StreamCompactionObserved {
		t.Fatalf("应识别 context_compaction 输出项")
	}
}

func TestResponsesCompactionRequestDetection(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "remote-v2-trigger", body: `{"input":[{"type":"context_compaction"}]}`, want: true},
		{name: "new-trigger", body: `{"input":[{"type":"compaction"}]}`, want: true},
		{name: "retained-compaction", body: `{"input":[{"type":"context_compaction","encrypted_content":"opaque"}]}`, want: false},
		{name: "server-side", body: `{"context_management":[{"type":"compaction","compact_threshold":200000}]}`, want: true},
		{name: "normal", body: `{"input":[{"type":"message","role":"user"}]}`, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isResponsesCompactionRequest([]byte(test.body)); got != test.want {
				t.Fatalf("isResponsesCompactionRequest()=%v，期望 %v", got, test.want)
			}
		})
	}
}

func TestClassifyStreamErrorKind(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{err: fmt.Errorf("error streaming response: %w", io.ErrUnexpectedEOF), want: "unexpected_eof"},
		{err: errors.New("error streaming response: read: connection reset by peer"), want: "connection_reset"},
		{err: errors.New("error streaming response: i/o timeout"), want: "timeout"},
		{err: fmt.Errorf("%w: empty upstream stream", errIncompleteStream), want: "empty_stream"},
	}
	for _, test := range tests {
		if got := classifyStreamErrorKind(test.err); got != test.want {
			t.Fatalf("classifyStreamErrorKind(%q)=%q，期望 %q", test.err, got, test.want)
		}
	}
}

func TestParseEventPayloadHandlesLineByLineSSEWithoutTrailingNewline(t *testing.T) {
	reqLog := &ReqeustLog{IsStream: true}
	var remainder strings.Builder

	parseEventPayload("event: message_start", ClaudeCodeParseTokenUsageFromResponse, reqLog, &remainder, "claude")
	if remainder.Len() != 0 {
		t.Fatalf("event 行应被立即消费，remainder=%q", remainder.String())
	}

	parseEventPayload(
		"data: {\"message\":{\"usage\":{\"input_tokens\":77,\"cache_creation_input_tokens\":5}}}",
		ClaudeCodeParseTokenUsageFromResponse,
		reqLog,
		&remainder,
		"claude",
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

func TestReqeustLogHookCodexStreamAcceptsCompletedJSONFallback(t *testing.T) {
	reqLog := &ReqeustLog{IsStream: true}
	hook := ReqeustLogHook(nil, "codex", reqLog)

	payload := `{"type":"response","status":"completed","response":{"usage":{"input_tokens":66,"output_tokens":8}}}`
	_, _ = hook([]byte(payload))

	if err := validateStreamCompletion("codex", reqLog); err != nil {
		t.Fatalf("完整 JSON fallback 不应被视为未完成: %v", err)
	}
}

func TestOpenAIResponsesStreamLifecycleHookCombinesMultilineDataPayload(t *testing.T) {
	reqLog := &ReqeustLog{IsStream: true, streamCompletionRequired: true}
	hook := openAIResponsesStreamLifecycleHook(reqLog)

	_, _ = hook([]byte("event: response.completed"))
	_, _ = hook([]byte(`data: {"response":`))
	_, _ = hook([]byte(`data: {"status":"completed"}}`))
	_, _ = hook([]byte(""))

	if err := validateStreamCompletion("claude", reqLog); err != nil {
		t.Fatalf("多行 response.completed 不应被误判为未完成: %v", err)
	}
}

func TestClaudeResponsesStreamSessionHookCombinesMultilineDataPayload(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	provider := Provider{ID: 1, Name: "OpenAI Responses", APIURL: "https://example.test", APIKey: "test-key"}
	plan := providerRequestPlan{ContinuationSessionKey: "session-a"}
	hook := relay.newClaudeResponsesStreamSessionHook(provider, plan)

	_, _ = hook([]byte("event: response.created"))
	_, _ = hook([]byte(`data: {"response":`))
	_, _ = hook([]byte(`data: {"id":"resp_multiline"}}`))
	_, _ = hook([]byte(""))
	_, _ = hook([]byte("event: response.completed"))
	_, _ = hook([]byte(`data: {"response":`))
	_, _ = hook([]byte(`data: {"status":"completed"}}`))
	_, _ = hook([]byte(""))

	if got := relay.getClaudeResponsesPreviousResponseID(provider, "session-a"); got != "resp_multiline" {
		t.Fatalf("多行 SSE session hook 绑定 response_id=%q，期望 resp_multiline", got)
	}
}

func TestClaudeResponsesSessionBindingsSweepExpiredAndCapSize(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	provider := Provider{ID: 1, Name: "OpenAI Responses", APIURL: "https://example.test", APIKey: "test-key"}
	now := time.Now()

	relay.claudeResponsesMu.Lock()
	relay.claudeResponses = map[string]claudeResponsesSessionBinding{
		"expired": {ResponseID: "resp_expired", ExpiresAt: now.Add(-time.Minute)},
	}
	for i := 0; i < claudeResponsesMaxSessionBindings+10; i++ {
		relay.claudeResponses[fmt.Sprintf("old-%04d", i)] = claudeResponsesSessionBinding{
			ResponseID: fmt.Sprintf("resp_%04d", i),
			ExpiresAt:  now.Add(time.Duration(i) * time.Second),
		}
	}
	relay.claudeResponsesMu.Unlock()

	relay.bindClaudeResponsesPreviousResponseID(provider, "session-new", "resp_new")

	relay.claudeResponsesMu.Lock()
	defer relay.claudeResponsesMu.Unlock()
	if _, ok := relay.claudeResponses["expired"]; ok {
		t.Fatalf("过期 claudeResponses session binding 未被清理")
	}
	if len(relay.claudeResponses) > claudeResponsesMaxSessionBindings {
		t.Fatalf("claudeResponses session binding 数=%d，期望不超过 %d", len(relay.claudeResponses), claudeResponsesMaxSessionBindings)
	}
	if _, ok := relay.claudeResponses["old-0000"]; ok {
		t.Fatalf("超过容量时应优先清理最早过期的 session binding")
	}
}

func TestClaudeResponsesMemoryKeysHashProviderAPIKey(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	provider := Provider{ID: 1, Name: "OpenAI Responses", APIURL: "https://example.test", APIKey: "sk-secret-value"}

	sessionKey := relay.claudeResponsesSessionKey(provider, "session-a")
	promptCacheKey := relay.openAICompatPromptCacheDisableKey(provider, "session-a")

	for _, key := range []string{sessionKey, promptCacheKey} {
		if strings.Contains(key, provider.APIKey) {
			t.Fatalf("内存 key 不应包含明文 API key: %q", key)
		}
		if !strings.Contains(key, "sha256:") {
			t.Fatalf("内存 key 应包含 API key 短哈希: %q", key)
		}
	}
}

func TestIsClientWriteAbortErrorOnlyTreatsDownstreamWriteFailuresAsClientAbort(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "downstream broken pipe",
			err:  fmt.Errorf("error writing response: broken pipe"),
			want: true,
		},
		{
			name: "upstream streaming reset",
			err:  fmt.Errorf("error streaming response: connection reset by peer"),
			want: false,
		},
		{
			name: "upstream non-standard read canceled",
			err:  fmt.Errorf("error reading non-standard response: context canceled"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isClientWriteAbortError(tt.err); got != tt.want {
				t.Fatalf("isClientWriteAbortError(%v) = %v, 期望 %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestUpdateFirstTokenFromPayloadSkipsNonStreamRequest(t *testing.T) {
	reqLog := &ReqeustLog{
		IsStream:         false,
		RequestStartedAt: time.Now().Add(-1 * time.Second),
	}

	updateFirstTokenFromPayload(`{"choices":[{"delta":{"content":"hello"}}]}`, "codex", reqLog)

	if reqLog.FirstTokenSec != 0 {
		t.Fatalf("非流式请求不应记录 TTFT，当前 first_token_sec=%.6f", reqLog.FirstTokenSec)
	}
}

func TestUpdateFirstTokenFromPayloadDetectsCodexRootDeltaEvent(t *testing.T) {
	reqLog := &ReqeustLog{
		IsStream:         true,
		RequestStartedAt: time.Now().Add(-1200 * time.Millisecond),
	}

	updateFirstTokenFromPayload(`{"type":"response.output_text.delta","delta":"hello"}`, "codex", reqLog)

	if reqLog.FirstTokenSec <= 0 {
		t.Fatalf("Codex root delta 事件应触发 TTFT 记录，当前 first_token_sec=%.6f", reqLog.FirstTokenSec)
	}
	if reqLog.FirstTokenSec > 10 {
		t.Fatalf("TTFT 数值异常，当前 first_token_sec=%.6f", reqLog.FirstTokenSec)
	}
}

func TestUpdateFirstTokenFromPayloadDetectsClaudeTextDelta(t *testing.T) {
	reqLog := &ReqeustLog{
		IsStream:         true,
		RequestStartedAt: time.Now().Add(-900 * time.Millisecond),
	}

	updateFirstTokenFromPayload(`{"type":"content_block_delta","delta":{"text":"hi"}}`, "claude", reqLog)

	if reqLog.FirstTokenSec <= 0 {
		t.Fatalf("Claude delta 文本应触发 TTFT 记录，当前 first_token_sec=%.6f", reqLog.FirstTokenSec)
	}
}

func TestUpdateFirstTokenFromPayloadDetectsCustomAnthropicData(t *testing.T) {
	reqLog := &ReqeustLog{
		IsStream:         true,
		RequestStartedAt: time.Now().Add(-900 * time.Millisecond),
	}

	updateFirstTokenFromPayload(`{"type":"message_start","message":{"id":"msg_1"}}`, "custom:tool-a", reqLog)

	if reqLog.FirstTokenSec <= 0 {
		t.Fatalf("自定义 Anthropic data 事件应触发 TTFT，当前 first_token_sec=%.6f", reqLog.FirstTokenSec)
	}
}

func TestUpdateFirstTokenFromPayloadDetectsGeminiCandidateText(t *testing.T) {
	reqLog := &ReqeustLog{
		IsStream:         true,
		RequestStartedAt: time.Now().Add(-900 * time.Millisecond),
	}

	updateFirstTokenFromPayload(`{"candidates":[{"content":{"parts":[{"text":"hello"}]}}]}`, "gemini", reqLog)

	if reqLog.FirstTokenSec <= 0 {
		t.Fatalf("Gemini candidates 文本应触发 TTFT 记录，当前 first_token_sec=%.6f", reqLog.FirstTokenSec)
	}
}

func TestUpdateFirstTokenFromPayloadSkipsCodexPreambleAndTerminalEvents(t *testing.T) {
	reqLog := &ReqeustLog{
		IsStream:         true,
		RequestStartedAt: time.Now().Add(-1 * time.Second),
	}

	for _, payload := range []string{
		`{"type":"response.created"}`,
		`{"type":"response.in_progress"}`,
		`{"type":"response.completed","response":{"usage":{"output_tokens":10}}}`,
	} {
		updateFirstTokenFromPayload(payload, "codex", reqLog)
	}

	if reqLog.FirstTokenSec != 0 {
		t.Fatalf("Codex 前置或终态事件不应触发 TTFT，当前 first_token_sec=%.6f", reqLog.FirstTokenSec)
	}
}

func TestUpdateFirstTokenFromPayloadDetectsCodexToolOutputEvent(t *testing.T) {
	reqLog := &ReqeustLog{
		IsStream:         true,
		RequestStartedAt: time.Now().Add(-1 * time.Second),
	}

	updateFirstTokenFromPayload(`{"type":"response.output_item.added","item":{"type":"function_call"}}`, "codex", reqLog)

	if reqLog.FirstTokenSec <= 0 {
		t.Fatalf("Codex 工具调用输出事件应触发 TTFT，当前 first_token_sec=%.6f", reqLog.FirstTokenSec)
	}
}

func TestUpdateFirstTokenFromPayloadSkipsOpenAIUsageOnlyChunk(t *testing.T) {
	reqLog := &ReqeustLog{
		IsStream:         true,
		RequestStartedAt: time.Now().Add(-1 * time.Second),
	}

	updateFirstTokenFromPayload(`{"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2}}`, "codex", reqLog)

	if reqLog.FirstTokenSec != 0 {
		t.Fatalf("OpenAI usage-only chunk 不应触发 TTFT，当前 first_token_sec=%.6f", reqLog.FirstTokenSec)
	}
}

func TestUpdateFirstTokenFromPayloadDetectsGeminiFunctionCall(t *testing.T) {
	reqLog := &ReqeustLog{
		IsStream:         true,
		RequestStartedAt: time.Now().Add(-1 * time.Second),
	}

	updateFirstTokenFromPayload(`{"candidates":[{"content":{"parts":[{"functionCall":{"name":"lookup"}}]}}]}`, "gemini", reqLog)

	if reqLog.FirstTokenSec <= 0 {
		t.Fatalf("Gemini 工具调用内容应触发 TTFT，当前 first_token_sec=%.6f", reqLog.FirstTokenSec)
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
