package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"muzzammil.xyz/jsonc"
)

// OpenCodeProvider OpenCode 供应商配置。
//
// OpenCode 的 live 配置写入 ~/.config/opencode/opencode.json 的 provider.{id}，
// 其中 SettingsConfig 对应单个 provider fragment，典型结构：
// {"npm":"@ai-sdk/openai-compatible","options":{"baseURL":"...","apiKey":"..."},"models":{...}}
type OpenCodeProvider struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	WebsiteURL           string         `json:"websiteUrl,omitempty"`
	APIKeyURL            string         `json:"apiKeyUrl,omitempty"`
	BaseURL              string         `json:"baseUrl,omitempty"`
	APIKey               string         `json:"apiKey,omitempty"`
	NPM                  string         `json:"npm,omitempty"`
	Icon                 string         `json:"icon,omitempty"`
	Description          string         `json:"description,omitempty"`
	Category             string         `json:"category,omitempty"`
	PartnerPromotionKey  string         `json:"partnerPromotionKey,omitempty"`
	Enabled              bool           `json:"enabled"`
	LiveConfigManaged    *bool          `json:"liveConfigManaged,omitempty"`
	IsInConfig           *bool          `json:"isInConfig,omitempty"`
	SortOrder            int            `json:"sortOrder,omitempty"`
	EnabledSortOrder     int            `json:"enabledSortOrder,omitempty"`
	DisabledSortOrder    int            `json:"disabledSortOrder,omitempty"`
	Level                int            `json:"level,omitempty"`
	SettingsConfig       map[string]any `json:"settingsConfig,omitempty"`
	RequestBodyOverrides map[string]any `json:"requestBodyOverrides,omitempty"`

	BudgetQuotaSettings        *BudgetQuotaSettings      `json:"budgetQuotaSettings,omitempty"`
	BudgetQuotaUsedAdjustments *BudgetQuotaAdjustments   `json:"budgetQuotaUsedAdjustments,omitempty"`
	ProviderQuotaQueryType     string                    `json:"providerQuotaQueryType,omitempty"`
	ProviderQuotaQueryConfig   *ProviderQuotaQueryConfig `json:"providerQuotaQueryConfig,omitempty"`
}

type opencodeProviderEnvelope struct {
	Providers []OpenCodeProvider `json:"providers"`
}

type OpenCodeProviderPreset struct {
	Name                string         `json:"name"`
	WebsiteURL          string         `json:"websiteUrl,omitempty"`
	APIKeyURL           string         `json:"apiKeyUrl,omitempty"`
	NPM                 string         `json:"npm,omitempty"`
	BaseURL             string         `json:"baseUrl,omitempty"`
	Description         string         `json:"description,omitempty"`
	Category            string         `json:"category,omitempty"`
	PartnerPromotionKey string         `json:"partnerPromotionKey,omitempty"`
	SettingsConfig      map[string]any `json:"settingsConfig,omitempty"`
}

type OpenCodeService struct {
	mu        sync.Mutex
	providers []OpenCodeProvider
	presets   []OpenCodeProviderPreset
}

func NewOpenCodeService() *OpenCodeService {
	service := &OpenCodeService{
		presets: getOpenCodePresets(),
	}
	if err := service.loadProviders(); err != nil {
		log.Printf("OpenCode providers load failed: %v", err)
	}
	return service
}

func (s *OpenCodeService) Start() error { return nil }
func (s *OpenCodeService) Stop() error  { return nil }

func (s *OpenCodeService) GetPresets() []OpenCodeProviderPreset {
	return s.presets
}

func (s *OpenCodeService) GetProviders() []OpenCodeProvider {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneOpenCodeProviders(s.providers)
}

