package services

import (
	"testing"

	modelpricing "codeswitch/resources/model-pricing"
)

func TestApplyLogPricing_PreservesStoredCostSnapshot(t *testing.T) {
	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}

	usage := modelpricing.UsageSnapshot{
		InputTokens:  1800,
		OutputTokens: 600,
	}
	modelBreakdown := pricing.CalculateCost("gpt-5", usage)
	responseBreakdown := pricing.CalculateCost("claude-sonnet-4", usage)
	if !modelBreakdown.HasPricing || !responseBreakdown.HasPricing {
		t.Fatalf("测试模型未命中价格表，前提不成立")
	}
	if floatEquals(modelBreakdown.TotalCost, responseBreakdown.TotalCost) {
		t.Fatalf("测试模型价格刚好相同，无法验证 response_model 优先逻辑")
	}

	logEntry := &ReqeustLog{
		Model:         "gpt-5",
		ResponseModel: "claude-sonnet-4",
		InputTokens:   usage.InputTokens,
		OutputTokens:  usage.OutputTokens,
		TotalCost:     modelBreakdown.TotalCost,
		PriceSource:   requestLogPriceSourceBuiltin,
	}

	applyLogPricing(pricing, logEntry)

	if !floatEquals(logEntry.TotalCost, modelBreakdown.TotalCost) {
		t.Fatalf("TotalCost = %.12f, 期望保留写入时快照 %.12f", logEntry.TotalCost, modelBreakdown.TotalCost)
	}
	if logEntry.MatchedPricingModel != "" {
		t.Fatalf("MatchedPricingModel = %q，已有快照时不应按当前 response_model 改写", logEntry.MatchedPricingModel)
	}
}

func TestApplyLogPricing_NormalizesNoneSourceForStoredCostSnapshot(t *testing.T) {
	logEntry := &ReqeustLog{
		Model:       "gpt-5",
		TotalCost:   0.024,
		PriceSource: requestLogPriceSourceNone,
	}

	applyLogPricing(nil, logEntry)

	if !logEntry.HasPricing {
		t.Fatalf("HasPricing = false，已有历史金额快照时应标记为已命中价格")
	}
	if logEntry.PriceSource != requestLogPriceSourceBuiltin {
		t.Fatalf("PriceSource = %q, 期望规范化为 %q", logEntry.PriceSource, requestLogPriceSourceBuiltin)
	}
}

func TestApplyLogPricing_FillsMissingTotalFromStoredBreakdown(t *testing.T) {
	logEntry := &ReqeustLog{
		Model:           "gpt-5",
		ResponseModel:   "gpt-5.5",
		InputCost:       0.001,
		OutputCost:      0.002,
		ReasoningCost:   0.003,
		Ephemeral5mCost: 0.004,
		Ephemeral1hCost: 0.005,
		CacheReadCost:   0.006,
		TotalCost:       0,
		PriceSource:     requestLogPriceSourceBuiltin,
	}

	applyLogPricing(nil, logEntry)

	wantTotal := 0.021
	if !floatEquals(logEntry.TotalCost, wantTotal) {
		t.Fatalf("TotalCost = %.12f, 期望按已存成本拆分补齐为 %.12f", logEntry.TotalCost, wantTotal)
	}
	if !logEntry.HasPricing {
		t.Fatalf("HasPricing = false，已有历史成本拆分时应标记为已命中价格")
	}
}

func TestApplyLogPricing_FillsMissingCostFromResponseModel(t *testing.T) {
	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}

	usage := modelpricing.UsageSnapshot{
		InputTokens:  1800,
		OutputTokens: 600,
	}
	responseBreakdown := pricing.CalculateCost("claude-sonnet-4", usage)
	if !responseBreakdown.HasPricing {
		t.Fatalf("测试模型未命中价格表，前提不成立")
	}

	logEntry := &ReqeustLog{
		Model:         "gpt-5",
		ResponseModel: "claude-sonnet-4",
		InputTokens:   usage.InputTokens,
		OutputTokens:  usage.OutputTokens,
		TotalCost:     0,
		PriceSource:   requestLogPriceSourceNone,
	}

	applyLogPricing(pricing, logEntry)

	if !floatEquals(logEntry.TotalCost, responseBreakdown.TotalCost) {
		t.Fatalf("TotalCost = %.12f, 缺少历史快照时期望按 response_model 补算为 %.12f", logEntry.TotalCost, responseBreakdown.TotalCost)
	}
	if logEntry.MatchedPricingModel != responseBreakdown.PricingModel {
		t.Fatalf("MatchedPricingModel = %q, 期望 %q", logEntry.MatchedPricingModel, responseBreakdown.PricingModel)
	}
}

func TestApplyLogPricing_PreservesStoredCostWhenMatchedModelIsUnchanged(t *testing.T) {
	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}

	usage := modelpricing.UsageSnapshot{
		InputTokens:     10170,
		OutputTokens:    919,
		CacheReadTokens: 16000,
	}
	breakdown := pricing.CalculateCost("gpt-5", usage)
	if !breakdown.HasPricing {
		t.Fatalf("测试模型未命中价格表，前提不成立")
	}
	oldStoredCost := breakdown.TotalCost * 0.25
	if floatEquals(oldStoredCost, breakdown.TotalCost) {
		t.Fatalf("测试旧金额与当前金额相同，无法验证历史快照保留")
	}

	logEntry := &ReqeustLog{
		Model:           "gpt-5",
		ResponseModel:   "gpt-5",
		InputTokens:     usage.InputTokens,
		OutputTokens:    usage.OutputTokens,
		CacheReadTokens: usage.CacheReadTokens,
		TotalCost:       oldStoredCost,
		PriceSource:     requestLogPriceSourceBuiltin,
	}

	applyLogPricing(pricing, logEntry)

	if !floatEquals(logEntry.TotalCost, oldStoredCost) {
		t.Fatalf("TotalCost = %.12f, 期望保留历史快照 %.12f", logEntry.TotalCost, oldStoredCost)
	}
	if !floatEquals(logEntry.CacheReadCost, 0) {
		t.Fatalf("CacheReadCost = %.12f, 只有 total_cost 快照时不应按当前价表补写拆分", logEntry.CacheReadCost)
	}
}

func TestApplyLogPricing_DoesNotFallbackToLocalModelWhenNoResponseOrRequestedModel(t *testing.T) {
	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}

	logEntry := &ReqeustLog{
		Model:        "gpt-5",
		InputTokens:  1800,
		OutputTokens: 600,
		TotalCost:    0,
		PriceSource:  requestLogPriceSourceNone,
	}

	applyLogPricing(pricing, logEntry)

	if !floatEquals(logEntry.TotalCost, 0) {
		t.Fatalf("TotalCost = %.12f, 期望保持 0", logEntry.TotalCost)
	}
	if logEntry.MatchedPricingModel != "" {
		t.Fatalf("MatchedPricingModel = %q, 期望为空", logEntry.MatchedPricingModel)
	}
	if logEntry.HasPricing {
		t.Fatalf("HasPricing = true，缺少 response_model/requested_model 时不应再按本地 model 标记为已命中价格")
	}
	if logEntry.PriceSource != requestLogPriceSourceNone {
		t.Fatalf("PriceSource = %q, 期望保持 %q", logEntry.PriceSource, requestLogPriceSourceNone)
	}
}
