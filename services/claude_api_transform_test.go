package services

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestAnthropicToResponsesRequestPreservesClaudeCodeShape(t *testing.T) {
	body, err := decodeJSONMap([]byte(`{
		"model":"gpt-5.4",
		"system":[
			{"type":"text","text":"x-anthropic-billing-header: ignore"},
			{"type":"text","text":"Be precise","cache_control":{"type":"ephemeral"}}
		],
		"messages":[
			{"role":"user","content":[{"type":"text","text":"Use tool"}]},
			{"role":"assistant","content":[
				{"type":"text","text":"Calling"},
				{"type":"tool_use","id":"toolu_123","name":"Read","input":{"file_path":"a.go"}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_123","content":[
					{"type":"text","text":"ok"},
					{"type":"image","source":{"media_type":"image/png","data":"abc"}}
				]}
			]}
		],
		"tools":[{"name":"Read","description":"read file","input_schema":{"type":"object"}}],
		"tool_choice":{"type":"tool","name":"Read"}
	}`))
	if err != nil {
		t.Fatalf("decodeJSONMap 失败: %v", err)
	}

	out, err := anthropicToResponsesRequest(body, "")
	if err != nil {
		t.Fatalf("anthropicToResponsesRequest 失败: %v", err)
	}
	data, err := encodeJSONMap(out)
	if err != nil {
		t.Fatalf("encodeJSONMap 失败: %v", err)
	}

	if got := gjson.GetBytes(data, "input.0.type").String(); got != "message" {
		t.Fatalf("input.0.type = %q, 期望 message", got)
	}
	if got := gjson.GetBytes(data, "input.0.role").String(); got != "developer" {
		t.Fatalf("system 应转为 developer message，got=%q", got)
	}
	if got := gjson.GetBytes(data, "input.0.content.0.text").String(); got != "Be precise" {
		t.Fatalf("developer text = %q", got)
	}
	if gjson.GetBytes(data, "instructions").Exists() {
		t.Fatalf("Responses 兼容路径不应把 system 放入 instructions: %s", data)
	}
	if strings.Contains(string(data), "cache_control") {
		t.Fatalf("Responses 出站请求不应透传 Anthropic cache_control: %s", data)
	}
	if gjson.GetBytes(data, "store").Exists() {
		t.Fatalf("Responses 兼容路径默认不应固定 store=false: %s", data)
	}
	if got := gjson.GetBytes(data, "input.3.call_id").String(); got != "toolu_123" {
		t.Fatalf("function_call.call_id = %q, 期望保留 tool_use id", got)
	}
	if got := gjson.GetBytes(data, "input.4.call_id").String(); got != "toolu_123" {
		t.Fatalf("function_call_output.call_id = %q", got)
	}
	if got := gjson.GetBytes(data, "input.4.output").String(); got != "ok" {
		t.Fatalf("function_call_output.output = %q", got)
	}
	if got := gjson.GetBytes(data, "input.5.content.0.image_url").String(); got != "data:image/png;base64,abc" {
		t.Fatalf("tool_result 图片未拆分为 user image input，got=%q", got)
	}
	if !gjson.GetBytes(data, "tools.0.parameters.properties").Exists() {
		t.Fatalf("tool parameters 应补齐 properties: %s", data)
	}
	if got := gjson.GetBytes(data, "tools.0.strict").Bool(); got {
		t.Fatalf("tool strict 应为 false")
	}
	if got := gjson.GetBytes(data, "tool_choice.type").String(); got != "function" {
		t.Fatalf("tool_choice.type = %q", got)
	}
	key := gjson.GetBytes(data, "prompt_cache_key").String()
	if !strings.HasPrefix(key, "anthropic-cache-") {
		t.Fatalf("prompt_cache_key = %q，期望基于 cache_control 派生", key)
	}
}

