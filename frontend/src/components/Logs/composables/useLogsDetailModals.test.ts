import { describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import type { LogsFiltersState } from '../types'

vi.mock('../../../services/logs', () => ({
  fetchProviderStatsV2: vi.fn(),
}))

import { buildFrontendDevLogsCostDetailMockData, resolveLogsCostDetailStats } from './useLogsDetailModals'

const createFilters = (overrides: Partial<LogsFiltersState> = {}): LogsFiltersState =>
  reactive({
    platform: '',
    provider: '',
    dateType: 'all',
    year: '',
    month: '',
    day: '',
    rangeStart: '',
    rangeEnd: '',
    ...overrides,
  })

describe('buildFrontendDevLogsCostDetailMockData', () => {
  it('returns the full demo provider list when no provider filter is applied', () => {
    const rows = buildFrontendDevLogsCostDetailMockData(createFilters())

    expect(rows.map(item => item.provider)).toEqual([
      '自建-sub2api',
      '智谱GLM-5.1',
      '公司千问',
      '豆包',
    ])
  })

  it('filters the demo rows by provider name or id', () => {
    const rows = buildFrontendDevLogsCostDetailMockData(createFilters({
      provider: 'glm-5',
    }))

    expect(rows).toHaveLength(1)
    expect(rows[0]?.provider).toBe('智谱GLM-5.1')
  })

  it('creates a fallback preview row when the selected provider is not in the canned list', () => {
    const rows = buildFrontendDevLogsCostDetailMockData(createFilters({
      provider: 'preview-only-provider',
    }))

    expect(rows).toHaveLength(1)
    expect(rows[0]?.provider).toBe('preview-only-provider')
    expect(rows[0]?.cost_total).toBeCloseTo(3.214, 10)
  })
})

describe('resolveLogsCostDetailStats', () => {
  it('returns frontend preview mock data without touching the backend when dev mock is enabled', async () => {
    const fetchProviderStats = vi.fn().mockResolvedValue([])

    const rows = await resolveLogsCostDetailStats({
      filters: createFilters(),
      range: {
        startAt: '2026-04-23 00:00:00',
        endAt: '2026-04-23 23:59:59',
      },
      useDevMock: true,
      fetchProviderStats,
    })

    expect(fetchProviderStats).not.toHaveBeenCalled()
    expect(rows).toHaveLength(4)
    expect(rows[0]?.provider).toBe('自建-sub2api')
  })
})
