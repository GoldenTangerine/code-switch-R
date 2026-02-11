package services

import "testing"

func TestBuildProviderModelPricingResponse_MapsCacheMultipliers(t *testing.T) {
	response := buildProviderModelPricingResponse(
		SiteTypeUnknown,
		"api/pricing",
		&providerPricingResponse{
			Success: true,
			Data: []providerModelPricing{
				{
					ModelName:             "claude-opus-4-6",
					QuotaType:             0,
					ModelRatio:            2.5,
					CompletionRatio:       5,
					CacheCreateMultiplier: 1.25,
					CacheReadRatio:        0.1,
				},
			},
			GroupRatio: map[string]float64{"default": 1},
		},
	)

	if response == nil {
		t.Fatalf("buildProviderModelPricingResponse 返回 nil")
	}
	if len(response.Models) != 1 {
		t.Fatalf("返回模型数量 = %d, 期望 1", len(response.Models))
	}

	item := response.Models[0]
	if item.CacheCreateMultiplier != 1.25 {
		t.Fatalf("CacheCreateMultiplier = %.4f, 期望 1.2500", item.CacheCreateMultiplier)
	}
	if item.CacheReadMultiplier != 0.1 {
		t.Fatalf("CacheReadMultiplier = %.4f, 期望 0.1000", item.CacheReadMultiplier)
	}
}
