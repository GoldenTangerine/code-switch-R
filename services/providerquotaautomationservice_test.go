/**
 * @name: 供应商额度自动停用服务测试
 * @Descripttion: 验证远端额度变化驱动供应商自动停用与恢复的完整状态流转
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-30 18:23:27
 * @LastEditTime: 2026-07-30 18:23:27
 * @FilePath: services/providerquotaautomationservice_test.go
 */
package services

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newQuotaAutomationQueryService(t *testing.T, remaining *atomic.Int64) *ProviderQuotaQueryService {
	t.Helper()
	client := &http.Client{Transport: quotaAutomationRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		body := fmt.Sprintf(`{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":%d}]}`, remaining.Load())
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})}
	service := NewProviderQuotaQueryService()
	service.client = client
	return service
}

type quotaAutomationRoundTripperFunc func(*http.Request) (*http.Response, error)

func (fn quotaAutomationRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func newQuotaAutomationServiceForTest(
	t *testing.T,
	autoDisableEnabled bool,
	remaining *atomic.Int64,
) (*ProviderQuotaAutomationService, *ProviderService, *OpenCodeService, *AppSettingsService) {
	t.Helper()
	useIsolatedHomeDir(t)

	appSettings := NewAppSettingsService(nil)
	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatalf("读取测试设置失败: %v", err)
	}
	settings.ProviderQuotaAutoDisableEnabled = autoDisableEnabled
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatalf("保存测试设置失败: %v", err)
	}

	providerService := NewProviderService()
	openCodeService := NewOpenCodeService()
	automation := NewProviderQuotaAutomationService(
		newQuotaAutomationQueryService(t, remaining),
		providerService,
		NewGeminiService("127.0.0.1:18100"),
		openCodeService,
		appSettings,
		nil,
		nil,
	)
	appSettings.BindProviderQuotaAutomationService(automation)
	return automation, providerService, openCodeService, appSettings
}

func requireProviderQuotaState(
	t *testing.T,
	providerService *ProviderService,
	providerID int64,
	wantEnabled bool,
	wantAutoDisabled bool,
	wantPaused bool,
) {
	t.Helper()
	providers, err := providerService.LoadProviders("codex")
	if err != nil {
		t.Fatalf("读取 Codex 供应商失败: %v", err)
	}
	for _, provider := range providers {
		if provider.ID != providerID {
			continue
		}
		if provider.Enabled != wantEnabled || provider.QuotaAutoDisabled != wantAutoDisabled || provider.QuotaAutoDisablePaused != wantPaused {
			t.Fatalf(
				"供应商状态不符合预期: got=(%v,%v,%v) want=(%v,%v,%v)",
				provider.Enabled,
				provider.QuotaAutoDisabled,
				provider.QuotaAutoDisablePaused,
				wantEnabled,
				wantAutoDisabled,
				wantPaused,
			)
		}
		return
	}
	t.Fatalf("未找到测试供应商: %d", providerID)
}

func TestProviderQuotaRecoverySettingsNormalizeInterval(t *testing.T) {
	useIsolatedHomeDir(t)
	appSettings := NewAppSettingsService(nil)
	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatalf("读取默认设置失败: %v", err)
	}
	if settings.QuotaRecoveryIntervalSeconds != defaultQuotaRecoveryIntervalSeconds {
		t.Fatalf("后台恢复间隔默认值错误: %d", settings.QuotaRecoveryIntervalSeconds)
	}
	if settings.QuotaRecoveryNotifyEnabled {
		t.Fatal("额度恢复通知默认应关闭")
	}

	settings.QuotaRecoveryIntervalSeconds = 1
	saved, err := appSettings.SaveAppSettings(settings)
	if err != nil {
		t.Fatalf("保存最小间隔失败: %v", err)
	}
	if saved.QuotaRecoveryIntervalSeconds != minQuotaRecoveryIntervalSeconds {
		t.Fatalf("后台恢复间隔未限制到最小值: %d", saved.QuotaRecoveryIntervalSeconds)
	}

	saved.QuotaRecoveryIntervalSeconds = maxQuotaRecoveryIntervalSeconds + 1
	saved, err = appSettings.SaveAppSettings(saved)
	if err != nil {
		t.Fatalf("保存最大间隔失败: %v", err)
	}
	if saved.QuotaRecoveryIntervalSeconds != maxQuotaRecoveryIntervalSeconds {
		t.Fatalf("后台恢复间隔未限制到最大值: %d", saved.QuotaRecoveryIntervalSeconds)
	}
}

