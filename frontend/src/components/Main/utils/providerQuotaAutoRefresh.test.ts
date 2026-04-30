import { describe, expect, it } from 'vitest'
import { shouldAutoRefreshProviderQuota } from './providerQuotaAutoRefresh'

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
})
