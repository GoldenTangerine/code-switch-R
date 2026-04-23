import { describe, expect, it } from 'vitest'
import type { ProviderDailyStat } from '../../../services/logs'
import { buildLogsCostDetailViewState } from './logsCostDetailModalState'

const createProviderDailyStat = (overrides: Partial<ProviderDailyStat> = {}): ProviderDailyStat => ({
  provider: 'provider-a',
  total_requests: 0,
  successful_requests: 0,
  failed_requests: 0,
  success_rate: 0,
  input_tokens: 0,
  output_tokens: 0,
  reasoning_tokens: 0,
  cache_create_tokens: 0,
  cache_read_tokens: 0,
  cost_total: 0,
  ...overrides,
})

const formatCurrency = (value?: number) => {
  if (value === undefined || value === null || Number.isNaN(value)) return '$0.0000'
  if (value >= 1) return `$${value.toFixed(2)}`
  if (value >= 0.01) return `$${value.toFixed(3)}`
  return `$${value.toFixed(4)}`
}

describe('buildLogsCostDetailViewState', () => {
  it('hides the summary card when the modal is in an error state', () => {
    const viewState = buildLogsCostDetailViewState({
      data: [createProviderDailyStat({ provider: 'provider-a', cost_total: 12.3 })],
      error: 'network down',
      formatCurrency,
    })

    expect(viewState.showSummary).toBe(false)
    expect(viewState.rows).toHaveLength(1)
  })

  it('aggregates total spend with shared precision rules for tiny values', () => {
    const viewState = buildLogsCostDetailViewState({
      data: [
        createProviderDailyStat({ provider: 'provider-a', cost_total: 0.0019 }),
        createProviderDailyStat({ provider: 'provider-b', cost_total: 0.0023 }),
      ],
      formatCurrency,
    })

    expect(viewState.totalAmount).toBeCloseTo(0.0042, 10)
    expect(viewState.totalAmountParts).toEqual({
      symbol: '$',
      whole: '0',
      fraction: '0042',
      formatted: '$0.0042',
    })
  })
})
