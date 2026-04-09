import { reactive, onUnmounted } from 'vue'
import type { AutomationCard } from '../../../data/cards'
import type { LogPlatform } from '../../../services/logs'
import { fetchCostSinceByProvider, fetchFiveHourQuotaStatusByProvider } from '../../../services/logs'
import {
  budgetQuotaOrder,
  normalizeBudgetQuotaAdjustments,
  normalizeBudgetUsedDisplay,
  normalizeBudgetQuotaSettings,
  resolveBudgetQuotaWindow,
  formatLocalDateTime,
  type BudgetQuotaKey,
} from '../../../utils/budgetUsage'
import { cardProviderRef } from '../adapters/providerCardMappers'
import type { ProviderQuotaDisplayItem, ProviderTab, TranslateFn } from '../types'

type UseProviderQuotasOptions = {
  t: TranslateFn
  getActiveTab: () => ProviderTab
  cards: Record<ProviderTab, AutomationCard[]>
}

const QUOTA_REFRESH_INTERVAL_MS = 30_000
const COUNTDOWN_TICK_INTERVAL_MS = 1_000
const MINUTE_MS = 60_000
const HOUR_MS = 60 * MINUTE_MS
const DAY_MS = 24 * HOUR_MS

const quotaLabelKey: Record<BudgetQuotaKey, string> = {
  five_hour: 'components.main.providers.quotaFiveHour',
  daily: 'components.main.providers.quotaDaily',
  weekly: 'components.main.providers.quotaWeekly',
  monthly: 'components.main.providers.quotaMonthly',
}

const createQuotaDisplayMap = (): Record<ProviderTab, Record<string, ProviderQuotaDisplayItem[]>> => ({
  claude: {},
  codex: {},
  gemini: {},
  others: {},
})

const tabToPlatform = (tab: ProviderTab): LogPlatform | '' => {
  if (tab === 'claude' || tab === 'codex' || tab === 'gemini') return tab
  return ''
}

const formatQuotaCountdown = (remainingMs: number) => {
  if (remainingMs <= 0) return '0h0m'

  const remainingDays = Math.floor(remainingMs / DAY_MS)
  if (remainingDays >= 1) {
    const remainingHours = Math.floor((remainingMs % DAY_MS) / HOUR_MS)
    return `${remainingDays}d${remainingHours}h`
  }

  const totalMinutes = Math.max(Math.floor(remainingMs / MINUTE_MS), 1)
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  return `${hours}h${minutes}m`
}

const formatCountdown = (nextReset: Date | null, now: Date): string => {
  if (!nextReset) return ''
  const remaining = nextReset.getTime() - now.getTime()
  return formatQuotaCountdown(remaining)
}

const hasCountdownCrossedReset = (nextReset: Date | null, previousTickAt: Date, now: Date) => {
  if (!nextReset) return false
  const nextResetTime = nextReset.getTime()
  return nextResetTime <= now.getTime() && nextResetTime > previousTickAt.getTime()
}

