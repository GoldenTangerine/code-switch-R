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

func TestNormalizeHomeProviderTabsFiltersInvalidAndDuplicateValues(t *testing.T) {
	got := normalizeHomeProviderTabs([]string{" opencode ", "invalid", "others", "opencode", "gemini"})
	want := []string{"opencode", "others", "gemini"}

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
