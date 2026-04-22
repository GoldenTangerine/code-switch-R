import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import type { LogSummary, RequestLog, RequestLogPageResult } from '../../../services/logs'
import type { LogsFiltersState } from '../types'

vi.mock('../../../../bindings/codeswitch/services/providerservice', () => ({
  LoadProviders: vi.fn(),
}))

vi.mock('../../../../bindings/codeswitch/services/geminiservice', () => ({
  GetProviders: vi.fn(),
}))

vi.mock('../../../services/logs', () => ({
  fetchRequestLogsPage: vi.fn(),
  fetchLogProviderRefs: vi.fn(),
  fetchLogSummaryV2: vi.fn(),
  fetchLogStatsV2: vi.fn(),
  fetchModelStatsV2: vi.fn(),
}))

import { LoadProviders } from '../../../../bindings/codeswitch/services/providerservice'
import { GetProviders as GetGeminiProviders } from '../../../../bindings/codeswitch/services/geminiservice'
import {
  fetchRequestLogsPage,
  fetchLogProviderRefs,
  fetchLogSummaryV2,
  fetchLogStatsV2,
  fetchModelStatsV2,
} from '../../../services/logs'
import { useLogsPageData } from './useLogsPageData'

const createFilters = (): LogsFiltersState =>
  reactive({
    platform: '',
    provider: '',
    dateType: 'all',
    year: '',
    month: '',
    day: '',
    rangeStart: '',
    rangeEnd: '',
  })

const createRequestLog = (id: number): RequestLog => ({
  id,
  platform: 'claude',
  model: `model-${id}`,
  provider: `provider-${id}`,
  http_code: 200,
  input_tokens: 10,
  output_tokens: 20,
  cache_create_tokens: 0,
  cache_read_tokens: 0,
  reasoning_tokens: 0,
  created_at: `2026-03-0${(id % 9) + 1} 12:00:00`,
})

const createPageResult = (
  items: RequestLog[],
  total: number,
  limit: number,
  offset: number,
): RequestLogPageResult => ({
  items,
  total,
  limit,
  offset,
})

const createSummary = (overrides: Partial<LogSummary> = {}): LogSummary => ({
  total_requests: 2,
  failed_requests: 0,
  success_rate: 1,
  input_tokens: 100,
  output_tokens: 50,
  cache_read_tokens: 25,
  total_tokens: 175,
  peak_tokens: 120,
  avg_tokens_per_request: 87.5,
  cost_total: 1.25,
  cost_input: 0.8,
  cost_cache_read: 0.1,
  saved_cost_estimate: 0.45,
  projected_daily_cost: 3,
  previous_cost_total: 0.75,
  comparison_available: true,
  activity_avg_qps: 0.2,
  activity_peak_qps: 0.4,
  activity_points: [0, 0.1, 0.2, 0.3],
  ...overrides,
})

const createDeferred = <T>() => {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    void rej
  })
  return { promise, resolve }
}

