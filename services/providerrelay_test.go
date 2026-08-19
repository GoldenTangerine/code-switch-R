package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
	"github.com/daodao97/xgo/xrequest"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type blockingGinResponseWriter struct {
	gin.ResponseWriter
	entered chan time.Time
	release chan struct{}
}

func (writer *blockingGinResponseWriter) Write(data []byte) (int, error) {
	writer.entered <- time.Now()
	<-writer.release
	return writer.ResponseWriter.Write(data)
}

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

func TestBuildProviderRequestPlanCapturesCompleteModelRoute(t *testing.T) {
	provider := Provider{
		ModelMapping: map[string]string{
			"claude-opus-*": "vendor-opus-*",
		},
		ModelMappingSupports1M: map[string]bool{
			"claude-opus-*": true,
		},
		RequestBodyOverrides: map[string]interface{}{
			"model": "forced-opus-model",
		},
	}
	plan, err := buildProviderRequestPlan(
		provider,
		[]byte(`{"model":"claude-opus-4.8","messages":[]}`),
		"/v1/messages",
		"claude-opus-4.8",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.ModelRouteCaptured {
		t.Fatalf("模型路由详情应标记为已记录")
	}
	if plan.ModelMappingPattern != "claude-opus-*" || plan.ModelMappingTarget != "vendor-opus-*" {
		t.Fatalf("模型映射规则错误: %#v", plan)
	}
	if !plan.ModelMappingSupports1M {
		t.Fatalf("模型映射应声明支持 1M: %#v", plan)
	}
	if plan.MappedModel != "vendor-opus-4.8" || plan.ModelOverride != "forced-opus-model" || plan.EffectiveModel != "forced-opus-model" {
		t.Fatalf("模型改写链错误: %#v", plan)
	}
}

func TestApplyModelMappingOneMHeader(t *testing.T) {
	tests := []struct {
		name      string
		headers   map[string]string
		kind      string
		apiFormat string
		enabled   bool
		expected  string
	}{
		{
			name:      "原生 Anthropic 合并现有 beta",
			headers:   map[string]string{"Anthropic-Beta": "claude-code-20250219"},
			kind:      "claude",
			apiFormat: claudeAPIFormatAnthropic,
			enabled:   true,
			expected:  "claude-code-20250219,context-1m-2025-08-07",
		},
		{
			name:      "已有 1M beta 时不重复",
			headers:   map[string]string{"anthropic-beta": "claude-code-20250219, context-1m-2025-08-07"},
			kind:      "claude",
			apiFormat: claudeAPIFormatAnthropic,
			enabled:   true,
			expected:  "claude-code-20250219, context-1m-2025-08-07",
		},
		{
			name:      "OpenAI 格式不注入",
			headers:   map[string]string{},
			kind:      "claude",
			apiFormat: claudeAPIFormatOpenAIChat,
			enabled:   true,
			expected:  "",
		},
		{
			name:      "未声明时不注入",
			headers:   map[string]string{},
			kind:      "claude",
			apiFormat: claudeAPIFormatAnthropic,
			enabled:   false,
			expected:  "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			applyModelMappingOneMHeader(test.headers, test.kind, test.apiFormat, test.enabled)
			if actual := getHeaderValueCaseInsensitive(test.headers, "anthropic-beta"); actual != test.expected {
				t.Fatalf("anthropic-beta = %q, 期望 %q", actual, test.expected)
			}
		})
	}
}

func TestBuildProviderRequestPlanAppliesModelMappingReasoningEffort(t *testing.T) {
	tests := []struct {
		name           string
		provider       Provider
		body           string
		endpoint       string
		requestedModel string
		effortPath     string
		wantEffort     string
		wantSource     string
	}{
		{
			name: "anthropic-thinking-enabled",
			provider: Provider{
				ModelMapping:                 map[string]string{"claude-*": "vendor-*"},
				ModelMappingReasoningEfforts: map[string]string{"claude-*": "high"},
			},
			body:           `{"model":"claude-opus-4.8","thinking":{"type":"adaptive"},"messages":[]}`,
			endpoint:       "/v1/messages",
			requestedModel: "claude-opus-4.8",
			effortPath:     "output_config.effort",
			wantEffort:     "high",
			wantSource:     reasoningEffortSourceModelMapping,
		},
		{
			name: "anthropic-existing-effort-without-thinking",
			provider: Provider{
				ModelMapping:                 map[string]string{"claude-*": "vendor-*"},
				ModelMappingReasoningEfforts: map[string]string{"claude-*": "xhigh"},
			},
			body:           `{"model":"claude-opus-4.8","output_config":{"effort":"low"},"messages":[]}`,
			endpoint:       "/v1/messages",
			requestedModel: "claude-opus-4.8",
			effortPath:     "output_config.effort",
			wantEffort:     "xhigh",
			wantSource:     reasoningEffortSourceModelMapping,
		},
		{
			name: "anthropic-does-not-enable-thinking",
			provider: Provider{
				ModelMapping:                 map[string]string{"claude-*": "vendor-*"},
				ModelMappingReasoningEfforts: map[string]string{"claude-*": "high"},
			},
			body:           `{"model":"claude-opus-4.8","messages":[]}`,
			endpoint:       "/v1/messages",
			requestedModel: "claude-opus-4.8",
			effortPath:     "output_config.effort",
			wantSource:     "",
		},
		{
			name: "openai-chat-max-normalized",
			provider: Provider{
				APIFormat:                    claudeAPIFormatOpenAIChat,
				ModelMapping:                 map[string]string{"claude-*": "vendor-*"},
				ModelMappingReasoningEfforts: map[string]string{"claude-*": "max"},
			},
			body:           `{"model":"claude-opus-4.8","thinking":{"type":"enabled","budget_tokens":8000},"messages":[]}`,
			endpoint:       "/v1/messages",
			requestedModel: "claude-opus-4.8",
			effortPath:     "reasoning_effort",
			wantEffort:     "xhigh",
			wantSource:     reasoningEffortSourceModelMapping,
		},
		{
			name: "openai-responses-custom-effort",
			provider: Provider{
				APIFormat:                    claudeAPIFormatOpenAIResponse,
				ModelMapping:                 map[string]string{"claude-*": "vendor-*"},
				ModelMappingReasoningEfforts: map[string]string{"claude-*": "vendor-ultra"},
			},
			body:           `{"model":"claude-opus-4.8","thinking":{"type":"adaptive"},"messages":[]}`,
			endpoint:       "/v1/messages",
			requestedModel: "claude-opus-4.8",
			effortPath:     "reasoning.effort",
			wantEffort:     "vendor-ultra",
			wantSource:     reasoningEffortSourceModelMapping,
		},
		{
			name: "codex-responses-existing-effort",
			provider: Provider{
				ModelMapping:                 map[string]string{"gpt-*": "vendor-*"},
				ModelMappingReasoningEfforts: map[string]string{"gpt-*": "max"},
				RequestBodyOverrides: map[string]interface{}{
					"reasoning": map[string]interface{}{"effort": "medium"},
				},
			},
			body:           `{"model":"gpt-5.4","reasoning":{"effort":"low"},"input":[]}`,
			endpoint:       "/responses",
			requestedModel: "gpt-5.4",
			effortPath:     "reasoning.effort",
			wantEffort:     "xhigh",
			wantSource:     reasoningEffortSourceModelMapping,
		},
		{
			name: "codex-responses-minimal-effort",
			provider: Provider{
				ModelMapping:                 map[string]string{"gpt-*": "vendor-*"},
				ModelMappingReasoningEfforts: map[string]string{"gpt-*": "minimal"},
			},
			body:           `{"model":"gpt-5.4","reasoning":{"effort":"minimal"},"input":[]}`,
			endpoint:       "/responses",
			requestedModel: "gpt-5.4",
			effortPath:     "reasoning.effort",
			wantEffort:     "minimal",
			wantSource:     reasoningEffortSourceModelMapping,
		},
		{
			name: "request-source",
			provider: Provider{
				ModelMapping: map[string]string{"gpt-*": "vendor-*"},
			},
			body:           `{"model":"gpt-5.4","reasoning":{"effort":"low"},"input":[]}`,
			endpoint:       "/responses",
			requestedModel: "gpt-5.4",
			effortPath:     "reasoning.effort",
			wantEffort:     "low",
			wantSource:     reasoningEffortSourceRequest,
		},
		{
			name: "request-body-override-source",
			provider: Provider{
				ModelMapping: map[string]string{"gpt-*": "vendor-*"},
				RequestBodyOverrides: map[string]interface{}{
					"reasoning": map[string]interface{}{"effort": "medium"},
				},
			},
			body:           `{"model":"gpt-5.4","reasoning":{"effort":"low"},"input":[]}`,
			endpoint:       "/responses",
			requestedModel: "gpt-5.4",
			effortPath:     "reasoning.effort",
			wantEffort:     "medium",
			wantSource:     reasoningEffortSourceRequestBodyOverride,
		},
		{
			name: "thinking-budget-override-source",
			provider: Provider{
				ModelMapping: map[string]string{"claude-*": "vendor-*"},
				RequestBodyOverrides: map[string]interface{}{
					"thinking": map[string]interface{}{"budget_tokens": 20000},
				},
			},
			body:           `{"model":"claude-opus-4.8","thinking":{"type":"enabled","budget_tokens":1000},"messages":[]}`,
			endpoint:       "/v1/messages",
			requestedModel: "claude-opus-4.8",
			effortPath:     "thinking.budget_tokens",
			wantEffort:     "20000",
			wantSource:     reasoningEffortSourceRequestBodyOverride,
		},
		{
			name: "lower-priority-thinking-override-does-not-own-source",
			provider: Provider{
				ModelMapping: map[string]string{"claude-*": "vendor-*"},
				RequestBodyOverrides: map[string]interface{}{
					"thinking": map[string]interface{}{"type": "adaptive"},
				},
			},
			body:           `{"model":"claude-opus-4.8","output_config":{"effort":"low"},"messages":[]}`,
			endpoint:       "/v1/messages",
			requestedModel: "claude-opus-4.8",
			effortPath:     "output_config.effort",
			wantEffort:     "low",
			wantSource:     reasoningEffortSourceRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := buildProviderRequestPlan(test.provider, []byte(test.body), test.endpoint, test.requestedModel)
			if err != nil {
				t.Fatal(err)
			}
			if got := gjson.GetBytes(plan.BodyBytes, test.effortPath).String(); got != test.wantEffort {
				t.Fatalf("%s=%q，期望 %q，body=%s", test.effortPath, got, test.wantEffort, plan.BodyBytes)
			}
			if plan.Reasoning.Source != test.wantSource {
				t.Fatalf("Reasoning.Source=%q，期望 %q", plan.Reasoning.Source, test.wantSource)
			}
			wantTargetPath := ""
			if test.wantSource != "" {
				wantTargetPath = test.effortPath
			}
			if plan.Reasoning.TargetPath != wantTargetPath {
				t.Fatalf("Reasoning.TargetPath=%q，期望 %q", plan.Reasoning.TargetPath, wantTargetPath)
			}
			wantModelMappingApplied := test.wantSource == reasoningEffortSourceModelMapping
			if plan.Reasoning.ModelMappingApplied != wantModelMappingApplied {
				t.Fatalf("Reasoning.ModelMappingApplied=%t，期望 %t", plan.Reasoning.ModelMappingApplied, wantModelMappingApplied)
			}
		})
	}
}

func TestBuildProviderRequestPlanCapturesConnectionParameters(t *testing.T) {
	provider := Provider{
		APIFormat:                    claudeAPIFormatOpenAIResponse,
		ModelMapping:                 map[string]string{"claude-*": "vendor-*"},
		ModelMappingReasoningEfforts: map[string]string{"claude-*": "high"},
		RequestBodyOverrides: map[string]interface{}{
			"max_tokens": 32768,
		},
	}
	plan, err := buildProviderRequestPlan(
		provider,
		[]byte(`{"model":"claude-opus-4.8","thinking":{"type":"adaptive"},"max_tokens":16384,"messages":[]}`),
		"/v1/messages",
		"claude-opus-4.8",
	)
	if err != nil {
		t.Fatal(err)
	}

	reasoning := providerRequestParameterByKey(plan.Parameters, providerRequestParameterReasoningEffort)
	if reasoning.RequestedValue != "xhigh" || reasoning.ActualValue != "high" || reasoning.Source != reasoningEffortSourceModelMapping {
		t.Fatalf("思考强度参数快照错误: %#v", reasoning)
	}
	maxOutput := providerRequestParameterByKey(plan.Parameters, providerRequestParameterMaxOutputTokens)
	if maxOutput.RequestedValue != "16384" || maxOutput.ActualValue != "32768" || maxOutput.Source != reasoningEffortSourceRequestBodyOverride {
		t.Fatalf("最大输出参数快照错误: %#v", maxOutput)
	}
}

func TestBuildProviderRequestPlanKeepsRequestSourceAcrossProtocolTransform(t *testing.T) {
	plan, err := buildProviderRequestPlan(
		Provider{APIFormat: claudeAPIFormatOpenAIResponse},
		[]byte(`{"model":"claude-opus-4.8","max_tokens":4096,"messages":[]}`),
		"/v1/messages",
		"claude-opus-4.8",
	)
	if err != nil {
		t.Fatal(err)
	}

	maxOutput := providerRequestParameterByKey(plan.Parameters, providerRequestParameterMaxOutputTokens)
	if maxOutput.RequestedValue != "4096" || maxOutput.ActualValue != "4096" || maxOutput.Source != reasoningEffortSourceRequest {
		t.Fatalf("协议转换不应改变最大输出来源: %#v", maxOutput)
	}
}