func (s *OpenCodeService) GetLiveProviderIds() ([]string, error) {
	liveProviders, err := readOpenCodeLiveProviders()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(liveProviders))
	for id := range liveProviders {
		normalized := normalizeOpenCodeProviderID(id)
		if normalized != "" {
			ids = append(ids, normalized)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (s *OpenCodeService) SaveProviders(providers []OpenCodeProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	nextProviders, syncLive, err := s.prepareOpenCodeProvidersSnapshot(providers)
	if err != nil {
		return err
	}

	previousProviders := cloneOpenCodeProviders(s.providers)
	s.providers = nextProviders
	if err := s.saveProvidersWithOptionalLiveSync(syncLive, previousProviders); err != nil {
		s.providers = previousProviders
		return err
	}
	return nil
}

func (s *OpenCodeService) AddProvider(provider OpenCodeProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider.ID = normalizeOpenCodeProviderIDForCreate(provider.ID)
	if provider.ID == "" {
		provider.ID = fmt.Sprintf("opencode-%d", time.Now().UnixNano())
	}

	for _, existing := range s.providers {
		if existing.ID == provider.ID {
			return fmt.Errorf("OpenCode 供应商 ID '%s' 已存在", provider.ID)
		}
	}

	liveProviders, err := readOpenCodeLiveProviders()
	if err != nil {
		return err
	}
	if openCodeLiveProviderIDExists(liveProviders, provider.ID) {
		return fmt.Errorf("OpenCode live 配置中已存在 provider '%s'，请先使用导入功能纳入管理，避免覆盖用户手写配置", provider.ID)
	}

	provider = normalizeOpenCodeProvider(provider)
	if provider.Enabled {
		provider.LiveConfigManaged = boolPtr(true)
		provider.IsInConfig = boolPtr(true)
	} else {
		provider.LiveConfigManaged = boolPtr(false)
		provider.IsInConfig = boolPtr(false)
	}
	s.providers = append(s.providers, provider)
	if err := s.saveProvidersWithOptionalLiveSync(shouldManageOpenCodeLiveProvider(provider)); err != nil {
		s.providers = s.providers[:len(s.providers)-1]
		return err
	}
	return nil
}

func (s *OpenCodeService) UpdateProvider(provider OpenCodeProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider.ID = normalizeOpenCodeProviderID(provider.ID)
	for i, existing := range s.providers {
		if existing.ID != provider.ID {
			continue
		}

		provider = normalizeOpenCodeProvider(provider)
		previous := existing
		previousProviders := cloneOpenCodeProviders(s.providers)
		s.providers[i] = provider
		if err := s.saveProvidersWithOptionalLiveSync(shouldManageOpenCodeLiveProvider(previous) || shouldManageOpenCodeLiveProvider(provider), previousProviders); err != nil {
			s.providers[i] = previous
			return err
		}
		return nil
	}

	return fmt.Errorf("未找到 ID 为 '%s' 的 OpenCode 供应商", provider.ID)
}

func (s *OpenCodeService) DeleteProvider(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = normalizeOpenCodeProviderID(id)
	for i, provider := range s.providers {
		if provider.ID != id {
			continue
		}

		previous := cloneOpenCodeProviders(s.providers)
		s.providers = append(s.providers[:i], s.providers[i+1:]...)
		var previousLiveData []byte
		previousLiveExists := false
		if shouldManageOpenCodeLiveProvider(provider) {
			var snapshotErr error
			previousLiveData, previousLiveExists, snapshotErr = readOpenCodeLiveConfigBytes()
			if snapshotErr != nil {
				s.providers = previous
				return snapshotErr
			}
			if err := removeOpenCodeLiveProvider(id); err != nil {
				s.providers = previous
				return err
			}
		}
		if err := s.saveProviders(); err != nil {
			s.providers = previous
			if shouldManageOpenCodeLiveProvider(provider) {
				_ = restoreOpenCodeLiveConfigBytes(previousLiveData, previousLiveExists)
			}
			return err
		}
		return nil
	}

	return fmt.Errorf("未找到 ID 为 '%s' 的 OpenCode 供应商", id)
}

func (s *OpenCodeService) ReorderProviders(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(ids) == 0 {
		return nil
	}

	providerMap := make(map[string]OpenCodeProvider, len(s.providers))
	for _, provider := range s.providers {
		providerMap[provider.ID] = provider
	}

	reordered := make([]OpenCodeProvider, 0, len(s.providers))
	for _, rawID := range ids {
		id := normalizeOpenCodeProviderID(rawID)
		provider, ok := providerMap[id]
		if !ok {
			continue
		}
		reordered = append(reordered, provider)
		delete(providerMap, id)
	}

	for _, provider := range s.providers {
		if _, ok := providerMap[provider.ID]; ok {
			reordered = append(reordered, provider)
			delete(providerMap, provider.ID)
		}
	}

	previous := cloneOpenCodeProviders(s.providers)
	s.providers = reordered
	if err := s.saveProviders(); err != nil {
		s.providers = previous
		return err
	}
	return nil
}

func (s *OpenCodeService) DuplicateProvider(sourceID string) (*OpenCodeProvider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourceID = normalizeOpenCodeProviderID(sourceID)
	var source *OpenCodeProvider
	for i := range s.providers {
		if s.providers[i].ID == sourceID {
			source = &s.providers[i]
			break
		}
	}
	if source == nil {
		return nil, fmt.Errorf("未找到 ID 为 '%s' 的 OpenCode 供应商", sourceID)
	}

	clone := *source
	clone.ID = normalizeOpenCodeProviderIDForCreate(fmt.Sprintf("%s-copy-%d", source.ID, time.Now().Unix()))
	liveProviders, err := readOpenCodeLiveProviders()
	if err != nil {
		return nil, err
	}
	for openCodeProviderIDExists(s.providers, clone.ID) || openCodeLiveProviderIDExists(liveProviders, clone.ID) {
		clone.ID = normalizeOpenCodeProviderIDForCreate(fmt.Sprintf("%s-copy-%d", source.ID, time.Now().UnixNano()))
	}
	clone.Name = source.Name + " (副本)"
	clone.Enabled = false
	clone.LiveConfigManaged = boolPtr(false)
	clone.IsInConfig = boolPtr(false)
	clone.SettingsConfig = cloneAnyMap(source.SettingsConfig)
	clone.RequestBodyOverrides = cloneAnyMap(source.RequestBodyOverrides)
	clone.BudgetQuotaSettings = cloneBudgetQuotaSettingsPtr(source.BudgetQuotaSettings)
	clone.BudgetQuotaUsedAdjustments = cloneBudgetQuotaAdjustmentsPtr(source.BudgetQuotaUsedAdjustments)
	clone.ProviderQuotaQueryConfig = cloneProviderQuotaQueryConfigPtr(source.ProviderQuotaQueryConfig)

	s.providers = append(s.providers, clone)
	if err := s.saveProviders(); err != nil {
		s.providers = s.providers[:len(s.providers)-1]
		return nil, err
	}

	return &clone, nil
}

func (s *OpenCodeService) ImportFromLive() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	liveProviders, err := readOpenCodeLiveProviders()
	if err != nil {
		return 0, err
	}

	existingIDs := make(map[string]bool, len(s.providers))
	for _, provider := range s.providers {
		existingIDs[provider.ID] = true
	}

	imported := 0
	keys := make([]string, 0, len(liveProviders))
	for id := range liveProviders {
		keys = append(keys, id)
	}
	sort.Strings(keys)

	for _, rawID := range keys {
		id := normalizeOpenCodeProviderID(rawID)
		if id == "" || existingIDs[id] {
			continue
		}

		fragment := liveProviders[rawID]
		provider := normalizeOpenCodeProvider(OpenCodeProvider{
			ID:                id,
			Name:              resolveOpenCodeProviderName(rawID, fragment),
			BaseURL:           extractOpenCodeBaseURL(fragment),
			APIKey:            extractOpenCodeAPIKey(fragment),
			NPM:               extractOpenCodeNPM(fragment),
			Enabled:           true,
			LiveConfigManaged: boolPtr(true),
			IsInConfig:        boolPtr(true),
			Level:             1,
			SettingsConfig:    cloneAnyMap(fragment),
		})
		s.providers = append(s.providers, provider)
		existingIDs[id] = true
		imported++
	}

	if imported == 0 {
		return 0, nil
	}
	return imported, s.saveProviders()
}

func (s *OpenCodeService) saveProvidersAndSyncLive(previousSnapshots ...[]OpenCodeProvider) error {
	previousLiveData, previousLiveExists, err := readOpenCodeLiveConfigBytes()
	if err != nil {
		return err
	}
	previousManagedData, previousManagedExists, err := readOpenCodeManagedProvidersBytes()
	if err != nil {
		return err
	}
	previousLiveConfig, err := readOpenCodeLiveConfig()
	if err != nil {
		return err
	}

	nextLiveConfig := cloneAnyMap(previousLiveConfig)
	if len(previousSnapshots) > 0 {
		removeMissingOpenCodeLiveProvidersFromConfig(nextLiveConfig, previousSnapshots[0], s.providers)
	}
	applyOpenCodeLiveProvidersToConfig(nextLiveConfig, s.providers)
	if err := writeOpenCodeLiveConfig(nextLiveConfig); err != nil {
		return err
	}

	if err := s.saveProviders(); err != nil {
		if rollbackErr := restoreOpenCodeLiveConfigBytes(previousLiveData, previousLiveExists); rollbackErr != nil {
			return fmt.Errorf("保存 OpenCode 供应商失败: %w；回滚 OpenCode live 配置也失败: %v", err, rollbackErr)
		}
		if rollbackErr := restoreOpenCodeManagedProvidersBytes(previousManagedData, previousManagedExists); rollbackErr != nil {
			return fmt.Errorf("保存 OpenCode 供应商失败: %w；回滚 OpenCode managed 配置也失败: %v", err, rollbackErr)
		}
		return err
	}
	return nil
}

func (s *OpenCodeService) saveProvidersWithOptionalLiveSync(syncLive bool, previousProviders ...[]OpenCodeProvider) error {
	if syncLive {
		return s.saveProvidersAndSyncLive(previousProviders...)
	}
	return s.saveProviders()
}

func (s *OpenCodeService) prepareOpenCodeProvidersSnapshot(providers []OpenCodeProvider) ([]OpenCodeProvider, bool, error) {
	existingManagedIDs := make(map[string]bool, len(s.providers))
	for _, provider := range s.providers {
		id := normalizeOpenCodeProviderID(provider.ID)
		if id != "" {
			existingManagedIDs[id] = shouldManageOpenCodeLiveProvider(provider)
		}
	}

	liveProviders, err := readOpenCodeLiveProviders()
	if err != nil {
		return nil, false, err
	}

	nextProviders := make([]OpenCodeProvider, 0, len(providers))
	seenIDs := make(map[string]bool, len(providers))
	nextManagedIDs := make(map[string]bool, len(providers))
	syncLive := false
	for _, provider := range providers {
		provider.ID = normalizeOpenCodeProviderID(provider.ID)
		if provider.ID == "" {
			return nil, false, fmt.Errorf("OpenCode 供应商 ID 不能为空")
		}
		if seenIDs[provider.ID] {
			return nil, false, fmt.Errorf("OpenCode 供应商 ID '%s' 重复", provider.ID)
		}
		if shouldManageOpenCodeLiveProvider(provider) && !existingManagedIDs[provider.ID] && openCodeLiveProviderIDExists(liveProviders, provider.ID) {
			return nil, false, fmt.Errorf("OpenCode live 配置中已存在 provider '%s'，请先使用导入功能纳入管理，避免覆盖用户手写配置", provider.ID)
		}

		provider = normalizeOpenCodeProvider(provider)
		seenIDs[provider.ID] = true
		nextProviders = append(nextProviders, provider)
		if shouldManageOpenCodeLiveProvider(provider) {
			nextManagedIDs[provider.ID] = true
			syncLive = true
		}
	}

	for _, provider := range s.providers {
		if shouldManageOpenCodeLiveProvider(provider) && !nextManagedIDs[provider.ID] {
			syncLive = true
			break
		}
	}

	return nextProviders, syncLive, nil
}

func (s *OpenCodeService) saveProviders() error {
	path, err := providerConfigPath("opencode", true)
	if err != nil {
		return err
	}

	providers := cloneOpenCodeProviders(s.providers)
	for i := range providers {
		providers[i] = normalizeOpenCodeProvider(providers[i])
	}

	data, err := json.MarshalIndent(opencodeProviderEnvelope{Providers: providers}, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 OpenCode 供应商失败: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *OpenCodeService) loadProviders() error {
	path, err := providerConfigPath("opencode", true)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			liveProviders, liveErr := importOpenCodeProvidersFromLiveSnapshot()
			if liveErr != nil {
				log.Printf("OpenCode live providers import skipped: %v", liveErr)
				return nil
			}
			s.providers = liveProviders
			if len(liveProviders) > 0 {
				return s.saveProviders()
			}
			return nil
		}
		return err
	}

	var envelope opencodeProviderEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("解析 OpenCode 供应商配置失败: %w", err)
	}

	s.providers = make([]OpenCodeProvider, 0, len(envelope.Providers))
	seen := make(map[string]bool, len(envelope.Providers))
	for _, provider := range envelope.Providers {
		provider = normalizeOpenCodeProvider(provider)
		if provider.ID == "" || seen[provider.ID] {
			continue
		}
		seen[provider.ID] = true
		s.providers = append(s.providers, provider)
	}
	return nil
}

