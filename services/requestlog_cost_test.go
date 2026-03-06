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

	result, ok := calculateProviderAPICost(nil, "", "", "", item, usage, nil, "claude-opus-4-5")
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

	result, ok := calculateProviderAPICost(nil, "", "", "", item, usage, nil, "any-model")
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

func TestCalculateProviderAPICost_SplitsClaudeCacheCreateBy5mAnd1h(t *testing.T) {
	item := ProviderModelPricingItem{
		QuotaType:     0,
		InputUSDPerM:  5,
		OutputUSDPerM: 25,
	}
	usage := modelpricing.UsageSnapshot{
		CacheCreateTokens: 30,
		CacheCreation: &modelpricing.CacheCreationDetail{
			Ephemeral5mTokens: 10,
			Ephemeral1hTokens: 20,
		},
	}
	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}

	result, ok := calculateProviderAPICost(nil, "", "", "", item, usage, pricing, "claude-opus-4-5")
	if !ok {
		t.Fatalf("calculateProviderAPICost 返回 hasPricing=false")
	}

	want5mCost := 10 * (5.0 / 1_000_000.0) * 1.25
	want1hCost := 20 * 0.00001
	wantCacheCreate := want5mCost + want1hCost

	if !floatEquals(result.Ephemeral5mCost, want5mCost) {
		t.Fatalf("Ephemeral5mCost = %.12f, 期望 %.12f", result.Ephemeral5mCost, want5mCost)
	}
	if !floatEquals(result.Ephemeral1hCost, want1hCost) {
		t.Fatalf("Ephemeral1hCost = %.12f, 期望 %.12f", result.Ephemeral1hCost, want1hCost)
	}
	if !floatEquals(result.CacheCreateCost, wantCacheCreate) {
		t.Fatalf("CacheCreateCost = %.12f, 期望 %.12f", result.CacheCreateCost, wantCacheCreate)
	}
}

func TestCalculateProviderAPICost_FillsMissing5mFromTotalWhenOnly1hProvided(t *testing.T) {
	item := ProviderModelPricingItem{
		QuotaType:     0,
		InputUSDPerM:  5,
		OutputUSDPerM: 25,
	}
	usage := modelpricing.UsageSnapshot{
		CacheCreateTokens: 30,
		CacheCreation: &modelpricing.CacheCreationDetail{
			Ephemeral5mTokens: 0,
			Ephemeral1hTokens: 20,
		},
	}
	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}

	result, ok := calculateProviderAPICost(nil, "", "", "", item, usage, pricing, "claude-opus-4-5")
	if !ok {
		t.Fatalf("calculateProviderAPICost 返回 hasPricing=false")
	}

	want5mCost := 10 * (5.0 / 1_000_000.0) * 1.25
	want1hCost := 20 * 0.00001
	wantCacheCreate := want5mCost + want1hCost

	if !floatEquals(result.Ephemeral5mCost, want5mCost) {
		t.Fatalf("Ephemeral5mCost = %.12f, 期望 %.12f", result.Ephemeral5mCost, want5mCost)
	}
	if !floatEquals(result.Ephemeral1hCost, want1hCost) {
		t.Fatalf("Ephemeral1hCost = %.12f, 期望 %.12f", result.Ephemeral1hCost, want1hCost)
	}
	if !floatEquals(result.CacheCreateCost, wantCacheCreate) {
		t.Fatalf("CacheCreateCost = %.12f, 期望 %.12f", result.CacheCreateCost, wantCacheCreate)
	}
}

func TestBuildRequestLogUsageSnapshotFills5mRemainder(t *testing.T) {
	usage := buildRequestLogUsageSnapshot(0, 0, 0, 50, 10, 15, 0)

	if usage.CacheCreation == nil {
		t.Fatalf("CacheCreation = nil, 期望存在")
	}
	if usage.CacheCreation.Ephemeral5mTokens != 35 {
		t.Fatalf("Ephemeral5mTokens = %d, 期望 35（自动补齐剩余 token）", usage.CacheCreation.Ephemeral5mTokens)
	}
	if usage.CacheCreation.Ephemeral1hTokens != 15 {
		t.Fatalf("Ephemeral1hTokens = %d, 期望 15", usage.CacheCreation.Ephemeral1hTokens)
	}
}

