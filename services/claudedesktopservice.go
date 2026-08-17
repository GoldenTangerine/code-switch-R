/**
 * @name: Claude Desktop 配置服务
 * @Descripttion: 管理 Claude Desktop 四文件事务式写入的供应商切换、Direct/Proxy 双模式与首次导入
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 03:45:00
 * @LastEditTime: 2026-08-17 03:45:00
 * @FilePath: services/claudedesktopservice.go
 */

package services

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/daodao97/xgo/xdb"
)

const (
	// Claude Desktop 主配置目录名（macOS: ~/Library/Application Support/Claude，Windows: %LOCALAPPDATA%\Claude）
	claudeDesktopDirName = "Claude"
	// 3p 模式配置目录后缀（与主目录同级：Claude-3p）
	claudeDesktop3pDirSuffix = "-3p"
	// 两目录共用的部署配置文件名（保留 mcpServers 等用户键，仅更新 deploymentMode）
	claudeDesktopConfigFileName = "claude_desktop_config.json"
	// profile 库目录名（位于 Claude-3p 目录内）
	claudeDesktopConfigLibraryDir = "configLibrary"
	// 固定 profile ID（cc-switch 约定，Claude Desktop 3p 模式激活项）
	claudeDesktopProfileID = "00000000-0000-4000-8000-000000157210"
	// profile 库元数据文件名（activeProfileId 指向固定 profile）
	claudeDesktopMetaFileName = "_meta.json"
	// claude_desktop_config.json 的部署模式键：第三方供应商写 3p，官方写 1p
	claudeDesktopDeploymentModeKey   = "deploymentMode"
	claudeDesktopDeploymentMode3p    = "3p"
	claudeDesktopDeploymentMode1p    = "1p"
	claudeDesktopEnterpriseConfigKey = "enterpriseConfig"
	// 本地代理网关 token 在 app_settings 表的存储键（Proxy 模式写入 profile 的 inferenceGatewayApiKey）
	claudeDesktopGatewayTokenSettingKey = "claude_desktop_gateway_token"
	// Claude Desktop 接入模式：direct 直连供应商 / proxy 本地代理（复用 :18100 Claude 链路）
	claudeDesktopModeDirect = "direct"
	claudeDesktopModeProxy  = "proxy"
)

// ClaudeDesktopModelRoute Claude Desktop 模型路由条目（profile 的 inferenceModels 元素）
type ClaudeDesktopModelRoute struct {
	Name          string `json:"name"`
	LabelOverride string `json:"labelOverride,omitempty"`
	Supports1M    bool   `json:"supports1m,omitempty"`
}

// claudeDesktopProfile Claude-3p/configLibrary/<PROFILE_ID>.json 的结构
type claudeDesktopProfile struct {
	CoworkEgressAllowedHosts     []string                  `json:"coworkEgressAllowedHosts"`
	DisableDeploymentModeChooser bool                      `json:"disableDeploymentModeChooser"`
	InferenceGatewayAPIKey       string                    `json:"inferenceGatewayApiKey"`
	InferenceGatewayAuthScheme   string                    `json:"inferenceGatewayAuthScheme"`
	InferenceGatewayBaseURL      string                    `json:"inferenceGatewayBaseUrl"`
	InferenceProvider            string                    `json:"inferenceProvider"`
	InferenceModels              []ClaudeDesktopModelRoute `json:"inferenceModels"`
}

// claudeDesktopMeta Claude-3p/configLibrary/_meta.json 的结构
type claudeDesktopMeta struct {
	ActiveProfileID string `json:"activeProfileId"`
}

// ClaudeDesktopSettingsService Claude Desktop 的 settings 四件套实现
type ClaudeDesktopSettingsService struct {
	relayAddr string
}

// NewClaudeDesktopSettingsService 创建 Claude Desktop 配置服务
func NewClaudeDesktopSettingsService(relayAddr string) *ClaudeDesktopSettingsService {
	if relayAddr == "" {
		relayAddr = ":18100"
	}
	return &ClaudeDesktopSettingsService{relayAddr: relayAddr}
}

