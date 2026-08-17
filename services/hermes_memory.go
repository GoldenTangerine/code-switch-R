/**
 * @name: Hermes Memory 子页服务
 * @Descripttion: 管理 ~/.hermes/memories 的 MEMORY.md/USER.md 整文件读写（§ 分隔条目）与 config.yaml 开关/预算
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 20:35:00
 * @LastEditTime: 2026-08-17 20:35:00
 * @FilePath: services/hermes_memory.go
 */

package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// Hermes memory 子目录（~/.hermes/memories）
	hermesMemoriesDirName = "memories"
	// agent 笔记文件（MEMORY.md）与用户画像文件（USER.md）
	hermesMemoryKindMemory = "memory"
	hermesMemoryKindUser   = "user"
	hermesMemoryFileName   = "MEMORY.md"
	hermesUserFileName     = "USER.md"
	// 条目分隔符：单独一行的 §
	hermesEntrySeparator = "§"
)

// HermesMemorySettings memory 子页设置（存于 config.yaml 顶层四键）
type HermesMemorySettings struct {
	MemoryEnabled      bool `json:"memory_enabled"`
	MemoryCharLimit    int  `json:"memory_char_limit"`
	UserProfileEnabled bool `json:"user_profile_enabled"`
	UserCharLimit      int  `json:"user_char_limit"`
}

// defaultHermesMemorySettings 缺省值（config.yaml 未写键时使用）
func defaultHermesMemorySettings() *HermesMemorySettings {
	return &HermesMemorySettings{
		MemoryEnabled:      true,
		MemoryCharLimit:    50000,
		UserProfileEnabled: true,
		UserCharLimit:      10000,
	}
}

// GetMemoryContent 读取 memory 整文件内容（kind: "memory" | "user"；文件缺失返回空串）
func (s *HermesService) GetMemoryContent(kind string) (string, error) {
	path, err := getHermesMemoryFilePath(kind)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// WriteMemoryContent 原子写入 memory 整文件内容（Markdown blob，调用方负责 § 分隔符拼接）
func (s *HermesService) WriteMemoryContent(kind, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := getHermesMemoryFilePath(kind)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// GetMemoryEntries 读取 memory 条目列表（按单独一行的 § 切分，空白条目过滤）
func (s *HermesService) GetMemoryEntries(kind string) ([]string, error) {
	content, err := s.GetMemoryContent(kind)
	if err != nil {
		return nil, err
	}
	return splitHermesMemoryEntries(content), nil
}

// SetMemorySettings 写入 memory 开关与字符预算（config.yaml 顶层四键，其余键保留）
func (s *HermesService) SetMemorySettings(enabled bool, charLimit int, userEnabled bool, userCharLimit int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return mutateHermesConfig(func(root *yaml.Node) error {
		hermesSetTopLevelValue(root, "memory_enabled", hermesEncodeValue(enabled))
		hermesSetTopLevelValue(root, "memory_char_limit", hermesEncodeValue(charLimit))
		hermesSetTopLevelValue(root, "user_profile_enabled", hermesEncodeValue(userEnabled))
		hermesSetTopLevelValue(root, "user_char_limit", hermesEncodeValue(userCharLimit))
		return nil
	})
}

// GetMemorySettings 读取 memory 开关与字符预算（缺键回退默认值）
func (s *HermesService) GetMemorySettings() (*HermesMemorySettings, error) {
	settings := defaultHermesMemorySettings()
	doc, err := readHermesLiveNode()
	if err != nil {
		return nil, err
	}
	root := hermesRootMapping(doc)
	if root == nil {
		return settings, nil
	}
	if node := hermesGetTopLevelValue(root, "memory_enabled"); node != nil {
		if value, ok := hermesDecodeNode(node).(bool); ok {
			settings.MemoryEnabled = value
		}
	}
	if node := hermesGetTopLevelValue(root, "user_profile_enabled"); node != nil {
		if value, ok := hermesDecodeNode(node).(bool); ok {
			settings.UserProfileEnabled = value
		}
	}
	settings.MemoryCharLimit = hermesNodeIntValue(root, "memory_char_limit", settings.MemoryCharLimit)
	settings.UserCharLimit = hermesNodeIntValue(root, "user_char_limit", settings.UserCharLimit)
	return settings, nil
}

// getHermesMemoryFilePath memory 文件路径（kind: "memory" → MEMORY.md，"user" → USER.md）
func getHermesMemoryFilePath(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case hermesMemoryKindMemory:
		return filepath.Join(getHermesDir(), hermesMemoriesDirName, hermesMemoryFileName), nil
	case hermesMemoryKindUser:
		return filepath.Join(getHermesDir(), hermesMemoriesDirName, hermesUserFileName), nil
	default:
		return "", fmt.Errorf("无效的 Hermes memory 类型: %s（支持 memory / user）", kind)
	}
}

// splitHermesMemoryEntries 按单独一行的 § 切分条目（首尾空白修剪，空白条目丢弃）
func splitHermesMemoryEntries(content string) []string {
	raw := strings.Split(content, "\n")
	entries := make([]string, 0, len(raw))
	var current []string
	for _, line := range raw {
		if strings.TrimSpace(line) == hermesEntrySeparator {
			entries = append(entries, strings.Join(current, "\n"))
			current = nil
			continue
		}
		current = append(current, line)
	}
	entries = append(entries, strings.Join(current, "\n"))

	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// hermesNodeIntValue 读取顶层整型键（兼容 int/int64/float64 解码形态，缺省回退 fallback）
func hermesNodeIntValue(root *yaml.Node, key string, fallback int) int {
	node := hermesGetTopLevelValue(root, key)
	if node == nil {
		return fallback
	}
	switch value := hermesDecodeNode(node).(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	}
	return fallback
}
