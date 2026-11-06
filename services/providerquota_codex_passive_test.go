/**
 * @name: Codex 被动额度采样测试
 * @Descripttion: 验证 Codex 窗口识别、样本有效性、持久化及代理接入。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-06 17:48:46
 * @LastEditTime: 2026-11-06 17:48:46
 * @FilePath: services/providerquota_codex_passive_test.go
 */
package services

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
	"github.com/gin-gonic/gin"
)

func newMemoryCodexPassiveQuotaService(t *testing.T) *ProviderQuotaQueryService {
	t.Helper()
	return &ProviderQuotaQueryService{
		client: &http.Client{Transport: quotaAutomationRoundTripperFunc(func(*http.Request) (*http.Response, error) {
			t.Error("被动额度查询不应发送 HTTP 请求")
			return nil, fmt.Errorf("unexpected HTTP request")
		})},
		codexPassive: &codexPassiveQuotaStore{snapshots: make(map[string]codexPassiveQuotaSnapshot)},
	}
}

func codexPassiveTestHeaders() http.Header {
	headers := make(http.Header)
	headers.Set("x-codex-primary-used-percent", "0")
	headers.Set("x-codex-primary-reset-after-seconds", "3600")
	headers.Set("x-codex-primary-window-minutes", "300")
	headers.Set("x-codex-secondary-used-percent", "42")
	headers.Set("x-codex-secondary-reset-after-seconds", "259200")
	headers.Set("x-codex-secondary-window-minutes", "10080")
	return headers
}

func TestCodexPassiveQuotaSamplesWindowsWithoutHTTP(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		t.Run(fmt.Sprintf("reversed=%t", reversed), func(t *testing.T) {
			service := newMemoryCodexPassiveQuotaService(t)
			now := time.Now().Truncate(time.Second)
			headers := codexPassiveTestHeaders()
			labels := []string{"5h", "7d"}
			if reversed {
				headers.Set("x-codex-primary-window-minutes", "10080")
				headers.Set("x-codex-secondary-window-minutes", "300")
				labels = []string{"7d", "5h"}
			}
			service.sampleCodexPassiveQuota("https://example.com/", "sample-key", headers, now)
			result := service.queryQuotaContext(context.Background(), "token_plan_codex_passive", "https://example.com", "sample-key", nil)
			if !result.Success || len(result.Items) != 2 {
				t.Fatalf("被动采样查询失败: %+v", result)
			}
			for index, used := range []float64{0, 42} {
				item := result.Items[index]
				if item.Used != used || item.Total != 100 || item.Unit != "%" || item.Label != labels[index] || !item.Active {
					t.Fatalf("窗口 %d 解析错误: %+v", index, item)
				}
			}
			if result.Items[0].NextReset != now.Add(time.Hour).UTC().Format(time.RFC3339) {
				t.Fatalf("相对重置时间转换错误: %+v", result.Items[0])
			}
			for _, identity := range [][2]string{{"https://example.com", "other-key"}, {"https://other.example.com", "sample-key"}} {
				result := service.QueryQuota("token_plan_codex_passive", identity[0], identity[1], nil)
				if len(result.Items) != 1 || result.Items[0].Active || result.Items[0].InvalidMessage != codexPassiveQuotaPending {
					t.Fatal("不同地址或密钥之间不应共用样本")
				}
			}
		})
	}
}

func TestCodexPassiveQuotaIgnoresInvalidHeadersAndPreservesPartialWindows(t *testing.T) {
	service := newMemoryCodexPassiveQuotaService(t)
	now := time.Now().Truncate(time.Second)
	service.sampleCodexPassiveQuota("https://example.com", "key", codexPassiveTestHeaders(), now)
	for _, invalid := range []string{"NaN", "+Inf", "-1", "bad", "", "1e999"} {
		headers := make(http.Header)
		headers.Set("x-codex-primary-used-percent", invalid)
		headers.Set("x-codex-primary-window-minutes", "0")
		headers.Set("x-codex-primary-reset-after-seconds", "999999999999999999")
		service.sampleCodexPassiveQuota("https://example.com", "key", headers, now)
	}
	headers := make(http.Header)
	headers.Set("x-codex-secondary-used-percent", "105")
	service.sampleCodexPassiveQuota("https://example.com", "key", headers, now.Add(time.Minute))
	items, _ := service.queryCodexPassiveQuota("https://example.com", "key", now.Add(time.Minute))
	if len(items) != 2 || items[0].Used != 0 || items[1].Used != 105 || items[1].Label != "7d" {
		t.Fatalf("部分窗口更新或无效响应头破坏了已有样本: %+v", items)
	}
	if items[1].NextReset != now.Add(72*time.Hour).UTC().Format(time.RFC3339) {
		t.Fatalf("缺少重置响应头时不应推迟重置时间: %+v", items[1])
	}
}

