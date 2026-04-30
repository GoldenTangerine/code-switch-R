import type { AutomationCard } from '../../../data/cards'
import { cloneBudgetQuotaAdjustments } from '../../../utils/budgetUsage'
import {
  normalizeProviderQuotaQueryConfig,
  normalizeProviderQuotaQueryType,
  sanitizeProviderQuotaQueryConfigForSave,
  serializeProviderQuotaQueryType,
} from '../../../utils/providerQuotaQuery'
import { getDefaultAuthType, getDefaultEndpoint } from '../constants'
import type { ProviderTab, VendorForm } from '../types'

type NormalizeLevelFn = (level: number | string | undefined) => number

export type ProviderAuthState = {
  selectedAuthType: string
  customAuthHeader: string
}

export const cloneProviderValue = <T>(value: T): T => {
  if (value == null) return value
  return JSON.parse(JSON.stringify(value))
}

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
  enabled: true,
  apiFormat: platform === 'claude' ? 'anthropic' : undefined,
  supportedModels: {},
  modelMapping: {},
  requestBodyOverrides: {},
  cliConfig: {},
  apiEndpoint: '',
  opencodeNpm: platform === 'opencode' ? '@ai-sdk/openai-compatible' : undefined,
  opencodeSettingsConfig: platform === 'opencode'
    ? createDefaultOpenCodeSettingsConfig()
    : undefined,
  category: platform === 'opencode' ? 'custom' : undefined,
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
  enabled: card.enabled,
  apiFormat: tabId === 'claude' ? (card.apiFormat || 'anthropic') : undefined,
  supportedModels: cloneProviderValue(card.supportedModels || {}),
  modelMapping: cloneProviderValue(card.modelMapping || {}),
  requestBodyOverrides: cloneProviderValue(card.requestBodyOverrides || {}),
  cliConfig: cloneProviderValue(card.cliConfig || {}),
  apiEndpoint: card.apiEndpoint || '',
  opencodeNpm: card.opencodeNpm || '',
  opencodeSettingsConfig: cloneProviderValue(card.opencodeSettingsConfig || {}),
  category: card.category || '',
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
}): VendorForm => ({
  name: form.name.trim(),
  apiUrl: form.apiUrl.trim(),
  apiKey: form.apiKey.trim(),
  officialSite: form.officialSite.trim(),
  apiKeyUrl: `${form.apiKeyUrl ?? ''}`.trim(),
  icon: (form.icon || defaultIconKey).toString().trim().toLowerCase() || defaultIconKey,
  level: form.level || 1,
  enabled: form.enabled,
  apiFormat: tabId === 'claude' ? (form.apiFormat || 'anthropic') : undefined,
  supportedModels: cloneProviderValue(form.supportedModels || {}),
  modelMapping: cloneProviderValue(form.modelMapping || {}),
  requestBodyOverrides: cloneProviderValue(form.requestBodyOverrides || {}),
  cliConfig: cloneProviderValue(form.cliConfig || {}),
  apiEndpoint: form.apiEndpoint || '',
  providerRef: form.providerRef || '',
  opencodeNpm: form.opencodeNpm || '',
  opencodeSettingsConfig: cloneProviderValue(form.opencodeSettingsConfig || {}),
  category: form.category || '',
  partnerPromotionKey: form.partnerPromotionKey || '',
  liveConfigManaged: tabId === 'opencode' ? form.enabled : form.liveConfigManaged,
  isInConfig: tabId === 'opencode' ? form.enabled : form.isInConfig,
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
})

export const buildPersistedProviderFieldsFromForm = (
  form: VendorForm,
  tabId: ProviderTab,
  normalizeLevel: NormalizeLevelFn,
) => ({
  apiKey: form.apiKey,
  officialSite: form.officialSite,
  apiKeyUrl: form.apiKeyUrl || '',
  icon: form.icon,
  level: normalizeLevel(form.level),
  enabled: form.enabled,
  apiFormat: tabId === 'claude' ? (form.apiFormat || 'anthropic') : undefined,
  supportedModels: cloneProviderValue(form.supportedModels || {}),
  modelMapping: cloneProviderValue(form.modelMapping || {}),
  requestBodyOverrides: cloneProviderValue(form.requestBodyOverrides || {}),
  cliConfig: cloneProviderValue(form.cliConfig || {}),
  apiEndpoint: form.apiEndpoint || '',
  providerRef: form.providerRef || '',
  opencodeNpm: form.opencodeNpm || '',
  opencodeSettingsConfig: cloneProviderValue(form.opencodeSettingsConfig || {}),
  category: form.category || '',
  partnerPromotionKey: form.partnerPromotionKey || '',
  liveConfigManaged: tabId === 'opencode' ? form.enabled : form.liveConfigManaged,
  isInConfig: tabId === 'opencode' ? form.enabled : form.isInConfig,
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
})
