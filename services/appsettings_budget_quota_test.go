package services

import (
	"encoding/json"
	"testing"
)

func TestBudgetQuotaSettingUnmarshalJSONSupportsSnakeAndCamelCase(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    BudgetQuotaSetting
	}{
		{
			name: "snake_case",
			payload: `{
				"total": 42,
				"refresh_time": "08:15",
				"refresh_day": 5,
				"refresh_month_day": 20
			}`,
			want: BudgetQuotaSetting{
				Total:           42,
				RefreshTime:     "08:15",
				RefreshDay:      5,
				RefreshMonthDay: 20,
			},
		},
		{
			name: "camel_case_from_frontend",
			payload: `{
				"total": 18,
				"refreshTime": "06:30",
				"refreshWeekday": 2,
				"refreshMonthDay": 11
			}`,
			want: BudgetQuotaSetting{
				Total:           18,
				RefreshTime:     "06:30",
				RefreshDay:      2,
				RefreshMonthDay: 11,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got BudgetQuotaSetting
			if err := json.Unmarshal([]byte(tt.payload), &got); err != nil {
				t.Fatalf("反序列化失败: %v", err)
			}
			if got.Total != tt.want.Total {
				t.Fatalf("Total = %v, want %v", got.Total, tt.want.Total)
			}
			if got.RefreshTime != tt.want.RefreshTime {
				t.Fatalf("RefreshTime = %q, want %q", got.RefreshTime, tt.want.RefreshTime)
			}
			if got.RefreshDay != tt.want.RefreshDay {
				t.Fatalf("RefreshDay = %d, want %d", got.RefreshDay, tt.want.RefreshDay)
			}
			if got.RefreshMonthDay != tt.want.RefreshMonthDay {
				t.Fatalf("RefreshMonthDay = %d, want %d", got.RefreshMonthDay, tt.want.RefreshMonthDay)
			}
		})
	}
}

func TestNormalizeBudgetQuotaUsedAdjustmentsMigratesLegacyAdjustmentOnlyToMatchingMode(t *testing.T) {
	got := normalizeBudgetQuotaUsedAdjustments(BudgetQuotaAdjustments{}, 2.75, true, budgetCycleModeWeekly)

	if got.FiveHour != 0 || got.Daily != 0 || got.Weekly != 2.75 || got.Monthly != 0 || got.Total != 0 {
		t.Fatalf("期望旧 adjustment 只迁移到 weekly 额度，实际 = %+v", got)
	}
}

func TestNormalizeBudgetQuotaUsedAdjustmentsPreservesPerQuotaValues(t *testing.T) {
	got := normalizeBudgetQuotaUsedAdjustments(BudgetQuotaAdjustments{
		FiveHour: 1.5,
		Daily:    -2,
		Weekly:   0,
		Monthly:  4.25,
	}, 9, true, budgetCycleModeMonthly)

	if got.FiveHour != 1.5 {
		t.Fatalf("FiveHour = %v, want 1.5", got.FiveHour)
	}
	if got.Daily != -2 {
		t.Fatalf("Daily = %v, want -2", got.Daily)
	}
	if got.Weekly != 0 {
		t.Fatalf("Weekly = %v, want 0", got.Weekly)
	}
	if got.Monthly != 4.25 {
		t.Fatalf("Monthly = %v, want 4.25", got.Monthly)
	}
	if got.Total != 0 {
		t.Fatalf("Total = %v, want 0", got.Total)
	}
}

func TestNormalizeBudgetSettingsMigratesLegacySingleQuotaIntoMatchingSlot(t *testing.T) {
	settings := AppSettings{
		BudgetTotal:           42,
		BudgetUsedAdjustment:  3.5,
		BudgetCycleEnabled:    true,
		BudgetCycleMode:       budgetCycleModeWeekly,
		BudgetRefreshTime:     "08:15",
		BudgetRefreshDay:      5,
		BudgetRefreshMonthDay: 20,
	}

	normalizeBudgetSettings(&settings)

	if settings.BudgetQuotaSettings.Weekly.Total != 42 {
		t.Fatalf("Weekly.Total = %v, want 42", settings.BudgetQuotaSettings.Weekly.Total)
	}
	if settings.BudgetQuotaSettings.Daily.Total != 0 || settings.BudgetQuotaSettings.Monthly.Total != 0 || settings.BudgetQuotaSettings.FiveHour.Total != 0 || settings.BudgetQuotaSettings.Total.Total != 0 {
		t.Fatalf("期望仅 weekly 被迁移，实际 quota settings = %+v", settings.BudgetQuotaSettings)
	}
	if settings.BudgetQuotaUsedAdjustments.Weekly != 3.5 {
		t.Fatalf("Weekly adjustment = %v, want 3.5", settings.BudgetQuotaUsedAdjustments.Weekly)
	}
	if settings.BudgetQuotaUsedAdjustments.Daily != 0 || settings.BudgetQuotaUsedAdjustments.Monthly != 0 || settings.BudgetQuotaUsedAdjustments.FiveHour != 0 || settings.BudgetQuotaUsedAdjustments.Total != 0 {
		t.Fatalf("期望仅 weekly adjustment 被迁移，实际 = %+v", settings.BudgetQuotaUsedAdjustments)
	}
	if settings.BudgetTotal != 42 || settings.BudgetUsedAdjustment != 3.5 || !settings.BudgetCycleEnabled || settings.BudgetCycleMode != budgetCycleModeWeekly {
		t.Fatalf("legacy 字段投影异常: %+v", settings)
	}
}

