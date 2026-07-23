package services

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	claudeMetadataSessionA = "user_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa_account_11111111-1111-1111-1111-111111111111_session_22222222-2222-2222-2222-222222222222"
	claudeMetadataSessionB = "user_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb_account_11111111-1111-1111-1111-111111111111_session_33333333-3333-3333-3333-333333333333"
)

func disableBlacklistForTest(t *testing.T) {
	t.Helper()
	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	if _, err := db.Exec(`UPDATE app_settings SET value = 'false' WHERE key = 'enable_blacklist'`); err != nil {
		t.Fatalf("关闭黑名单开关失败: %v", err)
	}
}

func TestClaudeResponsesContinuationBindsPreviousResponseID(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	var secondRequestBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		call := atomic.AddInt32(&callCount, 1)
		if r.URL.Path != "/responses" {
			t.Fatalf("upstream path = %q, 期望 /responses", r.URL.Path)
		}
		if call == 2 {
			secondRequestBody = string(body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_abc","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{
		ID:        1,
		Name:      "OpenAI Responses",
		APIURL:    upstream.URL,
		APIKey:    "test-key",
		Enabled:   true,
		Level:     1,
		APIFormat: claudeAPIFormatOpenAIResponse,
	}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	body := `{"model":"gpt-5.4","max_tokens":16,"system":"sys","metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"hi"}]}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("第 %d 次请求状态码=%d body=%s", i+1, w.Code, w.Body.String())
		}
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Fatalf("upstream 调用次数=%d，期望 2", callCount)
	}
	if !strings.Contains(secondRequestBody, `"previous_response_id":"resp_abc"`) {
		t.Fatalf("第二轮请求未携带 previous_response_id: %s", secondRequestBody)
	}
}

func TestClaudeResponsesContinuationSessionKeySeparatesAgentContexts(t *testing.T) {
	mainBody := []byte(`{"model":"gpt-5.4","system":"You are the main coding agent","metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"fix the login route"}],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`)
	mainFollowUpBody := []byte(`{"model":"gpt-5.4","system":"You are the main coding agent","metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"fix the login route"},{"role":"assistant","content":"checking"},{"role":"user","content":"continue"}],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`)
	exploreABody := []byte(`{"model":"gpt-5.4","system":"You are an Explore agent","metadata":{"user_id":"` + claudeMetadataSessionA + `","agent_id":"explore-a"},"messages":[{"role":"user","content":"inspect router files"}],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`)
	exploreBBody := []byte(`{"model":"gpt-5.4","system":"You are an Explore agent","metadata":{"user_id":"` + claudeMetadataSessionA + `","agent_id":"explore-b"},"messages":[{"role":"user","content":"inspect router files"}],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`)

	mainKey := deriveClaudeResponsesContinuationSessionKey(mainBody)
	mainFollowUpKey := deriveClaudeResponsesContinuationSessionKey(mainFollowUpBody)
	exploreAKey := deriveClaudeResponsesContinuationSessionKey(exploreABody)
	exploreBKey := deriveClaudeResponsesContinuationSessionKey(exploreBBody)

	if mainKey == "" || exploreAKey == "" || exploreBKey == "" {
		t.Fatalf("完整代理上下文应生成续链键，main=%q exploreA=%q exploreB=%q", mainKey, exploreAKey, exploreBKey)
	}
	if mainKey != mainFollowUpKey {
		t.Fatalf("同一代理后续轮次应保持续链键稳定，first=%q follow_up=%q", mainKey, mainFollowUpKey)
	}
	if mainKey == exploreAKey || mainKey == exploreBKey || exploreAKey == exploreBKey {
		t.Fatalf("同 session 的不同代理上下文必须隔离，main=%q exploreA=%q exploreB=%q", mainKey, exploreAKey, exploreBKey)
	}
}

func TestClaudeResponsesContinuationSessionKeyUsesStableAgentIdentity(t *testing.T) {
	firstBody := []byte(`{"model":"gpt-5.4","system":"You are an Explore agent","metadata":{"user_id":"` + claudeMetadataSessionA + `","agent_id":"explore-a"},"messages":[{"role":"user","content":"inspect router files"}],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`)
	followUpBody := []byte(`{"model":"gpt-5.4","system":"You are an Explore agent with an updated instruction","metadata":{"user_id":"` + claudeMetadataSessionA + `","agent_id":"explore-a"},"messages":[{"role":"user","content":"inspect router files"},{"role":"assistant","content":"checking"},{"role":"user","content":"continue"}],"tools":[{"name":"Read","input_schema":{"type":"object"}},{"name":"Glob","input_schema":{"type":"object"}}]}`)

	firstKey := deriveClaudeResponsesContinuationSessionKey(firstBody)
	followUpKey := deriveClaudeResponsesContinuationSessionKey(followUpBody)
	if firstKey == "" || firstKey != followUpKey {
		t.Fatalf("显式 agent_id 应生成稳定续链键，first=%q follow_up=%q", firstKey, followUpKey)
	}
}

func TestClaudeResponsesContinuationDisablesSubagentWithoutStableIdentity(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","system":"You are an agent for Claude Code, acting as an Explore agent","metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"inspect router files"}],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`)

	if got := deriveClaudeResponsesContinuationSessionKey(body); got != "" {
		t.Fatalf("缺少唯一代理标识的子代理不应启用续链，got=%q", got)
	}
}

func TestClaudeResponsesContinuationRequiresStableAgentContext(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","metadata":{"user_id":"` + claudeMetadataSessionA + `"}}`)

	if got := deriveClaudeResponsesContinuationSessionKey(body); got != "" {
		t.Fatalf("缺少 system、tools 和首条 user 内容时不应启用续链，got=%q", got)
	}
}

