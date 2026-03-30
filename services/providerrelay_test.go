package services

import (
	"encoding/json"
	"testing"

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