func TestBuildProviderRequestPlanUsesActualProtocolForMaxOutput(t *testing.T) {
	plan, err := buildProviderRequestPlan(
		Provider{
			RequestBodyOverrides: map[string]interface{}{
				"max_output_tokens": 8192,
			},
		},
		[]byte(`{"model":"claude-opus-4.8","max_tokens":4096,"messages":[]}`),
		"/v1/messages",
		"claude-opus-4.8",
	)
	if err != nil {
		t.Fatal(err)
	}

	maxOutput := providerRequestParameterByKey(plan.Parameters, providerRequestParameterMaxOutputTokens)
	if maxOutput.RequestedValue != "4096" || maxOutput.ActualValue != "4096" || maxOutput.Source != reasoningEffortSourceRequest {
		t.Fatalf("实际 Anthropic 协议应选择 max_tokens: %#v", maxOutput)
	}
}

func TestRequestMaxOutputMetadataSupportsProtocolAliases(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		protocol string
		want     string
	}{
		{name: "anthropic", body: `{"max_tokens":4096}`, protocol: providerRequestProtocolAnthropic, want: "4096"},
		{name: "openai-responses", body: `{"max_output_tokens":8192}`, protocol: providerRequestProtocolOpenAIResponses, want: "8192"},
		{name: "openai-chat", body: `{"max_completion_tokens":12288}`, protocol: providerRequestProtocolOpenAIChat, want: "12288"},
		{name: "gemini", body: `{"generationConfig":{"maxOutputTokens":16384}}`, protocol: providerRequestProtocolGemini, want: "16384"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := requestMaxOutputMetadata([]byte(test.body), test.protocol).Value; got != test.want {
				t.Fatalf("max output=%q，期望 %q", got, test.want)
			}
		})
	}
}

func TestBuildGeminiRequestParametersCapturesMaxOutputOverride(t *testing.T) {
	originalBody := []byte(`{"generationConfig":{"thinkingConfig":{"thinkingLevel":"low"},"maxOutputTokens":8192}}`)
	actualBody := []byte(`{"generationConfig":{"thinkingConfig":{"thinkingLevel":"high"},"maxOutputTokens":16384}}`)
	overrides := map[string]interface{}{
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": 16384,
		},
	}
	parameters := buildProviderRequestParameters(
		originalBody,
		actualBody,
		"gemini-2.5-pro",
		providerRequestReasoningMetadata{Effort: "high", Source: reasoningEffortSourceRequestBodyOverride},
		requestBodyOverridesMaxOutput(originalBody, actualBody, overrides, providerRequestProtocolGemini),
		providerRequestProtocolGemini,
		providerRequestProtocolGemini,
	)

	reasoning := providerRequestParameterByKey(parameters, providerRequestParameterReasoningEffort)
	if reasoning.RequestedValue != "low" || reasoning.ActualValue != "high" || reasoning.Source != reasoningEffortSourceRequestBodyOverride {
		t.Fatalf("Gemini 思考强度参数错误: %#v", reasoning)
	}
	maxOutput := providerRequestParameterByKey(parameters, providerRequestParameterMaxOutputTokens)
	if maxOutput.RequestedValue != "8192" || maxOutput.ActualValue != "16384" || maxOutput.Source != reasoningEffortSourceRequestBodyOverride {
		t.Fatalf("Gemini 最大输出参数错误: %#v", maxOutput)
	}
}

func providerRequestParameterByKey(parameters []ProviderConcurrencyRequestParameter, key string) ProviderConcurrencyRequestParameter {
	for _, parameter := range parameters {
		if parameter.Key == key {
			return parameter
		}
	}
	return ProviderConcurrencyRequestParameter{}
}

func TestBuildProviderRequestPlanDoesNotReinjectRememberedUnsupportedReasoning(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	provider := Provider{
		ID:                           7,
		APIURL:                       "https://provider.example.com",
		APIFormat:                    claudeAPIFormatOpenAIResponse,
		ModelMapping:                 map[string]string{"claude-*": "vendor-*"},
		ModelMappingReasoningEfforts: map[string]string{"claude-*": "high"},
	}
	effectiveEndpoint := resolveProviderEffectiveEndpoint("claude", provider, "/v1/messages")
	relay.rememberUnsupportedOptionalParams(provider, effectiveEndpoint, []string{"reasoning"})

	plan, err := relay.buildProviderRequestPlan(
		provider,
		[]byte(`{"model":"claude-opus-4.8","thinking":{"type":"adaptive"},"messages":[]}`),
		"/v1/messages",
		"claude-opus-4.8",
	)
	if err != nil {
		t.Fatal(err)
	}
	if gjson.GetBytes(plan.BodyBytes, "reasoning").Exists() {
		t.Fatalf("已记忆不支持 reasoning 后不应重新注入，body=%s", plan.BodyBytes)
	}
	if plan.Reasoning.Source != "" {
		t.Fatalf("未发送思考强度时来源应为空，got=%q", plan.Reasoning.Source)
	}
	if plan.Reasoning.ModelMappingApplied {
		t.Fatal("已清理模型映射强度时不应标记为已应用")
	}
}

func TestBuildClaudeCompatibilityRetryPlanRefreshesReasoningMetadata(t *testing.T) {
	plan := providerRequestPlan{
		BodyBytes: []byte(`{"model":"vendor-opus","reasoning":{"effort":"high"},"input":[]}`),
		Reasoning: providerRequestReasoningMetadata{
			Effort:              "high",
			Source:              reasoningEffortSourceModelMapping,
			TargetPath:          "reasoning.effort",
			ModelMappingApplied: true,
		},
		Parameters: []ProviderConcurrencyRequestParameter{
			{
				Key:            providerRequestParameterReasoningEffort,
				RequestedValue: "xhigh",
				ActualValue:    "high",
				Source:         reasoningEffortSourceModelMapping,
			},
			{
				Key:            providerRequestParameterMaxOutputTokens,
				RequestedValue: "4096",
				ActualValue:    "4096",
				Source:         reasoningEffortSourceRequest,
			},
		},
	}
	retryPlan, _ := buildClaudeCompatibilityRetryPlan(plan, claudeCompatibilityRetry{
		UnsupportedFields: []string{"reasoning"},
	}, "claude-opus-4.8")

	if gjson.GetBytes(retryPlan.BodyBytes, "reasoning").Exists() {
		t.Fatalf("兼容重试应删除 reasoning，body=%s", retryPlan.BodyBytes)
	}
	if retryPlan.Reasoning.Source != "" {
		t.Fatalf("兼容重试删除强度后来源应为空，got=%q", retryPlan.Reasoning.Source)
	}
	if retryPlan.Reasoning.ModelMappingApplied {
		t.Fatal("兼容重试删除强度后不应标记为已应用")
	}
	reasoning := providerRequestParameterByKey(retryPlan.Parameters, providerRequestParameterReasoningEffort)
	if reasoning.RequestedValue != "xhigh" || reasoning.ActualValue != "" || reasoning.Source != "" {
		t.Fatalf("兼容重试后的思考强度参数错误: %#v", reasoning)
	}
	maxOutput := providerRequestParameterByKey(retryPlan.Parameters, providerRequestParameterMaxOutputTokens)
	if maxOutput.RequestedValue != "4096" || maxOutput.ActualValue != "" || maxOutput.Source != "" {
		t.Fatalf("请求体中不存在最大输出时应刷新为空: %#v", maxOutput)
	}
}

func TestForwardRequestPersistsModelRouteDetails(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	previousLogQueue := GlobalDBQueueLogs
	logQueue := NewDBWriteQueue(db, 32, true)
	GlobalDBQueueLogs = logQueue
	t.Cleanup(func() {
		_ = logQueue.Shutdown(2 * time.Second)
		GlobalDBQueueLogs = previousLogQueue
	})

	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			t.Errorf("读取上游请求失败: %v", readErr)
		}
		if got := gjson.GetBytes(body, "model").String(); got != "forced-opus-model" {
			t.Errorf("上游实际模型 = %q, 期望 forced-opus-model", got)
		}
		if got := gjson.GetBytes(body, "output_config.effort").String(); got != "high" {
			t.Errorf("上游思考强度 = %q, 期望 high", got)
		}
		if got := request.Header.Get("anthropic-beta"); got != "claude-code-20250219,context-1m-2025-08-07" {
			t.Errorf("上游 anthropic-beta = %q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"msg_route","type":"message","model":"forced-opus-model","content":[],"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	provider := Provider{
		ID:      88,
		Name:    "Route Persistence",
		APIURL:  upstream.URL,
		APIKey:  "test-key",
		Enabled: true,
		ModelMapping: map[string]string{
			"claude-opus-*": "vendor-opus-*",
		},
		ModelMappingReasoningEfforts: map[string]string{
			"claude-opus-*": "high",
		},
		ModelMappingSupports1M: map[string]bool{
			"claude-opus-*": true,
		},
		RequestBodyOverrides: map[string]interface{}{
			"model": "forced-opus-model",
		},
	}
	requestedModel := "claude-opus-4.8"
	bodyBytes := []byte(`{"model":"claude-opus-4.8","thinking":{"type":"adaptive"},"messages":[],"stream":false}`)
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	plan, err := relay.buildProviderRequestPlan(provider, bodyBytes, "/v1/messages", requestedModel)
	if err != nil {
		t.Fatalf("构建请求计划失败: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(bodyBytes))
	response := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(response)
	ginContext.Request = request
	ok, err := relay.forwardRequestWithPlan(
		ginContext,
		"claude",
		provider,
		plan.EffectiveEndpoint,
		map[string]string{},
		map[string]string{"Content-Type": "application/json", "anthropic-beta": "claude-code-20250219"},
		plan.BodyBytes,
		false,
		plan.EffectiveModel,
		requestedModel,
		plan,
		false,
	)
	if err != nil || !ok {
		t.Fatalf("转发请求失败: ok=%v err=%v", ok, err)
	}

	page, err := NewLogService(nil).ListRequestLogsPageV2("claude", providerRefFromProvider(provider), "", 10, 0, "", "")
	if err != nil {
		t.Fatalf("读取请求日志失败: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("请求日志数量 = %d, 期望 1", len(page.Items))
	}
	logEntry := page.Items[0]
	if logEntry.RequestedModel != requestedModel || logEntry.Model != "forced-opus-model" {
		t.Fatalf("持久化模型路由 = %q -> %q", logEntry.RequestedModel, logEntry.Model)
	}
	if !logEntry.ModelRouteCaptured || logEntry.MappedModel != "vendor-opus-4.8" || logEntry.ModelMappingPattern != "claude-opus-*" || logEntry.ModelMappingTarget != "vendor-opus-*" || logEntry.ModelOverride != "forced-opus-model" {
		t.Fatalf("持久化模型路由详情错误: %#v", logEntry)
	}
	if logEntry.ReasoningEffort != "high" || logEntry.ReasoningEffortSource != reasoningEffortSourceModelMapping {
		t.Fatalf("持久化思考强度错误: %#v", logEntry)
	}
}

func TestForwardRequestPersistsStreamDiagnostics(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	previousLogQueue := GlobalDBQueueLogs
	logQueue := NewDBWriteQueue(db, 32, true)
	GlobalDBQueueLogs = logQueue
	t.Cleanup(func() {
		_ = logQueue.Shutdown(2 * time.Second)
		GlobalDBQueueLogs = previousLogQueue
	})

	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"context_compaction\",\"encrypted_content\":\"opaque\"}}\n\n"))
		_, _ = writer.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"))
	}))
	defer upstream.Close()

	provider := Provider{
		ID:      89,
		Name:    "Stream Diagnostics",
		APIURL:  upstream.URL,
		APIKey:  "test-key",
		Enabled: true,
	}
	bodyBytes := []byte(`{"model":"gpt-5.3-codex","stream":true,"input":[{"type":"context_compaction"}]}`)
	plan := providerRequestPlan{
		OriginalBodyBytes: bodyBytes,
		BodyBytes:         bodyBytes,
		EffectiveModel:    "gpt-5.3-codex",
		EffectiveEndpoint: "/responses",
	}
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	request := httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(bodyBytes))
	response := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(response)
	ginContext.Request = request
	ok, err := relay.forwardRequestWithPlan(
		ginContext,
		"codex",
		provider,
		plan.EffectiveEndpoint,
		map[string]string{},
		map[string]string{"Content-Type": "application/json"},
		plan.BodyBytes,
		true,
		plan.EffectiveModel,
		"gpt-5.3-codex",
		plan,
		false,
	)
	if err != nil || !ok {
		t.Fatalf("转发请求失败: ok=%v err=%v", ok, err)
	}

	page, err := NewLogService(nil).ListRequestLogsPageV2("codex", providerRefFromProvider(provider), "", 10, 0, "", "")
	if err != nil {
		t.Fatalf("读取请求日志失败: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("请求日志数量 = %d, 期望 1", len(page.Items))
	}
	logEntry := page.Items[0]
	if logEntry.StreamLastEvent != "response.completed" || logEntry.StreamTerminalEvent != "response.completed" || logEntry.StreamErrorKind != "" {
		t.Fatalf("持久化流生命周期错误: %#v", logEntry)
	}
	if !logEntry.StreamCompactionRequested || !logEntry.StreamCompactionObserved || logEntry.StreamBytes == 0 || logEntry.UpstreamProtocol == "" {
		t.Fatalf("持久化流诊断错误: %#v", logEntry)
	}
}

