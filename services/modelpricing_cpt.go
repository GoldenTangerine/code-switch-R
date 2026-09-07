/**
 * @name: CPT 价格表解析
 * @Descripttion: 将 CPT v1 JSON 报价转换为本地 token 价格及有序条件轨道。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-07 11:10:24
 * @LastEditTime: 2026-09-07 11:10:24
 * @FilePath: services/modelpricing_cpt.go
 */
package services

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"regexp"
	"strconv"
	"strings"

	modelpricing "codeswitch/resources/model-pricing"
)

type cptModel struct {
	Slug            string            `json:"slug"`
	ModelName       string            `json:"model_name"`
	Vendor          string            `json:"vendor"`
	DisplayName     string            `json:"display_name"`
	ModelType       string            `json:"model_type"`
	Aliases         []string          `json:"aliases"`
	Pricing         []json.RawMessage `json:"pricing"`
	MaxInputTokens  *int64            `json:"max_input_tokens"`
	MaxOutputTokens *int64            `json:"max_output_tokens"`
	Capabilities    map[string]*bool  `json:"capabilities"`
}

type cptVariant struct {
	Provider string                     `json:"provider"`
	Official bool                       `json:"official"`
	Charges  map[string]json.RawMessage `json:"charges"`
	Tracks   []json.RawMessage          `json:"tracks"`
}

