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
	service := NewCliConfigService(":18100")

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
