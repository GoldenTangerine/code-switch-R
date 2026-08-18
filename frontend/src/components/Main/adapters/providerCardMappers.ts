import type { ProviderDailyStat } from '../../../services/logs'
import type { BlacklistStatus } from '../../../services/blacklist'
import type { AutomationCard, ClaudeDesktopModelRoute, ModelMappingMissPolicy } from '../../../data/cards'
import type { ProviderTab } from '../types'
import {
  createDefaultOpenCodeSettingsConfig,
  isDefaultOpenCodeModels,
  normalizeClaudeAPIFormatValue,
  normalizeClaudeDesktopModelRoutes,
  normalizeModelMappingMissPolicy,
  normalizeModelMappingDisabled,
  normalizeModelMappingReasoningEfforts,
  normalizeModelMappingSupports1M,
  normalizeProviderConcurrencyLimit,
  normalizeSessionMaxSessions,
  normalizeSessionTTLMinutes,
  resolvePersistedAnthropicCacheTTL,
} from './providerFormMappers'
import {
  cloneBudgetQuotaAdjustments,
  cloneBudgetQuotaSettings,
  serializeOptionalBudgetQuotaAdjustments,
  serializeOptionalBudgetQuotaSettings,
} from '../../../utils/budgetUsage'
import {
  normalizeProviderQuotaQueryConfig,
  normalizeProviderQuotaQueryType,
  sanitizeProviderQuotaQueryConfigForSave,
  serializeProviderQuotaQueryType,
} from '../../../utils/providerQuotaQuery'
import type { GeminiProvider as GeminiProviderModel, Provider as PersistedProviderModel } from '../../../../bindings/codeswitch/services/models'
import { GetProviders as GetGeminiProviders } from '../../../../bindings/codeswitch/services/geminiservice'
import type { OpenCodeProvider as OpenCodeProviderModel } from '../../../services/opencode'

export type GeminiProvider = Awaited<ReturnType<typeof GetGeminiProviders>> extends (infer P)[] ? (P & {
  sortOrder?: number
  enabledSortOrder?: number
  disabledSortOrder?: number
  budgetQuotaSettings?: unknown | null
  budgetQuotaUsedAdjustments?: unknown | null
  providerQuotaQueryType?: unknown | null
  providerQuotaQueryConfig?: unknown | null
  hideLogBadge?: boolean
  quotaAutoDisabled?: boolean
  quotaAutoDisablePaused?: boolean
}) : any

export type OpenCodeProvider = OpenCodeProviderModel & {
  icon?: string
  sortOrder?: number
  enabledSortOrder?: number
  disabledSortOrder?: number
  budgetQuotaSettings?: unknown | null
  budgetQuotaUsedAdjustments?: unknown | null
  providerQuotaQueryType?: unknown | null
  providerQuotaQueryConfig?: unknown | null
  quotaAutoDisabled?: boolean
  quotaAutoDisablePaused?: boolean
}

