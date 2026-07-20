import type { AutomationCard, ModelMappingMissPolicy } from '../../data/cards'
import type { BlacklistStatus } from '../../services/blacklist'
import type { ConfigFile, ProxyInjection } from '../../services/customCliService'
import type { BudgetQuotaAdjustments, BudgetQuotaKey, BudgetQuotaSettings } from '../../utils/budgetUsage'
import type { ProviderQuotaQueryConfig, ProviderQuotaQueryType } from '../../utils/providerQuotaQuery'

export type TranslateFn = (key: string, ...args: any[]) => string

export type ProviderTab = 'claude' | 'codex' | 'gemini' | 'opencode' | 'others'

export type MainTabOption = {
  id: ProviderTab
  label: string
  icon: string
}

export interface MainTabStatus {
  proxyEnabled: boolean
  concurrencyLimited: boolean
  proxySupported: boolean
}

export type ResolvedTheme = 'light' | 'dark'

export type ProviderDragTargetPosition = 'before' | 'after'

export type ProviderDragTarget = {
  id: number
  position: ProviderDragTargetPosition
}

export type ProviderDragEndPayload = {
  dropEffect: DataTransfer['dropEffect'] | 'none'
  clientX: number | null
  clientY: number | null
  endedInsideList?: boolean | null
}

export interface LastUsedProvider {
  platform: string
  source_platform?: string
  tool_id?: string
  provider_id?: string
  provider_name: string
  updated_at: number
}

export type VendorForm = {
  providerRef?: string
  name: string
  apiUrl: string
  apiKey: string
  officialSite: string
  apiKeyUrl?: string
  icon: string
  enabled: boolean
  apiFormat?: 'anthropic' | 'openai_chat' | 'openai_responses'
  anthropicCacheTTL?: '' | '5m' | '1h'
  supportedModels?: Record<string, boolean>
  modelMapping?: Record<string, string>
  modelMappingDisabled?: Record<string, boolean>
  modelMappingReasoningEfforts?: Record<string, string>
  modelMappingSupports1M?: Record<string, boolean>
  modelMappingMissPolicy?: ModelMappingMissPolicy
  modelPassthroughPatterns?: string[]
  requestBodyOverrides?: Record<string, any>
  level?: number
  providerConcurrencyLimit?: number
  sessionMaxSessions?: number
  sessionTTLMinutes?: number
  apiEndpoint?: string
  opencodeNpm?: string
  opencodeSettingsConfig?: Record<string, any>
  category?: string
  authProvider?: string
  authAccountId?: string
  partnerPromotionKey?: string
  liveConfigManaged?: boolean
  isInConfig?: boolean
  cliConfig?: Record<string, any>
  cliConfigPersistValue?: Record<string, any>
  cliConfigShouldPersist?: boolean
  availabilityMonitorEnabled?: boolean
  connectivityAutoBlacklist?: boolean
  availabilityConfig?: {
    testModel?: string
    testEndpoint?: string
    timeout?: number
  }
  connectivityCheck?: boolean
  connectivityTestModel?: string
  connectivityTestEndpoint?: string
  connectivityAuthType?: string
  budgetQuotaSettings?: BudgetQuotaSettings
  budgetQuotaUsedAdjustments?: BudgetQuotaAdjustments
  providerQuotaQueryType?: ProviderQuotaQueryType
  providerQuotaQueryConfig?: ProviderQuotaQueryConfig
}

export type ProviderQuotaDisplayItem = {
  key: string
  label: string
  used: number
  total: number
  progressRatio: number
  countdownLabel: string
  nextReset: Date | null
  queriedAt?: number
  valueMode?: 'currency' | 'count'
  unit?: string
  extra?: string
  invalidMessage?: string
  refreshErrorMessage?: string
}

export type ProviderStatDisplay =
  | {
      state: 'loading' | 'empty'
      message: string
      unreadFailedRequests: number
      hasUnreadErrorLogs: boolean
    }
  | {
      state: 'ready'
      requests: string
      tokens: string
      costLabel: string
      costParts: ProviderCostDisplayPart[]
      costFormatted: string
      costValue: number
      ttft: string
      tps: string
      performanceHint: string
      successRateLabel: string
      successRateClass: string
      failedRequests: number
      unreadFailedRequests: number
      hasUnreadErrorLogs: boolean
    }

export type ProviderCostDisplayPart = {
  type: 'currency' | 'amount'
  value: string
}

export type ProviderCardViewModel = {
  card: AutomationCard
  dragging: boolean
  dragOver: boolean
  isLastUsed: boolean
  isDefaultHostedProvider: boolean
  isHighlighted: boolean
  isDirectApplied: boolean
  blacklistStatus: BlacklistStatus | null
  connectivityClass: string
  connectivityTooltip: string
  stats: ProviderStatDisplay
  concurrencyStatus?: ProviderConcurrencyStatusView
  concurrencyLimitEnabled: boolean
  quotaDisplay: ProviderQuotaDisplayItem[]
  quotaRefreshing: boolean
  formattedOfficialSite: string
  iconSvg: string
  vendorInitials: string
}

export type ProviderConcurrencyStatusView = {
  platform: string
  providerId: string
  providerName: string
  activeRequests: number
  limit: number
  requests?: ProviderConcurrencyRequestView[]
}

export type ProviderConcurrencyRequestView = {
  id: string
  platform: string
  providerId: string
  providerName: string
  userAgent?: string
  requestedModel?: string
  model?: string
  mappedModel?: string
  modelMappingPattern?: string
  modelMappingTarget?: string
  modelOverride?: string
  modelRouteCaptured?: boolean
  parameters?: ProviderConcurrencyRequestParameterView[]
  endpoint?: string
  isStream?: boolean
  startedAt: number
  durationMs: number
}

export type ProviderConcurrencyRequestParameterView = {
  key: 'reasoning_effort' | 'max_output_tokens'
  requestedValue?: string
  actualValue?: string
  source?: 'request' | 'request_body_override' | 'model_mapping' | ''
}


export type CustomCliToolDraft = {
  name: string
  configFiles: ConfigFile[]
  proxyInjection: ProxyInjection[]
}
