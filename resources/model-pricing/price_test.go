package modelpricing

import "testing"

func TestCalculateCostUsesFuzzyPricingForUnknownModel(t *testing.T) {
	service := newUnitTestPricingService()

	breakdown := service.CalculateCost("claude-opus-4-6", UsageSnapshot{
		InputTokens:  1000,
		OutputTokens: 500,
	})

	if !breakdown.HasPricing {
		t.Fatalf("HasPricing = false, 期望 true")
	}
	if !breakdown.FuzzyMatched {
		t.Fatalf("FuzzyMatched = false, 期望 true")
	}
	if breakdown.PricingModel != "claude-opus-4-1" {
		t.Fatalf("PricingModel = %q, 期望 %q", breakdown.PricingModel, "claude-opus-4-1")
	}
	if breakdown.TotalCost <= 0 {
		t.Fatalf("TotalCost = %f, 期望 > 0", breakdown.TotalCost)
	}
}

func TestCalculateCostExactMatchDoesNotMarkFuzzy(t *testing.T) {
	service := newUnitTestPricingService()

	breakdown := service.CalculateCost("claude-sonnet-4", UsageSnapshot{
		InputTokens:  500,
		OutputTokens: 200,
	})

	if !breakdown.HasPricing {
		t.Fatalf("HasPricing = false, 期望 true")
	}
	if breakdown.FuzzyMatched {
		t.Fatalf("FuzzyMatched = true, 期望 false")
	}
	if breakdown.PricingModel != "claude-sonnet-4" {
		t.Fatalf("PricingModel = %q, 期望 %q", breakdown.PricingModel, "claude-sonnet-4")
	}
}

func TestCalculateCostUnknownModelKeepsNoPricing(t *testing.T) {
	service := newUnitTestPricingService()

	breakdown := service.CalculateCost("totally-random-unknown-model", UsageSnapshot{
		InputTokens:  500,
		OutputTokens: 200,
	})

	if breakdown.HasPricing {
		t.Fatalf("HasPricing = true, 期望 false")
	}
	if breakdown.TotalCost != 0 {
		t.Fatalf("TotalCost = %f, 期望 0", breakdown.TotalCost)
	}
	if breakdown.PricingModel != "" {
		t.Fatalf("PricingModel = %q, 期望空字符串", breakdown.PricingModel)
	}
}

func TestCalculateCostAliasMatchNotMarkedFuzzy(t *testing.T) {
	service := newUnitTestPricingService()

	breakdown := service.CalculateCost("gpt-5-codex", UsageSnapshot{
		InputTokens:  1000,
		OutputTokens: 1000,
	})

	if !breakdown.HasPricing {
		t.Fatalf("HasPricing = false, 期望 true")
	}
	if breakdown.FuzzyMatched {
		t.Fatalf("FuzzyMatched = true, 期望 false")
	}
	if breakdown.PricingModel != "gpt-5" {
		t.Fatalf("PricingModel = %q, 期望 %q", breakdown.PricingModel, "gpt-5")
	}
}

func TestExplicitEphemeral1hCostPerToken_DoesNotUseFamilyFallback(t *testing.T) {
	service := newUnitTestPricingService()

	if explicit, ok := service.ExplicitEphemeral1hCostPerToken("claude-sonnet-proxy"); ok || explicit != 0 {
		t.Fatalf("ExplicitEphemeral1hCostPerToken = (%f, %v), 期望 (0, false)", explicit, ok)
	}

	if fallback := service.Ephemeral1hCostPerToken("claude-sonnet-proxy"); fallback <= 0 {
		t.Fatalf("Ephemeral1hCostPerToken = %f, 期望 > 0", fallback)
	}
}

func newUnitTestPricingService() *Service {
	pricing := map[string]*PricingEntry{
		"claude-opus-4-1": {
			InputCostPerToken:       0.000015,
			OutputCostPerToken:      0.000075,
			CacheReadInputTokenCost: 0.0000015,
		},
		"claude-sonnet-4": {
			InputCostPerToken:       0.000003,
			OutputCostPerToken:      0.000015,
			CacheReadInputTokenCost: 0.0000003,
		},
		"gpt-5": {
			InputCostPerToken:  0.00000125,
			OutputCostPerToken: 0.00001,
		},
	}

	normalized := map[string]string{}
	for key := range pricing {
		normalized[normalizeName(key)] = key
	}

	return &Service{
		pricingMap:   pricing,
		normalized:   normalized,
		ephemeral1h:  map[string]float64{"claude-opus-4-1": 0.00003},
		longContexts: map[string]LongContextPricing{},
	}
}