func getOpenCodePresets() []OpenCodeProviderPreset {
	return []OpenCodeProviderPreset{
		{
			Name:        "OpenAI Compatible",
			WebsiteURL:  "",
			NPM:         "@ai-sdk/openai-compatible",
			BaseURL:     "https://api.example.com/v1",
			Description: "OpenAI Compatible API for OpenCode",
			Category:    "custom",
			SettingsConfig: map[string]any{
				"npm": "@ai-sdk/openai-compatible",
				"options": map[string]any{
					"baseURL": "https://api.example.com/v1",
					"apiKey":  "",
				},
				"models": map[string]any{
					"gpt-4o": map[string]any{"name": "GPT-4o"},
				},
			},
		},
		{
			Name:       "Anthropic",
			WebsiteURL: "https://www.anthropic.com",
			NPM:        "@ai-sdk/anthropic",
			BaseURL:    "https://api.anthropic.com/v1",
			Category:   "official",
			SettingsConfig: map[string]any{
				"npm":  "@ai-sdk/anthropic",
				"name": "Anthropic",
				"options": map[string]any{
					"baseURL": "https://api.anthropic.com/v1",
					"apiKey":  "",
				},
				"models": map[string]any{
					"claude-3-5-sonnet-latest": map[string]any{"name": "Claude 3.5 Sonnet"},
				},
			},
		},
		{
			Name:       "DeepSeek",
			WebsiteURL: "https://www.deepseek.com",
			NPM:        "@ai-sdk/openai-compatible",
			BaseURL:    "https://api.deepseek.com/v1",
			Category:   "third_party",
			SettingsConfig: map[string]any{
				"npm":  "@ai-sdk/openai-compatible",
				"name": "DeepSeek",
				"options": map[string]any{
					"baseURL": "https://api.deepseek.com/v1",
					"apiKey":  "",
				},
				"models": map[string]any{
					"deepseek-chat": map[string]any{"name": "DeepSeek Chat"},
				},
			},
		},
	}
}

