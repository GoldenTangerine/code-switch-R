import type { AutomationCard } from '../../data/cards'
import type { BlacklistStatus } from '../../services/blacklist'
import type { ConfigFile, ProxyInjection } from '../../services/customCliService'

export type TranslateFn = (key: string, ...args: any[]) => string

export type ProviderTab = 'claude' | 'codex' | 'gemini' | 'others'

export type MainTabOption = {
  id: ProviderTab
  label: string
}

export type ResolvedTheme = 'light' | 'dark'

export type ProviderDragTargetPosition = 'before' | 'after'

export type ProviderDragTarget = {
  id: number
  position: ProviderDragTargetPosition
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
  name: string
  apiUrl: string
  apiKey: string
  officialSite: string
  icon: string
  enabled: boolean
  supportedModels?: Record<string, boolean>
  modelMapping?: Record<string, string>
  requestBodyOverrides?: Record<string, any>
  level?: number
  apiEndpoint?: string
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
  formattedOfficialSite: string
  iconSvg: string
  vendorInitials: string
}

export type CustomCliToolDraft = {
  name: string
  configFiles: ConfigFile[]
  proxyInjection: ProxyInjection[]
}