func TestCodexPassiveQuotaDoesNotInventUsageAfterResetOrWindowChange(t *testing.T) {
	service := newMemoryCodexPassiveQuotaService(t)
	now := time.Now().Truncate(time.Second)
	service.sampleCodexPassiveQuota("https://example.com", "key", codexPassiveTestHeaders(), now)
	headers := make(http.Header)
	headers.Set("x-codex-primary-reset-after-seconds", "3301")
	service.sampleCodexPassiveQuota("https://example.com", "key", headers, now.Add(5*time.Minute))
	items, _ := service.queryCodexPassiveQuota("https://example.com", "key", now.Add(5*time.Minute))
	if len(items) != 2 || !items[0].Active {
		t.Fatalf("同一窗口的倒计时取整不应丢失有效使用率: %+v", items)
	}
	items, _ = service.queryCodexPassiveQuota("https://example.com", "key", now.Add(2*time.Hour))
	if items[0].Active || items[0].InvalidMessage != codexPassiveQuotaStale || !items[1].Active {
		t.Fatalf("过期窗口应等待新样本，其他窗口应继续有效: %+v", items)
	}
	headers.Set("x-codex-primary-reset-after-seconds", "18000")
	service.sampleCodexPassiveQuota("https://example.com", "key", headers, now.Add(2*time.Hour))
	items, _ = service.queryCodexPassiveQuota("https://example.com", "key", now.Add(2*time.Hour))
	if len(items) != 1 || items[0].Key != "codex_secondary" {
		t.Fatalf("新窗口尚无使用率时不能继承旧样本: %+v", items)
	}
	headers.Set("x-codex-primary-used-percent", "10")
	service.sampleCodexPassiveQuota("https://example.com", "key", headers, now.Add(2*time.Hour))
	items, _ = service.queryCodexPassiveQuota("https://example.com", "key", now.Add(2*time.Hour))
	if len(items) != 2 || items[0].Used != 10 || !items[0].Active {
		t.Fatalf("新样本未恢复窗口: %+v", items)
	}
	headers.Del("x-codex-primary-used-percent")
	headers.Del("x-codex-primary-reset-after-seconds")
	headers.Set("x-codex-primary-window-minutes", "10080")
	service.sampleCodexPassiveQuota("https://example.com", "key", headers, now.Add(2*time.Hour))
	items, _ = service.queryCodexPassiveQuota("https://example.com", "key", now.Add(2*time.Hour))
	if len(items) != 1 || items[0].Key != "codex_secondary" {
		t.Fatalf("窗口长度变化时不能继承旧窗口使用率: %+v", items)
	}
}

func TestCodexPassiveQuotaUnknownWindowAndMissingResetExpire(t *testing.T) {
	service := newMemoryCodexPassiveQuotaService(t)
	now := time.Now().Truncate(time.Second)
	service.sampleCodexPassiveQuota("https://example.com", "key", http.Header{}, now)
	items, _ := service.queryCodexPassiveQuota("https://example.com", "key", now)
	if len(items) != 1 || items[0].Active || items[0].InvalidMessage != codexPassiveQuotaPending {
		t.Fatalf("缺少响应头时应等待采样: %+v", items)
	}
	headers := make(http.Header)
	headers.Set("x-codex-primary-used-percent", "37.5")
	service.sampleCodexPassiveQuota("https://example.com", "key", headers, now)
	items, _ = service.queryCodexPassiveQuota("https://example.com", "key", now)
	if len(items) != 1 || items[0].Used != 37.5 || items[0].Label != "" || items[0].NextReset != "" || !items[0].Active {
		t.Fatalf("缺少窗口信息时不应臆造周期或重置时间: %+v", items)
	}
	items, _ = service.queryCodexPassiveQuota("https://example.com", "key", now.Add(5*time.Hour))
	if items[0].Active || items[0].InvalidMessage != codexPassiveQuotaStale {
		t.Fatalf("缺少重置信息的旧样本不能永久有效: %+v", items)
	}
}

func TestCodexPassiveQuotaPersistsAndReloadsWithoutCredentials(t *testing.T) {
	useIsolatedHomeDir(t)
	service := NewProviderQuotaQueryService()
	now := time.Now().Truncate(time.Second)
	service.sampleCodexPassiveQuota("https://private.example.com/private-path", "private-sample-key", codexPassiveTestHeaders(), now)
	service.sampleClaudePassiveQuota("https://private.example.com/private-path", "private-sample-key", claudePassiveTestHeaders(now), now)
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	var key, raw string
	if err := db.QueryRow("SELECT cache_key, snapshot FROM provider_codex_quota_samples").Scan(&key, &raw); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private.example.com", "private-path", "private-sample-key"} {
		if strings.Contains(key+raw, secret) {
			t.Fatalf("采样表不应保存认证或私有地址: %s", secret)
		}
	}
	reloaded := NewProviderQuotaQueryService()
	for queryType, count := range map[string]int{"token_plan_codex_passive": 2, "token_plan_claude_passive": 3} {
		result := reloaded.QueryQuota(queryType, "https://private.example.com/private-path", "private-sample-key", nil)
		if !result.Success || len(result.Items) != count || result.Items[1].Used != 42 {
			t.Fatalf("重建服务后未隔离恢复 %s 样本: %+v", queryType, result)
		}
	}
}

