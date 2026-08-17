/**
 * @name: OpenClaw 配置服务
 * @Descripttion: 管理 ~/.openclaw/openclaw.json 的 additive 供应商切换、env/tools/agents 子页配置与首次导入
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 12:10:00
 * @LastEditTime: 2026-08-17 12:10:00
 * @FilePath: services/openclawservice.go
 */

package services

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// OpenClawProvider OpenClaw 供应商配置
// live 条目写入 ~/.openclaw/openclaw.json 的 models.providers.<id> 节（camelCase：
// {name, baseUrl, apiKey, model?}），CLIConfig 保存该节的原生片段（导入时无损快照）
type OpenClawProvider struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	APIURL    string         `json:"baseUrl,omitempty"` // 对应 live 条目的 baseUrl
	APIKey    string         `json:"apiKey,omitempty"`  // 对应 live 条目的 apiKey
	Model     string         `json:"model,omitempty"`   // 可选默认模型
	Enabled   bool           `json:"enabled"`           // 当前启用（additive 模式下的选中标记）
	Level     int            `json:"level,omitempty"`
	Category  string         `json:"category,omitempty"`
	CLIConfig map[string]any `json:"cliConfig,omitempty"` // live 原生片段
}

// OpenClawEnvConfig 顶层 env 节（vars 注入进程环境 / shellEnv 注入 shell 会话）
type OpenClawEnvConfig struct {
	Vars     map[string]string `json:"vars"`
	ShellEnv map[string]string `json:"shellEnv"`
}

// OpenClawToolsConfig 顶层 tools 节（profile + allow/deny 命令清单）
type OpenClawToolsConfig struct {
	Profile string   `json:"profile"`
	Allow   []string `json:"allow"`
	Deny    []string `json:"deny"`
}

// OpenClawService OpenClaw 配置服务（additive 共存模式：所有条目共存于原生配置，切换 = 标记启用）
type OpenClawService struct {
	mu        sync.Mutex
	providers []OpenClawProvider
}

// NewOpenClawService 创建 OpenClaw 配置服务（统一存储为空时自动从 live 首次导入）
func NewOpenClawService() *OpenClawService {
	service := &OpenClawService{}
	if err := service.loadProviders(); err != nil {
		log.Printf("OpenClaw providers load failed: %v", err)
	}
	return service
}

func (s *OpenClawService) Start() error { return nil }
func (s *OpenClawService) Stop() error  { return nil }

// GetProviders 获取全部供应商（统一存储视图）
func (s *OpenClawService) GetProviders() []OpenClawProvider {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneOpenClawProviders(s.providers)
}

// AddProvider 新增供应商并写入 live 条目（ID 缺省生成 openclaw-<unixnano>）
// live 冲突守卫：目标 ID 已存在于 live 时拒绝，需先走导入纳入管理
// 返回落库后的完整供应商（含生成的 ID），供前端精确回填
func (s *OpenClawService) AddProvider(provider OpenClawProvider) (OpenClawProvider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider = normalizeOpenClawProvider(provider)
	if provider.ID == "" {
		provider.ID = fmt.Sprintf("openclaw-%d", time.Now().UnixNano())
	}
	for _, existing := range s.providers {
		if existing.ID == provider.ID {
			return OpenClawProvider{}, fmt.Errorf("OpenClaw 供应商 ID '%s' 已存在", provider.ID)
		}
	}
	liveProviders, err := readOpenClawLiveProviders()
	if err != nil {
		return OpenClawProvider{}, err
	}
	if _, exists := liveProviders[provider.ID]; exists {
		return OpenClawProvider{}, fmt.Errorf("OpenClaw live 配置中已存在 provider '%s'，请先使用导入功能纳入管理，避免覆盖用户手写配置", provider.ID)
	}

	previous := cloneOpenClawProviders(s.providers)
	s.providers = append(s.providers, provider)
	if err := s.syncLiveAndSave(previous); err != nil {
		s.providers = previous
		return OpenClawProvider{}, err
	}
	return provider, nil
}