func TestAnthropicToResponsesRequestDerivesIsolatedPromptCacheKey(t *testing.T) {
	base := map[string]interface{}{
		"model":  "gpt-5.4",
		"system": "You are coding",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "first"},
		},
	}
	alt := map[string]interface{}{
		"model":  "gpt-5.4",
		"system": "You are coding",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "different"},
		},
	}

	first, err := anthropicToResponsesRequest(base, "")
	if err != nil {
		t.Fatalf("first transform failed: %v", err)
	}
	second, err := anthropicToResponsesRequest(base, "")
	if err != nil {
		t.Fatalf("second transform failed: %v", err)
	}
	third, err := anthropicToResponsesRequest(alt, "")
	if err != nil {
		t.Fatalf("third transform failed: %v", err)
	}

	if first["prompt_cache_key"] != second["prompt_cache_key"] {
		t.Fatalf("同一会话锚点应稳定: %v vs %v", first["prompt_cache_key"], second["prompt_cache_key"])
	}
	if first["prompt_cache_key"] == third["prompt_cache_key"] {
		t.Fatalf("不同首轮用户内容不应共享 prompt_cache_key: %v", first["prompt_cache_key"])
	}
}

func TestAnthropicToResponsesRequestDefaultsPromptCacheKeyToMetadataSession(t *testing.T) {
	firstSession := map[string]interface{}{
		"model":    "gpt-5.4",
		"system":   "You are coding",
		"metadata": map[string]interface{}{"user_id": "session-a"},
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "same prompt"},
		},
	}
	secondSession := map[string]interface{}{
		"model":    "gpt-5.4",
		"system":   "You are coding",
		"metadata": map[string]interface{}{"user_id": "session-b"},
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "same prompt"},
		},
	}

	first, err := anthropicToResponsesRequest(firstSession, "")
	if err != nil {
		t.Fatalf("first transform failed: %v", err)
	}
	second, err := anthropicToResponsesRequest(secondSession, "")
	if err != nil {
		t.Fatalf("second transform failed: %v", err)
	}

	if first["prompt_cache_key"] == second["prompt_cache_key"] {
		t.Fatalf("默认策略应按 metadata session 隔离 prompt_cache_key: %v", first["prompt_cache_key"])
	}
	if !strings.HasPrefix(asString(first["prompt_cache_key"]), "anthropic-metadata-") {
		t.Fatalf("默认 prompt_cache_key 应由 metadata.user_id 派生: %v", first["prompt_cache_key"])
	}
}

func TestAnthropicToResponsesRequestMetadataPromptCacheKeyUsesParsedClaudeCodeSession(t *testing.T) {
	firstSession := map[string]interface{}{
		"model": "gpt-5.4",
		"metadata": map[string]interface{}{
			"user_id": `{"device_id":"device-a","account_uuid":"account-a","session_id":"session-a"}`,
		},
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "same prompt"},
		},
	}
	sameSessionDifferentJSON := map[string]interface{}{
		"model": "gpt-5.4",
		"metadata": map[string]interface{}{
			"user_id": `{
				"session_id": "session-a",
				"account_uuid": "account-a",
				"device_id": "device-a"
			}`,
		},
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "same prompt"},
		},
	}

	first, err := anthropicToResponsesRequest(firstSession, "")
	if err != nil {
		t.Fatalf("first transform failed: %v", err)
	}
	second, err := anthropicToResponsesRequest(sameSessionDifferentJSON, "")
	if err != nil {
		t.Fatalf("second transform failed: %v", err)
	}

	if first["prompt_cache_key"] != second["prompt_cache_key"] {
		t.Fatalf("同一 Claude Code metadata session 不应因 JSON 顺序/空白漂移: %v vs %v", first["prompt_cache_key"], second["prompt_cache_key"])
	}
}

