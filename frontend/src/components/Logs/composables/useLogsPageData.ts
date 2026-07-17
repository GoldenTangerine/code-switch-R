import { computed, ref, watch, type Ref } from 'vue'
import { LoadProviders } from '../../../../bindings/codeswitch/services/providerservice'
import { GetProviders as GetGeminiProviders } from '../../../../bindings/codeswitch/services/geminiservice'
import {
  fetchRequestLogsPage,
  fetchLogProviderRefs,
  fetchLogSummaryV2,
  fetchLogStatsV2,
  fetchModelStatsV2,
  fetchProviderStatsV2,
  type RequestLog,
  type LogSummary,
  type LogStats,
  type LogPlatform,
  type ModelUsageStat,
  type ProviderDailyStat,
  type LogProviderRef,
  type LogDataSourceMode,
} from '../../../services/logs'
import type { LogProviderOption, LogsFiltersState } from '../types'
import { cloneLogsFiltersState, createLogsFiltersState, type LogsDateRange } from './useLogsFilters'

type UseLogsPageDataOptions = {
  filters: LogsFiltersState
  computeDateRange: () => LogsDateRange | null
  sourceMode?: Ref<LogDataSourceMode>
  shouldLoadProviderStats?: () => boolean
}

type AppliedLogsQuery = {
  filters: LogsFiltersState
  range: LogsDateRange
  sourceMode: LogDataSourceMode
}

const DEFAULT_PAGE_SIZE = 15
const PAGE_SIZE_OPTIONS = [10, 15, 30, 50]
const PROVIDER_CONFIG_CACHE_TTL_MS = 60_000

const normalizeProviderName = (value: string) => value.trim()
const normalizeProviderRef = (value: string | number | null | undefined) => `${value ?? ''}`.trim()
const providerNameKey = (value: string | null | undefined) => normalizeProviderName(value ?? '').toLowerCase()

const buildProviderOption = (
  providerIdRaw: string | number | null | undefined,
  providerNameRaw: string | null | undefined,
): LogProviderOption | null => {
  const providerName = normalizeProviderName(providerNameRaw ?? '')
  const providerId = normalizeProviderRef(providerIdRaw)
  if (!providerName && !providerId) return null
  const value = providerId || providerName
  if (!value) return null
  const displayName = providerName || providerId
  const label = providerId && providerName ? `${displayName} (${providerId})` : displayName
  return {
    value,
    label,
    providerId: providerId || undefined,
    providerName: providerName || displayName,
  }
}

const mergeProviderOptions = (options: LogProviderOption[]): LogProviderOption[] => {
  const idRefsByName = new Map<string, Set<string>>()
  for (const option of options) {
    const nameKey = providerNameKey(option.providerName || option.label || option.value)
    const providerId = normalizeProviderRef(option.providerId)
    if (!nameKey || !providerId) continue
    const refs = idRefsByName.get(nameKey) ?? new Set<string>()
    refs.add(providerId)
    idRefsByName.set(nameKey, refs)
  }

  const merged = new Map<string, LogProviderOption>()
  for (const option of options) {
    let value = normalizeProviderRef(option.value)
    let providerId = normalizeProviderRef(option.providerId)
    const nameKey = providerNameKey(option.providerName || option.label || option.value)
    if (!providerId && nameKey) {
      const refs = idRefsByName.get(nameKey)
      if (refs && refs.size === 1) {
        const [resolvedId] = Array.from(refs)
        if (resolvedId) {
          providerId = resolvedId
          value = resolvedId
        }
      }
    }
    if (!value) continue
    const normalized: LogProviderOption = {
      ...option,
      value,
      providerId: providerId || undefined,
    }
    const current = merged.get(value)
    if (!current) {
      merged.set(value, normalized)
      continue
    }
    const currentHasId = normalizeProviderRef(current.providerId) !== ''
    const normalizedHasId = normalizeProviderRef(normalized.providerId) !== ''
    if (
      (normalizedHasId && !currentHasId) ||
      (normalizedHasId === currentHasId && normalized.label.length >= current.label.length)
    ) {
      merged.set(value, normalized)
    }
  }
  const result = Array.from(merged.values())
  result.sort((a, b) => {
    const left = normalizeProviderName(a.providerName || a.label || a.value)
    const right = normalizeProviderName(b.providerName || b.label || b.value)
    if (left === right) {
      return normalizeProviderRef(a.value).localeCompare(normalizeProviderRef(b.value))
    }
    return left.localeCompare(right)
  })
  return result
}

