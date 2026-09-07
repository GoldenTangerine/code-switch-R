/**
 * @name: 供应商价格存在性展示
 * @Descripttion: 区分缺失报价和序列化省略的明确零价。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-07 11:23:00
 * @LastEditTime: 2026-09-07 11:23:00
 * @FilePath: frontend/src/components/Main/modals/providerModelPricePresence.ts
 */
import type { ProviderModelPricingItem } from '../../../services/providerModelPricing'

export function normalizeProviderPricePresence(item: ProviderModelPricingItem): ProviderModelPricingItem {
  if (!item.priceFieldsKnown) return item
  const hasCreate = item.hasCacheCreatePrice || (item.hasInputPrice && item.cacheCreateMultiplierSource === 'manual')
  const hasRead = item.hasCacheReadPrice || (item.hasInputPrice && item.cacheReadMultiplierSource === 'manual')
  return {
    ...item,
    inputUsdPerM: item.hasInputPrice ? item.inputUsdPerM ?? 0 : undefined,
    outputUsdPerM: item.hasOutputPrice ? item.outputUsdPerM ?? 0 : undefined,
    cacheCreate1hUsdPerM: item.hasCacheCreate1hPrice ? item.cacheCreate1hUsdPerM ?? 0 : undefined,
    cacheCreateMultiplier: hasCreate ? item.cacheCreateMultiplier ?? 0 : undefined,
    resolvedCacheCreateMultiplier: hasCreate ? item.resolvedCacheCreateMultiplier ?? item.cacheCreateMultiplier ?? 0 : undefined,
    cacheReadMultiplier: hasRead ? item.cacheReadMultiplier ?? 0 : undefined,
    resolvedCacheReadMultiplier: hasRead ? item.resolvedCacheReadMultiplier ?? item.cacheReadMultiplier ?? 0 : undefined,
    cacheCreateMultiplierSource: hasCreate ? item.cacheCreateMultiplierSource : undefined,
    cacheReadMultiplierSource: hasRead ? item.cacheReadMultiplierSource : undefined,
  }
}
