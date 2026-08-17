/**
 * @name: Hermes 配置服务
 * @Descripttion: 管理 ~/.hermes/config.yaml 的 additive 供应商切换（YAML Node 保未知键/注释）与首次导入
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 20:30:00
 * @LastEditTime: 2026-08-17 20:30:00
 * @FilePath: services/hermesservice.go
 */

package services

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// Hermes 配置目录名（~/.hermes；Windows 官方位于 %LOCALAPPDATA%\hermes，此处统一走 home 路径）
	hermesDirName        = ".hermes"
	hermesConfigFileName = "config.yaml"
)

// hermesConfigMu 保护 config.yaml 的「读 Node → 只改目标子树 → 写回」事务串行执行
// （供应商切换 / MCP 投影 / memory 设置三条链路共用，避免互相覆盖）
var hermesConfigMu sync.Mutex

// HermesProvider Hermes 供应商配置
// live 条目写入 ~/.hermes/config.yaml 顶层 custom_providers 数组（snake_case：
// {id, name, base_url, api_key, model}），CLIConfig 保存该条目的原生片段（导入时无损快照）
type HermesProvider struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	BaseURL   string         `json:"baseUrl,omitempty"` // 对应 live 条目的 base_url
	APIKey    string         `json:"apiKey,omitempty"`  // 对应 live 条目的 api_key
	Model     string         `json:"model,omitempty"`   // 该供应商的默认模型
	Enabled   bool           `json:"enabled"`           // 当前启用（additive 模式下的选中标记）
	Level     int            `json:"level,omitempty"`
	Category  string         `json:"category,omitempty"`
	CLIConfig map[string]any `json:"cliConfig,omitempty"` // live 原生片段
}

// HermesService Hermes 配置服务（additive 共存模式：全部条目共存于 custom_providers，切换 = 更新顶层 model 节）
type HermesService struct {
	mu        sync.Mutex
	providers []HermesProvider
}

// NewHermesService 创建 Hermes 配置服务（统一存储为空时自动从 live 首次导入）
func NewHermesService() *HermesService {
	service := &HermesService{}
	if err := service.loadProviders(); err != nil {
		log.Printf("Hermes providers load failed: %v", err)
	}
	return service
}

func (s *HermesService) Start() error { return nil }
func (s *HermesService) Stop() error  { return nil }

// GetProviders 获取全部供应商（统一存储视图）
func (s *HermesService) GetProviders() []HermesProvider {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneHermesProviders(s.providers)
}

// AddProvider 新增供应商并追加 live 条目（ID 缺省生成 hermes-<unixnano>）
// live 冲突守卫：目标 ID 已存在于 custom_providers 时拒绝，需先走导入纳入管理
// 返回落库后的完整供应商（含生成的 ID），供前端精确回填
func (s *HermesService) AddProvider(provider HermesProvider) (HermesProvider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider = normalizeHermesProvider(provider)
	if provider.Name == "" {
		return HermesProvider{}, fmt.Errorf("Hermes 供应商名称不能为空")
	}
	if provider.ID == "" {
		provider.ID = newHermesProviderID()
	}
	for _, existing := range s.providers {
		if existing.ID == provider.ID {
			return HermesProvider{}, fmt.Errorf("Hermes 供应商 ID '%s' 已存在", provider.ID)
		}
	}
	liveEntries, err := readHermesLiveCustomProviders()
	if err != nil {
		return HermesProvider{}, err
	}
	for _, entry := range liveEntries {
		if extractHermesString(entry, "id") == provider.ID {
			return HermesProvider{}, fmt.Errorf("Hermes live 配置中已存在 provider '%s'，请先使用导入功能纳入管理，避免覆盖用户手写配置", provider.ID)
		}
	}

	previous := cloneHermesProviders(s.providers)
	s.providers = append(s.providers, provider)
	if err := s.syncLiveAndSave(previous, nil); err != nil {
		s.providers = previous
		return HermesProvider{}, err
	}
	return provider, nil
}

