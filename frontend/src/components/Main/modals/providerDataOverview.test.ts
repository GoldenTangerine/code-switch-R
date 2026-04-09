import { describe, expect, it } from 'vitest'
import {
  buildProviderOverviewDays,
  buildProviderOverviewFallbackRows,
  buildProviderOverviewRange,
  sumLogStatsTokens,
} from './providerDataOverview'

describe('providerDataOverview', () => {
  it('fills missing days in the requested range with zero values', () => {
    const points = buildProviderOverviewDays({
      startDate: new Date('2026-04-03T10:00:00'),
      days: 4,
      series: [
        {
          day: '2026-04-03',
          total_requests: 2,
          input_tokens: 100,
          output_tokens: 50,
          reasoning_tokens: 10,
          cache_create_tokens: 0,
          cache_read_tokens: 5,
          total_cost: 1.25,
        },
        {
          day: '2026-04-05',
          total_requests: 1,
          input_tokens: 20,
          output_tokens: 15,
          reasoning_tokens: 0,
          cache_create_tokens: 0,
          cache_read_tokens: 0,
          total_cost: 0.42,
        },
      ],
    })

    expect(points).toEqual([
      expect.objectContaining({ dayKey: '2026-04-03', requests: 2, totalTokens: 165, cost: 1.25 }),
      expect.objectContaining({ dayKey: '2026-04-04', requests: 0, totalTokens: 0, cost: 0 }),
      expect.objectContaining({ dayKey: '2026-04-05', requests: 1, totalTokens: 35, cost: 0.42 }),
      expect.objectContaining({ dayKey: '2026-04-06', requests: 0, totalTokens: 0, cost: 0 }),
    ])
  })

  it('computes the rolling date range in local time', () => {
    const range = buildProviderOverviewRange(7, new Date('2026-04-09T16:00:00'))

    expect(range.startAt).toBe('2026-04-03 00:00:00')
    expect(range.endAt).toBe('2026-04-10 00:00:00')
  })

  it('builds fallback rows with the latest day first and respects the requested limit', () => {
    const rows = buildProviderOverviewFallbackRows([
      {
        dayKey: '2026-04-03',
        label: '4/3',
        timestamp: new Date('2026-04-03T00:00:00').getTime(),
        cost: 1.25,
        requests: 2,
        totalTokens: 165,
      },
      {
        dayKey: '2026-04-04',
        label: '4/4',
        timestamp: new Date('2026-04-04T00:00:00').getTime(),
        cost: 0.8,
        requests: 1,
        totalTokens: 120,
      },
      {
        dayKey: '2026-04-05',
        label: '4/5',
        timestamp: new Date('2026-04-05T00:00:00').getTime(),
        cost: 0.42,
        requests: 1,
        totalTokens: 35,
      },
    ], 2)

    expect(rows.map((row) => row.dayKey)).toEqual(['2026-04-05', '2026-04-04'])
  })

  it('sums token buckets from aggregated stats safely', () => {
    expect(sumLogStatsTokens({
      input_tokens: 100,
      output_tokens: 80,
      reasoning_tokens: 20,
      cache_create_tokens: 10,
      cache_read_tokens: 5,
    })).toBe(215)
  })
})
