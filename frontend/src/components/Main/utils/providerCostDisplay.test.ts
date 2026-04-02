import { describe, expect, it } from 'vitest'
import { buildProviderCostDisplay, getProviderCostFormatter } from './providerCostDisplay'

describe('buildProviderCostDisplay', () => {
  it('keeps currency before amount for zh locale', () => {
    const display = buildProviderCostDisplay(25.79, 'zh')

    expect(display.formatted).toBe(new Intl.NumberFormat('zh', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(25.79))
    expect(display.parts).toEqual([
      { type: 'currency', value: 'US$' },
      { type: 'amount', value: '25.79' },
    ])
  })

  it('keeps amount before currency for currency-suffix locales', () => {
    const display = buildProviderCostDisplay(25.79, 'fr-FR')

    expect(display.formatted).toBe(new Intl.NumberFormat('fr-FR', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(25.79))
    expect(display.parts).toEqual([
      { type: 'amount', value: '25,79' },
      { type: 'currency', value: '$US' },
    ])
  })

  it('reuses formatter instances and clamps invalid input to zero', () => {
    const first = getProviderCostFormatter('en')
    const second = getProviderCostFormatter('en')
    const display = buildProviderCostDisplay(Number.NaN, 'en')

    expect(second).toBe(first)
    expect(display.formatted).toBe('$0.00')
    expect(display.parts).toEqual([
      { type: 'currency', value: '$' },
      { type: 'amount', value: '0.00' },
    ])
  })
})
