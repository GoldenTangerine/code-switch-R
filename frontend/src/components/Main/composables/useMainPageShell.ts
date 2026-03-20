import { computed, onMounted, onUnmounted, reactive, ref, watch, type ComputedRef, type Ref } from 'vue'
import { fetchProxyStatus, enableProxy, disableProxy } from '../../../services/claudeSettings'
import { fetchGeminiProxyStatus, enableGeminiProxy, disableGeminiProxy } from '../../../services/geminiSettings'
import {
  fetchAppSettings,
  normalizeHeatmapGranularity,
  type AppSettings,
  type HeatmapGranularity,
} from '../../../services/appSettings'
import {
  areHeatmapDisplaySettingsEqual,
  DEFAULT_HEATMAP_DISPLAY_SETTINGS,
  normalizeHeatmapDisplaySettings,
  type HeatmapDisplaySettings,
} from '../../../data/heatmapDisplaySettings'
import {
  fetchConfigImportStatus,
  importFromCcSwitch,
  isFirstRun,
  markFirstRunDone,
  type ConfigImportStatus,
} from '../../../services/configImport'
import {
  getCurrentTheme,
  getResolvedTheme,
  onThemeChange,
  setTheme,
  type ThemeMode,
} from '../../../utils/ThemeManager'
import { showToast } from '../../../utils/toast'
import { disableCustomCliProxy, enableCustomCliProxy, type CustomCliTool } from '../../../services/customCliService'
import { PROVIDER_TAB_IDS } from '../constants'
import type { ProviderTab, ResolvedTheme, TranslateFn } from '../types'

type UseMainPageShellOptions = {
  t: TranslateFn
  activeTab: ComputedRef<ProviderTab>
  selectedToolId: Ref<string | null>
  customCliTools: Ref<CustomCliTool[]>
  customCliProxyStates: Record<string, boolean>
  loadProviders: () => Promise<void>
  refreshProviderPricingCachesOnStartup: () => Promise<void>
  refreshDirectAppliedStatus: (tab: ProviderTab) => Promise<void>
  loadProviderStats: (tab: ProviderTab) => Promise<void>
  loadAllProviderStats: () => Promise<void>
  loadBlacklistStatus: (tab: ProviderTab) => Promise<void>
  loadAvailabilityResults: () => Promise<void>
  pollUpdateState: () => Promise<void>
  checkForUpdates: (force?: boolean) => Promise<void>
  startUpdateTimer: () => void
  stopUpdateTimer: () => void
  startProviderStatsTimer: () => void
  stopProviderStatsTimer: () => void
  startStatusSync: () => void
  stopStatusSync: () => void
  loadLastUsedProviders: () => Promise<void>
  reloadHeatmap: () => Promise<void>
  navigateToSettings: () => void
}

