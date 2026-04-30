import { describe, expect, it } from 'vitest'
import { shouldShowProviderProxyToggle } from './providerProxyToggleVisibility'

describe('shouldShowProviderProxyToggle', () => {
  it('hides local proxy hosting toggle for OpenCode', () => {
    expect(shouldShowProviderProxyToggle('opencode')).toBe(false)
  })

  it('keeps local proxy hosting toggle visible for proxy-based tabs', () => {
    expect(shouldShowProviderProxyToggle('claude')).toBe(true)
    expect(shouldShowProviderProxyToggle('codex')).toBe(true)
    expect(shouldShowProviderProxyToggle('gemini')).toBe(true)
    expect(shouldShowProviderProxyToggle('others')).toBe(true)
  })
})