// UpdateProvider 更新供应商（live 条目整体替换为最新字段），返回更新后的完整供应商
func (s *HermesService) UpdateProvider(provider HermesProvider) (HermesProvider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider = normalizeHermesProvider(provider)
	if provider.ID == "" {
		return HermesProvider{}, fmt.Errorf("Hermes 供应商 ID 不能为空")
	}
	if provider.Name == "" {
		return HermesProvider{}, fmt.Errorf("Hermes 供应商名称不能为空")
	}
	for i, existing := range s.providers {
		if existing.ID != provider.ID {
			continue
		}
		previous := cloneHermesProviders(s.providers)
		s.providers[i] = provider
		if err := s.syncLiveAndSave(previous, nil); err != nil {
			s.providers[i] = existing
			return HermesProvider{}, err
		}
		return provider, nil
	}
	return HermesProvider{}, fmt.Errorf("未找到 ID 为 '%s' 的 Hermes 供应商", provider.ID)
}

// DeleteProvider 删除供应商（同步移除 live 条目）
func (s *HermesService) DeleteProvider(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	for i, provider := range s.providers {
		if provider.ID != id {
			continue
		}
		previous := cloneHermesProviders(s.providers)
		s.providers = append(s.providers[:i], s.providers[i+1:]...)
		if err := s.syncLiveAndSave(previous, nil); err != nil {
			s.providers = previous
			return err
		}
		return nil
	}
	return fmt.Errorf("未找到 ID 为 '%s' 的 Hermes 供应商", id)
}

// DuplicateProvider 复制供应商（副本默认未启用，生成新 ID）
func (s *HermesService) DuplicateProvider(sourceID string) (*HermesProvider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourceID = strings.TrimSpace(sourceID)
	var source *HermesProvider
	for i := range s.providers {
		if s.providers[i].ID == sourceID {
			source = &s.providers[i]
			break
		}
	}
	if source == nil {
		return nil, fmt.Errorf("未找到 ID 为 '%s' 的 Hermes 供应商", sourceID)
	}

	clone := *source
	clone.ID = newHermesProviderID()
	for hermesProviderIDExists(s.providers, clone.ID) {
		clone.ID = newHermesProviderID()
	}
	clone.Name = source.Name + " (副本)"
	clone.Enabled = false
	clone.CLIConfig = cloneAnyMap(source.CLIConfig)

	previous := cloneHermesProviders(s.providers)
	s.providers = append(s.providers, clone)
	if err := s.syncLiveAndSave(previous, nil); err != nil {
		s.providers = previous
		return nil, err
	}
	return &clone, nil
}

// SetCurrentProvider 切换语义：custom_providers 条目全部共存保留 + 顶层 model 节指向该供应商
// model 节写为 map 结构（{provider: <name>, model: <model>}，model 为空时省略 model 键）
func (s *HermesService) SetCurrentProvider(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	found := false
	var switchedTo *HermesProvider
	for i := range s.providers {
		if s.providers[i].ID == id {
			s.providers[i].Enabled = true
			switchedTo = &s.providers[i]
			found = true
		} else {
			s.providers[i].Enabled = false
		}
	}
	if !found {
		return fmt.Errorf("未找到 ID 为 '%s' 的 Hermes 供应商", id)
	}
	previous := cloneHermesProviders(s.providers)
	if err := s.syncLiveAndSave(previous, switchedTo); err != nil {
		// 回滚内存中的单选标记
		s.providers = previous
		return err
	}
	return nil
}

// ImportFromLive 导入 live custom_providers 的全部条目为供应商（导入后不修改原生文件）
func (s *HermesService) ImportFromLive() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	liveEntries, err := readHermesLiveCustomProviders()
	if err != nil {
		return 0, err
	}

	existingIDs := make(map[string]bool, len(s.providers))
	existingKeys := make(map[string]bool, len(s.providers))
	for _, provider := range s.providers {
		existingIDs[provider.ID] = true
		existingKeys[hermesEntryMatchKey(provider.Name, provider.BaseURL)] = true
	}

	currentName, _ := readHermesLiveModelSelection()
	imported := 0
	nextID := newHermesProviderID()
	for _, entry := range liveEntries {
		provider := buildHermesProviderFromFragment(entry, nextID, currentName)
		nextID = newHermesProviderID()
		if existingIDs[provider.ID] || existingKeys[hermesEntryMatchKey(provider.Name, provider.BaseURL)] {
			continue
		}
		existingIDs[provider.ID] = true
		existingKeys[hermesEntryMatchKey(provider.Name, provider.BaseURL)] = true
		s.providers = append(s.providers, provider)
		imported++
	}

	if imported == 0 {
		return 0, nil
	}
	return imported, SaveHermesProvidersToStore(s.providers)
}

