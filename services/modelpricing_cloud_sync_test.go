/**
 * @name: 云端同步回归测试
 * @Descripttion: 验证云端同步冲突选择与持久化行为。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-07 11:10:24
 * @LastEditTime: 2026-09-07 11:10:24
 * @FilePath: services/modelpricing_cloud_sync_test.go
 */
package services

import (
	"testing"

	modelpricing "codeswitch/resources/model-pricing"
)

func TestParseCloudPriceTableTOML(t *testing.T) {
	tomlText := `
[metadata]
version = "test"

[models."chat-model"]
display_name = "Chat Model"
mode = "chat"
input_cost_per_token = 0.000001
output_cost_per_token = 0.000002

[models."chat-model".pricing."openai"]
cache_creation_input_token_cost = 0.00000125
cache_creation_input_token_cost_above_1hr = 0.0000015
cache_read_input_token_cost = 0.0000001
group_multiplier = 1.5
`

	table, err := parseCloudPriceTableTOML(tomlText)
	if err != nil {
		t.Fatalf("parseCloudPriceTableTOML 返回错误: %v", err)
	}
	if len(table.Models) != 1 {
		t.Fatalf("len(table.Models) = %d, 期望 1", len(table.Models))
	}
	chatModel, ok := table.Models["chat-model"]
	if !ok {
		t.Fatalf("缺少 chat-model")
	}
	if table.Metadata["version"] != "test" {
		t.Fatalf("metadata.version = %v, 期望 test", table.Metadata["version"])
	}
	if got, ok := numberValue(chatModel["input_cost_per_token"]); !ok || !floatAlmostEqual(got, 0.000001) {
		t.Fatalf("chat-model.input_cost_per_token = %v, ok=%v", chatModel["input_cost_per_token"], ok)
	}
	pricingRaw, ok := asStringMap(chatModel["pricing"])
	if !ok {
		t.Fatalf("chat-model.pricing 未解析成 map")
	}
	providerRaw, ok := asStringMap(pricingRaw["openai"])
	if !ok {
		t.Fatalf("chat-model.pricing.openai 未解析成 map")
	}
	if got, ok := numberValue(providerRaw["cache_creation_input_token_cost_above_1hr"]); !ok || !floatAlmostEqual(got, 0.0000015) {
		t.Fatalf("chat-model.pricing.openai.cache_creation_input_token_cost_above_1hr = %v, ok=%v", providerRaw["cache_creation_input_token_cost_above_1hr"], ok)
	}
}

func TestBuildCloudSyncPricingMap_UsesNestedPricingFallback(t *testing.T) {
	rows := map[string]map[string]any{
		"nested-model": {
			"display_name":              "Nested Model",
			"mode":                      "chat",
			"litellm_provider":          "openai",
			"selected_pricing_provider": "openai",
			"pricing": map[string]any{
				"openai": map[string]any{
					"input_cost_per_token":                      0.000003,
					"output_cost_per_token":                     0.000006,
					"output_cost_per_reasoning_token":           0.000009,
					"cache_creation_input_token_cost":           0.00000375,
					"cache_creation_input_token_cost_above_1hr": 0.0000045,
					"cache_read_input_token_cost":               0.0000003,
					"group_multiplier":                          1.2,
				},
			},
		},
		"image-model": {
			"mode": "image_generation",
			"pricing": map[string]any{
				"openai": map[string]any{
					"output_cost_per_image": 0.02,
				},
			},
		},
	}

	syncMap := buildCloudSyncPricingMap(rows)
	if len(syncMap) != 1 {
		t.Fatalf("len(syncMap) = %d, 期望 1", len(syncMap))
	}
	entry, ok := syncMap["nested-model"]
	if !ok {
		t.Fatalf("未找到 nested-model")
	}
	if entry.DisplayName != "Nested Model" {
		t.Fatalf("DisplayName = %q, 期望 %q", entry.DisplayName, "Nested Model")
	}
	if !entry.Pricing.HasInputCostPerToken || !floatAlmostEqual(entry.Pricing.InputCostPerToken, 0.000003) {
		t.Fatalf("InputCostPerToken = %f, Has=%v", entry.Pricing.InputCostPerToken, entry.Pricing.HasInputCostPerToken)
	}
	if !entry.Pricing.HasOutputCostPerReasoningToken || !floatAlmostEqual(entry.Pricing.OutputCostPerReasoningToken, 0.000009) {
		t.Fatalf("OutputCostPerReasoningToken = %f, Has=%v", entry.Pricing.OutputCostPerReasoningToken, entry.Pricing.HasOutputCostPerReasoningToken)
	}
	if !entry.Pricing.HasGroupMultiplier || !floatAlmostEqual(entry.Pricing.GroupMultiplier, 1.2) {
		t.Fatalf("GroupMultiplier = %f, Has=%v", entry.Pricing.GroupMultiplier, entry.Pricing.HasGroupMultiplier)
	}
	if !entry.HasExplicitEphemeral || !floatAlmostEqual(entry.ExplicitEphemeral1h, 0.0000045) {
		t.Fatalf("ExplicitEphemeral1h = %f, Has=%v", entry.ExplicitEphemeral1h, entry.HasExplicitEphemeral)
	}
	if _, ok := syncMap["image-model"]; ok {
		t.Fatalf("image-model 不应被同步")
	}
}

