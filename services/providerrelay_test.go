package services

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tidwall/gjson"
)

// ==================== ReplaceModelInRequestBody 测试 ====================

func TestReplaceModelInRequestBody(t *testing.T) {
	tests := []struct {
		name          string
		inputJSON     string
		newModel      string
		expectError   bool
		expectedModel string
	}{
		// 成功场景
		{
			name: "简单替换",
			inputJSON: `{
				"model": "claude-sonnet-4",
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
			newModel:      "anthropic/claude-sonnet-4",
			expectError:   false,
			expectedModel: "anthropic/claude-sonnet-4",
		},
		{
			name: "复杂嵌套JSON",
			inputJSON: `{
				"model": "claude-opus-4",
				"messages": [
					{
						"role": "user",
						"content": "Test"
					}
				],
				"temperature": 0.7,
				"max_tokens": 1000,
				"metadata": {
					"user_id": "12345"
				}
			}`,
			newModel:      "gpt-4",
			expectError:   false,
			expectedModel: "gpt-4",
		},
		{
			name: "模型名包含特殊字符",
			inputJSON: `{
				"model": "claude-sonnet-4",
				"messages": []
			}`,
			newModel:      "anthropic/claude-3.5-sonnet@20241022",
			expectError:   false,
			expectedModel: "anthropic/claude-3.5-sonnet@20241022",
		},

		// 错误场景
		{
			name: "缺少model字段",
			inputJSON: `{
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
			newModel:    "any-model",
			expectError: true,
		},
		{
			name: "空JSON",
			inputJSON: `{
			}`,
			newModel:    "any-model",
			expectError: true,
		},
		{
			name:        "无效JSON",
			inputJSON:   `{invalid json}`,
			newModel:    "any-model",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes := []byte(tt.inputJSON)
			result, err := ReplaceModelInRequestBody(bodyBytes, tt.newModel)

			// 检查错误预期
			if tt.expectError && err == nil {
				t.Errorf("期望返回错误，但没有错误")
			}
			if !tt.expectError && err != nil {
				t.Errorf("不期望错误，但返回了: %v", err)
			}

			// 如果不期望错误，验证结果
			if !tt.expectError {
				// 验证返回的JSON是否有效
				if !json.Valid(result) {
					t.Errorf("返回的JSON无效")
				}

				// 验证模型名是否正确替换
				actualModel := gjson.GetBytes(result, "model").String()
				if actualModel != tt.expectedModel {
					t.Errorf("替换后的模型名 = %q, 期望 %q", actualModel, tt.expectedModel)
				}

				// 验证其他字段未被修改
				if gjson.GetBytes(bodyBytes, "messages").Exists() {
					originalMessages := gjson.GetBytes(bodyBytes, "messages").Raw
					resultMessages := gjson.GetBytes(result, "messages").Raw
					if originalMessages != resultMessages {
						t.Errorf("messages 字段被意外修改")
					}
				}
			}
		})
	}
}

