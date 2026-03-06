package services

import (
	modelpricing "codeswitch/resources/model-pricing"
	"strings"
)

const (
	providerAPIDefaultCacheCreateMultiplier = 1.25
	providerAPIDefaultCacheReadMultiplier   = 0.1
)

const (
	providerCacheMultiplierSourceManual   = "manual"
	providerCacheMultiplierSourceProvider = "provider"
	providerCacheMultiplierSourceBuiltin  = "builtin"
	providerCacheMultiplierSourceFallback = "fallback"
)

type requestLogCostResult struct {
	InputCost                 float64
	OutputCost                float64
	ReasoningCost             float64
	CacheCreateCost           float64
	CacheReadCost             float64
	Ephemeral5mCost           float64
	Ephemeral1hCost           float64
	TotalCost                 float64
	HasPricing                bool
	MatchedPricingModel       string
	PriceSource               string
	ProviderPricingAvailable  bool
	ProviderQuotaType         int
	ProviderInputUSDPerM      float64
	ProviderOutputUSDPerM     float64
	ProviderPerCallUnified    float64
	ProviderPerCallInput      float64
	ProviderPerCallOutput     float64
	ProviderPerCallUnifiedSet bool
	ProviderPerCallInputSet   bool
	ProviderPerCallOutputSet  bool
}

const (
	requestLogPriceSourceProviderAPI = "provider_api"
	requestLogPriceSourceBuiltin     = "builtin"
	requestLogPriceSourceNone        = "none"
)

func calculateRequestLogCost(
	providerService *ProviderService,
	pricing *modelpricing.Service,
	providerAPIURL string,
	providerAPIKey string,
	providerAuthType string,
	model string,
	inputTokens int,
	outputTokens int,
	reasoningTokens int,
	cacheCreateTokens int,
	ephemeral5mTokens int,
	ephemeral1hTokens int,
	cacheReadTokens int,
) requestLogCostResult {
	usage := buildRequestLogUsageSnapshot(
		inputTokens,
		outputTokens,
		reasoningTokens,
		cacheCreateTokens,
		ephemeral5mTokens,
		ephemeral1hTokens,
		cacheReadTokens,
	)

	if providerService != nil {
		if item, ok := providerService.ResolveCachedProviderModelPricing(providerAPIURL, providerAPIKey, providerAuthType, model); ok {
			if result, hasPricing := calculateProviderAPICost(providerService, providerAPIURL, providerAPIKey, providerAuthType, item, usage, pricing, model); hasPricing {
				result.PriceSource = requestLogPriceSourceProviderAPI
				result.HasPricing = true
				return result
			}
		}
	}

	if pricing != nil {
		breakdown := pricing.CalculateCost(model, usage)
		if breakdown.HasPricing {
			matchedPricingModel := ""
			if breakdown.FuzzyMatched && breakdown.PricingModel != "" {
				matchedPricingModel = breakdown.PricingModel
			}
			return requestLogCostResult{
				InputCost:           breakdown.InputCost,
				OutputCost:          breakdown.OutputCost,
				ReasoningCost:       breakdown.ReasoningCost,
				CacheCreateCost:     breakdown.CacheCreateCost,
				CacheReadCost:       breakdown.CacheReadCost,
				Ephemeral5mCost:     breakdown.Ephemeral5mCost,
				Ephemeral1hCost:     breakdown.Ephemeral1hCost,
				TotalCost:           breakdown.TotalCost,
				HasPricing:          true,
				MatchedPricingModel: matchedPricingModel,
				PriceSource:         requestLogPriceSourceBuiltin,
				ProviderQuotaType:   -1,
			}
		}
	}

	return requestLogCostResult{
		TotalCost:         0,
		PriceSource:       requestLogPriceSourceNone,
		ProviderQuotaType: -1,
	}
}

