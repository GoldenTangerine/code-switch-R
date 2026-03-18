package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const rollingBackupSuffix = ".bak"

// AtomicWriteJSON 原子写入 JSON 文件
// 写入临时文件后重命名，避免半写状态导致文件损坏
func AtomicWriteJSON(path string, data interface{}) error {
	// 序列化 JSON
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON 序列化失败: %w", err)
	}

	return AtomicWriteBytes(path, bytes)
}

// AtomicWriteBytes 原子写入字节数据
func AtomicWriteBytes(path string, data []byte) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录失败 %s: %w", dir, err)
	}

	// 生成临时文件路径（同目录下，避免跨文件系统问题）
	tmpPath := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())

	// 写入临时文件
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("写入临时文件失败 %s: %w", tmpPath, err)
	}

	// Windows: rename 目标存在时会失败，需要先删除
	if runtime.GOOS == "windows" {
		if _, err := os.Stat(path); err == nil {
			if err := os.Remove(path); err != nil {
				// 删除失败，清理临时文件
				os.Remove(tmpPath)
				return fmt.Errorf("删除目标文件失败 %s: %w", path, err)
			}
		}
	}

	// 原子重命名
	if err := os.Rename(tmpPath, path); err != nil {
		// 重命名失败，清理临时文件
		os.Remove(tmpPath)
		return fmt.Errorf("原子替换失败 %s -> %s: %w", tmpPath, path, err)
	}

	return nil
}

// AtomicWriteText 原子写入文本文件
func AtomicWriteText(path string, text string) error {
	return AtomicWriteBytes(path, []byte(text))
}

func backupPathFor(path string) string {
	return path + rollingBackupSuffix
}

func cleanupLegacyTimestampBackups(path string) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	pattern := base + rollingBackupSuffix + ".*"

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matched, _ := filepath.Match(pattern, entry.Name())
		if !matched {
			continue
		}
		legacyPath := filepath.Join(dir, entry.Name())
		if err := os.Remove(legacyPath); err != nil && !os.IsNotExist(err) {
			log.Printf("[Backup] 清理旧备份失败 %s: %v", legacyPath, err)
		}
	}
}

// CreateBackup 创建文件备份
// 统一写入固定的滚动备份文件，并清理旧的时间戳备份残留
func CreateBackup(path string) (string, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "", nil // 文件不存在，无需备份
	}
	if err != nil {
		return "", fmt.Errorf("检查原文件失败 %s: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("无法为目录创建备份: %s", path)
	}

	// 读取原文件
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取原文件失败 %s: %w", path, err)
	}

	backupPath := backupPathFor(path)
	tmpPath := fmt.Sprintf("%s.tmp.%d", backupPath, time.Now().UnixNano())

	if err := os.WriteFile(tmpPath, content, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("写入备份临时文件失败 %s: %w", tmpPath, err)
	}

	if runtime.GOOS == "windows" {
		if _, err := os.Stat(backupPath); err == nil {
			if err := os.Remove(backupPath); err != nil {
				_ = os.Remove(tmpPath)
				return "", fmt.Errorf("删除旧备份失败 %s: %w", backupPath, err)
			}
		}
	}

	if err := os.Rename(tmpPath, backupPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("写入备份文件失败 %s: %w", backupPath, err)
	}

	cleanupLegacyTimestampBackups(path)
	return backupPath, nil
}

// RestoreBackup 从备份恢复文件
func RestoreBackup(backupPath, targetPath string) error {
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", backupPath)
	}

	// 读取备份文件
	content, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("读取备份文件失败 %s: %w", backupPath, err)
	}

	// 原子写入目标文件
	return AtomicWriteBytes(targetPath, content)
}

// ReadJSONFile 读取 JSON 文件到指定结构
func ReadJSONFile(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// EnsureDir 确保目录存在
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// FindLatestBackup 查找可用于恢复的最新备份文件
// 优先兼容新的滚动备份文件，同时回退支持旧的 *.bak.<timestamp> 格式
func FindLatestBackup(configPath string) (string, error) {
	dir := filepath.Dir(configPath)
	base := filepath.Base(configPath)
	pattern := base + rollingBackupSuffix + ".*"

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("没有找到备份文件")
		}
		return "", err
	}

	var latestPath string
	var latestMod time.Time

	rollingPath := backupPathFor(configPath)
	if info, statErr := os.Stat(rollingPath); statErr == nil && !info.IsDir() {
		return rollingPath, nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matched, _ := filepath.Match(pattern, entry.Name())
		if !matched {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		if latestPath == "" || info.ModTime().After(latestMod) {
			latestPath = filepath.Join(dir, entry.Name())
			latestMod = info.ModTime()
		}
	}

	if latestPath == "" {
		return "", fmt.Errorf("没有找到备份文件")
	}

	return latestPath, nil
}

// OpenInExplorer 在系统文件管理器中打开指定目录
func OpenInExplorer(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default: // linux 等
		cmd = exec.Command("xdg-open", path)
	}

	return cmd.Start()
}
