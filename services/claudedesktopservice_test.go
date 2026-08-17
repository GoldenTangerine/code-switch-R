/**
 * @name: Claude Desktop 配置服务测试
 * @Descripttion: 覆盖四文件事务写入的 Direct/Proxy/官方三模式与失败回滚、首次导入
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 03:50:00
 * @LastEditTime: 2026-08-17 03:50:00
 * @FilePath: services/claudedesktopservice_test.go
 */

package services

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// claudeDesktopTestDirs 测试内解析 Claude Desktop 两目录（Windows 下 LOCALAPPDATA 一并指向隔离目录）
func claudeDesktopTestDirs(t *testing.T) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		home := os.Getenv("USERPROFILE")
		t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	}
	normalDir, thirdPartyDir, err := claudeDesktopDirs()
	if err != nil {
		t.Fatalf("解析 Claude Desktop 目录失败: %v", err)
	}
	return normalDir, thirdPartyDir
}

// writeClaudeDesktopTestFixture 写入测试 fixture 文件（自动创建父目录）
func writeClaudeDesktopTestFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建 fixture 目录失败: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("写入 fixture %s 失败: %v", path, err)
	}
}

// readClaudeDesktopTestJSON 读取并解析测试 JSON 文件
func readClaudeDesktopTestJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("解析 %s 失败: %v", path, err)
	}
}

// TestClaudeDesktopApplyDirect 直连模式：4 文件事务写入 + 用户键保留 + 状态摘要
func TestClaudeDesktopApplyDirect(t *testing.T) {
	useIsolatedHomeDir(t)
	normalDir, thirdPartyDir := claudeDesktopTestDirs(t)

	// 预置用户既有配置（mcpServers 等键必须保留）
	writeClaudeDesktopTestFixture(t, claudeDesktopConfigPath(normalDir),
		[]byte(`{"deploymentMode":"1p","mcpServers":{"fs":{"command":"npx"}}}`))

	if err := SaveProvidersToStore(string(PlatformClaudeDesktop), []Provider{{
		ID:       1,
		Name:     "PackyCode",
		APIURL:   "https://packycode.example.com/",
		APIKey:   "sk-direct-key",
		Enabled:  true,
		Category: "custom",
	}}); err != nil {
		t.Fatalf("保存供应商失败: %v", err)
	}

	service := NewClaudeDesktopSettingsService(":18100")
	if err := service.ApplySingleProvider(1); err != nil {
		t.Fatalf("应用直连供应商失败: %v", err)
	}

	// 两目录 claude_desktop_config.json：deploymentMode=3p，normal 目录保留用户键
	for _, dir := range []string{normalDir, thirdPartyDir} {
		config, err := readClaudeDesktopConfigMap(claudeDesktopConfigPath(dir))
		if err != nil {
			t.Fatalf("读取 %s 配置失败: %v", dir, err)
		}
		if config[claudeDesktopDeploymentModeKey] != claudeDesktopDeploymentMode3p {
			t.Errorf("目录 %s 的 deploymentMode 期望 3p，实际 %v", dir, config[claudeDesktopDeploymentModeKey])
		}
		if _, ok := config["mcpServers"]; dir == normalDir && !ok {
			t.Errorf("normal 目录的用户 mcpServers 键被误删")
		}
	}

	// 3p profile：直连 baseUrl/key + 固定网关字段 + 默认四模型
	profile, exists, err := readClaudeDesktopProfile()
	if err != nil {
		t.Fatalf("读取 profile 失败: %v", err)
	}
	if !exists {
		t.Fatal("期望 3p profile 已写入")
	}
	if profile.InferenceGatewayBaseURL != "https://packycode.example.com" {
		t.Errorf("profile baseUrl 期望规范化后的直连地址，实际 %s", profile.InferenceGatewayBaseURL)
	}
	if profile.InferenceGatewayAPIKey != "sk-direct-key" {
		t.Errorf("profile apiKey 期望供应商密钥，实际 %s", profile.InferenceGatewayAPIKey)
	}
	if profile.InferenceGatewayAuthScheme != "bearer" {
		t.Errorf("profile authScheme 期望 bearer，实际 %s", profile.InferenceGatewayAuthScheme)
	}
	if profile.InferenceProvider != "gateway" {
		t.Errorf("profile inferenceProvider 期望 gateway，实际 %s", profile.InferenceProvider)
	}
	if !profile.DisableDeploymentModeChooser {
		t.Error("profile disableDeploymentModeChooser 期望 true")
	}
	if len(profile.CoworkEgressAllowedHosts) != 1 || profile.CoworkEgressAllowedHosts[0] != "*" {
		t.Errorf("profile coworkEgressAllowedHosts 期望 [\"*\"]，实际 %v", profile.CoworkEgressAllowedHosts)
	}
	if len(profile.InferenceModels) != 4 || profile.InferenceModels[0].Name != "claude-sonnet-5" {
		t.Errorf("profile inferenceModels 期望默认四模型，实际 %v", profile.InferenceModels)
	}

	// _meta.json：activeProfileId 指向固定 profile
	var meta claudeDesktopMeta
	readClaudeDesktopTestJSON(t, claudeDesktopMetaPath(thirdPartyDir), &meta)
	if meta.ActiveProfileID != claudeDesktopProfileID {
		t.Errorf("_meta activeProfileId 期望 %s，实际 %s", claudeDesktopProfileID, meta.ActiveProfileID)
	}

	// 状态摘要：非官方、direct、按 APIURL 反查到供应商名
	status, err := service.GetStatus()
	if err != nil {
		t.Fatalf("读取状态失败: %v", err)
	}
	if status["official"] != false {
		t.Errorf("状态 official 期望 false，实际 %v", status["official"])
	}
	if status["mode"] != claudeDesktopModeDirect {
		t.Errorf("状态 mode 期望 direct，实际 %v", status["mode"])
	}
	if status["providerName"] != "PackyCode" {
		t.Errorf("状态 providerName 期望 PackyCode，实际 %v", status["providerName"])
	}
	if status["providerId"] != int64(1) {
		t.Errorf("状态 providerId 期望按 baseUrl 反查到 1，实际 %v", status["providerId"])
	}

	// 供应商已从存储删除（如用户删除后）：反查未命中回退 0，前端据此走新增态
	if err := SaveProvidersToStore(string(PlatformClaudeDesktop), []Provider{}); err != nil {
		t.Fatalf("清空供应商失败: %v", err)
	}
	status, err = service.GetStatus()
	if err != nil {
		t.Fatalf("读取状态失败: %v", err)
	}
	if status["providerId"] != int64(0) {
		t.Errorf("反查未命中时 providerId 期望 0，实际 %v", status["providerId"])
	}

	// 直连态代理开关应为关闭
	proxyStatus, err := service.ProxyStatus()
	if err != nil {
		t.Fatalf("读取代理状态失败: %v", err)
	}
	if proxyStatus.Enabled {
		t.Error("直连态代理状态期望关闭")
	}
}

