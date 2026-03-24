import { describe, expect, it } from 'vitest'
import { isHostedRouteActive } from './providerRoutingState'

describe('isHostedRouteActive', () => {
  it('returns true only when the last used provider is still routable under hosting mode', () => {
    expect(isHostedRouteActive({
      activeProxyState: true,
      isLastUsed: true,
      enabled: true,
      apiUrl: 'https://example.com',
      apiKey: 'secret',
      isBlacklisted: false,
    })).toBe(true)
  })

  it('returns false when hosting is off or the provider is no longer routable', () => {
    expect(isHostedRouteActive({
      activeProxyState: false,
      isLastUsed: true,
      enabled: true,
      apiUrl: 'https://example.com',
      apiKey: 'secret',
      isBlacklisted: false,
    })).toBe(false)

    expect(isHostedRouteActive({
      activeProxyState: true,
      isLastUsed: true,
      enabled: false,
      apiUrl: 'https://example.com',
      apiKey: 'secret',
      isBlacklisted: false,
    })).toBe(false)

    expect(isHostedRouteActive({
      activeProxyState: true,
      isLastUsed: true,
      enabled: true,
      apiUrl: 'https://example.com',
      apiKey: '',
      isBlacklisted: false,
    })).toBe(false)

    expect(isHostedRouteActive({
      activeProxyState: true,
      isLastUsed: true,
      enabled: true,
      apiUrl: 'https://example.com',
      apiKey: 'secret',
      isBlacklisted: true,
    })).toBe(false)
  })
})
