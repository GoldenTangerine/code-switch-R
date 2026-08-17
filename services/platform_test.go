/**
 * @name: 应用平台常量与能力判定测试
 * @Descripttion: 验证 platform 归一化与能力分组（代理/共存/MCP/Skill/Prompt）的判定逻辑
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 03:15:00
 * @LastEditTime: 2026-08-17 03:15:00
 * @FilePath: services/platform_test.go
 */

package services

import "testing"

func TestNormalizePlatform(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"claude", "claude"},
		{"Claude", "claude"},
		{"claude-code", "claude"},
		{"claude_code", "claude"},
		{"codex", "codex"},
		{"gemini", "gemini"},
		{"opencode", "opencode"},
		{"grok", "grokbuild"},
		{"grok-build", "grokbuild"},
		{"grok_build", "grokbuild"},
		{"grokbuild", "grokbuild"},
		{"claude-desktop", "claude-desktop"},
		{"openclaw", "openclaw"},
		{"hermes", "hermes"},
		{"pi", "pi"},
		{"  Codex  ", "codex"},
	}
	for _, c := range cases {
		if got := NormalizePlatform(c.input); got != c.expected {
			t.Errorf("NormalizePlatform(%q) = %q, 期望 %q", c.input, got, c.expected)
		}
	}
}

func TestIsCustomPlatform(t *testing.T) {
	if !IsCustomPlatform("custom:mytool") {
		t.Error("custom:mytool 应被识别为自定义平台")
	}
	if IsCustomPlatform("claude") {
		t.Error("claude 不应被识别为自定义平台")
	}
}

func TestIsAdditivePlatform(t *testing.T) {
	additive := []string{"opencode", "openclaw", "hermes", "pi"}
	for _, platform := range additive {
		if !IsAdditivePlatform(platform) {
			t.Errorf("%s 应为共存模式", platform)
		}
	}
	switched := []string{"claude", "codex", "gemini", "grokbuild", "claude-desktop", "custom:tool"}
	for _, platform := range switched {
		if IsAdditivePlatform(platform) {
			t.Errorf("%s 应为独占模式", platform)
		}
	}
}

func TestPlatformSupportsProxy(t *testing.T) {
	proxyable := []string{"claude", "codex", "gemini", "grokbuild"}
	for _, platform := range proxyable {
		if !PlatformSupportsProxy(platform) {
			t.Errorf("%s 应支持本地代理托管", platform)
		}
	}
	nonProxyable := []string{"opencode", "openclaw", "hermes", "pi", "claude-desktop"}
	for _, platform := range nonProxyable {
		if PlatformSupportsProxy(platform) {
			t.Errorf("%s 不应支持本地代理托管", platform)
		}
	}
}

func TestPlatformSupportsMCP(t *testing.T) {
	supported := []string{"claude", "codex", "gemini", "grokbuild", "opencode", "hermes"}
	for _, platform := range supported {
		if !PlatformSupportsMCP(platform) {
			t.Errorf("%s 应支持 MCP 投影", platform)
		}
	}
	unsupported := []string{"claude-desktop", "openclaw", "pi", "custom:tool"}
	for _, platform := range unsupported {
		if PlatformSupportsMCP(platform) {
			t.Errorf("%s 不应支持 MCP 投影", platform)
		}
	}
}

func TestPlatformSupportsSkillAndPrompt(t *testing.T) {
	supported := []string{"claude", "codex", "gemini", "grokbuild", "opencode", "openclaw", "hermes", "pi"}
	for _, platform := range supported {
		if !PlatformSupportsSkill(platform) {
			t.Errorf("%s 应支持 Skill 投影", platform)
		}
		if !PlatformSupportsPrompt(platform) {
			t.Errorf("%s 应支持 Prompt 投影", platform)
		}
	}
	for _, platform := range []string{"claude-desktop", "custom:tool"} {
		if PlatformSupportsSkill(platform) {
			t.Errorf("%s 不应支持 Skill 投影", platform)
		}
		if PlatformSupportsPrompt(platform) {
			t.Errorf("%s 不应支持 Prompt 投影", platform)
		}
	}
}
