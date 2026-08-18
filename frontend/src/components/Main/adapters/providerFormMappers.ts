import type { AutomationCard, ClaudeDesktopModelRoute, ModelMappingMissPolicy } from '../../../data/cards'
import { cloneBudgetQuotaAdjustments } from '../../../utils/budgetUsage'
import {
  hasProviderQuotaQueryType,
  normalizeProviderQuotaQueryConfig,
  normalizeProviderQuotaQueryType,
  sanitizeProviderQuotaQueryConfigForSave,
  serializeProviderQuotaQueryType,
} from '../../../utils/providerQuotaQuery'
import { getDefaultAuthType, getDefaultEndpoint } from '../constants'
import type { ProviderTab, VendorForm } from '../types'

type NormalizeLevelFn = (level: number | string | undefined) => number
type ClaudeAPIFormat = 'anthropic' | 'openai_chat' | 'openai_responses'
type AnthropicCacheTTL = '' | '5m' | '1h'

export type ProviderAuthState = {
  selectedAuthType: string
  customAuthHeader: string
}

export const cloneProviderValue = <T>(value: T): T => {
  if (value == null) return value
  return JSON.parse(JSON.stringify(value))
}

export const normalizeClaudeAPIFormatValue = (value: unknown): ClaudeAPIFormat => {
  const normalized = `${value ?? ''}`.trim().toLowerCase()
  if (normalized === 'openai_chat' || normalized === 'openai_responses') {
    return normalized
  }
  return 'anthropic'
}

export const normalizeAnthropicCacheTTL = (value: unknown): AnthropicCacheTTL => {
  const normalized = `${value ?? ''}`.trim().toLowerCase()
  return normalized === '5m' || normalized === '1h' ? normalized : ''
}

export const normalizeModelMappingMissPolicy = (value: unknown): ModelMappingMissPolicy => {
  const normalized = `${value ?? ''}`.trim().toLowerCase()
  return normalized === 'passthrough' ? 'passthrough' : 'block'
}

export const normalizeClaudeDesktopMode = (value: unknown): string => {
  const normalized = `${value ?? ''}`.trim().toLowerCase()
  return normalized === 'proxy' ? 'proxy' : 'direct'
}

// 归一化 Claude Desktop 模型路由：过滤空 name、按 name 去重，兼容 snake_case 键
export const normalizeClaudeDesktopModelRoutes = (value: unknown): ClaudeDesktopModelRoute[] => {
  if (!Array.isArray(value)) return []
  const seen = new Set<string>()
  const routes: ClaudeDesktopModelRoute[] = []
  value.forEach((entry) => {
    if (!entry || typeof entry !== 'object') return
    const record = entry as Record<string, unknown>
    const name = `${record.name ?? record.Name ?? ''}`.trim()
    if (!name || seen.has(name)) return
    seen.add(name)
    const labelOverride = `${record.labelOverride ?? record.label_override ?? ''}`.trim()
    const route: ClaudeDesktopModelRoute = { name }
    if (labelOverride) route.labelOverride = labelOverride
    if (record.supports1m === true || record.Supports1M === true || record.supports_1m === true) {
      route.supports1m = true
    }
    routes.push(route)
  })
  return routes
}

export const normalizeModelMappingReasoningEfforts = (
  value: unknown,
  modelMapping: Record<string, string> | undefined,
): Record<string, string> => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  const normalized: Record<string, string> = {}
  Object.entries(value as Record<string, unknown>).forEach(([key, effort]) => {
    const normalizedEffort = `${effort ?? ''}`.trim()
    if (normalizedEffort && Object.prototype.hasOwnProperty.call(modelMapping || {}, key)) {
      normalized[key] = normalizedEffort
    }
  })
  return normalized
}

export const normalizeModelMappingDisabled = (
  value: unknown,
  modelMapping: Record<string, string> | undefined,
): Record<string, boolean> => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  const normalized: Record<string, boolean> = {}
  Object.entries(value as Record<string, unknown>).forEach(([key, disabled]) => {
    if (disabled === true && Object.prototype.hasOwnProperty.call(modelMapping || {}, key)) {
      normalized[key] = true
    }
  })
  return normalized
}

