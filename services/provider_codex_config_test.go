package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestProviderService_SaveCodexProvidersUsesNestedConfigAndPreservesCLIConfig(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	service := NewProviderService()

	providers := []Provider{
		{
			ID:      7,
			Name:    "Test Codex",
			APIURL:  "https://api.example.com/v1",
			APIKey:  "sk-test",
			Enabled: true,
			CLIConfig: map[string]interface{}{
				"model": "gpt-5-codex",
				"features": map[string]interface{}{
					"parallel": true,
				},
			},
		},
	}

	if err := service.SaveProviders("codex", providers); err != nil {
		t.Fatalf("保存 Codex providers 失败: %v", err)
	}

	newPath := filepath.Join(homeDir, ".code-switch", "providers", "codex.json")
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("期望新路径存在: %s, err=%v", newPath, err)
	}

	legacyPath := filepath.Join(homeDir, ".code-switch", "codex.json")
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("不应继续写入旧路径: %s, err=%v", legacyPath, err)
	}

	loaded, err := service.LoadProviders("codex")
	if err != nil {
		t.Fatalf("读取 Codex providers 失败: %v", err)
	}
	assertCodexOfficialProviderNotLoaded(t, loaded)
	loadedProvider := findLoadedProviderByName(t, loaded, "Test Codex")

	if got := loadedProvider.CLIConfig["model"]; got != "gpt-5-codex" {
		t.Fatalf("期望 cliConfig.model 为 gpt-5-codex，实际为 %#v", got)
	}

	features, ok := loadedProvider.CLIConfig["features"].(map[string]interface{})
	if !ok {
		t.Fatalf("期望 cliConfig.features 为 map，实际为 %#v", loadedProvider.CLIConfig["features"])
	}
	if got, ok := features["parallel"].(bool); !ok || !got {
		t.Fatalf("期望 cliConfig.features.parallel 为 true，实际为 %#v", features["parallel"])
	}
}

