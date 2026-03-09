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

      <section
        v-if="showHeatmap"
        ref="heatmapContainerRef"
        class="contrib-wall"
        :aria-label="t('components.main.heatmap.ariaLabel')"
      >
        <div class="contrib-legend">
          <span>{{ t('components.main.heatmap.legendLow') }}</span>
          <span v-for="level in 5" :key="level" :class="['legend-box', intensityClass(level - 1)]" />
          <span>{{ t('components.main.heatmap.legendHigh') }}</span>
        </div>

        <div class="contrib-grid">
          <div
            v-for="(week, weekIndex) in usageHeatmap"
            :key="weekIndex"
            class="contrib-column"
          >
            <div
              v-for="(day, dayIndex) in week"
              :key="dayIndex"
              class="contrib-cell"
              :class="intensityClass(day.intensity)"
              @mouseenter="showUsageTooltip(day, $event)"
              @mousemove="showUsageTooltip(day, $event)"
              @mouseleave="hideUsageTooltip"
            />
          </div>
        </div>

        <div
          v-if="usageTooltip.visible"
          ref="tooltipRef"
          class="contrib-tooltip"
          :class="[usageTooltip.placement, { 'is-positioned': usageTooltip.positioned }]"
          :style="{ left: `${usageTooltip.left}px`, top: `${usageTooltip.top}px` }"
          role="tooltip"
        >
          <p class="tooltip-heading">{{ formattedTooltipLabel }}</p>
          <div class="tooltip-summary-grid">
            <div
              v-for="card in usageTooltipSummaryCards"
              :key="card.key"
              :class="['tooltip-summary-card', `is-${card.tone}`, { 'is-full': card.fullWidth }]"
            >
              <span class="tooltip-summary-label">{{ card.label }}</span>
              <span class="tooltip-summary-value">{{ card.value }}</span>
            </div>
          </div>
          <div class="tooltip-sections">
            <section
              v-for="section in usageTooltipSections"
              :key="section.key"
              class="tooltip-section"
            >
              <div class="tooltip-section-heading" :class="`is-${section.tone}`">
                {{ section.title }}
              </div>
              <div class="tooltip-section-body">
                <div
                  v-for="row in section.rows"
                  :key="row.key"
                  :class="['tooltip-row', `is-${row.tone}`, { 'is-active': row.active }]"
                >
                  <span class="tooltip-row-label">{{ row.label }}</span>
                  <span
                    :class="[
                      'tooltip-row-value',
                      `is-${row.tone}`,
                      { 'is-emphasis': row.emphasis || row.active },
                    ]"
                  >
                    {{ row.value }}
                  </span>
                </div>
              </div>
            </section>
          </div>
        </div>
      </section>

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
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch, type ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'
import { Browser, Call } from '@wailsio/runtime'
import { useRouter } from 'vue-router'
import { type UsageHeatmapDay } from '../../data/usageHeatmap'
import { useAdaptiveHeatmap } from '../../composables/useAdaptiveHeatmap'
import lobeIcons from '../../icons/lobeIconMap'
import BaseButton from '../common/BaseButton.vue'
import BaseModal from '../common/BaseModal.vue'
import CustomCliConfigEditor from '../common/CustomCliConfigEditor.vue'
import ProviderModelListModal from './ProviderModelListModal.vue'
import CliConfigModal from './modals/CliConfigModal.vue'
import CliDeleteConfirmModal from './modals/CliDeleteConfirmModal.vue'
import ProviderEditModal from './modals/ProviderEditModal.vue'
import MainCustomCliToolsBar from './components/MainCustomCliToolsBar.vue'
import MainHeroBanner from './components/MainHeroBanner.vue'
import MainPlatformTabs from './components/MainPlatformTabs.vue'
import MainToolbar from './components/MainToolbar.vue'
import ProviderCardGrid from './components/ProviderCardGrid.vue'
import { fetchProxyStatus, enableProxy, disableProxy } from '../../services/claudeSettings'
import { fetchGeminiProxyStatus, enableGeminiProxy, disableGeminiProxy } from '../../services/geminiSettings'
import { fetchProviderDailyStats, type ProviderDailyStat } from '../../services/logs'
import { fetchProviderModelPricing } from '../../services/providerModelPricing'
import {
  fetchAppSettings,
  normalizeHeatmapGranularity,
  type AppSettings,
  type HeatmapGranularity,
} from '../../services/appSettings'
import {
  areHeatmapDisplaySettingsEqual,
  DEFAULT_HEATMAP_DISPLAY_SETTINGS,
  normalizeHeatmapDisplaySettings,
  type HeatmapDisplaySettings,
  type HeatmapIntensityMetric,
} from '../../data/heatmapDisplaySettings'
import { fetchConfigImportStatus, importFromCcSwitch, isFirstRun, markFirstRunDone, type ConfigImportStatus } from '../../services/configImport'
import { saveCLIConfig, type CLIPlatform } from '../../services/cliConfig'
import { showToast } from '../../utils/toast'
import { getCurrentTheme, setTheme, type ThemeMode } from '../../utils/ThemeManager'
import { disableCustomCliProxy, enableCustomCliProxy, type CustomCliTool } from '../../services/customCliService'
import { PROVIDER_PRICING_CLICK_THROTTLE_MS, PROVIDER_PRICING_STARTUP_CONCURRENCY, SUCCESS_RATE_THRESHOLDS, MAIN_TABS, PROVIDER_TAB_IDS } from './constants'
import { useAvailabilityState } from './composables/useAvailabilityState'
import { useBlacklistState } from './composables/useBlacklistState'
import { useCustomCliTools } from './composables/useCustomCliTools'
import { useProviderCards } from './composables/useProviderCards'
import { useUpdatePolling } from './composables/useUpdatePolling'
import {
  cardProviderRef,
  createGeminiProviderRef,
  normalizeProviderKey,
  normalizeProviderRef,
  providerStatsKeyFromStat,
} from './adapters/providerCardMappers'
import { buildPersistedProviderFieldsFromForm } from './adapters/providerFormMappers'
import type {
  CustomCliToolDraft,
  ProviderCardViewModel,
  ProviderStatDisplay,
  ProviderTab,
  ResolvedTheme,
  VendorForm,
} from './types'
import type { AutomationCard } from '../../data/cards'

