import { describe, expect, it } from 'vitest'
import {
  buildBudgetUsageConfig,
  normalizeBudgetQuotaSettings,
  resolveBudgetQuotaWindow,
  resolveCycleStart,
  resolveNextCycleStart,
} from './budgetUsage'

describe('budgetUsage', () => {
  it('normalizes monthly refresh config values', () => {
    const config = buildBudgetUsageConfig(true, ' monthly ', ' ', 9, 99)

    expect(config.cycleEnabled).toBe(true)
    expect(config.cycleMode).toBe('monthly')
    expect(config.refreshTime).toBe('00:00')
    expect(config.refreshWeekday).toBe(6)
    expect(config.refreshMonthDay).toBe(31)
  })

  it('uses the previous month when the current monthly refresh point is still in the future', () => {
    const config = buildBudgetUsageConfig(true, 'monthly', '06:45', 1, 31)
    const now = new Date(2026, 2, 31, 6, 30, 0, 0)

    expect(resolveCycleStart(config, now).getTime()).toBe(new Date(2026, 1, 28, 6, 45, 0, 0).getTime())
  })

  it('resolves the next monthly cycle using the configured day with month-end clamping', () => {
    const config = buildBudgetUsageConfig(true, 'monthly', '06:45', 1, 31)
    const currentStart = new Date(2026, 0, 31, 6, 45, 0, 0)

    expect(resolveNextCycleStart(config, currentStart).getTime()).toBe(
      new Date(2026, 1, 28, 6, 45, 0, 0).getTime(),
    )
  })

  it('keeps weekly cycle boundaries unchanged', () => {
    const config = buildBudgetUsageConfig(true, 'weekly', '06:45', 1, 1)
    const now = new Date(2026, 2, 10, 8, 0, 0, 0)

    expect(resolveCycleStart(config, now).getTime()).toBe(new Date(2026, 2, 9, 6, 45, 0, 0).getTime())
    expect(resolveNextCycleStart(config, new Date(2026, 2, 9, 6, 45, 0, 0)).getTime()).toBe(
      new Date(2026, 2, 16, 6, 45, 0, 0).getTime(),
    )
  })

  it('migrates legacy single budget settings into the matching quota slot', () => {
    const quotas = normalizeBudgetQuotaSettings(undefined, {
      total: 42,
      cycleEnabled: true,
      cycleMode: 'weekly',
      refreshTime: '08:15',
      refreshWeekday: 5,
      refreshMonthDay: 20,
    })

    expect(quotas.weekly.total).toBe(42)
    expect(quotas.weekly.refreshTime).toBe('08:15')
    expect(quotas.weekly.refreshWeekday).toBe(5)
    expect(quotas.daily.total).toBe(0)
  })

  it('accepts snake_case quota refresh fields from persisted settings', () => {
    const quotas = normalizeBudgetQuotaSettings({
      daily: {
        total: 18,
        refresh_time: '08:15',
        refresh_day: 4,
        refresh_month_day: 21,
      },
    })

    expect(quotas.daily.total).toBe(18)
    expect(quotas.daily.refreshTime).toBe('08:15')
    expect(quotas.daily.refreshWeekday).toBe(4)
    expect(quotas.daily.refreshMonthDay).toBe(21)
  })

  it('uses a rolling 5 hour window for the 5 hour quota', () => {
    const now = new Date(2026, 2, 10, 12, 34, 0, 0)
    const window = resolveBudgetQuotaWindow('five_hour', {
      total: 12,
      refreshTime: '00:00',
      refreshWeekday: 1,
      refreshMonthDay: 1,
    }, now)

    expect(window.start.getTime()).toBe(new Date(2026, 2, 10, 7, 34, 0, 0).getTime())
    expect(window.nextReset).toBeNull()
  })
})
