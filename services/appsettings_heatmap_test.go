package services

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestAppSettingsSnapshotSaveAndExternalRefresh(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	service := NewAppSettingsService(nil)
	defer service.Stop()
	if err := service.Start(); err != nil {
		t.Fatal(err)
	}

	settings := service.defaultSettings()
	settings.EnableRoundRobin = true
	if _, err := service.SaveAppSettings(settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := service.GetAppSettings()
	if err != nil || !loaded.EnableRoundRobin {
		t.Fatalf("保存后快照未立即生效: %#v, %v", loaded, err)
	}

	if err := os.WriteFile(service.path, []byte(`{"enable_round_robin":`), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	loaded, err = service.GetAppSettings()
	if err != nil || !loaded.EnableRoundRobin {
		t.Fatalf("无效外部设置不应覆盖有效快照: %#v, %v", loaded, err)
	}

	settings.EnableRoundRobin = false
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(service.path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(deadline) {
		loaded, err = service.GetAppSettings()
		if err == nil && !loaded.EnableRoundRobin {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("外部设置未在一秒轮询周期内生效: %#v, %v", loaded, err)
}

func TestAppSettingsSnapshotDoesNotShareMutableFields(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	service := NewAppSettingsService(nil)
	defer service.Stop()

	settings := service.defaultSettings()
	settings.HomeProviderTabs = []string{"claude", "codex"}
	settings.ProviderConcurrencyLimits = map[string]bool{"claude": true}
	settings.ProviderQuotaQueryPresets = map[string]ProviderQuotaQueryPresetGroup{
		"custom": {
			DefaultID: "default",
			Items:     []ProviderQuotaQueryPresetEntry{{ID: "default", Name: "默认", Code: "return 1"}},
		},
	}
	if _, err := service.SaveAppSettings(settings); err != nil {
		t.Fatal(err)
	}

	settings.HomeProviderTabs[0] = "gemini"
	settings.ProviderConcurrencyLimits["claude"] = false
	modifiedPreset := settings.ProviderQuotaQueryPresets["custom"]
	modifiedPreset.Items[0].Code = "return 2"
	settings.ProviderQuotaQueryPresets["custom"] = modifiedPreset

	loaded, err := service.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.HomeProviderTabs[0] != "claude" || !loaded.ProviderConcurrencyLimits["claude"] {
		t.Fatalf("保存参数修改污染快照: %#v", loaded)
	}
	if loaded.ProviderQuotaQueryPresets["custom"].Items[0].Code != "return 1" {
		t.Fatalf("嵌套切片修改污染快照: %#v", loaded.ProviderQuotaQueryPresets)
	}

	loaded.HomeProviderTabs[0] = "opencode"
	loaded.ProviderConcurrencyLimits["claude"] = false
	loadedAgain, err := service.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	if loadedAgain.HomeProviderTabs[0] != "claude" || !loadedAgain.ProviderConcurrencyLimits["claude"] {
		t.Fatalf("读取结果修改污染快照: %#v", loadedAgain)
	}
}

func TestMainWindowDestroyDelayDefaultsAndNormalizes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	service := NewAppSettingsService(nil)
	defer service.Stop()

	settings := service.defaultSettings()
	if settings.MainWindowDestroyDelaySeconds != DefaultMainWindowDestroyDelaySeconds {
		t.Fatalf("默认销毁延迟 = %d, want %d", settings.MainWindowDestroyDelaySeconds, DefaultMainWindowDestroyDelaySeconds)
	}
	if err := json.Unmarshal([]byte(`{}`), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.MainWindowDestroyDelaySeconds != DefaultMainWindowDestroyDelaySeconds {
		t.Fatalf("旧配置缺失字段后的销毁延迟 = %d, want %d", settings.MainWindowDestroyDelaySeconds, DefaultMainWindowDestroyDelaySeconds)
	}

	tests := []struct {
		value int
		want  int
	}{
		{value: -1, want: minMainWindowDestroyDelaySeconds},
		{value: 0, want: 0},
		{value: 30, want: 30},
		{value: 301, want: maxMainWindowDestroyDelaySeconds},
	}
	for _, test := range tests {
		if got := normalizeMainWindowDestroyDelaySeconds(test.value); got != test.want {
			t.Fatalf("normalizeMainWindowDestroyDelaySeconds(%d) = %d, want %d", test.value, got, test.want)
		}
	}

	settings.MainWindowDestroyDelaySeconds = maxMainWindowDestroyDelaySeconds + 1
	saved, err := service.SaveAppSettings(settings)
	if err != nil {
		t.Fatal(err)
	}
	if saved.MainWindowDestroyDelaySeconds != maxMainWindowDestroyDelaySeconds {
		t.Fatalf("保存后的销毁延迟 = %d, want %d", saved.MainWindowDestroyDelaySeconds, maxMainWindowDestroyDelaySeconds)
	}

	saved, err = service.SetMainWindowDestroyDelay(45, 2)
	if err != nil {
		t.Fatal(err)
	}
	if saved.MainWindowDestroyDelaySeconds != 45 {
		t.Fatalf("专用接口保存后的销毁延迟 = %d, want 45", saved.MainWindowDestroyDelaySeconds)
	}
	stale, err := service.SetMainWindowDestroyDelay(10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if stale.MainWindowDestroyDelaySeconds != 45 {
		t.Fatalf("旧请求覆盖了新设置: %d", stale.MainWindowDestroyDelaySeconds)
	}
}

func TestNormalizeHomeProviderTabsDefaultsWhenEmpty(t *testing.T) {
	got := normalizeHomeProviderTabs(nil)
	want := []string{"claude", "codex", "gemini"}

	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %+v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("tab[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestLogsRefreshIntervalNormalizationAndDedicatedSave(t *testing.T) {
	for _, seconds := range []int{0, 5, 10, 30, 60} {
		if got := normalizeLogsRefreshIntervalSeconds(seconds); got != seconds {
			t.Fatalf("合法刷新间隔 %d 被改写为 %d", seconds, got)
		}
	}
	if got := normalizeLogsRefreshIntervalSeconds(15); got != defaultLogsRefreshIntervalSeconds {
		t.Fatalf("非法刷新间隔应回退为 %d，实际 %d", defaultLogsRefreshIntervalSeconds, got)
	}

	t.Setenv("HOME", t.TempDir())
	service := NewAppSettingsService(nil)
	settings := service.defaultSettings()
	settings.EnableRoundRobin = true
	if _, err := service.SaveAppSettings(settings); err != nil {
		t.Fatal(err)
	}
	updated, err := service.SetLogsRefreshInterval(5)
	if err != nil {
		t.Fatal(err)
	}
	if updated.LogsRefreshIntervalSeconds != 5 || !updated.EnableRoundRobin {
		t.Fatalf("专用保存不应覆盖其他设置: %+v", updated)
	}
}

func TestNormalizeHomeProviderTabsFiltersInvalidAndDuplicateValues(t *testing.T) {
	got := normalizeHomeProviderTabs([]string{
		" pi ",
		"invalid",
		"others",
		"opencode",
		"opencode",
		"grokbuild",
		"claude-desktop",
		"openclaw",
		"hermes",
		"gemini",
	})
	want := []string{"pi", "others", "opencode", "grokbuild", "claude-desktop", "openclaw", "hermes", "gemini"}

	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %+v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("tab[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestNormalizeProviderConcurrencyLimitsFiltersEmptyKeys(t *testing.T) {
	settings := AppSettings{
		ProviderConcurrencyLimits: map[string]bool{
			" claude ":      true,
			"custom:tool-a": true,
			" ":             true,
			"":              true,
			"codex":         false,
		},
	}

	normalizeProviderConcurrencyLimits(&settings)

	if settings.ProviderConcurrencyLimits == nil {
		t.Fatalf("ProviderConcurrencyLimits should be initialized")
	}
	if len(settings.ProviderConcurrencyLimits) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(settings.ProviderConcurrencyLimits), settings.ProviderConcurrencyLimits)
	}
	if !settings.ProviderConcurrencyLimits["claude"] {
		t.Fatalf("trimmed claude key should be enabled")
	}
	if !settings.ProviderConcurrencyLimits["custom:tool-a"] {
		t.Fatalf("custom tool key should be preserved")
	}
	if settings.ProviderConcurrencyLimits["codex"] {
		t.Fatalf("false switch state should be preserved")
	}
	if _, ok := settings.ProviderConcurrencyLimits[" "]; ok {
		t.Fatalf("blank key should be removed")
	}
	if _, ok := settings.ProviderConcurrencyLimits[""]; ok {
		t.Fatalf("empty key should be removed")
	}
}

func TestNormalizeProviderConcurrencyLimitsInitializesNilMap(t *testing.T) {
	settings := AppSettings{}

	normalizeProviderConcurrencyLimits(&settings)

	if settings.ProviderConcurrencyLimits == nil {
		t.Fatalf("ProviderConcurrencyLimits should not be nil")
	}
	if len(settings.ProviderConcurrencyLimits) != 0 {
		t.Fatalf("len = %d, want 0", len(settings.ProviderConcurrencyLimits))
	}
}

func TestNormalizeHeatmapDisplaySettingsIntensityMetricDefaultsToRequests(t *testing.T) {
	settings := AppSettings{
		HeatmapDailyScaleFactor:   defaultHeatmapDailyScale,
		HeatmapDailyIntensityMode: heatmapDailyModeHourlyScaled,
		HeatmapIntensityMetric:    "  totally-bogus  ",
		HeatmapIntensityStopL1:    defaultHeatmapIntensityL1,
		HeatmapIntensityStopL2:    defaultHeatmapIntensityL2,
		HeatmapIntensityStopL3:    defaultHeatmapIntensityL3,
	}

	normalizeHeatmapDisplaySettings(&settings)

	if settings.HeatmapIntensityMetric != heatmapIntensityMetricRequests {
		t.Fatalf("期望无效指标回退为 %q，实际 %q", heatmapIntensityMetricRequests, settings.HeatmapIntensityMetric)
	}
}

func TestNormalizeHeatmapDisplaySettingsPreservesKnownIntensityMetric(t *testing.T) {
	settings := AppSettings{
		HeatmapDailyScaleFactor:   defaultHeatmapDailyScale,
		HeatmapDailyIntensityMode: heatmapDailyModeHourlyScaled,
		HeatmapIntensityMetric:    heatmapIntensityMetricTotalTokens,
		HeatmapIntensityStopL1:    defaultHeatmapIntensityL1,
		HeatmapIntensityStopL2:    defaultHeatmapIntensityL2,
		HeatmapIntensityStopL3:    defaultHeatmapIntensityL3,
	}

	normalizeHeatmapDisplaySettings(&settings)

	if settings.HeatmapIntensityMetric != heatmapIntensityMetricTotalTokens {
		t.Fatalf("期望保留指标 %q，实际 %q", heatmapIntensityMetricTotalTokens, settings.HeatmapIntensityMetric)
	}
}
