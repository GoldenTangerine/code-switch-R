/**
 * @name: 云端价格同步
 * @Descripttion: 下载云端价格表并处理分层覆盖与冲突预览。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-07 11:10:24
 * @LastEditTime: 2026-09-07 11:10:24
 * @FilePath: services/modelpricing_cloud_sync.go
 */
package services

import (
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	modelpricing "codeswitch/resources/model-pricing"

	"github.com/pelletier/go-toml/v2"
)

const (
	cloudPriceTableURL      = "https://cch-plus.com/pricing/v1/models.json"
	cloudPriceTableMaxBytes = 64 * 1024 * 1024
	cloudPreviewCacheTTL    = 30 * time.Minute
)

var (
	loadCloudPriceTableJSONFunc     = fetchCloudPriceTableJSON
	savePrimaryPricingOverridesFunc = saveModelPricingOverridesToDB
	saveCloudPricingOverridesFunc   = saveCloudModelPricingOverridesToDB
)

type cloudPriceTable struct {
	Metadata map[string]any
	Models   map[string]map[string]any
}

type cloudSyncPricingEntry struct {
	MetadataFields       map[string]bool
	Model                string
	DisplayName          string
	Mode                 string
	LiteLLMProvider      string
	Pricing              modelpricing.PricingEntry
	ExplicitEphemeral1h  float64
	HasExplicitEphemeral bool
}

type cloudPricingPreviewCache struct {
	entries   map[string]cloudSyncPricingEntry
	fetchedAt time.Time
}

type CloudPriceTableSyncConflictResult struct {
	Provider  string                           `json:"provider"`
	FetchedAt string                           `json:"fetched_at"`
	Conflicts []CloudPriceTableSyncConflictRow `json:"conflicts"`
}

type CloudPriceTableSyncConflictRow struct {
	Model           string                         `json:"model"`
	DisplayName     string                         `json:"display_name,omitempty"`
	LiteLLMProvider string                         `json:"litellm_provider,omitempty"`
	Mode            string                         `json:"mode,omitempty"`
	Current         CloudPriceTableConflictPricing `json:"current"`
	Incoming        CloudPriceTableConflictPricing `json:"incoming"`
}

