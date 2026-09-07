/*
@name: 分项报价回归测试
@Descripttion: 验证供应商缺价补齐、免费报价和云端轨道隔离。
@version: 1.0.0
@Author: sm
@Date: 2026-09-07 11:21:47
@LastEditTime: 2026-09-07 11:21:47
@FilePath: services/requestlog_pricing_fields_test.go
*/
package services

import (
	"testing"

	modelpricing "codeswitch/resources/model-pricing"
)

func fieldwiseTestPricing(t *testing.T) *modelpricing.Service {
	t.Helper()
	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatal(err)
	}
	pricing.ApplyOverrides(map[string]modelpricing.PricingEntry{
		"test-fieldwise-model": {CloudPricing: &modelpricing.CloudPricingRules{
			Charges: map[string]float64{"prompt": 3e-6, "completion": 10e-6, "cache_read": 0.3e-6, "cache_write": 4e-6, "cache_write_1h": 6e-6},
			Tracks: []modelpricing.PricingTrack{
				{Label: "Priority", Factor: 2, Triggers: []modelpricing.PricingTrigger{{Kind: "body_matches", Field: "service_tier", Pattern: "^priority$"}}},
				{Label: "Base", Factor: 1},
			},
		}},
	}, nil)
	return pricing
}

func TestFieldwiseProviderCostKeepsProviderRateAndUsesCloudTrackForMissingFields(t *testing.T) {
	usage := modelpricing.UsageSnapshot{InputTokens: 100, OutputTokens: 50, CacheReadTokens: 20, CacheCreateTokens: 12,
		CacheCreation: &modelpricing.CacheCreationDetail{Ephemeral5mTokens: 7, Ephemeral1hTokens: 5},
		Context:       &modelpricing.PricingContext{ConditionsKnown: true, ServiceTier: "priority"}}
	item := ProviderModelPricingItem{PriceFieldsKnown: true, HasInputPrice: true, InputUSDPerM: 5, QuotaType: 0}
	result, ok := calculateProviderAPICost(nil, "", "", "", item, usage, fieldwiseTestPricing(t), "test-fieldwise-model")
	if !ok || result.PriceSource != "mixed" || !result.PricingSnapshot.Complete {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !floatEquals(result.InputCost, 100*5e-6) || !floatEquals(result.OutputCost, 50*20e-6) || !floatEquals(result.CacheReadCost, 20*0.6e-6) || !floatEquals(result.CacheCreateCost, 7*8e-6+5*12e-6) {
		t.Fatalf("incorrect split: %+v", result)
	}
	if result.PricingSnapshot.TrackLabel != "Priority" || result.PricingSnapshot.FieldSources["prompt"] != "provider_api" || result.PricingSnapshot.FieldSources["completion"] != "cloud" {
		t.Fatalf("incorrect snapshot: %+v", result.PricingSnapshot)
	}
}

func TestFieldwiseProviderCostZeroIsExplicitAndMissingRemainsUnknown(t *testing.T) {
	item := ProviderModelPricingItem{PriceFieldsKnown: true, HasInputPrice: true, InputUSDPerM: 0, QuotaType: 0}
	usage := modelpricing.UsageSnapshot{InputTokens: 100, OutputTokens: 50}
	result, ok := calculateProviderAPICost(nil, "", "", "", item, usage, nil, "unknown")
	if !ok || result.TotalCost != 0 || result.PricingSnapshot.Complete {
		t.Fatalf("missing output must remain unknown: %+v", result)
	}
	if price, exists := result.PricingSnapshot.UnitPrices["prompt"]; !exists || price != 0 {
		t.Fatal("explicit zero input was lost")
	}
	if _, exists := result.PricingSnapshot.UnitPrices["completion"]; exists {
		t.Fatal("missing output was converted to zero")
	}
	item.HasOutputPrice = true
	result, ok = calculateProviderAPICost(nil, "", "", "", item, usage, nil, "unknown")
	if !ok || result.TotalCost != 0 || !result.PricingSnapshot.Complete {
		t.Fatalf("explicit free model: %+v", result)
	}
}

func TestFieldwiseProviderCostNameOnlyDoesNotClaimPricing(t *testing.T) {
	item := ProviderModelPricingItem{Model: "test-fieldwise-model", PriceFieldsKnown: true, QuotaType: 0}
	_, ok := calculateProviderAPICost(nil, "", "", "", item, modelpricing.UsageSnapshot{InputTokens: 100}, fieldwiseTestPricing(t), item.Model)
	if ok {
		t.Fatal("name-only response must fall through to project pricing")
	}
}

func TestFieldwiseProviderCostExplicitZeroCacheMultiplier(t *testing.T) {
	item := ProviderModelPricingItem{PriceFieldsKnown: true, HasInputPrice: true, HasOutputPrice: true, HasCacheReadPrice: true, HasCacheCreatePrice: true, InputUSDPerM: 5, OutputUSDPerM: 10, QuotaType: 0}
	create, createSource, read, readSource := resolveProviderCacheMultiplierDetails(nil, "", "", "", item, fieldwiseTestPricing(t), "test-fieldwise-model")
	if create != 0 || read != 0 || createSource != providerCacheMultiplierSourceProvider || readSource != providerCacheMultiplierSourceProvider {
		t.Fatalf("zero cache quote must remain zero in UI enrichment: %v %s %v %s", create, createSource, read, readSource)
	}
	result, ok := calculateProviderAPICost(nil, "", "", "", item, modelpricing.UsageSnapshot{CacheCreateTokens: 10, CacheReadTokens: 10}, fieldwiseTestPricing(t), "test-fieldwise-model")
	if !ok || result.CacheCreateCost != 0 || result.CacheReadCost != 0 || !result.PricingSnapshot.Complete || result.PricingSnapshot.TrackLabel != "" {
		t.Fatalf("provider cache zero must win: %+v", result)
	}
}

func TestCalculateRequestLogCostPreservesCloudSnapshot(t *testing.T) {
	result := calculateRequestLogCost(nil, fieldwiseTestPricing(t), "", "", "", "test-fieldwise-model", "test-fieldwise-model", "test-fieldwise-model", 100, 10, 0, 0, 0, 0, 0, &modelpricing.PricingContext{ConditionsKnown: true, ServiceTier: "priority"})
	if result.PricingSnapshot == nil || result.PricingSnapshot.TrackLabel != "Priority" || !floatEquals(result.TotalCost, 100*6e-6+10*20e-6) {
		t.Fatalf("cloud context not applied: %+v", result)
	}
}

func TestFieldwiseProviderCostUsesRequestedModelForMissingResponseModelPrice(t *testing.T) {
	item := ProviderModelPricingItem{PriceFieldsKnown: true, HasInputPrice: true, InputUSDPerM: 5, QuotaType: 0}
	usage := modelpricing.UsageSnapshot{InputTokens: 100, OutputTokens: 10}
	result, ok := calculateProviderAPICost(nil, "", "", "", item, usage, fieldwiseTestPricing(t), "unrelated-private-response", "test-fieldwise-model")
	if !ok || !floatEquals(result.OutputCost, 10*10e-6) || result.PriceSource != "mixed" || !result.PricingSnapshot.Complete {
		t.Fatalf("requested model should provide missing output: %+v", result)
	}
}

func TestFieldwiseProviderCostDoesNotInventMissingOneHourPrice(t *testing.T) {
	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatal(err)
	}
	pricing.ApplyOverrides(map[string]modelpricing.PricingEntry{"private-output-only": {OutputCostPerToken: 1e-6, HasOutputCostPerToken: true}}, nil)
	item := ProviderModelPricingItem{PriceFieldsKnown: true, HasInputPrice: true, InputUSDPerM: 5, QuotaType: 0}
	usage := modelpricing.UsageSnapshot{InputTokens: 10, CacheCreateTokens: 10, CacheCreation: &modelpricing.CacheCreationDetail{Ephemeral1hTokens: 10}}
	result, ok := calculateProviderAPICost(nil, "", "", "", item, usage, pricing, "private-output-only")
	if !ok || result.PricingSnapshot.Complete {
		t.Fatalf("missing 1h price must be incomplete: %+v", result)
	}
	if _, exists := result.PricingSnapshot.UnitPrices["cache_write_1h"]; exists {
		t.Fatal("missing 1h price must not be stored as zero")
	}
}