func TestClaudeResponsesContinuationBindingsStayWithinAgentContext(t *testing.T) {
	provider := Provider{
		ID:        1,
		Name:      "OpenAI Responses",
		APIURL:    "https://example.com",
		APIKey:    "test-key",
		APIFormat: claudeAPIFormatOpenAIResponse,
	}
	relay := &ProviderRelayService{claudeResponses: make(map[string]claudeResponsesSessionBinding)}
	mainBody := []byte(`{"model":"gpt-5.4","max_tokens":16,"system":"You are the main coding agent","metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"fix the login route"}],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`)
	exploreABody := []byte(`{"model":"gpt-5.4","max_tokens":16,"system":"You are an Explore agent","metadata":{"user_id":"` + claudeMetadataSessionA + `","agent_id":"explore-a"},"messages":[{"role":"user","content":"inspect router files"}],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`)
	exploreBBody := []byte(`{"model":"gpt-5.4","max_tokens":16,"system":"You are an Explore agent","metadata":{"user_id":"` + claudeMetadataSessionA + `","agent_id":"explore-b"},"messages":[{"role":"user","content":"inspect router files"}],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`)

	mainPlan, err := relay.buildProviderRequestPlan(provider, mainBody, "/v1/messages", "gpt-5.4")
	if err != nil {
		t.Fatalf("构建主代理请求失败: %v", err)
	}
	relay.bindClaudeResponsesPreviousResponseID(provider, mainPlan.ContinuationSessionKey, "resp_main")

	exploreAPlan, err := relay.buildProviderRequestPlan(provider, exploreABody, "/v1/messages", "gpt-5.4")
	if err != nil {
		t.Fatalf("构建 Explore A 请求失败: %v", err)
	}
	if exploreAPlan.PreviousResponseID != "" {
		t.Fatalf("Explore A 不应续接主代理 response，got=%q", exploreAPlan.PreviousResponseID)
	}
	relay.bindClaudeResponsesPreviousResponseID(provider, exploreAPlan.ContinuationSessionKey, "resp_explore_a")

	exploreBPlan, err := relay.buildProviderRequestPlan(provider, exploreBBody, "/v1/messages", "gpt-5.4")
	if err != nil {
		t.Fatalf("构建 Explore B 请求失败: %v", err)
	}
	if exploreBPlan.PreviousResponseID != "" {
		t.Fatalf("Explore B 不应续接其他代理 response，got=%q", exploreBPlan.PreviousResponseID)
	}
	relay.bindClaudeResponsesPreviousResponseID(provider, exploreBPlan.ContinuationSessionKey, "resp_explore_b")

	mainFollowUpPlan, err := relay.buildProviderRequestPlan(provider, mainBody, "/v1/messages", "gpt-5.4")
	if err != nil {
		t.Fatalf("构建主代理后续请求失败: %v", err)
	}
	if mainFollowUpPlan.PreviousResponseID != "resp_main" {
		t.Fatalf("主代理应续接自身 response，got=%q", mainFollowUpPlan.PreviousResponseID)
	}
	if got := gjson.GetBytes(mainFollowUpPlan.BodyBytes, "previous_response_id").String(); got != "resp_main" {
		t.Fatalf("主代理出站请求 previous_response_id=%q，期望 resp_main", got)
	}

	exploreAFollowUpPlan, err := relay.buildProviderRequestPlan(provider, exploreABody, "/v1/messages", "gpt-5.4")
	if err != nil {
		t.Fatalf("构建 Explore A 后续请求失败: %v", err)
	}
	if exploreAFollowUpPlan.PreviousResponseID != "resp_explore_a" {
		t.Fatalf("Explore A 应续接自身 response，got=%q", exploreAFollowUpPlan.PreviousResponseID)
	}
}

