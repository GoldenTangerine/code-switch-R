package services

import (
	"database/sql"
	"math"
	"testing"
	"time"

	modelpricing "codeswitch/resources/model-pricing"
	"github.com/daodao97/xgo/xdb"
)

func TestProviderStatsRangeV2_PerformanceAggregatesByStableProviderKey(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-1",
		Provider:      "Acme",
		IsStream:      1,
		DurationSec:   2.0,
		FirstTokenSec: 0.2,
		OutputTokens:  100,
		CreatedAt:     "2026-02-25 10:00:00",
	})
	// 同 provider_id 使用不同 provider 名称，不应覆盖性能均值。
	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-1",
		Provider:      "Acme-Renamed",
		IsStream:      1,
		DurationSec:   1.4,
		FirstTokenSec: 0.4,
		OutputTokens:  60,
		CreatedAt:     "2026-02-25 11:00:00",
	})
	// first_token_sec 无效，不参与首字均值，但总耗时和输出 Tokens 有效时仍参与速度。
	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-1",
		Provider:      "Acme-Renamed",
		IsStream:      1,
		DurationSec:   1.2,
		FirstTokenSec: 0,
		OutputTokens:  30,
		CreatedAt:     "2026-02-25 11:30:00",
	})
	// 非流式，不应参与性能均值。
	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-1",
		Provider:      "Acme",
		IsStream:      0,
		DurationSec:   1.0,
		FirstTokenSec: 0.1,
		OutputTokens:  80,
		CreatedAt:     "2026-02-25 12:00:00",
	})

	ls := NewLogService(nil)
	stats, err := ls.ProviderStatsRangeV2("codex", "", "", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("ProviderStatsRangeV2 调用失败: %v", err)
	}

	stat := findProviderStatByID(stats, "pid-1")
	if stat == nil {
		t.Fatalf("未找到 provider_id=pid-1 的统计")
	}
	if stat.TotalRequests != 4 {
		t.Fatalf("期望 total_requests=4，实际 %d", stat.TotalRequests)
	}
	if stat.DurationSampleCount != 4 || !almostEqualFloatProviderStat(stat.AvgDurationSec, 1.4) {
		t.Fatalf("平均耗时统计错误: samples=%d avg=%f", stat.DurationSampleCount, stat.AvgDurationSec)
	}

	expectedTTFT := 0.3 // (0.2 + 0.4) / 2
	if !almostEqualFloatProviderStat(stat.AvgFirstTokenSec, expectedTTFT) {
		t.Fatalf("期望 avg_first_token_sec=%f，实际 %f", expectedTTFT, stat.AvgFirstTokenSec)
	}
	if stat.TTFTSampleCount != 2 {
		t.Fatalf("期望 ttft_sample_count=2，实际 %d", stat.TTFTSampleCount)
	}

	expectedTPS := (100.0/2.0 + 60.0/1.4 + 30.0/1.2) / 3
	if !almostEqualFloatProviderStat(stat.AvgTokensPerSec, expectedTPS) {
		t.Fatalf("期望 avg_tokens_per_sec=%f，实际 %f", expectedTPS, stat.AvgTokensPerSec)
	}
	if stat.TPSSampleCount != 3 {
		t.Fatalf("期望 tps_sample_count=3，实际 %d", stat.TPSSampleCount)
	}
}

