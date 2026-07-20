package services

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestBuildAvailabilityProbePlanForCodexResponses(t *testing.T) {
	provider := Provider{
		ID:   11,
		Name: "Codex Proxy",
		ModelMapping: map[string]string{
			"gpt-5.1-mini": "gpt-5.3-codex",
		},
		RequestBodyOverrides: map[string]interface{}{
			"metadata": map[string]interface{}{
				"scene": "availability",
			},
		},
	}

	plan, err := buildAvailabilityProbePlan("codex", &provider, "gpt-5.1-mini", "/responses")
	if err != nil {
		t.Fatalf("buildAvailabilityProbePlan 失败: %v", err)
	}

	if plan.ResponseFormat != claudeAPIFormatOpenAIResponse {
		t.Fatalf("ResponseFormat = %q, 期望 %q", plan.ResponseFormat, claudeAPIFormatOpenAIResponse)
	}
	if plan.EffectiveModel != "gpt-5.3-codex" {
		t.Fatalf("EffectiveModel = %q, 期望 gpt-5.3-codex", plan.EffectiveModel)
	}
	if plan.Headers["openai-beta"] != "responses=experimental" {
		t.Fatalf("openai-beta = %q, 期望 responses=experimental", plan.Headers["openai-beta"])
	}
	if plan.Headers["User-Agent"] != availabilityProbeUserAgentCodex {
		t.Fatalf("User-Agent = %q, 期望 %q", plan.Headers["User-Agent"], availabilityProbeUserAgentCodex)
	}
	if got := gjson.GetBytes(plan.BodyBytes, "model").String(); got != "gpt-5.3-codex" {
		t.Fatalf("body.model = %q, 期望 gpt-5.3-codex", got)
	}
	if got := gjson.GetBytes(plan.BodyBytes, "input.0.content.0.type").String(); got != "input_text" {
		t.Fatalf("input.0.content.0.type = %q, 期望 input_text", got)
	}
	if got := gjson.GetBytes(plan.BodyBytes, "input.0.content.0.text").String(); got != availabilityProbeUserInput {
		t.Fatalf("input.0.content.0.text = %q, 期望 %q", got, availabilityProbeUserInput)
	}
	if got := gjson.GetBytes(plan.BodyBytes, "instructions").String(); !strings.Contains(got, availabilityProbeExpectedText) {
		t.Fatalf("instructions = %q, 应包含 %q", got, availabilityProbeExpectedText)
	}
	if got := gjson.GetBytes(plan.BodyBytes, "metadata.scene").String(); got != "availability" {
		t.Fatalf("metadata.scene = %q, 期望 availability", got)
	}
}

func TestBuildAvailabilityProbePlanForClaudeEndpointShape(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		wantFormat     string
		assertBodyPath string
	}{
		{
			name:           "anthropic_messages",
			endpoint:       "/v1/messages",
			wantFormat:     claudeAPIFormatAnthropic,
			assertBodyPath: "messages.0.content",
		},
		{
			name:           "openai_chat",
			endpoint:       "/v1/chat/completions",
			wantFormat:     claudeAPIFormatOpenAIChat,
			assertBodyPath: "messages.1.content",
		},
		{
			name:           "responses",
			endpoint:       "/responses",
			wantFormat:     claudeAPIFormatOpenAIResponse,
			assertBodyPath: "input.0.content.0.text",
		},
	}

	provider := Provider{
		ID:        7,
		Name:      "Claude Provider",
		APIFormat: claudeAPIFormatAnthropic,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := buildAvailabilityProbePlan("claude", &provider, "claude-sonnet-4-5", tt.endpoint)
			if err != nil {
				t.Fatalf("buildAvailabilityProbePlan 失败: %v", err)
			}
			if plan.ResponseFormat != tt.wantFormat {
				t.Fatalf("ResponseFormat = %q, 期望 %q", plan.ResponseFormat, tt.wantFormat)
			}
			if got := gjson.GetBytes(plan.BodyBytes, tt.assertBodyPath).String(); got == "" {
				t.Fatalf("body.%s 为空，说明没有按端点形态生成请求体", tt.assertBodyPath)
			}
		})
	}
}

