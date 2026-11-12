/**
 * @name: 供应商额度展示转换
 * @Descripttion: 将额度查询结果转换为卡片快照并翻译采样状态。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-06 16:35:58
 * @LastEditTime: 2026-11-06 16:35:58
 * @FilePath: frontend/src/components/Main/utils/providerQuotaQueryDisplay.ts
 */
import type { AutomationCard } from '../../../data/cards'
import { checkProviderQuota, queryProviderQuota } from '../../../services/providerQuotaQuery'
import { roundBudgetValue } from '../../../utils/budgetUsage'
import {
  formatPassiveQuotaEstimateNote,
  hasProviderQuotaQueryType,
  normalizeProviderQuotaQueryConfig,
  localizeProviderQuotaMessage,
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

const codexWindowLabelKeyMap: Record<string, string> = {
  codex_primary: 'components.main.providers.quotaCodexPrimary',
  codex_secondary: 'components.main.providers.quotaCodexSecondary',
}

export type ProviderQuotaQueryDisplayResult = {
  items: ProviderQuotaSnapshotItem[]
  failureMessage?: string
  queriedAt?: number
}

export async function resolveProviderQuotaQueryDisplay({
  card,
  now,
  t,
  providerKind,
  providerID,
}: {
  card: AutomationCard
  now: Date
  t: TranslateFn
  providerKind?: string
  providerID?: string
}): Promise<ProviderQuotaQueryDisplayResult> {
  const queryConfig = normalizeProviderQuotaQueryConfig(
    card.providerQuotaQueryConfig,
    card.providerQuotaQueryType,
  )
  if (!hasProviderQuotaQueryType(queryConfig ?? card.providerQuotaQueryType, card.providerQuotaQueryType)) {
    return { items: [] }
  }

  const queryType = resolveProviderQuotaQueryType(queryConfig ?? card.providerQuotaQueryType)
  try {
    const response = providerKind && providerID
      ? await checkProviderQuota(providerKind, providerID)
      : await queryProviderQuota(queryConfig ?? queryType, card.apiUrl, card.apiKey)
    const responseItems = Array.isArray(response?.items) ? response.items : []
    const queriedAt = Number(response?.queriedAt)
    const normalizedQueriedAt = Number.isFinite(queriedAt) ? queriedAt : now.getTime()
    if (response?.error) {
      console.warn(`[ProviderQuotaQuery] ${card.name} query failed: ${response.error}`)
    }
    if (responseItems.length === 0) {
      return {
        items: [],
        failureMessage: `${response?.error ?? ''}`.trim() || t('components.main.providers.quotaQueryEmpty'),
        queriedAt: normalizedQueriedAt,
      }
    }

    return {
      queriedAt: normalizedQueriedAt,
      items: responseItems
        .filter((item) => item && `${item.key ?? ''}`.trim())
        .map((item) => {
        const used = normalizeDisplayValue(Number(item.used))
        const total = normalizeDisplayValue(Number(item.total))
        const nextReset = item.nextReset ? new Date(item.nextReset) : null
        const isActive = item.active !== false && item.isValid !== false
        const normalizedKey = `${item.key}`.trim()
        const defaultLabelKey = providerQuotaLabelKeyMap[normalizedKey]
        const codexWindowLabelKey = queryType === 'token_plan_codex_passive'
          ? codexWindowLabelKeyMap[normalizedKey]
          : undefined
        const invalidMessage = localizeProviderQuotaMessage(item.invalidMessage, t) || undefined
        const extra = [formatPassiveQuotaEstimateNote(item, t, now), `${item.extra ?? ''}`.trim()]
          .filter(Boolean)
          .join(' · ') || undefined
        return {
          key: normalizedKey,
          label: defaultLabelKey ? t(defaultLabelKey) : `${item.label ?? ''}`.trim()
            || (codexWindowLabelKey ? t(codexWindowLabelKey) : normalizedKey),
          used,
          total,
          unlimited: queryType === 'sub2api' && item.unlimited === true,
          progressRatio: resolveProgressRatio(used, total),
          countdownLabel: isActive
            ? formatProviderQuotaCountdownLabel(nextReset, now)
            : t('components.main.providers.quotaInactive'),
          nextReset,
          queriedAt: normalizedQueriedAt,
          trackedUsed: used,
          adjustment: 0,
          remaining: roundBudgetValue(total - used),
          isActive,
          valueMode: item.valueMode === 'currency' ? 'currency' : 'count',
          unit: `${item.unit ?? ''}`.trim() || undefined,
          extra,
          invalidMessage,
          estimate: `${item.estimate ?? ''}`.trim() || undefined,
          sampledAt: Number.isFinite(Number(item.sampledAt)) && Number(item.sampledAt) > 0
            ? Number(item.sampledAt)
            : undefined,
        }
      }),
    }
  } catch (error) {
    console.warn(`[ProviderQuotaQuery] ${card.name} query crashed:`, error)
    return {
      items: [],
      failureMessage: error instanceof Error
        ? error.message
        : t('components.main.providers.quotaQueryFailed'),
      queriedAt: now.getTime(),
    }
  }
}
