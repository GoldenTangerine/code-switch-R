import { describe, expect, it } from 'vitest'
import {
  formatClockCountdown,
  shouldUseSecondPrecisionTrayTicker,
  updateItemsAndCollectRefresh,
} from './trayCountdown'

describe('trayCountdown', () => {
  it('keeps positive countdowns from showing a premature zero clock', () => {
    expect(formatClockCountdown(5 * 60 * 60 * 1000)).toBe('05:00:00')
    expect(formatClockCountdown(999)).toBe('00:00:01')
    expect(formatClockCountdown(0)).toBe('00:00:00')
  })

  it('only enables second precision ticker for visible 5 hour countdowns', () => {
    const quotas = [
      { key: 'five_hour' as const, hasBudget: true, nextReset: new Date('2026-03-10T10:00:00Z') },
      { key: 'daily' as const, hasBudget: true, nextReset: new Date('2026-03-11T00:00:00Z') },
    ]

    expect(shouldUseSecondPrecisionTrayTicker('quotas', true, quotas)).toBe(true)
    expect(shouldUseSecondPrecisionTrayTicker('provider-quotas', true, quotas)).toBe(true)
    expect(shouldUseSecondPrecisionTrayTicker('summary', true, quotas)).toBe(false)
    expect(shouldUseSecondPrecisionTrayTicker('quotas', false, quotas)).toBe(false)
    expect(shouldUseSecondPrecisionTrayTicker('quotas', true, [{ ...quotas[0], key: 'daily' }])).toBe(false)
  })

  it('updates every item before returning whether a refresh is needed', () => {
    const visited: number[] = []

    const shouldRefresh = updateItemsAndCollectRefresh([1, 2, 3], (item) => {
      visited.push(item)
      return item !== 2
    })

    expect(visited).toEqual([1, 2, 3])
    expect(shouldRefresh).toBe(true)
  })
})