func TestClaudeResponsesForwardAddsOpenAIBetaHeader(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	seenHeaders := make([]string, 0, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		seenHeaders = append(seenHeaders, r.Header.Get("openai-beta"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_header","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{ID: 1, Name: "OpenAI Responses", APIURL: upstream.URL, APIKey: "test-key", Enabled: true, Level: 1, APIFormat: claudeAPIFormatOpenAIResponse}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	body := `{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	req1 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("首轮状态码=%d body=%s", w1.Code, w1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("OpenAI-Beta", "custom=1")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("显式 header 请求状态码=%d body=%s", w2.Code, w2.Body.String())
	}

	if atomic.LoadInt32(&callCount) != 2 {
		t.Fatalf("upstream 调用次数=%d，期望 2", callCount)
	}
	if len(seenHeaders) != 2 {
		t.Fatalf("记录 header 数=%d，期望 2", len(seenHeaders))
	}
	if seenHeaders[0] != "responses=experimental" {
		t.Fatalf("真实转发 openai-beta=%q，期望 responses=experimental", seenHeaders[0])
	}
	if seenHeaders[1] != "custom=1" {
		t.Fatalf("显式 openai-beta 不应被覆盖，got=%q", seenHeaders[1])
	}
}

func TestClaudeResponsesContinuationRequiresSessionMetadata(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	var secondRequestBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		call := atomic.AddInt32(&callCount, 1)
		if call == 2 {
			secondRequestBody = string(body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_abc","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{ID: 1, Name: "OpenAI Responses", APIURL: upstream.URL, APIKey: "test-key", Enabled: true, Level: 1, APIFormat: claudeAPIFormatOpenAIResponse}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	body := `{"model":"gpt-5.4","max_tokens":16,"system":"sys","messages":[{"role":"user","content":"hi"}]}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("第 %d 次请求状态码=%d body=%s", i+1, w.Code, w.Body.String())
		}
	}
	if strings.Contains(secondRequestBody, "previous_response_id") {
		t.Fatalf("缺少会话 metadata 时不应启用 previous_response_id: %s", secondRequestBody)
	}
}

func TestClaudeResponsesContinuationSeparatesPromptCacheKeyFromSessionKey(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	requestBodies := make([]string, 0, 3)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requestBodies = append(requestBodies, string(body))
		call := atomic.AddInt32(&callCount, 1)
		responseID := "resp_a1"
		if call == 2 {
			responseID = "resp_b1"
		} else if call == 3 {
			responseID = "resp_a2"
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + responseID + `","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{ID: 1, Name: "OpenAI Responses", APIURL: upstream.URL, APIKey: "test-key", Enabled: true, Level: 1, APIFormat: claudeAPIFormatOpenAIResponse}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	bodyA := `{"model":"gpt-5.4","max_tokens":16,"prompt_cache_key":"shared-cache","metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"hi"}]}`
	bodyB := `{"model":"gpt-5.4","max_tokens":16,"prompt_cache_key":"shared-cache","metadata":{"user_id":"` + claudeMetadataSessionB + `"},"messages":[{"role":"user","content":"hi"}]}`
	for i, body := range []string{bodyA, bodyB, bodyA} {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("第 %d 次请求状态码=%d body=%s", i+1, w.Code, w.Body.String())
		}
	}
	if len(requestBodies) != 3 {
		t.Fatalf("upstream 请求数=%d，期望 3", len(requestBodies))
	}
	if strings.Contains(requestBodies[1], "previous_response_id") {
		t.Fatalf("不同 metadata session 即使共享 prompt_cache_key 也不应续到 A 会话: %s", requestBodies[1])
	}
	if !strings.Contains(requestBodies[2], `"previous_response_id":"resp_a1"`) {
		t.Fatalf("A 会话第三轮应续到 A 自己的 response_id: %s", requestBodies[2])
	}
}

func TestClaudeResponsesContinuationRetriesWithoutExpiredPreviousResponseID(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	var retryRequestBody string
	var thirdRequestBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		call := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			_, _ = w.Write([]byte(`{"id":"resp_old","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
			return
		}
		if call == 2 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"previous_response_not_found","message":"previous response not found"}}`))
			return
		}
		if call == 3 {
			retryRequestBody = string(body)
			_, _ = w.Write([]byte(`{"id":"resp_new","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
			return
		}
		thirdRequestBody = string(body)
		_, _ = w.Write([]byte(`{"id":"resp_latest","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{
		ID:        1,
		Name:      "OpenAI Responses",
		APIURL:    upstream.URL,
		APIKey:    "test-key",
		Enabled:   true,
		Level:     1,
		APIFormat: claudeAPIFormatOpenAIResponse,
	}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	body := `{"model":"gpt-5.4","max_tokens":16,"system":"sys","metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"hi"}]}`
	req1 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("首轮状态码=%d body=%s", w1.Code, w1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("回退重试后状态码=%d body=%s", w2.Code, w2.Body.String())
	}
	if strings.Contains(retryRequestBody, "previous_response_id") {
		t.Fatalf("回退重试请求不应携带 previous_response_id: %s", retryRequestBody)
	}

	req3 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("第三轮状态码=%d body=%s", w3.Code, w3.Body.String())
	}
	if atomic.LoadInt32(&callCount) != 4 {
		t.Fatalf("upstream 调用次数=%d，期望 4", callCount)
	}
	if !strings.Contains(thirdRequestBody, `"previous_response_id":"resp_new"`) {
		t.Fatalf("not_found 回退成功后应重新绑定新 response_id，第三轮请求=%s", thirdRequestBody)
	}
}

func TestClaudeResponsesContinuationUnsupportedWebSocketMessageDisablesContinuation(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	requestBodies := make([]string, 0, 4)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requestBodies = append(requestBodies, string(body))
		call := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		switch call {
		case 1:
			_, _ = w.Write([]byte(`{"id":"resp_old","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
		case 2:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"previous_response_id is only supported on Responses WebSocket v2"}}`))
		default:
			_, _ = w.Write([]byte(`{"id":"resp_after","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
		}
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{ID: 1, Name: "OpenAI Responses", APIURL: upstream.URL, APIKey: "test-key", Enabled: true, Level: 1, APIFormat: claudeAPIFormatOpenAIResponse}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	body := `{"model":"gpt-5.4","max_tokens":16,"metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"hi"}]}`
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("第 %d 轮状态码=%d body=%s", i+1, w.Code, w.Body.String())
		}
	}
	if atomic.LoadInt32(&callCount) != 4 {
		t.Fatalf("upstream 调用次数=%d，期望 4", callCount)
	}
	if !strings.Contains(requestBodies[1], `"previous_response_id":"resp_old"`) {
		t.Fatalf("第二次请求应先携带 previous_response_id: %s", requestBodies[1])
	}
	if strings.Contains(requestBodies[2], "previous_response_id") {
		t.Fatalf("WebSocket-only 错误回退重试不应继续携带 previous_response_id: %s", requestBodies[2])
	}
	if strings.Contains(requestBodies[3], "previous_response_id") {
		t.Fatalf("判定 unsupported 后同 session 应短期禁用续链: %s", requestBodies[3])
	}
}

func TestClaudeResponsesPromptCacheKeyUnsupportedRetriesAndDisablesAutoInjection(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	requestBodies := make([]string, 0, 3)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requestBodies = append(requestBodies, string(body))
		call := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"unsupported parameter: prompt_cache_key"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"resp_ok","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{ID: 1, Name: "OpenAI Responses", APIURL: upstream.URL, APIKey: "test-key", Enabled: true, Level: 1, APIFormat: claudeAPIFormatOpenAIResponse}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	body := `{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("第 %d 轮状态码=%d body=%s", i+1, w.Code, w.Body.String())
		}
	}
	if atomic.LoadInt32(&callCount) != 3 {
		t.Fatalf("upstream 调用次数=%d，期望 3", callCount)
	}
	if !strings.Contains(requestBodies[0], "prompt_cache_key") {
		t.Fatalf("首轮应先尝试自动 prompt_cache_key: %s", requestBodies[0])
	}
	if strings.Contains(requestBodies[1], "prompt_cache_key") {
		t.Fatalf("不支持 prompt_cache_key 后的回退重试应移除该字段: %s", requestBodies[1])
	}
	if strings.Contains(requestBodies[2], "prompt_cache_key") {
		t.Fatalf("同 provider/session 短期内应禁用自动 prompt_cache_key 注入: %s", requestBodies[2])
	}
}

func TestExtractUnsupportedOptionalParamsRequiresExplicitUnsupportedMeaning(t *testing.T) {
	provider := Provider{APIFormat: claudeAPIFormatOpenAIResponse}
	got := extractUnsupportedOptionalParams("claude", provider, http.StatusBadRequest, []byte(`{
		"error":{"message":"Unsupported parameters: temperature, reasoning.effort and top_p"}
	}`))
	if strings.Join(got, ",") != "reasoning,temperature,top_p" {
		t.Fatalf("不支持字段解析结果=%v", got)
	}

	got = extractUnsupportedOptionalParams("claude", provider, http.StatusBadRequest, []byte(`{
		"error":{"message":"temperature must be between 0 and 2","param":"temperature","type":"invalid_request_error"}
	}`))
	if len(got) != 0 {
		t.Fatalf("普通值校验错误不应触发删字段: %v", got)
	}

	got = extractUnsupportedOptionalParams("claude", provider, http.StatusBadRequest, []byte(`{
		"error":{"message":"Unsupported model. The request also contained temperature."}
	}`))
	if len(got) != 0 {
		t.Fatalf("字段未与不支持语义绑定时不应触发删除: %v", got)
	}

	got = extractUnsupportedOptionalParams("claude", provider, http.StatusBadRequest, []byte(`{
		"error":{"message":"Unsupported parameter: reasoning.effort. Supported parameters: temperature and top_p"}
	}`))
	if strings.Join(got, ",") != "reasoning" {
		t.Fatalf("支持字段不应被误判为不支持字段: %v", got)
	}
}

func TestClaudeResponsesUnsupportedOptionalParamsRetryOnceAndRemember(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	requestBodies := make([]string, 0, 3)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requestBodies = append(requestBodies, string(body))
		call := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported parameters: prompt_cache_key, temperature and reasoning.effort"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"resp_ok","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{
		ID:        71,
		Name:      "Optional Params",
		APIURL:    upstream.URL,
		APIKey:    "test-key",
		Enabled:   true,
		Level:     1,
		APIFormat: claudeAPIFormatOpenAIResponse,
		RequestBodyOverrides: map[string]interface{}{
			"temperature": float64(0.2),
			"output_config": map[string]interface{}{
				"effort": "high",
			},
		},
	}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	body := `{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("第 %d 轮状态码=%d body=%s", i+1, w.Code, w.Body.String())
		}
	}

	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Fatalf("上游调用次数=%d，期望首次重试一次、后续直接应用记忆，共 3 次", got)
	}
	if !strings.Contains(requestBodies[0], `"temperature"`) || !strings.Contains(requestBodies[0], `"reasoning"`) {
		t.Fatalf("首次请求应包含待探测参数: %s", requestBodies[0])
	}
	for i, requestBody := range requestBodies[1:] {
		if strings.Contains(requestBody, `"temperature"`) || strings.Contains(requestBody, `"reasoning"`) || strings.Contains(requestBody, `"prompt_cache_key"`) {
			t.Fatalf("第 %d 个兼容请求仍包含已拒绝字段: %s", i+2, requestBody)
		}
	}
}

func TestUnsupportedOptionalParamsMemoryKeyIncludesEndpointAndFormat(t *testing.T) {
	relay := &ProviderRelayService{unsupportedOptionalParams: make(map[string]unsupportedOptionalParamsMemory)}
	provider := Provider{ID: 9, Name: "same", APIURL: "https://example.com/", APIFormat: claudeAPIFormatOpenAIResponse}
	relay.rememberUnsupportedOptionalParams(provider, "/responses", []string{"temperature"})
	body := []byte(`{"temperature":0.2,"messages":[]}`)

	if got := relay.removeRememberedUnsupportedOptionalParams(provider, "/responses", body); gjson.GetBytes(got, "temperature").Exists() {
		t.Fatalf("相同能力键应移除已记忆字段: %s", got)
	}
	if got := relay.removeRememberedUnsupportedOptionalParams(provider, "/v1/responses", body); !gjson.GetBytes(got, "temperature").Exists() {
		t.Fatalf("不同最终端点不应共享能力记忆: %s", got)
	}
	provider.APIFormat = claudeAPIFormatOpenAIChat
	if got := relay.removeRememberedUnsupportedOptionalParams(provider, "/responses", body); !gjson.GetBytes(got, "temperature").Exists() {
		t.Fatalf("不同 API 格式不应共享能力记忆: %s", got)
	}
}

func TestUnsupportedOptionalParamsMemoryNormalizesAPIURLAndExpires(t *testing.T) {
	relay := &ProviderRelayService{unsupportedOptionalParams: make(map[string]unsupportedOptionalParamsMemory)}
	provider := Provider{ID: 10, Name: "same", APIURL: "HTTPS://EXAMPLE.COM:443/v1/", APIFormat: claudeAPIFormatOpenAIResponse}
	relay.rememberUnsupportedOptionalParams(provider, "/responses", []string{"temperature"})
	body := []byte(`{"temperature":0.2,"messages":[]}`)

	equivalentProvider := provider
	equivalentProvider.APIURL = "https://example.com/v1"
	if got := relay.removeRememberedUnsupportedOptionalParams(equivalentProvider, "/responses", body); gjson.GetBytes(got, "temperature").Exists() {
		t.Fatalf("等价 API URL 应共享能力记忆: %s", got)
	}

	key := relay.unsupportedOptionalParamsKey(equivalentProvider, "/responses")
	entry := relay.unsupportedOptionalParams[key]
	entry.ExpiresAt = time.Now().Add(-time.Minute)
	relay.unsupportedOptionalParams[key] = entry
	if got := relay.removeRememberedUnsupportedOptionalParams(equivalentProvider, "/responses", body); !gjson.GetBytes(got, "temperature").Exists() {
		t.Fatalf("过期能力记忆应恢复参数探测: %s", got)
	}
}

func TestClaudeChatUnsupportedOptionalParamsRetryOnceAndRemember(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	requestBodies := make([]string, 0, 3)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requestBodies = append(requestBodies, string(body))
		call := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":{"message":"Unknown parameters: temperature and reasoning_effort"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"chatcmpl_ok","model":"gpt-5.4","choices":[{"finish_reason":"stop","message":{"content":"ok"}}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{
		ID:        72,
		Name:      "Chat Optional Params",
		APIURL:    upstream.URL,
		APIKey:    "test-key",
		Enabled:   true,
		Level:     1,
		APIFormat: claudeAPIFormatOpenAIChat,
		RequestBodyOverrides: map[string]interface{}{
			"temperature": float64(0.2),
			"output_config": map[string]interface{}{
				"effort": "high",
			},
		},
	}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	body := `{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("第 %d 轮状态码=%d body=%s", i+1, w.Code, w.Body.String())
		}
	}

	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Fatalf("上游调用次数=%d，期望 3", got)
	}
	if !strings.Contains(requestBodies[0], `"temperature"`) || !strings.Contains(requestBodies[0], `"reasoning_effort"`) {
		t.Fatalf("首次 Chat 请求应包含待探测参数: %s", requestBodies[0])
	}
	for i, requestBody := range requestBodies[1:] {
		if strings.Contains(requestBody, `"temperature"`) || strings.Contains(requestBody, `"reasoning_effort"`) {
			t.Fatalf("第 %d 个 Chat 兼容请求仍包含已拒绝字段: %s", i+2, requestBody)
		}
	}
}

func TestUnsupportedOptionalParamsSecondFailureDoesNotRetryAgain(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if call == 1 {
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported parameter: temperature"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"error":{"message":"Unsupported parameter: top_p"}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{
		ID:        73,
		Name:      "Retry Once",
		APIURL:    upstream.URL,
		APIKey:    "test-key",
		Enabled:   true,
		Level:     1,
		APIFormat: claudeAPIFormatOpenAIChat,
		RequestBodyOverrides: map[string]interface{}{
			"temperature": float64(0.2),
			"top_p":       float64(0.9),
		},
	}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"gpt-4o","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Fatalf("兼容重试次数失控，上游调用=%d，期望 2", got)
	}
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "top_p") {
		t.Fatalf("应透传第二次最终错误，status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestClaudeResponsesContinuationDoesNotRetryToolOutputWithoutPreviousResponseID(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			_, _ = w.Write([]byte(`{"id":"resp_tool","status":"completed","model":"gpt-5.4","output":[{"type":"function_call","call_id":"toolu_1","name":"Read","arguments":"{}"}],"usage":{"input_tokens":2,"output_tokens":1}}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"previous_response_not_found","message":"previous response not found"}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{
		ID:        1,
		Name:      "OpenAI Responses",
		APIURL:    upstream.URL,
		APIKey:    "test-key",
		Enabled:   true,
		Level:     1,
		APIFormat: claudeAPIFormatOpenAIResponse,
	}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	firstBody := `{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"call tool"}]}`
	req1 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(firstBody))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("首轮状态码=%d body=%s", w1.Code, w1.Body.String())
	}

	toolResultBody := `{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"call tool"},{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}]}`
	req2 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(toolResultBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("工具结果续链失效应透传上游错误，状态码=%d body=%s", w2.Code, w2.Body.String())
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Fatalf("携带 function_call_output 时不应自动重放，upstream 调用次数=%d", callCount)
	}
}

