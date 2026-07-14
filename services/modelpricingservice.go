package services

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	modelpricing "codeswitch/resources/model-pricing"

	"github.com/daodao97/xgo/xdb"
)

const (
	modelPricingOverridesSettingKey      = "model_pricing_overrides_v1"
	modelPricingCloudOverridesSettingKey = "model_pricing_cloud_overrides_v1"
)

const (
	modelPricingSourceBuiltin    = "builtin"
	modelPricingSourceManual     = "manual"
	modelPricingSourceClaudeSync = "claude_sync"
	modelPricingSourceCloudSync  = "cloud_sync"
)

type modelPricingOverrides struct {
	Pricing     map[string]modelpricing.PricingEntry `json:"pricing"`
	Ephemeral1h map[string]float64                   `json:"ephemeral_1h"`
	Meta        map[string]modelPricingMeta          `json:"meta,omitempty"`
}

type modelPricingMeta struct {
	Source    string `json:"source,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type ModelPricingRow struct {
	OriginalModel                  string  `json:"original_model,omitempty"`
	Model                          string  `json:"model"`
	InputCostPerToken              float64 `json:"input_cost_per_token"`
	OutputCostPerToken             float64 `json:"output_cost_per_token"`
	OutputCostPerReasoningToken    float64 `json:"output_cost_per_reasoning_token"`
	CacheCreationInputTokenCost    float64 `json:"cache_creation_input_token_cost"`
	HasCacheCreationInputTokenCost bool    `json:"has_cache_creation_input_token_cost"`
	CacheReadInputTokenCost        float64 `json:"cache_read_input_token_cost"`
	HasCacheReadInputTokenCost     bool    `json:"has_cache_read_input_token_cost"`
	Ephemeral1hCostPerToken        float64 `json:"ephemeral_1h_cost_per_token"`
	GroupMultiplier                float64 `json:"group_multiplier"`
	IsOverride                     bool    `json:"is_override"`
	IsCustom                       bool    `json:"is_custom"`
	Source                         string  `json:"source"`
	SourceUpdatedAt                string  `json:"source_updated_at,omitempty"`
}

type ModelPricingService struct {
	mu                   sync.RWMutex
	defaults             *modelpricing.Service
	effective            *modelpricing.Service
	localOverrides       modelPricingOverrides
	cloudOverrides       modelPricingOverrides
	overrides            modelPricingOverrides
	claudeModelRouting   *ClaudeModelRoutingService
	claudePricingPreview claudePricingPreviewCache
	cloudPricingPreview  cloudPricingPreviewCache
}

func NewModelPricingService() *ModelPricingService {
	defaults, err := modelpricing.NewService()
	if err != nil {
		log.Printf("pricing defaults init failed: %v", err)
	}

	svc := &ModelPricingService{
		defaults:       defaults,
		localOverrides: newEmptyModelPricingOverrides(),
		cloudOverrides: newEmptyModelPricingOverrides(),
		overrides:      newEmptyModelPricingOverrides(),
	}
	svc.mu.Lock()
	svc.rebuildLocked()
	svc.mu.Unlock()

	if err := svc.reloadFromDB(); err != nil {
		log.Printf("pricing overrides load failed: %v", err)
	}

	return svc
}

func (mps *ModelPricingService) Service() *modelpricing.Service {
	if mps == nil {
		return nil
	}
	mps.mu.RLock()
	svc := mps.effective
	mps.mu.RUnlock()
	return svc
}

func (mps *ModelPricingService) BindClaudeModelRoutingService(routing *ClaudeModelRoutingService) {
	if mps == nil {
		return
	}
	mps.mu.Lock()
	mps.claudeModelRouting = routing
	mps.mu.Unlock()
}

func (mps *ModelPricingService) notifyClaudeModelRoutingChanged() {
	if mps == nil {
		return
	}
	mps.mu.RLock()
	routing := mps.claudeModelRouting
	mps.mu.RUnlock()
	if routing != nil {
		routing.HandleModelLibraryChanged()
	}
}

func (mps *ModelPricingService) ListModelPricing() ([]ModelPricingRow, error) {
	if mps == nil {
		return nil, nil
	}

	mps.mu.RLock()
	defer mps.mu.RUnlock()

	svc := mps.effective
	defaults := mps.defaults
	overridePricing := mps.overrides.Pricing
	overrideEphemeral := mps.overrides.Ephemeral1h

	if svc == nil {
		return nil, nil
	}

	models := svc.Models()
	rows := make([]ModelPricingRow, 0, len(models))
	for _, model := range models {
		entry, ok := svc.PricingEntryExact(model)
		if !ok {
			continue
		}

		isOverride := false
		if _, ok := overridePricing[model]; ok {
			isOverride = true
		}
		if _, ok := overrideEphemeral[model]; ok {
			isOverride = true
		}

		// 默认过滤：只展示 token 定价模型，避免把图像模型（只有 per-image/per-pixel）塞进来刷屏。
		if !isOverride && !hasTokenPricing(entry) {
			continue
		}

		isCustom := false
		if defaults != nil {
			if _, ok := defaults.PricingEntryExact(model); !ok {
				isCustom = true
			}
		} else if isOverride {
			isCustom = true
		}

		meta := mps.overrides.Meta[model]
		ephemeral1hCostPerToken, _ := svc.ExplicitEphemeral1hCostPerToken(model)

		rows = append(rows, ModelPricingRow{
			Model:                          model,
			InputCostPerToken:              entry.InputCostPerToken,
			OutputCostPerToken:             entry.OutputCostPerToken,
			OutputCostPerReasoningToken:    entry.OutputCostPerReasoningToken,
			CacheCreationInputTokenCost:    entry.CacheCreationInputTokenCost,
			HasCacheCreationInputTokenCost: entry.HasCacheCreationInputTokenCost,
			CacheReadInputTokenCost:        entry.CacheReadInputTokenCost,
			HasCacheReadInputTokenCost:     entry.HasCacheReadInputTokenCost,
			Ephemeral1hCostPerToken:        ephemeral1hCostPerToken,
			GroupMultiplier:                effectiveModelPricingGroupMultiplier(entry),
			IsOverride:                     isOverride,
			IsCustom:                       isCustom,
			Source:                         resolveModelPricingSource(meta, isOverride || isCustom),
			SourceUpdatedAt:                strings.TrimSpace(meta.UpdatedAt),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Model < rows[j].Model
	})

	return rows, nil
}

func (mps *ModelPricingService) UpsertModelPricing(row ModelPricingRow) error {
	if mps == nil {
		return fmt.Errorf("nil pricing service")
	}

	originalModel := strings.TrimSpace(row.OriginalModel)
	model := strings.TrimSpace(row.Model)
	if model == "" {
		return fmt.Errorf("model 不能为空")
	}

	if err := validateNonNegative(row.InputCostPerToken, "input_cost_per_token"); err != nil {
		return err
	}
	if err := validateNonNegative(row.OutputCostPerToken, "output_cost_per_token"); err != nil {
		return err
	}
	if err := validateNonNegative(row.OutputCostPerReasoningToken, "output_cost_per_reasoning_token"); err != nil {
		return err
	}
	if err := validateNonNegative(row.CacheCreationInputTokenCost, "cache_creation_input_token_cost"); err != nil {
		return err
	}
	if err := validateNonNegative(row.CacheReadInputTokenCost, "cache_read_input_token_cost"); err != nil {
		return err
	}
	if err := validateNonNegative(row.Ephemeral1hCostPerToken, "ephemeral_1h_cost_per_token"); err != nil {
		return err
	}
	if err := validateNonNegative(row.GroupMultiplier, "group_multiplier"); err != nil {
		return err
	}

	mps.mu.Lock()
	notifyRouting := false
	defer func() {
		mps.mu.Unlock()
		if notifyRouting {
			mps.notifyClaudeModelRoutingChanged()
		}
	}()

	newOverrides := cloneModelPricingOverrides(mps.localOverrides)
	if mps.hasRenameConflictLocked(originalModel, model) {
		return fmt.Errorf("模型 %s 已存在，不能重命名覆盖", model)
	}
	if originalModel != "" && originalModel != model {
		delete(newOverrides.Pricing, originalModel)
		delete(newOverrides.Ephemeral1h, originalModel)
		delete(newOverrides.Meta, originalModel)
	}

	existing := modelpricing.PricingEntry{}
	if v, ok := newOverrides.Pricing[model]; ok {
		existing = v
	}
	newOverrides.Pricing[model] = applyModelPricingRowToEntry(existing, row)

	newOverrides.Ephemeral1h[model] = row.Ephemeral1hCostPerToken
	newOverrides.Meta[model] = modelPricingMeta{
		Source:    modelPricingSourceManual,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if err := saveModelPricingOverridesToDB(newOverrides); err != nil {
		return err
	}

	mps.localOverrides = newOverrides
	mps.rebuildLocked()
	notifyRouting = true
	return nil
}

func (mps *ModelPricingService) DeleteModelPricing(model string) error {
	if mps == nil {
		return fmt.Errorf("nil pricing service")
	}
	key := strings.TrimSpace(model)
	if key == "" {
		return nil
	}

	mps.mu.Lock()
	notifyRouting := false
	defer func() {
		mps.mu.Unlock()
		if notifyRouting {
			mps.notifyClaudeModelRoutingChanged()
		}
	}()

	newOverrides := cloneModelPricingOverrides(mps.localOverrides)
	delete(newOverrides.Pricing, key)
	delete(newOverrides.Ephemeral1h, key)
	delete(newOverrides.Meta, key)

	if err := saveModelPricingOverridesToDB(newOverrides); err != nil {
		return err
	}

	mps.localOverrides = newOverrides
	mps.rebuildLocked()
	notifyRouting = true
	return nil
}

func (mps *ModelPricingService) reloadFromDB() error {
	localOverrides, cloudOverrides, migratedCloudOverrides, err := loadModelPricingOverrideLayersFromDB()
	if err != nil {
		mps.mu.Lock()
		mps.rebuildLocked()
		mps.mu.Unlock()
		return err
	}

	if len(migratedCloudOverrides.Pricing) > 0 ||
		len(migratedCloudOverrides.Ephemeral1h) > 0 ||
		len(migratedCloudOverrides.Meta) > 0 {
		cloudOverrides = overlayModelPricingOverrides(cloudOverrides, migratedCloudOverrides)
		if err := saveCloudModelPricingOverridesToDB(cloudOverrides); err != nil {
			log.Printf("pricing cloud overrides migration save failed: %v", err)
		}
		if err := saveModelPricingOverridesToDB(localOverrides); err != nil {
			log.Printf("pricing primary overrides migration save failed: %v", err)
		}
	}

	mps.mu.Lock()
	mps.localOverrides = localOverrides
	mps.cloudOverrides = cloudOverrides
	mps.rebuildLocked()
	mps.mu.Unlock()
	return nil
}

func (mps *ModelPricingService) rebuildLocked() {
	if mps.defaults == nil {
		mps.effective = nil
		return
	}
	mps.overrides = overlayModelPricingOverrides(mps.cloudOverrides, mps.localOverrides)
	merged := mps.defaults.Clone()
	merged.ApplyOverrides(mps.overrides.Pricing, mps.overrides.Ephemeral1h)
	mps.effective = merged
}

func (mps *ModelPricingService) hasRenameConflictLocked(originalModel, targetModel string) bool {
	if mps == nil || mps.effective == nil {
		return false
	}
	original := strings.TrimSpace(originalModel)
	target := strings.TrimSpace(targetModel)
	if original == "" || target == "" || original == target {
		return false
	}
	_, exists := mps.effective.PricingEntryExact(target)
	return exists
}

func validateNonNegative(value float64, name string) error {
	if value < 0 {
		return fmt.Errorf("%s 不能为负数", name)
	}
	return nil
}

func applyModelPricingRowToEntry(existing modelpricing.PricingEntry, row ModelPricingRow) modelpricing.PricingEntry {
	existing.InputCostPerToken = row.InputCostPerToken
	existing.HasInputCostPerToken = true
	existing.OutputCostPerToken = row.OutputCostPerToken
	existing.HasOutputCostPerToken = true
	existing.OutputCostPerReasoningToken = row.OutputCostPerReasoningToken
	existing.HasOutputCostPerReasoningToken = true
	existing.CacheCreationInputTokenCost = row.CacheCreationInputTokenCost
	existing.HasCacheCreationInputTokenCost = row.HasCacheCreationInputTokenCost
	existing.CacheReadInputTokenCost = row.CacheReadInputTokenCost
	existing.HasCacheReadInputTokenCost = row.HasCacheReadInputTokenCost
	existing.GroupMultiplier = row.GroupMultiplier
	existing.HasGroupMultiplier = true
	return existing
}

func hasTokenPricing(entry modelpricing.PricingEntry) bool {
	return entry.InputCostPerToken != 0 ||
		entry.HasInputCostPerToken ||
		entry.OutputCostPerToken != 0 ||
		entry.HasOutputCostPerToken ||
		entry.OutputCostPerReasoningToken != 0 ||
		entry.HasOutputCostPerReasoningToken ||
		entry.CacheCreationInputTokenCost != 0 ||
		entry.HasCacheCreationInputTokenCost ||
		entry.CacheReadInputTokenCost != 0 ||
		entry.HasCacheReadInputTokenCost
}

func effectiveModelPricingGroupMultiplier(entry modelpricing.PricingEntry) float64 {
	if entry.HasGroupMultiplier || entry.GroupMultiplier != 0 {
		return entry.GroupMultiplier
	}
	return 1
}

func cloneModelPricingOverrides(src modelPricingOverrides) modelPricingOverrides {
	dst := newEmptyModelPricingOverridesWithCapacity(len(src.Pricing), len(src.Ephemeral1h), len(src.Meta))
	for key, value := range src.Pricing {
		dst.Pricing[key] = value
	}
	for key, value := range src.Ephemeral1h {
		dst.Ephemeral1h[key] = value
	}
	for key, value := range src.Meta {
		dst.Meta[key] = value
	}
	return dst
}

func newEmptyModelPricingOverrides() modelPricingOverrides {
	return newEmptyModelPricingOverridesWithCapacity(0, 0, 0)
}

func newEmptyModelPricingOverridesWithCapacity(pricingCap, ephCap, metaCap int) modelPricingOverrides {
	return modelPricingOverrides{
		Pricing:     make(map[string]modelpricing.PricingEntry, pricingCap),
		Ephemeral1h: make(map[string]float64, ephCap),
		Meta:        make(map[string]modelPricingMeta, metaCap),
	}
}

func ensureModelPricingOverridesInitialized(overrides *modelPricingOverrides) {
	if overrides == nil {
		return
	}
	if overrides.Pricing == nil {
		overrides.Pricing = make(map[string]modelpricing.PricingEntry)
	}
	if overrides.Ephemeral1h == nil {
		overrides.Ephemeral1h = make(map[string]float64)
	}
	if overrides.Meta == nil {
		overrides.Meta = make(map[string]modelPricingMeta)
	}
}

func loadModelPricingOverrideLayersFromDB() (modelPricingOverrides, modelPricingOverrides, modelPricingOverrides, error) {
	localOverrides, _, err := loadModelPricingOverridesFromDB(modelPricingOverridesSettingKey)
	if err != nil {
		return newEmptyModelPricingOverrides(), newEmptyModelPricingOverrides(), newEmptyModelPricingOverrides(), err
	}

	cloudOverrides, _, err := loadModelPricingOverridesFromDB(modelPricingCloudOverridesSettingKey)
	if err != nil {
		return newEmptyModelPricingOverrides(), newEmptyModelPricingOverrides(), newEmptyModelPricingOverrides(), err
	}

	localOverrides, migratedCloudOverrides := splitCloudOverridesFromPrimary(localOverrides)
	return localOverrides, cloudOverrides, migratedCloudOverrides, nil
}

func loadModelPricingOverridesFromDB(settingKey string) (modelPricingOverrides, bool, error) {
	overrides := newEmptyModelPricingOverrides()

	db, err := xdb.DB("default")
	if err != nil {
		return overrides, false, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	var raw string
	err = db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, settingKey).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return overrides, false, nil
		}
		return overrides, false, err
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return overrides, false, nil
	}

	if err := json.Unmarshal([]byte(raw), &overrides); err != nil {
		return overrides, true, fmt.Errorf("解析自定义价格表失败: %w", err)
	}

	ensureModelPricingOverridesInitialized(&overrides)
	return overrides, true, nil
}

func saveModelPricingOverridesToDB(overrides modelPricingOverrides) error {
	return saveModelPricingOverridesByKey(modelPricingOverridesSettingKey, overrides)
}

func saveCloudModelPricingOverridesToDB(overrides modelPricingOverrides) error {
	return saveModelPricingOverridesByKey(modelPricingCloudOverridesSettingKey, overrides)
}

func saveModelPricingOverridesByKey(settingKey string, overrides modelPricingOverrides) error {
	if GlobalDBQueue == nil {
		return fmt.Errorf("db queue not initialized")
	}

	ensureModelPricingOverridesInitialized(&overrides)
	if isModelPricingOverridesEmpty(overrides) {
		return GlobalDBQueue.Exec(`DELETE FROM app_settings WHERE key = ?`, settingKey)
	}

	payload, err := json.Marshal(overrides)
	if err != nil {
		return err
	}

	return GlobalDBQueue.Exec(`
		INSERT INTO app_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, settingKey, string(payload))
}