func TestRequestLogReasoningProtocolCosts(t *testing.T) {
	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatal(err)
	}
	pricing.ApplyOverrides(map[string]modelpricing.PricingEntry{"review-thinking-model": {CloudPricing: &modelpricing.CloudPricingRules{Charges: map[string]float64{"prompt": 1.25e-6, "completion": 10e-6}}}}, nil)
	for _, stream := range []bool{false, true} {
		for _, gemini := range []bool{false, true} {
			log := &ReqeustLog{IsStream: stream}
			if gemini {
				captureRequestLogPricingContext(log, []byte(`{}`), nil, "/v1beta/models/review-thinking-model:generateContent")
				GeminiParseTokenUsageFromResponse(`{"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":10,"thoughtsTokenCount":100,"totalTokenCount":210}}`, log)
			} else {
				captureRequestLogPricingContext(log, []byte(`{}`), nil, "/v1/responses")
				CodexParseTokenUsageFromResponse(`{"response":{"usage":{"input_tokens":100,"output_tokens":110,"output_tokens_details":{"reasoning_tokens":100}}}}`, log)
			}
			result := calculateRequestLogCost(nil, pricing, "", "", "", "review-thinking-model", "review-thinking-model", "review-thinking-model", log.InputTokens, log.OutputTokens, log.ReasoningTokens, 0, 0, 0, 0, log.PricingContext)
			if !floatEquals(result.TotalCost, .001225) || !result.PricingSnapshot.Complete {
				t.Fatalf("gemini=%v stream=%v cost=%+v", gemini, stream, result)
			}
			usage := modelpricing.UsageSnapshot{InputTokens: log.InputTokens, OutputTokens: log.OutputTokens, ReasoningTokens: log.ReasoningTokens, Context: log.PricingContext}
			for _, outputPresent := range []bool{false, true} {
				item := ProviderModelPricingItem{PriceFieldsKnown: true, HasInputPrice: true, InputUSDPerM: 2, HasOutputPrice: outputPresent, OutputUSDPerM: 10}
				mixed, ok := calculateProviderAPICost(nil, "", "", "", item, usage, pricing, "review-thinking-model")
				if !ok || !floatEquals(mixed.TotalCost, .0013) {
					t.Fatalf("gemini=%v providerOutput=%v mixed=%+v", gemini, outputPresent, mixed)
				}
			}
		}
	}
	log := &ReqeustLog{}
	parseGeminiUsageMetadata([]byte(`{"usageMetadata":{"promptTokenCount":100,"thoughtsTokenCount":100,"totalTokenCount":210}}`), log)
	if log.OutputTokens != 10 || log.ReasoningTokens != 100 {
		t.Fatalf("fallback output double counted thoughts: %+v", log)
	}
}
