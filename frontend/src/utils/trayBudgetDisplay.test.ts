import { describe, expect, it } from 'vitest'
import { createDefaultBudgetQuotaSettings } from './budgetUsage'
import {
  getVisibleTrayQuotaKeys,
  resolveTrayBudgetDisplayMode,
} from './trayBudgetDisplay'

describe('trayBudgetDisplay', () => {
  it('uses summary mode when all quotas are infinite', () => {
    const settings = createDefaultBudgetQuotaSettings()

    expect(resolveTrayBudgetDisplayMode(settings)).toBe('summary')
    expect(getVisibleTrayQuotaKeys(settings)).toEqual([])
  })

  it('only keeps finite quotas visible in mixed settings', () => {
    const settings = createDefaultBudgetQuotaSettings()
    settings.five_hour.total = 96
    settings.weekly.total = 324

    expect(resolveTrayBudgetDisplayMode(settings)).toBe('quotas')
    expect(getVisibleTrayQuotaKeys(settings)).toEqual(['five_hour', 'weekly'])
  })

  it('ignores provider-only total quota settings in tray mode selection', () => {
    const settings = createDefaultBudgetQuotaSettings()
    settings.total.total = 512

    expect(resolveTrayBudgetDisplayMode(settings)).toBe('summary')
    expect(getVisibleTrayQuotaKeys(settings)).toEqual([])
  })
})