// GetStatus live 配置状态摘要（configExists / providers 数 / 当前启用）
func (s *HermesService) GetStatus() (map[string]any, error) {
	liveEntries, err := readHermesLiveCustomProviders()
	if err != nil {
		return nil, err
	}
	status := map[string]any{
		"configExists":  providerConfigFileExists(getHermesConfigPath()),
		"providerCount": len(liveEntries),
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

// syncLiveAndSave 先同步 live（替换托管的 custom_providers 子树，按需更新 model 节）再落库
// 落库失败时回滚 live 快照；调用方负责回滚内存列表（previous 为同步前的内存快照）
func (s *HermesService) syncLiveAndSave(previous []HermesProvider, switchedTo *HermesProvider) error {
	previousLiveData, previousLiveExists, err := readHermesLiveConfigBytes()
	if err != nil {
		return err
	}
	if err := syncHermesLiveProviders(previous, s.providers, switchedTo); err != nil {
		return err
	}
	if err := SaveHermesProvidersToStore(s.providers); err != nil {
		if rollbackErr := restoreHermesLiveConfigBytes(previousLiveData, previousLiveExists); rollbackErr != nil {
			return fmt.Errorf("保存 Hermes 供应商失败: %w；回滚 live 配置也失败: %v", err, rollbackErr)
		}
		return err
	}
	return nil
}

// loadProviders 从统一存储加载；为空时从 live 首次导入（不修改原生文件）
func (s *HermesService) loadProviders() error {
	providers, err := LoadHermesProvidersFromStore()
	if err != nil {
		return err
	}

	if providers == nil {
		// 统一存储未初始化（无任何行）：尝试从 live 配置首次导入
		// 注意区分空哨兵（已初始化但列表为空，Load 返回空切片）：用户主动清空后不应再触发导入
		liveProviders, liveErr := importHermesProvidersFromLiveSnapshot()
		if liveErr != nil {
			log.Printf("Hermes live providers import skipped: %v", liveErr)
			s.providers = []HermesProvider{}
			return nil
		}
		s.providers = liveProviders
		if len(liveProviders) > 0 {
			return SaveHermesProvidersToStore(liveProviders)
		}
		return nil
	}

	s.providers = make([]HermesProvider, 0, len(providers))
	seen := make(map[string]bool, len(providers))
	for _, provider := range providers {
		provider = normalizeHermesProvider(provider)
		if provider.ID == "" || seen[provider.ID] {
			continue
		}
		seen[provider.ID] = true
		s.providers = append(s.providers, provider)
	}
	return nil
}

// syncHermesLiveProviders 替换本应用托管的 custom_providers 条目：
// 全量写入 next 条目，删除 previous 中已移除的条目，保留用户手工添加的其他条目与顶层键
// switchedTo 非空时同步更新顶层 model 节（切换语义）
func syncHermesLiveProviders(previous, next []HermesProvider, switchedTo *HermesProvider) error {
	hermesConfigMu.Lock()
	defer hermesConfigMu.Unlock()

	doc, err := readHermesLiveNode()
	if err != nil {
		return err
	}
	root := hermesRootMapping(doc)
	if root == nil {
		return fmt.Errorf("Hermes 配置顶层结构异常: %s", getHermesConfigPath())
	}

	seq := hermesGetTopLevelValue(root, "custom_providers")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		hermesSetTopLevelValue(root, "custom_providers", seq)
	}

	nextByID := make(map[string]HermesProvider, len(next))
	for _, provider := range next {
		if provider.ID != "" {
			nextByID[provider.ID] = provider
		}
	}
	prevIDs := make(map[string]bool, len(previous))
	for _, provider := range previous {
		if provider.ID != "" {
			prevIDs[provider.ID] = true
		}
	}
	used := make(map[string]bool, len(next))
	content := make([]*yaml.Node, 0, len(seq.Content)+len(next))
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			// 非映射元素（异常结构）原样保留，不做破坏性改写
			content = append(content, item)
			continue
		}
		var entry map[string]any
		if err := item.Decode(&entry); err != nil {
			content = append(content, item)
			continue
		}
		id := extractHermesString(entry, "id")
		if id != "" {
			if provider, ok := nextByID[id]; ok {
				content = append(content, buildHermesLiveEntryNode(provider))
				used[id] = true
				continue
			}
			if prevIDs[id] {
				// 已删除的托管条目：移除
				continue
			}
			content = append(content, item)
			continue
		}
		// 无 id 条目：按 name+base_url 匹配托管条目（导入后尚未写回 id 的过渡场景）
		// 仅给手写条目追加 id 字段建立关联，其余手写字段原样保留（静默覆盖语义）
		if provider, ok := matchHermesNoIDEntry(entry, next, used); ok {
			appendHermesEntryIDField(item, provider.ID)
			content = append(content, item)
			used[provider.ID] = true
			continue
		}
		content = append(content, item)
	}
	for _, provider := range next {
		if provider.ID == "" || used[provider.ID] {
			continue
		}
		content = append(content, buildHermesLiveEntryNode(provider))
	}
	seq.Content = content

	if switchedTo != nil {
		modelEntry := map[string]any{"provider": hermesProviderDisplayName(*switchedTo)}
		if model := strings.TrimSpace(switchedTo.Model); model != "" {
			modelEntry["model"] = model
		}
		hermesSetTopLevelValue(root, "model", hermesEncodeValue(modelEntry))
	}
	return writeHermesLiveNode(doc)
}

