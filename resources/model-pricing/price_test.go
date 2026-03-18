package modelpricing

import (
	"encoding/json"
	"math"
	"testing"
)

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

func TestCalculateCost_AppliesGroupMultiplierToAllCostParts(t *testing.T) {
	service := newUnitTestPricingService()
	service.pricingMap["claude-sonnet-4"].GroupMultiplier = 0.05
	service.pricingMap["claude-sonnet-4"].HasGroupMultiplier = true
	service.pricingMap["claude-sonnet-4"].CacheCreationInputTokenCost = 0.00000375

	breakdown := service.CalculateCost("claude-sonnet-4", UsageSnapshot{
		InputTokens:       1000,
		OutputTokens:      200,
		CacheCreateTokens: 100,
		CacheReadTokens:   50,
	})

	if breakdown.GroupMultiplier != 0.05 {
		t.Fatalf("GroupMultiplier = %f, 期望 0.05", breakdown.GroupMultiplier)
	}

	wantInput := 1000 * 0.000003 * 0.05
	wantOutput := 200 * 0.000015 * 0.05
	wantCacheCreate := 100 * 0.00000375 * 0.05
	wantCacheRead := 50 * 0.0000003 * 0.05
	wantTotal := wantInput + wantOutput + wantCacheCreate + wantCacheRead

	if math.Abs(breakdown.InputCost-wantInput) > 1e-12 {
		t.Fatalf("InputCost = %f, 期望 %f", breakdown.InputCost, wantInput)
	}
	if math.Abs(breakdown.OutputCost-wantOutput) > 1e-12 {
		t.Fatalf("OutputCost = %f, 期望 %f", breakdown.OutputCost, wantOutput)
	}
	if math.Abs(breakdown.CacheCreateCost-wantCacheCreate) > 1e-12 {
		t.Fatalf("CacheCreateCost = %f, 期望 %f", breakdown.CacheCreateCost, wantCacheCreate)
	}
	if math.Abs(breakdown.CacheReadCost-wantCacheRead) > 1e-12 {
		t.Fatalf("CacheReadCost = %f, 期望 %f", breakdown.CacheReadCost, wantCacheRead)
	}
	if math.Abs(breakdown.TotalCost-wantTotal) > 1e-12 {
		t.Fatalf("TotalCost = %f, 期望 %f", breakdown.TotalCost, wantTotal)
	}
}

func TestPricingEntryUnmarshal_RecognizesGroupMultiplierPresence(t *testing.T) {
	var entry PricingEntry
	if err := json.Unmarshal([]byte(`{"input_cost_per_token":0.1,"group_multiplier":0}`), &entry); err != nil {
		t.Fatalf("json.Unmarshal 失败: %v", err)
	}

	if !entry.HasGroupMultiplier {
		t.Fatalf("HasGroupMultiplier = false, 期望 true")
	}
	if entry.GroupMultiplier != 0 {
		t.Fatalf("GroupMultiplier = %f, 期望 0", entry.GroupMultiplier)
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
