package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readClaudeSettingsForTest(t *testing.T, homeDir string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(homeDir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("读取 Claude settings 失败: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("解析 Claude settings 失败: %v", err)
	}
	return payload
}

func writeClaudeSettingsForTest(t *testing.T, homeDir string, payload map[string]interface{}) {
	t.Helper()
	path := filepath.Join(homeDir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建 Claude 配置目录失败: %v", err)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("序列化 Claude settings 失败: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("写入 Claude settings 失败: %v", err)
	}
}

func TestClaudeSettingsServiceAPIKeyProxyRestoresBothAuthFields(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{
		"env": map[string]interface{}{
			"ANTHROPIC_BASE_URL":   "https://original.example.com",
			"ANTHROPIC_AUTH_TOKEN": "original-token",
			"ANTHROPIC_API_KEY":    "original-key",
			"KEEP_ME":              "yes",
		},
	})

	appSettings := NewAppSettingsService(nil)
	settings, _ := appSettings.GetAppSettings()
	settings.ClaudeProxyAuthField = claudeProxyAuthFieldAPIKey
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatalf("保存应用设置失败: %v", err)
	}
	service := NewClaudeSettingsService(":18100", appSettings)
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("启用 Claude 代理失败: %v", err)
	}

	env := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if got := anyToString(env[claudeAPIKeyEnvKey]); got != claudeProxyAuthValue {
		t.Fatalf("API Key 占位值错误: %q", got)
	}
	if _, exists := env[claudeAuthTokenEnvKey]; exists {
		t.Fatalf("API Key 模式不应保留 AUTH_TOKEN: %#v", env)
	}
	if got := anyToString(env["KEEP_ME"]); got != "yes" {
		t.Fatalf("无关字段被修改: %#v", env)
	}

	if err := service.DisableProxy(); err != nil {
		t.Fatalf("关闭 Claude 代理失败: %v", err)
	}
	restoredEnv := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(restoredEnv[claudeAuthTokenEnvKey]) != "original-token" || anyToString(restoredEnv[claudeAPIKeyEnvKey]) != "original-key" {
		t.Fatalf("认证字段未原样恢复: %#v", restoredEnv)
	}
}

func TestClaudeSettingsServiceDefaultProxyKeepsOnlyAuthToken(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{
		"env": map[string]interface{}{
			"ANTHROPIC_API_KEY": "original-key",
		},
	})

	service := NewClaudeSettingsService(":18100", nil)
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("启用 Claude 代理失败: %v", err)
	}
	env := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(env[claudeAuthTokenEnvKey]) != claudeProxyAuthValue {
		t.Fatalf("默认模式未写入 AUTH_TOKEN: %#v", env)
	}
	if _, exists := env[claudeAPIKeyEnvKey]; exists {
		t.Fatalf("默认模式不应同时保留 API Key: %#v", env)
	}

	if err := service.DisableProxy(); err != nil {
		t.Fatalf("关闭 Claude 代理失败: %v", err)
	}
	restoredEnv := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(restoredEnv[claudeAPIKeyEnvKey]) != "original-key" {
		t.Fatalf("原始 API Key 未恢复: %#v", restoredEnv)
	}
}

func TestClaudeSettingsServiceFallbackRemovesBothProxyPlaceholders(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{
		"env": map[string]interface{}{
			"ANTHROPIC_BASE_URL":   "http://127.0.0.1:18100",
			"ANTHROPIC_AUTH_TOKEN": claudeProxyAuthValue,
			"ANTHROPIC_API_KEY":    claudeProxyAuthValue,
			"KEEP_ME":              "yes",
		},
	})

	service := NewClaudeSettingsService(":18100", nil)
	if err := service.DisableProxy(); err != nil {
		t.Fatalf("无状态文件关闭 Claude 代理失败: %v", err)
	}
	env := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if _, exists := env[claudeAuthTokenEnvKey]; exists {
		t.Fatalf("兜底关闭未删除 AUTH_TOKEN 占位: %#v", env)
	}
	if _, exists := env[claudeAPIKeyEnvKey]; exists {
		t.Fatalf("兜底关闭未删除 API Key 占位: %#v", env)
	}
	if anyToString(env["KEEP_ME"]) != "yes" {
		t.Fatalf("兜底关闭误删无关字段: %#v", env)
	}
}