func TestAnthropicToResponsesRequestCanSharePromptCacheKeyAcrossMetadataSessions(t *testing.T) {
	firstSession := map[string]interface{}{
		"model":  "gpt-5.4",
		"system": "You are coding",
		"openai_compat_prompt_cache_key_strategy": "shared",
		"metadata": map[string]interface{}{"user_id": "session-a"},
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "same prompt"},
		},
	}
	secondSession := map[string]interface{}{
		"model":  "gpt-5.4",
		"system": "You are coding",
		"openai_compat_prompt_cache_key_strategy": "shared",
		"metadata": map[string]interface{}{"user_id": "session-b"},
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "same prompt"},
		},
	}

	first, err := anthropicToResponsesRequest(firstSession, "")
	if err != nil {
		t.Fatalf("first transform failed: %v", err)
	}
	second, err := anthropicToResponsesRequest(secondSession, "")
	if err != nil {
		t.Fatalf("second transform failed: %v", err)
	}

	if first["prompt_cache_key"] != second["prompt_cache_key"] {
		t.Fatalf("shared 策略应按缓存内容共享 prompt_cache_key: %v vs %v", first["prompt_cache_key"], second["prompt_cache_key"])
	}
	if strings.HasPrefix(asString(first["prompt_cache_key"]), "anthropic-metadata-") {
		t.Fatalf("shared 策略不应由 metadata.user_id 派生: %v", first["prompt_cache_key"])
	}
}

func TestAnthropicToResponsesRequestCacheControlPromptCacheKeyIgnoresLaterUserTurns(t *testing.T) {
	base := map[string]interface{}{
		"model": "gpt-5.4",
		"system": []interface{}{
			map[string]interface{}{"type": "text", "text": "stable system", "cache_control": map[string]interface{}{"type": "ephemeral"}},
		},
		"messages": []interface{}{
			map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": "first user", "cache_control": map[string]interface{}{"type": "ephemeral"}},
				},
			},
			map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": "later user", "cache_control": map[string]interface{}{"type": "ephemeral"}},
				},
			},
		},
	}
	withoutLaterCacheControl := map[string]interface{}{
		"model":  base["model"],
		"system": base["system"],
		"messages": []interface{}{
			base["messages"].([]interface{})[0],
			map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": "different later user", "cache_control": map[string]interface{}{"type": "ephemeral"}},
				},
			},
		},
	}

	first, err := anthropicToResponsesRequest(base, "")
	if err != nil {
		t.Fatalf("first transform failed: %v", err)
	}
	second, err := anthropicToResponsesRequest(withoutLaterCacheControl, "")
	if err != nil {
		t.Fatalf("second transform failed: %v", err)
	}

	if first["prompt_cache_key"] != second["prompt_cache_key"] {
		t.Fatalf("后续 user cache_control 不应导致 prompt_cache_key 漂移: %v vs %v", first["prompt_cache_key"], second["prompt_cache_key"])
	}
}

func TestAnthropicToResponsesRequestPreservesExplicitPromptCacheKey(t *testing.T) {
	body := map[string]interface{}{
		"model":            "gpt-5.4",
		"prompt_cache_key": "explicit-session",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
		},
	}

	out, err := anthropicToResponsesRequest(body, "")
	if err != nil {
		t.Fatalf("anthropicToResponsesRequest failed: %v", err)
	}

	if got := out["prompt_cache_key"]; got != "explicit-session" {
		t.Fatalf("prompt_cache_key = %v，期望保留显式请求体值", got)
	}
}

func TestBuildAnthropicUsageFromResponsesSubtractsCachedInputTokens(t *testing.T) {
	usage := buildAnthropicUsageFromResponses(map[string]interface{}{
		"input_tokens":  float64(100),
		"output_tokens": float64(7),
		"input_tokens_details": map[string]interface{}{
			"cached_tokens": float64(80),
		},
	})

	if got := usage["input_tokens"]; got != int64(20) {
		t.Fatalf("input_tokens=%v，期望扣除 cached tokens 后为 20", got)
	}
	if got := usage["cache_read_input_tokens"]; got != int64(80) {
		t.Fatalf("cache_read_input_tokens=%v，期望 80", got)
	}
}

