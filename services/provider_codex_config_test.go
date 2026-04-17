package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
	if len(loaded) != 1 {
		t.Fatalf("期望读取到 1 个 provider，实际为 %d", len(loaded))
	}

	if got := loaded[0].CLIConfig["model"]; got != "gpt-5-codex" {
		t.Fatalf("期望 cliConfig.model 为 gpt-5-codex，实际为 %#v", got)
	}

	features, ok := loaded[0].CLIConfig["features"].(map[string]interface{})
	if !ok {
		t.Fatalf("期望 cliConfig.features 为 map，实际为 %#v", loaded[0].CLIConfig["features"])
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
	if len(loaded) != 1 || loaded[0].Name != "Legacy Codex" {
		t.Fatalf("旧路径回退读取失败: %#v", loaded)
	}

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
	if len(loaded) != 1 {
		t.Fatalf("期望读取到 1 个 provider，实际为 %d", len(loaded))
	}
	if loaded[0].BudgetQuotaSettings == nil || loaded[0].BudgetQuotaSettings.Total.Total != 512 {
		t.Fatalf("total quota 反序列化失败: %+v", loaded[0].BudgetQuotaSettings)
	}
	if loaded[0].BudgetQuotaUsedAdjustments == nil || loaded[0].BudgetQuotaUsedAdjustments.Total != 12.34 {
		t.Fatalf("total quota adjustment 反序列化失败: %+v", loaded[0].BudgetQuotaUsedAdjustments)
	}
	if loaded[0].ProviderQuotaQueryType != string(ProviderQuotaQueryTypeTokenPlanGLM) {
		t.Fatalf("providerQuotaQueryType 反序列化失败: %q", loaded[0].ProviderQuotaQueryType)
	}
}
