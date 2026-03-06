package services

import (
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
