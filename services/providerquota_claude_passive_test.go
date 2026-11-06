/**
 * @name: Claude 被动额度采样测试
 * @Descripttion: 验证响应采样、窗口有效性、隔离持久化与代理接入。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-06 16:35:58
 * @LastEditTime: 2026-11-06 16:35:58
 * @FilePath: services/providerquota_claude_passive_test.go
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

func newMemoryClaudePassiveQuotaService(t *testing.T) *ProviderQuotaQueryService {
	t.Helper()
	return &ProviderQuotaQueryService{
		client: &http.Client{Transport: quotaAutomationRoundTripperFunc(func(*http.Request) (*http.Response, error) {
			t.Error("被动额度查询不应发送 HTTP 请求")
			return nil, fmt.Errorf("unexpected HTTP request")
		})},
		claudePassive: &claudePassiveQuotaStore{snapshots: make(map[string]claudePassiveQuotaSnapshot)},
	}
}

func claudePassiveTestHeaders(now time.Time) http.Header {
	headers := make(http.Header)
	headers.Set("anthropic-ratelimit-unified-5h-utilization", "0")
	headers.Set("anthropic-ratelimit-unified-5h-reset", fmt.Sprint(now.Add(time.Hour).UnixMilli()))
	headers.Set("anthropic-ratelimit-unified-7d-utilization", "0.42")
	headers.Set("anthropic-ratelimit-unified-7d-reset", fmt.Sprint(now.Add(3*24*time.Hour).Unix()))
	headers.Set("anthropic-ratelimit-unified-7d_oi-utilization", "1.05")
	headers.Set("anthropic-ratelimit-unified-7d_oi-reset", fmt.Sprint(now.Add(3*24*time.Hour).Unix()))
	return headers
}

func TestClaudePassiveQuotaSamplesWindowsWithoutHTTP(t *testing.T) {
	service := newMemoryClaudePassiveQuotaService(t)
	now := time.Now().Truncate(time.Second)
	service.sampleClaudePassiveQuota("https://example.com/", "sample-key", claudePassiveTestHeaders(now), now)
	result := service.queryQuotaContext(context.Background(), "token_plan_claude_passive", "https://example.com", "sample-key", nil)
	if !result.Success || len(result.Items) != 3 {
		t.Fatalf("被动采样查询失败: %+v", result)
	}
	for index, used := range []float64{0, 42, 105} {
		item := result.Items[index]
		if item.Used != used || item.Total != 100 || item.Unit != "%" || item.ValueMode != "count" || !item.Active {
			t.Fatalf("窗口 %d 解析错误: %+v", index, item)
		}
	}
	if result.Items[0].NextReset != now.Add(time.Hour).UTC().Format(time.RFC3339) {
		t.Fatalf("毫秒重置时间解析错误: %+v", result.Items[0])
	}
	for _, identity := range [][2]string{{"https://example.com", "other-key"}, {"https://other.example.com", "sample-key"}} {
		result := service.QueryQuota("token_plan_claude_passive", identity[0], identity[1], nil)
		if len(result.Items) != 1 || result.Items[0].InvalidMessage != claudePassiveQuotaPending {
			t.Fatal("不同地址或密钥之间不应共用样本")
		}
	}
}

func TestClaudePassiveQuotaIgnoresInvalidHeadersAndPreservesPartialWindows(t *testing.T) {
	service := newMemoryClaudePassiveQuotaService(t)
	now := time.Now().Truncate(time.Second)
	service.sampleClaudePassiveQuota("https://example.com", "key", claudePassiveTestHeaders(now), now)
	for _, invalid := range []string{"NaN", "+Inf", "-1", "bad", ""} {
		headers := make(http.Header)
		headers.Set("anthropic-ratelimit-unified-5h-utilization", invalid)
		headers.Set("anthropic-ratelimit-unified-5h-reset", "invalid")
		service.sampleClaudePassiveQuota("https://example.com", "key", headers, now)
	}
	headers := make(http.Header)
	headers.Set("anthropic-ratelimit-unified-7d-utilization", "0.6")
	service.sampleClaudePassiveQuota("https://example.com", "key", headers, now)
	items, _ := service.queryClaudePassiveQuota("https://example.com", "key", now)
	if len(items) != 3 || items[0].Used != 0 || items[1].Used != 60 || items[2].Used != 105 {
		t.Fatalf("部分窗口更新或无效响应头破坏了已有样本: %+v", items)
	}
}

func TestClaudePassiveQuotaDoesNotInventUsageAfterReset(t *testing.T) {
	service := newMemoryClaudePassiveQuotaService(t)
	now := time.Now().Truncate(time.Second)
	service.sampleClaudePassiveQuota("https://example.com", "key", claudePassiveTestHeaders(now), now)
	items, _ := service.queryClaudePassiveQuota("https://example.com", "key", now.Add(2*time.Hour))
	if items[0].Active || items[0].InvalidMessage != claudePassiveQuotaStale || !items[1].Active {
		t.Fatalf("过期窗口应等待新样本，其他窗口应继续有效: %+v", items)
	}
	headers := make(http.Header)
	headers.Set("anthropic-ratelimit-unified-5h-reset", fmt.Sprint(now.Add(6*time.Hour).Unix()))
	service.sampleClaudePassiveQuota("https://example.com", "key", headers, now.Add(2*time.Hour))
	items, _ = service.queryClaudePassiveQuota("https://example.com", "key", now.Add(2*time.Hour))
	for _, item := range items {
		if item.Key == "five_hour" {
			t.Fatalf("新窗口尚无使用率时不能继承旧样本: %+v", item)
		}
	}
	headers.Set("anthropic-ratelimit-unified-5h-utilization", "0.1")
	service.sampleClaudePassiveQuota("https://example.com", "key", headers, now.Add(2*time.Hour))
	items, _ = service.queryClaudePassiveQuota("https://example.com", "key", now.Add(2*time.Hour))
	if items[0].Used != 10 || !items[0].Active {
		t.Fatalf("新样本未恢复窗口: %+v", items)
	}
}

func TestClaudePassiveQuotaPersistsAndReloadsWithoutCredentials(t *testing.T) {
	useIsolatedHomeDir(t)
	service := NewProviderQuotaQueryService()
	now := time.Now().Truncate(time.Second)
	service.sampleClaudePassiveQuota("https://private.example.com/private-path", "private-sample-key", claudePassiveTestHeaders(now), now)
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	var key, raw string
	if err := db.QueryRow("SELECT cache_key, snapshot FROM provider_claude_quota_samples").Scan(&key, &raw); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private.example.com", "private-path", "private-sample-key"} {
		if strings.Contains(key+raw, secret) {
			t.Fatalf("采样表不应保存认证或私有地址: %s", secret)
		}
	}
	reloaded := NewProviderQuotaQueryService()
	result := reloaded.QueryQuota("token_plan_claude_passive", "https://private.example.com/private-path", "private-sample-key", nil)
	if !result.Success || len(result.Items) != 3 || result.Items[1].Used != 42 {
		t.Fatalf("重建服务后未恢复采样: %+v", result)
	}
}

func TestClaudePassiveQuotaConcurrentReadsAndSamples(t *testing.T) {
	service := newMemoryClaudePassiveQuotaService(t)
	now := time.Now().Truncate(time.Second)
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			service.sampleClaudePassiveQuota("https://example.com", "key", claudePassiveTestHeaders(now), now)
			service.queryQuotaContext(context.Background(), "token_plan_claude_passive", "https://example.com", "key", nil)
		}()
	}
	group.Wait()
}

func TestClaudePassiveQuotaRelayCapturesSuccessStreamAnd429(t *testing.T) {
	for _, mode := range []string{"json", "stream", "429"} {
		t.Run(mode, func(t *testing.T) {
			useIsolatedHomeDir(t)
			gin.SetMode(gin.TestMode)
			var calls atomic.Int64
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("anthropic-ratelimit-unified-5h-utilization", "0.25")
				w.Header().Set("anthropic-ratelimit-unified-5h-reset", fmt.Sprint(time.Now().Add(time.Hour).Unix()))
				if mode == "429" {
					w.WriteHeader(http.StatusTooManyRequests)
					fmt.Fprint(w, `{"error":{"type":"rate_limit_error","message":"limited"}}`)
				} else if mode == "stream" {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_sample\",\"usage\":{\"input_tokens\":1}}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
				} else {
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"id":"msg_sample","type":"message","content":[],"usage":{"input_tokens":1,"output_tokens":1}}`)
				}
			}))
			defer upstream.Close()
			providers := NewProviderService()
			defer providers.Stop()
			relay := NewProviderRelayService(providers, nil, nil, nil, nil, nil, "")
			query := newMemoryClaudePassiveQuotaService(t)
			relay.BindProviderQuotaAutomationService(&ProviderQuotaAutomationService{queryService: query})
			provider := Provider{ID: 1, Name: "Passive", APIURL: upstream.URL, APIKey: "sample-key", Enabled: true,
				ProviderQuotaQueryType: "token_plan_claude_passive"}
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ok, err := relay.forwardRequest(ctx, "claude", provider, "/v1/messages", nil,
				map[string]string{"Content-Type": "application/json"}, []byte(`{"model":"claude-test","max_tokens":1,"messages":[]}`), mode == "stream", "claude-test", "claude-test")
			if mode != "429" && (!ok || err != nil) {
				t.Fatalf("转发失败: %v", err)
			}
			if mode == "429" && (ok || err == nil) {
				t.Fatal("限流响应应保留转发错误")
			}
			result := query.QueryQuota("token_plan_claude_passive", upstream.URL, "sample-key", nil)
			if len(result.Items) != 1 || result.Items[0].Used != 25 || calls.Load() != 1 {
				t.Fatalf("采样或请求次数错误: result=%+v calls=%d", result, calls.Load())
			}
		})
	}
}

func TestClaudePassiveQuotaDoesNotAutoDisableProvider(t *testing.T) {
	var remaining atomic.Int64
	automation, providers, _, _ := newQuotaAutomationServiceForTest(t, true, &remaining)
	defer providers.Stop()
	query := newMemoryClaudePassiveQuotaService(t)
	automation.queryService = query
	provider := Provider{ID: 1, Name: "Passive", APIURL: "https://example.com", APIKey: "sample-key", Enabled: true,
		ProviderQuotaQueryType: "token_plan_claude_passive"}
	if err := providers.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatal(err)
	}
	headers := make(http.Header)
	headers.Set("anthropic-ratelimit-unified-5h-utilization", "1")
	query.sampleClaudePassiveQuota(provider.APIURL, provider.APIKey, headers, time.Now())
	result := automation.checkProviderQuota("claude", "1")
	if !result.Success || !result.ProviderEnabled || result.QuotaAutoDisabled || result.StateChanged {
		t.Fatalf("被动模式不应自动停用供应商: %+v", result)
	}
}