var cptDecimalPattern = regexp.MustCompile(`^[+]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

func parseCPTDecimal(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if !cptDecimalPattern.MatchString(value) {
		return 0, false
	}
	number, err := strconv.ParseFloat(value, 64)
	return number, err == nil && !math.IsInf(number, 0) && !math.IsNaN(number) && number >= 0
}

func parseCloudPriceTableJSON(text string) (map[string]cloudSyncPricingEntry, error) {
	var table struct {
		Schema   string            `json:"schema"`
		Currency string            `json:"currency"`
		Models   []json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal([]byte(text), &table); err != nil {
		return nil, fmt.Errorf("价格表 JSON 解析失败: %w", err)
	}
	if table.Schema != "cchp.pricing-table/v1" {
		return nil, fmt.Errorf("价格表格式无效：schema 不是 cchp.pricing-table/v1")
	}
	if table.Currency != "USD" {
		return nil, fmt.Errorf("价格表格式无效：仅支持 USD 币种")
	}
	if table.Models == nil {
		return nil, fmt.Errorf("价格表格式无效：缺少 models 数组")
	}
	models := make(map[string]cptModel)
	order := make([]string, 0, len(table.Models))
	for _, raw := range table.Models {
		var model cptModel
		if json.Unmarshal(raw, &model) != nil {
			continue
		}
		model.ModelName = strings.TrimSpace(model.ModelName)
		if model.ModelName == "" || strings.TrimSpace(model.Slug) == "" || strings.TrimSpace(model.Vendor) == "" || model.Pricing == nil {
			continue
		}
		old, exists := models[model.ModelName]
		if !exists {
			order = append(order, model.ModelName)
		}
		if !exists || preferCPTModel(old, model) {
			models[model.ModelName] = model
		}
	}
	entries := make(map[string]cloudSyncPricingEntry)
	for _, name := range order {
		model := models[name]
		var selected cloudSyncPricingEntry
		found := false
		for _, raw := range model.Pricing {
			var variant cptVariant
			if json.Unmarshal(raw, &variant) != nil || strings.TrimSpace(variant.Provider) == "" {
				continue
			}
			entry, ok := convertCPTVariant(model, variant)
			if !ok {
				continue
			}
			if !found || variant.Official {
				selected, found = entry, true
			}
			if variant.Official {
				break
			}
		}
		if found {
			entries[name] = selected
		}
	}
	for _, name := range order {
		entry, ok := entries[name]
		if !ok {
			continue
		}
		for index, alias := range models[name].Aliases {
			if index >= 64 {
				break
			}
			alias = strings.TrimSpace(alias)
			if alias == "" {
				continue
			}
			if _, canonical := models[alias]; canonical {
				continue
			}
			if _, exists := entries[alias]; exists {
				continue
			}
			copy := entry
			copy.MetadataFields = maps.Clone(entry.MetadataFields)
			copy.Model = alias
			copy.Pricing.CloudPricing = cloneCloudPricingRules(entry.Pricing.CloudPricing)
			entries[alias] = copy
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("云端价格表中没有可用的 token 定价模型")
	}
	return entries, nil
}

func preferCPTModel(old, candidate cptModel) bool {
	hasOfficial := func(model cptModel) bool {
		for _, raw := range model.Pricing {
			var variant cptVariant
			if json.Unmarshal(raw, &variant) == nil && variant.Official {
				return true
			}
		}
		return false
	}
	a, b := hasOfficial(old), hasOfficial(candidate)
	if a != b {
		return b
	}
	if (old.Vendor == "other") != (candidate.Vendor == "other") {
		return old.Vendor == "other"
	}
	return len(candidate.Pricing) > len(old.Pricing)
}

func convertCPTVariant(model cptModel, variant cptVariant) (cloudSyncPricingEntry, bool) {
	rules := &modelpricing.CloudPricingRules{Charges: make(map[string]float64), Tracks: make([]modelpricing.PricingTrack, 0, len(variant.Tracks))}
	fields := modelpricing.TokenChargeFields()
	record := make(map[string]any)
	for key, field := range fields {
		var charge struct {
			Price    string `json:"price"`
			Unit     string `json:"unit"`
			Currency string `json:"currency"`
		}
		rawCharge := variant.Charges[key]
		if key == "reasoning" && len(rawCharge) == 0 {
			rawCharge = variant.Charges["internal_reasoning"]
		}
		if json.Unmarshal(rawCharge, &charge) != nil || charge.Unit != "per_M_tokens" || (charge.Currency != "" && charge.Currency != "USD") {
			continue
		}
		price, ok := parseCPTDecimal(charge.Price)
		if !ok {
			continue
		}
		rules.Charges[key] = price / 1_000_000
		record[field] = price / 1_000_000
	}
	for _, raw := range variant.Tracks {
		var track struct {
			Label         string            `json:"label"`
			Factor        *string           `json:"factor"`
			ChargeFactors map[string]string `json:"charge_factors"`
			Triggers      []json.RawMessage `json:"triggers"`
		}
		if json.Unmarshal(raw, &track) != nil || track.Triggers == nil {
			return cloudSyncPricingEntry{}, false
		}
		converted := modelpricing.PricingTrack{Label: track.Label, Factor: 1, Triggers: make([]modelpricing.PricingTrigger, 0, len(track.Triggers))}
		for _, rawTrigger := range track.Triggers {
			var trigger modelpricing.PricingTrigger
			var presence struct {
				Threshold *int64 `json:"threshold"`
			}
			if json.Unmarshal(rawTrigger, &trigger) != nil || json.Unmarshal(rawTrigger, &presence) != nil {
				return cloudSyncPricingEntry{}, false
			}
			if trigger.Kind == "input_tokens_above" && (presence.Threshold == nil || *presence.Threshold < 0) {
				trigger.Kind = "invalid"
			}
			if trigger.Kind != "input_tokens_above" && trigger.Pattern == "" {
				trigger.Kind = "invalid"
			}
			converted.Triggers = append(converted.Triggers, trigger)
		}
		if track.Factor != nil {
			var ok bool
			converted.Factor, ok = parseCPTDecimal(*track.Factor)
			if !ok {
				return cloudSyncPricingEntry{}, false
			}
		}
		if len(track.ChargeFactors) > 0 {
			converted.ChargeFactors = make(map[string]float64, len(track.ChargeFactors))
			for key, value := range track.ChargeFactors {
				factor, ok := parseCPTDecimal(value)
				if !ok {
					return cloudSyncPricingEntry{}, false
				}
				if key == "internal_reasoning" {
					key = "reasoning"
				}
				converted.ChargeFactors[key] = factor
			}
		}
		rules.Tracks = append(rules.Tracks, converted)
	}
	entry, ok := buildCloudSyncPricingEntry(model.ModelName, record)
	if !ok {
		return cloudSyncPricingEntry{}, false
	}
	entry.DisplayName = model.DisplayName
	if entry.DisplayName == "" {
		entry.DisplayName = model.ModelName
	}
	entry.Mode = model.ModelType
	if entry.Mode == "" {
		entry.Mode = "chat"
	}
	entry.LiteLLMProvider = variant.Provider
	entry.Pricing.CloudPricing = rules.Clone()
	entry.MetadataFields = make(map[string]bool)
	if model.MaxInputTokens != nil && *model.MaxInputTokens > 0 {
		entry.Pricing.MaxInputTokens = *model.MaxInputTokens
		entry.MetadataFields["max_input_tokens"] = true
	}
	if model.MaxOutputTokens != nil && *model.MaxOutputTokens > 0 {
		entry.Pricing.MaxTokens = *model.MaxOutputTokens
		entry.MetadataFields["max_tokens"] = true
	}
	for key, target := range map[string]*bool{
		"computer_use": &entry.Pricing.SupportsComputerUse, "function_calling": &entry.Pricing.SupportsFunctionCalling,
		"pdf_input": &entry.Pricing.SupportsPDFInput, "prompt_caching": &entry.Pricing.SupportsPromptCaching,
		"reasoning": &entry.Pricing.SupportsReasoning, "structured_output": &entry.Pricing.SupportsResponseSchema,
		"vision": &entry.Pricing.SupportsVision,
	} {
		if value := model.Capabilities[key]; value != nil {
			*target = *value
			entry.MetadataFields[key] = true
		}
	}
	return entry, true
}

func cloneCloudPricingRules(source *modelpricing.CloudPricingRules) *modelpricing.CloudPricingRules {
	return source.Clone()
}
