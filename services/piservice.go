/**
 * @name: Pi 配置服务
 * @Descripttion: 管理 ~/.pi/agent/models.json 的 additive 供应商切换（providers 子树替换、未知键保留）与首次导入
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 01:25:00
 * @LastEditTime: 2026-08-17 01:25:00
 * @FilePath: services/piservice.go
 */

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
)

const (
	// Pi 配置目录（~/.pi/agent；同目录的 settings.json / auth.json 不由本服务管理）
	piDirName         = ".pi"
	piAgentChildDir   = "agent"
	piModelsFileName  = "models.json"
)

// PiProvider Pi 供应商配置
// live 条目写入 ~/.pi/agent/models.json 的顶层 providers.<id> 节（camelCase：
// {displayName, baseUrl, apiKey}），CLIConfig 保存该节的原生片段（api/models/compat 等导入时无损快照）
// Model 仅为本应用侧元数据（卡片默认模型展示），不写入 live 条目（Pi 的模型清单由 models 数组描述）
type PiProvider struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	BaseURL   string         `json:"baseUrl,omitempty"` // 对应 live 条目的 baseUrl
	APIKey    string         `json:"apiKey,omitempty"`  // 对应 live 条目的 apiKey
	Model     string         `json:"model,omitempty"`   // 应用侧默认模型（不写入 live）
	Enabled   bool           `json:"enabled"`           // 当前启用（additive 模式下的选中标记）
	Level     int            `json:"level,omitempty"`
	Category  string         `json:"category,omitempty"`
	CLIConfig map[string]any `json:"cliConfig,omitempty"` // live 原生片段
}

// PiService Pi 配置服务（additive 共存模式：所有条目共存于原生配置，切换 = 标记启用）
type PiService struct {
	mu        sync.Mutex
	providers []PiProvider
}

// NewPiService 创建 Pi 配置服务（统一存储为空时自动从 live 首次导入）
func NewPiService() *PiService {
	service := &PiService{}
	if err := service.loadProviders(); err != nil {
		log.Printf("Pi providers load failed: %v", err)
	}
	return service
}

func (s *PiService) Start() error { return nil }
func (s *PiService) Stop() error  { return nil }

// GetProviders 获取全部供应商（统一存储视图）
func (s *PiService) GetProviders() []PiProvider {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clonePiProviders(s.providers)
}

// AddProvider 新增供应商并写入 live 条目（ID 缺省生成 pi-<unixnano>）
// live 冲突守卫：目标 ID 已存在于 live providers 时拒绝，需先走导入纳入管理
// 返回落库后的完整供应商（含生成的 ID），供前端精确回填
func (s *PiService) AddProvider(provider PiProvider) (PiProvider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider = normalizePiProvider(provider)
	if provider.ID == "" {
		provider.ID = newPiProviderID()
	}
	for _, existing := range s.providers {
		if existing.ID == provider.ID {
			return PiProvider{}, fmt.Errorf("Pi 供应商 ID '%s' 已存在", provider.ID)
		}
	}
	liveProviders, err := readPiLiveProviders()
	if err != nil {
		return PiProvider{}, err
	}
	if _, exists := liveProviders[provider.ID]; exists {
		return PiProvider{}, fmt.Errorf("Pi live 配置中已存在 provider '%s'，请先使用导入功能纳入管理，避免覆盖用户手写配置", provider.ID)
	}

	previous := clonePiProviders(s.providers)
	s.providers = append(s.providers, provider)
	if err := s.syncLiveAndSave(previous); err != nil {
		s.providers = previous
		return PiProvider{}, err
	}
	return provider, nil
}

// UpdateProvider 更新供应商（live 条目整体替换为最新字段），返回更新后的完整供应商
func (s *PiService) UpdateProvider(provider PiProvider) (PiProvider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider = normalizePiProvider(provider)
	if provider.ID == "" {
		return PiProvider{}, fmt.Errorf("Pi 供应商 ID 不能为空")
	}
	for i, existing := range s.providers {
		if existing.ID != provider.ID {
			continue
		}
		previous := clonePiProviders(s.providers)
		s.providers[i] = provider
		if err := s.syncLiveAndSave(previous); err != nil {
			s.providers[i] = existing
			return PiProvider{}, err
		}
		return provider, nil
	}
	return PiProvider{}, fmt.Errorf("未找到 ID 为 '%s' 的 Pi 供应商", provider.ID)
}

