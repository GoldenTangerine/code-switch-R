//go:build !windows

/**
 * @name: 非 Windows 命令执行
 * @Descripttion: 提供适用于 macOS 和 Linux 的命令构造。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:40:19
 * @LastEditTime: 2026-11-20 14:40:19
 * @FilePath: services/cmd_other.go
 */
package services

import (
	"context"
	"os/exec"
)

// hideWindowCmd 在非 Windows 平台创建 exec.Cmd（无隐藏窗口逻辑）
func hideWindowCmd(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func cliVersionCommand(ctx context.Context, path string) *exec.Cmd {
	return exec.CommandContext(ctx, path, "--version")
}