export type PersistedProvider = PersistedProviderModel & {
  sortOrder?: number
  enabledSortOrder?: number
  disabledSortOrder?: number
  budgetQuotaSettings?: unknown | null
  budgetQuotaUsedAdjustments?: unknown | null
  providerQuotaQueryType?: unknown | null
  providerQuotaQueryConfig?: unknown | null
  anthropicCacheTTL?: unknown | null
  modelMappingMissPolicy?: ModelMappingMissPolicy
  modelMappingDisabled?: Record<string, boolean>
  modelMappingReasoningEfforts?: Record<string, string>
  modelMappingSupports1M?: Record<string, boolean>
  modelPassthroughPatterns?: string[]
  opencodeNpm?: string
  opencodeSettingsConfig?: Record<string, any>
  configTOML?: string
  claudeDesktopMode?: string
  claudeDesktopModelRoutes?: ClaudeDesktopModelRoute[]
  apiKeyUrl?: string
  category?: string
  authProvider?: string
  authAccountId?: string
  partnerPromotionKey?: string
  liveConfigManaged?: boolean
  isInConfig?: boolean
  hideLogBadge?: boolean
  quotaAutoDisabled?: boolean
  quotaAutoDisablePaused?: boolean
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

const normalizeBooleanRecord = (source: Record<string, unknown> | null | undefined): Record<string, boolean> => {
  const normalized: Record<string, boolean> = {}
  Object.entries(source ?? {}).forEach(([key, value]) => {
    if (typeof value === 'boolean') {
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

// 供应商引用工具（normalizeProviderKey / normalizeProviderRef / cardProviderRef）
// 的权威实现位于 utils/providerRefs（叶子模块，避免 services -> Main 的跨 chunk 循环依赖）；
// 此处 re-export 保持既有 import 路径兼容
import {
  normalizeProviderKey,
  normalizeProviderRef,
  cardProviderRef,
} from '../../../utils/providerRefs'

export { normalizeProviderKey, normalizeProviderRef, cardProviderRef }

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

export const createOpenCodeProviderRef = () => `opencode-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`

const extractOpenCodeOptions = (settingsConfig?: Record<string, any> | null): Record<string, any> => {
  const options = settingsConfig?.options
  return options && typeof options === 'object' && !Array.isArray(options) ? options : {}
}

const extractOpenCodeBaseUrl = (provider: OpenCodeProvider): string => {
  const options = extractOpenCodeOptions(provider.settingsConfig)
  return provider.baseUrl || `${options.baseURL ?? options.baseUrl ?? options.url ?? ''}`
}

const extractOpenCodeApiKey = (provider: OpenCodeProvider): string => {
  const options = extractOpenCodeOptions(provider.settingsConfig)
  return provider.apiKey || `${options.apiKey ?? options.api_key ?? ''}`
}

const extractOpenCodeNpm = (provider: OpenCodeProvider): string => {
  return provider.npm || `${provider.settingsConfig?.npm ?? ''}` || '@ai-sdk/openai-compatible'
}

const buildOpenCodeSettingsConfig = (card: AutomationCard, original?: OpenCodeProvider): Record<string, any> => {
  const base = cloneCardValue(card.opencodeSettingsConfig || original?.settingsConfig || {}) || {}
  const npm = card.opencodeNpm || `${base.npm ?? original?.npm ?? ''}` || '@ai-sdk/openai-compatible'
  const settingsConfig: Record<string, any> = {
    ...base,
    npm,
    name: card.name || base.name || original?.name || '',
  }
  const options = extractOpenCodeOptions(settingsConfig)
  delete options.baseURL
  delete options.baseUrl
  delete options.url
  delete options.apiKey
  delete options.api_key
  delete options.APIKey
  if (options.setCacheKey === undefined) options.setCacheKey = true
  if (card.apiUrl) options.baseURL = card.apiUrl
  if (card.apiKey) options.apiKey = card.apiKey
  if (Object.keys(options).length > 0) {
    settingsConfig.options = options
  } else {
    delete settingsConfig.options
  }
  if (!settingsConfig.models || typeof settingsConfig.models !== 'object' || Array.isArray(settingsConfig.models) || isDefaultOpenCodeModels(settingsConfig.models)) {
    settingsConfig.models = createDefaultOpenCodeSettingsConfig(npm).models
  }
  return settingsConfig
}

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
  apiKeyUrl: provider.apiKeyUrl || '',
  icon: provider.icon || '',
  tint: provider.tint || '',
  accent: provider.accent || '',
  enabled: provider.enabled,
  quotaAutoDisabled: provider.quotaAutoDisabled === true,
  quotaAutoDisablePaused: provider.quotaAutoDisablePaused === true,
  hideLogBadge: provider.hideLogBadge === true,
  apiFormat: platform === 'claude'
    ? normalizeClaudeAPIFormatValue(provider.apiFormat)
    : undefined,
  anthropicCacheTTL: platform === 'claude'
    ? resolvePersistedAnthropicCacheTTL('claude', provider.apiFormat, provider.anthropicCacheTTL)
    : '',
  sortOrder: provider.sortOrder || 0,
  enabledSortOrder: provider.enabledSortOrder || ((provider.enabled || provider.quotaAutoDisabled) ? (provider.sortOrder || 0) : undefined),
  disabledSortOrder: provider.disabledSortOrder || ((!provider.enabled && !provider.quotaAutoDisabled) ? (provider.sortOrder || 0) : undefined),
  supportedModels: normalizeBooleanRecord(provider.supportedModels),
  modelMapping: normalizeStringRecord(provider.modelMapping),
  modelMappingDisabled: normalizeModelMappingDisabled(
    provider.modelMappingDisabled,
    normalizeStringRecord(provider.modelMapping),
  ),
  modelMappingReasoningEfforts: normalizeModelMappingReasoningEfforts(
    provider.modelMappingReasoningEfforts,
    normalizeStringRecord(provider.modelMapping),
  ),
  modelMappingSupports1M: normalizeModelMappingSupports1M(
    provider.modelMappingSupports1M,
    normalizeStringRecord(provider.modelMapping),
  ),
  modelMappingMissPolicy: normalizeModelMappingMissPolicy(provider.modelMappingMissPolicy),
  modelPassthroughPatterns: cloneCardValue(provider.modelPassthroughPatterns || []),
  requestBodyOverrides: cloneCardValue(provider.requestBodyOverrides || {}),
  level: provider.level || 1,
  providerConcurrencyLimit: normalizeProviderConcurrencyLimit(provider.providerConcurrencyLimit),
  sessionMaxSessions: normalizeSessionMaxSessions(provider.sessionMaxSessions),
  sessionTTLMinutes: normalizeSessionTTLMinutes(provider.sessionTTLMinutes),
  apiEndpoint: provider.apiEndpoint || '',
  opencodeNpm: provider.opencodeNpm || '',
  opencodeSettingsConfig: cloneCardValue(provider.opencodeSettingsConfig || {}),
  configTOML: provider.configTOML || '',
  claudeDesktopMode: provider.claudeDesktopMode || '',
  claudeDesktopModelRoutes: normalizeClaudeDesktopModelRoutes(provider.claudeDesktopModelRoutes),
  category: provider.category || '',
  authProvider: provider.authProvider || '',
  authAccountId: provider.authAccountId || '',
  partnerPromotionKey: provider.partnerPromotionKey || '',
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
  providerQuotaQueryConfig: provider.providerQuotaQueryConfig == null
    ? undefined
    : cloneCardValue(normalizeProviderQuotaQueryConfig(
        provider.providerQuotaQueryConfig,
        provider.providerQuotaQueryType,
      )),
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
  quotaAutoDisabled: provider.quotaAutoDisabled === true,
  quotaAutoDisablePaused: provider.quotaAutoDisablePaused === true,
  hideLogBadge: provider.hideLogBadge === true,
  sortOrder: provider.sortOrder || index + 1,
  enabledSortOrder: provider.enabledSortOrder || ((provider.enabled || provider.quotaAutoDisabled) ? (provider.sortOrder || index + 1) : undefined),
  disabledSortOrder: provider.disabledSortOrder || ((!provider.enabled && !provider.quotaAutoDisabled) ? (provider.sortOrder || index + 1) : undefined),
  level: provider.level || 1,
  providerConcurrencyLimit: normalizeProviderConcurrencyLimit(provider.providerConcurrencyLimit),
  sessionMaxSessions: normalizeSessionMaxSessions(provider.sessionMaxSessions),
  sessionTTLMinutes: normalizeSessionTTLMinutes(provider.sessionTTLMinutes),
  cliConfig: extractGeminiCliConfig(provider),
  requestBodyOverrides: cloneCardValue(provider.requestBodyOverrides || {}),
  budgetQuotaSettings: provider.budgetQuotaSettings == null
    ? undefined
    : cloneBudgetQuotaSettings(provider.budgetQuotaSettings),
  budgetQuotaUsedAdjustments: provider.budgetQuotaUsedAdjustments == null
    ? undefined
    : cloneBudgetQuotaAdjustments(provider.budgetQuotaUsedAdjustments),
  providerQuotaQueryType: normalizeProviderQuotaQueryType(provider.providerQuotaQueryType),
  providerQuotaQueryConfig: provider.providerQuotaQueryConfig == null
    ? undefined
    : cloneCardValue(normalizeProviderQuotaQueryConfig(
        provider.providerQuotaQueryConfig,
        provider.providerQuotaQueryType,
      )),
  availabilityMonitorEnabled: false,
  connectivityAutoBlacklist: false,
  availabilityConfig: undefined,
})

export const opencodeToCard = (provider: OpenCodeProvider, index: number): AutomationCard => {
  const settingsConfig = cloneCardValue(provider.settingsConfig || {})
  return {
    id: 400 + index,
    providerRef: normalizeProviderRef(provider.id),
    name: provider.name,
    apiUrl: extractOpenCodeBaseUrl(provider),
    apiKey: extractOpenCodeApiKey(provider),
    officialSite: provider.websiteUrl || '',
    apiKeyUrl: provider.apiKeyUrl || '',
    icon: provider.icon || 'opencode',
    tint: 'rgba(14, 165, 233, 0.16)',
    accent: '#0ea5e9',
    enabled: provider.enabled,
    quotaAutoDisabled: provider.quotaAutoDisabled === true,
    quotaAutoDisablePaused: provider.quotaAutoDisablePaused === true,
    sortOrder: provider.sortOrder || index + 1,
    enabledSortOrder: provider.enabledSortOrder || ((provider.enabled || provider.quotaAutoDisabled) ? (provider.sortOrder || index + 1) : undefined),
    disabledSortOrder: provider.disabledSortOrder || ((!provider.enabled && !provider.quotaAutoDisabled) ? (provider.sortOrder || index + 1) : undefined),
    level: provider.level || 1,
    opencodeNpm: extractOpenCodeNpm(provider),
    opencodeSettingsConfig: settingsConfig,
    category: provider.category || '',
    partnerPromotionKey: provider.partnerPromotionKey || '',
    liveConfigManaged: provider.liveConfigManaged === true,
    isInConfig: provider.isInConfig === true,
    requestBodyOverrides: cloneCardValue(provider.requestBodyOverrides || {}),
    budgetQuotaSettings: provider.budgetQuotaSettings == null
      ? undefined
      : cloneBudgetQuotaSettings(provider.budgetQuotaSettings),
    budgetQuotaUsedAdjustments: provider.budgetQuotaUsedAdjustments == null
      ? undefined
      : cloneBudgetQuotaAdjustments(provider.budgetQuotaUsedAdjustments),
    providerQuotaQueryType: normalizeProviderQuotaQueryType(provider.providerQuotaQueryType),
    providerQuotaQueryConfig: provider.providerQuotaQueryConfig == null
      ? undefined
      : cloneCardValue(normalizeProviderQuotaQueryConfig(
          provider.providerQuotaQueryConfig,
          provider.providerQuotaQueryType,
        )),
    availabilityMonitorEnabled: false,
    connectivityAutoBlacklist: false,
    availabilityConfig: undefined,
  }
}

export const cardToGemini = (card: AutomationCard, original: GeminiProvider): GeminiProvider => ({
  ...original,
  name: card.name,
  baseUrl: card.apiUrl,
  apiKey: card.apiKey,
  model: `${card.cliConfig?.GEMINI_MODEL ?? ''}`.trim() || original.model || '',
  websiteUrl: card.officialSite,
  enabled: card.enabled,
  quotaAutoDisabled: card.quotaAutoDisabled === true,
  quotaAutoDisablePaused: card.quotaAutoDisablePaused === true,
  hideLogBadge: card.hideLogBadge === true,
  sortOrder: card.sortOrder || 0,
  enabledSortOrder: card.enabledSortOrder || 0,
  disabledSortOrder: card.disabledSortOrder || 0,
  level: card.level || 1,
  providerConcurrencyLimit: normalizeProviderConcurrencyLimit(card.providerConcurrencyLimit),
  sessionMaxSessions: normalizeSessionMaxSessions(card.sessionMaxSessions),
  sessionTTLMinutes: normalizeSessionTTLMinutes(card.sessionTTLMinutes),
  envConfig: buildGeminiEnvConfig(card, original),
  requestBodyOverrides: cloneCardValue(card.requestBodyOverrides || {}),
  budgetQuotaSettings: serializeOptionalBudgetQuotaSettings(card.budgetQuotaSettings),
  budgetQuotaUsedAdjustments: serializeOptionalBudgetQuotaAdjustments(card.budgetQuotaUsedAdjustments),
  providerQuotaQueryType: serializeProviderQuotaQueryType(card.providerQuotaQueryType),
  providerQuotaQueryConfig: sanitizeProviderQuotaQueryConfigForSave(
    cloneCardValue(card.providerQuotaQueryConfig),
    card.providerQuotaQueryType,
  ),
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
  quotaAutoDisabled: card.quotaAutoDisabled === true,
  quotaAutoDisablePaused: card.quotaAutoDisablePaused === true,
  hideLogBadge: card.hideLogBadge === true,
  sortOrder: card.sortOrder || 0,
  enabledSortOrder: card.enabledSortOrder || 0,
  disabledSortOrder: card.disabledSortOrder || 0,
  level: card.level || 1,
  providerConcurrencyLimit: normalizeProviderConcurrencyLimit(card.providerConcurrencyLimit),
  sessionMaxSessions: normalizeSessionMaxSessions(card.sessionMaxSessions),
  sessionTTLMinutes: normalizeSessionTTLMinutes(card.sessionTTLMinutes),
  envConfig: buildGeminiEnvConfig(card, {} as GeminiProvider),
  requestBodyOverrides: cloneCardValue(card.requestBodyOverrides || {}),
  budgetQuotaSettings: serializeOptionalBudgetQuotaSettings(card.budgetQuotaSettings),
  budgetQuotaUsedAdjustments: serializeOptionalBudgetQuotaAdjustments(card.budgetQuotaUsedAdjustments),
  providerQuotaQueryType: serializeProviderQuotaQueryType(card.providerQuotaQueryType),
  providerQuotaQueryConfig: sanitizeProviderQuotaQueryConfigForSave(
    cloneCardValue(card.providerQuotaQueryConfig),
    card.providerQuotaQueryType,
  ),
})

export const cardToOpenCode = (card: AutomationCard, original: OpenCodeProvider): OpenCodeProvider => ({
  ...original,
  id: normalizeProviderRef(card.providerRef) || original.id,
  name: card.name,
  websiteUrl: card.officialSite,
  apiKeyUrl: card.apiKeyUrl || original.apiKeyUrl || '',
  baseUrl: card.apiUrl,
  apiKey: card.apiKey,
  npm: card.opencodeNpm || original.npm || '@ai-sdk/openai-compatible',
  icon: card.icon || original.icon || 'opencode',
  category: card.category || original.category || '',
  partnerPromotionKey: card.partnerPromotionKey || original.partnerPromotionKey || '',
  enabled: card.enabled,
  quotaAutoDisabled: card.quotaAutoDisabled === true,
  quotaAutoDisablePaused: card.quotaAutoDisablePaused === true,
  liveConfigManaged: card.enabled,
  isInConfig: card.enabled,
  sortOrder: card.sortOrder || 0,
  enabledSortOrder: card.enabledSortOrder || 0,
  disabledSortOrder: card.disabledSortOrder || 0,
  level: card.level || 1,
  settingsConfig: buildOpenCodeSettingsConfig(card, original),
  requestBodyOverrides: cloneCardValue(card.requestBodyOverrides || {}),
  budgetQuotaSettings: serializeOptionalBudgetQuotaSettings(card.budgetQuotaSettings),
  budgetQuotaUsedAdjustments: serializeOptionalBudgetQuotaAdjustments(card.budgetQuotaUsedAdjustments),
  providerQuotaQueryType: serializeProviderQuotaQueryType(card.providerQuotaQueryType),
  providerQuotaQueryConfig: sanitizeProviderQuotaQueryConfigForSave(
    cloneCardValue(card.providerQuotaQueryConfig),
    card.providerQuotaQueryType,
  ),
})

export const createOpenCodeFromCard = (
  card: AutomationCard,
  providerID: string,
): OpenCodeProvider => ({
  id: providerID,
  name: card.name,
  websiteUrl: card.officialSite,
  apiKeyUrl: card.apiKeyUrl || '',
  baseUrl: card.apiUrl,
  apiKey: card.apiKey,
  npm: card.opencodeNpm || '@ai-sdk/openai-compatible',
  icon: card.icon || 'opencode',
  category: card.category || '',
  partnerPromotionKey: card.partnerPromotionKey || '',
  enabled: card.enabled,
  quotaAutoDisabled: card.quotaAutoDisabled === true,
  quotaAutoDisablePaused: card.quotaAutoDisablePaused === true,
  liveConfigManaged: card.enabled,
  isInConfig: card.enabled,
  sortOrder: card.sortOrder || 0,
  enabledSortOrder: card.enabledSortOrder || 0,
  disabledSortOrder: card.disabledSortOrder || 0,
  level: card.level || 1,
  settingsConfig: buildOpenCodeSettingsConfig(card),
  requestBodyOverrides: cloneCardValue(card.requestBodyOverrides || {}),
  budgetQuotaSettings: serializeOptionalBudgetQuotaSettings(card.budgetQuotaSettings),
  budgetQuotaUsedAdjustments: serializeOptionalBudgetQuotaAdjustments(card.budgetQuotaUsedAdjustments),
  providerQuotaQueryType: serializeProviderQuotaQueryType(card.providerQuotaQueryType),
  providerQuotaQueryConfig: sanitizeProviderQuotaQueryConfigForSave(
    cloneCardValue(card.providerQuotaQueryConfig),
    card.providerQuotaQueryType,
  ),
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
      anthropicCacheTTL: platform === 'claude'
        ? resolvePersistedAnthropicCacheTTL('claude', provider.apiFormat, provider.anthropicCacheTTL)
        : '',
      sortOrder: provider.sortOrder || 0,
      enabledSortOrder: provider.enabledSortOrder || 0,
      disabledSortOrder: provider.disabledSortOrder || 0,
      providerConcurrencyLimit: normalizeProviderConcurrencyLimit(provider.providerConcurrencyLimit),
      sessionMaxSessions: normalizeSessionMaxSessions(provider.sessionMaxSessions),
      sessionTTLMinutes: normalizeSessionTTLMinutes(provider.sessionTTLMinutes),
      requestBodyOverrides: cloneCardValue(provider.requestBodyOverrides || {}),
      opencodeNpm: provider.opencodeNpm || '',
      opencodeSettingsConfig: cloneCardValue(provider.opencodeSettingsConfig || {}),
      configTOML: provider.configTOML || '',
      claudeDesktopMode: provider.claudeDesktopMode || '',
      claudeDesktopModelRoutes: normalizeClaudeDesktopModelRoutes(provider.claudeDesktopModelRoutes),
      apiKeyUrl: provider.apiKeyUrl || '',
      category: provider.category || '',
      authProvider: provider.authProvider || '',
      authAccountId: provider.authAccountId || '',
      partnerPromotionKey: provider.partnerPromotionKey || '',
      liveConfigManaged: provider.liveConfigManaged,
      isInConfig: provider.isInConfig,
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
      providerQuotaQueryConfig: sanitizeProviderQuotaQueryConfigForSave(
        cloneCardValue(provider.providerQuotaQueryConfig),
        provider.providerQuotaQueryType,
      ),
    }
  })