func calculateProviderAPICost(
	providerService *ProviderService,
	providerAPIURL string,
	providerAPIKey string,
	providerAuthType string,
	item ProviderModelPricingItem,
	usage modelpricing.UsageSnapshot,
	pricing *modelpricing.Service,
	model string,
) (requestLogCostResult, bool) {
	result := requestLogCostResult{
		ProviderPricingAvailable: true,
		ProviderQuotaType:        item.QuotaType,
	}

	switch item.QuotaType {
	case 0:
		if item.InputUSDPerM < 0 || item.OutputUSDPerM < 0 {
			return requestLogCostResult{}, false
		}
		inputPerToken := item.InputUSDPerM / 1_000_000
		outputPerToken := item.OutputUSDPerM / 1_000_000
		cacheCreateMultiplier, cacheReadMultiplier := resolveProviderCacheMultipliers(
			providerService,
			providerAPIURL,
			providerAPIKey,
			providerAuthType,
			item,
			pricing,
			model,
		)
		cacheCreate5mPerToken := inputPerToken * cacheCreateMultiplier
		cacheReadPerToken := inputPerToken * cacheReadMultiplier
		cacheCreate5mRaw := 0
		cacheCreate1hRaw := 0
		if usage.CacheCreation != nil {
			cacheCreate5mRaw = usage.CacheCreation.Ephemeral5mTokens
			cacheCreate1hRaw = usage.CacheCreation.Ephemeral1hTokens
		}
		cacheCreateTokens, cacheCreate5mTokens, cacheCreate1hTokens := completeCacheCreationTokenSplit(
			usage.CacheCreateTokens,
			cacheCreate5mRaw,
			cacheCreate1hRaw,
		)

		result.ProviderInputUSDPerM = item.InputUSDPerM
		result.ProviderOutputUSDPerM = item.OutputUSDPerM
		result.InputCost = float64(usage.InputTokens) * inputPerToken
		result.OutputCost = float64(usage.OutputTokens) * outputPerToken
		result.ReasoningCost = float64(usage.ReasoningTokens) * outputPerToken
		if cacheCreateTokens > 0 {
			result.Ephemeral5mCost = float64(cacheCreate5mTokens) * cacheCreate5mPerToken
			if cacheCreate1hTokens > 0 {
				cacheCreate1hPerToken := resolveProviderEphemeral1hPerToken(pricing, model, cacheCreate5mPerToken)
				result.Ephemeral1hCost = float64(cacheCreate1hTokens) * cacheCreate1hPerToken
			}
			result.CacheCreateCost = result.Ephemeral5mCost + result.Ephemeral1hCost
		}
		result.CacheReadCost = float64(usage.CacheReadTokens) * cacheReadPerToken
		result.TotalCost = result.InputCost + result.OutputCost + result.ReasoningCost + result.CacheCreateCost + result.CacheReadCost
		return result, true
	case 1:
		if item.PerCallPrice == nil {
			return requestLogCostResult{}, false
		}

		if item.PerCallPrice.Unified != nil {
			if *item.PerCallPrice.Unified < 0 {
				return requestLogCostResult{}, false
			}
			result.ProviderPerCallUnified = *item.PerCallPrice.Unified
			result.ProviderPerCallUnifiedSet = true
			result.TotalCost = *item.PerCallPrice.Unified
			return result, true
		}

		hasPricing := false
		if item.PerCallPrice.Input != nil {
			if *item.PerCallPrice.Input < 0 {
				return requestLogCostResult{}, false
			}
			result.ProviderPerCallInput = *item.PerCallPrice.Input
			result.ProviderPerCallInputSet = true
			result.InputCost = *item.PerCallPrice.Input
			hasPricing = true
		}
		if item.PerCallPrice.Output != nil {
			if *item.PerCallPrice.Output < 0 {
				return requestLogCostResult{}, false
			}
			result.ProviderPerCallOutput = *item.PerCallPrice.Output
			result.ProviderPerCallOutputSet = true
			result.OutputCost = *item.PerCallPrice.Output
			hasPricing = true
		}
		result.TotalCost = result.InputCost + result.OutputCost
		return result, hasPricing
	default:
		return requestLogCostResult{}, false
	}
}