// defaultClaudeDesktopModelRoutes 默认模型路由（未配置 ClaudeDesktopModelRoutes 时写入）
func defaultClaudeDesktopModelRoutes() []ClaudeDesktopModelRoute {
	return []ClaudeDesktopModelRoute{
		{Name: "claude-sonnet-5"},
		{Name: "claude-opus-5"},
		{Name: "claude-fable-5"},
		{Name: "claude-haiku-4-5"},
	}
}

// normalizeClaudeDesktopMode 归一化接入模式（未知值回落 direct）
func normalizeClaudeDesktopMode(mode string) string {
	if strings.ToLower(strings.TrimSpace(mode)) == claudeDesktopModeProxy {
		return claudeDesktopModeProxy
	}
	return claudeDesktopModeDirect
}

// claudeDesktopDirs 返回 Claude Desktop 的主目录与 3p 目录
// 仅支持 macOS / Windows；Linux 返回不支持错误
func claudeDesktopDirs() (string, string, error) {
	if runtime.GOOS == "linux" {
		return "", "", fmt.Errorf("当前平台（Linux）不支持 Claude Desktop")
	}
	var base string
	if runtime.GOOS == "windows" {
		// Claude Desktop 在 Windows 使用 %LOCALAPPDATA%\Claude（非 Roaming AppData）
		base = os.Getenv("LOCALAPPDATA")
	}
	if base == "" {
		configDir, err := os.UserConfigDir() // macOS: ~/Library/Application Support
		if err != nil {
			return "", "", fmt.Errorf("定位用户配置目录失败: %w", err)
		}
		base = configDir
	}
	normal := filepath.Join(base, claudeDesktopDirName)
	thirdParty := filepath.Join(base, claudeDesktopDirName+claudeDesktop3pDirSuffix)
	return normal, thirdParty, nil
}

// claudeDesktopConfigPath 指定目录下的 claude_desktop_config.json 路径
func claudeDesktopConfigPath(dir string) string {
	return filepath.Join(dir, claudeDesktopConfigFileName)
}

// claudeDesktopProfilePath 指定目录下的固定 profile 路径
func claudeDesktopProfilePath(dir string) string {
	return filepath.Join(dir, claudeDesktopConfigLibraryDir, claudeDesktopProfileID+".json")
}

// claudeDesktopMetaPath 指定目录下的 profile 库元数据路径
func claudeDesktopMetaPath(dir string) string {
	return filepath.Join(dir, claudeDesktopConfigLibraryDir, claudeDesktopMetaFileName)
}

// readClaudeDesktopConfigMap 读取 claude_desktop_config.json 为通用 map（保留全部用户键；文件缺失视为空配置）
func readClaudeDesktopConfigMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
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
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析 Claude Desktop 配置失败: %w", err)
	}
	return config, nil
}

// readClaudeDesktopProfile 读取 3p 目录的当前 profile（文件缺失返回 nil, false）
func readClaudeDesktopProfile() (*claudeDesktopProfile, bool, error) {
	_, thirdPartyDir, err := claudeDesktopDirs()
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(claudeDesktopProfilePath(thirdPartyDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, false, nil
	}
	var profile claudeDesktopProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, false, fmt.Errorf("解析 Claude Desktop profile 失败: %w", err)
	}
	return &profile, true, nil
}

// marshalClaudeDesktopJSON 统一缩进序列化（Claude Desktop 侧为两空格 JSON）
func marshalClaudeDesktopJSON(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}

// ========== 事务式多文件写入（快照 → 写入/删除 → 校验 → 失败整体回滚） ==========

// claudeDesktopWrite 待覆盖写入的文件
type claudeDesktopWrite struct {
	path string
	data []byte
}

// claudeDesktopFileSnapshot 单文件快照（存在性 + 字节）
type claudeDesktopFileSnapshot struct {
	exists bool
	data   []byte
}

// claudeDesktopTransaction Claude Desktop 事务式多文件写入
type claudeDesktopTransaction struct {
	writes      []claudeDesktopWrite
	removePaths []string
}

