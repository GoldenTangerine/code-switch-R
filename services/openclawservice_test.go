/**
 * @name: OpenClaw 配置服务测试
 * @Descripttion: 验证 additive 供应商 CRUD、切换标记、live 冲突守卫、env/tools/agents 子页读写与首次导入
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 12:30:00
 * @LastEditTime: 2026-08-17 12:30:00
 * @FilePath: services/openclawservice_test.go
 */

package services

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeOpenClawTestConfig 写入 JSON5 语料到隔离 HOME 的 live 配置
func writeOpenClawTestConfig(t *testing.T, homeDir string, content string) {
	t.Helper()
	configPath := filepath.Join(homeDir, openClawDirName, openClawConfigFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 OpenClaw 配置目录失败: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("写入 OpenClaw live 配置失败: %v", err)
	}
}

// TestOpenClawAddProviderWritesLiveEntry 新增供应商写入 camelCase live 条目并保留其他顶层键
func TestOpenClawAddProviderWritesLiveEntry(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeOpenClawTestConfig(t, homeDir, `{ env: { vars: { USER_KEY: 'keep', }, }, }`)

	service := &OpenClawService{}
	_, err := service.AddProvider(OpenClawProvider{
		Name:    "DeepSeek",
		APIURL:  "https://api.deepseek.com",
		APIKey:  "sk-openclaw",
		Model:   "deepseek-chat",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("新增 OpenClaw 供应商失败: %v", err)
	}

	providers := service.GetProviders()
	if len(providers) != 1 {
		t.Fatalf("供应商数量 = %d, want 1", len(providers))
	}
	if !strings.HasPrefix(providers[0].ID, "openclaw-") {
		t.Fatalf("自动生成 ID 前缀错误: %s", providers[0].ID)
	}

	live, err := readOpenClawLiveMap()
	if err != nil {
		t.Fatalf("读取 OpenClaw live 配置失败: %v", err)
	}
	models := openClawChildReadOnly(live, "models")
	rawProviders := openClawChildReadOnly(models, "providers")
	entry, ok := rawProviders[providers[0].ID].(map[string]any)
	if !ok {
		t.Fatalf("live 中缺少新增条目: %#v", rawProviders)
	}
	if entry["name"] != "DeepSeek" || entry["baseUrl"] != "https://api.deepseek.com" ||
		entry["apiKey"] != "sk-openclaw" || entry["model"] != "deepseek-chat" {
		t.Fatalf("live 条目字段错误: %#v", entry)
	}
	env := openClawChildReadOnly(live, "env")
	if openClawStringMap(env["vars"])["USER_KEY"] != "keep" {
		t.Fatalf("用户 env 节被破坏: %#v", live)
	}
}

// TestOpenClawAddProviderRejectsExistingLiveProvider live 冲突守卫
func TestOpenClawAddProviderRejectsExistingLiveProvider(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeOpenClawTestConfig(t, homeDir, `{ models: { providers: { manual: { baseUrl: 'https://x', }, }, }, }`)

	service := &OpenClawService{}
	_, err := service.AddProvider(OpenClawProvider{ID: "manual", Name: "Manual", Enabled: true})
	if err == nil {
		t.Fatal("期望新增已存在 live provider 时返回冲突错误")
	}
	if !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("冲突错误文案不正确: %v", err)
	}
}

