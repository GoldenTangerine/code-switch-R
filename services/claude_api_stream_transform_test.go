package services

import (
	"strings"
	"testing"
)

func TestClaudeResponsesStreamTransformHookSupportsNamedEvents(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)

	if flush, data := hook([]byte("event: response.created")); flush || len(data) != 0 {
		t.Fatalf("event 行不应直接输出，flush=%v data=%q", flush, string(data))
	}

	flush, data := hook([]byte(`data: {"response":{"id":"resp_1","model":"gpt-5","usage":{"input_tokens":12,"output_tokens":3}}}`))
	if !flush {
		t.Fatalf("response.created 的 data 行应输出转换结果")
	}

	got := string(data)
	if !strings.Contains(got, "event: message_start\n") {
		t.Fatalf("缺少 message_start 事件: %s", got)
	}
	if !strings.Contains(got, `"model":"gpt-5"`) {
		t.Fatalf("message_start 未携带 model: %s", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("转换后的 SSE 数据应以换行结尾，实际=%q", got)
	}
}

func TestClaudeChatStreamTransformHookSubtractsCachedInputTokens(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIChat, true)

	_, _ = hook([]byte(`data: {"id":"chatcmpl_1","model":"gpt-5.4","choices":[{"delta":{"content":"hello"}}]}`))
	flush, data := hook([]byte(`data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":80}}}`))

	if !flush {
		t.Fatalf("Chat completion finish chunk 应输出 message_delta")
	}
	got := string(data)
	if !strings.Contains(got, `"input_tokens":20`) {
		t.Fatalf("Chat 流式 usage 应扣除 cached tokens: %s", got)
	}
	if !strings.Contains(got, `"cache_read_input_tokens":80`) {
		t.Fatalf("Chat 流式 usage 应保留 cache_read_input_tokens: %s", got)
	}
}

func TestClaudeResponsesStreamTransformHookConvertsNamedDeltaAndCompletedEvents(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)

	_, _ = hook([]byte("event: response.created"))
	_, _ = hook([]byte(`data: {"response":{"id":"resp_1","model":"gpt-5","usage":{"input_tokens":1,"output_tokens":0}}}`))

	_, _ = hook([]byte("event: response.output_text.delta"))
	flush, deltaData := hook([]byte(`data: {"item_id":"msg_1","content_index":0,"delta":"hello"}`))
	if !flush {
		t.Fatalf("response.output_text.delta 的 data 行应输出转换结果")
	}
	gotDelta := string(deltaData)
	if !strings.Contains(gotDelta, `"type":"content_block_start"`) {
		t.Fatalf("delta 事件缺少 content_block_start: %s", gotDelta)
	}
	if !strings.Contains(gotDelta, `"type":"content_block_delta"`) {
		t.Fatalf("delta 事件缺少 content_block_delta: %s", gotDelta)
	}
	if !strings.Contains(gotDelta, `"text":"hello"`) {
		t.Fatalf("delta 事件未携带文本: %s", gotDelta)
	}
	if !strings.HasSuffix(gotDelta, "\n") {
		t.Fatalf("delta 转换结果应以换行结尾，实际=%q", gotDelta)
	}

	_, _ = hook([]byte("event: response.completed"))
	flush, completedData := hook([]byte(`data: {"response":{"status":"completed","usage":{"input_tokens":1,"output_tokens":5}}}`))
	if !flush {
		t.Fatalf("response.completed 的 data 行应输出转换结果")
	}
	gotCompleted := string(completedData)
	if !strings.Contains(gotCompleted, `"type":"message_delta"`) {
		t.Fatalf("completed 事件缺少 message_delta: %s", gotCompleted)
	}
	if !strings.Contains(gotCompleted, `"type":"message_stop"`) {
		t.Fatalf("completed 事件缺少 message_stop: %s", gotCompleted)
	}
	if !strings.HasSuffix(gotCompleted, "\n") {
		t.Fatalf("completed 转换结果应以换行结尾，实际=%q", gotCompleted)
	}
}