export function useLogsPageData(options: UseLogsPageDataOptions) {
  const { filters, computeDateRange } = options
  const sourceMode = options.sourceMode ?? ref<LogDataSourceMode>('proxy')
  const shouldLoadProviderStats = options.shouldLoadProviderStats ?? (() => true)

  const logs = ref<RequestLog[]>([])
  const summary = ref<LogSummary | null>(null)
  const stats = ref<LogStats | null>(null)
  const modelStats = ref<ModelUsageStat[]>([])
  const providerStats = ref<ProviderDailyStat[]>([])
  const modelOptions = ref<string[]>([])
  const loading = ref(false)
  let loadingRequestCount = 0
  const page = ref(1)
  const pageSize = ref(DEFAULT_PAGE_SIZE)
  const logsTotalCount = ref(0)
  const logsRequestId = ref(0)
  const summaryRequestId = ref(0)
  const statsRequestId = ref(0)
  const modelStatsRequestId = ref(0)
  const providerStatsRequestId = ref(0)
  const modelOptionsRequestId = ref(0)
  const providerOptionsRequestId = ref(0)
  const providerOptions = ref<LogProviderOption[]>([])
  const providerConfigCache = new Map<string, { loadedAt: number; options: LogProviderOption[] }>()
  const appliedQuery = ref<AppliedLogsQuery>(buildFallbackAppliedQuery())

  const totalPages = computed(() => Math.max(1, Math.ceil(logsTotalCount.value / pageSize.value)))
  const pagedLogs = computed(() => logs.value)
  const appliedFilters = computed(() => appliedQuery.value.filters)
  const appliedDateRange = computed(() => appliedQuery.value.range)
  const appliedSourceMode = computed(() => appliedQuery.value.sourceMode)

  const withLoading = async <T>(task: () => Promise<T>): Promise<T> => {
    loadingRequestCount += 1
    loading.value = true
    try {
      return await task()
    } finally {
      loadingRequestCount = Math.max(0, loadingRequestCount - 1)
      loading.value = loadingRequestCount > 0
    }
  }

  function buildFallbackAppliedQuery(): AppliedLogsQuery {
    return {
      filters: createLogsFiltersState(),
      range: {
        startAt: '',
        endAt: '',
      },
      sourceMode: sourceMode.value,
    }
  }

  function cloneAppliedQuery(query: AppliedLogsQuery): AppliedLogsQuery {
    return {
      filters: cloneLogsFiltersState(query.filters),
      range: {
        startAt: query.range.startAt,
        endAt: query.range.endAt,
      },
      sourceMode: query.sourceMode,
    }
  }

  function buildCurrentAppliedQuery(): AppliedLogsQuery | null {
    const range = computeDateRange()
    if (range == null) return null
    return {
      filters: cloneLogsFiltersState(filters),
      range: {
        startAt: range.startAt,
        endAt: range.endAt,
      },
      sourceMode: sourceMode.value,
    }
  }

  function matchesCurrentModelOptionsScope(query: AppliedLogsQuery): boolean {
    const current = buildCurrentAppliedQuery()
    return current != null
      && current.filters.platform === query.filters.platform
      && current.filters.provider === query.filters.provider
      && current.sourceMode === query.sourceMode
      && current.range.startAt === query.range.startAt
      && current.range.endAt === query.range.endAt
  }

  const loadProviderNamesFromConfig = async (platform: LogPlatform | ''): Promise<LogProviderOption[]> => {
    const cacheKey = platform
    const now = Date.now()
    const cached = providerConfigCache.get(cacheKey)
    if (cached && now - cached.loadedAt < PROVIDER_CONFIG_CACHE_TTL_MS) {
      return cached.options
    }

    const list: LogProviderOption[] = []
    const pushProvider = (providerIdRaw: string | number | null | undefined, providerNameRaw: string | null | undefined) => {
      const option = buildProviderOption(providerIdRaw, providerNameRaw)
      if (option) {
        list.push(option)
      }
    }

    const includeClaude = platform === '' || platform === 'claude'
    const includeCodex = platform === '' || platform === 'codex'
    const includeGemini = platform === '' || platform === 'gemini'

    if (includeClaude) {
      try {
        const providers = await LoadProviders('claude')
        for (const provider of providers ?? []) {
          pushProvider(provider?.id, provider?.name)
        }
      } catch (error) {
        console.error('failed to load claude providers from config', error)
      }
    }

    if (includeCodex) {
      try {
        const providers = await LoadProviders('codex')
        for (const provider of providers ?? []) {
          pushProvider(provider?.id, provider?.name)
        }
      } catch (error) {
        console.error('failed to load codex providers from config', error)
      }
    }

    if (includeGemini) {
      try {
        const providers = await GetGeminiProviders()
        for (const provider of providers ?? []) {
          pushProvider(provider?.id, provider?.name)
        }
      } catch (error) {
        console.error('failed to load gemini providers from config', error)
      }
    }

    const result = mergeProviderOptions(list)
    providerConfigCache.set(cacheKey, { loadedAt: now, options: result })
    return result
  }

  const buildProviderOptionsFromRefs = (refs: LogProviderRef[]): LogProviderOption[] => {
    const list: LogProviderOption[] = []
    for (const ref of refs ?? []) {
      const option = buildProviderOption(ref.provider_id, ref.provider)
      if (option) {
        list.push(option)
      }
    }
    return mergeProviderOptions(list)
  }

  const syncProviderOptionsFromLogs = (items: RequestLog[]) => {
    if (!items.length) return
    const nextOptions = [...providerOptions.value]
    for (const item of items) {
      const option = buildProviderOption(item.provider_id, item.provider)
      if (option) {
        nextOptions.push(option)
      }
    }
    providerOptions.value = mergeProviderOptions(nextOptions)
  }

  const loadProviderOptions = async (query: AppliedLogsQuery) => {
    const requestId = ++providerOptionsRequestId.value
    const [fromLogs, fromConfig] = await Promise.all([
      fetchLogProviderRefs(query.filters.platform, query.sourceMode).catch((error) => {
        console.error('failed to load provider refs from request logs', error)
        return [] as LogProviderRef[]
      }),
      (query.sourceMode === 'session' ? Promise.resolve([]) : loadProviderNamesFromConfig(query.filters.platform)).catch((error) => {
        console.error('failed to load providers from config', error)
        return [] as LogProviderOption[]
      }),
    ])
    if (requestId !== providerOptionsRequestId.value) return

    providerOptions.value = mergeProviderOptions([
      ...buildProviderOptionsFromRefs(fromLogs ?? []),
      ...(fromConfig ?? []),
    ])
  }

  const loadLogs = async (query: AppliedLogsQuery, targetPage = page.value) => {
    const requestId = ++logsRequestId.value
    const normalizedPage = Math.max(1, Math.floor(Number(targetPage) || 1))
    const limit = pageSize.value
    try {
      const result = await fetchRequestLogsPage({
        platform: query.filters.platform,
        provider: query.filters.provider,
        model: query.filters.model,
        limit,
        offset: (normalizedPage - 1) * limit,
        startAt: query.range.startAt,
        endAt: query.range.endAt,
        sourceMode: query.sourceMode,
      })
      if (requestId !== logsRequestId.value) return
      const total = Math.max(0, Number(result?.total ?? 0))
      const items = result?.items ?? []
      const nextTotalPages = Math.max(1, Math.ceil(total / limit))
      if (total > 0 && normalizedPage > nextTotalPages) {
        page.value = nextTotalPages
        logsTotalCount.value = total
        await loadLogs(query, nextTotalPages)
        return
      }
      logsTotalCount.value = total
      page.value = total > 0 ? normalizedPage : 1
      logs.value = items
    } catch (error) {
      if (requestId !== logsRequestId.value) return
      logs.value = []
      logsTotalCount.value = 0
      page.value = 1
      console.error('failed to load request logs', error)
    }
  }

  const loadStats = async (query: AppliedLogsQuery) => {
    const requestId = ++statsRequestId.value
    try {
      const data = await fetchLogStatsV2({
        platform: query.filters.platform,
        provider: query.filters.provider,
        model: query.filters.model,
        startAt: query.range.startAt,
        endAt: query.range.endAt,
        sourceMode: query.sourceMode,
      })
      if (requestId !== statsRequestId.value) return
      stats.value = data ?? null
    } catch (error) {
      if (requestId !== statsRequestId.value) return
      console.error('failed to load log stats', error)
      stats.value = null
    }
  }

  const loadSummary = async (query: AppliedLogsQuery) => {
    const requestId = ++summaryRequestId.value
    try {
      const data = await fetchLogSummaryV2({
        platform: query.filters.platform,
        provider: query.filters.provider,
        model: query.filters.model,
        startAt: query.range.startAt,
        endAt: query.range.endAt,
        sourceMode: query.sourceMode,
      })
      if (requestId !== summaryRequestId.value) return
      summary.value = data ?? null
    } catch (error) {
      if (requestId !== summaryRequestId.value) return
      console.error('failed to load log summary', error)
      summary.value = null
    }
  }

  const loadModelStats = async (query: AppliedLogsQuery) => {
    const requestId = ++modelStatsRequestId.value
    const shouldSyncModelOptions = !query.filters.model && matchesCurrentModelOptionsScope(query)
    const optionsRequestId = shouldSyncModelOptions ? ++modelOptionsRequestId.value : 0
    try {
      const data = await fetchModelStatsV2({
        platform: query.filters.platform,
        provider: query.filters.provider,
        model: query.filters.model,
        startAt: query.range.startAt,
        endAt: query.range.endAt,
        sourceMode: query.sourceMode,
      })
      if (requestId !== modelStatsRequestId.value) return
      modelStats.value = data ?? []
      if (optionsRequestId > 0 && optionsRequestId === modelOptionsRequestId.value && matchesCurrentModelOptionsScope(query)) {
        modelOptions.value = Array.from(new Set(modelStats.value.map((item) => item.model.trim()).filter(Boolean)))
      }
    } catch (error) {
      if (requestId !== modelStatsRequestId.value) return
      console.error('failed to load model stats', error)
      modelStats.value = []
      if (optionsRequestId > 0 && optionsRequestId === modelOptionsRequestId.value && matchesCurrentModelOptionsScope(query)) {
        modelOptions.value = []
      }
    }
  }

  const loadProviderStats = async (query: AppliedLogsQuery) => {
    const requestId = ++providerStatsRequestId.value
    try {
      const data = await fetchProviderStatsV2({
        platform: query.filters.platform,
        provider: query.filters.provider,
        model: query.filters.model,
        startAt: query.range.startAt,
        endAt: query.range.endAt,
        sourceMode: query.sourceMode,
      })
      if (requestId !== providerStatsRequestId.value) return
      providerStats.value = data ?? []
    } catch (error) {
      if (requestId !== providerStatsRequestId.value) return
      console.error('failed to load provider stats', error)
      providerStats.value = []
    }
  }

  const loadModelOptions = async (query: AppliedLogsQuery) => {
    const requestId = ++modelOptionsRequestId.value
    try {
      const data = await fetchModelStatsV2({
        platform: query.filters.platform,
        provider: query.filters.provider,
        startAt: query.range.startAt,
        endAt: query.range.endAt,
        sourceMode: query.sourceMode,
      })
      if (requestId !== modelOptionsRequestId.value) return
      modelOptions.value = Array.from(new Set((data ?? []).map((item) => item.model.trim()).filter(Boolean)))
    } catch (error) {
      if (requestId !== modelOptionsRequestId.value) return
      console.error('failed to load model options', error)
      modelOptions.value = []
    }
  }

  const loadDashboardByQuery = async (query: AppliedLogsQuery) => {
    await withLoading(async () => {
      await Promise.all([
        loadLogs(query),
        loadSummary(query),
        loadStats(query),
        loadModelStats(query),
        shouldLoadProviderStats() ? loadProviderStats(query) : Promise.resolve(),
        query.filters.model && matchesCurrentModelOptionsScope(query) ? loadModelOptions(query) : Promise.resolve(),
        loadProviderOptions(query),
      ])
      syncProviderOptionsFromLogs(logs.value)
    })
  }

  const loadDashboard = async () => {
    await loadDashboardByQuery(cloneAppliedQuery(appliedQuery.value))
  }

  const loadAppliedProviderStats = async () => {
    await withLoading(() => loadProviderStats(cloneAppliedQuery(appliedQuery.value)))
  }

  const applyDashboardFilters = async () => {
    const nextQuery = buildCurrentAppliedQuery()
    if (nextQuery == null) {
      return
    }
    appliedQuery.value = cloneAppliedQuery(nextQuery)
    await loadDashboardByQuery(nextQuery)
  }

  const applyDashboardSourceMode = async (value: LogDataSourceMode) => {
    const nextQuery = buildCurrentAppliedQuery() ?? cloneAppliedQuery(appliedQuery.value)
    nextQuery.sourceMode = value
    nextQuery.filters.provider = ''
    nextQuery.filters.model = ''
    page.value = 1
    appliedQuery.value = cloneAppliedQuery(nextQuery)
    await loadDashboardByQuery(nextQuery)
  }

  const resetPage = () => {
    page.value = 1
  }

  const setPage = async (value: number) => {
    const normalized = Math.max(1, Math.floor(Number(value) || 1))
    const nextPage = Math.min(normalized, totalPages.value)
    if (nextPage === page.value) return
    await withLoading(() => loadLogs(cloneAppliedQuery(appliedQuery.value), nextPage))
  }

  const setPageSize = async (value: number) => {
    const normalized = Math.max(1, Math.floor(Number(value) || DEFAULT_PAGE_SIZE))
    const nextPageSize = PAGE_SIZE_OPTIONS.includes(normalized) ? normalized : DEFAULT_PAGE_SIZE
    if (nextPageSize === pageSize.value) return
    pageSize.value = nextPageSize
    page.value = 1
    await withLoading(() => loadLogs(cloneAppliedQuery(appliedQuery.value), 1))
  }

  watch(
    () => [filters.platform, filters.provider] as const,
    async ([platform], [previousPlatform]) => {
      const query = buildCurrentAppliedQuery()
      if (platform !== previousPlatform) {
        if (query != null) {
          await loadProviderOptions(query)
        }
        if (filters.provider && !providerOptions.value.some((option) => option.value === filters.provider)) {
          filters.provider = ''
        }
      }
      if (query != null) {
        await loadModelOptions(query)
      }
    },
  )

  const initialQuery = buildCurrentAppliedQuery()
  if (initialQuery != null) {
    appliedQuery.value = cloneAppliedQuery(initialQuery)
  }

  return {
    logs,
    summary,
    stats,
    modelStats,
    providerStats,
    modelOptions,
    loading,
    page,
    pageSize,
    pageSizeOptions: PAGE_SIZE_OPTIONS,
    providerOptions,
    pagedLogs,
    totalPages,
    appliedFilters,
    appliedDateRange,
    appliedSourceMode,
    applyDashboardFilters,
    applyDashboardSourceMode,
    loadDashboard,
    loadAppliedProviderStats,
    setPage,
    setPageSize,
    resetPage,
  }
}
