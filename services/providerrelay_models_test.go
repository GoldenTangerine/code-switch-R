package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func useIsolatedHomeDir(t *testing.T) string {
	t.Helper()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	return homeDir
}

func assertProviderConfigExists(t *testing.T, homeDir string, filename string) {
	t.Helper()
	configPath := filepath.Join(homeDir, ".code-switch", filename)
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("期望测试配置文件存在: %s, err=%v", configPath, err)
	}
}

// TestModelsHandler 测试 /v1/models 端点处理器
func TestModelsHandler(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)

	// 设置测试环境
	gin.SetMode(gin.TestMode)

	// 创建模拟的上游服务器
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法
		if r.Method != "GET" {
			t.Errorf("期望 GET 请求，收到 %s", r.Method)
		}

		// 验证路径
		if r.URL.Path != "/v1/models" {
			t.Errorf("期望路径 /v1/models，收到 %s", r.URL.Path)
		}

		// 验证 Authorization 头
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			t.Error("缺少 Authorization 头")
		}
		if authHeader != "Bearer test-api-key" {
			t.Errorf("Authorization 头不正确，期望 'Bearer test-api-key'，收到 '%s'", authHeader)
		}
		if value := r.Header.Get("x-api-key"); value != "" {
			t.Errorf("客户端 x-api-key 不应转发到 Claude 上游，收到 %q", value)
		}
		if value := r.Header.Get("x-goog-api-key"); value != "" {
			t.Errorf("客户端 x-goog-api-key 不应转发到 Claude 上游，收到 %q", value)
		}

		// 返回模拟的模型列表
		response := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"id":       "claude-sonnet-4",
					"object":   "model",
					"created":  1234567890,
					"owned_by": "anthropic",
				},
				{
					"id":       "claude-opus-4",
					"object":   "model",
					"created":  1234567890,
					"owned_by": "anthropic",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer upstreamServer.Close()

	// 创建测试用的 ProviderService
	providerService := NewProviderService()
	settingsService := &SettingsService{}
	blacklistService := NewBlacklistService(settingsService, nil)

	// 创建测试用的 provider（使用模拟服务器的 URL）
	testProvider := Provider{
		ID:      1,
		Name:    "TestProvider",
		APIURL:  upstreamServer.URL,
		APIKey:  "test-api-key",
		Enabled: true,
		Level:   1,
	}

	// 保存 provider 配置
	err := providerService.SaveProviders("claude", []Provider{testProvider})
	if err != nil {
		t.Fatalf("保存 provider 配置失败: %v", err)
	}
	assertProviderConfigExists(t, homeDir, "claude-code.json")

	// 创建 ProviderRelayService
	relayService := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")

	// 创建测试路由
	router := gin.New()
	relayService.registerRoutes(router)

	// 创建测试请求
	req := httptest.NewRequest("GET", "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer client-placeholder")
	req.Header.Set("x-api-key", "client-placeholder")
	req.Header.Set("x-goog-api-key", "client-placeholder")
	w := httptest.NewRecorder()

	// 执行请求
	router.ServeHTTP(w, req)

	// 验证响应状态码
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，收到 %d", http.StatusOK, w.Code)
		t.Logf("响应体: %s", w.Body.String())
	}

	// 验证响应内容类型
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("期望 Content-Type 为 'application/json'，收到 '%s'", contentType)
	}

	// 验证响应体可以解析为 JSON
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("响应体不是有效的 JSON: %v", err)
		t.Logf("响应体: %s", w.Body.String())
	}

	// 验证响应包含 data 字段
	if _, ok := response["data"]; !ok {
		t.Error("响应缺少 'data' 字段")
	}
}