func TestApplyRequestBodyOverrides(t *testing.T) {
	tests := []struct {
		name            string
		inputJSON       string
		overrides       map[string]interface{}
		expectError     bool
		expectedModel   string
		expectedTemp    float64
		expectedTraceID string
		expectedRegion  string
	}{
		{
			name: "覆盖现有字段并新增字段",
			inputJSON: `{
				"model": "claude-sonnet-4",
				"temperature": 0.7,
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
			overrides: map[string]interface{}{
				"temperature": 0.2,
				"metadata": map[string]interface{}{
					"trace_id": "trace-001",
					"region":   "cn-east",
				},
			},
			expectedModel:   "claude-sonnet-4",
			expectedTemp:    0.2,
			expectedTraceID: "trace-001",
			expectedRegion:  "cn-east",
		},
		{
			name:      "空请求体时自动补空对象",
			inputJSON: ``,
			overrides: map[string]interface{}{
				"model":      "forced-model",
				"max_tokens": 256,
			},
			expectedModel: "forced-model",
		},
		{
			name:        "非法JSON返回错误",
			inputJSON:   `{invalid json}`,
			overrides:   map[string]interface{}{"temperature": 0.1},
			expectError: true,
		},
		{
			name:        "根节点不是对象返回错误",
			inputJSON:   `[]`,
			overrides:   map[string]interface{}{"temperature": 0.1},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ApplyRequestBodyOverrides([]byte(tt.inputJSON), tt.overrides)
			if tt.expectError {
				if err == nil {
					t.Fatalf("期望返回错误，但实际没有")
				}
				return
			}
			if err != nil {
				t.Fatalf("不期望错误，但返回了: %v", err)
			}
			if !json.Valid(result) {
				t.Fatalf("返回的 JSON 无效: %s", string(result))
			}

			if tt.expectedModel != "" && gjson.GetBytes(result, "model").String() != tt.expectedModel {
				t.Fatalf("model = %q, 期望 %q", gjson.GetBytes(result, "model").String(), tt.expectedModel)
			}
			if tt.expectedTemp != 0 && gjson.GetBytes(result, "temperature").Float() != tt.expectedTemp {
				t.Fatalf("temperature = %v, 期望 %v", gjson.GetBytes(result, "temperature").Float(), tt.expectedTemp)
			}
			if tt.expectedTraceID != "" && gjson.GetBytes(result, "metadata.trace_id").String() != tt.expectedTraceID {
				t.Fatalf("metadata.trace_id = %q, 期望 %q", gjson.GetBytes(result, "metadata.trace_id").String(), tt.expectedTraceID)
			}
			if tt.expectedRegion != "" && gjson.GetBytes(result, "metadata.region").String() != tt.expectedRegion {
				t.Fatalf("metadata.region = %q, 期望 %q", gjson.GetBytes(result, "metadata.region").String(), tt.expectedRegion)
			}
		})
	}
}

func TestApplyProviderAnthropicCacheTTLOverride_OnlyUpdatesExistingEphemeralCacheControl(t *testing.T) {
	provider := Provider{
		APIFormat:         "anthropic",
		AnthropicCacheTTL: "1h",
	}
	body := []byte(`{"alpha":1,"cache_control":{"type":"ephemeral"},"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"5m"}},{"type":"text","text":"plain"}],"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral"}},{"type":"text","text":"non","cache_control":{"type":"persistent","ttl":"5m"}}]}],"tools":[{"name":"a","input_schema":{},"cache_control":{"type":"ephemeral"}}],"omega":2}`)

	result := applyProviderAnthropicCacheTTLOverride(provider, "/v1/messages", body)

	if got := gjson.GetBytes(result, "cache_control.ttl").String(); got != "1h" {
		t.Fatalf("top-level cache_control.ttl = %q, 期望 1h", got)
	}
	if got := gjson.GetBytes(result, "system.0.cache_control.ttl").String(); got != "1h" {
		t.Fatalf("system cache_control.ttl = %q, 期望 1h", got)
	}
	if gjson.GetBytes(result, "system.1.cache_control").Exists() {
		t.Fatalf("不应给没有 cache_control 的 system block 新增缓存断点: %s", result)
	}
	if got := gjson.GetBytes(result, "messages.0.content.0.cache_control.ttl").String(); got != "1h" {
		t.Fatalf("message cache_control.ttl = %q, 期望 1h", got)
	}
	if got := gjson.GetBytes(result, "messages.0.content.1.cache_control.ttl").String(); got != "5m" {
		t.Fatalf("非 ephemeral cache_control.ttl = %q, 期望保留 5m", got)
	}
	if got := gjson.GetBytes(result, "tools.0.cache_control.ttl").String(); got != "1h" {
		t.Fatalf("tool cache_control.ttl = %q, 期望 1h", got)
	}
}

func TestApplyProviderAnthropicCacheTTLOverride_RespectsScopeAndDefault(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]}`)

	emptyResult := applyProviderAnthropicCacheTTLOverride(Provider{
		APIFormat: "anthropic",
	}, "/v1/messages", body)
	if string(emptyResult) != string(body) {
		t.Fatalf("空 TTL 不应修改请求体，got=%s", emptyResult)
	}

	openAIResult := applyProviderAnthropicCacheTTLOverride(Provider{
		APIFormat:         "openai_chat",
		AnthropicCacheTTL: "5m",
	}, "/v1/messages", body)
	if string(openAIResult) != string(body) {
		t.Fatalf("OpenAI 兼容格式不应修改 Anthropic cache_control，got=%s", openAIResult)
	}

	otherEndpointResult := applyProviderAnthropicCacheTTLOverride(Provider{
		APIFormat:         "anthropic",
		AnthropicCacheTTL: "5m",
	}, "/responses", body)
	if string(otherEndpointResult) != string(body) {
		t.Fatalf("非 /v1/messages 端点不应修改请求体，got=%s", otherEndpointResult)
	}

	forced5m := applyProviderAnthropicCacheTTLOverride(Provider{
		APIFormat:         "anthropic",
		AnthropicCacheTTL: "5m",
	}, "/v1/messages", body)
	if got := gjson.GetBytes(forced5m, "messages.0.content.0.cache_control.ttl").String(); got != "5m" {
		t.Fatalf("cache_control.ttl = %q, 期望 5m", got)
	}
}

func TestRequestBodyOverridesOverrideMappedModel(t *testing.T) {
	provider := Provider{
		Name: "Override Provider",
		ModelMapping: map[string]string{
			"claude-sonnet-4": "anthropic/claude-sonnet-4",
		},
		RequestBodyOverrides: map[string]interface{}{
			"model":       "forced-provider-model",
			"temperature": 0.15,
			"metadata": map[string]interface{}{
				"route": "vendor-a",
			},
		},
	}

	bodyBytes := []byte(`{
		"model": "claude-sonnet-4",
		"messages": [{"role": "user", "content": "hi"}]
	}`)
	requestedModel := "claude-sonnet-4"

	effectiveModel := provider.GetEffectiveModel(requestedModel)
	modifiedBody, err := ReplaceModelInRequestBody(bodyBytes, effectiveModel)
	if err != nil {
		t.Fatalf("ReplaceModelInRequestBody 失败: %v", err)
	}

	modifiedBody, err = ApplyRequestBodyOverrides(modifiedBody, provider.RequestBodyOverrides)
	if err != nil {
		t.Fatalf("ApplyRequestBodyOverrides 失败: %v", err)
	}

	finalModel := resolveModelFromRequestBody(modifiedBody, effectiveModel)
	if finalModel != "forced-provider-model" {
		t.Fatalf("最终模型 = %q, 期望 %q", finalModel, "forced-provider-model")
	}
	if gjson.GetBytes(modifiedBody, "temperature").Float() != 0.15 {
		t.Fatalf("temperature = %v, 期望 0.15", gjson.GetBytes(modifiedBody, "temperature").Float())
	}
	if gjson.GetBytes(modifiedBody, "metadata.route").String() != "vendor-a" {
		t.Fatalf("metadata.route = %q, 期望 %q", gjson.GetBytes(modifiedBody, "metadata.route").String(), "vendor-a")
	}
}

func TestProviderResolvedModelSupport(t *testing.T) {
	tests := []struct {
		name           string
		provider       Provider
		requestedModel string
		effectiveModel string
		expectAllowed  bool
	}{
		{
			name: "最终模型命中 native whitelist",
			provider: Provider{
				SupportedModels: map[string]bool{
					"anthropic/claude-sonnet-4": true,
				},
			},
			requestedModel: "claude-sonnet-4",
			effectiveModel: "anthropic/claude-sonnet-4",
			expectAllowed:  true,
		},
		{
			name: "最终模型不在 native whitelist 中应拦截",
			provider: Provider{
				SupportedModels: map[string]bool{
					"anthropic/claude-sonnet-4": true,
				},
			},
			requestedModel: "claude-sonnet-4",
			effectiveModel: "openai/gpt-4.1",
			expectAllowed:  false,
		},
		{
			name: "未配置 native whitelist 时允许最终模型变化",
			provider: Provider{
				ModelMapping: map[string]string{
					"claude-*": "anthropic/claude-*",
				},
			},
			requestedModel: "claude-sonnet-4",
			effectiveModel: "forced-provider-model",
			expectAllowed:  true,
		},
		{
			name: "未改模型时沿用原有支持判断",
			provider: Provider{
				ModelMapping: map[string]string{
					"claude-sonnet-4": "anthropic/claude-sonnet-4",
				},
			},
			requestedModel: "claude-sonnet-4",
			effectiveModel: "claude-sonnet-4",
			expectAllowed:  true,
		},
		{
			name: "未改模型且映射未命中时默认拦截",
			provider: Provider{
				ModelMapping: map[string]string{
					"claude-*": "anthropic/claude-*",
				},
			},
			requestedModel: "gpt-4",
			effectiveModel: "gpt-4",
			expectAllowed:  false,
		},
		{
			name: "未改模型且映射未命中时允许原样转发",
			provider: Provider{
				ModelMapping: map[string]string{
					"claude-*": "anthropic/claude-*",
				},
				ModelMappingMissPolicy: ModelMappingMissPolicyPassthrough,
			},
			requestedModel: "gpt-4",
			effectiveModel: "gpt-4",
			expectAllowed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed := tt.provider.IsResolvedModelSupported(tt.requestedModel, tt.effectiveModel)
			if allowed != tt.expectAllowed {
				t.Fatalf("IsResolvedModelSupported(%q, %q) = %v, 期望 %v",
					tt.requestedModel, tt.effectiveModel, allowed, tt.expectAllowed)
			}
		})
	}
}

func TestBuildProviderRequestPlanUsesOverrideModelForFiltering(t *testing.T) {
	provider := Provider{
		Name: "Filterable Provider",
		SupportedModels: map[string]bool{
			"forced-provider-model": true,
		},
		RequestBodyOverrides: map[string]interface{}{
			"model": "forced-provider-model",
		},
	}

	bodyBytes := []byte(`{
		"model": "claude-sonnet-4",
		"messages": [{"role": "user", "content": "hi"}]
	}`)

	plan, err := buildProviderRequestPlan(provider, bodyBytes, "/v1/messages", "claude-sonnet-4")
	if err != nil {
		t.Fatalf("buildProviderRequestPlan 失败: %v", err)
	}

	if plan.EffectiveModel != "forced-provider-model" {
		t.Fatalf("最终模型 = %q, 期望 %q", plan.EffectiveModel, "forced-provider-model")
	}

	if !provider.IsResolvedModelSupported("claude-sonnet-4", plan.EffectiveModel) {
		t.Fatalf("期望最终模型命中过滤校验，但实际被判定为不支持")
	}
}

// ==================== 端到端场景测试 ====================

func TestModelMappingEndToEnd(t *testing.T) {
	// 模拟真实场景：用户请求 claude-sonnet-4，需要映射到 OpenRouter 的格式
	provider := Provider{
		Name: "OpenRouter",
		SupportedModels: map[string]bool{
			"anthropic/claude-sonnet-4":   true,
			"anthropic/claude-opus-4":     true,
			"openai/gpt-4":                true,
			"google/gemini-pro":           true,
			"meta-llama/llama-3.1-405b":   true,
			"anthropic/claude-3.5-sonnet": true,
			"anthropic/claude-3.5-haiku":  true,
		},
		ModelMapping: map[string]string{
			"claude-*": "anthropic/claude-*",
			"gpt-*":    "openai/gpt-*",
			"gemini-*": "google/gemini-*",
			"llama-*":  "meta-llama/llama-*",
		},
	}

	scenarios := []struct {
		requestedModel string
		shouldSupport  bool
		effectiveModel string
	}{
		// 通配符映射场景
		{"claude-sonnet-4", true, "anthropic/claude-sonnet-4"},
		{"claude-opus-4", true, "anthropic/claude-opus-4"},
		{"claude-3.5-sonnet", true, "anthropic/claude-3.5-sonnet"},
		{"gpt-4", true, "openai/gpt-4"},
		{"gpt-4-turbo", true, "openai/gpt-4-turbo"},
		{"gemini-pro", true, "google/gemini-pro"},
		{"llama-3.1-405b", true, "meta-llama/llama-3.1-405b"},

		// 不支持的模型
		{"deepseek-v3", false, "deepseek-v3"},
		{"qwen-max", false, "qwen-max"},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.requestedModel, func(t *testing.T) {
			// 1. 检查是否支持
			supported := provider.IsModelSupported(scenario.requestedModel)
			if supported != scenario.shouldSupport {
				t.Errorf("IsModelSupported(%q) = %v, 期望 %v",
					scenario.requestedModel, supported, scenario.shouldSupport)
			}

			// 2. 获取有效模型名
			effectiveModel := provider.GetEffectiveModel(scenario.requestedModel)
			if effectiveModel != scenario.effectiveModel {
				t.Errorf("GetEffectiveModel(%q) = %q, 期望 %q",
					scenario.requestedModel, effectiveModel, scenario.effectiveModel)
			}

			// 3. 如果支持，测试请求体替换
			if scenario.shouldSupport {
				requestBody := `{"model": "` + scenario.requestedModel + `", "messages": []}`
				result, err := ReplaceModelInRequestBody([]byte(requestBody), effectiveModel)
				if err != nil {
					t.Fatalf("ReplaceModelInRequestBody 失败: %v", err)
				}

				actualModel := gjson.GetBytes(result, "model").String()
				if actualModel != scenario.effectiveModel {
					t.Errorf("请求体中的模型 = %q, 期望 %q", actualModel, scenario.effectiveModel)
				}
			}
		})
	}
}

// ==================== 配置验证集成测试 ====================

func TestProviderConfigValidation(t *testing.T) {
	// 场景 1：完美配置
	validProvider := Provider{
		Name: "ValidProvider",
		SupportedModels: map[string]bool{
			"anthropic/claude-sonnet-4": true,
			"anthropic/claude-opus-4":   true,
		},
		ModelMapping: map[string]string{
			"claude-sonnet-4": "anthropic/claude-sonnet-4",
			"claude-opus-4":   "anthropic/claude-opus-4",
		},
	}

	errors := validProvider.ValidateConfiguration()
	if len(errors) != 0 {
		t.Errorf("完美配置不应有错误，但返回了: %v", errors)
	}

	// 场景 2：错误配置 - 映射目标不存在
	invalidProvider := Provider{
		Name: "InvalidProvider",
		SupportedModels: map[string]bool{
			"model-a": true,
		},
		ModelMapping: map[string]string{
			"external": "non-existent-model",
		},
	}

	errors = invalidProvider.ValidateConfiguration()
	if len(errors) == 0 {
		t.Errorf("错误配置应该返回验证错误")
	}

	// 场景 3：通配符配置
	wildcardProvider := Provider{
		Name: "WildcardProvider",
		SupportedModels: map[string]bool{
			"anthropic/claude-*": true,
			"openai/gpt-*":       true,
		},
		ModelMapping: map[string]string{
			"claude-*": "anthropic/claude-*",
			"gpt-*":    "openai/gpt-*",
		},
	}

	errors = wildcardProvider.ValidateConfiguration()
	if len(errors) != 0 {
		t.Errorf("通配符配置不应有错误，但返回了: %v", errors)
	}
}

func TestExtractRequestLogReasoningEffort(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		requestedModel string
		want           string
	}{
		{
			name: "nested reasoning effort",
			body: `{"reasoning":{"effort":"HIGH"}}`,
			want: "high",
		},
		{
			name: "flat x-high effort",
			body: `{"reasoning_effort":"x-high"}`,
			want: "xhigh",
		},
		{
			name: "claude output config max",
			body: `{"output_config":{"effort":"max"}}`,
			want: "max",
		},
		{
			name: "thinking adaptive",
			body: `{"thinking":{"type":"adaptive"}}`,
			want: "xhigh",
		},
		{
			name: "thinking budget",
			body: `{"thinking":{"type":"enabled","budget_tokens":5000}}`,
			want: "medium",
		},
		{
			name:           "model suffix fallback",
			body:           `{}`,
			requestedModel: "gpt-5.5-high",
			want:           "high",
		},
		{
			name: "gemini thinking level",
			body: `{"generationConfig":{"thinkingConfig":{"thinkingLevel":"x-high"}}}`,
			want: "xhigh",
		},
		{
			name: "snake case thinking budget",
			body: `{"generation_config":{"thinking_config":{"thinkingBudget":22000}}}`,
			want: "high",
		},
		{
			name: "gemini no thinking budget",
			body: `{"thinkingConfig":{"thinkingBudget":0}}`,
			want: "",
		},
		{
			name: "unknown effort",
			body: `{"reasoning_effort":"ultra"}`,
			want: "ultra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRequestLogReasoningEffort([]byte(tt.body), tt.requestedModel)
			if got != tt.want {
				t.Fatalf("extractRequestLogReasoningEffort() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractRequestLogReasoningEffortAfterGeminiOverrides(t *testing.T) {
	body, err := buildGeminiRequestBody([]byte(`{"contents":[{"parts":[{"text":"hi"}]}]}`), GeminiProvider{
		RequestBodyOverrides: map[string]any{
			"thinkingConfig": map[string]any{
				"includeThoughts": true,
				"thinkingBudget":  22000,
			},
		},
	})
	if err != nil {
		t.Fatalf("buildGeminiRequestBody failed: %v", err)
	}

	got := extractRequestLogReasoningEffort(body, "gemini-2.5-pro")
	if got != "high" {
		t.Fatalf("extractRequestLogReasoningEffort() = %q, want %q", got, "high")
	}
}

func TestDeriveCodexThreadSessionHash(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "thread id",
			body: `{"thread_id":"thread-a"}`,
			want: shortSHA256Hex("codex.thread_id=thread-a"),
		},
		{
			name: "thread context id",
			body: `{"id":"thread-a","cwd":"/repo"}`,
			want: shortSHA256Hex("codex.thread_context.id=thread-a"),
		},
		{
			name: "response id is ignored",
			body: `{"id":"resp_abc","output":[]}`,
			want: "",
		},
		{
			name: "thread id before session id",
			body: `{"session_id":"sess-a","thread_id":"thread-a"}`,
			want: shortSHA256Hex("codex.thread_id=thread-a"),
		},
		{
			name: "nested params cwd",
			body: `{"params":{"cwd":"/repo","archived":false}}`,
			want: shortSHA256Hex("codex.cwd=/repo"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveCodexThreadSessionHash([]byte(tt.body))
			if got != tt.want {
				t.Fatalf("deriveCodexThreadSessionHash() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveToolPairSessionHash(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		body     string
		want     string
	}{
		{
			name:     "claude tool use and result pair",
			platform: "claude",
			body:     `{"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Read"}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}]}`,
			want:     toolSessionHash("claude", []string{"toolu_1"}),
		},
		{
			name:     "claude only tool use ignored",
			platform: "claude",
			body:     `{"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Read"}]}]}`,
			want:     "",
		},
		{
			name:     "claude only tool result ignored",
			platform: "claude",
			body:     `{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}]}`,
			want:     "",
		},
		{
			name:     "responses function call and output pair",
			platform: "codex",
			body:     `{"input":[{"type":"function_call","call_id":"call_1","name":"shell"},{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`,
			want:     toolSessionHash("codex", []string{"call_1"}),
		},
		{
			name:     "chat tool call and role tool pair",
			platform: "codex",
			body:     `{"messages":[{"role":"assistant","tool_calls":[{"id":"call_2","type":"function","function":{"name":"shell"}}]},{"role":"tool","tool_call_id":"call_2","content":"ok"}]}`,
			want:     toolSessionHash("codex", []string{"call_2"}),
		},
		{
			name:     "nested body tool pair",
			platform: "claude",
			body:     `{"body":{"messages":[{"content":[{"type":"tool_use","id":"toolu_2"},{"type":"tool_result","tool_use_id":"toolu_2"}]}]}}`,
			want:     toolSessionHash("claude", []string{"toolu_2"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveToolPairSessionHash(tt.platform, []byte(tt.body))
			if got != tt.want {
				t.Fatalf("deriveToolPairSessionHash() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToolResponseSessionBinding(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	provider := Provider{
		ID:                 1,
		Name:               "Provider A",
		SessionMaxSessions: 5,
		SessionTTLMinutes:  5,
	}
	response := `{"output":[{"type":"function_call","call_id":"call_1","name":"shell"}]}`

	collector := &sessionAffinityToolResponseCollector{
		prs:      relay,
		kind:     "codex",
		provider: provider,
		callIDs:  map[string]bool{},
	}
	collector.observePayload(response)
	sessionHash := toolSessionHash("codex", []string{"call_1"})
	if binding := relay.sessionAffinity[sessionAffinityStateKey("codex", sessionHash)]; binding != nil {
		t.Fatalf("响应完成前不应写入 confirmed binding，got %#v", binding)
	}
	collector.commit()

	binding := relay.sessionAffinity[sessionAffinityStateKey("codex", sessionHash)]
	if binding == nil || !binding.Confirmed || binding.ProviderID != "1" {
		t.Fatalf("响应侧工具调用应补充 confirmed binding，got %#v", binding)
	}

	body := []byte(`{"input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`)
	got := relay.deriveRelaySessionHash("codex", body)
	if got != sessionHash {
		t.Fatalf("后续工具结果应命中响应侧注册的会话 hash，got %q want %q", got, sessionHash)
	}
}

func TestToolResponseCollectorSkipsWhenOriginalRequestHasSession(t *testing.T) {
	appSettings := &AppSettingsService{path: t.TempDir() + "/settings.json"}
	if _, err := appSettings.SaveAppSettings(AppSettings{
		SessionAffinity: map[string]bool{"claude": true},
	}); err != nil {
		t.Fatalf("保存测试设置失败: %v", err)
	}
	relay := NewProviderRelayService(nil, nil, nil, nil, appSettings, nil, "")
	plan := providerRequestPlan{
		OriginalBodyBytes: []byte(`{"metadata":{"session_id":"session-a"}}`),
		BodyBytes:         []byte(`{"input":[]}`),
	}

	collector := relay.newSessionAffinityToolResponseCollector("claude", Provider{ID: 1, Name: "Provider A"}, plan)
	if collector != nil {
		t.Fatalf("原始请求已有 sessionHash 时不应创建响应侧工具会话 collector")
	}
}

func TestExtractResponseToolCallIDsIgnoresPlainResponseID(t *testing.T) {
	got := extractResponseToolCallIDs(`{"id":"resp_abc","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`)
	if len(got) != 0 {
		t.Fatalf("普通响应 id 不应被当作工具调用会话，got %#v", got)
	}
}

// ==================== 性能测试 ====================

func BenchmarkIsModelSupported(b *testing.B) {
	provider := Provider{
		SupportedModels: map[string]bool{
			"claude-sonnet-4": true,
			"claude-opus-4":   true,
			"gpt-4":           true,
			"gpt-4-turbo":     true,
		},
		ModelMapping: map[string]string{
			"claude-*": "anthropic/claude-*",
			"gpt-*":    "openai/gpt-*",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.IsModelSupported("claude-sonnet-4")
	}
}

func BenchmarkGetEffectiveModel(b *testing.B) {
	provider := Provider{
		ModelMapping: map[string]string{
			"claude-*": "anthropic/claude-*",
			"gpt-*":    "openai/gpt-*",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetEffectiveModel("claude-sonnet-4")
	}
}

func BenchmarkReplaceModelInRequestBody(b *testing.B) {
	bodyBytes := []byte(`{
		"model": "claude-sonnet-4",
		"messages": [{"role": "user", "content": "Hello"}],
		"temperature": 0.7,
		"max_tokens": 1000
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ReplaceModelInRequestBody(bodyBytes, "anthropic/claude-sonnet-4")
	}
}

func TestSessionAffinityConcurrentPendingFailuresReleaseBinding(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	sessionHash := "session-a"

	attemptA := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", 5, 5, true)
	attemptB := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", 5, 5, true)
	if attemptA == 0 || attemptB == 0 || attemptA != attemptB {
		t.Fatalf("pending 并发应共享同一次临时绑定 attempt，got A=%d B=%d", attemptA, attemptB)
	}

	relay.finishSessionProviderRequest(platform, sessionHash)
	relay.restoreOrReleaseSessionBinding(platform, sessionHash, nil, attemptA)
	if binding := relay.sessionAffinity[sessionAffinityStateKey(platform, sessionHash)]; binding == nil || binding.ActiveRequests != 1 {
		t.Fatalf("仍有 in-flight 请求时不应释放 binding，got %#v", binding)
	}

	relay.finishSessionProviderRequest(platform, sessionHash)
	relay.restoreOrReleaseSessionBinding(platform, sessionHash, nil, attemptB)
	if binding := relay.sessionAffinity[sessionAffinityStateKey(platform, sessionHash)]; binding != nil {
		t.Fatalf("全部失败后应释放新会话临时 binding，got %#v", binding)
	}
}

func TestSessionAffinityConcurrentPendingSuccessAndFailureKeepsConfirmedBinding(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	sessionHash := "session-b"

	attemptA := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", 5, 5, true)
	attemptB := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", 5, 5, true)
	relay.finishSessionProviderRequest(platform, sessionHash)
	relay.confirmSessionProviderBinding(platform, sessionHash, attemptA)
	relay.finishSessionProviderRequest(platform, sessionHash)
	relay.restoreOrReleaseSessionBinding(platform, sessionHash, nil, attemptB)

	binding := relay.sessionAffinity[sessionAffinityStateKey(platform, sessionHash)]
	if binding == nil || !binding.Confirmed || binding.Pending || binding.ProviderID != "provider-a" {
		t.Fatalf("应保留成功确认的 binding，got %#v", binding)
	}
}

func TestSessionAffinityNewSessionFailoverRebindsToNextProvider(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	sessionHash := "session-c"

	attemptA := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", 5, 5, true)
	if attemptA <= 0 {
		t.Fatalf("新会话首次 provider 应创建 pending attempt，got %d", attemptA)
	}
	relay.finishSessionProviderRequest(platform, sessionHash)
	relay.restoreOrReleaseSessionBinding(platform, sessionHash, nil, attemptA)

	attemptB := relay.beginSessionProviderRequest(platform, sessionHash, "provider-b", "Provider B", 5, 5, true)
	if attemptB <= 0 || attemptB == attemptA {
		t.Fatalf("A 失败后应允许 B 创建新 attempt，got A=%d B=%d", attemptA, attemptB)
	}
	relay.finishSessionProviderRequest(platform, sessionHash)
	relay.confirmSessionProviderBinding(platform, sessionHash, attemptB)
	binding := relay.sessionAffinity[sessionAffinityStateKey(platform, sessionHash)]
	if binding == nil || !binding.Confirmed || binding.ProviderID != "provider-b" {
		t.Fatalf("B 成功后应绑定到 B，got %#v", binding)
	}
}

func TestSessionAffinityConfirmedSessionFailoverMigratesProvider(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	sessionHash := "session-d"

	attemptA := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", 5, 5, true)
	relay.confirmSessionProviderBinding(platform, sessionHash, attemptA)
	relay.finishSessionProviderRequest(platform, sessionHash)
	original := relay.getSessionBindingSnapshot(platform, sessionHash)

	attemptB := relay.beginSessionProviderRequest(platform, sessionHash, "provider-b", "Provider B", 5, 5, false)
	if attemptB <= 0 {
		t.Fatalf("老会话 A 失败后应允许创建迁移 attempt，got %d", attemptB)
	}
	relay.finishSessionProviderRequest(platform, sessionHash)
	relay.confirmSessionProviderBinding(platform, sessionHash, attemptB)

	binding := relay.sessionAffinity[sessionAffinityStateKey(platform, sessionHash)]
	if original == nil || binding == nil || !binding.Confirmed || binding.ProviderID != "provider-b" {
		t.Fatalf("迁移成功后应绑定到 B，original=%#v binding=%#v", original, binding)
	}
}

func TestSessionAffinityConfirmedSessionAllFailKeepsOriginalProvider(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	sessionHash := "session-e"

	attemptA := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", 5, 5, true)
	relay.confirmSessionProviderBinding(platform, sessionHash, attemptA)
	relay.finishSessionProviderRequest(platform, sessionHash)
	original := relay.getSessionBindingSnapshot(platform, sessionHash)

	attemptB := relay.beginSessionProviderRequest(platform, sessionHash, "provider-b", "Provider B", 5, 5, false)
	relay.finishSessionProviderRequest(platform, sessionHash)
	relay.restoreOrReleaseSessionBinding(platform, sessionHash, original, attemptB)

	binding := relay.sessionAffinity[sessionAffinityStateKey(platform, sessionHash)]
	if binding == nil || !binding.Confirmed || binding.ProviderID != "provider-a" || binding.ActiveRequests != 0 {
		t.Fatalf("全部失败后应恢复原 Provider A，got %#v", binding)
	}
}

func TestBeginSessionProviderRequestSkipsDifferentProviderDuringActiveMigration(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	sessionHash := "session-f"

	attemptA := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", 5, 5, true)
	relay.confirmSessionProviderBinding(platform, sessionHash, attemptA)

	wrongAttempt := relay.beginSessionProviderRequest(platform, sessionHash, "provider-b", "Provider B", 5, 5, false)
	if wrongAttempt >= 0 {
		t.Fatalf("原 Provider 仍有 in-flight 时不同 provider 应跳过，got attempt=%d", wrongAttempt)
	}
	binding := relay.sessionAffinity[sessionAffinityStateKey(platform, sessionHash)]
	if binding == nil || binding.ProviderID != "provider-a" || binding.ActiveRequests != 1 {
		t.Fatalf("不应把计数或 provider 改到错误 provider，got %#v", binding)
	}
}

func TestProviderSessionLoadsIncludesBoundSessionsAndActiveRequests(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	relay.sessionAffinity[sessionAffinityStateKey(platform, "session-a")] = &providerSessionBinding{
		Platform:       platform,
		SessionHash:    "session-a",
		ProviderID:     "provider-a",
		LastSeen:       time.Now(),
		ActiveRequests: 2,
		Confirmed:      true,
	}
	relay.sessionAffinity[sessionAffinityStateKey(platform, "session-b")] = &providerSessionBinding{
		Platform:    platform,
		SessionHash: "session-b",
		ProviderID:  "provider-a",
		LastSeen:    time.Now(),
		Pending:     true,
	}
	relay.sessionAffinity[sessionAffinityStateKey(platform, "session-c")] = &providerSessionBinding{
		Platform:       platform,
		SessionHash:    "session-c",
		ProviderID:     "provider-b",
		LastSeen:       time.Now(),
		ActiveRequests: 1,
		Confirmed:      true,
	}

	loads := relay.providerSessionLoads(platform)
	if got := loads["provider-a"]; got.BoundSessions != 2 || got.ActiveRequests != 2 {
		t.Fatalf("provider-a load = %#v, want bound=2 active=2", got)
	}
	if got := loads["provider-b"]; got.BoundSessions != 1 || got.ActiveRequests != 1 {
		t.Fatalf("provider-b load = %#v, want bound=1 active=1", got)
	}
}

func TestOrderProvidersForSessionAffinityUsesWeightedLoadRate(t *testing.T) {
	providers := []Provider{
		{ID: 1, Name: "A", SessionMaxSessions: 5},
		{ID: 2, Name: "B", SessionMaxSessions: 5},
		{ID: 3, Name: "C", SessionMaxSessions: 5},
	}
	loads := map[string]providerSessionLoad{
		"1": {ProviderID: "1", BoundSessions: 1, ActiveRequests: 3},
		"2": {ProviderID: "2", BoundSessions: 2},
	}

	ordered := orderProvidersForSessionAffinity(providers, loads)
	got := []string{providerRefFromProvider(ordered[0]), providerRefFromProvider(ordered[1]), providerRefFromProvider(ordered[2])}
	want := []string{"3", "2", "1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestOrderProvidersForSessionAffinityFallsBackToLowestLoadRateWhenAllFull(t *testing.T) {
	providers := []Provider{
		{ID: 1, Name: "A", SessionMaxSessions: 2},
		{ID: 2, Name: "B", SessionMaxSessions: 4},
		{ID: 3, Name: "C", SessionMaxSessions: 3},
	}
	loads := map[string]providerSessionLoad{
		"1": {ProviderID: "1", BoundSessions: 2, ActiveRequests: 1},
		"2": {ProviderID: "2", BoundSessions: 4, ActiveRequests: 1},
		"3": {ProviderID: "3", BoundSessions: 3, ActiveRequests: 3},
	}

	ordered := orderProvidersForSessionAffinity(providers, loads)
	got := []string{providerRefFromProvider(ordered[0]), providerRefFromProvider(ordered[1]), providerRefFromProvider(ordered[2])}
	want := []string{"2", "1", "3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestReorderProviderAttemptsForSessionKeepsExistingBindingFirst(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	sessionHash := "session-existing"
	relay.sessionAffinity[sessionAffinityStateKey(platform, sessionHash)] = &providerSessionBinding{
		Platform:    platform,
		SessionHash: sessionHash,
		ProviderID:  "2",
		LastSeen:    time.Now(),
		Confirmed:   true,
	}
	providers := []Provider{
		{ID: 1, Name: "A", SessionMaxSessions: 1},
		{ID: 2, Name: "B", SessionMaxSessions: 1},
		{ID: 3, Name: "C", SessionMaxSessions: 1},
	}

	ordered := relay.reorderProviderAttemptsForSession(platform, providers, sessionHash, true)
	if providerRefFromProvider(ordered[0]) != "2" {
		t.Fatalf("已绑定会话应优先原 provider，got %s", ordered[0].Name)
	}
}

func TestOrderGeminiProvidersForSessionAffinityUsesWeightedLoadRate(t *testing.T) {
	providers := []GeminiProvider{
		{ID: "a", Name: "A", SessionMaxSessions: 2},
		{ID: "b", Name: "B", SessionMaxSessions: 4},
		{ID: "c", Name: "C", SessionMaxSessions: 2},
	}
	loads := map[string]providerSessionLoad{
		"a": {ProviderID: "a", BoundSessions: 1, ActiveRequests: 1},
		"b": {ProviderID: "b", BoundSessions: 1},
		"c": {ProviderID: "c", BoundSessions: 2},
	}

	ordered := orderGeminiProvidersForSessionAffinity(providers, loads)
	got := []string{providerRefFromGeminiProvider(ordered[0]), providerRefFromGeminiProvider(ordered[1]), providerRefFromGeminiProvider(ordered[2])}
	want := []string{"b", "a", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("gemini order = %v, want %v", got, want)
		}
	}
}

func TestRoundRobinOrderPreviewDoesNotAdvanceState(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	providers := []Provider{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}, {ID: 3, Name: "C"}}

	first := relay.roundRobinOrder("claude", 1, providers)
	preview := relay.roundRobinOrderPreview("claude", 1, providers)
	second := relay.roundRobinOrder("claude", 1, providers)

	if providerRefFromProvider(first[0]) != "1" {
		t.Fatalf("首次轮询应从原始第一个开始，got %s", first[0].Name)
	}
	if providerRefFromProvider(preview[0]) != "2" {
		t.Fatalf("preview 应基于当前状态预览下一位，got %s", preview[0].Name)
	}
	if providerRefFromProvider(second[0]) != "2" {
		t.Fatalf("preview 不应推进状态，下一次真实轮询仍应为第二位，got %s", second[0].Name)
	}
}
