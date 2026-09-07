/*
@name: 云端条件计费测试
@Descripttion: 验证轨道条件边界、字段倍率、缺失价格及深拷贝。
@version: 1.0.0
@Author: sm
@Date: 2026-09-07 11:13:46
@LastEditTime: 2026-09-07 11:13:46
@FilePath: resources/model-pricing/cloud_test.go
*/
package modelpricing

import (
	"encoding/json"
	"math"
	"testing"
)

func TestCloudReasoningProtocolAccounting(t *testing.T) {
	for _, mode := range []string{OutputExcludesReasoning, OutputIncludesReasoning} {
		for _, explicit := range []bool{false, true} {
			charges := map[string]float64{"completion": 10e-6}
			want := 110 * 20e-6
			if explicit {
				charges["reasoning"] = 5e-6
				want = 10*20e-6 + 100*10e-6
			}
			entry := &PricingEntry{CloudPricing: &CloudPricingRules{Charges: charges, Tracks: []PricingTrack{{Factor: 2}}}}
			usage := UsageSnapshot{OutputTokens: 10, ReasoningTokens: 100, Context: &PricingContext{OutputTokenMode: mode}}
			if mode == OutputIncludesReasoning {
				usage.OutputTokens = 110
			}
			got := calculateCloudCost(entry, usage, CostBreakdown{})
			if math.Abs(got.TotalCost-want) > 1e-12 || !got.PricingSnapshot.Complete {
				t.Fatalf("mode=%s explicit=%v got=%+v want=%g", mode, explicit, got, want)
			}
		}
	}
	got := calculateCloudCost(&PricingEntry{CloudPricing: &CloudPricingRules{Charges: map[string]float64{"prompt": 0}}}, UsageSnapshot{ReasoningTokens: 10, Context: &PricingContext{OutputTokenMode: OutputExcludesReasoning}}, CostBreakdown{})
	if got.PricingSnapshot.Complete {
		t.Fatal("unpriced thinking must be incomplete")
	}
}

func TestCloudCompiledTriggersSurviveCloneAndReload(t *testing.T) {
	rules := &CloudPricingRules{Tracks: []PricingTrack{{Factor: 1, Triggers: []PricingTrigger{{Kind: "body_matches", Field: "service_tier", Pattern: "^priority$"}}}}}
	compiled := rules.Clone()
	trigger := compiled.Tracks[0].Triggers[0]
	if trigger.compiled == nil {
		t.Fatal("trigger not prepared at load")
	}
	if compiled.Clone().Tracks[0].Triggers[0].compiled != trigger.compiled {
		t.Fatal("immutable regexp was recompiled")
	}
	if !rules.Equal(compiled) {
		t.Fatal("prepared cache changed pricing equality")
	}
	data, err := json.Marshal(compiled)
	if err != nil {
		t.Fatal(err)
	}
	var restored CloudPricingRules
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !restored.Equal(compiled) {
		t.Fatal("cache leaked into persisted rules")
	}
	restored.Tracks[0].Triggers[0].Pattern = "^flex$"
	changed := restored.Clone()
	if !matchPricingTrigger(changed.Tracks[0].Triggers[0], UsageSnapshot{Context: &PricingContext{ConditionsKnown: true, ServiceTier: "flex"}}) {
		t.Fatal("updated pattern used stale cache")
	}
	changed.Tracks[0].Triggers[0].Pattern = "["
	invalid := changed.Clone()
	if matchPricingTrigger(invalid.Tracks[0].Triggers[0], UsageSnapshot{Context: &PricingContext{ConditionsKnown: true, ServiceTier: "flex"}}) {
		t.Fatal("invalid regexp matched")
	}
}

func TestCloudTracksFirstMatchAndBoundary(t *testing.T) {
	rules := &CloudPricingRules{Charges: map[string]float64{"prompt": 1, "completion": 2, "cache_write": 3, "cache_write_1h": 4, "cache_read": 0}, Tracks: []PricingTrack{
		{Label: "priority-long", Factor: 2, ChargeFactors: map[string]float64{"completion": 3}, Triggers: []PricingTrigger{{Kind: "input_tokens_above", Threshold: 100, Inclusive: true}, {Kind: "body_matches", Field: "service_tier", Pattern: "^priority$"}}},
		{Label: "base", Factor: 1},
	}}
	service, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	service.ApplyOverrides(map[string]PricingEntry{"cloud": {CloudPricing: rules, GroupMultiplier: 0.5, HasGroupMultiplier: true}}, nil)
	usage := UsageSnapshot{InputTokens: 60, OutputTokens: 10, CacheCreateTokens: 20, CacheReadTokens: 20, CacheCreation: &CacheCreationDetail{Ephemeral5mTokens: 10, Ephemeral1hTokens: 10}, Context: &PricingContext{ServiceTier: "priority", ConditionsKnown: true}}
	got := service.CalculateCost("cloud", usage)
	if got.PricingSnapshot.TrackLabel != "priority-long" || got.TotalCost != 160 || !got.PricingSnapshot.Complete {
		t.Fatalf("unexpected costs: %+v snapshot=%+v", got, got.PricingSnapshot)
	}
	usage.InputTokens--
	if got := service.CalculateCost("cloud", usage); got.PricingSnapshot.TrackLabel != "base" {
		t.Fatalf("below inclusive boundary: %+v", got)
	}
	usage.InputTokens++
	usage.Context = nil
	if got := service.CalculateCost("cloud", usage); got.PricingSnapshot.TrackLabel != "base" {
		t.Fatalf("historical request guessed conditions: %+v", got)
	}
	usage.Context = &PricingContext{ConditionsKnown: true, ServiceTier: "flex"}
	if got := service.CalculateCost("cloud", usage); got.PricingSnapshot.TrackLabel != "base" {
		t.Fatalf("AND condition ignored: %+v", got)
	}
}

