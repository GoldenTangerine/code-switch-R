/**
 * @name: Provider 代理链路性能基线
 * @Descripttion: 测量 Provider 深拷贝在本地成功代理完整链路中的耗时与分配占比
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-01 01:47:08
 * @LastEditTime: 2026-09-01 01:47:08
 * @FilePath: services/provider_proxy_chain_benchmark_test.go
 */

package services

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/daodao97/xgo/xdb"
	"github.com/daodao97/xgo/xlog"
	"github.com/gin-gonic/gin"
)

const providerProxyChainBenchmarkResponse = `{"id":"resp_benchmark","status":"completed","model":"gpt-5.3-codex","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`

type providerProxyChainBenchmarkHarness struct {
	router        http.Handler
	requestBody   []byte
	upstream      *httptest.Server
	relay         *ProviderRelayService
	upstreamCalls atomic.Int64
}

func configureProviderProxyChainBenchmark(tb testing.TB) func() {
	tb.Helper()
	db, err := xdb.DB("default")
	if err != nil {
		tb.Fatal(err)
	}
	var previousBlacklistValue string
	if err := db.QueryRow(`SELECT value FROM app_settings WHERE key = 'enable_blacklist'`).Scan(&previousBlacklistValue); err != nil {
		tb.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE app_settings SET value = 'false' WHERE key = 'enable_blacklist'`); err != nil {
		tb.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM request_log`); err != nil {
		tb.Fatal(err)
	}
	return func() {
		_, _ = db.Exec(`DELETE FROM request_log`)
		_, _ = db.Exec(`UPDATE app_settings SET value = ? WHERE key = 'enable_blacklist'`, previousBlacklistValue)
	}
}

func newProviderProxyChainBenchmarkHarness(tb testing.TB, providerCount int) *providerProxyChainBenchmarkHarness {
	tb.Helper()
	gin.SetMode(gin.TestMode)
	harness := &providerProxyChainBenchmarkHarness{
		requestBody: []byte(`{"model":"gpt-5.3-codex","input":"benchmark"}`),
	}
	harness.upstream = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		harness.upstreamCalls.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(providerProxyChainBenchmarkResponse))
	}))

	providers := buildProviderSnapshotBenchmarkProviders(providerCount)
	for index := range providers {
		providers[index].APIURL = harness.upstream.URL
		providers[index].APIKey = fmt.Sprintf("benchmark-key-%03d", index+1)
		providers[index].Enabled = true
		providers[index].Level = 1
		providers[index].SupportedModels["gpt-5.3-codex"] = true
		providers[index].configErrors = nil
	}
	providerService := NewProviderService()
	providerService.storeProviderSnapshot(
		"codex",
		providers,
		sha256.Sum256([]byte(fmt.Sprintf("provider-proxy-chain-%d", providerCount))),
		true,
	)
	blacklistService := NewBlacklistService(NewSettingsService(), nil)
	harness.relay = NewProviderRelayService(providerService, nil, blacklistService, nil, nil, nil, "")
	router := gin.New()
	harness.relay.registerRoutes(router)
	harness.router = router
	tb.Cleanup(func() {
		_ = harness.relay.Stop()
		_ = providerService.Stop()
		harness.upstream.Close()
	})
	return harness
}

func (harness *providerProxyChainBenchmarkHarness) execute() *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(harness.requestBody))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	harness.router.ServeHTTP(response, request)
	return response
}

func assertProviderProxyChainBenchmarkResult(tb testing.TB, providerCount int, response *httptest.ResponseRecorder) {
	tb.Helper()
	if providerCount == 0 {
		if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "没有可用的 provider") {
			tb.Fatalf("无 Provider 对照结果异常: status=%d body=%s", response.Code, response.Body.String())
		}
		return
	}
	if response.Code != http.StatusOK || response.Body.String() != providerProxyChainBenchmarkResponse {
		tb.Fatalf("成功代理结果异常: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestProviderProxyChainBenchmarkFixture(t *testing.T) {
	restore := configureProviderProxyChainBenchmark(t)
	defer restore()
	for _, providerCount := range []int{0, 1, 10, 100} {
		t.Run(fmt.Sprintf("%d_providers", providerCount), func(t *testing.T) {
			harness := newProviderProxyChainBenchmarkHarness(t, providerCount)
			response := harness.execute()
			assertProviderProxyChainBenchmarkResult(t, providerCount, response)
			wantCalls := int64(0)
			if providerCount > 0 {
				wantCalls = 1
			}
			if calls := harness.upstreamCalls.Load(); calls != wantCalls {
				t.Fatalf("上游调用次数=%d，期望=%d", calls, wantCalls)
			}
		})
	}
}

func BenchmarkProviderProxyChain(b *testing.B) {
	restore := configureProviderProxyChainBenchmark(b)
	defer restore()
	originalLogger := xlog.GetLogger()
	xlog.SetLogger(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})))
	b.Cleanup(func() {
		xlog.SetLogger(originalLogger)
	})
	for _, providerCount := range []int{0, 1, 10, 100} {
		b.Run(fmt.Sprintf("%d_providers", providerCount), func(b *testing.B) {
			harness := newProviderProxyChainBenchmarkHarness(b, providerCount)
			previousStdout := os.Stdout
			devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
			if err != nil {
				b.Fatal(err)
			}
			os.Stdout = devNull
			defer func() {
				os.Stdout = previousStdout
				_ = devNull.Close()
			}()
			response := harness.execute()
			assertProviderProxyChainBenchmarkResult(b, providerCount, response)
			callsBefore := harness.upstreamCalls.Load()
			semanticSize := providerBenchmarkJSONSize(b, buildProviderSnapshotBenchmarkProviders(providerCount))

			b.ReportAllocs()
			b.ReportMetric(float64(providerCount), "providers/op")
			b.ReportMetric(float64(semanticSize), "provider-semantic-bytes/op")
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				response = harness.execute()
			}
			b.StopTimer()
			assertProviderProxyChainBenchmarkResult(b, providerCount, response)
			wantCalls := callsBefore
			if providerCount > 0 {
				wantCalls += int64(b.N)
			}
			if calls := harness.upstreamCalls.Load(); calls != wantCalls {
				b.Fatalf("上游调用次数=%d，期望=%d", calls, wantCalls)
			}
		})
	}
}