func isModelPricingOverridesEmpty(overrides modelPricingOverrides) bool {
	return len(overrides.Pricing) == 0 &&
		len(overrides.Ephemeral1h) == 0 &&
		len(overrides.Meta) == 0
}

func overlayModelPricingOverrides(base modelPricingOverrides, overlay modelPricingOverrides) modelPricingOverrides {
	merged := cloneModelPricingOverrides(base)
	for key, value := range overlay.Pricing {
		merged.Pricing[key] = value
	}
	for key, value := range overlay.Ephemeral1h {
		merged.Ephemeral1h[key] = value
	}
	for key, value := range overlay.Meta {
		merged.Meta[key] = value
	}
	return merged
}

func splitCloudOverridesFromPrimary(primary modelPricingOverrides) (modelPricingOverrides, modelPricingOverrides) {
	localOverrides := cloneModelPricingOverrides(primary)
	cloudOverrides := newEmptyModelPricingOverrides()
	visited := make(map[string]struct{})

	moveKey := func(model string) {
		if _, ok := visited[model]; ok {
			return
		}
		visited[model] = struct{}{}

		meta := localOverrides.Meta[model]
		if normalizeModelPricingSource(meta.Source) != modelPricingSourceCloudSync {
			return
		}

		if entry, ok := localOverrides.Pricing[model]; ok {
			cloudOverrides.Pricing[model] = entry
			delete(localOverrides.Pricing, model)
		}
		if eph, ok := localOverrides.Ephemeral1h[model]; ok {
			cloudOverrides.Ephemeral1h[model] = eph
			delete(localOverrides.Ephemeral1h, model)
		}
		if metaValue, ok := localOverrides.Meta[model]; ok {
			cloudOverrides.Meta[model] = metaValue
			delete(localOverrides.Meta, model)
		}
	}

	for model := range localOverrides.Meta {
		moveKey(model)
	}
	for model := range localOverrides.Pricing {
		moveKey(model)
	}
	for model := range localOverrides.Ephemeral1h {
		moveKey(model)
	}

	return localOverrides, cloudOverrides
}

func resolveModelPricingSource(meta modelPricingMeta, fallbackManual bool) string {
	source := normalizeModelPricingSource(meta.Source)
	if source != "" {
		return source
	}
	if fallbackManual {
		return modelPricingSourceManual
	}
	return modelPricingSourceBuiltin
}

func normalizeModelPricingSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case modelPricingSourceBuiltin:
		return modelPricingSourceBuiltin
	case modelPricingSourceManual:
		return modelPricingSourceManual
	case modelPricingSourceClaudeSync:
		return modelPricingSourceClaudeSync
	case modelPricingSourceCloudSync:
		return modelPricingSourceCloudSync
	default:
		return ""
	}
}
