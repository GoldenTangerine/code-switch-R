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

func TestEnrichProviderModelPricingResponse_PreservesProviderMultipliers(t *testing.T) {
	providerService := NewProviderService()
	response := &ProviderModelPricingResponse{
		Models: []ProviderModelPricingItem{
			{
				Model:                 "claude-opus-4-5",
				QuotaType:             0,
				InputUSDPerM:          15,
				OutputUSDPerM:         75,
				CacheCreateMultiplier: 1.25,
				CacheReadMultiplier:   0.1,
			},
		},
	}

	providerService.enrichProviderModelPricingResponse(response, "", "", "")

	item := response.Models[0]
	if item.CacheCreateMultiplier != 1.25 || item.CacheReadMultiplier != 0.1 {
		t.Fatalf("provider 原始倍率被污染: create=%.4f read=%.4f", item.CacheCreateMultiplier, item.CacheReadMultiplier)
	}
	if item.ResolvedCacheCreateMultiplier != 1.25 || item.ResolvedCacheReadMultiplier != 0.1 {
		t.Fatalf("provider resolved 倍率错误: create=%.4f read=%.4f", item.ResolvedCacheCreateMultiplier, item.ResolvedCacheReadMultiplier)
	}
	if item.CacheCreateMultiplierSource != providerCacheMultiplierSourceProvider {
		t.Fatalf("CacheCreateMultiplierSource = %q, 期望 %q", item.CacheCreateMultiplierSource, providerCacheMultiplierSourceProvider)
	}
	if item.CacheReadMultiplierSource != providerCacheMultiplierSourceProvider {
		t.Fatalf("CacheReadMultiplierSource = %q, 期望 %q", item.CacheReadMultiplierSource, providerCacheMultiplierSourceProvider)
	}
}

func TestEnrichProviderModelPricingResponse_DefaultsForGPT(t *testing.T) {
	providerService := NewProviderService()
	response := &ProviderModelPricingResponse{
		Models: []ProviderModelPricingItem{
			{
				Model:                 "gpt-4.1",
				QuotaType:             0,
				InputUSDPerM:          2,
				OutputUSDPerM:         8,
				CompletionRatio:       4,
				CacheCreateMultiplier: 0,
				CacheReadMultiplier:   0,
			},
		},
	}

	providerService.enrichProviderModelPricingResponse(response, "", "", "")

	item := response.Models[0]
	if item.CacheCreateMultiplier != 0 {
		t.Fatalf("GPT CacheCreateMultiplier 原始值 = %.4f, 期望保持 0.0000", item.CacheCreateMultiplier)
	}
	if item.CacheReadMultiplier != 0 {
		t.Fatalf("GPT CacheReadMultiplier 原始值 = %.4f, 期望保持 0.0000", item.CacheReadMultiplier)
	}
	if item.ResolvedCacheCreateMultiplier != 1 {
		t.Fatalf("GPT ResolvedCacheCreateMultiplier = %.4f, 期望 1.0000", item.ResolvedCacheCreateMultiplier)
	}
	if item.ResolvedCacheReadMultiplier != 1 {
		t.Fatalf("GPT ResolvedCacheReadMultiplier = %.4f, 期望 1.0000", item.ResolvedCacheReadMultiplier)
	}
	if item.CacheCreateMultiplierSource != providerCacheMultiplierSourceFallback {
		t.Fatalf("GPT CacheCreateMultiplierSource = %q, 期望 %q", item.CacheCreateMultiplierSource, providerCacheMultiplierSourceFallback)
	}
	if item.CacheReadMultiplierSource != providerCacheMultiplierSourceFallback {
		t.Fatalf("GPT CacheReadMultiplierSource = %q, 期望 %q", item.CacheReadMultiplierSource, providerCacheMultiplierSourceFallback)
	}
}

func TestEnrichProviderModelPricingResponse_DefaultsForClaude(t *testing.T) {
	providerService := NewProviderService()
	response := &ProviderModelPricingResponse{
		Models: []ProviderModelPricingItem{
			{
				Model:           "claude-sonnet-4-5",
				QuotaType:       0,
				InputUSDPerM:    3,
				OutputUSDPerM:   15,
				CompletionRatio: 5,
			},
		},
	}

	providerService.enrichProviderModelPricingResponse(response, "", "", "")

	item := response.Models[0]
	if item.CacheCreateMultiplier != 0 {
		t.Fatalf("Claude CacheCreateMultiplier 原始值 = %.4f, 期望保持 0.0000", item.CacheCreateMultiplier)
	}
	if item.CacheReadMultiplier != 0 {
		t.Fatalf("Claude CacheReadMultiplier 原始值 = %.4f, 期望保持 0.0000", item.CacheReadMultiplier)
	}
	if item.ResolvedCacheCreateMultiplier != providerAPIDefaultCacheCreateMultiplier {
		t.Fatalf("Claude ResolvedCacheCreateMultiplier = %.4f, 期望 %.4f", item.ResolvedCacheCreateMultiplier, providerAPIDefaultCacheCreateMultiplier)
	}
	if item.ResolvedCacheReadMultiplier != providerAPIDefaultCacheReadMultiplier {
		t.Fatalf("Claude ResolvedCacheReadMultiplier = %.4f, 期望 %.4f", item.ResolvedCacheReadMultiplier, providerAPIDefaultCacheReadMultiplier)
	}
	if item.CacheCreateMultiplierSource != providerCacheMultiplierSourceFallback {
		t.Fatalf("Claude CacheCreateMultiplierSource = %q, 期望 %q", item.CacheCreateMultiplierSource, providerCacheMultiplierSourceFallback)
	}
	if item.CacheReadMultiplierSource != providerCacheMultiplierSourceFallback {
		t.Fatalf("Claude CacheReadMultiplierSource = %q, 期望 %q", item.CacheReadMultiplierSource, providerCacheMultiplierSourceFallback)
	}
}