// UpdateProvider 更新供应商（live 条目整体替换为最新字段），返回更新后的完整供应商
func (s *OpenClawService) UpdateProvider(provider OpenClawProvider) (OpenClawProvider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider = normalizeOpenClawProvider(provider)
	if provider.ID == "" {
		return OpenClawProvider{}, fmt.Errorf("OpenClaw 供应商 ID 不能为空")
	}
	for i, existing := range s.providers {
		if existing.ID != provider.ID {
			continue
		}
		previous := cloneOpenClawProviders(s.providers)
		s.providers[i] = provider
		if err := s.syncLiveAndSave(previous); err != nil {
			s.providers[i] = existing
			return OpenClawProvider{}, err
		}
		return provider, nil
	}
	return OpenClawProvider{}, fmt.Errorf("未找到 ID 为 '%s' 的 OpenClaw 供应商", provider.ID)
}

// DeleteProvider 删除供应商（同步移除 live 条目）
func (s *OpenClawService) DeleteProvider(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	for i, provider := range s.providers {
		if provider.ID != id {
			continue
		}
		previous := cloneOpenClawProviders(s.providers)
		s.providers = append(s.providers[:i], s.providers[i+1:]...)
		if err := s.syncLiveAndSave(previous); err != nil {
			s.providers = previous
			return err
		}
		return nil
	}
	return fmt.Errorf("未找到 ID 为 '%s' 的 OpenClaw 供应商", id)
}

// DuplicateProvider 复制供应商（副本默认未启用，生成新 ID）
func (s *OpenClawService) DuplicateProvider(sourceID string) (*OpenClawProvider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourceID = strings.TrimSpace(sourceID)
	var source *OpenClawProvider
	for i := range s.providers {
		if s.providers[i].ID == sourceID {
			source = &s.providers[i]
			break
		}
	}
	if source == nil {
		return nil, fmt.Errorf("未找到 ID 为 '%s' 的 OpenClaw 供应商", sourceID)
	}

	clone := *source
	clone.ID = fmt.Sprintf("openclaw-%d", time.Now().UnixNano())
	for openClawProviderIDExists(s.providers, clone.ID) {
		clone.ID = fmt.Sprintf("openclaw-%d", time.Now().UnixNano())
	}
	clone.Name = source.Name + " (副本)"
	clone.Enabled = false
	clone.CLIConfig = cloneAnyMap(source.CLIConfig)

	previous := cloneOpenClawProviders(s.providers)
	s.providers = append(s.providers, clone)
	if err := s.syncLiveAndSave(previous); err != nil {
		s.providers = previous
		return nil, err
	}
	return &clone, nil
}

// SetCurrentProvider 切换语义：写 live 该条目（additive 共存，其余条目保留）+ 统一存储单选标记
func (s *OpenClawService) SetCurrentProvider(id string) error {
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
		return fmt.Errorf("未找到 ID 为 '%s' 的 OpenClaw 供应商", id)
	}
	previous := cloneOpenClawProviders(s.providers)
	if err := s.syncLiveAndSave(previous); err != nil {
		// 回滚内存中的单选标记
		s.providers = previous
		return err
	}
	return nil
}

// ImportFromLive 导入 live models.providers 的全部条目为供应商（导入后不修改原生文件）
func (s *OpenClawService) ImportFromLive() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	liveProviders, err := readOpenClawLiveProviders()
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
		s.providers = append(s.providers, buildOpenClawProviderFromFragment(id, fragment))
		existingIDs[id] = true
		imported++
	}

	if imported == 0 {
		return 0, nil
	}
	return imported, SaveOpenClawProvidersToStore(s.providers)
}

