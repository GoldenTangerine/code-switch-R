package services

import (
	"math"
	"testing"

	modelpricing "codeswitch/resources/model-pricing"
)

func floatEquals(got, want float64) bool {
	return math.Abs(got-want) <= 1e-12
}

func TestCalculateProviderAPICost_AppliesClaudeCacheFallbackMultiplier(t *testing.T) {
	item := ProviderModelPricingItem{
		QuotaType:     0,
		InputUSDPerM:  5,
		OutputUSDPerM: 25,
	}
	usage := modelpricing.UsageSnapshot{
		InputTokens:       3,
		OutputTokens:      82,
		CacheCreateTokens: 28544,
	}

	result, ok := calculateProviderAPICost(item, usage, nil, "claude-opus-4-5")
	if !ok {
		t.Fatalf("calculateProviderAPICost 返回 hasPricing=false")
	}

	wantInput := float64(usage.InputTokens) * (5.0 / 1_000_000.0)
	wantOutput := float64(usage.OutputTokens) * (25.0 / 1_000_000.0)
	wantCacheCreate := float64(usage.CacheCreateTokens) * ((5.0 * 1.25) / 1_000_000.0)
	wantTotal := wantInput + wantOutput + wantCacheCreate

	if !floatEquals(result.InputCost, wantInput) {
		t.Fatalf("InputCost = %.12f, 期望 %.12f", result.InputCost, wantInput)
	}
	if !floatEquals(result.OutputCost, wantOutput) {
		t.Fatalf("OutputCost = %.12f, 期望 %.12f", result.OutputCost, wantOutput)
	}
	if !floatEquals(result.CacheCreateCost, wantCacheCreate) {
		t.Fatalf("CacheCreateCost = %.12f, 期望 %.12f", result.CacheCreateCost, wantCacheCreate)
	}
	if !floatEquals(result.TotalCost, wantTotal) {
		t.Fatalf("TotalCost = %.12f, 期望 %.12f", result.TotalCost, wantTotal)
	}
}

func TestCalculateProviderAPICost_UsesProviderCacheMultipliersWhenProvided(t *testing.T) {
	item := ProviderModelPricingItem{
		QuotaType:             0,
		InputUSDPerM:          10,
		OutputUSDPerM:         20,
		CacheCreateMultiplier: 1.6,
		CacheReadMultiplier:   0.2,
	}
	usage := modelpricing.UsageSnapshot{
		InputTokens:       100,
		OutputTokens:      50,
		CacheCreateTokens: 10,
		CacheReadTokens:   30,
	}

	result, ok := calculateProviderAPICost(item, usage, nil, "any-model")
	if !ok {
		t.Fatalf("calculateProviderAPICost 返回 hasPricing=false")
	}

	inputPerToken := 10.0 / 1_000_000.0
	outputPerToken := 20.0 / 1_000_000.0
	wantCacheCreate := float64(usage.CacheCreateTokens) * inputPerToken * 1.6
	wantCacheRead := float64(usage.CacheReadTokens) * inputPerToken * 0.2
	wantTotal := float64(usage.InputTokens)*inputPerToken +
		float64(usage.OutputTokens)*outputPerToken +
		wantCacheCreate +
		wantCacheRead

	if !floatEquals(result.CacheCreateCost, wantCacheCreate) {
		t.Fatalf("CacheCreateCost = %.12f, 期望 %.12f", result.CacheCreateCost, wantCacheCreate)
	}
	if !floatEquals(result.CacheReadCost, wantCacheRead) {
		t.Fatalf("CacheReadCost = %.12f, 期望 %.12f", result.CacheReadCost, wantCacheRead)
	}
	if !floatEquals(result.TotalCost, wantTotal) {
		t.Fatalf("TotalCost = %.12f, 期望 %.12f", result.TotalCost, wantTotal)
	}
}
