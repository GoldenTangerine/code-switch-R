package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateBackupUsesRollingFileAndRemovesLegacyBackups(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(configPath, []byte(`{"env":{"A":"1"}}`), 0o600); err != nil {
		t.Fatalf("写入原配置失败: %v", err)
	}

	legacyPaths := []string{
		configPath + ".bak.111",
		configPath + ".bak.222",
	}
	for _, legacyPath := range legacyPaths {
		if err := os.WriteFile(legacyPath, []byte("legacy"), 0o600); err != nil {
			t.Fatalf("写入旧备份失败 %s: %v", legacyPath, err)
		}
	}

	backupPath, err := CreateBackup(configPath)
	if err != nil {
		t.Fatalf("CreateBackup 返回错误: %v", err)
	}

	expected := configPath + ".bak"
	if backupPath != expected {
		t.Fatalf("期望备份路径为 %s，实际为 %s", expected, backupPath)
	}

	content, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("读取滚动备份失败: %v", err)
	}
	if string(content) != `{"env":{"A":"1"}}` {
		t.Fatalf("滚动备份内容不正确: %s", string(content))
	}

	for _, legacyPath := range legacyPaths {
		if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
			t.Fatalf("旧时间戳备份未清理: %s", legacyPath)
		}
	}
}

func TestFindLatestBackupSupportsRollingAndLegacyFormats(t *testing.T) {
	t.Run("prefer rolling backup", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "config.toml")
		rollingPath := configPath + ".bak"
		if err := os.WriteFile(rollingPath, []byte("rolling"), 0o600); err != nil {
			t.Fatalf("写入滚动备份失败: %v", err)
		}

		got, err := FindLatestBackup(configPath)
		if err != nil {
			t.Fatalf("FindLatestBackup 返回错误: %v", err)
		}
		if got != rollingPath {
			t.Fatalf("期望返回滚动备份 %s，实际为 %s", rollingPath, got)
		}
	})

	t.Run("fallback to newest legacy backup", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "config.toml")
		oldPath := configPath + ".bak.111"
		newPath := configPath + ".bak.222"

		if err := os.WriteFile(oldPath, []byte("old"), 0o600); err != nil {
			t.Fatalf("写入旧备份失败: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
		if err := os.WriteFile(newPath, []byte("new"), 0o600); err != nil {
			t.Fatalf("写入新备份失败: %v", err)
		}

		got, err := FindLatestBackup(configPath)
		if err != nil {
			t.Fatalf("FindLatestBackup 返回错误: %v", err)
		}
		if got != newPath {
			t.Fatalf("期望返回最新旧格式备份 %s，实际为 %s", newPath, got)
		}
	})

	t.Run("prefer rolling backup even if legacy is newer", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "config.toml")
		rollingPath := configPath + ".bak"
		newLegacyPath := configPath + ".bak.999"

		if err := os.WriteFile(rollingPath, []byte("rolling"), 0o600); err != nil {
			t.Fatalf("写入滚动备份失败: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
		if err := os.WriteFile(newLegacyPath, []byte("legacy"), 0o600); err != nil {
			t.Fatalf("写入旧格式备份失败: %v", err)
		}

		got, err := FindLatestBackup(configPath)
		if err != nil {
			t.Fatalf("FindLatestBackup 返回错误: %v", err)
		}
		if got != rollingPath {
			t.Fatalf("期望优先返回滚动备份 %s，实际为 %s", rollingPath, got)
		}
	})
}
