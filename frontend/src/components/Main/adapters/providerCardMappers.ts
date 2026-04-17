import type { ProviderDailyStat } from '../../../services/logs'
import type { BlacklistStatus } from '../../../services/blacklist'
import type { AutomationCard } from '../../../data/cards'
import type { ProviderTab } from '../types'
import {
  cloneBudgetQuotaAdjustments,
  cloneBudgetQuotaSettings,
  serializeOptionalBudgetQuotaAdjustments,
  serializeOptionalBudgetQuotaSettings,
} from '../../../utils/budgetUsage'
import {
  normalizeProviderQuotaQueryType,
  serializeProviderQuotaQueryType,
} from '../../../utils/providerQuotaQuery'
import type { GeminiProvider as GeminiProviderModel, Provider as PersistedProviderModel } from '../../../../bindings/codeswitch/services/models'
import { GetProviders as GetGeminiProviders } from '../../../../bindings/codeswitch/services/geminiservice'

export type GeminiProvider = Awaited<ReturnType<typeof GetGeminiProviders>> extends (infer P)[] ? (P & {
  sortOrder?: number
  enabledSortOrder?: number
  disabledSortOrder?: number
  budgetQuotaSettings?: unknown | null
  budgetQuotaUsedAdjustments?: unknown | null
  providerQuotaQueryType?: unknown | null
}) : any

export type PersistedProvider = PersistedProviderModel & {
  sortOrder?: number
  enabledSortOrder?: number
  disabledSortOrder?: number
  budgetQuotaSettings?: unknown | null
  budgetQuotaUsedAdjustments?: unknown | null
  providerQuotaQueryType?: unknown | null
}

const GEMINI_LOCKED_ENV_KEYS = new Set(['GOOGLE_GEMINI_BASE_URL', 'GEMINI_API_KEY'])

const cloneCardValue = <T>(value: T): T => {
  if (value == null) return value
  return JSON.parse(JSON.stringify(value))
}

const normalizeStringRecord = (source: Record<string, unknown> | null | undefined): Record<string, string> => {
  const normalized: Record<string, string> = {}
  Object.entries(source ?? {}).forEach(([key, value]) => {
    if (typeof value === 'string') {
      normalized[key] = value
    }
  })
  return normalized
}

const extractGeminiCliConfig = (provider: GeminiProvider): Record<string, any> => {
  const envConfig = provider?.envConfig ?? {}
  const cliConfig: Record<string, any> = {}

  Object.entries(envConfig).forEach(([key, value]) => {
    if (GEMINI_LOCKED_ENV_KEYS.has(key)) return
    cliConfig[key] = value
  })

  if (provider?.model) {
    cliConfig.GEMINI_MODEL = provider.model
  }

  return cliConfig
}

const buildGeminiEnvConfig = (card: AutomationCard, original: GeminiProvider): Record<string, string> => {
  const nextEnv = normalizeStringRecord(original?.envConfig)

  if (card.apiUrl) {
    nextEnv.GOOGLE_GEMINI_BASE_URL = card.apiUrl
  } else {
    delete nextEnv.GOOGLE_GEMINI_BASE_URL
  }

  if (card.apiKey) {
    nextEnv.GEMINI_API_KEY = card.apiKey
  } else {
    delete nextEnv.GEMINI_API_KEY
  }

  const cliConfig = cloneCardValue(card.cliConfig || {})
  Object.entries(cliConfig).forEach(([key, value]) => {
    if (GEMINI_LOCKED_ENV_KEYS.has(key)) return
    const text = `${value ?? ''}`.trim()
    if (text) {
      nextEnv[key] = text
    } else {
      delete nextEnv[key]
    }
  })

  return nextEnv
}

const normalizeAvailabilityConfig = (value: Record<string, any> | null | undefined) => {
  if (!value) return undefined
  return {
    testModel: `${value.testModel ?? ''}`.trim(),
    testEndpoint: `${value.testEndpoint ?? ''}`.trim(),
    timeout: Number(value.timeout) || 15000,
  }
}

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

export const providerToCard = (
  provider: PersistedProvider,
  platform?: ProviderTab | string,
): AutomationCard => ({
  id: Number(provider.id),
  providerRef: normalizeProviderRef(provider.id),
  name: provider.name,
  apiUrl: provider.apiUrl || '',
  apiKey: provider.apiKey || '',
  officialSite: provider.officialSite || '',
  icon: provider.icon || '',
  tint: provider.tint || '',
  accent: provider.accent || '',
  enabled: provider.enabled,
  apiFormat: platform === 'claude'
    ? ((provider.apiFormat as AutomationCard['apiFormat']) || 'anthropic')
    : undefined,
  sortOrder: provider.sortOrder || 0,
  enabledSortOrder: provider.enabledSortOrder || (provider.enabled ? (provider.sortOrder || 0) : undefined),
  disabledSortOrder: provider.disabledSortOrder || (!provider.enabled ? (provider.sortOrder || 0) : undefined),
  supportedModels: cloneCardValue(provider.supportedModels || {}),
  modelMapping: cloneCardValue(provider.modelMapping || {}),
  requestBodyOverrides: cloneCardValue(provider.requestBodyOverrides || {}),
  level: provider.level || 1,
  apiEndpoint: provider.apiEndpoint || '',
  cliConfig: cloneCardValue(provider.cliConfig || {}),
  availabilityMonitorEnabled: !!provider.availabilityMonitorEnabled,
  connectivityAutoBlacklist: !!provider.connectivityAutoBlacklist,
  availabilityConfig: normalizeAvailabilityConfig(provider.availabilityConfig),
  connectivityCheck: !!provider.connectivityCheck,
  connectivityTestModel: provider.connectivityTestModel || '',
  connectivityTestEndpoint: provider.connectivityTestEndpoint || '',
  connectivityAuthType: provider.connectivityAuthType || '',
  budgetQuotaSettings: provider.budgetQuotaSettings == null
    ? undefined
    : cloneBudgetQuotaSettings(provider.budgetQuotaSettings),
  budgetQuotaUsedAdjustments: provider.budgetQuotaUsedAdjustments == null
    ? undefined
    : cloneBudgetQuotaAdjustments(provider.budgetQuotaUsedAdjustments),
  providerQuotaQueryType: normalizeProviderQuotaQueryType(provider.providerQuotaQueryType),
})

