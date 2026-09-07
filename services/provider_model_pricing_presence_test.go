/*
@name: 供应商价格存在性测试
@Descripttion: 验证缺失、免费和部分供应商报价解析。
@version: 1.0.0
@Author: sm
@Date: 2026-09-07 11:20:00
@LastEditTime: 2026-09-07 11:20:00
@FilePath: services/provider_model_pricing_presence_test.go
*/
package services

import (
	"encoding/json"
	"testing"
)

func TestProviderPricingFieldPresence(t *testing.T) {
	cases := []struct {
		name, raw                   string
		input, output, create, read bool
		inputPrice, outputPrice     float64
	}{
		{"name only", `{"model_name":"m"}`, false, false, false, false, 0, 0},
		{"explicit free", `{"model_name":"m","model_ratio":0,"completion_ratio":0,"cache_creation_ratio":0,"cache_read_ratio":0}`, true, true, true, true, 0, 0},
		{"input only", `{"model_name":"m","model_ratio":2}`, true, false, false, false, 4, 0},
		{"completion multiplier only", `{"model_name":"m","completion_ratio":3}`, false, false, false, false, 0, 0},
		{"complete", `{"model_name":"m","model_ratio":2,"completion_ratio":3,"cache_creation_ratio":1.25,"cache_read_ratio":0.1}`, true, true, true, true, 4, 12},
		{"null", `{"model_name":"m","model_ratio":null,"completion_ratio":null}`, false, false, false, false, 0, 0},
		{"negative", `{"model_name":"m","model_ratio":-1,"completion_ratio":3}`, false, false, false, false, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var model providerModelPricing
			if err := json.Unmarshal([]byte(c.raw), &model); err != nil {
				t.Fatal(err)
			}
			result := buildProviderModelPricingResponse(SiteTypeNewAPI, "api/pricing", &providerPricingResponse{Data: []providerModelPricing{model}}).Models[0]
			if !result.PriceFieldsKnown || result.HasInputPrice != c.input || result.HasOutputPrice != c.output || result.HasCacheCreatePrice != c.create || result.HasCacheReadPrice != c.read || result.InputUSDPerM != c.inputPrice || result.OutputUSDPerM != c.outputPrice {
				t.Fatalf("wrong presence or price: %+v", result)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var restored ProviderModelPricingItem
			if err := json.Unmarshal(encoded, &restored); err != nil {
				t.Fatal(err)
			}
			if restored.HasInputPrice != result.HasInputPrice || restored.HasOutputPrice != result.HasOutputPrice || !restored.PriceFieldsKnown {
				t.Fatal("round trip lost field presence")
			}
		})
	}
}

func TestOneHubPricingUsesIndependentPrices(t *testing.T) {
	var models oneHubModelPricing
	if err := json.Unmarshal([]byte(`{"priced":{"price":{"type":"tokens","input":3,"output":15}},"free-input":{"price":{"type":"tokens","input":0,"output":8}},"output-only":{"price":{"type":"tokens","output":7}},"missing":{"price":{"type":"tokens"}}}`), &models); err != nil {
		t.Fatal(err)
	}
	response := buildProviderModelPricingResponse(SiteTypeOneHub, "one-hub", transformOneHubPricing(models, oneHubUserGroupMap{"default": {Ratio: 2}}))
	byName := map[string]ProviderModelPricingItem{}
	for _, item := range response.Models {
		byName[item.Model] = item
	}
	if item := byName["priced"]; item.InputUSDPerM != 6 || item.OutputUSDPerM != 30 || !item.HasInputPrice || !item.HasOutputPrice {
		t.Fatalf("wrong onehub values: %+v", item)
	}
	if item := byName["free-input"]; item.InputUSDPerM != 0 || item.OutputUSDPerM != 16 || !item.HasInputPrice || !item.HasOutputPrice {
		t.Fatalf("free input lost output: %+v", item)
	}
	if item := byName["output-only"]; item.HasInputPrice || !item.HasOutputPrice || item.OutputUSDPerM != 14 {
		t.Fatalf("partial price lost: %+v", item)
	}
	if item := byName["missing"]; item.HasInputPrice || item.HasOutputPrice {
		t.Fatalf("missing became free: %+v", item)
	}
}

func TestPerCallPricePreservesMissingAndZero(t *testing.T) {
	if got := parsePerCallPrice(json.RawMessage(`{}`), 1); got != nil {
		t.Fatalf("empty price became free: %+v", got)
	}
	got := parsePerCallPrice(json.RawMessage(`{"input":0}`), 1)
	if got == nil || got.Input == nil || *got.Input != 0 || got.Output != nil {
		t.Fatalf("partial/free call price lost: %+v", got)
	}
}
