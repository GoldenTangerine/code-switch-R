/**
 * @name: 应用平台常量与能力判定
 * @Descripttion: 集中定义所有受管应用的 platform 标识及其能力分组（代理/共存/MCP/Skill/Prompt）
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 02:30:00
 * @LastEditTime: 2026-08-17 03:10:00
 * @FilePath: services/platform.go
 */

package services

import "strings"

// 受管应用 platform 标识（贯穿 SQLite providers_store 表、request_log.platform、前端 tabId）
// claude / codex / gemini 三个常量已在 cliconfigservice.go 中定义（CLIPlatform 类型），此处仅新增应用
const (
	PlatformOpenCode      CLIPlatform = "opencode"
	PlatformGrokBuild     CLIPlatform = "grokbuild"
	PlatformClaudeDesktop CLIPlatform = "claude-desktop"
	PlatformOpenClaw      CLIPlatform = "openclaw"
	PlatformHermes        CLIPlatform = "hermes"
	PlatformPi            CLIPlatform = "pi"
)

// CustomPlatformPrefix 自定义 CLI 工具的 platform 前缀（custom:{toolId}）
const CustomPlatformPrefix = "custom:"

// NormalizePlatform 归一化 platform 标识，兼容历史别名写法
// claude-code / claude_code → claude；grok / grok-build / grok_build → grokbuild
func NormalizePlatform(kind string) string {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	switch CLIPlatform(normalized) {
	case "claude-code", "claude_code":
		return string(PlatformClaude)
	case "grok", "grok-build", "grok_build":
		return string(PlatformGrokBuild)
	default:
		return normalized
	}
}

// IsCustomPlatform 判断是否为自定义 CLI 工具平台（custom:{toolId}）
func IsCustomPlatform(platform string) bool {
	return strings.HasPrefix(platform, CustomPlatformPrefix)
}

// IsAdditivePlatform 判断是否为「共存模式」应用
// 共存模式：所有供应商以条目形式共存于原生配置文件，切换 = 启用某条目（opencode/openclaw/hermes/pi）
// 独占模式：同一时刻仅一个供应商写入原生配置（claude/codex/gemini/grokbuild/claude-desktop 及自定义 CLI）
func IsAdditivePlatform(platform string) bool {
	switch CLIPlatform(NormalizePlatform(platform)) {
	case PlatformOpenCode, PlatformOpenClaw, PlatformHermes, PlatformPi:
		return true
	default:
		return false
	}
}

// PlatformSupportsProxy 判断应用是否支持本地代理托管（:18100 代理开关）
// claude/codex/gemini/grokbuild 走代理调度；claude-desktop 通过 Direct/Proxy 双模式间接复用 Claude 链路
func PlatformSupportsProxy(platform string) bool {
	switch CLIPlatform(NormalizePlatform(platform)) {
	case PlatformClaude, PlatformCodex, PlatformGemini, PlatformGrokBuild:
		return true
	default:
		return false
	}
}

// PlatformSupportsMCP 判断应用是否支持 MCP 配置投影
// claude-desktop / openclaw / pi 的原生配置不支持 MCP 节点
func PlatformSupportsMCP(platform string) bool {
	if IsCustomPlatform(platform) {
		return false
	}
	switch CLIPlatform(NormalizePlatform(platform)) {
	case PlatformClaude, PlatformCodex, PlatformGemini, PlatformGrokBuild, PlatformOpenCode, PlatformHermes:
		return true
	default:
		return false
	}
}

// PlatformSupportsSkill 判断应用是否支持 Skill 投影（claude-desktop 不支持）
func PlatformSupportsSkill(platform string) bool {
	if IsCustomPlatform(platform) {
		return false
	}
	return CLIPlatform(NormalizePlatform(platform)) != PlatformClaudeDesktop
}

// PlatformSupportsPrompt 判断应用是否支持提示词文件投影（claude-desktop 不支持）
func PlatformSupportsPrompt(platform string) bool {
	if IsCustomPlatform(platform) {
		return false
	}
	return CLIPlatform(NormalizePlatform(platform)) != PlatformClaudeDesktop
}