func TestClaudeSettingsServiceReconcileWithoutStateStillAllowsDisable(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{
		"env": map[string]interface{}{
			"ANTHROPIC_BASE_URL":   "http://127.0.0.1:18100",
			"ANTHROPIC_AUTH_TOKEN": claudeProxyAuthValue,
			"KEEP_ME":              "yes",
		},
	})

	service := NewClaudeSettingsService(":18100", nil)
	if err := service.ReconcileProxyAuthField(); err != nil {
		t.Fatalf("无状态启动协调失败: %v", err)
	}
	if exists, err := ProxyStateExists("claude"); err != nil || exists {
		t.Fatalf("无状态协调不应创建错误恢复基线: exists=%v err=%v", exists, err)
	}
	if err := service.DisableProxy(); err != nil {
		t.Fatalf("无状态协调后关闭代理失败: %v", err)
	}
	env := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if _, exists := env["ANTHROPIC_BASE_URL"]; exists {
		t.Fatalf("关闭后仍保留代理地址: %#v", env)
	}
	if _, exists := env[claudeAuthTokenEnvKey]; exists {
		t.Fatalf("关闭后仍保留代理占位认证: %#v", env)
	}
	if anyToString(env["KEEP_ME"]) != "yes" {
		t.Fatalf("关闭时误删无关字段: %#v", env)
	}
}

func TestClaudeSettingsServiceReappliesAuthFieldWhileProxyEnabled(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{"env": map[string]interface{}{}})

	appSettings := NewAppSettingsService(nil)
	service := NewClaudeSettingsService(":18100", appSettings)
	appSettings.BindClaudeSettingsService(service)
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("启用 Claude 代理失败: %v", err)
	}

	settings, _ := appSettings.GetAppSettings()
	settings.ClaudeProxyAuthField = claudeProxyAuthFieldAPIKey
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatalf("切换认证字段失败: %v", err)
	}
	env := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(env[claudeAPIKeyEnvKey]) != claudeProxyAuthValue {
		t.Fatalf("未立即切换到 API Key: %#v", env)
	}
	if _, exists := env[claudeAuthTokenEnvKey]; exists {
		t.Fatalf("即时切换后仍存在 AUTH_TOKEN: %#v", env)
	}
	state, err := LoadProxyState("claude")
	if err != nil {
		t.Fatalf("读取 Claude 代理状态失败: %v", err)
	}
	if state.InjectedAuthField != claudeProxyAuthFieldAPIKey {
		t.Fatalf("代理状态未记录当前认证字段: %#v", state)
	}
}

func TestClaudeSettingsServiceMigratesV1StateBeforeRestore(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{
		"env": map[string]interface{}{
			"ANTHROPIC_BASE_URL":   "http://127.0.0.1:18100",
			"ANTHROPIC_AUTH_TOKEN": claudeProxyAuthValue,
			"ANTHROPIC_API_KEY":    "legacy-original-key",
		},
	})
	originalToken := "legacy-original-token"
	if err := SaveProxyState("claude", &ProxyState{
		Version:           1,
		TargetPath:        filepath.Join(homeDir, ".claude", "settings.json"),
		FileExisted:       true,
		EnvExisted:        true,
		OriginalAuthToken: &originalToken,
		InjectedBaseURL:   "http://127.0.0.1:18100",
		InjectedAuthToken: claudeProxyAuthValue,
	}); err != nil {
		t.Fatalf("写入 v1 状态失败: %v", err)
	}

	service := NewClaudeSettingsService(":18100", nil)
	if err := service.DisableProxy(); err != nil {
		t.Fatalf("关闭 v1 Claude 代理失败: %v", err)
	}
	env := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(env[claudeAuthTokenEnvKey]) != originalToken || anyToString(env[claudeAPIKeyEnvKey]) != "legacy-original-key" {
		t.Fatalf("v1 状态迁移后恢复错误: %#v", env)
	}
}

