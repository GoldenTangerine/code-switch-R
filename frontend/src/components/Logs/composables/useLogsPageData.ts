import { computed, ref, watch } from 'vue'
import { LoadProviders } from '../../../../bindings/codeswitch/services/providerservice'
import { GetProviders as GetGeminiProviders } from '../../../../bindings/codeswitch/services/geminiservice'
import {
  fetchRequestLogsPage,
  fetchLogProviderRefs,
  fetchLogSummaryV2,
  fetchLogStatsV2,
  fetchModelStatsV2,
  type RequestLog,
  type LogSummary,
  type LogStats,
  type LogPlatform,
  type ModelUsageStat,
  type LogProviderRef,
} from '../../../services/logs'
import type { LogProviderOption, LogsFiltersState } from '../types'

type LogsDateRange = {
  startAt: string
  endAt: string
}

type UseLogsPageDataOptions = {
  filters: LogsFiltersState
  computeDateRange: () => LogsDateRange | null
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

  const logs = ref<RequestLog[]>([])
  const summary = ref<LogSummary | null>(null)
  const stats = ref<LogStats | null>(null)
  const modelStats = ref<ModelUsageStat[]>([])
  const loading = ref(false)
  const page = ref(1)
  const pageSize = ref(DEFAULT_PAGE_SIZE)
  const logsTotalCount = ref(0)
  const logsRequestId = ref(0)
  const summaryRequestId = ref(0)
  const statsRequestId = ref(0)
  const modelStatsRequestId = ref(0)
  const providerOptionsRequestId = ref(0)
  const providerOptions = ref<LogProviderOption[]>([])
  const providerConfigCache = new Map<string, { loadedAt: number; options: LogProviderOption[] }>()

  const totalPages = computed(() => Math.max(1, Math.ceil(logsTotalCount.value / pageSize.value)))
  const pagedLogs = computed(() => logs.value)

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

  const loadProviderOptions = async () => {
    const requestId = ++providerOptionsRequestId.value
    const [fromLogs, fromConfig] = await Promise.all([
      fetchLogProviderRefs(filters.platform).catch((error) => {
        console.error('failed to load provider refs from request logs', error)
        return [] as LogProviderRef[]
      }),
      loadProviderNamesFromConfig(filters.platform).catch((error) => {
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

  const loadLogs = async (targetPage = page.value) => {
    const range = computeDateRange()
    if (range == null) {
      return
    }
    const requestId = ++logsRequestId.value
    const normalizedPage = Math.max(1, Math.floor(Number(targetPage) || 1))
    const limit = pageSize.value
    loading.value = true
    try {
      const result = await fetchRequestLogsPage({
        platform: filters.platform,
        provider: filters.provider,
        limit,
        offset: (normalizedPage - 1) * limit,
        startAt: range.startAt,
        endAt: range.endAt,
      })
      if (requestId !== logsRequestId.value) return
      const total = Math.max(0, Number(result?.total ?? 0))
      const items = result?.items ?? []
      const nextTotalPages = Math.max(1, Math.ceil(total / limit))
      if (total > 0 && normalizedPage > nextTotalPages) {
        page.value = nextTotalPages
        logsTotalCount.value = total
        void loadLogs(nextTotalPages)
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
    } finally {
      if (requestId === logsRequestId.value) {
        loading.value = false
      }
    }
  }

  const loadStats = async () => {
    const requestId = ++statsRequestId.value
    try {
      const range = computeDateRange()
      if (range == null) {
        if (requestId === statsRequestId.value) {
          stats.value = null
        }
        return
      }
      const data = await fetchLogStatsV2({
        platform: filters.platform,
        provider: filters.provider,
        startAt: range.startAt,
        endAt: range.endAt,
      })
      if (requestId !== statsRequestId.value) return
      stats.value = data ?? null
    } catch (error) {
      if (requestId !== statsRequestId.value) return
      console.error('failed to load log stats', error)
      stats.value = null
    }
  }

  const loadSummary = async () => {
    const requestId = ++summaryRequestId.value
    try {
      const range = computeDateRange()
      if (range == null) {
        if (requestId === summaryRequestId.value) {
          summary.value = null
        }
        return
      }
      const data = await fetchLogSummaryV2({
        platform: filters.platform,
        provider: filters.provider,
        startAt: range.startAt,
        endAt: range.endAt,
      })
      if (requestId !== summaryRequestId.value) return
      summary.value = data ?? null
    } catch (error) {
      if (requestId !== summaryRequestId.value) return
      console.error('failed to load log summary', error)
      summary.value = null
    }
  }

  const loadModelStats = async () => {
    const requestId = ++modelStatsRequestId.value
    try {
      const range = computeDateRange()
      if (range == null) {
        if (requestId === modelStatsRequestId.value) {
          modelStats.value = []
        }
        return
      }
      const data = await fetchModelStatsV2({
        platform: filters.platform,
        provider: filters.provider,
        startAt: range.startAt,
        endAt: range.endAt,
      })
      if (requestId !== modelStatsRequestId.value) return
      modelStats.value = data ?? []
    } catch (error) {
      if (requestId !== modelStatsRequestId.value) return
      console.error('failed to load model stats', error)
      modelStats.value = []
    }
  }

  const loadDashboard = async () => {
    await Promise.all([
      loadLogs(),
      loadSummary(),
      loadStats(),
      loadModelStats(),
      loadProviderOptions(),
    ])
    syncProviderOptionsFromLogs(logs.value)
  }

  const resetPage = () => {
    page.value = 1
  }

  const setPage = async (value: number) => {
    const normalized = Math.max(1, Math.floor(Number(value) || 1))
    const nextPage = Math.min(normalized, totalPages.value)
    if (nextPage === page.value) return
    await loadLogs(nextPage)
  }

  const setPageSize = async (value: number) => {
    const normalized = Math.max(1, Math.floor(Number(value) || DEFAULT_PAGE_SIZE))
    const nextPageSize = PAGE_SIZE_OPTIONS.includes(normalized) ? normalized : DEFAULT_PAGE_SIZE
    if (nextPageSize === pageSize.value) return
    pageSize.value = nextPageSize
    page.value = 1
    await loadLogs(1)
  }

  watch(
    () => filters.platform,
    async () => {
      await loadProviderOptions()
      if (filters.provider && !providerOptions.value.some((option) => option.value === filters.provider)) {
        filters.provider = ''
      }
    },
  )

  return {
    logs,
    summary,
    stats,
    modelStats,
    loading,
    page,
    pageSize,
    pageSizeOptions: PAGE_SIZE_OPTIONS,
    providerOptions,
    pagedLogs,
    totalPages,
    loadDashboard,
    setPage,
    setPageSize,
    resetPage,
  }
}