func TestClaudeResponsesStreamTransformHookConvertsFunctionCallEvents(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)

	_, _ = hook([]byte("event: response.created"))
	_, _ = hook([]byte(`data: {"response":{"id":"resp_1","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":0}}}`))

	_, _ = hook([]byte("event: response.output_item.added"))
	flush, startData := hook([]byte(`data: {"output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"toolu_1","name":"List"}}`))
	if !flush {
		t.Fatalf("response.output_item.added 应输出 tool_use start")
	}
	gotStart := string(startData)
	if !strings.Contains(gotStart, `"type":"tool_use"`) ||
		!strings.Contains(gotStart, `"id":"toolu_1"`) ||
		!strings.Contains(gotStart, `"name":"List"`) {
		t.Fatalf("tool_use start 转换不正确: %s", gotStart)
	}

	_, _ = hook([]byte("event: response.function_call_arguments.delta"))
	flush, deltaData := hook([]byte(`data: {"output_index":0,"item_id":"fc_1","delta":"{\"file_path\":\"a.go\"}"}`))
	if !flush {
		t.Fatalf("response.function_call_arguments.delta 应输出 input_json_delta")
	}
	if !strings.Contains(string(deltaData), `"partial_json":"{\"file_path\":\"a.go\"}"`) {
		t.Fatalf("tool args delta 转换不正确: %s", string(deltaData))
	}

	_, _ = hook([]byte("event: response.function_call_arguments.done"))
	flush, doneData := hook([]byte(`data: {"output_index":0,"item_id":"fc_1"}`))
	if !flush || !strings.Contains(string(doneData), `"type":"content_block_stop"`) {
		t.Fatalf("tool args done 应关闭 content block: flush=%v data=%s", flush, string(doneData))
	}
}

func TestClaudeResponsesStreamTransformHookUsesDoneOnlyFunctionArguments(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)

	_, _ = hook([]byte("event: response.created"))
	_, _ = hook([]byte(`data: {"response":{"id":"resp_1","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":0}}}`))

	_, _ = hook([]byte("event: response.output_item.added"))
	_, _ = hook([]byte(`data: {"output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"toolu_1","name":"Read"}}`))

	_, _ = hook([]byte("event: response.function_call_arguments.done"))
	flush, doneData := hook([]byte(`data: {"output_index":0,"item_id":"fc_1","item":{"type":"function_call","arguments":"{\"file_path\":\"done.go\"}"}}`))
	if !flush {
		t.Fatalf("done-only function arguments 应输出")
	}
	got := string(doneData)
	if !strings.Contains(got, `"partial_json":"{\"file_path\":\"done.go\"}"`) {
		t.Fatalf("done-only function arguments 未转为 input_json_delta: %s", got)
	}
	if !strings.Contains(got, `"type":"content_block_stop"`) {
		t.Fatalf("done-only function arguments 后应关闭 content block: %s", got)
	}
}

func TestClaudeResponsesStreamTransformHookUsesOutputItemDoneFunctionArguments(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)

	_, _ = hook([]byte("event: response.created"))
	_, _ = hook([]byte(`data: {"response":{"id":"resp_1","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":0}}}`))

	_, _ = hook([]byte("event: response.output_item.done"))
	flush, data := hook([]byte(`data: {"output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"toolu_1","name":"List","arguments":"{\"file_path\":\"done.go\"}"}}`))
	if !flush {
		t.Fatalf("output_item.done function_call 应输出 tool_use block")
	}
	got := string(data)
	if !strings.Contains(got, `"type":"tool_use"`) ||
		!strings.Contains(got, `"id":"toolu_1"`) ||
		!strings.Contains(got, `"name":"List"`) {
		t.Fatalf("output_item.done 未创建 tool_use block: %s", got)
	}
	if !strings.Contains(got, `"partial_json":"{\"file_path\":\"done.go\"}"`) {
		t.Fatalf("output_item.done 未输出完整 arguments: %s", got)
	}
	if !strings.Contains(got, `"type":"content_block_stop"`) {
		t.Fatalf("output_item.done 应关闭 tool_use block: %s", got)
	}
}

