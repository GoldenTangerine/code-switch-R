import { Call } from '@wailsio/runtime'
import type { AutomationCard } from '../data/cards'

const PROVIDER_SERVICE = 'codeswitch/services.ProviderService'

export type ProviderModelPerCallPrice = {
  unified?: number
  input?: number
  output?: number
}

export type ProviderModelPricingItem = {
  model: string
  description?: string
  // 0=token pricing, 1=per-call, -1=unknown (fallback /v1/models)
  quotaType: number
  modelRatio: number
  completionRatio: number
  cacheCreateMultiplier?: number
  cacheReadMultiplier?: number
  resolvedCacheCreateMultiplier?: number
  resolvedCacheReadMultiplier?: number
  cacheCreateMultiplierSource?: 'manual' | 'provider' | 'builtin' | 'fallback' | string
  cacheReadMultiplierSource?: 'manual' | 'provider' | 'builtin' | 'fallback' | string
  ownerBy?: string
  inputUsdPerM?: number
  outputUsdPerM?: number
  perCallPrice?: ProviderModelPerCallPrice
}

export type ProviderModelPricingResponse = {
  siteType: string
  pricingSource: string
  models: ProviderModelPricingItem[]
}

export async function fetchProviderModelPricing(
  provider: AutomationCard,
  platform: string,
): Promise<ProviderModelPricingResponse> {
  return await Call.ByName(
    `${PROVIDER_SERVICE}.FetchProviderModelPricing`,
    provider.apiUrl,
    provider.apiKey,
    platform,
    provider.connectivityAuthType || '',
  )
}

export async function upsertProviderModelPricingOverride(
  provider: AutomationCard,
  model: string,
  cacheCreateMultiplier: number,
  hasCacheCreateMultiplier: boolean,
  cacheReadMultiplier: number,
  hasCacheReadMultiplier: boolean,
): Promise<void> {
  await Call.ByName(
    `${PROVIDER_SERVICE}.UpsertProviderModelPricingOverride`,
    provider.apiUrl,
    provider.apiKey,
    provider.connectivityAuthType || '',
    model,
    cacheCreateMultiplier,
    hasCacheCreateMultiplier,
    cacheReadMultiplier,
    hasCacheReadMultiplier,
  )
}

export async function deleteProviderModelPricingOverride(
  provider: AutomationCard,
  model: string,
): Promise<void> {
  await Call.ByName(
    `${PROVIDER_SERVICE}.DeleteProviderModelPricingOverride`,
    provider.apiUrl,
    provider.apiKey,
    provider.connectivityAuthType || '',
    model,
  )
}