// mutateHermesConfig 读 Node → 修改顶层映射 → 写回（供 MCP 投影 / memory 设置等链路复用）
// mutate 返回错误时放弃写回，保持原文件不变
func mutateHermesConfig(mutate func(root *yaml.Node) error) error {
	hermesConfigMu.Lock()
	defer hermesConfigMu.Unlock()

	doc, err := readHermesLiveNode()
	if err != nil {
		return err
	}
	root := hermesRootMapping(doc)
	if root == nil {
		return fmt.Errorf("Hermes 配置顶层结构异常: %s", getHermesConfigPath())
	}
	if err := mutate(root); err != nil {
		return err
	}
	return writeHermesLiveNode(doc)
}

// readHermesLiveCustomProviders 读取 live custom_providers 全部条目（保持文件顺序）
func readHermesLiveCustomProviders() ([]map[string]any, error) {
	doc, err := readHermesLiveNode()
	if err != nil {
		return nil, err
	}
	root := hermesRootMapping(doc)
	if root == nil {
		return nil, nil
	}
	seq := hermesGetTopLevelValue(root, "custom_providers")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil, nil
	}
	entries := make([]map[string]any, 0, len(seq.Content))
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		var entry map[string]any
		if err := item.Decode(&entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// readHermesLiveModelSelection 读取顶层 model 节的当前选择
// 兼容两种现值写法：map 结构 {provider, model} 与旧式纯字符串（仅读出 model 名）
func readHermesLiveModelSelection() (providerName string, modelName string) {
	doc, err := readHermesLiveNode()
	if err != nil {
		return "", ""
	}
	root := hermesRootMapping(doc)
	if root == nil {
		return "", ""
	}
	node := hermesGetTopLevelValue(root, "model")
	if node == nil {
		return "", ""
	}
	switch node.Kind {
	case yaml.MappingNode:
		var selection map[string]any
		if err := node.Decode(&selection); err == nil {
			return extractHermesString(selection, "provider"), extractHermesString(selection, "model")
		}
	case yaml.ScalarNode:
		return "", strings.TrimSpace(node.Value)
	}
	return "", ""
}

// importHermesProvidersFromLiveSnapshot 全量导入 live 条目（构造器首次接入路径）
// 启用标记来源：顶层 model 节的 provider 名；无匹配时回退首个条目
func importHermesProvidersFromLiveSnapshot() ([]HermesProvider, error) {
	entries, err := readHermesLiveCustomProviders()
	if err != nil {
		return nil, err
	}
	currentName, _ := readHermesLiveModelSelection()

	providers := make([]HermesProvider, 0, len(entries))
	nextID := newHermesProviderID()
	matched := false
	for _, entry := range entries {
		provider := buildHermesProviderFromFragment(entry, nextID, currentName)
		nextID = newHermesProviderID()
		if provider.Enabled {
			matched = true
		}
		providers = append(providers, provider)
	}
	if len(providers) > 0 && !matched {
		providers[0].Enabled = true
	}
	return providers, nil
}

// buildHermesProviderFromFragment 从 live 原生片段构造供应商（导入路径）
// fallbackID 用于条目缺少 id 字段时分配托管 ID
func buildHermesProviderFromFragment(entry map[string]any, fallbackID string, currentName string) HermesProvider {
	id := extractHermesString(entry, "id")
	if id == "" {
		id = fallbackID
	}
	name := extractHermesString(entry, "name")
	if name == "" {
		name = id
	}
	return normalizeHermesProvider(HermesProvider{
		ID:        id,
		Name:      name,
		BaseURL:   extractHermesString(entry, "base_url"),
		APIKey:    extractHermesString(entry, "api_key"),
		Model:     extractHermesString(entry, "model"),
		Enabled:   currentName != "" && name == currentName,
		Category:  "custom",
		CLIConfig: cloneAnyMap(entry),
	})
}

// buildHermesLiveEntryNode 构造 live 条目节点（snake_case，托管字段在前、原生片段其余键在后）
func buildHermesLiveEntryNode(provider HermesProvider) *yaml.Node {
	entry := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	appendHermesStringKey(entry, "id", provider.ID)
	appendHermesStringKey(entry, "name", provider.Name)
	appendHermesStringKey(entry, "base_url", provider.BaseURL)
	appendHermesStringKey(entry, "api_key", provider.APIKey)
	appendHermesStringKey(entry, "model", provider.Model)
	if provider.CLIConfig != nil {
		keys := make([]string, 0, len(provider.CLIConfig))
		for key := range provider.CLIConfig {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			switch key {
			case "id", "name", "base_url", "api_key", "model":
				// 托管字段以结构化字段为准，跳过片段中的同名旧值
				continue
			}
			entry.Content = append(entry.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
				hermesEncodeValue(provider.CLIConfig[key]),
			)
		}
	}
	return entry
}

func appendHermesStringKey(entry *yaml.Node, key string, value string) {
	if value == "" {
		return
	}
	entry.Content = append(entry.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)
}

// appendHermesEntryIDField 给无 id 的手写条目节点头部追加 id 字段（不改动其余字段与注释）
func appendHermesEntryIDField(entry *yaml.Node, id string) {
	entry.Content = append([]*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "id"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: id},
	}, entry.Content...)
}

