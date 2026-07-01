<template>
  <div class="main-shell">
    <MainToolbar
      :has-update-available="hasUpdateAvailable"
      :update-ready="updateReady"
      :download-progress="downloadProgress"
      :github-tooltip="githubTooltip"
      :theme-icon="themeIcon"
      :show-import-button="showImportButton"
      :import-button-tooltip="importButtonTooltip"
      :import-busy="importBusy"
      @github-click="handleGithubClick"
      @toggle-theme="toggleTheme"
      @import="handleImportClick"
      @settings="goToSettings"
    />

    <div class="contrib-page">
      <MainHeroBanner
        :show-first-run-prompt="showFirstRunPrompt"
        :show-home-title="showHomeTitle"
        @go-to-import-settings="goToImportSettings"
        @dismiss-first-run="dismissFirstRunPrompt"
      />

      <MainUsageHeatmap
        v-if="showHeatmap"
        ref="heatmapRef"
        :granularity="heatmapGranularity"
        :display-settings="heatmapDisplaySettings"
      />

      <section class="automation-section">
        <MainPlatformTabs
          :tabs="tabs"
          :selected-index="selectedIndex"
          :current-proxy-label="currentProxyLabel"
          :show-proxy-toggle="shouldShowProviderProxyToggle(activeTab)"
          :active-proxy-state="activeProxyState"
          :active-proxy-busy="activeProxyBusy"
          :refreshing="refreshing"
          @change="onTabChange"
          @toggle-proxy="onProxyToggle"
          @create="openCreateModal"
          @refresh="refreshAllData"
        />

        <MainCustomCliToolsBar
          v-if="activeTab === 'others'"
          v-model:selected-tool-id="selectedToolId"
          :custom-cli-tools="customCliTools"
          @change="onToolSelect"
          @create="openCliToolModal"
          @edit="editCurrentCliTool"
          @delete="deleteCurrentCliTool"
        />

        <ProviderCardGrid
          :cards="activeCardViewModels"
          :active-tab="activeTab"
          :active-proxy-state="activeProxyState"
          :resolved-theme="resolvedTheme"
          :is-sorting="draggingId !== null"
          :format-blacklist-countdown="formatBlacklistCountdown"
          :bind-card-ref="bindCardRef"
          @card-click="handleProviderCardClick"
          @dragstart="onDragStart"
          @dragover-card="onDragOverCard"
          @dragleave-list="onDragLeaveList"
          @dragend="onDragEnd"
          @drop="onDrop"
          @open-site="handleOpenSite"
          @unblock-and-reset="handleUnblockAndReset"
          @reset-level="handleResetLevel"
          @toggle-enabled="handleProviderEnabledChange"
          @direct-apply="handleDirectApply"
          @configure="configure"
          @open-provider-data="openProviderDataOverview"
          @open-model-list="openModelList"
          @open-provider-logs="openProviderLogs"
          @open-provider-cost-trend="openProviderCostTrend"
          @refresh-provider-quota="handleRefreshProviderQuota"
          @duplicate="handleDuplicate"
          @remove="requestRemove"
        />

        <CustomCliConfigEditor
          v-if="activeTab === 'others' && selectedToolId && selectedCustomCliTool"
          :tool-id="selectedToolId"
          :tool-name="selectedCustomCliTool.name"
          :config-files="selectedCustomCliTool.configFiles"
          @saved="onConfigFileSaved"
        />
      </section>

      <ProviderModelListModal
        :open="modelListModalOpen"
        :provider="modelListModalProvider"
        :platform="activeTab"
        @close="closeModelListModal"
      />
      <ProviderLogsModal
        :open="providerLogsModalOpen"
        :provider="providerLogsModalProvider"
        :platform="providerLogsModalPlatform"
        :resolved-theme="resolvedTheme"
        @close="closeProviderLogsModal"
        @marked-read="handleProviderLogsMarkedRead"
      />
      <ProviderDataOverviewModal
        :open="providerDataOverviewModalOpen"
        :provider="providerDataOverviewModalProvider"
        :platform="providerDataOverviewModalPlatform"
        :resolved-theme="resolvedTheme"
        @close="closeProviderDataOverviewModal"
      />
      <ProviderCostTrendModal
        :open="providerCostTrendModalOpen"
        :provider="providerCostTrendModalProvider"
        :platform="providerCostTrendModalPlatform"
        :resolved-theme="resolvedTheme"
        @close="closeProviderCostTrendModal"
      />

      <ProviderEditModal
        ref="providerEditModalRef"
        :open="providerModalState.open"
        :tab-id="providerModalState.tabId"
        :card="providerModalState.card"
        :cards="cards[providerModalState.tabId] ?? []"
        :active-proxy-state="activeProxyState"
        @close="closeProviderModal"
        @submit="submitProviderModal"
        @submit-and-apply="submitAndApplyProviderModal"
        @open-provider-quota-query-config="openProviderQuotaQueryConfigModal"
      />
      <ProviderQuotaQueryConfigModal
        :open="providerQuotaQueryConfigModalState.open"
        :model-value="providerQuotaQueryConfigModalState.modelValue"
        :provider-api-url="providerQuotaQueryConfigModalState.providerApiUrl"
        :provider-api-key="providerQuotaQueryConfigModalState.providerApiKey"
        @close="closeProviderQuotaQueryConfigModal"
        @save="handleProviderQuotaQueryConfigModalSave"
      />


      <BaseModal
        :open="confirmState.open"
        :title="t('components.main.form.confirmDeleteTitle')"
        variant="confirm"
        @close="closeConfirm"
      >
        <div class="confirm-body">
          <p>
            {{ t('components.main.form.confirmDeleteMessage', { name: confirmState.card?.name ?? '' }) }}
          </p>
        </div>
        <footer class="form-actions confirm-actions">
          <BaseButton variant="outline" type="button" @click="closeConfirm">
            {{ t('components.main.form.actions.cancel') }}
          </BaseButton>
          <BaseButton variant="danger" type="button" @click="confirmRemove">
            {{ t('components.main.form.actions.delete') }}
          </BaseButton>
        </footer>
      </BaseModal>

      <CliConfigModal
        :open="cliToolModalState.open"
        :tool="cliToolModalState.tool"
        @close="closeCliToolModal"
        @submit="submitCliToolModal"
      />

      <CliDeleteConfirmModal
        :open="cliToolConfirmState.open"
        :tool="cliToolConfirmState.tool"
        @close="closeCliToolConfirm"
        @confirm="confirmDeleteCliTool"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch, type ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'