func TestProviderStatsRangeV2_PerformanceUsesTotalDurationWithoutMinimumWindow(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-3",
		Provider:      "OutlierGuard",
		IsStream:      1,
		DurationSec:   2.2,
		FirstTokenSec: 0.2,
		OutputTokens:  200,
		CreatedAt:     "2026-02-25 14:00:00",
	})
	// 即使首字接近总耗时，速度仍应使用完整总耗时。
	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-3",
		Provider:      "OutlierGuard",
		IsStream:      1,
		DurationSec:   0.401,
		FirstTokenSec: 0.4,
		OutputTokens:  5000,
		CreatedAt:     "2026-02-25 14:30:00",
	})

	ls := NewLogService(nil)
	stats, err := ls.ProviderStatsRangeV2("codex", "", "", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("ProviderStatsRangeV2 调用失败: %v", err)
	}

	stat := findProviderStatByID(stats, "pid-3")
	if stat == nil {
		t.Fatalf("未找到 provider_id=pid-3 的统计")
	}
	expectedTPS := (200.0/2.2 + 5000.0/0.401) / 2
	if !almostEqualFloatProviderStat(stat.AvgTokensPerSec, expectedTPS) {
		t.Fatalf("期望 avg_tokens_per_sec=%f，实际 %f", expectedTPS, stat.AvgTokensPerSec)
	}
	if stat.TTFTSampleCount != 2 {
		t.Fatalf("期望 ttft_sample_count=2，实际 %d", stat.TTFTSampleCount)
	}
	if stat.TPSSampleCount != 2 {
		t.Fatalf("期望 tps_sample_count=2，实际 %d", stat.TPSSampleCount)
	}
}

func TestProviderStatsRangeV2_PerformanceSeparatesTTFTAndTPSSamples(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-2",
		Provider:      "NoStreamA",
		IsStream:      1,
		DurationSec:   2.0,
		FirstTokenSec: 0,
		OutputTokens:  50,
		CreatedAt:     "2026-02-25 13:00:00",
	})
	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-2",
		Provider:      "NoStreamB",
		IsStream:      0,
		DurationSec:   1.0,
		FirstTokenSec: 0.1,
		OutputTokens:  40,
		CreatedAt:     "2026-02-25 14:00:00",
	})

	ls := NewLogService(nil)
	stats, err := ls.ProviderStatsRangeV2("codex", "", "", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("ProviderStatsRangeV2 调用失败: %v", err)
	}

	stat := findProviderStatByID(stats, "pid-2")
	if stat == nil {
		t.Fatalf("未找到 provider_id=pid-2 的统计")
	}
	if stat.AvgFirstTokenSec != 0 {
		t.Fatalf("无有效样本时 avg_first_token_sec 应为 0，实际 %f", stat.AvgFirstTokenSec)
	}
	if !almostEqualFloatProviderStat(stat.AvgTokensPerSec, 25) {
		t.Fatalf("速度不依赖首字，期望 avg_tokens_per_sec=25，实际 %f", stat.AvgTokensPerSec)
	}
	if stat.TTFTSampleCount != 0 {
		t.Fatalf("无有效样本时 ttft_sample_count 应为 0，实际 %d", stat.TTFTSampleCount)
	}
	if stat.TPSSampleCount != 1 {
		t.Fatalf("期望 tps_sample_count=1，实际 %d", stat.TPSSampleCount)
	}
}

func TestProviderStatsRangeV2_FallbackToProviderNameWhenProviderIDNotFound(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "",
		Provider:      "legacy-provider",
		IsStream:      1,
		DurationSec:   1.0,
		FirstTokenSec: 0.2,
		OutputTokens:  40,
		CreatedAt:     "2026-02-25 15:00:00",
	})

	ls := NewLogService(nil)
	stats, err := ls.ProviderStatsRangeV2("codex", "legacy-provider", "", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("ProviderStatsRangeV2 调用失败: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("期望返回 1 条 provider 统计，实际 %d", len(stats))
	}
	if stats[0].Provider != "legacy-provider" {
		t.Fatalf("期望 provider=legacy-provider，实际 %s", stats[0].Provider)
	}
	if !almostEqualFloatProviderStat(stats[0].AvgFirstTokenSec, 0.2) {
		t.Fatalf("期望 avg_first_token_sec=0.2，实际 %f", stats[0].AvgFirstTokenSec)
	}
	if !almostEqualFloatProviderStat(stats[0].AvgTokensPerSec, 40.0) {
		t.Fatalf("期望 avg_tokens_per_sec=40，实际 %f", stats[0].AvgTokensPerSec)
	}
	if stats[0].TTFTSampleCount != 1 {
		t.Fatalf("期望 ttft_sample_count=1，实际 %d", stats[0].TTFTSampleCount)
	}
	if stats[0].TPSSampleCount != 1 {
		t.Fatalf("期望 tps_sample_count=1，实际 %d", stats[0].TPSSampleCount)
	}
}

