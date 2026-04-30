import { reactive, onUnmounted } from 'vue'
import type { AutomationCard } from '../../../data/cards'
import type { LogPlatform } from '../../../services/logs'
import {
  normalizeBudgetQuotaSettings,
  providerBudgetQuotaOrder,
} from '../../../utils/budgetUsage'
import {
  hasProviderQuotaQueryType,
  normalizeProviderQuotaQueryConfig,
  resolveProviderQuotaAutoQueryIntervalMinutes,
  resolveProviderQuotaQueryType,
  sanitizeProviderQuotaQueryConfigForSave,
} from '../../../utils/providerQuotaQuery'
import { cardProviderRef } from '../adapters/providerCardMappers'
import type { ProviderQuotaDisplayItem, ProviderTab, TranslateFn } from '../types'
import {
  formatProviderQuotaCountdownLabel,
  hasProviderQuotaCountdownCrossedReset,
  resolveProviderQuotaSnapshot,
  type ProviderQuotaSnapshotItem,
} from '../utils/providerQuotaSnapshot'
import { resolveProviderQuotaQueryDisplay } from '../utils/providerQuotaQueryDisplay'

type UseProviderQuotasOptions = {
  t: TranslateFn
  getActiveTab: () => ProviderTab
  cards: Record<ProviderTab, AutomationCard[]>
  resolveAutoRefreshRemoteQuotaRefs?: () => Set<string>
}

const QUOTA_REFRESH_INTERVAL_MS = 30_000
const COUNTDOWN_TICK_INTERVAL_MS = 1_000

const createQuotaDisplayMap = (): Record<ProviderTab, Record<string, ProviderQuotaDisplayItem[]>> => ({
  claude: {},
  codex: {},
  gemini: {},
  opencode: {},
  others: {},
})

const createQuotaRefreshStateMap = (): Record<ProviderTab, Record<string, boolean>> => ({
  claude: {},
  codex: {},
  gemini: {},
  opencode: {},
  others: {},
})

type RemoteQuotaCacheEntry = {
  items: ProviderQuotaSnapshotItem[]
  fetchedAt: number
  cacheIdentity: string
  staleFallbackCount: number
}

const createRemoteQuotaCacheMap = (): Record<ProviderTab, Record<string, RemoteQuotaCacheEntry>> => ({
  claude: {},
  codex: {},
  gemini: {},
  opencode: {},
  others: {},
})

const tabToPlatform = (tab: ProviderTab): LogPlatform | '' => {
  if (tab === 'claude' || tab === 'codex' || tab === 'gemini') return tab
  return ''
}

function buildRemoteQuotaCacheIdentity(card: AutomationCard): string {
  const normalizedConfig = normalizeProviderQuotaQueryConfig(
    card.providerQuotaQueryConfig,
    card.providerQuotaQueryType,
  )
  return JSON.stringify([
    resolveProviderQuotaQueryType(normalizedConfig ?? card.providerQuotaQueryType),
    sanitizeProviderQuotaQueryConfigForSave(normalizedConfig, card.providerQuotaQueryType) ?? null,
    String(card.apiUrl ?? '').trim(),
    String(card.apiKey ?? '').trim(),
  ])
}

function resolveRemoteQuotaCacheTTL(card: AutomationCard): number {
  const normalizedConfig = normalizeProviderQuotaQueryConfig(
    card.providerQuotaQueryConfig,
    card.providerQuotaQueryType,
  )
  const autoQueryIntervalMinutes = resolveProviderQuotaAutoQueryIntervalMinutes(
    normalizedConfig,
    card.providerQuotaQueryType,
  )

  if (autoQueryIntervalMinutes <= 0) {
    return Number.POSITIVE_INFINITY
  }

  return autoQueryIntervalMinutes * 60_000
}

/**
 * @author sm
 * 供应商级别预算额度状态管理
 * 定时获取每个供应商各周期的已用金额，计算进度和倒计时
 */