func TestClaudeResponsesStreamTransformHookDoesNotDuplicateToolUseOnOutputItemDone(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)

	_, _ = hook([]byte("event: response.created"))
	_, _ = hook([]byte(`data: {"response":{"id":"resp_1","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":0}}}`))
	_, _ = hook([]byte("event: response.output_item.added"))
	_, _ = hook([]byte(`data: {"output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"toolu_1","name":"List"}}`))
	_, _ = hook([]byte("event: response.function_call_arguments.delta"))
	_, _ = hook([]byte(`data: {"output_index":0,"item_id":"fc_1","delta":"{\"file_path\":\"a.go\"}"}`))
	_, _ = hook([]byte("event: response.function_call_arguments.done"))
	_, _ = hook([]byte(`data: {"output_index":0,"item_id":"fc_1"}`))

	_, _ = hook([]byte("event: response.output_item.done"))
	flush, data := hook([]byte(`data: {"output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"toolu_1","name":"List","arguments":"{\"file_path\":\"a.go\"}"}}`))
	if flush || len(data) > 0 {
		t.Fatalf("标准工具调用顺序下 output_item.done 不应重复输出 tool_use，flush=%v data=%s", flush, string(data))
	}
}

func TestClaudeResponsesStreamTransformHookHandlesDoneAndFailedTerminalEvents(t *testing.T) {
	doneHook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)
	_, _ = doneHook([]byte("event: response.created"))
	_, _ = doneHook([]byte(`data: {"response":{"id":"resp_1","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":0}}}`))
	_, _ = doneHook([]byte("event: response.done"))
	flush, doneData := doneHook([]byte(`data: {"response":{"status":"completed","usage":{"input_tokens":1,"output_tokens":2}}}`))
	if !flush || !strings.Contains(string(doneData), `"type":"message_stop"`) {
		t.Fatalf("response.done 应完成 Claude message，flush=%v data=%s", flush, string(doneData))
	}

	failedHook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)
	_, _ = failedHook([]byte("event: response.created"))
	_, _ = failedHook([]byte(`data: {"response":{"id":"resp_2","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":0}}}`))
	_, _ = failedHook([]byte("event: response.failed"))
	flush, failedData := failedHook([]byte(`data: {"response":{"status":"failed","error":{"message":"boom"}}}`))
	if !flush || !strings.Contains(string(failedData), `"type":"message_stop"`) {
		t.Fatalf("response.failed 也应向 Claude 侧收尾，flush=%v data=%s", flush, string(failedData))
	}
}

func TestClaudeResponsesStreamTransformHookConvertsReasoningSummaryText(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)

	_, _ = hook([]byte("event: response.created"))
	_, _ = hook([]byte(`data: {"response":{"id":"resp_1","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":0}}}`))

	_, _ = hook([]byte("event: response.reasoning_summary_text.delta"))
	flush, deltaData := hook([]byte(`data: {"item_id":"rs_1","content_index":0,"delta":"thinking"}`))
	if !flush || !strings.Contains(string(deltaData), `"type":"thinking_delta"`) {
		t.Fatalf("reasoning_summary_text.delta 应转为 thinking_delta，flush=%v data=%s", flush, string(deltaData))
	}

	_, _ = hook([]byte("event: response.reasoning_summary_text.done"))
	flush, doneData := hook([]byte(`data: {"item_id":"rs_1","content_index":0}`))
	if !flush || !strings.Contains(string(doneData), `"type":"content_block_stop"`) {
		t.Fatalf("reasoning_summary_text.done 应关闭 thinking block，flush=%v data=%s", flush, string(doneData))
	}
}

func TestClaudeResponsesStreamTransformHookCombinesMultilineDataPayload(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)

	if flush, data := hook([]byte("data: {\"type\":\"response.created\",")); flush || len(data) != 0 {
		t.Fatalf("多行 data 第一段不应直接输出，flush=%v data=%q", flush, string(data))
	}
	flush, data := hook([]byte("data: \"response\":{\"id\":\"resp_1\",\"model\":\"gpt-5.4\"}}"))
	if !flush {
		t.Fatalf("拼出合法 JSON 后应立即触发多行 data 合并输出")
	}
	got := string(data)
	if !strings.Contains(got, "event: message_start\n") {
		t.Fatalf("多行 data 未转换为 message_start: %s", got)
	}
	if !strings.Contains(got, `"id":"resp_1"`) {
		t.Fatalf("多行 data 未保留 response id: %s", got)
	}
}

