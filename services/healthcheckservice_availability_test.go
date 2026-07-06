package services

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
)

func TestHealthCheckServiceCheckProviderUsesModelProbeForCodex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("请求路径 = %q, 期望 /responses", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("Authorization = %q, 期望 Bearer sk-test", got)
		}
		if got := r.Header.Get("openai-beta"); got != "responses=experimental" {
			t.Fatalf("openai-beta = %q, 期望 responses=experimental", got)
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("读取请求体失败: %v", err)
		}
		body := string(bodyBytes)
		if !strings.Contains(body, `"instructions":"You are an echo bot. Reply with exactly pong."`) {
			t.Fatalf("请求体缺少 pong 指令: %s", body)
		}
		if !strings.Contains(body, `"type":"input_text"`) || !strings.Contains(body, `"text":"ping"`) {
			t.Fatalf("请求体不是 Responses API 探测格式: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-5.3-codex","output":[{"type":"message","content":[{"type":"output_text","text":"pong"}]}]}`))
	}))
	defer server.Close()

	hcs := NewHealthCheckService(nil, nil, nil)
	result := hcs.checkProvider(context.Background(), Provider{
		ID:     1,
		Name:   "CodexTest",
		APIURL: server.URL,
		APIKey: "sk-test",
		AvailabilityConfig: &AvailabilityConfig{
			TestModel:    "gpt-5.3-codex",
			TestEndpoint: "/responses",
			Timeout:      5000,
		},
	}, "codex")

	if result.Status != HealthStatusOperational {
		t.Fatalf("Status = %q, 期望 %q, error=%q", result.Status, HealthStatusOperational, result.ErrorMessage)
	}
	if result.Model != "gpt-5.3-codex" {
		t.Fatalf("Model = %q, 期望 gpt-5.3-codex", result.Model)
	}
	if result.Endpoint != "/responses" {
		t.Fatalf("Endpoint = %q, 期望 /responses", result.Endpoint)
	}
}

func TestHealthCheckServiceCheckProviderMarksValidationFailureWhenResponseIsNotPong(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	}))
	defer server.Close()

	hcs := NewHealthCheckService(nil, nil, nil)
	result := hcs.checkProvider(context.Background(), Provider{
		ID:     2,
		Name:   "ChatProxy",
		APIURL: server.URL,
		APIKey: "sk-test",
		AvailabilityConfig: &AvailabilityConfig{
			TestModel:    "gpt-4.1-mini",
			TestEndpoint: "/v1/chat/completions",
			Timeout:      5000,
		},
	}, "codex")

	if result.Status != HealthStatusValidationError {
		t.Fatalf("Status = %q, 期望 %q, error=%q", result.Status, HealthStatusValidationError, result.ErrorMessage)
	}
	if !strings.Contains(result.ErrorMessage, availabilityProbeExpectedText) {
		t.Fatalf("ErrorMessage = %q, 应包含 %q", result.ErrorMessage, availabilityProbeExpectedText)
	}
}

func TestHealthCheckServiceHandleBlacklistIntegrationTreatsValidationFailureAsFailure(t *testing.T) {
	hcs := NewHealthCheckService(nil, nil, nil)
	provider := &Provider{
		ID:                        3,
		Name:                      "ValidationFailProvider",
		ConnectivityAutoBlacklist: true,
	}
	result := &HealthCheckResult{
		ProviderID:   provider.ID,
		ProviderName: provider.Name,
		Platform:     "codex",
		Status:       HealthStatusValidationError,
	}

	hcs.handleBlacklistIntegration(provider, result)

	counterKey := "codex:" + providerRefFromNumericID(provider.ID, provider.Name)
	counter := hcs.failCounters[counterKey]
	if counter == nil {
		t.Fatalf("应创建失败计数器")
	}
	if counter.ConsecutiveFails != 1 {
		t.Fatalf("ConsecutiveFails = %d, 期望 1", counter.ConsecutiveFails)
	}
}

func TestHealthCheckServiceGetLogBasedResultsAggregatesRequestLogs(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	providerService := NewProviderService()
	if err := providerService.SaveProviders("codex", []Provider{
		{
			ID:      42,
			Name:    "LogProbe",
			Enabled: true,
		},
		{
			ID:      43,
			Name:    "WeightedProbe",
			Enabled: true,
		},
	}); err != nil {
		t.Fatalf("保存供应商失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	rangeSpec := resolveLogAvailabilityRange(LogAvailabilityRange24H, time.Now())
	failedBucketAt := rangeSpec.Start.Add(rangeSpec.BucketDuration*10 + time.Second)
	degradedBucketAt := rangeSpec.Start.Add(rangeSpec.BucketDuration*20 + time.Second)
	latestBucketAt := rangeSpec.Start.Add(rangeSpec.BucketDuration*30 + time.Second)
	insertAvailabilityRequestLog(t, db, "codex", "42", "LogProbe", "gpt-ok", 200, 0.12, failedBucketAt.Format(timeLayout))
	insertAvailabilityRequestLog(t, db, "codex", "42", "LogProbe", "gpt-failed", 503, 0.36, failedBucketAt.Add(time.Second).Format(timeLayout))
	insertAvailabilityRequestLog(t, db, "codex", "42", "LogProbe", "gpt-slow", 200, 6.50, degradedBucketAt.Format(timeLayout))
	insertAvailabilityRequestLog(t, db, "codex", "42", "LogProbe", "gpt-latest", 200, 0.18, latestBucketAt.Format(timeLayout))
	insertAvailabilityRequestLog(t, db, "codex", "", "42", "wrong-name-fallback", 200, 0.1, latestBucketAt.Add(time.Second).Format(timeLayout))
	insertAvailabilityRequestLog(t, db, "codex", "43", "WeightedProbe", "weighted-many-1", 200, 0.10, failedBucketAt.Format(timeLayout))
	insertAvailabilityRequestLog(t, db, "codex", "43", "WeightedProbe", "weighted-many-2", 200, 0.10, failedBucketAt.Add(time.Second).Format(timeLayout))
	insertAvailabilityRequestLog(t, db, "codex", "43", "WeightedProbe", "weighted-many-3", 200, 0.10, failedBucketAt.Add(2*time.Second).Format(timeLayout))
	insertAvailabilityRequestLog(t, db, "codex", "43", "WeightedProbe", "weighted-single", 200, 1.00, degradedBucketAt.Format(timeLayout))

	settingsService := NewSettingsService()
	if _, err := db.Exec(`
		INSERT INTO app_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, "availability_operational_threshold_ms", "7000"); err != nil {
		t.Fatalf("设置可用性阈值失败: %v", err)
	}

	hcs := NewHealthCheckService(providerService, nil, settingsService)
	results, err := hcs.GetLogBasedResults(LogAvailabilityRange24H)
	if err != nil {
		t.Fatalf("GetLogBasedResults 调用失败: %v", err)
	}

	codexTimelines := results["codex"]
	if len(codexTimelines) != 2 {
		t.Fatalf("期望 2 个 codex timeline，实际 %d", len(codexTimelines))
	}

	timeline := findAvailabilityTimelineByProvider(t, codexTimelines, 42)
	if !timeline.AvailabilityMonitorEnabled {
		t.Fatalf("日志模式应始终允许展示 timeline")
	}
	if len(timeline.Items) != LogAvailabilityHistoryLimit {
		t.Fatalf("期望固定 %d 个 bucket，实际 %d", LogAvailabilityHistoryLimit, len(timeline.Items))
	}
	if timeline.Latest == nil || timeline.Latest.Status != HealthStatusOperational {
		t.Fatalf("期望最新 bucket 正常，实际 %+v", timeline.Latest)
	}
	if timeline.Latest.Model != "wrong-name-fallback" {
		t.Fatalf("期望 provider 字段回退映射后的最新模型 wrong-name-fallback，实际 %q", timeline.Latest.Model)
	}

	failedBucket := findAvailabilityBucketByStatus(t, timeline.Items, HealthStatusFailed)
	if failedBucket.LatencyMs != 240 {
		t.Fatalf("期望失败 bucket 平均延迟 240ms，实际 %d", failedBucket.LatencyMs)
	}
	if !strings.Contains(failedBucket.ErrorMessage, "1/2 请求失败") {
		t.Fatalf("期望失败 bucket 错误信息包含失败数量，实际 %q", failedBucket.ErrorMessage)
	}

	if degradedBucket := findOptionalAvailabilityBucketByStatus(timeline.Items, HealthStatusDegraded); degradedBucket != nil {
		t.Fatalf("自定义阈值 7000ms 下 6500ms 不应告警，实际 %+v", degradedBucket)
	}

	if math.Abs(timeline.Uptime-66.66666666666666) > 0.0001 {
		t.Fatalf("期望空桶不参与可用率且自定义阈值下 2/3 bucket 可用，实际 %.8f", timeline.Uptime)
	}
	if timeline.AvgLatencyMs != 2260 {
		t.Fatalf("期望请求级加权平均延迟 2260ms，实际 %d", timeline.AvgLatencyMs)
	}

	weightedTimeline := findAvailabilityTimelineByProvider(t, codexTimelines, 43)
	if weightedTimeline.AvgLatencyMs != 325 {
		t.Fatalf("期望 WeightedProbe 请求级加权平均延迟 325ms，实际 %d", weightedTimeline.AvgLatencyMs)
	}

}

