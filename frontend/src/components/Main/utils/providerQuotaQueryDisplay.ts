import type { AutomationCard } from '../../../data/cards'
import { queryProviderQuota } from '../../../services/providerQuotaQuery'
import { roundBudgetValue } from '../../../utils/budgetUsage'
import {
  hasProviderQuotaQueryType,
  normalizeProviderQuotaQueryConfig,
  resolveProviderQuotaQueryType,
} from '../../../utils/providerQuotaQuery'
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
  const queryConfig = normalizeProviderQuotaQueryConfig(
    card.providerQuotaQueryConfig,
    card.providerQuotaQueryType,
  )
  if (!hasProviderQuotaQueryType(queryConfig ?? card.providerQuotaQueryType, card.providerQuotaQueryType)) {
    return []
  }

  const queryType = resolveProviderQuotaQueryType(queryConfig ?? card.providerQuotaQueryType)
  try {
    const response = await queryProviderQuota(queryConfig ?? queryType, card.apiUrl, card.apiKey)
    const responseItems = Array.isArray(response?.items) ? response.items : []
    if (response?.error) {
      console.warn(`[ProviderQuotaQuery] ${card.name} query failed: ${response.error}`)
    }
    if (responseItems.length === 0) {
      return []
    }

    return responseItems
      .filter((item) => item && `${item.key ?? ''}`.trim())
      .map((item) => {
        const used = normalizeDisplayValue(Number(item.used))
        const total = normalizeDisplayValue(Number(item.total))
        const nextReset = item.nextReset ? new Date(item.nextReset) : null
        const isActive = item.active !== false && item.isValid !== false
        const normalizedKey = `${item.key}`.trim()
        const defaultLabelKey = providerQuotaLabelKeyMap[normalizedKey]
        const invalidMessage = `${item.invalidMessage ?? ''}`.trim() || undefined
        const extra = `${item.extra ?? ''}`.trim() || undefined
        return {
          key: normalizedKey,
          label: `${item.label ?? ''}`.trim() || (defaultLabelKey ? t(defaultLabelKey) : normalizedKey),
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
          valueMode: item.valueMode === 'currency' ? 'currency' : 'count',
          unit: `${item.unit ?? ''}`.trim() || undefined,
          extra,
          invalidMessage,
        }
      })
  } catch (error) {
    console.warn(`[ProviderQuotaQuery] ${card.name} query crashed:`, error)
    return []
  }
}