// TestCustomModelsHandler 测试自定义 CLI 工具的 /v1/models 端点
func TestCustomModelsHandler(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)

	// 设置测试环境
	gin.SetMode(gin.TestMode)

	// 创建模拟的上游服务器
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法
		if r.Method != "GET" {
			t.Errorf("期望 GET 请求，收到 %s", r.Method)
		}

		// 验证路径
		if r.URL.Path != "/v1/models" {
			t.Errorf("期望路径 /v1/models，收到 %s", r.URL.Path)
		}

		// 验证 Authorization 头
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer custom-api-key" {
			t.Errorf("Authorization 头不正确，期望 'Bearer custom-api-key'，收到 '%s'", authHeader)
		}

		// 返回模拟的模型列表
		response := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"id":      "custom-model-1",
					"object":  "model",
					"created": 1234567890,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer upstreamServer.Close()

	// 创建测试用的 ProviderService
	providerService := NewProviderService()
	settingsService := &SettingsService{}
	blacklistService := NewBlacklistService(settingsService, nil)

	// 创建测试用的 provider（使用模拟服务器的 URL）
	testProvider := Provider{
		ID:      1,
		Name:    "CustomTestProvider",
		APIURL:  upstreamServer.URL,
		APIKey:  "custom-api-key",
		Enabled: true,
		Level:   1,
	}

	// 保存 provider 配置（使用自定义 CLI 工具的 kind）
	toolId := "mytool"
	kind := "custom:" + toolId
	err := providerService.SaveProviders(kind, []Provider{testProvider})
	if err != nil {
		t.Fatalf("保存 provider 配置失败: %v", err)
	}
	assertProviderConfigExists(t, homeDir, filepath.Join("providers", "mytool.json"))

	// 创建 ProviderRelayService
	relayService := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")

	// 创建测试路由
	router := gin.New()
	relayService.registerRoutes(router)

	// 创建测试请求
	req := httptest.NewRequest("GET", "/custom/mytool/v1/models", nil)
	w := httptest.NewRecorder()

	// 执行请求
	router.ServeHTTP(w, req)

	// 验证响应状态码
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，收到 %d", http.StatusOK, w.Code)
		t.Logf("响应体: %s", w.Body.String())
	}

	// 验证响应内容类型
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("期望 Content-Type 为 'application/json'，收到 '%s'", contentType)
	}

	// 验证响应体可以解析为 JSON
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("响应体不是有效的 JSON: %v", err)
		t.Logf("响应体: %s", w.Body.String())
	}

	// 验证响应包含 data 字段
	if _, ok := response["data"]; !ok {
		t.Error("响应缺少 'data' 字段")
	}
}

// TestModelsHandler_NoProviders 测试没有可用 provider 的情况
func TestModelsHandler_NoProviders(t *testing.T) {
	useIsolatedHomeDir(t)

	gin.SetMode(gin.TestMode)

	// 创建空的 ProviderService
	providerService := NewProviderService()
	settingsService := &SettingsService{}
	blacklistService := NewBlacklistService(settingsService, nil)

	// 创建 ProviderRelayService（没有配置任何 provider）
	relayService := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")

	// 创建测试路由
	router := gin.New()
	relayService.registerRoutes(router)

	// 创建测试请求
	req := httptest.NewRequest("GET", "/v1/models", nil)
	w := httptest.NewRecorder()

	// 执行请求
	router.ServeHTTP(w, req)

	// 验证响应状态码应该是 404（没有可用的 provider）
	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 %d，收到 %d", http.StatusNotFound, w.Code)
	}

	// 验证响应包含错误信息
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("响应体不是有效的 JSON: %v", err)
	}

	if _, ok := response["error"]; !ok {
		t.Error("响应缺少 'error' 字段")
	}
}

func TestModelsHandler_AggregatesClaudeRoutingModels(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)

	appSettings := NewAppSettingsService(nil)
	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ClaudeModelRoutingEnabled = true
	settings.ClaudeModelAggregationEnabled = true
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatal(err)
	}

	routing := NewClaudeModelRoutingService(nil, appSettings, nil)
	routing.routes = map[string][]claudeModelRouteProvider{
		"claude-4-5": {{Metadata: ProviderModelPricingItem{Model: "vendor-a", DisplayName: "Claude 4.5"}}},
		"claude-5":   {{Metadata: ProviderModelPricingItem{Model: "vendor-b", DisplayName: "Claude 5"}}},
	}
	relay := NewProviderRelayService(NewProviderService(), nil, NewBlacklistService(&SettingsService{}, nil), nil, appSettings, nil, "")
	relay.BindClaudeModelRoutingService(routing)
	router := gin.New()
	relay.registerRoutes(router)

	req := httptest.NewRequest("GET", "/v1/models?limit=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var response ClaudeModelListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "claude-4-5" || !response.HasMore {
		t.Fatalf("聚合响应错误: %#v", response)
	}
}

func TestSessionProviderPreferenceLatestActiveRequestWinsAndFailureRollsBack(t *testing.T) {
	relay := &ProviderRelayService{}
	first := relay.beginSessionProviderPreferenceRequest("claude", "session-a")
	relay.updateSessionProviderPreferenceAttempt("claude", "session-a", first, "provider-a", "A")
	second := relay.beginSessionProviderPreferenceRequest("claude", "session-a")
	relay.updateSessionProviderPreferenceAttempt("claude", "session-a", second, "provider-b", "B")

	preferred, ok := relay.sessionProviderPreference("claude", "session-a")
	if !ok || preferred.ProviderID != "provider-b" {
		t.Fatalf("最新主请求未优先: %#v, ok=%v", preferred, ok)
	}

	relay.finishSessionProviderPreferenceRequest("claude", "session-a", second, false)
	preferred, ok = relay.sessionProviderPreference("claude", "session-a")
	if !ok || preferred.ProviderID != "provider-a" {
		t.Fatalf("失败后未回滚到仍在进行的请求: %#v, ok=%v", preferred, ok)
	}

	relay.finishSessionProviderPreferenceRequest("claude", "session-a", first, true)
	preferred, ok = relay.sessionProviderPreference("claude", "session-a")
	if !ok || preferred.ProviderID != "provider-a" {
		t.Fatalf("成功供应商未成为会话首选: %#v, ok=%v", preferred, ok)
	}
}

