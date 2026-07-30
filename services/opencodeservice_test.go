package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeReadsLiveConfigWithCommentsAndTrailingCommas(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 OpenCode 配置目录失败: %v", err)
	}
	data := []byte(`{
  // OpenCode allows comments in user config
  "provider": {
    "deepseek": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "DeepSeek",
      "options": {
        "baseURL": "https://api.deepseek.com/v1",
        "apiKey": "sk-live",
      },
      "models": {
        "deepseek-chat": { "name": "DeepSeek Chat" },
      },
    },
  },
}`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("写入 OpenCode live 配置失败: %v", err)
	}

	providers, err := readOpenCodeLiveProviders()
	if err != nil {
		t.Fatalf("读取 OpenCode live 配置失败: %v", err)
	}
	provider := providers["deepseek"]
	if provider == nil {
		t.Fatalf("期望读取到 deepseek provider，实际为 %#v", providers)
	}
	if got := extractOpenCodeBaseURL(provider); got != "https://api.deepseek.com/v1" {
		t.Fatalf("baseURL 不正确: %q", got)
	}
	if got := extractOpenCodeAPIKey(provider); got != "sk-live" {
		t.Fatalf("apiKey 不正确: %q", got)
	}
}

func TestOpenCodeAddProviderRejectsExistingLiveProvider(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 OpenCode 配置目录失败: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{"provider":{"deepseek":{"npm":"@ai-sdk/openai-compatible"}}}`), 0o644); err != nil {
		t.Fatalf("写入 OpenCode live 配置失败: %v", err)
	}

	service := &OpenCodeService{}
	err := service.AddProvider(OpenCodeProvider{
		ID:      "deepseek",
		Name:    "Managed DeepSeek",
		Enabled: true,
	})
	if err == nil {
		t.Fatal("期望新增已存在 live provider 时返回冲突错误")
	}
	if !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("冲突错误文案不正确: %v", err)
	}
}

func TestOpenCodeGetLiveProviderIdsReturnsSortedIDs(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 OpenCode 配置目录失败: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{"provider":{"zeta":{"npm":"@ai-sdk/openai-compatible"},"alpha":{"npm":"@ai-sdk/openai-compatible"}}}`), 0o644); err != nil {
		t.Fatalf("写入 OpenCode live 配置失败: %v", err)
	}

	service := &OpenCodeService{}
	ids, err := service.GetLiveProviderIds()
	if err != nil {
		t.Fatalf("读取 OpenCode live provider ids 失败: %v", err)
	}
	if got, want := strings.Join(ids, ","), "alpha,zeta"; got != want {
		t.Fatalf("live provider ids = %q, want %q", got, want)
	}
}

func TestOpenCodeSettingsConfigClearsManagedOptions(t *testing.T) {
	provider := normalizeOpenCodeProvider(OpenCodeProvider{
		ID:      "custom",
		Name:    "Custom",
		Enabled: true,
		SettingsConfig: map[string]any{
			"npm": "@ai-sdk/openai-compatible",
			"options": map[string]any{
				"baseURL": "https://old.example.com/v1",
				"apiKey":  "old-key",
			},
			"models": map[string]any{
				"gpt-4o": map[string]any{"name": "GPT-4o"},
			},
		},
	})

	if options := mapFromAny(provider.SettingsConfig["options"]); len(options) != 0 {
		t.Fatalf("期望清空 baseURL/apiKey 后删除 options，实际为 %#v", options)
	}
}

