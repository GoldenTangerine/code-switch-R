export type HomeProviderTab = 'claude' | 'codex' | 'gemini' | 'opencode' | 'grokbuild' | 'claude-desktop' | 'openclaw' | 'hermes' | 'pi' | 'others'

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
  { id: 'grokbuild', label: 'Grok Build', icon: 'grok' },
  { id: 'claude-desktop', label: 'Claude Desktop', icon: 'claude' },
  { id: 'openclaw', label: 'OpenClaw', icon: 'openclaw' },
  { id: 'hermes', label: 'Hermes', icon: 'hermes' },
  { id: 'pi', label: 'Pi', icon: 'pi' },
  { id: 'others', label: '其他', icon: 'others' },
] as const satisfies readonly HomeProviderTabOption[]

export const DEFAULT_HOME_PROVIDER_TABS = ['claude', 'codex', 'gemini'] as const satisfies readonly HomeProviderTab[]

export const HOME_PROVIDER_TAB_IDS = HOME_PROVIDER_TAB_OPTIONS.map((tab) => tab.id) as HomeProviderTab[]

const HOME_PROVIDER_TAB_OPTION_BY_ID = new Map<HomeProviderTab, HomeProviderTabOption>(
  HOME_PROVIDER_TAB_OPTIONS.map((tab): [HomeProviderTab, HomeProviderTabOption] => [tab.id, { ...tab }]),
)

export function resolveHomeProviderTabOptions(tabs: readonly HomeProviderTab[]): HomeProviderTabOption[] {
  return tabs.flatMap((tabId) => {
    const option = HOME_PROVIDER_TAB_OPTION_BY_ID.get(tabId)
    return option ? [option] : []
  })
}

export function normalizeHomeProviderTabs(value: unknown): HomeProviderTab[] {
  if (!Array.isArray(value)) return [...DEFAULT_HOME_PROVIDER_TABS]

  const validTabIds = new Set<HomeProviderTab>(HOME_PROVIDER_TAB_IDS)
  const normalizedTabs = value.filter((tab): tab is HomeProviderTab => (
    typeof tab === 'string' && validTabIds.has(tab as HomeProviderTab)
  ))

  return normalizedTabs.length > 0
    ? [...new Set(normalizedTabs)]
    : [...DEFAULT_HOME_PROVIDER_TABS]
}

export function setHomeProviderTabVisibility(
  tabs: readonly HomeProviderTab[],
  tabId: HomeProviderTab,
  isVisible: boolean,
): HomeProviderTab[] {
  const normalizedTabs = normalizeHomeProviderTabs(tabs)
  if (isVisible) {
    return normalizedTabs.includes(tabId) ? normalizedTabs : [...normalizedTabs, tabId]
  }
  if (normalizedTabs.length <= 1 || !normalizedTabs.includes(tabId)) return normalizedTabs
  return normalizedTabs.filter((currentTabId) => currentTabId !== tabId)
}

export function reorderHomeProviderTabs(
  tabs: readonly HomeProviderTab[],
  sourceTabId: HomeProviderTab,
  targetTabId: HomeProviderTab,
): HomeProviderTab[] {
  const nextTabs = normalizeHomeProviderTabs(tabs)
  const sourceIndex = nextTabs.indexOf(sourceTabId)
  const targetIndex = nextTabs.indexOf(targetTabId)
  if (sourceIndex < 0 || targetIndex < 0 || sourceIndex === targetIndex) return nextTabs

  nextTabs.splice(sourceIndex, 1)
  nextTabs.splice(targetIndex, 0, sourceTabId)
  return nextTabs
}

export function moveHomeProviderTab(
  tabs: readonly HomeProviderTab[],
  tabId: HomeProviderTab,
  offset: number,
): HomeProviderTab[] {
  const nextTabs = normalizeHomeProviderTabs(tabs)
  const sourceIndex = nextTabs.indexOf(tabId)
  if (sourceIndex < 0 || offset === 0) return nextTabs

  const targetIndex = Math.min(Math.max(sourceIndex + offset, 0), nextTabs.length - 1)
  if (targetIndex === sourceIndex) return nextTabs

  nextTabs.splice(sourceIndex, 1)
  nextTabs.splice(targetIndex, 0, tabId)
  return nextTabs
}

export function resolveHomeProviderTabSelectionIndex(
  previousTabs: readonly HomeProviderTab[],
  nextTabs: readonly HomeProviderTab[],
  selectedIndex: number,
): number {
  if (nextTabs.length === 0) return 0

  const normalizedSelectedIndex = Number.isInteger(selectedIndex) && selectedIndex >= 0 ? selectedIndex : 0
  const previousActiveTab = previousTabs[normalizedSelectedIndex]
  if (previousActiveTab) {
    const nextIndex = nextTabs.indexOf(previousActiveTab)
    return nextIndex >= 0 ? nextIndex : 0
  }
  return normalizedSelectedIndex < nextTabs.length ? normalizedSelectedIndex : 0
}