func TestGeminiProxyPersistsErrorMessage(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}
	if _, err := db.Exec(`UPDATE app_settings SET value = 'false' WHERE key = 'enable_blacklist'`); err != nil {
		t.Fatalf("关闭黑名单失败: %v", err)
	}

	previousWriteQueue := GlobalDBQueue
	previousLogQueue := GlobalDBQueueLogs
	writeQueue := NewDBWriteQueue(db, 32, false)
	logQueue := NewDBWriteQueue(db, 32, true)
	GlobalDBQueue = writeQueue
	GlobalDBQueueLogs = logQueue
	t.Cleanup(func() {
		_ = writeQueue.Shutdown(2 * time.Second)
		_ = logQueue.Shutdown(2 * time.Second)
		GlobalDBQueue = previousWriteQueue
		GlobalDBQueueLogs = previousLogQueue
	})

	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte(`{"error":{"message":"gemini upstream unavailable"}}`))
	}))
	defer upstream.Close()

	geminiService := NewGeminiService("")
	provider := GeminiProvider{
		ID:      "gemini-error-log",
		Name:    "Gemini Error Log",
		BaseURL: upstream.URL,
		APIKey:  "test-key",
		Model:   "gemini-test",
		Enabled: true,
		Level:   1,
	}
	if err := geminiService.AddProvider(provider); err != nil {
		t.Fatalf("保存 Gemini provider 失败: %v", err)
	}
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	relay := NewProviderRelayService(NewProviderService(), geminiService, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	request := httptest.NewRequest(
		http.MethodPost,
		"/gemini/v1beta/models/gemini-test:generateContent",
		strings.NewReader(`{"contents":[{"parts":[{"text":"hello"}]}]}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("Gemini 上游错误状态码=%d body=%s", response.Code, response.Body.String())
	}
	page, err := NewLogService(nil).ListRequestLogsPageV2("gemini", provider.ID, "", 10, 0, "", "")
	if err != nil {
		t.Fatalf("读取 Gemini 请求日志失败: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("Gemini 请求日志数量=%d，期望 1", len(page.Items))
	}
	logEntry := page.Items[0]
	if logEntry.HttpCode != http.StatusServiceUnavailable || !strings.Contains(logEntry.ErrorMessage, "status 503") {
		t.Fatalf("Gemini 错误诊断未正确落库: %#v", logEntry)
	}
}

func TestGeminiProxyPersistsFailedAttemptBeforeFallbackSuccess(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}
	if _, err := db.Exec(`UPDATE app_settings SET value = 'false' WHERE key = 'enable_blacklist'`); err != nil {
		t.Fatalf("关闭黑名单失败: %v", err)
	}

	previousWriteQueue := GlobalDBQueue
	previousLogQueue := GlobalDBQueueLogs
	writeQueue := NewDBWriteQueue(db, 32, false)
	logQueue := NewDBWriteQueue(db, 32, true)
	GlobalDBQueue = writeQueue
	GlobalDBQueueLogs = logQueue
	t.Cleanup(func() {
		_ = writeQueue.Shutdown(2 * time.Second)
		_ = logQueue.Shutdown(2 * time.Second)
		GlobalDBQueue = previousWriteQueue
		GlobalDBQueueLogs = previousLogQueue
	})

	failedUpstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte(`{"error":{"message":"first provider unavailable"}}`))
	}))
	defer failedUpstream.Close()
	successUpstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer successUpstream.Close()

	geminiService := NewGeminiService("")
	failedProvider := GeminiProvider{ID: "gemini-failed", Name: "Gemini Failed", BaseURL: failedUpstream.URL, APIKey: "test-key-1", Model: "gemini-test", Enabled: true, Level: 1}
	successProvider := GeminiProvider{ID: "gemini-success", Name: "Gemini Success", BaseURL: successUpstream.URL, APIKey: "test-key-2", Model: "gemini-test", Enabled: true, Level: 2}
	for _, provider := range []GeminiProvider{failedProvider, successProvider} {
		if err := geminiService.AddProvider(provider); err != nil {
			t.Fatalf("保存 Gemini provider 失败: %v", err)
		}
	}

	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	relay := NewProviderRelayService(NewProviderService(), geminiService, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)
	request := httptest.NewRequest(
		http.MethodPost,
		"/gemini/v1beta/models/gemini-test:generateContent",
		strings.NewReader(`{"contents":[{"parts":[{"text":"hello"}]}]}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("Gemini 降级请求状态码=%d body=%s", response.Code, response.Body.String())
	}

	failedPage, err := NewLogService(nil).ListRequestLogsPageV2("gemini", failedProvider.ID, "", 10, 0, "", "")
	if err != nil {
		t.Fatalf("读取失败供应商日志失败: %v", err)
	}
	if len(failedPage.Items) != 1 || failedPage.Items[0].HttpCode != http.StatusServiceUnavailable || !strings.Contains(failedPage.Items[0].ErrorMessage, "first provider unavailable") {
		t.Fatalf("失败供应商日志未保留: %#v", failedPage.Items)
	}
	successPage, err := NewLogService(nil).ListRequestLogsPageV2("gemini", successProvider.ID, "", 10, 0, "", "")
	if err != nil {
		t.Fatalf("读取成功供应商日志失败: %v", err)
	}
	if len(successPage.Items) != 1 || successPage.Items[0].HttpCode != http.StatusOK || successPage.Items[0].ErrorMessage != "" {
		t.Fatalf("成功供应商日志错误: %#v", successPage.Items)
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

func TestClaudeRoutedModelSupportTrustsExplicitMapping(t *testing.T) {
	provider := Provider{
		SupportedModels: map[string]bool{
			"kimi-for-coding": true,
		},
		ModelMapping: map[string]string{
			"claude-opus-*": "kimi-k2.7",
		},
		RequestBodyOverrides: map[string]interface{}{
			"model": "forced-provider-model",
		},
	}
	if !provider.isClaudeRoutedModelSupported("claude-opus-4.8", "forced-provider-model") {
		t.Fatal("Claude 映射命中后应信任最终模型，包括请求体强制覆盖")
	}
	if provider.isClaudeRoutedModelSupported("claude-sonnet-4.8", "forced-provider-model") {
		t.Fatal("未命中映射时不应绕过最终模型白名单")
	}
	if provider.IsResolvedModelSupported("claude-opus-4.8", "forced-provider-model") {
		t.Fatal("通用模型校验必须保持严格，避免影响 Codex 和自定义 CLI")
	}
}

func TestClaudeProxyTrustsExplicitMappingEndToEnd(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var upstreamCalls int32
	var upstreamModel string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		body, _ := io.ReadAll(r.Body)
		upstreamModel = gjson.GetBytes(body, "model").String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_test","type":"message","role":"assistant","model":"kimi-k2.7","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	appSettings := NewAppSettingsService(nil)
	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ClaudeModelRoutingEnabled = true
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatal(err)
	}

	providerService := NewProviderService()
	provider := Provider{
		ID:      1,
		Name:    "Trusted Mapping",
		APIURL:  upstream.URL,
		APIKey:  "test-key",
		Enabled: true,
		Level:   1,
		SupportedModels: map[string]bool{
			"kimi-for-coding": true,
		},
		ModelMapping: map[string]string{
			"claude-opus-*": "kimi-k2.7",
		},
	}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存供应商失败: %v", err)
	}

	blacklistService := NewBlacklistService(NewSettingsService(), nil)
	routing := NewClaudeModelRoutingService(providerService, appSettings, nil)
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, appSettings, nil, "")
	relay.BindClaudeModelRoutingService(routing)
	router := gin.New()
	relay.registerRoutes(router)

	requestBody := `{"model":"claude-opus-4.8","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	if atomic.LoadInt32(&upstreamCalls) != 1 {
		t.Fatalf("上游调用次数 = %d，期望 1", upstreamCalls)
	}
	if upstreamModel != "kimi-k2.7" {
		t.Fatalf("上游模型 = %q，期望 kimi-k2.7", upstreamModel)
	}
}

func TestClaudeProxyNeverPassesManagedSubagentAliasThrough(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	provider := Provider{
		ID:                     1,
		Name:                   "Passthrough Only",
		APIURL:                 upstream.URL,
		APIKey:                 "test-key",
		Enabled:                true,
		Level:                  1,
		ModelMapping:           map[string]string{"code-switch-*": "vendor-*"},
		ModelMappingMissPolicy: ModelMappingMissPolicyPassthrough,
	}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatal(err)
	}

	blacklistService := NewBlacklistService(NewSettingsService(), nil)
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	body := fmt.Sprintf(`{"model":%q,"max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`, claudeManagedSubagentModel)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码=%d body=%s，期望内部别名被拦截", w.Code, w.Body.String())
	}
	if atomic.LoadInt32(&upstreamCalls) != 0 {
		t.Fatalf("内部 Subagent 别名不应到达上游，调用次数=%d", upstreamCalls)
	}
}

func newClaudeRoutingTestRouter(t *testing.T, providers []Provider) (*gin.Engine, *AppSettingsService, *ProviderRelayService) {
	t.Helper()
	providerService := NewProviderService()
	if err := providerService.SaveProviders("claude", providers); err != nil {
		t.Fatalf("保存 Claude 测试供应商失败: %v", err)
	}

	appSettings := NewAppSettingsService(nil)
	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatalf("读取测试应用设置失败: %v", err)
	}
	settings.ClaudeModelRoutingEnabled = true
	settings.ClaudeModelAggregationEnabled = true
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatalf("开启 Claude 模型路由失败: %v", err)
	}

	blacklistService := NewBlacklistService(NewSettingsService(), nil)
	routing := NewClaudeModelRoutingService(providerService, appSettings, nil)
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, appSettings, nil, "")
	relay.BindClaudeModelRoutingService(routing)
	router := gin.New()
	relay.registerRoutes(router)
	return router, appSettings, relay
}

func sendClaudeRoutingTestRequest(t *testing.T, router http.Handler, model string, sessionID string) {
	t.Helper()
	metadata := ""
	if sessionID != "" {
		metadata = fmt.Sprintf(`,"metadata":{"session_id":%q}`, sessionID)
	}
	body := fmt.Sprintf(`{"model":%q,"max_tokens":16,"messages":[{"role":"user","content":"hi"}]%s}`, model, metadata)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Claude 测试请求状态码=%d body=%s", w.Code, w.Body.String())
	}
}

func TestClaudeSubagentFollowsSessionProviderAcrossLevelsAndHonorsRoutingSession(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callsMu sync.Mutex
	calls := make([]string, 0, 4)
	newUpstream := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			model := gjson.GetBytes(body, "model").String()
			callsMu.Lock()
			calls = append(calls, name+":"+model)
			callsMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"msg_test","type":"message","role":"assistant","model":"vendor-model","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
		}))
	}
	aUpstream := newUpstream("A")
	bUpstream := newUpstream("B")
	defer aUpstream.Close()
	defer bUpstream.Close()

	providers := []Provider{
		{
			ID:                     1,
			Name:                   "A",
			APIURL:                 aUpstream.URL,
			APIKey:                 "a-key",
			Enabled:                true,
			Level:                  1,
			ModelMapping:           map[string]string{claudeManagedSubagentModel: "a-subagent"},
			ModelMappingMissPolicy: ModelMappingMissPolicyBlock,
		},
		{
			ID:      2,
			Name:    "B",
			APIURL:  bUpstream.URL,
			APIKey:  "b-key",
			Enabled: true,
			Level:   5,
			ModelMapping: map[string]string{
				"claude-main-test":         "b-main",
				claudeManagedSubagentModel: "b-subagent",
			},
			ModelMappingMissPolicy: ModelMappingMissPolicyBlock,
		},
	}
	router, appSettings, _ := newClaudeRoutingTestRouter(t, providers)

	sendClaudeRoutingTestRequest(t, router, "claude-main-test", "session-preferred")
	sendClaudeRoutingTestRequest(t, router, claudeManagedSubagentModel, "session-preferred")
	sendClaudeRoutingTestRequest(t, router, claudeManagedSubagentModel, "")

	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatalf("读取 Claude 路由设置失败: %v", err)
	}
	settings.ClaudeModelRoutingEnabled = false
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatalf("关闭 Claude 路由失败: %v", err)
	}
	sendClaudeRoutingTestRequest(t, router, claudeManagedSubagentModel, "session-preferred")

	callsMu.Lock()
	got := append([]string(nil), calls...)
	callsMu.Unlock()
	want := []string{
		"B:b-main",
		"B:b-subagent",
		"A:a-subagent",
		"A:a-subagent",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Claude 供应商调用顺序=%v，期望=%v", got, want)
	}
}