const { t, locale } = useI18n()
const router = useRouter()

const themeMode = ref<ThemeMode>(getCurrentTheme())
const resolvedTheme = computed<ResolvedTheme>(() => {
  if (themeMode.value === 'systemdefault') {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }
  return themeMode.value
})
const themeIcon = computed<'sun' | 'moon'>(() => (resolvedTheme.value === 'dark' ? 'moon' : 'sun'))

const tabs = MAIN_TABS
const selectedIndex = ref(0)
const activeTab = computed<ProviderTab>(() => tabs[selectedIndex.value]?.id ?? tabs[0].id)

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

const heatmapContainerRef = ref<HTMLElement | null>(null)
const heatmapGranularity = ref<HeatmapGranularity>('hourly')
const heatmapDisplaySettings = ref<HeatmapDisplaySettings>({
  ...DEFAULT_HEATMAP_DISPLAY_SETTINGS,
})
const {
  displayData: usageHeatmap,
  init: initHeatmap,
  cleanup: cleanupHeatmap,
  reload: reloadHeatmap,
} = useAdaptiveHeatmap(heatmapContainerRef, heatmapGranularity, heatmapDisplaySettings)

const tooltipRef = ref<HTMLElement | null>(null)
const showHeatmap = ref(true)
const showHomeTitle = ref(true)

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
  isActiveProxyEnabled: () => activeProxyState.value,
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
    proxyStates.others = enabled
  },
  loadCustomCliProviders,
  clearOthersCards: () => {
    cards.others.splice(0, cards.others.length)
  },
})