func TestPreviewCloudPriceTableSyncConflicts_ReturnsManualConflicts(t *testing.T) {
	mps := newTestModelPricingService(t)
	manualEntry := buildTestPricingEntry(0.000009, 0.000018, 0.000010, 0.0000009, 1)
	mps.localOverrides.Pricing["manual-model"] = manualEntry
	mps.localOverrides.Ephemeral1h["manual-model"] = 0.000012
	mps.localOverrides.Meta["manual-model"] = modelPricingMeta{
		Source:    modelPricingSourceManual,
		UpdatedAt: "2026-04-21T00:00:00Z",
	}
	mps.rebuildLocked()

	restoreCloudSyncHooks(t)
	loadCloudPriceTableJSONFunc = func() (string, error) {
		return testCloudPriceTableJSON(), nil
	}

	result, err := mps.PreviewCloudPriceTableSyncConflicts()
	if err != nil {
		t.Fatalf("PreviewCloudPriceTableSyncConflicts 失败: %v", err)
	}
	if result.Provider != "cloud" {
		t.Fatalf("Provider = %q, 期望 cloud", result.Provider)
	}
	if len(result.Conflicts) != 1 {
		t.Fatalf("len(result.Conflicts) = %d, 期望 1", len(result.Conflicts))
	}
	conflict := result.Conflicts[0]
	if conflict.Model != "manual-model" {
		t.Fatalf("conflict.Model = %q, 期望 manual-model", conflict.Model)
	}
	if conflict.DisplayName != "Manual Model" {
		t.Fatalf("conflict.DisplayName = %q, 期望 Manual Model", conflict.DisplayName)
	}
	if !floatAlmostEqual(conflict.Current.InputCostPerToken, manualEntry.InputCostPerToken) {
		t.Fatalf("Current.InputCostPerToken = %f, 期望 %f", conflict.Current.InputCostPerToken, manualEntry.InputCostPerToken)
	}
	if !floatAlmostEqual(conflict.Incoming.InputCostPerToken, 0.000001) {
		t.Fatalf("Incoming.InputCostPerToken = %f, 期望 %f", conflict.Incoming.InputCostPerToken, 0.000001)
	}
}

