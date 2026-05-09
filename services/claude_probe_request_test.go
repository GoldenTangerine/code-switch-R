package services

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestBuildClaudeProbeRequest(t *testing.T) {
	tests := []struct {
		name               string
		provider           Provider
		endpoint           string
		model              string
		wantSuccessField   string
		wantEndpointFormat string
		assertBody         func(t *testing.T, body []byte)
	}{
		{
			name: "anthropic",
			provider: Provider{
				ID:        1,
				Name:      "Anthropic Provider",
				APIFormat: claudeAPIFormatAnthropic,
			},
			endpoint:         "/v1/messages",
			model:            "claude-haiku-4-5-20251001",
			wantSuccessField: "content",
			assertBody: func(t *testing.T, body []byte) {
				t.Helper()
				if got := gjson.GetBytes(body, "model").String(); got != "claude-haiku-4-5-20251001" {
					t.Fatalf("model = %q, 期望 %q", got, "claude-haiku-4-5-20251001")
				}
				if got := gjson.GetBytes(body, "max_tokens").Int(); got != 1 {
					t.Fatalf("max_tokens = %d, 期望 1", got)
				}
				if got := gjson.GetBytes(body, "messages.0.role").String(); got != "user" {
					t.Fatalf("messages.0.role = %q, 期望 user", got)
				}
				if got := gjson.GetBytes(body, "messages.0.content").String(); got != "hi" {
					t.Fatalf("messages.0.content = %q, 期望 hi", got)
				}
				if gjson.GetBytes(body, "input").Exists() {
					t.Fatalf("Anthropic 请求不应包含 input")
				}
			},
		},
		{
			name: "openai_chat",
			provider: Provider{
				ID:        2,
				Name:      "Chat Provider",
				APIFormat: claudeAPIFormatOpenAIChat,
			},
			endpoint:         "/v1/chat/completions",
			model:            "gpt-4.1",
			wantSuccessField: "choices",
			assertBody: func(t *testing.T, body []byte) {
				t.Helper()
				if got := gjson.GetBytes(body, "model").String(); got != "gpt-4.1" {
					t.Fatalf("model = %q, 期望 %q", got, "gpt-4.1")
				}
				if got := gjson.GetBytes(body, "max_tokens").Int(); got != 1 {
					t.Fatalf("max_tokens = %d, 期望 1", got)
				}
				if got := gjson.GetBytes(body, "messages.0.role").String(); got != "user" {
					t.Fatalf("messages.0.role = %q, 期望 user", got)
				}
				if got := gjson.GetBytes(body, "messages.0.content").String(); got != "hi" {
					t.Fatalf("messages.0.content = %q, 期望 hi", got)
				}
				if gjson.GetBytes(body, "input").Exists() {
					t.Fatalf("OpenAI Chat 请求不应包含 input")
				}
			},
		},
		{
			name: "openai_responses",
			provider: Provider{
				ID:        42,
				Name:      "Responses Provider",
				APIFormat: claudeAPIFormatOpenAIResponse,
			},
			endpoint:         "/responses",
			model:            "gpt-5.4",
			wantSuccessField: "output",
			assertBody: func(t *testing.T, body []byte) {
				t.Helper()
				if got := gjson.GetBytes(body, "model").String(); got != "gpt-5.4" {
					t.Fatalf("model = %q, 期望 %q", got, "gpt-5.4")
				}
				if got := gjson.GetBytes(body, "max_output_tokens").Int(); got != 16 {
					t.Fatalf("max_output_tokens = %d, 期望 16", got)
				}
				if got := gjson.GetBytes(body, "input.0.role").String(); got != "user" {
					t.Fatalf("input.0.role = %q, 期望 user", got)
				}
				if got := gjson.GetBytes(body, "input.0.content.0.type").String(); got != "input_text" {
					t.Fatalf("input.0.content.0.type = %q, 期望 input_text", got)
				}
				if got := gjson.GetBytes(body, "input.0.content.0.text").String(); got != "hi" {
					t.Fatalf("input.0.content.0.text = %q, 期望 hi", got)
				}
				gotCacheKey := gjson.GetBytes(body, "prompt_cache_key").String()
				if gotCacheKey == "" {
					t.Fatalf("prompt_cache_key 不应为空")
				}
				if gotCacheKey == "42" {
					t.Fatalf("prompt_cache_key 不应复用 provider ID，实际=%q", gotCacheKey)
				}
				if gjson.GetBytes(body, "messages").Exists() {
					t.Fatalf("Responses 请求不应包含 messages")
				}
				if gjson.GetBytes(body, "store").Exists() {
					t.Fatalf("Responses 探测请求默认不应固定 store=false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := buildClaudeProbeRequest(&tt.provider, tt.endpoint, tt.model)
			if err != nil {
				t.Fatalf("buildClaudeProbeRequest 失败: %v", err)
			}
			if spec.SuccessContains != tt.wantSuccessField {
				t.Fatalf("SuccessContains = %q, 期望 %q", spec.SuccessContains, tt.wantSuccessField)
			}
			tt.assertBody(t, spec.Body)
		})
	}
}

func TestBuildProviderRequestPlanForClaudeAPIFormats(t *testing.T) {
	tests := []struct {
		name             string
		provider         Provider
		wantEndpoint     string
		assertBodyFields func(t *testing.T, body []byte)
	}{
		{
			name: "anthropic_keeps_messages_endpoint",
			provider: Provider{
				Name:      "Anthropic",
				APIFormat: claudeAPIFormatAnthropic,
			},
			wantEndpoint: "/v1/messages",
			assertBodyFields: func(t *testing.T, body []byte) {
				t.Helper()
				if !gjson.GetBytes(body, "messages").Exists() {
					t.Fatalf("Anthropic body 应包含 messages")
				}
			},
		},
		{
			name: "openai_chat_switches_endpoint",
			provider: Provider{
				Name:      "Chat",
				APIFormat: claudeAPIFormatOpenAIChat,
			},
			wantEndpoint: "/v1/chat/completions",
			assertBodyFields: func(t *testing.T, body []byte) {
				t.Helper()
				if !gjson.GetBytes(body, "messages").Exists() {
					t.Fatalf("OpenAI Chat body 应包含 messages")
				}
				if gjson.GetBytes(body, "input").Exists() {
					t.Fatalf("OpenAI Chat body 不应包含 input")
				}
			},
		},
		{
			name: "openai_responses_switches_endpoint",
			provider: Provider{
				ID:        7,
				Name:      "Responses",
				APIFormat: claudeAPIFormatOpenAIResponse,
			},
			wantEndpoint: "/responses",
			assertBodyFields: func(t *testing.T, body []byte) {
				t.Helper()
				if !gjson.GetBytes(body, "input").Exists() {
					t.Fatalf("Responses body 应包含 input")
				}
				if gjson.GetBytes(body, "messages").Exists() {
					t.Fatalf("Responses body 不应包含 messages")
				}
				if gjson.GetBytes(body, "store").Exists() {
					t.Fatalf("Responses body 默认不应固定 store=false")
				}
			},
		},
	}

	bodyBytes := []byte(`{"model":"claude-sonnet-4","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := buildProviderRequestPlan(tt.provider, bodyBytes, "/v1/messages", "claude-sonnet-4")
			if err != nil {
				t.Fatalf("buildProviderRequestPlan 失败: %v", err)
			}
			if plan.EffectiveEndpoint != tt.wantEndpoint {
				t.Fatalf("EffectiveEndpoint = %q, 期望 %q", plan.EffectiveEndpoint, tt.wantEndpoint)
			}
			tt.assertBodyFields(t, plan.BodyBytes)
		})
	}
}

func TestResolveClaudeProbeAPIFormatPrefersEndpointShape(t *testing.T) {
	provider := Provider{APIFormat: claudeAPIFormatAnthropic}

	if got := resolveClaudeProbeAPIFormat(&provider, "/responses"); got != claudeAPIFormatOpenAIResponse {
		t.Fatalf("endpoint=/responses 时 apiFormat = %q, 期望 %q", got, claudeAPIFormatOpenAIResponse)
	}
	if got := resolveClaudeProbeAPIFormat(&provider, "/v1/chat/completions"); got != claudeAPIFormatOpenAIChat {
		t.Fatalf("endpoint=/v1/chat/completions 时 apiFormat = %q, 期望 %q", got, claudeAPIFormatOpenAIChat)
	}
	if got := resolveClaudeProbeAPIFormat(&provider, "/v1/messages"); got != claudeAPIFormatAnthropic {
		t.Fatalf("endpoint=/v1/messages 时 apiFormat = %q, 期望 %q", got, claudeAPIFormatAnthropic)
	}
}
