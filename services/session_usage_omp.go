/**
 * @name: Oh My Pi 会话用量同步
 * @Descripttion: 解析 Oh My Pi 本地会话记录并写入请求日志统计，解析引擎与 Pi 共用。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_omp.go
 */

package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// piFamilyOmp Oh My Pi 是独立于 Pi 的另一个编码代理，仅会话格式相同：
// 多一行标题槽，且 model_usage 条目也带用量；子代理有独立会话文件，不读 toolResult 里的 details
var piFamilyOmp = &piFamilyFlavor{
	label:             "Oh My Pi",
	platform:          sessionPlatformOhMyPi,
	dataSource:        requestLogDataSourceOhMyPiSession,
	idPrefix:          "omp_session",
	hashTag:           "omp-session",
	modelUsageCarrier: true,
	titleSlot:         true,
	resolveRoot:       ompResolveSessionRoot,
}

// syncOhMyPiSessionUsage Oh My Pi 会话用量同步入口；home 为用户主目录
func syncOhMyPiSessionUsage(ls *LogService, scope string, home string) (SessionSyncSourceResult, error) {
	return piFamilySync(ls, scope, home, piFamilyOmp)
}

// ompResolveSessionRoot Oh My Pi 的会话根目录：
//  1. PI_CODING_AGENT_SESSION_DIR（仅本机，必须是绝对路径，会话文件直接位于其下）
//  2. <数据目录>/sessions（按项目分目录）
//
// 数据目录规则（环境变量只在本机生效）：
//   - agent 目录：OMP_PROFILE / PI_PROFILE 指定 profile 时为 ~/<PI_CONFIG_DIR 或 .omp>/profiles/<profile>/agent，
//     否则取 PI_CODING_AGENT_DIR，默认 ~/.omp/agent
//   - Linux / macOS 上 agent 目录为默认位置且 $XDG_DATA_HOME/omp 已存在时，数据目录改用它；其余情况等于 agent 目录
func ompResolveSessionRoot(home string, scope string) (piFamilyRoot, error) {
	native := scope == "native"
	if native {
		if raw := os.Getenv("PI_CODING_AGENT_SESSION_DIR"); raw != "" {
			if !filepath.IsAbs(raw) {
				return piFamilyRoot{}, fmt.Errorf("Oh My Pi 会话目录为相对路径 %q，无法全局枚举", raw)
			}
			info, err := os.Stat(raw)
			if err != nil || !info.IsDir() {
				return piFamilyRoot{}, nil
			}
			return piFamilyRoot{Path: filepath.Clean(raw)}, nil
		}
	}

	profile := ""
	configDir := ""
	agentEnv := ""
	inheritedProfile := ""
	if native {
		var err error
		profile, err = ompNormalizeProfile(ompFirstEnv("OMP_PROFILE", "PI_PROFILE"))
		if err != nil {
			return piFamilyRoot{}, err
		}
		configDir = os.Getenv("PI_CONFIG_DIR")
		agentEnv = os.Getenv("PI_CODING_AGENT_DIR")
		// PI_PROFILE 非法时按未设置处理，与 cc-switch 一致
		inheritedProfile, _ = ompNormalizeProfile(os.Getenv("PI_PROFILE"))
	}

	agentDir, err := ompNativeAgentDir(home, configDir, profile, inheritedProfile, agentEnv)
	if err != nil {
		return piFamilyRoot{}, err
	}
	dataDir := agentDir
	if native && piFamilyUsesXDG() {
		defaultAgent, defaultErr := ompNativeAgentDir(home, configDir, profile, "", "")
		xdg := os.Getenv("XDG_DATA_HOME")
		if defaultErr == nil && xdg != "" && agentDir == defaultAgent {
			candidate := filepath.Join(xdg, "omp")
			if profile != "" {
				candidate = filepath.Join(candidate, "profiles", profile)
			}
			if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
				dataDir = candidate
			}
		}
	}
	return piFamilyRoot{Path: filepath.Join(dataDir, "sessions"), ProjectLayout: true}, nil
}

// ompNativeAgentDir 计算 Oh My Pi 原生 agent 目录
func ompNativeAgentDir(home string, configDir string, profile string, inheritedProfile string, agentEnv string) (string, error) {
	if configDir == "" {
		configDir = ".omp"
	}
	root := filepath.Join(home, strings.TrimLeft(configDir, "/\\"))
	if profile != "" {
		return filepath.Join(root, "profiles", profile, "agent"), nil
	}
	// 父 omp 进程可能导出了它自己 profile 的 agent 目录；显式默认模式下要忽略这个派生值
	if agentEnv != "" && inheritedProfile != "" && filepath.Clean(agentEnv) == filepath.Join(root, "profiles", inheritedProfile, "agent") {
		agentEnv = ""
	}
	if agentEnv != "" {
		if filepath.IsAbs(agentEnv) {
			return filepath.Clean(agentEnv), nil
		}
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("无法解析 Oh My Pi agent 目录: %w", err)
		}
		return filepath.Join(cwd, agentEnv), nil
	}
	return filepath.Join(root, "agent"), nil
}

// ompNormalizeProfile 校验 profile 名；空串与 default 视为未指定
func ompNormalizeProfile(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || name == "default" {
		return "", nil
	}
	reserved := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	reservedDevice := reserved == "CON" || reserved == "PRN" || reserved == "AUX" || reserved == "NUL" ||
		(len(reserved) == 4 && (strings.HasPrefix(reserved, "COM") || strings.HasPrefix(reserved, "LPT")) &&
			reserved[3] >= '0' && reserved[3] <= '9')
	isLowerOrDigit := func(char byte) bool { return (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') }
	valid := len(name) <= 64 && isLowerOrDigit(name[0]) && !strings.HasSuffix(name, ".") && !reservedDevice
	for index := 0; valid && index < len(name); index++ {
		char := name[index]
		valid = isLowerOrDigit(char) || char == '.' || char == '_' || char == '-'
	}
	if !valid {
		return "", fmt.Errorf("Oh My Pi profile 名称不合法: %s", name)
	}
	return name, nil
}

// ompFirstEnv 依次读取环境变量，返回第一个非空值
func ompFirstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}
