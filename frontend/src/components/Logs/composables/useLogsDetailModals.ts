import { reactive, type Ref } from 'vue'
import { fetchProviderStatsV2, type LogDataSourceMode, type ProviderDailyStat } from '../../../services/logs'
import { extractErrorMessage } from '../../../utils/error'
import type { LogsFiltersState } from '../types'
import type { LogsDateRange } from './useLogsFilters'

type UseLogsDetailModalsOptions = {
  appliedFilters: Ref<LogsFiltersState>
  appliedDateRange: Ref<LogsDateRange>
  sourceMode: Ref<LogDataSourceMode>
}

type BrowserWindowWithWailsBridge = Window & {
  chrome?: {
    webview?: {
      postMessage?: (...args: any[]) => void
    }
  }
  webkit?: {
    messageHandlers?: {
      external?: {
        postMessage?: (...args: any[]) => void
      }
    }
  }
}

type ResolveLogsCostDetailStatsOptions = {
  filters: LogsFiltersState
  range: LogsDateRange
  useDevMock?: boolean
  fetchProviderStats?: typeof fetchProviderStatsV2
  sourceMode?: LogDataSourceMode
}

function hasDesktopRuntimeBridge() {
  if (typeof window === 'undefined') {
    return false
  }
  const browserWindow = window as BrowserWindowWithWailsBridge
  return Boolean(
    browserWindow.chrome?.webview?.postMessage
    || browserWindow.webkit?.messageHandlers?.external?.postMessage,
  )
}

export function shouldUseFrontendDevLogsCostDetailMock() {
  return import.meta.env.DEV
    && typeof window !== 'undefined'
    && !hasDesktopRuntimeBridge()
}

function createDevMockProviderDailyStat(overrides: Partial<ProviderDailyStat>): ProviderDailyStat {
  return {
    provider_id: '',
    provider: '',
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
    avg_first_token_sec: 0,
    avg_tokens_per_sec: 0,
    ttft_sample_count: 0,
    tps_sample_count: 0,
    ...overrides,
  }
}

function normalizeProviderFilter(value: string | null | undefined) {
  return `${value ?? ''}`.trim().toLowerCase()
}

export function buildFrontendDevLogsCostDetailMockData(filters: LogsFiltersState): ProviderDailyStat[] {
  const mockRows: ProviderDailyStat[] = [
    createDevMockProviderDailyStat({
      provider_id: 'sub2api',
      provider: '自建-sub2api',
      total_requests: 126,
      successful_requests: 122,
      failed_requests: 4,
      success_rate: 122 / 126,
      input_tokens: 1_420_000,
      output_tokens: 318_000,
      cache_create_tokens: 86_000,
      cache_read_tokens: 128_000,
      cost_total: 52.78,
      avg_first_token_sec: 0.92,
      avg_tokens_per_sec: 37.8,
      ttft_sample_count: 98,
      tps_sample_count: 94,
    }),
    createDevMockProviderDailyStat({
      provider_id: 'glm-5-1',
      provider: '智谱GLM-5.1',
      total_requests: 41,
      successful_requests: 40,
      failed_requests: 1,
      success_rate: 40 / 41,
      input_tokens: 242_000,
      output_tokens: 69_000,
      cache_create_tokens: 18_000,
      cache_read_tokens: 22_000,
      cost_total: 7.9,
      avg_first_token_sec: 1.14,
      avg_tokens_per_sec: 29.3,
      ttft_sample_count: 32,
      tps_sample_count: 30,
    }),
    createDevMockProviderDailyStat({
      provider_id: 'qwen-company',
      provider: '公司千问',
      total_requests: 16,
      successful_requests: 16,
      failed_requests: 0,
      success_rate: 1,
      input_tokens: 76_000,
      output_tokens: 21_000,
      cache_create_tokens: 4_000,
      cache_read_tokens: 7_000,
      cost_total: 2,
      avg_first_token_sec: 0.86,
      avg_tokens_per_sec: 42.6,
      ttft_sample_count: 14,
      tps_sample_count: 14,
    }),
    createDevMockProviderDailyStat({
      provider_id: 'doubao',
      provider: '豆包',
      total_requests: 9,
      successful_requests: 9,
      failed_requests: 0,
      success_rate: 1,
      input_tokens: 3_400,
      output_tokens: 1_200,
      cache_create_tokens: 0,
      cache_read_tokens: 180,
      cost_total: 0.067,
      avg_first_token_sec: 0.74,
      avg_tokens_per_sec: 55.2,
      ttft_sample_count: 9,
      tps_sample_count: 9,
    }),
  ]

  const providerFilter = normalizeProviderFilter(filters.provider)
  if (!providerFilter) {
    return mockRows.map(item => ({ ...item }))
  }

  const filteredRows = mockRows.filter((item) => {
    const providerId = normalizeProviderFilter(item.provider_id)
    const providerName = normalizeProviderFilter(item.provider)
    return providerId.includes(providerFilter) || providerName.includes(providerFilter)
  })

  if (filteredRows.length > 0) {
    return filteredRows.map(item => ({ ...item }))
  }

  const providerName = `${filters.provider ?? ''}`.trim() || '前端预览供应商'
  return [
    createDevMockProviderDailyStat({
      provider_id: providerName,
      provider: providerName,
      total_requests: 24,
      successful_requests: 23,
      failed_requests: 1,
      success_rate: 23 / 24,
      input_tokens: 84_000,
      output_tokens: 20_500,
      cache_create_tokens: 6_400,
      cache_read_tokens: 5_600,
      cost_total: 3.214,
      avg_first_token_sec: 0.98,
      avg_tokens_per_sec: 31.4,
      ttft_sample_count: 18,
      tps_sample_count: 17,
    }),
  ]
}

