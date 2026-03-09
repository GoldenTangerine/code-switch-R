import { describe, expect, it } from 'vitest'
import {
  buildBudgetUsageConfig,
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
})
