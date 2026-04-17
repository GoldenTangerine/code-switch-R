import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AutomationCard } from '../../../data/cards'
import { createDefaultBudgetQuotaSettings } from '../../../utils/budgetUsage'
import type { ProviderTab } from '../types'

vi.mock('../../../services/logs', () => ({
  fetchCostByProvider: vi.fn(),
  fetchCostSinceByProvider: vi.fn(),
  fetchFiveHourQuotaStatusByProvider: vi.fn(),
}))

vi.mock('../../../services/providerQuotaQuery', () => ({
  queryProviderQuota: vi.fn(),
}))

import { fetchCostByProvider, fetchCostSinceByProvider, fetchFiveHourQuotaStatusByProvider } from '../../../services/logs'
import { queryProviderQuota } from '../../../services/providerQuotaQuery'
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
    vi.mocked(queryProviderQuota).mockResolvedValue({
      success: false,
      queryType: 'none',
      items: [],
    })
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
        total: 0,
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

  it('normalizes persisted noisy adjustments before computing quota progress', async () => {
    const cards = createCardRecord()
    const quotas = createDefaultBudgetQuotaSettings()
    quotas.daily.total = 44.6

    const card = createCard(7, {
      budgetQuotaSettings: quotas,
      budgetQuotaUsedAdjustments: {
        five_hour: 0,
        daily: 44.600099995,
        weekly: 0,
        monthly: 0,
        total: 0,
      },
    })
    cards.claude.push(card)

    vi.mocked(fetchCostSinceByProvider).mockResolvedValue(0)
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
        used: 44.6,
        progressRatio: 1,
        remaining: 0,
      }),
    ])
  })

  it('uses cumulative provider cost for total quotas without countdown refresh state', async () => {
    const cards = createCardRecord()
    const quotas = createDefaultBudgetQuotaSettings()
    quotas.total.total = 250

    const card = createCard(8, { budgetQuotaSettings: quotas })
    cards.claude.push(card)

    vi.mocked(fetchCostByProvider).mockResolvedValue(123.45)
    vi.mocked(fetchCostSinceByProvider).mockResolvedValue(0)
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

    expect(fetchCostByProvider).toHaveBeenCalledWith('claude', '8', 'Provider 8')
    expect(fetchCostSinceByProvider).not.toHaveBeenCalled()
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'total',
        used: 123.45,
        progressRatio: 0.4938,
        countdownLabel: '',
        nextReset: null,
      }),
    ])
  })

  it('refreshes provider quotas once the countdown crosses the reset point', async () => {
    vi.setSystemTime(new Date('2026-04-09T15:59:59.500'))

    const cards = createCardRecord()
    const quotas = createDefaultBudgetQuotaSettings()
    quotas.daily.total = 100
    quotas.daily.refreshTime = '16:00'

    const card = createCard(6, { budgetQuotaSettings: quotas })
    cards.codex.push(card)

    vi.mocked(fetchCostSinceByProvider)
      .mockResolvedValueOnce(42)
      .mockResolvedValueOnce(64)
    vi.mocked(fetchFiveHourQuotaStatusByProvider).mockResolvedValue({
      active: false,
      window_start: '',
      next_reset: '',
      used: 0,
    })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      cards,
    })

    await quotaState.refreshProviderQuotas()
    quotaState.startTimers()

    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'daily',
        used: 42,
        countdownLabel: '0h1m',
      }),
    ])

    await vi.advanceTimersByTimeAsync(1_000)

    expect(fetchCostSinceByProvider).toHaveBeenCalledTimes(2)
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'daily',
        used: 64,
        countdownLabel: '23h59m',
      }),
    ])

    quotaState.stopTimers()
  })

  it('shows provider query quota on cards even without local budget settings', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const card = createCard(9, {
      providerQuotaQueryType: 'token_plan_glm',
      budgetQuotaSettings: undefined,
    })
    cards.codex.push(card)

    vi.mocked(queryProviderQuota).mockResolvedValue({
      success: true,
      queryType: 'token_plan_glm',
      items: [
        {
          key: 'five_hour',
          used: 25,
          total: 100,
          nextReset: '2026-04-09T15:00:00.000Z',
          active: true,
        },
      ],
    })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    expect(queryProviderQuota).toHaveBeenCalledWith('token_plan_glm', card.apiUrl, card.apiKey)
    expect(fetchCostSinceByProvider).not.toHaveBeenCalled()
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'five_hour',
        used: 25,
        total: 100,
        progressRatio: 0.25,
        countdownLabel: '5h0m',
        valueMode: 'count',
      }),
    ])
  })

  it('prefers provider query result over local budget-log quota calculation', async () => {
    const cards = createCardRecord()
    const quotas = createDefaultBudgetQuotaSettings()
    quotas.daily.total = 100

    const card = createCard(10, {
      providerQuotaQueryType: 'token_plan_kimi',
      budgetQuotaSettings: quotas,
    })
    cards.claude.push(card)

    vi.mocked(queryProviderQuota).mockResolvedValue({
      success: true,
      queryType: 'token_plan_kimi',
      items: [
        {
          key: 'weekly',
          used: 30,
          total: 120,
          nextReset: '2026-04-12T00:00:00.000Z',
          active: true,
        },
      ],
    })
    vi.mocked(fetchCostSinceByProvider).mockResolvedValue(88)

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'claude',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    expect(queryProviderQuota).toHaveBeenCalledWith('token_plan_kimi', card.apiUrl, card.apiKey)
    expect(fetchCostSinceByProvider).not.toHaveBeenCalled()
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'weekly',
        used: 30,
        total: 120,
        progressRatio: 0.25,
        valueMode: 'count',
      }),
    ])
  })

  it('reuses cached remote quota within ttl instead of querying every 30 seconds', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const card = createCard(11, {
      providerQuotaQueryType: 'token_plan_kimi',
    })
    cards.claude.push(card)

    vi.mocked(queryProviderQuota).mockResolvedValue({
      success: true,
      queryType: 'token_plan_kimi',
      items: [
        {
          key: 'weekly',
          used: 30,
          total: 120,
          nextReset: '2026-04-12T00:00:00.000Z',
          active: true,
        },
      ],
    })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'claude',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    vi.setSystemTime(new Date('2026-04-09T10:00:30.000Z'))
    await quotaState.refreshProviderQuotas()

    expect(queryProviderQuota).toHaveBeenCalledTimes(1)
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'weekly',
        used: 30,
        total: 120,
        valueMode: 'count',
      }),
    ])
  })

  it('refreshes cached remote quota again after cache ttl expires', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const card = createCard(12, {
      providerQuotaQueryType: 'token_plan_minimax',
    })
    cards.codex.push(card)

    vi.mocked(queryProviderQuota)
      .mockResolvedValueOnce({
        success: true,
        queryType: 'token_plan_minimax',
        items: [
          {
            key: 'five_hour',
            used: 20,
            total: 100,
            nextReset: '2026-04-09T16:00:00.000Z',
            active: true,
          },
        ],
      })
      .mockResolvedValueOnce({
        success: true,
        queryType: 'token_plan_minimax',
        items: [
          {
            key: 'five_hour',
            used: 45,
            total: 100,
            nextReset: '2026-04-09T21:00:00.000Z',
            active: true,
          },
        ],
      })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    vi.setSystemTime(new Date('2026-04-09T10:05:01.000Z'))
    await quotaState.refreshProviderQuotas()

    expect(queryProviderQuota).toHaveBeenCalledTimes(2)
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'five_hour',
        used: 45,
        total: 100,
        valueMode: 'count',
      }),
    ])
  })

  it('forces remote quota refresh when countdown crosses reset before cache ttl expires', async () => {
    vi.setSystemTime(new Date('2026-04-09T14:59:59.500Z'))

    const cards = createCardRecord()
    const card = createCard(13, {
      providerQuotaQueryType: 'token_plan_glm',
    })
    cards.codex.push(card)

    vi.mocked(queryProviderQuota)
      .mockResolvedValueOnce({
        success: true,
        queryType: 'token_plan_glm',
        items: [
          {
            key: 'five_hour',
            used: 25,
            total: 100,
            nextReset: '2026-04-09T15:00:00.000Z',
            active: true,
          },
        ],
      })
      .mockResolvedValueOnce({
        success: true,
        queryType: 'token_plan_glm',
        items: [
          {
            key: 'five_hour',
            used: 2,
            total: 100,
            nextReset: '2026-04-09T20:01:00.000Z',
            active: true,
          },
        ],
      })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      cards,
    })

    await quotaState.refreshProviderQuotas()
    quotaState.startTimers()

    await vi.advanceTimersByTimeAsync(1_000)

    expect(queryProviderQuota).toHaveBeenCalledTimes(2)
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'five_hour',
        used: 2,
        total: 100,
        countdownLabel: '5h0m',
        valueMode: 'count',
      }),
    ])

    quotaState.stopTimers()
  })
})
