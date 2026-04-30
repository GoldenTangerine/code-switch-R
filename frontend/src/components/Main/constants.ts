import type { ProviderTab } from './types'

export const MAIN_TABS = [
  { id: 'claude', label: 'Claude Code' },
  { id: 'codex', label: 'Codex' },
  { id: 'gemini', label: 'Gemini' },
  { id: 'opencode', label: 'OpenCode' },
  { id: 'others', label: '其他' },
] as const

export const PROVIDER_TAB_IDS = MAIN_TABS.map((tab) => tab.id) as ProviderTab[]

export const RELEASE_PAGE_URL = 'https://github.com/GoldenTangerine/code-switch-R/releases'
export const RELEASE_API_URL = 'https://api.github.com/repos/GoldenTangerine/code-switch-R/releases/latest'
export const RELEASE_API_POLL_INTERVAL_MS = 30 * 60 * 1000

export const PROVIDER_PRICING_CLICK_THROTTLE_MS = 2_000
export const PROVIDER_PRICING_STARTUP_CONCURRENCY = 4

export const SUCCESS_RATE_THRESHOLDS = {
  healthy: 0.95,
  warning: 0.8,
} as const

export const AUTH_TYPE_OPTIONS = [
  { value: 'bearer', label: 'Bearer' },
  { value: 'x-api-key', label: 'X-API-Key' },
] as const

export const getDefaultEndpoint = (platform: string) => {
  const defaults: Record<string, string> = {
    claude: '/v1/messages',
    codex: '/responses',
    opencode: '/v1/chat/completions',
  }
  return defaults[platform] || '/v1/chat/completions'
}

export const getDefaultAuthType = (_platform: string) => 'bearer'