func normalizeOpenCodeProvider(provider OpenCodeProvider) OpenCodeProvider {
	provider.ID = normalizeOpenCodeProviderID(provider.ID)
	provider.Name = strings.TrimSpace(provider.Name)
	provider.WebsiteURL = strings.TrimSpace(provider.WebsiteURL)
	provider.APIKeyURL = strings.TrimSpace(provider.APIKeyURL)
	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	provider.APIKey = strings.TrimSpace(provider.APIKey)
	provider.NPM = strings.TrimSpace(provider.NPM)
	provider.Icon = strings.TrimSpace(provider.Icon)
	provider.Description = strings.TrimSpace(provider.Description)
	provider.Category = strings.TrimSpace(provider.Category)
	provider.PartnerPromotionKey = strings.TrimSpace(provider.PartnerPromotionKey)
	if provider.Level <= 0 {
		provider.Level = 1
	}
	provider.LiveConfigManaged = normalizeOpenCodeBoolPtr(provider.LiveConfigManaged)
	provider.IsInConfig = normalizeOpenCodeBoolPtr(provider.IsInConfig)
	if provider.NPM == "" {
		provider.NPM = extractOpenCodeNPM(provider.SettingsConfig)
	}
	if provider.NPM == "" {
		provider.NPM = "@ai-sdk/openai-compatible"
	}
	provider.SettingsConfig = buildOpenCodeSettingsConfig(provider)
	return provider
}