import { Browser, Call } from '@wailsio/runtime'
import { useRouter } from 'vue-router'
import BaseButton from '../common/BaseButton.vue'
import BaseModal from '../common/BaseModal.vue'
import CustomCliConfigEditor from '../common/CustomCliConfigEditor.vue'
import CliConfigModal from './modals/CliConfigModal.vue'
import CliDeleteConfirmModal from './modals/CliDeleteConfirmModal.vue'
import ProviderEditModal from './modals/ProviderEditModal.vue'
import ProviderLogsModal from './modals/ProviderLogsModal.vue'
import ProviderCostTrendModal from './modals/ProviderCostTrendModal.vue'
import ProviderDataOverviewModal from './modals/ProviderDataOverviewModal.vue'
import ProviderModelListModal from './modals/ProviderModelListModal.vue'
import ProviderQuotaQueryConfigModal from './modals/ProviderQuotaQueryConfigModal.vue'
import MainCustomCliToolsBar from './components/MainCustomCliToolsBar.vue'
import MainHeroBanner from './components/MainHeroBanner.vue'
import MainPlatformTabs from './components/MainPlatformTabs.vue'
import MainToolbar from './components/MainToolbar.vue'
import MainUsageHeatmap from './components/MainUsageHeatmap.vue'
import ProviderCardGrid from './components/ProviderCardGrid.vue'
import { showToast } from '../../utils/toast'
import {
  getProviderDisplayIconSvg,
  preloadProviderDisplayIcons,
} from '../../utils/providerIconAssets'
import { DEFAULT_HOME_PROVIDER_TABS } from '../../data/homeProviderTabs'
import type { CustomCliTool } from '../../services/customCliService'
import { MAIN_TABS } from './constants'
import { useAvailabilityState } from './composables/useAvailabilityState'
import { useBlacklistState } from './composables/useBlacklistState'
import { useCustomCliTools } from './composables/useCustomCliTools'
import { useMainPageShell } from './composables/useMainPageShell'
import { useProviderCards } from './composables/useProviderCards'
import { useProviderForm } from './composables/useProviderForm'
import { useProviderQuotas } from './composables/useProviderQuotas'
import { useProviderStats } from './composables/useProviderStats'
import { useUpdatePolling } from './composables/useUpdatePolling'
import { cardProviderRef, normalizeProviderRef } from './adapters/providerCardMappers'
import { shouldAutoRefreshProviderQuota } from './utils/providerQuotaAutoRefresh'
import { shouldShowProviderProxyToggle } from './utils/providerProxyToggleVisibility'
import { getDefaultHostedProviderRef, isHostedRouteActive } from './utils/providerRoutingState'
import { hasProviderQuotaQueryType } from '../../utils/providerQuotaQuery'
import type { CustomCliToolDraft, ProviderCardViewModel, ProviderConcurrencyStatusView, ProviderTab, VendorForm } from './types'
import type { AutomationCard } from '../../data/cards'