export const normalizeModelMappingSupports1M = (
  value: unknown,
  modelMapping: Record<string, string> | undefined,
): Record<string, boolean> => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  const normalized: Record<string, boolean> = {}
  Object.entries(value as Record<string, unknown>).forEach(([key, supports1M]) => {
    if (supports1M === true && Object.prototype.hasOwnProperty.call(modelMapping || {}, key)) {
      normalized[key] = true
    }
  })
  return normalized
}

export const normalizeProviderConcurrencyLimit = (value: unknown): number | undefined => {
  const raw = `${value ?? ''}`.trim()
  if (raw === '') return undefined
  const numeric = Number(raw)
  if (!Number.isFinite(numeric) || numeric < 0) return undefined
  return Math.min(999, Math.floor(numeric))
}

export const normalizeSessionMaxSessions = (value: unknown): number => {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric < 1) return 5
  return Math.min(999, Math.floor(numeric))
}

export const normalizeSessionTTLMinutes = (value: unknown): number => {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric < 1) return 30
  return Math.min(1440, Math.floor(numeric))
}


export const resolvePersistedAnthropicCacheTTL = (
  tabId: ProviderTab,
  apiFormat: unknown,
  ttl: unknown,
): AnthropicCacheTTL => (
  tabId === 'claude' && normalizeClaudeAPIFormatValue(apiFormat) === 'anthropic'
    ? normalizeAnthropicCacheTTL(ttl)
    : ''
)

export const getDefaultOpenCodeModels = (npm = '@ai-sdk/openai-compatible') => {
  switch (npm.trim()) {
    case '@ai-sdk/anthropic':
      return { 'claude-3-5-sonnet-latest': { name: 'Claude 3.5 Sonnet' } }
    case '@ai-sdk/google':
      return { 'gemini-2.5-pro': { name: 'Gemini 2.5 Pro' } }
    default:
      return { 'gpt-4o': { name: 'GPT-4o' } }
  }
}

export const isDefaultOpenCodeModels = (models: unknown) => {
  if (!models || typeof models !== 'object' || Array.isArray(models)) return false
  const modelKeys = Object.keys(models as Record<string, unknown>)
  if (modelKeys.length !== 1) return false
  return ['gpt-4o', 'claude-3-5-sonnet-latest', 'gemini-2.5-pro'].includes(modelKeys[0])
}

export const createDefaultOpenCodeSettingsConfig = (
  npm = '@ai-sdk/openai-compatible',
  name = 'OpenCode Provider',
  baseUrl = '',
  apiKey = '',
): Record<string, any> => {
  const options: Record<string, any> = { setCacheKey: true }
  if (baseUrl) options.baseURL = baseUrl
  if (apiKey) options.apiKey = apiKey

  return {
    npm,
    name,
    ...(Object.keys(options).length > 0 ? { options } : {}),
    models: getDefaultOpenCodeModels(npm),
  }
}

