import type { AutomationCard } from '../../data/cards'
import { hasConfiguredBudgetQuotaSettings } from '../../utils/budgetUsage'
import { hasProviderQuotaQueryType } from '../../utils/providerQuotaQuery'

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