func (t *claudeDesktopTransaction) write(path string, data []byte) {
	t.writes = append(t.writes, claudeDesktopWrite{path: path, data: data})
}

func (t *claudeDesktopTransaction) remove(path string) {
	t.removePaths = append(t.removePaths, path)
}

// commit 执行事务：先快照全部目标文件，再依次写入/删除，写完逐文件回读校验；任一步失败整体恢复快照
func (t *claudeDesktopTransaction) commit() error {
	snapshotPaths := make([]string, 0, len(t.writes)+len(t.removePaths))
	for _, item := range t.writes {
		snapshotPaths = append(snapshotPaths, item.path)
	}
	snapshotPaths = append(snapshotPaths, t.removePaths...)

	snapshots := make(map[string]*claudeDesktopFileSnapshot, len(snapshotPaths))
	for _, path := range snapshotPaths {
		snapshot, err := snapshotClaudeDesktopFile(path)
		if err != nil {
			return fmt.Errorf("快照 Claude Desktop 配置 %s 失败: %w", path, err)
		}
		snapshots[path] = snapshot
	}

	rollback := func(cause error) error {
		restoreClaudeDesktopSnapshots(snapshots)
		return cause
	}

	for _, item := range t.writes {
		if err := atomicWriteFile(item.path, item.data, 0o644); err != nil {
			return rollback(fmt.Errorf("写入 Claude Desktop 配置 %s 失败（已回滚）: %w", item.path, err))
		}
	}
	for _, path := range t.removePaths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return rollback(fmt.Errorf("删除 Claude Desktop 配置 %s 失败（已回滚）: %w", path, err))
		}
	}
	// 写后校验：逐文件回读比对字节，防止半写入中间态
	for _, item := range t.writes {
		data, err := os.ReadFile(item.path)
		if err != nil || !bytes.Equal(data, item.data) {
			return rollback(fmt.Errorf("校验 Claude Desktop 配置 %s 失败（已回滚）", item.path))
		}
	}
	return nil
}

// snapshotClaudeDesktopFile 读取单文件快照（不存在标记 exists=false）
func snapshotClaudeDesktopFile(path string) (*claudeDesktopFileSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &claudeDesktopFileSnapshot{exists: false}, nil
		}
		return nil, err
	}
	return &claudeDesktopFileSnapshot{exists: true, data: data}, nil
}

// restoreClaudeDesktopSnapshots 恢复快照（原不存在的文件删除，原存在的文件原子写回原字节；失败仅记录日志）
func restoreClaudeDesktopSnapshots(snapshots map[string]*claudeDesktopFileSnapshot) {
	for path, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		if !snapshot.exists {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				log.Printf("[ClaudeDesktop] ⚠️ 回滚删除 %s 失败: %v", path, err)
			}
			continue
		}
		if err := atomicWriteFile(path, snapshot.data, 0o644); err != nil {
			log.Printf("[ClaudeDesktop] ⚠️ 回滚恢复 %s 失败: %v", path, err)
		}
	}
}

// ========== 四件套：应用 / 状态 / 导入 ==========

// ApplySingleProvider 应用指定供应商：
// - Direct：inferenceGatewayBaseUrl/ApiKey 直连供应商
// - Proxy：指向本地代理（:18100 的 Claude 链路），Key 使用 app_settings 持久化的网关 token
// - 官方条目（Category=="official"）：deploymentMode 写 1p 并清除 profile 自定义内容
func (s *ClaudeDesktopSettingsService) ApplySingleProvider(providerID int64) error {
	providers, err := LoadProvidersFromStore(string(PlatformClaudeDesktop))
	if err != nil {
		return fmt.Errorf("加载 Claude Desktop 供应商失败: %w", err)
	}
	provider, found := findProviderByID(providers, providerID)
	if !found {
		return fmt.Errorf("未找到 ID 为 %d 的 Claude Desktop 供应商", providerID)
	}

	if provider.Category == "official" {
		return s.applyOfficial()
	}

	var baseURL, apiKey string
	if normalizeClaudeDesktopMode(provider.ClaudeDesktopMode) == claudeDesktopModeProxy {
		baseURL = s.proxyBaseURL()
		apiKey, err = s.gatewayToken()
		if err != nil {
			return err
		}
	} else {
		baseURL = normalizeURLTrimSlash(provider.APIURL)
		if baseURL == "" {
			return fmt.Errorf("供应商 %s 缺少 API 地址，无法直连应用", provider.Name)
		}
		apiKey = strings.TrimSpace(provider.APIKey)
	}

	models := provider.ClaudeDesktopModelRoutes
	if len(models) == 0 {
		models = defaultClaudeDesktopModelRoutes()
	}
	return s.applyGateway(baseURL, apiKey, models)
}

