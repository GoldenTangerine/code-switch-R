import type { AutomationCard } from '../../../data/cards'
import {
  fetchCostSinceByProvider,
  fetchFiveHourQuotaStatusByProvider,
  type LogPlatform,
} from '../../../services/logs'
import {
  budgetQuotaOrder,
  formatLocalDateTime,
  normalizeBudgetQuotaAdjustments,
  normalizeBudgetQuotaSettings,
  normalizeBudgetUsedDisplay,
  resolveBudgetCurrentUsedValue,
  resolveBudgetQuotaWindow,
  roundBudgetValue,
  type BudgetQuotaKey,
} from '../../../utils/budgetUsage'
import { cardProviderRef } from '../adapters/providerCardMappers'
import type { ProviderQuotaDisplayItem, TranslateFn } from '../types'

const MINUTE_MS = 60_000
const HOUR_MS = 60 * MINUTE_MS
const DAY_MS = 24 * HOUR_MS

export const providerQuotaLabelKeyMap: Record<BudgetQuotaKey, string> = {
  five_hour: 'components.main.providers.quotaFiveHour',
  daily: 'components.main.providers.quotaDaily',
  weekly: 'components.main.providers.quotaWeekly',
  monthly: 'components.main.providers.quotaMonthly',
}

export type ProviderQuotaSnapshotItem = ProviderQuotaDisplayItem & {
  trackedUsed: number
  adjustment: number
  remaining: number
  isActive: boolean
}

export const formatProviderQuotaCountdown = (remainingMs: number) => {
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

export const formatProviderQuotaCountdownLabel = (nextReset: Date | null, now: Date): string => {
  if (!nextReset) return ''
  return formatProviderQuotaCountdown(nextReset.getTime() - now.getTime())
}

export const hasProviderQuotaCountdownCrossedReset = (
  nextReset: Date | null,
  previousTickAt: Date,
  now: Date,
) => {
  if (!nextReset) return false
  const nextResetTime = nextReset.getTime()
  return nextResetTime <= now.getTime() && nextResetTime > previousTickAt.getTime()
}

const resolveProgressRatio = (used: number, total: number) => {
  if (!Number.isFinite(total) || total <= 0) return 0
  return normalizeBudgetUsedDisplay(used) / total
}

export const resolveProviderQuotaSnapshot = async ({
  card,
  platform,
  now,
  t,
}: {
  card: AutomationCard
  platform: LogPlatform | ''
  now: Date
  t: TranslateFn
}): Promise<ProviderQuotaSnapshotItem[]> => {
  const settings = normalizeBudgetQuotaSettings(card.budgetQuotaSettings)
  const adjustments = normalizeBudgetQuotaAdjustments(card.budgetQuotaUsedAdjustments)
  const visibleKeys = budgetQuotaOrder.filter((key) => settings[key].total > 0)
  if (visibleKeys.length === 0) return []

  const providerRef = cardProviderRef(card) || card.name
  const items: ProviderQuotaSnapshotItem[] = []

  for (const key of visibleKeys) {
    const setting = settings[key]

    try {
      const adjustment = adjustments[key]
      let trackedUsed = 0
      let used = 0
      let nextReset: Date | null = null
      let isActive = true

      if (key === 'five_hour') {
        const status = await fetchFiveHourQuotaStatusByProvider(platform, providerRef, card.name)
        isActive = status.active
        if (status.active) {
          trackedUsed = roundBudgetValue(normalizeBudgetUsedDisplay(status.used))
          used = resolveBudgetCurrentUsedValue(trackedUsed, adjustment)
          nextReset = new Date(status.next_reset)
        }
      } else {
        const window = resolveBudgetQuotaWindow(key, setting, now)
        nextReset = window.nextReset
        trackedUsed = roundBudgetValue(normalizeBudgetUsedDisplay(await fetchCostSinceByProvider(
          formatLocalDateTime(window.start),
          platform,
          providerRef,
          card.name,
        )))
        used = resolveBudgetCurrentUsedValue(trackedUsed, adjustment)
      }

      items.push({
        key,
        label: t(providerQuotaLabelKeyMap[key]),
        used,
        total: setting.total,
        progressRatio: resolveProgressRatio(used, setting.total),
        countdownLabel: isActive
          ? formatProviderQuotaCountdownLabel(nextReset, now)
          : t('components.main.providers.quotaInactive'),
        nextReset,
        trackedUsed,
        adjustment,
        remaining: roundBudgetValue(setting.total - used),
        isActive,
      })
    } catch (error) {
      console.warn(`[ProviderQuotaSnapshot] Failed to resolve ${key} for ${card.name}:`, error)
    }
  }

  return items
}
