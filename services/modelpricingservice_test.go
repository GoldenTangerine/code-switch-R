package services

import (
	"math"
	"testing"

	modelpricing "codeswitch/resources/model-pricing"
)

func TestModelPricingServiceHasRenameConflictLocked(t *testing.T) {
	defaults, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}
	models := defaults.Models()
	if len(models) == 0 {
		t.Fatalf("默认模型列表为空")
	}

	existingModel := models[0]
	mps := &ModelPricingService{
		effective: defaults,
	}

	if !mps.hasRenameConflictLocked("custom-model", existingModel) {
		t.Fatalf("target = %q 时应识别为重命名冲突", existingModel)
	}
	if mps.hasRenameConflictLocked(existingModel, existingModel) {
		t.Fatalf("原模型名与目标模型名相同时不应判定冲突")
	}
	if mps.hasRenameConflictLocked(existingModel, existingModel+"-copy") {
		t.Fatalf("目标模型名不存在时不应判定冲突")
	}
}

func TestListModelPricing_IncludesBuiltinFreeAndZeroCacheModels(t *testing.T) {
	defaults, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}

	mps := &ModelPricingService{
		defaults:  defaults,
		effective: defaults,
		overrides: modelPricingOverrides{
			Pricing:     map[string]modelpricing.PricingEntry{},
			Ephemeral1h: map[string]float64{},
			Meta:        map[string]modelPricingMeta{},
		},
	}

	rows, err := mps.ListModelPricing()
	if err != nil {
		t.Fatalf("ListModelPricing 失败: %v", err)
	}

	findRow := func(model string) *ModelPricingRow {
		for idx := range rows {
			if rows[idx].Model == model {
				return &rows[idx]
			}
		}
		return nil
	}

	glm51 := findRow("GLM-5.1")
	if glm51 == nil {
		t.Fatalf("未找到 GLM-5.1 内置价格")
	}
	if glm51.CacheCreationInputTokenCost != 0 {
		t.Fatalf("GLM-5.1 CacheCreationInputTokenCost = %f, 期望 0", glm51.CacheCreationInputTokenCost)
	}
	if math.Abs(glm51.CacheReadInputTokenCost-0.0000002857) > 1e-12 {
		t.Fatalf("GLM-5.1 CacheReadInputTokenCost = %f, 期望 %f", glm51.CacheReadInputTokenCost, 0.0000002857)
	}
	if glm51.GroupMultiplier != 1 {
		t.Fatalf("GLM-5.1 GroupMultiplier = %f, 期望 1", glm51.GroupMultiplier)
	}

	freeModel := findRow("GLM-4.7-Flash")
	if freeModel == nil {
		t.Fatalf("未找到 GLM-4.7-Flash 免费模型")
	}
	if freeModel.InputCostPerToken != 0 || freeModel.OutputCostPerToken != 0 {
		t.Fatalf("GLM-4.7-Flash 免费模型价格异常: input=%f output=%f", freeModel.InputCostPerToken, freeModel.OutputCostPerToken)
	}
}

func TestApplyModelPricingRowToEntry_AllowsDefaultCacheBackfillWhenCacheFieldsAbsent(t *testing.T) {
	defaults, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}

	row := ModelPricingRow{
		Model:                          "manual-cache-absent",
		InputCostPerToken:              0.000008,
		OutputCostPerToken:             0.000028,
		OutputCostPerReasoningToken:    0,
		CacheCreationInputTokenCost:    0,
		HasCacheCreationInputTokenCost: false,
		CacheReadInputTokenCost:        0,
		HasCacheReadInputTokenCost:     false,
		Ephemeral1hCostPerToken:        0,
		GroupMultiplier:                1,
	}

	entry := applyModelPricingRowToEntry(modelpricing.PricingEntry{}, row)
	if entry.HasCacheCreationInputTokenCost {
		t.Fatalf("HasCacheCreationInputTokenCost = true, 期望 false")
	}
	if entry.HasCacheReadInputTokenCost {
		t.Fatalf("HasCacheReadInputTokenCost = true, 期望 false")
	}

	svc := defaults.Clone()
	svc.ApplyOverrides(map[string]modelpricing.PricingEntry{
		row.Model: entry,
	}, nil)

	resolved, ok := svc.PricingEntryExact(row.Model)
	if !ok {
		t.Fatalf("未找到覆盖后的模型 %q", row.Model)
	}

	wantCreate := row.InputCostPerToken * 1.25
	wantRead := row.InputCostPerToken * 0.1
	if math.Abs(resolved.CacheCreationInputTokenCost-wantCreate) > 1e-12 {
		t.Fatalf("CacheCreationInputTokenCost = %f, 期望 %f", resolved.CacheCreationInputTokenCost, wantCreate)
	}
	if math.Abs(resolved.CacheReadInputTokenCost-wantRead) > 1e-12 {
		t.Fatalf("CacheReadInputTokenCost = %f, 期望 %f", resolved.CacheReadInputTokenCost, wantRead)
	}
	if resolved.HasCacheCreationInputTokenCost {
		t.Fatalf("HasCacheCreationInputTokenCost = true, 期望 false")
	}
	if resolved.HasCacheReadInputTokenCost {
		t.Fatalf("HasCacheReadInputTokenCost = true, 期望 false")
	}
}

