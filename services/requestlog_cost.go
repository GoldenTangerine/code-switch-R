package services

import (
	modelpricing "codeswitch/resources/model-pricing"
)

func calculateRequestLogTotalCost(model string, inputTokens int, outputTokens int, reasoningTokens int, cacheCreateTokens int, cacheReadTokens int) float64 {
	pricing, err := modelpricing.DefaultService()
	if err != nil || pricing == nil {
		return 0
	}
	usage := modelpricing.UsageSnapshot{
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		ReasoningTokens:   reasoningTokens,
		CacheCreateTokens: cacheCreateTokens,
		CacheReadTokens:   cacheReadTokens,
	}
	return pricing.CalculateCost(model, usage).TotalCost
}