func TestClaudeResponsesStreamTransformHookCombinesCompactMultilineDataPayload(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)

	if flush, data := hook([]byte(`data: {"type":"response.output_text.delta","item_id":"msg_1","content_index":0,"delta":"hel`)); flush || len(data) != 0 {
		t.Fatalf("字符串中间拆分的第一段不应直接输出，flush=%v data=%q", flush, string(data))
	}
	flush, data := hook([]byte(`data: lo"}`))
	if !flush {
		t.Fatalf("拼出合法 JSON 后应立即触发紧凑合并输出")
	}
	got := string(data)
	if !strings.Contains(got, `"type":"text_delta"`) || !strings.Contains(got, `"text":"hello"`) {
		t.Fatalf("紧凑合并未正确保留字符串内容: %s", got)
	}
}

func TestClaudeResponsesStreamTransformHookDoesNotDuplicateDoneArgumentsAfterDelta(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)

	_, _ = hook([]byte("event: response.created"))
	_, _ = hook([]byte(`data: {"response":{"id":"resp_1","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":0}}}`))
	_, _ = hook([]byte("event: response.output_item.added"))
	_, _ = hook([]byte(`data: {"output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"toolu_1","name":"List"}}`))
	_, _ = hook([]byte("event: response.function_call_arguments.delta"))
	_, _ = hook([]byte(`data: {"output_index":0,"item_id":"fc_1","delta":"{\"file_path\":\"delta.go\"}"}`))

	_, _ = hook([]byte("event: response.function_call_arguments.done"))
	flush, doneData := hook([]byte(`data: {"output_index":0,"item_id":"fc_1","item":{"type":"function_call","arguments":"{\"file_path\":\"delta.go\"}"}}`))
	if !flush {
		t.Fatalf("done after delta 应至少输出 content_block_stop")
	}
	got := string(doneData)
	if strings.Contains(got, "partial_json") {
		t.Fatalf("已收到 delta 后不应重复输出 done.arguments: %s", got)
	}
	if !strings.Contains(got, `"type":"content_block_stop"`) {
		t.Fatalf("done after delta 应关闭 content block: %s", got)
	}
}

func TestClaudeResponsesStreamTransformHookSanitizesReadToolPagesAfterDelta(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)

	_, _ = hook([]byte("event: response.created"))
	_, _ = hook([]byte(`data: {"response":{"id":"resp_1","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":0}}}`))
	_, _ = hook([]byte("event: response.output_item.added"))
	_, _ = hook([]byte(`data: {"output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"toolu_1","name":"Read"}}`))
	_, _ = hook([]byte("event: response.function_call_arguments.delta"))
	flush, deltaData := hook([]byte(`data: {"output_index":0,"item_id":"fc_1","delta":"{\"file_path\":\"a.go\",\"pages\":\"\"}"}`))
	if flush || len(deltaData) > 0 {
		t.Fatalf("Read 参数 delta 应先缓冲等待清洗，flush=%v data=%s", flush, string(deltaData))
	}

	_, _ = hook([]byte("event: response.function_call_arguments.done"))
	flush, doneData := hook([]byte(`data: {"output_index":0,"item_id":"fc_1"}`))
	if !flush {
		t.Fatalf("Read done 应输出清洗后的参数")
	}
	got := string(doneData)
	if strings.Contains(got, `pages`) {
		t.Fatalf("Read 参数 pages 空字符串应被清理: %s", got)
	}
	if !strings.Contains(got, `file_path`) || !strings.Contains(got, `a.go`) {
		t.Fatalf("Read 清洗后应保留 file_path: %s", got)
	}
}

func TestClaudeChatStreamTransformHookSanitizesReadToolPagesAtFinish(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIChat, true)

	_, _ = hook([]byte(`data: {"id":"chatcmpl_1","model":"gpt-5.4","choices":[{"delta":{"tool_calls":[{"index":0,"id":"toolu_1","function":{"name":"Read","arguments":"{\"file_path\":\"a.go\",\"pages\":\"\"}"}}]}}]}`))
	flush, finishData := hook([]byte(`data: {"id":"chatcmpl_1","model":"gpt-5.4","choices":[{"finish_reason":"tool_calls","delta":{}}]}`))
	if !flush {
		t.Fatalf("Chat finish 应输出清洗后的 Read 参数和 message_delta")
	}
	got := string(finishData)
	if strings.Contains(got, `pages`) {
		t.Fatalf("Chat Read 参数 pages 空字符串应被清理: %s", got)
	}
	if !strings.Contains(got, `file_path`) || !strings.Contains(got, `a.go`) {
		t.Fatalf("Chat Read 清洗后应保留 file_path: %s", got)
	}
}

