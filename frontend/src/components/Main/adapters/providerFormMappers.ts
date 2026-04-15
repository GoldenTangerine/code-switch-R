import type { AutomationCard } from '../../../data/cards'
import { cloneBudgetQuotaAdjustments } from '../../../utils/budgetUsage'
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

export const createDefaultVendorForm = (
  platform: ProviderTab | string,
  defaultIconKey: string,
): VendorForm => ({
  name: '',
  apiUrl: '',
  apiKey: '',
  officialSite: '',
  icon: defaultIconKey,
  level: 1,
  enabled: true,
  apiFormat: platform === 'claude' ? 'anthropic' : undefined,
  supportedModels: {},
  modelMapping: {},
  requestBodyOverrides: {},
  cliConfig: {},
  apiEndpoint: '',
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
})

export const createVendorFormFromCard = (
  card: AutomationCard,
  tabId: ProviderTab,
): VendorForm => ({
  name: card.name,
  apiUrl: card.apiUrl,
  apiKey: card.apiKey,
  officialSite: card.officialSite,
  icon: card.icon,
  level: card.level || 1,
  enabled: card.enabled,
  apiFormat: tabId === 'claude' ? (card.apiFormat || 'anthropic') : undefined,
  supportedModels: cloneProviderValue(card.supportedModels || {}),
  modelMapping: cloneProviderValue(card.modelMapping || {}),
  requestBodyOverrides: cloneProviderValue(card.requestBodyOverrides || {}),
  cliConfig: cloneProviderValue(card.cliConfig || {}),
  apiEndpoint: card.apiEndpoint || '',
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
  icon: (form.icon || defaultIconKey).toString().trim().toLowerCase() || defaultIconKey,
  level: form.level || 1,
  enabled: form.enabled,
  apiFormat: tabId === 'claude' ? (form.apiFormat || 'anthropic') : undefined,
  supportedModels: cloneProviderValue(form.supportedModels || {}),
  modelMapping: cloneProviderValue(form.modelMapping || {}),
  requestBodyOverrides: cloneProviderValue(form.requestBodyOverrides || {}),
  cliConfig: cloneProviderValue(form.cliConfig || {}),
  apiEndpoint: form.apiEndpoint || '',
  availabilityMonitorEnabled: !!form.availabilityMonitorEnabled,
  connectivityAutoBlacklist: !!form.connectivityAutoBlacklist,
  availabilityConfig: {
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
})

export const buildPersistedProviderFieldsFromForm = (
  form: VendorForm,
  tabId: ProviderTab,
  normalizeLevel: NormalizeLevelFn,
) => ({
  apiKey: form.apiKey,
  officialSite: form.officialSite,
  icon: form.icon,
  level: normalizeLevel(form.level),
  enabled: form.enabled,
  apiFormat: tabId === 'claude' ? (form.apiFormat || 'anthropic') : undefined,
  supportedModels: cloneProviderValue(form.supportedModels || {}),
  modelMapping: cloneProviderValue(form.modelMapping || {}),
  requestBodyOverrides: cloneProviderValue(form.requestBodyOverrides || {}),
  cliConfig: cloneProviderValue(form.cliConfig || {}),
  apiEndpoint: form.apiEndpoint || '',
  availabilityMonitorEnabled: !!form.availabilityMonitorEnabled,
  connectivityAutoBlacklist: !!form.connectivityAutoBlacklist,
  availabilityConfig: {
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
})