type MainUsageHeatmapExpose = {
  reload: () => Promise<void>
}

type ProviderEditModalExpose = {
  applyProviderQuotaQueryConfig: (nextConfig: VendorForm['providerQuotaQueryConfig']) => void
}

const { t, locale } = useI18n()
const router = useRouter()

const visibleProviderTabs = ref<ProviderTab[]>([...DEFAULT_HOME_PROVIDER_TABS])
const visibleProviderTabSet = computed(() => new Set<ProviderTab>(visibleProviderTabs.value))
const tabs = computed(() => MAIN_TABS.filter((tab) => visibleProviderTabSet.value.has(tab.id)))
const selectedIndex = ref(0)
const activeTab = computed<ProviderTab>(() => tabs.value[selectedIndex.value]?.id ?? tabs.value[0]?.id ?? 'claude')
const heatmapRef = ref<MainUsageHeatmapExpose | null>(null)
const providerEditModalRef = ref<ProviderEditModalExpose | null>(null)
const concurrencyStatuses = ref<ProviderConcurrencyStatusView[]>([])
const {
  hasUpdateAvailable,
  updateReady,
  downloadProgress,
  checkForUpdates,
  pollUpdateState,
  startUpdateTimer,
  stopUpdateTimer,
  handleGithubClick,
  getGithubTooltip,
} = useUpdatePolling(t)
const githubTooltip = computed(() => getGithubTooltip())

let pageShell: ReturnType<typeof useMainPageShell>

const {
  cards,
  draggingId,
  dragOverId,
  normalizeLevel,
  refreshDirectAppliedStatus,
  handleDirectApply,
  isDirectApplied,
  loadCustomCliProviders,
  persistProviders,
  loadProvidersFromDisk,
  importOpenCodeLiveProviders,
  removeProvider,
  duplicateProvider,
  onDragStart,
  onDragOverCard,
  onDragLeaveList,
  onDrop,
  onDragEnd,
  moveCardToStatusGroup,
  appendCardToGroup,
} = useProviderCards({
  t,
  getActiveTab: () => activeTab.value,
  isActiveProxyEnabled: () => pageShell.activeProxyState.value,
  getSelectedToolId: () => selectedToolId.value,
})