export function useMainPageShell(options: UseMainPageShellOptions) {
  const {
    t,
    activeTab,
    selectedToolId,
    customCliTools,
    customCliProxyStates,
    loadProviders,
    refreshProviderPricingCachesOnStartup,
    refreshDirectAppliedStatus,
    loadProviderStats,
    loadAllProviderStats,
    loadBlacklistStatus,
    loadAvailabilityResults,
    pollUpdateState,
    checkForUpdates,
    startUpdateTimer,
    stopUpdateTimer,
    startProviderStatsTimer,
    stopProviderStatsTimer,
    startStatusSync,
    stopStatusSync,
    loadLastUsedProviders,
    reloadHeatmap,
    navigateToSettings,
  } = options

  const themeMode = ref<ThemeMode>(getCurrentTheme())
  const resolvedTheme = ref<ResolvedTheme>(getResolvedTheme(themeMode.value))
  const themeIcon = computed<'sun' | 'moon'>(() => (resolvedTheme.value === 'dark' ? 'moon' : 'sun'))
  const syncThemeState = () => {
    themeMode.value = getCurrentTheme()
    resolvedTheme.value = getResolvedTheme(themeMode.value)
  }

  const proxyStates = reactive<Record<ProviderTab, boolean>>({
    claude: false,
    codex: false,
    gemini: false,
    others: false,
  })
  const proxyBusy = reactive<Record<ProviderTab, boolean>>({
    claude: false,
    codex: false,
    gemini: false,
    others: false,
  })
  const activeProxyState = computed(() => proxyStates[activeTab.value])
  const activeProxyBusy = computed(() => proxyBusy[activeTab.value])

  const syncOthersProxyState = (enabled: boolean) => {
    proxyStates.others = enabled
  }

  const heatmapGranularity = ref<HeatmapGranularity>('hourly')
  const heatmapDisplaySettings = ref<HeatmapDisplaySettings>({
    ...DEFAULT_HEATMAP_DISPLAY_SETTINGS,
  })
  const showHeatmap = ref(true)
  const showHomeTitle = ref(true)
  const showFirstRunPrompt = ref(false)
  const importStatus = ref<ConfigImportStatus | null>(null)
  const importBusy = ref(false)
  const refreshing = ref(false)

  const showImportButton = computed(() => {
    const status = importStatus.value
    if (!status) return false
    return status.config_exists && (status.pending_providers || status.pending_mcp)
  })

  const importButtonTooltip = computed(() => {
    if (!showImportButton.value || !importStatus.value) {
      return t('components.main.controls.import')
    }
    return t('components.main.importConfig.tooltip', {
      providers: importStatus.value.pending_provider_count,
      servers: importStatus.value.pending_mcp_count,
    })
  })

  const loadAppSettings = async () => {
    try {
      const data: AppSettings = await fetchAppSettings()
      showHeatmap.value = data?.show_heatmap ?? true

      const nextGranularity = normalizeHeatmapGranularity(data?.heatmap_granularity)
      if (heatmapGranularity.value !== nextGranularity) {
        heatmapGranularity.value = nextGranularity
      }

      const nextDisplaySettings = normalizeHeatmapDisplaySettings({
        dailyScaleFactor: data?.heatmap_daily_scale_factor,
        dailyIntensityMode: data?.heatmap_daily_intensity_mode,
        intensityMetric: data?.heatmap_intensity_metric,
        intensityStopL1: data?.heatmap_intensity_stop_l1,
        intensityStopL2: data?.heatmap_intensity_stop_l2,
        intensityStopL3: data?.heatmap_intensity_stop_l3,
      })
      if (!areHeatmapDisplaySettingsEqual(heatmapDisplaySettings.value, nextDisplaySettings)) {
        heatmapDisplaySettings.value = nextDisplaySettings
      }

      showHomeTitle.value = data?.show_home_title ?? true
    } catch (error) {
      console.error('failed to load app settings', error)
      showHeatmap.value = true
      if (heatmapGranularity.value !== 'hourly') {
        heatmapGranularity.value = 'hourly'
      }
      if (!areHeatmapDisplaySettingsEqual(heatmapDisplaySettings.value, DEFAULT_HEATMAP_DISPLAY_SETTINGS)) {
        heatmapDisplaySettings.value = { ...DEFAULT_HEATMAP_DISPLAY_SETTINGS }
      }
      showHomeTitle.value = true
      showToast(t('components.main.errors.loadAppSettingsFailed'), 'warning')
    }
  }

  const refreshImportStatus = async () => {
    try {
      importStatus.value = await fetchConfigImportStatus()
    } catch (error) {
      console.error('Failed to load cc-switch import status', error)
      importStatus.value = null
    }
  }

  const checkFirstRun = async () => {
    try {
      const firstRun = await isFirstRun()
      if (firstRun) {
        showFirstRunPrompt.value = true
      }
    } catch (error) {
      console.error('Failed to check first run', error)
    }
  }

  const dismissFirstRunPrompt = async () => {
    showFirstRunPrompt.value = false
    try {
      await markFirstRunDone()
    } catch (error) {
      console.error('Failed to mark first run done', error)
    }
  }

  const goToImportSettings = async () => {
    await dismissFirstRunPrompt()
    navigateToSettings()
  }

  const refreshProxyState = async (tab: ProviderTab) => {
    try {
      if (tab === 'others') {
        if (selectedToolId.value) {
          proxyStates[tab] = customCliProxyStates[selectedToolId.value] ?? false
        } else {
          proxyStates[tab] = false
        }
      } else if (tab === 'gemini') {
        const status = await fetchGeminiProxyStatus()
        proxyStates[tab] = Boolean(status?.enabled)
      } else {
        const status = await fetchProxyStatus(tab as 'claude' | 'codex')
        proxyStates[tab] = Boolean(status?.enabled)
      }
    } catch (error) {
      console.error(`Failed to fetch proxy status for ${tab}`, error)
      proxyStates[tab] = false
    }
  }

  const onProxyToggle = async () => {
    const tab = activeTab.value
    if (proxyBusy[tab]) return

    proxyBusy[tab] = true
    const nextState = !proxyStates[tab]

    try {
      if (tab === 'others') {
        if (!selectedToolId.value) {
          showToast(t('components.main.customCli.selectToolFirst'), 'error')
          return
        }
        if (nextState) {
          await enableCustomCliProxy(selectedToolId.value)
        } else {
          await disableCustomCliProxy(selectedToolId.value)
        }
        customCliProxyStates[selectedToolId.value] = nextState
      } else if (tab === 'gemini') {
        if (nextState) {
          await enableGeminiProxy()
        } else {
          await disableGeminiProxy()
        }
      } else {
        if (nextState) {
          await enableProxy(tab as 'claude' | 'codex')
        } else {
          await disableProxy(tab as 'claude' | 'codex')
        }
      }
      proxyStates[tab] = nextState
    } catch (error) {
      console.error(`Failed to toggle proxy for ${tab}`, error)
    } finally {
      proxyBusy[tab] = false
    }
  }

  const refreshAllData = async () => {
    if (refreshing.value) return

    refreshing.value = true
    try {
      await Promise.all([
        reloadHeatmap(),
        loadProviders(),
        ...PROVIDER_TAB_IDS.map(refreshProxyState),
        ...PROVIDER_TAB_IDS.map((tab) => refreshDirectAppliedStatus(tab)),
        loadAllProviderStats(),
        ...PROVIDER_TAB_IDS.map((tab) => loadBlacklistStatus(tab)),
        loadAvailabilityResults(),
        refreshImportStatus(),
        pollUpdateState(),
      ])
    } catch (error) {
      console.error('Failed to refresh data', error)
    } finally {
      refreshing.value = false
    }
  }

  const currentProxyLabel = computed(() => {
    const tab = activeTab.value
    if (tab === 'claude') {
      return t('components.main.relayToggle.hostClaude')
    }
    if (tab === 'codex') {
      return t('components.main.relayToggle.hostCodex')
    }
    if (tab === 'gemini') {
      return t('components.main.relayToggle.hostGemini')
    }
    const tool = customCliTools.value.find((item) => item.id === selectedToolId.value)
    return tool?.name || t('components.main.relayToggle.hostOthers')
  })

  const goToSettings = () => {
    navigateToSettings()
  }

  const toggleTheme = () => {
    const nextTheme = resolvedTheme.value === 'dark' ? 'light' : 'dark'
    setTheme(nextTheme)
  }

  const handleTabActivated = (tab: ProviderTab) => {
    void refreshProxyState(tab)
    void refreshDirectAppliedStatus(tab)
    void loadProviderStats(tab)
  }

  const handleImportClick = async () => {
    if (importBusy.value) return

    importBusy.value = true
    try {
      const result = await importFromCcSwitch()
      importStatus.value = result?.status ?? null
      const importedProviders = result?.imported_providers ?? 0
      const importedMcp = result?.imported_mcp ?? 0
      if (importedProviders > 0) {
        await loadProviders()
      }
      if (importedProviders > 0 || importedMcp > 0) {
        showToast(
          t('components.main.importConfig.success', {
            providers: importedProviders,
            servers: importedMcp,
          }),
        )
      } else if (result?.status?.config_exists) {
        showToast(t('components.main.importConfig.empty'))
      }
    } catch (error) {
      console.error('Failed to import cc-switch config', error)
      showToast(t('components.main.importConfig.error'), 'error')
    } finally {
      importBusy.value = false
    }
  }

  const handleAppSettingsUpdated = () => {
    void loadAppSettings()
    void pollUpdateState()
  }

  let handleProvidersUpdated: (() => void) | undefined
  let cleanupThemeListener: (() => void) | undefined

  watch(activeTab, (newTab) => {
    void loadBlacklistStatus(newTab)
  })

  onMounted(async () => {
    syncThemeState()
    await loadAppSettings()
    await loadProviders()
    void refreshProviderPricingCachesOnStartup()
    await Promise.all(PROVIDER_TAB_IDS.map(refreshProxyState))
    await Promise.all(PROVIDER_TAB_IDS.map((tab) => refreshDirectAppliedStatus(tab)))
    await loadAllProviderStats()
    await pollUpdateState()
    await checkForUpdates(false)
    await refreshImportStatus()
    await checkFirstRun()
    startProviderStatsTimer()
    startUpdateTimer()
    await Promise.all(PROVIDER_TAB_IDS.map((tab) => loadBlacklistStatus(tab)))
    await loadAvailabilityResults()
    startStatusSync()

    window.addEventListener('app-settings-updated', handleAppSettingsUpdated)
    handleProvidersUpdated = () => {
      void loadProviders()
    }
    window.addEventListener('providers-updated', handleProvidersUpdated)
    cleanupThemeListener = onThemeChange(({ mode, resolvedTheme: nextResolvedTheme }) => {
      themeMode.value = mode
      resolvedTheme.value = nextResolvedTheme
    })

    await loadLastUsedProviders()
  })

  onUnmounted(() => {
    stopProviderStatsTimer()
    stopUpdateTimer()
    stopStatusSync()
    window.removeEventListener('app-settings-updated', handleAppSettingsUpdated)
    if (handleProvidersUpdated) {
      window.removeEventListener('providers-updated', handleProvidersUpdated)
      handleProvidersUpdated = undefined
    }
    cleanupThemeListener?.()
    cleanupThemeListener = undefined
  })

  return {
    resolvedTheme,
    themeIcon,
    toggleTheme,
    showHeatmap,
    showHomeTitle,
    heatmapGranularity,
    heatmapDisplaySettings,
    showFirstRunPrompt,
    dismissFirstRunPrompt,
    goToImportSettings,
    showImportButton,
    importButtonTooltip,
    importBusy,
    handleImportClick,
    activeProxyState,
    activeProxyBusy,
    syncOthersProxyState,
    onProxyToggle,
    refreshing,
    refreshAllData,
    currentProxyLabel,
    goToSettings,
    handleTabActivated,
  }
}
