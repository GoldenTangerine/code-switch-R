/**
 * @name: 供应商统一存储测试
 * @Descripttion: 验证 JSON→SQLite 迁移幂等性、nil/空语义与各平台适配层的字段往返一致性
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 04:05:00
 * @LastEditTime: 2026-08-17 04:05:00
 * @FilePath: services/providerstore_test.go
 */

package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
)

// writeLegacyFixtureJSON 写入指定路径的旧 JSON 文件
func writeLegacyFixtureJSON(t *testing.T, path string, content any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建 fixture 目录失败: %v", err)
	}
	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("序列化 fixture 失败: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("写入 fixture 失败: %v", err)
	}
}

func TestMigrateProviderJSONFilesToStoreAndIdempotency(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)

	// 四种格式的旧 JSON fixture
	writeLegacyFixtureJSON(t,
		filepath.Join(homeDir, ".code-switch", "claude-code.json"),
		providerEnvelope{Providers: []Provider{
			{ID: 1, Name: "Claude A", APIURL: "https://a.example.com", APIKey: "key-a", Enabled: true, Level: 2},
			{ID: 2, Name: "Claude B", APIURL: "https://b.example.com", APIKey: "key-b", Enabled: false},
		}})
	writeLegacyFixtureJSON(t,
		filepath.Join(homeDir, ".code-switch", "providers", "codex.json"),
		providerEnvelope{Providers: []Provider{
			{ID: 5, Name: "Codex A", APIURL: "https://c.example.com", APIKey: "key-c", Enabled: true},
		}})
	writeLegacyFixtureJSON(t,
		filepath.Join(homeDir, ".code-switch", "providers", "opencode.json"),
		opencodeProviderEnvelope{Providers: []OpenCodeProvider{
			{ID: "oc-1", Name: "OpenCode A", Enabled: true, NPM: "@ai-sdk/openai-compatible",
				SettingsConfig: map[string]any{"npm": "@ai-sdk/openai-compatible"}},
		}})
	writeLegacyFixtureJSON(t,
		filepath.Join(homeDir, ".code-switch", "gemini-providers.json"),
		[]GeminiProvider{
			{ID: "gem-1", Name: "Gemini A", Enabled: true, BaseURL: "https://g.example.com"},
		})

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateProviderJSONFilesToStore(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	// 验证各平台迁移行数与内容
	claudeLoaded, err := LoadProvidersFromStore("claude")
	if err != nil || len(claudeLoaded) != 2 || claudeLoaded[0].Name != "Claude A" || claudeLoaded[0].Level != 2 {
		t.Fatalf("claude 迁移结果错误: %#v, %v", claudeLoaded, err)
	}
	codexLoaded, err := LoadProvidersFromStore("codex")
	if err != nil || len(codexLoaded) != 1 || codexLoaded[0].Name != "Codex A" {
		t.Fatalf("codex 迁移结果错误: %#v, %v", codexLoaded, err)
	}
	openCodeLoaded, err := LoadOpenCodeProvidersFromStore()
	if err != nil || len(openCodeLoaded) != 1 || openCodeLoaded[0].ID != "oc-1" || openCodeLoaded[0].NPM != "@ai-sdk/openai-compatible" {
		t.Fatalf("opencode 迁移结果错误: %#v, %v", openCodeLoaded, err)
	}
	geminiLoaded, err := LoadGeminiProvidersFromStore()
	if err != nil || len(geminiLoaded) != 1 || geminiLoaded[0].ID != "gem-1" {
		t.Fatalf("gemini 迁移结果错误: %#v, %v", geminiLoaded, err)
	}

	// 验证原文件全部备份为 .bak
	for _, path := range []string{
		filepath.Join(homeDir, ".code-switch", "claude-code.json"),
		filepath.Join(homeDir, ".code-switch", "providers", "codex.json"),
		filepath.Join(homeDir, ".code-switch", "providers", "opencode.json"),
		filepath.Join(homeDir, ".code-switch", "gemini-providers.json"),
	} {
		if providerConfigFileExists(path) {
			t.Fatalf("迁移后原文件应改名 .bak: %s", path)
		}
		if !providerConfigFileExists(path + ".bak") {
			t.Fatalf("迁移后应生成 .bak 备份: %s", path+".bak")
		}
	}

	// 幂等：再次执行不重复、不报错、不改名已有 .bak
	if err := MigrateProviderJSONFilesToStore(db); err != nil {
		t.Fatalf("重复迁移应幂等: %v", err)
	}
	claudeAgain, err := LoadProvidersFromStore("claude")
	if err != nil || len(claudeAgain) != 2 {
		t.Fatalf("重复迁移后行数应保持不变: %#v, %v", claudeAgain, err)
	}
}

