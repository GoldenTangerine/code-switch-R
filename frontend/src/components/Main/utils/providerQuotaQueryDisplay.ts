import type { AutomationCard } from '../../../data/cards'
import { queryProviderQuota } from '../../../services/providerQuotaQuery'
import { roundBudgetValue } from '../../../utils/budgetUsage'
import { hasProviderQuotaQueryType, normalizeProviderQuotaQueryType } from '../../../utils/providerQuotaQuery'
import type { TranslateFn } from '../types'
import {
  formatProviderQuotaCountdownLabel,
  providerQuotaLabelKeyMap,
  type ProviderQuotaSnapshotItem,
} from './providerQuotaSnapshot'

function resolveProgressRatio(used: number, total: number) {
  if (!Number.isFinite(total) || total <= 0) return 0
  if (!Number.isFinite(used) || used <= 0) return 0
  return used / total
}

function normalizeDisplayValue(value: number) {
  if (!Number.isFinite(value)) return 0
  return Math.max(value, 0)
}

export async function resolveProviderQuotaQueryDisplay({
  card,
  now,
  t,
}: {
  card: AutomationCard
  now: Date
  t: TranslateFn
}): Promise<ProviderQuotaSnapshotItem[]> {
  if (!hasProviderQuotaQueryType(card.providerQuotaQueryType)) {
    return []
  }

  const queryType = normalizeProviderQuotaQueryType(card.providerQuotaQueryType)
  try {
    const response = await queryProviderQuota(queryType, card.apiUrl, card.apiKey)
    if (!response?.success || !Array.isArray(response.items) || response.items.length === 0) {
      if (response?.error) {
        console.warn(`[ProviderQuotaQuery] ${card.name} query failed: ${response.error}`)
      }
      return []
    }

    return response.items
      .filter((item) => item && providerQuotaLabelKeyMap[item.key])
      .map((item) => {
        const used = normalizeDisplayValue(Number(item.used))
        const total = normalizeDisplayValue(Number(item.total))
        const nextReset = item.nextReset ? new Date(item.nextReset) : null
        const isActive = item.active !== false
        return {
          key: item.key,
          label: t(providerQuotaLabelKeyMap[item.key]),
          used,
          total,
          progressRatio: resolveProgressRatio(used, total),
          countdownLabel: isActive
            ? formatProviderQuotaCountdownLabel(nextReset, now)
            : t('components.main.providers.quotaInactive'),
          nextReset,
          trackedUsed: used,
          adjustment: 0,
          remaining: roundBudgetValue(total - used),
          isActive,
          valueMode: 'count' as const,
        }
      })
  } catch (error) {
    console.warn(`[ProviderQuotaQuery] ${card.name} query crashed:`, error)
    return []
  }
}
