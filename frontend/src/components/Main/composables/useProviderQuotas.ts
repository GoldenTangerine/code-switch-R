import { reactive, onUnmounted } from 'vue'
import type { AutomationCard } from '../../../data/cards'
import type { LogPlatform } from '../../../services/logs'
import {
  budgetQuotaOrder,
  normalizeBudgetQuotaSettings,
} from '../../../utils/budgetUsage'
import { cardProviderRef } from '../adapters/providerCardMappers'
import type { ProviderQuotaDisplayItem, ProviderTab, TranslateFn } from '../types'
import {
  formatProviderQuotaCountdownLabel,
  hasProviderQuotaCountdownCrossedReset,
  resolveProviderQuotaSnapshot,
} from '../utils/providerQuotaSnapshot'

type UseProviderQuotasOptions = {
  t: TranslateFn
  getActiveTab: () => ProviderTab
  cards: Record<ProviderTab, AutomationCard[]>
}

const QUOTA_REFRESH_INTERVAL_MS = 30_000
const COUNTDOWN_TICK_INTERVAL_MS = 1_000

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
    return resolveProviderQuotaSnapshot({
      card,
      platform,
      now,
      t,
    })
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
        item.countdownLabel = formatProviderQuotaCountdownLabel(item.nextReset, now)
        // 倒计时刚归零时触发一次数据刷新，使进度条和已用金额及时更新
        if (hasProviderQuotaCountdownCrossedReset(item.nextReset, previousTickAt, now)) {
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
