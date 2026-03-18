package services

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/daodao97/xgo/xdb"
	"github.com/gin-gonic/gin"
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