func TestBuildAnthropicUsageFromOpenAISubtractsCachedInputTokens(t *testing.T) {
	usage := buildAnthropicUsageFromOpenAI(map[string]interface{}{
		"prompt_tokens":     float64(100),
		"completion_tokens": float64(7),
		"prompt_tokens_details": map[string]interface{}{
			"cached_tokens": float64(80),
		},
	})

	if got := usage["input_tokens"]; got != int64(20) {
		t.Fatalf("input_tokens=%v，期望扣除 cached tokens 后为 20", got)
	}
	if got := usage["cache_read_input_tokens"]; got != int64(80) {
		t.Fatalf("cache_read_input_tokens=%v，期望 80", got)
	}
}

func TestResponsesToAnthropicResponseSanitizesReadToolPages(t *testing.T) {
	body := map[string]interface{}{
		"id":     "resp_1",
		"status": "completed",
		"model":  "gpt-5.4",
		"output": []interface{}{
			map[string]interface{}{
				"type":      "function_call",
				"call_id":   "toolu_1",
				"name":      "Read",
				"arguments": `{"file_path":"a.go","pages":""}`,
			},
		},
	}

	out, err := responsesToAnthropicResponse(body)
	if err != nil {
		t.Fatalf("responsesToAnthropicResponse failed: %v", err)
	}
	data, _ := json.Marshal(out)

	if gjson.GetBytes(data, "content.0.input.pages").Exists() {
		t.Fatalf("Read 工具 pages 空字符串应被清理: %s", data)
	}
	if got := gjson.GetBytes(data, "content.0.input.file_path").String(); got != "a.go" {
		t.Fatalf("file_path=%q，期望 a.go", got)
	}
}

func TestResponsesToAnthropicResponseRejectsFailedAndEmptyIncompleteStatus(t *testing.T) {
	tests := []struct {
		name string
		body map[string]interface{}
		want string
	}{
		{
			name: "failed",
			body: map[string]interface{}{
				"id":     "resp_failed",
				"status": "failed",
				"error":  map[string]interface{}{"message": "boom"},
				"output": []interface{}{},
			},
			want: "OpenAI Responses response failed: boom",
		},
		{
			name: "incomplete",
			body: map[string]interface{}{
				"id":                 "resp_incomplete",
				"status":             "incomplete",
				"incomplete_details": map[string]interface{}{"reason": "max_output_tokens"},
				"output":             []interface{}{},
			},
			want: "OpenAI Responses response incomplete: max_output_tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := responsesToAnthropicResponse(tt.body)
			if err == nil {
				t.Fatalf("期望 %s 状态返回错误", tt.name)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("错误=%q，期望包含 %q", err.Error(), tt.want)
			}
		})
	}
}

func TestResponsesToAnthropicResponseMapsIncompleteWithOutputToMaxTokens(t *testing.T) {
	body := map[string]interface{}{
		"id":                 "resp_incomplete",
		"status":             "incomplete",
		"model":              "gpt-5.4",
		"incomplete_details": map[string]interface{}{"reason": "max_output_tokens"},
		"output": []interface{}{
			map[string]interface{}{
				"type": "message",
				"content": []interface{}{
					map[string]interface{}{"type": "output_text", "text": "partial"},
				},
			},
		},
		"usage": map[string]interface{}{"input_tokens": float64(2), "output_tokens": float64(1)},
	}

	out, err := responsesToAnthropicResponse(body)
	if err != nil {
		t.Fatalf("带输出的 incomplete 应转换为 Claude max_tokens，而不是错误: %v", err)
	}
	data, _ := json.Marshal(out)
	if got := gjson.GetBytes(data, "stop_reason").String(); got != "max_tokens" {
		t.Fatalf("stop_reason=%q，期望 max_tokens，body=%s", got, data)
	}
	if got := gjson.GetBytes(data, "content.0.text").String(); got != "partial" {
		t.Fatalf("content.0.text=%q，期望 partial，body=%s", got, data)
	}
}

