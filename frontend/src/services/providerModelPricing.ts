import { Call } from '@wailsio/runtime'
import type { AutomationCard } from '../data/cards'

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
    'codeswitch/services.ProviderService.FetchProviderModelPricing',
    provider.apiUrl,
    provider.apiKey,
    platform,
    provider.connectivityAuthType || '',
  )
}