export async function resolveLogsCostDetailStats(
  options: ResolveLogsCostDetailStatsOptions,
): Promise<ProviderDailyStat[]> {
  const {
    filters,
    range,
    useDevMock = false,
    fetchProviderStats = fetchProviderStatsV2,
    sourceMode = 'proxy',
  } = options

  if (useDevMock) {
    return buildFrontendDevLogsCostDetailMockData(filters)
  }

  return fetchProviderStats({
    platform: filters.platform,
    provider: filters.provider,
    model: filters.model,
    startAt: range.startAt,
    endAt: range.endAt,
    sourceMode,
  })
}

export function useLogsDetailModals(options: UseLogsDetailModalsOptions) {
  const { appliedFilters, appliedDateRange, sourceMode } = options

  const costDetailModal = reactive<{
    open: boolean
    loading: boolean
    error: string
    updatedAt: number
    data: ProviderDailyStat[]
  }>({
    open: false,
    loading: false,
    error: '',
    updatedAt: 0,
    data: [],
  })

  const tokenDetailModal = reactive<{
    open: boolean
  }>({
    open: false,
  })

  let activeCostDetailRequestId = 0

  const openCostDetailModal = async () => {
    const requestId = ++activeCostDetailRequestId
    costDetailModal.open = true
    costDetailModal.loading = true
    costDetailModal.error = ''
    costDetailModal.updatedAt = 0
    costDetailModal.data = []

    try {
      const range = appliedDateRange.value
      const filters = appliedFilters.value
      const stats = await resolveLogsCostDetailStats({
        filters,
        range,
        sourceMode: sourceMode.value,
        useDevMock: shouldUseFrontendDevLogsCostDetailMock(),
      })
      if (requestId !== activeCostDetailRequestId) return
      costDetailModal.data = (stats ?? [])
        .filter(item => item.cost_total > 0)
        .sort((a, b) => b.cost_total - a.cost_total)
      costDetailModal.updatedAt = Date.now()
    } catch (error) {
      if (requestId !== activeCostDetailRequestId) return
      costDetailModal.error = extractErrorMessage(error)
      console.error('failed to load provider daily stats', error)
    } finally {
      if (requestId !== activeCostDetailRequestId) return
      costDetailModal.loading = false
    }
  }

  const closeCostDetailModal = () => {
    activeCostDetailRequestId += 1
    costDetailModal.open = false
    costDetailModal.loading = false
  }

  const openTokenDetailModal = () => {
    tokenDetailModal.open = true
  }

  const closeTokenDetailModal = () => {
    tokenDetailModal.open = false
  }

  const handleCardClick = (key: string) => {
    if (key === 'cost') {
      void openCostDetailModal()
    } else if (key === 'tokens') {
      openTokenDetailModal()
    }
  }

  return {
    costDetailModal,
    tokenDetailModal,
    openCostDetailModal,
    closeCostDetailModal,
    openTokenDetailModal,
    closeTokenDetailModal,
    handleCardClick,
  }
}