func TestClaudeResponsesContinuationDoesNotRetryUnsafeToolOutputReplay(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			_, _ = w.Write([]byte(`{"id":"resp_tool","status":"completed","model":"gpt-5.4","output":[{"type":"function_call","call_id":"toolu_1","name":"Read","arguments":"{}"}],"usage":{"input_tokens":2,"output_tokens":1}}`))
			return
		}
		if call == 2 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"previous_response_not_found","message":"previous response not found"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"resp_after_retry","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"unsafe retry happened"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{
		ID:        1,
		Name:      "OpenAI Responses",
		APIURL:    upstream.URL,
		APIKey:    "test-key",
		Enabled:   true,
		Level:     1,
		APIFormat: claudeAPIFormatOpenAIResponse,
	}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	firstBody := `{"model":"gpt-5.4","max_tokens":16,"metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"call tool"}]}`
	req1 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(firstBody))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("首轮状态码=%d body=%s", w1.Code, w1.Body.String())
	}

	unsafeToolResultBody := `{"model":"gpt-5.4","max_tokens":16,"metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"call tool"},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}]}`
	req2 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(unsafeToolResultBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("缺少 function_call 配对时不应去掉 previous_response_id 回放，状态码=%d body=%s", w2.Code, w2.Body.String())
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Fatalf("unsafe 工具结果回放不应触发第三次请求，upstream 调用次数=%d", callCount)
	}
}