func TestProviderService_LoadCodexProvidersFallsBackToLegacyPath(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	legacyPath := filepath.Join(homeDir, ".code-switch", "codex.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("创建旧目录失败: %v", err)
	}

	payload, err := json.Marshal(providerEnvelope{
		Providers: []Provider{
			{
				ID:      9,
				Name:    "Legacy Codex",
				APIURL:  "https://legacy.example.com",
				APIKey:  "legacy-key",
				Enabled: true,
				CLIConfig: map[string]interface{}{
					"model": "legacy-model",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("序列化旧配置失败: %v", err)
	}
	if err := os.WriteFile(legacyPath, payload, 0o644); err != nil {
		t.Fatalf("写入旧配置失败: %v", err)
	}

	service := NewProviderService()
	loaded, err := service.LoadProviders("codex")
	if err != nil {
		t.Fatalf("读取旧路径 Codex providers 失败: %v", err)
	}
	assertCodexOfficialProviderNotLoaded(t, loaded)
	_ = findLoadedProviderByName(t, loaded, "Legacy Codex")

	snapshot, err := loadProviderSnapshot("codex")
	if err != nil {
		t.Fatalf("读取 provider snapshot 失败: %v", err)
	}
	if len(snapshot) != 1 || snapshot[0].Name != "Legacy Codex" {
		t.Fatalf("snapshot 未按旧路径回退读取: %#v", snapshot)
	}
}

func TestProviderService_LoadProvidersSupportsTotalQuotaFieldAfterInitDatabase(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	configPath := filepath.Join(homeDir, ".code-switch", "providers", "codex.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 provider 目录失败: %v", err)
	}

	payload, err := json.Marshal(providerEnvelope{
		Providers: []Provider{
			{
				ID:      11,
				Name:    "Quota Total Codex",
				APIURL:  "https://quota.example.com",
				APIKey:  "quota-key",
				Enabled: true,
				BudgetQuotaSettings: &BudgetQuotaSettings{
					Total: BudgetQuotaSetting{
						Total:           512,
						RefreshTime:     "00:00",
						RefreshDay:      1,
						RefreshMonthDay: 1,
					},
				},
				BudgetQuotaUsedAdjustments: &BudgetQuotaAdjustments{
					Total: 12.34,
				},
				ProviderQuotaQueryType: string(ProviderQuotaQueryTypeTokenPlanGLM),
				ProviderQuotaQueryConfig: &ProviderQuotaQueryConfig{
					Enabled:           true,
					TemplateType:      string(ProviderQuotaTemplateTypeTokenPlan),
					TokenPlanProvider: "glm",
					Timeout:           10,
					AutoQueryInterval: 5,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("序列化 provider 配置失败: %v", err)
	}
	if err := os.WriteFile(configPath, payload, 0o644); err != nil {
		t.Fatalf("写入 provider 配置失败: %v", err)
	}

	service := NewProviderService()
	loaded, err := service.LoadProviders("codex")
	if err != nil {
		t.Fatalf("读取 provider 配置失败: %v", err)
	}
	assertCodexOfficialProviderNotLoaded(t, loaded)
	loadedProvider := findLoadedProviderByName(t, loaded, "Quota Total Codex")
	if loadedProvider.BudgetQuotaSettings == nil || loadedProvider.BudgetQuotaSettings.Total.Total != 512 {
		t.Fatalf("total quota 反序列化失败: %+v", loadedProvider.BudgetQuotaSettings)
	}
	if loadedProvider.BudgetQuotaUsedAdjustments == nil || loadedProvider.BudgetQuotaUsedAdjustments.Total != 12.34 {
		t.Fatalf("total quota adjustment 反序列化失败: %+v", loadedProvider.BudgetQuotaUsedAdjustments)
	}
	if loadedProvider.ProviderQuotaQueryType != string(ProviderQuotaQueryTypeTokenPlanGLM) {
		t.Fatalf("providerQuotaQueryType 反序列化失败: %q", loadedProvider.ProviderQuotaQueryType)
	}
	if loadedProvider.ProviderQuotaQueryConfig == nil || loadedProvider.ProviderQuotaQueryConfig.TokenPlanProvider != "glm" {
		t.Fatalf("providerQuotaQueryConfig 反序列化失败: %+v", loadedProvider.ProviderQuotaQueryConfig)
	}
}

func TestProviderServiceLoadCodexMissingConfigReturnsNilForDefaultPresets(t *testing.T) {
	_ = useIsolatedHomeDir(t)
	loaded, err := NewProviderService().LoadProviders("codex")
	if err != nil {
		t.Fatalf("读取缺失 Codex provider 配置失败: %v", err)
	}
	if loaded != nil {
		t.Fatalf("缺失配置时应保留前端默认预设初始化路径，实际为 %#v", loaded)
	}
}

func TestProviderServiceCustomOfficialCategoryDoesNotGetFiltered(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	configPath := filepath.Join(homeDir, ".code-switch", "providers", "codex.json")
	payload, err := json.Marshal(providerEnvelope{Providers: []Provider{{
		ID:       201,
		Name:     "Custom Official Channel",
		APIURL:   "https://official-channel.example.com/v1",
		APIKey:   "sk-official-channel",
		Enabled:  true,
		Category: "official",
	}}})
	if err != nil {
		t.Fatalf("序列化 provider 配置失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 provider 目录失败: %v", err)
	}
	if err := os.WriteFile(configPath, payload, 0o644); err != nil {
		t.Fatalf("写入 provider 配置失败: %v", err)
	}

	loaded, err := NewProviderService().LoadProviders("codex")
	if err != nil {
		t.Fatalf("读取 Codex provider 失败: %v", err)
	}
	assertCodexOfficialProviderNotLoaded(t, loaded)
	_ = findLoadedProviderByName(t, loaded, "Custom Official Channel")

	runtimeProviders := filterRuntimeProviders("codex", loaded)
	if findProviderByName(runtimeProviders, "Codex 官方登录") != nil {
		t.Fatalf("运行时应过滤内置官方卡片，实际为 %#v", runtimeProviders)
	}
	if findProviderByName(runtimeProviders, "Custom Official Channel") == nil {
		t.Fatalf("运行时不应按 category 误过滤自定义 official provider，实际为 %#v", runtimeProviders)
	}

	availabilityProviders := filterAvailabilityProviders("codex", loaded)
	if findProviderByName(availabilityProviders, "Codex 官方登录") != nil {
		t.Fatalf("可用性监控应过滤内置官方卡片，实际为 %#v", availabilityProviders)
	}
	if findProviderByName(availabilityProviders, "Custom Official Channel") == nil {
		t.Fatalf("可用性监控不应按 category 误过滤自定义 official provider，实际为 %#v", availabilityProviders)
	}

	service := NewCodexSettingsService(":18100")
	if err := service.ApplySingleProvider(201); err != nil {
		t.Fatalf("自定义 official category provider 应按普通第三方直连应用: %v", err)
	}
	config := readCodexConfigMapForTest(t, homeDir)
	providerConfig := readCodexProviderConfigForTest(t, config, "custom-official-channel")
	if got := providerConfig["base_url"]; got != "https://official-channel.example.com/v1" {
		t.Fatalf("自定义 official category provider 不应走内置官方分支，实际为 %#v", providerConfig)
	}
}

func TestCodexSettingsServiceApplyOfficialProviderIsRemoved(t *testing.T) {
	_ = useIsolatedHomeDir(t)
	service := NewCodexSettingsService(":18100")
	if err := service.ApplySingleProvider(200); err == nil {
		t.Fatalf("旧 Codex 官方登录卡片已移除，不应允许直连应用")
	}
}

func TestCodexSettingsServiceApplyRemovedOfficialProviderDoesNotClearAuth(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeCodexConfigForTest(t, homeDir, `
model_provider = 'openai'
preferred_auth_method = 'apikey'

[model_providers.openai]
base_url = 'https://stale-third.example.com/v1'
name = 'openai'
requires_openai_auth = false
wire_api = 'responses'
`)

	service := NewCodexSettingsService(":18100")
	if err := service.ApplySingleProvider(200); err == nil {
		t.Fatalf("旧 Codex 官方登录卡片已移除，不应清理现有配置")
	}
	config := readCodexConfigMapForTest(t, homeDir)
	if got := config["model_provider"]; got != "openai" {
		t.Fatalf("失败的旧官方应用不应修改配置，实际为 %#v", config)
	}
}

func TestCodexSettingsServicePreserveOfficialAuthClearsDirectAuthKey(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	provider := Provider{
		ID:      7,
		Name:    "Third Party",
		APIURL:  "https://third.example.com/v1",
		APIKey:  "sk-third",
		Enabled: true,
	}
	if err := NewProviderService().SaveProviders("codex", []Provider{provider}); err != nil {
		t.Fatalf("保存 Codex provider 失败: %v", err)
	}
	writeCodexAppSettingsForTest(t, homeDir, AppSettings{PreserveCodexOfficialAuth: true})
	if err := os.MkdirAll(filepath.Join(homeDir, ".codex"), 0o755); err != nil {
		t.Fatalf("创建 Codex 目录失败: %v", err)
	}
	authPath := filepath.Join(homeDir, ".codex", "auth.json")
	if err := os.WriteFile(authPath, []byte(`{"OPENAI_API_KEY":"sk-old","tokens":{"access":"official"}}`), 0o644); err != nil {
		t.Fatalf("写入 Codex auth 失败: %v", err)
	}

	service := NewCodexSettingsService(":18100")
	if err := service.ApplySingleProvider(int(provider.ID)); err != nil {
		t.Fatalf("保留官方登录时应用第三方 provider 失败: %v", err)
	}
	authPayload := readCodexAuthForTest(t, homeDir)
	if got := authPayload[codexEnvKey]; got != nil {
		t.Fatalf("保留官方登录时应清理旧 OPENAI_API_KEY，实际为 %#v", authPayload)
	}
	if authPayload["tokens"] == nil {
		t.Fatalf("保留官方登录时不应破坏其他认证字段，实际为 %#v", authPayload)
	}
	config := readCodexConfigMapForTest(t, homeDir)
	providerConfig := readCodexProviderConfigForTest(t, config, "third-party")
	if got := providerConfig["experimental_bearer_token"]; got != "sk-third" {
		t.Fatalf("保留官方登录时应把第三方 key 写入 provider token，实际为 %#v", providerConfig)
	}
}

func TestAppSettingsServiceRollsBackWhenCodexRewriteFails(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeCodexAppSettingsForTest(t, homeDir, AppSettings{PreserveCodexOfficialAuth: false})
	writeCodexConfigForTest(t, homeDir, "invalid = [")

	service := NewAppSettingsService(nil)
	service.BindCodexSettingsService(NewCodexSettingsService(":18100"))
	if _, err := service.SaveAppSettings(AppSettings{PreserveCodexOfficialAuth: true}); err == nil {
		t.Fatal("Codex 配置重写失败时应返回错误")
	}

	settings := readCodexAppSettingsForTest(t, homeDir)
	if settings.PreserveCodexOfficialAuth {
		t.Fatalf("Codex 配置重写失败时应回滚 app settings，实际为 %+v", settings)
	}
}

func TestCodexSettingsServiceReapplyUnifiedThirdPartyPreservesBaseURL(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	provider := Provider{
		ID:      7,
		Name:    "Third Party",
		APIURL:  "https://third.example.com/v1",
		APIKey:  "sk-third",
		Enabled: true,
	}
	if err := NewProviderService().SaveProviders("codex", []Provider{provider}); err != nil {
		t.Fatalf("保存 Codex provider 失败: %v", err)
	}

	writeCodexAppSettingsForTest(t, homeDir, AppSettings{
		PreserveCodexOfficialAuth: true,
		UnifyCodexSessionHistory:  true,
	})
	service := NewCodexSettingsService(":18100")
	if err := service.ApplySingleProvider(int(provider.ID)); err != nil {
		t.Fatalf("应用第三方 provider 失败: %v", err)
	}

	writeCodexAppSettingsForTest(t, homeDir, AppSettings{
		PreserveCodexOfficialAuth: false,
		UnifyCodexSessionHistory:  true,
	})
	if err := service.ReapplyCurrentConfigForSettings(AppSettings{
		PreserveCodexOfficialAuth: false,
		UnifyCodexSessionHistory:  true,
	}); err != nil {
		t.Fatalf("重新应用 Codex 设置失败: %v", err)
	}

	config := readCodexConfigMapForTest(t, homeDir)
	providerConfig := readCodexProviderConfigForTest(t, config, codexProviderKey)
	if got := providerConfig["base_url"]; got != "https://third.example.com/v1" {
		t.Fatalf("统一历史下重新应用不应丢失第三方 base_url，实际为 %#v", providerConfig)
	}
	if got := providerConfig["name"]; got == codexOfficialName {
		t.Fatalf("第三方直连不应被重写为官方 provider，实际为 %#v", providerConfig)
	}
	authContent, err := os.ReadFile(filepath.Join(homeDir, ".codex", "auth.json"))
	if err != nil {
		t.Fatalf("期望 auth.json 写入第三方 key: %v", err)
	}
	if !strings.Contains(string(authContent), "sk-third") {
		t.Fatalf("期望 auth.json 保存第三方 key，实际内容:\n%s", string(authContent))
	}
}

func TestCodexSettingsServiceReapplyUnifiedUnknownThirdPartyDoesNotRewriteOfficial(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeCodexConfigForTest(t, homeDir, `
model_provider = 'code-switch-r'
preferred_auth_method = 'apikey'

[model_providers.code-switch-r]
base_url = 'https://unknown-third.example.com/v1'
name = 'code-switch-r'
requires_openai_auth = false
wire_api = 'responses'
`)

	service := NewCodexSettingsService(":18100")
	if err := service.ReapplyCurrentConfigForSettings(AppSettings{UnifyCodexSessionHistory: true}); err != nil {
		t.Fatalf("重新应用 Codex 设置失败: %v", err)
	}

	config := readCodexConfigMapForTest(t, homeDir)
	providerConfig := readCodexProviderConfigForTest(t, config, codexProviderKey)
	if got := providerConfig["base_url"]; got != "https://unknown-third.example.com/v1" {
		t.Fatalf("未匹配到本地 provider 时不应改写第三方 base_url，实际为 %#v", providerConfig)
	}
	if got := providerConfig["name"]; got == codexOfficialName {
		t.Fatalf("未匹配到本地 provider 时不应改写为官方 provider，实际为 %#v", providerConfig)
	}
}

func TestCodexSettingsServiceReapplyUnknownOpenAIProviderDoesNotRewriteOfficial(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeCodexConfigForTest(t, homeDir, `
model_provider = 'openai'
preferred_auth_method = 'apikey'

[model_providers.openai]
base_url = 'https://custom-openai.example.com/v1'
name = 'openai'
requires_openai_auth = false
wire_api = 'responses'
`)

	service := NewCodexSettingsService(":18100")
	if err := service.ReapplyCurrentConfigForSettings(AppSettings{UnifyCodexSessionHistory: true}); err != nil {
		t.Fatalf("重新应用 Codex 设置失败: %v", err)
	}

	config := readCodexConfigMapForTest(t, homeDir)
	if got := config["model_provider"]; got != codexOfficialProvider {
		t.Fatalf("未知 openai provider 不应被改写到共享官方桶，实际为 %#v", config)
	}
	providerConfig := readCodexProviderConfigForTest(t, config, codexOfficialProvider)
	if got := providerConfig["base_url"]; got != "https://custom-openai.example.com/v1" {
		t.Fatalf("未知 openai provider 不应丢失 base_url，实际为 %#v", providerConfig)
	}
}

func TestCodexHistoryMigrationMigratesOfficialSessionsOnly(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeCodexConfigForTest(t, homeDir, `
model_provider = 'code-switch-r'

[model_providers.code-switch-r]
name = 'OpenAI'
requires_openai_auth = true
supports_websockets = true
wire_api = 'responses'
`)
	sessionPath := filepath.Join(homeDir, ".codex", "sessions", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("创建 Codex sessions 目录失败: %v", err)
	}
	lines := []string{
		`{"type":"session_meta","id":"official-session","model_provider":"openai"}`,
		`{"type":"session_meta","id":"third-session","model_provider":"third-party"}`,
	}
	if err := os.WriteFile(sessionPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("写入 Codex history 失败: %v", err)
	}

	result, err := MigrateCodexHistoryToUnifiedBucket(true)
	if err != nil {
		t.Fatalf("迁移 Codex history 失败: %v", err)
	}
	if result.MigratedJSONLFiles != 1 {
		t.Fatalf("期望迁移 1 个 JSONL 文件，实际为 %+v", result)
	}
	records := readJSONLRecordsForTest(t, sessionPath)
	if got := records["official-session"]["model_provider"]; got != codexProviderKey {
		t.Fatalf("官方历史应迁入共享 provider，实际为 %#v", records["official-session"])
	}
	if got := records["third-session"]["model_provider"]; got != "third-party" {
		t.Fatalf("第三方历史不应在无原 provider ledger 时迁移，实际为 %#v", records["third-session"])
	}
}

func TestProviderServiceLoadCodexDoesNotInjectRemovedBuiltInCard(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	configPath := filepath.Join(homeDir, ".code-switch", "providers", "codex.json")
	payload, err := json.Marshal(providerEnvelope{Providers: []Provider{{
		ID:      9,
		Name:    "Only Disk Provider",
		APIURL:  "https://disk.example.com",
		APIKey:  "sk-disk",
		Enabled: true,
	}}})
	if err != nil {
		t.Fatalf("序列化 provider 配置失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 provider 目录失败: %v", err)
	}
	if err := os.WriteFile(configPath, payload, 0o644); err != nil {
		t.Fatalf("写入 provider 配置失败: %v", err)
	}

	if _, err := NewProviderService().LoadProviders("codex"); err != nil {
		t.Fatalf("读取 Codex provider 失败: %v", err)
	}

	current, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("读取 provider 配置失败: %v", err)
	}
	if string(current) != string(payload) {
		t.Fatalf("加载 Codex provider 不应重写磁盘配置\n原始: %s\n当前: %s", string(payload), string(current))
	}
}

func writeCodexAppSettingsForTest(t *testing.T, homeDir string, settings AppSettings) {
	t.Helper()
	settingsPath := filepath.Join(homeDir, ".code-switch", "app.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("创建 app settings 目录失败: %v", err)
	}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("序列化 app settings 失败: %v", err)
	}
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatalf("写入 app settings 失败: %v", err)
	}
}

func readCodexAppSettingsForTest(t *testing.T, homeDir string) AppSettings {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(homeDir, ".code-switch", "app.json"))
	if err != nil {
		t.Fatalf("读取 app settings 失败: %v", err)
	}
	var settings AppSettings
	if err := json.Unmarshal(content, &settings); err != nil {
		t.Fatalf("解析 app settings 失败: %v\n%s", err, string(content))
	}
	return settings
}

func writeCodexConfigForTest(t *testing.T, homeDir string, content string) {
	t.Helper()
	configPath := filepath.Join(homeDir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 Codex config 目录失败: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("写入 Codex config 失败: %v", err)
	}
}

func readCodexConfigForTest(t *testing.T, homeDir string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(homeDir, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("读取 Codex config 失败: %v", err)
	}
	return string(content)
}

func readCodexConfigMapForTest(t *testing.T, homeDir string) map[string]any {
	t.Helper()
	content := readCodexConfigForTest(t, homeDir)
	var config map[string]any
	if err := toml.Unmarshal([]byte(content), &config); err != nil {
		t.Fatalf("解析 Codex config 失败: %v\n%s", err, content)
	}
	return config
}

func readCodexProviderConfigForTest(t *testing.T, config map[string]any, key string) map[string]any {
	t.Helper()
	modelProviders, ok := config["model_providers"].(map[string]any)
	if !ok {
		t.Fatalf("期望 model_providers 为 map，实际为 %#v", config["model_providers"])
	}
	provider, ok := modelProviders[key].(map[string]any)
	if !ok {
		t.Fatalf("期望 model_providers.%s 为 map，实际为 %#v", key, modelProviders[key])
	}
	return provider
}

func readCodexAuthForTest(t *testing.T, homeDir string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(homeDir, ".codex", "auth.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}
		}
		t.Fatalf("读取 Codex auth 失败: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("解析 Codex auth 失败: %v\n%s", err, string(content))
	}
	if payload == nil {
		return map[string]any{}
	}
	return payload
}

func assertCodexOfficialProviderNotLoaded(t *testing.T, providers []Provider) {
	t.Helper()
	for _, provider := range providers {
		if isCodexOfficialProviderCard(provider) {
			t.Fatalf("加载结果不应包含已移除的 Codex 官方登录卡片: %#v", providers)
		}
	}
}

func findLoadedProviderByName(t *testing.T, providers []Provider, name string) Provider {
	t.Helper()
	for _, provider := range providers {
		if provider.Name == name {
			return provider
		}
	}
	t.Fatalf("未找到 provider %q: %#v", name, providers)
	return Provider{}
}

func findProviderByName(providers []Provider, name string) *Provider {
	for i := range providers {
		if providers[i].Name == name {
			return &providers[i]
		}
	}
	return nil
}

func readJSONLRecordsForTest(t *testing.T, path string) map[string]map[string]any {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 JSONL 失败: %v", err)
	}
	records := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("解析 JSONL 行失败: %v\n%s", err, line)
		}
		records[anyToString(payload["id"])] = payload
	}
	return records
}

