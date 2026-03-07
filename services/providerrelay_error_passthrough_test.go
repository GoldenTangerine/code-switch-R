package services

import (
	"encoding/json"
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