func TestCloudTriggerMatching(t *testing.T) {
	usage := UsageSnapshot{InputTokens: 100, Context: &PricingContext{ConditionsKnown: true, Operation: "responses.create", Headers: map[string]string{"Anthropic-Beta": "context-1m-2026-09-07"}}}
	cases := []struct {
		trigger PricingTrigger
		want    bool
	}{
		{PricingTrigger{Kind: "input_tokens_above", Threshold: 100}, false},
		{PricingTrigger{Kind: "input_tokens_above", Threshold: 100, Inclusive: true}, true},
		{PricingTrigger{Kind: "header_matches", Header: "anthropic-beta", Pattern: `context-1m-\d{4}-\d{2}-\d{2}`}, true},
		{PricingTrigger{Kind: "endpoint_matches", Pattern: `^batch\.`}, false},
		{PricingTrigger{Kind: "endpoint_matches", Pattern: `^responses\.`}, true},
		{PricingTrigger{Kind: "header_matches", Header: "anthropic-beta", Pattern: `[`}, false},
		{PricingTrigger{Kind: "body_matches", Field: "unknown", Pattern: `.*`}, false},
		{PricingTrigger{Kind: "unknown"}, false},
	}
	for _, c := range cases {
		if got := matchPricingTrigger(c.trigger, usage); got != c.want {
			t.Errorf("%+v: got %v want %v", c.trigger, got, c.want)
		}
	}
}

func TestCloudMissingAndFreePrices(t *testing.T) {
	entry := &PricingEntry{CloudPricing: &CloudPricingRules{Charges: map[string]float64{"prompt": 0}, Tracks: []PricingTrack{{Label: "discount", Factor: 0.5}}}}
	got := calculateCloudCost(entry, UsageSnapshot{InputTokens: 20}, CostBreakdown{})
	if !got.HasPricing || !got.PricingSnapshot.Complete || got.TotalCost != 0 {
		t.Fatalf("free price lost: %+v", got)
	}
	got = calculateCloudCost(entry, UsageSnapshot{InputTokens: 20, OutputTokens: 1}, CostBreakdown{})
	if got.PricingSnapshot.Complete {
		t.Fatal("missing output price reported complete")
	}
	if _, ok := got.PricingSnapshot.UnitPrices["completion"]; ok {
		t.Fatal("missing output price synthesized")
	}
}

func TestCloudNoLegacyLongContextOrDefaultDoubleFactor(t *testing.T) {
	service, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	service.ApplyOverrides(map[string]PricingEntry{"claude-sonnet-4-5": {CloudPricing: &CloudPricingRules{Charges: map[string]float64{"prompt": 0.000001}, Tracks: []PricingTrack{{Label: "default", Factor: 2}}}}}, nil)
	got := service.CalculateCost("claude-sonnet-4-5", UsageSnapshot{InputTokens: 300000, Context: &PricingContext{ConditionsKnown: true}})
	if math.Abs(got.TotalCost-0.6) > 1e-10 || got.IsLongContext {
		t.Fatalf("unexpected default/legacy pricing: %+v", got)
	}
}

func TestCloudRulesDeepClone(t *testing.T) {
	service, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	rules := &CloudPricingRules{Charges: map[string]float64{"prompt": 1}, Tracks: []PricingTrack{{Label: "base", Factor: 1, ChargeFactors: map[string]float64{"prompt": 2}, Triggers: []PricingTrigger{{Kind: "input_tokens_above", Threshold: 1}}}}}
	service.ApplyOverrides(map[string]PricingEntry{"cloud": {CloudPricing: rules}}, nil)
	rules.Charges["prompt"] = 99
	clone := service.Clone()
	clone.pricingMap["cloud"].CloudPricing.Tracks[0].ChargeFactors["prompt"] = 99
	clone.pricingMap["cloud"].CloudPricing.Tracks[0].Triggers[0].Threshold = 99
	entry, _ := service.PricingEntryExact("cloud")
	if entry.CloudPricing.Charges["prompt"] != 1 || entry.CloudPricing.Tracks[0].ChargeFactors["prompt"] != 2 || entry.CloudPricing.Tracks[0].Triggers[0].Threshold != 1 {
		t.Fatal("cloud pricing shares mutable data")
	}
	entry.CloudPricing.Charges["prompt"] = 99
	if service.pricingMap["cloud"].CloudPricing.Charges["prompt"] != 1 {
		t.Fatal("exact lookup shares mutable data")
	}
}
