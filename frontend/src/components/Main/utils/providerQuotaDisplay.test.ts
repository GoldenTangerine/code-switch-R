import { describe, expect, it } from 'vitest'
import type { ProviderQuotaDisplayItem } from '../types'
import { formatQuotaUsagePercent, getQuotaProgressClass, getQuotaProgressPercent, getQuotaUsagePercentValue } from './providerQuotaDisplay'

const createQuotaItem = (progressRatio: number): ProviderQuotaDisplayItem => ({
  key: 'daily',
  label: '24h',
  used: 44,
  total: 100,
  progressRatio,
  countdownLabel: '5h12m',
  nextReset: null,
})

describe('providerQuotaDisplay', () => {
  it('keeps fill percent aligned with usage ratio instead of overflowing the bar', () => {
    const item = createQuotaItem(0.44)

    expect(getQuotaUsagePercentValue(item)).toBe(44)
    expect(getQuotaProgressPercent(item)).toBe(44)
    expect(formatQuotaUsagePercent(item)).toBe('44%')
    expect(getQuotaProgressClass(item)).toBe('quota-progress--ok')
  })

  it('clamps visual fill at 100 percent but keeps over-limit state', () => {
    const item = createQuotaItem(1.38)

    expect(getQuotaUsagePercentValue(item)).toBe(138)
    expect(getQuotaProgressPercent(item)).toBe(100)
    expect(formatQuotaUsagePercent(item)).toBe('138%')
    expect(getQuotaProgressClass(item)).toBe('quota-progress--over')
  })

  it('formats tiny usage as less than one percent', () => {
    const item = createQuotaItem(0.004)

    expect(getQuotaProgressPercent(item)).toBe(0.4)
    expect(formatQuotaUsagePercent(item)).toBe('<1%')
  })
})