func TestProviderQuotaRecoverySchedulerWaitsBeforeFirstCheck(t *testing.T) {
	var remaining atomic.Int64
	remaining.Store(10)
	automation, providerService, _, _ := newQuotaAutomationServiceForTest(t, true, &remaining)
	if err := providerService.SaveProviders("codex", []Provider{{
		ID:                     401,
		Name:                   "Waiting Provider",
		APIURL:                 "https://api.deepseek.com/v1",
		APIKey:                 "test-key",
		Enabled:                false,
		QuotaAutoDisabled:      true,
		ProviderQuotaQueryType: string(ProviderQuotaQueryTypeBalance),
	}}); err != nil {
		t.Fatalf("保存待恢复供应商失败: %v", err)
	}

	if err := automation.Start(); err != nil {
		t.Fatalf("启动额度恢复调度失败: %v", err)
	}
	t.Cleanup(func() { _ = automation.Stop() })
	time.Sleep(100 * time.Millisecond)
	requireProviderQuotaState(t, providerService, 401, false, true, false)
}

func TestProviderQuotaRecoverySchedulerResetsIntervalAndRestoresProvider(t *testing.T) {
	var remaining atomic.Int64
	remaining.Store(10)
	automation, providerService, _, appSettings := newQuotaAutomationServiceForTest(t, true, &remaining)
	if err := providerService.SaveProviders("codex", []Provider{{
		ID:                     402,
		Name:                   "Interval Reset Provider",
		APIURL:                 "https://api.deepseek.com/v1",
		APIKey:                 "test-key",
		Enabled:                false,
		QuotaAutoDisabled:      true,
		ProviderQuotaQueryType: string(ProviderQuotaQueryTypeBalance),
	}}); err != nil {
		t.Fatalf("保存间隔热更新供应商失败: %v", err)
	}

	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatalf("读取后台恢复设置失败: %v", err)
	}
	settings.QuotaRecoveryIntervalSeconds = maxQuotaRecoveryIntervalSeconds
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatalf("保存初始后台恢复间隔失败: %v", err)
	}
	if err := automation.Start(); err != nil {
		t.Fatalf("启动额度恢复调度失败: %v", err)
	}
	t.Cleanup(func() { _ = automation.Stop() })

	// 先让调度器进入长周期等待，再验证设置变更会重置当前计时器。
	time.Sleep(100 * time.Millisecond)
	settings.QuotaRecoveryIntervalSeconds = minQuotaRecoveryIntervalSeconds
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatalf("热更新后台恢复间隔失败: %v", err)
	}

	deadline := time.Now().Add(time.Duration(minQuotaRecoveryIntervalSeconds+2) * time.Second)
	for time.Now().Before(deadline) {
		providers, loadErr := providerService.LoadProviders("codex")
		if loadErr != nil {
			t.Fatalf("读取自动恢复结果失败: %v", loadErr)
		}
		if len(providers) == 1 && providers[0].Enabled && !providers[0].QuotaAutoDisabled {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	requireProviderQuotaState(t, providerService, 402, true, false, false)
}

func TestProviderQuotaRecoverySchedulerStopsPromptlyWhileWaiting(t *testing.T) {
	var remaining atomic.Int64
	automation, providerService, _, appSettings := newQuotaAutomationServiceForTest(t, true, &remaining)
	if err := providerService.SaveProviders("codex", []Provider{{
		ID:                     403,
		Name:                   "Stopping Provider",
		APIURL:                 "https://api.deepseek.com/v1",
		APIKey:                 "test-key",
		Enabled:                false,
		QuotaAutoDisabled:      true,
		ProviderQuotaQueryType: string(ProviderQuotaQueryTypeBalance),
	}}); err != nil {
		t.Fatalf("保存停止测试供应商失败: %v", err)
	}

	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatalf("读取后台恢复设置失败: %v", err)
	}
	settings.QuotaRecoveryIntervalSeconds = maxQuotaRecoveryIntervalSeconds
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatalf("保存停止测试间隔失败: %v", err)
	}
	if err := automation.Start(); err != nil {
		t.Fatalf("启动额度恢复调度失败: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	startedAt := time.Now()
	if err := automation.Stop(); err != nil {
		t.Fatalf("停止额度恢复调度失败: %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("等待中的额度恢复调度停止过慢: %v", elapsed)
	}
	requireProviderQuotaState(t, providerService, 403, false, true, false)
}

func TestProviderQuotaRecoveryPassRestoresManagedStatesSerially(t *testing.T) {
	var remaining atomic.Int64
	remaining.Store(10)
	automation, providerService, _, _ := newQuotaAutomationServiceForTest(t, true, &remaining)
	providers := []Provider{
		{
			ID:                     501,
			Name:                   "Auto Disabled Provider",
			APIURL:                 "https://api.deepseek.com/v1",
			APIKey:                 "test-key",
			Enabled:                false,
			QuotaAutoDisabled:      true,
			ProviderQuotaQueryType: string(ProviderQuotaQueryTypeBalance),
		},
		{
			ID:                     502,
			Name:                   "Temporarily Enabled Provider",
			APIURL:                 "https://api.deepseek.com/v1",
			APIKey:                 "test-key",
			Enabled:                true,
			QuotaAutoDisablePaused: true,
			ProviderQuotaQueryType: string(ProviderQuotaQueryTypeBalance),
		},
	}
	if err := providerService.SaveProviders("codex", providers); err != nil {
		t.Fatalf("保存批量恢复供应商失败: %v", err)
	}

	snapshots, err := automation.listManagedProviderSnapshots()
	if err != nil {
		t.Fatalf("读取待恢复供应商失败: %v", err)
	}
	recoveredNames := automation.runRecoveryPass(snapshots)
	if len(recoveredNames) != 2 || recoveredNames[0] != providers[0].Name || recoveredNames[1] != providers[1].Name {
		t.Fatalf("合并恢复结果错误: %#v", recoveredNames)
	}
	requireProviderQuotaState(t, providerService, 501, true, false, false)
	requireProviderQuotaState(t, providerService, 502, true, false, false)
}

func TestProviderQuotaRecoveryNotificationBody(t *testing.T) {
	if body := providerQuotaRecoveryNotificationBody([]string{"Provider A"}); body != "Provider A 额度已恢复" {
		t.Fatalf("单供应商恢复通知错误: %s", body)
	}
	if body := providerQuotaRecoveryNotificationBody([]string{"Provider A", "Provider B"}); body != "2 个供应商额度已恢复" {
		t.Fatalf("多供应商恢复通知错误: %s", body)
	}
}

func TestProviderQuotaAutomationLifecycle(t *testing.T) {
	var remaining atomic.Int64
	automation, providerService, _, appSettings := newQuotaAutomationServiceForTest(t, true, &remaining)
	providers := []Provider{
		{
			ID:                     101,
			Name:                   "Auto Provider",
			APIURL:                 "https://api.deepseek.com/v1",
			APIKey:                 "test-key",
			Enabled:                true,
			ProviderQuotaQueryType: string(ProviderQuotaQueryTypeBalance),
		},
		{
			ID:                     102,
			Name:                   "Manual Provider",
			APIURL:                 "https://api.deepseek.com/v1",
			APIKey:                 "test-key",
			Enabled:                false,
			ProviderQuotaQueryType: string(ProviderQuotaQueryTypeBalance),
		},
	}
	if err := providerService.SaveProviders("codex", providers); err != nil {
		t.Fatalf("保存测试供应商失败: %v", err)
	}

	result := automation.CheckProviderQuota("codex", "101")
	if !result.Success || !result.StateChanged || !result.QuotaAutoDisabled {
		t.Fatalf("零余额应自动停用供应商: %#v", result)
	}
	requireProviderQuotaState(t, providerService, 101, false, true, false)

	automation.CheckProviderQuota("codex", "102")
	requireProviderQuotaState(t, providerService, 102, false, false, false)

	remaining.Store(10)
	automation.CheckProviderQuota("codex", "101")
	requireProviderQuotaState(t, providerService, 101, true, false, false)
	requireProviderQuotaState(t, providerService, 102, false, false, false)

	remaining.Store(0)
	automation.CheckProviderQuota("codex", "101")
	if _, err := automation.TemporarilyEnableProvider("codex", "101"); err != nil {
		t.Fatalf("临时启用供应商失败: %v", err)
	}
	requireProviderQuotaState(t, providerService, 101, true, false, true)
	automation.CheckProviderQuota("codex", "101")
	requireProviderQuotaState(t, providerService, 101, true, false, true)

	if _, err := automation.ResumeProviderQuotaAutomation("codex", "101"); err != nil {
		t.Fatalf("恢复自动停用失败: %v", err)
	}
	requireProviderQuotaState(t, providerService, 101, false, true, false)

	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatalf("读取自动停用设置失败: %v", err)
	}
	settings.ProviderQuotaAutoDisableEnabled = false
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatalf("关闭自动停用设置失败: %v", err)
	}
	requireProviderQuotaState(t, providerService, 101, true, false, false)
}

