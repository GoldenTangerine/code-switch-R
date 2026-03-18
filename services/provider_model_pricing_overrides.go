package services

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/daodao97/xgo/xdb"
)

const providerPricingOverridesSettingKey = "provider_model_pricing_overrides_v1"

type providerPricingOverrideItem struct {
	GroupMultiplier          float64 `json:"group_multiplier,omitempty"`
	HasGroupMultiplier       bool    `json:"has_group_multiplier,omitempty"`
	CacheCreateMultiplier    float64 `json:"cache_create_multiplier,omitempty"`
	HasCacheCreateMultiplier bool    `json:"has_cache_create_multiplier,omitempty"`
	CacheReadMultiplier      float64 `json:"cache_read_multiplier,omitempty"`
	HasCacheReadMultiplier   bool    `json:"has_cache_read_multiplier,omitempty"`
	UpdatedAt                string  `json:"updated_at,omitempty"`
}

type providerPricingOverrideStore struct {
	Providers map[string]map[string]providerPricingOverrideItem `json:"providers"`
}

func newProviderPricingOverrideStore() providerPricingOverrideStore {
	return providerPricingOverrideStore{
		Providers: make(map[string]map[string]providerPricingOverrideItem),
	}
}

func cloneProviderPricingOverrideStore(src providerPricingOverrideStore) providerPricingOverrideStore {
	dst := newProviderPricingOverrideStore()
	for scopeKey, modelOverrides := range src.Providers {
		if len(modelOverrides) == 0 {
			continue
		}
		cloned := make(map[string]providerPricingOverrideItem, len(modelOverrides))
		for modelKey, item := range modelOverrides {
			cloned[modelKey] = item
		}
		dst.Providers[scopeKey] = cloned
	}
	return dst
}

func providerPricingOverrideScopeKey(apiURL, apiKey, authType string) string {
	url := strings.TrimSpace(strings.ToLower(apiURL))
	key := strings.TrimSpace(apiKey)
	auth := strings.TrimSpace(strings.ToLower(authType))
	if url == "" || key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%s|%s|%x", url, auth, sum)
}

func loadProviderPricingOverridesFromDB() (providerPricingOverrideStore, error) {
	store := newProviderPricingOverrideStore()

	db, err := xdb.DB("default")
	if err != nil {
		return store, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	var raw string
	err = db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, providerPricingOverridesSettingKey).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store, nil
		}
		return store, err
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return store, nil
	}

	if err := json.Unmarshal([]byte(raw), &store); err != nil {
		return store, fmt.Errorf("解析供应商模型缓存倍率覆盖配置失败: %w", err)
	}
	if store.Providers == nil {
		store.Providers = make(map[string]map[string]providerPricingOverrideItem)
	}
	return store, nil
}