describe('useLogsPageData', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(LoadProviders).mockResolvedValue([])
    vi.mocked(GetGeminiProviders).mockResolvedValue([])
    vi.mocked(fetchLogProviderRefs).mockResolvedValue([])
    vi.mocked(fetchRequestLogsPage).mockResolvedValue(createPageResult([], 0, 15, 0))
    vi.mocked(fetchLogSummaryV2).mockResolvedValue(createSummary())
    vi.mocked(fetchLogStatsV2).mockResolvedValue({
      total_requests: 0,
      input_tokens: 0,
      output_tokens: 0,
      reasoning_tokens: 0,
      cache_create_tokens: 0,
      cache_read_tokens: 0,
      cost_total: 0,
      cost_input: 0,
      cost_output: 0,
      cost_cache_create: 0,
      cost_cache_read: 0,
      series: [],
    })
    vi.mocked(fetchModelStatsV2).mockResolvedValue([])
  })

  it('loads main logs through backend pagination and derives total pages from total count', async () => {
    vi.mocked(fetchRequestLogsPage).mockResolvedValueOnce(
      createPageResult([createRequestLog(1), createRequestLog(2)], 47, 15, 0),
    )

    const filters = createFilters()
    const { loadDashboard, logs, pagedLogs, page, totalPages } = useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    await loadDashboard()

    expect(fetchRequestLogsPage).toHaveBeenCalledWith({
      platform: '',
      provider: '',
      limit: 15,
      offset: 0,
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    })
    expect(page.value).toBe(1)
    expect(totalPages.value).toBe(4)
    expect(logs.value).toHaveLength(2)
    expect(pagedLogs.value.map((item) => item.id)).toEqual([1, 2])
  })

  it('loads summary through the unified aggregation endpoint with the same filter range', async () => {
    const filters = createFilters()
    const { loadDashboard, summary } = useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    await loadDashboard()

    expect(fetchLogSummaryV2).toHaveBeenCalledWith({
      platform: '',
      provider: '',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    })
    expect(summary.value).toEqual(createSummary())
  })

  it('reloads logs when switching page', async () => {
    vi.mocked(fetchRequestLogsPage)
      .mockResolvedValueOnce(createPageResult([createRequestLog(1)], 47, 15, 0))
      .mockResolvedValueOnce(createPageResult([createRequestLog(16)], 47, 15, 15))

    const filters = createFilters()
    const { loadDashboard, setPage, page, logs } = useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    await loadDashboard()
    await setPage(2)

    expect(fetchRequestLogsPage).toHaveBeenNthCalledWith(2, {
      platform: '',
      provider: '',
      limit: 15,
      offset: 15,
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    })
    expect(page.value).toBe(2)
    expect(logs.value.map((item) => item.id)).toEqual([16])
  })

  it('resets to the first page and reloads when page size changes', async () => {
    vi.mocked(fetchRequestLogsPage)
      .mockResolvedValueOnce(createPageResult([createRequestLog(1)], 47, 15, 0))
      .mockResolvedValueOnce(createPageResult([createRequestLog(16)], 47, 15, 15))
      .mockResolvedValueOnce(createPageResult([createRequestLog(1), createRequestLog(2)], 47, 30, 0))

    const filters = createFilters()
    const { loadDashboard, setPage, setPageSize, page, pageSize, logs } = useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    await loadDashboard()
    await setPage(2)
    await setPageSize(30)

    expect(fetchRequestLogsPage).toHaveBeenNthCalledWith(3, {
      platform: '',
      provider: '',
      limit: 30,
      offset: 0,
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    })
    expect(pageSize.value).toBe(30)
    expect(page.value).toBe(1)
    expect(logs.value.map((item) => item.id)).toEqual([1, 2])
  })

  it('falls back to the last valid page when the current page exceeds the latest total', async () => {
    vi.mocked(fetchRequestLogsPage)
      .mockResolvedValueOnce(createPageResult([createRequestLog(1)], 61, 15, 0))
      .mockResolvedValueOnce(createPageResult([createRequestLog(46)], 61, 15, 45))
      .mockResolvedValueOnce(createPageResult([], 20, 15, 45))
      .mockResolvedValueOnce(createPageResult([createRequestLog(16)], 20, 15, 15))

    const filters = createFilters()
    const { loadDashboard, setPage, page, totalPages, logs } = useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    await loadDashboard()
    await setPage(4)
    await loadDashboard()

    expect(fetchRequestLogsPage).toHaveBeenNthCalledWith(3, {
      platform: '',
      provider: '',
      limit: 15,
      offset: 45,
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    })
    expect(fetchRequestLogsPage).toHaveBeenNthCalledWith(4, {
      platform: '',
      provider: '',
      limit: 15,
      offset: 15,
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    })
    expect(page.value).toBe(2)
    expect(totalPages.value).toBe(2)
    expect(logs.value.map((item) => item.id)).toEqual([16])
  })

  it('ignores stale summary responses when a newer dashboard request finishes first', async () => {
    const firstSummary = createDeferred<LogSummary>()
    const latestSummary = createSummary({
      total_requests: 9,
      total_tokens: 900,
      previous_cost_total: 1.5,
    })

    vi.mocked(fetchLogSummaryV2)
      .mockImplementationOnce(() => firstSummary.promise)
      .mockResolvedValueOnce(latestSummary)

    const filters = createFilters()
    const { loadDashboard, summary } = useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    const firstLoad = loadDashboard()
    filters.provider = 'provider-new'
    const secondLoad = loadDashboard()

    await secondLoad
    expect(summary.value).toEqual(latestSummary)

    firstSummary.resolve(createSummary({ total_requests: 1, total_tokens: 100 }))
    await firstLoad

    expect(summary.value).toEqual(latestSummary)
    expect(fetchLogSummaryV2).toHaveBeenNthCalledWith(1, {
      platform: '',
      provider: '',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    })
    expect(fetchLogSummaryV2).toHaveBeenNthCalledWith(2, {
      platform: '',
      provider: 'provider-new',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    })
  })
})