type CloudPriceTableConflictPricing struct {
	CloudPricing                *modelpricing.CloudPricingRules `json:"cloud_pricing,omitempty"`
	InputCostPerToken           float64                         `json:"input_cost_per_token"`
	OutputCostPerToken          float64                         `json:"output_cost_per_token"`
	OutputCostPerReasoningToken float64                         `json:"output_cost_per_reasoning_token"`
	CacheCreationInputTokenCost float64                         `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost     float64                         `json:"cache_read_input_token_cost"`
	Ephemeral1hCostPerToken     float64                         `json:"ephemeral_1h_cost_per_token"`
	GroupMultiplier             float64                         `json:"group_multiplier"`
}

func (mps *ModelPricingService) PreviewCloudPriceTableSyncConflicts() (CloudPriceTableSyncConflictResult, error) {
	now := time.Now().UTC()
	result := CloudPriceTableSyncConflictResult{
		Provider:  "cloud",
		FetchedAt: now.Format(time.RFC3339),
	}
	if mps == nil {
		return result, fmt.Errorf("nil pricing service")
	}

	entries, err := loadCloudSyncPricingEntries()
	if err != nil {
		return result, err
	}
	mps.storeCloudPricingPreviewCache(entries, now)

	models := make([]string, 0, len(entries))
	for model := range entries {
		models = append(models, model)
	}
	sort.Strings(models)

	mps.mu.RLock()
	defer mps.mu.RUnlock()

	conflicts := make([]CloudPriceTableSyncConflictRow, 0, len(models))
	for _, model := range models {
		entry := entries[model]
		if !mps.hasManualOverrideLocked(model) {
			continue
		}

		currentEntry, ok := mps.effective.PricingEntryExact(model)
		if !ok {
			continue
		}
		currentExplicit1h, _ := mps.effective.ExplicitEphemeral1hCostPerToken(model)
		conflicts = append(conflicts, CloudPriceTableSyncConflictRow{
			Model:           model,
			DisplayName:     entry.DisplayName,
			LiteLLMProvider: entry.LiteLLMProvider,
			Mode:            entry.Mode,
			Current:         buildCloudConflictPricing(currentEntry, currentExplicit1h),
			Incoming:        buildCloudConflictPricing(entry.Pricing, resolveExplicitEphemeral1hValue(entry)),
		})
	}

	result.Conflicts = conflicts
	return result, nil
}

func (mps *ModelPricingService) SyncCloudPriceTable(overwriteManualModels []string) (ModelPricingSyncResult, error) {
	result := ModelPricingSyncResult{
		Provider: "cloud",
		SyncedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if mps == nil {
		return result, fmt.Errorf("nil pricing service")
	}

	entries, err := mps.loadCloudSyncPricingEntriesForApply()
	if err != nil {
		return result, err
	}
	if len(entries) == 0 {
		return result, fmt.Errorf("云端价格表中没有可同步的 token 定价模型")
	}

	overwriteSet := make(map[string]struct{}, len(overwriteManualModels))
	for _, model := range overwriteManualModels {
		value := strings.TrimSpace(model)
		if value == "" {
			continue
		}
		overwriteSet[value] = struct{}{}
	}

	mps.mu.Lock()
	notifyRouting := false
	defer func() {
		mps.mu.Unlock()
		if notifyRouting {
			mps.notifyClaudeModelRoutingChanged()
		}
	}()

	newLocalOverrides := cloneModelPricingOverrides(mps.localOverrides)
	newCloudOverrides := cloneModelPricingOverrides(mps.cloudOverrides)

	models := make([]string, 0, len(entries))
	for model := range entries {
		models = append(models, model)
	}
	sort.Strings(models)

	localChanged := false
	cloudChanged := false

	for _, model := range models {
		entry := entries[model]
		result.TotalModels++

		currentSource := mps.resolvePrimarySourceLocked(newLocalOverrides, model)
		oldEntry, oldHasEntry := resolvePricingEntryFromLayers(newLocalOverrides, newCloudOverrides, mps.defaults, model)
		oldExplicit1h, oldHasExplicit1h := resolveExplicitEphemeral1hFromLayers(newLocalOverrides, newCloudOverrides, mps.defaults, model)
		oldSource := resolvePricingSourceFromLayers(newLocalOverrides, newCloudOverrides, model)
		hadAnyLayerOverride := hasOverrideInLayer(newLocalOverrides, model) || hasOverrideInLayer(newCloudOverrides, model)

		if currentSource == modelPricingSourceClaudeSync {
			result.UnchangedModels++
			continue
		}
		if currentSource == modelPricingSourceManual {
			if _, ok := overwriteSet[model]; !ok {
				result.UnchangedModels++
				result.SkippedManualModels = append(result.SkippedManualModels, model)
				continue
			}
			if deleteOverrideFromLayer(&newLocalOverrides, model) {
				localChanged = true
			}
		}

		baseEntry, _ := resolvePricingEntryFromLayers(newLocalOverrides, newCloudOverrides, mps.defaults, model)
		entry.Pricing = mergeCloudPricingEntry(baseEntry, entry)

		if applyCloudEntryToOverrides(&newCloudOverrides, entry, result.SyncedAt) {
			cloudChanged = true
		}

		changed := !oldHasEntry ||
			!modelPricingEntriesEquivalent(oldEntry, entry.Pricing) ||
			oldHasExplicit1h != entry.HasExplicitEphemeral ||
			(oldHasExplicit1h && !floatAlmostEqual(oldExplicit1h, resolveExplicitEphemeral1hValue(entry))) ||
			oldSource != modelPricingSourceCloudSync
		if !changed {
			result.UnchangedModels++
			continue
		}

		result.ChangedModels++
		if hadAnyLayerOverride {
			result.UpdatedModels++
		} else {
			result.CreatedModels++
		}
	}

	if !localChanged && !cloudChanged {
		return result, nil
	}

	if localChanged {
		if err := savePrimaryPricingOverridesFunc(newLocalOverrides); err != nil {
			return result, err
		}
	}
	if cloudChanged {
		if err := saveCloudPricingOverridesFunc(newCloudOverrides); err != nil {
			return result, err
		}
	}

	mps.localOverrides = newLocalOverrides
	mps.cloudOverrides = newCloudOverrides
	mps.rebuildLocked()
	notifyRouting = true
	return result, nil
}

func loadCloudSyncPricingEntries() (map[string]cloudSyncPricingEntry, error) {
	jsonText, err := loadCloudPriceTableJSONFunc()
	if err != nil {
		return nil, err
	}

	return parseCloudPriceTableJSON(jsonText)
}

func (mps *ModelPricingService) loadCloudSyncPricingEntriesForApply() (map[string]cloudSyncPricingEntry, error) {
	if cachedEntries, ok := mps.consumeCloudPricingPreviewCache(time.Now().UTC()); ok {
		return cachedEntries, nil
	}
	return loadCloudSyncPricingEntries()
}

func (mps *ModelPricingService) storeCloudPricingPreviewCache(entries map[string]cloudSyncPricingEntry, fetchedAt time.Time) {
	if mps == nil {
		return
	}
	mps.mu.Lock()
	mps.cloudPricingPreview = cloudPricingPreviewCache{
		entries:   cloneCloudSyncPricingEntries(entries),
		fetchedAt: fetchedAt,
	}
	mps.mu.Unlock()
}

func (mps *ModelPricingService) consumeCloudPricingPreviewCache(now time.Time) (map[string]cloudSyncPricingEntry, bool) {
	if mps == nil {
		return nil, false
	}

	mps.mu.Lock()
	defer mps.mu.Unlock()

	cache := mps.cloudPricingPreview
	if len(cache.entries) == 0 {
		return nil, false
	}
	if !cache.fetchedAt.IsZero() && now.Sub(cache.fetchedAt) > cloudPreviewCacheTTL {
		mps.cloudPricingPreview = cloudPricingPreviewCache{}
		return nil, false
	}

	mps.cloudPricingPreview = cloudPricingPreviewCache{}
	return cloneCloudSyncPricingEntries(cache.entries), true
}

func fetchCloudPriceTableJSON() (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, cloudPriceTableURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "code-switch-r/cloud-price-sync")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("云端价格表拉取失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, cloudPriceTableMaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("云端价格表拉取失败：读取响应失败: %w", err)
	}
	if len(body) > cloudPriceTableMaxBytes {
		return "", fmt.Errorf("云端价格表拉取失败：内容超过 64 MiB 上限")
	}

	if err := validateCloudPriceTableFinalURL(resp.Request.URL, cloudPriceTableURL); err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("云端价格表拉取失败：HTTP %d", resp.StatusCode)
	}

	text := strings.TrimSpace(string(body))
	if text == "" {
		return "", fmt.Errorf("云端价格表拉取失败：内容为空")
	}
	return text, nil
}

func validateCloudPriceTableFinalURL(finalURL *url.URL, rawExpectedURL string) error {
	if finalURL == nil {
		return nil
	}

	expectedURL, err := url.Parse(rawExpectedURL)
	if err != nil {
		return fmt.Errorf("云端价格表地址配置无效：%w", err)
	}

	if !strings.EqualFold(finalURL.Scheme, expectedURL.Scheme) ||
		!strings.EqualFold(finalURL.Host, expectedURL.Host) ||
		finalURL.EscapedPath() != expectedURL.EscapedPath() {
		return fmt.Errorf("云端价格表拉取失败：重定向到非预期地址")
	}
	return nil
}

func parseCloudPriceTableTOML(tomlText string) (cloudPriceTable, error) {
	var raw map[string]any
	if strings.TrimSpace(tomlText) == "" {
		return cloudPriceTable{}, fmt.Errorf("价格表格式无效：内容为空")
	}
	if err := toml.Unmarshal([]byte(tomlText), &raw); err != nil {
		return cloudPriceTable{}, fmt.Errorf("价格表 TOML 解析失败: %w", err)
	}

	modelsRaw, ok := asStringMap(raw["models"])
	if !ok {
		return cloudPriceTable{}, fmt.Errorf("价格表格式无效：缺少 models 表")
	}

	models := make(map[string]map[string]any, len(modelsRaw))
	for modelName, value := range modelsRaw {
		record, ok := asStringMap(value)
		if !ok {
			continue
		}
		models[modelName] = record
	}
	if len(models) == 0 {
		return cloudPriceTable{}, fmt.Errorf("价格表格式无效：models 为空")
	}

	metadata, _ := asStringMap(raw["metadata"])
	return cloudPriceTable{
		Metadata: metadata,
		Models:   models,
	}, nil
}

func buildCloudSyncPricingMap(rows map[string]map[string]any) map[string]cloudSyncPricingEntry {
	result := make(map[string]cloudSyncPricingEntry)
	for modelName, rawRecord := range rows {
		model := strings.TrimSpace(modelName)
		if model == "" {
			continue
		}

		entry, ok := buildCloudSyncPricingEntry(model, rawRecord)
		if !ok {
			continue
		}
		result[model] = entry
	}
	return result
}

func buildCloudSyncPricingEntry(model string, rawRecord map[string]any) (cloudSyncPricingEntry, bool) {
	candidates := resolveCloudPricingCandidateMaps(rawRecord)

	pricing := modelpricing.PricingEntry{}
	pricing.InputCostPerToken, pricing.HasInputCostPerToken = resolveCloudNumericField(candidates, "input_cost_per_token")
	pricing.OutputCostPerToken, pricing.HasOutputCostPerToken = resolveCloudNumericField(candidates, "output_cost_per_token")
	pricing.OutputCostPerReasoningToken, pricing.HasOutputCostPerReasoningToken = resolveCloudNumericField(candidates, "output_cost_per_reasoning_token")
	pricing.CacheCreationInputTokenCost, pricing.HasCacheCreationInputTokenCost = resolveCloudNumericField(candidates, "cache_creation_input_token_cost")
	pricing.CacheReadInputTokenCost, pricing.HasCacheReadInputTokenCost = resolveCloudNumericField(candidates, "cache_read_input_token_cost")
	pricing.GroupMultiplier, pricing.HasGroupMultiplier = resolveCloudNumericField(candidates, "group_multiplier")

	explicitEphemeral1h, hasExplicitEphemeral := resolveCloudNumericField(candidates, "cache_creation_input_token_cost_above_1hr")
	if hasExplicitEphemeral {
		pricing.CacheCreationInputTokenCostAbove1Hr = explicitEphemeral1h
	}

	if !pricing.HasInputCostPerToken &&
		!pricing.HasOutputCostPerToken &&
		!pricing.HasOutputCostPerReasoningToken &&
		!pricing.HasCacheCreationInputTokenCost &&
		!pricing.HasCacheReadInputTokenCost &&
		!hasExplicitEphemeral {
		return cloudSyncPricingEntry{}, false
	}

	return cloudSyncPricingEntry{
		Model:                model,
		DisplayName:          strings.TrimSpace(stringValue(rawRecord["display_name"])),
		Mode:                 strings.TrimSpace(stringValue(rawRecord["mode"])),
		LiteLLMProvider:      strings.TrimSpace(stringValue(rawRecord["litellm_provider"])),
		Pricing:              pricing,
		ExplicitEphemeral1h:  explicitEphemeral1h,
		HasExplicitEphemeral: hasExplicitEphemeral,
	}, true
}

func resolveCloudPricingCandidateMaps(rawRecord map[string]any) []map[string]any {
	candidates := make([]map[string]any, 0, 8)
	if rawRecord == nil {
		return candidates
	}
	candidates = append(candidates, rawRecord)

	pricingRaw, ok := asStringMap(rawRecord["pricing"])
	if !ok || len(pricingRaw) == 0 {
		return candidates
	}

	addedKeys := make(map[string]struct{}, len(pricingRaw))
	appendPricingKey := func(key string) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		if _, exists := addedKeys[key]; exists {
			return
		}
		record, ok := asStringMap(pricingRaw[key])
		if !ok {
			return
		}
		addedKeys[key] = struct{}{}
		candidates = append(candidates, record)
	}
	appendPricingByPrefix := func(prefix string) {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			return
		}
		keys := make([]string, 0, len(pricingRaw))
		for key := range pricingRaw {
			if key == prefix || strings.HasPrefix(key, prefix+"/") {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			appendPricingKey(key)
		}
	}

	selectedPricingProvider := strings.TrimSpace(stringValue(rawRecord["selected_pricing_provider"]))
	appendPricingKey(selectedPricingProvider)
	appendPricingByPrefix(selectedPricingProvider)

	litellmProvider := strings.TrimSpace(stringValue(rawRecord["litellm_provider"]))
	appendPricingKey(litellmProvider)
	appendPricingByPrefix(litellmProvider)

	remainingKeys := make([]string, 0, len(pricingRaw))
	for key := range pricingRaw {
		remainingKeys = append(remainingKeys, key)
	}
	sort.Strings(remainingKeys)
	for _, key := range remainingKeys {
		appendPricingKey(key)
	}

	return candidates
}

func resolveCloudNumericField(candidates []map[string]any, key string) (float64, bool) {
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		value, exists := candidate[key]
		if !exists {
			continue
		}
		number, ok := numberValue(value)
		if ok {
			return number, true
		}
	}
	return 0, false
}

func asStringMap(value any) (map[string]any, bool) {
	typed, ok := value.(map[string]any)
	return typed, ok
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", value)
	}
}

func cloneCloudSyncPricingEntries(entries map[string]cloudSyncPricingEntry) map[string]cloudSyncPricingEntry {
	if len(entries) == 0 {
		return nil
	}
	cloned := make(map[string]cloudSyncPricingEntry, len(entries))
	for key, value := range entries {
		value.MetadataFields = maps.Clone(value.MetadataFields)
		value.Pricing.CloudPricing = cloneCloudPricingRules(value.Pricing.CloudPricing)
		cloned[key] = value
	}
	return cloned
}

func buildCloudConflictPricing(entry modelpricing.PricingEntry, explicitEphemeral1h float64) CloudPriceTableConflictPricing {
	normalized := normalizeComparableModelPricingEntry(entry)
	return CloudPriceTableConflictPricing{
		CloudPricing:                cloneCloudPricingRules(entry.CloudPricing),
		InputCostPerToken:           normalized.InputCostPerToken,
		OutputCostPerToken:          normalized.OutputCostPerToken,
		OutputCostPerReasoningToken: normalized.OutputCostPerReasoningToken,
		CacheCreationInputTokenCost: normalized.CacheCreationInputTokenCost,
		CacheReadInputTokenCost:     normalized.CacheReadInputTokenCost,
		Ephemeral1hCostPerToken:     explicitEphemeral1h,
		GroupMultiplier:             effectiveModelPricingGroupMultiplier(normalized),
	}
}

func resolveExplicitEphemeral1hValue(entry cloudSyncPricingEntry) float64 {
	if entry.HasExplicitEphemeral {
		return entry.ExplicitEphemeral1h
	}
	return 0
}

func normalizeComparableModelPricingEntry(entry modelpricing.PricingEntry) modelpricing.PricingEntry {
	normalized := entry
	if normalized.CloudPricing == nil && !normalized.HasCacheCreationInputTokenCost &&
		normalized.CacheCreationInputTokenCost == 0 &&
		normalized.InputCostPerToken > 0 {
		normalized.CacheCreationInputTokenCost = normalized.InputCostPerToken * 1.25
	}
	if normalized.CloudPricing == nil && !normalized.HasCacheReadInputTokenCost &&
		normalized.CacheReadInputTokenCost == 0 &&
		normalized.InputCostPerToken > 0 {
		normalized.CacheReadInputTokenCost = normalized.InputCostPerToken * 0.1
	}
	if !normalized.HasGroupMultiplier && normalized.GroupMultiplier != 0 {
		normalized.HasGroupMultiplier = true
	}
	if !normalized.HasGroupMultiplier {
		normalized.GroupMultiplier = 1
	}
	return normalized
}

func modelPricingEntriesEquivalent(a modelpricing.PricingEntry, b modelpricing.PricingEntry) bool {
	left := normalizeComparableModelPricingEntry(a)
	right := normalizeComparableModelPricingEntry(b)
	return left.CloudPricing.Equal(right.CloudPricing) &&
		left.MaxInputTokens == right.MaxInputTokens && left.MaxTokens == right.MaxTokens &&
		left.SupportsComputerUse == right.SupportsComputerUse && left.SupportsFunctionCalling == right.SupportsFunctionCalling &&
		left.SupportsPDFInput == right.SupportsPDFInput && left.SupportsPromptCaching == right.SupportsPromptCaching &&
		left.SupportsReasoning == right.SupportsReasoning && left.SupportsResponseSchema == right.SupportsResponseSchema && left.SupportsVision == right.SupportsVision &&
		floatAlmostEqual(left.InputCostPerToken, right.InputCostPerToken) &&
		left.HasInputCostPerToken == right.HasInputCostPerToken &&
		floatAlmostEqual(left.OutputCostPerToken, right.OutputCostPerToken) &&
		left.HasOutputCostPerToken == right.HasOutputCostPerToken &&
		floatAlmostEqual(left.OutputCostPerReasoningToken, right.OutputCostPerReasoningToken) &&
		left.HasOutputCostPerReasoningToken == right.HasOutputCostPerReasoningToken &&
		floatAlmostEqual(left.CacheCreationInputTokenCost, right.CacheCreationInputTokenCost) &&
		left.HasCacheCreationInputTokenCost == right.HasCacheCreationInputTokenCost &&
		floatAlmostEqual(left.CacheReadInputTokenCost, right.CacheReadInputTokenCost) &&
		left.HasCacheReadInputTokenCost == right.HasCacheReadInputTokenCost &&
		floatAlmostEqual(left.CacheCreationInputTokenCostAbove1Hr, right.CacheCreationInputTokenCostAbove1Hr) &&
		floatAlmostEqual(left.GroupMultiplier, right.GroupMultiplier) &&
		left.HasGroupMultiplier == right.HasGroupMultiplier
}

func mergeCloudPricingEntry(base modelpricing.PricingEntry, incoming cloudSyncPricingEntry) modelpricing.PricingEntry {
	if incoming.Pricing.CloudPricing != nil {
		merged := incoming.Pricing
		merged.CloudPricing = cloneCloudPricingRules(incoming.Pricing.CloudPricing)
		if !incoming.MetadataFields["max_input_tokens"] {
			merged.MaxInputTokens = base.MaxInputTokens
		}
		if !incoming.MetadataFields["max_tokens"] {
			merged.MaxTokens = base.MaxTokens
		}
		if !incoming.MetadataFields["computer_use"] {
			merged.SupportsComputerUse = base.SupportsComputerUse
		}
		if !incoming.MetadataFields["function_calling"] {
			merged.SupportsFunctionCalling = base.SupportsFunctionCalling
		}
		if !incoming.MetadataFields["pdf_input"] {
			merged.SupportsPDFInput = base.SupportsPDFInput
		}
		if !incoming.MetadataFields["prompt_caching"] {
			merged.SupportsPromptCaching = base.SupportsPromptCaching
		}
		if !incoming.MetadataFields["reasoning"] {
			merged.SupportsReasoning = base.SupportsReasoning
		}
		if !incoming.MetadataFields["structured_output"] {
			merged.SupportsResponseSchema = base.SupportsResponseSchema
		}
		if !incoming.MetadataFields["vision"] {
			merged.SupportsVision = base.SupportsVision
		}
		return merged
	}
	merged := base
	if incoming.Pricing.HasInputCostPerToken {
		merged.InputCostPerToken = incoming.Pricing.InputCostPerToken
		merged.HasInputCostPerToken = true
	}
	if incoming.Pricing.HasOutputCostPerToken {
		merged.OutputCostPerToken = incoming.Pricing.OutputCostPerToken
		merged.HasOutputCostPerToken = true
	}
	if incoming.Pricing.HasOutputCostPerReasoningToken {
		merged.OutputCostPerReasoningToken = incoming.Pricing.OutputCostPerReasoningToken
		merged.HasOutputCostPerReasoningToken = true
	}
	if incoming.Pricing.HasCacheCreationInputTokenCost {
		merged.CacheCreationInputTokenCost = incoming.Pricing.CacheCreationInputTokenCost
		merged.HasCacheCreationInputTokenCost = true
	}
	if incoming.Pricing.HasCacheReadInputTokenCost {
		merged.CacheReadInputTokenCost = incoming.Pricing.CacheReadInputTokenCost
		merged.HasCacheReadInputTokenCost = true
	}
	if incoming.Pricing.HasGroupMultiplier {
		merged.GroupMultiplier = incoming.Pricing.GroupMultiplier
		merged.HasGroupMultiplier = true
	}
	if incoming.HasExplicitEphemeral {
		merged.CacheCreationInputTokenCostAbove1Hr = incoming.ExplicitEphemeral1h
	}
	return merged
}

func (mps *ModelPricingService) hasManualOverrideLocked(model string) bool {
	if mps == nil {
		return false
	}
	return mps.resolvePrimarySourceLocked(mps.localOverrides, model) == modelPricingSourceManual
}

func (mps *ModelPricingService) resolvePrimarySourceLocked(overrides modelPricingOverrides, model string) string {
	return resolveModelPricingSource(overrides.Meta[model], hasOverrideInLayer(overrides, model))
}

func hasOverrideInLayer(overrides modelPricingOverrides, model string) bool {
	if _, ok := overrides.Pricing[model]; ok {
		return true
	}
	if _, ok := overrides.Ephemeral1h[model]; ok {
		return true
	}
	if _, ok := overrides.Meta[model]; ok {
		return true
	}
	return false
}

func deleteOverrideFromLayer(overrides *modelPricingOverrides, model string) bool {
	if overrides == nil {
		return false
	}
	changed := false
	if _, ok := overrides.Pricing[model]; ok {
		delete(overrides.Pricing, model)
		changed = true
	}
	if _, ok := overrides.Ephemeral1h[model]; ok {
		delete(overrides.Ephemeral1h, model)
		changed = true
	}
	if _, ok := overrides.Meta[model]; ok {
		delete(overrides.Meta, model)
		changed = true
	}
	return changed
}

func applyCloudEntryToOverrides(overrides *modelPricingOverrides, entry cloudSyncPricingEntry, syncedAt string) bool {
	if overrides == nil {
		return false
	}
	ensureModelPricingOverridesInitialized(overrides)

	existingPricing, hadPricing := overrides.Pricing[entry.Model]
	existingExplicit1h, hadExplicit1h := overrides.Ephemeral1h[entry.Model]
	existingMeta, hadMeta := overrides.Meta[entry.Model]

	changed := !hadPricing ||
		!modelPricingEntriesEquivalent(existingPricing, entry.Pricing) ||
		hadExplicit1h != entry.HasExplicitEphemeral ||
		(hadExplicit1h && !floatAlmostEqual(existingExplicit1h, resolveExplicitEphemeral1hValue(entry))) ||
		!hadMeta ||
		normalizeModelPricingSource(existingMeta.Source) != modelPricingSourceCloudSync
	if !changed {
		return false
	}

	pricing := entry.Pricing
	pricing.CloudPricing = cloneCloudPricingRules(pricing.CloudPricing)
	overrides.Pricing[entry.Model] = pricing
	if entry.HasExplicitEphemeral {
		overrides.Ephemeral1h[entry.Model] = entry.ExplicitEphemeral1h
	} else {
		delete(overrides.Ephemeral1h, entry.Model)
	}
	overrides.Meta[entry.Model] = modelPricingMeta{
		Source:    modelPricingSourceCloudSync,
		UpdatedAt: syncedAt,
	}

	return true
}

func resolvePricingEntryFromLayers(
	localOverrides modelPricingOverrides,
	cloudOverrides modelPricingOverrides,
	defaults *modelpricing.Service,
	model string,
) (modelpricing.PricingEntry, bool) {
	if entry, ok := localOverrides.Pricing[model]; ok {
		return entry, true
	}
	if entry, ok := cloudOverrides.Pricing[model]; ok {
		return entry, true
	}
	if defaults == nil {
		return modelpricing.PricingEntry{}, false
	}
	entry, ok := defaults.PricingEntryExact(model)
	return entry, ok
}

func resolveExplicitEphemeral1hFromLayers(
	localOverrides modelPricingOverrides,
	cloudOverrides modelPricingOverrides,
	defaults *modelpricing.Service,
	model string,
) (float64, bool) {
	if value, ok := localOverrides.Ephemeral1h[model]; ok {
		return value, true
	}
	if value, ok := cloudOverrides.Ephemeral1h[model]; ok {
		return value, true
	}
	if defaults == nil {
		return 0, false
	}
	return defaults.ExplicitEphemeral1hCostPerToken(model)
}

func resolvePricingSourceFromLayers(
	localOverrides modelPricingOverrides,
	cloudOverrides modelPricingOverrides,
	model string,
) string {
	if source := resolveModelPricingSource(localOverrides.Meta[model], hasOverrideInLayer(localOverrides, model)); source != modelPricingSourceBuiltin {
		return source
	}
	return resolveModelPricingSource(cloudOverrides.Meta[model], hasOverrideInLayer(cloudOverrides, model))
}