func TestProviderDailyStats_UsesLatestFiveSuccessfulStreamingSamplesAcrossDays(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	now := time.Now().UTC()
	for index := 0; index < 6; index++ {
		insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
			Platform:      "codex",
			ProviderID:    "pid-latest",
			Provider:      "Latest Provider",
			IsStream:      1,
			DurationSec:   float64(index + 1),
			FirstTokenSec: float64(index+1) / 10,
			OutputTokens:  100,
			CreatedAt:     now.AddDate(0, 0, -(index + 1)).Format(timeLayout),
		})
	}
	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-latest",
		Provider:      "Latest Provider",
		HttpCode:      500,
		IsStream:      1,
		DurationSec:   0.1,
		FirstTokenSec: 0.01,
		OutputTokens:  1000,
		CreatedAt:     now.Add(-2 * time.Hour).Format(timeLayout),
	})
	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:     "codex",
		ProviderID:   "pid-latest",
		Provider:     "Latest Provider",
		IsStream:     0,
		DurationSec:  1,
		OutputTokens: 25,
		CreatedAt:    now.Add(-time.Hour).Format(timeLayout),
	})
	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-failed-only",
		Provider:      "Failed Only",
		HttpCode:      500,
		IsStream:      1,
		DurationSec:   1,
		FirstTokenSec: 0.2,
		OutputTokens:  50,
		CreatedAt:     now.Add(-30 * time.Minute).Format(timeLayout),
	})

	ls := NewLogService(nil)
	stats, err := ls.ProviderDailyStats("codex")
	if err != nil {
		t.Fatalf("ProviderDailyStats 调用失败: %v", err)
	}
	stat := findProviderStatByID(stats, "pid-latest")
	if stat == nil {
		t.Fatalf("未找到 provider_id=pid-latest 的统计")
	}
	if stat.TotalRequests != 2 {
		t.Fatalf("当日请求统计不应受跨天性能样本影响，期望 2，实际 %d", stat.TotalRequests)
	}
	if !almostEqualFloatProviderStat(stat.AvgFirstTokenSec, 0.3) {
		t.Fatalf("期望最近 5 条首字平均为 0.3，实际 %f", stat.AvgFirstTokenSec)
	}
	expectedTPS := (100.0/1 + 100.0/2 + 100.0/3 + 100.0/4 + 100.0/5) / 5
	if !almostEqualFloatProviderStat(stat.AvgTokensPerSec, expectedTPS) {
		t.Fatalf("期望最近 5 条单条速度平均为 %f，实际 %f", expectedTPS, stat.AvgTokensPerSec)
	}
	if stat.TTFTSampleCount != 5 || stat.TPSSampleCount != 5 {
		t.Fatalf("期望首速样本数均为 5，实际 ttft=%d tps=%d", stat.TTFTSampleCount, stat.TPSSampleCount)
	}
	failedOnlyStat := findProviderStatByID(stats, "pid-failed-only")
	if failedOnlyStat == nil {
		t.Fatalf("未找到 provider_id=pid-failed-only 的当日统计")
	}
	if failedOnlyStat.AvgFirstTokenSec != 0 || failedOnlyStat.AvgTokensPerSec != 0 {
		t.Fatalf("失败请求不应进入首页性能样本，实际 ttft=%f tps=%f", failedOnlyStat.AvgFirstTokenSec, failedOnlyStat.AvgTokensPerSec)
	}
}