// GetStatus live 配置状态摘要（settingsPath / configExists / providers 数 / 当前启用）
func (s *OpenClawService) GetStatus() (map[string]any, error) {
	liveProviders, err := readOpenClawLiveProviders()
	if err != nil {
		return nil, err
	}
	status := map[string]any{
		"settingsPath":  getOpenClawConfigPath(),
		"configExists":  providerConfigFileExists(getOpenClawConfigPath()),
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

// ========== env 子页：顶层 env 节读写 ==========

// GetEnvConfig 读取顶层 env 节的 vars / shellEnv
func (s *OpenClawService) GetEnvConfig() (*OpenClawEnvConfig, error) {
	config, err := readOpenClawLiveMap()
	if err != nil {
		return nil, err
	}
	env := openClawChildReadOnly(config, "env")
	return &OpenClawEnvConfig{
		Vars:     openClawStringMap(env["vars"]),
		ShellEnv: openClawStringMap(env["shellEnv"]),
	}, nil
}

// SetEnvConfig 写入顶层 env 节（只改 env 键，保留 models/tools/agents 等其他顶层键）
func (s *OpenClawService) SetEnvConfig(vars map[string]string, shellEnv map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, err := readOpenClawLiveMap()
	if err != nil {
		return err
	}
	env := openClawChildMap(config, "env")
	if len(vars) > 0 {
		env["vars"] = vars
	} else {
		delete(env, "vars")
	}
	if len(shellEnv) > 0 {
		env["shellEnv"] = shellEnv
	} else {
		delete(env, "shellEnv")
	}
	return writeOpenClawLiveMap(config)
}

// ========== tools 子页：顶层 tools 节读写 ==========

// GetToolsConfig 读取顶层 tools 节（profile / allow / deny）
func (s *OpenClawService) GetToolsConfig() (*OpenClawToolsConfig, error) {
	config, err := readOpenClawLiveMap()
	if err != nil {
		return nil, err
	}
	tools := openClawChildReadOnly(config, "tools")
	profile, _ := tools["profile"].(string)
	return &OpenClawToolsConfig{
		Profile: strings.TrimSpace(profile),
		Allow:   openClawStringSlice(tools["allow"]),
		Deny:    openClawStringSlice(tools["deny"]),
	}, nil
}

// SetToolsConfig 写入顶层 tools 节（profile 限 minimal/coding/messaging/full）
func (s *OpenClawService) SetToolsConfig(profile string, allow []string, deny []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	profile = strings.TrimSpace(profile)
	switch profile {
	case "minimal", "coding", "messaging", "full":
	default:
		return fmt.Errorf("无效的 OpenClaw tools profile: %s", profile)
	}

	config, err := readOpenClawLiveMap()
	if err != nil {
		return err
	}
	tools := openClawChildMap(config, "tools")
	tools["profile"] = profile
	if len(allow) > 0 {
		tools["allow"] = allow
	} else {
		delete(tools, "allow")
	}
	if len(deny) > 0 {
		tools["deny"] = deny
	} else {
		delete(tools, "deny")
	}
	return writeOpenClawLiveMap(config)
}

// ========== agents 子页：顶层 agents.defaults 节读写 ==========

// GetAgentsConfig 读取顶层 agents.defaults（内部结构透传，前端编辑）
func (s *OpenClawService) GetAgentsConfig() (map[string]any, error) {
	config, err := readOpenClawLiveMap()
	if err != nil {
		return nil, err
	}
	agents := openClawChildReadOnly(config, "agents")
	defaults := openClawChildReadOnly(agents, "defaults")
	if len(defaults) == 0 {
		return map[string]any{}, nil
	}
	if cloned := cloneAnyMap(defaults); cloned != nil {
		return cloned, nil
	}
	return map[string]any{}, nil
}

// SetAgentsConfig 写入顶层 agents.defaults（只改 agents 键，保留其他顶层键）
func (s *OpenClawService) SetAgentsConfig(defaults map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, err := readOpenClawLiveMap()
	if err != nil {
		return err
	}
	agents := openClawChildMap(config, "agents")
	if len(defaults) == 0 {
		delete(agents, "defaults")
	} else {
		agents["defaults"] = defaults
	}
	return writeOpenClawLiveMap(config)
}

// ========== 内部实现 ==========

// syncLiveAndSave 先同步 live（替换本应用 providers 子树）再落库；落库失败时回滚 live 快照
// 调用方负责回滚内存列表（previous 为同步前的内存快照）
func (s *OpenClawService) syncLiveAndSave(previous []OpenClawProvider) error {
	previousLiveData, previousLiveExists, err := readOpenClawLiveConfigBytes()
	if err != nil {
		return err
	}
	if err := syncOpenClawLiveProviders(previous, s.providers); err != nil {
		return err
	}
	if err := SaveOpenClawProvidersToStore(s.providers); err != nil {
		if rollbackErr := restoreOpenClawLiveConfigBytes(previousLiveData, previousLiveExists); rollbackErr != nil {
			return fmt.Errorf("保存 OpenClaw 供应商失败: %w；回滚 live 配置也失败: %v", err, rollbackErr)
		}
		return err
	}
	return nil
}

// loadProviders 从统一存储加载；为空时从 live 首次导入（不修改原生文件）
func (s *OpenClawService) loadProviders() error {
	providers, err := LoadOpenClawProvidersFromStore()
	if err != nil {
		return err
	}

	if providers == nil {
		// 统一存储未初始化（无任何行）：尝试从 live 配置首次导入
		// 注意区分空哨兵（已初始化但列表为空，Load 返回空切片）：用户主动清空后不应再触发导入
		liveProviders, liveErr := importOpenClawProvidersFromLiveSnapshot()
		if liveErr != nil {
			log.Printf("OpenClaw live providers import skipped: %v", liveErr)
			s.providers = []OpenClawProvider{}
			return nil
		}
		s.providers = liveProviders
		if len(liveProviders) > 0 {
			return SaveOpenClawProvidersToStore(liveProviders)
		}
		return nil
	}

	s.providers = make([]OpenClawProvider, 0, len(providers))
	seen := make(map[string]bool, len(providers))
	for _, provider := range providers {
		provider = normalizeOpenClawProvider(provider)
		if provider.ID == "" || seen[provider.ID] {
			continue
		}
		seen[provider.ID] = true
		s.providers = append(s.providers, provider)
	}
	return nil
}

// readOpenClawLiveProviders 读取 live models.providers 全部条目（id → 原生片段）
func readOpenClawLiveProviders() (map[string]map[string]any, error) {
	config, err := readOpenClawLiveMap()
	if err != nil {
		return nil, err
	}
	providers := map[string]map[string]any{}
	models, _ := config["models"].(map[string]any)
	rawProviders, _ := models["providers"].(map[string]any)
	for id, value := range rawProviders {
		fragment, ok := value.(map[string]any)
		if !ok {
			continue
		}
		providers[id] = fragment
	}
	return providers, nil
}

// syncOpenClawLiveProviders 替换本应用 providers 子树：
// 全量写入 next 条目，删除 previous 中已移除的条目，保留用户手工添加的其他条目与顶层键
func syncOpenClawLiveProviders(previous, next []OpenClawProvider) error {
	config, err := readOpenClawLiveMap()
	if err != nil {
		return err
	}
	models := openClawChildMap(config, "models")
	rawProviders := openClawChildMap(models, "providers")

	nextIDs := make(map[string]bool, len(next))
	for _, provider := range next {
		provider = normalizeOpenClawProvider(provider)
		if provider.ID == "" {
			continue
		}
		nextIDs[provider.ID] = true
		rawProviders[provider.ID] = buildOpenClawLiveEntry(provider)
	}
	for _, provider := range previous {
		provider = normalizeOpenClawProvider(provider)
		if provider.ID == "" || nextIDs[provider.ID] {
			continue
		}
		delete(rawProviders, provider.ID)
	}
	return writeOpenClawLiveMap(config)
}

// buildOpenClawLiveEntry 构造 live 条目（camelCase；托管字段以结构化字段为准）
func buildOpenClawLiveEntry(provider OpenClawProvider) map[string]any {
	entry := cloneAnyMap(provider.CLIConfig)
	if entry == nil {
		entry = map[string]any{}
	}
	delete(entry, "name")
	if name := strings.TrimSpace(provider.Name); name != "" {
		entry["name"] = name
	}
	delete(entry, "baseUrl")
	if provider.APIURL != "" {
		entry["baseUrl"] = provider.APIURL
	}
	delete(entry, "apiKey")
	if provider.APIKey != "" {
		entry["apiKey"] = provider.APIKey
	}
	delete(entry, "model")
	if provider.Model != "" {
		entry["model"] = provider.Model
	}
	return entry
}

// buildOpenClawProviderFromFragment 从 live 原生片段构造供应商（导入路径）
func buildOpenClawProviderFromFragment(id string, fragment map[string]any) OpenClawProvider {
	return normalizeOpenClawProvider(OpenClawProvider{
		ID:        id,
		Name:      resolveOpenClawProviderName(id, fragment),
		APIURL:    extractOpenClawString(fragment, "baseUrl"),
		APIKey:    extractOpenClawString(fragment, "apiKey"),
		Model:     extractOpenClawString(fragment, "model"),
		Enabled:   true,
		Category:  "custom",
		CLIConfig: cloneAnyMap(fragment),
	})
}

// importOpenClawProvidersFromLiveSnapshot 全量导入 live 条目（构造器首次接入路径）
func importOpenClawProvidersFromLiveSnapshot() ([]OpenClawProvider, error) {
	liveProviders, err := readOpenClawLiveProviders()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(liveProviders))
	for id := range liveProviders {
		keys = append(keys, id)
	}
	sort.Strings(keys)

	providers := make([]OpenClawProvider, 0, len(keys))
	for _, id := range keys {
		if strings.TrimSpace(id) == "" {
			continue
		}
		providers = append(providers, buildOpenClawProviderFromFragment(id, liveProviders[id]))
	}
	return providers, nil
}

func resolveOpenClawProviderName(id string, fragment map[string]any) string {
	if name := extractOpenClawString(fragment, "name"); name != "" {
		return name
	}
	return id
}

func extractOpenClawString(fragment map[string]any, key string) string {
	if fragment == nil {
		return ""
	}
	value, _ := fragment[key].(string)
	return strings.TrimSpace(value)
}

func normalizeOpenClawProvider(provider OpenClawProvider) OpenClawProvider {
	provider.ID = strings.TrimSpace(provider.ID)
	provider.Name = strings.TrimSpace(provider.Name)
	provider.APIURL = strings.TrimSpace(provider.APIURL)
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

func openClawProviderIDExists(providers []OpenClawProvider, id string) bool {
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

func cloneOpenClawProviders(providers []OpenClawProvider) []OpenClawProvider {
	cloned := make([]OpenClawProvider, len(providers))
	for i, provider := range providers {
		cloned[i] = provider
		cloned[i].CLIConfig = cloneAnyMap(provider.CLIConfig)
	}
	return cloned
}

// openClawChildMap 取子 map（不存在时创建并挂到父节点，供原地修改后写回）
func openClawChildMap(parent map[string]any, key string) map[string]any {
	child, _ := parent[key].(map[string]any)
	if child == nil {
		child = map[string]any{}
		parent[key] = child
	}
	return child
}

// openClawChildReadOnly 只读取子 map（不存在返回空 map，不修改父节点）
func openClawChildReadOnly(parent map[string]any, key string) map[string]any {
	child, _ := parent[key].(map[string]any)
	if child == nil {
		return map[string]any{}
	}
	return child
}

// openClawStringMap 任意值转 string map（非字符串值格式化，nil 跳过）
func openClawStringMap(value any) map[string]string {
	result := map[string]string{}
	raw, ok := value.(map[string]any)
	if !ok {
		return result
	}
	for key, item := range raw {
		if text, ok := item.(string); ok {
			result[key] = text
			continue
		}
		if item != nil {
			result[key] = fmt.Sprint(item)
		}
	}
	return result
}

// openClawStringSlice 任意值转 string 切片（非字符串元素跳过）
func openClawStringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}