// TestOpenClawSetCurrentProviderRadioMarksStore 切换 = 单选标记，live 条目共存保留
func TestOpenClawSetCurrentProviderRadioMarksStore(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeOpenClawTestConfig(t, homeDir, `{}`)

	service := &OpenClawService{}
	if _, err := service.AddProvider(OpenClawProvider{Name: "First", APIURL: "https://first", Enabled: true}); err != nil {
		t.Fatalf("新增第一个供应商失败: %v", err)
	}
	if _, err := service.AddProvider(OpenClawProvider{Name: "Second", APIURL: "https://second", Enabled: false}); err != nil {
		t.Fatalf("新增第二个供应商失败: %v", err)
	}
	providers := service.GetProviders()
	firstID, secondID := providers[0].ID, providers[1].ID

	if err := service.SetCurrentProvider(secondID); err != nil {
		t.Fatalf("切换当前供应商失败: %v", err)
	}
	marked := service.GetProviders()
	enabledCount := 0
	for _, provider := range marked {
		if provider.Enabled {
			enabledCount++
		}
	}
	if enabledCount != 1 {
		t.Fatalf("启用标记数量 = %d, want 1（单选）", enabledCount)
	}
	if !marked[1].Enabled || marked[0].Enabled {
		t.Fatalf("切换后标记错误: %#v", marked)
	}

	// additive 共存：两个条目都保留在 live
	liveProviders, err := readOpenClawLiveProviders()
	if err != nil {
		t.Fatalf("读取 live providers 失败: %v", err)
	}
	if liveProviders[firstID] == nil || liveProviders[secondID] == nil {
		t.Fatalf("期望两个条目共存于 live: %#v", liveProviders)
	}

	status, err := service.GetStatus()
	if err != nil {
		t.Fatalf("读取状态失败: %v", err)
	}
	if status["currentProviderId"] != secondID || status["providerCount"] != 2 {
		t.Fatalf("状态摘要错误: %#v", status)
	}
}

// TestOpenClawDeleteProviderRemovesLiveEntry 删除供应商同步移除 live 条目
func TestOpenClawDeleteProviderRemovesLiveEntry(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeOpenClawTestConfig(t, homeDir, `{}`)

	service := &OpenClawService{}
	if _, err := service.AddProvider(OpenClawProvider{Name: "Temp", Enabled: true}); err != nil {
		t.Fatalf("新增供应商失败: %v", err)
	}
	providerID := service.GetProviders()[0].ID
	if err := service.DeleteProvider(providerID); err != nil {
		t.Fatalf("删除供应商失败: %v", err)
	}

	if providers := service.GetProviders(); len(providers) != 0 {
		t.Fatalf("删除后供应商数量 = %d, want 0", len(providers))
	}
	liveProviders, err := readOpenClawLiveProviders()
	if err != nil {
		t.Fatalf("读取 live providers 失败: %v", err)
	}
	if _, exists := liveProviders[providerID]; exists {
		t.Fatalf("删除后 live 条目仍存在: %#v", liveProviders)
	}
}

// TestOpenClawImportFromLiveAdoptsAllEntries 导入全部 live 条目且不修改原生文件
func TestOpenClawImportFromLiveAdoptsAllEntries(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	liveJSON5 := `{
  // 手写配置
  models: {
    providers: {
      deepseek: { name: 'DeepSeek', baseUrl: 'https://api.deepseek.com', apiKey: 'sk-live', },
      'openrouter.io': { baseUrl: 'https://openrouter.ai/api/v1', model: 'a/b', },
    },
  },
}`
	writeOpenClawTestConfig(t, homeDir, liveJSON5)
	configPath := filepath.Join(homeDir, openClawDirName, openClawConfigFileName)
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("读取 live 配置失败: %v", err)
	}

	service := &OpenClawService{}
	imported, err := service.ImportFromLive()
	if err != nil {
		t.Fatalf("导入 live providers 失败: %v", err)
	}
	if imported != 2 {
		t.Fatalf("导入数量 = %d, want 2", imported)
	}

	providers := service.GetProviders()
	if len(providers) != 2 {
		t.Fatalf("供应商数量 = %d, want 2", len(providers))
	}
	byID := map[string]OpenClawProvider{}
	for _, provider := range providers {
		byID[provider.ID] = provider
	}
	deepseek := byID["deepseek"]
	if deepseek.APIURL != "https://api.deepseek.com" || deepseek.APIKey != "sk-live" || !deepseek.Enabled {
		t.Fatalf("deepseek 导入字段错误: %#v", deepseek)
	}
	if byID["openrouter.io"].Model != "a/b" {
		t.Fatalf("openrouter.io 导入字段错误: %#v", byID["openrouter.io"])
	}

	// 重复导入幂等
	if imported, err = service.ImportFromLive(); err != nil || imported != 0 {
		t.Fatalf("重复导入应幂等: imported=%d err=%v", imported, err)
	}

	// 导入不修改原生文件
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("读取 live 配置失败: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("导入不应修改原生文件")
	}
}