func normalizeOpenCodeProviderID(value string) string {
	return strings.TrimSpace(value)
}

func normalizeOpenCodeBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	normalized := *value
	return &normalized
}

func shouldManageOpenCodeLiveProvider(provider OpenCodeProvider) bool {
	if provider.LiveConfigManaged != nil {
		return *provider.LiveConfigManaged
	}
	if provider.IsInConfig != nil {
		return *provider.IsInConfig
	}
	return provider.Enabled
}

func boolPtr(value bool) *bool {
	return &value
}

func normalizeOpenCodeProviderIDForCreate(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ' || r == '.':
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func openCodeProviderIDExists(providers []OpenCodeProvider, id string) bool {
	id = normalizeOpenCodeProviderID(id)
	if id == "" {
		return false
	}
	for _, provider := range providers {
		if provider.ID == id {
			return true
		}
	}
	return false
}

func openCodeLiveProviderIDExists(providers map[string]map[string]any, id string) bool {
	id = normalizeOpenCodeProviderID(id)
	if id == "" {
		return false
	}
	_, ok := providers[id]
	return ok
}

func buildOpenCodeSettingsConfig(provider OpenCodeProvider) map[string]any {
	settings := cloneAnyMap(provider.SettingsConfig)
	if settings == nil {
		settings = map[string]any{}
	}
	settings["npm"] = provider.NPM
	if provider.Name != "" {
		settings["name"] = provider.Name
	}

	options := mapFromAny(settings["options"])
	if options == nil {
		options = map[string]any{}
	}
	delete(options, "baseURL")
	delete(options, "baseUrl")
	delete(options, "url")
	delete(options, "apiKey")
	delete(options, "api_key")
	delete(options, "APIKey")
	if provider.BaseURL != "" {
		options["baseURL"] = provider.BaseURL
	}
	if provider.APIKey != "" {
		options["apiKey"] = provider.APIKey
	}
	if len(options) > 0 {
		settings["options"] = options
	} else {
		delete(settings, "options")
	}

	if _, ok := settings["models"].(map[string]any); !ok {
		settings["models"] = defaultOpenCodeModels(provider.NPM)
	}
	return settings
}

func defaultOpenCodeModels(npm string) map[string]any {
	switch strings.TrimSpace(npm) {
	case "@ai-sdk/anthropic":
		return map[string]any{"claude-3-5-sonnet-latest": map[string]any{"name": "Claude 3.5 Sonnet"}}
	case "@ai-sdk/google":
		return map[string]any{"gemini-2.5-pro": map[string]any{"name": "Gemini 2.5 Pro"}}
	default:
		return map[string]any{"gpt-4o": map[string]any{"name": "GPT-4o"}}
	}
}

func getOpenCodeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config", "opencode")
	}
	return filepath.Join(home, ".config", "opencode")
}