export const createDefaultVendorForm = (
  platform: ProviderTab | string,
  defaultIconKey: string,
): VendorForm => ({
  providerRef: '',
  name: '',
  apiUrl: '',
  apiKey: '',
  officialSite: '',
  apiKeyUrl: '',
  icon: defaultIconKey,
  level: 1,
  hideLogBadge: false,
  providerConcurrencyLimit: undefined,
  sessionMaxSessions: 5,
  sessionTTLMinutes: 30,
  enabled: true,
  apiFormat: platform === 'claude' ? 'anthropic' : undefined,
  anthropicCacheTTL: '',
  supportedModels: {},
  modelMapping: {},
  modelMappingDisabled: {},
  modelMappingReasoningEfforts: {},
  modelMappingSupports1M: {},
  modelMappingMissPolicy: 'block',
  modelPassthroughPatterns: [],
  requestBodyOverrides: {},
  cliConfig: {},
  apiEndpoint: '',
  opencodeNpm: platform === 'opencode' ? '@ai-sdk/openai-compatible' : undefined,
  opencodeSettingsConfig: platform === 'opencode'
    ? createDefaultOpenCodeSettingsConfig()
    : undefined,
  configTOML: '',
  claudeDesktopMode: platform === 'claude-desktop' ? 'direct' : undefined,
  claudeDesktopModelRoutes: platform === 'claude-desktop' ? [] : undefined,
  category: platform === 'opencode' || platform === 'grokbuild' || platform === 'claude-desktop' || platform === 'openclaw' || platform === 'hermes' || platform === 'pi' ? 'custom' : undefined,
  authProvider: '',
  authAccountId: '',
  partnerPromotionKey: '',
  liveConfigManaged: platform === 'opencode' ? true : undefined,
  isInConfig: platform === 'opencode' ? true : undefined,
  availabilityMonitorEnabled: false,
  connectivityAutoBlacklist: false,
  availabilityConfig: {
    testModel: '',
    testEndpoint: getDefaultEndpoint(platform),
    timeout: 15000,
  },
  connectivityCheck: false,
  connectivityTestModel: '',
  connectivityTestEndpoint: '',
  connectivityAuthType: '',
  budgetQuotaSettings: undefined,
  budgetQuotaUsedAdjustments: undefined,
  providerQuotaQueryType: 'none',
  providerQuotaQueryConfig: undefined,
  quotaAutoDisabled: false,
  quotaAutoDisablePaused: false,
})

export const createVendorFormFromCard = (
  card: AutomationCard,
  tabId: ProviderTab,
): VendorForm => ({
  providerRef: card.providerRef || '',
  name: card.name,
  apiUrl: card.apiUrl,
  apiKey: card.apiKey,
  officialSite: card.officialSite,
  apiKeyUrl: card.apiKeyUrl || '',
  icon: card.icon,
  level: card.level || 1,
  hideLogBadge: card.hideLogBadge === true,
  providerConcurrencyLimit: card.providerConcurrencyLimit ?? undefined,
  sessionMaxSessions: normalizeSessionMaxSessions(card.sessionMaxSessions),
  sessionTTLMinutes: normalizeSessionTTLMinutes(card.sessionTTLMinutes),
  enabled: card.quotaAutoDisabled ? true : card.enabled,
  quotaAutoDisabled: card.quotaAutoDisabled === true,
  quotaAutoDisablePaused: card.quotaAutoDisablePaused === true,
  apiFormat: tabId === 'claude' ? normalizeClaudeAPIFormatValue(card.apiFormat) : undefined,
  anthropicCacheTTL: resolvePersistedAnthropicCacheTTL(tabId, card.apiFormat, card.anthropicCacheTTL),
  supportedModels: cloneProviderValue(card.supportedModels || {}),
  modelMapping: cloneProviderValue(card.modelMapping || {}),
  modelMappingDisabled: normalizeModelMappingDisabled(
    card.modelMappingDisabled,
    card.modelMapping,
  ),
  modelMappingReasoningEfforts: normalizeModelMappingReasoningEfforts(
    card.modelMappingReasoningEfforts,
    card.modelMapping,
  ),
  modelMappingSupports1M: normalizeModelMappingSupports1M(
    card.modelMappingSupports1M,
    card.modelMapping,
  ),
  modelMappingMissPolicy: normalizeModelMappingMissPolicy(card.modelMappingMissPolicy),
  modelPassthroughPatterns: cloneProviderValue(card.modelPassthroughPatterns || []),
  requestBodyOverrides: cloneProviderValue(card.requestBodyOverrides || {}),
  cliConfig: cloneProviderValue(card.cliConfig || {}),
  apiEndpoint: card.apiEndpoint || '',
  opencodeNpm: card.opencodeNpm || '',
  opencodeSettingsConfig: cloneProviderValue(card.opencodeSettingsConfig || {}),
  configTOML: card.configTOML || '',
  claudeDesktopMode: tabId === 'claude-desktop' ? normalizeClaudeDesktopMode(card.claudeDesktopMode) : undefined,
  claudeDesktopModelRoutes: tabId === 'claude-desktop'
    ? normalizeClaudeDesktopModelRoutes(card.claudeDesktopModelRoutes)
    : undefined,
  category: card.category || '',
  authProvider: card.authProvider || '',
  authAccountId: card.authAccountId || '',
  partnerPromotionKey: card.partnerPromotionKey || '',
  liveConfigManaged: card.liveConfigManaged,
  isInConfig: card.isInConfig,
  // 旧字段兼容统一放在这里，避免创建、编辑两条链路口径飘掉。
  availabilityMonitorEnabled: card.availabilityMonitorEnabled ?? card.connectivityCheck ?? false,
  connectivityAutoBlacklist: card.connectivityAutoBlacklist ?? false,
  availabilityConfig: {
    testModel: card.availabilityConfig?.testModel || card.connectivityTestModel || '',
    testEndpoint:
      card.availabilityConfig?.testEndpoint ||
      card.connectivityTestEndpoint ||
      getDefaultEndpoint(tabId),
    timeout: card.availabilityConfig?.timeout || 15000,
  },
  connectivityCheck: false,
  connectivityTestModel: '',
  connectivityTestEndpoint: '',
  connectivityAuthType: card.connectivityAuthType || '',
  budgetQuotaSettings: cloneProviderValue(card.budgetQuotaSettings) || undefined,
  budgetQuotaUsedAdjustments: cloneBudgetQuotaAdjustments(card.budgetQuotaUsedAdjustments),
  providerQuotaQueryType: normalizeProviderQuotaQueryType(card.providerQuotaQueryType),
  providerQuotaQueryConfig: cloneProviderValue(normalizeProviderQuotaQueryConfig(
    card.providerQuotaQueryConfig,
    card.providerQuotaQueryType,
  )),
})

