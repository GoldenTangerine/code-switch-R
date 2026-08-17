/**
 * @name: Hermes 配置服务测试
 * @Descripttion: 验证 YAML 未知键/注释保留、custom_providers additive 写、model 节切换、memory § 切分与设置持久化
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 20:40:00
 * @LastEditTime: 2026-08-17 20:40:00
 * @FilePath: services/hermesservice_test.go
 */

package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// writeHermesTestConfig 写入 YAML 语料到隔离 HOME 的 live 配置
func writeHermesTestConfig(t *testing.T, homeDir string, content string) {
	t.Helper()
	configPath := filepath.Join(homeDir, hermesDirName, hermesConfigFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 Hermes 配置目录失败: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("写入 Hermes live 配置失败: %v", err)
	}
}

// readHermesTestConfigFile 读取 live 配置原始文本
func readHermesTestConfigFile(t *testing.T, homeDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(homeDir, hermesDirName, hermesConfigFileName))
	if err != nil {
		t.Fatalf("读取 Hermes live 配置失败: %v", err)
	}
	return string(data)
}

// TestHermesYAMLRoundTripPreservesUnknownKeysAndComments 新增供应商后未知键与注释保留
func TestHermesYAMLRoundTripPreservesUnknownKeysAndComments(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeHermesTestConfig(t, homeDir, `# 顶部注释
ui:
  theme: dark # 行内注释
logging:
  level: info
  rotate: 10
`)

	service := &HermesService{}
	if _, err := service.AddProvider(HermesProvider{
		Name:    "DeepSeek",
		BaseURL: "https://api.deepseek.com",
		APIKey:  "sk-hermes",
		Model:   "deepseek-chat",
		Enabled: true,
	}); err != nil {
		t.Fatalf("新增 Hermes 供应商失败: %v", err)
	}

	// 未知键保留（Node 级往返）
	doc, err := readHermesLiveNode()
	if err != nil {
		t.Fatalf("读取 Hermes live 配置失败: %v", err)
	}
	ui := hermesGetTopLevelValue(hermesRootMapping(doc), "ui")
	if ui == nil {
		t.Fatal("未知顶层键 ui 丢失")
	}
	uiMap, _ := hermesDecodeNode(ui).(map[string]any)
	if uiMap["theme"] != "dark" {
		t.Fatalf("ui.theme = %#v, want dark", uiMap["theme"])
	}
	logging := hermesGetTopLevelValue(hermesRootMapping(doc), "logging")
	loggingMap, _ := hermesDecodeNode(logging).(map[string]any)
	if loggingMap["level"] != "info" || loggingMap["rotate"] != 10 {
		t.Fatalf("logging 节被破坏: %#v", loggingMap)
	}

	// 注释保留（文本级检查）
	written := readHermesTestConfigFile(t, homeDir)
	if !strings.Contains(written, "# 顶部注释") || !strings.Contains(written, "# 行内注释") {
		t.Fatalf("注释丢失，写回内容:\n%s", written)
	}
}

// TestHermesAddProviderAppendsCustomProviders additive 写：手写无 id 条目保留 + 托管条目 snake_case
func TestHermesAddProviderAppendsCustomProviders(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeHermesTestConfig(t, homeDir, `custom_providers:
  - name: Manual
    base_url: https://manual.example
`)

	service := &HermesService{}
	if _, err := service.AddProvider(HermesProvider{
		Name:    "DeepSeek",
		BaseURL: "https://api.deepseek.com",
		APIKey:  "sk-hermes",
		Model:   "deepseek-chat",
	}); err != nil {
		t.Fatalf("新增 Hermes 供应商失败: %v", err)
	}

	providers := service.GetProviders()
	if len(providers) != 1 {
		t.Fatalf("供应商数量 = %d, want 1", len(providers))
	}
	if !strings.HasPrefix(providers[0].ID, "hermes-") {
		t.Fatalf("自动生成 ID 前缀错误: %s", providers[0].ID)
	}

	live, err := readHermesLiveCustomProviders()
	if err != nil {
		t.Fatalf("读取 live custom_providers 失败: %v", err)
	}
	if len(live) != 2 {
		t.Fatalf("live 条目数量 = %d, want 2（手写条目保留）", len(live))
	}
	managed := live[1]
	if managed["id"] != providers[0].ID || managed["name"] != "DeepSeek" ||
		managed["base_url"] != "https://api.deepseek.com" ||
		managed["api_key"] != "sk-hermes" || managed["model"] != "deepseek-chat" {
		t.Fatalf("live 托管条目字段错误: %#v", managed)
	}
	if live[0]["name"] != "Manual" || live[0]["base_url"] != "https://manual.example" {
		t.Fatalf("手写条目被破坏: %#v", live[0])
	}
}

