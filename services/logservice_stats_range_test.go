package services

import (
	"database/sql"
	"testing"

	modelpricing "codeswitch/resources/model-pricing"
	"github.com/daodao97/xgo/xdb"
)

func TestStatsRangeV2_UsesResponseModelToRefreshAggregatedCost(t *testing.T) {
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

	usageB := modelpricing.UsageSnapshot{
		InputTokens:  800,
		OutputTokens: 120,
	}
	modelBreakdownB := pricing.CalculateCost("gpt-5-mini", usageB)
	responseBreakdownB := pricing.CalculateCost("claude-sonnet-4", usageB)
	if !modelBreakdownB.HasPricing || !responseBreakdownB.HasPricing {
		t.Fatalf("测试模型未命中价格表，前提不成立")
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
	stats, err := ls.StatsRangeV2("codex", "", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("StatsRangeV2 调用失败: %v", err)
	}

	wantTotalCost := responseBreakdownA.TotalCost + responseBreakdownB.TotalCost
	if !floatEquals(stats.CostTotal, wantTotalCost) {
		t.Fatalf("CostTotal = %.12f, 期望按 response_model 汇总为 %.12f", stats.CostTotal, wantTotalCost)
	}
	if !floatEquals(stats.CostInput, responseBreakdownA.InputCost+responseBreakdownB.InputCost) {
		t.Fatalf("CostInput = %.12f, 期望 %.12f", stats.CostInput, responseBreakdownA.InputCost+responseBreakdownB.InputCost)
	}
	if !floatEquals(stats.CostOutput, responseBreakdownA.OutputCost+responseBreakdownB.OutputCost) {
		t.Fatalf("CostOutput = %.12f, 期望 %.12f", stats.CostOutput, responseBreakdownA.OutputCost+responseBreakdownB.OutputCost)
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
	if !floatEquals(nonZeroBuckets[0].TotalCost, responseBreakdownA.TotalCost) {
		t.Fatalf("首个非空桶 TotalCost = %.12f, 期望 %.12f", nonZeroBuckets[0].TotalCost, responseBreakdownA.TotalCost)
	}
	if !floatEquals(nonZeroBuckets[1].TotalCost, responseBreakdownB.TotalCost) {
		t.Fatalf("第二个非空桶 TotalCost = %.12f, 期望 %.12f", nonZeroBuckets[1].TotalCost, responseBreakdownB.TotalCost)
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