func TestAnthropicToResponsesRequestAddsReasoningContinuityFields(t *testing.T) {
	body := map[string]interface{}{
		"model":      "gpt-5.4",
		"max_tokens": float64(16),
		"store":      false,
		"output_config": map[string]interface{}{
			"effort": "high",
		},
		"include": []interface{}{"message.input_image.image_url"},
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hi"},
		},
	}

	out, err := anthropicToResponsesRequest(body, "")
	if err != nil {
		t.Fatalf("anthropicToResponsesRequest failed: %v", err)
	}
	data, _ := json.Marshal(out)

	if got := gjson.GetBytes(data, "text.verbosity").String(); got != "medium" {
		t.Fatalf("text.verbosity=%q，期望 medium，body=%s", got, data)
	}
	if got := gjson.GetBytes(data, "include.#(==\"reasoning.encrypted_content\")").String(); got != "reasoning.encrypted_content" {
		t.Fatalf("store=false 时应包含 encrypted reasoning include，body=%s", data)
	}
	if got := gjson.GetBytes(data, "include.#(==\"message.input_image.image_url\")").String(); got != "message.input_image.image_url" {
		t.Fatalf("应保留用户原有 include，body=%s", data)
	}
}

func TestAnthropicToResponsesRequestMapsZeroThinkingBudgetToMedium(t *testing.T) {
	body := map[string]interface{}{
		"model": "gpt-5.4",
		"thinking": map[string]interface{}{
			"type":          "enabled",
			"budget_tokens": float64(0),
		},
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hi"},
		},
	}

	out, err := anthropicToResponsesRequest(body, "")
	if err != nil {
		t.Fatalf("anthropicToResponsesRequest failed: %v", err)
	}
	data, _ := json.Marshal(out)

	if got := gjson.GetBytes(data, "reasoning.effort").String(); got != "medium" {
		t.Fatalf("thinking budget_tokens=0 应映射为 medium，got=%q body=%s", got, data)
	}
}

func TestAnthropicToResponsesRequestRespectsDisableParallelToolUse(t *testing.T) {
	body := map[string]interface{}{
		"model": "gpt-5.4",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hi"},
		},
		"tools": []interface{}{
			map[string]interface{}{"name": "Read", "input_schema": map[string]interface{}{"type": "object"}},
		},
		"tool_choice": map[string]interface{}{
			"type":                      "auto",
			"disable_parallel_tool_use": true,
		},
	}

	out, err := anthropicToResponsesRequest(body, "")
	if err != nil {
		t.Fatalf("anthropicToResponsesRequest failed: %v", err)
	}
	data, _ := json.Marshal(out)

	if got := gjson.GetBytes(data, "parallel_tool_calls").Bool(); got {
		t.Fatalf("disable_parallel_tool_use=true 时 parallel_tool_calls 应为 false，body=%s", data)
	}
}

func TestOpenAIToAnthropicResponseSanitizesReadToolPages(t *testing.T) {
	body := map[string]interface{}{
		"id":    "chatcmpl_1",
		"model": "gpt-5.4",
		"choices": []interface{}{
			map[string]interface{}{
				"finish_reason": "tool_calls",
				"message": map[string]interface{}{
					"tool_calls": []interface{}{
						map[string]interface{}{
							"id": "toolu_1",
							"function": map[string]interface{}{
								"name":      "Read",
								"arguments": `{"file_path":"a.go","pages":""}`,
							},
						},
					},
				},
			},
		},
	}

	out, err := openAIToAnthropicResponse(body)
	if err != nil {
		t.Fatalf("openAIToAnthropicResponse failed: %v", err)
	}
	data, _ := json.Marshal(out)

	if gjson.GetBytes(data, "content.0.input.pages").Exists() {
		t.Fatalf("OpenAI Chat Read 工具 pages 空字符串应被清理: %s", data)
	}
}

func TestOpenAICompatModelAliasesEnableReasoningAndCacheBranches(t *testing.T) {
	for _, model := range []string{"openai/gpt-5.4", "gpt5", "codex-mini-latest"} {
		if !supportsReasoningEffort(model) {
			t.Fatalf("supportsReasoningEffort(%q)=false，期望 true", model)
		}
		if !supportsOpenAICompatPromptCache(model) {
			t.Fatalf("supportsOpenAICompatPromptCache(%q)=false，期望 true", model)
		}
	}
}

