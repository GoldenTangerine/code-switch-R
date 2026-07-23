package services

import (
	"database/sql"
	"fmt"
	"testing"

	modelpricing "codeswitch/resources/model-pricing"
	"github.com/daodao97/xgo/xdb"
)

func TestStatsRangeV2_PreservesStoredAggregatedCostSnapshot(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}

	usageA := modelpricing.UsageSnapshot{
		InputTokens:  1200,
		OutputTokens: 300,
	}
	modelBreakdownA := pricing.CalculateCost("gpt-5", usageA)
	responseBreakdownA := pricing.CalculateCost("claude-sonnet-4", usageA)
	if !modelBreakdownA.HasPricing || !responseBreakdownA.HasPricing {
		t.Fatalf("测试模型未命中价格表，前提不成立")
	}
	if floatEquals(modelBreakdownA.TotalCost, responseBreakdownA.TotalCost) {
		t.Fatalf("测试模型价格刚好相同，无法验证历史金额快照")
	}

	usageB := modelpricing.UsageSnapshot{
		InputTokens:  800,
		OutputTokens: 120,
	}
	modelBreakdownB := pricing.CalculateCost("gpt-5-mini", usageB)
	responseBreakdownB := pricing.CalculateCost("claude-sonnet-4", usageB)
	if !modelBreakdownB.HasPricing || !responseBreakdownB.HasPricing {
		t.Fatalf("测试模型未命中价格表，前提不成立")
	}
	if floatEquals(modelBreakdownB.TotalCost, responseBreakdownB.TotalCost) {
		t.Fatalf("测试模型价格刚好相同，无法验证历史金额快照")
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForStatsRange(t, db, statsRangeLogEntry{
		Platform:      "codex",
		Model:         "gpt-5",
		ResponseModel: "claude-sonnet-4",
		InputTokens:   usageA.InputTokens,
		OutputTokens:  usageA.OutputTokens,
		TotalCost:     modelBreakdownA.TotalCost,
		CreatedAt:     "2026-02-25 10:00:00",
	})
	insertRequestLogForStatsRange(t, db, statsRangeLogEntry{
		Platform:      "codex",
		Model:         "gpt-5-mini",
		ResponseModel: "claude-sonnet-4",
		InputTokens:   usageB.InputTokens,
		OutputTokens:  usageB.OutputTokens,
		TotalCost:     modelBreakdownB.TotalCost,
		CreatedAt:     "2026-02-25 11:00:00",
	})

	ls := NewLogService(nil)
	stats, err := ls.StatsRangeV2("codex", "", "", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("StatsRangeV2 调用失败: %v", err)
	}

	wantTotalCost := modelBreakdownA.TotalCost + modelBreakdownB.TotalCost
	if !floatEquals(stats.CostTotal, wantTotalCost) {
		t.Fatalf("CostTotal = %.12f, 期望按历史快照汇总为 %.12f", stats.CostTotal, wantTotalCost)
	}

	nonZeroBuckets := make([]LogStatsSeries, 0, 2)
	for _, bucket := range stats.Series {
		if bucket.TotalRequests > 0 || bucket.TotalCost > 0 {
			nonZeroBuckets = append(nonZeroBuckets, bucket)
		}
	}
	if len(nonZeroBuckets) != 2 {
		t.Fatalf("期望命中 2 个非空统计桶，实际 %d", len(nonZeroBuckets))
	}
	if !floatEquals(nonZeroBuckets[0].TotalCost, modelBreakdownA.TotalCost) {
		t.Fatalf("首个非空桶 TotalCost = %.12f, 期望 %.12f", nonZeroBuckets[0].TotalCost, modelBreakdownA.TotalCost)
	}
	if !floatEquals(nonZeroBuckets[1].TotalCost, modelBreakdownB.TotalCost) {
		t.Fatalf("第二个非空桶 TotalCost = %.12f, 期望 %.12f", nonZeroBuckets[1].TotalCost, modelBreakdownB.TotalCost)
	}
}

func TestRequestOutcomeControlsSuccessRateAndFailureLists(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	rows := []struct {
		httpCode int
		outcome  string
		reason   string
	}{
		{httpCode: 200, outcome: requestOutcomeSuccess, reason: requestOutcomeReasonProtocolCompleted},
		{httpCode: 502, outcome: requestOutcomeFailure, reason: requestOutcomeReasonUpstreamStreamError},
		{httpCode: 499, outcome: requestOutcomeExcluded, reason: requestOutcomeReasonClientAbort},
		{httpCode: 502},
		{httpCode: 503, outcome: "unknown"},
		{httpCode: 503, outcome: "  " + requestOutcomeSuccess + "  "},
	}
	for index, row := range rows {
		if _, err := db.Exec(`
			INSERT INTO request_log (
				platform, model, provider_id, provider, http_code,
				request_outcome, outcome_reason, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, "codex", "gpt-5", "outcome-provider", "Outcome Provider", row.httpCode, row.outcome, row.reason, fmt.Sprintf("2026-02-25 10:0%d:00", index)); err != nil {
			t.Fatalf("插入测试日志失败: %v", err)
		}
	}

	ls := NewLogService(nil)
	summary, err := ls.SummaryRangeV2("codex", "outcome-provider", "", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("SummaryRangeV2 调用失败: %v", err)
	}
	if summary.TotalRequests != 6 || summary.SuccessfulRequests != 2 || summary.FailedRequests != 3 || summary.ExcludedRequests != 1 {
		t.Fatalf("三态汇总错误: %#v", summary)
	}
	if !floatEquals(summary.SuccessRate, 2.0/5.0) {
		t.Fatalf("成功率=%f，期望 %f", summary.SuccessRate, 2.0/5.0)
	}

	providerStats, err := ls.ProviderStatsRangeV2("codex", "outcome-provider", "", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil || len(providerStats) != 1 {
		t.Fatalf("ProviderStatsRangeV2 结果错误: stats=%#v err=%v", providerStats, err)
	}
	if providerStats[0].ExcludedRequests != 1 || !floatEquals(providerStats[0].SuccessRate, 2.0/5.0) {
		t.Fatalf("供应商三态统计错误: %#v", providerStats[0])
	}

	failedPage, err := ls.ListFailedRequestLogsPageV2("codex", "outcome-provider", 20, 0, "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil || failedPage.Total != 3 {
		t.Fatalf("失败列表应包含显式失败、旧日志失败和未知结果失败: total=%d err=%v", failedPage.Total, err)
	}
}

type statsRangeLogEntry struct {
	Platform       string
	Model          string
	RequestedModel string
	ResponseModel  string
	InputTokens    int
	OutputTokens   int
	TotalCost      float64
	CreatedAt      string
}

func insertRequestLogForStatsRange(t *testing.T, db *sql.DB, entry statsRangeLogEntry) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO request_log (
			platform,
			model,
			requested_model,
			response_model,
			provider,
			http_code,
			input_tokens,
			output_tokens,
			cache_create_tokens,
			cache_read_tokens,
			reasoning_tokens,
			total_cost,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.Platform,
		entry.Model,
		entry.RequestedModel,
		entry.ResponseModel,
		"stats-provider",
		200,
		entry.InputTokens,
		entry.OutputTokens,
		0,
		0,
		0,
		entry.TotalCost,
		entry.CreatedAt,
	)
	if err != nil {
		t.Fatalf("插入 request_log 失败: %v", err)
	}
}