func TestNormalizeCacheCreationTokenSplitDoesNotExpandTotal(t *testing.T) {
	total, five, one := normalizeCacheCreationTokenSplit(30, 30, 20)

	if total != 30 {
		t.Fatalf("total = %d, 期望 30（不应被明细放大）", total)
	}
	if five != 10 {
		t.Fatalf("five = %d, 期望 10（裁剪为 total-one）", five)
	}
	if one != 20 {
		t.Fatalf("one = %d, 期望 20", one)
	}
}

func TestResolveProviderCacheMultiplierDetails_ManualZeroOverrideSkipsFallback(t *testing.T) {
	providerService := &ProviderService{
		providerPricingOverrides: newProviderPricingOverrideStore(),
	}
	scopeKey := providerPricingOverrideScopeKey("https://example.com/v1", "secret-key", "")
	providerService.providerPricingOverrides.Providers[scopeKey] = map[string]providerPricingOverrideItem{
		normalizeProviderPricingModelName("gpt-4.1"): {
			CacheCreateMultiplier:    0,
			HasCacheCreateMultiplier: true,
			CacheReadMultiplier:      0,
			HasCacheReadMultiplier:   true,
		},
	}

	create, createSource, read, readSource := resolveProviderCacheMultiplierDetails(
		providerService,
		"https://example.com/v1",
		"secret-key",
		"",
		ProviderModelPricingItem{Model: "gpt-4.1", QuotaType: 0},
		nil,
		"gpt-4.1",
	)

	if !floatEquals(create, 0) {
		t.Fatalf("create multiplier = %.12f, 期望 0", create)
	}
	if !floatEquals(read, 0) {
		t.Fatalf("read multiplier = %.12f, 期望 0", read)
	}
	if createSource != providerCacheMultiplierSourceManual {
		t.Fatalf("create source = %q, 期望 %q", createSource, providerCacheMultiplierSourceManual)
	}
	if readSource != providerCacheMultiplierSourceManual {
		t.Fatalf("read source = %q, 期望 %q", readSource, providerCacheMultiplierSourceManual)
	}
}

func TestResolveProviderCacheMultiplierDetails_PartialManualOverrideKeepsProviderValue(t *testing.T) {
	providerService := &ProviderService{
		providerPricingOverrides: newProviderPricingOverrideStore(),
	}
	scopeKey := providerPricingOverrideScopeKey("https://example.com/v1", "secret-key", "")
	providerService.providerPricingOverrides.Providers[scopeKey] = map[string]providerPricingOverrideItem{
		normalizeProviderPricingModelName("custom-model"): {
			CacheCreateMultiplier:    2.5,
			HasCacheCreateMultiplier: true,
		},
	}

	create, createSource, read, readSource := resolveProviderCacheMultiplierDetails(
		providerService,
		"https://example.com/v1",
		"secret-key",
		"",
		ProviderModelPricingItem{
			Model:               "custom-model",
			QuotaType:           0,
			CacheReadMultiplier: 0.2,
		},
		nil,
		"custom-model",
	)

	if !floatEquals(create, 2.5) {
		t.Fatalf("create multiplier = %.12f, 期望 2.5", create)
	}
	if !floatEquals(read, 0.2) {
		t.Fatalf("read multiplier = %.12f, 期望 0.2", read)
	}
	if createSource != providerCacheMultiplierSourceManual {
		t.Fatalf("create source = %q, 期望 %q", createSource, providerCacheMultiplierSourceManual)
	}
	if readSource != providerCacheMultiplierSourceProvider {
		t.Fatalf("read source = %q, 期望 %q", readSource, providerCacheMultiplierSourceProvider)
	}
}
