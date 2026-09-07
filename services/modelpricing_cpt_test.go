/**
 * @name: CPT 解析回归测试
 * @Descripttion: 验证 CPT 报价选择、单位转换、条件轨道及快照隔离。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-07 11:10:24
 * @LastEditTime: 2026-09-07 11:10:24
 * @FilePath: services/modelpricing_cpt_test.go
 */
package services

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	modelpricing "codeswitch/resources/model-pricing"
)

func TestCloudPriceTableLiveCompatibility(t *testing.T) {
	if os.Getenv("CCH_PRICING_LIVE_TEST") != "1" {
		t.Skip("set CCH_PRICING_LIVE_TEST=1 to validate the live CPT feed")
	}
	payload, err := fetchCloudPriceTableJSON()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := parseCloudPriceTableJSON(payload)
	if err != nil {
		t.Fatal(err)
	}
	for model, entry := range entries {
		if entry.Pricing.CloudPricing == nil {
			t.Fatalf("model %q has no cloud pricing rules", model)
		}
	}
	t.Logf("parsed %d model entries from %d bytes", len(entries), len(payload))
}

func TestCPTOfficialSyncClearsCloudRules(t *testing.T) {
	_, queue := newDBWriteQueueTestFixture(t, 16, false)
	if err := queue.Exec(`CREATE TABLE app_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatal(err)
	}
	previous := GlobalDBQueue
	GlobalDBQueue = queue
	t.Cleanup(func() { GlobalDBQueue = previous })
	for _, staleOfficial := range []bool{false, true} {
		mps := newTestModelPricingService(t)
		entry := buildTestPricingEntry(3e-6, 15e-6, 3.75e-6, .3e-6, 1)
		entry.CloudPricing = &modelpricing.CloudPricingRules{Charges: map[string]float64{"prompt": 9e-6, "completion": 18e-6}}
		layer := &mps.cloudOverrides
		source := modelPricingSourceCloudSync
		if staleOfficial {
			layer = &mps.localOverrides
			source = modelPricingSourceClaudeSync
		}
		layer.Pricing["claude-sonnet-4-5"] = entry
		layer.Meta["claude-sonnet-4-5"] = modelPricingMeta{Source: source}
		layer.Ephemeral1h["claude-sonnet-4-5"] = 6e-6
		mps.rebuildLocked()
		mps.storeClaudePricingPreviewCache([]claudeOfficialModelPricing{{DisplayName: "Claude Sonnet 4.5", InputPerToken: 3e-6, OutputPerToken: 15e-6, CacheCreate5mPerToken: 3.75e-6, CacheCreate1hPerToken: 6e-6, CacheReadPerToken: .3e-6}}, time.Now())
		if _, err := mps.SyncClaudeOfficialPricing(); err != nil {
			t.Fatal(err)
		}
		entry, _ = mps.effective.PricingEntryExact("claude-sonnet-4-5")
		got := mps.effective.CalculateCost("claude-sonnet-4-5", modelpricing.UsageSnapshot{InputTokens: 1000000})
		if entry.CloudPricing != nil || !floatEquals(got.InputCost, 3) {
			t.Fatalf("stale=%v official pricing ignored: %+v", staleOfficial, got)
		}
	}
}

func TestCPTMetadataSurvivesMergeAndSerialization(t *testing.T) {
	entries, err := parseCloudPriceTableJSON(`{"schema":"cchp.pricing-table/v1","currency":"USD","models":[{"slug":"test/model","model_name":"model","vendor":"test","max_input_tokens":1000000,"max_output_tokens":64000,"capabilities":{"vision":false,"reasoning":true},"pricing":[{"provider":"test","official":true,"charges":{"prompt":{"price":"1","unit":"per_M_tokens"}}}]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	base := modelpricing.PricingEntry{MaxInputTokens: 200000, MaxTokens: 8000, SupportsVision: true, SupportsFunctionCalling: true}
	merged := mergeCloudPricingEntry(base, entries["model"])
	if merged.MaxInputTokens != 1000000 || merged.MaxTokens != 64000 || merged.SupportsVision || !merged.SupportsReasoning || !merged.SupportsFunctionCalling {
		t.Fatalf("incorrect metadata merge: %+v", merged)
	}
	data, err := json.Marshal(merged)
	if err != nil {
		t.Fatal(err)
	}
	var restored modelpricing.PricingEntry
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.MaxInputTokens != merged.MaxInputTokens || restored.MaxTokens != merged.MaxTokens || restored.SupportsVision || !restored.SupportsReasoning || !restored.SupportsFunctionCalling {
		t.Fatalf("metadata lost after reload: %+v", restored)
	}
	missing := entries["model"]
	missing.MetadataFields = nil
	missing.Pricing.MaxInputTokens = 0
	missing.Pricing.SupportsVision = false
	retained := mergeCloudPricingEntry(base, missing)
	if retained.MaxInputTokens != base.MaxInputTokens || !retained.SupportsVision {
		t.Fatal("missing metadata cleared defaults")
	}
	changed := merged
	changed.MaxInputTokens++
	if modelPricingEntriesEquivalent(merged, changed) {
		t.Fatal("metadata-only change ignored")
	}
}

type cptTestTransport func(*http.Request) (*http.Response, error)

func (f cptTestTransport) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

type cptOversizedBody struct{}

func (cptOversizedBody) Read(buffer []byte) (int, error) { clear(buffer); return len(buffer), nil }

func TestFetchCloudPriceTableJSON(t *testing.T) {
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	for _, oversized := range []bool{false, true} {
		http.DefaultTransport = cptTestTransport(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != cloudPriceTableURL || request.Header.Get("Accept") != "application/json" {
				t.Fatalf("unexpected download request: %v", request)
			}
			var body io.Reader = strings.NewReader(testCloudPriceTableJSON())
			if oversized {
				body = cptOversizedBody{}
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(body), Request: request}, nil
		})
		text, err := fetchCloudPriceTableJSON()
		if oversized {
			if err == nil || !strings.Contains(err.Error(), "64 MiB") {
				t.Fatalf("missing size error: %v", err)
			}
			continue
		}
		if err != nil || text != testCloudPriceTableJSON() {
			t.Fatalf("download failed: %v", err)
		}
	}
}

func testCloudPriceTableJSON() string {
	return `{"schema":"cchp.pricing-table/v1","currency":"USD","models":[
	{"slug":"manual-model","model_name":"manual-model","vendor":"openai","display_name":"Manual Model","pricing":[{"provider":"openai","official":true,"charges":{"prompt":{"price":"1","unit":"per_M_tokens"},"completion":{"price":"2","unit":"per_M_tokens"},"cache_write":{"price":"1.25","unit":"per_M_tokens"},"cache_write_1h":{"price":"1.5","unit":"per_M_tokens"},"cache_read":{"price":"0.1","unit":"per_M_tokens"}}}]},
	{"slug":"cloud-model","model_name":"cloud-model","vendor":"openai","pricing":[{"provider":"openai","official":true,"charges":{"prompt":{"price":"3","unit":"per_M_tokens"},"completion":{"price":"6","unit":"per_M_tokens"}},"tracks":[{"label":"default","factor":"1.2","triggers":[]}]}]}
	]}`
}

func TestParseCloudPriceTableJSON(t *testing.T) {
	entries, err := parseCloudPriceTableJSON(testCloudPriceTableJSON())
	if err != nil {
		t.Fatal(err)
	}
	entry := entries["manual-model"]
	if entry.Pricing.InputCostPerToken != 0.000001 || !entry.Pricing.HasInputCostPerToken || entry.ExplicitEphemeral1h != 0.0000015 {
		t.Fatalf("incorrect conversion: %+v", entry)
	}
	if entries["cloud-model"].Pricing.CloudPricing.Tracks[0].Factor != 1.2 {
		t.Fatal("missing default track")
	}
	for _, value := range []string{"{}", `{"schema":"wrong","currency":"USD","models":[]}`, `{"schema":"cchp.pricing-table/v1","currency":"CNY","models":[]}`, `{"schema":"cchp.pricing-table/v1","currency":"USD","models":{}}`, `{"schema":"cchp.pricing-table/v1","currency":"USD","models":[]}`} {
		if _, err := parseCloudPriceTableJSON(value); err == nil {
			t.Fatalf("accepted invalid table: %s", value)
		}
	}
}

func TestCPTQuoteSelectionAndAliases(t *testing.T) {
	text := `{"schema":"cchp.pricing-table/v1","currency":"USD","models":[
	{"slug":"a","model_name":"a","vendor":"other","aliases":["b","alias"],"pricing":[{"provider":"reseller","charges":{"prompt":{"price":"20","unit":"per_M_tokens"}}},{"provider":"invalid","official":true,"charges":{"prompt":{"price":"NaN","unit":"per_M_tokens"}}},{"provider":"official","official":true,"charges":{"prompt":{"price":"0","unit":"per_M_tokens"},"completion":{"price":"2","unit":"per_M_tokens","currency":"CNY"}}}]},
	{"slug":"b","model_name":"b","vendor":"openai","pricing":[{"provider":"openai","charges":{"prompt":{"price":"3","unit":"per_M_tokens"}}}]}
	]}`
	entries, err := parseCloudPriceTableJSON(text)
	if err != nil {
		t.Fatal(err)
	}
	if entries["a"].LiteLLMProvider != "official" || !entries["a"].Pricing.HasInputCostPerToken || entries["a"].Pricing.HasOutputCostPerToken {
		t.Fatalf("incorrect selected quote: %+v", entries["a"])
	}
	if entries["b"].Pricing.InputCostPerToken != 0.000003 || entries["alias"].Pricing.InputCostPerToken != 0 {
		t.Fatal("canonical alias precedence failed")
	}
	for _, invalid := range []string{"-1", "NaN", "Inf", "0x10", "1e999"} {
		if _, ok := parseCPTDecimal(invalid); ok {
			t.Fatalf("accepted invalid decimal %s", invalid)
		}
	}
}

func TestCPTTrackChangesAndPreviewIsolation(t *testing.T) {
	entries, err := parseCloudPriceTableJSON(testCloudPriceTableJSON())
	if err != nil {
		t.Fatal(err)
	}
	mps := newTestModelPricingService(t)
	mps.storeCloudPricingPreviewCache(entries, time.Now())
	entries["cloud-model"].Pricing.CloudPricing.Tracks[0].Factor = 99
	cached, ok := mps.consumeCloudPricingPreviewCache(time.Now())
	if !ok || cached["cloud-model"].Pricing.CloudPricing.Tracks[0].Factor != 1.2 {
		t.Fatal("preview cache shares mutable rules")
	}
	overrides := newEmptyModelPricingOverrides()
	if !applyCloudEntryToOverrides(&overrides, cached["cloud-model"], "first") {
		t.Fatal("initial sync missing")
	}
	cloned := cloneModelPricingOverrides(overrides)
	cloned.Pricing["cloud-model"].CloudPricing.Tracks[0].Factor = 2
	if overrides.Pricing["cloud-model"].CloudPricing.Tracks[0].Factor != 1.2 {
		t.Fatal("override clone shares rules")
	}
	if modelPricingEntriesEquivalent(overrides.Pricing["cloud-model"], cloned.Pricing["cloud-model"]) {
		t.Fatal("track-only change ignored")
	}
	encoded, err := json.Marshal(cloned)
	if err != nil {
		t.Fatal(err)
	}
	var restored modelPricingOverrides
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if !modelPricingEntriesEquivalent(cloned.Pricing["cloud-model"], restored.Pricing["cloud-model"]) {
		t.Fatal("rules changed after persistence")
	}
	if _, err := parseCloudPriceTableJSON(strings.ReplaceAll(testCloudPriceTableJSON(), `"factor":"1.2"`, `"factor":"-2"`)); err != nil {
		t.Fatal("one bad variant should not invalidate usable models")
	}
}

func TestCPTManualEditClearsCloudRules(t *testing.T) {
	entries, err := parseCloudPriceTableJSON(testCloudPriceTableJSON())
	if err != nil {
		t.Fatal(err)
	}
	entry := applyModelPricingRowToEntry(entries["cloud-model"].Pricing, ModelPricingRow{InputCostPerToken: 4, GroupMultiplier: 1})
	if entry.CloudPricing != nil || entry.InputCostPerToken != 4 {
		t.Fatalf("manual edit retained cloud rules: %+v", entry)
	}
}

func TestCPTConflictPreservesOrReplacesWholeRules(t *testing.T) {
	for _, overwrite := range []bool{false, true} {
		t.Run(map[bool]string{false: "keep", true: "replace"}[overwrite], func(t *testing.T) {
			restoreCloudSyncHooks(t)
			mps := newTestModelPricingService(t)
			manual := buildTestPricingEntry(9, 18, 10, 1, 1)
			manual.CloudPricing = &modelpricing.CloudPricingRules{Charges: map[string]float64{"prompt": 9}, Tracks: []modelpricing.PricingTrack{{Label: "old", Factor: 3}}}
			mps.localOverrides.Pricing["cloud-model"] = manual
			mps.localOverrides.Meta["cloud-model"] = modelPricingMeta{Source: modelPricingSourceManual}
			mps.rebuildLocked()
			loads := 0
			loadCloudPriceTableJSONFunc = func() (string, error) { loads++; return testCloudPriceTableJSON(), nil }
			savePrimaryPricingOverridesFunc = func(modelPricingOverrides) error { return nil }
			saveCloudPricingOverridesFunc = func(modelPricingOverrides) error { return nil }
			preview, err := mps.PreviewCloudPriceTableSyncConflicts()
			if err != nil {
				t.Fatal(err)
			}
			if len(preview.Conflicts) != 1 || preview.Conflicts[0].Current.CloudPricing.Tracks[0].Label != "old" || preview.Conflicts[0].Incoming.CloudPricing.Tracks[0].Label != "default" {
				t.Fatalf("missing rules in conflict: %+v", preview)
			}
			var selected []string
			if overwrite {
				selected = []string{"cloud-model"}
			}
			if _, err := mps.SyncCloudPriceTable(selected); err != nil {
				t.Fatal(err)
			}
			if loads != 1 {
				t.Fatal("apply did not consume preview snapshot")
			}
			current, _ := mps.effective.PricingEntryExact("cloud-model")
			expected := "old"
			if overwrite {
				expected = "default"
			}
			if current.CloudPricing.Tracks[0].Label != expected {
				t.Fatalf("wrong retained rules: %+v", current.CloudPricing)
			}
		})
	}
}