func TestClaudeSubagentFallbackPreservesRoundRobinOrder(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callsMu sync.Mutex
	calls := make([]string, 0, 3)
	var failB int32
	newUpstream := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			model := gjson.GetBytes(body, "model").String()
			callsMu.Lock()
			calls = append(calls, name+":"+model)
			callsMu.Unlock()
			if name == "B" && model == "b-subagent" && atomic.LoadInt32(&failB) == 1 {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`{"error":"test failure"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"msg_test","type":"message","role":"assistant","model":"vendor-model","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
		}))
	}
	aUpstream := newUpstream("A")
	bUpstream := newUpstream("B")
	cUpstream := newUpstream("C")
	defer aUpstream.Close()
	defer bUpstream.Close()
	defer cUpstream.Close()

	providers := []Provider{
		{
			ID:                     1,
			Name:                   "A",
			APIURL:                 aUpstream.URL,
			APIKey:                 "a-key",
			Enabled:                true,
			Level:                  1,
			ModelMapping:           map[string]string{claudeManagedSubagentModel: "a-subagent"},
			ModelMappingMissPolicy: ModelMappingMissPolicyBlock,
		},
		{
			ID:      2,
			Name:    "B",
			APIURL:  bUpstream.URL,
			APIKey:  "b-key",
			Enabled: true,
			Level:   1,
			ModelMapping: map[string]string{
				"claude-main-test":         "b-main",
				claudeManagedSubagentModel: "b-subagent",
			},
			ModelMappingMissPolicy: ModelMappingMissPolicyBlock,
		},
		{
			ID:                     3,
			Name:                   "C",
			APIURL:                 cUpstream.URL,
			APIKey:                 "c-key",
			Enabled:                true,
			Level:                  1,
			ModelMapping:           map[string]string{claudeManagedSubagentModel: "c-subagent"},
			ModelMappingMissPolicy: ModelMappingMissPolicyBlock,
		},
	}
	router, appSettings, relay := newClaudeRoutingTestRouter(t, providers)
	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatalf("读取 Claude 路由设置失败: %v", err)
	}
	settings.EnableRoundRobin = true
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatalf("开启 Claude 轮询失败: %v", err)
	}

	sendClaudeRoutingTestRequest(t, router, "claude-main-test", "session-round-robin")
	relay.markRoundRobinProviderAttempt("claude", providers[1])
	// 预置历史轮询起点为首选 B，验证 B 失败后应从 C 继续，而不是因 B 被移出分组而回到 A。
	sendClaudeRoutingTestRequest(t, router, claudeManagedSubagentModel, "session-round-robin")
	atomic.StoreInt32(&failB, 1)
	callsMu.Lock()
	calls = calls[:0]
	callsMu.Unlock()

	sendClaudeRoutingTestRequest(t, router, claudeManagedSubagentModel, "session-round-robin")
	callsMu.Lock()
	got := append([]string(nil), calls...)
	callsMu.Unlock()
	want := []string{"B:b-subagent", "C:c-subagent"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Subagent 降级顺序=%v，期望=%v", got, want)
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

func TestApplyGeminiRequestLogReasoningTracksOverrideSource(t *testing.T) {
	originalBody := []byte(`{"generationConfig":{"thinkingConfig":{"thinkingBudget":1000}}}`)
	provider := GeminiProvider{
		RequestBodyOverrides: map[string]interface{}{
			"generationConfig": map[string]interface{}{
				"thinkingConfig": map[string]interface{}{
					"thinkingBudget": 20000,
				},
			},
		},
	}
	currentBody, err := buildGeminiRequestBody(originalBody, provider)
	if err != nil {
		t.Fatal(err)
	}
	requestLog := &ReqeustLog{RequestedModel: "gemini-2.5-pro"}
	applyGeminiRequestLogReasoning(requestLog, originalBody, currentBody, provider.RequestBodyOverrides)

	if requestLog.ReasoningEffort != "high" {
		t.Fatalf("ReasoningEffort=%q，期望 high", requestLog.ReasoningEffort)
	}
	if requestLog.ReasoningEffortSource != reasoningEffortSourceRequestBodyOverride {
		t.Fatalf("ReasoningEffortSource=%q，期望 %q", requestLog.ReasoningEffortSource, reasoningEffortSourceRequestBodyOverride)
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
		{
			name: "generic id does not override parent session",
			body: `{"id":"request-a","parent_session_id":"session-parent"}`,
			want: shortSHA256Hex("codex.session_id=session-parent"),
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

func TestDeriveCodexSessionIdentityFromTurnMetadata(t *testing.T) {
	root := deriveRelaySessionIdentityWithHeaders("codex", []byte(`{"model":"gpt-5"}`), map[string]string{
		"X-Codex-Turn-Metadata": `{"session_id":"session-root","thread_id":"thread-root"}`,
	})
	if root.NodeHash != shortSHA256Hex("codex.thread_id=thread-root") || root.Role != "root" {
		t.Fatalf("Codex 根会话头解析错误: %#v", root)
	}

	child := deriveRelaySessionIdentityWithHeaders("codex", []byte(`{"model":"gpt-5"}`), map[string]string{
		"X-Codex-Turn-Metadata":    `{"session_id":"session-child","thread_id":"thread-child","parent_thread_id":"metadata-parent"}`,
		"X-Codex-Parent-Thread-Id": "thread-root",
	})
	if child.NodeHash != shortSHA256Hex("codex.thread_id=thread-child") {
		t.Fatalf("Codex 子会话 thread hash 错误: %#v", child)
	}
	if child.ParentHash != root.NodeHash || child.RootHash != root.NodeHash || child.Role != "child" {
		t.Fatalf("Codex 父子会话关系错误: root=%#v child=%#v", root, child)
	}
}

func TestDeriveCodexSessionIdentityCanonicalizesBodyThreadRelation(t *testing.T) {
	root := deriveRelaySessionIdentity("codex", []byte(`{"metadata":{"thread_id":"thread-root"}}`))
	child := deriveRelaySessionIdentity("codex", []byte(`{"metadata":{"thread_id":"thread-child","parent_thread_id":"thread-root"}}`))
	if root.NodeHash == "" || child.NodeHash == "" || child.ParentHash != root.NodeHash || child.Role != "child" {
		t.Fatalf("Codex 请求体父子会话关系错误: root=%#v child=%#v", root, child)
	}
}

func TestDeriveCodexSessionIdentitySupportsRootAndParentFields(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "root thread",
			body: `{"root_thread_id":"thread-root"}`,
			want: shortSHA256Hex("codex.thread_id=thread-root"),
		},
		{
			name: "parent conversation",
			body: `{"parent_conversation_id":"conversation-parent"}`,
			want: shortSHA256Hex("codex.conversation_id=conversation-parent"),
		},
		{
			name: "parent session",
			body: `{"parent_session_id":"session-parent"}`,
			want: shortSHA256Hex("codex.session_id=session-parent"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity := deriveRelaySessionIdentity("codex", []byte(tt.body))
			if identity.IdentitySource != sessionIdentitySourceCodexExplicit || identity.NodeHash != tt.want {
				t.Fatalf("Codex 显式会话归一错误: %#v", identity)
			}
			if identity.ParentHash != "" || identity.RootHash != identity.NodeHash || identity.Role != "root" {
				t.Fatalf("Codex 仅父级或根级标识不应形成自引用: %#v", identity)
			}
		})
	}
}

func TestDeriveCodexSessionIdentityUsesRootAsParentFallback(t *testing.T) {
	identity := deriveRelaySessionIdentity("codex", []byte(`{"session_id":"session-child","root_session_id":"session-root"}`))
	rootHash := shortSHA256Hex("codex.session_id=session-root")
	if identity.NodeHash != shortSHA256Hex("codex.session_id=session-child") || identity.ParentHash != rootHash || identity.RootHash != rootHash || identity.Role != "child" {
		t.Fatalf("Codex 显式根会话关系错误: %#v", identity)
	}
}

func TestDeriveCodexSessionIdentitySupportsHeaderOnlyParentOrRoot(t *testing.T) {
	parent := deriveRelaySessionIdentityWithHeaders("codex", []byte(`{"model":"gpt-5"}`), map[string]string{
		"X-Codex-Turn-Metadata": `{"parent_thread_id":"thread-parent"}`,
	})
	if parent.IdentitySource != sessionIdentitySourceCodexExplicit || parent.NodeHash != shortSHA256Hex("codex.thread_id=thread-parent") || parent.Role != "root" {
		t.Fatalf("Codex 仅父线程头归一错误: %#v", parent)
	}

	root := deriveRelaySessionIdentityWithHeaders("codex", []byte(`{"model":"gpt-5"}`), map[string]string{
		"X-Codex-Turn-Metadata": `{"root_thread_id":"thread-root"}`,
	})
	if root.IdentitySource != sessionIdentitySourceCodexExplicit || root.NodeHash != shortSHA256Hex("codex.thread_id=thread-root") || root.RootHash != root.NodeHash {
		t.Fatalf("Codex 仅根线程头归一错误: %#v", root)
	}
}

func TestDeriveCodexSessionIdentityPreservesBodyParentWhenHeaderProvidesNode(t *testing.T) {
	identity := deriveRelaySessionIdentityWithHeaders("codex", []byte(`{"parent_session_id":"session-parent"}`), map[string]string{
		"X-Codex-Session-Id": "session-child",
	})
	if identity.NodeHash != shortSHA256Hex("codex.session_id=session-child") {
		t.Fatalf("Codex 请求头节点归一错误: %#v", identity)
	}
	if identity.ParentHash != shortSHA256Hex("codex.session_id=session-parent") || identity.RootHash != identity.ParentHash || identity.Role != "child" {
		t.Fatalf("Codex 请求体父关系丢失: %#v", identity)
	}
}

func TestDeriveCodexSessionIdentityPreservesBodyRootWhenHeaderProvidesNode(t *testing.T) {
	identity := deriveRelaySessionIdentityWithHeaders("codex", []byte(`{"root_session_id":"session-root"}`), map[string]string{
		"X-Codex-Session-Id": "session-child",
	})
	rootHash := shortSHA256Hex("codex.session_id=session-root")
	if identity.NodeHash != shortSHA256Hex("codex.session_id=session-child") || identity.ParentHash != rootHash || identity.RootHash != rootHash || identity.Role != "child" {
		t.Fatalf("Codex 请求体根关系丢失: %#v", identity)
	}
}

func TestDeriveCodexSessionIdentityFromClientMetadata(t *testing.T) {
	body := []byte(`{"model":"gpt-5","client_metadata":{"thread_id":"thread-child","x-codex-parent-thread-id":"thread-root","x-codex-turn-metadata":"{\"session_id\":\"session-child\",\"thread_id\":\"thread-child\"}"}}`)
	identity := deriveRelaySessionIdentityWithHeaders("codex", body, nil)
	if identity.NodeHash != shortSHA256Hex("codex.thread_id=thread-child") {
		t.Fatalf("client_metadata thread hash 错误: %#v", identity)
	}
	if identity.ParentHash != shortSHA256Hex("codex.thread_id=thread-root") || identity.Role != "child" {
		t.Fatalf("client_metadata 父子关系错误: %#v", identity)
	}
}

func TestDeriveCodexSessionIdentityPrefersCursorConversation(t *testing.T) {
	identity := deriveRelaySessionIdentityWithHeaders("codex", []byte(`{"metadata":{"cursorConversationId":"cursor-body","thread_id":"thread-body"}}`), map[string]string{
		"X-Cursor-Conversation-Id": "cursor-header",
	})
	if identity.IdentitySource != sessionIdentitySourceCursorConversation {
		t.Fatalf("Cursor 会话来源错误: %#v", identity)
	}
	if identity.NodeHash != shortSHA256Hex("codex.cursor_conversation_id=cursor-header") {
		t.Fatalf("Cursor 会话 hash 错误: %#v", identity)
	}

	bodyIdentity := deriveRelaySessionIdentityWithHeaders("codex", []byte(`{"cursorConversationId":"cursor-body"}`), nil)
	if bodyIdentity.IdentitySource != sessionIdentitySourceCursorConversation || bodyIdentity.NodeHash != shortSHA256Hex("codex.cursor_conversation_id=cursor-body") {
		t.Fatalf("Cursor 顶层会话标识归一错误: %#v", bodyIdentity)
	}
}

func TestDeriveCodexSessionIdentityUsesPromptCacheKeyOnlyForResponsesInput(t *testing.T) {
	responses := deriveRelaySessionIdentity("codex", []byte(`{"model":"gpt-5.6-sol","input":[],"prompt_cache_key":"cache-a"}`))
	if responses.IdentitySource != sessionIdentitySourcePromptCacheKey || responses.NodeHash != shortSHA256Hex("codex.prompt_cache_key=cache-a") {
		t.Fatalf("Responses prompt_cache_key 来源错误: %#v", responses)
	}
	stringInput := deriveRelaySessionIdentity("codex", []byte(`{"model":"gpt-5.6-sol","input":"hello","prompt_cache_key":"cache-a"}`))
	if stringInput.IdentitySource != sessionIdentitySourcePromptCacheKey || stringInput.NodeHash != responses.NodeHash {
		t.Fatalf("字符串 input 的 Responses 请求来源错误: %#v", stringInput)
	}

	chat := deriveRelaySessionIdentity("codex", []byte(`{"model":"gpt-5.6-sol","messages":[],"prompt_cache_key":"cache-a"}`))
	if chat.IdentitySource == sessionIdentitySourcePromptCacheKey || chat.NodeHash != "" {
		t.Fatalf("非 Responses 请求不应使用 prompt_cache_key: %#v", chat)
	}

	explicit := deriveRelaySessionIdentity("codex", []byte(`{"input":[],"prompt_cache_key":"cache-a","root_thread_id":"thread-root"}`))
	if explicit.IdentitySource != sessionIdentitySourceCodexExplicit || explicit.NodeHash != shortSHA256Hex("codex.thread_id=thread-root") {
		t.Fatalf("Codex 显式线程应覆盖 prompt_cache_key: %#v", explicit)
	}
}

func TestDecorateSessionConcurrencyMetaKeepsIdentitySourceWhenAffinityDisabled(t *testing.T) {
	appSettings := &AppSettingsService{path: t.TempDir() + "/settings.json"}
	relay := NewProviderRelayService(nil, nil, nil, nil, appSettings, nil, "")
	meta := providerConcurrencyRequestMeta{}
	relay.decorateSessionConcurrencyMeta("codex", []byte(`{"input":[],"prompt_cache_key":"cache-a"}`), nil, &meta)
	if meta.SessionIdentitySource != sessionIdentitySourcePromptCacheKey {
		t.Fatalf("关闭粘滞时仍应记录身份来源: %#v", meta)
	}
}

func TestSessionIdentityUsesOriginalProviderRequestBody(t *testing.T) {
	original := []byte(`{"input":[],"prompt_cache_key":"cache-a"}`)
	modified := []byte(`{"input":[],"prompt_cache_key":"cache-b"}`)
	plan := providerRequestPlan{OriginalBodyBytes: original, BodyBytes: modified}
	if got := sessionIdentityBodyBytes(plan, modified); !bytes.Equal(got, original) {
		t.Fatalf("会话身份应使用原始请求体: got=%s want=%s", got, original)
	}
	if got := sessionIdentityBodyBytes(providerRequestPlan{}, modified); !bytes.Equal(got, modified) {
		t.Fatalf("缺少原始请求体时应回退当前请求体: got=%s want=%s", got, modified)
	}
}

func TestDecorateCodexConcurrencyMetaFromTurnMetadataHeader(t *testing.T) {
	appSettings := &AppSettingsService{path: t.TempDir() + "/settings.json"}
	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatalf("读取应用设置失败: %v", err)
	}
	settings.SessionAffinityEnabled["codex"] = true
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatalf("保存应用设置失败: %v", err)
	}

	relay := NewProviderRelayService(nil, nil, nil, nil, appSettings, nil, "")
	headers := map[string]string{
		"X-Codex-Turn-Metadata": `{"session_id":"session-root","thread_id":"thread-root"}`,
	}
	identity := deriveRelaySessionIdentityWithHeaders("codex", []byte(`{"model":"gpt-5"}`), headers)
	relay.rememberSessionRelation("codex", identity)
	attemptID := relay.beginSessionProviderRequest("codex", identity.NodeHash, "provider-a", "Provider A", "codex-tui/0.147.0", 5, 30, true, false, false)
	relay.finishSessionProviderRequest("codex", identity.NodeHash)
	relay.confirmSessionProviderBinding("codex", identity.NodeHash, attemptID)

	meta := providerConcurrencyRequestMeta{}
	relay.decorateSessionConcurrencyMeta("codex", []byte(`{"model":"gpt-5"}`), headers, &meta)
	if meta.SessionNumber <= 0 || meta.RootSessionNumber != meta.SessionNumber || meta.SessionRole != "root" || !meta.SessionSwitchable {
		t.Fatalf("Codex 活跃连接会话元数据错误: %#v", meta)
	}
	collector := relay.newSessionAffinityToolResponseCollector("codex", Provider{ID: 1, Name: "Provider A"}, providerRequestPlan{
		OriginalBodyBytes: []byte(`{"model":"gpt-5","input":[]}`),
	}, "codex-tui/0.147.0", headers)
	if collector != nil {
		t.Fatalf("请求头已有 Codex 会话身份时不应创建额外工具会话")
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
	relay := NewProviderRelayService(nil, nil, nil, nil, appSettings, nil, "")
	plan := providerRequestPlan{
		OriginalBodyBytes: []byte(`{"metadata":{"session_id":"session-a"}}`),
		BodyBytes:         []byte(`{"input":[]}`),
	}

	collector := relay.newSessionAffinityToolResponseCollector("claude", Provider{ID: 1, Name: "Provider A"}, plan, "claude-cli/1.0.0", nil)
	if collector != nil {
		t.Fatalf("原始请求已有 sessionHash 时不应创建响应侧工具会话 collector")
	}
}

func TestSessionAffinityStatusesIncludeUserAgent(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	sessionHash := "session-user-agent"
	userAgent := "claude-cli/2.1.84 (external, cli)"

	attemptID := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", userAgent, 5, 5, true, false, false)
	if attemptID <= 0 {
		t.Fatalf("应创建会话绑定 attempt，got %d", attemptID)
	}
	relay.finishSessionProviderRequest(platform, sessionHash)
	relay.confirmSessionProviderBinding(platform, sessionHash, attemptID)

	statuses := relay.GetSessionAffinityStatuses(platform)
	if len(statuses) != 1 || len(statuses[0].Sessions) != 1 {
		t.Fatalf("会话状态数量不匹配: %#v", statuses)
	}
	if got := statuses[0].Sessions[0].UserAgent; got != userAgent {
		t.Fatalf("user agent = %q，期望 %q", got, userAgent)
	}
}

func TestExtractResponseToolCallIDsIgnoresPlainResponseID(t *testing.T) {
	got := extractResponseToolCallIDs(`{"id":"resp_abc","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`)
	if len(got) != 0 {
		t.Fatalf("普通响应 id 不应被当作工具调用会话，got %#v", got)
	}
}

func TestRelaySessionIdentityCanonicalizesParentAndSubagentHashes(t *testing.T) {
	parent := deriveRelaySessionIdentity("claude", []byte(`{"metadata":{"session_id":"root-session"}}`))
	child := deriveRelaySessionIdentity("claude", []byte(`{"metadata":{"session_id":"child-session","parent_session_id":"root-session"}}`))
	if parent.NodeHash == "" || child.NodeHash == "" {
		t.Fatalf("父子会话 hash 不应为空: parent=%q child=%q", parent.NodeHash, child.NodeHash)
	}
	if child.ParentHash != parent.NodeHash {
		t.Fatalf("parent_session_id 应匹配 session_id: parent=%q childParent=%q", parent.NodeHash, child.ParentHash)
	}
	if child.Role != "child" {
		t.Fatalf("子会话 role = %q，期望 child", child.Role)
	}

	mainIdentity := deriveRelaySessionIdentity("claude", []byte(`{"model":"claude-main","metadata":{"session_id":"root-session"}}`))
	subagent := deriveRelaySessionIdentity("claude", []byte(`{"model":"code-switch-r-subagent","metadata":{"session_id":"root-session","agent_id":"agent-a"}}`))
	if subagent.NodeHash == mainIdentity.NodeHash || subagent.ParentHash != mainIdentity.NodeHash || subagent.Role != "child" {
		t.Fatalf("带 agent_id 的子代理身份错误: main=%#v subagent=%#v", mainIdentity, subagent)
	}
	anonymousSubagent := deriveRelaySessionIdentity("claude", []byte(`{"model":"code-switch-r-subagent","metadata":{"session_id":"root-session"}}`))
	if anonymousSubagent.NodeHash == mainIdentity.NodeHash || anonymousSubagent.ParentHash != mainIdentity.NodeHash || anonymousSubagent.Role != "child" {
		t.Fatalf("无独立 ID 的子代理应继承主会话: main=%#v subagent=%#v", mainIdentity, anonymousSubagent)
	}
}

func TestSessionRelationKeepsImmediateParentForMultiLevelTree(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	root := deriveRelaySessionIdentity("claude", []byte(`{"metadata":{"session_id":"root"}}`))
	child := deriveRelaySessionIdentity("claude", []byte(`{"metadata":{"session_id":"child","parent_session_id":"root"}}`))
	grandchild := deriveRelaySessionIdentity("claude", []byte(`{"metadata":{"session_id":"grandchild","parent_session_id":"child"}}`))
	relay.rememberSessionRelation("claude", root)
	relay.rememberSessionRelation("claude", child)
	relay.rememberSessionRelation("claude", grandchild)
	relation := relay.sessionRelationLocked("claude", grandchild.NodeHash)
	if relation.ParentHash != child.NodeHash {
		t.Fatalf("多层关系应保留最近父节点: got=%q want=%q", relation.ParentHash, child.NodeHash)
	}
	if relation.RootHash != root.NodeHash {
		t.Fatalf("多层关系 root hash 错误: got=%q want=%q", relation.RootHash, root.NodeHash)
	}
}

func TestSessionAffinityToolCollectorFactoryRegistersResponseSession(t *testing.T) {
	appSettings := &AppSettingsService{path: t.TempDir() + "/settings.json"}
	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatalf("读取应用设置失败: %v", err)
	}
	settings.SessionAffinityEnabled["claude"] = true
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatalf("保存应用设置失败: %v", err)
	}
	relay := NewProviderRelayService(nil, nil, nil, nil, appSettings, nil, "")
	provider := Provider{ID: 1, Name: "Provider A", SessionMaxSessions: 5, SessionTTLMinutes: 5}
	collector := relay.newSessionAffinityToolResponseCollector("claude", provider, providerRequestPlan{
		OriginalBodyBytes: []byte(`{"model":"claude","messages":[{"role":"user","content":"run"}]}`),
	}, "claude-cli/1.0", nil)
	if collector == nil {
		t.Fatal("Sticky 开启且请求无稳定会话时应创建 Collector")
	}
	collector.observePayload(`{"content":[{"type":"tool_use","id":"toolu_1"}]}`)
	collector.commit()
	hash := toolSessionHash("claude", []string{"toolu_1"})
	binding := relay.sessionAffinity[sessionAffinityStateKey("claude", hash)]
	if binding == nil || !binding.Confirmed || binding.ProviderID != "1" {
		t.Fatalf("真实 Collector 应注册响应工具会话: %#v", binding)
	}
}

