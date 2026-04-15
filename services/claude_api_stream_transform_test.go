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
