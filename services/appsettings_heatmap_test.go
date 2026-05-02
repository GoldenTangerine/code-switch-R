package services

import "testing"

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
