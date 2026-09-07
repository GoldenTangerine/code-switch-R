/*
@name: 请求分项报价
@Descripttion: 按字段补齐供应商缺失价格并记录实际计费快照。
@version: 1.0.0
@Author: sm
@Date: 2026-09-07 11:21:47
@LastEditTime: 2026-09-07 11:21:47
@FilePath: services/requestlog_pricing_fields.go
*/
package services

import (
	"math"

	modelpricing "codeswitch/resources/model-pricing"
)

func requestLogLocalPricingSnapshot(pricing *modelpricing.Service, model string, usage modelpricing.UsageSnapshot, cost modelpricing.CostBreakdown) *modelpricing.PricingSnapshot {
	if cost.PricingSnapshot != nil {
		return cost.PricingSnapshot
	}
	snapshot := &modelpricing.PricingSnapshot{UnitPrices: map[string]float64{}, FieldSources: map[string]string{}, Complete: true}
	if pricing == nil || !cost.HasPricing {
		snapshot.Complete = false
		return snapshot
	}
	entry, ok := pricing.PricingEntryExact(cost.PricingModel)
	if !ok {
		snapshot.Complete = false
		return snapshot
	}
	five, one := requestLogCacheUsage(usage)
	for _, field := range []struct {
		key     string
		price   float64
		present bool
		tokens  int
		cost    float64
	}{
		{"prompt", entry.InputCostPerToken, entry.HasInputCostPerToken, usage.InputTokens, cost.InputCost},
		{"completion", entry.OutputCostPerToken, entry.HasOutputCostPerToken, usage.BillableOutputTokens(), cost.OutputCost},
		{"reasoning", entry.OutputCostPerReasoningToken, entry.HasOutputCostPerReasoningToken, usage.ReasoningTokens, cost.ReasoningCost},
		{"cache_write", entry.CacheCreationInputTokenCost, entry.HasCacheCreationInputTokenCost, five, cost.Ephemeral5mCost},
		{"cache_read", entry.CacheReadInputTokenCost, entry.HasCacheReadInputTokenCost, usage.CacheReadTokens, cost.CacheReadCost},
	} {
		if !field.present && field.price == 0 && field.cost == 0 {
			if field.tokens > 0 && field.key != "reasoning" {
				snapshot.Complete = false
			}
			continue
		}
		price := field.price * cost.GroupMultiplier
		if field.tokens > 0 {
			price = field.cost / float64(field.tokens)
		}
		snapshot.UnitPrices[field.key] = price
		snapshot.FieldSources[field.key] = "builtin"
	}
	if price, exists := pricing.ExplicitEphemeral1hCostPerToken(cost.PricingModel); exists || cost.Ephemeral1hCost > 0 {
		if one > 0 {
			price = cost.Ephemeral1hCost / float64(one)
		} else {
			price *= cost.GroupMultiplier
		}
		snapshot.UnitPrices["cache_write_1h"] = price
		snapshot.FieldSources["cache_write_1h"] = "builtin"
	} else if one > 0 {
		snapshot.Complete = false
	}
	return snapshot
}

func requestLogCacheUsage(usage modelpricing.UsageSnapshot) (int, int) {
	five, one := 0, 0
	if usage.CacheCreation != nil {
		five, one = usage.CacheCreation.Ephemeral5mTokens, usage.CacheCreation.Ephemeral1hTokens
	}
	_, five, one = completeCacheCreationTokenSplit(usage.CacheCreateTokens, five, one)
	return five, one
}