func TestResponseContainsExpectedFieldChecksJSONFieldExistence(t *testing.T) {
	if responseContainsExpectedField([]byte(`{"usage":{"output_tokens":5}}`), "output") {
		t.Fatalf("不应把 output_tokens 误判为 output 字段存在")
	}
	if !responseContainsExpectedField([]byte(`{"output":[]}`), "output") {
		t.Fatalf("应识别顶层 output 字段")
	}
	if responseContainsExpectedField([]byte(`{"error":{"message":"choices missing"}}`), "choices") {
		t.Fatalf("不应把错误消息里的 choices 文本误判为 choices 字段存在")
	}
	if !responseContainsExpectedField([]byte(`{"choices":[]}`), "choices") {
		t.Fatalf("应识别顶层 choices 字段")
	}
}

func TestClaudeChatStreamTransformHookUsesReasoningAliasFallback(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIChat, true)

	flush, data := hook([]byte(`data: {"id":"chatcmpl_1","model":"gpt-5.4","choices":[{"delta":{"reasoning_content":"fallback"}}]}`))
	if !flush {
		t.Fatalf("reasoning_content 应输出 thinking 事件")
	}
	if got := string(data); !strings.Contains(got, `"type":"thinking_delta"`) || !strings.Contains(got, `"thinking":"fallback"`) {
		t.Fatalf("reasoning_content 未正确映射: %s", got)
	}
}

func TestClaudeResponsesStreamTransformHookSupportsReasoningTextLifecycle(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)

	_, _ = hook([]byte("event: response.reasoning_text.delta"))
	flush, deltaData := hook([]byte(`data: {"output_index":0,"item_id":"rs_1","delta":"thinking"}`))
	if !flush {
		t.Fatalf("reasoning_text.delta 应输出 thinking 生命周期")
	}
	if got := string(deltaData); !strings.Contains(got, `"type":"thinking_delta"`) || !strings.Contains(got, `"thinking":"thinking"`) {
		t.Fatalf("reasoning_text.delta 映射异常: %s", got)
	}

	_, _ = hook([]byte("event: response.reasoning_text.done"))
	flush, doneData := hook([]byte(`data: {"output_index":0,"item_id":"rs_1"}`))
	if !flush || !strings.Contains(string(doneData), `"type":"content_block_stop"`) {
		t.Fatalf("reasoning_text.done 应关闭 thinking block: %s", doneData)
	}
}

func TestClaudeResponsesStreamTransformHookMapsWebSearchOnce(t *testing.T) {
	hook := newClaudeResponseTransformHook(claudeAPIFormatOpenAIResponse, true)
	payload := `data: {"output_index":1,"item":{"id":"ws_1","type":"web_search_call","status":"completed","action":{"query":"golang"}}}`

	_, _ = hook([]byte("event: response.output_item.done"))
	flush, first := hook([]byte(payload))
	if !flush {
		t.Fatalf("web_search_call done 应输出配对内容块")
	}
	got := string(first)
	if strings.Count(got, `"type":"server_tool_use"`) != 1 || strings.Count(got, `"type":"web_search_tool_result"`) != 1 {
		t.Fatalf("Web Search 配对事件异常: %s", got)
	}
	if !strings.Contains(got, `"id":"srvtoolu_ws_1"`) || !strings.Contains(got, `"tool_use_id":"srvtoolu_ws_1"`) {
		t.Fatalf("Web Search 配对 ID 异常: %s", got)
	}

	_, _ = hook([]byte("event: response.output_item.done"))
	flush, duplicate := hook([]byte(payload))
	if flush || len(duplicate) != 0 {
		t.Fatalf("重复 web_search_call done 不应再次输出，flush=%v data=%s", flush, duplicate)
	}
}
