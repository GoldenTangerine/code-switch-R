import { describe, expect, it } from 'vitest'
import { buildProviderCostTrend, toChartRgba } from './providerCostTrend'

describe('buildProviderCostTrend', () => {
  it('sorts by time and accumulates total_cost into cumulativeCost', () => {
    const trend = buildProviderCostTrend([
      { created_at: '2026-04-02 10:30:00', total_cost: 1.2 } as any,
      { created_at: '2026-04-02 09:00:00', total_cost: 0.8 } as any,
      { created_at: '2026-04-02 11:00:00', total_cost: 2.5 } as any,
    ])

    expect(trend.map((item) => item.time)).toEqual([
      '2026-04-02 09:00:00',
      '2026-04-02 10:30:00',
      '2026-04-02 11:00:00',
    ])
    expect(trend.map((item) => item.cumulativeCost)).toEqual([0.8, 2, 4.5])
  })

  it('keeps zero-cost records and ignores invalid numeric input', () => {
    const trend = buildProviderCostTrend([
      { created_at: '2026-04-02 09:00:00', total_cost: 'bad-value' } as any,
      { created_at: '2026-04-02 09:10:00', total_cost: 0 } as any,
      { created_at: '2026-04-02 09:20:00', total_cost: 0.35 } as any,
    ])

    expect(trend.map((item) => item.cost)).toEqual([0, 0, 0.35])
    expect(trend.map((item) => item.cumulativeCost)).toEqual([0, 0, 0.35])
  })

  it('normalizes chart colors safely for area gradients', () => {
    expect(toChartRgba('#0a84ff', 0.32)).toBe('rgba(10, 132, 255, 0.32)')
    expect(toChartRgba('#0aff5cff', 0.5)).toBe('rgba(10, 255, 92, 0.5)')
    expect(toChartRgba('#0a84ff80', 0.5)).toBe('rgba(10, 132, 255, 0.251)')
    expect(toChartRgba('rgba(12, 34, 56, 0.4)', 0.5)).toBe('rgba(12, 34, 56, 0.2)')
    expect(toChartRgba('oops', 0.5)).toBe('rgba(37, 99, 235, 0.5)')
  })
})