// TestClaudeDesktopApplyProxy 代理模式：指向本地代理 + 网关 token 生成与复用
func TestClaudeDesktopApplyProxy(t *testing.T) {
	useIsolatedHomeDir(t)
	_, _ = claudeDesktopTestDirs(t)

	if err := SaveProvidersToStore(string(PlatformClaudeDesktop), []Provider{{
		ID:                1,
		Name:              "本地代理供应商",
		APIURL:            "",
		APIKey:            "",
		Enabled:           true,
		Category:          "custom",
		ClaudeDesktopMode: claudeDesktopModeProxy,
	}}); err != nil {
		t.Fatalf("保存供应商失败: %v", err)
	}

	service := NewClaudeDesktopSettingsService(":18100")
	if err := service.ApplySingleProvider(1); err != nil {
		t.Fatalf("应用代理供应商失败: %v", err)
	}

	profile, exists, err := readClaudeDesktopProfile()
	if err != nil || !exists {
		t.Fatalf("读取 profile 失败: exists=%v err=%v", exists, err)
	}
	if profile.InferenceGatewayBaseURL != "http://127.0.0.1:18100/v1/messages" {
		t.Errorf("代理态 baseUrl 期望本地代理地址，实际 %s", profile.InferenceGatewayBaseURL)
	}
	token := profile.InferenceGatewayAPIKey
	if len(token) != 64 {
		t.Errorf("网关 token 期望 32 字节 hex（64 字符），实际长度 %d", len(token))
	}
	if _, decErr := hex.DecodeString(token); decErr != nil {
		t.Errorf("网关 token 不是合法 hex: %v", decErr)
	}

	proxyStatus, err := service.ProxyStatus()
	if err != nil {
		t.Fatalf("读取代理状态失败: %v", err)
	}
	if !proxyStatus.Enabled {
		t.Error("代理态 ProxyStatus 期望开启")
	}

	// 再次应用：token 从 app_settings 读取，保持稳定
	if err := service.ApplySingleProvider(1); err != nil {
		t.Fatalf("二次应用代理供应商失败: %v", err)
	}
	profileAgain, _, err := readClaudeDesktopProfile()
	if err != nil {
		t.Fatalf("二次读取 profile 失败: %v", err)
	}
	if profileAgain.InferenceGatewayAPIKey != token {
		t.Errorf("网关 token 期望复用持久化值，实际 %s", profileAgain.InferenceGatewayAPIKey)
	}
}

