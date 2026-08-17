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
  checkProviderQuota: vi.fn(),
}))

import { fetchCostByProvider, fetchCostSinceByProvider, fetchFiveHourQuotaStatusByProvider } from '../../../services/logs'
import { checkProviderQuota, queryProviderQuota, type ProviderQuotaQueryResult } from '../../../services/providerQuotaQuery'
import { shouldAutoRefreshProviderQuota } from '../utils/providerQuotaAutoRefresh'
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
  opencode: [],
  grokbuild: [],
  'claude-desktop': [],
  openclaw: [],
  hermes: [],
  pi: [],
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
    vi.mocked(checkProviderQuota).mockResolvedValue({
      success: false,
      queryType: 'none',
      items: [],
      providerEnabled: true,
      quotaAutoDisabled: false,
      quotaAutoDisablePaused: false,
      stateChanged: false,
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
      t: (key: string, params?: Record<string, string>) => {
        if (key === 'components.main.providers.quotaRefreshFailedCached') {
          return `刷新失败（${params?.reason}），当前仍显示上次成功获取的数据`
        }
        if (key === 'components.main.providers.quotaQueryFailed') {
          return '额度查询失败'
        }
        return key
      },
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

  it('uses provider-aware quota checks when a provider kind resolver is supplied', async () => {
    const cards = createCardRecord()
    const card = createCard(29, { providerQuotaQueryType: 'balance' })
    cards.others.push(card)
    vi.mocked(checkProviderQuota).mockResolvedValue({
      success: true,
      queryType: 'balance',
      items: [{ key: 'balance', used: 2, total: 10, active: true }],
      providerEnabled: true,
      quotaAutoDisabled: false,
      quotaAutoDisablePaused: false,
      stateChanged: false,
    })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'others',
      cards,
      resolveProviderKind: () => 'custom:test-cli',
    })

    await quotaState.refreshProviderQuotas()

    expect(checkProviderQuota).toHaveBeenCalledWith('custom:test-cli', '29')
    expect(queryProviderQuota).not.toHaveBeenCalled()
  })

  it('preserves invalid messages and extra details from remote quota items', async () => {
    const cards = createCardRecord()
    const card = createCard(18, {
      providerQuotaQueryType: 'newapi',
      budgetQuotaSettings: undefined,
    })
    cards.codex.push(card)

    vi.mocked(queryProviderQuota).mockResolvedValue({
      success: true,
      queryType: 'newapi',
      items: [
        {
          key: 'quota_1',
          label: 'NewAPI',
          used: 0,
          total: 0,
          active: false,
          invalidMessage: 'Access token expired',
          extra: '请重新检查 User ID',
        },
      ],
    })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'quota_1',
        label: 'NewAPI',
        isActive: false,
        countdownLabel: 'components.main.providers.quotaInactive',
        invalidMessage: 'Access token expired',
        extra: '请重新检查 User ID',
      }),
    ])
  })

  it('keeps rendering remote failure items even when the query returns success=false', async () => {
    const cards = createCardRecord()
    const card = createCard(181, {
      providerQuotaQueryType: 'balance',
      budgetQuotaSettings: undefined,
      apiUrl: 'https://openrouter.ai/api/v1/chat/completions',
    })
    cards.codex.push(card)

    vi.mocked(queryProviderQuota).mockResolvedValue({
      success: false,
      queryType: 'balance',
      error: '官方余额查询认证失败 (HTTP 401)',
      items: [
        {
          key: 'openrouter',
          label: 'OpenRouter',
          used: 0,
          total: 0,
          active: false,
          invalidMessage: '官方余额查询认证失败 (HTTP 401)',
        },
      ],
    })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'openrouter',
        label: 'OpenRouter',
        isActive: false,
        countdownLabel: 'components.main.providers.quotaInactive',
        invalidMessage: '官方余额查询认证失败 (HTTP 401)',
      }),
    ])
  })

  it('creates a visible remote error item when the first balance query returns no displayable data', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const card = createCard(185, {
      providerQuotaQueryType: 'balance',
      budgetQuotaSettings: undefined,
      apiUrl: 'https://openrouter.ai/api/v1/chat/completions',
    })
    cards.codex.push(card)

    vi.mocked(queryProviderQuota).mockResolvedValue({
      success: false,
      queryType: 'balance',
      error: 'temporary upstream timeout',
      queriedAt: Date.UTC(2026, 3, 9, 10, 0, 0),
      items: [],
    })

    const quotaState = useProviderQuotas({
      t: (key: string, params?: Record<string, string>) => {
        if (key === 'components.main.providers.quotaQueryStatusLabel') {
          return '额度查询'
        }
        if (key === 'components.main.providers.quotaQueryFailed') {
          return '额度查询失败'
        }
        if (key === 'components.main.providers.quotaQueryEmpty') {
          return '远端额度接口没有返回可展示的数据'
        }
        if (key === 'components.main.providers.quotaRefreshFailedCached') {
          return `刷新失败（${params?.reason}），当前仍显示上次成功获取的数据`
        }
        return key
      },
      getActiveTab: () => 'codex',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'remote_quota_query_error',
        label: '额度查询',
        queriedAt: Date.UTC(2026, 3, 9, 10, 0, 0),
        invalidMessage: 'temporary upstream timeout',
        total: 0,
        used: 0,
        valueMode: 'currency',
      }),
    ])
  })

  it('keeps queriedAt on remote balance items and preserves it when ttl cache is reused', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const card = createCard(183, {
      providerQuotaQueryType: 'balance',
      budgetQuotaSettings: undefined,
      apiUrl: 'https://openrouter.ai/api/v1/chat/completions',
    })
    cards.codex.push(card)

    vi.mocked(queryProviderQuota).mockResolvedValue({
      success: true,
      queryType: 'balance',
      queriedAt: Date.UTC(2026, 3, 9, 9, 58, 0),
      items: [
        {
          key: 'openrouter',
          label: 'OpenRouter',
          used: 8,
          total: 50,
          active: true,
          valueMode: 'currency',
          unit: 'USD',
        },
      ],
    })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    vi.setSystemTime(new Date('2026-04-09T10:00:30.000Z'))
    await quotaState.refreshProviderQuotas()

    expect(queryProviderQuota).toHaveBeenCalledTimes(1)
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'openrouter',
        queriedAt: Date.UTC(2026, 3, 9, 9, 58, 0),
        used: 8,
        total: 50,
        valueMode: 'currency',
        unit: 'USD',
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

  it('limits timer-based remote quota polling to currently active providers', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const activeCard = createCard(21, {
      providerRef: 'active-ref',
      providerQuotaQueryType: 'token_plan_kimi',
    })
    const idleCard = createCard(22, {
      providerRef: 'idle-ref',
      providerQuotaQueryType: 'token_plan_kimi',
    })
    cards.claude.push(activeCard, idleCard)

    vi.mocked(queryProviderQuota).mockImplementation(async (queryTypeOrConfig, apiUrl) => ({
      success: true,
      queryType: typeof queryTypeOrConfig === 'string' ? queryTypeOrConfig : 'token_plan_kimi',
      items: [
        {
          key: apiUrl === activeCard.apiUrl ? 'weekly' : 'five_hour',
          used: apiUrl === activeCard.apiUrl ? 10 : 88,
          total: 100,
          nextReset: '2026-04-12T00:00:00.000Z',
          active: true,
        },
      ],
    }))

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'claude',
      cards,
      resolveAutoRefreshRemoteQuotaRefs: () => new Set(['active-ref']),
    })

    quotaState.startTimers()
    await vi.advanceTimersByTimeAsync(30_000)

    expect(queryProviderQuota).toHaveBeenCalledTimes(1)
    expect(queryProviderQuota).toHaveBeenCalledWith('token_plan_kimi', activeCard.apiUrl, activeCard.apiKey)
    expect(quotaState.getQuotaDisplay(activeCard)).toEqual([
      expect.objectContaining({
        key: 'weekly',
        used: 10,
        total: 100,
      }),
    ])
    expect(quotaState.getQuotaDisplay(idleCard)).toEqual([])

    quotaState.stopTimers()
  })

  it('keeps timer-based remote quota polling active for OpenCode balance query providers', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const opencodeCard = createCard(25, {
      providerRef: 'opencode-balance-ref',
      providerQuotaQueryType: 'balance',
    })
    const disabledOpenCodeCard = createCard(26, {
      providerRef: 'disabled-opencode-balance-ref',
      providerQuotaQueryType: 'balance',
      enabled: false,
    })
    cards.opencode.push(opencodeCard, disabledOpenCodeCard)

    vi.mocked(queryProviderQuota).mockResolvedValue({
      success: true,
      queryType: 'balance',
      queriedAt: new Date('2026-04-09T10:00:30.000Z').getTime(),
      items: [
        {
          key: 'balance',
          label: 'Balance',
          used: 0,
          total: 47.89,
          valueMode: 'currency',
          unit: 'USD',
          active: true,
        },
      ],
    })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'opencode',
      cards,
      resolveAutoRefreshRemoteQuotaRefs: () => new Set(
        cards.opencode
          .filter((card) => shouldAutoRefreshProviderQuota('opencode', card, false))
          .map((card) => card.providerRef || card.name),
      ),
    })

    quotaState.startTimers()
    await vi.advanceTimersByTimeAsync(30_000)

    expect(queryProviderQuota).toHaveBeenCalledTimes(1)
    expect(queryProviderQuota).toHaveBeenCalledWith('balance', opencodeCard.apiUrl, opencodeCard.apiKey)
    expect(queryProviderQuota).not.toHaveBeenCalledWith('balance', disabledOpenCodeCard.apiUrl, disabledOpenCodeCard.apiKey)
    expect(quotaState.getQuotaDisplay(opencodeCard)).toEqual([
      expect.objectContaining({
        key: 'balance',
        used: 0,
        total: 47.89,
        remaining: 47.89,
        nextReset: null,
        valueMode: 'currency',
        unit: 'USD',
      }),
    ])

    quotaState.stopTimers()
  })

  it('only refreshes the targeted remote provider when targetRefs are specified', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const targetCard = createCard(23, {
      providerRef: 'target-ref',
      providerQuotaQueryType: 'token_plan_kimi',
    })
    const siblingCard = createCard(24, {
      providerRef: 'sibling-ref',
      providerQuotaQueryType: 'token_plan_kimi',
    })
    cards.claude.push(targetCard, siblingCard)

    vi.mocked(queryProviderQuota)
      .mockResolvedValueOnce({
        success: true,
        queryType: 'token_plan_kimi',
        items: [
          {
            key: 'weekly',
            used: 10,
            total: 100,
            nextReset: '2026-04-12T00:00:00.000Z',
            active: true,
          },
        ],
      })
      .mockResolvedValueOnce({
        success: true,
        queryType: 'token_plan_kimi',
        items: [
          {
            key: 'weekly',
            used: 20,
            total: 100,
            nextReset: '2026-04-12T00:00:00.000Z',
            active: true,
          },
        ],
      })
      .mockResolvedValueOnce({
        success: true,
        queryType: 'token_plan_kimi',
        items: [
          {
            key: 'weekly',
            used: 33,
            total: 100,
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

    expect(queryProviderQuota).toHaveBeenCalledTimes(2)
    expect(quotaState.getQuotaDisplay(targetCard)[0]?.used).toBe(10)
    expect(quotaState.getQuotaDisplay(siblingCard)[0]?.used).toBe(20)

    const targetedRefreshPromise = quotaState.refreshProviderQuotas({
      targetRefs: new Set(['target-ref']),
      forceRemoteRefs: new Set(['target-ref']),
    })

    await Promise.resolve()

    expect(quotaState.isQuotaRefreshing(targetCard)).toBe(true)
    expect(quotaState.isQuotaRefreshing(siblingCard)).toBe(false)

    await targetedRefreshPromise

    expect(queryProviderQuota).toHaveBeenCalledTimes(3)
    expect(queryProviderQuota).toHaveBeenLastCalledWith('token_plan_kimi', targetCard.apiUrl, targetCard.apiKey)
    expect(quotaState.getQuotaDisplay(targetCard)[0]?.used).toBe(33)
    expect(quotaState.getQuotaDisplay(siblingCard)[0]?.used).toBe(20)
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

  it('only falls back to an expired remote quota cache once after ttl refresh failures', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const card = createCard(182, {
      providerQuotaQueryType: 'token_plan_kimi',
    })
    cards.claude.push(card)

    vi.mocked(queryProviderQuota)
      .mockResolvedValueOnce({
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
      .mockResolvedValueOnce({
        success: false,
        queryType: 'token_plan_kimi',
        items: [],
        error: 'temporary upstream timeout',
      })
      .mockResolvedValueOnce({
        success: false,
        queryType: 'token_plan_kimi',
        items: [],
        error: 'temporary upstream timeout',
      })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'claude',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    vi.setSystemTime(new Date('2026-04-09T10:05:01.000Z'))
    await quotaState.refreshProviderQuotas()
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'weekly',
        used: 30,
        total: 120,
      }),
    ])

    vi.setSystemTime(new Date('2026-04-09T10:05:31.000Z'))
    await quotaState.refreshProviderQuotas()

    expect(queryProviderQuota).toHaveBeenCalledTimes(3)
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'remote_quota_query_error',
        invalidMessage: 'temporary upstream timeout',
        total: 0,
        used: 0,
      }),
    ])
  })

  it('re-fetches remote quota immediately when explicit forceRemoteRefs are provided', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const card = createCard(14, {
      providerQuotaQueryType: 'token_plan_kimi',
    })
    cards.claude.push(card)

    vi.mocked(queryProviderQuota)
      .mockResolvedValueOnce({
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
      .mockResolvedValueOnce({
        success: true,
        queryType: 'token_plan_kimi',
        items: [
          {
            key: 'weekly',
            used: 45,
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
    const providerRef = card.providerRef || `${card.id}`
    await quotaState.refreshProviderQuotas({
      forceRemoteRefs: new Set([providerRef]),
    })

    expect(queryProviderQuota).toHaveBeenCalledTimes(2)
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'weekly',
        used: 45,
        total: 120,
        valueMode: 'count',
      }),
    ])
  })

  it('preserves cached remote balance when manual force refresh returns no items', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const card = createCard(184, {
      providerQuotaQueryType: 'balance',
      budgetQuotaSettings: undefined,
      apiUrl: 'https://openrouter.ai/api/v1/chat/completions',
    })
    cards.codex.push(card)

    const firstQueriedAt = Date.UTC(2026, 3, 9, 9, 58, 0)
    vi.mocked(queryProviderQuota)
      .mockResolvedValueOnce({
        success: true,
        queryType: 'balance',
        queriedAt: firstQueriedAt,
        items: [
          {
            key: 'openrouter',
            label: 'OpenRouter',
            used: 8,
            total: 50,
            active: true,
            valueMode: 'currency',
            unit: 'USD',
          },
        ],
      })
      .mockResolvedValueOnce({
        success: false,
        queryType: 'balance',
        error: 'temporary upstream timeout',
        items: [],
      })

    const quotaState = useProviderQuotas({
      t: (key: string, params?: Record<string, string>) => {
        if (key === 'components.main.providers.quotaRefreshFailedCached') {
          return `刷新失败（${params?.reason}），当前仍显示上次成功获取的数据`
        }
        if (key === 'components.main.providers.quotaQueryFailed') {
          return '额度查询失败'
        }
        return key
      },
      getActiveTab: () => 'codex',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    const providerRef = card.providerRef || `${card.id}`
    vi.setSystemTime(new Date('2026-04-09T10:01:00.000Z'))
    await quotaState.refreshProviderQuotas({
      targetRefs: new Set([providerRef]),
      forceRemoteRefs: new Set([providerRef]),
    })

    expect(queryProviderQuota).toHaveBeenCalledTimes(2)
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'openrouter',
        queriedAt: firstQueriedAt,
        used: 8,
        total: 50,
        valueMode: 'currency',
        unit: 'USD',
        refreshErrorMessage: '刷新失败（temporary upstream timeout），当前仍显示上次成功获取的数据',
      }),
    ])
  })

  it('re-fetches remote quota when provider query type changes within cache ttl', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const card = createCard(15, {
      providerQuotaQueryType: 'token_plan_kimi',
    })
    cards.claude.push(card)

    vi.mocked(queryProviderQuota)
      .mockResolvedValueOnce({
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
      .mockResolvedValueOnce({
        success: true,
        queryType: 'token_plan_glm',
        items: [
          {
            key: 'five_hour',
            used: 8,
            total: 100,
            nextReset: '2026-04-09T15:00:00.000Z',
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

    card.providerQuotaQueryType = 'token_plan_glm'
    vi.setSystemTime(new Date('2026-04-09T10:00:30.000Z'))
    await quotaState.refreshProviderQuotas()

    expect(queryProviderQuota).toHaveBeenCalledTimes(2)
    expect(queryProviderQuota).toHaveBeenNthCalledWith(2, 'token_plan_glm', card.apiUrl, card.apiKey)
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'five_hour',
        used: 8,
        total: 100,
        valueMode: 'count',
      }),
    ])
  })

  it('re-fetches remote quota when provider credentials change within cache ttl', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const card = createCard(16, {
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
            used: 55,
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

    card.apiKey = 'rotated-key-16'
    vi.setSystemTime(new Date('2026-04-09T10:00:30.000Z'))
    await quotaState.refreshProviderQuotas()

    expect(queryProviderQuota).toHaveBeenCalledTimes(2)
    expect(queryProviderQuota).toHaveBeenNthCalledWith(2, 'token_plan_minimax', card.apiUrl, 'rotated-key-16')
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'five_hour',
        used: 55,
        total: 100,
        valueMode: 'count',
      }),
    ])
  })

  it('waits for queued refresh requests to finish before resolving shared refresh promise', async () => {
    vi.setSystemTime(new Date('2026-04-09T10:00:00.000Z'))

    const cards = createCardRecord()
    const card = createCard(17, {
      providerQuotaQueryType: 'token_plan_glm',
    })
    cards.codex.push(card)

    let resolveFirstRequest: ((value: ProviderQuotaQueryResult) => void) | null = null

    vi.mocked(queryProviderQuota)
      .mockImplementationOnce(() => new Promise<ProviderQuotaQueryResult>((resolve) => {
        resolveFirstRequest = resolve
      }))
      .mockResolvedValueOnce({
        success: true,
        queryType: 'token_plan_glm',
        items: [
          {
            key: 'five_hour',
            used: 60,
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

    const firstRefreshPromise = quotaState.refreshProviderQuotas()
    await Promise.resolve()

    const providerRef = card.providerRef || `${card.id}`
    const queuedRefreshPromise = quotaState.refreshProviderQuotas({
      forceRemoteRefs: new Set([providerRef]),
    })

    expect(queuedRefreshPromise).toBe(firstRefreshPromise)
    expect(queryProviderQuota).toHaveBeenCalledTimes(1)
    expect(quotaState.isQuotaRefreshing(card)).toBe(true)

    if (!resolveFirstRequest) {
      throw new Error('expected first remote quota request to be pending')
    }

    const resolvePendingRequest: (value: ProviderQuotaQueryResult) => void = resolveFirstRequest
    resolvePendingRequest({
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

    await queuedRefreshPromise

    expect(queryProviderQuota).toHaveBeenCalledTimes(2)
    expect(quotaState.isQuotaRefreshing(card)).toBe(false)
    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'five_hour',
        used: 60,
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

  it('forces sub2api currency quota refresh when countdown crosses reset', async () => {
    vi.setSystemTime(new Date('2026-08-24T07:58:35.500Z'))

    const cards = createCardRecord()
    const card = createCard(14, {
      providerQuotaQueryType: 'sub2api',
      providerQuotaQueryConfig: {
        enabled: true,
        templateType: 'sub2api',
        code: 'sub2api-code',
      },
    })
    cards.codex.push(card)

    vi.mocked(queryProviderQuota)
      .mockResolvedValueOnce({
        success: true,
        queryType: 'sub2api',
        items: [
          {
            key: 'weekly',
            used: 4.8,
            total: 200,
            nextReset: '2026-08-24T07:58:36.000Z',
            active: true,
            valueMode: 'currency',
            unit: 'USD',
          },
        ],
      })
      .mockResolvedValueOnce({
        success: true,
        queryType: 'sub2api',
        items: [
          {
            key: 'weekly',
            used: 0,
            total: 200,
            nextReset: '2026-08-31T07:58:36.000Z',
            active: true,
            valueMode: 'currency',
            unit: 'USD',
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
        key: 'weekly',
        used: 0,
        total: 200,
        valueMode: 'currency',
      }),
    ])

    quotaState.stopTimers()
  })

  it('uses localized cycle labels and preserves unlimited sub2api state', async () => {
    vi.setSystemTime(new Date('2026-08-17T08:00:00.000Z'))

    const cards = createCardRecord()
    const card = createCard(15, {
      providerQuotaQueryType: 'sub2api',
      providerQuotaQueryConfig: {
        enabled: true,
        templateType: 'sub2api',
        code: 'sub2api-code',
      },
    })
    cards.codex.push(card)

    vi.mocked(queryProviderQuota).mockResolvedValue({
      success: true,
      queryType: 'sub2api',
      queriedAt: Date.now(),
      items: [
        {
          key: 'daily',
          label: 'Balance',
          used: 1,
          total: 10,
          nextReset: '2026-08-18T00:00:00.000Z',
          active: true,
          valueMode: 'currency',
          unit: 'USD',
        },
        {
          key: 'balance',
          label: 'Unlimited',
          used: 0,
          total: 0,
          active: true,
          unlimited: true,
          valueMode: 'currency',
          unit: 'USD',
        },
      ],
    })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'daily',
        label: 'components.main.providers.quotaDaily',
      }),
      expect.objectContaining({
        key: 'balance',
        label: 'Unlimited',
        unlimited: true,
      }),
    ])
  })

  it('ignores unlimited flags returned for non-sub2api balance queries', async () => {
    const cards = createCardRecord()
    const card = createCard(16, {
      providerQuotaQueryType: 'general',
      providerQuotaQueryConfig: {
        enabled: true,
        templateType: 'general',
        code: 'general-code',
      },
    })
    cards.codex.push(card)

    vi.mocked(queryProviderQuota).mockResolvedValue({
      success: true,
      queryType: 'general',
      queriedAt: Date.now(),
      items: [{
        key: 'balance',
        label: 'Balance',
        used: 0,
        total: 0,
        unlimited: true,
        active: false,
        valueMode: 'currency',
        unit: 'USD',
      }],
    })

    const quotaState = useProviderQuotas({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      cards,
    })

    await quotaState.refreshProviderQuotas()

    expect(quotaState.getQuotaDisplay(card)).toEqual([
      expect.objectContaining({
        key: 'balance',
        total: 0,
        unlimited: false,
      }),
    ])
  })
})
