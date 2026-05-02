export type HomeProviderTab = 'claude' | 'codex' | 'gemini' | 'opencode' | 'others'

export type HomeProviderTabOption = {
  id: HomeProviderTab
  label: string
  icon: string
}

export const HOME_PROVIDER_TAB_OPTIONS = [
  { id: 'claude', label: 'Claude Code', icon: 'claude' },
  { id: 'codex', label: 'Codex', icon: 'openai' },
  { id: 'gemini', label: 'Gemini', icon: 'gemini' },
  { id: 'opencode', label: 'OpenCode', icon: 'opencode' },
  { id: 'others', label: '其他', icon: 'others' },
] as const satisfies readonly HomeProviderTabOption[]

export const DEFAULT_HOME_PROVIDER_TABS = ['claude', 'codex', 'gemini'] as const satisfies readonly HomeProviderTab[]

export const HOME_PROVIDER_TAB_IDS = HOME_PROVIDER_TAB_OPTIONS.map((tab) => tab.id) as HomeProviderTab[]

export const normalizeHomeProviderTabs = (value: unknown): HomeProviderTab[] => {
  if (!Array.isArray(value)) return [...DEFAULT_HOME_PROVIDER_TABS]

  const validTabIds = new Set<HomeProviderTab>(HOME_PROVIDER_TAB_IDS)
  const normalizedTabs = value.filter((tab): tab is HomeProviderTab => (
    typeof tab === 'string' && validTabIds.has(tab as HomeProviderTab)
  ))

  return normalizedTabs.length > 0
    ? [...new Set(normalizedTabs)]
    : [...DEFAULT_HOME_PROVIDER_TABS]
}