const switchToPlatform = (platform: ProviderTab) => {
  const tabIndex = tabs.findIndex((tab) => tab.id === platform)
  if (tabIndex >= 0 && selectedIndex.value !== tabIndex) {
    selectedIndex.value = tabIndex
  }
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

const providerStatsMap = reactive<Record<ProviderTab, Record<string, ProviderDailyStat>>>({
  claude: {},
  codex: {},
  gemini: {},
  others: {},
})
const providerStatsLoaded = reactive<Record<ProviderTab, boolean>>({
  claude: false,
  codex: false,
  gemini: false,
  others: false,
})
let providerStatsTimer: number | undefined

const modelListModalOpen = ref(false)
const modelListModalProvider = ref<AutomationCard | null>(null)

const importStatus = ref<ConfigImportStatus | null>(null)
const importBusy = ref(false)
const showFirstRunPrompt = ref(false)

const providerModalState = reactive({
  open: false,
  tabId: tabs[0].id as ProviderTab,
  card: null as AutomationCard | null,
})

const confirmState = reactive({
  open: false,
  card: null as AutomationCard | null,
  tabId: tabs[0].id as ProviderTab,
})

const cliToolModalState = reactive({
  open: false,
  tool: null as CustomCliTool | null,
})

const cliToolConfirmState = reactive({
  open: false,
  tool: null as CustomCliTool | null,
})

const activeCards = computed(() => cards[activeTab.value] ?? [])

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

const intensityClass = (value: number) => `gh-level-${value}`

type TooltipPlacement = 'above' | 'below'
type UsageTooltipTone = 'neutral' | 'info' | 'warning' | 'success' | 'violet' | 'rose'
type UsageTooltipSummaryCard = {
  key: string
  label: string
  value: string
  tone: UsageTooltipTone
  fullWidth?: boolean
}
type UsageTooltipRow = {
  key: string
  label: string
  value: string
  tone: UsageTooltipTone
  emphasis?: boolean
  active?: boolean
}
type UsageTooltipSection = {
  key: string
  title: string
  tone: UsageTooltipTone
  rows: UsageTooltipRow[]
}
type UsageTooltipSectionKey = 'activity' | 'tokens' | 'cost'
type UsageTooltipMetricRowDefinition = {
  key: string
  metric: HeatmapIntensityMetric
  section: UsageTooltipSectionKey
  label: string
  emphasis?: boolean
}

const usageTooltip = reactive({
  visible: false,
  positioned: false,
  label: '',
  dateKey: '',
  left: 0,
  top: 0,
  placement: 'above' as TooltipPlacement,
  requests: 0,
  inputTokens: 0,
  outputTokens: 0,
  totalTokens: 0,
  reasoningTokens: 0,
  cost: 0,
  intensity: 0,
  intensityValue: 0,
  intensityPeakValue: 0,
})

const formatMetric = (value: number) => value.toLocaleString()

const formatTokenNumber = (value: number) => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  }
  if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  }
  if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}k`
  }
  return value.toLocaleString()
}

const toPositiveFiniteNumber = (value: unknown) => {
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric) || numeric <= 0) return 0
  return numeric
}

const formatAverageFirstTokenMs = (value: unknown) => {
  const seconds = toPositiveFiniteNumber(value)
  if (seconds <= 0) return '—'
  const milliseconds = seconds * 1000
  const precision = milliseconds >= 100 ? 0 : milliseconds >= 10 ? 1 : 2
  return `${milliseconds.toFixed(precision)} ms`
}

const formatAverageTokensPerSecond = (value: unknown) => {
  const tokensPerSecond = toPositiveFiniteNumber(value)
  if (tokensPerSecond <= 0) return '—'
  const precision = tokensPerSecond >= 100 ? 1 : 2
  return `${tokensPerSecond.toFixed(precision)} tokens/s`
}

const tooltipDateFormatter = computed(() =>
  new Intl.DateTimeFormat(locale.value || 'en', {
    month: 'short',
    day: 'numeric',
    ...(heatmapGranularity.value === 'daily'
      ? {}
      : {
          hour: '2-digit',
          minute: '2-digit',
        }),
  }),
)

const currencyFormatter = computed(() =>
  new Intl.NumberFormat(locale.value || 'en', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }),
)

const formattedTooltipLabel = computed(() => {
  if (!usageTooltip.dateKey) return usageTooltip.label
  const date = new Date(usageTooltip.dateKey)
  if (Number.isNaN(date.getTime())) {
    return usageTooltip.label
  }
  return tooltipDateFormatter.value.format(date)
})

const formatTooltipTokenMetricValue = (value: number) => {
  const normalized = Math.max(0, Math.round(value || 0))
  const compact = formatTokenNumber(normalized)
  if (normalized < 1_000) return compact
  return `${compact} (${normalized.toLocaleString()})`
}

const formatTooltipCostMetricValue = (value: number) => {
  const normalized = Math.max(0, Number(value || 0))
  if (!Number.isFinite(normalized)) return '$0.00'
  const maximumFractionDigits =
    normalized >= 100 ? 2 : normalized >= 1 ? 4 : normalized >= 0.01 ? 4 : 6
  return new Intl.NumberFormat(locale.value || 'en', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits,
  }).format(normalized)
}

const formatTooltipPercentValue = (value: number) => {
  const normalized = Number.isFinite(value) ? Math.min(Math.max(value, 0), 1) : 0
  const percentValue = normalized * 100
  const maximumFractionDigits =
    percentValue > 0 && percentValue < 10 ? 1 : Number.isInteger(percentValue) ? 0 : 1
  return new Intl.NumberFormat(locale.value || 'en', {
    style: 'percent',
    minimumFractionDigits: 0,
    maximumFractionDigits,
  }).format(normalized)
}

const getHeatmapMetricLabel = (metric: HeatmapIntensityMetric) => {
  switch (metric) {
    case 'cost':
      return t('components.general.heatmapDisplay.intensityMetricCost')
    case 'total_tokens':
      return t('components.general.heatmapDisplay.intensityMetricTotalTokens')
    case 'input_tokens':
      return t('components.general.heatmapDisplay.intensityMetricInputTokens')
    case 'output_tokens':
      return t('components.general.heatmapDisplay.intensityMetricOutputTokens')
    case 'reasoning_tokens':
      return t('components.general.heatmapDisplay.intensityMetricReasoningTokens')
    case 'requests':
    default:
      return t('components.general.heatmapDisplay.intensityMetricRequests')
  }
}

const getUsageTooltipMetricRawValue = (metric: HeatmapIntensityMetric) => {
  switch (metric) {
    case 'cost':
      return usageTooltip.cost
    case 'total_tokens':
      return usageTooltip.totalTokens
    case 'input_tokens':
      return usageTooltip.inputTokens
    case 'output_tokens':
      return usageTooltip.outputTokens
    case 'reasoning_tokens':
      return usageTooltip.reasoningTokens
    case 'requests':
    default:
      return usageTooltip.requests
  }
}

const formatHeatmapMetricValue = (metric: HeatmapIntensityMetric, value: number) => {
  switch (metric) {
    case 'cost':
      return formatTooltipCostMetricValue(value)
    case 'total_tokens':
    case 'input_tokens':
    case 'output_tokens':
    case 'reasoning_tokens':
      return formatTooltipTokenMetricValue(value)
    case 'requests':
    default:
      return formatMetric(Math.max(0, Math.round(value || 0)))
  }
}

const heatmapIntensityMetric = computed<HeatmapIntensityMetric>(() => heatmapDisplaySettings.value.intensityMetric)
const heatmapIntensityMetricLabel = computed(() => getHeatmapMetricLabel(heatmapIntensityMetric.value))
const heatmapIntensityMetricValue = computed(() =>
  formatHeatmapMetricValue(
    heatmapIntensityMetric.value,
    getUsageTooltipMetricRawValue(heatmapIntensityMetric.value),
  ),
)
const heatmapIntensityLevelRatioValue = computed(() => {
  const normalizedLevel = Math.max(0, Math.round(usageTooltip.intensity || 0))
  const peakRatio =
    usageTooltip.intensityPeakValue > 0
      ? usageTooltip.intensityValue / usageTooltip.intensityPeakValue
      : 0
  return t('components.main.heatmap.metrics.brightnessLevelRatioValue', {
    level: normalizedLevel,
    ratio: formatTooltipPercentValue(peakRatio),
  })
})

const HEATMAP_TOOLTIP_SECTION_ORDER: UsageTooltipSectionKey[] = ['activity', 'tokens', 'cost']

const getHeatmapMetricTone = (metric: HeatmapIntensityMetric): UsageTooltipTone => {
  switch (metric) {
    case 'cost':
      return 'success'
    case 'total_tokens':
      return 'warning'
    case 'input_tokens':
      return 'info'
    case 'output_tokens':
      return 'violet'
    case 'reasoning_tokens':
      return 'rose'
    case 'requests':
    default:
      return 'info'
  }
}

const getHeatmapIntensityTone = (intensity: number): UsageTooltipTone => {
  if (intensity >= 4) return 'success'
  if (intensity >= 2) return 'warning'
  if (intensity >= 1) return 'info'
  return 'neutral'
}

const isActiveHeatmapMetric = (metric: HeatmapIntensityMetric) => heatmapIntensityMetric.value === metric

const usageTooltipSectionMeta = computed<Record<UsageTooltipSectionKey, { title: string; tone: UsageTooltipTone }>>(() => ({
  activity: {
    title: t('components.main.heatmap.sections.activity'),
    tone: 'info',
  },
  tokens: {
    title: t('components.main.heatmap.sections.tokens'),
    tone: 'warning',
  },
  cost: {
    title: t('components.main.heatmap.sections.cost'),
    tone: 'success',
  },
}))

const usageTooltipMetricRowDefinitions = computed<UsageTooltipMetricRowDefinition[]>(() => [
  {
    key: 'requests',
    metric: 'requests',
    section: 'activity',
    label: t('components.main.heatmap.metrics.requests'),
    emphasis: true,
  },
  {
    key: 'totalTokens',
    metric: 'total_tokens',
    section: 'tokens',
    label: t('components.main.heatmap.metrics.totalTokens'),
    emphasis: true,
  },
  {
    key: 'inputTokens',
    metric: 'input_tokens',
    section: 'tokens',
    label: t('components.main.heatmap.metrics.inputTokens'),
  },
  {
    key: 'outputTokens',
    metric: 'output_tokens',
    section: 'tokens',
    label: t('components.main.heatmap.metrics.outputTokens'),
  },
  {
    key: 'reasoningTokens',
    metric: 'reasoning_tokens',
    section: 'tokens',
    label: t('components.main.heatmap.metrics.reasoningTokens'),
  },
  {
    key: 'cost',
    metric: 'cost',
    section: 'cost',
    label: t('components.main.heatmap.metrics.cost'),
    emphasis: true,
  },
])

const usageTooltipSummaryCards = computed<UsageTooltipSummaryCard[]>(() => [
  {
    key: 'brightnessMetric',
    label: t('components.main.heatmap.metrics.brightnessMetric'),
    value: heatmapIntensityMetricLabel.value,
    tone: 'neutral',
  },
  {
    key: 'brightnessValue',
    label: t('components.main.heatmap.metrics.brightnessValue'),
    value: heatmapIntensityMetricValue.value,
    tone: getHeatmapMetricTone(heatmapIntensityMetric.value),
  },
  {
    key: 'brightnessLevelRatio',
    label: t('components.main.heatmap.metrics.brightnessLevelRatio'),
    value: heatmapIntensityLevelRatioValue.value,
    tone: getHeatmapIntensityTone(usageTooltip.intensity),
    fullWidth: true,
  },
])

const usageTooltipSections = computed<UsageTooltipSection[]>(() =>
  HEATMAP_TOOLTIP_SECTION_ORDER.map((sectionKey) => {
    const sectionMeta = usageTooltipSectionMeta.value[sectionKey]
    const rows = usageTooltipMetricRowDefinitions.value
      .filter((item) => item.section === sectionKey)
      .map<UsageTooltipRow>((item) => ({
        key: item.key,
        label: item.label,
        value: formatHeatmapMetricValue(item.metric, getUsageTooltipMetricRawValue(item.metric)),
        tone: getHeatmapMetricTone(item.metric),
        emphasis: item.emphasis,
        active: isActiveHeatmapMetric(item.metric),
      }))
    const activeRow = rows.find((row) => row.active)
    return {
      key: sectionKey,
      title: sectionMeta.title,
      tone: activeRow?.tone ?? sectionMeta.tone,
      rows,
    }
  }),
)

const clamp = (value: number, min: number, max: number) => {
  if (max <= min) return min
  return Math.min(Math.max(value, min), max)
}

const TOOLTIP_DEFAULT_WIDTH = 336
const TOOLTIP_DEFAULT_HEIGHT = 384
const TOOLTIP_VERTICAL_OFFSET = 12
const TOOLTIP_HORIZONTAL_MARGIN = 20
const TOOLTIP_VERTICAL_MARGIN = 24

const getTooltipSize = () => {
  const rect = tooltipRef.value?.getBoundingClientRect()
  return {
    width: rect?.width ?? TOOLTIP_DEFAULT_WIDTH,
    height: rect?.height ?? TOOLTIP_DEFAULT_HEIGHT,
  }
}

const viewportSize = () => {
  if (typeof window !== 'undefined') {
    return { width: window.innerWidth, height: window.innerHeight }
  }
  if (typeof document !== 'undefined' && document.documentElement) {
    return {
      width: document.documentElement.clientWidth,
      height: document.documentElement.clientHeight,
    }
  }
  return {
    width: heatmapContainerRef.value?.clientWidth ?? 0,
    height: heatmapContainerRef.value?.clientHeight ?? 0,
  }
}

const applyUsageTooltipMetrics = (day: UsageHeatmapDay) => {
  usageTooltip.label = day.label
  usageTooltip.dateKey = day.dateKey
  usageTooltip.requests = day.requests
  usageTooltip.inputTokens = day.inputTokens
  usageTooltip.outputTokens = day.outputTokens
  usageTooltip.totalTokens = day.totalTokens
  usageTooltip.reasoningTokens = day.reasoningTokens
  usageTooltip.cost = day.cost
  usageTooltip.intensity = day.intensity
  usageTooltip.intensityValue = day.intensityValue
  usageTooltip.intensityPeakValue = day.intensityPeakValue
}

const updateUsageTooltipPosition = (cellRect: DOMRect | ReturnType<HTMLElement['getBoundingClientRect']>) => {
  const { width: tooltipWidth, height: tooltipHeight } = getTooltipSize()
  const { width: viewportWidth, height: viewportHeight } = viewportSize()
  const centerX = cellRect.left + cellRect.width / 2
  const halfWidth = tooltipWidth / 2
  const minLeft = TOOLTIP_HORIZONTAL_MARGIN + halfWidth
  const maxLeft = viewportWidth > 0 ? viewportWidth - halfWidth - TOOLTIP_HORIZONTAL_MARGIN : centerX
  usageTooltip.left = clamp(centerX, minLeft, maxLeft)

  const anchorTop = cellRect.top
  const anchorBottom = cellRect.bottom
  const canShowAbove = anchorTop - tooltipHeight - TOOLTIP_VERTICAL_OFFSET >= TOOLTIP_VERTICAL_MARGIN
  const viewportBottomLimit = viewportHeight > 0 ? viewportHeight - tooltipHeight - TOOLTIP_VERTICAL_MARGIN : anchorBottom
  const shouldPlaceBelow = !canShowAbove
  usageTooltip.placement = shouldPlaceBelow ? 'below' : 'above'
  const desiredTop = shouldPlaceBelow
    ? anchorBottom + TOOLTIP_VERTICAL_OFFSET
    : anchorTop - tooltipHeight - TOOLTIP_VERTICAL_OFFSET
  usageTooltip.top = clamp(desiredTop, TOOLTIP_VERTICAL_MARGIN, viewportBottomLimit)
}

let usageTooltipPositionRequestId = 0

const finalizeUsageTooltipPosition = async (
  cellRect: DOMRect | ReturnType<HTMLElement['getBoundingClientRect']>,
  positionRequestId: number,
) => {
  await nextTick()
  if (!usageTooltip.visible || positionRequestId !== usageTooltipPositionRequestId) return
  updateUsageTooltipPosition(cellRect)
  usageTooltip.positioned = true
}

const showUsageTooltip = (day: UsageHeatmapDay, event: MouseEvent) => {
  const target = event.currentTarget as HTMLElement | null
  if (!target) return

  const cellRect = target.getBoundingClientRect()
  const isInitialRender = !usageTooltip.visible
  const positionRequestId = ++usageTooltipPositionRequestId
  applyUsageTooltipMetrics(day)

  if (isInitialRender || !usageTooltip.positioned) {
    usageTooltip.visible = true
    usageTooltip.positioned = false
    void finalizeUsageTooltipPosition(cellRect, positionRequestId)
    return
  }

  updateUsageTooltipPosition(cellRect)
  usageTooltip.positioned = true
  void finalizeUsageTooltipPosition(cellRect, positionRequestId)
}

const hideUsageTooltip = () => {
  usageTooltipPositionRequestId += 1
  usageTooltip.positioned = false
  usageTooltip.visible = false
}

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

const buildProviderPricingRefreshTargets = () => {
  const seen = new Set<string>()
  const targets: Array<{ card: AutomationCard; tab: ProviderTab }> = []

  for (const tab of PROVIDER_TAB_IDS) {
    for (const card of cards[tab]) {
      const apiUrl = String(card.apiUrl ?? '').trim()
      const apiKey = String(card.apiKey ?? '').trim()
      if (!apiUrl || !apiKey) continue

      const dedupeKey = getProviderPricingRefreshKey(card)
      if (!dedupeKey || seen.has(dedupeKey)) continue

      seen.add(dedupeKey)
      targets.push({ card, tab })
    }
  }

  return targets
}

const providerPricingRefreshLastRunAt = new Map<string, number>()
const providerPricingRefreshInFlight = new Map<string, Promise<void>>()

type ProviderPricingRefreshOptions = {
  force?: boolean
  silent?: boolean
}

const getProviderPricingRefreshKey = (card: AutomationCard) => {
  const apiUrl = String(card.apiUrl ?? '').trim().toLowerCase()
  const apiKey = String(card.apiKey ?? '').trim()
  if (!apiUrl || !apiKey) return ''
  const authType = String(card.connectivityAuthType ?? '').trim().toLowerCase()
  return `${apiUrl}|${apiKey}|${authType}`
}

const refreshProviderPricingForCard = (
  card: AutomationCard,
  tab: ProviderTab,
  options: ProviderPricingRefreshOptions = {},
) => {
  const refreshKey = getProviderPricingRefreshKey(card)
  if (!refreshKey) return Promise.resolve()

  const inFlight = providerPricingRefreshInFlight.get(refreshKey)
  if (inFlight) return inFlight

  if (!options.force) {
    const lastRunAt = providerPricingRefreshLastRunAt.get(refreshKey) ?? 0
    if (Date.now() - lastRunAt < PROVIDER_PRICING_CLICK_THROTTLE_MS) {
      return Promise.resolve()
    }
  }

  providerPricingRefreshLastRunAt.set(refreshKey, Date.now())
  const task = fetchProviderModelPricing(card, tab)
    .then(() => undefined)
    .catch((error) => {
      if (!options.silent) {
        console.warn('[ProviderPricing] refresh failed:', card.name, error)
      }
    })
    .finally(() => {
      providerPricingRefreshInFlight.delete(refreshKey)
    })

  providerPricingRefreshInFlight.set(refreshKey, task)
  return task
}

const runTasksWithConcurrencyLimit = async (tasks: Array<() => Promise<void>>, limit: number) => {
  if (tasks.length === 0) return

  const safeLimit = Math.max(1, Math.min(limit, tasks.length))
  let index = 0

  const workers = Array.from({ length: safeLimit }, async () => {
    while (index < tasks.length) {
      const current = index
      index++
      await tasks[current]()
    }
  })

  await Promise.all(workers)
}

const refreshProviderPricingCachesOnStartup = async () => {
  const targets = buildProviderPricingRefreshTargets()
  if (targets.length === 0) return

  const tasks = targets.map(({ card, tab }) => (
    () => refreshProviderPricingForCard(card, tab, { force: true, silent: true })
  ))
  await runTasksWithConcurrencyLimit(tasks, PROVIDER_PRICING_STARTUP_CONCURRENCY)
}

const handleProviderCardClick = (card: AutomationCard) => {
  const apiUrl = String(card.apiUrl ?? '').trim()
  const apiKey = String(card.apiKey ?? '').trim()
  if (!apiUrl || !apiKey) return
  void refreshProviderPricingForCard(card, activeTab.value, { silent: true })
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
  router.push('/settings')
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

const loadProviderStats = async (tab: ProviderTab) => {
  if (tab === 'others') {
    providerStatsLoaded[tab] = true
    return
  }

  try {
    const stats = await fetchProviderDailyStats(tab as 'claude' | 'codex' | 'gemini')
    const mapped: Record<string, ProviderDailyStat> = {}
    ;(stats ?? []).forEach((stat) => {
      mapped[providerStatsKeyFromStat(stat)] = stat
    })
    providerStatsMap[tab] = mapped
    providerStatsLoaded[tab] = true
  } catch (error) {
    console.error(`Failed to load provider stats for ${tab}`, error)
    if (!providerStatsLoaded[tab]) {
      providerStatsLoaded[tab] = true
    }
  }
}

const refreshing = ref(false)
const refreshAllData = async () => {
  if (refreshing.value) return

  refreshing.value = true
  try {
    await Promise.all([
      reloadHeatmap(),
      loadProvidersFromDisk(loadCustomCliTools),
      ...PROVIDER_TAB_IDS.map(refreshProxyState),
      ...PROVIDER_TAB_IDS.map((tab) => refreshDirectAppliedStatus(tab)),
      ...PROVIDER_TAB_IDS.map((tab) => loadProviderStats(tab)),
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

const formatSuccessRateLabel = (value: number) => {
  const percent = clamp(value, 0, 1) * 100
  const decimals = percent >= 99.5 || percent === 0 ? 0 : 1
  return `${t('components.main.providers.successRate')}: ${percent.toFixed(decimals)}%`
}

const successRateClassName = (value: number) => {
  const rate = clamp(value, 0, 1)
  if (rate >= SUCCESS_RATE_THRESHOLDS.healthy) {
    return 'success-good'
  }
  if (rate >= SUCCESS_RATE_THRESHOLDS.warning) {
    return 'success-warn'
  }
  return 'success-bad'
}

const providerStatDisplay = (card: AutomationCard): ProviderStatDisplay => {
  const tab = activeTab.value
  if (!providerStatsLoaded[tab]) {
    return { state: 'loading', message: t('components.main.providers.loading') }
  }

  const statKey = cardProviderRef(card) || normalizeProviderKey(card.name)
  const stat = providerStatsMap[tab]?.[statKey] ?? providerStatsMap[tab]?.[normalizeProviderKey(card.name)]
  if (!stat) {
    return { state: 'empty', message: t('components.main.providers.noData') }
  }

  const inputTokens = Number.isFinite(Number(stat.input_tokens)) ? Number(stat.input_tokens) : 0
  const outputTokens = Number.isFinite(Number(stat.output_tokens)) ? Number(stat.output_tokens) : 0
  const cacheReadTokens = Number.isFinite(Number(stat.cache_read_tokens)) ? Number(stat.cache_read_tokens) : 0
  const totalTokens = Math.max(0, inputTokens + outputTokens + cacheReadTokens)
  const successRateValue = Number.isFinite(stat.success_rate) ? clamp(stat.success_rate, 0, 1) : null
  const successRateLabel = successRateValue !== null ? formatSuccessRateLabel(successRateValue) : ''
  const successRateClass = successRateValue !== null ? successRateClassName(successRateValue) : ''
  const ttftSampleCountRaw = Number(stat.ttft_sample_count ?? 0)
  const tpsSampleCountRaw = Number(stat.tps_sample_count ?? 0)
  const ttftSampleCount = Number.isFinite(ttftSampleCountRaw) ? Math.max(0, Math.floor(ttftSampleCountRaw)) : 0
  const tpsSampleCount = Number.isFinite(tpsSampleCountRaw) ? Math.max(0, Math.floor(tpsSampleCountRaw)) : 0
  const performanceHint = t('components.main.providers.performanceHint', {
    ttftSamples: formatMetric(ttftSampleCount),
    tpsSamples: formatMetric(tpsSampleCount),
    minWindowMs: 50,
  })

  return {
    state: 'ready',
    requests: `${t('components.main.providers.requests')}: ${formatMetric(stat.total_requests)}`,
    tokens: `${t('components.main.providers.tokens')}: ${formatTokenNumber(totalTokens)}`,
    cost: `${t('components.main.providers.cost')}: ${currencyFormatter.value.format(Math.max(stat.cost_total, 0))}`,
    ttft: formatAverageFirstTokenMs(stat.avg_first_token_sec),
    tps: formatAverageTokensPerSecond(stat.avg_tokens_per_sec),
    performanceHint,
    successRateLabel,
    successRateClass,
  }
}

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

const startProviderStatsTimer = () => {
  stopProviderStatsTimer()
  providerStatsTimer = window.setInterval(() => {
    PROVIDER_TAB_IDS.forEach((tab) => {
      void loadProviderStats(tab)
    })
    void loadAvailabilityResults()
  }, 60_000)
}

const stopProviderStatsTimer = () => {
  if (providerStatsTimer) {
    clearInterval(providerStatsTimer)
    providerStatsTimer = undefined
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
  router.push('/settings')
}

const toggleTheme = () => {
  const nextTheme = resolvedTheme.value === 'dark' ? 'light' : 'dark'
  themeMode.value = nextTheme
  setTheme(nextTheme)
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

const bindCardRef = (card: AutomationCard) => (element: Element | ComponentPublicInstance | null) => {
  const target = element instanceof HTMLElement ? element : null
  if (isHighlightedCard(card)) {
    scrollToCard(target)
  }
}

const openModelList = (card: AutomationCard) => {
  if (!card.apiUrl || !card.apiKey) {
    showToast(t('components.main.modelList.apiKeyRequired'), 'error')
    return
  }
  modelListModalProvider.value = card
  modelListModalOpen.value = true
}

const closeModelListModal = () => {
  modelListModalOpen.value = false
  modelListModalProvider.value = null
}

const openCreateModal = () => {
  providerModalState.tabId = activeTab.value
  providerModalState.card = null
  providerModalState.open = true
}

const openEditModal = (card: AutomationCard) => {
  providerModalState.tabId = activeTab.value
  providerModalState.card = card
  providerModalState.open = true
}

const closeProviderModal = () => {
  providerModalState.open = false
}

const saveProviderModal = async (form: VendorForm, applyAfterSave = false) => {
  const tabId = providerModalState.tabId
  const list = cards[tabId]
  if (!list) return

  const editingCard = providerModalState.card
  let savedCard: AutomationCard | null = null
  const providerFields = buildPersistedProviderFieldsFromForm(form, tabId, normalizeLevel)

  if (editingCard) {
    const previousLevel = normalizeLevel(editingCard.level)
    const nextLevel = providerFields.level

    Object.assign(editingCard, {
      name: form.name || editingCard.name,
      apiUrl: form.apiUrl || editingCard.apiUrl,
      ...providerFields,
    })

    if (previousLevel !== nextLevel) {
      sortProvidersByLevel(list)
    }
    savedCard = editingCard
    await persistProviders(tabId)
  } else {
    const newCardId = Date.now()
    const providerRef = tabId === 'gemini' ? createGeminiProviderRef() : `${newCardId}`
    const newCard: AutomationCard = {
      id: newCardId,
      providerRef,
      name: form.name || 'Untitled vendor',
      apiUrl: form.apiUrl,
      accent: '#0a84ff',
      tint: 'rgba(15, 23, 42, 0.12)',
      ...providerFields,
    }
    list.push(newCard)
    sortProvidersByLevel(list)
    savedCard = newCard
    await persistProviders(tabId)
  }

  const cliConfig = form.cliConfig
  const supportedPlatforms: CLIPlatform[] = ['claude', 'codex', 'gemini']
  if (cliConfig && Object.keys(cliConfig).length > 0 && supportedPlatforms.includes(tabId as CLIPlatform)) {
    try {
      await saveCLIConfig(tabId as CLIPlatform, cliConfig)
    } catch (error) {
      console.error('保存 CLI 配置失败:', error)
    }
  }

  closeProviderModal()
  window.dispatchEvent(new CustomEvent('providers-updated'))

  if (!applyAfterSave || !savedCard || tabId === 'others') return

  try {
    if (tabId === 'claude') {
      await Call.ByName('codeswitch/services.ClaudeSettingsService.ApplySingleProvider', savedCard.id)
    } else if (tabId === 'codex') {
      await Call.ByName('codeswitch/services.CodexSettingsService.ApplySingleProvider', savedCard.id)
    } else if (tabId === 'gemini') {
      const providerRef = normalizeProviderRef(savedCard.providerRef)
      if (providerRef) {
        await Call.ByName('codeswitch/services.GeminiService.ApplySingleProvider', providerRef)
      }
    }
    await refreshDirectAppliedStatus(tabId)
    showToast(t('components.main.directApply.success', { name: savedCard.name }), 'success')
  } catch (error) {
    console.error('Apply after save failed', error)
    showToast(t('components.main.directApply.failed'), 'error')
  }
}

const submitProviderModal = async (form: VendorForm) => {
  await saveProviderModal(form, false)
}

const submitAndApplyProviderModal = async (form: VendorForm) => {
  await saveProviderModal(form, true)
}

const configure = (card: AutomationCard) => {
  openEditModal(card)
}

const requestRemove = (card: AutomationCard) => {
  confirmState.card = card
  confirmState.tabId = activeTab.value
  confirmState.open = true
}

const closeConfirm = () => {
  confirmState.open = false
  confirmState.card = null
}

const confirmRemove = async () => {
  if (!confirmState.card) return
  await removeProvider(confirmState.card.id, confirmState.tabId)
  closeConfirm()
}

const handleDuplicate = async (card: AutomationCard) => {
  const duplicated = await duplicateProvider(card)
  if (duplicated) {
    await loadProvidersFromDisk(loadCustomCliTools)
  }
}

const handleProviderEnabledChange = async (card: AutomationCard, enabled: boolean) => {
  card.enabled = enabled
  await persistProviders(activeTab.value)
}

const onTabChange = (index: number) => {
  selectedIndex.value = index
  const nextTab = tabs[index]?.id
  if (!nextTab) return

  void refreshProxyState(nextTab)
  void refreshDirectAppliedStatus(nextTab)
  void loadProviderStats(nextTab)
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
      await loadProvidersFromDisk(loadCustomCliTools)
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

const handleAppSettingsUpdated = () => {
  void loadAppSettings()
  void pollUpdateState()
}

let handleProvidersUpdated: (() => void) | undefined

watch(activeTab, (newTab) => {
  void loadBlacklistStatus(newTab)
})

onMounted(async () => {
  await loadAppSettings()
  void initHeatmap()
  await loadProvidersFromDisk(loadCustomCliTools)
  void refreshProviderPricingCachesOnStartup()
  await Promise.all(PROVIDER_TAB_IDS.map(refreshProxyState))
  await Promise.all(PROVIDER_TAB_IDS.map((tab) => refreshDirectAppliedStatus(tab)))
  await Promise.all(PROVIDER_TAB_IDS.map((tab) => loadProviderStats(tab)))
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
    void loadProvidersFromDisk(loadCustomCliTools)
  }
  window.addEventListener('providers-updated', handleProvidersUpdated)

  await loadLastUsedProviders()
})

onUnmounted(() => {
  cleanupHeatmap()
  stopProviderStatsTimer()
  stopUpdateTimer()
  stopStatusSync()
  window.removeEventListener('app-settings-updated', handleAppSettingsUpdated)
  if (handleProvidersUpdated) {
    window.removeEventListener('providers-updated', handleProvidersUpdated)
    handleProvidersUpdated = undefined
  }
})
</script>