// matchHermesNoIDEntry 在托管列表中按 name+base_url 匹配无 id 的 live 条目
func matchHermesNoIDEntry(entry map[string]any, next []HermesProvider, used map[string]bool) (HermesProvider, bool) {
	name := extractHermesString(entry, "name")
	baseURL := extractHermesString(entry, "base_url")
	if name == "" && baseURL == "" {
		return HermesProvider{}, false
	}
	for _, provider := range next {
		if used[provider.ID] {
			continue
		}
		if provider.Name == name && provider.BaseURL == baseURL {
			return provider, true
		}
	}
	return HermesProvider{}, false
}

func hermesEntryMatchKey(name, baseURL string) string {
	return name + "\x00" + baseURL
}

func hermesProviderDisplayName(provider HermesProvider) string {
	if name := strings.TrimSpace(provider.Name); name != "" {
		return name
	}
	return provider.ID
}

func newHermesProviderID() string {
	return fmt.Sprintf("hermes-%d", time.Now().UnixNano())
}

func extractHermesString(fragment map[string]any, key string) string {
	if fragment == nil {
		return ""
	}
	value, _ := fragment[key].(string)
	return strings.TrimSpace(value)
}

func normalizeHermesProvider(provider HermesProvider) HermesProvider {
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

func hermesProviderIDExists(providers []HermesProvider, id string) bool {
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

func cloneHermesProviders(providers []HermesProvider) []HermesProvider {
	cloned := make([]HermesProvider, len(providers))
	for i, provider := range providers {
		cloned[i] = provider
		cloned[i].CLIConfig = cloneAnyMap(provider.CLIConfig)
	}
	return cloned
}

// ========== YAML Node 读写（保未知键与注释） ==========

// getHermesDir Hermes 配置目录（~/.hermes）
func getHermesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", hermesDirName)
	}
	return filepath.Join(home, hermesDirName)
}