func TestProviderServiceEnsureCodexOAuthProviderCreatesManagedProvider(t *testing.T) {
	_ = useIsolatedHomeDir(t)
	service := NewProviderService()
	provider, err := service.EnsureCodexOAuthProvider("acct_123", "user@example.com")
	if err != nil {
		t.Fatalf("创建 Codex OAuth provider 失败: %v", err)
	}
	if provider.APIKey != "" {
		t.Fatalf("Codex OAuth provider 不应持久化 API Key: %#v", provider)
	}
	if provider.APIURL != codexOAuthBackendAPIBaseURL || provider.AuthProvider != CodexOAuthProviderName || provider.AuthAccountID != "acct_123" {
		t.Fatalf("Codex OAuth provider 字段不正确: %#v", provider)
	}
	if provider.Enabled {
		t.Fatalf("EnsureCodexOAuthProvider 不应自动启用非默认账号 provider: %#v", provider)
	}

	loaded, err := service.LoadProviders("codex")
	if err != nil {
		t.Fatalf("读取 Codex providers 失败: %v", err)
	}
	assertCodexOfficialProviderNotLoaded(t, loaded)
	if len(filterRuntimeProviders("codex", loaded)) != 1 {
		t.Fatalf("Codex OAuth provider 应参与运行时候选: %#v", loaded)
	}
	if !providerHasRelayAuth("codex", loaded[0]) {
		t.Fatalf("Codex OAuth provider 无 API Key 时仍应具备 relay 认证能力: %#v", loaded[0])
	}
	if providerHasRelayAuth("custom:tool", loaded[0]) {
		t.Fatalf("Codex OAuth 托管认证不应泄漏到自定义 CLI: %#v", loaded[0])
	}
}