func TestCodexPassiveQuotaConcurrentReadsAndSamples(t *testing.T) {
	service := newMemoryCodexPassiveQuotaService(t)
	now := time.Now().Truncate(time.Second)
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			service.sampleCodexPassiveQuota("https://example.com", "key", codexPassiveTestHeaders(), now)
			service.queryQuotaContext(context.Background(), "token_plan_codex_passive", "https://example.com", "key", nil)
		}()
	}
	group.Wait()
}

func TestCodexPassiveQuotaRelayCapturesSuccessStreamAnd429(t *testing.T) {
	for _, mode := range []string{"json", "stream", "429", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			useIsolatedHomeDir(t)
			gin.SetMode(gin.TestMode)
			var calls atomic.Int64
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("x-codex-primary-used-percent", "25")
				w.Header().Set("x-codex-primary-reset-after-seconds", "3600")
				w.Header().Set("x-codex-primary-window-minutes", "300")
				if mode == "429" {
					w.WriteHeader(http.StatusTooManyRequests)
					fmt.Fprint(w, `{"error":{"type":"rate_limit_error","message":"limited"}}`)
				} else if mode == "stream" {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n")
				} else {
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"id":"resp_sample","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`)
				}
			}))
			defer upstream.Close()
			providers := NewProviderService()
			defer providers.Stop()
			relay := NewProviderRelayService(providers, nil, nil, nil, nil, nil, "")
			query := newMemoryCodexPassiveQuotaService(t)
			relay.BindProviderQuotaAutomationService(&ProviderQuotaAutomationService{queryService: query})
			provider := Provider{ID: 1, Name: "Codex Passive", APIURL: upstream.URL, APIKey: "sample-key", Enabled: true,
				ProviderQuotaQueryType: "token_plan_codex_passive"}
			if mode == "disabled" {
				provider.ProviderQuotaQueryConfig = &ProviderQuotaQueryConfig{Enabled: false, TemplateType: "token_plan", TokenPlanProvider: "codex_passive"}
			}
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/responses", nil)
			ok, err := relay.forwardRequest(ctx, "codex", provider, "/responses", nil,
				map[string]string{"Content-Type": "application/json"}, []byte(`{"model":"gpt-5-codex","input":[]}`), mode == "stream", "gpt-5-codex", "gpt-5-codex")
			if mode != "429" && (!ok || err != nil) {
				t.Fatalf("转发失败: %v", err)
			}
			if mode == "429" && (ok || err == nil) {
				t.Fatal("限流响应应保留转发错误")
			}
			result := query.QueryQuota("token_plan_codex_passive", upstream.URL, "sample-key", nil)
			if len(result.Items) != 1 || calls.Load() != 1 {
				t.Fatalf("采样或请求次数错误: result=%+v calls=%d", result, calls.Load())
			}
			if mode == "disabled" {
				if result.Items[0].InvalidMessage != codexPassiveQuotaPending {
					t.Fatalf("关闭额度查询时不应采样: %+v", result)
				}
			} else if result.Items[0].Used != 25 || result.Items[0].Label != "5h" {
				t.Fatalf("未采集 Codex 响应头: %+v", result)
			}
		})
	}
}

func TestCodexPassiveQuotaDoesNotAutoDisableProvider(t *testing.T) {
	var remaining atomic.Int64
	automation, providers, _, _ := newQuotaAutomationServiceForTest(t, true, &remaining)
	defer providers.Stop()
	query := newMemoryCodexPassiveQuotaService(t)
	automation.queryService = query
	provider := Provider{ID: 1, Name: "Codex Passive", APIURL: "https://example.com", APIKey: "sample-key", Enabled: true,
		ProviderQuotaQueryType: "token_plan_codex_passive"}
	if err := providers.SaveProviders("codex", []Provider{provider}); err != nil {
		t.Fatal(err)
	}
	headers := make(http.Header)
	headers.Set("x-codex-primary-used-percent", "100")
	query.sampleCodexPassiveQuota(provider.APIURL, provider.APIKey, headers, time.Now())
	result := automation.checkProviderQuota("codex", "1")
	if !result.Success || !result.ProviderEnabled || result.QuotaAutoDisabled || result.StateChanged {
		t.Fatalf("被动模式不应自动停用供应商: %+v", result)
	}
}