// getHermesConfigPath Hermes 主配置文件路径（~/.hermes/config.yaml）
func getHermesConfigPath() string {
	return filepath.Join(getHermesDir(), hermesConfigFileName)
}

// readHermesLiveNode 读取 live 配置为文档 Node（保留全部顶层键与注释）
// 文件不存在或为空视为空配置；顶层为 null 视为空映射，其他非映射结构返回错误（避免破坏性覆盖）
func readHermesLiveNode() (*yaml.Node, error) {
	data, err := os.ReadFile(getHermesConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return newHermesEmptyDoc(), nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return newHermesEmptyDoc(), nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("解析 Hermes 配置失败 (%s): %w", getHermesConfigPath(), err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return newHermesEmptyDoc(), nil
	}
	root := doc.Content[0]
	if root.Kind == yaml.ScalarNode && root.Tag == "!!null" {
		return newHermesEmptyDoc(), nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("Hermes 配置顶层不是映射 (%s)", getHermesConfigPath())
	}
	return &doc, nil
}

// writeHermesLiveNode 原子写回 live 配置（两空格缩进；仅替换过的子树丢失注释，其余节点注释保留）
// 统一走 atomicWriteFile（临时文件 + fsync + rename），崩溃不会留下半写入文件
func writeHermesLiveNode(doc *yaml.Node) error {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("Hermes 配置文档节点无效")
	}
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc.Content[0]); err != nil {
		_ = encoder.Close()
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	return atomicWriteFile(getHermesConfigPath(), buf.Bytes(), 0o644)
}

// newHermesEmptyDoc 构造空映射文档节点
func newHermesEmptyDoc() *yaml.Node {
	return &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}},
	}
}

// hermesRootMapping 取文档的顶层映射节点（非映射结构返回 nil）
func hermesRootMapping(doc *yaml.Node) *yaml.Node {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	return root
}

// hermesGetTopLevelValue 读取顶层 key 的 value 节点（不存在返回 nil）
func hermesGetTopLevelValue(root *yaml.Node, key string) *yaml.Node {
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key && root.Content[i].Kind == yaml.ScalarNode {
			return root.Content[i+1]
		}
	}
	return nil
}

// hermesSetTopLevelValue 替换或追加顶层 key 的 value 子树（保持其他键及其注释不变）
func hermesSetTopLevelValue(root *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key && root.Content[i].Kind == yaml.ScalarNode {
			root.Content[i+1] = value
			return
		}
	}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}

// hermesEncodeValue 任意值编码为 YAML 节点
func hermesEncodeValue(value any) *yaml.Node {
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null"}
	}
	return &node
}

// hermesDecodeNode 节点解码为通用值（nil 或失败返回 nil）
func hermesDecodeNode(node *yaml.Node) any {
	if node == nil {
		return nil
	}
	var value any
	if err := node.Decode(&value); err != nil {
		return nil
	}
	return value
}

// readHermesLiveConfigBytes 快照 live 配置原始字节（事务回滚用）
func readHermesLiveConfigBytes() ([]byte, bool, error) {
	path := getHermesConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

// restoreHermesLiveConfigBytes 恢复 live 配置快照（原不存在则删除）
func restoreHermesLiveConfigBytes(data []byte, exists bool) error {
	path := getHermesConfigPath()
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