func TestMigrateSkipsPlatformWhenStoreAlreadyInitialized(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)

	// 先在 store 写入 claude 数据（模拟已在新版本使用）
	if err := SaveProvidersToStore("claude", []Provider{{ID: 9, Name: "Existing", Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	// 再放一份旧 JSON（不应被迁移覆盖）
	legacyPath := filepath.Join(homeDir, ".code-switch", "claude-code.json")
	writeLegacyFixtureJSON(t, legacyPath,
		providerEnvelope{Providers: []Provider{{ID: 1, Name: "Legacy Should Skip", Enabled: true}}})

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateProviderJSONFilesToStore(db); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadProvidersFromStore("claude")
	if err != nil || len(loaded) != 1 || loaded[0].Name != "Existing" {
		t.Fatalf("已初始化平台不应被旧 JSON 覆盖: %#v, %v", loaded, err)
	}
	// 存储已初始化的旧文件也应备份为 .bak（消除每次启动的重复解析）
	if providerConfigFileExists(legacyPath) || !providerConfigFileExists(legacyPath+".bak") {
		t.Fatalf("存储已初始化平台的旧文件应直接备份为 .bak: %s", legacyPath)
	}
}

// TestProviderStorePreservesSavedOrderAcrossReload 多元素乱序保存后加载必须保持保存顺序
// 覆盖两类 id：数值型（1/3/20）与字符串型（gemini-2/gemini-10），词典序排序会打乱用户顺序
func TestProviderStorePreservesSavedOrderAcrossReload(t *testing.T) {
	useIsolatedHomeDir(t)

	// Provider 数值 id：保存顺序 3 → 1 → 20（按数值排序会得到 1 → 3 → 20）
	if err := SaveProvidersToStore("claude", []Provider{
		{ID: 3, Name: "Third", Enabled: true},
		{ID: 1, Name: "First", Enabled: true},
		{ID: 20, Name: "Twentieth", Enabled: true},
	}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	loaded, err := LoadProvidersFromStore("claude")
	if err != nil || len(loaded) != 3 {
		t.Fatalf("回读失败: %#v, %v", loaded, err)
	}
	if loaded[0].ID != 3 || loaded[1].ID != 1 || loaded[2].ID != 20 {
		t.Fatalf("保存顺序丢失（期望 3→1→20）: %d→%d→%d", loaded[0].ID, loaded[1].ID, loaded[2].ID)
	}

	// Gemini 字符串 id：保存顺序 gemini-10 → gemini-2 → gemini-1（词典序会得到 gemini-1 → gemini-10 → gemini-2）
	if err := SaveGeminiProvidersToStore([]GeminiProvider{
		{ID: "gemini-10", Name: "G10", Enabled: true},
		{ID: "gemini-2", Name: "G2", Enabled: true},
		{ID: "gemini-1", Name: "G1", Enabled: true},
	}); err != nil {
		t.Fatalf("保存 Gemini 失败: %v", err)
	}
	gLoaded, err := LoadGeminiProvidersFromStore()
	if err != nil || len(gLoaded) != 3 {
		t.Fatalf("Gemini 回读失败: %#v, %v", gLoaded, err)
	}
	if gLoaded[0].ID != "gemini-10" || gLoaded[1].ID != "gemini-2" || gLoaded[2].ID != "gemini-1" {
		t.Fatalf("Gemini 保存顺序丢失（期望 gemini-10→gemini-2→gemini-1，不再按词典序）: %s→%s→%s",
			gLoaded[0].ID, gLoaded[1].ID, gLoaded[2].ID)
	}
}

// TestSaveProvidersToStoreRejectsDuplicateIDs 同平台重复 id 必须显式报错（主键会静默吞掉后一行）
func TestSaveProvidersToStoreRejectsDuplicateIDs(t *testing.T) {
	useIsolatedHomeDir(t)

	err := SaveProvidersToStore("codex", []Provider{
		{ID: 7, Name: "A", Enabled: true},
		{ID: 7, Name: "B", Enabled: true},
	})
	if err == nil {
		t.Fatal("重复 id 保存应返回错误")
	}
	if !strings.Contains(err.Error(), "重复") {
		t.Fatalf("错误文案应说明重复 id: %v", err)
	}
}

// TestMigrateSkipsCorruptedPlatformJSON 单平台旧 JSON 损坏：跳过该平台并保留原文件，其余平台正常迁移
func TestMigrateSkipsCorruptedPlatformJSON(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)

	corruptPath := filepath.Join(homeDir, ".code-switch", "claude-code.json")
	writeLegacyFixtureJSON(t, corruptPath, "not-a-json-envelope{")
	writeLegacyFixtureJSON(t,
		filepath.Join(homeDir, ".code-switch", "providers", "codex.json"),
		providerEnvelope{Providers: []Provider{{ID: 5, Name: "Codex OK", Enabled: true}}})

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateProviderJSONFilesToStore(db); err != nil {
		t.Fatalf("单平台损坏不应导致整体迁移失败: %v", err)
	}

	// 损坏平台：未迁移、原文件保留（未改名 .bak）
	claudeLoaded, err := LoadProvidersFromStore("claude")
	if err != nil || claudeLoaded != nil {
		t.Fatalf("损坏平台的存储应保持未初始化: %#v, %v", claudeLoaded, err)
	}
	if !providerConfigFileExists(corruptPath) || providerConfigFileExists(corruptPath+".bak") {
		t.Fatalf("损坏平台的原文件应保留且不生成 .bak: %s", corruptPath)
	}

	// 健康平台：正常迁移
	codexLoaded, err := LoadProvidersFromStore("codex")
	if err != nil || len(codexLoaded) != 1 || codexLoaded[0].Name != "Codex OK" {
		t.Fatalf("健康平台应正常迁移: %#v, %v", codexLoaded, err)
	}
}

// TestExecTxGroupRollsBackOnFailure 事务组任一语句失败时整体回滚，不留下半写入
func TestExecTxGroupRollsBackOnFailure(t *testing.T) {
	useIsolatedHomeDir(t)

	// 前置数据
	if err := SaveProvidersToStore("claude", []Provider{{ID: 1, Name: "Existing", Enabled: true}}); err != nil {
		t.Fatal(err)
	}

	// 第二条语句非法（表不存在）→ 整组回滚，DELETE 不得生效
	tasks := []WriteTask{
		{SQL: "DELETE FROM providers_store WHERE platform = ?", Args: []interface{}{"claude"}},
		{SQL: "INSERT INTO no_such_table VALUES (1)", Args: nil},
	}
	if err := GlobalDBQueue.ExecTxGroup(tasks); err == nil {
		t.Fatal("事务组含非法语句应返回错误")
	}

	loaded, err := LoadProvidersFromStore("claude")
	if err != nil || len(loaded) != 1 || loaded[0].Name != "Existing" {
		t.Fatalf("事务组失败后应整体回滚（数据不得被截断）: %#v, %v", loaded, err)
	}

	// 全组合法 → 原子生效
	tasks = []WriteTask{
		{SQL: "DELETE FROM providers_store WHERE platform = ?", Args: []interface{}{"claude"}},
		{SQL: "INSERT INTO providers_store (platform, id, name, payload, updated_at) VALUES (?, ?, ?, ?, ?)",
			Args: []interface{}{"claude", "42", "Tx OK", "{}", time.Now().Unix()}},
	}
	if err := GlobalDBQueue.ExecTxGroup(tasks); err != nil {
		t.Fatalf("合法事务组应成功: %v", err)
	}
	loaded, err = LoadProvidersFromStore("claude")
	if err != nil || len(loaded) != 1 || loaded[0].ID != 42 {
		t.Fatalf("事务组成功后应写入新行: %#v, %v", loaded, err)
	}
}

// TestAdditiveServicesSkipImportWhenStoreEmptySentinel 存储为「已初始化但空列表」（哨兵行）时，
// 四个 additive 服务构造不得再从 live 首次导入（用户主动清空后不复活）
func TestAdditiveServicesSkipImportWhenStoreEmptySentinel(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)

	// live 配置均有条目
	writePiTestConfig(t, homeDir, `{"providers": {"only": {"baseUrl": "https://only"}}}`)
	writeOpenClawTestConfig(t, homeDir, `{"models": {"providers": {"only": {"name": "Only", "baseUrl": "https://only"}}}}`)
	writeHermesTestConfig(t, homeDir, `custom_providers:
  - name: Only
    base_url: https://only
`)
	// OpenCode live 配置位于 ~/.config/opencode/opencode.json（provider 单数键）
	openCodeConfigPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(openCodeConfigPath), 0o755); err != nil {
		t.Fatalf("创建 OpenCode 配置目录失败: %v", err)
	}
	if err := os.WriteFile(openCodeConfigPath, []byte(`{"provider": {"only": {"name": "Only", "npm": "@ai-sdk/openai-compatible"}}}`), 0o644); err != nil {
		t.Fatalf("写入 OpenCode live 配置失败: %v", err)
	}

	// 统一存储写空列表（哨兵行 = 已初始化但为空）
	if err := SavePiProvidersToStore([]PiProvider{}); err != nil {
		t.Fatal(err)
	}
	if err := SaveOpenClawProvidersToStore([]OpenClawProvider{}); err != nil {
		t.Fatal(err)
	}
	if err := SaveHermesProvidersToStore([]HermesProvider{}); err != nil {
		t.Fatal(err)
	}
	if err := SaveOpenCodeProvidersToStore([]OpenCodeProvider{}); err != nil {
		t.Fatal(err)
	}

	if providers := NewPiService().GetProviders(); len(providers) != 0 {
		t.Fatalf("Pi 空哨兵不应触发 live 导入: %#v", providers)
	}
	if providers := NewOpenClawService().GetProviders(); len(providers) != 0 {
		t.Fatalf("OpenClaw 空哨兵不应触发 live 导入: %#v", providers)
	}
	if providers := NewHermesService().GetProviders(); len(providers) != 0 {
		t.Fatalf("Hermes 空哨兵不应触发 live 导入: %#v", providers)
	}
	if providers := NewOpenCodeService().GetProviders(); len(providers) != 0 {
		t.Fatalf("OpenCode 空哨兵不应触发 live 导入: %#v", providers)
	}
}

