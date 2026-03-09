package services

import (
	"math"
	"testing"
	"time"
)

func TestBuildBudgetUsageConfig(t *testing.T) {
	t.Run("normalizes weekly config", func(t *testing.T) {
		cfg := BuildBudgetUsageConfig(true, " weekly ", " ", 9, 99)
		if !cfg.CycleEnabled {
			t.Fatal("CycleEnabled should remain true")
		}
		if cfg.CycleMode != "weekly" {
			t.Fatalf("CycleMode = %q, want weekly", cfg.CycleMode)
		}
		if cfg.RefreshTime != "00:00" {
			t.Fatalf("RefreshTime = %q, want 00:00", cfg.RefreshTime)
		}
		if cfg.RefreshWeekday != 6 {
			t.Fatalf("RefreshWeekday = %d, want 6", cfg.RefreshWeekday)
		}
		if cfg.RefreshMonthDay != 31 {
			t.Fatalf("RefreshMonthDay = %d, want 31", cfg.RefreshMonthDay)
		}
	})

	t.Run("falls back to daily and valid refresh ranges", func(t *testing.T) {
		cfg := BuildBudgetUsageConfig(false, "something", "06:30", -3, 0)
		if cfg.CycleMode != "daily" {
			t.Fatalf("CycleMode = %q, want daily", cfg.CycleMode)
		}
		if cfg.RefreshWeekday != 0 {
			t.Fatalf("RefreshWeekday = %d, want 0", cfg.RefreshWeekday)
		}
		if cfg.RefreshMonthDay != 1 {
			t.Fatalf("RefreshMonthDay = %d, want 1", cfg.RefreshMonthDay)
		}
	})
}

func TestResolveBudgetCycleStartDaily(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	cfg := BuildBudgetUsageConfig(true, "daily", "06:45", 1, 1)

	nowBeforeRefresh := time.Date(2026, 2, 10, 5, 0, 0, 0, loc)
	gotBefore := ResolveBudgetCycleStart(cfg, nowBeforeRefresh)
	wantBefore := time.Date(2026, 2, 9, 6, 45, 0, 0, loc)
	if !gotBefore.Equal(wantBefore) {
		t.Fatalf("before refresh start = %s, want %s", gotBefore, wantBefore)
	}

	nowAfterRefresh := time.Date(2026, 2, 10, 8, 0, 0, 0, loc)
	gotAfter := ResolveBudgetCycleStart(cfg, nowAfterRefresh)
	wantAfter := time.Date(2026, 2, 10, 6, 45, 0, 0, loc)
	if !gotAfter.Equal(wantAfter) {
		t.Fatalf("after refresh start = %s, want %s", gotAfter, wantAfter)
	}
}

func TestResolveBudgetCycleStartWeekly(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	cfg := BuildBudgetUsageConfig(true, "weekly", "06:45", 1, 1) // Monday

	nowBeforeThisWeekTarget := time.Date(2026, 2, 8, 8, 0, 0, 0, loc) // Sunday
	gotBefore := ResolveBudgetCycleStart(cfg, nowBeforeThisWeekTarget)
	wantBefore := time.Date(2026, 2, 2, 6, 45, 0, 0, loc)
	if !gotBefore.Equal(wantBefore) {
		t.Fatalf("weekly before target start = %s, want %s", gotBefore, wantBefore)
	}

	nowAfterThisWeekTarget := time.Date(2026, 2, 10, 8, 0, 0, 0, loc) // Tuesday
	gotAfter := ResolveBudgetCycleStart(cfg, nowAfterThisWeekTarget)
	wantAfter := time.Date(2026, 2, 9, 6, 45, 0, 0, loc)
	if !gotAfter.Equal(wantAfter) {
		t.Fatalf("weekly after target start = %s, want %s", gotAfter, wantAfter)
	}
}

func TestResolveBudgetCycleStartMonthly(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	cfg := BuildBudgetUsageConfig(true, "monthly", "06:45", 1, 31)

	nowBeforeThisMonthTarget := time.Date(2026, time.March, 31, 6, 30, 0, 0, loc)
	gotBefore := ResolveBudgetCycleStart(cfg, nowBeforeThisMonthTarget)
	wantBefore := time.Date(2026, time.February, 28, 6, 45, 0, 0, loc)
	if !gotBefore.Equal(wantBefore) {
		t.Fatalf("monthly before target start = %s, want %s", gotBefore, wantBefore)
	}

	nowAfterThisMonthTarget := time.Date(2026, time.March, 31, 8, 0, 0, 0, loc)
	gotAfter := ResolveBudgetCycleStart(cfg, nowAfterThisMonthTarget)
	wantAfter := time.Date(2026, time.March, 31, 6, 45, 0, 0, loc)
	if !gotAfter.Equal(wantAfter) {
		t.Fatalf("monthly after target start = %s, want %s", gotAfter, wantAfter)
	}
}

func TestResolveBudgetCycleStartWhenCycleDisabled(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	cfg := BuildBudgetUsageConfig(false, "weekly", "06:45", 1, 1)
	now := time.Date(2026, 2, 10, 17, 30, 0, 0, loc)
	got := ResolveBudgetCycleStart(cfg, now)
	want := time.Date(2026, 2, 10, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("cycle disabled start = %s, want %s", got, want)
	}
}

func TestParseBudgetRefreshTime(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		hour   int
		minute int
	}{
		{name: "normal", input: "09:30", hour: 9, minute: 30},
		{name: "clamped high", input: "99:120", hour: 23, minute: 59},
		{name: "clamped low", input: "-1:-2", hour: 0, minute: 0},
		{name: "invalid", input: "oops", hour: 0, minute: 0},
		{name: "empty", input: "", hour: 0, minute: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hour, minute := parseBudgetRefreshTime(tt.input)
			if hour != tt.hour || minute != tt.minute {
				t.Fatalf("parseBudgetRefreshTime(%q) = (%d,%d), want (%d,%d)", tt.input, hour, minute, tt.hour, tt.minute)
			}
		})
	}
}

func TestComputeBudgetUsed(t *testing.T) {
	tests := []struct {
		name       string
		rawUsed    float64
		adjustment float64
		want       float64
	}{
		{name: "normal add", rawUsed: 10.5, adjustment: 2.25, want: 12.75},
		{name: "negative result clamps to zero", rawUsed: 1.0, adjustment: -3.0, want: 0},
		{name: "raw nan keeps adjustment", rawUsed: math.NaN(), adjustment: 2.0, want: 2.0},
		{name: "adjustment inf ignored", rawUsed: 3.0, adjustment: math.Inf(1), want: 3.0},
		{name: "raw inf ignored", rawUsed: math.Inf(1), adjustment: 1.5, want: 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeBudgetUsed(tt.rawUsed, tt.adjustment)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("ComputeBudgetUsed(%v, %v) = %v, want %v", tt.rawUsed, tt.adjustment, got, tt.want)
			}
		})
	}
}

func TestResolveBudgetRawUsedNilLogService(t *testing.T) {
	cfg := BuildBudgetUsageConfig(true, "weekly", "06:45", 1, 1)
	got := ResolveBudgetRawUsed(nil, "claude", cfg, time.Now())
	if got != 0 {
		t.Fatalf("ResolveBudgetRawUsed(nil, ...) = %v, want 0", got)
	}
}