func resolveProviderCacheMultipliers(
	providerService *ProviderService,
	providerAPIURL string,
	providerAPIKey string,
	providerAuthType string,
	item ProviderModelPricingItem,
	pricing *modelpricing.Service,
	model string,
) (float64, float64) {
	cacheCreateMultiplier, _, cacheReadMultiplier, _ := resolveProviderCacheMultiplierDetails(
		providerService,
		providerAPIURL,
		providerAPIKey,
		providerAuthType,
		item,
		pricing,
		model,
	)
	return cacheCreateMultiplier, cacheReadMultiplier
}

func resolveProviderCacheMultiplierDetails(
	providerService *ProviderService,
	providerAPIURL string,
	providerAPIKey string,
	providerAuthType string,
	item ProviderModelPricingItem,
	pricing *modelpricing.Service,
	model string,
) (float64, string, float64, string) {
	cacheCreateMultiplier := 0.0
	cacheReadMultiplier := 0.0
	cacheCreateSource := ""
	cacheReadSource := ""

	if providerService != nil {
		if override, ok := providerService.resolveProviderModelPricingOverride(providerAPIURL, providerAPIKey, providerAuthType, model); ok {
			if override.HasCacheCreateMultiplier {
				cacheCreateMultiplier = override.CacheCreateMultiplier
				cacheCreateSource = providerCacheMultiplierSourceManual
			}
			if override.HasCacheReadMultiplier {
				cacheReadMultiplier = override.CacheReadMultiplier
				cacheReadSource = providerCacheMultiplierSourceManual
			}
		}
	}

	if cacheCreateSource == "" && item.CacheCreateMultiplier > 0 {
		cacheCreateMultiplier = item.CacheCreateMultiplier
	}
	if cacheReadSource == "" && item.CacheReadMultiplier > 0 {
		cacheReadMultiplier = item.CacheReadMultiplier
	}
	if cacheCreateSource == "" && item.CacheCreateMultiplier > 0 {
		cacheCreateSource = providerCacheMultiplierSourceProvider
	}
	if cacheReadSource == "" && item.CacheReadMultiplier > 0 {
		cacheReadSource = providerCacheMultiplierSourceProvider
	}

	if cacheCreateSource == "" || cacheReadSource == "" {
		derivedCreate, derivedRead, ok := deriveCacheMultipliersFromBuiltinPricing(pricing, model)
		if ok {
			if cacheCreateSource == "" && derivedCreate >= 0 {
				cacheCreateMultiplier = derivedCreate
				cacheCreateSource = providerCacheMultiplierSourceBuiltin
			}
			if cacheReadSource == "" && derivedRead >= 0 {
				cacheReadMultiplier = derivedRead
				cacheReadSource = providerCacheMultiplierSourceBuiltin
			}
		}
	}

	lowerModel := strings.ToLower(strings.TrimSpace(model))
	if cacheCreateSource == "" {
		if strings.Contains(lowerModel, "claude") {
			cacheCreateMultiplier = providerAPIDefaultCacheCreateMultiplier
		} else {
			cacheCreateMultiplier = 1
		}
		cacheCreateSource = providerCacheMultiplierSourceFallback
	}
	if cacheReadSource == "" {
		if strings.Contains(lowerModel, "claude") {
			cacheReadMultiplier = providerAPIDefaultCacheReadMultiplier
		} else {
			cacheReadMultiplier = 1
		}
		cacheReadSource = providerCacheMultiplierSourceFallback
	}

	return cacheCreateMultiplier, cacheCreateSource, cacheReadMultiplier, cacheReadSource
}