export const resolveProviderAuthState = (
  connectivityAuthType: string | undefined,
  tabId: ProviderTab,
): ProviderAuthState => {
  const storedAuth = (connectivityAuthType || '').trim()
  const normalizedStoredAuth = storedAuth.toLowerCase()

  if (!storedAuth) {
    return {
      selectedAuthType: getDefaultAuthType(tabId),
      customAuthHeader: '',
    }
  }

  if (normalizedStoredAuth === 'bearer' || normalizedStoredAuth === 'x-api-key') {
    return {
      selectedAuthType: normalizedStoredAuth,
      customAuthHeader: '',
    }
  }

  return {
    selectedAuthType: getDefaultAuthType(tabId),
    customAuthHeader: storedAuth,
  }
}

export const buildNormalizedVendorForm = ({
  form,
  tabId,
  defaultIconKey,
  resolveAuthType,
}: {
  form: VendorForm
  tabId: ProviderTab
  defaultIconKey: string
  resolveAuthType: () => string
}): VendorForm => {
  const apiFormat = tabId === 'claude' ? normalizeClaudeAPIFormatValue(form.apiFormat) : undefined
  const hasRemoteQuota = hasProviderQuotaQueryType(form.providerQuotaQueryConfig ?? form.providerQuotaQueryType, form.providerQuotaQueryType)
  const preserveAutoDisabled = hasRemoteQuota && form.enabled && form.quotaAutoDisabled === true
  const preservePaused = hasRemoteQuota && form.enabled && form.quotaAutoDisablePaused === true
  const persistedEnabled = preserveAutoDisabled ? false : form.enabled
  return {
    name: form.name.trim(),
    apiUrl: form.apiUrl.trim(),
    apiKey: form.apiKey.trim(),
    officialSite: form.officialSite.trim(),
    apiKeyUrl: `${form.apiKeyUrl ?? ''}`.trim(),
    icon: (form.icon || defaultIconKey).toString().trim().toLowerCase() || defaultIconKey,
    level: form.level || 1,
    hideLogBadge: form.hideLogBadge === true,
    providerConcurrencyLimit: normalizeProviderConcurrencyLimit(form.providerConcurrencyLimit),
    sessionMaxSessions: normalizeSessionMaxSessions(form.sessionMaxSessions),
    sessionTTLMinutes: normalizeSessionTTLMinutes(form.sessionTTLMinutes),
    enabled: persistedEnabled,
    quotaAutoDisabled: preserveAutoDisabled,
    quotaAutoDisablePaused: preservePaused,
    apiFormat,
    anthropicCacheTTL: resolvePersistedAnthropicCacheTTL(tabId, apiFormat, form.anthropicCacheTTL),
    supportedModels: cloneProviderValue(form.supportedModels || {}),
    modelMapping: cloneProviderValue(form.modelMapping || {}),
    modelMappingDisabled: normalizeModelMappingDisabled(
      form.modelMappingDisabled,
      form.modelMapping,
    ),
    modelMappingReasoningEfforts: normalizeModelMappingReasoningEfforts(
      form.modelMappingReasoningEfforts,
      form.modelMapping,
    ),
    modelMappingSupports1M: normalizeModelMappingSupports1M(
      form.modelMappingSupports1M,
      form.modelMapping,
    ),
    modelMappingMissPolicy: normalizeModelMappingMissPolicy(form.modelMappingMissPolicy),
    modelPassthroughPatterns: cloneProviderValue(form.modelPassthroughPatterns || []),
    requestBodyOverrides: cloneProviderValue(form.requestBodyOverrides || {}),
    cliConfig: cloneProviderValue(form.cliConfig || {}),
    apiEndpoint: form.apiEndpoint || '',
    providerRef: form.providerRef || '',
    opencodeNpm: form.opencodeNpm || '',
    opencodeSettingsConfig: cloneProviderValue(form.opencodeSettingsConfig || {}),
    configTOML: `${form.configTOML ?? ''}`,
    claudeDesktopMode: tabId === 'claude-desktop' ? normalizeClaudeDesktopMode(form.claudeDesktopMode) : undefined,
    claudeDesktopModelRoutes: tabId === 'claude-desktop'
      ? normalizeClaudeDesktopModelRoutes(form.claudeDesktopModelRoutes)
      : undefined,
    category: form.category || '',
    authProvider: form.authProvider || '',
    authAccountId: form.authAccountId || '',
    partnerPromotionKey: form.partnerPromotionKey || '',
    liveConfigManaged: tabId === 'opencode' ? persistedEnabled : form.liveConfigManaged,
    isInConfig: tabId === 'opencode' ? persistedEnabled : form.isInConfig,
    availabilityMonitorEnabled: tabId === 'opencode' ? false : !!form.availabilityMonitorEnabled,
    connectivityAutoBlacklist: tabId === 'opencode' ? false : !!form.connectivityAutoBlacklist,
    availabilityConfig: tabId === 'opencode'
      ? { testModel: '', testEndpoint: '', timeout: 15000 }
      : {
        testModel: form.availabilityConfig?.testModel || '',
        testEndpoint: form.availabilityConfig?.testEndpoint || getDefaultEndpoint(tabId),
        timeout: form.availabilityConfig?.timeout || 15000,
      },
    connectivityCheck: false,
    connectivityTestModel: '',
    connectivityTestEndpoint: '',
    connectivityAuthType: resolveAuthType().trim() || getDefaultAuthType(tabId),
    budgetQuotaSettings: cloneProviderValue(form.budgetQuotaSettings) || undefined,
    budgetQuotaUsedAdjustments: cloneBudgetQuotaAdjustments(form.budgetQuotaUsedAdjustments),
    providerQuotaQueryType: normalizeProviderQuotaQueryType(form.providerQuotaQueryType),
    providerQuotaQueryConfig: cloneProviderValue(normalizeProviderQuotaQueryConfig(
      form.providerQuotaQueryConfig,
      form.providerQuotaQueryType,
    )),
  }
}

