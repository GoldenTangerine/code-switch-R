package services

import (
	"testing"

	modelpricing "codeswitch/resources/model-pricing"
)

func TestApplyLogPricing_UsesResponseModelToRefreshDisplayedCost(t *testing.T) {
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

	if !floatEquals(logEntry.TotalCost, responseBreakdown.TotalCost) {
		t.Fatalf("TotalCost = %.12f, 期望按 response_model 纠偏为 %.12f", logEntry.TotalCost, responseBreakdown.TotalCost)
	}
	if logEntry.MatchedPricingModel != responseBreakdown.PricingModel {
		t.Fatalf("MatchedPricingModel = %q, 期望 %q", logEntry.MatchedPricingModel, responseBreakdown.PricingModel)
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