func TestClaudeResponsesContinuationTrimsToolOutputToLatestTurn(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	var secondRequestBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		call := atomic.AddInt32(&callCount, 1)
		if call == 2 {
			secondRequestBody = string(body)
		}
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			_, _ = w.Write([]byte(`{"id":"resp_tool","status":"completed","model":"gpt-5.4","output":[{"type":"function_call","call_id":"toolu_1","name":"Read","arguments":"{}"}],"usage":{"input_tokens":2,"output_tokens":1}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"resp_after_tool","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{
		ID:        1,
		Name:      "OpenAI Responses",
		APIURL:    upstream.URL,
		APIKey:    "test-key",
		Enabled:   true,
		Level:     1,
		APIFormat: claudeAPIFormatOpenAIResponse,
	}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	firstBody := `{"model":"gpt-5.4","max_tokens":16,"metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"call tool"}]}`
	req1 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(firstBody))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("首轮状态码=%d body=%s", w1.Code, w1.Body.String())
	}

	toolResultBody := `{"model":"gpt-5.4","max_tokens":16,"metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"call tool"},{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}]}`
	req2 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(toolResultBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("工具结果续链状态码=%d body=%s", w2.Code, w2.Body.String())
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Fatalf("upstream 调用次数=%d，期望 2", callCount)
	}
	if !strings.Contains(secondRequestBody, `"previous_response_id":"resp_tool"`) {
		t.Fatalf("第二轮请求未携带 previous_response_id: %s", secondRequestBody)
	}
	if got := gjson.Get(secondRequestBody, "input.#").Int(); got != 1 {
		t.Fatalf("工具结果续链应只发送最近 function_call_output，input.#=%d body=%s", got, secondRequestBody)
	}
	if got := gjson.Get(secondRequestBody, "input.0.type").String(); got != "function_call_output" {
		t.Fatalf("input.0.type=%q，期望 function_call_output，body=%s", got, secondRequestBody)
	}
	if strings.Contains(secondRequestBody, `"type":"function_call"`) {
		t.Fatalf("携带 previous_response_id 时不应重放历史 function_call: %s", secondRequestBody)
	}
}

func TestClaudeResponsesContinuationRetriesExpiredPreviousResponseIDWithToolPairs(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	var failedContinuationBody string
	var retryRequestBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		call := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		switch call {
		case 1:
			_, _ = w.Write([]byte(`{"id":"resp_tool","status":"completed","model":"gpt-5.4","output":[{"type":"function_call","call_id":"toolu_1","name":"Read","arguments":"{}"}],"usage":{"input_tokens":2,"output_tokens":1}}`))
		case 2:
			failedContinuationBody = string(body)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"previous_response_not_found","message":"previous response not found"}}`))
		default:
			retryRequestBody = string(body)
			_, _ = w.Write([]byte(`{"id":"resp_after_retry","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
		}
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{
		ID:        1,
		Name:      "OpenAI Responses",
		APIURL:    upstream.URL,
		APIKey:    "test-key",
		Enabled:   true,
		Level:     1,
		APIFormat: claudeAPIFormatOpenAIResponse,
	}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	firstBody := `{"model":"gpt-5.4","max_tokens":16,"metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"call tool"}]}`
	req1 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(firstBody))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("首轮状态码=%d body=%s", w1.Code, w1.Body.String())
	}

	toolResultBody := `{"model":"gpt-5.4","max_tokens":16,"metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"call tool"},{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}]}`
	req2 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(toolResultBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("previous_response_id 失效后工具结果回退重试应成功，状态码=%d body=%s", w2.Code, w2.Body.String())
	}
	if atomic.LoadInt32(&callCount) != 3 {
		t.Fatalf("upstream 调用次数=%d，期望 3", callCount)
	}
	if !strings.Contains(failedContinuationBody, `"previous_response_id":"resp_tool"`) {
		t.Fatalf("第二次请求应先尝试 previous_response_id: %s", failedContinuationBody)
	}
	if strings.Contains(retryRequestBody, "previous_response_id") {
		t.Fatalf("回退重试请求不应携带 previous_response_id: %s", retryRequestBody)
	}
	if got := gjson.Get(retryRequestBody, `input.#(type=="function_call").call_id`).String(); got != "toolu_1" {
		t.Fatalf("回退重试应保留 function_call 配对，got=%q body=%s", got, retryRequestBody)
	}
	if got := gjson.Get(retryRequestBody, `input.#(type=="function_call_output").call_id`).String(); got != "toolu_1" {
		t.Fatalf("回退重试应保留 function_call_output，got=%q body=%s", got, retryRequestBody)
	}
}

func TestClaudeResponsesStreamToolCallBindsContinuationWithoutDuplicateToolUse(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var callCount int32
	var secondRequestBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		call := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: response.created\n"))
			_, _ = w.Write([]byte(`data: {"response":{"id":"resp_stream","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":0}}}` + "\n\n"))
			_, _ = w.Write([]byte("event: response.output_item.added\n"))
			_, _ = w.Write([]byte(`data: {"output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"toolu_stream","name":"Read"}}` + "\n\n"))
			_, _ = w.Write([]byte("event: response.function_call_arguments.delta\n"))
			_, _ = w.Write([]byte(`data: {"output_index":0,"item_id":"fc_1","delta":"{\"file_path\":\"a.go\"}"}` + "\n\n"))
			_, _ = w.Write([]byte("event: response.function_call_arguments.done\n"))
			_, _ = w.Write([]byte(`data: {"output_index":0,"item_id":"fc_1"}` + "\n\n"))
			_, _ = w.Write([]byte("event: response.output_item.done\n"))
			_, _ = w.Write([]byte(`data: {"output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"toolu_stream","name":"Read","arguments":"{\"file_path\":\"a.go\"}"}}` + "\n\n"))
			_, _ = w.Write([]byte("event: response.done\n"))
			_, _ = w.Write([]byte(`data: {"response":{"status":"completed","usage":{"input_tokens":1,"output_tokens":1}}}` + "\n\n"))
			return
		}
		secondRequestBody = string(body)
		_, _ = w.Write([]byte(`{"id":"resp_after_tool","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{ID: 1, Name: "OpenAI Responses", APIURL: upstream.URL, APIKey: "test-key", Enabled: true, Level: 1, APIFormat: claudeAPIFormatOpenAIResponse}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	firstBody := `{"model":"gpt-5.4","max_tokens":16,"stream":true,"metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"call tool"}],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`
	req1 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(firstBody))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("首轮流式工具调用状态码=%d body=%s", w1.Code, w1.Body.String())
	}
	if got := strings.Count(w1.Body.String(), `"type":"tool_use"`); got != 1 {
		t.Fatalf("标准 Responses 工具流不应重复 tool_use，count=%d body=%s", got, w1.Body.String())
	}

	toolResultBody := `{"model":"gpt-5.4","max_tokens":16,"metadata":{"user_id":"` + claudeMetadataSessionA + `"},"messages":[{"role":"user","content":"call tool"},{"role":"assistant","content":[{"type":"tool_use","id":"toolu_stream","name":"Read","input":{"file_path":"a.go"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_stream","content":"ok"}]}],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`
	req2 := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(toolResultBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("工具结果续链状态码=%d body=%s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(secondRequestBody, `"previous_response_id":"resp_stream"`) {
		t.Fatalf("流式工具调用完成后下一轮应携带 previous_response_id: %s", secondRequestBody)
	}
}

func TestTrimClaudeResponsesInputToLatestTurnPreservesToolOutputImages(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.4",
		"input":[
			{"type":"message","role":"developer","content":[{"type":"input_text","text":"sys"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"old"}]},
			{"type":"function_call_output","call_id":"toolu_1","output":"ok"},
			{"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,abc"}]}
		]
	}`)

	trimmed := trimClaudeResponsesInputToLatestTurn(body)

	if got := gjson.GetBytes(trimmed, "input.#").Int(); got != 2 {
		t.Fatalf("裁剪后 input.#=%d，期望保留 function_call_output + image message，body=%s", got, string(trimmed))
	}
	if got := gjson.GetBytes(trimmed, "input.0.type").String(); got != "function_call_output" {
		t.Fatalf("input.0.type=%q，期望 function_call_output，body=%s", got, string(trimmed))
	}
	if got := gjson.GetBytes(trimmed, "input.1.content.0.type").String(); got != "input_image" {
		t.Fatalf("input.1 应保留工具结果图片，got=%q body=%s", got, string(trimmed))
	}
}

func TestTrimClaudeResponsesInputToLatestTurnPreservesParallelToolOutputImages(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.4",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,old"}]},
			{"type":"function_call_output","call_id":"toolu_1","output":"one"},
			{"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,one"}]},
			{"type":"function_call_output","call_id":"toolu_2","output":"two"},
			{"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,two"}]}
		]
	}`)

	trimmed := trimClaudeResponsesInputToLatestTurn(body)

	if got := gjson.GetBytes(trimmed, "input.#").Int(); got != 4 {
		t.Fatalf("裁剪后 input.#=%d，期望保留两组 function_call_output + image，body=%s", got, string(trimmed))
	}
	if got := gjson.GetBytes(trimmed, "input.0.call_id").String(); got != "toolu_1" {
		t.Fatalf("应从第一组工具结果开始保留，got=%q body=%s", got, string(trimmed))
	}
	if strings.Contains(string(trimmed), "data:image/png;base64,old") {
		t.Fatalf("工具结果前的普通图片不应被误归入工具结果裁剪: %s", string(trimmed))
	}
}