const {
  customCliTools,
  selectedToolId,
  customCliProxyStates,
  selectedCustomCliTool,
  loadCustomCliTools,
  onToolSelect,
  saveCliTool,
  deleteCliToolById,
} = useCustomCliTools({
  t,
  setOthersProxyState: (enabled) => {
    pageShell.syncOthersProxyState(enabled)
  },
  loadCustomCliProviders,
  clearOthersCards: () => {
    cards.others.splice(0, cards.others.length)
  },
})

const switchToPlatform = (platform: ProviderTab) => {
  const tabIndex = tabs.value.findIndex((tab) => tab.id === platform)
  if (tabIndex < 0) return
  if (selectedIndex.value !== tabIndex) {
    selectedIndex.value = tabIndex
  }
  // 事件驱动切平台也要补齐 tab 激活副作用，避免页面状态还停留在旧平台。
  handleTabActivated(platform)
}

const {
  loadBlacklistStatus,
  handleUnblockAndReset,
  handleResetLevel,
  formatBlacklistCountdown,
  getProviderBlacklistStatus,
  loadLastUsedProviders,
  isLastUsedProvider,
  isHighlightedCard,
  scrollToCard,
  startStatusSync,
  stopStatusSync,
} = useBlacklistState({
  t,
  getActiveTab: () => activeTab.value,
  getSelectedToolId: () => selectedToolId.value,
  switchToPlatform,
})

const { loadAvailabilityResults, getConnectivityIndicatorClass, getConnectivityTooltip } =
  useAvailabilityState(t, () => activeTab.value)

const {
  loadProviderStats,
  loadAllProviderStats,
  providerStatDisplay,
  refreshProviderPricingCachesOnStartup,
  handleProviderCardClick,
  startProviderStatsTimer,
  stopProviderStatsTimer,
} = useProviderStats({
  t,
  getLocale: () => locale.value,
  getActiveTab: () => activeTab.value,
  cards,
  refreshAvailabilityResults: loadAvailabilityResults,
})

const {
  getQuotaDisplay,
  isQuotaRefreshing,
  refreshProviderQuotas,
  startTimers: startQuotaTimers,
  stopTimers: stopQuotaTimers,
} = useProviderQuotas({
  t,
  getActiveTab: () => activeTab.value,
  cards,
  resolveAutoRefreshRemoteQuotaRefs: () => {
    const refs = new Set<string>()
    const tab = activeTab.value
    const activeTabCards = cards[tab] ?? []

    activeTabCards.forEach((card) => {
      if (!hasProviderQuotaQueryType(card.providerQuotaQueryConfig ?? card.providerQuotaQueryType, card.providerQuotaQueryType)) {
        return
      }

      const isCurrentlyActive = pageShell.activeProxyState.value
        ? isHostedRouteActive({
            activeProxyState: true,
            isLastUsed: isLastUsedProvider(card),
            enabled: card.enabled,
            apiUrl: card.apiUrl,
            apiKey: card.apiKey,
            isBlacklisted: getProviderBlacklistStatus(card)?.isBlacklisted === true,
        })
        : isDirectApplied(card)

      if (!shouldAutoRefreshProviderQuota(tab, card, isCurrentlyActive)) return

      const ref = cardProviderRef(card) || card.name
      if (ref) {
        refs.add(ref)
      }
    })

    return refs
  },
})

const resolveManualRefreshRemoteQuotaRefs = () => {
  const refs = new Set<string>()
  const activeTabCards = cards[activeTab.value] ?? []

  activeTabCards.forEach((card) => {
    if (!hasProviderQuotaQueryType(card.providerQuotaQueryConfig ?? card.providerQuotaQueryType, card.providerQuotaQueryType)) return
    const ref = cardProviderRef(card) || card.name
    if (ref) {
      refs.add(ref)
    }
  })

  return refs
}