func calculateFieldwiseProviderCost(ps *ProviderService, apiURL, apiKey, authType string, item ProviderModelPricingItem, usage modelpricing.UsageSnapshot, pricing *modelpricing.Service, model string, localModels ...string) (requestLogCostResult, bool) {
	local := modelpricing.CostBreakdown{}
	if pricing != nil {
		for _, candidate := range buildRequestLogPricingModelCandidates(append([]string{model}, localModels...)...) {
			local = pricing.CalculateCost(candidate, usage)
			if local.HasPricing {
				break
			}
		}
	}
	snapshot := requestLogLocalPricingSnapshot(pricing, model, usage, local)
	multiplier, _ := resolveProviderGroupMultiplierDetails(ps, apiURL, apiKey, authType, model)
	providerFields := 0
	set := func(key string, price float64, present bool) {
		if !present || price < 0 || math.IsNaN(price) || math.IsInf(price, 0) {
			return
		}
		snapshot.UnitPrices[key] = price * multiplier
		snapshot.FieldSources[key] = requestLogPriceSourceProviderAPI
		providerFields++
	}
	set("prompt", item.InputUSDPerM/1_000_000, item.HasInputPrice)
	set("completion", item.OutputUSDPerM/1_000_000, item.HasOutputPrice)
	set("reasoning", item.OutputUSDPerM/1_000_000, item.HasOutputPrice)
	createFactor, createSource, readFactor, readSource := resolveProviderCacheMultiplierDetails(ps, apiURL, apiKey, authType, item, pricing, model)
	if item.HasCacheCreatePrice && createSource != providerCacheMultiplierSourceManual {
		createFactor = item.CacheCreateMultiplier
	}
	if item.HasCacheReadPrice && readSource != providerCacheMultiplierSourceManual {
		readFactor = item.CacheReadMultiplier
	}
	set("cache_write", item.InputUSDPerM/1_000_000*createFactor, item.HasInputPrice && (item.HasCacheCreatePrice || createSource == providerCacheMultiplierSourceManual))
	set("cache_read", item.InputUSDPerM/1_000_000*readFactor, item.HasInputPrice && (item.HasCacheReadPrice || readSource == providerCacheMultiplierSourceManual))
	set("cache_write_1h", item.CacheCreate1hUSDPerM/1_000_000, item.HasCacheCreate1hPrice)
	if providerFields == 0 {
		return requestLogCostResult{}, false
	}

	snapshot.Complete = true
	usedProvider, usedProject, usedCloud := false, false, false
	charge := func(key string, tokens int) float64 {
		price, exists := snapshot.UnitPrices[key]
		if tokens > 0 {
			if !exists {
				snapshot.Complete = false
			}
			if exists {
				if snapshot.FieldSources[key] == requestLogPriceSourceProviderAPI {
					usedProvider = true
				} else {
					usedProject = true
				}
				if snapshot.FieldSources[key] == "cloud" {
					usedCloud = true
				}
			}
		}
		return float64(tokens) * price
	}
	five, one := requestLogCacheUsage(usage)
	result := requestLogCostResult{
		PricingSnapshot: snapshot, HasPricing: true, ProviderPricingAvailable: true,
		ProviderQuotaType: 0, ProviderInputUSDPerM: item.InputUSDPerM, ProviderOutputUSDPerM: item.OutputUSDPerM,
		GroupMultiplier: multiplier,
	}
	result.InputCost = charge("prompt", usage.InputTokens)
	result.OutputCost = charge("completion", usage.BillableOutputTokens())
	if _, exists := snapshot.UnitPrices["reasoning"]; exists {
		result.ReasoningCost = charge("reasoning", usage.ReasoningTokens)
	}
	result.Ephemeral5mCost = charge("cache_write", five)
	result.Ephemeral1hCost = charge("cache_write_1h", one)
	result.CacheCreateCost = result.Ephemeral5mCost + result.Ephemeral1hCost
	result.CacheReadCost = charge("cache_read", usage.CacheReadTokens)
	result.TotalCost = result.InputCost + result.OutputCost + result.ReasoningCost + result.CacheCreateCost + result.CacheReadCost
	result.PriceSource = requestLogPriceSourceProviderAPI
	if usedProject {
		result.MatchedPricingModel = local.PricingModel
		result.PriceSource = requestLogPriceSourceBuiltin
		if usedProvider {
			result.PriceSource = requestLogPriceSourceMixed
		}
	}
	if !usedCloud {
		snapshot.TrackLabel = ""
	}
	return result, true
}