func getOpenCodeConfigPath() string {
	return filepath.Join(getOpenCodeDir(), "opencode.json")
}

func readOpenCodeManagedProvidersBytes() ([]byte, bool, error) {
	path, err := providerConfigPath("opencode", true)
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

func restoreOpenCodeManagedProvidersBytes(data []byte, exists bool) error {
	path, err := providerConfigPath("opencode", true)
	if err != nil {
		return err
	}
	if !exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(path, data, 0o644)
}

func readOpenCodeLiveConfigBytes() ([]byte, bool, error) {
	path := getOpenCodeConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

func restoreOpenCodeLiveConfigBytes(data []byte, exists bool) error {
	path := getOpenCodeConfigPath()
	if !exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func readOpenCodeLiveConfig() (map[string]any, error) {
	path := getOpenCodeConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{"$schema": "https://opencode.ai/config.json"}, nil
		}
		return nil, err
	}

	var config map[string]any
	if err := unmarshalOpenCodeLiveConfig(data, &config); err != nil {
		return nil, fmt.Errorf("解析 OpenCode 配置失败 (%s): %w", path, err)
	}
	if config == nil {
		config = map[string]any{}
	}
	return config, nil
}

func unmarshalOpenCodeLiveConfig(data []byte, target any) error {
	if err := json.Unmarshal(data, target); err == nil {
		return nil
	}

	jsonData := jsonc.ToJSON(data)
	if err := json.Unmarshal(jsonData, target); err == nil {
		return nil
	}

	cleanedData := stripJSONTrailingCommas(jsonData)
	return json.Unmarshal(cleanedData, target)
}

func stripJSONTrailingCommas(data []byte) []byte {
	result := make([]byte, 0, len(data))
	inString := false
	escaped := false

	for index := 0; index < len(data); index++ {
		char := data[index]
		if inString {
			result = append(result, char)
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == '"' {
				inString = false
			}
			continue
		}

		if char == '"' {
			inString = true
			result = append(result, char)
			continue
		}

		if char == ',' {
			nextIndex := index + 1
			for nextIndex < len(data) && isJSONWhitespace(data[nextIndex]) {
				nextIndex++
			}
			if nextIndex < len(data) && (data[nextIndex] == '}' || data[nextIndex] == ']') {
				continue
			}
		}

		result = append(result, char)
	}

	return result
}

func isJSONWhitespace(char byte) bool {
	return char == ' ' || char == '\n' || char == '\r' || char == '\t'
}

func writeOpenCodeLiveConfig(config map[string]any) error {
	dir := getOpenCodeDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getOpenCodeConfigPath(), data, 0o644)
}

