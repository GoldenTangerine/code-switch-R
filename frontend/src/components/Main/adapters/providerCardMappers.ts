import type { ProviderDailyStat } from '../../../services/logs'
import type { BlacklistStatus } from '../../../services/blacklist'
import type { AutomationCard } from '../../../data/cards'
import { GetProviders as GetGeminiProviders } from '../../../../bindings/codeswitch/services/geminiservice'

export type GeminiProvider = Awaited<ReturnType<typeof GetGeminiProviders>> extends (infer P)[] ? P : any

export const normalizeProviderKey = (value: string) => value?.trim().toLowerCase() ?? ''

export const normalizeProviderRef = (value: string | number | null | undefined) => `${value ?? ''}`.trim()

export const cardProviderRef = (card: AutomationCard): string => {
  const ref = normalizeProviderRef(card.providerRef)
  if (ref) return ref
  if (Number.isFinite(card.id)) return `${card.id}`
  return ''
}

export const providerStatsKeyFromStat = (stat: ProviderDailyStat): string => {
  const ref = normalizeProviderRef(stat.provider_id)
  if (ref) return ref
  return normalizeProviderKey(stat.provider)
}

export const blacklistStatusKeyFromStatus = (status: BlacklistStatus): string => {
  const ref = normalizeProviderRef(status.providerId)
  if (ref) return ref
  return normalizeProviderKey(status.providerName)
}

export const blacklistStatusKeyFromCard = (card: AutomationCard): string => {
  const ref = cardProviderRef(card)
  if (ref) return ref
  return normalizeProviderKey(card.name)
}

export const createGeminiProviderRef = () => `gemini-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`

export const geminiToCard = (provider: GeminiProvider, index: number): AutomationCard => ({
  id: 300 + index,
  providerRef: normalizeProviderRef(provider.id),
  name: provider.name,
  apiUrl: provider.baseUrl || '',
  apiKey: provider.apiKey || '',
  officialSite: provider.websiteUrl || '',
  icon: 'gemini',
  tint: 'rgba(251, 146, 60, 0.18)',
  accent: '#fb923c',
  enabled: provider.enabled,
  level: provider.level || 1,
  availabilityMonitorEnabled: false,
  connectivityAutoBlacklist: false,
  availabilityConfig: undefined,
})

export const cardToGemini = (card: AutomationCard, original: GeminiProvider): GeminiProvider => ({
  ...original,
  name: card.name,
  baseUrl: card.apiUrl,
  apiKey: card.apiKey,
  websiteUrl: card.officialSite,
  enabled: card.enabled,
  level: card.level || 1,
})

export const serializeProviders = (providers: AutomationCard[]) =>
  providers.map((provider) => {
    const { providerRef, ...persistable } = provider
    return {
      ...persistable,
      availabilityMonitorEnabled: !!provider.availabilityMonitorEnabled,
      connectivityAutoBlacklist: !!provider.connectivityAutoBlacklist,
      availabilityConfig: provider.availabilityConfig
        ? {
            testModel: provider.availabilityConfig.testModel || '',
            testEndpoint: provider.availabilityConfig.testEndpoint || '',
            timeout: provider.availabilityConfig.timeout || 15000,
          }
        : undefined,
      connectivityCheck: false,
      connectivityTestModel: '',
      connectivityTestEndpoint: '',
      connectivityAuthType: provider.connectivityAuthType || '',
    }
  })