export const deserializeProviders = (
  providers: PersistedProvider[],
  platform?: ProviderTab | string,
): AutomationCard[] => {
  return providers.map((provider) => providerToCard(provider, platform))
}

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
  sortOrder: provider.sortOrder || index + 1,
  enabledSortOrder: provider.enabledSortOrder || (provider.enabled ? (provider.sortOrder || index + 1) : undefined),
  disabledSortOrder: provider.disabledSortOrder || (!provider.enabled ? (provider.sortOrder || index + 1) : undefined),
  level: provider.level || 1,
  cliConfig: extractGeminiCliConfig(provider),
  requestBodyOverrides: cloneCardValue(provider.requestBodyOverrides || {}),
  budgetQuotaSettings: provider.budgetQuotaSettings == null
    ? undefined
    : cloneBudgetQuotaSettings(provider.budgetQuotaSettings),
  budgetQuotaUsedAdjustments: provider.budgetQuotaUsedAdjustments == null
    ? undefined
    : cloneBudgetQuotaAdjustments(provider.budgetQuotaUsedAdjustments),
  providerQuotaQueryType: normalizeProviderQuotaQueryType(provider.providerQuotaQueryType),
  availabilityMonitorEnabled: false,
  connectivityAutoBlacklist: false,
  availabilityConfig: undefined,
})

export const cardToGemini = (card: AutomationCard, original: GeminiProvider): GeminiProvider => ({
  ...original,
  name: card.name,
  baseUrl: card.apiUrl,
  apiKey: card.apiKey,
  model: `${card.cliConfig?.GEMINI_MODEL ?? ''}`.trim() || original.model || '',
  websiteUrl: card.officialSite,
  enabled: card.enabled,
  sortOrder: card.sortOrder || 0,
  enabledSortOrder: card.enabledSortOrder || 0,
  disabledSortOrder: card.disabledSortOrder || 0,
  level: card.level || 1,
  envConfig: buildGeminiEnvConfig(card, original),
  requestBodyOverrides: cloneCardValue(card.requestBodyOverrides || {}),
  budgetQuotaSettings: serializeOptionalBudgetQuotaSettings(card.budgetQuotaSettings),
  budgetQuotaUsedAdjustments: serializeOptionalBudgetQuotaAdjustments(card.budgetQuotaUsedAdjustments),
  providerQuotaQueryType: serializeProviderQuotaQueryType(card.providerQuotaQueryType),
})

export const createGeminiFromCard = (
  card: AutomationCard,
  providerID: string,
): GeminiProvider => ({
  id: providerID,
  name: card.name,
  baseUrl: card.apiUrl,
  apiKey: card.apiKey,
  model: `${card.cliConfig?.GEMINI_MODEL ?? ''}`.trim(),
  websiteUrl: card.officialSite,
  enabled: card.enabled,
  sortOrder: card.sortOrder || 0,
  enabledSortOrder: card.enabledSortOrder || 0,
  disabledSortOrder: card.disabledSortOrder || 0,
  level: card.level || 1,
  envConfig: buildGeminiEnvConfig(card, {} as GeminiProvider),
  requestBodyOverrides: cloneCardValue(card.requestBodyOverrides || {}),
  budgetQuotaSettings: serializeOptionalBudgetQuotaSettings(card.budgetQuotaSettings),
  budgetQuotaUsedAdjustments: serializeOptionalBudgetQuotaAdjustments(card.budgetQuotaUsedAdjustments),
  providerQuotaQueryType: serializeProviderQuotaQueryType(card.providerQuotaQueryType),
})

export const serializeProviders = (
  providers: AutomationCard[],
  platform?: ProviderTab | string,
): PersistedProvider[] =>
  providers.map((provider) => {
    const { providerRef, ...persistable } = provider
    return {
      ...persistable,
      ...(platform === 'claude'
        ? { apiFormat: provider.apiFormat || 'anthropic' }
        : {}),
      sortOrder: provider.sortOrder || 0,
      enabledSortOrder: provider.enabledSortOrder || 0,
      disabledSortOrder: provider.disabledSortOrder || 0,
      requestBodyOverrides: cloneCardValue(provider.requestBodyOverrides || {}),
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
      budgetQuotaSettings: serializeOptionalBudgetQuotaSettings(provider.budgetQuotaSettings),
      budgetQuotaUsedAdjustments: serializeOptionalBudgetQuotaAdjustments(provider.budgetQuotaUsedAdjustments),
      providerQuotaQueryType: serializeProviderQuotaQueryType(provider.providerQuotaQueryType),
    }
  })
