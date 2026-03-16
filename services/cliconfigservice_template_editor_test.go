package services

import (
	"strings"
	"testing"
)

func TestRenderTemplateEditorContentCodexUsesTOML(t *testing.T) {
	service := &CliConfigService{}

	rendered, err := service.RenderTemplateEditorContent(string(PlatformCodex), map[string]interface{}{
		"disable_response_storage": true,
		"model":                    "gpt-5.1-codex-max",
		"model_provider":           "myCode",
		"model_reasoning_effort":   "high",
		"preferred_auth_method":    "oauth",
		"features": map[string]interface{}{
			"parallel": true,
		},
		"model_providers": map[string]interface{}{
			"code-switch-r": map[string]interface{}{
				"base_url":             "http://127.0.0.1:8080",
				"name":                 "code-switch-r",
				"requires_openai_auth": false,
				"wire_api":             "responses",
			},
			"myCode": map[string]interface{}{
				"base_url":             "",
				"name":                 "myCode",
				"requires_openai_auth": true,
				"wire_api":             "responses",
			},
		},
	})
	if err != nil {
		t.Fatalf("RenderTemplateEditorContent 返回错误: %v", err)
	}

	if rendered.Format != "toml" {
		t.Fatalf("期望格式为 toml，实际为 %q", rendered.Format)
	}
	if !strings.Contains(rendered.Content, "model = ") || !strings.Contains(rendered.Content, "gpt-5.1-codex-max") {
		t.Fatalf("渲染结果缺少 model 字段: %s", rendered.Content)
	}
	if !strings.Contains(rendered.Content, "[features]") {
		t.Fatalf("渲染结果缺少 features table: %s", rendered.Content)
	}
	if !strings.Contains(rendered.Content, "[model_providers.myCode]") {
		t.Fatalf("渲染结果缺少 model_providers.myCode table: %s", rendered.Content)
	}
	if strings.Contains(rendered.Content, "\nmodel_provider =") || strings.HasPrefix(rendered.Content, "model_provider =") {
		t.Fatalf("渲染结果不应包含 model_provider: %s", rendered.Content)
	}
	if strings.Contains(rendered.Content, "\npreferred_auth_method =") || strings.HasPrefix(rendered.Content, "preferred_auth_method =") {
		t.Fatalf("渲染结果不应包含 preferred_auth_method: %s", rendered.Content)
	}
	if strings.Contains(rendered.Content, "[model_providers.code-switch-r]") {
		t.Fatalf("渲染结果不应包含 code-switch-r provider: %s", rendered.Content)
	}
	if strings.Contains(rendered.Content, "\n[model_providers]\n") {
		t.Fatalf("渲染结果不应保留多余的 [model_providers] 头: %s", rendered.Content)
	}
}

