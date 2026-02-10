package services

import (
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	BudgetUsageTimeLayout    = "2006-01-02 15:04:05"
	defaultBudgetRefreshTime = "00:00"
	budgetCycleModeDaily     = "daily"
	budgetCycleModeWeekly    = "weekly"
)

type BudgetUsageConfig struct {
	CycleEnabled bool
	CycleMode    string
	RefreshTime  string
	RefreshDay   int
}

func BuildBudgetUsageConfig(cycleEnabled bool, cycleMode string, refreshTime string, refreshDay int) BudgetUsageConfig {
	return BudgetUsageConfig{
		CycleEnabled: cycleEnabled,
		CycleMode:    normalizeBudgetCycleMode(cycleMode),
		RefreshTime:  normalizeBudgetRefreshTime(refreshTime),
		RefreshDay:   normalizeBudgetRefreshDay(refreshDay),
	}
}

func ResolveBudgetCycleStart(config BudgetUsageConfig, now time.Time) time.Time {
	normalized := BuildBudgetUsageConfig(
		config.CycleEnabled,
		config.CycleMode,
		config.RefreshTime,
		config.RefreshDay,
	)
	if !normalized.CycleEnabled {
		return startOfBudgetDay(now)
	}

	hour, minute := parseBudgetRefreshTime(normalized.RefreshTime)
	if normalized.CycleMode == budgetCycleModeWeekly {
		target := now
		diff := normalized.RefreshDay - int(target.Weekday())
		target = target.AddDate(0, 0, diff)
		target = time.Date(target.Year(), target.Month(), target.Day(), hour, minute, 0, 0, target.Location())
		if now.Before(target) {
			target = target.AddDate(0, 0, -7)
		}
		return target
	}

	start := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if now.Before(start) {
		start = start.AddDate(0, 0, -1)
	}
	return start
}

func ResolveBudgetRawUsed(logService *LogService, platform string, config BudgetUsageConfig, now time.Time) float64 {
	if logService == nil {
		return 0
	}

	normalized := BuildBudgetUsageConfig(
		config.CycleEnabled,
		config.CycleMode,
		config.RefreshTime,
		config.RefreshDay,
	)

	if normalized.CycleEnabled {
		cycleStart := ResolveBudgetCycleStart(normalized, now)
		used, err := logService.CostSince(cycleStart.Format(BudgetUsageTimeLayout), platform)
		if err != nil {
			return 0
		}
		return normalizeBudgetRawUsed(used)
	}

	stats, err := logService.StatsSince(platform)
	if err != nil {
		return 0
	}
	return normalizeBudgetRawUsed(stats.CostTotal)
}

func ResolveBudgetUsed(logService *LogService, platform string, config BudgetUsageConfig, adjustment float64, now time.Time) float64 {
	rawUsed := ResolveBudgetRawUsed(logService, platform, config, now)
	return ComputeBudgetUsed(rawUsed, adjustment)
}

func ComputeBudgetUsed(rawUsed float64, adjustment float64) float64 {
	normalizedRaw := normalizeBudgetRawUsed(rawUsed)
	normalizedAdjustment := normalizeBudgetAdjustment(adjustment)
	used := normalizedRaw + normalizedAdjustment
	if math.IsNaN(used) || math.IsInf(used, 0) {
		return 0
	}
	if used < 0 {
		return 0
	}
	return used
}

func normalizeBudgetCycleMode(value string) string {
	if strings.TrimSpace(strings.ToLower(value)) == budgetCycleModeWeekly {
		return budgetCycleModeWeekly
	}
	return budgetCycleModeDaily
}

func normalizeBudgetRefreshTime(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return defaultBudgetRefreshTime
	}
	return normalized
}

func normalizeBudgetRefreshDay(value int) int {
	if value < 0 {
		return 0
	}
	if value > 6 {
		return 6
	}
	return value
}

func parseBudgetRefreshTime(value string) (int, int) {
	parts := strings.Split(normalizeBudgetRefreshTime(value), ":")
	if len(parts) < 2 {
		return 0, 0
	}

	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		hour = 0
	}
	minute, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		minute = 0
	}

	if hour < 0 {
		hour = 0
	}
	if hour > 23 {
		hour = 23
	}
	if minute < 0 {
		minute = 0
	}
	if minute > 59 {
		minute = 59
	}
	return hour, minute
}

func startOfBudgetDay(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
}

func normalizeBudgetRawUsed(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value < 0 {
		return 0
	}
	return value
}

func normalizeBudgetAdjustment(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}