func TestSessionAffinityConcurrentMigrationKeepsFallbackAvailable(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	sessionHash := "session-concurrent-migration"
	first := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", "", 5, 5, true, false, false)
	relay.confirmSessionProviderBinding(platform, sessionHash, first)
	second := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", "", 5, 5, false, false, false)
	if second != 0 {
		t.Fatalf("同 provider 并发请求应复用 binding，got %d", second)
	}
	original := relay.getSessionBindingSnapshot(platform, sessionHash)
	fallback := relay.beginSessionProviderRequest(platform, sessionHash, "provider-b", "Provider B", "", 5, 5, false, false, false)
	if fallback <= 0 {
		t.Fatalf("存在其他活跃请求时应允许后备 provider 迁移，got %d", fallback)
	}
	relay.finishSessionProviderRequest(platform, sessionHash, "provider-b")
	relay.restoreOrReleaseSessionBinding(platform, sessionHash, original, fallback)
	binding := relay.sessionAffinity[sessionAffinityStateKey(platform, sessionHash)]
	if binding == nil || binding.ProviderID != "provider-a" || binding.ActiveRequests != 2 || binding.ActiveRequestsByProvider["provider-a"] != 2 {
		t.Fatalf("后备失败后应恢复原 provider 且保留活跃计数: %#v", binding)
	}
}

func TestSessionAffinityRevisionProtectsDescendantFailure(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	rootHash := "root-revision"
	childHash := "child-revision"
	relay.sessionRelations[sessionAffinityStateKey(platform, childHash)] = sessionRelation{ParentHash: rootHash, RootHash: rootHash, Role: "child", LastSeen: time.Now()}
	relay.sessionAffinity[sessionAffinityStateKey(platform, rootHash)] = &providerSessionBinding{Platform: platform, SessionHash: rootHash, RootHash: rootHash, ProviderID: "provider-a", ProviderName: "Provider A", Confirmed: true, LastSeen: time.Now(), MaxSessions: 5, TTLMinutes: 5}
	relay.sessionAffinity[sessionAffinityStateKey(platform, childHash)] = &providerSessionBinding{Platform: platform, SessionHash: childHash, ParentHash: rootHash, RootHash: rootHash, SessionRole: "child", ProviderID: "provider-a", ProviderName: "Provider A", Confirmed: true, LastSeen: time.Now(), MaxSessions: 5, TTLMinutes: 5}
	original := relay.getSessionBindingSnapshot(platform, childHash)
	attempt := relay.beginSessionProviderRequest(platform, childHash, "provider-b", "Provider B", "", 5, 5, false, false, false)
	if attempt <= 0 {
		t.Fatalf("子会话应创建迁移 attempt，got %d", attempt)
	}
	relay.sessionAffinityMu.Lock()
	relay.propagateSessionProviderBindingLocked(platform, rootHash, "provider-c", "Provider C", 7, 9)
	relay.sessionAffinityMu.Unlock()
	relay.finishSessionProviderRequest(platform, childHash, "provider-b")
	relay.restoreOrReleaseSessionBinding(platform, childHash, original, attempt)
	binding := relay.sessionAffinity[sessionAffinityStateKey(platform, childHash)]
	if binding == nil || binding.ProviderID != "provider-c" || binding.MaxSessions != 7 || binding.TTLMinutes != 9 {
		t.Fatalf("revision 变化后旧失败不得回滚后代: %#v", binding)
	}
}