const {
  modelListModalOpen,
  modelListModalProvider,
  providerLogsModalOpen,
  providerLogsModalProvider,
  providerLogsModalPlatform,
  providerDataOverviewModalOpen,
  providerDataOverviewModalProvider,
  providerDataOverviewModalPlatform,
  providerCostTrendModalOpen,
  providerCostTrendModalProvider,
  providerCostTrendModalPlatform,
  providerModalState,
  confirmState,
  openModelList,
  closeModelListModal,
  openProviderLogs,
  closeProviderLogsModal,
  openProviderDataOverview,
  closeProviderDataOverviewModal,
  openProviderCostTrend,
  closeProviderCostTrendModal,
  openCreateModal,
  closeProviderModal,
  submitProviderModal,
  submitAndApplyProviderModal,
  configure,
  requestRemove,
  closeConfirm,
  confirmRemove,
  handleDuplicate,
  handleProviderEnabledChange,
} = useProviderForm({
  initialTab: activeTab.value,
  t,
  showToast,
  getActiveTab: () => activeTab.value,
  getSelectedToolId: () => selectedToolId.value,
  cards,
  normalizeLevel,
  persistProviders,
  refreshDirectAppliedStatus,
  removeProvider,
  duplicateProvider,
  reloadProviders: () => loadProvidersFromDisk(loadCustomCliTools),
  moveCardToStatusGroup,
  appendCardToGroup,
})

const handleProviderLogsMarkedRead = () => {
  if (!providerLogsModalPlatform.value) return
  void loadProviderStats(providerLogsModalPlatform.value)
}

pageShell = useMainPageShell({
  t,
  activeTab,
  visibleProviderTabs,
  selectedToolId,
  customCliTools,
  customCliProxyStates,
  loadProviders: () => loadProvidersFromDisk(loadCustomCliTools),
  importOpenCodeLiveProviders,
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
  refreshProviderQuotas,
  resolveManualRefreshRemoteQuotaRefs,
  startQuotaTimers,
  stopQuotaTimers,
  startStatusSync,
  stopStatusSync,
  loadLastUsedProviders,
  reloadHeatmap: () => heatmapRef.value?.reload() ?? Promise.resolve(),
  navigateToSettings: () => {
    router.push('/settings')
  },
})

const {
  resolvedTheme,
  themeIcon,
  toggleTheme,
  showHeatmap,
  showHomeTitle,
  enableRoundRobin,
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
  onProxyToggle,
  refreshing,
  refreshAllData,
  currentProxyLabel,
  goToSettings,
  handleTabActivated,
} = pageShell

const cliToolModalState = reactive({
  open: false,
  tool: null as CustomCliTool | null,
})

const cliToolConfirmState = reactive({
  open: false,
  tool: null as CustomCliTool | null,
})

const providerQuotaQueryConfigModalState = reactive({
  open: false,
  modelValue: undefined as VendorForm['providerQuotaQueryConfig'],
  providerApiUrl: '',
  providerApiKey: '',
})

const activeCards = computed(() => cards[activeTab.value] ?? [])

const normalizeUrlWithScheme = (value: string) => {
  if (!value) return ''
  try {
    const url = new URL(value)
    return url.toString()
  } catch {
    return `https://${value}`
  }
}

const openOfficialSite = (site: string) => {
  const target = normalizeUrlWithScheme(site)
  if (!target) return

  Browser.OpenURL(target).catch(() => {
    console.error('failed to open link', target)
  })
}

const handleOpenSite = (card: AutomationCard) => {
  openOfficialSite(card.officialSite)
}

const handleRefreshProviderQuota = (card: AutomationCard) => {
  const ref = cardProviderRef(card) || card.name
  if (!ref) return
  void refreshProviderQuotas({
    targetRefs: new Set([ref]),
    forceRemoteRefs: new Set([ref]),
  })
}

const formatOfficialSite = (site: string) => {
  if (!site) return ''
  try {
    const url = new URL(normalizeUrlWithScheme(site))
    return url.hostname.replace(/^www\./, '')
  } catch {
    return site
  }
}

const iconSvg = (name: string) => {
  return getProviderDisplayIconSvg(name)
}