// TestHermesSetCurrentProviderUpdatesModelNode 切换 = 更新顶层 model 节，custom_providers 全量共存
func TestHermesSetCurrentProviderUpdatesModelNode(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeHermesTestConfig(t, homeDir, `ui:
  theme: dark
`)

	service := &HermesService{}
	if _, err := service.AddProvider(HermesProvider{Name: "First", BaseURL: "https://first", Model: "first-model", Enabled: true}); err != nil {
		t.Fatalf("新增第一个供应商失败: %v", err)
	}
	if _, err := service.AddProvider(HermesProvider{Name: "Second", BaseURL: "https://second", Model: "second-model"}); err != nil {
		t.Fatalf("新增第二个供应商失败: %v", err)
	}
	providers := service.GetProviders()
	secondID := providers[1].ID

	if err := service.SetCurrentProvider(secondID); err != nil {
		t.Fatalf("切换当前供应商失败: %v", err)
	}

	// 单选标记
	marked := service.GetProviders()
	enabledCount := 0
	for _, provider := range marked {
		if provider.Enabled {
			enabledCount++
		}
	}
	if enabledCount != 1 || !marked[1].Enabled || marked[0].Enabled {
		t.Fatalf("切换后标记错误: %#v", marked)
	}

	// 顶层 model 节指向第二个供应商
	providerName, modelName := readHermesLiveModelSelection()
	if providerName != "Second" || modelName != "second-model" {
		t.Fatalf("model 节选择错误: provider=%q model=%q", providerName, modelName)
	}
	doc, err := readHermesLiveNode()
	if err != nil {
		t.Fatalf("读取 live 配置失败: %v", err)
	}
	modelNode := hermesGetTopLevelValue(hermesRootMapping(doc), "model")
	if modelNode == nil || modelNode.Kind != yaml.MappingNode {
		t.Fatalf("model 节应为映射: %#v", modelNode)
	}

	// additive 共存：两个条目都保留，未知键不丢
	live, err := readHermesLiveCustomProviders()
	if err != nil {
		t.Fatalf("读取 live custom_providers 失败: %v", err)
	}
	if len(live) != 2 {
		t.Fatalf("live 条目数量 = %d, want 2", len(live))
	}
	if hermesGetTopLevelValue(hermesRootMapping(doc), "ui") == nil {
		t.Fatal("未知顶层键 ui 丢失")
	}

	status, err := service.GetStatus()
	if err != nil {
		t.Fatalf("读取状态失败: %v", err)
	}
	if status["currentProviderId"] != secondID || status["providerCount"] != 2 {
		t.Fatalf("状态摘要错误: %#v", status)
	}
}

// TestHermesDeleteProviderRemovesLiveEntry 删除供应商同步移除 live 托管条目
func TestHermesDeleteProviderRemovesLiveEntry(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeHermesTestConfig(t, homeDir, `custom_providers:
  - name: Manual
    base_url: https://manual.example
`)

	service := &HermesService{}
	if _, err := service.AddProvider(HermesProvider{Name: "Temp", BaseURL: "https://temp"}); err != nil {
		t.Fatalf("新增供应商失败: %v", err)
	}
	providerID := service.GetProviders()[0].ID
	if err := service.DeleteProvider(providerID); err != nil {
		t.Fatalf("删除供应商失败: %v", err)
	}

	if providers := service.GetProviders(); len(providers) != 0 {
		t.Fatalf("删除后供应商数量 = %d, want 0", len(providers))
	}
	live, err := readHermesLiveCustomProviders()
	if err != nil {
		t.Fatalf("读取 live custom_providers 失败: %v", err)
	}
	if len(live) != 1 || live[0]["name"] != "Manual" {
		t.Fatalf("删除后 live 条目错误（手写条目应保留）: %#v", live)
	}
}