func TestHealthCheckServiceGetLogBasedResultsColorsByErrorRatio(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	providerService := NewProviderService()
	if err := providerService.SaveProviders("codex", []Provider{
		{
			ID:      44,
			Name:    "RatioProbe",
			Enabled: true,
		},
	}); err != nil {
		t.Fatalf("保存供应商失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	rangeSpec := resolveLogAvailabilityRange(LogAvailabilityRange24H, time.Now())
	greenBucketAt := rangeSpec.Start.Add(rangeSpec.BucketDuration*8 + time.Second)
	yellowBucketAt := rangeSpec.Start.Add(rangeSpec.BucketDuration*16 + time.Second)
	redBucketAt := rangeSpec.Start.Add(rangeSpec.BucketDuration*24 + time.Second)
	slowBucketAt := rangeSpec.Start.Add(rangeSpec.BucketDuration*32 + time.Second)

	insertAvailabilityRequestLogsForRatio(t, db, "codex", "44", "RatioProbe", greenBucketAt, 20, 1, 0)
	insertAvailabilityRequestLogsForRatio(t, db, "codex", "44", "RatioProbe", yellowBucketAt, 20, 2, 0)
	insertAvailabilityRequestLogsForRatio(t, db, "codex", "44", "RatioProbe", redBucketAt, 20, 5, 0)
	insertAvailabilityRequestLogsForRatio(t, db, "codex", "44", "RatioProbe", slowBucketAt, 20, 0, 20)

	hcs := NewHealthCheckService(providerService, nil, NewSettingsService())
	results, err := hcs.GetLogBasedResults(LogAvailabilityRange24H)
	if err != nil {
		t.Fatalf("GetLogBasedResults 调用失败: %v", err)
	}

	timeline := findAvailabilityTimelineByProvider(t, results["codex"], 44)
	greenBucket := findAvailabilityBucketByRequestStats(t, timeline.Items, 20, 1, 0)
	if greenBucket.Status != HealthStatusOperational {
		t.Fatalf("5%% 错误率应为绿色，实际 %+v", greenBucket)
	}
	if math.Abs(greenBucket.ErrorRate-5) > 0.0001 {
		t.Fatalf("绿色 bucket ErrorRate = %.4f，期望 5", greenBucket.ErrorRate)
	}

	yellowBucket := findAvailabilityBucketByRequestStats(t, timeline.Items, 20, 2, 0)
	if yellowBucket.Status != HealthStatusDegraded {
		t.Fatalf("10%% 错误率应为黄色，实际 %+v", yellowBucket)
	}

	redBucket := findAvailabilityBucketByRequestStats(t, timeline.Items, 20, 5, 0)
	if redBucket.Status != HealthStatusFailed {
		t.Fatalf("25%% 错误率应为红色，实际 %+v", redBucket)
	}

	slowBucket := findAvailabilityBucketByRequestStats(t, timeline.Items, 20, 0, 20)
	if slowBucket.Status != HealthStatusOperational {
		t.Fatalf("无错误的高延迟 bucket 应保持绿色，实际 %+v", slowBucket)
	}
	if !strings.Contains(slowBucket.ErrorMessage, "20/20 请求延迟超过 6000ms") {
		t.Fatalf("高延迟应保留为单独提示，实际 %q", slowBucket.ErrorMessage)
	}

	if math.Abs(timeline.Uptime-50) > 0.0001 {
		t.Fatalf("期望仅绿色 bucket 计入可用率 50%%，实际 %.8f", timeline.Uptime)
	}
	if timeline.AvgLatencyMs != 2405 {
		t.Fatalf("期望黄色 bucket 继续参与平均延迟且红色 bucket 不参与，实际 %d", timeline.AvgLatencyMs)
	}
}

func findAvailabilityTimelineByProvider(t *testing.T, timelines []ProviderTimeline, providerID int64) ProviderTimeline {
	t.Helper()
	for _, timeline := range timelines {
		if timeline.ProviderID == providerID {
			return timeline
		}
	}
	t.Fatalf("未找到 providerID=%d 的 timeline", providerID)
	return ProviderTimeline{}
}

func findAvailabilityBucketByStatus(t *testing.T, items []HealthCheckResult, status string) HealthCheckResult {
	t.Helper()
	if item := findOptionalAvailabilityBucketByStatus(items, status); item != nil {
		return *item
	}
	t.Fatalf("未找到状态为 %s 的 bucket", status)
	return HealthCheckResult{}
}

func findOptionalAvailabilityBucketByStatus(items []HealthCheckResult, status string) *HealthCheckResult {
	for _, item := range items {
		if item.Status == status {
			bucket := item
			return &bucket
		}
	}
	return nil
}

func findAvailabilityBucketByRequestStats(t *testing.T, items []HealthCheckResult, totalRequests, failedRequests, slowRequests int) HealthCheckResult {
	t.Helper()
	for _, item := range items {
		if item.TotalRequests == totalRequests && item.FailedRequests == failedRequests && item.SlowRequests == slowRequests {
			return item
		}
	}
	t.Fatalf("未找到请求统计为 total=%d failed=%d slow=%d 的 bucket", totalRequests, failedRequests, slowRequests)
	return HealthCheckResult{}
}

func insertAvailabilityRequestLogsForRatio(
	t *testing.T,
	db *sql.DB,
	platform string,
	providerID string,
	provider string,
	bucketAt time.Time,
	totalRequests int,
	failedRequests int,
	slowRequests int,
) {
	t.Helper()
	for index := 0; index < totalRequests; index++ {
		httpCode := 200
		durationSec := 0.1
		if index < failedRequests {
			httpCode = 500
			durationSec = 0.2
		} else if index < failedRequests+slowRequests {
			durationSec = 7.0
		}
		insertAvailabilityRequestLog(
			t,
			db,
			platform,
			providerID,
			provider,
			fmt.Sprintf("ratio-%d-%d-%d-%d", totalRequests, failedRequests, slowRequests, index),
			httpCode,
			durationSec,
			bucketAt.Add(time.Duration(index)*time.Second).Format(timeLayout),
		)
	}
}

func insertAvailabilityRequestLog(
	t *testing.T,
	db *sql.DB,
	platform string,
	providerID string,
	provider string,
	model string,
	httpCode int,
	durationSec float64,
	createdAt string,
) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO request_log (
			platform,
			model,
			provider_id,
			provider,
			http_code,
			duration_sec,
			input_tokens,
			output_tokens,
			cache_create_tokens,
			cache_read_tokens,
			reasoning_tokens,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, platform, model, providerID, provider, httpCode, durationSec, 1, 1, 0, 0, 0, createdAt)
	if err != nil {
		t.Fatalf("插入 request_log 失败: %v", err)
	}
}