// DeleteProvider 删除供应商（同步移除 live 条目）
func (s *PiService) DeleteProvider(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	for i, provider := range s.providers {
		if provider.ID != id {
			continue
		}
		previous := clonePiProviders(s.providers)
		s.providers = append(s.providers[:i], s.providers[i+1:]...)
		if err := s.syncLiveAndSave(previous); err != nil {
			s.providers = previous
			return err
		}
		return nil
	}
	return fmt.Errorf("未找到 ID 为 '%s' 的 Pi 供应商", id)
}

// DuplicateProvider 复制供应商（副本默认未启用，生成新 ID）
func (s *PiService) DuplicateProvider(sourceID string) (*PiProvider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourceID = strings.TrimSpace(sourceID)
	var source *PiProvider
	for i := range s.providers {
		if s.providers[i].ID == sourceID {
			source = &s.providers[i]
			break
		}
	}
	if source == nil {
		return nil, fmt.Errorf("未找到 ID 为 '%s' 的 Pi 供应商", sourceID)
	}

	clone := *source
	clone.ID = newPiProviderID()
	for piProviderIDExists(s.providers, clone.ID) {
		clone.ID = newPiProviderID()
	}
	clone.Name = source.Name + " (副本)"
	clone.Enabled = false
	clone.CLIConfig = cloneAnyMap(source.CLIConfig)

	previous := clonePiProviders(s.providers)
	s.providers = append(s.providers, clone)
	if err := s.syncLiveAndSave(previous); err != nil {
		s.providers = previous
		return nil, err
	}
	return &clone, nil
}

// SetCurrentProvider 切换语义：models.json 的 providers 无全局 default 概念，
// 仅统一存储单选标记 + 同步 live 托管子树（全部条目共存保留）
func (s *PiService) SetCurrentProvider(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	found := false
	for i := range s.providers {
		if s.providers[i].ID == id {
			s.providers[i].Enabled = true
			found = true
		} else {
			s.providers[i].Enabled = false
		}
	}
	if !found {
		return fmt.Errorf("未找到 ID 为 '%s' 的 Pi 供应商", id)
	}
	previous := clonePiProviders(s.providers)
	if err := s.syncLiveAndSave(previous); err != nil {
		// 回滚内存中的单选标记
		s.providers = previous
		return err
	}
	return nil
}

// ImportFromLive 导入 live providers 的全部条目为供应商（导入后不修改原生文件）
func (s *PiService) ImportFromLive() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	liveProviders, err := readPiLiveProviders()
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

	for _, id := range keys {
		if strings.TrimSpace(id) == "" || existingIDs[id] {
			continue
		}
		fragment := liveProviders[id]
		s.providers = append(s.providers, buildPiProviderFromFragment(id, fragment))
		existingIDs[id] = true
		imported++
	}

	if imported == 0 {
		return 0, nil
	}
	return imported, SavePiProvidersToStore(s.providers)
}

// GetStatus live 配置状态摘要（configExists / providers 数 / 当前启用）
func (s *PiService) GetStatus() (map[string]any, error) {
	liveProviders, err := readPiLiveProviders()
	if err != nil {
		return nil, err
	}
	status := map[string]any{
		"configExists":  providerConfigFileExists(getPiModelsConfigPath()),
		"providerCount": len(liveProviders),
	}
	s.mu.Lock()
	for _, provider := range s.providers {
		if provider.Enabled {
			status["currentProviderId"] = provider.ID
			status["currentProviderName"] = provider.Name
			break
		}
	}
	s.mu.Unlock()
	return status, nil
}

// ========== 内部实现 ==========

