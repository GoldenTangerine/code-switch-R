/*
@name: 云端条件计费
@Descripttion: 按有序条件轨道计算云端模型报价并保存实际单价。
@version: 1.0.0
@Author: sm
@Date: 2026-09-07 11:13:46
@LastEditTime: 2026-09-07 11:13:46
@FilePath: resources/model-pricing/cloud.go
*/
package modelpricing

import (
	"maps"
	"regexp"
	"slices"
	"strings"
)

type CloudPricingRules struct {
	Charges map[string]float64 `json:"charges"`
	Tracks  []PricingTrack     `json:"tracks"`
}

type PricingTrack struct {
	Label         string             `json:"label"`
	Factor        float64            `json:"factor"`
	ChargeFactors map[string]float64 `json:"charge_factors,omitempty"`
	Triggers      []PricingTrigger   `json:"triggers"`
}

type PricingTrigger struct {
	Kind            string `json:"kind"`
	Threshold       int64  `json:"threshold,omitempty"`
	Inclusive       bool   `json:"inclusive,omitempty"`
	Field           string `json:"field,omitempty"`
	Header          string `json:"header,omitempty"`
	Pattern         string `json:"pattern,omitempty"`
	compiledPattern string
	compiled        *regexp.Regexp
	prepared        bool
}

