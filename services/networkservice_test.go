package services

import (
	"strings"
	"testing"
)

func TestBuildWSLClaudeConfigScriptUsesRollingBackupAndProtectsBackupSymlink(t *testing.T) {
	script := buildWSLClaudeConfigScript("http://127.0.0.1:18100")

	containsAll(t, script,
		`backup_path="$config_path.bak"`,
		`ensure_backup_path_safe()`,
		`create_backup "$config_path" "$backup_path"`,
		`Refusing to modify backup: $1 is a symlink.`,
	)
	if strings.Contains(script, `.bak.$ts`) {
		t.Fatalf("Claude 脚本不应再生成时间戳备份: %s", script)
	}
}

func TestBuildWSLCodexConfigScriptUsesRollingBackupAndProtectsBackupSymlink(t *testing.T) {
	script := buildWSLCodexConfigScript("http://127.0.0.1:18100")

	containsAll(t, script,
		`backup_config_path="$config_path.bak"`,
		`backup_auth_path="$auth_path.bak"`,
		`create_backup "$config_path" "$backup_config_path"`,
		`create_backup "$auth_path" "$backup_auth_path"`,
		`ensure_backup_path_safe()`,
	)
	if strings.Contains(script, `.bak.$ts`) {
		t.Fatalf("Codex 脚本不应再生成时间戳备份: %s", script)
	}
}

func TestBuildWSLGeminiConfigScriptUsesRollingBackupAndProtectsBackupSymlink(t *testing.T) {
	script := buildWSLGeminiConfigScript("http://127.0.0.1:18100")

	containsAll(t, script,
		`backup_path="$env_path.bak"`,
		`create_backup "$env_path" "$backup_path"`,
		`ensure_backup_path_safe()`,
		`gemini_base_url='http://127.0.0.1:18100/gemini'`,
	)
	if strings.Contains(script, `.bak.$ts`) {
		t.Fatalf("Gemini 脚本不应再生成时间戳备份: %s", script)
	}
}

func containsAll(t *testing.T, content string, expected ...string) {
	t.Helper()
	for _, item := range expected {
		if !strings.Contains(content, item) {
			t.Fatalf("期望脚本包含 %q", item)
		}
	}
}