// syncLiveAndSave 先同步 live（替换本应用 providers 子树）再落库；落库失败时回滚 live 快照
// 调用方负责回滚内存列表（previous 为同步前的内存快照）
func (s *PiService) syncLiveAndSave(previous []PiProvider) error {
	previousLiveData, previousLiveExists, err := readPiLiveConfigBytes()
	if err != nil {
		return err
	}
	if err := syncPiLiveProviders(previous, s.providers); err != nil {
		return err
	}
	if err := SavePiProvidersToStore(s.providers); err != nil {
		if rollbackErr := restorePiLiveConfigBytes(previousLiveData, previousLiveExists); rollbackErr != nil {
			return fmt.Errorf("保存 Pi 供应商失败: %w；回滚 live 配置也失败: %v", err, rollbackErr)
		}
		return err
	}
	return nil
}

// loadProviders 从统一存储加载；为空时从 live 首次导入（不修改原生文件）
func (s *PiService) loadProviders() error {
	providers, err := LoadPiProvidersFromStore()
	if err != nil {
		return err
	}

	if providers == nil {
		// 统一存储未初始化（无任何行）：尝试从 live 配置首次导入
		// 注意区分空哨兵（已初始化但列表为空，Load 返回空切片）：用户主动清空后不应再触发导入
		liveProviders, liveErr := importPiProvidersFromLiveSnapshot()
		if liveErr != nil {
			log.Printf("Pi live providers import skipped: %v", liveErr)
			s.providers = []PiProvider{}
			return nil
		}
		s.providers = liveProviders
		if len(liveProviders) > 0 {
			return SavePiProvidersToStore(liveProviders)
		}
		return nil
	}

	s.providers = make([]PiProvider, 0, len(providers))
	seen := make(map[string]bool, len(providers))
	for _, provider := range providers {
		provider = normalizePiProvider(provider)
		if provider.ID == "" || seen[provider.ID] {
			continue
		}
		seen[provider.ID] = true
		s.providers = append(s.providers, provider)
	}
	return nil
}

// readPiLiveProviders 读取 live 顶层 providers 全部条目（id → 原生片段）
func readPiLiveProviders() (map[string]map[string]any, error) {
	config, err := readPiLiveMap()
	if err != nil {
		return nil, err
	}
	providers := map[string]map[string]any{}
	rawProviders, _ := config["providers"].(map[string]any)
	for id, value := range rawProviders {
		fragment, ok := value.(map[string]any)
		if !ok {
			continue
		}
		providers[id] = fragment
	}
	return providers, nil
}

// syncPiLiveProviders 替换本应用 providers 子树：
// 全量写入 next 条目，删除 previous 中已移除的条目，保留用户手工添加的其他条目与顶层键
func syncPiLiveProviders(previous, next []PiProvider) error {
	config, err := readPiLiveMap()
	if err != nil {
		return err
	}
	rawProviders := piChildMap(config, "providers")

	nextIDs := make(map[string]bool, len(next))
	for _, provider := range next {
		provider = normalizePiProvider(provider)
		if provider.ID == "" {
			continue
		}
		nextIDs[provider.ID] = true
		rawProviders[provider.ID] = buildPiLiveEntry(provider)
	}
	for _, provider := range previous {
		provider = normalizePiProvider(provider)
		if provider.ID == "" || nextIDs[provider.ID] {
			continue
		}
		delete(rawProviders, provider.ID)
	}
	return writePiLiveMap(config)
}

// buildPiLiveEntry 构造 live 条目（托管字段 displayName/baseUrl/apiKey 以结构化字段为准）
// api/models/compat/headers 等原生键来自 CLIConfig 快照，随更新无损往返
func buildPiLiveEntry(provider PiProvider) map[string]any {
	entry := cloneAnyMap(provider.CLIConfig)
	if entry == nil {
		entry = map[string]any{}
	}
	delete(entry, "displayName")
	if name := strings.TrimSpace(provider.Name); name != "" {
		entry["displayName"] = name
	}
	delete(entry, "baseUrl")
	if provider.BaseURL != "" {
		entry["baseUrl"] = provider.BaseURL
	}
	delete(entry, "apiKey")
	if provider.APIKey != "" {
		entry["apiKey"] = provider.APIKey
	}
	return entry
}

// buildPiProviderFromFragment 从 live 原生片段构造供应商（导入路径）
func buildPiProviderFromFragment(id string, fragment map[string]any) PiProvider {
	return normalizePiProvider(PiProvider{
		ID:        id,
		Name:      resolvePiProviderName(id, fragment),
		BaseURL:   extractPiString(fragment, "baseUrl"),
		APIKey:    extractPiString(fragment, "apiKey"),
		Enabled:   true,
		Category:  "custom",
		CLIConfig: cloneAnyMap(fragment),
	})
}

