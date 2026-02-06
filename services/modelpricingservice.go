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

	modelpricing "codeswitch/resources/model-pricing"

	"github.com/daodao97/xgo/xdb"
)

const modelPricingOverridesSettingKey = "model_pricing_overrides_v1"

type modelPricingOverrides struct {
	Pricing     map[string]modelpricing.PricingEntry `json:"pricing"`
	Ephemeral1h map[string]float64                   `json:"ephemeral_1h"`
}

type ModelPricingRow struct {
	Model                       string  `json:"model"`
	InputCostPerToken           float64 `json:"input_cost_per_token"`
	OutputCostPerToken          float64 `json:"output_cost_per_token"`
	OutputCostPerReasoningToken float64 `json:"output_cost_per_reasoning_token"`
	CacheCreationInputTokenCost float64 `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost     float64 `json:"cache_read_input_token_cost"`
	Ephemeral1hCostPerToken     float64 `json:"ephemeral_1h_cost_per_token"`
	IsOverride                  bool    `json:"is_override"`
	IsCustom                    bool    `json:"is_custom"`
}

type ModelPricingService struct {
	mu        sync.RWMutex
	defaults  *modelpricing.Service
	effective *modelpricing.Service
	overrides modelPricingOverrides
}

func NewModelPricingService() *ModelPricingService {
	defaults, err := modelpricing.NewService()
	if err != nil {
		log.Printf("pricing defaults init failed: %v", err)
	}

	svc := &ModelPricingService{
		defaults: defaults,
		overrides: modelPricingOverrides{
			Pricing:     make(map[string]modelpricing.PricingEntry),
			Ephemeral1h: make(map[string]float64),
		},
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

		rows = append(rows, ModelPricingRow{
			Model:                       model,
			InputCostPerToken:           entry.InputCostPerToken,
			OutputCostPerToken:          entry.OutputCostPerToken,
			OutputCostPerReasoningToken: entry.OutputCostPerReasoningToken,
			CacheCreationInputTokenCost: entry.CacheCreationInputTokenCost,
			CacheReadInputTokenCost:     entry.CacheReadInputTokenCost,
			Ephemeral1hCostPerToken:     svc.Ephemeral1hCostPerToken(model),
			IsOverride:                  isOverride,
			IsCustom:                    isCustom,
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

	mps.mu.Lock()
	defer mps.mu.Unlock()

	newOverrides := cloneModelPricingOverrides(mps.overrides)

	existing := modelpricing.PricingEntry{}
	if v, ok := newOverrides.Pricing[model]; ok {
		existing = v
	}
	existing.InputCostPerToken = row.InputCostPerToken
	existing.OutputCostPerToken = row.OutputCostPerToken
	existing.OutputCostPerReasoningToken = row.OutputCostPerReasoningToken
	existing.CacheCreationInputTokenCost = row.CacheCreationInputTokenCost
	existing.CacheReadInputTokenCost = row.CacheReadInputTokenCost
	newOverrides.Pricing[model] = existing

	newOverrides.Ephemeral1h[model] = row.Ephemeral1hCostPerToken

	if err := saveModelPricingOverridesToDB(newOverrides); err != nil {
		return err
	}

	mps.overrides = newOverrides
	mps.rebuildLocked()
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
	defer mps.mu.Unlock()

	newOverrides := cloneModelPricingOverrides(mps.overrides)
	delete(newOverrides.Pricing, key)
	delete(newOverrides.Ephemeral1h, key)

	if err := saveModelPricingOverridesToDB(newOverrides); err != nil {
		return err
	}

	mps.overrides = newOverrides
	mps.rebuildLocked()
	return nil
}

func (mps *ModelPricingService) reloadFromDB() error {
	mps.mu.Lock()
	defer mps.mu.Unlock()

	overrides, err := loadModelPricingOverridesFromDB()
	if err != nil {
		mps.rebuildLocked()
		return err
	}
	mps.overrides = overrides
	mps.rebuildLocked()
	return nil
}

func (mps *ModelPricingService) rebuildLocked() {
	if mps.defaults == nil {
		mps.effective = nil
		return
	}
	merged := mps.defaults.Clone()
	merged.ApplyOverrides(mps.overrides.Pricing, mps.overrides.Ephemeral1h)
	mps.effective = merged
}

func validateNonNegative(value float64, name string) error {
	if value < 0 {
		return fmt.Errorf("%s 不能为负数", name)
	}
	return nil
}

func hasTokenPricing(entry modelpricing.PricingEntry) bool {
	return entry.InputCostPerToken != 0 ||
		entry.OutputCostPerToken != 0 ||
		entry.OutputCostPerReasoningToken != 0 ||
		entry.CacheCreationInputTokenCost != 0 ||
		entry.CacheReadInputTokenCost != 0
}

func cloneModelPricingOverrides(src modelPricingOverrides) modelPricingOverrides {
	dst := modelPricingOverrides{
		Pricing:     make(map[string]modelpricing.PricingEntry, len(src.Pricing)),
		Ephemeral1h: make(map[string]float64, len(src.Ephemeral1h)),
	}
	for key, value := range src.Pricing {
		dst.Pricing[key] = value
	}
	for key, value := range src.Ephemeral1h {
		dst.Ephemeral1h[key] = value
	}
	return dst
}

func loadModelPricingOverridesFromDB() (modelPricingOverrides, error) {
	overrides := modelPricingOverrides{
		Pricing:     make(map[string]modelpricing.PricingEntry),
		Ephemeral1h: make(map[string]float64),
	}

	db, err := xdb.DB("default")
	if err != nil {
		return overrides, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	var raw string
	err = db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, modelPricingOverridesSettingKey).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return overrides, nil
		}
		return overrides, err
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return overrides, nil
	}

	if err := json.Unmarshal([]byte(raw), &overrides); err != nil {
		return overrides, fmt.Errorf("解析自定义价格表失败: %w", err)
	}

	if overrides.Pricing == nil {
		overrides.Pricing = make(map[string]modelpricing.PricingEntry)
	}
	if overrides.Ephemeral1h == nil {
		overrides.Ephemeral1h = make(map[string]float64)
	}

	return overrides, nil
}

func saveModelPricingOverridesToDB(overrides modelPricingOverrides) error {
	if GlobalDBQueue == nil {
		return fmt.Errorf("db queue not initialized")
	}

	payload, err := json.Marshal(overrides)
	if err != nil {
		return err
	}

	return GlobalDBQueue.Exec(`
		INSERT INTO app_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, modelPricingOverridesSettingKey, string(payload))
}