func TestReorderProvidersWithinHighestSessionLevelPutsFullProvidersLast(t *testing.T) {
	providers := []Provider{
		{ID: 1, Name: "Full", Level: 1, SessionMaxSessions: 1},
		{ID: 2, Name: "Available", Level: 1, SessionMaxSessions: 2},
	}
	loads := map[string]providerSessionLoad{
		"1": {ProviderID: "1", BoundSessions: 1},
		"2": {ProviderID: "2", BoundSessions: 0},
	}
	ordered := reorderProvidersWithinHighestSessionLevel(providers, loads)
	if providerRefFromProvider(ordered[0]) != "2" || providerRefFromProvider(ordered[1]) != "1" {
		t.Fatalf("Sticky 新会话应先选未满 provider: %#v", ordered)
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

func BenchmarkXRequestCurlPreparation(b *testing.B) {
	for _, size := range []int{4 << 10, 256 << 10, 2 << 20} {
		body := `{"model":"claude-opus-4-6","input":"` + strings.Repeat("x", size) + `"}`
		prepare := func() error {
			request, err := http.NewRequest(http.MethodPost, "https://example.com/v1/messages", strings.NewReader(body))
			if err != nil {
				return err
			}
			_, err = xrequest.GetCurlCommand(request)
			return err
		}
		b.Run(fmt.Sprintf("%dKB/serial", size>>10), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := prepare(); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("%dKB/parallel16", size>>10), func(b *testing.B) {
			b.ReportAllocs()
			jobs := make(chan struct{}, 16)
			var workers sync.WaitGroup
			workers.Add(16)
			for worker := 0; worker < 16; worker++ {
				go func() {
					defer workers.Done()
					for range jobs {
						if err := prepare(); err != nil {
							b.Error(err)
						}
					}
				}()
			}
			for i := 0; i < b.N; i++ {
				jobs <- struct{}{}
			}
			close(jobs)
			workers.Wait()
		})
	}
}

type firstWriteRecorder struct {
	*httptest.ResponseRecorder
	once       sync.Once
	firstWrite chan struct{}
}

func (writer *firstWriteRecorder) Write(data []byte) (int, error) {
	written, err := writer.ResponseRecorder.Write(data)
	writer.once.Do(func() { close(writer.firstWrite) })
	return written, err
}

func (writer *firstWriteRecorder) Flush() {
	writer.ResponseRecorder.Flush()
}

func TestForwardRelayResponseFlushesFirstSSELineImmediately(t *testing.T) {
	reader, upstreamWriter := io.Pipe()
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       reader,
	}
	writer := &firstWriteRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		firstWrite:       make(chan struct{}),
	}
	done := make(chan error, 1)
	go func() {
		_, err := forwardRelayResponse(response, writer, true)
		done <- err
	}()

	if _, err := upstreamWriter.Write([]byte("data: first\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-writer.firstWrite:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("首个 SSE 行未及时转发")
	}
	if got := writer.Body.String(); got != "data: first\n" {
		t.Fatalf("首个 SSE 行 = %q", got)
	}
	_ = upstreamWriter.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestForwardRelayResponseTreatsSingleLineStreamBodyAsJSON(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}
	writer := httptest.NewRecorder()
	if _, err := forwardRelayResponse(response, writer, true); err != nil {
		t.Fatal(err)
	}
	if got := writer.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := writer.Body.String(); got != `{"ok":true}` {
		t.Fatalf("body = %q", got)
	}
}

func TestForwardRelayResponseDoesNotCommitHeadersForEmptyStream(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
			"X-Upstream":   []string{"empty-stream"},
		},
		Body: io.NopCloser(strings.NewReader("")),
	}
	writer := httptest.NewRecorder()
	written, err := forwardRelayResponse(response, writer, true)
	if !errors.Is(err, errIncompleteStream) {
		t.Fatalf("空流错误 = %v，期望包含 errIncompleteStream", err)
	}
	if written != 0 || writer.Body.Len() != 0 {
		t.Fatalf("空流不应写入客户端，written=%d body=%q", written, writer.Body.String())
	}
	if got := writer.Header().Get("X-Upstream"); got != "" {
		t.Fatalf("空流不应提交上游响应头，X-Upstream=%q", got)
	}
}

func TestRelayPerformanceTraceAccumulatesCompatibilityAttempts(t *testing.T) {
	base := time.Now()
	requestLog := &ReqeustLog{}
	first := &relayPerformanceTrace{
		requestStartedAt:  base,
		getConnAt:         base.Add(2 * time.Millisecond),
		firstResponseByte: base.Add(12 * time.Millisecond),
		dnsDuration:       3 * time.Millisecond,
		connectDuration:   4 * time.Millisecond,
		tlsDuration:       5 * time.Millisecond,
	}
	first.apply(requestLog, time.Time{})

	secondStart := base.Add(20 * time.Millisecond)
	second := &relayPerformanceTrace{
		requestStartedAt:  secondStart,
		getConnAt:         secondStart.Add(3 * time.Millisecond),
		firstResponseByte: secondStart.Add(11 * time.Millisecond),
		connectionReused:  true,
	}
	second.apply(requestLog, secondStart.Add(12*time.Millisecond))

	if requestLog.ProxyPrepareMs != 5 || requestLog.UpstreamTTFBMs != 18 {
		t.Fatalf("兼容重试耗时未累计: prepare=%v, ttfb=%v", requestLog.ProxyPrepareMs, requestLog.UpstreamTTFBMs)
	}
	if requestLog.DNSMs != 3 || requestLog.ConnectMs != 4 || requestLog.TLSMs != 5 || requestLog.ProxyStreamDelayMs != 1 {
		t.Fatalf("网络阶段耗时异常: %#v", requestLog)
	}
	if !requestLog.ConnectionReused {
		t.Fatalf("连接复用状态应反映最后一次尝试")
	}
}

func TestRelayTimedResponseWriterRecordsWriteStart(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	blockingWriter := &blockingGinResponseWriter{
		ResponseWriter: context.Writer,
		entered:        make(chan time.Time, 1),
		release:        make(chan struct{}),
	}
	timedWriter := &relayTimedResponseWriter{ResponseWriter: blockingWriter}
	done := make(chan struct{})
	go func() {
		_, _ = timedWriter.Write([]byte("data"))
		close(done)
	}()

	<-blockingWriter.entered
	time.Sleep(20 * time.Millisecond)
	releasedAt := time.Now()
	close(blockingWriter.release)
	<-done
	if firstWrite := timedWriter.firstWriteAt(); firstWrite.IsZero() || !firstWrite.Before(releasedAt) {
		t.Fatalf("首写时间应记录调用开始而不是阻塞结束: %v >= %v", firstWrite, releasedAt)
	}
}

func TestProviderRelaySharedTransportReusesConnection(t *testing.T) {
	var newConnections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			newConnections.Add(1)
		}
	}
	server.Start()
	defer server.Close()

	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	defer relay.upstreamTransport.CloseIdleConnections()
	client := &http.Client{Transport: relay.upstreamTransport}
	for i := 0; i < 2; i++ {
		response, err := client.Get(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}
	if got := newConnections.Load(); got != 1 {
		t.Fatalf("新建连接数 = %d，期望 1", got)
	}
}

func TestForwardRequestCancelsUpstreamWhenClientDisconnects(t *testing.T) {
	upstreamStarted := make(chan struct{})
	releaseUpstream := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		close(upstreamStarted)
		select {
		case <-request.Context().Done():
		case <-releaseUpstream:
		}
	}))
	defer func() {
		close(releaseUpstream)
		server.Close()
	}()

	requestContext, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5.4"}`)).WithContext(requestContext)
	responseRecorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(responseRecorder)
	ginContext.Request = request
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	provider := Provider{ID: 1, Name: "Cancelable", APIURL: server.URL, APIKey: "key", Enabled: true}

	result := make(chan error, 1)
	go func() {
		_, err := relay.forwardRequest(
			ginContext,
			"codex",
			provider,
			"/responses",
			nil,
			nil,
			[]byte(`{"model":"gpt-5.4"}`),
			false,
			"gpt-5.4",
			"gpt-5.4",
		)
		result <- err
	}()

	select {
	case <-upstreamStarted:
	case <-time.After(time.Second):
		t.Fatal("上游请求未启动")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, errClientAbort) {
			t.Fatalf("返回错误 = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("转发请求未及时返回")
	}
}

func TestBlacklistSuccessNeedsWrite(t *testing.T) {
	now := time.Now()
	config := *DefaultBlacklistLevelConfig()
	config.EnableLevelBlacklist = true
	if blacklistSuccessNeedsWrite(config, 0, 0, 0, sql.NullTime{}, 0, sql.NullTime{}, now) {
		t.Fatal("健康记录不应反复写入")
	}
	if !blacklistSuccessNeedsWrite(config, 1, 0, 0, sql.NullTime{}, 0, sql.NullTime{}, now) {
		t.Fatal("失败计数未清零时应写入")
	}
	if !blacklistSuccessNeedsWrite(config, 0, 1, 0, sql.NullTime{}, 0, sql.NullTime{}, now) {
		t.Fatal("健康检查失败计数未清零时应写入")
	}
	recoveredAt := sql.NullTime{Time: now.Add(-2 * time.Hour), Valid: true}
	if !blacklistSuccessNeedsWrite(config, 0, 0, 2, recoveredAt, 1, sql.NullTime{}, now) {
		t.Fatal("跨过新的降级小时时应写入")
	}
}

