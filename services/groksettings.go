/**
 * @name: Grok Build 配置服务
 * @Descripttion: 管理 ~/.grok/config.toml 的供应商切换、直连应用、代理接管与首次导入
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 05:30:00
 * @LastEditTime: 2026-08-17 05:30:00
 * @FilePath: services/groksettings.go
 */

package services

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	grokDirName        = ".grok"
	grokConfigFileName = "config.toml"
	// 代理接管时写入选中 profile 的占位 API Key
	grokProxyManagedAPIKey = "PROXY_MANAGED"
	grokDefaultAPIBackend  = "responses"
	grokProxyBaseURLPath   = "/grokbuild/v1"
)

// grokModelConfig ~/.grok/config.toml 的 [model.<profile>] 表
type grokModelConfig struct {
	Model         string `toml:"model" json:"model"`
	BaseURL       string `toml:"base_url" json:"base_url"`
	Name          string `toml:"name" json:"name"`
	APIKey        string `toml:"api_key,omitempty" json:"api_key,omitempty"`
	EnvKey        string `toml:"env_key,omitempty" json:"env_key,omitempty"`
	APIBackend    string `toml:"api_backend" json:"api_backend"`
	ContextWindow int64  `toml:"context_window" json:"context_window"`
}

// grokLiveConfig ~/.grok/config.toml 结构化视图
type grokLiveConfig struct {
	Models struct {
		Default string `toml:"default"`
	} `toml:"models"`
	Model map[string]grokModelConfig `toml:"model"`
}

// GrokSettingsService Grok Build 的 settings 四件套实现
type GrokSettingsService struct {
	relayAddr string
}

// NewGrokSettingsService 创建 Grok 配置服务
func NewGrokSettingsService(relayAddr string) *GrokSettingsService {
	if relayAddr == "" {
		relayAddr = ":18100"
	}
	return &GrokSettingsService{relayAddr: relayAddr}
}

func getGrokDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".grok"
	}
	return filepath.Join(home, grokDirName)
}

func getGrokConfigPath() string {
	return filepath.Join(getGrokDir(), grokConfigFileName)
}

func getGrokBackupPath() string {
	return getGrokConfigPath() + ".code-switch.bak"
}

// grokProxyBaseURL 代理接管写入的 base_url（Grok CLI 按 api_backend 在其后拼 /responses 或 /chat/completions）
func (s *GrokSettingsService) grokProxyBaseURL() string {
	return "http://127.0.0.1:" + localProxyPort(s.relayAddr) + grokProxyBaseURLPath
}

// readGrokLiveConfig 解析 ~/.grok/config.toml（文件不存在视为空配置）
func readGrokLiveConfig() (*grokLiveConfig, error) {
	data, err := os.ReadFile(getGrokConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &grokLiveConfig{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return &grokLiveConfig{}, nil
	}
	var config grokLiveConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析 Grok 配置失败: %w", err)
	}
	return &config, nil
}

// readGrokLiveMap 以通用 map 视图读取 ~/.grok/config.toml（保留 mcp_servers 等全部顶层键）
func readGrokLiveMap() (map[string]any, error) {
	data, err := os.ReadFile(getGrokConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	config := map[string]any{}
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析 Grok 配置失败: %w", err)
	}
	return config, nil
}

// grokMapModelTable 取 map 视图中的 [model.*] 表
func grokMapModelTable(config map[string]any) map[string]any {
	table, _ := config["model"].(map[string]any)
	if table == nil {
		table = map[string]any{}
	}
	return table
}

// grokMapSetModelsDefault 设置 map 视图中的 [models].default
func grokMapSetModelsDefault(config map[string]any, profile string) {
	models, _ := config["models"].(map[string]any)
	if models == nil {
		models = map[string]any{}
	}
	models["default"] = profile
	config["models"] = models
}

// writeGrokLiveMap 原子写回 ~/.grok/config.toml（map 全量，保留未知顶层键）
func writeGrokLiveMap(config map[string]any) error {
	data, err := toml.Marshal(config)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(getGrokDir(), 0o755); err != nil {
		return err
	}
	tmp := getGrokConfigPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, getGrokConfigPath())
}

// isGrokOfficialLiveConfig 官方态判定：无 [models] 且无任何 [model.*] 表
// （官方 xAI OAuth 登录时 Grok CLI 自带凭据，config.toml 无自定义模型表）
func isGrokOfficialLiveConfig(config *grokLiveConfig) bool {
	return config == nil || (config.Models.Default == "" && len(config.Model) == 0)
}