// TestHermesImportFromLiveAdoptsAllEntries 导入全部 live 条目且不修改原生文件（幂等）
func TestHermesImportFromLiveAdoptsAllEntries(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeHermesTestConfig(t, homeDir, `# 手写配置
custom_providers:
  - id: deepseek
    name: DeepSeek
    base_url: https://api.deepseek.com
    api_key: sk-live
    model: deepseek-chat
  - name: OpenRouter
    base_url: https://openrouter.ai/api/v1
    model: a/b
model:
  provider: DeepSeek
  model: deepseek-chat
`)
	configPath := filepath.Join(homeDir, hermesDirName, hermesConfigFileName)
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("读取 live 配置失败: %v", err)
	}

	service := &HermesService{}
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
	byID := map[string]HermesProvider{}
	for _, provider := range providers {
		byID[provider.ID] = provider
	}
	deepseek := byID["deepseek"]
	if deepseek.BaseURL != "https://api.deepseek.com" || deepseek.APIKey != "sk-live" || deepseek.Model != "deepseek-chat" {
		t.Fatalf("deepseek 导入字段错误: %#v", deepseek)
	}
	if !deepseek.Enabled {
		t.Fatal("model 节指向 DeepSeek，导入后应为启用态")
	}

	// 重复导入幂等（含无 id 条目按 name+base_url 去重）
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

// TestHermesImportThenSwitchWritesIDToNoIDEntry 导入无 id 条目后切换：仅回写 id，手写字段原样保留且不重复追加
func TestHermesImportThenSwitchWritesIDToNoIDEntry(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeHermesTestConfig(t, homeDir, `custom_providers:
  - name: Solo
    base_url: https://solo
    model: solo-model
    api: openai-completions
`)

	service := &HermesService{}
	if _, err := service.ImportFromLive(); err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	providerID := service.GetProviders()[0].ID

	if err := service.SetCurrentProvider(providerID); err != nil {
		t.Fatalf("切换失败: %v", err)
	}
	live, err := readHermesLiveCustomProviders()
	if err != nil {
		t.Fatalf("读取 live custom_providers 失败: %v", err)
	}
	if len(live) != 1 {
		t.Fatalf("切换后条目数量 = %d, want 1（不应重复追加）", len(live))
	}
	entry := live[0]
	if entry["id"] != providerID {
		t.Fatalf("无 id 条目切换后未回写托管 id: %#v", entry)
	}
	// 同名同 URL 静默覆盖语义：除追加 id 外，手写条目其余字段一律不动
	if entry["name"] != "Solo" || entry["base_url"] != "https://solo" || entry["model"] != "solo-model" {
		t.Fatalf("手写条目字段被覆写: %#v", entry)
	}
	if entry["api"] != "openai-completions" {
		t.Fatalf("手写条目未知键被破坏: %#v", entry)
	}
	if providerName, _ := readHermesLiveModelSelection(); providerName != "Solo" {
		t.Fatalf("model 节 provider = %q, want Solo", providerName)
	}
}

// TestHermesNewServiceImportsFromLiveOnFirstRun 构造器首次接入：存储为空时自动导入
func TestHermesNewServiceImportsFromLiveOnFirstRun(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeHermesTestConfig(t, homeDir, `custom_providers:
  - id: only
    name: Only
    base_url: https://only
`)

	service := NewHermesService()
	providers := service.GetProviders()
	if len(providers) != 1 || providers[0].ID != "only" {
		t.Fatalf("首次导入失败: %#v", providers)
	}

	// 导入结果已落入统一存储（重建服务后仍可读取）
	reloaded := NewHermesService()
	if reloadedProviders := reloaded.GetProviders(); len(reloadedProviders) != 1 || reloadedProviders[0].ID != "only" {
		t.Fatalf("统一存储读取失败: %#v", reloadedProviders)
	}
}