func TestClaudeSettingsServiceManagesSubagentModelWithProxyLifecycle(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{
		"env": map[string]interface{}{
			claudeSubagentModelEnvKey: "original-subagent",
		},
	})

	service := NewClaudeSettingsService(":18100", nil)
	if err := service.SetSubagentModelRequired(true); err != nil {
		t.Fatalf("记录 Subagent 配置需求失败: %v", err)
	}
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("启用 Claude 代理失败: %v", err)
	}
	env := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(env[claudeSubagentModelEnvKey]) != claudeManagedSubagentModel {
		t.Fatalf("未注入托管 Subagent 模型: %#v", env)
	}

	if err := service.SetSubagentModelRequired(false); err != nil {
		t.Fatalf("清空 Subagent 配置后协调失败: %v", err)
	}
	env = readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(env[claudeSubagentModelEnvKey]) != "original-subagent" {
		t.Fatalf("清空配置后未恢复原始 Subagent 模型: %#v", env)
	}

	if err := service.SetSubagentModelRequired(true); err != nil {
		t.Fatalf("重新启用 Subagent 配置失败: %v", err)
	}
	if err := service.DisableProxy(); err != nil {
		t.Fatalf("关闭 Claude 代理失败: %v", err)
	}
	env = readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(env[claudeSubagentModelEnvKey]) != "original-subagent" {
		t.Fatalf("关闭代理后未恢复原始 Subagent 模型: %#v", env)
	}
}

func TestClaudeSettingsServiceDoesNotOverwriteManualSubagentModelChange(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{"env": map[string]interface{}{}})

	service := NewClaudeSettingsService(":18100", nil)
	if err := service.SetSubagentModelRequired(true); err != nil {
		t.Fatal(err)
	}
	if err := service.EnableProxy(); err != nil {
		t.Fatal(err)
	}
	payload := readClaudeSettingsForTest(t, homeDir)
	env := payload["env"].(map[string]interface{})
	env[claudeSubagentModelEnvKey] = "manual-subagent"
	writeClaudeSettingsForTest(t, homeDir, payload)

	if err := service.SetSubagentModelRequired(false); err != nil {
		t.Fatal(err)
	}
	if err := service.DisableProxy(); err != nil {
		t.Fatal(err)
	}
	env = readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(env[claudeSubagentModelEnvKey]) != "manual-subagent" {
		t.Fatalf("用户手动修改的 Subagent 模型不应被覆盖: %#v", env)
	}
}

func TestClaudeSettingsServiceMigratesV2StateWithoutChangingSubagentModel(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	originalAPIKey := "original-v2-api-key"
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{
		"env": map[string]interface{}{
			"ANTHROPIC_BASE_URL":      "http://127.0.0.1:18100",
			"ANTHROPIC_AUTH_TOKEN":    claudeProxyAuthValue,
			"ANTHROPIC_API_KEY":       claudeProxyAuthValue,
			claudeSubagentModelEnvKey: "existing-subagent",
		},
	})
	if err := SaveProxyState("claude", &ProxyState{
		Version:           2,
		TargetPath:        filepath.Join(homeDir, ".claude", "settings.json"),
		FileExisted:       true,
		EnvExisted:        true,
		OriginalAPIKey:    &originalAPIKey,
		InjectedBaseURL:   "http://127.0.0.1:18100",
		InjectedAuthToken: claudeProxyAuthValue,
	}); err != nil {
		t.Fatal(err)
	}

	service := NewClaudeSettingsService(":18100", nil)
	if err := service.DisableProxy(); err != nil {
		t.Fatal(err)
	}
	env := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(env[claudeSubagentModelEnvKey]) != "existing-subagent" {
		t.Fatalf("v2 状态迁移不应修改已有 Subagent 模型: %#v", env)
	}
	if anyToString(env[claudeAPIKeyEnvKey]) != originalAPIKey {
		t.Fatalf("v2 状态迁移不应覆盖已保存的原始 API Key: %#v", env)
	}
}

func TestClaudeSettingsServiceReapplyV2StatePreservesOriginalAPIKey(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	originalAPIKey := "original-v2-api-key"
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{
		"env": map[string]interface{}{
			"ANTHROPIC_BASE_URL": "http://127.0.0.1:18100",
			"ANTHROPIC_API_KEY":  claudeProxyAuthValue,
		},
	})
	if err := SaveProxyState("claude", &ProxyState{
		Version:           2,
		TargetPath:        filepath.Join(homeDir, ".claude", "settings.json"),
		FileExisted:       true,
		EnvExisted:        true,
		OriginalAPIKey:    &originalAPIKey,
		InjectedBaseURL:   "http://127.0.0.1:18100",
		InjectedAuthToken: claudeProxyAuthValue,
		InjectedAuthField: claudeProxyAuthFieldAPIKey,
	}); err != nil {
		t.Fatal(err)
	}

	service := NewClaudeSettingsService(":18100", nil)
	if err := service.ReapplyProxyAuthField(claudeProxyAuthFieldAPIKey); err != nil {
		t.Fatal(err)
	}
	state, err := LoadProxyState("claude")
	if err != nil {
		t.Fatal(err)
	}
	if state.OriginalAPIKey == nil || *state.OriginalAPIKey != originalAPIKey {
		t.Fatalf("v2 重应用覆盖了原始 API Key: %#v", state)
	}
	if err := service.DisableProxy(); err != nil {
		t.Fatal(err)
	}
	env := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(env[claudeAPIKeyEnvKey]) != originalAPIKey {
		t.Fatalf("v2 重应用后无法恢复原始 API Key: %#v", env)
	}
}

