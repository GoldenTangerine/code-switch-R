/**
 * @name: Grok Build 配置服务测试
 * @Descripttion: 覆盖代理接管的幂等/字段保留/备份恢复死锁解除、relayAddr 规范化与导入跳过代理态
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 07:30:00
 * @LastEditTime: 2026-08-17 07:30:00
 * @FilePath: services/groksettings_test.go
 */

package services

import (
	"os"
	"strings"
	"testing"
)

// writeGrokTestConfig 写入 TOML 语料到隔离 HOME 的 live 配置
func writeGrokTestConfig(t *testing.T, content string) {
	t.Helper()
	if err := os.MkdirAll(getGrokDir(), 0o755); err != nil {
		t.Fatalf("创建 Grok 配置目录失败: %v", err)
	}
	if err := os.WriteFile(getGrokConfigPath(), []byte(content), 0o644); err != nil {
		t.Fatalf("写入 Grok live 配置失败: %v", err)
	}
}

// grokDirectConfigTOML 直连态 fixture（选中 profile main）
func grokDirectConfigTOML(baseURL string) string {
	return "[models]\ndefault = \"main\"\n\n[model.main]\nmodel = \"grok-4\"\nbase_url = \"" + baseURL + "\"\nname = \"xAI\"\napi_key = \"sk-live\"\napi_backend = \"chat_completions\"\n"
}

// TestGrokProxyBaseURLNormalizesHostPortRelayAddr relayAddr 为 host:port 形式时端口需正确提取
func TestGrokProxyBaseURLNormalizesHostPortRelayAddr(t *testing.T) {
	useIsolatedHomeDir(t)

	cases := map[string]string{
		":18100":            "http://127.0.0.1:18100/grokbuild/v1",
		"127.0.0.1:18100":   "http://127.0.0.1:18100/grokbuild/v1",
		"http://127.0.0.1:18100": "http://127.0.0.1:18100/grokbuild/v1",
		"":                  "http://127.0.0.1:18100/grokbuild/v1",
	}
	for relayAddr, want := range cases {
		if got := NewGrokSettingsService(relayAddr).grokProxyBaseURL(); got != want {
			t.Errorf("relayAddr %q 的代理地址 = %q, want %q", relayAddr, got, want)
		}
	}
}

// TestGrokEnableProxyIdempotentPreservesBackup 重复开启代理不再覆盖接管前的备份
func TestGrokEnableProxyIdempotentPreservesBackup(t *testing.T) {
	useIsolatedHomeDir(t)
	writeGrokTestConfig(t, grokDirectConfigTOML("https://api.x.ai/v1"))
	service := NewGrokSettingsService(":18100")

	if err := service.EnableProxy(); err != nil {
		t.Fatalf("首次开启代理失败: %v", err)
	}
	backup, err := os.ReadFile(getGrokBackupPath())
	if err != nil {
		t.Fatalf("接管后应生成备份: %v", err)
	}
	if !strings.Contains(string(backup), "https://api.x.ai/v1") {
		t.Fatalf("备份内容应为接管前的直连配置: %s", backup)
	}

	// 二次开启：幂等返回，不得用「已接管态」覆盖备份
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("重复开启代理应幂等成功: %v", err)
	}
	backupAgain, err := os.ReadFile(getGrokBackupPath())
	if err != nil {
		t.Fatalf("幂等开启后备份应保留: %v", err)
	}
	if string(backupAgain) != string(backup) {
		t.Fatalf("重复开启代理覆盖了接管前的备份:\n%s\n---\n%s", backup, backupAgain)
	}
}

