package services

import "testing"

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
