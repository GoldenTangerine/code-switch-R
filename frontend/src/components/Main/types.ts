import type { AutomationCard } from '../../data/cards'
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
  supportedModels?: Record<string, boolean>
  modelMapping?: Record<string, string>
  requestBodyOverrides?: Record<string, any>
  level?: number
  apiEndpoint?: string
  opencodeNpm?: string
  opencodeSettingsConfig?: Record<string, any>
  category?: string
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
  | { state: 'loading' | 'empty'; message: string }
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
      hasErrorLogsToday: boolean
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
  quotaDisplay: ProviderQuotaDisplayItem[]
  quotaRefreshing: boolean
  formattedOfficialSite: string
  iconSvg: string
  vendorInitials: string
}

export type CustomCliToolDraft = {
  name: string
  configFiles: ConfigFile[]
  proxyInjection: ProxyInjection[]
}
