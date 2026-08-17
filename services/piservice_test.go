/**
 * @name: Pi 配置服务测试
 * @Descripttion: 验证 models.json 的 additive 供应商 CRUD、切换标记、live 冲突守卫、未知键保留与首次导入
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 01:25:00
 * @LastEditTime: 2026-08-17 01:25:00
 * @FilePath: services/piservice_test.go
 */

package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePiTestConfig 写入 JSON 语料到隔离 HOME 的 live 配置
func writePiTestConfig(t *testing.T, homeDir string, content string) {
	t.Helper()
	configPath := filepath.Join(homeDir, piDirName, piAgentChildDir, piModelsFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 Pi 配置目录失败: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("写入 Pi live 配置失败: %v", err)
	}
}

// TestPiAddProviderWritesLiveEntry 新增供应商写入 live 条目并保留其他条目与顶层键
func TestPiAddProviderWritesLiveEntry(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writePiTestConfig(t, homeDir, `{
  "$schema": "keep-me",
  "providers": {
    "manual": {
      "displayName": "Manual",
      "baseUrl": "https://manual.example.com",
      "api": "openai-completions",
      "models": [{ "id": "m-1" }]
    }
  }
}`)

	service := &PiService{}
	_, err := service.AddProvider(PiProvider{
		Name:    "DeepSeek",
		BaseURL: "https://api.deepseek.com",
		APIKey:  "sk-pi",
		Enabled: true,
		CLIConfig: map[string]any{
			"api": "openai-completions",
		},
	})
	if err != nil {
		t.Fatalf("新增 Pi 供应商失败: %v", err)
	}

	providers := service.GetProviders()
	if len(providers) != 1 {
		t.Fatalf("供应商数量 = %d, want 1", len(providers))
	}
	if !strings.HasPrefix(providers[0].ID, "pi-") {
		t.Fatalf("自动生成 ID 前缀错误: %s", providers[0].ID)
	}

	live, err := readPiLiveMap()
	if err != nil {
		t.Fatalf("读取 Pi live 配置失败: %v", err)
	}
	if live["$schema"] != "keep-me" {
		t.Fatalf("未知顶层键被破坏: %#v", live)
	}
	rawProviders, _ := live["providers"].(map[string]any)

	// 新增条目：托管字段 + 原生片段中的 api 键都要写入
	entry, ok := rawProviders[providers[0].ID].(map[string]any)
	if !ok {
		t.Fatalf("live 中缺少新增条目: %#v", rawProviders)
	}
	if entry["displayName"] != "DeepSeek" || entry["baseUrl"] != "https://api.deepseek.com" ||
		entry["apiKey"] != "sk-pi" || entry["api"] != "openai-completions" {
		t.Fatalf("live 条目字段错误: %#v", entry)
	}

	// 用户手写条目原样保留（含 api/models 未知键）
	manual, ok := rawProviders["manual"].(map[string]any)
	if !ok || manual["baseUrl"] != "https://manual.example.com" || manual["api"] != "openai-completions" {
		t.Fatalf("用户手写条目被破坏: %#v", rawProviders["manual"])
	}
	models, _ := manual["models"].([]any)
	if len(models) != 1 {
		t.Fatalf("手写条目 models 数组被破坏: %#v", manual["models"])
	}
}

// TestPiAddProviderRejectsExistingLiveProvider live 冲突守卫
func TestPiAddProviderRejectsExistingLiveProvider(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writePiTestConfig(t, homeDir, `{"providers": {"manual": {"baseUrl": "https://x"}}}`)

	service := &PiService{}
	_, err := service.AddProvider(PiProvider{ID: "manual", Name: "Manual", Enabled: true})
	if err == nil {
		t.Fatal("期望新增已存在 live provider 时返回冲突错误")
	}
	if !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("冲突错误文案不正确: %v", err)
	}
}