const vendorInitials = (name: string) => {
  if (!name) return 'AI'
  return name
    .split(/\s+/)
    .filter(Boolean)
    .map((word) => word[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()
}

const activeHostedProviderRef = computed(() => {
  if (!activeProxyState.value) return null

  const hostedCard = activeCards.value.find((card) => {
    const blacklistStatus = getProviderBlacklistStatus(card)
    return isHostedRouteActive({
      activeProxyState: true,
      isLastUsed: isLastUsedProvider(card),
      enabled: card.enabled,
      apiUrl: card.apiUrl,
      apiKey: card.apiKey,
      isBlacklisted: blacklistStatus?.isBlacklisted === true,
    })
  })

  return hostedCard ? cardProviderRef(hostedCard) : null
})

const defaultHostedProviderRef = computed(() => {
  if (!activeProxyState.value || activeHostedProviderRef.value || enableRoundRobin.value) return null

  return getDefaultHostedProviderRef(
    activeCards.value,
    (card) => getProviderBlacklistStatus(card)?.isBlacklisted === true,
  )
})

const concurrencyStatusMap = computed(() => {
  const map = new Map<string, ProviderConcurrencyStatusView>()
  concurrencyStatuses.value.forEach((status) => {
    map.set(`${status.platform}:${normalizeProviderRef(status.providerId)}`, status)
  })
  return map
})

const activeConcurrencyPlatform = computed(() => (
  activeTab.value === 'others' && selectedToolId.value
    ? `custom:${selectedToolId.value}`
    : activeTab.value
))

const loadConcurrencyStatuses = async () => {
  try {
    const result = await Call.ByName('codeswitch/services.ProviderConcurrencyService.GetProviderConcurrencyStatuses', activeConcurrencyPlatform.value)
    concurrencyStatuses.value = Array.isArray(result) ? result as ProviderConcurrencyStatusView[] : []
  } catch (error) {
    console.error('Failed to load provider concurrency statuses', error)
    concurrencyStatuses.value = []
  }
}

const getConcurrencyStatusForCard = (card: AutomationCard): ProviderConcurrencyStatusView | undefined => {
  const providerRef = cardProviderRef(card)
  const platform = activeConcurrencyPlatform.value
  const existing = concurrencyStatusMap.value.get(`${platform}:${providerRef}`)
  if (existing) {
    return existing
  }
  return {
    platform,
    providerId: providerRef,
    providerName: card.name,
    activeRequests: 0,
    limit: card.providerConcurrencyLimit || 0,
  }
}

const activeCardViewModels = computed<ProviderCardViewModel[]>(() =>
  activeCards.value.map((card) => ({
    card,
    dragging: draggingId.value === card.id,
    dragOver: dragOverId.value === card.id && draggingId.value !== card.id,
    isLastUsed: isLastUsedProvider(card),
    isDefaultHostedProvider: defaultHostedProviderRef.value === cardProviderRef(card),
    isHighlighted: isHighlightedCard(card),
    isDirectApplied: isDirectApplied(card),
    blacklistStatus: getProviderBlacklistStatus(card),
    connectivityClass: getConnectivityIndicatorClass(card.id),
    connectivityTooltip: getConnectivityTooltip(card.id),
    stats: providerStatDisplay(card),
    concurrencyStatus: getConcurrencyStatusForCard(card),
    quotaDisplay: getQuotaDisplay(card),
    quotaRefreshing: isQuotaRefreshing(card),
    formattedOfficialSite: formatOfficialSite(card.officialSite),
    iconSvg: iconSvg(card.icon),
    vendorInitials: vendorInitials(card.name),
  })),
)

watch(
  tabs,
  (nextTabs, previousTabs) => {
    if (nextTabs.length === 0) {
      selectedIndex.value = 0
      return
    }

    const previousActiveTab = previousTabs?.[selectedIndex.value]?.id
    if (previousActiveTab) {
      const nextIndex = nextTabs.findIndex((tab) => tab.id === previousActiveTab)
      if (nextIndex >= 0) {
        selectedIndex.value = nextIndex
        return
      }
      selectedIndex.value = 0
      return
    }

    if (selectedIndex.value >= nextTabs.length) {
      selectedIndex.value = 0
    }
  },
  { immediate: true },
)

watch(
  () => activeCards.value.map((card) => card.icon),
  (icons) => {
    void preloadProviderDisplayIcons(icons)
  },
  { immediate: true },
)

watch(selectedToolId, () => {
  void loadLastUsedProviders()
  void loadConcurrencyStatuses()
})

watch(activeTab, () => {
  void loadConcurrencyStatuses()
})

let concurrencyStatusTimer: number | undefined
if (typeof window !== 'undefined') {
  concurrencyStatusTimer = window.setInterval(() => {
    void loadConcurrencyStatuses()
  }, 2000)
}

onUnmounted(() => {
  if (concurrencyStatusTimer !== undefined) {
    window.clearInterval(concurrencyStatusTimer)
    concurrencyStatusTimer = undefined
  }
})

const bindCardRef = (card: AutomationCard) => (element: Element | ComponentPublicInstance | null) => {
  const target = element instanceof HTMLElement ? element : null
  if (isHighlightedCard(card)) {
    scrollToCard(target)
  }
}

const onTabChange = (index: number) => {
  const nextTab = tabs.value[index]?.id
  if (!nextTab) return
  switchToPlatform(nextTab)
}

const onConfigFileSaved = () => {
  console.log('[CustomCliConfigEditor] Config file saved')
}

const openProviderQuotaQueryConfigModal = (payload: {
  modelValue: VendorForm['providerQuotaQueryConfig']
  providerApiUrl: string
  providerApiKey: string
}) => {
  providerQuotaQueryConfigModalState.modelValue = payload.modelValue
    ? { ...payload.modelValue }
    : undefined
  providerQuotaQueryConfigModalState.providerApiUrl = `${payload.providerApiUrl ?? ''}`
  providerQuotaQueryConfigModalState.providerApiKey = `${payload.providerApiKey ?? ''}`
  providerQuotaQueryConfigModalState.open = true
}

const closeProviderQuotaQueryConfigModal = () => {
  providerQuotaQueryConfigModalState.open = false
}

const handleProviderQuotaQueryConfigModalSave = (nextConfig: VendorForm['providerQuotaQueryConfig']) => {
  providerEditModalRef.value?.applyProviderQuotaQueryConfig(nextConfig)
  closeProviderQuotaQueryConfigModal()
}

const openCliToolModal = () => {
  cliToolModalState.tool = null
  cliToolModalState.open = true
}

const editCurrentCliTool = () => {
  if (!selectedCustomCliTool.value) return
  cliToolModalState.tool = selectedCustomCliTool.value
  cliToolModalState.open = true
}

const closeCliToolModal = () => {
  cliToolModalState.open = false
  cliToolModalState.tool = null
}

const submitCliToolModal = async (draft: CustomCliToolDraft) => {
  const saved = await saveCliTool(draft, cliToolModalState.tool?.id ?? null)
  if (saved) {
    closeCliToolModal()
  }
}

const deleteCurrentCliTool = () => {
  if (!selectedCustomCliTool.value) return
  cliToolConfirmState.tool = selectedCustomCliTool.value
  cliToolConfirmState.open = true
}

const closeCliToolConfirm = () => {
  cliToolConfirmState.open = false
  cliToolConfirmState.tool = null
}

const confirmDeleteCliTool = async () => {
  if (!cliToolConfirmState.tool) return
  const deleted = await deleteCliToolById(cliToolConfirmState.tool.id)
  if (deleted) {
    closeCliToolConfirm()
  }
}

watch(() => providerModalState.open, (open) => {
  if (!open) {
    closeProviderQuotaQueryConfigModal()
  }
})
</script>