func TestBuildAvailabilityProbePlansPrioritizeResponsesPresetByModelHint(t *testing.T) {
	provider := Provider{ID: 13, Name: "Preset Priority"}

	codexPlans, err := buildAvailabilityProbePlans("codex", &provider, "gpt-5.3-codex", "/responses")
	if err != nil {
		t.Fatalf("buildAvailabilityProbePlans(codex) 失败: %v", err)
	}
	if len(codexPlans) < 2 {
		t.Fatalf("codex plans 数量不足: %d", len(codexPlans))
	}
	if codexPlans[0].BodyPreset != availabilityProbeBodyPresetResponsesCodex {
		t.Fatalf("codex 首个 body preset = %q, 期望 %q", codexPlans[0].BodyPreset, availabilityProbeBodyPresetResponsesCodex)
	}
	if !strings.Contains(string(codexPlans[0].BodyBytes), `"tool_choice":"auto"`) {
		t.Fatalf("codex 首个 preset 应包含 tool_choice: %s", string(codexPlans[0].BodyBytes))
	}

	gptPlans, err := buildAvailabilityProbePlans("codex", &provider, "gpt-4.1-mini", "/responses")
	if err != nil {
		t.Fatalf("buildAvailabilityProbePlans(gpt) 失败: %v", err)
	}
	if len(gptPlans) < 2 {
		t.Fatalf("gpt plans 数量不足: %d", len(gptPlans))
	}
	if gptPlans[0].BodyPreset != availabilityProbeBodyPresetResponsesGPT {
		t.Fatalf("gpt 首个 body preset = %q, 期望 %q", gptPlans[0].BodyPreset, availabilityProbeBodyPresetResponsesGPT)
	}
	if strings.Contains(string(gptPlans[0].BodyBytes), `"tool_choice":"auto"`) {
		t.Fatalf("gpt 首个 preset 不应包含 tool_choice: %s", string(gptPlans[0].BodyBytes))
	}
	if gptPlans[1].BodyPreset != availabilityProbeBodyPresetResponsesCodex {
		t.Fatalf("gpt 第二个 body preset = %q, 期望 %q", gptPlans[1].BodyPreset, availabilityProbeBodyPresetResponsesCodex)
	}
}

func TestExtractAvailabilityResponseText(t *testing.T) {
	tests := []struct {
		name           string
		responseFormat string
		body           string
		want           string
	}{
		{
			name:           "anthropic",
			responseFormat: claudeAPIFormatAnthropic,
			body:           `{"content":[{"type":"text","text":"pong"}]}`,
			want:           "pong",
		},
		{
			name:           "openai_chat",
			responseFormat: claudeAPIFormatOpenAIChat,
			body:           `{"choices":[{"message":{"content":"pong"}}]}`,
			want:           "pong",
		},
		{
			name:           "responses",
			responseFormat: claudeAPIFormatOpenAIResponse,
			body:           `{"output":[{"type":"message","content":[{"type":"output_text","text":"pong"}]}]}`,
			want:           "pong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAvailabilityResponseText([]byte(tt.body), tt.responseFormat)
			if got != tt.want {
				t.Fatalf("extractAvailabilityResponseText() = %q, 期望 %q", got, tt.want)
			}
			if !responseContainsExpectedText([]byte(tt.body), tt.responseFormat, "PONG") {
				t.Fatalf("responseContainsExpectedText() 应大小写不敏感匹配 PONG")
			}
		})
	}
}

func TestResponseContainsExpectedTextIgnoresResponsesRefusal(t *testing.T) {
	body := []byte(`{"output":[{"type":"message","content":[{"type":"refusal","refusal":"我不能只回复 pong"}]}]}`)
	if responseContainsExpectedText(body, claudeAPIFormatOpenAIResponse, "pong") {
		t.Fatalf("Responses refusal 不应被误判为成功")
	}
}