type PricingContext struct {
	ServiceTier     string            `json:"service_tier,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	Operation       string            `json:"operation,omitempty"`
	ConditionsKnown bool              `json:"conditions_known"`
	OutputTokenMode string            `json:"output_token_mode,omitempty"`
}

const (
	OutputIncludesReasoning = "includes_reasoning"
	OutputExcludesReasoning = "excludes_reasoning"
)

// TokenChargeFields is the supported CPT-to-local field mapping.
func TokenChargeFields() map[string]string {
	return map[string]string{
		"prompt": "input_cost_per_token", "completion": "output_cost_per_token",
		"reasoning": "output_cost_per_reasoning_token", "cache_read": "cache_read_input_token_cost",
		"cache_write": "cache_creation_input_token_cost", "cache_write_1h": "cache_creation_input_token_cost_above_1hr",
	}
}

// BillableOutputTokens excludes reasoning only when the protocol already includes it.
func (u UsageSnapshot) BillableOutputTokens() int {
	if u.Context != nil && u.Context.OutputTokenMode == OutputIncludesReasoning {
		return max(0, u.OutputTokens-u.ReasoningTokens)
	}
	return u.OutputTokens
}

func (u UsageSnapshot) HasReasoningTokenMode() bool {
	return u.Context != nil && (u.Context.OutputTokenMode == OutputIncludesReasoning || u.Context.OutputTokenMode == OutputExcludesReasoning)
}

type PricingSnapshot struct {
	TrackLabel   string             `json:"track_label,omitempty"`
	UnitPrices   map[string]float64 `json:"unit_prices"`
	FieldSources map[string]string  `json:"field_sources"`
	Complete     bool               `json:"complete"`
}

func (r *CloudPricingRules) Clone() *CloudPricingRules {
	if r == nil {
		return nil
	}
	copy := &CloudPricingRules{Charges: maps.Clone(r.Charges), Tracks: slices.Clone(r.Tracks)}
	for i := range copy.Tracks {
		copy.Tracks[i].ChargeFactors = maps.Clone(r.Tracks[i].ChargeFactors)
		copy.Tracks[i].Triggers = slices.Clone(r.Tracks[i].Triggers)
		for j := range copy.Tracks[i].Triggers {
			copy.Tracks[i].Triggers[j].prepare()
		}
	}
	return copy
}

func (r *CloudPricingRules) Equal(other *CloudPricingRules) bool {
	if r == nil || other == nil {
		return r == other
	}
	if !maps.Equal(r.Charges, other.Charges) || len(r.Tracks) != len(other.Tracks) {
		return false
	}
	for i, a := range r.Tracks {
		b := other.Tracks[i]
		if a.Label != b.Label || a.Factor != b.Factor || !maps.Equal(a.ChargeFactors, b.ChargeFactors) || len(a.Triggers) != len(b.Triggers) {
			return false
		}
		for j, left := range a.Triggers {
			right := b.Triggers[j]
			left.compiled, right.compiled = nil, nil
			left.compiledPattern, right.compiledPattern = "", ""
			left.prepared, right.prepared = false, false
			if left != right {
				return false
			}
		}
	}
	return true
}

func (t *PricingTrigger) prepare() {
	if t.prepared && t.compiledPattern == t.Pattern {
		return
	}
	t.prepared, t.compiledPattern = true, t.Pattern
	t.compiled = nil
	if t.Kind == "body_matches" || t.Kind == "header_matches" || t.Kind == "endpoint_matches" {
		t.compiled, _ = regexp.Compile(t.Pattern)
	}
}

func matchPricingTrigger(trigger PricingTrigger, usage UsageSnapshot) bool {
	if usage.Context == nil || !usage.Context.ConditionsKnown {
		return false
	}
	var value string
	switch trigger.Kind {
	case "input_tokens_above":
		five, one := resolveCacheTokens(usage)
		total := int64(usage.InputTokens) + int64(usage.CacheReadTokens) + int64(five) + int64(one)
		return total > trigger.Threshold || (trigger.Inclusive && total == trigger.Threshold)
	case "body_matches":
		if trigger.Field != "service_tier" {
			return false
		}
		value = usage.Context.ServiceTier
	case "header_matches":
		for name, content := range usage.Context.Headers {
			if strings.EqualFold(name, trigger.Header) {
				value = content
				break
			}
		}
	case "endpoint_matches":
		value = usage.Context.Operation
	default:
		return false
	}
	if value == "" {
		return false
	}
	trigger.prepare()
	return trigger.compiled != nil && trigger.compiled.MatchString(value)
}

func calculateCloudCost(entry *PricingEntry, usage UsageSnapshot, result CostBreakdown) CostBreakdown {
	rules := entry.CloudPricing
	var selected *PricingTrack
	for i := range rules.Tracks {
		track := &rules.Tracks[i]
		matches := true
		for _, trigger := range track.Triggers {
			if !matchPricingTrigger(trigger, usage) {
				matches = false
				break
			}
		}
		if matches {
			selected = track
			break
		}
	}
	multiplier := 1.0
	if entry.HasGroupMultiplier || entry.GroupMultiplier != 0 {
		multiplier = entry.GroupMultiplier
	}
	snapshot := &PricingSnapshot{UnitPrices: make(map[string]float64), FieldSources: make(map[string]string), Complete: true}
	if selected != nil {
		snapshot.TrackLabel = selected.Label
		for _, trigger := range selected.Triggers {
			if trigger.Kind == "input_tokens_above" {
				result.IsLongContext = true
			}
		}
	}
	for field, price := range rules.Charges {
		factor := 1.0
		if selected != nil {
			factor = selected.Factor
			if override, ok := selected.ChargeFactors[field]; ok {
				factor = override
			}
		}
		snapshot.UnitPrices[field] = price * factor * multiplier
		snapshot.FieldSources[field] = "cloud"
	}
	cost := func(field string, tokens int) float64 {
		price, exists := snapshot.UnitPrices[field]
		if tokens > 0 && !exists {
			snapshot.Complete = false
		}
		return float64(tokens) * price
	}
	five, one := resolveCacheTokens(usage)
	if usage.HasReasoningTokenMode() {
		if _, exists := snapshot.UnitPrices["reasoning"]; !exists {
			if price, priced := snapshot.UnitPrices["completion"]; priced {
				snapshot.UnitPrices["reasoning"] = price
				snapshot.FieldSources["reasoning"] = snapshot.FieldSources["completion"]
			}
		}
		usage.OutputTokens = usage.BillableOutputTokens()
	}
	result.InputCost = cost("prompt", usage.InputTokens)
	result.OutputCost = cost("completion", usage.OutputTokens)
	// Reasoning is already included in output unless the quote explicitly prices it separately.
	if _, exists := snapshot.UnitPrices["reasoning"]; exists || usage.HasReasoningTokenMode() {
		result.ReasoningCost = cost("reasoning", usage.ReasoningTokens)
	}
	result.Ephemeral5mCost = cost("cache_write", five)
	result.Ephemeral1hCost = cost("cache_write_1h", one)
	result.CacheCreateCost = result.Ephemeral5mCost + result.Ephemeral1hCost
	result.CacheReadCost = cost("cache_read", usage.CacheReadTokens)
	result.TotalCost = result.InputCost + result.OutputCost + result.ReasoningCost + result.CacheCreateCost + result.CacheReadCost
	result.GroupMultiplier = multiplier
	result.HasPricing = len(snapshot.UnitPrices) > 0
	result.PricingSnapshot = snapshot
	return result
}