func TestOpenCodeImportFromLivePreservesRawProviderKey(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 OpenCode 配置目录失败: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{"provider":{"OpenAI.Custom":{"npm":"@ai-sdk/openai-compatible","models":{"gpt-4o":{"name":"GPT-4o"}}}}}`), 0o644); err != nil {
		t.Fatalf("写入 OpenCode live 配置失败: %v", err)
	}

	service := &OpenCodeService{}
	imported, err := service.ImportFromLive()
	if err != nil {
		t.Fatalf("导入 OpenCode live provider 失败: %v", err)
	}
	if imported != 1 {
		t.Fatalf("导入数量 = %d, want 1", imported)
	}
	providers := service.GetProviders()
	if len(providers) != 1 || providers[0].ID != "OpenAI.Custom" {
		t.Fatalf("期望保留 raw provider key，实际 providers=%#v", providers)
	}

	providers[0].Name = "Updated"
	if err := service.UpdateProvider(providers[0]); err != nil {
		t.Fatalf("更新导入 provider 失败: %v", err)
	}
	liveProviders, err := readOpenCodeLiveProviders()
	if err != nil {
		t.Fatalf("读取 OpenCode live provider 失败: %v", err)
	}
	if liveProviders["OpenAI.Custom"] == nil {
		t.Fatalf("期望 raw key 存在，实际 live providers=%#v", liveProviders)
	}
	if liveProviders["openai-custom"] != nil {
		t.Fatalf("不应写入 normalized key，实际 live providers=%#v", liveProviders)
	}
}

func TestOpenCodeLiveWriteFailureDoesNotDirtyManagedProviders(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	providersDir := filepath.Join(homeDir, ".code-switch", "providers")
	if err := os.MkdirAll(providersDir, 0o755); err != nil {
		t.Fatalf("创建 managed providers 目录失败: %v", err)
	}
	managedPath := filepath.Join(providersDir, "opencode.json")
	originalManaged := []byte(`{"providers":[{"id":"stable","name":"Stable","enabled":true,"settingsConfig":{"npm":"@ai-sdk/openai-compatible","models":{"gpt-4o":{"name":"GPT-4o"}}}}]}`)
	if err := os.WriteFile(managedPath, originalManaged, 0o644); err != nil {
		t.Fatalf("写入 managed providers 失败: %v", err)
	}
	configDir := filepath.Join(homeDir, ".config", "opencode")
	if err := os.MkdirAll(filepath.Dir(configDir), 0o755); err != nil {
		t.Fatalf("创建 .config 目录失败: %v", err)
	}
	if err := os.WriteFile(configDir, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("阻塞 OpenCode config dir 失败: %v", err)
	}

	service := &OpenCodeService{}
	err := service.AddProvider(OpenCodeProvider{ID: "new-provider", Name: "New", Enabled: true})
	if err == nil {
		t.Fatal("期望 live 写失败时 AddProvider 返回错误")
	}
	managedData, readErr := os.ReadFile(managedPath)
	if readErr != nil {
		t.Fatalf("读取 managed providers 失败: %v", readErr)
	}
	if string(managedData) != string(originalManaged) {
		t.Fatalf("live 写失败不应污染 managed providers，实际=%s", managedData)
	}
}

func TestOpenCodeManagedSaveFailureRollsBackLiveConfig(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 OpenCode 配置目录失败: %v", err)
	}
	originalLive := []byte(`{"provider":{"stable":{"npm":"@ai-sdk/openai-compatible","models":{"gpt-4o":{"name":"GPT-4o"}}}}}`)
	if err := os.WriteFile(configPath, originalLive, 0o644); err != nil {
		t.Fatalf("写入 OpenCode live 配置失败: %v", err)
	}
	providersDir := filepath.Join(homeDir, ".code-switch", "providers")
	if err := os.MkdirAll(filepath.Dir(providersDir), 0o755); err != nil {
		t.Fatalf("创建 .code-switch 目录失败: %v", err)
	}
	if err := os.WriteFile(providersDir, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("阻塞 managed providers dir 失败: %v", err)
	}

	service := &OpenCodeService{}
	err := service.AddProvider(OpenCodeProvider{ID: "new-provider", Name: "New", Enabled: true})
	if err == nil {
		t.Fatal("期望 managed providers 保存失败时 AddProvider 返回错误")
	}
	liveProviders, readErr := readOpenCodeLiveProviders()
	if readErr != nil {
		t.Fatalf("读取 OpenCode live 配置失败: %v", readErr)
	}
	if liveProviders["stable"] == nil || liveProviders["new-provider"] != nil || len(liveProviders) != 1 {
		t.Fatalf("managed 保存失败应回滚 live config，实际=%#v", liveProviders)
	}
}

func TestOpenCodeSaveProvidersRejectsLiveConflictWithoutDirtyingManagedProviders(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 OpenCode 配置目录失败: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{"provider":{"user-live":{"npm":"@ai-sdk/openai-compatible"}}}`), 0o644); err != nil {
		t.Fatalf("写入 OpenCode live 配置失败: %v", err)
	}
	providersDir := filepath.Join(homeDir, ".code-switch", "providers")
	if err := os.MkdirAll(providersDir, 0o755); err != nil {
		t.Fatalf("创建 managed providers 目录失败: %v", err)
	}
	managedPath := filepath.Join(providersDir, "opencode.json")
	originalManaged := []byte(`{"providers":[{"id":"stable","name":"Stable","enabled":false}]}`)
	if err := os.WriteFile(managedPath, originalManaged, 0o644); err != nil {
		t.Fatalf("写入 managed providers 失败: %v", err)
	}

	service := &OpenCodeService{}
	if err := service.loadProviders(); err != nil {
		t.Fatalf("加载 managed providers 失败: %v", err)
	}
	err := service.SaveProviders([]OpenCodeProvider{
		{ID: "stable", Name: "Stable", Enabled: false},
		{ID: "user-live", Name: "User Live", Enabled: true},
	})
	if err == nil {
		t.Fatal("期望批量保存命中 unmanaged live provider 时返回错误")
	}
	managedData, readErr := os.ReadFile(managedPath)
	if readErr != nil {
		t.Fatalf("读取 managed providers 失败: %v", readErr)
	}
	if string(managedData) != string(originalManaged) {
		t.Fatalf("批量保存失败不应污染 managed providers，实际=%s", managedData)
	}
}