// selectedGrokProfile 当前生效的 profile（[models].default，缺省取字典序首个）
func selectedGrokProfile(config *grokLiveConfig) string {
	if config == nil || len(config.Model) == 0 {
		return ""
	}
	if name := strings.TrimSpace(config.Models.Default); name != "" {
		if _, ok := config.Model[name]; ok {
			return name
		}
	}
	names := make([]string, 0, len(config.Model))
	for name := range config.Model {
		names = append(names, name)
	}
	sort.Strings(names)
	return names[0]
}

// GrokProxyStatus Grok 代理状态
type GrokProxyStatus struct {
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"baseURL"`
}

// ProxyStatus 检查 Grok 是否处于代理接管状态（选中 profile 的 base_url 指向本地代理）
func (s *GrokSettingsService) ProxyStatus() (*GrokProxyStatus, error) {
	status := &GrokProxyStatus{BaseURL: s.grokProxyBaseURL()}
	config, err := readGrokLiveConfig()
	if err != nil {
		return status, err
	}
	profile := selectedGrokProfile(config)
	if profile == "" {
		return status, nil
	}
	entry, ok := config.Model[profile]
	if !ok {
		return status, nil
	}
	status.Enabled = strings.HasPrefix(entry.BaseURL, s.grokProxyBaseURL())
	return status, nil
}

// EnableProxy 代理接管：备份当前 config.toml 后，将选中 profile 指向本地代理
// 仅替换 base_url / api_key，api_backend 保留用户原值（空时才写默认 responses），
// 其余顶层键（如 mcp_servers）原样保留；已处于代理态时幂等返回
func (s *GrokSettingsService) EnableProxy() error {
	if status, err := s.ProxyStatus(); err == nil && status.Enabled {
		// 幂等短路：重复开启代理不再覆盖备份（保留首次接管前的直连快照）
		return nil
	}
	view, err := readGrokLiveConfig()
	if err != nil {
		return err
	}
	if isGrokOfficialLiveConfig(view) {
		return fmt.Errorf("当前为 Grok 官方登录态（无自定义模型配置），请先应用一个供应商再开启代理")
	}
	profile := selectedGrokProfile(view)
	if profile == "" {
		return fmt.Errorf("Grok 配置中未找到可用的 [model.*] 表")
	}

	// 备份原文件（接管前的直连配置）
	if _, statErr := os.Stat(getGrokConfigPath()); statErr == nil {
		data, readErr := os.ReadFile(getGrokConfigPath())
		if readErr != nil {
			return readErr
		}
		if writeErr := os.WriteFile(getGrokBackupPath(), data, 0o644); writeErr != nil {
			return fmt.Errorf("备份 Grok 配置失败: %w", writeErr)
		}
	}

	config, err := readGrokLiveMap()
	if err != nil {
		return err
	}
	entry, _ := grokMapModelTable(config)[profile].(map[string]any)
	if entry == nil {
		entry = map[string]any{}
	}
	entry["base_url"] = s.grokProxyBaseURL()
	entry["api_key"] = grokProxyManagedAPIKey
	if backend, _ := entry["api_backend"].(string); strings.TrimSpace(backend) == "" {
		// 保留用户原值（如 chat_completions），仅空值时补默认
		entry["api_backend"] = grokDefaultAPIBackend
	}
	grokMapModelTable(config)[profile] = entry
	grokMapSetModelsDefault(config, profile)
	return writeGrokLiveMap(config)
}

// DisableProxy 关闭代理：优先从备份恢复接管前的直连配置；
// 备份缺失且当前确为代理态时，清除 live 的 model/models 节回到未配置态，
// 让用户可以重新应用供应商（解除「无备份 → 无法关闭 → 无法切换」的死锁）
func (s *GrokSettingsService) DisableProxy() error {
	data, err := os.ReadFile(getGrokBackupPath())
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		status, statusErr := s.ProxyStatus()
		if statusErr != nil {
			return statusErr
		}
		if !status.Enabled {
			return fmt.Errorf("未找到代理接管备份，无法恢复")
		}
		config, readErr := readGrokLiveMap()
		if readErr != nil {
			return readErr
		}
		delete(config, "model")
		delete(config, "models")
		if writeErr := writeGrokLiveMap(config); writeErr != nil {
			return writeErr
		}
		log.Printf("[GrokSettings] 代理接管备份缺失，已清除模型配置节回到未配置态")
		return nil
	}
	// 恢复备份走原子写（tmp + rename），避免半写入损坏 config.toml
	if err := os.MkdirAll(getGrokDir(), 0o755); err != nil {
		return err
	}
	tmp := getGrokConfigPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, getGrokConfigPath()); err != nil {
		return err
	}
	return os.Remove(getGrokBackupPath())
}

// ApplySingleProvider 直连应用指定供应商：替换 config.toml 的模型配置节
// provider.ConfigTOML 为该供应商的 TOML 片段（[model.<profile>] + [models].default）；
// 其余顶层键（如 mcp_servers）保留；空 TOML 即回到官方态（清空模型节）
func (s *GrokSettingsService) ApplySingleProvider(providerID int64) error {
	providers, err := LoadProvidersFromStore("grokbuild")
	if err != nil {
		return fmt.Errorf("加载 Grok 供应商失败: %w", err)
	}
	var provider *Provider
	for index := range providers {
		if providers[index].ID == providerID {
			provider = &providers[index]
			break
		}
	}
	if provider == nil {
		return fmt.Errorf("未找到 ID 为 %d 的 Grok 供应商", providerID)
	}

	if status, statusErr := s.ProxyStatus(); statusErr == nil && status.Enabled {
		return fmt.Errorf("本地代理已启用，请先关闭代理再切换供应商")
	}

	config, err := readGrokLiveMap()
	if err != nil {
		return err
	}

	if strings.TrimSpace(provider.ConfigTOML) == "" {
		// 官方态：清空模型节（Grok CLI 回落自带 xAI OAuth）
		delete(config, "model")
		delete(config, "models")
		return writeGrokLiveMap(config)
	}

	providerView := grokLiveConfig{}
	if err := toml.Unmarshal([]byte(provider.ConfigTOML), &providerView); err != nil {
		return fmt.Errorf("供应商 TOML 配置无效: %w", err)
	}
	profile := selectedGrokProfile(&providerView)
	if profile == "" {
		return fmt.Errorf("供应商 TOML 中未找到 [model.*] 表")
	}
	providerMap := map[string]any{}
	if err := toml.Unmarshal([]byte(provider.ConfigTOML), &providerMap); err != nil {
		return fmt.Errorf("供应商 TOML 配置无效: %w", err)
	}
	providerModelTable := grokMapModelTable(providerMap)
	entry, ok := providerModelTable[profile]
	if !ok {
		return fmt.Errorf("供应商 TOML 中未找到 profile %s", profile)
	}
	// 单供应商整体替换：仅保留选中 profile 的表
	config["model"] = map[string]any{profile: entry}
	grokMapSetModelsDefault(config, profile)
	return writeGrokLiveMap(config)
}

// GetStatus Grok 当前配置状态摘要
func (s *GrokSettingsService) GetStatus() (map[string]any, error) {
	config, err := readGrokLiveConfig()
	if err != nil {
		return nil, err
	}
	status := map[string]any{
		"official":    isGrokOfficialLiveConfig(config),
		"configExists": providerConfigFileExists(getGrokConfigPath()),
	}
	profile := selectedGrokProfile(config)
	if profile != "" {
		entry := config.Model[profile]
		status["profile"] = profile
		status["model"] = entry.Model
		status["baseUrl"] = entry.BaseURL
		status["apiBackend"] = entry.APIBackend
	}
	return status, nil
}

// ImportFromLive 首次接入：检测 ~/.grok/config.toml 的自定义模型表并导入为供应商
// 仅导入非官方态的选中 profile；导入后不修改原生文件
func (s *GrokSettingsService) ImportFromLive() (bool, error) {
	config, err := readGrokLiveConfig()
	if err != nil {
		return false, err
	}
	if isGrokOfficialLiveConfig(config) {
		return false, nil
	}
	profile := selectedGrokProfile(config)
	if profile == "" {
		return false, nil
	}

	// 代理接管态不导入：选中 profile 的 base_url 指向本地代理（127.0.0.1:18100），
	// 导入会把代理占位地址误存为供应商直连地址
	entry := config.Model[profile]
	if strings.HasPrefix(entry.BaseURL, s.grokProxyBaseURL()) {
		return false, nil
	}

	existing, err := LoadProvidersFromStore("grokbuild")
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		// 已有供应商数据（此前导入过），不重复导入
		return false, nil
	}

	single := grokLiveConfig{}
	single.Model = map[string]grokModelConfig{profile: entry}
	single.Models.Default = profile
	payload, err := toml.Marshal(single)
	if err != nil {
		return false, err
	}

	name := strings.TrimSpace(entry.Name)
	if name == "" {
		name = profile
	}
	imported := []Provider{{
		ID:         1,
		Name:       name,
		APIURL:     entry.BaseURL,
		APIKey:     entry.APIKey,
		Enabled:    true,
		Category:   "custom",
		ConfigTOML: string(payload),
	}}
	if err := SaveProvidersToStore("grokbuild", imported); err != nil {
		return false, err
	}
	return true, nil
}
