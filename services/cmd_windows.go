//go:build windows

/**
 * @name: Windows 命令执行
 * @Descripttion: 构造隐藏控制台窗口的命令并支持 CLI 批处理启动器。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:40:19
 * @LastEditTime: 2026-11-20 14:40:19
 * @FilePath: services/cmd_windows.go
 */
package services

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// hideWindowCmd 在 Windows 上创建隐藏控制台窗口的 exec.Cmd
func hideWindowCmd(name string, args ...string) *exec.Cmd {
	return hideWindowsConsole(exec.Command(name, args...))
}

func hideWindowsConsole(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func cliVersionCommand(ctx context.Context, path string) *exec.Cmd {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cmd", ".bat":
		cmdPath := filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
		cmd := hideWindowsConsole(exec.CommandContext(ctx, cmdPath))
		// 延迟展开在 CMD 识别元字符之后取值，保留安装路径里的空格和特殊字符。
		cmd.SysProcAttr.CmdLine = `cmd.exe /d /v:on /s /c ""!CODE_SWITCH_CLI_VERSION_PATH!" --version"`
		cmd.Env = append(cmd.Environ(), "CODE_SWITCH_CLI_VERSION_PATH="+path)
		return cmd
	default:
		return hideWindowsConsole(exec.CommandContext(ctx, path, "--version"))
	}
}
