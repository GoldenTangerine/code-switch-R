import { reactive } from 'vue'
import type { AutomationCard } from '../../../data/cards'
import {
  fetchProviderDailyStats,
  fetchProviderUnreadFailedStats,
  type ProviderDailyStat,
  type ProviderUnreadFailedStat,
} from '../../../services/logs'
import { fetchProviderModelPricing } from '../../../services/providerModelPricing'
import {
  PROVIDER_PRICING_CLICK_THROTTLE_MS,
  PROVIDER_PRICING_STARTUP_CONCURRENCY,
  PROVIDER_TAB_IDS,
  SUCCESS_RATE_THRESHOLDS,
} from '../constants'
import {
  cardProviderRef,
  normalizeProviderKey,
  normalizeProviderRef,
  providerStatsKeyFromStat,
} from '../adapters/providerCardMappers'
import type { ProviderStatDisplay, ProviderTab, TranslateFn } from '../types'
import { buildProviderCostDisplay } from '../utils/providerCostDisplay'

type UseProviderStatsOptions = {
  t: TranslateFn
  getLocale: () => string
  getActiveTab: () => ProviderTab
  cards: Record<ProviderTab, AutomationCard[]>
  refreshAvailabilityResults: () => Promise<void>
}

type ProviderPricingRefreshOptions = {
  force?: boolean
  silent?: boolean
}

const clamp = (value: number, min: number, max: number) => {
  if (max <= min) return min
  return Math.min(Math.max(value, min), max)
}

const formatMetric = (value: number) => value.toLocaleString()

