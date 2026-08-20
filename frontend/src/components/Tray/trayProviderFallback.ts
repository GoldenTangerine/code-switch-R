import type { AutomationCard } from '../../data/cards'
import { hasConfiguredBudgetQuotaSettings } from '../../utils/budgetUsage'
import { hasProviderQuotaQueryType } from '../../utils/providerQuotaQuery'
import type { ProviderQuotaSnapshotItem } from '../Main/utils/providerQuotaSnapshot'
import { providerQuotaLabelKeyMap } from '../Main/utils/providerQuotaSnapshot'
import {
  isProviderQuotaBalanceItem,
} from '../Main/utils/providerQuotaCardDisplay'
import type { TranslateFn } from '../Main/types'

export interface TrayProviderQuotaDisplay {
  key: string
  title: string
  used: number
  total: number
  unlimited: boolean
  valueMode: 'currency' | 'count'
  unit?: string
  extra: string
  invalidMessage: string
  displayKind: 'progress' | 'balance' | 'error'
  countdownLabel: string
  nextReset: Date | null
}

export function resolveTrayProviderQuotaDisplay(
  item: ProviderQuotaSnapshotItem,
  t: TranslateFn,
): TrayProviderQuotaDisplay {
  const normalizedKey = `${item.key ?? ''}`.trim()
  const labelKey = providerQuotaLabelKeyMap[normalizedKey]
  const title = labelKey ? t(labelKey) : `${item.label ?? ''}`.trim() || normalizedKey
  const used = Number.isFinite(Number(item.used)) ? Math.max(Number(item.used), 0) : 0
  const total = Number.isFinite(Number(item.total)) ? Math.max(Number(item.total), 0) : 0
  const invalidMessage = `${item.invalidMessage ?? ''}`.trim()
  const displayKind = invalidMessage
    ? 'error'
    : isProviderQuotaBalanceItem(item)
      ? 'balance'
      : 'progress'

  return {
    key: normalizedKey || title,
    title,
    used,
    total,
    unlimited: item.unlimited === true,
    valueMode: item.valueMode === 'count' ? 'count' : 'currency',
    unit: `${item.unit ?? ''}`.trim() || undefined,
    extra: `${item.extra ?? ''}`.trim(),
    invalidMessage,
    displayKind,
    countdownLabel: `${item.countdownLabel ?? ''}`.trim(),
    nextReset: item.nextReset,
  }
}

export function shouldShowTrayProviderQuotaMeta(
  quota: Pick<TrayProviderQuotaDisplay, 'displayKind' | 'countdownLabel' | 'extra' | 'invalidMessage'>,
): boolean {
  return quota.displayKind !== 'error'
    && Boolean(quota.countdownLabel || quota.extra || quota.invalidMessage)
}

export function normalizeTrayProviderLevel(value: unknown): number {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric <= 0) return 1
  return Math.floor(numeric)
}

export function selectTrayFallbackProvider(providers: readonly AutomationCard[]): AutomationCard | null {
  return listTrayFallbackProviders(providers)[0] ?? null
}

export function listTrayFallbackProviders(providers: readonly AutomationCard[]): AutomationCard[] {
  const enabledProviders = providers.filter((provider) => provider.enabled)
  if (enabledProviders.length === 0) return []

  const orderedLevels = Array.from(new Set(
    enabledProviders.map((provider) => normalizeTrayProviderLevel(provider.level)),
  )).sort((left, right) => left - right)

  const fallbackProviders: AutomationCard[] = []
  for (const level of orderedLevels) {
    fallbackProviders.push(...enabledProviders.filter((provider) => (
      normalizeTrayProviderLevel(provider.level) === level
      && hasTrayFallbackProviderQuotaConfig(provider)
    )))
  }

  return fallbackProviders
}

export function hasTrayFallbackProviderQuotaConfig(provider: AutomationCard | null | undefined): boolean {
  if (!provider) return false
  return hasProviderQuotaQueryType(
    provider.providerQuotaQueryConfig ?? provider.providerQuotaQueryType,
    provider.providerQuotaQueryType,
  ) || hasConfiguredBudgetQuotaSettings(provider.budgetQuotaSettings)
}