func TestReconcileClaudeSubagentModelRecoversInterruptedTransitions(t *testing.T) {
	t.Run("完成已写入设置的注入", func(t *testing.T) {
		env := map[string]interface{}{claudeSubagentModelEnvKey: claudeManagedSubagentModel}
		state := &ProxyState{
			InjectedSubagentModel:   claudeManagedSubagentModel,
			SubagentModelTransition: claudeSubagentTransitionInject,
		}

		if changed := reconcileClaudeSubagentModelEnv(env, state, true); changed {
			t.Fatal("恢复已完成的注入不应重复修改设置")
		}
		if !state.SubagentModelManaged || state.SubagentModelTransition != "" {
			t.Fatalf("注入过渡状态未正确收敛: %#v", state)
		}
	})

	t.Run("完成已写入设置的恢复", func(t *testing.T) {
		original := "original-subagent"
		env := map[string]interface{}{claudeSubagentModelEnvKey: original}
		state := &ProxyState{
			OriginalSubagentModel:   &original,
			InjectedSubagentModel:   claudeManagedSubagentModel,
			SubagentModelManaged:    true,
			SubagentModelTransition: claudeSubagentTransitionRestore,
		}

		if changed := reconcileClaudeSubagentModelEnv(env, state, false); changed {
			t.Fatal("恢复已完成的原值写入不应重复修改设置")
		}
		if state.SubagentModelManaged || state.SubagentModelTransition != "" || state.SubagentModelUserOverridden {
			t.Fatalf("恢复过渡状态未正确收敛: %#v", state)
		}
	})

	t.Run("恢复重试继续记录过渡状态", func(t *testing.T) {
		original := "original-subagent"
		env := map[string]interface{}{claudeSubagentModelEnvKey: claudeManagedSubagentModel}
		state := &ProxyState{
			OriginalSubagentModel:   &original,
			InjectedSubagentModel:   claudeManagedSubagentModel,
			SubagentModelTransition: claudeSubagentTransitionRestore,
		}

		changed := reconcileClaudeSubagentModelEnv(env, state, false)
		transition := claudeSubagentTransition(state.SubagentModelManaged, changed)
		if !changed || transition != claudeSubagentTransitionRestore || anyToString(env[claudeSubagentModelEnvKey]) != original {
			t.Fatalf("恢复重试未保持可续接状态: changed=%v transition=%q env=%#v state=%#v", changed, transition, env, state)
		}
	})
}

func TestClaudeSettingsServiceDisableRecoversInterruptedSubagentRestore(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	original := "original-subagent"
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{
		"env": map[string]interface{}{
			"ANTHROPIC_BASE_URL":      "http://127.0.0.1:18100",
			"ANTHROPIC_AUTH_TOKEN":    claudeProxyAuthValue,
			claudeSubagentModelEnvKey: claudeManagedSubagentModel,
		},
	})
	if err := SaveProxyState("claude", &ProxyState{
		Version:                     proxyStateVersion,
		TargetPath:                  filepath.Join(homeDir, ".claude", "settings.json"),
		FileExisted:                 true,
		EnvExisted:                  true,
		InjectedBaseURL:             "http://127.0.0.1:18100",
		InjectedAuthToken:           claudeProxyAuthValue,
		OriginalSubagentModel:       &original,
		InjectedSubagentModel:       claudeManagedSubagentModel,
		SubagentModelManaged:        false,
		SubagentModelTransition:     claudeSubagentTransitionRestore,
		SubagentModelUserOverridden: false,
	}); err != nil {
		t.Fatal(err)
	}

	service := NewClaudeSettingsService(":18100", nil)
	if err := service.DisableProxy(); err != nil {
		t.Fatal(err)
	}
	env := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(env[claudeSubagentModelEnvKey]) != original {
		t.Fatalf("关闭代理未恢复中断过渡的原始 Subagent 模型: %#v", env)
	}
}