func TestTrimClaudeResponsesInputForTailReplayGuardPreservesToolPairs(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.4",
		"input":[
			{"type":"message","role":"developer","content":[{"type":"input_text","text":"sys"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"old"}]},
			{"type":"function_call","call_id":"toolu_1","name":"Read","arguments":"{}"},
			{"type":"function_call_output","call_id":"toolu_1","output":"one"},
			{"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,one"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"latest"}]}
		]
	}`)

	trimmed := trimClaudeResponsesInputForTailReplayGuard(body, 4)

	if got := gjson.GetBytes(trimmed, "input.#").Int(); got != 5 {
		t.Fatalf("tail replay guard 应允许超过预算以保留工具配对，input.#=%d body=%s", got, string(trimmed))
	}
	if got := gjson.GetBytes(trimmed, "input.0.role").String(); got != "developer" {
		t.Fatalf("developer 前缀应保留，got=%q body=%s", got, string(trimmed))
	}
	if got := gjson.GetBytes(trimmed, "input.1.type").String(); got != "function_call" {
		t.Fatalf("工具结果前的 function_call 应保留，got=%q body=%s", got, string(trimmed))
	}
	if strings.Contains(string(trimmed), `"text":"old"`) {
		t.Fatalf("旧普通消息应被裁剪: %s", string(trimmed))
	}
}

func TestClaudeResponsesPlanDoesNotTailTrimNormalRequestWithoutContinuation(t *testing.T) {
	messages := make([]interface{}, 0, 85)
	for i := 0; i < 85; i++ {
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": "msg",
		})
	}
	body, err := json.Marshal(map[string]interface{}{
		"model":      "gpt-5.4",
		"system":     "sys",
		"max_tokens": float64(16),
		"messages":   messages,
	})
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}

	plan, err := buildProviderRequestPlan(Provider{
		Name:      "OpenAI Responses",
		APIURL:    "https://example.com",
		APIKey:    "test-key",
		APIFormat: claudeAPIFormatOpenAIResponse,
	}, body, "/v1/messages", "gpt-5.4")
	if err != nil {
		t.Fatalf("buildProviderRequestPlan failed: %v", err)
	}

	if got := gjson.GetBytes(plan.BodyBytes, "input.#").Int(); got != 86 {
		t.Fatalf("无 previous_response_id 的正常请求不应 tail trim，input.#=%d body=%s", got, string(plan.BodyBytes))
	}
	if got := gjson.GetBytes(plan.ContinuationRetryBodyBytes, "input.#").Int(); got >= 86 {
		t.Fatalf("回退重试体应保留 tail replay guard，input.#=%d body=%s", got, string(plan.ContinuationRetryBodyBytes))
	}
}

func TestProxyHandlerAllProvidersFailedReturnsLastUpstreamErrorRaw(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var firstProviderCalls int32
	upstreamFail500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&firstProviderCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"provider-1 failed"}`))
	}))
	defer upstreamFail500.Close()

	var secondProviderCalls int32
	finalRawBody := `{"error":{"type":"rate_limit_error","message":"周消费超限","code":"rate_limit_exceeded","limit_type":"usd_weekly"}}`
	upstreamFail402 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&secondProviderCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Upstream-Trace", "trace-402")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(finalRawBody))
	}))
	defer upstreamFail402.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)

	providers := []Provider{
		{
			ID:      1,
			Name:    "Provider-L1-500",
			APIURL:  upstreamFail500.URL,
			APIKey:  "test-key-1",
			Enabled: true,
			Level:   1,
		},
		{
			ID:      2,
			Name:    "Provider-L1-402",
			APIURL:  upstreamFail402.URL,
			APIKey:  "test-key-2",
			Enabled: true,
			Level:   1,
		},
	}
	if err := providerService.SaveProviders("codex", providers); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}

	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	reqBody := `{"model":"claude-opus-4-6","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if atomic.LoadInt32(&firstProviderCalls) != 1 {
		t.Fatalf("第一个 provider 调用次数 = %d, 期望 1", atomic.LoadInt32(&firstProviderCalls))
	}
	if atomic.LoadInt32(&secondProviderCalls) != 1 {
		t.Fatalf("第二个 provider 调用次数 = %d, 期望 1", atomic.LoadInt32(&secondProviderCalls))
	}

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("状态码 = %d, 期望 %d, body=%s", w.Code, http.StatusPaymentRequired, w.Body.String())
	}

	if got := strings.TrimSpace(w.Body.String()); got != finalRawBody {
		t.Fatalf("响应体不匹配\ngot:  %s\nwant: %s", got, finalRawBody)
	}

	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("Content-Type = %q, 期望包含 application/json", got)
	}

	if got := w.Header().Get("X-Upstream-Trace"); got != "trace-402" {
		t.Fatalf("X-Upstream-Trace = %q, 期望 trace-402", got)
	}
}

func TestProxyHandlerSingle502CountsOnceAtThresholdNineAndPersistsDiagnostic(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}
	if _, err := db.Exec(`UPDATE app_settings SET value = 'true' WHERE key = 'enable_blacklist'`); err != nil {
		t.Fatalf("开启黑名单失败: %v", err)
	}
	if _, err := db.Exec(`UPDATE app_settings SET value = '9' WHERE key = 'blacklist_failure_threshold'`); err != nil {
		t.Fatalf("设置拉黑阈值失败: %v", err)
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

	var failedProviderCalls int32
	failedUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&failedProviderCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"temporary upstream failure","api_key":"secret-value"}}`))
	}))
	defer failedUpstream.Close()

	var fallbackProviderCalls int32
	fallbackUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fallbackProviderCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_ok","status":"completed"}`))
	}))
	defer fallbackUpstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	providers := []Provider{
		{ID: 1, Name: "Failed Once", APIURL: failedUpstream.URL, APIKey: "test-key-1", Enabled: true, Level: 1},
		{ID: 2, Name: "Fallback", APIURL: fallbackUpstream.URL, APIKey: "test-key-2", Enabled: true, Level: 1},
	}
	if err := providerService.SaveProviders("codex", providers); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}

	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)
	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5.3-codex","input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("降级请求状态码=%d body=%s", w.Code, w.Body.String())
	}
	if got := atomic.LoadInt32(&failedProviderCalls); got != 1 {
		t.Fatalf("失败 provider 调用次数=%d，期望 1", got)
	}
	if got := atomic.LoadInt32(&fallbackProviderCalls); got != 1 {
		t.Fatalf("后备 provider 调用次数=%d，期望 1", got)
	}

	var failureCount int
	if err := db.QueryRow(`SELECT failure_count FROM provider_blacklist WHERE platform = 'codex' AND provider_id = '1'`).Scan(&failureCount); err != nil {
		t.Fatalf("读取失败计数失败: %v", err)
	}
	if failureCount != 1 {
		t.Fatalf("单次 502 应只累计一次失败，当前=%d，期望 1/9", failureCount)
	}
	var blacklistedUntil sql.NullTime
	if err := db.QueryRow(`SELECT blacklisted_until FROM provider_blacklist WHERE platform = 'codex' AND provider_id = '1'`).Scan(&blacklistedUntil); err != nil {
		t.Fatalf("读取拉黑状态失败: %v", err)
	}
	if blacklistedUntil.Valid {
		t.Fatalf("单次 502 不应在阈值 9 时拉黑，blacklisted_until=%v", blacklistedUntil.Time)
	}

	var httpCode int
	var errorMessage string
	var errorSource string
	if err := db.QueryRow(`SELECT http_code, error_message, error_source FROM request_log WHERE platform = 'codex' AND provider_id = '1' ORDER BY id DESC LIMIT 1`).Scan(&httpCode, &errorMessage, &errorSource); err != nil {
		t.Fatalf("读取失败请求日志失败: %v", err)
	}
	if httpCode != http.StatusBadGateway || !strings.Contains(errorMessage, "status 502") || !strings.Contains(errorMessage, "temporary upstream failure") {
		t.Fatalf("失败请求诊断未正确落库: http_code=%d error_message=%q", httpCode, errorMessage)
	}
	if strings.Contains(errorMessage, "secret-value") || !strings.Contains(errorMessage, requestLogPayloadRedactedValue) {
		t.Fatalf("失败摘要未脱敏: %q", errorMessage)
	}
	if errorSource != requestErrorSourceProviderResponse {
		t.Fatalf("错误来源=%q，期望 %q", errorSource, requestErrorSourceProviderResponse)
	}
}