func TestExecuteAvailabilityProbeUsesSelectedClaudeAuthHeader(t *testing.T) {
	tests := []struct {
		name          string
		authType      string
		wantHeader    string
		wantValue     string
		wantVersion   bool
		absentHeaders []string
	}{
		{name: "default_auth_token", authType: "", wantHeader: "Authorization", wantValue: "Bearer sk-claude", wantVersion: true, absentHeaders: []string{"x-api-key"}},
		{name: "api_key", authType: "x-api-key", wantHeader: "x-api-key", wantValue: "sk-claude", wantVersion: true, absentHeaders: []string{"Authorization"}},
		{name: "custom", authType: "X-Custom-Auth", wantHeader: "X-Custom-Auth", wantValue: "sk-claude", wantVersion: true, absentHeaders: []string{"Authorization", "x-api-key"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get(tt.wantHeader); got != tt.wantValue {
					t.Errorf("%s = %q, 期望 %q", tt.wantHeader, got, tt.wantValue)
				}
				for _, header := range tt.absentHeaders {
					if got := r.Header.Get(header); got != "" {
						t.Errorf("%s 应为空，实际为 %q", header, got)
					}
				}
				if got := r.Header.Get("anthropic-version"); (got != "") != tt.wantVersion {
					t.Errorf("anthropic-version = %q, wantVersion=%v", got, tt.wantVersion)
				}

				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("读取请求体失败: %v", err)
				}
				if !strings.Contains(string(bodyBytes), `"messages"`) {
					t.Errorf("Claude 探测请求体应包含 messages: %s", string(bodyBytes))
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"pong"}]}`))
			}))
			defer server.Close()

			provider := &Provider{
				ID:                   9,
				Name:                 "Claude Auth",
				APIURL:               server.URL,
				APIKey:               "sk-claude",
				ConnectivityAuthType: tt.authType,
				AvailabilityConfig: &AvailabilityConfig{
					TestModel:    "claude-sonnet-4-5",
					TestEndpoint: "/v1/messages",
					Timeout:      5000,
				},
			}

			result, err := executeAvailabilityProbe(
				context.Background(),
				&http.Client{Timeout: 0},
				provider,
				"claude",
				resolveProviderAvailabilityModel(provider, "claude"),
				resolveProviderAvailabilityEndpoint(provider, "claude"),
				resolveProviderAvailabilityTimeout(provider),
			)
			if err != nil {
				t.Fatalf("executeAvailabilityProbe 失败: %v", err)
			}
			if result.HTTPStatusCode != 200 {
				t.Fatalf("HTTPStatusCode = %d, 期望 200", result.HTTPStatusCode)
			}
		})
	}
}

func TestApplyProviderAuthHeadersSkipsAnthropicVersionForTransformedClaudeAPI(t *testing.T) {
	headers := make(http.Header)
	applyProviderAuthHeaders(headers, &Provider{
		APIKey:    "test-key",
		APIFormat: claudeAPIFormatOpenAIChat,
	}, "claude")

	if got := headers.Get("Authorization"); got != "Bearer test-key" {
		t.Fatalf("Authorization = %q, 期望 Bearer test-key", got)
	}
	if got := headers.Get("anthropic-version"); got != "" {
		t.Fatalf("OpenAI 转换格式不应添加 anthropic-version，实际为 %q", got)
	}
}

func TestExecuteAvailabilityProbeFallsBackToNextResponsesPresetOnInvalidRequest(t *testing.T) {
	requestBodies := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("请求路径 = %q, 期望 /responses", r.URL.Path)
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("读取请求体失败: %v", err)
		}
		body := string(bodyBytes)
		requestBodies = append(requestBodies, body)

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(body, `"tool_choice":"auto"`) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":{"message":"unknown field tool_choice"}}`))
			return
		}

		_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"pong"}]}]}`))
	}))
	defer server.Close()

	provider := &Provider{ID: 15, Name: "Retry Preset", APIURL: server.URL, APIKey: "sk-test"}
	result, err := executeAvailabilityProbe(context.Background(), &http.Client{Timeout: 0}, provider, "codex", "gpt-5.3-codex", "/responses", 5000)
	if err != nil {
		t.Fatalf("executeAvailabilityProbe 失败: %v", err)
	}

	if len(requestBodies) != 2 {
		t.Fatalf("请求次数 = %d, 期望 2", len(requestBodies))
	}
	if !strings.Contains(requestBodies[0], `"tool_choice":"auto"`) {
		t.Fatalf("首个 preset 应该是 codex 风格: %s", requestBodies[0])
	}
	if strings.Contains(requestBodies[1], `"tool_choice":"auto"`) {
		t.Fatalf("fallback preset 应切换到更精简的 gpt 风格: %s", requestBodies[1])
	}
	if result.Plan.EffectiveEndpoint != "/responses" {
		t.Fatalf("最终 endpoint = %q, 期望 /responses", result.Plan.EffectiveEndpoint)
	}
	if result.Plan.BodyPreset != availabilityProbeBodyPresetResponsesGPT {
		t.Fatalf("最终 body preset = %q, 期望 %q", result.Plan.BodyPreset, availabilityProbeBodyPresetResponsesGPT)
	}
	if result.HTTPStatusCode != 200 {
		t.Fatalf("HTTPStatusCode = %d, 期望 200", result.HTTPStatusCode)
	}

}

func TestExecuteAvailabilityProbeRetriesWithoutPromptCacheKeyWhenUnsupported(t *testing.T) {
	requestBodies := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("读取请求体失败: %v", err)
		}
		body := string(bodyBytes)
		requestBodies = append(requestBodies, body)

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(body, "prompt_cache_key") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"unsupported parameter: prompt_cache_key"}}`))
			return
		}

		_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"pong"}]}]}`))
	}))
	defer server.Close()

	provider := &Provider{
		ID:        17,
		Name:      "Prompt Cache Probe",
		APIURL:    server.URL,
		APIKey:    "sk-test",
		APIFormat: claudeAPIFormatOpenAIResponse,
		RequestBodyOverrides: map[string]interface{}{
			"prompt_cache_key": "probe-cache",
		},
	}
	result, err := executeAvailabilityProbe(context.Background(), &http.Client{Timeout: 0}, provider, "claude", "gpt-5.4", "/responses", 5000)
	if err != nil {
		t.Fatalf("executeAvailabilityProbe 失败: %v", err)
	}

	if len(requestBodies) != 2 {
		t.Fatalf("请求次数 = %d, 期望 2", len(requestBodies))
	}
	if !strings.Contains(requestBodies[0], "prompt_cache_key") {
		t.Fatalf("首个探测请求应包含 prompt_cache_key: %s", requestBodies[0])
	}
	if strings.Contains(requestBodies[1], "prompt_cache_key") {
		t.Fatalf("prompt_cache_key 不兼容重试应移除该字段: %s", requestBodies[1])
	}
	if result.HTTPStatusCode != 200 {
		t.Fatalf("HTTPStatusCode = %d, 期望 200", result.HTTPStatusCode)
	}

	result, err = executeAvailabilityProbe(context.Background(), &http.Client{Timeout: 0}, provider, "claude", "gpt-5.4", "/responses", 5000)
	if err != nil {
		t.Fatalf("第二次 executeAvailabilityProbe 失败: %v", err)
	}
	if result.HTTPStatusCode != 200 {
		t.Fatalf("第二次 HTTPStatusCode = %d, 期望 200", result.HTTPStatusCode)
	}
	if len(requestBodies) != 3 {
		t.Fatalf("第二次探测应复用禁用状态直接成功，总请求次数=%d，期望 3", len(requestBodies))
	}
	if strings.Contains(requestBodies[2], "prompt_cache_key") {
		t.Fatalf("缓存禁用状态应让后续探测首包直接移除 prompt_cache_key: %s", requestBodies[2])
	}
}

func TestRuntimePromptCacheDisableSharedWithAvailabilityProbe(t *testing.T) {
	resetOpenAICompatPromptCacheDisabledForTest(t)

	provider := &Provider{
		ID:        1701,
		Name:      "Runtime Disable Shared Probe",
		APIURL:    "https://example.com",
		APIKey:    "sk-runtime-disable",
		APIFormat: claudeAPIFormatOpenAIResponse,
		RequestBodyOverrides: map[string]interface{}{
			"prompt_cache_key": "probe-cache",
		},
	}

	baselinePlan, err := buildAvailabilityProbePlan("claude", provider, "gpt-5.4", "/responses")
	if err != nil {
		t.Fatalf("baseline buildAvailabilityProbePlan 失败: %v", err)
	}
	if !gjson.GetBytes(baselinePlan.BodyBytes, "prompt_cache_key").Exists() {
		t.Fatalf("baseline 探测请求应包含 prompt_cache_key: %s", string(baselinePlan.BodyBytes))
	}

	(&ProviderRelayService{}).disableOpenAICompatPromptCache(*provider, "runtime-session")

	plan, err := buildAvailabilityProbePlan("claude", provider, "gpt-5.4", "/responses")
	if err != nil {
		t.Fatalf("buildAvailabilityProbePlan 失败: %v", err)
	}
	if gjson.GetBytes(plan.BodyBytes, "prompt_cache_key").Exists() {
		t.Fatalf("runtime 禁用状态应让 availability probe 首包移除 prompt_cache_key: %s", string(plan.BodyBytes))
	}
}

func TestAvailabilityProbePromptCacheDisableSharedWithRuntime(t *testing.T) {
	resetOpenAICompatPromptCacheDisabledForTest(t)

	provider := Provider{
		ID:        1702,
		Name:      "Probe Disable Shared Runtime",
		APIURL:    "https://example.com",
		APIKey:    "sk-probe-disable",
		APIFormat: claudeAPIFormatOpenAIResponse,
	}
	bodyBytes := []byte(`{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`)
	relay := &ProviderRelayService{}

	baselinePlan, err := relay.buildProviderRequestPlan(provider, bodyBytes, "/v1/messages", "gpt-5.4")
	if err != nil {
		t.Fatalf("baseline buildProviderRequestPlan 失败: %v", err)
	}
	if !gjson.GetBytes(baselinePlan.BodyBytes, "prompt_cache_key").Exists() {
		t.Fatalf("baseline runtime 自动注入应包含 prompt_cache_key: %s", string(baselinePlan.BodyBytes))
	}

	disableOpenAICompatPromptCache(provider, "")

	plan, err := relay.buildProviderRequestPlan(provider, bodyBytes, "/v1/messages", "gpt-5.4")
	if err != nil {
		t.Fatalf("buildProviderRequestPlan 失败: %v", err)
	}
	if gjson.GetBytes(plan.BodyBytes, "prompt_cache_key").Exists() {
		t.Fatalf("probe 禁用状态应让 runtime 自动注入跳过 prompt_cache_key: %s", string(plan.BodyBytes))
	}
	if plan.PromptCacheKey != "" {
		t.Fatalf("probe 禁用状态下 plan.PromptCacheKey = %q，期望为空", plan.PromptCacheKey)
	}
}

func resetOpenAICompatPromptCacheDisabledForTest(t *testing.T) {
	t.Helper()
	clearOpenAICompatPromptCacheDisabledForTest()
	t.Cleanup(clearOpenAICompatPromptCacheDisabledForTest)
}

func clearOpenAICompatPromptCacheDisabledForTest() {
	openAICompatPromptCacheDisabledMu.Lock()
	defer openAICompatPromptCacheDisabledMu.Unlock()
	openAICompatPromptCacheDisabled = nil
}

func TestExecuteAvailabilityProbeFallsBackOnContentMismatch(t *testing.T) {
	requestBodies := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("读取请求体失败: %v", err)
		}
		body := string(bodyBytes)
		requestBodies = append(requestBodies, body)

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(body, `"tool_choice":"auto"`) {
			_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}]}`))
			return
		}

		_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"pong"}]}]}`))
	}))
	defer server.Close()

	provider := &Provider{ID: 16, Name: "Mismatch Retry", APIURL: server.URL, APIKey: "sk-test"}
	result, err := executeAvailabilityProbe(context.Background(), &http.Client{Timeout: 0}, provider, "codex", "gpt-5.3-codex", "/responses", 5000)
	if err != nil {
		t.Fatalf("executeAvailabilityProbe 失败: %v", err)
	}
	if len(requestBodies) != 2 {
		t.Fatalf("请求次数 = %d, 期望 2", len(requestBodies))
	}
	if result.Plan.BodyPreset != availabilityProbeBodyPresetResponsesGPT {
		t.Fatalf("content mismatch fallback 的 body preset = %q, 期望 %q", result.Plan.BodyPreset, availabilityProbeBodyPresetResponsesGPT)
	}
	if result.HTTPStatusCode != 200 {
		t.Fatalf("HTTPStatusCode = %d, 期望 200", result.HTTPStatusCode)
	}
}

func TestExecuteAvailabilityProbeFallsBackToAlternateEndpointOnRetryableHTTPStatus(t *testing.T) {
	requestPaths := make([]string, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/responses" {
			_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"pong"}]}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"route not found"}}`))
	}))
	defer server.Close()

	provider := &Provider{ID: 17, Name: "Endpoint Retry", APIURL: server.URL, APIKey: "sk-test"}
	result, err := executeAvailabilityProbe(context.Background(), &http.Client{Timeout: 0}, provider, "codex", "gpt-4.1-mini", "/responses", 5000)
	if err != nil {
		t.Fatalf("executeAvailabilityProbe 失败: %v", err)
	}
	if len(requestPaths) < 3 {
		t.Fatalf("请求次数 = %d, 期望至少 3 次（同路径 preset fallback + 备用 endpoint）", len(requestPaths))
	}
	if result.Plan.EffectiveEndpoint != "/v1/responses" {
		t.Fatalf("最终 endpoint = %q, 期望 /v1/responses", result.Plan.EffectiveEndpoint)
	}
	if result.HTTPStatusCode != 200 {
		t.Fatalf("HTTPStatusCode = %d, 期望 200", result.HTTPStatusCode)
	}
}

