package services

import (
	modelpricing "codeswitch/resources/model-pricing"
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
	cacheReadTokens int,
) requestLogCostResult {
	usage := modelpricing.UsageSnapshot{
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		ReasoningTokens:   reasoningTokens,
		CacheCreateTokens: cacheCreateTokens,
		CacheReadTokens:   cacheReadTokens,
	}

	if providerService != nil {
		if item, ok := providerService.ResolveCachedProviderModelPricing(providerAPIURL, providerAPIKey, providerAuthType, model); ok {
			if result, hasPricing := calculateProviderAPICost(item, usage); hasPricing {
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

func calculateProviderAPICost(item ProviderModelPricingItem, usage modelpricing.UsageSnapshot) (requestLogCostResult, bool) {
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

		result.ProviderInputUSDPerM = item.InputUSDPerM
		result.ProviderOutputUSDPerM = item.OutputUSDPerM
		result.InputCost = float64(usage.InputTokens) * inputPerToken
		result.OutputCost = float64(usage.OutputTokens) * outputPerToken
		result.ReasoningCost = float64(usage.ReasoningTokens) * outputPerToken
		result.CacheCreateCost = float64(usage.CacheCreateTokens) * inputPerToken
		result.CacheReadCost = float64(usage.CacheReadTokens) * inputPerToken
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