func TestApplyModelPricingRowToEntry_PreservesExplicitZeroCachePricing(t *testing.T) {
	defaults, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}

	row := ModelPricingRow{
		Model:                          "manual-cache-explicit-zero",
		InputCostPerToken:              0.000008,
		OutputCostPerToken:             0.000028,
		OutputCostPerReasoningToken:    0,
		CacheCreationInputTokenCost:    0,
		HasCacheCreationInputTokenCost: true,
		CacheReadInputTokenCost:        0,
		HasCacheReadInputTokenCost:     true,
		Ephemeral1hCostPerToken:        0,
		GroupMultiplier:                1,
	}

	entry := applyModelPricingRowToEntry(modelpricing.PricingEntry{}, row)
	if !entry.HasCacheCreationInputTokenCost {
		t.Fatalf("HasCacheCreationInputTokenCost = false, 期望 true")
	}
	if !entry.HasCacheReadInputTokenCost {
		t.Fatalf("HasCacheReadInputTokenCost = false, 期望 true")
	}

	svc := defaults.Clone()
	svc.ApplyOverrides(map[string]modelpricing.PricingEntry{
		row.Model: entry,
	}, nil)

	resolved, ok := svc.PricingEntryExact(row.Model)
	if !ok {
		t.Fatalf("未找到覆盖后的模型 %q", row.Model)
	}
	if resolved.CacheCreationInputTokenCost != 0 {
		t.Fatalf("CacheCreationInputTokenCost = %f, 期望 0", resolved.CacheCreationInputTokenCost)
	}
	if resolved.CacheReadInputTokenCost != 0 {
		t.Fatalf("CacheReadInputTokenCost = %f, 期望 0", resolved.CacheReadInputTokenCost)
	}
	if !resolved.HasCacheCreationInputTokenCost {
		t.Fatalf("HasCacheCreationInputTokenCost = false, 期望 true")
	}
	if !resolved.HasCacheReadInputTokenCost {
		t.Fatalf("HasCacheReadInputTokenCost = false, 期望 true")
	}
}

func TestListModelPricing_PropagatesCachePresenceFlagsForOverrides(t *testing.T) {
	defaults, err := modelpricing.NewService()
	if err != nil {
		t.Fatalf("初始化价格服务失败: %v", err)
	}

	absentRow := ModelPricingRow{
		Model:                          "manual-cache-row-absent",
		InputCostPerToken:              0.000008,
		OutputCostPerToken:             0.000028,
		CacheCreationInputTokenCost:    0,
		HasCacheCreationInputTokenCost: false,
		CacheReadInputTokenCost:        0,
		HasCacheReadInputTokenCost:     false,
		GroupMultiplier:                1,
	}
	explicitRow := ModelPricingRow{
		Model:                          "manual-cache-row-explicit-zero",
		InputCostPerToken:              0.000008,
		OutputCostPerToken:             0.000028,
		CacheCreationInputTokenCost:    0,
		HasCacheCreationInputTokenCost: true,
		CacheReadInputTokenCost:        0,
		HasCacheReadInputTokenCost:     true,
		GroupMultiplier:                1,
	}

	overrides := map[string]modelpricing.PricingEntry{
		absentRow.Model:   applyModelPricingRowToEntry(modelpricing.PricingEntry{}, absentRow),
		explicitRow.Model: applyModelPricingRowToEntry(modelpricing.PricingEntry{}, explicitRow),
	}
	effective := defaults.Clone()
	effective.ApplyOverrides(overrides, nil)

	mps := &ModelPricingService{
		defaults:  defaults,
		effective: effective,
		overrides: modelPricingOverrides{
			Pricing:     overrides,
			Ephemeral1h: map[string]float64{},
			Meta:        map[string]modelPricingMeta{},
		},
	}

	rows, err := mps.ListModelPricing()
	if err != nil {
		t.Fatalf("ListModelPricing 失败: %v", err)
	}

	findRow := func(model string) *ModelPricingRow {
		for idx := range rows {
			if rows[idx].Model == model {
				return &rows[idx]
			}
		}
		return nil
	}

	absent := findRow(absentRow.Model)
	if absent == nil {
		t.Fatalf("未找到 absence 覆盖模型 %q", absentRow.Model)
	}
	if absent.HasCacheCreationInputTokenCost {
		t.Fatalf("absence 行 HasCacheCreationInputTokenCost = true, 期望 false")
	}
	if absent.HasCacheReadInputTokenCost {
		t.Fatalf("absence 行 HasCacheReadInputTokenCost = true, 期望 false")
	}
	if absent.CacheCreationInputTokenCost <= 0 {
		t.Fatalf("absence 行 CacheCreationInputTokenCost = %f, 期望自动补齐后的正数", absent.CacheCreationInputTokenCost)
	}
	if absent.CacheReadInputTokenCost <= 0 {
		t.Fatalf("absence 行 CacheReadInputTokenCost = %f, 期望自动补齐后的正数", absent.CacheReadInputTokenCost)
	}

	explicit := findRow(explicitRow.Model)
	if explicit == nil {
		t.Fatalf("未找到 explicit zero 覆盖模型 %q", explicitRow.Model)
	}
	if !explicit.HasCacheCreationInputTokenCost {
		t.Fatalf("explicit 行 HasCacheCreationInputTokenCost = false, 期望 true")
	}
	if !explicit.HasCacheReadInputTokenCost {
		t.Fatalf("explicit 行 HasCacheReadInputTokenCost = false, 期望 true")
	}
	if explicit.CacheCreationInputTokenCost != 0 {
		t.Fatalf("explicit 行 CacheCreationInputTokenCost = %f, 期望 0", explicit.CacheCreationInputTokenCost)
	}
	if explicit.CacheReadInputTokenCost != 0 {
		t.Fatalf("explicit 行 CacheReadInputTokenCost = %f, 期望 0", explicit.CacheReadInputTokenCost)
	}
}