func saveProviderPricingOverridesToDB(store providerPricingOverrideStore) error {
	if GlobalDBQueue == nil {
		return fmt.Errorf("db queue not initialized")
	}

	payload, err := json.Marshal(store)
	if err != nil {
		return err
	}

	return GlobalDBQueue.Exec(`
		INSERT INTO app_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, providerPricingOverridesSettingKey, string(payload))
}

func (ps *ProviderService) resolveProviderModelPricingOverride(apiURL, apiKey, authType, model string) (providerPricingOverrideItem, bool) {
	if ps == nil {
		return providerPricingOverrideItem{}, false
	}

	scopeKey := providerPricingOverrideScopeKey(apiURL, apiKey, authType)
	modelKey := normalizeProviderPricingModelName(model)
	if scopeKey == "" || modelKey == "" {
		return providerPricingOverrideItem{}, false
	}

	ps.providerPricingOverridesMu.RLock()
	defer ps.providerPricingOverridesMu.RUnlock()

	models, ok := ps.providerPricingOverrides.Providers[scopeKey]
	if !ok || len(models) == 0 {
		return providerPricingOverrideItem{}, false
	}

	item, ok := models[modelKey]
	if !ok {
		return providerPricingOverrideItem{}, false
	}
	if !item.HasGroupMultiplier && !item.HasCacheCreateMultiplier && !item.HasCacheReadMultiplier {
		return providerPricingOverrideItem{}, false
	}
	return item, true
}

func (ps *ProviderService) UpsertProviderModelPricingOverride(
	apiURL,
	apiKey,
	authType,
	model string,
	groupMultiplier float64,
	hasGroupMultiplier bool,
	cacheCreateMultiplier float64,
	hasCacheCreateMultiplier bool,
	cacheReadMultiplier float64,
	hasCacheReadMultiplier bool,
) error {
	if ps == nil {
		return fmt.Errorf("nil provider service")
	}
	if hasGroupMultiplier {
		if err := validateNonNegative(groupMultiplier, "group_multiplier"); err != nil {
			return err
		}
	}
	if hasCacheCreateMultiplier {
		if err := validateNonNegative(cacheCreateMultiplier, "cache_create_multiplier"); err != nil {
			return err
		}
	}
	if hasCacheReadMultiplier {
		if err := validateNonNegative(cacheReadMultiplier, "cache_read_multiplier"); err != nil {
			return err
		}
	}

	scopeKey := providerPricingOverrideScopeKey(apiURL, apiKey, authType)
	if scopeKey == "" {
		return fmt.Errorf("apiUrl 和 apiKey 不能为空")
	}
	modelKey := normalizeProviderPricingModelName(model)
	if modelKey == "" {
		return fmt.Errorf("model 不能为空")
	}

	if !hasGroupMultiplier && !hasCacheCreateMultiplier && !hasCacheReadMultiplier {
		return ps.DeleteProviderModelPricingOverride(apiURL, apiKey, authType, model)
	}

	ps.providerPricingOverridesMu.Lock()
	defer ps.providerPricingOverridesMu.Unlock()

	store := cloneProviderPricingOverrideStore(ps.providerPricingOverrides)
	if store.Providers == nil {
		store.Providers = make(map[string]map[string]providerPricingOverrideItem)
	}
	modelOverrides, ok := store.Providers[scopeKey]
	if !ok {
		modelOverrides = make(map[string]providerPricingOverrideItem)
		store.Providers[scopeKey] = modelOverrides
	}
	modelOverrides[modelKey] = providerPricingOverrideItem{
		GroupMultiplier:          groupMultiplier,
		HasGroupMultiplier:       hasGroupMultiplier,
		CacheCreateMultiplier:    cacheCreateMultiplier,
		HasCacheCreateMultiplier: hasCacheCreateMultiplier,
		CacheReadMultiplier:      cacheReadMultiplier,
		HasCacheReadMultiplier:   hasCacheReadMultiplier,
		UpdatedAt:                time.Now().UTC().Format(time.RFC3339),
	}

	if err := saveProviderPricingOverridesToDB(store); err != nil {
		return err
	}
	ps.providerPricingOverrides = store
	return nil
}

func (ps *ProviderService) DeleteProviderModelPricingOverride(apiURL, apiKey, authType, model string) error {
	if ps == nil {
		return fmt.Errorf("nil provider service")
	}

	scopeKey := providerPricingOverrideScopeKey(apiURL, apiKey, authType)
	if scopeKey == "" {
		return fmt.Errorf("apiUrl 和 apiKey 不能为空")
	}
	modelKey := normalizeProviderPricingModelName(model)
	if modelKey == "" {
		return nil
	}

	ps.providerPricingOverridesMu.Lock()
	defer ps.providerPricingOverridesMu.Unlock()

	store := cloneProviderPricingOverrideStore(ps.providerPricingOverrides)
	modelOverrides, ok := store.Providers[scopeKey]
	if !ok {
		return nil
	}
	delete(modelOverrides, modelKey)
	if len(modelOverrides) == 0 {
		delete(store.Providers, scopeKey)
	}

	if err := saveProviderPricingOverridesToDB(store); err != nil {
		return err
	}
	ps.providerPricingOverrides = store
	return nil
}