func TestNormalizeTemplateEditorContentCodexParsesTOML(t *testing.T) {
	service := &CliConfigService{}

	input := strings.TrimSpace(`
disable_response_storage = true
model = "gpt-5.1-codex-max"
model_reasoning_effort = "high"
model_provider = "myCode"
preferred_auth_method = "oauth"

[experimental]
use_freeform_apply_patch = true

[features]
parallel = true

[model_providers.code-switch-r]
base_url = "http://127.0.0.1:8080"
name = "code-switch-r"
requires_openai_auth = false
wire_api = "responses"

[model_providers.myCode]
base_url = ""
name = "myCode"
requires_openai_auth = true
wire_api = "responses"
`)

	normalized, err := service.NormalizeTemplateEditorContent(string(PlatformCodex), input)
	if err != nil {
		t.Fatalf("NormalizeTemplateEditorContent 返回错误: %v", err)
	}

	if normalized.Format != "toml" {
		t.Fatalf("期望格式为 toml，实际为 %q", normalized.Format)
	}
	if got := anyToString(normalized.Editable["model"]); got != "gpt-5.1-codex-max" {
		t.Fatalf("期望 model 为 gpt-5.1-codex-max，实际为 %q", got)
	}
	if got, ok := normalized.Editable["disable_response_storage"].(bool); !ok || !got {
		t.Fatalf("期望 disable_response_storage 为 true，实际为 %#v", normalized.Editable["disable_response_storage"])
	}
	if _, exists := normalized.Editable["model_provider"]; exists {
		t.Fatalf("标准化结果不应包含 model_provider: %#v", normalized.Editable["model_provider"])
	}
	if _, exists := normalized.Editable["preferred_auth_method"]; exists {
		t.Fatalf("标准化结果不应包含 preferred_auth_method: %#v", normalized.Editable["preferred_auth_method"])
	}

	features, ok := normalized.Editable["features"].(map[string]interface{})
	if !ok {
		t.Fatalf("期望 features 为 map，实际为 %#v", normalized.Editable["features"])
	}
	if got, ok := features["parallel"].(bool); !ok || !got {
		t.Fatalf("期望 features.parallel 为 true，实际为 %#v", features["parallel"])
	}

	modelProviders, ok := normalized.Editable["model_providers"].(map[string]interface{})
	if !ok {
		t.Fatalf("期望 model_providers 为 map，实际为 %#v", normalized.Editable["model_providers"])
	}
	if _, exists := modelProviders["code-switch-r"]; exists {
		t.Fatalf("标准化结果不应包含 code-switch-r provider: %#v", modelProviders["code-switch-r"])
	}
	providerValue, ok := modelProviders["myCode"].(map[string]interface{})
	if !ok {
		t.Fatalf("期望 model_providers.myCode 为 map，实际为 %#v", modelProviders["myCode"])
	}
	if got := anyToString(providerValue["wire_api"]); got != "responses" {
		t.Fatalf("期望 model_providers.myCode.wire_api 为 responses，实际为 %q", got)
	}

	if !strings.Contains(normalized.Content, "[model_providers.myCode]") {
		t.Fatalf("标准化结果缺少 model_providers.myCode table: %s", normalized.Content)
	}
	if strings.Contains(normalized.Content, "[model_providers.code-switch-r]") {
		t.Fatalf("标准化结果不应包含 code-switch-r provider: %s", normalized.Content)
	}
}

func TestNormalizeTemplateEditorContentClaudeStripsLockedEnvFields(t *testing.T) {
	service := &CliConfigService{}

	normalized, err := service.NormalizeTemplateEditorContent(string(PlatformClaude), strings.TrimSpace(`{
  "env": {
    "ANTHROPIC_BASE_URL": "https://example.com",
    "ANTHROPIC_AUTH_TOKEN": "secret",
    "ANTHROPIC_CUSTOM_HEADER": "value"
  },
  "model": "claude-sonnet-4-5"
}`))
	if err != nil {
		t.Fatalf("NormalizeTemplateEditorContent 返回错误: %v", err)
	}

	envValue, ok := normalized.Editable["env"].(map[string]interface{})
	if !ok {
		t.Fatalf("期望 env 为 map，实际为 %#v", normalized.Editable["env"])
	}
	if _, exists := envValue["ANTHROPIC_BASE_URL"]; exists {
		t.Fatalf("标准化结果不应包含 ANTHROPIC_BASE_URL: %#v", envValue["ANTHROPIC_BASE_URL"])
	}
	if _, exists := envValue["ANTHROPIC_AUTH_TOKEN"]; exists {
		t.Fatalf("标准化结果不应包含 ANTHROPIC_AUTH_TOKEN: %#v", envValue["ANTHROPIC_AUTH_TOKEN"])
	}
	if got := anyToString(envValue["ANTHROPIC_CUSTOM_HEADER"]); got != "value" {
		t.Fatalf("期望 env.ANTHROPIC_CUSTOM_HEADER 为 value，实际为 %q", got)
	}
}
