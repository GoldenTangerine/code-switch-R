/**
 * @name: 供应商价格存在性测试
 * @Descripttion: 验证报价缺失与明确零价的展示边界。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-07 11:23:00
 * @LastEditTime: 2026-09-07 11:23:00
 * @FilePath: frontend/src/components/Main/modals/providerModelPricePresence.test.ts
 */
import { describe, expect, it } from 'vitest'
import type { ProviderModelPricingItem } from '../../../services/providerModelPricing'
import { normalizeProviderPricePresence } from './providerModelPricePresence'

const base: ProviderModelPricingItem = { model: 'example', quotaType: 0, modelRatio: 0, completionRatio: 0 }

describe('provider model price presence', () => {
  it('does not present missing fields or derived cache prices as free', () => {
    const result = normalizeProviderPricePresence({ ...base, priceFieldsKnown: true,
      inputUsdPerM: 0, outputUsdPerM: 0, resolvedCacheCreateMultiplier: 1.25,
      resolvedCacheReadMultiplier: 0.1, cacheCreateMultiplierSource: 'builtin',
    })
    expect(result.inputUsdPerM).toBeUndefined()
    expect(result.outputUsdPerM).toBeUndefined()
    expect(result.resolvedCacheCreateMultiplier).toBeUndefined()
    expect(result.resolvedCacheReadMultiplier).toBeUndefined()
  })

  it('restores omitted explicit zero prices while preserving fractional prices', () => {
    const result = normalizeProviderPricePresence({ ...base, priceFieldsKnown: true,
      hasInputPrice: true, hasOutputPrice: true, outputUsdPerM: 0.01234,
      hasCacheCreatePrice: true, hasCacheReadPrice: true, hasCacheCreate1hPrice: true,
    })
    expect(result.inputUsdPerM).toBe(0)
    expect(result.outputUsdPerM).toBe(0.01234)
    expect(result.cacheCreate1hUsdPerM).toBe(0)
    expect(result.resolvedCacheCreateMultiplier).toBe(0)
    expect(result.resolvedCacheReadMultiplier).toBe(0)
  })

  it('preserves legacy data and valid manual cache multipliers', () => {
    expect(normalizeProviderPricePresence(base)).toBe(base)
    const result = normalizeProviderPricePresence({ ...base, priceFieldsKnown: true,
      hasInputPrice: true, inputUsdPerM: 2, cacheReadMultiplierSource: 'manual',
      resolvedCacheReadMultiplier: 0.25,
    })
    expect(result.resolvedCacheReadMultiplier).toBe(0.25)
  })
})
