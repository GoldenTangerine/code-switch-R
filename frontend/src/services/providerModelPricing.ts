import { Call } from '@wailsio/runtime'
import type { AutomationCard } from '../data/cards'

const PROVIDER_SERVICE = 'codeswitch/services.ProviderService'

export type ProviderModelPricingSource = 'auto' | 'api/pricing' | 'one-hub' | 'v1/models'

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
  cacheCreate1hUsdPerM?: number
  perCallPrice?: ProviderModelPerCallPrice
}

export type ProviderModelPricingDebugAttempt = {
  source: string
  endpoint: string
  method: string
  url: string
  authType: string
  requestHeaders?: Record<string, string>
  statusCode?: number
  responseHeaders?: Record<string, string>
  contentType?: string
  responseBody?: string
  responseBodyBytes?: number
  responseBodyTruncated?: boolean
  durationMs?: number
  error?: string
}

export type ProviderModelPricingDebug = {
  baseUrl: string
  platform?: string
  requestedSource: string
  resolvedSource?: string
  configuredAuthType: string
  authCandidates?: string[]
  attempts?: ProviderModelPricingDebugAttempt[]
}

export type ProviderModelPricingResponse = {
  siteType: string
  pricingSource: ProviderModelPricingSource
  models: ProviderModelPricingItem[]
  fetchError?: string
  imported?: boolean
  challengeDetected?: boolean
  challengeMessage?: string
  debug?: ProviderModelPricingDebug
}

export async function fetchProviderModelPricing(
  provider: AutomationCard,
  platform: string,
  source: ProviderModelPricingSource = 'auto',
): Promise<ProviderModelPricingResponse> {
  return await Call.ByName(
    `${PROVIDER_SERVICE}.FetchProviderModelPricingWithSource`,
    provider.apiUrl,
    provider.apiKey,
    platform,
    provider.connectivityAuthType || '',
    source,
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