// applyGateway 第三方供应商应用：两目录 deploymentMode=3p + 写入 3p profile 与 _meta（4 文件事务）
func (s *ClaudeDesktopSettingsService) applyGateway(baseURL, apiKey string, models []ClaudeDesktopModelRoute) error {
	normalDir, thirdPartyDir, err := claudeDesktopDirs()
	if err != nil {
		return err
	}

	tx := &claudeDesktopTransaction{}
	// 两目录 claude_desktop_config.json：仅更新 deploymentMode，保留 mcpServers 等用户键
	for _, dir := range []string{normalDir, thirdPartyDir} {
		configPath := claudeDesktopConfigPath(dir)
		configMap, err := readClaudeDesktopConfigMap(configPath)
		if err != nil {
			return err
		}
		configMap[claudeDesktopDeploymentModeKey] = claudeDesktopDeploymentMode3p
		payload, err := marshalClaudeDesktopJSON(configMap)
		if err != nil {
			return err
		}
		tx.write(configPath, payload)
	}

	profile := claudeDesktopProfile{
		CoworkEgressAllowedHosts:     []string{"*"},
		DisableDeploymentModeChooser: true,
		InferenceGatewayAPIKey:       apiKey,
		InferenceGatewayAuthScheme:   "bearer",
		InferenceGatewayBaseURL:      baseURL,
		InferenceProvider:            "gateway",
		InferenceModels:              models,
	}
	profilePayload, err := marshalClaudeDesktopJSON(profile)
	if err != nil {
		return err
	}
	tx.write(claudeDesktopProfilePath(thirdPartyDir), profilePayload)

	metaPayload, err := marshalClaudeDesktopJSON(claudeDesktopMeta{ActiveProfileID: claudeDesktopProfileID})
	if err != nil {
		return err
	}
	tx.write(claudeDesktopMetaPath(thirdPartyDir), metaPayload)
	return tx.commit()
}

// applyOfficial 官方条目应用：两目录 deploymentMode=1p、删除 enterpriseConfig，
// 仅清理 thirdPartyDir 的 profile/_meta（normalDir 的 configLibrary 属用户本地资产，不删除）
func (s *ClaudeDesktopSettingsService) applyOfficial() error {
	normalDir, thirdPartyDir, err := claudeDesktopDirs()
	if err != nil {
		return err
	}

	tx := &claudeDesktopTransaction{}
	for _, dir := range []string{normalDir, thirdPartyDir} {
		configPath := claudeDesktopConfigPath(dir)
		configMap, err := readClaudeDesktopConfigMap(configPath)
		if err != nil {
			return err
		}
		configMap[claudeDesktopDeploymentModeKey] = claudeDesktopDeploymentMode1p
		delete(configMap, claudeDesktopEnterpriseConfigKey)
		payload, err := marshalClaudeDesktopJSON(configMap)
		if err != nil {
			return err
		}
		tx.write(configPath, payload)
	}
	// 清除 thirdParty 目录 profile 的自定义内容（恢复官方）
	tx.remove(claudeDesktopProfilePath(thirdPartyDir))
	tx.remove(claudeDesktopMetaPath(thirdPartyDir))
	return tx.commit()
}

