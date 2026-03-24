import { describe, expect, it } from 'vitest'
import { getDefaultHostedProviderRef, isHostedProviderRoutable, isHostedRouteActive } from './providerRoutingState'

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

  it('detects whether a provider is still routable', () => {
    expect(isHostedProviderRoutable({
      id: 1,
      enabled: true,
      apiUrl: 'https://example.com',
      apiKey: 'secret',
    }, false)).toBe(true)

    expect(isHostedProviderRoutable({
      id: 1,
      enabled: true,
      apiUrl: 'https://example.com',
      apiKey: '',
    }, false)).toBe(false)
  })

  it('falls back to the first routable provider in the highest-priority level as default', () => {
    expect(getDefaultHostedProviderRef([
      { id: 1, enabled: false, apiUrl: 'https://a.com', apiKey: 'a', level: 1 },
      { id: 2, enabled: true, apiUrl: 'https://b.com', apiKey: 'b', level: 2 },
      { id: 3, enabled: true, apiUrl: 'https://c.com', apiKey: 'c', level: 1, providerRef: 'provider-c' },
    ], (provider) => provider.id === 1)).toBe('provider-c')
  })
})