func TestNormalizeBudgetSettingsProjectsMultiQuotaBackToLegacyFields(t *testing.T) {
	settings := AppSettings{
		BudgetQuotaSettings: BudgetQuotaSettings{
			FiveHour: BudgetQuotaSetting{Total: 9, RefreshTime: "00:00", RefreshDay: 1, RefreshMonthDay: 1},
			Daily:    BudgetQuotaSetting{Total: 12, RefreshTime: "07:30", RefreshDay: 2, RefreshMonthDay: 5},
			Weekly:   BudgetQuotaSetting{Total: 48, RefreshTime: "08:15", RefreshDay: 5, RefreshMonthDay: 10},
			Monthly:  BudgetQuotaSetting{Total: 128, RefreshTime: "09:45", RefreshDay: 3, RefreshMonthDay: 20},
			Total:    BudgetQuotaSetting{Total: 512, RefreshTime: "00:00", RefreshDay: 1, RefreshMonthDay: 1},
		},
		BudgetQuotaUsedAdjustments: BudgetQuotaAdjustments{
			FiveHour: 1,
			Daily:    2.5,
			Weekly:   3.5,
			Monthly:  4.5,
			Total:    9,
		},
	}

	normalizeBudgetSettings(&settings)

	if settings.BudgetTotal != 12 {
		t.Fatalf("BudgetTotal = %v, want 12", settings.BudgetTotal)
	}
	if settings.BudgetUsedAdjustment != 2.5 {
		t.Fatalf("BudgetUsedAdjustment = %v, want 2.5", settings.BudgetUsedAdjustment)
	}
	if !settings.BudgetCycleEnabled {
		t.Fatalf("BudgetCycleEnabled = false, want true")
	}
	if settings.BudgetCycleMode != budgetCycleModeDaily {
		t.Fatalf("BudgetCycleMode = %q, want %q", settings.BudgetCycleMode, budgetCycleModeDaily)
	}
	if settings.BudgetRefreshTime != "07:30" || settings.BudgetRefreshDay != 2 || settings.BudgetRefreshMonthDay != 5 {
		t.Fatalf("legacy 刷新字段投影异常: time=%q day=%d monthDay=%d", settings.BudgetRefreshTime, settings.BudgetRefreshDay, settings.BudgetRefreshMonthDay)
	}
}

func TestNormalizeBudgetSettingsClearsLegacyFieldsWhenOnlyFiveHourQuotaIsConfigured(t *testing.T) {
	settings := AppSettings{
		BudgetQuotaSettings: BudgetQuotaSettings{
			FiveHour: BudgetQuotaSetting{Total: 9, RefreshTime: "00:00", RefreshDay: 1, RefreshMonthDay: 1},
			Total:    BudgetQuotaSetting{Total: 240, RefreshTime: "00:00", RefreshDay: 1, RefreshMonthDay: 1},
		},
		BudgetQuotaUsedAdjustments: BudgetQuotaAdjustments{
			FiveHour: 1.5,
			Total:    4,
		},
		BudgetTotal:           88,
		BudgetUsedAdjustment:  6.25,
		BudgetCycleEnabled:    true,
		BudgetCycleMode:       budgetCycleModeMonthly,
		BudgetRefreshTime:     "11:30",
		BudgetRefreshDay:      6,
		BudgetRefreshMonthDay: 28,
	}

	normalizeBudgetSettings(&settings)

	if settings.BudgetTotal != 0 || settings.BudgetUsedAdjustment != 0 {
		t.Fatalf("legacy 数值字段应清零，实际 total=%v adjustment=%v", settings.BudgetTotal, settings.BudgetUsedAdjustment)
	}
	if settings.BudgetCycleEnabled {
		t.Fatalf("BudgetCycleEnabled = true, want false")
	}
	if settings.BudgetCycleMode != budgetCycleModeDaily || settings.BudgetRefreshTime != "00:00" || settings.BudgetRefreshDay != defaultBudgetRefreshWeekday || settings.BudgetRefreshMonthDay != defaultBudgetRefreshMonthDay {
		t.Fatalf("legacy 默认字段异常: mode=%q time=%q day=%d monthDay=%d", settings.BudgetCycleMode, settings.BudgetRefreshTime, settings.BudgetRefreshDay, settings.BudgetRefreshMonthDay)
	}
}