func readOpenCodeLiveProviders() (map[string]map[string]any, error) {
	config, err := readOpenCodeLiveConfig()
	if err != nil {
		return nil, err
	}
	providers := map[string]map[string]any{}
	rawProviders := mapFromAny(config["provider"])
	for id, raw := range rawProviders {
		fragment := mapFromAny(raw)
		if fragment == nil {
			continue
		}
		providers[id] = fragment
	}
	return providers, nil
}

func writeOpenCodeLiveProviders(providers []OpenCodeProvider) error {
	config, err := readOpenCodeLiveConfig()
	if err != nil {
		return err
	}
	applyOpenCodeLiveProvidersToConfig(config, providers)
	return writeOpenCodeLiveConfig(config)
}

func applyOpenCodeLiveProvidersToConfig(config map[string]any, providers []OpenCodeProvider) {
	rawProviders := mapFromAny(config["provider"])
	if rawProviders == nil {
		rawProviders = map[string]any{}
	}

	for _, provider := range providers {
		provider = normalizeOpenCodeProvider(provider)
		if provider.ID == "" {
			continue
		}
		if !shouldManageOpenCodeLiveProvider(provider) {
			continue
		}
		rawProviders[provider.ID] = provider.SettingsConfig
	}

	config["provider"] = rawProviders
}

func removeMissingOpenCodeLiveProvidersFromConfig(config map[string]any, previousProviders []OpenCodeProvider, nextProviders []OpenCodeProvider) {
	rawProviders := mapFromAny(config["provider"])
	if rawProviders == nil {
		return
	}
	nextManaged := make(map[string]bool, len(nextProviders))
	for _, provider := range nextProviders {
		provider = normalizeOpenCodeProvider(provider)
		if provider.ID != "" && shouldManageOpenCodeLiveProvider(provider) {
			nextManaged[provider.ID] = true
		}
	}
	for _, provider := range previousProviders {
		provider = normalizeOpenCodeProvider(provider)
		if provider.ID == "" || !shouldManageOpenCodeLiveProvider(provider) || nextManaged[provider.ID] {
			continue
		}
		delete(rawProviders, provider.ID)
	}
	config["provider"] = rawProviders
}