// TestGrokEnableProxyPreservesAPIBackend 代理接管保留用户原 api_backend，仅空值补默认
func TestGrokEnableProxyPreservesAPIBackend(t *testing.T) {
	useIsolatedHomeDir(t)

	// 用户显式配置 chat_completions：接管后保留
	writeGrokTestConfig(t, grokDirectConfigTOML("https://api.x.ai/v1"))
	service := NewGrokSettingsService(":18100")
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("开启代理失败: %v", err)
	}
	config, err := readGrokLiveConfig()
	if err != nil {
		t.Fatal(err)
	}
	if backend := config.Model["main"].APIBackend; backend != "chat_completions" {
		t.Fatalf("用户原 api_backend 应保留: %q", backend)
	}

	// 未配置 api_backend：接管后补默认 responses
	writeGrokTestConfig(t, "[models]\ndefault = \"main\"\n\n[model.main]\nbase_url = \"https://api.x.ai/v1\"\nname = \"xAI\"\n")
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("开启代理失败: %v", err)
	}
	config, err = readGrokLiveConfig()
	if err != nil {
		t.Fatal(err)
	}
	if backend := config.Model["main"].APIBackend; backend != grokDefaultAPIBackend {
		t.Fatalf("空 api_backend 应补默认 %q: %q", grokDefaultAPIBackend, backend)
	}
}

// TestGrokDisableProxyRestoresBackup 有备份时恢复直连配置并清理备份
func TestGrokDisableProxyRestoresBackup(t *testing.T) {
	useIsolatedHomeDir(t)
	direct := grokDirectConfigTOML("https://api.x.ai/v1")
	writeGrokTestConfig(t, direct)
	service := NewGrokSettingsService(":18100")

	if err := service.EnableProxy(); err != nil {
		t.Fatalf("开启代理失败: %v", err)
	}
	if err := service.DisableProxy(); err != nil {
		t.Fatalf("关闭代理失败: %v", err)
	}

	data, err := os.ReadFile(getGrokConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != direct {
		t.Fatalf("关闭代理后应恢复接管前的直连配置:\n%s", data)
	}
	if _, err := os.Stat(getGrokBackupPath()); !os.IsNotExist(err) {
		t.Fatalf("恢复后备份应删除: %v", err)
	}
}

// TestGrokDisableProxyWithoutBackupClearsModelSections 备份缺失时的死锁解除：
// 代理态 → 清除 model/models 回到未配置态；非代理态 → 保持报错不动配置
func TestGrokDisableProxyWithoutBackupClearsModelSections(t *testing.T) {
	useIsolatedHomeDir(t)
	service := NewGrokSettingsService(":18100")

	// 代理态 + 无备份：清除模型节，用户可重新应用供应商
	writeGrokTestConfig(t, grokDirectConfigTOML(service.grokProxyBaseURL()))
	if err := service.DisableProxy(); err != nil {
		t.Fatalf("备份缺失的代理态关闭应成功: %v", err)
	}
	config, err := readGrokLiveConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !isGrokOfficialLiveConfig(config) {
		t.Fatalf("备份缺失关闭后应回到未配置态（model/models 节清除）: %#v", config)
	}

	// 非代理态 + 无备份：无从恢复，保持报错且不动配置
	direct := grokDirectConfigTOML("https://api.x.ai/v1")
	writeGrokTestConfig(t, direct)
	if err := service.DisableProxy(); err == nil {
		t.Fatal("非代理态且无备份应返回错误")
	}
	data, readErr := os.ReadFile(getGrokConfigPath())
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != direct {
		t.Fatalf("非代理态无备份时不应改动配置:\n%s", data)
	}
}

// TestGrokImportFromLiveSkipsProxyState 代理接管态不导入（避免把代理占位地址存为供应商）
func TestGrokImportFromLiveSkipsProxyState(t *testing.T) {
	useIsolatedHomeDir(t)
	service := NewGrokSettingsService(":18100")
	writeGrokTestConfig(t, grokDirectConfigTOML(service.grokProxyBaseURL()))

	imported, err := service.ImportFromLive()
	if err != nil {
		t.Fatalf("代理态导入应静默跳过: %v", err)
	}
	if imported {
		t.Fatal("代理接管态不应导入供应商")
	}
	loaded, err := LoadProvidersFromStore("grokbuild")
	if err != nil || loaded != nil {
		t.Fatalf("代理态导入不应写入供应商: %#v, %v", loaded, err)
	}
}