export function useProviderQuotas(options: UseProviderQuotasOptions) {
  const { t, getActiveTab, cards, resolveAutoRefreshRemoteQuotaRefs } = options

  // tab -> providerRef -> ProviderQuotaDisplayItem[]
  const quotaDisplayMap = reactive(createQuotaDisplayMap())
  const quotaRefreshStateMap = reactive(createQuotaRefreshStateMap())
  const remoteQuotaCacheMap = createRemoteQuotaCacheMap()

  let refreshTimer: ReturnType<typeof globalThis.setInterval> | undefined
  let countdownTimer: ReturnType<typeof globalThis.setInterval> | undefined
  let lastCountdownTickAt: Date | null = null
  let refreshTask: Promise<void> | null = null
  let refreshQueued = false
  let queuedForceRemoteRefs = new Set<string>()
  let queuedAutoRefreshRemoteRefs: Set<string> | null = null
  let queuedTargetRefs: Set<string> | null = null

  const cloneRemoteQuotaItems = (
    items: ProviderQuotaSnapshotItem[],
    now: Date,
  ): ProviderQuotaSnapshotItem[] => (
    items.map((item) => {
      const nextReset = item.nextReset ? new Date(item.nextReset) : null
      return {
        ...item,
        nextReset,
        countdownLabel: item.isActive
          ? formatProviderQuotaCountdownLabel(nextReset, now)
          : t('components.main.providers.quotaInactive'),
      }
    })
  )

  const buildRemoteQuotaErrorItems = ({
    queriedAt,
    message,
  }: {
    queriedAt: number
    message: string
  }): ProviderQuotaSnapshotItem[] => {
    const normalizedMessage = `${message ?? ''}`.trim() || t('components.main.providers.quotaQueryFailed')
    return [{
      key: 'remote_quota_query_error',
      label: t('components.main.providers.quotaQueryStatusLabel'),
      used: 0,
      total: 0,
      progressRatio: 0,
      countdownLabel: '',
      nextReset: null,
      queriedAt,
      trackedUsed: 0,
      adjustment: 0,
      remaining: 0,
      isActive: false,
      valueMode: 'currency',
      invalidMessage: normalizedMessage,
    }]
  }

  const attachRefreshErrorMessage = (
    items: ProviderQuotaSnapshotItem[],
    now: Date,
    message: string,
  ): ProviderQuotaSnapshotItem[] => {
    const normalizedMessage = `${message ?? ''}`.trim() || t('components.main.providers.quotaQueryFailed')
    const fallbackMessage = t('components.main.providers.quotaRefreshFailedCached', {
      reason: normalizedMessage,
    })

    return cloneRemoteQuotaItems(items, now).map((item) => {
      const existingExtra = `${item.extra ?? ''}`.trim()
      return {
        ...item,
        refreshErrorMessage: fallbackMessage,
        extra: existingExtra || undefined,
      }
    })
  }

  const resolveRemoteQuotaForCard = async ({
    card,
    tab,
    ref,
    now,
    forceRefresh,
  }: {
    card: AutomationCard
    tab: ProviderTab
    ref: string
    now: Date
    forceRefresh: boolean
  }): Promise<ProviderQuotaSnapshotItem[]> => {
    const tabCache = remoteQuotaCacheMap[tab]
    const cacheEntry = tabCache[ref]
    const cacheIdentity = buildRemoteQuotaCacheIdentity(card)
    const cacheTTL = resolveRemoteQuotaCacheTTL(card)
    const cacheMatchesIdentity = cacheEntry?.cacheIdentity === cacheIdentity
    const cacheIsFresh = cacheMatchesIdentity && cacheEntry
      ? (now.getTime() - cacheEntry.fetchedAt) < cacheTTL
      : false

    if (cacheEntry && cacheIsFresh && !forceRefresh) {
      return cloneRemoteQuotaItems(cacheEntry.items, now)
    }

    const queryResult = await resolveProviderQuotaQueryDisplay({
      card,
      now,
      t,
    })
    const fetchedItems = queryResult.items
    const failureMessage = `${queryResult.failureMessage ?? ''}`.trim()
    const failureQueriedAt = Number.isFinite(queryResult.queriedAt)
      ? Number(queryResult.queriedAt)
      : now.getTime()

    if (fetchedItems.length > 0) {
      const cachedItems = cloneRemoteQuotaItems(fetchedItems, now)
      tabCache[ref] = {
        items: cachedItems,
        fetchedAt: now.getTime(),
        cacheIdentity,
        staleFallbackCount: 0,
      }
      return cloneRemoteQuotaItems(cachedItems, now)
    }

    if (cacheEntry && cacheMatchesIdentity) {
      if (forceRefresh) {
        return attachRefreshErrorMessage(cacheEntry.items, now, failureMessage)
      }
      if (cacheEntry.staleFallbackCount >= 1) {
        delete tabCache[ref]
        return failureMessage
          ? buildRemoteQuotaErrorItems({ queriedAt: failureQueriedAt, message: failureMessage })
          : []
      }
      tabCache[ref] = {
        ...cacheEntry,
        staleFallbackCount: cacheEntry.staleFallbackCount + 1,
      }
      return cloneRemoteQuotaItems(cacheEntry.items, now)
    }

    if (forceRefresh || !cacheMatchesIdentity) {
      delete tabCache[ref]
      return failureMessage
        ? buildRemoteQuotaErrorItems({ queriedAt: failureQueriedAt, message: failureMessage })
        : []
    }

    return []
  }

  const resolveQuotaForCard = async (
    card: AutomationCard,
    tab: ProviderTab,
    ref: string,
    platform: LogPlatform | '',
    now: Date,
    forceRemoteRefresh: boolean,
  ): Promise<ProviderQuotaDisplayItem[]> => {
    if (hasProviderQuotaQueryType(card.providerQuotaQueryConfig ?? card.providerQuotaQueryType, card.providerQuotaQueryType)) {
      return resolveRemoteQuotaForCard({
        card,
        tab,
        ref,
        now,
        forceRefresh: forceRemoteRefresh,
      })
    }

    return resolveProviderQuotaSnapshot({
      card,
      platform,
      now,
      t,
    })
  }

  const runRefreshProviderQuotas = async (
    forceRemoteRefs: Set<string>,
    autoRefreshRemoteRefs?: Set<string>,
    targetRefs?: Set<string>,
  ) => {
    try {
      const tab = getActiveTab()
      const platform = tabToPlatform(tab)
      const tabCards = cards[tab] ?? []
      const tabQuotaDisplayMap = quotaDisplayMap[tab]
      const tabQuotaRefreshStateMap = quotaRefreshStateMap[tab]
      const tabRemoteQuotaCache = remoteQuotaCacheMap[tab]
      const now = new Date()
      const currentRefs = new Set<string>()
      const isPartialRefresh = targetRefs !== undefined

      const tasks = tabCards.map(async (card) => {
        const ref = cardProviderRef(card) || card.name
        if (!ref) return
        currentRefs.add(ref)
        if (targetRefs && !targetRefs.has(ref)) return

        const settings = normalizeBudgetQuotaSettings(card.budgetQuotaSettings)
        const hasQuota = providerBudgetQuotaOrder.some((key) => settings[key].total > 0)
        const hasRemoteQuota = hasProviderQuotaQueryType(
          card.providerQuotaQueryConfig ?? card.providerQuotaQueryType,
          card.providerQuotaQueryType,
        )
        if (!hasQuota && !hasRemoteQuota) {
          // 清理无额度的供应商
          if (tabQuotaDisplayMap[ref]) {
            delete tabQuotaDisplayMap[ref]
          }
          if (tabQuotaRefreshStateMap[ref] !== undefined) {
            delete tabQuotaRefreshStateMap[ref]
          }
          if (tabRemoteQuotaCache[ref]) {
            delete tabRemoteQuotaCache[ref]
          }
          return
        }

        if (!hasRemoteQuota && tabRemoteQuotaCache[ref]) {
          delete tabRemoteQuotaCache[ref]
        }
        if (!hasRemoteQuota && tabQuotaRefreshStateMap[ref] !== undefined) {
          delete tabQuotaRefreshStateMap[ref]
        }

        if (
          hasRemoteQuota
          && autoRefreshRemoteRefs
          && !forceRemoteRefs.has(ref)
          && !autoRefreshRemoteRefs.has(ref)
        ) {
          if (!tabQuotaDisplayMap[ref] && tabRemoteQuotaCache[ref]?.cacheIdentity === buildRemoteQuotaCacheIdentity(card)) {
            tabQuotaDisplayMap[ref] = cloneRemoteQuotaItems(tabRemoteQuotaCache[ref].items, now)
          }
          tabQuotaRefreshStateMap[ref] = false
          return
        }

        if (hasRemoteQuota) {
          tabQuotaRefreshStateMap[ref] = true
        }

        try {
          const items = await resolveQuotaForCard(
            card,
            tab,
            ref,
            platform,
            now,
            forceRemoteRefs.has(ref),
          )
          tabQuotaDisplayMap[ref] = items
        } finally {
          if (hasRemoteQuota) {
            tabQuotaRefreshStateMap[ref] = false
          }
        }
      })

      await Promise.all(tasks)

      if (!isPartialRefresh) {
        for (const ref of Object.keys(tabQuotaDisplayMap)) {
          if (!currentRefs.has(ref)) {
            delete tabQuotaDisplayMap[ref]
          }
        }
        for (const ref of Object.keys(tabRemoteQuotaCache)) {
          if (!currentRefs.has(ref)) {
            delete tabRemoteQuotaCache[ref]
          }
        }
        for (const ref of Object.keys(tabQuotaRefreshStateMap)) {
          if (!currentRefs.has(ref)) {
            delete tabQuotaRefreshStateMap[ref]
          }
        }
      }
    } catch (error) {
      console.error('[ProviderQuota] refresh failed:', error)
    }
  }

  const refreshProviderQuotas = (
    options: {
      forceRemoteRefs?: Set<string>
      autoRefreshRemoteRefs?: Set<string>
      targetRefs?: Set<string>
    } = {},
  ) => {
    const hadPendingRefresh = refreshQueued || refreshTask !== null
    refreshQueued = true
    options.forceRemoteRefs?.forEach((ref) => queuedForceRemoteRefs.add(ref))
    const hasAutoRefreshLimit = Object.prototype.hasOwnProperty.call(options, 'autoRefreshRemoteRefs')
    if (!hadPendingRefresh) {
      queuedAutoRefreshRemoteRefs = hasAutoRefreshLimit
        ? new Set(options.autoRefreshRemoteRefs ?? [])
        : null
    } else if (!hasAutoRefreshLimit) {
      queuedAutoRefreshRemoteRefs = null
    } else if (queuedAutoRefreshRemoteRefs !== null) {
      options.autoRefreshRemoteRefs?.forEach((ref) => queuedAutoRefreshRemoteRefs?.add(ref))
    }
    const hasTargetLimit = Object.prototype.hasOwnProperty.call(options, 'targetRefs')
    if (!hadPendingRefresh) {
      queuedTargetRefs = hasTargetLimit
        ? new Set(options.targetRefs ?? [])
        : null
    } else if (!hasTargetLimit) {
      queuedTargetRefs = null
    } else if (queuedTargetRefs !== null) {
      options.targetRefs?.forEach((ref) => queuedTargetRefs?.add(ref))
    }

    if (!refreshTask) {
      refreshTask = (async () => {
        try {
          while (refreshQueued) {
            refreshQueued = false
            const nextForceRemoteRefs = queuedForceRemoteRefs
            queuedForceRemoteRefs = new Set<string>()
            const nextAutoRefreshRemoteRefs = queuedAutoRefreshRemoteRefs
            queuedAutoRefreshRemoteRefs = null
            const nextTargetRefs = queuedTargetRefs
            queuedTargetRefs = null
            await runRefreshProviderQuotas(
              nextForceRemoteRefs,
              nextAutoRefreshRemoteRefs ?? undefined,
              nextTargetRefs ?? undefined,
            )
          }
        } finally {
          refreshTask = null
        }
      })()
    }

    return refreshTask
  }

  const updateCountdowns = () => {
    const tabQuotaDisplayMap = quotaDisplayMap[getActiveTab()]
    const autoRefreshRemoteRefs = resolveAutoRefreshRemoteQuotaRefs?.()
    const now = new Date()
    const previousTickAt = lastCountdownTickAt ?? new Date(now.getTime() - COUNTDOWN_TICK_INTERVAL_MS)
    let needsRefresh = false
    const forceRemoteRefs = new Set<string>()
    for (const ref in tabQuotaDisplayMap) {
      const items = tabQuotaDisplayMap[ref]
      if (!items) continue
      for (const item of items) {
        if (!item.nextReset) continue
        item.countdownLabel = formatProviderQuotaCountdownLabel(item.nextReset, now)
        // 倒计时刚归零时触发一次数据刷新，使进度条和已用金额及时更新
        if (hasProviderQuotaCountdownCrossedReset(item.nextReset, previousTickAt, now)) {
          needsRefresh = true
          const canAutoRefreshRemote = !autoRefreshRemoteRefs || autoRefreshRemoteRefs.has(ref)
          if (item.valueMode === 'count' && canAutoRefreshRemote) {
            forceRemoteRefs.add(ref)
          }
        }
      }
    }
    lastCountdownTickAt = now
    if (needsRefresh) {
      void refreshProviderQuotas({
        forceRemoteRefs,
        ...(autoRefreshRemoteRefs ? { autoRefreshRemoteRefs } : {}),
      })
    }
  }

  const getQuotaDisplay = (card: AutomationCard): ProviderQuotaDisplayItem[] => {
    const ref = cardProviderRef(card) || card.name
    return quotaDisplayMap[getActiveTab()][ref] ?? []
  }

  const isQuotaRefreshing = (card: AutomationCard) => {
    const ref = cardProviderRef(card) || card.name
    return quotaRefreshStateMap[getActiveTab()][ref] === true
  }

  const startTimers = () => {
    stopTimers()
    lastCountdownTickAt = new Date()
    refreshTimer = globalThis.setInterval(() => {
      const autoRefreshRemoteRefs = resolveAutoRefreshRemoteQuotaRefs?.()
      void refreshProviderQuotas(
        autoRefreshRemoteRefs
          ? { autoRefreshRemoteRefs }
          : undefined,
      )
    }, QUOTA_REFRESH_INTERVAL_MS)
    countdownTimer = globalThis.setInterval(updateCountdowns, COUNTDOWN_TICK_INTERVAL_MS)
  }

  const stopTimers = () => {
    if (refreshTimer !== undefined) {
      globalThis.clearInterval(refreshTimer)
      refreshTimer = undefined
    }
    if (countdownTimer !== undefined) {
      globalThis.clearInterval(countdownTimer)
      countdownTimer = undefined
    }
    lastCountdownTickAt = null
  }

  onUnmounted(stopTimers)

  return {
    quotaDisplayMap,
    quotaRefreshStateMap,
    refreshProviderQuotas,
    getQuotaDisplay,
    isQuotaRefreshing,
    startTimers,
    stopTimers,
  }
}
