import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, reactive, ref } from 'vue'
import type { LogDashboardAggregateV1, LogStats, LogSummary, RequestLog, RequestLogPageResult } from '../../../services/logs'
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
  fetchLogDashboardAggregateV1: vi.fn(),
  fetchLogSummaryV2: vi.fn(),
  fetchLogStatsV2: vi.fn(),
  fetchModelStatsV2: vi.fn(),
  fetchProviderStatsV2: vi.fn(),
}))

import { LoadProviders } from '../../../../bindings/codeswitch/services/providerservice'
import { GetProviders as GetGeminiProviders } from '../../../../bindings/codeswitch/services/geminiservice'
import {
  fetchRequestLogsPage,
  fetchLogProviderRefs,
  fetchLogDashboardAggregateV1,
  fetchLogSummaryV2,
  fetchLogStatsV2,
  fetchModelStatsV2,
  fetchProviderStatsV2,
} from '../../../services/logs'
import { useLogsPageData } from './useLogsPageData'

const createFilters = (): LogsFiltersState =>
  reactive({
    platform: '',
    provider: '',
    model: '',
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

const createStats = (): LogStats => ({
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

const createDashboardAggregate = (
  overrides: Partial<LogDashboardAggregateV1> = {},
): LogDashboardAggregateV1 => ({
  summary: createSummary(),
  stats: createStats(),
  model_stats: [],
  provider_stats: [],
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
    vi.mocked(fetchLogDashboardAggregateV1).mockResolvedValue(createDashboardAggregate())
    vi.mocked(fetchLogSummaryV2).mockResolvedValue(createSummary())
    vi.mocked(fetchLogStatsV2).mockResolvedValue(createStats())
    vi.mocked(fetchModelStatsV2).mockResolvedValue([])
    vi.mocked(fetchProviderStatsV2).mockResolvedValue([])
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
      model: '',
      limit: 15,
      offset: 0,
      sourceMode: 'proxy',
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

    expect(fetchLogDashboardAggregateV1).toHaveBeenCalledWith({
      platform: '',
      provider: '',
      model: '',
      sourceMode: 'proxy',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    }, true)
    expect(summary.value).toEqual(createSummary())
  })

  it('reloads with the last applied filters instead of pending draft edits', async () => {
    const filters = createFilters()
    filters.provider = 'provider-applied'

    const { applyDashboardFilters, loadDashboard } = useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    await applyDashboardFilters()

    filters.provider = 'provider-draft'
    await loadDashboard()

    expect(fetchLogDashboardAggregateV1).toHaveBeenNthCalledWith(1, {
      platform: '',
      provider: 'provider-applied',
      model: '',
      sourceMode: 'proxy',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    }, true)
    expect(fetchLogDashboardAggregateV1).toHaveBeenNthCalledWith(2, {
      platform: '',
      provider: 'provider-applied',
      model: '',
      sourceMode: 'proxy',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    }, true)
    expect(fetchRequestLogsPage).toHaveBeenNthCalledWith(2, {
      platform: '',
      provider: 'provider-applied',
      model: '',
      limit: 15,
      offset: 0,
      sourceMode: 'proxy',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    })
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
      model: '',
      limit: 15,
      offset: 15,
      sourceMode: 'proxy',
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
      model: '',
      limit: 30,
      offset: 0,
      sourceMode: 'proxy',
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
      model: '',
      limit: 15,
      offset: 45,
      sourceMode: 'proxy',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    })
    expect(fetchRequestLogsPage).toHaveBeenNthCalledWith(4, {
      platform: '',
      provider: '',
      model: '',
      limit: 15,
      offset: 15,
      sourceMode: 'proxy',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    })
    expect(page.value).toBe(2)
    expect(totalPages.value).toBe(2)
    expect(logs.value.map((item) => item.id)).toEqual([16])
  })

  it('ignores stale summary responses when a newer dashboard request finishes first', async () => {
    const firstAggregate = createDeferred<LogDashboardAggregateV1>()
    const latestSummary = createSummary({
      total_requests: 9,
      total_tokens: 900,
      previous_cost_total: 1.5,
    })

    vi.mocked(fetchLogDashboardAggregateV1)
      .mockImplementationOnce(() => firstAggregate.promise)
      .mockResolvedValueOnce(createDashboardAggregate({ summary: latestSummary }))

    const filters = createFilters()
    const { loadDashboard, applyDashboardFilters, summary } = useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    const firstLoad = loadDashboard()
    filters.provider = 'provider-new'
    const secondLoad = applyDashboardFilters()

    await secondLoad
    expect(summary.value).toEqual(latestSummary)

    firstAggregate.resolve(createDashboardAggregate({
      summary: createSummary({ total_requests: 1, total_tokens: 100 }),
    }))
    await firstLoad

    expect(summary.value).toEqual(latestSummary)
    expect(fetchLogDashboardAggregateV1).toHaveBeenNthCalledWith(1, {
      platform: '',
      provider: '',
      model: '',
      sourceMode: 'proxy',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    }, true)
    expect(fetchLogDashboardAggregateV1).toHaveBeenNthCalledWith(2, {
      platform: '',
      provider: 'provider-new',
      model: '',
      sourceMode: 'proxy',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    }, true)
  })

  it('applies the pricing model filter while keeping model options unfiltered by that model', async () => {
    const selectedModelStat = {
      model: 'claude-sonnet-4-5',
      total_requests: 2,
      input_tokens: 100,
      output_tokens: 50,
      cache_read_tokens: 0,
      total_tokens: 150,
      cost_total: 0.25,
    }
    const otherModelStat = {
      ...selectedModelStat,
      model: 'claude-opus-4-1',
      total_requests: 1,
      total_tokens: 75,
      cost_total: 0.5,
    }
    vi.mocked(fetchLogDashboardAggregateV1).mockResolvedValueOnce(createDashboardAggregate({
      model_stats: [selectedModelStat],
    }))
    vi.mocked(fetchModelStatsV2).mockResolvedValueOnce([selectedModelStat, otherModelStat])

    const filters = createFilters()
    filters.model = selectedModelStat.model
    const { applyDashboardFilters, modelOptions, modelStats } = useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    await applyDashboardFilters()

    const filteredQuery = {
      platform: '',
      provider: '',
      model: selectedModelStat.model,
      sourceMode: 'proxy',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    }
    expect(fetchLogDashboardAggregateV1).toHaveBeenCalledWith(filteredQuery, true)
    expect(fetchLogSummaryV2).not.toHaveBeenCalled()
    expect(fetchLogStatsV2).not.toHaveBeenCalled()
    expect(fetchProviderStatsV2).not.toHaveBeenCalled()
    expect(fetchModelStatsV2).toHaveBeenCalledTimes(1)
    expect(fetchModelStatsV2).toHaveBeenCalledWith({
      platform: '',
      provider: '',
      sourceMode: 'proxy',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    })
    expect(fetchRequestLogsPage).toHaveBeenCalledWith({
      ...filteredQuery,
      limit: 15,
      offset: 0,
    })
    expect(modelStats.value).toEqual([selectedModelStat])
    expect(modelOptions.value).toEqual([selectedModelStat.model, otherModelStat.model])
  })

  it('keeps selected-model refresh calls fixed and reuses the provider config cache', async () => {
    const filters = createFilters()
    filters.model = 'gpt-5'
    const { applyDashboardFilters, loadDashboard } = useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    await applyDashboardFilters()

    expect(fetchRequestLogsPage).toHaveBeenCalledTimes(1)
    expect(fetchLogDashboardAggregateV1).toHaveBeenCalledTimes(1)
    expect(fetchModelStatsV2).toHaveBeenCalledTimes(1)
    expect(fetchLogSummaryV2).not.toHaveBeenCalled()
    expect(fetchLogStatsV2).not.toHaveBeenCalled()
    expect(fetchProviderStatsV2).not.toHaveBeenCalled()
    expect(fetchLogProviderRefs).toHaveBeenCalledTimes(1)
    expect(LoadProviders).toHaveBeenCalledTimes(2)
    expect(GetGeminiProviders).toHaveBeenCalledTimes(1)

    await loadDashboard()

    expect(fetchRequestLogsPage).toHaveBeenCalledTimes(2)
    expect(fetchLogDashboardAggregateV1).toHaveBeenCalledTimes(2)
    expect(fetchModelStatsV2).toHaveBeenCalledTimes(2)
    expect(fetchLogSummaryV2).not.toHaveBeenCalled()
    expect(fetchLogStatsV2).not.toHaveBeenCalled()
    expect(fetchProviderStatsV2).not.toHaveBeenCalled()
    expect(fetchLogProviderRefs).toHaveBeenCalledTimes(2)
    expect(LoadProviders).toHaveBeenCalledTimes(2)
    expect(GetGeminiProviders).toHaveBeenCalledTimes(1)
  })

  it('keeps dashboard loading active until provider stats finish', async () => {
    const logsDeferred = createDeferred<RequestLogPageResult>()
    const aggregateDeferred = createDeferred<LogDashboardAggregateV1>()
    vi.mocked(fetchRequestLogsPage).mockReturnValueOnce(logsDeferred.promise)
    vi.mocked(fetchLogDashboardAggregateV1).mockReturnValueOnce(aggregateDeferred.promise)

    const filters = createFilters()
    const { loadDashboard, loading } = useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    const pendingLoad = loadDashboard()
    await vi.waitFor(() => {
      expect(fetchRequestLogsPage).toHaveBeenCalledTimes(1)
      expect(fetchLogDashboardAggregateV1).toHaveBeenCalledTimes(1)
    })
    logsDeferred.resolve(createPageResult([], 0, 15, 0))
    for (let index = 0; index < 10; index += 1) {
      await Promise.resolve()
    }
    expect(loading.value).toBe(true)

    aggregateDeferred.resolve(createDashboardAggregate())
    await pendingLoad
    expect(loading.value).toBe(false)
  })

  it('reloads model options for the pending provider selection', async () => {
    const filters = createFilters()
    useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    filters.provider = 'provider-next'
    await nextTick()
    await vi.waitFor(() => expect(fetchModelStatsV2).toHaveBeenCalled())

    expect(fetchModelStatsV2).toHaveBeenLastCalledWith({
      platform: '',
      provider: 'provider-next',
      sourceMode: 'proxy',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    })
  })

  it('does not let an applied-query refresh overwrite pending provider model options', async () => {
    const providerAModel = {
      model: 'model-a',
      total_requests: 1,
      input_tokens: 10,
      output_tokens: 5,
      cache_read_tokens: 0,
      total_tokens: 15,
      cost_total: 0.1,
    }
    const providerBModel = { ...providerAModel, model: 'model-b' }
    const filters = createFilters()
    filters.provider = 'provider-a'
    const { loadDashboard, modelOptions } = useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    vi.mocked(fetchModelStatsV2).mockResolvedValueOnce([providerBModel])
    filters.provider = 'provider-b'
    await nextTick()
    await vi.waitFor(() => expect(modelOptions.value).toEqual(['model-b']))

    vi.mocked(fetchModelStatsV2).mockResolvedValueOnce([providerAModel])
    await loadDashboard()

    expect(modelOptions.value).toEqual(['model-b'])
  })

  it('skips provider aggregation until it is explicitly requested', async () => {
    const filters = createFilters()
    const { loadAppliedProviderStats, loadDashboard } = useLogsPageData({
      filters,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
      shouldLoadProviderStats: () => false,
    })

    await loadDashboard()
    expect(fetchLogDashboardAggregateV1).toHaveBeenCalledWith(expect.any(Object), false)
    expect(fetchProviderStatsV2).not.toHaveBeenCalled()

    await loadAppliedProviderStats()
    expect(fetchProviderStatsV2).toHaveBeenCalledTimes(1)
  })

  it('uses one applied source mode for details and every dashboard aggregation', async () => {
    const filters = createFilters()
    const sourceMode = ref<'proxy' | 'session' | 'all'>('session')
    const { applyDashboardFilters } = useLogsPageData({
      filters,
      sourceMode,
      computeDateRange: () => ({ startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }),
    })

    await applyDashboardFilters()

    const query = expect.objectContaining({ sourceMode: 'session' })
    expect(fetchRequestLogsPage).toHaveBeenCalledWith(query)
    expect(fetchLogDashboardAggregateV1).toHaveBeenCalledWith(query, true)
    expect(fetchLogSummaryV2).not.toHaveBeenCalled()
    expect(fetchLogStatsV2).not.toHaveBeenCalled()
    expect(fetchProviderStatsV2).not.toHaveBeenCalled()
    expect(fetchLogProviderRefs).toHaveBeenCalledWith('', 'session')
  })

  it('applies a source change with the last valid range when the date draft is invalid', async () => {
    const filters = createFilters()
    const sourceMode = ref<'proxy' | 'session' | 'all'>('proxy')
    let isRangeValid = true
    const { appliedSourceMode, applyDashboardFilters, applyDashboardSourceMode } = useLogsPageData({
      filters,
      sourceMode,
      computeDateRange: () => isRangeValid
        ? { startAt: '2026-03-01 00:00:00', endAt: '2026-03-31 23:59:59' }
        : null,
    })

    await applyDashboardFilters()
    isRangeValid = false
    sourceMode.value = 'session'
    filters.provider = 'draft-provider'
    filters.model = 'draft-model'
    vi.clearAllMocks()

    await applyDashboardSourceMode('session')

    expect(appliedSourceMode.value).toBe('session')
    expect(fetchRequestLogsPage).toHaveBeenCalledWith(expect.objectContaining({
      provider: '',
      model: '',
      sourceMode: 'session',
      startAt: '2026-03-01 00:00:00',
      endAt: '2026-03-31 23:59:59',
    }))
    expect(fetchLogProviderRefs).toHaveBeenCalledWith('', 'session')
  })
})