func TestSyncCloudPriceTable_SkipsManualOverrides(t *testing.T) {
	mps := newTestModelPricingService(t)
	manualEntry := buildTestPricingEntry(0.000009, 0.000018, 0.000010, 0.0000009, 1)
	mps.localOverrides.Pricing["manual-model"] = manualEntry
	mps.localOverrides.Ephemeral1h["manual-model"] = 0.000012
	mps.localOverrides.Meta["manual-model"] = modelPricingMeta{
		Source:    modelPricingSourceManual,
		UpdatedAt: "2026-04-21T00:00:00Z",
	}
	mps.rebuildLocked()

	restoreCloudSyncHooks(t)
	loadCloudPriceTableJSONFunc = func() (string, error) {
		return testCloudPriceTableJSON(), nil
	}

	primarySaveCalls := 0
	cloudSaveCalls := 0
	savePrimaryPricingOverridesFunc = func(overrides modelPricingOverrides) error {
		primarySaveCalls++
		return nil
	}
	saveCloudPricingOverridesFunc = func(overrides modelPricingOverrides) error {
		cloudSaveCalls++
		return nil
	}

	result, err := mps.SyncCloudPriceTable(nil)
	if err != nil {
		t.Fatalf("SyncCloudPriceTable 失败: %v", err)
	}
	if result.CreatedModels != 1 {
		t.Fatalf("CreatedModels = %d, 期望 1", result.CreatedModels)
	}
	if result.UpdatedModels != 0 {
		t.Fatalf("UpdatedModels = %d, 期望 0", result.UpdatedModels)
	}
	if result.UnchangedModels != 1 {
		t.Fatalf("UnchangedModels = %d, 期望 1", result.UnchangedModels)
	}
	if len(result.SkippedManualModels) != 1 || result.SkippedManualModels[0] != "manual-model" {
		t.Fatalf("SkippedManualModels = %v, 期望 [manual-model]", result.SkippedManualModels)
	}
	if primarySaveCalls != 0 {
		t.Fatalf("primarySaveCalls = %d, 期望 0", primarySaveCalls)
	}
	if cloudSaveCalls != 1 {
		t.Fatalf("cloudSaveCalls = %d, 期望 1", cloudSaveCalls)
	}

	manualPricing := mps.localOverrides.Pricing["manual-model"]
	if !floatAlmostEqual(manualPricing.InputCostPerToken, manualEntry.InputCostPerToken) {
		t.Fatalf("manual-model 被错误覆盖: %f", manualPricing.InputCostPerToken)
	}

	cloudPricing, ok := mps.cloudOverrides.Pricing["cloud-model"]
	if !ok {
		t.Fatalf("未写入 cloud-model")
	}
	if !floatAlmostEqual(cloudPricing.InputCostPerToken, 0.000003) {
		t.Fatalf("cloud-model.InputCostPerToken = %f, 期望 %f", cloudPricing.InputCostPerToken, 0.000003)
	}
	if cloudPricing.CloudPricing == nil || len(cloudPricing.CloudPricing.Tracks) != 1 || cloudPricing.CloudPricing.Tracks[0].Factor != 1.2 {
		t.Fatalf("cloud-model 未保存默认计费轨道: %+v", cloudPricing.CloudPricing)
	}
	if mps.cloudOverrides.Meta["cloud-model"].Source != modelPricingSourceCloudSync {
		t.Fatalf("cloud-model.Source = %q, 期望 %q", mps.cloudOverrides.Meta["cloud-model"].Source, modelPricingSourceCloudSync)
	}
}