func buildRequestLogUsageSnapshot(
	inputTokens int,
	outputTokens int,
	reasoningTokens int,
	cacheCreateTokens int,
	ephemeral5mTokens int,
	ephemeral1hTokens int,
	cacheReadTokens int,
) modelpricing.UsageSnapshot {
	normalizedTotal, normalized5m, normalized1h := completeCacheCreationTokenSplit(
		cacheCreateTokens,
		ephemeral5mTokens,
		ephemeral1hTokens,
	)
	usage := modelpricing.UsageSnapshot{
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		ReasoningTokens:   reasoningTokens,
		CacheCreateTokens: normalizedTotal,
		CacheReadTokens:   cacheReadTokens,
	}
	if normalized5m > 0 || normalized1h > 0 {
		usage.CacheCreation = &modelpricing.CacheCreationDetail{
			Ephemeral5mTokens: normalized5m,
			Ephemeral1hTokens: normalized1h,
		}
	}
	return usage
}

func normalizeCacheCreationTokenSplit(totalTokens int, ephemeral5mTokens int, ephemeral1hTokens int) (int, int, int) {
	total := totalTokens
	five := ephemeral5mTokens
	one := ephemeral1hTokens

	if total < 0 {
		total = 0
	}
	if five < 0 {
		five = 0
	}
	if one < 0 {
		one = 0
	}

	if total == 0 && (five > 0 || one > 0) {
		total = five + one
	}
	if total <= 0 {
		return 0, 0, 0
	}

	if one > total {
		one = total
	}
	remaining := total - one
	if remaining < 0 {
		remaining = 0
	}
	if five > remaining {
		five = remaining
	}

	return total, five, one
}

func completeCacheCreationTokenSplit(totalTokens int, ephemeral5mTokens int, ephemeral1hTokens int) (int, int, int) {
	total, five, one := normalizeCacheCreationTokenSplit(totalTokens, ephemeral5mTokens, ephemeral1hTokens)
	if total <= 0 {
		return 0, 0, 0
	}

	assigned := five + one
	if assigned < total {
		five += total - assigned
	}

	return total, five, one
}

func resolveProviderEphemeral1hPerToken(pricing *modelpricing.Service, model string, fallbackPerToken float64) float64 {
	if pricing != nil && strings.TrimSpace(model) != "" {
		breakdown := pricing.CalculateCost(model, modelpricing.UsageSnapshot{
			CacheCreateTokens: 1,
			CacheCreation: &modelpricing.CacheCreationDetail{
				Ephemeral1hTokens: 1,
			},
		})
		if breakdown.Ephemeral1hCost > 0 {
			return breakdown.Ephemeral1hCost
		}
	}
	return fallbackPerToken
}

func deriveCacheMultipliersFromBuiltinPricing(pricing *modelpricing.Service, model string) (float64, float64, bool) {
	if pricing == nil || strings.TrimSpace(model) == "" {
		return 0, 0, false
	}

	inputBreakdown := pricing.CalculateCost(model, modelpricing.UsageSnapshot{InputTokens: 1})
	if !inputBreakdown.HasPricing || inputBreakdown.InputCost <= 0 {
		return 0, 0, false
	}

	cacheCreateMultiplier := 0.0
	cacheCreateBreakdown := pricing.CalculateCost(model, modelpricing.UsageSnapshot{CacheCreateTokens: 1})
	if cacheCreateBreakdown.CacheCreateCost > 0 {
		cacheCreateMultiplier = cacheCreateBreakdown.CacheCreateCost / inputBreakdown.InputCost
	}

	cacheReadMultiplier := 0.0
	cacheReadBreakdown := pricing.CalculateCost(model, modelpricing.UsageSnapshot{CacheReadTokens: 1})
	if cacheReadBreakdown.CacheReadCost > 0 {
		cacheReadMultiplier = cacheReadBreakdown.CacheReadCost / inputBreakdown.InputCost
	}

	if cacheCreateMultiplier <= 0 && cacheReadMultiplier <= 0 {
		return 0, 0, false
	}

	return cacheCreateMultiplier, cacheReadMultiplier, true
}