// importPiProvidersFromLiveSnapshot 全量导入 live 条目（构造器首次接入路径）
func importPiProvidersFromLiveSnapshot() ([]PiProvider, error) {
	liveProviders, err := readPiLiveProviders()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(liveProviders))
	for id := range liveProviders {
		keys = append(keys, id)
	}
	sort.Strings(keys)

	providers := make([]PiProvider, 0, len(keys))
	for _, id := range keys {
		if strings.TrimSpace(id) == "" {
			continue
		}
		providers = append(providers, buildPiProviderFromFragment(id, liveProviders[id]))
	}
	return providers, nil
}

// resolvePiProviderName 条目显示名：displayName 优先，缺失时回退条目 ID
func resolvePiProviderName(id string, fragment map[string]any) string {
	if name := extractPiString(fragment, "displayName"); name != "" {
		return name
	}
	return id
}

func newPiProviderID() string {
	return fmt.Sprintf("pi-%d", time.Now().UnixNano())
}

func extractPiString(fragment map[string]any, key string) string {
	if fragment == nil {
		return ""
	}
	value, _ := fragment[key].(string)
	return strings.TrimSpace(value)
}

func normalizePiProvider(provider PiProvider) PiProvider {
	provider.ID = strings.TrimSpace(provider.ID)
	provider.Name = strings.TrimSpace(provider.Name)
	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	provider.APIKey = strings.TrimSpace(provider.APIKey)
	provider.Model = strings.TrimSpace(provider.Model)
	provider.Category = strings.TrimSpace(provider.Category)
	if provider.Category == "" {
		provider.Category = "custom"
	}
	if provider.Level <= 0 {
		provider.Level = 1
	}
	return provider
}

func piProviderIDExists(providers []PiProvider, id string) bool {
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

func clonePiProviders(providers []PiProvider) []PiProvider {
	cloned := make([]PiProvider, len(providers))
	for i, provider := range providers {
		cloned[i] = provider
		cloned[i].CLIConfig = cloneAnyMap(provider.CLIConfig)
	}
	return cloned
}

// piChildMap 取子 map（不存在时创建并挂到父节点，供原地修改后写回）
func piChildMap(parent map[string]any, key string) map[string]any {
	child, _ := parent[key].(map[string]any)
	if child == nil {
		child = map[string]any{}
		parent[key] = child
	}
	return child
}

// ========== live 文件读写（标准 JSON，原子写） ==========

// getPiAgentDir Pi agent 配置目录（~/.pi/agent）
func getPiAgentDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", piDirName, piAgentChildDir)
	}
	return filepath.Join(home, piDirName, piAgentChildDir)
}

// getPiModelsConfigPath Pi 模型配置文件路径（~/.pi/agent/models.json）
func getPiModelsConfigPath() string {
	return filepath.Join(getPiAgentDir(), piModelsFileName)
}

// readPiLiveMap 读取 live 配置为通用 map（保留顶层全部键）
// 文件不存在或为空视为空配置
func readPiLiveMap() (map[string]any, error) {
	data, err := os.ReadFile(getPiModelsConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析 Pi 配置失败 (%s): %w", getPiModelsConfigPath(), err)
	}
	if config == nil {
		config = map[string]any{}
	}
	return config, nil
}

// writePiLiveMap 原子写回 live 配置（标准 JSON 两空格缩进）
// 统一走 atomicWriteFile（临时文件 + fsync + rename），崩溃不会留下半写入文件
func writePiLiveMap(config map[string]any) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(getPiModelsConfigPath(), data, 0o644)
}

// readPiLiveConfigBytes 快照 live 配置原始字节（事务回滚用）
func readPiLiveConfigBytes() ([]byte, bool, error) {
	path := getPiModelsConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

// restorePiLiveConfigBytes 恢复 live 配置快照（原不存在则删除）
func restorePiLiveConfigBytes(data []byte, exists bool) error {
	path := getPiModelsConfigPath()
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