func TestProviderQuotaAutomationDoesNotReopenManuallyDisabledProvider(t *testing.T) {
	var remaining atomic.Int64
	automation, providerService, _, _ := newQuotaAutomationServiceForTest(t, true, &remaining)
	provider := Provider{
		ID:                     201,
		Name:                   "Manual Close Provider",
		APIURL:                 "https://api.deepseek.com/v1",
		APIKey:                 "test-key",
		Enabled:                true,
		ProviderQuotaQueryType: string(ProviderQuotaQueryTypeBalance),
	}
	if err := providerService.SaveProviders("codex", []Provider{provider}); err != nil {
		t.Fatalf("保存测试供应商失败: %v", err)
	}
	automation.CheckProviderQuota("codex", "201")

	providers, err := providerService.LoadProviders("codex")
	if err != nil {
		t.Fatalf("读取测试供应商失败: %v", err)
	}
	providers[0].Enabled = false
	providers[0].QuotaAutoDisabled = false
	providers[0].QuotaAutoDisablePaused = false
	if err := providerService.SaveProviders("codex", providers); err != nil {
		t.Fatalf("手动关闭供应商失败: %v", err)
	}

	remaining.Store(10)
	automation.CheckProviderQuota("codex", "201")
	requireProviderQuotaState(t, providerService, 201, false, false, false)
}