func removeOpenCodeLiveProvider(id string) error {
	config, err := readOpenCodeLiveConfig()
	if err != nil {
		return err
	}
	rawProviders := mapFromAny(config["provider"])
	if rawProviders == nil {
		return nil
	}
	delete(rawProviders, normalizeOpenCodeProviderID(id))
	config["provider"] = rawProviders
	return writeOpenCodeLiveConfig(config)
}

func importOpenCodeProvidersFromLiveSnapshot() ([]OpenCodeProvider, error) {
	liveProviders, err := readOpenCodeLiveProviders()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(liveProviders))
	for id := range liveProviders {
		keys = append(keys, id)
	}
	sort.Strings(keys)

	providers := make([]OpenCodeProvider, 0, len(keys))
	for _, rawID := range keys {
		id := normalizeOpenCodeProviderID(rawID)
		if id == "" {
			continue
		}
		fragment := liveProviders[rawID]
		providers = append(providers, normalizeOpenCodeProvider(OpenCodeProvider{
			ID:                id,
			Name:              resolveOpenCodeProviderName(rawID, fragment),
			BaseURL:           extractOpenCodeBaseURL(fragment),
			APIKey:            extractOpenCodeAPIKey(fragment),
			NPM:               extractOpenCodeNPM(fragment),
			Enabled:           true,
			LiveConfigManaged: boolPtr(true),
			IsInConfig:        boolPtr(true),
			Level:             1,
			SettingsConfig:    cloneAnyMap(fragment),
		}))
	}
	return providers, nil
}

func resolveOpenCodeProviderName(id string, settings map[string]any) string {
	if name, ok := settings["name"].(string); ok && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return id
}

func extractOpenCodeNPM(settings map[string]any) string {
	if settings == nil {
		return ""
	}
	if npm, ok := settings["npm"].(string); ok {
		return strings.TrimSpace(npm)
	}
	return ""
}

func extractOpenCodeBaseURL(settings map[string]any) string {
	options := mapFromAny(settings["options"])
	for _, key := range []string{"baseURL", "baseUrl", "url"} {
		if value, ok := options[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func extractOpenCodeAPIKey(settings map[string]any) string {
	options := mapFromAny(settings["options"])
	for _, key := range []string{"apiKey", "api_key", "APIKey"} {
		if value, ok := options[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return cloneAnyMap(typed)
	}
	if typed, ok := value.(map[string]interface{}); ok {
		return cloneAnyMap(typed)
	}
	return nil
}

func cloneAnyMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var cloned map[string]any
	if err := json.Unmarshal(data, &cloned); err != nil || cloned == nil {
		return map[string]any{}
	}
	return cloned
}

func cloneOpenCodeProviders(providers []OpenCodeProvider) []OpenCodeProvider {
	cloned := make([]OpenCodeProvider, len(providers))
	for i, provider := range providers {
		cloned[i] = provider
		cloned[i].SettingsConfig = cloneAnyMap(provider.SettingsConfig)
		cloned[i].RequestBodyOverrides = cloneAnyMap(provider.RequestBodyOverrides)
		cloned[i].BudgetQuotaSettings = cloneBudgetQuotaSettingsPtr(provider.BudgetQuotaSettings)
		cloned[i].BudgetQuotaUsedAdjustments = cloneBudgetQuotaAdjustmentsPtr(provider.BudgetQuotaUsedAdjustments)
		cloned[i].ProviderQuotaQueryConfig = cloneProviderQuotaQueryConfigPtr(provider.ProviderQuotaQueryConfig)
	}
	return cloned
}

func cloneBudgetQuotaSettingsPtr(value *BudgetQuotaSettings) *BudgetQuotaSettings {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneBudgetQuotaAdjustmentsPtr(value *BudgetQuotaAdjustments) *BudgetQuotaAdjustments {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneProviderQuotaQueryConfigPtr(value *ProviderQuotaQueryConfig) *ProviderQuotaQueryConfig {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var cloned ProviderQuotaQueryConfig
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil
	}
	return &cloned
}