// TestHermesMCPSyncKeepsOtherTopLevelKeys MCP 投影只改 mcp_servers 子树，不覆盖其他顶层键
func TestHermesMCPSyncKeepsOtherTopLevelKeys(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeHermesTestConfig(t, homeDir, `custom_providers:
  - id: keep
    name: Keep
    base_url: https://keep
model:
  provider: Keep
  model: keep-model
`)

	ms := NewMCPService()
	if err := ms.syncHermesServers([]MCPServer{{
		Name:           "fetch",
		Type:           "stdio",
		Command:        "npx",
		Args:           []string{"-y", "fetch-mcp"},
		EnablePlatform: []string{"hermes"},
	}}); err != nil {
		t.Fatalf("MCP 投影失败: %v", err)
	}

	live, err := readHermesLiveCustomProviders()
	if err != nil {
		t.Fatalf("读取 live custom_providers 失败: %v", err)
	}
	if len(live) != 1 || live[0]["id"] != "keep" {
		t.Fatalf("MCP 投影破坏 custom_providers: %#v", live)
	}
	if providerName, modelName := readHermesLiveModelSelection(); providerName != "Keep" || modelName != "keep-model" {
		t.Fatalf("MCP 投影破坏 model 节: provider=%q model=%q", providerName, modelName)
	}

	enabled := loadHermesEnabledServers()
	if _, ok := enabled["fetch"]; !ok {
		t.Fatalf("mcp_servers 投影缺失: %#v", enabled)
	}
}

// TestHermesMemoryEntriesSplitAndJoin memory § 切分与整文件读写往返
func TestHermesMemoryEntriesSplitAndJoin(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)

	service := &HermesService{}
	content := "entry one line1\nline2\n§\nentry two\n\n§\n\n§\nentry three"
	if err := service.WriteMemoryContent(hermesMemoryKindMemory, content); err != nil {
		t.Fatalf("写入 MEMORY.md 失败: %v", err)
	}

	got, err := service.GetMemoryEntries(hermesMemoryKindMemory)
	if err != nil {
		t.Fatalf("读取 memory 条目失败: %v", err)
	}
	want := []string{"entry one line1\nline2", "entry two", "entry three"}
	if len(got) != len(want) {
		t.Fatalf("条目数量 = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("条目 %d = %q, want %q", i, got[i], want[i])
		}
	}

	// 整文件内容往返
	blob, err := service.GetMemoryContent(hermesMemoryKindMemory)
	if err != nil || blob != content {
		t.Fatalf("整文件读写往返错误: err=%v", err)
	}

	// user 类型落到 USER.md；缺失文件返回空串；非法类型报错
	if err := service.WriteMemoryContent(hermesMemoryKindUser, "profile"); err != nil {
		t.Fatalf("写入 USER.md 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(homeDir, hermesDirName, hermesMemoriesDirName, hermesUserFileName)); err != nil {
		t.Fatalf("USER.md 未创建: %v", err)
	}
	if missing, err := service.GetMemoryContent("memory"); err != nil || missing != content {
		t.Fatalf("memory 读取异常: %v", err)
	}
	if _, err := service.GetMemoryEntries("invalid"); err == nil {
		t.Fatal("非法 memory 类型应返回错误")
	}
}

// TestHermesMemorySettingsPersist memory 开关/预算写入 config.yaml 顶层并保留其他键
func TestHermesMemorySettingsPersist(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeHermesTestConfig(t, homeDir, `logging:
  level: info
`)

	service := &HermesService{}
	if err := service.SetMemorySettings(false, 1234, true, 567); err != nil {
		t.Fatalf("写入 memory 设置失败: %v", err)
	}

	settings, err := service.GetMemorySettings()
	if err != nil {
		t.Fatalf("读取 memory 设置失败: %v", err)
	}
	if settings.MemoryEnabled || settings.MemoryCharLimit != 1234 ||
		!settings.UserProfileEnabled || settings.UserCharLimit != 567 {
		t.Fatalf("memory 设置往返错误: %#v", settings)
	}

	// 其他顶层键保留
	doc, err := readHermesLiveNode()
	if err != nil {
		t.Fatalf("读取 live 配置失败: %v", err)
	}
	logging := hermesGetTopLevelValue(hermesRootMapping(doc), "logging")
	loggingMap, _ := hermesDecodeNode(logging).(map[string]any)
	if loggingMap["level"] != "info" {
		t.Fatalf("logging 节被破坏: %#v", loggingMap)
	}

	// 缺键回退默认值
	writeHermesTestConfig(t, homeDir, `logging:
  level: info
`)
	defaults, err := service.GetMemorySettings()
	if err != nil {
		t.Fatalf("读取默认设置失败: %v", err)
	}
	expected := defaultHermesMemorySettings()
	if *defaults != *expected {
		t.Fatalf("默认值错误: %#v", defaults)
	}
}
