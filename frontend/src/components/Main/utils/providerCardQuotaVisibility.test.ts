import { describe, expect, it } from 'vitest'
import type { ProviderQuotaDisplayItem } from '../types'
import { resolveProviderCardQuotaSectionMode } from './providerCardQuotaVisibility'

describe('resolveProviderCardQuotaSectionMode', () => {
  it('returns hidden when no quota items exist', () => {
    expect(resolveProviderCardQuotaSectionMode({ state: 'ready' }, [])).toBe('hidden')
    expect(resolveProviderCardQuotaSectionMode({ state: 'empty' }, [])).toBe('hidden')
  })

  it('keeps quota inline with performance metrics when stats are ready', () => {
    expect(resolveProviderCardQuotaSectionMode({
      state: 'ready',
    }, [
      {
        key: 'daily',
        label: 'Daily',
        used: 1,
        total: 10,
        progressRatio: 0.1,
        countdownLabel: '1h0m',
        nextReset: null,
        valueMode: 'count',
      },
    ])).toBe('inline-with-performance')
  })

  it('shows quota as standalone row when stats are empty or loading', () => {
    const quotaDisplay: ProviderQuotaDisplayItem[] = [
      {
        key: 'weekly',
        label: 'Weekly',
        used: 5,
        total: 10,
        progressRatio: 0.5,
        countdownLabel: '2d0h',
        nextReset: null,
        valueMode: 'count',
      },
    ]

    expect(resolveProviderCardQuotaSectionMode({ state: 'empty' }, quotaDisplay)).toBe('standalone')
    expect(resolveProviderCardQuotaSectionMode({ state: 'loading' }, quotaDisplay)).toBe('standalone')
  })
})
