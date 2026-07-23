import { describe, expect, it, vi } from 'vitest'
import type { AutomationCard } from '../../../data/cards'

const {
  fetchProviderDailyStatsMock,
  fetchProviderUnreadFailedStatsMock,
  fetchProviderModelPricingMock,
} = vi.hoisted(() => ({
  fetchProviderDailyStatsMock: vi.fn(),
  fetchProviderUnreadFailedStatsMock: vi.fn(),
  fetchProviderModelPricingMock: vi.fn(),
}))

vi.mock('../../../services/logs', () => ({
  fetchProviderDailyStats: fetchProviderDailyStatsMock,
  fetchProviderUnreadFailedStats: fetchProviderUnreadFailedStatsMock,
}))

vi.mock('../../../services/providerModelPricing', () => ({
  fetchProviderModelPricing: fetchProviderModelPricingMock,
}))

vi.mock('@wailsio/runtime', () => ({
  Call: {
    ByName: vi.fn(),
  },
}))

import { resolveProviderUnreadFailedRequestsForCard, useProviderStats } from './useProviderStats'

const buildCard = (overrides: Partial<AutomationCard> = {}): AutomationCard => ({
  id: 102,
  name: 'kimi',
  apiUrl: 'https://api.moonshot.cn/anthropic',
  apiKey: 'sk-test',
  officialSite: 'https://kimi.moonshot.cn',
  icon: 'kimi',
  tint: 'rgba(16, 185, 129, 0.16)',
  accent: '#10b981',
  enabled: true,
  ...overrides,
})

describe('resolveProviderUnreadFailedRequestsForCard', () => {
  it('uses provider identity before name fallback to avoid stale hosted red dots', () => {
    const card = buildCard()

    expect(resolveProviderUnreadFailedRequestsForCard(card, { kimi: 3 })).toBe(0)
    expect(resolveProviderUnreadFailedRequestsForCard(card, { 102: 2, kimi: 3 })).toBe(2)
  })

  it('falls back to provider name only when the card has no identity', () => {
    const card = buildCard({ id: Number.NaN, providerRef: '', name: 'kimi' })

    expect(resolveProviderUnreadFailedRequestsForCard(card, { kimi: 1 })).toBe(1)
  })
})

describe('useProviderStats duration display', () => {
  it('formats minute, hour, and empty TTFT values through the provider display flow', async () => {
    const card = buildCard()
    const cards = {
      claude: [card],
      codex: [],
      gemini: [],
      opencode: [],
      others: [],
    }
    const stats = useProviderStats({
      t: (key: string) => key,
      getLocale: () => 'en',
      getActiveTab: () => 'claude',
      cards,
      refreshAvailabilityResults: vi.fn().mockResolvedValue(undefined),
    })
    fetchProviderUnreadFailedStatsMock.mockResolvedValue([])

    for (const [seconds, expected] of [
      [65, '1m 05s'],
      [3665, '1h 01m 05s'],
      [0, '—'],
    ] as const) {
      fetchProviderDailyStatsMock.mockResolvedValue([{
        provider_id: '102',
        provider: 'kimi',
        total_requests: 1,
        input_tokens: 0,
        output_tokens: 0,
        cache_read_tokens: 0,
        avg_first_token_sec: seconds,
        avg_tokens_per_sec: 0,
      }])

      await stats.loadProviderStats('claude')

      const display = stats.providerStatDisplay(card)
      expect(display.state).toBe('ready')
      if (display.state !== 'ready') {
        throw new Error(`provider stat display state = ${display.state}`)
      }
      expect(display.ttft).toBe(expected)
    }
  })

  it('shows an unavailable rate when all requests are excluded', async () => {
    const card = buildCard()
    const stats = useProviderStats({
      t: (key: string) => key,
      getLocale: () => 'en',
      getActiveTab: () => 'claude',
      cards: {
        claude: [card],
        codex: [],
        gemini: [],
        opencode: [],
        others: [],
      },
      refreshAvailabilityResults: vi.fn().mockResolvedValue(undefined),
    })
    fetchProviderUnreadFailedStatsMock.mockResolvedValue([])
    fetchProviderDailyStatsMock.mockResolvedValue([{
      provider_id: '102',
      provider: 'kimi',
      total_requests: 3,
      successful_requests: 0,
      failed_requests: 0,
      excluded_requests: 3,
      success_rate: 0,
      input_tokens: 0,
      output_tokens: 0,
      cache_read_tokens: 0,
    }])

    await stats.loadProviderStats('claude')

    const display = stats.providerStatDisplay(card)
    expect(display.state).toBe('ready')
    if (display.state !== 'ready') throw new Error(`provider stat display state = ${display.state}`)
    expect(display.successRateLabel).toContain('—')
    expect(display.successRateClass).toBe('')
  })
})