func TestExecuteAvailabilityProbeDoesNotRetryOnNetworkError(t *testing.T) {
	requestCount := 0
	client := &http.Client{
		Transport: availabilityProbeRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			requestCount++
			return nil, context.DeadlineExceeded
		}),
	}

	provider := &Provider{ID: 18, Name: "No Retry Network", APIURL: "https://example.com", APIKey: "sk-test"}
	_, err := executeAvailabilityProbe(context.Background(), client, provider, "codex", "gpt-5.3-codex", "/responses", 5000)
	if err == nil {
		t.Fatalf("网络错误时应返回错误")
	}
	if requestCount != 1 {
		t.Fatalf("网络错误请求次数 = %d, 期望 1", requestCount)
	}
}

func TestResolveProviderAvailabilityEndpointTrimsConfiguredEndpoint(t *testing.T) {
	provider := &Provider{
		APIEndpoint: "/v1/chat/completions",
		AvailabilityConfig: &AvailabilityConfig{
			TestEndpoint: "  responses  ",
		},
	}

	if got := resolveProviderAvailabilityEndpoint(provider, "codex"); got != "/responses" {
		t.Fatalf("resolveProviderAvailabilityEndpoint() = %q, 期望 /responses", got)
	}
}

type availabilityProbeRoundTripperFunc func(*http.Request) (*http.Response, error)

func (fn availabilityProbeRoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}
