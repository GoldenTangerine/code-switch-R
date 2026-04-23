import { describe, expect, it } from 'vitest'
import { resolveProviderQuotaCurrencyCode } from './providerQuotaValueFormat'

describe('providerQuotaValueFormat', () => {
  it('maps common RMB-style units to CNY', () => {
    expect(resolveProviderQuotaCurrencyCode('¥')).toBe('CNY')
    expect(resolveProviderQuotaCurrencyCode('￥')).toBe('CNY')
    expect(resolveProviderQuotaCurrencyCode('RMB')).toBe('CNY')
  })

  it('keeps valid ISO currency codes and rejects unknown symbols', () => {
    expect(resolveProviderQuotaCurrencyCode('USD')).toBe('USD')
    expect(resolveProviderQuotaCurrencyCode('JPY')).toBe('JPY')
    expect(resolveProviderQuotaCurrencyCode('tokens')).toBeUndefined()
    expect(resolveProviderQuotaCurrencyCode('')).toBeUndefined()
  })
})