// TestPiSetCurrentProviderRadioMarksStore 切换 = 单选标记，live 条目共存保留
func TestPiSetCurrentProviderRadioMarksStore(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writePiTestConfig(t, homeDir, `{}`)

	service := &PiService{}
	if _, err := service.AddProvider(PiProvider{Name: "First", BaseURL: "https://first", Enabled: true}); err != nil {
		t.Fatalf("新增第一个供应商失败: %v", err)
	}
	if _, err := service.AddProvider(PiProvider{Name: "Second", BaseURL: "https://second", Enabled: false}); err != nil {
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
	liveProviders, err := readPiLiveProviders()
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

// TestPiUpdateProviderRoundTrip 更新供应商：托管字段替换、原生片段键保留
func TestPiUpdateProviderRoundTrip(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writePiTestConfig(t, homeDir, `{}`)

	service := &PiService{}
	if _, err := service.AddProvider(PiProvider{
		Name:    "Old",
		BaseURL: "https://old",
		APIKey:  "sk-old",
		Enabled: true,
		CLIConfig: map[string]any{
			"api":    "anthropic-messages",
			"compat": map[string]any{"supportsDeveloperRole": false},
		},
	}); err != nil {
		t.Fatalf("新增供应商失败: %v", err)
	}
	providerID := service.GetProviders()[0].ID

	if _, err := service.UpdateProvider(PiProvider{
		ID:        providerID,
		Name:      "New",
		BaseURL:   "https://new",
		APIKey:    "sk-new",
		Model:     "claude-sonnet-4-5",
		Enabled:   true,
		CLIConfig: map[string]any{
			"api":    "anthropic-messages",
			"compat": map[string]any{"supportsDeveloperRole": false},
		},
	}); err != nil {
		t.Fatalf("更新供应商失败: %v", err)
	}

	updated := service.GetProviders()[0]
	if updated.Name != "New" || updated.BaseURL != "https://new" || updated.Model != "claude-sonnet-4-5" {
		t.Fatalf("更新后内存字段错误: %#v", updated)
	}

	liveProviders, err := readPiLiveProviders()
	if err != nil {
		t.Fatalf("读取 live providers 失败: %v", err)
	}
	entry, ok := liveProviders[providerID]
	if !ok {
		t.Fatalf("live 中缺少条目: %#v", liveProviders)
	}
	if entry["displayName"] != "New" || entry["baseUrl"] != "https://new" || entry["apiKey"] != "sk-new" {
		t.Fatalf("live 条目托管字段错误: %#v", entry)
	}
	// Model 为应用侧元数据，不应写入 live 条目
	if _, exists := entry["model"]; exists {
		t.Fatalf("Model 不应写入 live 条目: %#v", entry)
	}
	// 原生片段键随更新保留
	if entry["api"] != "anthropic-messages" {
		t.Fatalf("原生片段 api 键丢失: %#v", entry)
	}
}

// TestPiDeleteProviderRemovesLiveEntry 删除供应商同步移除 live 条目
func TestPiDeleteProviderRemovesLiveEntry(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writePiTestConfig(t, homeDir, `{}`)

	service := &PiService{}
	if _, err := service.AddProvider(PiProvider{Name: "Temp", Enabled: true}); err != nil {
		t.Fatalf("新增供应商失败: %v", err)
	}
	providerID := service.GetProviders()[0].ID
	if err := service.DeleteProvider(providerID); err != nil {
		t.Fatalf("删除供应商失败: %v", err)
	}

	if providers := service.GetProviders(); len(providers) != 0 {
		t.Fatalf("删除后供应商数量 = %d, want 0", len(providers))
	}
	liveProviders, err := readPiLiveProviders()
	if err != nil {
		t.Fatalf("读取 live providers 失败: %v", err)
	}
	if _, exists := liveProviders[providerID]; exists {
		t.Fatalf("删除后 live 条目仍存在: %#v", liveProviders)
	}
}

// TestPiImportFromLiveAdoptsAllEntries 导入全部 live 条目且不修改原生文件
func TestPiImportFromLiveAdoptsAllEntries(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writePiTestConfig(t, homeDir, `{
  "providers": {
    "ollama": {
      "displayName": "Ollama",
      "baseUrl": "http://localhost:11434/v1",
      "api": "openai-completions",
      "models": [{ "id": "llama3" }]
    },
    "openrouter.io": {
      "baseUrl": "https://openrouter.ai/api/v1",
      "apiKey": "sk-live"
    }
  }
}`)
	configPath := filepath.Join(homeDir, piDirName, piAgentChildDir, piModelsFileName)
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("读取 live 配置失败: %v", err)
	}

	service := &PiService{}
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
	byID := map[string]PiProvider{}
	for _, provider := range providers {
		byID[provider.ID] = provider
	}
	ollama := byID["ollama"]
	if ollama.Name != "Ollama" || ollama.BaseURL != "http://localhost:11434/v1" || !ollama.Enabled {
		t.Fatalf("ollama 导入字段错误: %#v", ollama)
	}
	if ollama.CLIConfig["api"] != "openai-completions" {
		t.Fatalf("ollama 原生片段丢失: %#v", ollama.CLIConfig)
	}
	// 无 displayName 的条目回退条目 ID 作为名称
	if byID["openrouter.io"].Name != "openrouter.io" || byID["openrouter.io"].APIKey != "sk-live" {
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

// TestPiNewServiceImportsFromLiveOnFirstRun 构造器首次接入：存储为空时自动导入
func TestPiNewServiceImportsFromLiveOnFirstRun(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writePiTestConfig(t, homeDir, `{"providers": {"only": {"baseUrl": "https://only"}}}`)

	service := NewPiService()
	providers := service.GetProviders()
	if len(providers) != 1 || providers[0].ID != "only" {
		t.Fatalf("首次导入失败: %#v", providers)
	}

	// 导入结果已落入统一存储（重建服务后仍可读取）
	reloaded := NewPiService()
	if reloadedProviders := reloaded.GetProviders(); len(reloadedProviders) != 1 || reloadedProviders[0].ID != "only" {
		t.Fatalf("统一存储读取失败: %#v", reloadedProviders)
	}
}

// TestPiDuplicateProviderCreatesDisabledCopy 复制供应商生成新 ID 且默认未启用
func TestPiDuplicateProviderCreatesDisabledCopy(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writePiTestConfig(t, homeDir, `{}`)

	service := &PiService{}
	if _, err := service.AddProvider(PiProvider{Name: "Source", BaseURL: "https://src", Enabled: true}); err != nil {
		t.Fatalf("新增供应商失败: %v", err)
	}
	sourceID := service.GetProviders()[0].ID

	duplicated, err := service.DuplicateProvider(sourceID)
	if err != nil {
		t.Fatalf("复制供应商失败: %v", err)
	}
	if duplicated == nil || duplicated.ID == sourceID || duplicated.Enabled {
		t.Fatalf("副本字段错误: %#v", duplicated)
	}
	if duplicated.Name != "Source (副本)" {
		t.Fatalf("副本名称错误: %s", duplicated.Name)
	}

	// live 中两个条目共存
	liveProviders, err := readPiLiveProviders()
	if err != nil {
		t.Fatalf("读取 live providers 失败: %v", err)
	}
	if liveProviders[sourceID] == nil || liveProviders[duplicated.ID] == nil {
		t.Fatalf("期望两个条目共存于 live: %#v", liveProviders)
	}
}