func TestProxyHandlerSingleProvider503ReturnsRawUpstreamError(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var providerCalls int32
	finalRawBody := "{\"error\":{\"type\":\"new_api_error\",\"message\":\"当前模型 claude-opus-4-6 负载已经达到上限，请稍后重试 (request id:\\n     202603121813186562816676KXPmIKr)\"},\"type\":\"error\"}"
	upstreamFail503 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&providerCalls, 1)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Upstream-Trace", "trace-503")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(finalRawBody))
	}))
	defer upstreamFail503.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)

	providers := []Provider{
		{
			ID:      1,
			Name:    "Any Router",
			APIURL:  upstreamFail503.URL,
			APIKey:  "test-key-503",
			Enabled: true,
			Level:   1,
		},
	}
	if err := providerService.SaveProviders("codex", providers); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}

	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	reqBody := `{"model":"claude-opus-4-6","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if atomic.LoadInt32(&providerCalls) != 1 {
		t.Fatalf("provider 调用次数 = %d, 期望 1", atomic.LoadInt32(&providerCalls))
	}

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("状态码 = %d, 期望 %d, body=%s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}

	if got := strings.TrimSpace(w.Body.String()); got != finalRawBody {
		t.Fatalf("响应体不匹配\ngot:  %s\nwant: %s", got, finalRawBody)
	}

	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("Content-Type = %q, 期望包含 application/json", got)
	}

	if got := w.Header().Get("X-Upstream-Trace"); got != "trace-503" {
		t.Fatalf("X-Upstream-Trace = %q, 期望 trace-503", got)
	}
}

func TestProxyHandlerNetworkErrorStillReturns502Summary(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)

	providers := []Provider{
		{
			ID:      1,
			Name:    "BrokenProvider",
			APIURL:  "http://127.0.0.1:1",
			APIKey:  "test-key",
			Enabled: true,
			Level:   1,
		},
	}
	if err := providerService.SaveProviders("codex", providers); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}

	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	reqBody := `{"model":"claude-opus-4-6","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("状态码 = %d, 期望 %d, body=%s", w.Code, http.StatusBadGateway, w.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("响应不是合法 JSON: %v, body=%s", err, w.Body.String())
	}
	if _, ok := payload["error"]; !ok {
		t.Fatalf("响应缺少 error 字段: %v", payload)
	}
}