// TestClaudeDesktopApplyOfficial 官方模式：deploymentMode=1p + 清除 enterpriseConfig 与 profile 自定义内容
func TestClaudeDesktopApplyOfficial(t *testing.T) {
	useIsolatedHomeDir(t)
	normalDir, thirdPartyDir := claudeDesktopTestDirs(t)

	// 预置第三方态现场：两目录 3p 配置（含 cc-switch 的 enterpriseConfig）+ 3p profile/_meta
	for _, dir := range []string{normalDir, thirdPartyDir} {
		writeClaudeDesktopTestFixture(t, claudeDesktopConfigPath(dir),
			[]byte(`{"deploymentMode":"3p","enterpriseConfig":{"host":"legacy"},"mcpServers":{"fs":{"command":"npx"}}}`))
	}
	writeClaudeDesktopTestFixture(t, claudeDesktopProfilePath(thirdPartyDir),
		[]byte(`{"inferenceProvider":"gateway","inferenceGatewayBaseUrl":"https://old.example.com","inferenceGatewayApiKey":"k","inferenceGatewayAuthScheme":"bearer","inferenceModels":[{"name":"claude-sonnet-5"}]}`))
	writeClaudeDesktopTestFixture(t, claudeDesktopMetaPath(thirdPartyDir),
		[]byte(`{"activeProfileId":"`+claudeDesktopProfileID+`"}`))
	// normalDir 的 configLibrary 属用户本地资产（手写 profile 库），官方回退不得删除
	writeClaudeDesktopTestFixture(t, claudeDesktopProfilePath(normalDir),
		[]byte(`{"inferenceProvider":"gateway","inferenceGatewayBaseUrl":"https://user-local.example.com"}`))
	writeClaudeDesktopTestFixture(t, claudeDesktopMetaPath(normalDir),
		[]byte(`{"activeProfileId":"user-local"}`))

	if err := SaveProvidersToStore(string(PlatformClaudeDesktop), []Provider{{
		ID:       1,
		Name:     "官方",
		Enabled:  true,
		Category: "official",
	}}); err != nil {
		t.Fatalf("保存官方供应商失败: %v", err)
	}

	service := NewClaudeDesktopSettingsService(":18100")
	if err := service.ApplySingleProvider(1); err != nil {
		t.Fatalf("应用官方供应商失败: %v", err)
	}

	for _, dir := range []string{normalDir, thirdPartyDir} {
		config, err := readClaudeDesktopConfigMap(claudeDesktopConfigPath(dir))
		if err != nil {
			t.Fatalf("读取 %s 配置失败: %v", dir, err)
		}
		if config[claudeDesktopDeploymentModeKey] != claudeDesktopDeploymentMode1p {
			t.Errorf("目录 %s 的 deploymentMode 期望 1p，实际 %v", dir, config[claudeDesktopDeploymentModeKey])
		}
		if _, ok := config[claudeDesktopEnterpriseConfigKey]; ok {
			t.Errorf("目录 %s 的 enterpriseConfig 未删除", dir)
		}
		if _, ok := config["mcpServers"]; !ok {
			t.Errorf("目录 %s 的用户 mcpServers 键被误删", dir)
		}
	}
	if _, err := os.Stat(claudeDesktopProfilePath(thirdPartyDir)); !os.IsNotExist(err) {
		t.Errorf("官方态 3p profile 未删除: %v", err)
	}
	if _, err := os.Stat(claudeDesktopMetaPath(thirdPartyDir)); !os.IsNotExist(err) {
		t.Errorf("官方态 3p _meta 未删除: %v", err)
	}
	// normalDir 的 configLibrary 用户资产保留
	if _, err := os.Stat(claudeDesktopProfilePath(normalDir)); err != nil {
		t.Errorf("官方态不应删除 normalDir 的用户本地 profile: %v", err)
	}
	if _, err := os.Stat(claudeDesktopMetaPath(normalDir)); err != nil {
		t.Errorf("官方态不应删除 normalDir 的用户本地 _meta: %v", err)
	}

	status, err := service.GetStatus()
	if err != nil {
		t.Fatalf("读取状态失败: %v", err)
	}
	if status["official"] != true {
		t.Errorf("官方态状态 official 期望 true，实际 %v", status["official"])
	}
}