func TestProviderQuotaAutomationBindingRepairsDisabledSettingState(t *testing.T) {
	useIsolatedHomeDir(t)
	providerService := NewProviderService()
	if err := providerService.SaveProviders("codex", []Provider{{
		ID:                     301,
		Name:                   "Stale Auto State",
		APIURL:                 "https://api.deepseek.com/v1",
		APIKey:                 "test-key",
		Enabled:                false,
		ProviderQuotaQueryType: string(ProviderQuotaQueryTypeBalance),
		QuotaAutoDisabled:      true,
	}}); err != nil {
		t.Fatalf("保存残留自动停用状态失败: %v", err)
	}

	appSettings := NewAppSettingsService(nil)
	automation := NewProviderQuotaAutomationService(
		NewProviderQuotaQueryService(),
		providerService,
		NewGeminiService("127.0.0.1:18100"),
		NewOpenCodeService(),
		appSettings,
		nil,
		nil,
	)
	appSettings.BindProviderQuotaAutomationService(automation)
	requireProviderQuotaState(t, providerService, 301, true, false, false)
}

func TestProviderQuotaAutomationSyncsOpenCodeLiveConfig(t *testing.T) {
	var remaining atomic.Int64
	automation, _, openCodeService, _ := newQuotaAutomationServiceForTest(t, true, &remaining)
	provider := OpenCodeProvider{
		ID:                     "deepseek-managed",
		Name:                   "DeepSeek Managed",
		BaseURL:                "https://api.deepseek.com/v1",
		APIKey:                 "test-key",
		Enabled:                true,
		ProviderQuotaQueryType: string(ProviderQuotaQueryTypeBalance),
		SettingsConfig: map[string]any{
			"npm": "@ai-sdk/openai-compatible",
			"models": map[string]any{
				"deepseek-chat": map[string]any{"name": "DeepSeek Chat"},
			},
		},
	}
	if err := openCodeService.AddProvider(provider); err != nil {
		t.Fatalf("新增 OpenCode 测试供应商失败: %v", err)
	}

	automation.CheckProviderQuota("opencode", provider.ID)
	providers := openCodeService.GetProviders()
	if len(providers) != 1 || providers[0].Enabled || !providers[0].QuotaAutoDisabled {
		t.Fatalf("OpenCode 供应商未自动停用: %#v", providers)
	}
	liveProviders, err := readOpenCodeLiveProviders()
	if err != nil {
		t.Fatalf("读取 OpenCode live 配置失败: %v", err)
	}
	if liveProviders[provider.ID] != nil {
		t.Fatalf("自动停用后仍存在于 OpenCode live 配置: %#v", liveProviders)
	}

	remaining.Store(10)
	automation.CheckProviderQuota("opencode", provider.ID)
	providers = openCodeService.GetProviders()
	if len(providers) != 1 || !providers[0].Enabled || providers[0].QuotaAutoDisabled {
		t.Fatalf("OpenCode 供应商额度恢复后未启用: %#v", providers)
	}
	liveProviders, err = readOpenCodeLiveProviders()
	if err != nil {
		t.Fatalf("读取恢复后的 OpenCode live 配置失败: %v", err)
	}
	if liveProviders[provider.ID] == nil {
		t.Fatalf("额度恢复后未写回 OpenCode live 配置: %#v", liveProviders)
	}
}