func TestProviderStatsRangeV2_PreservesStoredCostSnapshot(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}

	usage := modelpricing.UsageSnapshot{
		InputTokens:  1600,
		OutputTokens: 420,
	}
	modelBreakdown := pricing.CalculateCost("gpt-5", usage)
	responseBreakdown := pricing.CalculateCost("claude-sonnet-4", usage)
	if !modelBreakdown.HasPricing || !responseBreakdown.HasPricing {
		t.Fatalf("测试模型未命中价格表，前提不成立")
	}
	if almostEqualFloatProviderStat(modelBreakdown.TotalCost, responseBreakdown.TotalCost) {
		t.Fatalf("测试模型价格刚好相同，无法验证历史金额快照")
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		Model:         "gpt-5",
		ResponseModel: "claude-sonnet-4",
		ProviderID:    "pid-cost",
		Provider:      "Cost Provider",
		InputTokens:   usage.InputTokens,
		OutputTokens:  usage.OutputTokens,
		TotalCost:     modelBreakdown.TotalCost,
		CreatedAt:     "2026-02-25 10:00:00",
	})

	ls := NewLogService(nil)
	stats, err := ls.ProviderStatsRangeV2("codex", "pid-cost", "", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("ProviderStatsRangeV2 调用失败: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("期望返回 1 条 provider 统计，实际 %d", len(stats))
	}
	if !almostEqualFloatProviderStat(stats[0].CostTotal, modelBreakdown.TotalCost) {
		t.Fatalf("CostTotal = %.12f, 期望保留历史快照 %.12f", stats[0].CostTotal, modelBreakdown.TotalCost)
	}
}

func TestDefaultRangeEnd_DayBoundaryUsesNextLocalMidnight(t *testing.T) {
	start := time.Date(2026, 2, 28, 0, 0, 0, 0, time.Local)
	expected := start.AddDate(0, 0, 1)
	if end := defaultRangeEnd(start); !end.Equal(expected) {
		t.Fatalf("期望日边界默认结束时间为次日零点 %s，实际 %s", expected.Format(timeLayout), end.Format(timeLayout))
	}

	nonBoundary := time.Date(2026, 2, 28, 9, 30, 0, 0, time.Local)
	expectedNonBoundary := nonBoundary.Add(24 * time.Hour)
	if end := defaultRangeEnd(nonBoundary); !end.Equal(expectedNonBoundary) {
		t.Fatalf("期望非日边界默认结束时间为 +24h %s，实际 %s", expectedNonBoundary.Format(timeLayout), end.Format(timeLayout))
	}
}

type providerStatsLogEntry struct {
	Platform       string
	Model          string
	RequestedModel string
	ResponseModel  string
	ProviderID     string
	Provider       string
	HttpCode       int
	InputTokens    int
	IsStream       int
	DurationSec    float64
	FirstTokenSec  float64
	OutputTokens   int
	TotalCost      float64
	CreatedAt      string
}

func insertRequestLogForProviderStats(t *testing.T, db *sql.DB, entry providerStatsLogEntry) {
	t.Helper()
	httpCode := entry.HttpCode
	if httpCode == 0 {
		httpCode = 200
	}
	_, err := db.Exec(`
		INSERT INTO request_log (
			platform,
			model,
			requested_model,
			response_model,
			provider_id,
			provider,
			http_code,
			input_tokens,
			output_tokens,
			cache_create_tokens,
			cache_read_tokens,
			reasoning_tokens,
			is_stream,
			duration_sec,
			first_token_sec,
			total_cost,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.Platform,
		entry.Model,
		entry.RequestedModel,
		entry.ResponseModel,
		entry.ProviderID,
		entry.Provider,
		httpCode,
		entry.InputTokens,
		entry.OutputTokens,
		0,
		0,
		0,
		entry.IsStream,
		entry.DurationSec,
		entry.FirstTokenSec,
		entry.TotalCost,
		entry.CreatedAt,
	)
	if err != nil {
		t.Fatalf("插入 request_log 失败: %v", err)
	}
}

func findProviderStatByID(stats []ProviderDailyStat, providerID string) *ProviderDailyStat {
	for i := range stats {
		if stats[i].ProviderID == providerID {
			return &stats[i]
		}
	}
	return nil
}

func almostEqualFloatProviderStat(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}