func TestAnthropicToOpenAIRequestStripsAnthropicCacheControl(t *testing.T) {
	body, err := decodeJSONMap([]byte(`{
		"model":"gpt-5.4",
		"system":[{"type":"text","text":"system","cache_control":{"type":"ephemeral"}}],
		"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral"}}]}],
		"tools":[{"name":"Read","input_schema":{"type":"object"},"cache_control":{"type":"ephemeral"}}]
	}`))
	if err != nil {
		t.Fatalf("decodeJSONMap 失败: %v", err)
	}

	out, err := anthropicToOpenAIRequest(body)
	if err != nil {
		t.Fatalf("anthropicToOpenAIRequest 失败: %v", err)
	}
	data, err := encodeJSONMap(out)
	if err != nil {
		t.Fatalf("encodeJSONMap 失败: %v", err)
	}

	if strings.Contains(string(data), "cache_control") {
		t.Fatalf("OpenAI Chat 出站请求不应透传 Anthropic cache_control: %s", data)
	}
}

func TestAnthropicToOpenAIRequestStripsAnthropicBillingHeader(t *testing.T) {
	body, err := decodeJSONMap([]byte(`{
		"model":"gpt-5.4",
		"system":[
			{"type":"text","text":"x-anthropic-billing-header: ignore"},
			{"type":"text","text":"real system"}
		],
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("decodeJSONMap 失败: %v", err)
	}

	out, err := anthropicToOpenAIRequest(body)
	if err != nil {
		t.Fatalf("anthropicToOpenAIRequest 失败: %v", err)
	}
	data, err := encodeJSONMap(out)
	if err != nil {
		t.Fatalf("encodeJSONMap 失败: %v", err)
	}

	if strings.Contains(string(data), "x-anthropic-billing-header") {
		t.Fatalf("OpenAI Chat 出站请求不应透传 Anthropic billing header: %s", data)
	}
	if got := gjson.GetBytes(data, "messages.0.content").String(); got != "real system" {
		t.Fatalf("system content=%q，期望 real system，body=%s", got, data)
	}
}

func TestAnthropicToOpenAIRequestMapsToolChoice(t *testing.T) {
	tests := []struct {
		name       string
		choiceJSON string
		wantPath   string
		want       string
	}{
		{
			name:       "forced tool",
			choiceJSON: `{"type":"tool","name":"Read"}`,
			wantPath:   "tool_choice.function.name",
			want:       "Read",
		},
		{
			name:       "any",
			choiceJSON: `{"type":"any"}`,
			wantPath:   "tool_choice",
			want:       "required",
		},
		{
			name:       "auto",
			choiceJSON: `{"type":"auto"}`,
			wantPath:   "tool_choice",
			want:       "auto",
		},
		{
			name:       "none",
			choiceJSON: `{"type":"none"}`,
			wantPath:   "tool_choice",
			want:       "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := decodeJSONMap([]byte(`{
				"model":"gpt-5.4",
				"messages":[{"role":"user","content":"hi"}],
				"tools":[{"name":"Read","input_schema":{"type":"object"}}],
				"tool_choice":` + tt.choiceJSON + `
			}`))
			if err != nil {
				t.Fatalf("decodeJSONMap 失败: %v", err)
			}

			out, err := anthropicToOpenAIRequest(body)
			if err != nil {
				t.Fatalf("anthropicToOpenAIRequest 失败: %v", err)
			}
			data, err := encodeJSONMap(out)
			if err != nil {
				t.Fatalf("encodeJSONMap 失败: %v", err)
			}

			if got := gjson.GetBytes(data, tt.wantPath).String(); got != tt.want {
				t.Fatalf("%s = %q，期望 %q，完整请求=%s", tt.wantPath, got, tt.want, data)
			}
			if tt.name == "forced tool" {
				if got := gjson.GetBytes(data, "tool_choice.type").String(); got != "function" {
					t.Fatalf("强制工具调用应映射为 function，got=%q data=%s", got, data)
				}
			}
		})
	}
}
