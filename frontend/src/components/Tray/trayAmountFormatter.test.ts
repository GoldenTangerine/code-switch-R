import { describe, expect, it } from 'vitest'
import {
  formatTrayQuotaValueParts,
  joinTrayAmountParts,
} from './trayAmountFormatter'

describe('trayAmountFormatter', () => {
  it('splits USD currency into unit and amount parts', () => {
    const parts = formatTrayQuotaValueParts(476.83, 'currency', 'USD', 'en-US')

    expect(parts.some((part) => part.role === 'unit')).toBe(true)
    expect(parts.some((part) => part.role === 'amount')).toBe(true)
    expect(joinTrayAmountParts(parts)).toContain('476.83')
  })

  it('splits CNY currency aliases into unit and amount parts', () => {
    const parts = formatTrayQuotaValueParts(12.34, 'currency', 'RMB', 'zh-CN')

    expect(parts.some((part) => part.role === 'unit')).toBe(true)
    expect(parts.some((part) => part.role === 'amount')).toBe(true)
    expect(joinTrayAmountParts(parts)).toContain('12.34')
  })

  it('keeps count values and units as separate parts', () => {
    expect(formatTrayQuotaValueParts(42, 'count', 'tokens', 'en-US')).toEqual([
      { role: 'amount', value: '42' },
      { role: 'literal', value: ' ' },
      { role: 'unit', value: 'tokens' },
    ])
  })
})
