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
          :format-blacklist-countdown="formatBlacklistCountdown"
          :bind-card-ref="bindCardRef"
          @card-click="handleProviderCardClick"
          @dragstart="onDragStart"
          @dragend="onDragEnd"
          @drop="onDrop"
          @open-site="handleOpenSite"
          @unblock-and-reset="handleUnblockAndReset"
          @reset-level="handleResetLevel"
          @toggle-enabled="handleProviderEnabledChange"
          @direct-apply="handleDirectApply"
          @configure="configure"
          @open-model-list="openModelList"
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

      <ProviderEditModal
        :open="providerModalState.open"
        :tab-id="providerModalState.tabId"
        :card="providerModalState.card"
        :active-proxy-state="activeProxyState"
        @close="closeProviderModal"
        @submit="submitProviderModal"
        @submit-and-apply="submitAndApplyProviderModal"
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
import { computed, reactive, ref, watch, type ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'
import { Browser } from '@wailsio/runtime'
import { useRouter } from 'vue-router'
import lobeIcons, { preloadLobeIcons } from '../../icons/lobeIconMap'
import BaseButton from '../common/BaseButton.vue'
import BaseModal from '../common/BaseModal.vue'
import CustomCliConfigEditor from '../common/CustomCliConfigEditor.vue'
import CliConfigModal from './modals/CliConfigModal.vue'
import CliDeleteConfirmModal from './modals/CliDeleteConfirmModal.vue'
import ProviderEditModal from './modals/ProviderEditModal.vue'
import ProviderModelListModal from './modals/ProviderModelListModal.vue'
import MainCustomCliToolsBar from './components/MainCustomCliToolsBar.vue'
import MainHeroBanner from './components/MainHeroBanner.vue'
import MainPlatformTabs from './components/MainPlatformTabs.vue'
import MainToolbar from './components/MainToolbar.vue'
import MainUsageHeatmap from './components/MainUsageHeatmap.vue'
import ProviderCardGrid from './components/ProviderCardGrid.vue'
import { showToast } from '../../utils/toast'
import type { CustomCliTool } from '../../services/customCliService'
import { MAIN_TABS } from './constants'
import { useAvailabilityState } from './composables/useAvailabilityState'
import { useBlacklistState } from './composables/useBlacklistState'
import { useCustomCliTools } from './composables/useCustomCliTools'
import { useMainPageShell } from './composables/useMainPageShell'
import { useProviderCards } from './composables/useProviderCards'
import { useProviderForm } from './composables/useProviderForm'
import { useProviderStats } from './composables/useProviderStats'
import { useUpdatePolling } from './composables/useUpdatePolling'
import type { CustomCliToolDraft, ProviderCardViewModel, ProviderTab } from './types'
import type { AutomationCard } from '../../data/cards'

type MainUsageHeatmapExpose = {
  reload: () => Promise<void>
}

const { t, locale } = useI18n()
const router = useRouter()

const tabs = MAIN_TABS
const selectedIndex = ref(0)
const activeTab = computed<ProviderTab>(() => tabs[selectedIndex.value]?.id ?? tabs[0].id)
const heatmapRef = ref<MainUsageHeatmapExpose | null>(null)

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
  normalizeLevel,
  sortProvidersByLevel,
  refreshDirectAppliedStatus,
  handleDirectApply,
  isDirectApplied,
  loadCustomCliProviders,
  persistProviders,
  loadProvidersFromDisk,
  removeProvider,
  duplicateProvider,
  onDragStart,
  onDrop,
  onDragEnd,
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
  const tabIndex = tabs.findIndex((tab) => tab.id === platform)
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
  modelListModalOpen,
  modelListModalProvider,
  providerModalState,
  confirmState,
  openModelList,
  closeModelListModal,
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
  initialTab: tabs[0].id,
  t,
  showToast,
  getActiveTab: () => activeTab.value,
  cards,
  normalizeLevel,
  sortProvidersByLevel,
  persistProviders,
  refreshDirectAppliedStatus,
  removeProvider,
  duplicateProvider,
  reloadProviders: () => loadProvidersFromDisk(loadCustomCliTools),
})

pageShell = useMainPageShell({
  t,
  activeTab,
  selectedToolId,
  customCliTools,
  customCliProxyStates,
  loadProviders: () => loadProvidersFromDisk(loadCustomCliTools),
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
  if (!name) return ''
  return lobeIcons[name.toLowerCase()] ?? ''
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

const activeCardViewModels = computed<ProviderCardViewModel[]>(() =>
  activeCards.value.map((card) => ({
    card,
    dragging: draggingId.value === card.id,
    isLastUsed: isLastUsedProvider(card),
    isHighlighted: isHighlightedCard(card),
    isDirectApplied: isDirectApplied(card),
    blacklistStatus: getProviderBlacklistStatus(card),
    connectivityClass: getConnectivityIndicatorClass(card.id),
    connectivityTooltip: getConnectivityTooltip(card.id),
    stats: providerStatDisplay(card),
    formattedOfficialSite: formatOfficialSite(card.officialSite),
    iconSvg: iconSvg(card.icon),
    vendorInitials: vendorInitials(card.name),
  })),
)

watch(
  () => activeCards.value.map((card) => card.icon),
  (icons) => {
    void preloadLobeIcons(icons)
  },
  { immediate: true },
)

const bindCardRef = (card: AutomationCard) => (element: Element | ComponentPublicInstance | null) => {
  const target = element instanceof HTMLElement ? element : null
  if (isHighlightedCard(card)) {
    scrollToCard(target)
  }
}

const onTabChange = (index: number) => {
  const nextTab = tabs[index]?.id
  if (!nextTab) return
  switchToPlatform(nextTab)
}

const onConfigFileSaved = () => {
  console.log('[CustomCliConfigEditor] Config file saved')
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
</script>