// TestOpenClawNewServiceImportsFromLiveOnFirstRun 构造器首次接入：存储为空时自动导入
func TestOpenClawNewServiceImportsFromLiveOnFirstRun(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeOpenClawTestConfig(t, homeDir, `{ models: { providers: { only: { baseUrl: 'https://only', }, }, }, }`)

	service := NewOpenClawService()
	providers := service.GetProviders()
	if len(providers) != 1 || providers[0].ID != "only" {
		t.Fatalf("首次导入失败: %#v", providers)
	}

	// 导入结果已落入统一存储（重建服务后仍可读取）
	reloaded := NewOpenClawService()
	if reloadedProviders := reloaded.GetProviders(); len(reloadedProviders) != 1 || reloadedProviders[0].ID != "only" {
		t.Fatalf("统一存储读取失败: %#v", reloadedProviders)
	}
}

// TestOpenClawEnvConfigRoundTrip env 子页读写并保留其他顶层键
func TestOpenClawEnvConfigRoundTrip(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeOpenClawTestConfig(t, homeDir, `{ models: { providers: { keep: { baseUrl: 'https://keep', }, }, }, }`)

	service := &OpenClawService{}
	if err := service.SetEnvConfig(
		map[string]string{"API_TIMEOUT": "30"},
		map[string]string{"HTTP_PROXY": "http://127.0.0.1:7890"},
	); err != nil {
		t.Fatalf("写入 env 配置失败: %v", err)
	}
	config, err := service.GetEnvConfig()
	if err != nil {
		t.Fatalf("读取 env 配置失败: %v", err)
	}
	if config.Vars["API_TIMEOUT"] != "30" || config.ShellEnv["HTTP_PROXY"] != "http://127.0.0.1:7890" {
		t.Fatalf("env 读写往返错误: %#v", config)
	}

	liveProviders, err := readOpenClawLiveProviders()
	if err != nil {
		t.Fatalf("读取 live providers 失败: %v", err)
	}
	if liveProviders["keep"] == nil {
		t.Fatalf("SetEnvConfig 破坏了 models 节")
	}
}

// TestOpenClawToolsConfigRoundTrip tools 子页读写与 profile 校验
func TestOpenClawToolsConfigRoundTrip(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeOpenClawTestConfig(t, homeDir, `{}`)

	service := &OpenClawService{}
	if err := service.SetToolsConfig("coding", []string{"git", "ls"}, []string{"rm"}); err != nil {
		t.Fatalf("写入 tools 配置失败: %v", err)
	}
	config, err := service.GetToolsConfig()
	if err != nil {
		t.Fatalf("读取 tools 配置失败: %v", err)
	}
	if config.Profile != "coding" || !reflect.DeepEqual(config.Allow, []string{"git", "ls"}) || !reflect.DeepEqual(config.Deny, []string{"rm"}) {
		t.Fatalf("tools 读写往返错误: %#v", config)
	}

	if err := service.SetToolsConfig("unknown", nil, nil); err == nil {
		t.Fatal("期望非法 profile 返回错误")
	}
}

// TestOpenClawAgentsConfigRoundTrip agents.defaults 子页透传读写
func TestOpenClawAgentsConfigRoundTrip(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeOpenClawTestConfig(t, homeDir, `{}`)

	service := &OpenClawService{}
	defaults := map[string]any{
		"model":          map[string]any{"primary": "only/primary", "fallbacks": []any{"only/fallback"}},
		"timeoutSeconds": 120,
	}
	if err := service.SetAgentsConfig(defaults); err != nil {
		t.Fatalf("写入 agents 配置失败: %v", err)
	}
	got, err := service.GetAgentsConfig()
	if err != nil {
		t.Fatalf("读取 agents 配置失败: %v", err)
	}
	if !reflect.DeepEqual(got["timeoutSeconds"], float64(120)) {
		t.Fatalf("timeoutSeconds = %#v, want 120", got["timeoutSeconds"])
	}
	model := openClawChildReadOnly(got, "model")
	if model["primary"] != "only/primary" {
		t.Fatalf("model.primary = %#v", model["primary"])
	}
}
