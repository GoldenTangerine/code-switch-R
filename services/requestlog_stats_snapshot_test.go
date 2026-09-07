/*
@name: 统计回填计费快照测试
@Descripttion: 验证统计回填保留已记录的零费用与缺价结果。
@version: 1.0.0
@Author: sm
@Date: 2026-09-07 11:30:00
@LastEditTime: 2026-09-07 11:30:00
@FilePath: services/requestlog_stats_snapshot_test.go
*/
package services

import (
	"context"
	"testing"

	modelpricing "codeswitch/resources/model-pricing"
)

func TestEstimateBackfillPreservesRecordedPricing(t *testing.T) {
	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatal(err)
	}
	for _, stored := range []float64{0, 12, -1} {
		if got := estimateBackfillTotalCost(pricing, "claude-sonnet-4-5", 1000000, 1000, 0, 0, 0, stored, true); got != stored {
			t.Fatalf("recorded %v became %v", stored, got)
		}
	}
	if got := estimateBackfillTotalCost(pricing, "claude-sonnet-4-5", 1000000, 1000, 0, 0, 0, 0, false); got <= 0 {
		t.Fatal("legacy missing cost no longer estimated")
	}
}

func TestBackfillStatsPreservesSnapshotAndFreePricing(t *testing.T) {
	_, db, _ := prepareSessionUsageTest(t)
	if err := dropRequestLogStatsInsertTriggersWithDB(db); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		provider, snapshot string
		hasPricing         int
	}{
		{"free-provider", "", 1},
		{"missing-price", `{"unit_prices":{},"field_sources":{},"complete":false}`, 0},
		{"legacy", "", 0},
	} {
		err := GlobalDBQueue.ExecCtx(context.Background(), `INSERT INTO request_log (platform,provider_id,provider,model,input_tokens,total_cost,has_pricing,pricing_snapshot,created_at,data_source) VALUES (?,?,?,?,?,?,?,?,?,?)`, "claude", c.provider, c.provider, "claude-sonnet-4-5", 1000000, 0, c.hasPricing, c.snapshot, "2026-09-01 01:00:00", "proxy")
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := backfillRequestLogStatsWithDB(db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{requestLogStatsHourlyTable, requestLogStatsDailyTable} {
		for _, provider := range []string{"free-provider", "missing-price", "legacy"} {
			var total float64
			if err := db.QueryRow("SELECT total_cost FROM "+table+" WHERE provider_id = ?", provider).Scan(&total); err != nil {
				t.Fatal(err)
			}
			if provider == "legacy" {
				if total <= 0 {
					t.Fatal("legacy estimate missing")
				}
			} else if total != 0 {
				t.Fatalf("%s %s changed recorded zero to %v", table, provider, total)
			}
		}
	}
}

func TestPricingResponseHookNonstreamFragments(t *testing.T) {
	log := &ReqeustLog{PricingContext: &modelpricing.PricingContext{ConditionsKnown: true, ServiceTier: "priority"}}
	hook := requestLogPricingResponseHook(log)
	hook([]byte(`{"response":{"service_tier":"`))
	if log.PricingContext.ServiceTier != "priority" {
		t.Fatal("partial JSON changed tier")
	}
	hook([]byte(`flex","model":"upstream"}}`))
	if log.PricingContext.ServiceTier != "flex" {
		t.Fatal("nonstream fragmented JSON not captured")
	}
	if log.ResponseModel != "" {
		t.Fatal("pricing hook changed unrelated response metadata")
	}
}