func TestProviderSaveReconcilesClaudeSubagentModelOutsideProxySetup(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{"env": map[string]interface{}{}})

	claudeSettings := NewClaudeSettingsService(":18100", nil)
	if err := claudeSettings.EnableProxy(); err != nil {
		t.Fatal(err)
	}
	providerService := NewProviderService()
	providerService.BindClaudeSettingsService(claudeSettings)
	provider := Provider{
		ID:      1,
		Name:    "Subagent Provider",
		APIURL:  "https://example.com",
		APIKey:  "test-key",
		Enabled: true,
		ModelMapping: map[string]string{
			claudeDefaultModelMappingKey: "vendor-fallback",
		},
	}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatal(err)
	}
	env := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(env[claudeSubagentModelEnvKey]) != claudeManagedSubagentModel {
		t.Fatalf("保存供应商后未注入 Subagent 模型: %#v", env)
	}

	provider.ModelMapping = nil
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatal(err)
	}
	env = readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if _, exists := env[claudeSubagentModelEnvKey]; exists {
		t.Fatalf("清空供应商映射后未恢复 Subagent 模型: %#v", env)
	}
}

func TestClaudeSettingsServiceApplySingleProviderRejectsTransformedAPIFormat(t *testing.T) {
	useIsolatedHomeDir(t)

	providerPath, err := providerFilePath("claude")
	if err != nil {
		t.Fatalf("获取 provider 配置路径失败: %v", err)
	}

	payload, err := json.Marshal(providerEnvelope{
		Providers: []Provider{
			{
				ID:        42,
				Name:      "OpenAI Compatible Claude",
				APIURL:    "https://example.com",
				APIKey:    "test-key",
				APIFormat: claudeAPIFormatOpenAIChat,
			},
			{
				ID:                   44,
				Name:                 "Custom Header Claude",
				APIURL:               "https://example.com",
				APIKey:               "test-key",
				ConnectivityAuthType: "X-Custom-Auth",
			},
		},
	})
	if err != nil {
		t.Fatalf("序列化 provider 配置失败: %v", err)
	}

	if err := os.WriteFile(providerPath, payload, 0o600); err != nil {
		t.Fatalf("写入 provider 配置失败: %v", err)
	}

	service := NewClaudeSettingsService(":18100", nil)
	err = service.ApplySingleProvider(42)
	if err == nil {
		t.Fatal("期望直连应用被拒绝，但实际返回成功")
	}
	if !strings.Contains(err.Error(), "仅支持托管路由") {
		t.Fatalf("错误信息未命中托管路由限制，err=%v", err)
	}
	err = service.ApplySingleProvider(44)
	if err == nil || !strings.Contains(err.Error(), "仅支持托管路由") {
		t.Fatalf("自定义 Header 直连应用应被拒绝，err=%v", err)
	}
}

func TestClaudeSettingsServiceApplySingleProviderUsesSelectedAuthField(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	writeClaudeSettingsForTest(t, homeDir, map[string]interface{}{
		"env": map[string]interface{}{
			claudeAuthTokenEnvKey: "old-token",
		},
	})

	providerPath, err := providerFilePath("claude")
	if err != nil {
		t.Fatalf("获取 provider 配置路径失败: %v", err)
	}
	payload, err := json.Marshal(providerEnvelope{Providers: []Provider{
		{
			ID:                   43,
			Name:                 "Claude API Key",
			APIURL:               "https://example.com",
			APIKey:               "new-key",
			ConnectivityAuthType: "x-api-key",
		},
	}})
	if err != nil {
		t.Fatalf("序列化 provider 配置失败: %v", err)
	}
	if err := os.WriteFile(providerPath, payload, 0o600); err != nil {
		t.Fatalf("写入 provider 配置失败: %v", err)
	}

	service := NewClaudeSettingsService(":18100", nil)
	if err := service.ApplySingleProvider(43); err != nil {
		t.Fatalf("应用 Claude API Key 供应商失败: %v", err)
	}
	env := readClaudeSettingsForTest(t, homeDir)["env"].(map[string]interface{})
	if anyToString(env[claudeAPIKeyEnvKey]) != "new-key" {
		t.Fatalf("未写入 API Key: %#v", env)
	}
	if _, exists := env[claudeAuthTokenEnvKey]; exists {
		t.Fatalf("API Key 模式不应保留 AUTH_TOKEN: %#v", env)
	}
	appliedID, err := service.GetDirectAppliedProviderID()
	if err != nil || appliedID == nil || *appliedID != 43 {
		t.Fatalf("直连供应商识别错误: id=%v err=%v", appliedID, err)
	}
}