func TestProviderServiceSelectCodexOAuthProviderDisablesOtherOAuthProviders(t *testing.T) {
	_ = useIsolatedHomeDir(t)
	service := NewProviderService()
	if _, err := service.EnsureCodexOAuthProvider("acct_old", "old@example.com"); err != nil {
		t.Fatalf("创建旧账号 provider 失败: %v", err)
	}
	selected, err := service.SelectCodexOAuthProvider("acct_new", "new@example.com")
	if err != nil {
		t.Fatalf("选择默认 Codex OAuth provider 失败: %v", err)
	}
	if !selected.Enabled || selected.AuthAccountID != "acct_new" {
		t.Fatalf("默认账号 provider 应启用并绑定新账号: %#v", selected)
	}

	loaded, err := service.LoadProviders("codex")
	if err != nil {
		t.Fatalf("读取 Codex providers 失败: %v", err)
	}
	for _, provider := range loaded {
		if !isCodexOAuthProvider(provider) {
			continue
		}
		if provider.AuthAccountID == "acct_new" && !provider.Enabled {
			t.Fatalf("新默认账号 provider 应保持启用: %#v", provider)
		}
		if provider.AuthAccountID == "acct_old" && provider.Enabled {
			t.Fatalf("旧 OAuth provider 应被禁用: %#v", provider)
		}
	}
}
