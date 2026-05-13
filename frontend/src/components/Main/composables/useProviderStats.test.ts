import { describe, expect, it, vi } from 'vitest'
import type { AutomationCard } from '../../../data/cards'
import { resolveProviderUnreadFailedRequestsForCard } from './useProviderStats'

vi.mock('@wailsio/runtime', () => ({
  Call: {
    ByName: vi.fn(),
  },
}))

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