export const buildPersistedProviderFieldsFromForm = (
  form: VendorForm,
  tabId: ProviderTab,
  normalizeLevel: NormalizeLevelFn,
) => {
  const apiFormat = tabId === 'claude' ? normalizeClaudeAPIFormatValue(form.apiFormat) : undefined
  const hasRemoteQuota = hasProviderQuotaQueryType(form.providerQuotaQueryConfig ?? form.providerQuotaQueryType, form.providerQuotaQueryType)
  const preserveAutoDisabled = hasRemoteQuota && form.enabled && form.quotaAutoDisabled === true
  const preservePaused = hasRemoteQuota && form.enabled && form.quotaAutoDisablePaused === true
  const persistedEnabled = preserveAutoDisabled ? false : form.enabled
  return {
    apiKey: form.apiKey,
    officialSite: form.officialSite,
    apiKeyUrl: form.apiKeyUrl || '',
    icon: form.icon,
    level: normalizeLevel(form.level),
    hideLogBadge: form.hideLogBadge === true,
    providerConcurrencyLimit: normalizeProviderConcurrencyLimit(form.providerConcurrencyLimit),
    sessionMaxSessions: normalizeSessionMaxSessions(form.sessionMaxSessions),
    sessionTTLMinutes: normalizeSessionTTLMinutes(form.sessionTTLMinutes),
    enabled: persistedEnabled,
    quotaAutoDisabled: preserveAutoDisabled,
    quotaAutoDisablePaused: preservePaused,
    apiFormat,
    anthropicCacheTTL: resolvePersistedAnthropicCacheTTL(tabId, apiFormat, form.anthropicCacheTTL),
    supportedModels: cloneProviderValue(form.supportedModels || {}),
    modelMapping: cloneProviderValue(form.modelMapping || {}),
    modelMappingDisabled: normalizeModelMappingDisabled(
      form.modelMappingDisabled,
      form.modelMapping,
    ),
    modelMappingReasoningEfforts: normalizeModelMappingReasoningEfforts(
      form.modelMappingReasoningEfforts,
      form.modelMapping,
    ),
    modelMappingSupports1M: normalizeModelMappingSupports1M(
      form.modelMappingSupports1M,
      form.modelMapping,
    ),
    modelMappingMissPolicy: normalizeModelMappingMissPolicy(form.modelMappingMissPolicy),
    modelPassthroughPatterns: cloneProviderValue(form.modelPassthroughPatterns || []),
    requestBodyOverrides: cloneProviderValue(form.requestBodyOverrides || {}),
    cliConfig: cloneProviderValue(form.cliConfig || {}),
    apiEndpoint: form.apiEndpoint || '',
    providerRef: form.providerRef || '',
    opencodeNpm: form.opencodeNpm || '',
    opencodeSettingsConfig: cloneProviderValue(form.opencodeSettingsConfig || {}),
    configTOML: `${form.configTOML ?? ''}`,
    claudeDesktopMode: tabId === 'claude-desktop' ? normalizeClaudeDesktopMode(form.claudeDesktopMode) : undefined,
    claudeDesktopModelRoutes: tabId === 'claude-desktop'
      ? normalizeClaudeDesktopModelRoutes(form.claudeDesktopModelRoutes)
      : undefined,
    category: form.category || '',
    authProvider: form.authProvider || '',
    authAccountId: form.authAccountId || '',
    partnerPromotionKey: form.partnerPromotionKey || '',
    liveConfigManaged: tabId === 'opencode' ? persistedEnabled : form.liveConfigManaged,
    isInConfig: tabId === 'opencode' ? persistedEnabled : form.isInConfig,
    availabilityMonitorEnabled: tabId === 'opencode' ? false : !!form.availabilityMonitorEnabled,
    connectivityAutoBlacklist: tabId === 'opencode' ? false : !!form.connectivityAutoBlacklist,
    availabilityConfig: tabId === 'opencode'
      ? { testModel: '', testEndpoint: '', timeout: 15000 }
      : {
        testModel: form.availabilityConfig?.testModel || '',
        testEndpoint: form.availabilityConfig?.testEndpoint || getDefaultEndpoint(tabId),
        timeout: form.availabilityConfig?.timeout || 15000,
      },
    connectivityCheck: false,
    connectivityTestModel: '',
    connectivityTestEndpoint: '',
    connectivityAuthType: form.connectivityAuthType || '',
    budgetQuotaSettings: cloneProviderValue(form.budgetQuotaSettings) || undefined,
    budgetQuotaUsedAdjustments: cloneBudgetQuotaAdjustments(form.budgetQuotaUsedAdjustments),
    providerQuotaQueryType: serializeProviderQuotaQueryType(form.providerQuotaQueryType),
    providerQuotaQueryConfig: sanitizeProviderQuotaQueryConfigForSave(
      cloneProviderValue(form.providerQuotaQueryConfig),
      form.providerQuotaQueryType,
    ),
  }
}
