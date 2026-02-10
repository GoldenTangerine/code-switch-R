package services

import (
	modelpricing "codeswitch/resources/model-pricing"
)

type requestLogCostResult struct {
	TotalCost   float64
	PriceSource string
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
			if totalCost, hasPricing := calculateProviderAPICost(item, usage); hasPricing {
				return requestLogCostResult{
					TotalCost:   totalCost,
					PriceSource: requestLogPriceSourceProviderAPI,
				}
			}
		}
	}

	if pricing != nil {
		breakdown := pricing.CalculateCost(model, usage)
		if breakdown.HasPricing {
			return requestLogCostResult{
				TotalCost:   breakdown.TotalCost,
				PriceSource: requestLogPriceSourceBuiltin,
			}
		}
	}

	return requestLogCostResult{
		TotalCost:   0,
		PriceSource: requestLogPriceSourceNone,
	}
}

func calculateProviderAPICost(item ProviderModelPricingItem, usage modelpricing.UsageSnapshot) (float64, bool) {
	switch item.QuotaType {
	case 0:
		if item.InputUSDPerM < 0 || item.OutputUSDPerM < 0 {
			return 0, false
		}
		inputPerToken := item.InputUSDPerM / 1_000_000
		outputPerToken := item.OutputUSDPerM / 1_000_000

		totalCost := 0.0
		totalCost += float64(usage.InputTokens) * inputPerToken
		totalCost += float64(usage.OutputTokens) * outputPerToken
		totalCost += float64(usage.ReasoningTokens) * outputPerToken
		totalCost += float64(usage.CacheCreateTokens) * inputPerToken
		totalCost += float64(usage.CacheReadTokens) * inputPerToken
		return totalCost, true
	case 1:
		if item.PerCallPrice == nil {
			return 0, false
		}

		if item.PerCallPrice.Unified != nil {
			if *item.PerCallPrice.Unified < 0 {
				return 0, false
			}
			return *item.PerCallPrice.Unified, true
		}

		totalCost := 0.0
		hasPricing := false
		if item.PerCallPrice.Input != nil {
			if *item.PerCallPrice.Input < 0 {
				return 0, false
			}
			totalCost += *item.PerCallPrice.Input
			hasPricing = true
		}
		if item.PerCallPrice.Output != nil {
			if *item.PerCallPrice.Output < 0 {
				return 0, false
			}
			totalCost += *item.PerCallPrice.Output
			hasPricing = true
		}
		return totalCost, hasPricing
	default:
		return 0, false
	}
}