func TestSessionProviderPreferenceOlderSuccessDoesNotReplaceNewerSuccess(t *testing.T) {
	relay := &ProviderRelayService{}
	older := relay.beginSessionProviderPreferenceRequest("claude", "session-b")
	relay.updateSessionProviderPreferenceAttempt("claude", "session-b", older, "provider-a", "A")
	newer := relay.beginSessionProviderPreferenceRequest("claude", "session-b")
	relay.updateSessionProviderPreferenceAttempt("claude", "session-b", newer, "provider-b", "B")

	relay.finishSessionProviderPreferenceRequest("claude", "session-b", newer, true)
	relay.finishSessionProviderPreferenceRequest("claude", "session-b", older, true)

	preferred, ok := relay.sessionProviderPreference("claude", "session-b")
	if !ok || preferred.ProviderID != "provider-b" {
		t.Fatalf("旧请求覆盖了较新的成功结果: %#v, ok=%v", preferred, ok)
	}
}

func TestSessionProviderPreferenceEvictsLeastRecentlyUsedInactiveSession(t *testing.T) {
	relay := &ProviderRelayService{}
	for index := 0; index <= sessionProviderPreferenceMaxInactive; index++ {
		sessionHash := fmt.Sprintf("session-%d", index)
		generation := relay.beginSessionProviderPreferenceRequest("claude", sessionHash)
		relay.updateSessionProviderPreferenceAttempt("claude", sessionHash, generation, "provider-a", "A")
		relay.finishSessionProviderPreferenceRequest("claude", sessionHash, generation, true)
	}

	if _, ok := relay.sessionProviderPreference("claude", "session-0"); ok {
		t.Fatal("最久未使用的非活动会话未被淘汰")
	}
	if preferred, ok := relay.sessionProviderPreference("claude", fmt.Sprintf("session-%d", sessionProviderPreferenceMaxInactive)); !ok || preferred.ProviderID != "provider-a" {
		t.Fatalf("最新会话被错误淘汰: %#v, ok=%v", preferred, ok)
	}
}

func TestBuildProviderAttemptGroupsPromotesPreferredProviderAcrossLevels(t *testing.T) {
	providers := []Provider{
		{ID: 1, Name: "A", Level: 1},
		{ID: 2, Name: "C", Level: 1},
		{ID: 3, Name: "B", Level: 5},
		{ID: 4, Name: "D", Level: 6},
	}

	levels, groups := buildProviderAttemptGroups(providers, providerRefFromProvider(providers[2]))
	ordered := make([]string, 0, len(providers))
	for _, level := range levels {
		for _, provider := range groups[level] {
			ordered = append(ordered, provider.Name)
		}
	}
	want := []string{"B", "A", "C", "D"}
	if !reflect.DeepEqual(ordered, want) {
		t.Fatalf("供应商顺序 = %v，期望 %v", ordered, want)
	}
}

func TestForwardRequestWithPlanDoesNotPublishPreferenceBeforeConcurrencySlot(t *testing.T) {
	relay := NewProviderRelayService(nil, nil, nil, nil, nil, nil, "")
	generation := relay.beginSessionProviderPreferenceRequest("claude", "session-busy")
	limit := 0
	provider := Provider{
		ID:                       1,
		Name:                     "Busy",
		APIURL:                   "http://127.0.0.1",
		ProviderConcurrencyLimit: &limit,
	}
	writer := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(writer)
	plan := providerRequestPlan{
		BodyBytes:                  []byte(`{"model":"claude-opus-4.8"}`),
		EffectiveModel:             "claude-opus-4.8",
		EffectiveEndpoint:          "/v1/messages",
		SessionPreferenceHash:      "session-busy",
		SessionPreferenceGeneration: generation,
	}
	ok, err := relay.forwardRequestWithPlan(
		context,
		"claude",
		provider,
		plan.EffectiveEndpoint,
		map[string]string{},
		map[string]string{},
		plan.BodyBytes,
		false,
		plan.EffectiveModel,
		plan.EffectiveModel,
		plan,
		true,
	)
	if ok || err != errProviderConcurrencyLimit {
		t.Fatalf("满载供应商结果 = ok:%v err:%v，期望并发限制错误", ok, err)
	}

	key := sessionAffinityStateKey("claude", "session-busy")
	relay.sessionProviderPreferenceMu.Lock()
	state := relay.sessionProviderPreferences[key]
	attempt := state.Active[generation]
	relay.sessionProviderPreferenceMu.Unlock()
	if attempt.ProviderID != "" {
		t.Fatalf("并发槽拒绝前不应发布供应商偏好: %#v", attempt)
	}
	relay.finishSessionProviderPreferenceRequest("claude", "session-busy", generation, false)
}
