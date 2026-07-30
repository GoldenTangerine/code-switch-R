import { describe, expect, it } from 'vitest'
import type { AutomationCard } from '../../../data/cards'
import {
  applyProviderQuotaStateChange,
  shouldAutoRefreshProviderQuota,
} from './providerQuotaAutoRefresh'

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
  sortOrder: id,
  ...overrides,
})

describe('shouldAutoRefreshProviderQuota', () => {
  it('keeps OpenCode quota query providers refreshed even when they are not currently active', () => {
    expect(shouldAutoRefreshProviderQuota('opencode', { enabled: true }, false)).toBe(true)
    expect(shouldAutoRefreshProviderQuota('opencode', { enabled: false, isInConfig: true }, false)).toBe(true)
  })

  it('does not auto refresh disabled OpenCode providers', () => {
    expect(shouldAutoRefreshProviderQuota('opencode', { enabled: false }, false)).toBe(false)
  })

  it('keeps other provider tabs limited to currently active providers', () => {
    expect(shouldAutoRefreshProviderQuota('claude', { enabled: true }, true)).toBe(true)
    expect(shouldAutoRefreshProviderQuota('claude', { enabled: true }, false)).toBe(false)
  })

  it('always refreshes auto-disabled and temporarily enabled providers', () => {
    expect(shouldAutoRefreshProviderQuota('claude', { enabled: false, quotaAutoDisabled: true }, false)).toBe(true)
    expect(shouldAutoRefreshProviderQuota('codex', { enabled: true, quotaAutoDisablePaused: true }, false)).toBe(true)
  })

  it('updates and reorders only the provider named by a quota state event', () => {
    const first = createCard(1, { enabledSortOrder: 1 })
    const target = createCard(2, { enabledSortOrder: 2 })
    const disabled = createCard(3, { enabled: false, disabledSortOrder: 1 })
    const cards = [first, target, disabled]

    expect(applyProviderQuotaStateChange(cards, {
      providerId: '2',
      enabled: false,
      quotaAutoDisabled: true,
      quotaAutoDisablePaused: false,
    })).toBe(true)

    expect(cards.map((card) => card.id)).toEqual([1, 2, 3])
    expect(cards[0]).toBe(first)
    expect(cards[1]).toBe(target)
    expect(target).toMatchObject({
      enabled: false,
      quotaAutoDisabled: true,
      quotaAutoDisablePaused: false,
    })

    expect(applyProviderQuotaStateChange(cards, {
      providerId: 'missing',
      enabled: true,
      quotaAutoDisabled: false,
      quotaAutoDisablePaused: false,
    })).toBe(false)
  })
})
