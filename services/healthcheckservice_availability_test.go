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
	"sync"
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

func TestHealthCheckServiceValidationFailureDoesNotAffectBlacklist(t *testing.T) {
	result := &HealthCheckResult{
		Status:         HealthStatusValidationError,
		HTTPStatusCode: http.StatusOK,
	}

	if isHealthBlacklistFailure(result) {
		t.Fatal("响应内容校验失败不应参与健康检查拉黑计数")
	}
}

func TestHealthCheckBlacklistCountersAreIndependentAndManualChecksDoNotCount(t *testing.T) {
	useIsolatedHomeDir(t)
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
		t.Fatalf("设置真实请求阈值失败: %v", err)
	}
	if _, err := db.Exec(`UPDATE app_settings SET value = '2' WHERE key = 'availability_failure_threshold'`); err != nil {
		t.Fatalf("设置健康检查阈值失败: %v", err)
	}

	previousQueue := GlobalDBQueue
	writeQueue := NewDBWriteQueue(db, 32, false)
	GlobalDBQueue = writeQueue
	t.Cleanup(func() {
		_ = writeQueue.Shutdown(2 * time.Second)
		GlobalDBQueue = previousQueue
	})

	settingsService := NewSettingsService()
	blacklistService := NewBlacklistService(settingsService, nil)
	healthCheckService := NewHealthCheckService(nil, blacklistService, settingsService)
	provider := &Provider{
		ID:                        9,
		Name:                      "CounterProvider",
		ConnectivityAutoBlacklist: true,
	}
	failedResult := &HealthCheckResult{
		ProviderID:     provider.ID,
		ProviderName:   provider.Name,
		Platform:       "codex",
		Status:         HealthStatusFailed,
		HTTPStatusCode: http.StatusBadGateway,
		ErrorMessage:   "upstream health status 502",
	}

	healthCheckService.handleBlacklistIntegration(provider, failedResult, false)
	var rowCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM provider_blacklist WHERE platform = 'codex' AND provider_id = '9'`).Scan(&rowCount); err != nil {
		t.Fatalf("查询手动检测计数失败: %v", err)
	}
	if rowCount != 0 {
		t.Fatalf("手动健康检查不应创建失败计数，记录数=%d", rowCount)
	}

	healthCheckService.handleBlacklistIntegration(provider, failedResult, true)
	if err := blacklistService.RecordFailureWithReasonByID("codex", "9", provider.Name, "upstream status 502"); err != nil {
		t.Fatalf("记录真实请求失败失败: %v", err)
	}

	statuses, err := blacklistService.GetBlacklistStatus("codex")
	if err != nil {
		t.Fatalf("读取黑名单状态失败: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("黑名单状态数量=%d，期望 1", len(statuses))
	}
	status := statuses[0]
	if status.FailureCount != 1 || status.FailureThreshold != 9 {
		t.Fatalf("真实请求计数=%d/%d，期望 1/9", status.FailureCount, status.FailureThreshold)
	}
	if status.HealthFailureCount != 1 || status.HealthFailureThreshold != 2 {
		t.Fatalf("健康检查计数=%d/%d，期望 1/2", status.HealthFailureCount, status.HealthFailureThreshold)
	}
	if status.IsBlacklisted {
		t.Fatal("两套计数均未达到各自阈值时不应拉黑")
	}

	if err := blacklistService.RecordSuccessByID("codex", "9", provider.Name); err != nil {
		t.Fatalf("记录真实请求成功失败: %v", err)
	}
	statuses, err = blacklistService.GetBlacklistStatus("codex")
	if err != nil {
		t.Fatalf("重新读取黑名单状态失败: %v", err)
	}
	if len(statuses) != 1 || statuses[0].FailureCount != 0 || statuses[0].HealthFailureCount != 0 {
		t.Fatalf("真实请求成功应清零两套计数: %#v", statuses)
	}

	healthCheckService.handleBlacklistIntegration(provider, failedResult, true)
	healthCheckService.handleBlacklistIntegration(provider, failedResult, true)
	statuses, err = blacklistService.GetBlacklistStatus("codex")
	if err != nil {
		t.Fatalf("读取健康检查拉黑状态失败: %v", err)
	}
	if len(statuses) != 1 || !statuses[0].IsBlacklisted {
		t.Fatalf("连续两次后台健康检查 502 应触发拉黑: %#v", statuses)
	}
	if statuses[0].FailureCount != 0 || statuses[0].HealthFailureCount != 2 {
		t.Fatalf("健康检查触发时应保留 2/2 计数: %#v", statuses[0])
	}
	if statuses[0].BlacklistTriggerSource != BlacklistTriggerSourceHealth || !strings.Contains(statuses[0].BlacklistReason, "status 502") {
		t.Fatalf("健康检查触发来源或原因不正确: %#v", statuses[0])
	}
}

func TestBlacklistActiveSuccessPreservesTriggerDetails(t *testing.T) {
	useIsolatedHomeDir(t)
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

	until := time.Now().Add(time.Hour)
	if _, err := db.Exec(`
		INSERT INTO provider_blacklist (
			platform, provider_id, provider_name, failure_count, health_failure_count,
			blacklisted_at, blacklisted_until, blacklist_trigger_source, blacklist_reason
		) VALUES ('codex', 'active-success', 'Active Success', 9, 2, ?, ?, ?, ?)
	`, time.Now(), until, BlacklistTriggerSourceHealth, "upstream status 502"); err != nil {
		t.Fatalf("插入活动黑名单失败: %v", err)
	}

	previousQueue := GlobalDBQueue
	writeQueue := NewDBWriteQueue(db, 32, false)
	GlobalDBQueue = writeQueue
	t.Cleanup(func() {
		_ = writeQueue.Shutdown(2 * time.Second)
		GlobalDBQueue = previousQueue
	})

	service := NewBlacklistService(NewSettingsService(), nil)
	if err := service.RecordSuccessByID("codex", "active-success", "Active Success"); err != nil {
		t.Fatalf("记录成功失败: %v", err)
	}

	var requestCount, healthCount int
	var source, reason string
	if err := db.QueryRow(`
		SELECT failure_count, health_failure_count, blacklist_trigger_source, blacklist_reason
		FROM provider_blacklist WHERE provider_id = 'active-success'
	`).Scan(&requestCount, &healthCount, &source, &reason); err != nil {
		t.Fatalf("读取活动黑名单失败: %v", err)
	}
	if requestCount != 9 || healthCount != 2 || source != BlacklistTriggerSourceHealth || reason != "upstream status 502" {
		t.Fatalf("活动黑名单详情被成功请求清空: request=%d health=%d source=%q reason=%q", requestCount, healthCount, source, reason)
	}
}

func TestHealthCheckSuccessWithoutFailuresDoesNotWriteDatabase(t *testing.T) {
	useIsolatedHomeDir(t)
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

	previousQueue := GlobalDBQueue
	writeQueue := NewDBWriteQueue(db, 32, false)
	GlobalDBQueue = writeQueue
	t.Cleanup(func() {
		_ = writeQueue.Shutdown(2 * time.Second)
		GlobalDBQueue = previousQueue
	})

	service := NewBlacklistService(NewSettingsService(), nil)
	before := writeQueue.GetStats().TotalWrites
	if err := service.RecordHealthCheckSuccessByID("codex", "never-failed", "Never Failed"); err != nil {
		t.Fatalf("记录健康检查成功失败: %v", err)
	}
	if after := writeQueue.GetStats().TotalWrites; after != before {
		t.Fatalf("无健康失败计数时不应写数据库: before=%d after=%d", before, after)
	}
}

func TestBlacklistSuccessWaitsForRunningMutation(t *testing.T) {
	useIsolatedHomeDir(t)
	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	service := NewBlacklistService(NewSettingsService(), nil)
	key := blacklistRuntimeKey("codex", "waits-for-mutation")
	service.identityMu.Lock()
	service.boundIdentities[key] = "Waits For Mutation"
	service.identityMu.Unlock()

	service.operationMu.Lock()
	done := make(chan error, 1)
	go func() {
		done <- service.RecordSuccessByID("codex", "waits-for-mutation", "Waits For Mutation")
	}()

	select {
	case err := <-done:
		service.operationMu.Unlock()
		t.Fatalf("成功记录不应越过正在执行的失败写入: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	service.operationMu.Unlock()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("记录成功失败: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("失败写入结束后成功记录未继续执行")
	}
}

func TestHealthCheckFailureCounterIsSerialized(t *testing.T) {
	useIsolatedHomeDir(t)
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
	if _, err := db.Exec(`
		INSERT INTO provider_blacklist (platform, provider_id, provider_name, health_failure_count)
		VALUES ('codex', 'serialized', 'Serialized', 0)
	`); err != nil {
		t.Fatalf("插入计数记录失败: %v", err)
	}

	previousQueue := GlobalDBQueue
	writeQueue := NewDBWriteQueue(db, 128, false)
	GlobalDBQueue = writeQueue
	t.Cleanup(func() {
		_ = writeQueue.Shutdown(2 * time.Second)
		GlobalDBQueue = previousQueue
	})

	service := NewBlacklistService(NewSettingsService(), nil)
	const attempts = 9
	var wg sync.WaitGroup
	errs := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- service.RecordHealthCheckFailureByID("codex", "serialized", "Serialized", "upstream status 502", attempts)
		}()
	}
	wg.Wait()
	close(errs)
	for callErr := range errs {
		if callErr != nil {
			t.Fatalf("记录健康检查失败失败: %v", callErr)
		}
	}

	var count int
	if err := db.QueryRow(`SELECT health_failure_count FROM provider_blacklist WHERE provider_id = 'serialized'`).Scan(&count); err != nil {
		t.Fatalf("读取健康失败计数失败: %v", err)
	}
	if count != attempts {
		t.Fatalf("并发健康失败计数=%d，期望=%d", count, attempts)
	}
}

func TestSettingsServiceSavesBlacklistThresholdsTogether(t *testing.T) {
	useIsolatedHomeDir(t)
	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}
	previousQueue := GlobalDBQueue
	writeQueue := NewDBWriteQueue(db, 32, false)
	GlobalDBQueue = writeQueue
	t.Cleanup(func() {
		_ = writeQueue.Shutdown(2 * time.Second)
		GlobalDBQueue = previousQueue
	})

	settings := NewSettingsService()
	if err := settings.UpdateBlacklistSettingsWithHealthThreshold(9, 1800, 7); err != nil {
		t.Fatalf("保存双阈值失败: %v", err)
	}

	var requestThreshold, healthThreshold string
	if err := db.QueryRow(`SELECT value FROM app_settings WHERE key = 'blacklist_failure_threshold'`).Scan(&requestThreshold); err != nil {
		t.Fatalf("读取请求阈值失败: %v", err)
	}
	if err := db.QueryRow(`SELECT value FROM app_settings WHERE key = 'availability_failure_threshold'`).Scan(&healthThreshold); err != nil {
		t.Fatalf("读取健康阈值失败: %v", err)
	}
	if requestThreshold != "9" || healthThreshold != "7" {
		t.Fatalf("双阈值保存结果错误: request=%q health=%q", requestThreshold, healthThreshold)
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

func TestApplyLogAvailabilityRequestFallsBackForUnknownOutcome(t *testing.T) {
	start := time.Date(2026, time.July, 23, 10, 0, 0, 0, time.UTC)
	rangeSpec := logAvailabilityRangeSpec{
		Start:          start,
		End:            start.Add(time.Minute),
		BucketDuration: time.Minute,
	}
	buckets := make([]logAvailabilityBucket, 1)

	applyLogAvailabilityRequest(buckets, rangeSpec, logAvailabilityRequestLog{
		HTTPCode:  503,
		Outcome:   "unknown",
		CreatedAt: start.Add(time.Second),
	}, 5000)
	applyLogAvailabilityRequest(buckets, rangeSpec, logAvailabilityRequestLog{
		HTTPCode:  503,
		Outcome:   "  " + requestOutcomeSuccess + "  ",
		CreatedAt: start.Add(2 * time.Second),
	}, 5000)
	applyLogAvailabilityRequest(buckets, rangeSpec, logAvailabilityRequestLog{
		HTTPCode:  503,
		Outcome:   "  " + requestOutcomeExcluded + "  ",
		CreatedAt: start.Add(3 * time.Second),
	}, 5000)

	if buckets[0].TotalRequests != 2 || buckets[0].FailedCount != 1 {
		t.Fatalf("可用性三态回退错误: %#v", buckets[0])
	}
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