// proxyBaseURL Proxy 模式写入的 inferenceGatewayBaseUrl（复用 :18100 Claude 代理链路）
func (s *ClaudeDesktopSettingsService) proxyBaseURL() string {
	return s.localProxyPrefix() + "v1/messages"
}

// localProxyPrefix 本地代理地址前缀（含尾斜杠，用于 Proxy 状态判定）
func (s *ClaudeDesktopSettingsService) localProxyPrefix() string {
	return "http://127.0.0.1:" + localProxyPort(s.relayAddr) + "/"
}

// localProxyPort 从 relayAddr 提取端口（兼容 ":18100" / "127.0.0.1:18100" / "http://127.0.0.1:18100"）
// relayAddr 可能是 host:port 形式，直接 TrimPrefix 拼接会得到 "127.0.0.1:127.0.0.1:18100" 这类错误地址
func localProxyPort(relayAddr string) string {
	addr := strings.TrimSpace(relayAddr)
	if addr == "" {
		return "18100"
	}
	if _, port, err := net.SplitHostPort(addr); err == nil && port != "" {
		return port
	}
	if parsed, parseErr := url.Parse(addr); parseErr == nil && parsed.Port() != "" {
		return parsed.Port()
	}
	if strings.HasPrefix(addr, ":") {
		return strings.TrimPrefix(addr, ":")
	}
	return "18100"
}