func TestBlacklistSnapshotKeepsPreviousStateWhenRowScanFails(t *testing.T) {
	useIsolatedHomeDir(t)
	if err := InitDatabase(); err != nil {
		t.Fatal(err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM provider_blacklist`); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(time.Hour)
	if _, err := db.Exec(`
		INSERT INTO provider_blacklist (
			platform, provider_id, provider_name, failure_count, blacklist_level, blacklisted_until
		) VALUES (?, ?, ?, ?, ?, ?)
	`, "claude", "1", "Provider", 0, 1, until); err != nil {
		t.Fatal(err)
	}

	service := NewBlacklistService(NewSettingsService(), nil)
	defer service.Stop()
	if blacklisted, _ := service.IsBlacklistedByID("claude", "1", "Provider"); !blacklisted {
		t.Fatalf("初始黑名单状态未加载")
	}
	if _, err := db.Exec(`UPDATE provider_blacklist SET failure_count = 'invalid' WHERE provider_id = '1'`); err != nil {
		t.Fatal(err)
	}

	service.RefreshRuntimeSnapshot()
	if blacklisted, _ := service.IsBlacklistedByID("claude", "1", "Provider"); !blacklisted {
		t.Fatalf("扫描异常不应发布缺失供应商的不完整快照")
	}
}

func TestBlacklistFailureSourceMigrationClearsStaleCountsAndMarksActiveRows(t *testing.T) {
	useIsolatedHomeDir(t)
	if err := InitDatabase(); err != nil {
		t.Fatal(err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM app_settings WHERE key = 'blacklist_failure_sources_v1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM provider_blacklist`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO provider_blacklist (
			platform, provider_id, provider_name, failure_count, health_failure_count, blacklisted_until
		) VALUES
			('codex', 'stale', 'Stale', 7, 2, ?),
			('codex', 'active', 'Active', 9, 3, ?)
	`, time.Now().Add(-time.Minute), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	if err := migrateBlacklistFailureSourcesWithDB(db); err != nil {
		t.Fatal(err)
	}

	var requestCount int
	var healthCount int
	if err := db.QueryRow(`SELECT failure_count, health_failure_count FROM provider_blacklist WHERE provider_id = 'stale'`).Scan(&requestCount, &healthCount); err != nil {
		t.Fatal(err)
	}
	if requestCount != 0 || healthCount != 0 {
		t.Fatalf("未拉黑旧计数未清零: request=%d health=%d", requestCount, healthCount)
	}
	var source string
	var reason string
	if err := db.QueryRow(`SELECT blacklist_trigger_source, blacklist_reason FROM provider_blacklist WHERE provider_id = 'active'`).Scan(&source, &reason); err != nil {
		t.Fatal(err)
	}
	if source != BlacklistTriggerSourceLegacy || reason == "" {
		t.Fatalf("活动旧黑名单未标记来源: source=%q reason=%q", source, reason)
	}
}

func TestResolveProviderModelWithoutBodyCopyMatchesOverrides(t *testing.T) {
	provider := Provider{
		ModelMapping:         map[string]string{"claude-*": "vendor-*"},
		RequestBodyOverrides: map[string]interface{}{"model": "override-model"},
	}
	if got := resolveProviderModelWithoutBodyCopy(provider, "claude-opus"); got != "override-model" {
		t.Fatalf("顶层 model 覆盖结果 = %q", got)
	}
	provider.RequestBodyOverrides["model"] = nil
	if got := resolveProviderModelWithoutBodyCopy(provider, "claude-opus"); got != "vendor-opus" {
		t.Fatalf("null model 覆盖应回退到映射模型，实际 %q", got)
	}
}

func TestSessionAffinityConcurrentPendingFailuresReleaseBinding(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	sessionHash := "session-a"

	attemptA := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", "", 5, 5, true, false, false)
	attemptB := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", "", 5, 5, true, false, false)
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

	attemptA := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", "", 5, 5, true, false, false)
	attemptB := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", "", 5, 5, true, false, false)
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

	attemptA := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", "", 5, 5, true, false, false)
	if attemptA <= 0 {
		t.Fatalf("新会话首次 provider 应创建 pending attempt，got %d", attemptA)
	}
	relay.finishSessionProviderRequest(platform, sessionHash)
	relay.restoreOrReleaseSessionBinding(platform, sessionHash, nil, attemptA)

	attemptB := relay.beginSessionProviderRequest(platform, sessionHash, "provider-b", "Provider B", "", 5, 5, true, false, false)
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

	attemptA := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", "", 5, 5, true, false, false)
	relay.confirmSessionProviderBinding(platform, sessionHash, attemptA)
	relay.finishSessionProviderRequest(platform, sessionHash)
	original := relay.getSessionBindingSnapshot(platform, sessionHash)

	attemptB := relay.beginSessionProviderRequest(platform, sessionHash, "provider-b", "Provider B", "", 5, 5, false, false, false)
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

	attemptA := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", "", 5, 5, true, false, false)
	relay.confirmSessionProviderBinding(platform, sessionHash, attemptA)
	relay.finishSessionProviderRequest(platform, sessionHash)
	original := relay.getSessionBindingSnapshot(platform, sessionHash)

	attemptB := relay.beginSessionProviderRequest(platform, sessionHash, "provider-b", "Provider B", "", 5, 5, false, false, false)
	relay.finishSessionProviderRequest(platform, sessionHash)
	relay.restoreOrReleaseSessionBinding(platform, sessionHash, original, attemptB)

	binding := relay.sessionAffinity[sessionAffinityStateKey(platform, sessionHash)]
	if binding == nil || !binding.Confirmed || binding.ProviderID != "provider-a" || binding.ActiveRequests != 0 {
		t.Fatalf("全部失败后应恢复原 Provider A，got %#v", binding)
	}
}

func TestBeginSessionProviderRequestAllowsDifferentProviderDuringActiveMigration(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	sessionHash := "session-f"

	attemptA := relay.beginSessionProviderRequest(platform, sessionHash, "provider-a", "Provider A", "", 5, 5, true, false, false)
	relay.confirmSessionProviderBinding(platform, sessionHash, attemptA)

	fallbackAttempt := relay.beginSessionProviderRequest(platform, sessionHash, "provider-b", "Provider B", "", 5, 5, false, false, false)
	if fallbackAttempt <= 0 {
		t.Fatalf("原 Provider 仍有 in-flight 时应允许后备 provider 迁移，got attempt=%d", fallbackAttempt)
	}
	binding := relay.sessionAffinity[sessionAffinityStateKey(platform, sessionHash)]
	if binding == nil || binding.ProviderID != "provider-b" || binding.ActiveRequests != 2 || binding.ActiveRequestsByProvider["provider-a"] != 1 || binding.ActiveRequestsByProvider["provider-b"] != 1 {
		t.Fatalf("迁移期间应保留两个 provider 的活跃计数，got %#v", binding)
	}
	loads := relay.providerSessionLoads(platform)
	if loads["provider-a"].ActiveRequests != 1 || loads["provider-b"].ActiveRequests != 1 {
		t.Fatalf("迁移期间负载应分别统计两个 provider，got %#v", loads)
	}
	statuses := relay.GetSessionAffinityStatuses(platform)
	for _, status := range statuses {
		if status.ProviderID == "provider-b" && status.ActiveRequests != 1 {
			t.Fatalf("会话状态不应把旧 provider 请求计入新 provider，got %#v", status)
		}
	}
	relay.finishSessionProviderRequest(platform, sessionHash, "provider-a")
	relay.finishSessionProviderRequest(platform, sessionHash, "provider-b")
}

func TestBeginSessionProviderRequestRejectsFullProviderUnlessOverflowAllowed(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	relay.sessionAffinity[sessionAffinityStateKey(platform, "session-a")] = &providerSessionBinding{
		Platform:    platform,
		SessionHash: "session-a",
		ProviderID:  "provider-a",
		LastSeen:    time.Now(),
		Confirmed:   true,
	}

	rejected := relay.beginSessionProviderRequest(platform, "session-new", "provider-a", "Provider A", "", 1, 5, true, false, false)
	if rejected != -1 {
		t.Fatalf("full provider must reject new non-overflow binding, got %d", rejected)
	}
	if binding := relay.sessionAffinity[sessionAffinityStateKey(platform, "session-new")]; binding != nil {
		t.Fatalf("rejected binding should not create session state: %#v", binding)
	}

	accepted := relay.beginSessionProviderRequest(platform, "session-overflow", "provider-a", "Provider A", "", 1, 5, true, true, false)
	if accepted <= 0 {
		t.Fatalf("overflow fallback should allow full provider, got %d", accepted)
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

func TestOrderProvidersForSessionAffinityUsesBoundSessionCapacity(t *testing.T) {
	providers := []Provider{
		{ID: 1, Name: "A", SessionMaxSessions: 5},
		{ID: 2, Name: "B", SessionMaxSessions: 5},
		{ID: 3, Name: "C", SessionMaxSessions: 5},
	}
	loads := map[string]providerSessionLoad{
		"1": {ProviderID: "1", BoundSessions: 4, ActiveRequests: 3},
		"2": {ProviderID: "2", BoundSessions: 5},
		"3": {ProviderID: "3", BoundSessions: 4},
	}

	ordered := orderProvidersForSessionAffinity(providers, loads)
	got := make([]string, 0, len(ordered))
	for _, provider := range ordered {
		got = append(got, providerRefFromProvider(provider))
	}
	want := []string{"1", "3"}
	if len(got) != len(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
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

	ordered, _ := relay.reorderProviderAttemptsForSession(platform, providers, sessionHash, true, "1", relay.providerSessionLoads(platform))
	if providerRefFromProvider(ordered[0]) != "2" {
		t.Fatalf("已绑定会话应优先原 provider，got %s", ordered[0].Name)
	}
}

func TestReorderProviderAttemptsForSessionFillsInProviderOrder(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	platform := "claude"
	providers := []Provider{
		{ID: 1, Name: "A", SessionMaxSessions: 2},
		{ID: 2, Name: "B", SessionMaxSessions: 2},
		{ID: 3, Name: "C", SessionMaxSessions: 2},
	}
	relay.sessionAffinity[sessionAffinityStateKey(platform, "session-a")] = &providerSessionBinding{
		Platform:    platform,
		SessionHash: "session-a",
		ProviderID:  "1",
		LastSeen:    time.Now(),
		Confirmed:   true,
	}
	relay.sessionAffinity[sessionAffinityStateKey(platform, "session-b")] = &providerSessionBinding{
		Platform:    platform,
		SessionHash: "session-b",
		ProviderID:  "2",
		LastSeen:    time.Now(),
		Confirmed:   true,
	}

	ordered, _ := relay.reorderProviderAttemptsForSession(platform, providers, "session-new", true, "1", relay.providerSessionLoads(platform))
	got := []string{providerRefFromProvider(ordered[0]), providerRefFromProvider(ordered[1]), providerRefFromProvider(ordered[2])}
	want := []string{"1", "2", "3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestReorderProviderAttemptsForSessionPrioritizesFirstProviderWithoutSessionAcrossLevels(t *testing.T) {
	providers := []Provider{
		{ID: 2, Name: "Higher Level", Level: 1, SessionMaxSessions: 2},
		{ID: 1, Name: "First in Home", Level: 2, SessionMaxSessions: 2},
	}
	loads := map[string]providerSessionLoad{
		"2": {ProviderID: "2", BoundSessions: 1},
	}

	ordered, _ := orderProvidersForNewSession(providers, "1", loads)
	if providerRefFromProvider(ordered[0]) != "1" {
		t.Fatalf("首位零会话 provider 未跨 Level 优先，order=%v", []string{providerRefFromProvider(ordered[0]), providerRefFromProvider(ordered[1])})
	}
}

func TestReorderProviderAttemptsForSessionPrioritizesFirstProviderWhenAffinityEnabled(t *testing.T) {
	appSettings := &AppSettingsService{}
	appSettings.snapshot.Store(AppSettings{SessionAffinityEnabled: map[string]bool{"claude": true}})
	relay := NewProviderRelayService(nil, nil, nil, nil, appSettings, nil, "")
	providers := []Provider{
		{ID: 2, Name: "Higher Level", Level: 1, SessionMaxSessions: 2},
		{ID: 1, Name: "First in Home", Level: 2, SessionMaxSessions: 2},
	}

	ordered, _ := relay.reorderProviderAttemptsForSession("claude", providers, "session-new", true, "1", relay.providerSessionLoads("claude"))
	if providerRefFromProvider(ordered[0]) != "1" {
		t.Fatalf("真实会话调度入口未优先首位零会话 provider，order=%v", []string{providerRefFromProvider(ordered[0]), providerRefFromProvider(ordered[1])})
	}
}

func TestConcurrentNewSessionsOnlyOneClaimsFirstProvider(t *testing.T) {
	appSettings := &AppSettingsService{}
	appSettings.snapshot.Store(AppSettings{SessionAffinityEnabled: map[string]bool{"claude": true}})
	relay := NewProviderRelayService(nil, nil, nil, nil, appSettings, nil, "")
	providers := []Provider{
		{ID: 2, Name: "Higher Level", Level: 1, SessionMaxSessions: 5},
		{ID: 1, Name: "First in Home", Level: 2, SessionMaxSessions: 5},
	}
	ready := make(chan struct{}, 2)
	start := make(chan struct{})
	selected := make(chan string, 2)
	var workers sync.WaitGroup

	for index := 0; index < 2; index++ {
		workers.Add(1)
		go func(sessionHash string) {
			defer workers.Done()
			loads := relay.providerSessionLoads("claude")
			ordered, requireFirstProviderWithoutSession := relay.reorderProviderAttemptsForSession("claude", providers, sessionHash, true, "1", loads)
			ready <- struct{}{}
			<-start
			for providerIndex, provider := range ordered {
				attemptID := relay.beginSessionProviderRequest("claude", sessionHash, providerRefFromProvider(provider), provider.Name, "", providerSessionMaxSessions(provider), providerSessionTTLMinutes(provider), true, isProviderSessionOverflowAttempt(provider, loads), requireFirstProviderWithoutSession && providerIndex == 0)
				if attemptID >= 0 {
					selected <- providerRefFromProvider(provider)
					return
				}
			}
		}(fmt.Sprintf("session-%d", index))
	}

	<-ready
	<-ready
	close(start)
	workers.Wait()
	close(selected)

	selectionCounts := map[string]int{}
	for providerID := range selected {
		selectionCounts[providerID]++
	}
	if selectionCounts["1"] != 1 || selectionCounts["2"] != 1 {
		t.Fatalf("并发新会话 provider 分布 = %v，期望首位和后备各 1", selectionCounts)
	}
}

func TestReorderProviderAttemptsForSessionUsesExistingOrderWhenFirstProviderHasSession(t *testing.T) {
	providers := []Provider{
		{ID: 2, Name: "Higher Level", Level: 1, SessionMaxSessions: 2},
		{ID: 1, Name: "First in Home", Level: 2, SessionMaxSessions: 2},
	}
	loads := map[string]providerSessionLoad{
		"1": {ProviderID: "1", BoundSessions: 1},
	}

	ordered, _ := orderProvidersForNewSession(providers, "1", loads)
	if providerRefFromProvider(ordered[0]) != "2" {
		t.Fatalf("首位已有会话时应恢复现有 Level/负载顺序，order=%v", []string{providerRefFromProvider(ordered[0]), providerRefFromProvider(ordered[1])})
	}
}

func TestOrderProvidersForNewSessionCountsPendingFirstProviderBinding(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	providers := []Provider{
		{ID: 2, Name: "Higher Level", Level: 1, SessionMaxSessions: 2},
		{ID: 1, Name: "First in Home", Level: 2, SessionMaxSessions: 2},
	}
	attemptID := relay.beginSessionProviderRequest("claude", "session-pending", "1", "First in Home", "", 2, 30, true, false, false)
	if attemptID <= 0 {
		t.Fatalf("创建首位 provider 临时绑定失败，attempt=%d", attemptID)
	}

	ordered, _ := orderProvidersForNewSession(providers, "1", relay.providerSessionLoads("claude"))
	if providerRefFromProvider(ordered[0]) != "2" {
		t.Fatalf("首位 provider 的临时绑定应计入会话，order=%v", []string{providerRefFromProvider(ordered[0]), providerRefFromProvider(ordered[1])})
	}
}

func TestReorderGeminiProviderAttemptsForSessionPrioritizesFirstProviderWithoutSessionAcrossLevels(t *testing.T) {
	providers := []GeminiProvider{
		{ID: "gemini-2", Name: "Higher Level", Level: 1, SessionMaxSessions: 2},
		{ID: "gemini-1", Name: "First in Home", Level: 2, SessionMaxSessions: 2},
	}
	loads := map[string]providerSessionLoad{
		"gemini-2": {ProviderID: "gemini-2", BoundSessions: 1},
	}

	ordered, _ := orderGeminiProvidersForNewSession(providers, "gemini-1", loads)
	if providerRefFromGeminiProvider(ordered[0]) != "gemini-1" {
		t.Fatalf("Gemini 首位零会话 provider 未跨 Level 优先，order=%v", []string{providerRefFromGeminiProvider(ordered[0]), providerRefFromGeminiProvider(ordered[1])})
	}
}

func TestConcurrentNewGeminiSessionsOnlyOneClaimsFirstProvider(t *testing.T) {
	appSettings := &AppSettingsService{}
	appSettings.snapshot.Store(AppSettings{SessionAffinityEnabled: map[string]bool{"gemini": true}})
	relay := NewProviderRelayService(nil, nil, nil, nil, appSettings, nil, "")
	providers := []GeminiProvider{
		{ID: "gemini-2", Name: "Higher Level", Level: 1, SessionMaxSessions: 5},
		{ID: "gemini-1", Name: "First in Home", Level: 2, SessionMaxSessions: 5},
	}
	ready := make(chan struct{}, 2)
	start := make(chan struct{})
	selected := make(chan string, 2)
	var workers sync.WaitGroup

	for index := 0; index < 2; index++ {
		workers.Add(1)
		go func(sessionHash string) {
			defer workers.Done()
			loads := relay.providerSessionLoads("gemini")
			ordered, requireFirstProviderWithoutSession := relay.reorderGeminiProviderAttemptsForSession(providers, sessionHash, true, "gemini-1", loads)
			ready <- struct{}{}
			<-start
			for providerIndex, provider := range ordered {
				attemptID := relay.beginSessionProviderRequest("gemini", sessionHash, providerRefFromGeminiProvider(provider), provider.Name, "", geminiProviderSessionMaxSessions(provider), geminiProviderSessionTTLMinutes(provider), true, isGeminiProviderSessionOverflowAttempt(provider, loads), requireFirstProviderWithoutSession && providerIndex == 0)
				if attemptID >= 0 {
					selected <- providerRefFromGeminiProvider(provider)
					return
				}
			}
		}(fmt.Sprintf("gemini-session-%d", index))
	}

	<-ready
	<-ready
	close(start)
	workers.Wait()
	close(selected)

	selectionCounts := map[string]int{}
	for providerID := range selected {
		selectionCounts[providerID]++
	}
	if selectionCounts["gemini-1"] != 1 || selectionCounts["gemini-2"] != 1 {
		t.Fatalf("并发 Gemini 新会话 provider 分布 = %v，期望首位和后备各 1", selectionCounts)
	}
}

func TestOrderGeminiProvidersForNewSessionUsesExistingOrderWhenFirstProviderHasSession(t *testing.T) {
	providers := []GeminiProvider{
		{ID: "gemini-2", Name: "Higher Level", Level: 1, SessionMaxSessions: 2},
		{ID: "gemini-1", Name: "First in Home", Level: 2, SessionMaxSessions: 2},
	}
	loads := map[string]providerSessionLoad{
		"gemini-1": {ProviderID: "gemini-1", BoundSessions: 1},
	}

	ordered, _ := orderGeminiProvidersForNewSession(providers, "gemini-1", loads)
	if providerRefFromGeminiProvider(ordered[0]) != "gemini-2" {
		t.Fatalf("Gemini 首位已有会话时应恢复现有 Level/负载顺序，order=%v", []string{providerRefFromGeminiProvider(ordered[0]), providerRefFromGeminiProvider(ordered[1])})
	}
}

func TestOrderGeminiProvidersForSessionAffinityUsesBoundSessionCapacity(t *testing.T) {
	providers := []GeminiProvider{
		{ID: "a", Name: "A", SessionMaxSessions: 2},
		{ID: "b", Name: "B", SessionMaxSessions: 4},
		{ID: "c", Name: "C", SessionMaxSessions: 2},
	}
	loads := map[string]providerSessionLoad{
		"a": {ProviderID: "a", BoundSessions: 1, ActiveRequests: 3},
		"b": {ProviderID: "b", BoundSessions: 1},
		"c": {ProviderID: "c", BoundSessions: 2},
	}

	ordered := orderGeminiProvidersForSessionAffinity(providers, loads)
	got := make([]string, 0, len(ordered))
	for _, provider := range ordered {
		got = append(got, providerRefFromGeminiProvider(provider))
	}
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("gemini order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("gemini order = %v, want %v", got, want)
		}
	}
}

func TestProviderConcurrencySlotLimitAndRelease(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	providerID := "provider-a"
	limit := 1

	_, release, ok := relay.acquireProviderConcurrencySlot("claude", providerID, &limit, true, providerConcurrencyRequestMeta{})
	if !ok {
		t.Fatalf("first provider concurrency slot should be acquired")
	}
	if got := relay.providerConcurrencyCount("claude", providerID); got != 1 {
		t.Fatalf("active concurrency = %d, want 1", got)
	}
	if _, _, ok := relay.acquireProviderConcurrencySlot("claude", providerID, &limit, true, providerConcurrencyRequestMeta{}); ok {
		t.Fatalf("second provider concurrency slot should be rejected")
	}
	release()
	if got := relay.providerConcurrencyCount("claude", providerID); got != 0 {
		t.Fatalf("active concurrency after release = %d, want 0", got)
	}
	if _, releaseAgain, ok := relay.acquireProviderConcurrencySlot("claude", providerID, &limit, true, providerConcurrencyRequestMeta{}); !ok {
		t.Fatalf("provider concurrency slot should be reusable after release")
	} else {
		releaseAgain()
	}
}

func TestProviderConcurrencySlotUnlimitedWhenLimitEmpty(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	providerID := "provider-a"
	releases := make([]func(), 0, 3)

	for i := 0; i < 3; i++ {
		_, release, ok := relay.acquireProviderConcurrencySlot("claude", providerID, nil, true, providerConcurrencyRequestMeta{})
		if !ok {
			t.Fatalf("unlimited provider concurrency slot should be acquired")
		}
		releases = append(releases, release)
	}
	if got := relay.providerConcurrencyCount("claude", providerID); got != 3 {
		t.Fatalf("unlimited slot should be tracked, got %d", got)
	}
	for _, release := range releases {
		release()
	}
	if got := relay.providerConcurrencyCount("claude", providerID); got != 0 {
		t.Fatalf("unlimited slot should be released, got %d", got)
	}
}

func TestProviderConcurrencySlotRejectsZeroLimit(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	limit := 0

	if _, _, ok := relay.acquireProviderConcurrencySlot("claude", "provider-a", &limit, true, providerConcurrencyRequestMeta{}); ok {
		t.Fatalf("zero provider concurrency limit should reject new slots")
	}
	if got := relay.providerConcurrencyCount("claude", "provider-a"); got != 0 {
		t.Fatalf("active concurrency = %d, want 0", got)
	}
}

func TestProviderConcurrencyLimitJSONDistinguishesEmptyAndZero(t *testing.T) {
	var empty Provider
	if err := json.Unmarshal([]byte(`{"name":"empty"}`), &empty); err != nil {
		t.Fatalf("decode empty provider concurrency limit: %v", err)
	}
	if empty.ProviderConcurrencyLimit != nil {
		t.Fatalf("empty provider concurrency limit = %v, want nil", *empty.ProviderConcurrencyLimit)
	}

	var full Provider
	if err := json.Unmarshal([]byte(`{"name":"full","providerConcurrencyLimit":0}`), &full); err != nil {
		t.Fatalf("decode zero provider concurrency limit: %v", err)
	}
	if full.ProviderConcurrencyLimit == nil || *full.ProviderConcurrencyLimit != 0 {
		t.Fatalf("zero provider concurrency limit = %v, want 0", full.ProviderConcurrencyLimit)
	}

	encoded, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("encode zero provider concurrency limit: %v", err)
	}
	if !strings.Contains(string(encoded), `"providerConcurrencyLimit":0`) {
		t.Fatalf("encoded provider should preserve zero concurrency limit: %s", encoded)
	}

	negative := -1
	if normalized := normalizeProviderConcurrencyLimit(&negative); normalized != nil {
		t.Fatalf("negative provider concurrency limit = %d, want nil", *normalized)
	}
}

func TestProviderConcurrencySlotTracksWithoutEnforcingLimit(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	providerID := "provider-a"
	limit := 0
	releases := make([]func(), 0, 3)

	for i := 0; i < 3; i++ {
		_, release, ok := relay.acquireProviderConcurrencySlot("claude", providerID, &limit, false, providerConcurrencyRequestMeta{})
		if !ok {
			t.Fatalf("provider concurrency slot should be acquired when limit is disabled")
		}
		releases = append(releases, release)
	}
	if got := relay.providerConcurrencyCount("claude", providerID); got != 3 {
		t.Fatalf("disabled limit should still track active concurrency, got %d", got)
	}
	for _, release := range releases {
		release()
	}
	if got := relay.providerConcurrencyCount("claude", providerID); got != 0 {
		t.Fatalf("disabled limit slots should be released, got %d", got)
	}
}

func TestProviderConcurrencySlotTracksRequestDetails(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	providerID := "provider-a"
	limit := 1

	updateParameters, release, ok := relay.acquireProviderConcurrencySlot("claude", providerID, &limit, false, providerConcurrencyRequestMeta{
		ProviderName:        "Provider A",
		UserAgent:           "Codex-CLI/1.0",
		RequestedModel:      "claude-sonnet-4.8",
		Model:               "vendor-sonnet-4.8",
		MappedModel:         "vendor-sonnet-4.8",
		ModelMappingPattern: "claude-sonnet-*",
		ModelMappingTarget:  "vendor-sonnet-*",
		ModelRouteCaptured:  true,
		Parameters: []ProviderConcurrencyRequestParameter{
			{Key: providerRequestParameterReasoningEffort, RequestedValue: "medium", ActualValue: "high", Source: reasoningEffortSourceModelMapping},
		},
		Endpoint: "/v1/messages",
		IsStream: true,
	})
	if !ok {
		t.Fatalf("provider concurrency slot should be acquired")
	}

	details := relay.providerConcurrencyRequestDetails("claude", providerID)
	if len(details) != 1 {
		t.Fatalf("active request details = %d, want 1", len(details))
	}
	if details[0].UserAgent != "Codex-CLI/1.0" {
		t.Fatalf("user agent = %q, want Codex-CLI/1.0", details[0].UserAgent)
	}
	if details[0].RequestedModel != "claude-sonnet-4.8" || details[0].Model != "vendor-sonnet-4.8" {
		t.Fatalf("model route = %q -> %q", details[0].RequestedModel, details[0].Model)
	}
	if !details[0].ModelRouteCaptured || details[0].MappedModel != "vendor-sonnet-4.8" || details[0].ModelMappingPattern != "claude-sonnet-*" || details[0].ModelMappingTarget != "vendor-sonnet-*" {
		t.Fatalf("model route details = %#v", details[0])
	}
	if details[0].Endpoint != "/v1/messages" {
		t.Fatalf("endpoint = %q, want /v1/messages", details[0].Endpoint)
	}
	if !details[0].IsStream {
		t.Fatalf("is stream = false, want true")
	}
	if parameter := providerRequestParameterByKey(details[0].Parameters, providerRequestParameterReasoningEffort); parameter.ActualValue != "high" {
		t.Fatalf("active request parameters = %#v", details[0].Parameters)
	}
	updateParameters([]ProviderConcurrencyRequestParameter{
		{Key: providerRequestParameterReasoningEffort, RequestedValue: "medium"},
	})
	details = relay.providerConcurrencyRequestDetails("claude", providerID)
	if parameter := providerRequestParameterByKey(details[0].Parameters, providerRequestParameterReasoningEffort); parameter.RequestedValue != "medium" || parameter.ActualValue != "" {
		t.Fatalf("updated active request parameters = %#v", details[0].Parameters)
	}

	release()
	if details := relay.providerConcurrencyRequestDetails("claude", providerID); len(details) != 0 {
		t.Fatalf("active request details after release = %d, want 0", len(details))
	}
}

func TestNextProviderSwitchTargetAfterSkipsUnavailableProviders(t *testing.T) {
	targets := []providerSwitchTarget{
		{ProviderID: "a", ProviderName: "Provider A"},
		{ProviderID: "b", ProviderName: "Provider B"},
		{ProviderID: "c", ProviderName: "Provider C"},
	}

	next, ok := nextProviderSwitchTargetAfter(targets, 0, func(target providerSwitchTarget) bool {
		return target.ProviderID == "b"
	})
	if !ok {
		t.Fatalf("expected next provider switch target")
	}
	if next.ProviderID != "c" {
		t.Fatalf("next provider id = %q, want c", next.ProviderID)
	}
}

func TestNextProviderSwitchTargetAfterReturnsFalseWithoutCandidate(t *testing.T) {
	targets := []providerSwitchTarget{
		{ProviderID: "a", ProviderName: "Provider A"},
		{ProviderID: "b", ProviderName: "Provider B"},
	}

	if next, ok := nextProviderSwitchTargetAfter(targets, 0, func(providerSwitchTarget) bool {
		return true
	}); ok {
		t.Fatalf("unexpected next provider switch target: %+v", next)
	}
}

func TestProviderFailureCountingOnlyIncludesAvailabilityFailures(t *testing.T) {
	if shouldRecordProviderFailure(errProviderConcurrencyLimit) {
		t.Fatalf("provider concurrency limit should not count as provider failure")
	}
	if shouldRecordProviderFailure(fmt.Errorf("wrapped: %w", errProviderConcurrencyLimit)) {
		t.Fatalf("wrapped provider concurrency limit should not count as provider failure")
	}
	if shouldRecordProviderFailure(errClientAbort) {
		t.Fatalf("client abort should not count as provider failure")
	}
	if shouldRecordProviderFailure(errors.New("proxy conversion failed")) {
		t.Fatalf("proxy internal error should not count as provider failure")
	}
	if shouldRecordProviderFailure(newUpstreamErrorResponse(http.StatusBadRequest, "application/json", nil, nil)) {
		t.Fatalf("HTTP 4xx should not count as provider failure")
	}
	if !shouldRecordProviderFailure(newUpstreamErrorResponse(http.StatusTooManyRequests, "application/json", nil, nil)) {
		t.Fatalf("HTTP 429 should count as provider failure")
	}
	if !shouldRecordProviderFailure(newUpstreamErrorResponse(http.StatusBadGateway, "application/json", nil, nil)) {
		t.Fatalf("HTTP 5xx should count as provider failure")
	}
	if !shouldRecordProviderFailure(&net.DNSError{Err: "temporary failure", Name: "provider.example"}) {
		t.Fatalf("network error should count as provider failure")
	}
}

func TestProviderConcurrencyLimitErrorWrites429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	if !writeProviderConcurrencyLimitErrorIfAny(c, errProviderConcurrencyLimit) {
		t.Fatalf("provider concurrency limit should be handled")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusTooManyRequests, w.Body.String())
	}
	if got := gjson.Get(w.Body.String(), "error").String(); got != "all providers are busy" {
		t.Fatalf("error = %q, want all providers are busy", got)
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

func TestRemoveClaudeClientAuthHeadersRemovesStandardAndSelectedCustomHeader(t *testing.T) {
	headers := map[string]string{
		"authorization":  "Bearer client-token",
		"X-Api-Key":      "client-key",
		"X-Goog-Api-Key": "client-google-key",
		"X-Custom-Auth":  "client-custom-key",
		"User-Agent":     "claude-code",
	}

	removeClaudeClientAuthHeaders(headers, "X-Custom-Auth")

	for _, key := range []string{"Authorization", "x-api-key", "x-goog-api-key", "x-custom-auth"} {
		if value := getHeaderValueCaseInsensitive(headers, key); value != "" {
			t.Fatalf("认证头 %s 未清理: %q", key, value)
		}
	}
	if got := headers["User-Agent"]; got != "claude-code" {
		t.Fatalf("无关请求头被修改: %#v", headers)
	}
}