func TestClaudeResponsesNonStreamFailedStatusTriggersProviderFailover(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var failedProviderCalls int32
	upstreamFailed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&failedProviderCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_failed","status":"failed","model":"gpt-5.4","error":{"message":"boom"},"output":[],"usage":{"input_tokens":2,"output_tokens":0}}`))
	}))
	defer upstreamFailed.Close()

	var successProviderCalls int32
	upstreamSuccess := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&successProviderCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_ok","status":"completed","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok from fallback"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
	}))
	defer upstreamSuccess.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	providers := []Provider{
		{ID: 1, Name: "Failed Responses", APIURL: upstreamFailed.URL, APIKey: "test-key-1", Enabled: true, Level: 1, APIFormat: claudeAPIFormatOpenAIResponse},
		{ID: 2, Name: "Fallback Responses", APIURL: upstreamSuccess.URL, APIKey: "test-key-2", Enabled: true, Level: 1, APIFormat: claudeAPIFormatOpenAIResponse},
	}
	if err := providerService.SaveProviders("claude", providers); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	reqBody := `{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("failed status 应触发 provider failover，状态码=%d body=%s", w.Code, w.Body.String())
	}
	if atomic.LoadInt32(&failedProviderCalls) != 1 {
		t.Fatalf("failed provider 调用次数=%d，期望 1", failedProviderCalls)
	}
	if atomic.LoadInt32(&successProviderCalls) != 1 {
		t.Fatalf("fallback provider 调用次数=%d，期望 1", successProviderCalls)
	}
	if !strings.Contains(w.Body.String(), "ok from fallback") {
		t.Fatalf("响应应来自 fallback provider: %s", w.Body.String())
	}
}

func TestClaudeResponsesNonStreamIncompleteWithOutputReturnsMaxTokens(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var providerCalls int32
	upstreamIncomplete := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&providerCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_incomplete","status":"incomplete","model":"gpt-5.4","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","content":[{"type":"output_text","text":"partial"}]}],"usage":{"input_tokens":2,"output_tokens":1}}`))
	}))
	defer upstreamIncomplete.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	provider := Provider{ID: 1, Name: "Incomplete Responses", APIURL: upstreamIncomplete.URL, APIKey: "test-key", Enabled: true, Level: 1, APIFormat: claudeAPIFormatOpenAIResponse}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	reqBody := `{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("带输出的 incomplete 应作为 max_tokens 成功返回，状态码=%d body=%s", w.Code, w.Body.String())
	}
	if atomic.LoadInt32(&providerCalls) != 1 {
		t.Fatalf("provider 调用次数=%d，期望 1", providerCalls)
	}
	if got := gjson.Get(w.Body.String(), "stop_reason").String(); got != "max_tokens" {
		t.Fatalf("stop_reason=%q，期望 max_tokens，body=%s", got, w.Body.String())
	}
	if got := gjson.Get(w.Body.String(), "content.0.text").String(); got != "partial" {
		t.Fatalf("content.0.text=%q，期望 partial，body=%s", got, w.Body.String())
	}
}

func TestForwardRequestCodexIncompleteStreamReturnsStartedError(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"type\":\"response.created\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n"))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	ok, err := relay.forwardRequest(
		ctx,
		"codex",
		Provider{Name: "BrokenStream", APIURL: upstream.URL, APIKey: "test-key", Enabled: true},
		"/responses",
		map[string]string{},
		map[string]string{"Content-Type": "application/json"},
		[]byte(`{"model":"gpt-5.3-codex","stream":true,"input":"hello"}`),
		true,
		"gpt-5.3-codex",
		"gpt-5.3-codex",
	)

	if ok {
		t.Fatalf("缺少 response.completed 的流式响应不应判定为成功")
	}
	if !errors.Is(err, errIncompleteStream) {
		t.Fatalf("错误 = %v, 期望包含 errIncompleteStream", err)
	}
	if !errors.Is(err, errResponseStarted) {
		t.Fatalf("错误 = %v, 期望包含 errResponseStarted", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("写给客户端的状态码 = %d, 期望 200", w.Code)
	}
	if got := w.Body.String(); !strings.Contains(got, "response.output_text.delta") {
		t.Fatalf("响应体未透传上游流内容: %s", got)
	}
}

func TestForwardRequestClaudeResponsesFailedStreamReturnsStartedError(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: response.created\n"))
		_, _ = w.Write([]byte("data: {\"response\":{\"id\":\"resp_1\",\"model\":\"gpt-5.4\",\"usage\":{\"input_tokens\":1,\"output_tokens\":0}}}\n\n"))
		_, _ = w.Write([]byte("event: response.output_text.delta\n"))
		_, _ = w.Write([]byte("data: {\"item_id\":\"msg_1\",\"content_index\":0,\"delta\":\"hello\"}\n\n"))
		_, _ = w.Write([]byte("event: response.failed\n"))
		_, _ = w.Write([]byte("data: {\"response\":{\"status\":\"failed\",\"error\":{\"message\":\"boom\"}}}\n\n"))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	ok, err := relay.forwardRequest(
		ctx,
		"claude",
		Provider{Name: "FailedResponsesStream", APIURL: upstream.URL, APIKey: "test-key", Enabled: true, APIFormat: claudeAPIFormatOpenAIResponse},
		"/responses",
		map[string]string{},
		map[string]string{"Content-Type": "application/json"},
		[]byte(`{"model":"gpt-5.4","stream":true,"input":"hello"}`),
		true,
		"gpt-5.4",
		"gpt-5.4",
	)

	if ok {
		t.Fatalf("response.failed 的 Claude Responses 转换流不应判定为成功")
	}
	if !errors.Is(err, errIncompleteStream) {
		t.Fatalf("错误 = %v, 期望包含 errIncompleteStream", err)
	}
	if !errors.Is(err, errResponseStarted) {
		t.Fatalf("错误 = %v, 期望包含 errResponseStarted", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("写给客户端的状态码 = %d, 期望 200", w.Code)
	}
	if got := w.Body.String(); !strings.Contains(got, "message_stop") {
		t.Fatalf("转换流应向 Claude 侧收尾，响应体=%s", got)
	}
}

func TestForwardRequestCodexStreamAcceptsCompletedJSONFallback(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"type":"response","status":"completed","response":{"usage":{"input_tokens":12,"output_tokens":3}}}`))
	}))
	defer upstream.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	ok, err := relay.forwardRequest(
		ctx,
		"codex",
		Provider{Name: "JSONFallback", APIURL: upstream.URL, APIKey: "test-key", Enabled: true},
		"/responses",
		map[string]string{},
		map[string]string{"Content-Type": "application/json"},
		[]byte(`{"model":"gpt-5.3-codex","stream":true,"input":"hello"}`),
		true,
		"gpt-5.3-codex",
		"gpt-5.3-codex",
	)

	if !ok {
		t.Fatalf("完整 JSON fallback 应视为成功，当前错误: %v", err)
	}
	if err != nil {
		t.Fatalf("成功请求不应返回错误: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("写给客户端的状态码 = %d, 期望 200", w.Code)
	}
	if got := strings.TrimSpace(w.Body.String()); !strings.Contains(got, `"status":"completed"`) {
		t.Fatalf("响应体未透传完整 JSON fallback: %s", got)
	}
}

func TestProxyHandlerIncompleteCodexStreamDoesNotFallbackAfterResponseStarted(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var brokenProviderCalls int32
	upstreamBroken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&brokenProviderCalls, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"type\":\"response.created\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n"))
	}))
	defer upstreamBroken.Close()

	var fallbackProviderCalls int32
	upstreamCompleted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fallbackProviderCalls, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"type\":\"response.created\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\"}\n\n"))
	}))
	defer upstreamCompleted.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)

	providers := []Provider{
		{
			ID:      1,
			Name:    "BrokenStream",
			APIURL:  upstreamBroken.URL,
			APIKey:  "test-key-1",
			Enabled: true,
			Level:   1,
		},
		{
			ID:      2,
			Name:    "FallbackShouldNotRun",
			APIURL:  upstreamCompleted.URL,
			APIKey:  "test-key-2",
			Enabled: true,
			Level:   1,
		},
	}
	if err := providerService.SaveProviders("codex", providers); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}

	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	reqBody := `{"model":"gpt-5.3-codex","stream":true,"input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if atomic.LoadInt32(&brokenProviderCalls) != 1 {
		t.Fatalf("异常 provider 调用次数 = %d, 期望 1", atomic.LoadInt32(&brokenProviderCalls))
	}
	if atomic.LoadInt32(&fallbackProviderCalls) != 0 {
		t.Fatalf("fallback provider 调用次数 = %d, 期望 0", atomic.LoadInt32(&fallbackProviderCalls))
	}
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", w.Code)
	}
	if got := w.Body.String(); !strings.Contains(got, "response.output_text.delta") {
		t.Fatalf("响应体未保留首个 provider 的流内容: %s", got)
	}
}

func TestProxyHandlerEmptyCodexStreamFallsBackBeforeResponseStarted(t *testing.T) {
	useIsolatedHomeDir(t)
	gin.SetMode(gin.TestMode)
	disableBlacklistForTest(t)

	var emptyProviderCalls int32
	upstreamEmpty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&emptyProviderCalls, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstreamEmpty.Close()

	var fallbackProviderCalls int32
	upstreamCompleted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fallbackProviderCalls, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"type\":\"response.created\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"))
	}))
	defer upstreamCompleted.Close()

	providerService := NewProviderService()
	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	providers := []Provider{
		{ID: 1, Name: "EmptyStream", APIURL: upstreamEmpty.URL, APIKey: "test-key-1", Enabled: true, Level: 1},
		{ID: 2, Name: "FallbackCompleted", APIURL: upstreamCompleted.URL, APIKey: "test-key-2", Enabled: true, Level: 1},
	}
	if err := providerService.SaveProviders("codex", providers); err != nil {
		t.Fatalf("保存 providers 失败: %v", err)
	}

	relay := NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	relay.registerRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5.3-codex","stream":true,"input":[{"type":"context_compaction"}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if atomic.LoadInt32(&emptyProviderCalls) != 1 || atomic.LoadInt32(&fallbackProviderCalls) != 1 {
		t.Fatalf("provider 调用次数异常: empty=%d fallback=%d", emptyProviderCalls, fallbackProviderCalls)
	}
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "response.completed") {
		t.Fatalf("应仅返回 fallback 完整流，status=%d body=%s", w.Code, w.Body.String())
	}
}