func TestSyncCloudPriceTable_OverwritesSelectedManualOverrides(t *testing.T) {
	mps := newTestModelPricingService(t)
	manualEntry := buildTestPricingEntry(0.000009, 0.000018, 0.000010, 0.0000009, 1)
	mps.localOverrides.Pricing["manual-model"] = manualEntry
	mps.localOverrides.Ephemeral1h["manual-model"] = 0.000012
	mps.localOverrides.Meta["manual-model"] = modelPricingMeta{
		Source:    modelPricingSourceManual,
		UpdatedAt: "2026-04-21T00:00:00Z",
	}
	mps.rebuildLocked()

	restoreCloudSyncHooks(t)
	loadCloudPriceTableJSONFunc = func() (string, error) {
		return testCloudPriceTableJSON(), nil
	}

	primarySaveCalls := 0
	cloudSaveCalls := 0
	savePrimaryPricingOverridesFunc = func(overrides modelPricingOverrides) error {
		primarySaveCalls++
		return nil
	}
	saveCloudPricingOverridesFunc = func(overrides modelPricingOverrides) error {
		cloudSaveCalls++
		return nil
	}

	result, err := mps.SyncCloudPriceTable([]string{"manual-model"})
	if err != nil {
		t.Fatalf("SyncCloudPriceTable 失败: %v", err)
	}
	if result.CreatedModels != 1 {
		t.Fatalf("CreatedModels = %d, 期望 1", result.CreatedModels)
	}
	if result.UpdatedModels != 1 {
		t.Fatalf("UpdatedModels = %d, 期望 1", result.UpdatedModels)
	}
	if result.UnchangedModels != 0 {
		t.Fatalf("UnchangedModels = %d, 期望 0", result.UnchangedModels)
	}
	if len(result.SkippedManualModels) != 0 {
		t.Fatalf("SkippedManualModels = %v, 期望空", result.SkippedManualModels)
	}
	if primarySaveCalls != 1 {
		t.Fatalf("primarySaveCalls = %d, 期望 1", primarySaveCalls)
	}
	if cloudSaveCalls != 1 {
		t.Fatalf("cloudSaveCalls = %d, 期望 1", cloudSaveCalls)
	}
	if _, ok := mps.localOverrides.Pricing["manual-model"]; ok {
		t.Fatalf("manual-model 不应继续留在 localOverrides")
	}
	cloudPricing, ok := mps.cloudOverrides.Pricing["manual-model"]
	if !ok {
		t.Fatalf("manual-model 未迁移到 cloudOverrides")
	}
	if !floatAlmostEqual(cloudPricing.InputCostPerToken, 0.000001) {
		t.Fatalf("manual-model.InputCostPerToken = %f, 期望 %f", cloudPricing.InputCostPerToken, 0.000001)
	}
	if mps.cloudOverrides.Meta["manual-model"].Source != modelPricingSourceCloudSync {
		t.Fatalf("manual-model.Source = %q, 期望 %q", mps.cloudOverrides.Meta["manual-model"].Source, modelPricingSourceCloudSync)
	}
}

func newTestModelPricingService(t *testing.T) *ModelPricingService {
	t.Helper()
	defaults, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}
	mps := &ModelPricingService{
		defaults:       defaults,
		localOverrides: newEmptyModelPricingOverrides(),
		cloudOverrides: newEmptyModelPricingOverrides(),
		overrides:      newEmptyModelPricingOverrides(),
	}
	mps.rebuildLocked()
	return mps
}

func buildTestPricingEntry(input, output, cacheCreate, cacheRead, group float64) modelpricing.PricingEntry {
	return modelpricing.PricingEntry{
		InputCostPerToken:              input,
		HasInputCostPerToken:           true,
		OutputCostPerToken:             output,
		HasOutputCostPerToken:          true,
		CacheCreationInputTokenCost:    cacheCreate,
		HasCacheCreationInputTokenCost: true,
		CacheReadInputTokenCost:        cacheRead,
		HasCacheReadInputTokenCost:     true,
		GroupMultiplier:                group,
		HasGroupMultiplier:             true,
	}
}

func restoreCloudSyncHooks(t *testing.T) {
	t.Helper()
	originalLoad := loadCloudPriceTableJSONFunc
	originalSavePrimary := savePrimaryPricingOverridesFunc
	originalSaveCloud := saveCloudPricingOverridesFunc
	t.Cleanup(func() {
		loadCloudPriceTableJSONFunc = originalLoad
		savePrimaryPricingOverridesFunc = originalSavePrimary
		saveCloudPricingOverridesFunc = originalSaveCloud
	})
}

func testCloudPriceTableTOML() string {
	return `
[metadata]
version = "test"

[models."manual-model"]
display_name = "Manual Model"
mode = "chat"
input_cost_per_token = 0.000001
output_cost_per_token = 0.000002
cache_creation_input_token_cost = 0.00000125
cache_creation_input_token_cost_above_1hr = 0.0000015
cache_read_input_token_cost = 0.0000001

[models."cloud-model"]
display_name = "Cloud Model"
mode = "chat"
litellm_provider = "openai"
selected_pricing_provider = "openai"

[models."cloud-model".pricing."openai"]
input_cost_per_token = 0.000003
output_cost_per_token = 0.000006
output_cost_per_reasoning_token = 0.000009
cache_creation_input_token_cost = 0.00000375
cache_creation_input_token_cost_above_1hr = 0.0000045
cache_read_input_token_cost = 0.0000003
group_multiplier = 1.2
`
}
