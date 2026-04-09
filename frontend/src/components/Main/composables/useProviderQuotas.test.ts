import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AutomationCard } from '../../../data/cards'
import { createDefaultBudgetQuotaSettings } from '../../../utils/budgetUsage'
import type { ProviderTab } from '../types'

vi.mock('../../../services/logs', () => ({
  fetchCostSinceByProvider: vi.fn(),
  fetchFiveHourQuotaStatusByProvider: vi.fn(),
}))

import { fetchCostSinceByProvider, fetchFiveHourQuotaStatusByProvider } from '../../../services/logs'
import { useProviderQuotas } from './useProviderQuotas'

const createCard = (
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
  sortOrder: 1,
  supportedModels: {},
  modelMapping: {},
  requestBodyOverrides: {},
  cliConfig: {},
  apiEndpoint: '',
  availabilityMonitorEnabled: false,
  connectivityAutoBlacklist: false,
  availabilityConfig: {
    testModel: '',
    testEndpoint: '/responses',
    timeout: 15000,
  },
  connectivityCheck: false,
  connectivityTestModel: '',
  connectivityTestEndpoint: '',
  connectivityAuthType: '',
  ...overrides,
})

const createCardRecord = (): Record<ProviderTab, AutomationCard[]> => ({
  claude: [],
  codex: [],
  gemini: [],
  others: [],
})

describe('useProviderQuotas', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('keeps quota display isolated by tab even when provider refs overlap', async () => {
    const cards = createCardRecord()
    const sharedRef = 'shared-ref'
    const claudeQuotas = createDefaultBudgetQuotaSettings()
    claudeQuotas.daily.total = 10
    const codexQuotas = createDefaultBudgetQuotaSettings()
    codexQuotas.daily.total = 20

    const claudeCard = createCard(1, { providerRef: sharedRef, budgetQuotaSettings: claudeQuotas })
    const codexCard = createCard(2, { providerRef: sharedRef, budgetQuotaSettings: codexQuotas })
    cards.claude.push(claudeCard)
    cards.codex.push(codexCard)

    let activeTab: ProviderTab = 'claude'
    vi.mocked(fetchCostSinceByProvider).mockImplementation(async (_start, platform) => {
      if (platform === 'claude') return 2
      if (platform === 'codex') return 7
      return 0
    })
    vi.mocked(fetchFiveHourQuotaStatusByProvider).mockResolvedValue({
      active: false,
      window_start: '',
      next_reset: '',
      used: 0,
    })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => activeTab,
      cards,
    })

    await quotaState.refreshProviderQuotas()
    expect(quotaState.getQuotaDisplay(claudeCard)[0]?.used).toBe(2)

    activeTab = 'codex'
    await quotaState.refreshProviderQuotas()
    expect(quotaState.getQuotaDisplay(codexCard)[0]?.used).toBe(7)

    activeTab = 'claude'
    expect(quotaState.getQuotaDisplay(claudeCard)[0]?.used).toBe(2)
  })

  it('shows inactive label for five hour quotas without an active cycle', async () => {
    const cards = createCardRecord()
    const quotas = createDefaultBudgetQuotaSettings()
    quotas.five_hour.total = 15
    const card = createCard(3, { budgetQuotaSettings: quotas })
    cards.codex.push(card)

    vi.mocked(fetchFiveHourQuotaStatusByProvider).mockResolvedValue({
      active: false,
      window_start: '',
      next_reset: '',
      used: 0,
    })
    vi.mocked(fetchCostSinceByProvider).mockResolvedValue(0)

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'five_hour',
        countdownLabel: 'components.main.providers.quotaInactive',
        nextReset: null,
        used: 0,
      }),
    ])
  })

  it('formats countdowns for provider quotas and keeps progress ratio from used cost', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:47:15'))

    const cards = createCardRecord()
    const quotas = createDefaultBudgetQuotaSettings()
    quotas.daily.total = 100
    quotas.daily.refreshTime = '16:00'
    quotas.weekly.total = 100
    quotas.weekly.refreshTime = '19:00'
    quotas.weekly.refreshWeekday = 0

    const card = createCard(4, { budgetQuotaSettings: quotas })
    cards.codex.push(card)

    vi.mocked(fetchFiveHourQuotaStatusByProvider).mockResolvedValue({
      active: false,
      window_start: '',
      next_reset: '',
      used: 0,
    })
    vi.mocked(fetchCostSinceByProvider).mockResolvedValue(42.43)

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'daily',
        used: 42.43,
        progressRatio: 0.4243,
        countdownLabel: '5h12m',
      }),
      expect.objectContaining({
        key: 'weekly',
        used: 42.43,
        progressRatio: 0.4243,
        countdownLabel: '3d8h',
      }),
    ])
  })

  it('adds provider quota used adjustments on top of tracked usage', async () => {
    const cards = createCardRecord()
    const quotas = createDefaultBudgetQuotaSettings()
    quotas.daily.total = 100

    const card = createCard(5, {
      budgetQuotaSettings: quotas,
      budgetQuotaUsedAdjustments: {
        five_hour: 0,
        daily: 7.57,
        weekly: 0,
        monthly: 0,
      },
    })
    cards.claude.push(card)

    vi.mocked(fetchCostSinceByProvider).mockResolvedValue(42.43)
    vi.mocked(fetchFiveHourQuotaStatusByProvider).mockResolvedValue({
      active: false,
      window_start: '',
      next_reset: '',
      used: 0,
    })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'claude',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'daily',
        used: 50,
        progressRatio: 0.5,
      }),
    ])
  })
})