func TestOpenCodeSaveProvidersHonorsLiveConfigManagedFlag(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 OpenCode 配置目录失败: %v", err)
	}
	originalLive := []byte(`{"provider":{"user-live":{"npm":"@ai-sdk/openai-compatible"}}}`)
	if err := os.WriteFile(configPath, originalLive, 0o644); err != nil {
		t.Fatalf("写入 OpenCode live 配置失败: %v", err)
	}

	service := &OpenCodeService{}
	if err := service.SaveProviders([]OpenCodeProvider{
		{ID: "db-only", Name: "DB Only", Enabled: true, LiveConfigManaged: boolPtr(false), IsInConfig: boolPtr(false)},
	}); err != nil {
		t.Fatalf("保存 DB-only OpenCode provider 失败: %v", err)
	}
	liveData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("读取 OpenCode live 配置失败: %v", err)
	}
	if string(liveData) != string(originalLive) {
		t.Fatalf("DB-only provider 不应重写 live config，实际=%s", liveData)
	}
}

func TestOpenCodeSaveProvidersRemovesPreviouslyManagedProviderFromLive(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 OpenCode 配置目录失败: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{"provider":{"managed":{"npm":"@ai-sdk/openai-compatible"},"user-live":{"npm":"@ai-sdk/openai-compatible"}}}`), 0o644); err != nil {
		t.Fatalf("写入 OpenCode live 配置失败: %v", err)
	}
	service := &OpenCodeService{
		providers: []OpenCodeProvider{
			{ID: "managed", Name: "Managed", Enabled: true, LiveConfigManaged: boolPtr(true), IsInConfig: boolPtr(true), SettingsConfig: map[string]any{"npm": "@ai-sdk/openai-compatible"}},
		},
	}

	if err := service.SaveProviders([]OpenCodeProvider{
		{ID: "managed", Name: "Managed", Enabled: false, LiveConfigManaged: boolPtr(false), IsInConfig: boolPtr(false), SettingsConfig: map[string]any{"npm": "@ai-sdk/openai-compatible"}},
	}); err != nil {
		t.Fatalf("保存移出 live config 的 provider 失败: %v", err)
	}
	liveProviders, err := readOpenCodeLiveProviders()
	if err != nil {
		t.Fatalf("读取 OpenCode live provider 失败: %v", err)
	}
	if liveProviders["managed"] != nil || liveProviders["user-live"] == nil {
		t.Fatalf("应只移除本应用 managed provider，实际=%#v", liveProviders)
	}
}

func TestOpenCodeDisabledProviderSaveDoesNotRewriteLiveConfig(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 OpenCode 配置目录失败: %v", err)
	}
	originalLive := []byte(`{
  // keep my hand-written config as-is
  "provider": {
    "user.provider": {
      "npm": "@ai-sdk/openai-compatible",
    },
  },
}`)
	if err := os.WriteFile(configPath, originalLive, 0o644); err != nil {
		t.Fatalf("写入 OpenCode live 配置失败: %v", err)
	}

	service := &OpenCodeService{}
	if err := service.AddProvider(OpenCodeProvider{ID: "disabled-provider", Name: "Disabled", Enabled: false}); err != nil {
		t.Fatalf("新增禁用 OpenCode provider 失败: %v", err)
	}
	if _, err := service.DuplicateProvider("disabled-provider"); err != nil {
		t.Fatalf("复制禁用 OpenCode provider 失败: %v", err)
	}
	if err := service.ReorderProviders([]string{"disabled-provider-copy-missing", "disabled-provider"}); err != nil {
		t.Fatalf("排序禁用 OpenCode provider 失败: %v", err)
	}

	liveData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("读取 OpenCode live 配置失败: %v", err)
	}
	if string(liveData) != string(originalLive) {
		t.Fatalf("禁用 provider 的本地操作不应重写 live config，实际=%s", liveData)
	}
}

func TestOpenCodeDuplicateProviderClearsLiveFlags(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 OpenCode 配置目录失败: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{"provider":{"managed":{"npm":"@ai-sdk/openai-compatible"}}}`), 0o644); err != nil {
		t.Fatalf("写入 OpenCode live 配置失败: %v", err)
	}
	service := &OpenCodeService{
		providers: []OpenCodeProvider{
			{
				ID:                     "managed",
				Name:                   "Managed",
				Enabled:                true,
				LiveConfigManaged:      boolPtr(true),
				IsInConfig:             boolPtr(true),
				SettingsConfig:         map[string]any{"npm": "@ai-sdk/openai-compatible"},
				QuotaAutoDisabled:      true,
				QuotaAutoDisablePaused: true,
			},
		},
	}

	duplicated, err := service.DuplicateProvider("managed")
	if err != nil {
		t.Fatalf("复制 OpenCode provider 失败: %v", err)
	}
	if duplicated == nil {
		t.Fatal("期望返回复制后的 OpenCode provider")
	}
	if duplicated.Enabled || duplicated.QuotaAutoDisabled || duplicated.QuotaAutoDisablePaused || shouldManageOpenCodeLiveProvider(*duplicated) {
		t.Fatalf("复制后的 OpenCode provider 应为禁用且 DB-only，实际=%#v", duplicated)
	}
}
