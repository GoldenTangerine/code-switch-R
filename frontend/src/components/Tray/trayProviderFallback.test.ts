import { describe, expect, it } from 'vitest'
import type { AutomationCard } from '../../data/cards'
import { createDefaultBudgetQuotaSettings } from '../../utils/budgetUsage'
import {
  hasTrayFallbackProviderQuotaConfig,
  listTrayFallbackProviders,
  normalizeTrayProviderLevel,
  selectTrayFallbackProvider,
} from './trayProviderFallback'

const createProvider = (
  id: number,
  overrides: Partial<AutomationCard> = {},
): AutomationCard => ({
  id,
  providerRef: `${id}`,
  name: `Provider ${id}`,
  apiUrl: `https://example-${id}.com`,
  apiKey: `key-${id}`,
  officialSite: `https://example-${id}.com`,
  icon: 'openai',
  tint: 'rgba(10, 132, 255, 0.14)',
  accent: '#0a84ff',
  enabled: true,
  level: 1,
  sortOrder: id,
  ...overrides,
})

describe('trayProviderFallback', () => {
  it('normalizes missing or invalid provider levels to level 1', () => {
    expect(normalizeTrayProviderLevel(undefined)).toBe(1)
    expect(normalizeTrayProviderLevel(0)).toBe(1)
    expect(normalizeTrayProviderLevel(-2)).toBe(1)
    expect(normalizeTrayProviderLevel(3.8)).toBe(3)
  })

  it('selects the first quota-configured provider in route order', () => {
    const quotas = createDefaultBudgetQuotaSettings()
    quotas.weekly.total = 50
    const laterQuotas = createDefaultBudgetQuotaSettings()
    laterQuotas.monthly.total = 200
    const providers = [
      createProvider(1, { enabled: false, level: 1 }),
      createProvider(2, { enabled: true, level: 1 }),
      createProvider(3, { enabled: true, level: 2, budgetQuotaSettings: laterQuotas }),
      createProvider(4, { enabled: true, level: 2, budgetQuotaSettings: quotas }),
      createProvider(5, { enabled: true, level: 3, providerQuotaQueryType: 'balance' }),
    ]

    expect(selectTrayFallbackProvider(providers)?.id).toBe(3)
    expect(listTrayFallbackProviders(providers).map((provider) => provider.id)).toEqual([3, 4, 5])
  })

  it('keeps same-level providers in their loaded route order', () => {
    const weeklyQuotas = createDefaultBudgetQuotaSettings()
    weeklyQuotas.weekly.total = 50
    const fiveHourQuotas = createDefaultBudgetQuotaSettings()
    fiveHourQuotas.five_hour.total = 10
    const providers = [
      createProvider(4, { enabled: true, level: 2, budgetQuotaSettings: weeklyQuotas }),
      createProvider(2, { enabled: true, level: 1 }),
      createProvider(3, { enabled: true, level: 2, providerQuotaQueryType: 'balance' }),
      createProvider(1, { enabled: true, level: 2, budgetQuotaSettings: fiveHourQuotas }),
    ]

    expect(listTrayFallbackProviders(providers).map((provider) => provider.id)).toEqual([4, 3, 1])
  })

  it('treats a missing level as the highest priority level', () => {
    const quotas = createDefaultBudgetQuotaSettings()
    quotas.five_hour.total = 10
    const providers = [
      createProvider(1, { enabled: true, level: 4, providerQuotaQueryType: 'balance' }),
      createProvider(2, { enabled: true, level: undefined, budgetQuotaSettings: quotas }),
    ]

    expect(selectTrayFallbackProvider(providers)?.id).toBe(2)
  })

  it('returns null when no enabled provider has quota config', () => {
    expect(selectTrayFallbackProvider([
      createProvider(1, { enabled: false }),
      createProvider(2, { enabled: true }),
    ])).toBeNull()
  })

  it('detects local and remote quota configs on fallback providers', () => {
    const localQuotas = createDefaultBudgetQuotaSettings()
    localQuotas.weekly.total = 50

    expect(hasTrayFallbackProviderQuotaConfig(createProvider(1))).toBe(false)
    expect(hasTrayFallbackProviderQuotaConfig(createProvider(2, {
      budgetQuotaSettings: localQuotas,
    }))).toBe(true)
    expect(hasTrayFallbackProviderQuotaConfig(createProvider(3, {
      providerQuotaQueryType: 'balance',
    }))).toBe(true)
  })
})
