import { computed, onActivated, onDeactivated, onMounted, onUnmounted, reactive, ref, watch, type ComputedRef, type Ref } from 'vue'
import { Call, Events } from '@wailsio/runtime'
import { fetchProxyStatus, enableProxy, disableProxy } from '../../../services/claudeSettings'
import { fetchGeminiProxyStatus, enableGeminiProxy, disableGeminiProxy } from '../../../services/geminiSettings'
import {
  disableGrokProxy,
  enableGrokProxy,
  fetchGrokProxyStatus,
  importGrokFromLive,
} from '../../../services/grokSettings'
import {
  fetchClaudeDesktopProxyStatus,
  importClaudeDesktopFromLive,
} from '../../../services/claudeDesktopSettings'
import { importOpenClawFromLive } from '../../../services/openClaw'
import { importHermesFromLive } from '../../../services/hermes'
import { importPiFromLive } from '../../../services/pi'
import {
  fetchAppSettings,
  normalizeHeatmapGranularity,
  saveAppSettings,
  type AppSettings,
  type HeatmapGranularity,
} from '../../../services/appSettings'
import {
  areHeatmapDisplaySettingsEqual,
  DEFAULT_HEATMAP_DISPLAY_SETTINGS,
  normalizeHeatmapDisplaySettings,
  type HeatmapDisplaySettings,
} from '../../../data/heatmapDisplaySettings'
import { normalizeHomeProviderTabs } from '../../../data/homeProviderTabs'
import {
  fetchConfigImportStatus,
  importLegacyConfig,
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
import { bindMainPollingVisibility, createMainPollingLifecycle } from './mainPollingLifecycle'

type BrowserWindowWithWailsBridge = Window & {
  chrome?: {
    webview?: {
      postMessage?: (...args: any[]) => void
    }
  }
  webkit?: {
    messageHandlers?: {
      external?: {
        postMessage?: (...args: any[]) => void
      }
    }
  }
}

type UseMainPageShellOptions = {
  t: TranslateFn
  activeTab: ComputedRef<ProviderTab>
  visibleProviderTabs: Ref<ProviderTab[]>
  selectedToolId: Ref<string | null>
  customCliTools: Ref<CustomCliTool[]>
  customCliProxyStates: Record<string, boolean>
  loadProviders: () => Promise<void>
  importOpenCodeLiveProviders: () => Promise<number | null>
  refreshProviderPricingCachesOnStartup: () => Promise<void>
  refreshDirectAppliedStatus: (tab: ProviderTab) => Promise<void>
  loadProviderStats: (tab: ProviderTab) => Promise<void>
  loadAllProviderStats: () => Promise<void>
  loadBlacklistStatus: (tab: ProviderTab) => Promise<void>
  loadAvailabilityResults: () => Promise<void>
  loadConcurrencyStatuses: () => Promise<void>
  startConcurrencyStatusTimer: () => void
  stopConcurrencyStatusTimer: () => void
  pollUpdateState: () => Promise<void>
  checkForUpdates: (force?: boolean) => Promise<void>
  startUpdateTimer: () => void
  stopUpdateTimer: () => void
  startProviderStatsTimer: () => void
  stopProviderStatsTimer: () => void
  refreshProviderQuotas: (options?: {
    forceRemoteRefs?: Set<string>
    autoRefreshRemoteRefs?: Set<string>
    targetRefs?: Set<string>
  }) => Promise<void>
  resolveManualRefreshRemoteQuotaRefs?: () => Set<string>
  startQuotaTimers: () => void
  stopQuotaTimers: () => void
  startStatusSync: () => void
  stopStatusSync: () => void
  loadLastUsedProviders: () => Promise<void>
  reloadHeatmap: () => Promise<void>
  navigateToSettings: () => void
}

const hasDesktopRuntimeBridge = () => {
  if (typeof window === 'undefined') {
    return false
  }
  const browserWindow = window as BrowserWindowWithWailsBridge
  return Boolean(
    browserWindow.chrome?.webview?.postMessage ||
    browserWindow.webkit?.messageHandlers?.external?.postMessage,
  )
}

const shouldUseBrowserPreviewProxyMock = () => (
  import.meta.env.DEV
  && typeof window !== 'undefined'
  && !hasDesktopRuntimeBridge()
)

export function useMainPageShell(options: UseMainPageShellOptions) {
  const {
    t,
    activeTab,
    visibleProviderTabs,
    selectedToolId,
    customCliTools,
    customCliProxyStates,
    loadProviders,
    importOpenCodeLiveProviders,
    refreshProviderPricingCachesOnStartup,
    refreshDirectAppliedStatus,
    loadProviderStats,
    loadAllProviderStats,
    loadBlacklistStatus,
    loadAvailabilityResults,
    loadConcurrencyStatuses,
    startConcurrencyStatusTimer,
    stopConcurrencyStatusTimer,
    pollUpdateState,
    checkForUpdates,
    startUpdateTimer,
    stopUpdateTimer,
    startProviderStatsTimer,
    stopProviderStatsTimer,
    refreshProviderQuotas,
    resolveManualRefreshRemoteQuotaRefs,
    startQuotaTimers,
    stopQuotaTimers,
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
    opencode: false,
    grokbuild: false,
    'claude-desktop': false,
    openclaw: false,
    hermes: false,
    pi: false,
    others: false,
  })
  const proxyBusy = reactive<Record<ProviderTab, boolean>>({
    claude: false,
    codex: false,
    gemini: false,
    opencode: false,
    grokbuild: false,
    'claude-desktop': false,
    openclaw: false,
    hermes: false,
    pi: false,
    others: false,
  })
  const providerConcurrencyLimitStates = reactive<Record<string, boolean>>({})
  const providerConcurrencyLimitBusy = ref(false)
  const sessionAffinityStates = reactive<Record<string, boolean>>({})
  const sessionAffinityBusy = ref(false)
  const activeProxyState = computed(() => (
    activeTab.value === 'others'
      ? selectedToolId.value
        ? Boolean(customCliProxyStates[selectedToolId.value])
        : false
      : proxyStates[activeTab.value]
  ))
  const activeProxyBusy = computed(() => proxyBusy[activeTab.value])
  const resolveProviderConcurrencyLimitKey = (tab: ProviderTab = activeTab.value) => (
    tab === 'others' && selectedToolId.value
      ? `custom:${selectedToolId.value}`
      : tab
  )
  const activeProviderConcurrencyLimitState = computed(() => (
    providerConcurrencyLimitStates[resolveProviderConcurrencyLimitKey()] === true
  ))
  const resolveSessionAffinityKey = (tab: ProviderTab = activeTab.value) => resolveProviderConcurrencyLimitKey(tab)
  const activeSessionAffinityState = computed(() => sessionAffinityStates[resolveSessionAffinityKey()] === true)

  const syncOthersProxyState = (enabled: boolean) => {
    proxyStates.others = enabled
  }

  const heatmapGranularity = ref<HeatmapGranularity>('daily')
  const heatmapDisplaySettings = ref<HeatmapDisplaySettings>({
    ...DEFAULT_HEATMAP_DISPLAY_SETTINGS,
  })
  const showHeatmap = ref(true)
  const showHomeTitle = ref(true)
  const enableRoundRobin = ref(false)
  const claudeModelRoutingEnabled = ref(false)
  const showFirstRunPrompt = ref(false)
  const importStatus = ref<ConfigImportStatus | null>(null)
  const importBusy = ref(false)
  const refreshing = ref(false)

  const showImportButton = computed(() => {
    if (activeTab.value === 'opencode') {
      return true
    }
    if (activeTab.value === 'grokbuild') {
      return true
    }
    if (activeTab.value === 'claude-desktop') {
      return true
    }
    if (activeTab.value === 'openclaw') {
      return true
    }
    if (activeTab.value === 'hermes') {
      return true
    }
    if (activeTab.value === 'pi') {
      return true
    }
    const status = importStatus.value
    if (!status) return false
    return status.config_exists && (status.pending_providers || status.pending_mcp)
  })

  const importButtonTooltip = computed(() => {
    if (activeTab.value === 'opencode') {
      return t('components.main.importConfig.opencodeTooltip')
    }
    if (activeTab.value === 'grokbuild') {
      return t('components.main.importConfig.grokTooltip')
    }
    if (activeTab.value === 'claude-desktop') {
      return t('components.main.importConfig.claudeDesktopTooltip')
    }
    if (activeTab.value === 'openclaw') {
      return t('components.main.importConfig.openclawTooltip')
    }
    if (activeTab.value === 'hermes') {
      return t('components.main.importConfig.hermesTooltip')
    }
    if (activeTab.value === 'pi') {
      return t('components.main.importConfig.piTooltip')
    }
    if (!showImportButton.value || !importStatus.value) {
      return t('components.main.controls.import')
    }
    return t('components.main.importConfig.tooltip', {
      providers: importStatus.value.pending_provider_count,
      servers: importStatus.value.pending_mcp_count,
    })
  })

  let appSettingsRequestSeq = 0

  const loadAppSettings = async () => {
    const requestSeq = ++appSettingsRequestSeq
    try {
      const data: AppSettings = await fetchAppSettings()
      if (requestSeq !== appSettingsRequestSeq) return
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
      visibleProviderTabs.value = normalizeHomeProviderTabs(data?.home_provider_tabs) as ProviderTab[]
      enableRoundRobin.value = data?.enable_round_robin ?? false
      claudeModelRoutingEnabled.value = data?.claude_model_routing_enabled === true
      Object.keys(providerConcurrencyLimitStates).forEach((key) => {
        delete providerConcurrencyLimitStates[key]
      })
      Object.entries(data?.provider_concurrency_limits ?? {}).forEach(([key, enabled]) => {
        providerConcurrencyLimitStates[key] = enabled === true
      })
      Object.keys(sessionAffinityStates).forEach((key) => {
        delete sessionAffinityStates[key]
      })
      Object.entries(data?.session_affinity_enabled ?? {}).forEach(([key, enabled]) => {
        sessionAffinityStates[key] = enabled === true
      })
    } catch (error) {
      if (requestSeq !== appSettingsRequestSeq) return
      console.error('failed to load app settings', error)
      showHeatmap.value = true
      if (heatmapGranularity.value !== 'daily') {
        heatmapGranularity.value = 'daily'
      }
      if (!areHeatmapDisplaySettingsEqual(heatmapDisplaySettings.value, DEFAULT_HEATMAP_DISPLAY_SETTINGS)) {
        heatmapDisplaySettings.value = { ...DEFAULT_HEATMAP_DISPLAY_SETTINGS }
      }
      showHomeTitle.value = true
      visibleProviderTabs.value = normalizeHomeProviderTabs(null) as ProviderTab[]
      enableRoundRobin.value = false
      claudeModelRoutingEnabled.value = false
      Object.keys(providerConcurrencyLimitStates).forEach((key) => {
        delete providerConcurrencyLimitStates[key]
      })
      Object.keys(sessionAffinityStates).forEach((key) => {
        delete sessionAffinityStates[key]
      })
      showToast(t('components.main.errors.loadAppSettingsFailed'), 'warning')
    }
  }

  const refreshImportStatus = async () => {
    try {
      importStatus.value = await fetchConfigImportStatus()
    } catch (error) {
      console.error('Failed to load config import status', error)
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
    if (shouldUseBrowserPreviewProxyMock()) {
      if (tab === 'opencode' || tab === 'openclaw' || tab === 'hermes' || tab === 'pi') {
        proxyStates[tab] = false
      } else if (tab === 'others') {
        proxyStates[tab] = selectedToolId.value
          ? Boolean(customCliProxyStates[selectedToolId.value])
          : false
      }
      return
    }

    try {
      if (tab === 'opencode' || tab === 'openclaw' || tab === 'hermes' || tab === 'pi') {
        // OpenClaw / Hermes / Pi 为 additive 共存模式：供应商直接写入原生配置，无本地代理开关
        proxyStates[tab] = false
      } else if (tab === 'others') {
        if (selectedToolId.value) {
          proxyStates[tab] = customCliProxyStates[selectedToolId.value] ?? false
        } else {
          proxyStates[tab] = false
        }
      } else if (tab === 'gemini') {
        const status = await fetchGeminiProxyStatus()
        proxyStates[tab] = Boolean(status?.enabled)
      } else if (tab === 'grokbuild') {
        const status = await fetchGrokProxyStatus()
        proxyStates[tab] = Boolean(status?.enabled)
      } else if (tab === 'claude-desktop') {
        // Claude Desktop 无独立代理开关：ProxyStatus 反映当前是否处于本地代理模式
        const status = await fetchClaudeDesktopProxyStatus()
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
    if (tab === 'opencode' || tab === 'claude-desktop' || tab === 'openclaw' || tab === 'hermes' || tab === 'pi') {
      proxyStates[tab] = false
      return
    }
    if (proxyBusy[tab]) return

    proxyBusy[tab] = true
    const nextState = !proxyStates[tab]

    try {
      if (shouldUseBrowserPreviewProxyMock()) {
        if (tab === 'others') {
          if (!selectedToolId.value) {
            showToast(t('components.main.customCli.selectToolFirst'), 'error')
            return
          }
          customCliProxyStates[selectedToolId.value] = nextState
        }
        proxyStates[tab] = nextState
        return
      }

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
      } else if (tab === 'grokbuild') {
        if (nextState) {
          await enableGrokProxy()
        } else {
          await disableGrokProxy()
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

  const onProviderConcurrencyLimitToggle = async () => {
    const tab = activeTab.value
    if (tab === 'opencode' || tab === 'openclaw' || tab === 'hermes' || tab === 'pi' || providerConcurrencyLimitBusy.value) return
    if (tab === 'others' && !selectedToolId.value) {
      showToast(t('components.main.customCli.selectToolFirst'), 'error')
      return
    }

    providerConcurrencyLimitBusy.value = true
    const key = resolveProviderConcurrencyLimitKey(tab)
    const nextState = !providerConcurrencyLimitStates[key]
    try {
      const settings = await fetchAppSettings()
      const nextLimits = {
        ...(settings.provider_concurrency_limits ?? {}),
        [key]: nextState,
      }
      await saveAppSettings({
        ...settings,
        provider_concurrency_limits: nextLimits,
      })
      providerConcurrencyLimitStates[key] = nextState
      window.dispatchEvent(new CustomEvent('app-settings-updated'))
    } catch (error) {
      console.error('Failed to toggle provider concurrency limit', error)
      showToast(t('components.main.errors.saveSettingsFailed'), 'error')
    } finally {
      providerConcurrencyLimitBusy.value = false
    }
  }

  const onSessionAffinityToggle = async () => {
    const tab = activeTab.value
    if (tab === 'opencode' || tab === 'grokbuild' || tab === 'claude-desktop' || tab === 'openclaw' || tab === 'hermes' || tab === 'pi' || sessionAffinityBusy.value) return
    if (tab === 'others' && !selectedToolId.value) {
      showToast(t('components.main.customCli.selectToolFirst'), 'error')
      return
    }
    sessionAffinityBusy.value = true
    const key = resolveSessionAffinityKey(tab)
    const nextState = !sessionAffinityStates[key]
    try {
      const settings = await fetchAppSettings()
      const nextMap = {
        ...(settings.session_affinity_enabled ?? {}),
        [key]: nextState,
      }
      await saveAppSettings({ ...settings, session_affinity_enabled: nextMap })
      if (!nextState) {
        await Call.ByName('codeswitch/services.ProviderConcurrencyService.ClearSessionAffinity', key)
      }
      sessionAffinityStates[key] = nextState
      window.dispatchEvent(new CustomEvent('app-settings-updated'))
    } catch (error) {
      console.error('Failed to toggle session affinity', error)
      showToast(t('components.main.errors.saveSettingsFailed'), 'error')
    } finally {
      sessionAffinityBusy.value = false
    }
  }

  const refreshAllData = async () => {
    if (refreshing.value) return

    refreshing.value = true
    try {
      await loadProviders()
      const forceRemoteRefs = resolveManualRefreshRemoteQuotaRefs?.()
      await Promise.all([
        reloadHeatmap(),
        ...PROVIDER_TAB_IDS.map(refreshProxyState),
        ...PROVIDER_TAB_IDS.map((tab) => refreshDirectAppliedStatus(tab)),
        loadAllProviderStats(),
        ...PROVIDER_TAB_IDS.map((tab) => loadBlacklistStatus(tab)),
        loadLastUsedProviders(),
        loadAvailabilityResults(),
        refreshImportStatus(),
        pollUpdateState(),
        refreshProviderQuotas(
          forceRemoteRefs && forceRemoteRefs.size > 0
            ? { forceRemoteRefs }
            : undefined,
        ),
      ])
    } catch (error) {
      console.error('Failed to refresh data', error)
    } finally {
      refreshing.value = false
    }
  }

  const pollingLifecycle = createMainPollingLifecycle({
    initialVisible: typeof document === 'undefined' || !document.hidden,
    startPolling: () => {
      startProviderStatsTimer()
      startQuotaTimers()
      startUpdateTimer()
      startStatusSync()
      startConcurrencyStatusTimer()
    },
    stopPolling: () => {
      stopProviderStatsTimer()
      stopQuotaTimers()
      stopUpdateTimer()
      stopStatusSync()
      stopConcurrencyStatusTimer()
    },
    refresh: async () => {
      void checkForUpdates(false)
      await Promise.all([
        refreshAllData(),
        loadConcurrencyStatuses(),
      ])
    },
  })

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
    if (tab === 'opencode') {
      return 'OpenCode'
    }
    if (tab === 'grokbuild') {
      return t('components.main.relayToggle.hostGrok')
    }
    if (tab === 'claude-desktop') {
      return t('components.main.relayToggle.hostClaudeDesktop')
    }
    if (tab === 'openclaw') {
      return 'OpenClaw'
    }
    if (tab === 'hermes') {
      return 'Hermes'
    }
    if (tab === 'pi') {
      return 'Pi'
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
      if (activeTab.value === 'opencode') {
        const imported = await importOpenCodeLiveProviders()
        if (imported === null) return
        if (imported > 0) {
          showToast(t('components.main.importConfig.opencodeSuccess', { providers: imported }))
        } else {
          showToast(t('components.main.importConfig.opencodeEmpty'))
        }
        return
      }

      if (activeTab.value === 'grokbuild') {
        const imported = await importGrokFromLive()
        if (imported) {
          await loadProviders()
          showToast(t('components.main.importConfig.grokSuccess'))
        } else {
          showToast(t('components.main.importConfig.grokEmpty'))
        }
        return
      }

      if (activeTab.value === 'claude-desktop') {
        const imported = await importClaudeDesktopFromLive()
        if (imported) {
          await loadProviders()
          showToast(t('components.main.importConfig.claudeDesktopSuccess'))
        } else {
          showToast(t('components.main.importConfig.claudeDesktopEmpty'))
        }
        return
      }

      if (activeTab.value === 'openclaw') {
        const imported = await importOpenClawFromLive()
        if (imported) {
          await loadProviders()
          showToast(t('components.main.importConfig.openclawSuccess'))
        } else {
          showToast(t('components.main.importConfig.openclawEmpty'))
        }
        return
      }

      if (activeTab.value === 'hermes') {
        const imported = await importHermesFromLive()
        if (imported) {
          await loadProviders()
          showToast(t('components.main.importConfig.hermesSuccess'))
        } else {
          showToast(t('components.main.importConfig.hermesEmpty'))
        }
        return
      }

      if (activeTab.value === 'pi') {
        const imported = await importPiFromLive()
        if (imported) {
          await loadProviders()
          showToast(t('components.main.importConfig.piSuccess'))
        } else {
          showToast(t('components.main.importConfig.piEmpty'))
        }
        return
      }

      const result = await importLegacyConfig()
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
      console.error('Failed to import config', error)
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
  let unbindPollingVisibility: (() => void) | undefined

  watch(activeTab, (newTab) => {
    void loadBlacklistStatus(newTab)
    void refreshProviderQuotas()
  })

  onActivated(() => {
    pollingLifecycle.setPageActive(true)
  })

  onDeactivated(() => {
    pollingLifecycle.setPageActive(false)
  })

  onMounted(async () => {
    unbindPollingVisibility = bindMainPollingVisibility(pollingLifecycle, {
      onWindowHide: (handler) => Events.On(Events.Types.Common.WindowHide, handler),
      onWindowShow: (handler) => Events.On(Events.Types.Common.WindowShow, handler),
      addVisibilityListener: (handler) => document.addEventListener('visibilitychange', handler),
      removeVisibilityListener: (handler) => document.removeEventListener('visibilitychange', handler),
      isDocumentHidden: () => document.hidden,
    })
    window.addEventListener('app-settings-updated', handleAppSettingsUpdated)
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
    void refreshProviderQuotas()
    await Promise.all(PROVIDER_TAB_IDS.map((tab) => loadBlacklistStatus(tab)))
    await loadAvailabilityResults()
    pollingLifecycle.start()

    handleProvidersUpdated = () => {
      void (async () => {
        try {
          await loadProviders()
          await refreshProviderQuotas()
        } catch (error) {
          console.error('Failed to reload providers after providers-updated event', error)
        }
      })()
    }
    window.addEventListener('providers-updated', handleProvidersUpdated)
    cleanupThemeListener = onThemeChange(({ mode, resolvedTheme: nextResolvedTheme }) => {
      themeMode.value = mode
      resolvedTheme.value = nextResolvedTheme
    })

    await loadLastUsedProviders()
  })

  onUnmounted(() => {
    pollingLifecycle.dispose()
    unbindPollingVisibility?.()
    unbindPollingVisibility = undefined
    window.removeEventListener('app-settings-updated', handleAppSettingsUpdated)
    if (handleProvidersUpdated) {
      window.removeEventListener('providers-updated', handleProvidersUpdated)
      handleProvidersUpdated = undefined
    }
    cleanupThemeListener?.()
    cleanupThemeListener = undefined
  })

  return {
    proxyStates,
    resolvedTheme,
    themeIcon,
    toggleTheme,
    showHeatmap,
    showHomeTitle,
    enableRoundRobin,
    claudeModelRoutingEnabled,
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
    providerConcurrencyLimitStates,
    activeProviderConcurrencyLimitState,
    providerConcurrencyLimitBusy,
    sessionAffinityStates,
    activeSessionAffinityState,
    sessionAffinityBusy,
    syncOthersProxyState,
    onProxyToggle,
    onProviderConcurrencyLimitToggle,
    onSessionAffinityToggle,
    refreshing,
    refreshAllData,
    currentProxyLabel,
    goToSettings,
    handleTabActivated,
  }
}
