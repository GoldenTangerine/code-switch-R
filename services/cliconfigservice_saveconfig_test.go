package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestCliConfigServiceSaveConfigCodexUsesProviderContext(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	service := NewCliConfigService(":18100", nil)

	editable := map[string]interface{}{
		"model":                    "gpt-5-codex",
		"disable_response_storage": true,
		"features": map[string]interface{}{
			"parallel": true,
		},
	}

	if err := service.SaveConfig(
		string(PlatformCodex),
		editable,
		"https://api.vendor.example/v1/",
		"sk-provider",
		"My Codex Provider",
	); err != nil {
		t.Fatalf("SaveConfig 返回错误: %v", err)
	}

	configPath := filepath.Join(homeDir, ".codex", "config.toml")
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("读取 config.toml 失败: %v", err)
	}

	raw := make(map[string]interface{})
	if err := toml.Unmarshal(configContent, &raw); err != nil {
		t.Fatalf("解析 config.toml 失败: %v", err)
	}

	if got := anyToString(raw["model_provider"]); got != "my-codex-provider" {
		t.Fatalf("期望 model_provider 为 my-codex-provider，实际为 %q", got)
	}
	if got := anyToString(raw["preferred_auth_method"]); got != codexPreferredAuth {
		t.Fatalf("期望 preferred_auth_method 为 %q，实际为 %q", codexPreferredAuth, got)
	}

	modelProviders := normalizeTomlGenericMap(raw["model_providers"])
	providerMap := normalizeTomlGenericMap(modelProviders["my-codex-provider"])
	if got := anyToString(providerMap["base_url"]); got != "https://api.vendor.example/v1" {
		t.Fatalf("期望 base_url 为供应商 API 地址，实际为 %q", got)
	}
	if got := anyToString(providerMap["wire_api"]); got != codexWireAPI {
		t.Fatalf("期望 wire_api 为 %q，实际为 %q", codexWireAPI, got)
	}

	features := normalizeTomlGenericMap(raw["features"])
	if got, ok := features["parallel"].(bool); !ok || !got {
		t.Fatalf("期望 features.parallel 为 true，实际为 %#v", features["parallel"])
	}

	authPath := filepath.Join(homeDir, ".codex", "auth.json")
	authContent, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("读取 auth.json 失败: %v", err)
	}

	authPayload := make(map[string]string)
	if err := json.Unmarshal(authContent, &authPayload); err != nil {
		t.Fatalf("解析 auth.json 失败: %v", err)
	}
	if got := authPayload[codexEnvKey]; got != "sk-provider" {
		t.Fatalf("期望 auth.json 中 %s 为供应商 key，实际为 %q", codexEnvKey, got)
	}
}

func TestCliConfigServiceClaudeProxyPreviewUsesSelectedAuthField(t *testing.T) {
	useIsolatedHomeDir(t)
	appSettings := NewAppSettingsService(nil)
	settings, _ := appSettings.GetAppSettings()
	settings.ClaudeProxyAuthField = claudeProxyAuthFieldAPIKey
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatalf("保存应用设置失败: %v", err)
	}
	service := NewCliConfigService(":18100", appSettings)

	snapshots, err := service.GetConfigSnapshots(string(PlatformClaude), "", "", "proxy")
	if err != nil {
		t.Fatalf("获取 Claude 代理预览失败: %v", err)
	}
	if len(snapshots.PreviewFiles) != 1 {
		t.Fatalf("Claude 代理预览文件数量错误: %d", len(snapshots.PreviewFiles))
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(snapshots.PreviewFiles[0].Content), &payload); err != nil {
		t.Fatalf("解析 Claude 代理预览失败: %v", err)
	}
	env := payload["env"].(map[string]interface{})
	if anyToString(env[claudeAPIKeyEnvKey]) != claudeProxyAuthValue {
		t.Fatalf("代理预览未使用 API Key: %#v", env)
	}
	if _, exists := env[claudeAuthTokenEnvKey]; exists {
		t.Fatalf("代理预览不应同时包含 AUTH_TOKEN: %#v", env)
	}
}

func TestCliConfigServiceClaudeDirectModePreservesExistingAPIKey(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{
		"env": map[string]interface{}{
			claudeAPIKeyEnvKey: "existing-api-key",
		},
	})
	service := NewCliConfigService(":18100", nil)

	snapshots, err := service.GetConfigSnapshots(string(PlatformClaude), "https://direct.example.com", "direct-token", "direct")
	if err != nil {
		t.Fatalf("获取 Claude 直连预览失败: %v", err)
	}
	var preview map[string]interface{}
	if err := json.Unmarshal([]byte(snapshots.PreviewFiles[0].Content), &preview); err != nil {
		t.Fatalf("解析 Claude 直连预览失败: %v", err)
	}
	previewEnv := preview["env"].(map[string]interface{})
	if anyToString(previewEnv[claudeAPIKeyEnvKey]) != "existing-api-key" {
		t.Fatalf("直连预览不应删除既有 API Key: %#v", previewEnv)
	}

	editable := map[string]interface{}{
		"env": map[string]interface{}{
			claudeAPIKeyEnvKey: "existing-api-key",
		},
	}
	if err := service.SaveConfig(string(PlatformClaude), editable, "https://direct.example.com", "direct-token", ""); err != nil {
		t.Fatalf("保存 Claude 直连配置失败: %v", err)
	}
	savedEnv := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(savedEnv[claudeAPIKeyEnvKey]) != "existing-api-key" {
		t.Fatalf("直连保存不应删除既有 API Key: %#v", savedEnv)
	}
	config, err := service.getClaudeConfig()
	if err != nil {
		t.Fatalf("读取 Claude 直连高级配置失败: %v", err)
	}
	editableEnv, _ := config.Editable["env"].(map[string]interface{})
	if anyToString(editableEnv[claudeAPIKeyEnvKey]) != "existing-api-key" {
		t.Fatalf("直连 API Key 应保持可编辑: %#v", config.Editable)
	}
}
