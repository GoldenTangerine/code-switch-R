package services

import (
	"testing"
	"time"
)

func TestParseClaudeModelPricingTable(t *testing.T) {
	pageHTML := `
<html><body>
  <table>
    <thead><tr><th>Model</th><th>Batch input</th><th>Batch output</th></tr></thead>
    <tbody><tr><td>Claude Sonnet 4.5</td><td>$1.5 / MTok</td><td>$7.5 / MTok</td></tr></tbody>
  </table>
  <table>
    <thead>
      <tr>
        <th>Model</th>
        <th>Base input</th>
        <th>5m Cache Writes</th>
        <th>1h Cache Writes</th>
        <th>Cache Hits &amp; Refreshes</th>
        <th>Output</th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td>Claude Opus 4.5</td>
        <td>$5 / MTok</td>
        <td>$6.25 / MTok</td>
        <td>$10 / MTok</td>
        <td>$0.50 / MTok</td>
        <td>$25 / MTok</td>
      </tr>
      <tr>
        <td>Claude Sonnet 3.7 (deprecated)</td>
        <td>$3 / MTok</td>
        <td>$3.75 / MTok</td>
        <td>$6 / MTok</td>
        <td>$0.30 / MTok</td>
        <td>$15 / MTok</td>
      </tr>
    </tbody>
  </table>
</body></html>`

	rows, err := parseClaudeModelPricingTable(pageHTML)
	if err != nil {
		t.Fatalf("parseClaudeModelPricingTable 返回错误: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, 期望 2", len(rows))
	}

	if rows[0].DisplayName != "Claude Opus 4.5" {
		t.Fatalf("rows[0].DisplayName = %q, 期望 %q", rows[0].DisplayName, "Claude Opus 4.5")
	}
	if rows[0].InputPerToken != 0.000005 {
		t.Fatalf("rows[0].InputPerToken = %f, 期望 %f", rows[0].InputPerToken, 0.000005)
	}
	if rows[0].CacheCreate1hPerToken != 0.00001 {
		t.Fatalf("rows[0].CacheCreate1hPerToken = %f, 期望 %f", rows[0].CacheCreate1hPerToken, 0.00001)
	}
	if rows[0].OutputPerToken != 0.000025 {
		t.Fatalf("rows[0].OutputPerToken = %f, 期望 %f", rows[0].OutputPerToken, 0.000025)
	}

	if rows[1].DisplayName != "Claude Sonnet 3.7" {
		t.Fatalf("rows[1].DisplayName = %q, 期望 %q", rows[1].DisplayName, "Claude Sonnet 3.7")
	}
}

func TestResolveClaudePricingTargetModels(t *testing.T) {
	targets, recognized := resolveClaudePricingTargetModels("Claude Sonnet 3.7")
	if !recognized {
		t.Fatalf("recognized = false, 期望 true")
	}
	expected := map[string]struct{}{
		"claude-sonnet-3-7":          {},
		"claude-3-7-sonnet-latest":   {},
		"claude-3-7-sonnet-20250219": {},
	}
	if len(targets) != len(expected) {
		t.Fatalf("len(targets) = %d, 期望 %d", len(targets), len(expected))
	}
	for _, model := range targets {
		if _, ok := expected[model]; !ok {
			t.Fatalf("未期望的目标模型: %s", model)
		}
	}
}