// TestClaudeDesktopApplyRollback 失败回滚：任一文件写入失败时整体恢复快照
func TestClaudeDesktopApplyRollback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 下 chmod 不阻止目录写入，跳过回滚用例")
	}
	useIsolatedHomeDir(t)
	normalDir, thirdPartyDir := claudeDesktopTestDirs(t)

	// 预置两目录既有配置（回滚后必须恢复原字节）
	originalNormal := []byte(`{"deploymentMode":"1p","mcpServers":{"fs":{"command":"npx"}}}`)
	originalThirdParty := []byte(`{"deploymentMode":"3p","mcpServers":{"fs":{"command":"npx"}}}`)
	writeClaudeDesktopTestFixture(t, claudeDesktopConfigPath(normalDir), originalNormal)
	writeClaudeDesktopTestFixture(t, claudeDesktopConfigPath(thirdPartyDir), originalThirdParty)

	// configLibrary 预置为只读目录：快照阶段（读不存在文件）不受影响，profile 写入在临时文件创建阶段失败
	configLibraryDir := filepath.Join(thirdPartyDir, claudeDesktopConfigLibraryDir)
	if err := os.MkdirAll(configLibraryDir, 0o755); err != nil {
		t.Fatalf("创建 configLibrary 失败: %v", err)
	}
	if err := os.Chmod(configLibraryDir, 0o555); err != nil {
		t.Fatalf("设置 configLibrary 只读失败: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(configLibraryDir, 0o755) })

	if err := SaveProvidersToStore(string(PlatformClaudeDesktop), []Provider{{
		ID:       1,
		Name:     "PackyCode",
		APIURL:   "https://packycode.example.com",
		APIKey:   "sk-key",
		Enabled:  true,
		Category: "custom",
	}}); err != nil {
		t.Fatalf("保存供应商失败: %v", err)
	}

	service := NewClaudeDesktopSettingsService(":18100")
	if err := service.ApplySingleProvider(1); err == nil {
		t.Fatal("期望写入失败返回错误")
	} else if !strings.Contains(err.Error(), "已回滚") {
		t.Errorf("失败错误应提示已回滚，实际: %v", err)
	}

	// 回滚校验：两目录配置恢复原字节，profile/_meta 未落盘
	for path, expected := range map[string][]byte{
		claudeDesktopConfigPath(normalDir):     originalNormal,
		claudeDesktopConfigPath(thirdPartyDir): originalThirdParty,
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("回滚后读取 %s 失败: %v", path, err)
		}
		if string(data) != string(expected) {
			t.Errorf("回滚后 %s 期望恢复原字节，实际: %s", path, data)
		}
	}
	for _, path := range []string{
		claudeDesktopProfilePath(thirdPartyDir),
		claudeDesktopMetaPath(thirdPartyDir),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("回滚后 %s 不应存在: %v", path, err)
		}
	}
}

// TestClaudeDesktopImportFromLive 首次导入：检测 3p profile 的网关配置并转为 direct 供应商
func TestClaudeDesktopImportFromLive(t *testing.T) {
	useIsolatedHomeDir(t)
	_, thirdPartyDir := claudeDesktopTestDirs(t)

	writeClaudeDesktopTestFixture(t, claudeDesktopProfilePath(thirdPartyDir),
		[]byte(`{"inferenceProvider":"gateway","inferenceGatewayBaseUrl":"https://my-gw.example.com/v1/","inferenceGatewayApiKey":"sk-live","inferenceGatewayAuthScheme":"bearer","inferenceModels":[{"name":"claude-sonnet-5","labelOverride":"Sonnet","supports1m":true},{"name":"claude-opus-5"}]}`))

	service := NewClaudeDesktopSettingsService(":18100")
	imported, err := service.ImportFromLive()
	if err != nil {
		t.Fatalf("首次导入失败: %v", err)
	}
	if !imported {
		t.Fatal("期望检测到网关配置并导入")
	}

	providers, err := LoadProvidersFromStore(string(PlatformClaudeDesktop))
	if err != nil {
		t.Fatalf("读取导入结果失败: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("期望导入 1 个供应商，实际 %d", len(providers))
	}
	provider := providers[0]
	if provider.Name != "my-gw.example.com" {
		t.Errorf("导入供应商名期望取 baseUrl 主机名，实际 %s", provider.Name)
	}
	if provider.APIURL != "https://my-gw.example.com/v1" {
		t.Errorf("导入 APIURL 期望规范化（去尾斜杠），实际 %s", provider.APIURL)
	}
	if provider.APIKey != "sk-live" {
		t.Errorf("导入 APIKey 期望沿用 profile 密钥，实际 %s", provider.APIKey)
	}
	if provider.ClaudeDesktopMode != claudeDesktopModeDirect {
		t.Errorf("导入接入模式期望 direct，实际 %s", provider.ClaudeDesktopMode)
	}
	if len(provider.ClaudeDesktopModelRoutes) != 2 ||
		provider.ClaudeDesktopModelRoutes[0].LabelOverride != "Sonnet" ||
		!provider.ClaudeDesktopModelRoutes[0].Supports1M {
		t.Errorf("导入模型路由期望完整保留，实际 %v", provider.ClaudeDesktopModelRoutes)
	}

	// 已有供应商数据后不重复导入
	again, err := service.ImportFromLive()
	if err != nil {
		t.Fatalf("二次导入检查失败: %v", err)
	}
	if again {
		t.Error("已有供应商数据时不应重复导入")
	}
}