const formatTokenNumber = (value: number) => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  }
  if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  }
  if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}k`
  }
  return value.toLocaleString()
}

const toPositiveFiniteNumber = (value: unknown) => {
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric) || numeric <= 0) return 0
  return numeric
}

const formatAverageFirstTokenMs = (value: unknown) => {
  const seconds = toPositiveFiniteNumber(value)
  if (seconds <= 0) return '—'
  const milliseconds = seconds * 1000
  const precision = milliseconds >= 100 ? 0 : milliseconds >= 10 ? 1 : 2
  return `${milliseconds.toFixed(precision)} ms`
}

const formatAverageTokensPerSecond = (value: unknown) => {
  const tokensPerSecond = toPositiveFiniteNumber(value)
  if (tokensPerSecond <= 0) return '—'
  const precision = tokensPerSecond >= 100 ? 1 : 2
  return `${tokensPerSecond.toFixed(precision)} tokens/s`
}

const normalizeUnreadFailedRequests = (value: unknown) => {
  const numeric = Number(value ?? 0)
  return Number.isFinite(numeric) ? Math.max(0, Math.floor(numeric)) : 0
}

const providerUnreadStatsKeyFromStat = (stat: ProviderUnreadFailedStat) => {
  const ref = normalizeProviderRef(stat.provider_id)
  if (ref) return ref
  return normalizeProviderKey(stat.provider)
}

export const resolveProviderUnreadFailedRequestsForCard = (
  card: AutomationCard,
  unreadMap: Record<string, number> | undefined,
) => {
  const statKey = cardProviderRef(card)
  if (statKey) {
    return normalizeUnreadFailedRequests(unreadMap?.[statKey])
  }

  return normalizeUnreadFailedRequests(unreadMap?.[normalizeProviderKey(card.name)])
}

export function useProviderStats(options: UseProviderStatsOptions) {
  const { t, getLocale, getActiveTab, cards, refreshAvailabilityResults } = options

  const providerStatsMap = reactive<Record<ProviderTab, Record<string, ProviderDailyStat>>>({
    claude: {},
    codex: {},
    gemini: {},
    opencode: {},
    others: {},
  })
  const providerStatsLoaded = reactive<Record<ProviderTab, boolean>>({
    claude: false,
    codex: false,
    gemini: false,
    opencode: false,
    others: false,
  })
  const providerUnreadFailedMap = reactive<Record<ProviderTab, Record<string, number>>>({
    claude: {},
    codex: {},
    gemini: {},
    opencode: {},
    others: {},
  })

  let providerStatsTimer: number | undefined

  const buildProviderPricingRefreshTargets = () => {
    const seen = new Set<string>()
    const targets: Array<{ card: AutomationCard; tab: ProviderTab }> = []

    for (const tab of PROVIDER_TAB_IDS) {
      for (const card of cards[tab]) {
        const apiUrl = String(card.apiUrl ?? '').trim()
        const apiKey = String(card.apiKey ?? '').trim()
        if (!apiUrl || !apiKey) continue

        const dedupeKey = getProviderPricingRefreshKey(card)
        if (!dedupeKey || seen.has(dedupeKey)) continue

        seen.add(dedupeKey)
        targets.push({ card, tab })
      }
    }

    return targets
  }

  const providerPricingRefreshLastRunAt = new Map<string, number>()
  const providerPricingRefreshInFlight = new Map<string, Promise<void>>()

  const getProviderPricingRefreshKey = (card: AutomationCard) => {
    const apiUrl = String(card.apiUrl ?? '').trim().toLowerCase()
    const apiKey = String(card.apiKey ?? '').trim()
    if (!apiUrl || !apiKey) return ''
    const authType = String(card.connectivityAuthType ?? '').trim().toLowerCase()
    return `${apiUrl}|${apiKey}|${authType}`
  }

  const refreshProviderPricingForCard = (
    card: AutomationCard,
    tab: ProviderTab,
    options: ProviderPricingRefreshOptions = {},
  ) => {
    const refreshKey = getProviderPricingRefreshKey(card)
    if (!refreshKey) return Promise.resolve()

    const inFlight = providerPricingRefreshInFlight.get(refreshKey)
    if (inFlight) return inFlight

    if (!options.force) {
      const lastRunAt = providerPricingRefreshLastRunAt.get(refreshKey) ?? 0
      if (Date.now() - lastRunAt < PROVIDER_PRICING_CLICK_THROTTLE_MS) {
        return Promise.resolve()
      }
    }

    providerPricingRefreshLastRunAt.set(refreshKey, Date.now())
    const task = fetchProviderModelPricing(card, tab)
      .then(() => undefined)
      .catch((error) => {
        if (!options.silent) {
          console.warn('[ProviderPricing] refresh failed:', card.name, error)
        }
      })
      .finally(() => {
        providerPricingRefreshInFlight.delete(refreshKey)
      })

    providerPricingRefreshInFlight.set(refreshKey, task)
    return task
  }

  const runTasksWithConcurrencyLimit = async (tasks: Array<() => Promise<void>>, limit: number) => {
    if (tasks.length === 0) return

    const safeLimit = Math.max(1, Math.min(limit, tasks.length))
    let index = 0

    const workers = Array.from({ length: safeLimit }, async () => {
      while (index < tasks.length) {
        const current = index
        index++
        await tasks[current]()
      }
    })

    await Promise.all(workers)
  }

  const refreshProviderPricingCachesOnStartup = async () => {
    const targets = buildProviderPricingRefreshTargets()
    if (targets.length === 0) return

    // 同一套凭据只预热一次，避免首页启动把同源 API 站点怼得直冒烟。
    const tasks = targets.map(({ card, tab }) => (
      () => refreshProviderPricingForCard(card, tab, { force: true, silent: true })
    ))
    await runTasksWithConcurrencyLimit(tasks, PROVIDER_PRICING_STARTUP_CONCURRENCY)
  }

  const handleProviderCardClick = (card: AutomationCard) => {
    const apiUrl = String(card.apiUrl ?? '').trim()
    const apiKey = String(card.apiKey ?? '').trim()
    if (!apiUrl || !apiKey) return
    void refreshProviderPricingForCard(card, getActiveTab(), { silent: true })
  }

  const loadProviderStats = async (tab: ProviderTab) => {
    if (tab === 'others' || tab === 'opencode') {
      providerStatsMap[tab] = {}
      providerUnreadFailedMap[tab] = {}
      providerStatsLoaded[tab] = true
      return
    }

    const platform = tab as 'claude' | 'codex' | 'gemini'
    const [dailyResult, unreadResult] = await Promise.allSettled([
      fetchProviderDailyStats(platform),
      fetchProviderUnreadFailedStats(platform),
    ])

    if (dailyResult.status === 'fulfilled') {
      const mapped: Record<string, ProviderDailyStat> = {}
      ;(dailyResult.value ?? []).forEach((stat) => {
        mapped[providerStatsKeyFromStat(stat)] = stat
      })
      providerStatsMap[tab] = mapped
    } else {
      console.error(`Failed to load provider stats for ${tab}`, dailyResult.reason)
    }

    if (unreadResult.status === 'fulfilled') {
      const unreadMapped: Record<string, number> = {}
      ;(unreadResult.value ?? []).forEach((stat) => {
        unreadMapped[providerUnreadStatsKeyFromStat(stat)] = normalizeUnreadFailedRequests(stat.unread_failed_requests)
      })
      providerUnreadFailedMap[tab] = unreadMapped
    } else {
      // 未读提醒比实时精确更重要：短暂查询失败时保留上一轮红点，避免把仍需处理的错误提示吞掉。
      console.error(`Failed to load provider unread failure stats for ${tab}`, unreadResult.reason)
    }

    providerStatsLoaded[tab] = true
  }

  const loadAllProviderStats = async () => {
    await Promise.all(PROVIDER_TAB_IDS.map((tab) => loadProviderStats(tab)))
  }

  const formatSuccessRateLabel = (value: number) => {
    const percent = clamp(value, 0, 1) * 100
    const decimals = percent >= 99.5 || percent === 0 ? 0 : 1
    return `${t('components.main.providers.successRate')}: ${percent.toFixed(decimals)}%`
  }

  const successRateClassName = (value: number) => {
    const rate = clamp(value, 0, 1)
    if (rate >= SUCCESS_RATE_THRESHOLDS.healthy) {
      return 'success-good'
    }
    if (rate >= SUCCESS_RATE_THRESHOLDS.warning) {
      return 'success-warn'
    }
    return 'success-bad'
  }

  const providerStatDisplay = (card: AutomationCard): ProviderStatDisplay => {
    const tab = getActiveTab()
    const statKey = cardProviderRef(card) || normalizeProviderKey(card.name)
    const unreadFailedRequests = resolveProviderUnreadFailedRequestsForCard(card, providerUnreadFailedMap[tab])
    const hasUnreadErrorLogs = unreadFailedRequests > 0

    if (!providerStatsLoaded[tab]) {
      return {
        state: 'loading',
        message: t('components.main.providers.loading'),
        unreadFailedRequests: 0,
        hasUnreadErrorLogs: false,
      }
    }

    const stat = providerStatsMap[tab]?.[statKey] ?? providerStatsMap[tab]?.[normalizeProviderKey(card.name)]
    if (!stat) {
      return {
        state: 'empty',
        message: t('components.main.providers.noData'),
        unreadFailedRequests,
        hasUnreadErrorLogs,
      }
    }

    const inputTokens = Number.isFinite(Number(stat.input_tokens)) ? Number(stat.input_tokens) : 0
    const outputTokens = Number.isFinite(Number(stat.output_tokens)) ? Number(stat.output_tokens) : 0
    const cacheReadTokens = Number.isFinite(Number(stat.cache_read_tokens)) ? Number(stat.cache_read_tokens) : 0
    const totalTokens = Math.max(0, inputTokens + outputTokens + cacheReadTokens)
    const totalRequests = Number(stat.total_requests ?? 0)
    const successRateValue = totalRequests > 0 && Number.isFinite(stat.success_rate) ? clamp(stat.success_rate, 0, 1) : null
    const successRateLabel = successRateValue !== null ? formatSuccessRateLabel(successRateValue) : ''
    const successRateClass = successRateValue !== null ? successRateClassName(successRateValue) : ''
    const ttftSampleCountRaw = Number(stat.ttft_sample_count ?? 0)
    const tpsSampleCountRaw = Number(stat.tps_sample_count ?? 0)
    const ttftSampleCount = Number.isFinite(ttftSampleCountRaw) ? Math.max(0, Math.floor(ttftSampleCountRaw)) : 0
    const tpsSampleCount = Number.isFinite(tpsSampleCountRaw) ? Math.max(0, Math.floor(tpsSampleCountRaw)) : 0
    const failedRequestsRaw = Number(stat.failed_requests ?? 0)
    const failedRequests = Number.isFinite(failedRequestsRaw) ? Math.max(0, Math.floor(failedRequestsRaw)) : 0
    const costTotalRaw = Number(stat.cost_total ?? 0)
    const normalizedCost = Number.isFinite(costTotalRaw) ? Math.max(costTotalRaw, 0) : 0
    const costDisplay = buildProviderCostDisplay(normalizedCost, getLocale() || 'en')
    const performanceHint = t('components.main.providers.performanceHint', {
      ttftSamples: formatMetric(ttftSampleCount),
      tpsSamples: formatMetric(tpsSampleCount),
    })

    return {
      state: 'ready',
      requests: `${t('components.main.providers.requests')}: ${formatMetric(stat.total_requests)}`,
      tokens: `${t('components.main.providers.tokens')}: ${formatTokenNumber(totalTokens)}`,
      costLabel: t('components.main.providers.cost'),
      costParts: costDisplay.parts,
      costFormatted: costDisplay.formatted,
      costValue: normalizedCost,
      ttft: formatAverageFirstTokenMs(stat.avg_first_token_sec),
      tps: formatAverageTokensPerSecond(stat.avg_tokens_per_sec),
      performanceHint,
      successRateLabel,
      successRateClass,
      failedRequests,
      unreadFailedRequests,
      hasUnreadErrorLogs,
    }
  }

  const startProviderStatsTimer = () => {
    stopProviderStatsTimer()
    providerStatsTimer = window.setInterval(() => {
      void loadAllProviderStats()
      void refreshAvailabilityResults()
    }, 60_000)
  }

  const stopProviderStatsTimer = () => {
    if (providerStatsTimer) {
      clearInterval(providerStatsTimer)
      providerStatsTimer = undefined
    }
  }

  return {
    loadProviderStats,
    loadAllProviderStats,
    providerStatDisplay,
    refreshProviderPricingCachesOnStartup,
    handleProviderCardClick,
    startProviderStatsTimer,
    stopProviderStatsTimer,
  }
}