// gatewayToken 读取或生成本地代理网关 token（app_settings 表持久化，32 字节 hex）
func (s *ClaudeDesktopSettingsService) gatewayToken() (string, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return "", fmt.Errorf("获取数据库连接失败: %w", err)
	}
	var token string
	err = db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, claudeDesktopGatewayTokenSettingKey).Scan(&token)
	if err == nil && strings.TrimSpace(token) != "" {
		return token, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	raw := make([]byte, 32)
	if _, randErr := rand.Read(raw); randErr != nil {
		return "", fmt.Errorf("生成 Claude Desktop 网关 token 失败: %w", randErr)
	}
	token = hex.EncodeToString(raw)
	if err := GlobalDBQueue.Exec(`
		INSERT INTO app_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, claudeDesktopGatewayTokenSettingKey, token); err != nil {
		return "", fmt.Errorf("保存 Claude Desktop 网关 token 失败: %w", err)
	}
	return token, nil
}

// ClaudeDesktopProxyStatus Claude Desktop 代理状态
type ClaudeDesktopProxyStatus struct {
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"baseURL"`
}

// ProxyStatus 检查当前 profile 是否指向本地代理（:18100 Claude 链路）
func (s *ClaudeDesktopSettingsService) ProxyStatus() (*ClaudeDesktopProxyStatus, error) {
	status := &ClaudeDesktopProxyStatus{BaseURL: s.proxyBaseURL()}
	profile, exists, err := readClaudeDesktopProfile()
	if err != nil {
		return status, err
	}
	if exists {
		status.Enabled = strings.HasPrefix(profile.InferenceGatewayBaseURL, s.localProxyPrefix())
	}
	return status, nil
}

// GetStatus Claude Desktop 当前配置状态摘要（mode/baseUrl/供应商名）
func (s *ClaudeDesktopSettingsService) GetStatus() (map[string]any, error) {
	status := map[string]any{}
	normalDir, _, err := claudeDesktopDirs()
	if err != nil {
		// Linux 等不支持平台：返回不支持标记而非报错，便于前端禁用入口
		status["supported"] = false
		return status, nil
	}

	normalConfigPath := claudeDesktopConfigPath(normalDir)
	configMap, err := readClaudeDesktopConfigMap(normalConfigPath)
	if err != nil {
		return nil, err
	}
	profile, exists, err := readClaudeDesktopProfile()
	if err != nil {
		return nil, err
	}

	gatewayActive := exists && profile.InferenceProvider == "gateway"
	status["supported"] = true
	status["configExists"] = providerConfigFileExists(normalConfigPath)
	if mode, ok := configMap[claudeDesktopDeploymentModeKey].(string); ok && mode != "" {
		status["deploymentMode"] = mode
	}
	status["official"] = !gatewayActive

	if gatewayActive {
		mode := claudeDesktopModeDirect
		if strings.HasPrefix(profile.InferenceGatewayBaseURL, s.localProxyPrefix()) {
			mode = claudeDesktopModeProxy
		}
		status["mode"] = mode
		status["baseUrl"] = profile.InferenceGatewayBaseURL
		status["modelCount"] = len(profile.InferenceModels)
		status["providerName"] = s.resolveAppliedProviderName(mode, profile.InferenceGatewayBaseURL)
		// 直连态按 baseUrl 反查统一存储的供应商 ID（代理态/未命中返回 0），供前端精确回填
		providerID := int64(0)
		if mode == claudeDesktopModeDirect {
			providerID = s.resolveAppliedProviderID(profile.InferenceGatewayBaseURL)
		}
		status["providerId"] = providerID
	}
	return status, nil
}

// resolveAppliedProviderID 按当前 profile 的 baseUrl 反查统一存储的供应商 ID（未命中返回 0）
func (s *ClaudeDesktopSettingsService) resolveAppliedProviderID(baseURL string) int64 {
	providers, err := LoadProvidersFromStore(string(PlatformClaudeDesktop))
	if err != nil {
		return 0
	}
	for _, provider := range providers {
		if urlsEqualFold(provider.APIURL, baseURL) {
			return provider.ID
		}
	}
	return 0
}

// resolveAppliedProviderName 按当前 profile 反查供应商名（直连按 APIURL 匹配；代理态汇总为本地代理）
func (s *ClaudeDesktopSettingsService) resolveAppliedProviderName(mode, baseURL string) string {
	if mode == claudeDesktopModeProxy {
		return "本地代理"
	}
	providers, err := LoadProvidersFromStore(string(PlatformClaudeDesktop))
	if err != nil {
		return ""
	}
	for _, provider := range providers {
		if urlsEqualFold(provider.APIURL, baseURL) {
			return provider.Name
		}
	}
	return ""
}

// ImportFromLive 首次接入：检测 3p profile 的网关配置并导入为供应商（导入后不修改原生文件）
func (s *ClaudeDesktopSettingsService) ImportFromLive() (bool, error) {
	if _, _, err := claudeDesktopDirs(); err != nil {
		// 不支持平台（Linux）静默跳过
		return false, nil
	}
	profile, exists, err := readClaudeDesktopProfile()
	if err != nil || !exists {
		return false, err
	}
	if profile.InferenceProvider != "gateway" {
		return false, nil
	}
	baseURL := normalizeURLTrimSlash(profile.InferenceGatewayBaseURL)
	if baseURL == "" {
		return false, nil
	}
	// 代理态不导入：baseUrl 指向本地代理（127.0.0.1:18100），
	// 导入会把代理地址误存为供应商直连地址
	if strings.HasPrefix(profile.InferenceGatewayBaseURL, s.localProxyPrefix()) {
		return false, nil
	}

	existing, err := LoadProvidersFromStore(string(PlatformClaudeDesktop))
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		// 已有供应商数据（此前导入过），不重复导入
		return false, nil
	}

	imported := []Provider{{
		ID:                       1,
		Name:                     claudeDesktopImportName(baseURL),
		APIURL:                   baseURL,
		APIKey:                   strings.TrimSpace(profile.InferenceGatewayAPIKey),
		Enabled:                  true,
		Category:                 "custom",
		ClaudeDesktopMode:        claudeDesktopModeDirect,
		ClaudeDesktopModelRoutes: profile.InferenceModels,
	}}
	if err := SaveProvidersToStore(string(PlatformClaudeDesktop), imported); err != nil {
		return false, err
	}
	return true, nil
}

// claudeDesktopImportName 导入供应商命名：优先取 baseURL 的主机名
func claudeDesktopImportName(baseURL string) string {
	if parsed, err := url.Parse(baseURL); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return "Claude Desktop 供应商"
}
