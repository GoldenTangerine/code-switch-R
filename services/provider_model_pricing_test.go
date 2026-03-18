package services

import (
	"testing"

	modelpricing "codeswitch/resources/model-pricing"
)

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

func TestEnrichProviderModelPricingResponse_AppliesManualGroupMultiplier(t *testing.T) {
	providerService := NewProviderService()
	scopeKey := providerPricingOverrideScopeKey("https://example.com/v1", "secret-key", "")
	providerService.providerPricingOverrides.Providers[scopeKey] = map[string]providerPricingOverrideItem{
		normalizeProviderPricingModelName("gpt-4.1"): {
			GroupMultiplier:    0.05,
			HasGroupMultiplier: true,
		},
	}

	response := &ProviderModelPricingResponse{
		Models: []ProviderModelPricingItem{
			{
				Model:         "gpt-4.1",
				QuotaType:     1,
				PerCallPrice:  &ProviderModelPerCallPrice{Unified: func() *float64 { v := 0.2; return &v }()},
				InputUSDPerM:  2,
				OutputUSDPerM: 8,
			},
		},
	}

	providerService.enrichProviderModelPricingResponse(response, "https://example.com/v1", "secret-key", "")

	item := response.Models[0]
	if item.GroupMultiplier != 0.05 {
		t.Fatalf("GroupMultiplier = %.4f, 期望 0.0500", item.GroupMultiplier)
	}
	if item.GroupMultiplierSource != providerGroupMultiplierSourceManual {
		t.Fatalf("GroupMultiplierSource = %q, 期望 %q", item.GroupMultiplierSource, providerGroupMultiplierSourceManual)
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

func TestEnrichProviderModelPricingResponse_SetsExplicit1hCachePrice(t *testing.T) {
	pricingService, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化模型价格服务失败: %v", err)
	}

	providerService := NewProviderService()
	providerService.BindModelPricingService(&ModelPricingService{effective: pricingService})
	response := &ProviderModelPricingResponse{
		Models: []ProviderModelPricingItem{
			{
				Model:           "claude-sonnet-4",
				QuotaType:       0,
				InputUSDPerM:    3,
				OutputUSDPerM:   15,
				CompletionRatio: 5,
			},
		},
	}

	providerService.enrichProviderModelPricingResponse(response, "", "", "")

	item := response.Models[0]
	if diff := item.CacheCreate1hUSDPerM - 6; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("CacheCreate1hUSDPerM = %.4f, 期望 6.0000", item.CacheCreate1hUSDPerM)
	}
}

func TestEnrichProviderModelPricingResponse_DoesNotUseFamilyFallbackAsExplicit1h(t *testing.T) {
	pricingService, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化模型价格服务失败: %v", err)
	}

	providerService := NewProviderService()
	providerService.BindModelPricingService(&ModelPricingService{effective: pricingService})
	response := &ProviderModelPricingResponse{
		Models: []ProviderModelPricingItem{
			{
				Model:           "claude-sonnet-proxy",
				QuotaType:       0,
				InputUSDPerM:    3,
				OutputUSDPerM:   15,
				CompletionRatio: 5,
			},
		},
	}

	providerService.enrichProviderModelPricingResponse(response, "", "", "")

	if response.Models[0].CacheCreate1hUSDPerM != 0 {
		t.Fatalf("CacheCreate1hUSDPerM = %.4f, 期望 0.0000", response.Models[0].CacheCreate1hUSDPerM)
	}
}

func TestNormalizeProviderModelPricingSource(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "", want: providerModelPricingSourceAuto},
		{input: "auto", want: providerModelPricingSourceAuto},
		{input: "/api/pricing", want: providerModelPricingSourceCommon},
		{input: "pricing", want: providerModelPricingSourceCommon},
		{input: "onehub", want: providerModelPricingSourceOneHub},
		{input: "/api/available_model", want: providerModelPricingSourceOneHub},
		{input: "/v1/models", want: providerModelPricingSourceOpenAIList},
		{input: "models", want: providerModelPricingSourceOpenAIList},
		{input: "weird-source", want: ""},
	}

	for _, tt := range tests {
		if got := normalizeProviderModelPricingSource(tt.input); got != tt.want {
			t.Fatalf("normalizeProviderModelPricingSource(%q) = %q, 期望 %q", tt.input, got, tt.want)
		}
	}
}

func TestShouldCacheProviderModelPricingResponse(t *testing.T) {
	tests := []struct {
		name            string
		requestedSource string
		response        *ProviderModelPricingResponse
		want            bool
	}{
		{
			name:            "auto fallback still caches",
			requestedSource: providerModelPricingSourceAuto,
			response: &ProviderModelPricingResponse{
				PricingSource: providerModelPricingSourceOpenAIList,
			},
			want: true,
		},
		{
			name:            "explicit common does not warm shared cache",
			requestedSource: providerModelPricingSourceCommon,
			response: &ProviderModelPricingResponse{
				PricingSource: providerModelPricingSourceCommon,
			},
			want: false,
		},
		{
			name:            "explicit one hub does not warm shared cache",
			requestedSource: providerModelPricingSourceOneHub,
			response: &ProviderModelPricingResponse{
				PricingSource: providerModelPricingSourceOneHub,
			},
			want: false,
		},
		{
			name:            "explicit openai list does not cache",
			requestedSource: providerModelPricingSourceOpenAIList,
			response: &ProviderModelPricingResponse{
				PricingSource: providerModelPricingSourceOpenAIList,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		if got := shouldCacheProviderModelPricingResponse(tt.requestedSource, tt.response); got != tt.want {
			t.Fatalf("%s: shouldCacheProviderModelPricingResponse(%q) = %v, 期望 %v", tt.name, tt.requestedSource, got, tt.want)
		}
	}
}
