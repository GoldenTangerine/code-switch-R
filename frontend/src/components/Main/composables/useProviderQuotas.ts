import { reactive, onUnmounted } from 'vue'
import type { AutomationCard } from '../../../data/cards'
import type { LogPlatform } from '../../../services/logs'
import {
  normalizeBudgetQuotaSettings,
  providerBudgetQuotaOrder,
} from '../../../utils/budgetUsage'
import {
  hasProviderQuotaQueryType,
  normalizeProviderQuotaQueryType,
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
}

const QUOTA_REFRESH_INTERVAL_MS = 30_000
const REMOTE_QUOTA_CACHE_TTL_MS = 5 * 60_000
const COUNTDOWN_TICK_INTERVAL_MS = 1_000

const createQuotaDisplayMap = (): Record<ProviderTab, Record<string, ProviderQuotaDisplayItem[]>> => ({
  claude: {},
  codex: {},
  gemini: {},
  others: {},
})

type RemoteQuotaCacheEntry = {
  items: ProviderQuotaSnapshotItem[]
  fetchedAt: number
  cacheIdentity: string
}

const createRemoteQuotaCacheMap = (): Record<ProviderTab, Record<string, RemoteQuotaCacheEntry>> => ({
  claude: {},
  codex: {},
  gemini: {},
  others: {},
})

const tabToPlatform = (tab: ProviderTab): LogPlatform | '' => {
  if (tab === 'claude' || tab === 'codex' || tab === 'gemini') return tab
  return ''
}

function buildRemoteQuotaCacheIdentity(card: AutomationCard): string {
  return JSON.stringify([
    normalizeProviderQuotaQueryType(card.providerQuotaQueryType),
    String(card.apiUrl ?? '').trim(),
    String(card.apiKey ?? '').trim(),
  ])
}

/**
 * @author sm
 * 供应商级别预算额度状态管理
 * 定时获取每个供应商各周期的已用金额，计算进度和倒计时
 */
export function useProviderQuotas(options: UseProviderQuotasOptions) {
  const { t, getActiveTab, cards } = options

  // tab -> providerRef -> ProviderQuotaDisplayItem[]
  const quotaDisplayMap = reactive(createQuotaDisplayMap())
  const remoteQuotaCacheMap = createRemoteQuotaCacheMap()

  let refreshTimer: ReturnType<typeof globalThis.setInterval> | undefined
  let countdownTimer: ReturnType<typeof globalThis.setInterval> | undefined
  let lastCountdownTickAt: Date | null = null
  let refreshTask: Promise<void> | null = null
  let refreshQueued = false
  let queuedForceRemoteRefs = new Set<string>()

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
    const cacheMatchesIdentity = cacheEntry?.cacheIdentity === cacheIdentity
    const cacheIsFresh = cacheMatchesIdentity && cacheEntry
      ? (now.getTime() - cacheEntry.fetchedAt) < REMOTE_QUOTA_CACHE_TTL_MS
      : false

    if (cacheEntry && cacheIsFresh && !forceRefresh) {
      return cloneRemoteQuotaItems(cacheEntry.items, now)
    }

    const fetchedItems = await resolveProviderQuotaQueryDisplay({
      card,
      now,
      t,
    })

    if (fetchedItems.length > 0) {
      const cachedItems = cloneRemoteQuotaItems(fetchedItems, now)
      tabCache[ref] = {
        items: cachedItems,
        fetchedAt: now.getTime(),
        cacheIdentity,
      }
      return cloneRemoteQuotaItems(cachedItems, now)
    }

    if (forceRefresh || !cacheMatchesIdentity) {
      delete tabCache[ref]
      return []
    }

    if (cacheEntry && cacheMatchesIdentity) {
      return cloneRemoteQuotaItems(cacheEntry.items, now)
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
    if (hasProviderQuotaQueryType(card.providerQuotaQueryType)) {
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

  const runRefreshProviderQuotas = async (forceRemoteRefs: Set<string>) => {
    try {
      const tab = getActiveTab()
      const platform = tabToPlatform(tab)
      const tabCards = cards[tab] ?? []
      const tabQuotaDisplayMap = quotaDisplayMap[tab]
      const tabRemoteQuotaCache = remoteQuotaCacheMap[tab]
      const now = new Date()
      const currentRefs = new Set<string>()

      const tasks = tabCards.map(async (card) => {
        const ref = cardProviderRef(card) || card.name
        if (!ref) return
        currentRefs.add(ref)

        const settings = normalizeBudgetQuotaSettings(card.budgetQuotaSettings)
        const hasQuota = providerBudgetQuotaOrder.some((key) => settings[key].total > 0)
        const hasRemoteQuota = hasProviderQuotaQueryType(card.providerQuotaQueryType)
        if (!hasQuota && !hasRemoteQuota) {
          // 清理无额度的供应商
          if (tabQuotaDisplayMap[ref]) {
            delete tabQuotaDisplayMap[ref]
          }
          if (tabRemoteQuotaCache[ref]) {
            delete tabRemoteQuotaCache[ref]
          }
          return
        }

        if (!hasRemoteQuota && tabRemoteQuotaCache[ref]) {
          delete tabRemoteQuotaCache[ref]
        }

        const items = await resolveQuotaForCard(
          card,
          tab,
          ref,
          platform,
          now,
          forceRemoteRefs.has(ref),
        )
        tabQuotaDisplayMap[ref] = items
      })

      await Promise.all(tasks)

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
    } catch (error) {
      console.error('[ProviderQuota] refresh failed:', error)
    }
  }

  const refreshProviderQuotas = (
    options: {
      forceRemoteRefs?: Set<string>
    } = {},
  ) => {
    refreshQueued = true
    options.forceRemoteRefs?.forEach((ref) => queuedForceRemoteRefs.add(ref))

    if (!refreshTask) {
      refreshTask = (async () => {
        try {
          while (refreshQueued) {
            refreshQueued = false
            const nextForceRemoteRefs = queuedForceRemoteRefs
            queuedForceRemoteRefs = new Set<string>()
            await runRefreshProviderQuotas(nextForceRemoteRefs)
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
          if (item.valueMode === 'count') {
            forceRemoteRefs.add(ref)
          }
        }
      }
    }
    lastCountdownTickAt = now
    if (needsRefresh) {
      void refreshProviderQuotas({ forceRemoteRefs })
    }
  }

  const getQuotaDisplay = (card: AutomationCard): ProviderQuotaDisplayItem[] => {
    const ref = cardProviderRef(card) || card.name
    return quotaDisplayMap[getActiveTab()][ref] ?? []
  }

  const startTimers = () => {
    stopTimers()
    lastCountdownTickAt = new Date()
    refreshTimer = globalThis.setInterval(() => {
      void refreshProviderQuotas()
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
    refreshProviderQuotas,
    getQuotaDisplay,
    startTimers,
    stopTimers,
  }
}