func TestProviderStoreNilVersusEmptySemantics(t *testing.T) {
	useIsolatedHomeDir(t)

	// 未初始化：返回 nil（前端默认预设初始化路径依赖）
	loaded, err := LoadProvidersFromStore("claude")
	if err != nil || loaded != nil {
		t.Fatalf("未初始化平台应返回 nil: %#v, %v", loaded, err)
	}

	// 保存空列表后：返回空切片（非 nil）
	if err := SaveProvidersToStore("claude", []Provider{}); err != nil {
		t.Fatalf("保存空列表失败: %v", err)
	}
	loaded, err = LoadProvidersFromStore("claude")
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || len(loaded) != 0 {
		t.Fatalf("已初始化空平台应返回空切片而非 nil: %#v", loaded)
	}

	// 有数据再清空：保持空切片语义
	if err := SaveProvidersToStore("claude", []Provider{{ID: 1, Name: "A", Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	if err := SaveProvidersToStore("claude", []Provider{}); err != nil {
		t.Fatal(err)
	}
	loaded, err = LoadProvidersFromStore("claude")
	if err != nil || loaded == nil || len(loaded) != 0 {
		t.Fatalf("清空后应保持空切片语义: %#v, %v", loaded, err)
	}
}

func TestProviderStoreRoundTripPreservesRepresentativeFields(t *testing.T) {
	useIsolatedHomeDir(t)

	original := []Provider{{
		ID:      77,
		Name:    "Round Trip",
		APIURL:  "https://rt.example.com",
		APIKey:  "rt-key",
		Enabled: true,
		Level:   3,
		Category: "third_party",
		APIFormat: "anthropic",
		SupportedModels:              map[string]bool{"claude-*": true},
		ModelMapping:                 map[string]string{"claude-*": "vendor/*"},
		ModelMappingDisabled:         map[string]bool{"claude-*": true},
		ModelMappingSupports1M:       map[string]bool{"claude-*": true},
		ModelPassthroughPatterns:     []string{"glm-*"},
		RequestBodyOverrides:         map[string]interface{}{"metadata": map[string]interface{}{"region": "a"}},
		CLIConfig:                    map[string]interface{}{"model": "gpt-5"},
	}}
	if err := SaveProvidersToStore("claude", original); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	loaded, err := LoadProvidersFromStore("claude")
	if err != nil || len(loaded) != 1 {
		t.Fatalf("回读失败: %#v, %v", loaded, err)
	}
	got := loaded[0]
	if got.ID != 77 || got.Name != "Round Trip" || got.Level != 3 || got.Category != "third_party" || got.APIFormat != "anthropic" {
		t.Fatalf("公共字段往返不一致: %#v", got)
	}
	if !got.SupportedModels["claude-*"] || got.ModelMapping["claude-*"] != "vendor/*" || !got.ModelMappingDisabled["claude-*"] || !got.ModelMappingSupports1M["claude-*"] {
		t.Fatalf("模型映射字段往返不一致: %#v", got)
	}
	if len(got.ModelPassthroughPatterns) != 1 || got.ModelPassthroughPatterns[0] != "glm-*" {
		t.Fatalf("透传模式往返不一致: %#v", got.ModelPassthroughPatterns)
	}
	if got.CLIConfig["model"] != "gpt-5" {
		t.Fatalf("CLIConfig 往返不一致: %#v", got.CLIConfig)
	}
	overrides, _ := got.RequestBodyOverrides["metadata"].(map[string]interface{})
	if overrides["region"] != "a" {
		t.Fatalf("RequestBodyOverrides 往返不一致: %#v", got.RequestBodyOverrides)
	}
}

func TestGeminiOpenCodeStoreRoundTrip(t *testing.T) {
	useIsolatedHomeDir(t)

	gemini := []GeminiProvider{{
		ID: "g-1", Name: "G", Enabled: true, Level: 2,
		EnvConfig:      map[string]string{"GEMINI_API_KEY": "k"},
		SettingsConfig: map[string]any{"security": map[string]any{"auth": "gemini-api-key"}},
	}}
	if err := SaveGeminiProvidersToStore(gemini); err != nil {
		t.Fatalf("保存 Gemini 失败: %v", err)
	}
	gLoaded, err := LoadGeminiProvidersFromStore()
	if err != nil || len(gLoaded) != 1 || gLoaded[0].Level != 2 || gLoaded[0].EnvConfig["GEMINI_API_KEY"] != "k" {
		t.Fatalf("Gemini 往返不一致: %#v, %v", gLoaded, err)
	}

	openCode := []OpenCodeProvider{{
		ID: "o-1", Name: "O", Enabled: true, NPM: "@ai-sdk/anthropic",
		SettingsConfig: map[string]any{"npm": "@ai-sdk/anthropic"},
	}}
	if err := SaveOpenCodeProvidersToStore(openCode); err != nil {
		t.Fatalf("保存 OpenCode 失败: %v", err)
	}
	oLoaded, err := LoadOpenCodeProvidersFromStore()
	if err != nil || len(oLoaded) != 1 || oLoaded[0].NPM != "@ai-sdk/anthropic" {
		t.Fatalf("OpenCode 往返不一致: %#v, %v", oLoaded, err)
	}
}