func TestResolveClaudePricingTargetModels_NormalizesOfficialDisplayName(t *testing.T) {
	targets, recognized := resolveClaudePricingTargetModels("Claude Opus 4.6")
	if !recognized {
		t.Fatalf("recognized = false, 期望 true")
	}
	expected := "claude-opus-4-6"
	found := false
	for _, model := range targets {
		if model == expected {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("未找到映射目标 %q，实际: %v", expected, targets)
	}
}

func TestParseClaudeModelPricingTable_UsesHeaderIndexInsteadOfFixedColumnPosition(t *testing.T) {
	pageHTML := `
<html><body>
  <table>
    <thead>
      <tr>
        <th>Output</th>
        <th>Model</th>
        <th>5m Cache Writes</th>
        <th>Cache Hits &amp; Refreshes</th>
        <th>Notes</th>
        <th>Base input</th>
        <th>1h Cache Writes</th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td>$25 / MTok</td>
        <td>Claude Opus 4.5</td>
        <td>$6.25 / MTok</td>
        <td>$0.50 / MTok</td>
        <td>stable</td>
        <td>$5 / MTok</td>
        <td>$10 / MTok</td>
      </tr>
    </tbody>
  </table>
</body></html>`

	rows, err := parseClaudeModelPricingTable(pageHTML)
	if err != nil {
		t.Fatalf("parseClaudeModelPricingTable 返回错误: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, 期望 1", len(rows))
	}
	row := rows[0]
	if row.DisplayName != "Claude Opus 4.5" {
		t.Fatalf("DisplayName = %q, 期望 %q", row.DisplayName, "Claude Opus 4.5")
	}
	if row.InputPerToken != 0.000005 {
		t.Fatalf("InputPerToken = %f, 期望 %f", row.InputPerToken, 0.000005)
	}
	if row.CacheCreate5mPerToken != 0.00000625 {
		t.Fatalf("CacheCreate5mPerToken = %f, 期望 %f", row.CacheCreate5mPerToken, 0.00000625)
	}
	if row.CacheCreate1hPerToken != 0.00001 {
		t.Fatalf("CacheCreate1hPerToken = %f, 期望 %f", row.CacheCreate1hPerToken, 0.00001)
	}
	if row.CacheReadPerToken != 0.0000005 {
		t.Fatalf("CacheReadPerToken = %f, 期望 %f", row.CacheReadPerToken, 0.0000005)
	}
	if row.OutputPerToken != 0.000025 {
		t.Fatalf("OutputPerToken = %f, 期望 %f", row.OutputPerToken, 0.000025)
	}
}

func TestParseClaudeModelPricingTable_SupportsTokenSuffixedHeaders(t *testing.T) {
	pageHTML := `
<html><body>
  <table>
    <thead>
      <tr>
        <th>Model</th>
        <th>Base Input Tokens</th>
        <th>5m Cache Writes</th>
        <th>1h Cache Writes</th>
        <th>Cache Hits &amp; Refreshes</th>
        <th>Output Tokens</th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td>Claude Opus 4.6</td>
        <td>$5 / MTok</td>
        <td>$6.25 / MTok</td>
        <td>$10 / MTok</td>
        <td>$0.50 / MTok</td>
        <td>$25 / MTok</td>
      </tr>
    </tbody>
  </table>
</body></html>`

	rows, err := parseClaudeModelPricingTable(pageHTML)
	if err != nil {
		t.Fatalf("parseClaudeModelPricingTable 返回错误: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, 期望 1", len(rows))
	}
	if rows[0].DisplayName != "Claude Opus 4.6" {
		t.Fatalf("rows[0].DisplayName = %q, 期望 %q", rows[0].DisplayName, "Claude Opus 4.6")
	}
	if rows[0].InputPerToken != 0.000005 {
		t.Fatalf("rows[0].InputPerToken = %f, 期望 %f", rows[0].InputPerToken, 0.000005)
	}
	if rows[0].OutputPerToken != 0.000025 {
		t.Fatalf("rows[0].OutputPerToken = %f, 期望 %f", rows[0].OutputPerToken, 0.000025)
	}
}

func TestResolveClaudePricingTargetModels_AllOfficialNamesMapped(t *testing.T) {
	for displayName, expectedTargets := range claudeOfficialDisplayNameToModels {
		targets, recognized := resolveClaudePricingTargetModels(displayName)
		if !recognized {
			t.Fatalf("displayName=%q recognized=false, 期望 true", displayName)
		}
		if len(targets) == 0 {
			t.Fatalf("displayName=%q 未返回任何目标模型", displayName)
		}
		for _, expected := range expectedTargets {
			found := false
			for _, got := range targets {
				if got == expected {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("displayName=%q 缺少目标模型 %q，实际: %v", displayName, expected, targets)
			}
		}
	}
}

func TestNormalizeClaudeFallbackModelKey(t *testing.T) {
	if got := normalizeClaudeFallbackModelKey(" Claude Unknown 9.9 "); got != "claude-unknown-9-9" {
		t.Fatalf("normalizeClaudeFallbackModelKey 返回 %q，期望 %q", got, "claude-unknown-9-9")
	}
	if got := normalizeClaudeFallbackModelKey("Claude Sonnet 3.7 (deprecated)"); got != "claude-sonnet-3-7" {
		t.Fatalf("normalizeClaudeFallbackModelKey 返回 %q，期望 %q", got, "claude-sonnet-3-7")
	}
	if got := normalizeClaudeFallbackModelKey("!!!"); got != "" {
		t.Fatalf("normalizeClaudeFallbackModelKey 返回 %q，期望空字符串", got)
	}
}

func TestBuildClaudeSyncPricingMap_UsesManualFallbackForUnmappedDisplayNames(t *testing.T) {
	rows := []claudeOfficialModelPricing{
		{DisplayName: "Claude Opus 4.5"},
		{DisplayName: "Claude Unknown 9.9"},
	}
	syncMap, unrecognized := buildClaudeSyncPricingMap(rows)
	if len(unrecognized) != 0 {
		t.Fatalf("unrecognized = %v, 期望空", unrecognized)
	}
	mappedTarget, ok := syncMap["claude-opus-4-5"]
	if !ok {
		t.Fatalf("未找到已映射模型 claude-opus-4-5")
	}
	if mappedTarget.Source != modelPricingSourceClaudeSync {
		t.Fatalf("mappedTarget.Source = %q, 期望 %q", mappedTarget.Source, modelPricingSourceClaudeSync)
	}
	fallbackTarget, ok := syncMap["claude-unknown-9-9"]
	if !ok {
		t.Fatalf("未找到 fallback 自定义模型 claude-unknown-9-9")
	}
	if fallbackTarget.Source != modelPricingSourceManual {
		t.Fatalf("fallbackTarget.Source = %q, 期望 %q", fallbackTarget.Source, modelPricingSourceManual)
	}
	if fallbackTarget.Pricing.DisplayName != "Claude Unknown 9.9" {
		t.Fatalf("fallbackTarget.Pricing.DisplayName = %q, 期望 %q", fallbackTarget.Pricing.DisplayName, "Claude Unknown 9.9")
	}
}

func TestBuildClaudeSyncPricingMap_ReturnsUnrecognizedDisplayNamesWhenFallbackKeyMissing(t *testing.T) {
	rows := []claudeOfficialModelPricing{
		{DisplayName: "!!!"},
	}
	syncMap, unrecognized := buildClaudeSyncPricingMap(rows)
	if len(syncMap) != 0 {
		t.Fatalf("len(syncMap) = %d, 期望 0", len(syncMap))
	}
	if len(unrecognized) != 1 || unrecognized[0] != "!!!" {
		t.Fatalf("unrecognized = %v, 期望 [!!!]", unrecognized)
	}
}

func TestBuildClaudePricingPreviewRows_ReturnsMappedTargetsAndPrices(t *testing.T) {
	rows := []claudeOfficialModelPricing{
		{
			DisplayName:           "Claude Opus 4.6",
			InputPerToken:         0.000005,
			CacheCreate5mPerToken: 0.00000625,
			CacheCreate1hPerToken: 0.00001,
			CacheReadPerToken:     0.0000005,
			OutputPerToken:        0.000025,
		},
	}

	previewRows, unrecognized := buildClaudePricingPreviewRows(rows)
	if len(unrecognized) != 0 {
		t.Fatalf("unrecognized = %v, 期望空", unrecognized)
	}
	if len(previewRows) != 1 {
		t.Fatalf("len(previewRows) = %d, 期望 1", len(previewRows))
	}

	row := previewRows[0]
	if row.DisplayName != "Claude Opus 4.6" {
		t.Fatalf("DisplayName = %q, 期望 %q", row.DisplayName, "Claude Opus 4.6")
	}
	if !row.IsRecognized {
		t.Fatalf("IsRecognized = false, 期望 true")
	}
	if len(row.TargetModels) == 0 || row.TargetModels[0] != "claude-opus-4-6" {
		t.Fatalf("TargetModels = %v, 期望包含 claude-opus-4-6", row.TargetModels)
	}
	if row.InputCostPerToken != 0.000005 {
		t.Fatalf("InputCostPerToken = %f, 期望 %f", row.InputCostPerToken, 0.000005)
	}
	if row.OutputCostPerToken != 0.000025 {
		t.Fatalf("OutputCostPerToken = %f, 期望 %f", row.OutputCostPerToken, 0.000025)
	}
	if row.CacheCreationInputTokenCost != 0.00000625 {
		t.Fatalf("CacheCreationInputTokenCost = %f, 期望 %f", row.CacheCreationInputTokenCost, 0.00000625)
	}
	if row.CacheReadInputTokenCost != 0.0000005 {
		t.Fatalf("CacheReadInputTokenCost = %f, 期望 %f", row.CacheReadInputTokenCost, 0.0000005)
	}
	if row.Ephemeral1hCostPerToken != 0.00001 {
		t.Fatalf("Ephemeral1hCostPerToken = %f, 期望 %f", row.Ephemeral1hCostPerToken, 0.00001)
	}
}

func TestBuildClaudePricingPreviewRows_ReturnsUnrecognizedDisplayNames(t *testing.T) {
	rows := []claudeOfficialModelPricing{
		{DisplayName: "Claude Unknown 9.9"},
		{DisplayName: "Claude Opus 4.6"},
	}

	previewRows, unrecognized := buildClaudePricingPreviewRows(rows)
	if len(previewRows) != 2 {
		t.Fatalf("len(previewRows) = %d, 期望 2", len(previewRows))
	}
	if len(unrecognized) != 0 {
		t.Fatalf("unrecognized = %v, 期望空", unrecognized)
	}

	foundFallback := false
	for _, row := range previewRows {
		if row.DisplayName != "Claude Unknown 9.9" {
			continue
		}
		foundFallback = true
		if row.IsRecognized {
			t.Fatalf("Claude Unknown 9.9 不应被视为已映射")
		}
		if len(row.TargetModels) != 1 || row.TargetModels[0] != "claude-unknown-9-9" {
			t.Fatalf("TargetModels = %v, 期望 [claude-unknown-9-9]", row.TargetModels)
		}
	}
	if !foundFallback {
		t.Fatalf("未找到 Claude Unknown 9.9 的预览行")
	}
}

func TestBuildClaudePricingPreviewRows_ReturnsUnrecognizedDisplayNamesWhenFallbackKeyMissing(t *testing.T) {
	rows := []claudeOfficialModelPricing{
		{DisplayName: "!!!"},
	}

	previewRows, unrecognized := buildClaudePricingPreviewRows(rows)
	if len(previewRows) != 1 {
		t.Fatalf("len(previewRows) = %d, 期望 1", len(previewRows))
	}
	if len(unrecognized) != 1 || unrecognized[0] != "!!!" {
		t.Fatalf("unrecognized = %v, 期望 [!!!]", unrecognized)
	}
	if len(previewRows[0].TargetModels) != 0 {
		t.Fatalf("TargetModels = %v, 期望空", previewRows[0].TargetModels)
	}
}

func TestConsumeClaudePricingPreviewCache_FreshAndOneShot(t *testing.T) {
	mps := &ModelPricingService{}
	now := time.Now().UTC()
	sourceRows := []claudeOfficialModelPricing{
		{DisplayName: "Claude Opus 4.6", InputPerToken: 0.000005},
	}
	mps.storeClaudePricingPreviewCache(sourceRows, now)

	cachedRows, ok := mps.consumeClaudePricingPreviewCache(now.Add(5 * time.Minute))
	if !ok {
		t.Fatalf("首次 consume 期望命中缓存")
	}
	if len(cachedRows) != 1 || cachedRows[0].DisplayName != "Claude Opus 4.6" {
		t.Fatalf("cachedRows = %v, 期望包含 Claude Opus 4.6", cachedRows)
	}

	// 验证只消费一次，避免后续同步误复用旧预览
	if secondRows, secondOk := mps.consumeClaudePricingPreviewCache(now.Add(6 * time.Minute)); secondOk || len(secondRows) != 0 {
		t.Fatalf("二次 consume 应为空，实际 secondOk=%v secondRows=%v", secondOk, secondRows)
	}
}

func TestConsumeClaudePricingPreviewCache_Expired(t *testing.T) {
	mps := &ModelPricingService{}
	now := time.Now().UTC()
	mps.storeClaudePricingPreviewCache(
		[]claudeOfficialModelPricing{{DisplayName: "Claude Opus 4.6"}},
		now.Add(-claudePreviewCacheTTL-time.Second),
	)

	cachedRows, ok := mps.consumeClaudePricingPreviewCache(now)
	if ok || len(cachedRows) != 0 {
		t.Fatalf("过期缓存应不可用，实际 ok=%v rows=%v", ok, cachedRows)
	}
}
