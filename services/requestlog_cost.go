package services

import (
	modelpricing "codeswitch/resources/model-pricing"
)

func calculateRequestLogTotalCost(pricing *modelpricing.Service, model string, inputTokens int, outputTokens int, reasoningTokens int, cacheCreateTokens int, cacheReadTokens int) float64 {
	if pricing == nil {
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