const resolveProgressRatio = (used: number, total: number) => {
  if (!Number.isFinite(total) || total <= 0) return 0
  return normalizeBudgetUsedDisplay(used) / total
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

  let refreshTimer: ReturnType<typeof globalThis.setInterval> | undefined
  let countdownTimer: ReturnType<typeof globalThis.setInterval> | undefined
  let lastCountdownTickAt: Date | null = null
  let refreshInFlight = false
  let refreshQueued = false

  const resolveQuotaForCard = async (
    card: AutomationCard,
    platform: LogPlatform | '',
    now: Date,
  ): Promise<ProviderQuotaDisplayItem[]> => {
    const settings = normalizeBudgetQuotaSettings(card.budgetQuotaSettings)
    const adjustments = normalizeBudgetQuotaAdjustments(card.budgetQuotaUsedAdjustments)
    const visibleKeys = budgetQuotaOrder.filter((key) => settings[key].total > 0)
    if (visibleKeys.length === 0) return []

    const ref = cardProviderRef(card)
    const items: ProviderQuotaDisplayItem[] = []

    for (const key of visibleKeys) {
      const setting = settings[key]
      try {
        let used = 0
        let nextReset: Date | null = null

        if (key === 'five_hour') {
          const status = await fetchFiveHourQuotaStatusByProvider(platform, ref, card.name)
          if (status.active) {
            used = normalizeBudgetUsedDisplay(status.used + adjustments[key])
            nextReset = new Date(status.next_reset)
          }
          items.push({
            key,
            label: t(quotaLabelKey[key]),
            used,
            total: setting.total,
            progressRatio: resolveProgressRatio(used, setting.total),
            countdownLabel: status.active
              ? formatCountdown(nextReset, now)
              : t('components.main.providers.quotaInactive'),
            nextReset,
          })
          continue
        } else {
          const window = resolveBudgetQuotaWindow(key, setting, now)
          nextReset = window.nextReset
          const startStr = formatLocalDateTime(window.start)
          const trackedUsed = await fetchCostSinceByProvider(startStr, platform, ref, card.name)
          used = normalizeBudgetUsedDisplay(trackedUsed + adjustments[key])
        }

        const progressRatio = resolveProgressRatio(used, setting.total)

        items.push({
          key,
          label: t(quotaLabelKey[key]),
          used,
          total: setting.total,
          progressRatio,
          countdownLabel: formatCountdown(nextReset, now),
          nextReset,
        })
      } catch (error) {
        console.warn(`[ProviderQuota] Failed to resolve ${key} for ${card.name}:`, error)
      }
    }

    return items
  }

  const refreshProviderQuotas = async () => {
    if (refreshInFlight) {
      // 正在刷新时排队，等当前结束后自动重跑，避免丢失切 tab 等触发
      refreshQueued = true
      return
    }
    refreshInFlight = true

    try {
      const tab = getActiveTab()
      const platform = tabToPlatform(tab)
      const tabCards = cards[tab] ?? []
      const tabQuotaDisplayMap = quotaDisplayMap[tab]
      const now = new Date()
      const currentRefs = new Set<string>()

      const tasks = tabCards.map(async (card) => {
        const ref = cardProviderRef(card) || card.name
        if (!ref) return
        currentRefs.add(ref)

        const settings = normalizeBudgetQuotaSettings(card.budgetQuotaSettings)
        const hasQuota = budgetQuotaOrder.some((key) => settings[key].total > 0)
        if (!hasQuota) {
          // 清理无额度的供应商
          if (tabQuotaDisplayMap[ref]) {
            delete tabQuotaDisplayMap[ref]
          }
          return
        }

        const items = await resolveQuotaForCard(card, platform, now)
        tabQuotaDisplayMap[ref] = items
      })

      await Promise.all(tasks)

      for (const ref of Object.keys(tabQuotaDisplayMap)) {
        if (!currentRefs.has(ref)) {
          delete tabQuotaDisplayMap[ref]
        }
      }
    } catch (error) {
      console.error('[ProviderQuota] refresh failed:', error)
    } finally {
      refreshInFlight = false
      // 如果排队了新请求，立即执行
      if (refreshQueued) {
        refreshQueued = false
        void refreshProviderQuotas()
      }
    }
  }

  const updateCountdowns = () => {
    const tabQuotaDisplayMap = quotaDisplayMap[getActiveTab()]
    const now = new Date()
    const previousTickAt = lastCountdownTickAt ?? new Date(now.getTime() - COUNTDOWN_TICK_INTERVAL_MS)
    let needsRefresh = false
    for (const ref in tabQuotaDisplayMap) {
      const items = tabQuotaDisplayMap[ref]
      if (!items) continue
      for (const item of items) {
        if (!item.nextReset) continue
        item.countdownLabel = formatCountdown(item.nextReset, now)
        // 倒计时刚归零时触发一次数据刷新，使进度条和已用金额及时更新
        if (hasCountdownCrossedReset(item.nextReset, previousTickAt, now)) {
          needsRefresh = true
        }
      }
    }
    lastCountdownTickAt = now
    if (needsRefresh) {
      void refreshProviderQuotas()
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
