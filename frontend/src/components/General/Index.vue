<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Call, Events, System } from '@wailsio/runtime'
import ListItem from '../Setting/ListRow.vue'
import LanguageSwitcher from '../Setting/LanguageSwitcher.vue'
import ThemeSetting from '../Setting/ThemeSetting.vue'
import NetworkWslSettings from '../Setting/NetworkWslSettings.vue'
import ModelPricingModal from '../Setting/ModelPricingModal.vue'
import InlineModal from '../common/InlineModal.vue'
import {
  fetchAppSettings,
  saveAppSettings,
  saveMainWindowDestroyDelay,
  DEFAULT_MAIN_WINDOW_DESTROY_DELAY_SECONDS,
  MAX_MAIN_WINDOW_DESTROY_DELAY_SECONDS,
  MIN_MAIN_WINDOW_DESTROY_DELAY_SECONDS,
  DEFAULT_PROVIDER_QUOTA_RECOVERY_INTERVAL_SECONDS,
  MAX_PROVIDER_QUOTA_RECOVERY_INTERVAL_SECONDS,
  MIN_PROVIDER_QUOTA_RECOVERY_INTERVAL_SECONDS,
  normalizeHeatmapGranularity,
  normalizeMainWindowDestroyDelaySeconds,
  normalizeProviderQuotaRecoveryIntervalSeconds,
  type AppSettings,
  type ClaudeModelMetadataMergeStrategy,
  type ClaudeProxyAuthField,
  normalizeClaudeProxyAuthField,
} from '../../services/appSettings'
import {
  getClaudeModelRoutingStatus,
  refreshClaudeModelRoutes,
  type ClaudeModelRoutingStatus,
} from '../../services/claudeModelRouting'
import {
  DEFAULT_HEATMAP_DISPLAY_SETTINGS,
  normalizeHeatmapDailyIntensityMode,
  normalizeHeatmapDisplaySettings,
  normalizeHeatmapIntensityMetric,
  type HeatmapDisplaySettings,
} from '../../data/heatmapDisplaySettings'
import {
  HOME_PROVIDER_TAB_OPTIONS,
  moveHomeProviderTab,
  normalizeHomeProviderTabs,
  reorderHomeProviderTabs,
  resolveHomeProviderTabOptions,
  setHomeProviderTabVisibility,
  type HomeProviderTab,
} from '../../data/homeProviderTabs'
import {
  getProviderDisplayIconSvg,
  preloadProviderDisplayIcons,
} from '../../utils/providerIconAssets'
import { checkUpdate, downloadUpdate, restartApp, getUpdateState, setAutoCheckEnabled, type UpdateInfo, type UpdateState } from '../../services/update'
import { fetchCurrentVersion } from '../../services/version'
import { getBlacklistSettings, updateBlacklistSettingsWithHealthThreshold, getHealthBlacklistThreshold, getLevelBlacklistEnabled, setLevelBlacklistEnabled, getBlacklistEnabled, setBlacklistEnabled, type BlacklistSettings } from '../../services/settings'
import { fetchConfigImportStatus, importFromPath, type ConfigImportStatus } from '../../services/configImport'
import { fetchWebDAVConfig, previewWebDAVContent, saveWebDAVConfig, testWebDAVConfig, syncToWebDAV, loadFromWebDAV, type WebDAVSyncConfig } from '../../services/webdavSync'
import { hasCodexUnifiedHistoryBackup, restoreCodexUnifiedHistory } from '../../services/claudeSettings'
import { fetchCostSince, fetchFiveHourQuotaStatus } from '../../services/logs'
import { useI18n } from 'vue-i18n'
import { extractErrorMessage } from '../../utils/error'
import { showToast } from '../../utils/toast'
import {
  budgetQuotaOrder,
  cloneBudgetQuotaAdjustments,
  cloneBudgetQuotaSettings,
  createDefaultBudgetQuotaAdjustments,
  createDefaultBudgetQuotaSettings,
  formatLocalDateTime,
  normalizeBudgetAdjustmentPrecision,
  normalizeBudgetEditableAmount,
  normalizeBudgetQuotaAdjustments,
  normalizeBudgetQuotaSettings,
  normalizeBudgetUsedDisplay,
  projectBudgetQuotaToLegacy,
  resolveBudgetQuotaWindow,
  resolveBudgetCurrentUsedValue,
  type BudgetQuotaKey,
  type BudgetQuotaAdjustments,
  type BudgetQuotaSetting,
  type BudgetQuotaSettings,
} from '../../utils/budgetUsage'

const { t } = useI18n()

const router = useRouter()
// 从 localStorage 读取缓存值作为初始值，避免加载时的视觉闪烁
const getCachedValue = (key: string, defaultValue: boolean): boolean => {
  const cached = localStorage.getItem(`app-settings-${key}`)
  return cached !== null ? cached === 'true' : defaultValue
}
const getCachedNumber = (key: string, defaultValue: number): number => {
  const cached = localStorage.getItem(`app-settings-${key}`)
  if (cached === null) return defaultValue
  const parsed = Number(cached)
  return Number.isFinite(parsed) ? parsed : defaultValue
}
const getCachedString = (key: string, defaultValue: string): string => {
  const cached = localStorage.getItem(`app-settings-${key}`)
  return cached !== null ? cached : defaultValue
}
const getCachedJson = <T,>(key: string, defaultValue: T): T => {
  const cached = localStorage.getItem(`app-settings-${key}`)
  if (cached === null) return defaultValue
  try {
    return JSON.parse(cached) as T
  } catch {
    return defaultValue
  }
}
const getCachedHomeProviderTabs = (): HomeProviderTab[] => {
  return normalizeHomeProviderTabs(getCachedJson('homeProviderTabs', null))
}

const getHomeProviderTabIconSvg = (icon: string) => getProviderDisplayIconSvg(icon)

const getHomeProviderTabInitials = (label: string) => {
  return label
    .split(/\s+/)
    .filter(Boolean)
    .map((word) => word[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()
}

const mapBudgetQuotaValues = (resolveValue: (key: BudgetQuotaKey) => number): BudgetQuotaAdjustments => {
  const nextValues = createDefaultBudgetQuotaAdjustments()
  budgetQuotaOrder.forEach((key) => {
    nextValues[key] = resolveValue(key)
  })
  return nextValues
}

type BudgetQuotaUsageStatus = 'inactive' | 'loading' | 'ready' | 'error'
type BudgetQuotaUsageStatuses = Record<BudgetQuotaKey, BudgetQuotaUsageStatus>

const createDefaultBudgetQuotaUsageStatuses = (
  status: BudgetQuotaUsageStatus = 'inactive',
): BudgetQuotaUsageStatuses => ({
  five_hour: status,
  daily: status,
  weekly: status,
  monthly: status,
  total: status,
})

const mapBudgetQuotaUsageStatuses = (
  resolveStatus: (key: BudgetQuotaKey) => BudgetQuotaUsageStatus,
): BudgetQuotaUsageStatuses => {
  const nextStatuses = createDefaultBudgetQuotaUsageStatuses()
  budgetQuotaOrder.forEach((key) => {
    nextStatuses[key] = resolveStatus(key)
  })
  return nextStatuses
}

const defaultUpdateHistoryKeepCount = 3
const minUpdateHistoryKeepCount = 1
const maxUpdateHistoryKeepCount = 20
const heatmapEnabled = ref(getCachedValue('heatmap', true))
const heatmapGranularity = ref(normalizeHeatmapGranularity(getCachedString('heatmapGranularity', 'daily')))
const initialHeatmapDisplaySettings = normalizeHeatmapDisplaySettings({
  dailyScaleFactor: getCachedNumber('heatmapDailyScaleFactor', DEFAULT_HEATMAP_DISPLAY_SETTINGS.dailyScaleFactor),
  dailyIntensityMode: normalizeHeatmapDailyIntensityMode(
    getCachedString('heatmapDailyIntensityMode', DEFAULT_HEATMAP_DISPLAY_SETTINGS.dailyIntensityMode),
  ),
  intensityMetric: normalizeHeatmapIntensityMetric(
    getCachedString('heatmapIntensityMetric', DEFAULT_HEATMAP_DISPLAY_SETTINGS.intensityMetric),
  ),
  intensityStopL1: getCachedNumber('heatmapIntensityStopL1', DEFAULT_HEATMAP_DISPLAY_SETTINGS.intensityStopL1),
  intensityStopL2: getCachedNumber('heatmapIntensityStopL2', DEFAULT_HEATMAP_DISPLAY_SETTINGS.intensityStopL2),
  intensityStopL3: getCachedNumber('heatmapIntensityStopL3', DEFAULT_HEATMAP_DISPLAY_SETTINGS.intensityStopL3),
})
const heatmapDailyScaleFactor = ref(initialHeatmapDisplaySettings.dailyScaleFactor)
const heatmapDailyIntensityMode = ref(initialHeatmapDisplaySettings.dailyIntensityMode)
const heatmapIntensityMetric = ref(initialHeatmapDisplaySettings.intensityMetric)
const heatmapIntensityStopL1 = ref(initialHeatmapDisplaySettings.intensityStopL1)
const heatmapIntensityStopL2 = ref(initialHeatmapDisplaySettings.intensityStopL2)
const heatmapIntensityStopL3 = ref(initialHeatmapDisplaySettings.intensityStopL3)
const homeTitleVisible = ref(getCachedValue('homeTitle', true))
const homeProviderTabs = ref<HomeProviderTab[]>(getCachedHomeProviderTabs())
const visibleHomeProviderTabOptions = computed(() => resolveHomeProviderTabOptions(homeProviderTabs.value))
const hiddenHomeProviderTabOptions = computed(() => HOME_PROVIDER_TAB_OPTIONS.filter((tab) => !homeProviderTabs.value.includes(tab.id)))
const draggedHomeProviderTab = ref<HomeProviderTab | null>(null)
const dragOverHomeProviderTab = ref<HomeProviderTab | null>(null)
const dragOverHomeProviderTabPosition = ref<'before' | 'after' | null>(null)

const commitHomeProviderTabs = (nextTabs: readonly HomeProviderTab[]) => {
  const normalizedTabs = normalizeHomeProviderTabs(nextTabs)
  const currentTabs = normalizeHomeProviderTabs(homeProviderTabs.value)
  if (
    normalizedTabs.length === currentTabs.length
    && normalizedTabs.every((tabId, index) => tabId === currentTabs[index])
  ) return

  homeProviderTabs.value = normalizedTabs
  persistAppSettings()
}

const toggleHomeProviderTab = (tabId: HomeProviderTab, checked: boolean) => {
  commitHomeProviderTabs(setHomeProviderTabVisibility(homeProviderTabs.value, tabId, checked))
}

const resetHomeProviderTabDragTarget = () => {
  dragOverHomeProviderTab.value = null
  dragOverHomeProviderTabPosition.value = null
}

const resetHomeProviderTabDragState = () => {
  draggedHomeProviderTab.value = null
  resetHomeProviderTabDragTarget()
}

const handleHomeProviderTabDragStart = (event: DragEvent, tabId: HomeProviderTab) => {
  draggedHomeProviderTab.value = tabId
  resetHomeProviderTabDragTarget()
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', tabId)
  }
}

const handleHomeProviderTabDragOver = (event: DragEvent, targetTabId: HomeProviderTab) => {
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'

  const sourceTabId = draggedHomeProviderTab.value
  if (!sourceTabId || sourceTabId === targetTabId) {
    resetHomeProviderTabDragTarget()
    return
  }

  const currentTabs = normalizeHomeProviderTabs(homeProviderTabs.value)
  const sourceIndex = currentTabs.indexOf(sourceTabId)
  const targetIndex = currentTabs.indexOf(targetTabId)
  if (sourceIndex < 0 || targetIndex < 0) {
    resetHomeProviderTabDragTarget()
    return
  }

  dragOverHomeProviderTab.value = targetTabId
  dragOverHomeProviderTabPosition.value = sourceIndex < targetIndex ? 'after' : 'before'
}

const handleHomeProviderTabDragLeave = (event: DragEvent, targetTabId: HomeProviderTab) => {
  const currentTarget = event.currentTarget as HTMLElement | null
  const relatedTarget = event.relatedTarget as Node | null
  if (currentTarget && relatedTarget && currentTarget.contains(relatedTarget)) return
  if (dragOverHomeProviderTab.value === targetTabId) resetHomeProviderTabDragTarget()
}

const handleHomeProviderTabDrop = (event: DragEvent, targetTabId: HomeProviderTab) => {
  event.preventDefault()
  const sourceTabId = draggedHomeProviderTab.value
  resetHomeProviderTabDragState()
  if (!sourceTabId || sourceTabId === targetTabId) return
  commitHomeProviderTabs(reorderHomeProviderTabs(homeProviderTabs.value, sourceTabId, targetTabId))
}

const handleHomeProviderTabDragEnd = () => {
  resetHomeProviderTabDragState()
}

const moveVisibleHomeProviderTab = (tabId: HomeProviderTab, offset: number) => {
  commitHomeProviderTabs(moveHomeProviderTab(homeProviderTabs.value, tabId, offset))
}
const autoStartEnabled = ref(getCachedValue('autoStart', false))
const isMacPlatform = System.IsMac()
const mainWindowDestroyDelaySeconds = ref(
  normalizeMainWindowDestroyDelaySeconds(getCachedNumber('mainWindowDestroyDelaySeconds', DEFAULT_MAIN_WINDOW_DESTROY_DELAY_SECONDS))
)
const autoUpdateEnabled = ref(getCachedValue('autoUpdate', true))
const updateHistoryKeepCount = ref(getCachedNumber('updateHistoryKeepCount', defaultUpdateHistoryKeepCount))
const autoConnectivityTestEnabled = ref(getCachedValue('autoConnectivityTest', false))
const providerQuotaAutoDisableEnabled = ref(getCachedValue('providerQuotaAutoDisable', false))
const providerQuotaRecoveryIntervalSeconds = ref(
  normalizeProviderQuotaRecoveryIntervalSeconds(
    getCachedNumber('providerQuotaRecoveryIntervalSeconds', DEFAULT_PROVIDER_QUOTA_RECOVERY_INTERVAL_SECONDS),
  ),
)
const providerQuotaRecoveryNotifyEnabled = ref(getCachedValue('providerQuotaRecoveryNotify', false))
const switchNotifyEnabled = ref(getCachedValue('switchNotify', true)) // 切换通知开关
const roundRobinEnabled = ref(getCachedValue('roundRobin', false))    // 同 Level 轮询开关
const claudeModelRoutingEnabled = ref(getCachedValue('claudeModelRouting', false))
const claudeModelAggregationEnabled = ref(getCachedValue('claudeModelAggregation', false))
const claudeModelMetadataMergeStrategy = ref<ClaudeModelMetadataMergeStrategy>(
  getCachedString('claudeModelMetadataMergeStrategy', 'aggressive') === 'conservative'
    ? 'conservative'
    : 'aggressive',
)
const claudeModelRoutingStatus = ref<ClaudeModelRoutingStatus | null>(null)
const claudeModelRefreshBusy = ref(false)
const claudeModelMetadataStrategies: ClaudeModelMetadataMergeStrategy[] = ['aggressive', 'conservative']
const claudeProxyAuthField = ref<ClaudeProxyAuthField>(
  normalizeClaudeProxyAuthField(getCachedString('claudeProxyAuthField', 'auth_token')),
)
const claudeProxyAuthFields: ClaudeProxyAuthField[] = ['auth_token', 'api_key']
const preserveCodexOfficialAuthOnSwitch = ref(getCachedValue('preserveCodexOfficialAuthOnSwitch', false))
const unifyCodexSessionHistory = ref(getCachedValue('unifyCodexSessionHistory', false))
const unifyCodexMigrateExisting = ref(false)
const codexUnifyEnableConfirmOpen = ref(false)
const codexUnifyDisableConfirmOpen = ref(false)
const codexUnifyRestoreBackup = ref(true)
const codexUnifyHasBackup = ref(false)
const codexUnifyRestoreBusy = ref(false)
const captureRequestLogPayloadEnabled = ref(getCachedValue('captureRequestLogPayload', false))
const sanitizeRequestLogPayloadEnabled = ref(getCachedValue('sanitizeRequestLogPayload', true))
const budgetQuotaUsedAdjustments = ref<BudgetQuotaAdjustments>(normalizeBudgetQuotaAdjustments(
  getCachedJson('budgetQuotaUsedAdjustments', createDefaultBudgetQuotaAdjustments()),
  {
    adjustment: getCachedNumber('budgetUsedAdjustment', 0),
    cycleEnabled: getCachedValue('budgetCycleEnabled', false),
    cycleMode: getCachedString('budgetCycleMode', 'daily'),
  },
))
const budgetQuotaSettings = ref<BudgetQuotaSettings>(normalizeBudgetQuotaSettings(
  getCachedJson('budgetQuotaSettings', createDefaultBudgetQuotaSettings()),
))
const budgetQuotaTrackedUsage = ref<BudgetQuotaAdjustments>(createDefaultBudgetQuotaAdjustments())
const budgetQuotaCurrentUsed = ref<BudgetQuotaAdjustments>(createDefaultBudgetQuotaAdjustments())
const budgetQuotaUsageStatuses = ref<BudgetQuotaUsageStatuses>(createDefaultBudgetQuotaUsageStatuses())
const budgetForecastMethod = ref(getCachedString('budgetForecastMethod', 'cycle'))
const budgetForecastDisplay = ref(getCachedString('budgetForecastDisplay', 'datetime'))
const budgetShowCountdown = ref(getCachedValue('budgetShowCountdown', false))
const budgetShowForecast = ref(getCachedValue('budgetShowForecast', false))
const budgetQuotaUsedAdjustmentsCodex = ref<BudgetQuotaAdjustments>(normalizeBudgetQuotaAdjustments(
  getCachedJson('budgetQuotaUsedAdjustmentsCodex', createDefaultBudgetQuotaAdjustments()),
  {
    adjustment: getCachedNumber('budgetUsedAdjustmentCodex', 0),
    cycleEnabled: getCachedValue('budgetCycleEnabledCodex', false),
    cycleMode: getCachedString('budgetCycleModeCodex', 'daily'),
  },
))
const budgetQuotaSettingsCodex = ref<BudgetQuotaSettings>(normalizeBudgetQuotaSettings(
  getCachedJson('budgetQuotaSettingsCodex', createDefaultBudgetQuotaSettings()),
))
const budgetQuotaTrackedUsageCodex = ref<BudgetQuotaAdjustments>(createDefaultBudgetQuotaAdjustments())
const budgetQuotaCurrentUsedCodex = ref<BudgetQuotaAdjustments>(createDefaultBudgetQuotaAdjustments())
const budgetQuotaUsageStatusesCodex = ref<BudgetQuotaUsageStatuses>(createDefaultBudgetQuotaUsageStatuses())
const budgetForecastMethodCodex = ref(getCachedString('budgetForecastMethodCodex', 'cycle'))
const budgetForecastDisplayCodex = ref(getCachedString('budgetForecastDisplayCodex', 'datetime'))
const budgetShowCountdownCodex = ref(getCachedValue('budgetShowCountdownCodex', false))
const budgetShowForecastCodex = ref(getCachedValue('budgetShowForecastCodex', false))
const budgetQuotaUsageLoading = ref(false)
const budgetQuotaUsageLoadingCodex = ref(false)
const settingsLoading = ref(true)
const saveBusy = ref(false)
let saveQueued = false
let persistTimer: number | undefined
let persistIdleWaiters: Array<() => void> = []
let mainWindowDestroyDelayRequestSeq = 0
let budgetQuotaUsageRequestSeq = 0
let budgetQuotaUsageRequestSeqCodex = 0
const defaultPersistDebounceMs = 150
const minPersistDebounceMs = 0
const maxPersistDebounceMs = 2000
const rawPersistDebounceMs = import.meta.env.VITE_SETTINGS_PERSIST_DEBOUNCE_MS
const envPersistDebounceMs =
  typeof rawPersistDebounceMs === 'string' && rawPersistDebounceMs.trim() !== ''
    ? Number(rawPersistDebounceMs)
    : defaultPersistDebounceMs
const persistDebounceMs = Number.isFinite(envPersistDebounceMs)
  ? Math.min(Math.max(Math.round(envPersistDebounceMs), minPersistDebounceMs), maxPersistDebounceMs)
  : defaultPersistDebounceMs
const monthDayOptions = Array.from({ length: 31 }, (_, index) => index + 1)
const weekdayOptions = [
  { value: 1, labelKey: 'components.general.label.weekdayMon' },
  { value: 2, labelKey: 'components.general.label.weekdayTue' },
  { value: 3, labelKey: 'components.general.label.weekdayWed' },
  { value: 4, labelKey: 'components.general.label.weekdayThu' },
  { value: 5, labelKey: 'components.general.label.weekdayFri' },
  { value: 6, labelKey: 'components.general.label.weekdaySat' },
  { value: 0, labelKey: 'components.general.label.weekdaySun' },
]

type BudgetQuotaDefinition = {
  key: BudgetQuotaKey
  titleKey: string
  hintKey: string
  showWeekday: boolean
  showMonthDay: boolean
  showTime: boolean
}

type BudgetQuotaPlatform = 'claude' | 'codex'

const budgetQuotaDefinitions: BudgetQuotaDefinition[] = [
  {
    key: 'five_hour',
    titleKey: 'components.general.label.budgetQuotaFiveHour',
    hintKey: 'components.general.label.budgetQuotaFiveHourHint',
    showWeekday: false,
    showMonthDay: false,
    showTime: false,
  },
  {
    key: 'daily',
    titleKey: 'components.general.label.budgetQuotaDaily',
    hintKey: 'components.general.label.budgetQuotaDailyHint',
    showWeekday: false,
    showMonthDay: false,
    showTime: true,
  },
  {
    key: 'weekly',
    titleKey: 'components.general.label.budgetQuotaWeekly',
    hintKey: 'components.general.label.budgetQuotaWeeklyHint',
    showWeekday: true,
    showMonthDay: false,
    showTime: true,
  },
  {
    key: 'monthly',
    titleKey: 'components.general.label.budgetQuotaMonthly',
    hintKey: 'components.general.label.budgetQuotaMonthlyHint',
    showWeekday: false,
    showMonthDay: true,
    showTime: true,
  },
]

const formatBudgetLimitLabel = (total: number) => {
  if (total <= 0) return '∞'
  if (total >= 1) return `$${total.toFixed(2)}`
  if (total >= 0.01) return `$${total.toFixed(3)}`
  return `$${total.toFixed(4)}`
}

const buildBudgetQuotaCurrentUsed = (
  trackedUsage: BudgetQuotaAdjustments,
  adjustments: BudgetQuotaAdjustments,
  statuses: BudgetQuotaUsageStatuses,
) => {
  return mapBudgetQuotaValues((key) => (
    statuses[key] === 'ready'
      ? resolveBudgetCurrentUsedValue(trackedUsage[key], adjustments[key])
      : 0
  ))
}

const getBudgetQuotaRefs = (platform: BudgetQuotaPlatform) => {
  return platform === 'codex'
    ? {
      settings: budgetQuotaSettingsCodex,
      adjustments: budgetQuotaUsedAdjustmentsCodex,
      trackedUsage: budgetQuotaTrackedUsageCodex,
      currentUsed: budgetQuotaCurrentUsedCodex,
      statuses: budgetQuotaUsageStatusesCodex,
      loading: budgetQuotaUsageLoadingCodex,
    }
    : {
      settings: budgetQuotaSettings,
      adjustments: budgetQuotaUsedAdjustments,
      trackedUsage: budgetQuotaTrackedUsage,
      currentUsed: budgetQuotaCurrentUsed,
      statuses: budgetQuotaUsageStatuses,
      loading: budgetQuotaUsageLoading,
    }
}

const syncBudgetQuotaCurrentUsedForPlatform = (platform: BudgetQuotaPlatform) => {
  const quotaRefs = getBudgetQuotaRefs(platform)
  quotaRefs.currentUsed.value = buildBudgetQuotaCurrentUsed(
    quotaRefs.trackedUsage.value,
    quotaRefs.adjustments.value,
    quotaRefs.statuses.value,
  )
}

const getBudgetQuotaUsageStatus = (platform: BudgetQuotaPlatform, key: BudgetQuotaKey) => {
  return getBudgetQuotaRefs(platform).statuses.value[key]
}

const isBudgetQuotaCurrentUsedEditable = (platform: BudgetQuotaPlatform, key: BudgetQuotaKey) => {
  return getBudgetQuotaUsageStatus(platform, key) === 'ready'
}

const getBudgetQuotaCurrentUsedHint = (platform: BudgetQuotaPlatform, key: BudgetQuotaKey) => {
  const status = getBudgetQuotaUsageStatus(platform, key)
  if (status === 'inactive') {
    return t('components.general.label.budgetQuotaUsedInactiveHint')
  }
  if (status === 'loading') {
    return t('components.general.label.budgetQuotaUsedLoadingHint')
  }
  if (status === 'error') {
    return t('components.general.label.budgetQuotaUsedUnavailableHint')
  }
  return t('components.general.label.budgetQuotaUsedAdjustmentHint')
}

const nextBudgetQuotaUsageRequestId = (platform: BudgetQuotaPlatform) => {
  if (platform === 'codex') {
    budgetQuotaUsageRequestSeqCodex += 1
    return budgetQuotaUsageRequestSeqCodex
  }
  budgetQuotaUsageRequestSeq += 1
  return budgetQuotaUsageRequestSeq
}

const isBudgetQuotaUsageRequestCurrent = (platform: BudgetQuotaPlatform, requestId: number) => {
  return platform === 'codex'
    ? requestId === budgetQuotaUsageRequestSeqCodex
    : requestId === budgetQuotaUsageRequestSeq
}

const refreshBudgetQuotaUsage = async (platform: BudgetQuotaPlatform) => {
  const requestId = nextBudgetQuotaUsageRequestId(platform)
  const quotaRefs = getBudgetQuotaRefs(platform)
  const quotaSettings = normalizeBudgetQuotaSettings(quotaRefs.settings.value)
  const activeQuotaKeys = budgetQuotaOrder.filter((key) => quotaSettings[key].total > 0)
  const activeQuotaKeySet = new Set<BudgetQuotaKey>(activeQuotaKeys)

  quotaRefs.loading.value = activeQuotaKeys.length > 0
  quotaRefs.statuses.value = mapBudgetQuotaUsageStatuses((key) => (
    activeQuotaKeySet.has(key) ? 'loading' : 'inactive'
  ))
  quotaRefs.trackedUsage.value = createDefaultBudgetQuotaAdjustments()
  quotaRefs.currentUsed.value = createDefaultBudgetQuotaAdjustments()

  if (activeQuotaKeys.length === 0) {
    quotaRefs.loading.value = false
    return
  }

  try {
    const now = new Date()
    const nextTrackedUsage = createDefaultBudgetQuotaAdjustments()
    const nextStatuses = mapBudgetQuotaUsageStatuses((key) => (
      activeQuotaKeySet.has(key) ? 'error' : 'inactive'
    ))
    const results = await Promise.allSettled(
      activeQuotaKeys.map(async (key) => {
        if (key === 'five_hour') {
          const snapshot = await fetchFiveHourQuotaStatus(platform)
          return {
            key,
            usage: normalizeBudgetUsedDisplay(Number(snapshot?.used ?? 0)),
          }
        }
        const setting = quotaSettings[key] as BudgetQuotaSetting
        const window = resolveBudgetQuotaWindow(key, setting, now)
        const usage = await fetchCostSince(formatLocalDateTime(window.start), platform)
        return {
          key,
          usage: normalizeBudgetUsedDisplay(Number(usage)),
        }
      }),
    )
    if (!isBudgetQuotaUsageRequestCurrent(platform, requestId)) return
    results.forEach((result) => {
      if (result.status !== 'fulfilled') return
      nextTrackedUsage[result.value.key] = result.value.usage
      nextStatuses[result.value.key] = 'ready'
    })
    quotaRefs.trackedUsage.value = nextTrackedUsage
    quotaRefs.statuses.value = nextStatuses
    syncBudgetQuotaCurrentUsedForPlatform(platform)
  } catch (error) {
    console.error(`failed to load ${platform} quota usage`, error)
  } finally {
    if (isBudgetQuotaUsageRequestCurrent(platform, requestId)) {
      quotaRefs.loading.value = false
    }
  }
}

const refreshAllBudgetQuotaUsage = async () => {
  await Promise.all([
    refreshBudgetQuotaUsage('claude'),
    refreshBudgetQuotaUsage('codex'),
  ])
}

const handleBudgetQuotaCurrentUsedChange = (
  platform: BudgetQuotaPlatform,
  key: BudgetQuotaKey,
) => {
  const quotaRefs = getBudgetQuotaRefs(platform)
  if (quotaRefs.statuses.value[key] !== 'ready') return
  const nextUsed = normalizeBudgetEditableAmount(quotaRefs.currentUsed.value[key])
  quotaRefs.currentUsed.value[key] = nextUsed
  quotaRefs.adjustments.value[key] = normalizeBudgetAdjustmentPrecision(
    nextUsed - quotaRefs.trackedUsage.value[key],
  )
  syncBudgetQuotaCurrentUsedForPlatform(platform)
  persistAppSettings()
}

const handleBudgetQuotaConfigChange = (platform: BudgetQuotaPlatform) => {
  void refreshBudgetQuotaUsage(platform)
  persistAppSettings()
}

const syncAppSettingsCache = () => {
  localStorage.setItem('app-settings-heatmap', String(heatmapEnabled.value))
  localStorage.setItem('app-settings-heatmapGranularity', heatmapGranularity.value)
  localStorage.setItem('app-settings-heatmapDailyScaleFactor', String(heatmapDailyScaleFactor.value))
  localStorage.setItem('app-settings-heatmapDailyIntensityMode', heatmapDailyIntensityMode.value)
  localStorage.setItem('app-settings-heatmapIntensityMetric', heatmapIntensityMetric.value)
  localStorage.setItem('app-settings-heatmapIntensityStopL1', String(heatmapIntensityStopL1.value))
  localStorage.setItem('app-settings-heatmapIntensityStopL2', String(heatmapIntensityStopL2.value))
  localStorage.setItem('app-settings-heatmapIntensityStopL3', String(heatmapIntensityStopL3.value))
  localStorage.setItem('app-settings-homeTitle', String(homeTitleVisible.value))
  localStorage.setItem('app-settings-homeProviderTabs', JSON.stringify(normalizeHomeProviderTabs(homeProviderTabs.value)))
  localStorage.removeItem('app-settings-budgetUsedAdjustment')
  localStorage.setItem('app-settings-budgetQuotaUsedAdjustments', JSON.stringify(budgetQuotaUsedAdjustments.value))
  localStorage.setItem('app-settings-budgetQuotaSettings', JSON.stringify(budgetQuotaSettings.value))
  localStorage.setItem('app-settings-budgetForecastMethod', budgetForecastMethod.value)
  localStorage.setItem('app-settings-budgetForecastDisplay', budgetForecastDisplay.value)
  localStorage.setItem('app-settings-budgetShowCountdown', String(budgetShowCountdown.value))
  localStorage.setItem('app-settings-budgetShowForecast', String(budgetShowForecast.value))
  localStorage.removeItem('app-settings-budgetUsedAdjustmentCodex')
  localStorage.setItem('app-settings-budgetQuotaUsedAdjustmentsCodex', JSON.stringify(budgetQuotaUsedAdjustmentsCodex.value))
  localStorage.setItem('app-settings-budgetQuotaSettingsCodex', JSON.stringify(budgetQuotaSettingsCodex.value))
  localStorage.setItem('app-settings-budgetForecastMethodCodex', budgetForecastMethodCodex.value)
  localStorage.setItem('app-settings-budgetForecastDisplayCodex', budgetForecastDisplayCodex.value)
  localStorage.setItem('app-settings-budgetShowCountdownCodex', String(budgetShowCountdownCodex.value))
  localStorage.setItem('app-settings-budgetShowForecastCodex', String(budgetShowForecastCodex.value))
  localStorage.setItem('app-settings-autoStart', String(autoStartEnabled.value))
  localStorage.setItem('app-settings-mainWindowDestroyDelaySeconds', String(mainWindowDestroyDelaySeconds.value))
  localStorage.setItem('app-settings-autoUpdate', String(autoUpdateEnabled.value))
  localStorage.setItem('app-settings-updateHistoryKeepCount', String(updateHistoryKeepCount.value))
  localStorage.setItem('app-settings-autoConnectivityTest', String(autoConnectivityTestEnabled.value))
  localStorage.setItem('app-settings-providerQuotaAutoDisable', String(providerQuotaAutoDisableEnabled.value))
  localStorage.setItem('app-settings-providerQuotaRecoveryIntervalSeconds', String(providerQuotaRecoveryIntervalSeconds.value))
  localStorage.setItem('app-settings-providerQuotaRecoveryNotify', String(providerQuotaRecoveryNotifyEnabled.value))
  localStorage.setItem('app-settings-switchNotify', String(switchNotifyEnabled.value))
  localStorage.setItem('app-settings-roundRobin', String(roundRobinEnabled.value))
  localStorage.setItem('app-settings-claudeModelRouting', String(claudeModelRoutingEnabled.value))
  localStorage.setItem('app-settings-claudeModelAggregation', String(claudeModelAggregationEnabled.value))
  localStorage.setItem('app-settings-claudeModelMetadataMergeStrategy', claudeModelMetadataMergeStrategy.value)
  localStorage.setItem('app-settings-claudeProxyAuthField', claudeProxyAuthField.value)
  localStorage.setItem('app-settings-preserveCodexOfficialAuthOnSwitch', String(preserveCodexOfficialAuthOnSwitch.value))
  localStorage.setItem('app-settings-unifyCodexSessionHistory', String(unifyCodexSessionHistory.value))
  localStorage.setItem('app-settings-captureRequestLogPayload', String(captureRequestLogPayloadEnabled.value))
  localStorage.setItem('app-settings-sanitizeRequestLogPayload', String(sanitizeRequestLogPayloadEnabled.value))
  localStorage.removeItem('app-settings-budgetQuotaTrackedUsage')
  localStorage.removeItem('app-settings-budgetQuotaCurrentUsed')
  localStorage.removeItem('app-settings-budgetQuotaTrackedUsageCodex')
  localStorage.removeItem('app-settings-budgetQuotaCurrentUsedCodex')
}

// 更新相关状态
const updateState = ref<UpdateState | null>(null)
const checking = ref(false)
const downloading = ref(false)
const installing = ref(false)
const downloadProgress = ref<number | null>(null)
const appVersion = ref('')
const updateModalOpen = ref(false)
const updateCheckInfo = ref<UpdateInfo | null>(null)
const updateModalMessage = ref('')
const updateModalError = ref('')

const updateModalLatestVersion = computed(() => {
  if (updateCheckInfo.value?.version) return updateCheckInfo.value.version
  if (updateState.value?.latest_known_version) return updateState.value.latest_known_version
  return appVersion.value || '—'
})

const updateModalReleaseNotes = computed(() => {
  const notes = updateCheckInfo.value?.release_notes?.trim()
  if (notes) return notes
  const cachedNotes = updateState.value?.latest_release_notes?.trim()
  if (cachedNotes) return cachedNotes
  return t('components.general.update.releaseNotesEmpty')
})

const canTriggerUpdateFromModal = computed(() => {
  if (updateState.value?.update_ready) return true
  return Boolean(updateCheckInfo.value?.available)
})

const updateModalActionText = computed(() => {
  if (installing.value) return t('components.general.update.installing')
  if (updateState.value?.update_ready) return t('components.general.update.installAndRestart')
  if (downloading.value) {
    const progress = Math.round(downloadProgress.value ?? updateState.value?.download_progress ?? 0)
    return t('components.general.update.downloading', { progress })
  }
  return t('components.general.update.updateNow')
})

// 拉黑配置相关状态
const blacklistEnabled = ref(true)  // 拉黑功能总开关
const blacklistThreshold = ref(5)
const healthBlacklistThreshold = ref(3)
const blacklistDurationSeconds = ref(1800)
const levelBlacklistEnabled = ref(false)
const blacklistLoading = ref(false)
const blacklistSaving = ref(false)

// 配置导入相关状态
const importStatus = ref<ConfigImportStatus | null>(null)
const importPath = ref('')
const importing = ref(false)
const importLoading = ref(true)

// WebDAV 同步相关状态
const webdavLoading = ref(true)
const webdavSaving = ref(false)
const webdavTesting = ref(false)
const webdavUploading = ref(false)
const webdavDownloading = ref(false)
const webdavEndpoint = ref('')
const webdavUsername = ref('')
const webdavPassword = ref('')
const webdavRemoteDir = ref('')
const webdavRemoteFile = ref('codeswitch-config.zip')
const webdavTimeoutSeconds = ref(20)
const webdavManageModalOpen = ref(false)
const modalViewportPaddingX = 24
const webdavManageModalPanelMaxWidthPx = 980
const webdavManageModalPanelWidth = `min(${webdavManageModalPanelMaxWidthPx}px, calc(100vw - ${modalViewportPaddingX * 2}px))`

type WebDAVUploadStage =
  | 'idle'
  | 'ready'
  | 'start'
  | 'ensure_dir'
  | 'exporting'
  | 'exported'
  | 'uploading'
  | 'done'
  | 'error'

type WebDAVUploadLogLevel = 'info' | 'warn' | 'error'
type WebDAVUploadLog = { ts: number; level: WebDAVUploadLogLevel; text: string }

const webdavUploadModalOpen = ref(false)
const webdavUploadPreviewLoading = ref(false)
const webdavUploadIncludes = ref<string[]>([])
const webdavUploadStage = ref<WebDAVUploadStage>('idle')
const webdavUploadMessage = ref('')
const webdavUploadRemoteURL = ref('')
const webdavUploadSent = ref(0)
const webdavUploadTotal = ref(0)
const webdavUploadBytes = ref(0)
const webdavUploadError = ref('')
const webdavUploadLogs = ref<WebDAVUploadLog[]>([])

type WebDAVDownloadStage =
  | 'idle'
  | 'ready'
  | 'start'
  | 'backup'
  | 'fetch_manifest'
  | 'downloading'
  | 'importing'
  | 'done'
  | 'error'

type WebDAVDownloadLogLevel = 'info' | 'warn' | 'error'
type WebDAVDownloadLog = { ts: number; level: WebDAVDownloadLogLevel; text: string }

const webdavDownloadModalOpen = ref(false)
const webdavDownloadStage = ref<WebDAVDownloadStage>('idle')
const webdavDownloadMessage = ref('')
const webdavDownloadRemoteURL = ref('')
const webdavDownloadCurrentFile = ref('')
const webdavDownloadDoneCount = ref(0)
const webdavDownloadTotalCount = ref(0)
const webdavDownloadSent = ref(0)
const webdavDownloadTotal = ref(0)
const webdavDownloadBytes = ref(0)
const webdavDownloadBackupPath = ref('')
const webdavDownloadErrorFile = ref('')
const webdavDownloadError = ref('')
const webdavDownloadLogs = ref<WebDAVDownloadLog[]>([])

let unsubscribeWebdavSync: (() => void) | null = null

// 模型价格弹窗
const modelPricingModalOpen = ref(false)
const heatmapDisplayModalOpen = ref(false)
const heatmapGranularityDraft = ref(heatmapGranularity.value)
const heatmapDisplayDraft = ref<HeatmapDisplaySettings>({
  ...initialHeatmapDisplaySettings,
})

const getHeatmapDisplaySettingsFromState = (): HeatmapDisplaySettings =>
  normalizeHeatmapDisplaySettings({
    dailyScaleFactor: heatmapDailyScaleFactor.value,
    dailyIntensityMode: heatmapDailyIntensityMode.value,
    intensityMetric: heatmapIntensityMetric.value,
    intensityStopL1: heatmapIntensityStopL1.value,
    intensityStopL2: heatmapIntensityStopL2.value,
    intensityStopL3: heatmapIntensityStopL3.value,
  })

const applyHeatmapDisplaySettingsToState = (settings: Partial<HeatmapDisplaySettings> | HeatmapDisplaySettings) => {
  const normalized = normalizeHeatmapDisplaySettings(settings)
  heatmapDailyScaleFactor.value = normalized.dailyScaleFactor
  heatmapDailyIntensityMode.value = normalized.dailyIntensityMode
  heatmapIntensityMetric.value = normalized.intensityMetric
  heatmapIntensityStopL1.value = normalized.intensityStopL1
  heatmapIntensityStopL2.value = normalized.intensityStopL2
  heatmapIntensityStopL3.value = normalized.intensityStopL3
}

const heatmapIntensityMetricLabel = computed(() => {
  switch (heatmapIntensityMetric.value) {
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
})

const heatmapDisplayModeLabel = computed(() =>
  heatmapDailyIntensityMode.value === 'daily_peak'
    ? t('components.general.heatmapDisplay.dailyIntensityModeDailyPeak')
    : t('components.general.heatmapDisplay.dailyIntensityModeHourlyScaled')
)

const heatmapDisplaySummary = computed(() =>
  t('components.general.heatmapDisplay.summary', {
    metric: heatmapIntensityMetricLabel.value,
    mode: heatmapDisplayModeLabel.value,
    scale: heatmapDailyScaleFactor.value,
    l1: heatmapIntensityStopL1.value,
    l2: heatmapIntensityStopL2.value,
    l3: heatmapIntensityStopL3.value,
  })
)

const openHeatmapDisplayModal = () => {
  heatmapGranularityDraft.value = heatmapGranularity.value
  heatmapDisplayDraft.value = {
    ...getHeatmapDisplaySettingsFromState(),
  }
  heatmapDisplayModalOpen.value = true
}

const closeHeatmapDisplayModal = () => {
  heatmapDisplayModalOpen.value = false
}

const resetHeatmapDisplayDraft = () => {
  heatmapDisplayDraft.value = {
    ...DEFAULT_HEATMAP_DISPLAY_SETTINGS,
  }
}

const applyHeatmapDisplayDraft = () => {
  heatmapGranularity.value = normalizeHeatmapGranularity(heatmapGranularityDraft.value)
  applyHeatmapDisplaySettingsToState(heatmapDisplayDraft.value)
  heatmapDisplayModalOpen.value = false
  void persistAppSettingsNow()
}

const formatBytes = (bytes?: number) => {
  const value = Number(bytes ?? 0)
  if (!Number.isFinite(value) || value < 0) return '—'
  if (value === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let current = value
  let idx = 0
  while (current >= 1024 && idx < units.length - 1) {
    current /= 1024
    idx++
  }
  const digits = idx === 0 ? 0 : current >= 10 ? 1 : 2
  return `${current.toFixed(digits)} ${units[idx]}`
}

const webdavUploadPercent = computed(() => {
  const stage = webdavUploadStage.value
  if (stage === 'done') return 100
  if (stage === 'error') return Math.max(0, Math.min(100, 100))
  if (stage === 'ensure_dir') return 10
  if (stage === 'exporting') return 25
  if (stage === 'exported') return 35
  if (stage === 'uploading') {
    const total = Number(webdavUploadTotal.value || 0)
    const sent = Number(webdavUploadSent.value || 0)
    if (total > 0) {
      const ratio = Math.max(0, Math.min(1, sent / total))
      return 35 + Math.round(ratio * 60)
    }
    return 60
  }
  if (stage === 'start') return 5
  if (stage === 'ready') return 0
  return 0
})

const webdavDownloadPercent = computed(() => {
  const stage = webdavDownloadStage.value
  if (stage === 'done') return 100
  if (stage === 'error') return Math.max(0, Math.min(100, 100))
  if (stage === 'backup') return 10
  if (stage === 'fetch_manifest') return 20
  if (stage === 'importing') return 95
  if (stage === 'downloading') {
    const totalCount = Number(webdavDownloadTotalCount.value || 0)
    const doneCount = Number(webdavDownloadDoneCount.value || 0)
    if (totalCount > 0) {
      const ratio = Math.max(0, Math.min(1, doneCount / totalCount))
      return 20 + Math.round(ratio * 70)
    }

    const total = Number(webdavDownloadTotal.value || 0)
    const sent = Number(webdavDownloadSent.value || 0)
    if (total > 0) {
      const ratio = Math.max(0, Math.min(1, sent / total))
      return 20 + Math.round(ratio * 70)
    }
    if (sent > 0) {
      // total 不可得时给一个“伪进度”，避免进度条完全不动
      const scale = 50 * 1024 * 1024 // 50MB
      const ratio = Math.max(0, Math.min(1, sent / scale))
      return 20 + Math.round(ratio * 70)
    }
    return 60
  }
  if (stage === 'start') return 5
  if (stage === 'ready') return 0
  return 0
})

const resetWebdavUploadModal = () => {
  webdavUploadPreviewLoading.value = false
  webdavUploadIncludes.value = []
  webdavUploadStage.value = 'ready'
  webdavUploadMessage.value = ''
  webdavUploadRemoteURL.value = ''
  webdavUploadSent.value = 0
  webdavUploadTotal.value = 0
  webdavUploadBytes.value = 0
  webdavUploadError.value = ''
  webdavUploadLogs.value = []
}

const appendWebdavUploadLog = (text: string, level: WebDAVUploadLogLevel = 'info', ts?: number) => {
  const trimmed = String(text ?? '').trim()
  if (!trimmed) return
  webdavUploadLogs.value.push({ ts: Number(ts ?? Date.now()), level, text: trimmed })
  if (webdavUploadLogs.value.length > 200) {
    webdavUploadLogs.value = webdavUploadLogs.value.slice(-200)
  }
}

const resetWebdavDownloadModal = () => {
  webdavDownloadStage.value = 'ready'
  webdavDownloadMessage.value = ''
  webdavDownloadRemoteURL.value = ''
  webdavDownloadCurrentFile.value = ''
  webdavDownloadDoneCount.value = 0
  webdavDownloadTotalCount.value = 0
  webdavDownloadSent.value = 0
  webdavDownloadTotal.value = 0
  webdavDownloadBytes.value = 0
  webdavDownloadBackupPath.value = ''
  webdavDownloadErrorFile.value = ''
  webdavDownloadError.value = ''
  webdavDownloadLogs.value = []
}

const appendWebdavDownloadLog = (text: string, level: WebDAVDownloadLogLevel = 'info', ts?: number) => {
  const trimmed = String(text ?? '').trim()
  if (!trimmed) return
  webdavDownloadLogs.value.push({ ts: Number(ts ?? Date.now()), level, text: trimmed })
  if (webdavDownloadLogs.value.length > 200) {
    webdavDownloadLogs.value = webdavDownloadLogs.value.slice(-200)
  }
}

const loadWebdavUploadPreview = async () => {
  webdavUploadPreviewLoading.value = true
  try {
    const preview = await previewWebDAVContent()
    webdavUploadIncludes.value = preview?.includes ?? []
  } catch (error) {
    console.error('failed to preview webdav content', error)
    webdavUploadIncludes.value = []
  } finally {
    webdavUploadPreviewLoading.value = false
  }
}

const openWebdavUploadModal = async () => {
  if (webdavLoading.value || webdavUploading.value || webdavDownloading.value) return
  webdavUploadModalOpen.value = true
  resetWebdavUploadModal()
  await loadWebdavUploadPreview()
}

const closeWebdavUploadModal = () => {
  if (webdavUploading.value) {
    showToast(t('components.general.webdav.uploading'), 'warning')
    return
  }
  webdavUploadModalOpen.value = false
}

const openWebdavDownloadModal = () => {
  if (webdavLoading.value || webdavUploading.value || webdavDownloading.value) return
  webdavDownloadModalOpen.value = true
  resetWebdavDownloadModal()
}

const closeWebdavDownloadModal = () => {
  if (webdavDownloading.value) {
    showToast(t('components.general.webdav.downloading'), 'warning')
    return
  }
  webdavDownloadModalOpen.value = false
}

const startWebdavUpload = async () => {
  if (webdavLoading.value || webdavUploading.value || webdavDownloading.value) return
  webdavUploading.value = true
  webdavUploadError.value = ''
  webdavUploadStage.value = 'start'
  webdavUploadMessage.value = t('components.general.webdav.uploading')
	  try {
	    const result = await syncToWebDAV(buildWebDAVConfig())
	    webdavUploadBytes.value = Number(result?.bytes ?? webdavUploadBytes.value)
	    webdavUploadRemoteURL.value = result?.remote_url ?? webdavUploadRemoteURL.value
	    if (Array.isArray(result?.includes) && result.includes.length > 0) {
	      webdavUploadIncludes.value = result.includes
	    }
	    if (result?.ok) {
	      webdavUploadStage.value = 'done'
	      if (typeof result?.message === 'string' && result.message.trim()) {
	        webdavUploadMessage.value = result.message.trim()
	      }
	    }
	    showToast(t('components.general.webdav.uploadOk'), 'success')
	  } catch (error) {
	    console.error('webdav upload failed', error)
	    webdavUploadStage.value = 'error'
    webdavUploadError.value = extractErrorMessage(error)
    appendWebdavUploadLog(webdavUploadError.value, 'error')
    showToast(t('components.general.webdav.uploadFailed') + ': ' + webdavUploadError.value, 'error')
  } finally {
    webdavUploading.value = false
  }
}

const startWebdavDownload = async () => {
  if (webdavLoading.value || webdavDownloading.value || webdavUploading.value) return
  webdavDownloading.value = true
  webdavDownloadErrorFile.value = ''
  webdavDownloadError.value = ''
  webdavDownloadStage.value = 'start'
  webdavDownloadMessage.value = t('components.general.webdav.downloading')
  try {
    const result = await loadFromWebDAV(buildWebDAVConfig())
    webdavDownloadBytes.value = Number(result?.bytes ?? webdavDownloadBytes.value)
    webdavDownloadRemoteURL.value = result?.remote_url ?? webdavDownloadRemoteURL.value
    webdavDownloadBackupPath.value = result?.backup_path ?? webdavDownloadBackupPath.value

    if (result?.ok) {
      webdavDownloadStage.value = 'done'
      if (typeof result?.message === 'string' && result.message.trim()) {
        webdavDownloadMessage.value = result.message.trim()
      }
      showToast(t('components.general.webdav.downloadOk'), 'success')
      await loadAppSettings()
      await loadBlacklistSettings()
      await loadImportStatus()
    } else {
      webdavDownloadStage.value = 'error'
      webdavDownloadError.value = String(result?.message || t('components.general.webdav.downloadFailed'))
      appendWebdavDownloadLog(webdavDownloadError.value, 'error')
      showToast(t('components.general.webdav.downloadFailed') + ': ' + webdavDownloadError.value, 'error')
    }
  } catch (error) {
    console.error('webdav download failed', error)
    webdavDownloadStage.value = 'error'
    webdavDownloadError.value = extractErrorMessage(error)
    appendWebdavDownloadLog(webdavDownloadError.value, 'error')
    showToast(t('components.general.webdav.downloadFailed') + ': ' + webdavDownloadError.value, 'error')
  } finally {
    webdavDownloading.value = false
  }
}

const handleWebdavSyncEvent = (event: { data: Record<string, any> }) => {
  const data = event?.data ?? {}
  const type = String(data?.type || '').trim()
  if (!type) return

  if (type === 'upload') {
    const stage = String(data?.stage || '').trim()
    if (stage) {
      webdavUploadStage.value = stage as WebDAVUploadStage
    }
    const logText = typeof data?.log === 'string' ? data.log.trim() : ''
    if (logText) {
      const rawLevel = typeof data?.log_level === 'string' ? data.log_level.trim().toLowerCase() : ''
      const level: WebDAVUploadLogLevel = rawLevel === 'error' ? 'error' : rawLevel === 'warn' ? 'warn' : 'info'
      const ts = typeof data?.timestamp === 'number' ? data.timestamp : Date.now()
      appendWebdavUploadLog(logText, level, ts)
    }
    if (typeof data?.message === 'string' && data.message.trim()) {
      webdavUploadMessage.value = data.message.trim()
    }
    if (typeof data?.remote_url === 'string' && data.remote_url.trim()) {
      webdavUploadRemoteURL.value = data.remote_url.trim()
    }
    if (typeof data?.sent === 'number') {
      webdavUploadSent.value = data.sent
    }
    if (typeof data?.total === 'number') {
      webdavUploadTotal.value = data.total
    }
    if (typeof data?.bytes === 'number') {
      webdavUploadBytes.value = data.bytes
    }
    if (Array.isArray(data?.includes)) {
      webdavUploadIncludes.value = data.includes
    }
    if (stage === 'error') {
      webdavUploadError.value = String(data?.message || webdavUploadError.value || '同步失败')
      appendWebdavUploadLog(webdavUploadError.value, 'error')
    }
    return
  }

  if (type !== 'download') return

  const stage = String(data?.stage || '').trim()
  if (stage) {
    webdavDownloadStage.value = stage as WebDAVDownloadStage
  }

  const logText = typeof data?.log === 'string' ? data.log.trim() : ''
  if (logText) {
    const rawLevel = typeof data?.log_level === 'string' ? data.log_level.trim().toLowerCase() : ''
    const level: WebDAVDownloadLogLevel = rawLevel === 'error' ? 'error' : rawLevel === 'warn' ? 'warn' : 'info'
    const ts = typeof data?.timestamp === 'number' ? data.timestamp : Date.now()
    appendWebdavDownloadLog(logText, level, ts)
  }
  if (typeof data?.message === 'string' && data.message.trim()) {
    webdavDownloadMessage.value = data.message.trim()
  }
  if (typeof data?.remote_url === 'string' && data.remote_url.trim()) {
    webdavDownloadRemoteURL.value = data.remote_url.trim()
  }
  if (typeof data?.backup_path === 'string' && data.backup_path.trim()) {
    webdavDownloadBackupPath.value = data.backup_path.trim()
  }
  if (typeof data?.file === 'string' && data.file.trim()) {
    webdavDownloadCurrentFile.value = data.file.trim()
  }
  if (typeof data?.count_done === 'number') {
    webdavDownloadDoneCount.value = data.count_done
  }
  if (typeof data?.count_total === 'number') {
    webdavDownloadTotalCount.value = data.count_total
  }
  if (typeof data?.sent === 'number') {
    webdavDownloadSent.value = data.sent
  }
  if (typeof data?.total === 'number') {
    webdavDownloadTotal.value = data.total
  }
  if (typeof data?.bytes === 'number') {
    webdavDownloadBytes.value = data.bytes
  }

  if (stage === 'error') {
    webdavDownloadErrorFile.value = String(data?.file || webdavDownloadCurrentFile.value || '').trim()
    webdavDownloadError.value = String(data?.message || webdavDownloadError.value || '同步失败')
    appendWebdavDownloadLog(webdavDownloadError.value, 'error')
  }
}

const goBack = async () => {
  await flushPendingPersist()
  await router.push('/')
}

const normalizeBudgetForecastMethod = (value: string) => {
  const trimmed = value?.trim()
  if (trimmed === 'cycle' || trimmed === '10m' || trimmed === '1h' || trimmed === 'yesterday' || trimmed === 'last24h') {
    return trimmed
  }
  return 'cycle'
}

const normalizeBudgetForecastDisplay = (value: string) => {
  const trimmed = value?.trim()
  if (trimmed === 'datetime' || trimmed === 'remaining') {
    return trimmed
  }
  return 'datetime'
}

const normalizeUpdateHistoryKeepCount = (value: number) => {
  if (!Number.isFinite(value)) {
    return defaultUpdateHistoryKeepCount
  }
  const normalized = Math.floor(value)
  if (normalized < minUpdateHistoryKeepCount) {
    return minUpdateHistoryKeepCount
  }
  if (normalized > maxUpdateHistoryKeepCount) {
    return maxUpdateHistoryKeepCount
  }
  return normalized
}

const normalizeMainWindowDestroyDelayInput = (value: number) => normalizeMainWindowDestroyDelaySeconds(value)

const loadAppSettings = async () => {
  settingsLoading.value = true
  try {
    const data = await fetchAppSettings()
    heatmapEnabled.value = data?.show_heatmap ?? true
    heatmapGranularity.value = normalizeHeatmapGranularity(data?.heatmap_granularity)
    applyHeatmapDisplaySettingsToState({
      dailyScaleFactor: data?.heatmap_daily_scale_factor,
      dailyIntensityMode: data?.heatmap_daily_intensity_mode,
      intensityMetric: data?.heatmap_intensity_metric,
      intensityStopL1: data?.heatmap_intensity_stop_l1,
      intensityStopL2: data?.heatmap_intensity_stop_l2,
      intensityStopL3: data?.heatmap_intensity_stop_l3,
    })
    homeTitleVisible.value = data?.show_home_title ?? true
    homeProviderTabs.value = normalizeHomeProviderTabs(data?.home_provider_tabs)
    budgetQuotaUsedAdjustments.value = normalizeBudgetQuotaAdjustments(
      data?.budget_quota_used_adjustments,
      {
        adjustment: data?.budget_used_adjustment,
        cycleEnabled: data?.budget_cycle_enabled,
        cycleMode: data?.budget_cycle_mode,
      },
    )
    budgetQuotaSettings.value = normalizeBudgetQuotaSettings(data?.budget_quota_settings, {
      total: data?.budget_total,
      cycleEnabled: data?.budget_cycle_enabled,
      cycleMode: data?.budget_cycle_mode,
      refreshTime: data?.budget_refresh_time,
      refreshWeekday: data?.budget_refresh_day,
      refreshMonthDay: data?.budget_refresh_month_day,
    })
    budgetForecastMethod.value = normalizeBudgetForecastMethod(data?.budget_forecast_method ?? 'cycle')
    budgetForecastDisplay.value = normalizeBudgetForecastDisplay(data?.budget_forecast_display ?? 'datetime')
    budgetShowCountdown.value = data?.budget_show_countdown ?? false
    budgetShowForecast.value = data?.budget_show_forecast ?? false
    budgetQuotaUsedAdjustmentsCodex.value = normalizeBudgetQuotaAdjustments(
      data?.budget_quota_used_adjustments_codex,
      {
        adjustment: data?.budget_used_adjustment_codex,
        cycleEnabled: data?.budget_cycle_enabled_codex,
        cycleMode: data?.budget_cycle_mode_codex,
      },
    )
    budgetQuotaSettingsCodex.value = normalizeBudgetQuotaSettings(data?.budget_quota_settings_codex, {
      total: data?.budget_total_codex,
      cycleEnabled: data?.budget_cycle_enabled_codex,
      cycleMode: data?.budget_cycle_mode_codex,
      refreshTime: data?.budget_refresh_time_codex,
      refreshWeekday: data?.budget_refresh_day_codex,
      refreshMonthDay: data?.budget_refresh_month_day_codex,
    })
    budgetForecastMethodCodex.value = normalizeBudgetForecastMethod(data?.budget_forecast_method_codex ?? 'cycle')
    budgetForecastDisplayCodex.value = normalizeBudgetForecastDisplay(data?.budget_forecast_display_codex ?? 'datetime')
    budgetShowCountdownCodex.value = data?.budget_show_countdown_codex ?? false
    budgetShowForecastCodex.value = data?.budget_show_forecast_codex ?? false
    autoStartEnabled.value = data?.auto_start ?? false
    mainWindowDestroyDelaySeconds.value = normalizeMainWindowDestroyDelayInput(
      Number(data?.main_window_destroy_delay_seconds ?? DEFAULT_MAIN_WINDOW_DESTROY_DELAY_SECONDS)
    )
    autoUpdateEnabled.value = data?.auto_update ?? true
    updateHistoryKeepCount.value = normalizeUpdateHistoryKeepCount(
      Number(data?.update_history_keep_count ?? defaultUpdateHistoryKeepCount)
    )
    autoConnectivityTestEnabled.value = data?.auto_connectivity_test ?? false
    providerQuotaAutoDisableEnabled.value = data?.provider_quota_auto_disable_enabled ?? false
    providerQuotaRecoveryIntervalSeconds.value = normalizeProviderQuotaRecoveryIntervalSeconds(
      data?.provider_quota_recovery_interval_seconds,
    )
    providerQuotaRecoveryNotifyEnabled.value = data?.provider_quota_recovery_notify_enabled ?? false
    switchNotifyEnabled.value = data?.enable_switch_notify ?? true
    roundRobinEnabled.value = data?.enable_round_robin ?? false
    claudeModelRoutingEnabled.value = data?.claude_model_routing_enabled ?? false
    claudeModelAggregationEnabled.value = claudeModelRoutingEnabled.value && (data?.claude_model_aggregation_enabled ?? false)
    claudeModelMetadataMergeStrategy.value = data?.claude_model_metadata_merge_strategy === 'conservative'
      ? 'conservative'
      : 'aggressive'
    claudeProxyAuthField.value = normalizeClaudeProxyAuthField(data?.claude_proxy_auth_field)
    preserveCodexOfficialAuthOnSwitch.value = data?.preserve_codex_official_auth_on_switch ?? false
    unifyCodexSessionHistory.value = data?.unify_codex_session_history ?? false
    unifyCodexMigrateExisting.value = data?.unify_codex_migrate_existing ?? false
    captureRequestLogPayloadEnabled.value = data?.capture_request_log_payload ?? false
    sanitizeRequestLogPayloadEnabled.value = data?.sanitize_request_log_payload ?? true
    syncBudgetQuotaCurrentUsedForPlatform('claude')
    syncBudgetQuotaCurrentUsedForPlatform('codex')

    // 缓存到 localStorage，下次打开时直接显示正确状态
    syncAppSettingsCache()
    void refreshAllBudgetQuotaUsage()
  } catch (error) {
    console.error('failed to load app settings', error)
    heatmapEnabled.value = true
    heatmapGranularity.value = 'daily'
    applyHeatmapDisplaySettingsToState(DEFAULT_HEATMAP_DISPLAY_SETTINGS)
    homeTitleVisible.value = true
    homeProviderTabs.value = normalizeHomeProviderTabs(null)
    budgetQuotaUsedAdjustments.value = createDefaultBudgetQuotaAdjustments()
    budgetQuotaSettings.value = createDefaultBudgetQuotaSettings()
    budgetQuotaTrackedUsage.value = createDefaultBudgetQuotaAdjustments()
    budgetQuotaCurrentUsed.value = createDefaultBudgetQuotaAdjustments()
    budgetQuotaUsageStatuses.value = createDefaultBudgetQuotaUsageStatuses()
    budgetForecastMethod.value = 'cycle'
    budgetForecastDisplay.value = 'datetime'
    budgetShowCountdown.value = false
    budgetShowForecast.value = false
    budgetQuotaUsedAdjustmentsCodex.value = createDefaultBudgetQuotaAdjustments()
    budgetQuotaSettingsCodex.value = createDefaultBudgetQuotaSettings()
    budgetQuotaTrackedUsageCodex.value = createDefaultBudgetQuotaAdjustments()
    budgetQuotaCurrentUsedCodex.value = createDefaultBudgetQuotaAdjustments()
    budgetQuotaUsageStatusesCodex.value = createDefaultBudgetQuotaUsageStatuses()
    budgetForecastMethodCodex.value = 'cycle'
    budgetForecastDisplayCodex.value = 'datetime'
    budgetShowCountdownCodex.value = false
    budgetShowForecastCodex.value = false
    autoStartEnabled.value = false
    mainWindowDestroyDelaySeconds.value = DEFAULT_MAIN_WINDOW_DESTROY_DELAY_SECONDS
    autoUpdateEnabled.value = true
    updateHistoryKeepCount.value = defaultUpdateHistoryKeepCount
    autoConnectivityTestEnabled.value = false
    providerQuotaAutoDisableEnabled.value = false
    providerQuotaRecoveryIntervalSeconds.value = DEFAULT_PROVIDER_QUOTA_RECOVERY_INTERVAL_SECONDS
    providerQuotaRecoveryNotifyEnabled.value = false
    switchNotifyEnabled.value = true
    roundRobinEnabled.value = false
    claudeModelRoutingEnabled.value = false
    claudeModelAggregationEnabled.value = false
    claudeModelMetadataMergeStrategy.value = 'aggressive'
    claudeProxyAuthField.value = 'auth_token'
    preserveCodexOfficialAuthOnSwitch.value = false
    unifyCodexSessionHistory.value = false
    unifyCodexMigrateExisting.value = false
    captureRequestLogPayloadEnabled.value = false
    sanitizeRequestLogPayloadEnabled.value = true
    syncAppSettingsCache()
  } finally {
    settingsLoading.value = false
  }
}

const waitForPersistIdle = () => {
  if (!saveBusy.value && !saveQueued) return Promise.resolve()
  return new Promise<void>((resolve) => {
    persistIdleWaiters.push(resolve)
  })
}

const resolvePersistIdleWaiters = () => {
  const waiters = persistIdleWaiters
  persistIdleWaiters = []
  waiters.forEach((resolve) => resolve())
}

const persistAppSettingsNow = async () => {
  if (persistTimer) {
    window.clearTimeout(persistTimer)
    persistTimer = undefined
  }
  if (settingsLoading.value) return
  if (saveBusy.value) {
    saveQueued = true
    return
  }
  saveBusy.value = true
  try {
    const latestSettings = await fetchAppSettings()
    const normalizedBudgetQuotaUsedAdjustments = normalizeBudgetQuotaAdjustments(budgetQuotaUsedAdjustments.value)
    budgetQuotaUsedAdjustments.value = cloneBudgetQuotaAdjustments(normalizedBudgetQuotaUsedAdjustments)
    const normalizedBudgetQuotaSettings = normalizeBudgetQuotaSettings(budgetQuotaSettings.value)
    budgetQuotaSettings.value = cloneBudgetQuotaSettings(normalizedBudgetQuotaSettings)
    const normalizedBudgetForecastMethod = normalizeBudgetForecastMethod(budgetForecastMethod.value)
    budgetForecastMethod.value = normalizedBudgetForecastMethod
    const normalizedBudgetForecastDisplay = normalizeBudgetForecastDisplay(budgetForecastDisplay.value)
    budgetForecastDisplay.value = normalizedBudgetForecastDisplay
    syncBudgetQuotaCurrentUsedForPlatform('claude')
    const normalizedBudgetQuotaUsedAdjustmentsCodex = normalizeBudgetQuotaAdjustments(budgetQuotaUsedAdjustmentsCodex.value)
    budgetQuotaUsedAdjustmentsCodex.value = cloneBudgetQuotaAdjustments(normalizedBudgetQuotaUsedAdjustmentsCodex)
    const normalizedBudgetQuotaSettingsCodex = normalizeBudgetQuotaSettings(budgetQuotaSettingsCodex.value)
    budgetQuotaSettingsCodex.value = cloneBudgetQuotaSettings(normalizedBudgetQuotaSettingsCodex)
    const normalizedBudgetForecastMethodCodex = normalizeBudgetForecastMethod(budgetForecastMethodCodex.value)
    budgetForecastMethodCodex.value = normalizedBudgetForecastMethodCodex
    const normalizedBudgetForecastDisplayCodex = normalizeBudgetForecastDisplay(budgetForecastDisplayCodex.value)
    budgetForecastDisplayCodex.value = normalizedBudgetForecastDisplayCodex
    syncBudgetQuotaCurrentUsedForPlatform('codex')
    const normalizedUpdateHistoryKeepCount = normalizeUpdateHistoryKeepCount(updateHistoryKeepCount.value)
    updateHistoryKeepCount.value = normalizedUpdateHistoryKeepCount
    const normalizedMainWindowDestroyDelaySeconds = normalizeMainWindowDestroyDelayInput(mainWindowDestroyDelaySeconds.value)
    mainWindowDestroyDelaySeconds.value = normalizedMainWindowDestroyDelaySeconds
    const providerQuotaRecoveryIntervalInput = `${providerQuotaRecoveryIntervalSeconds.value ?? ''}`.trim()
    const normalizedProviderQuotaRecoveryIntervalSeconds = normalizeProviderQuotaRecoveryIntervalSeconds(
      providerQuotaRecoveryIntervalInput === '' ? Number.NaN : Number(providerQuotaRecoveryIntervalInput),
    )
    providerQuotaRecoveryIntervalSeconds.value = normalizedProviderQuotaRecoveryIntervalSeconds
    const normalizedHeatmapDisplay = getHeatmapDisplaySettingsFromState()
    heatmapDailyScaleFactor.value = normalizedHeatmapDisplay.dailyScaleFactor
    heatmapDailyIntensityMode.value = normalizedHeatmapDisplay.dailyIntensityMode
    heatmapIntensityMetric.value = normalizedHeatmapDisplay.intensityMetric
    heatmapIntensityStopL1.value = normalizedHeatmapDisplay.intensityStopL1
    heatmapIntensityStopL2.value = normalizedHeatmapDisplay.intensityStopL2
    heatmapIntensityStopL3.value = normalizedHeatmapDisplay.intensityStopL3
    homeProviderTabs.value = normalizeHomeProviderTabs(homeProviderTabs.value)
    const legacyBudget = projectBudgetQuotaToLegacy(
      normalizedBudgetQuotaSettings,
      normalizedBudgetQuotaUsedAdjustments,
    )
    const legacyBudgetCodex = projectBudgetQuotaToLegacy(
      normalizedBudgetQuotaSettingsCodex,
      normalizedBudgetQuotaUsedAdjustmentsCodex,
    )

    const payload: AppSettings = {
      show_heatmap: heatmapEnabled.value,
      heatmap_granularity: heatmapGranularity.value,
      heatmap_daily_scale_factor: normalizedHeatmapDisplay.dailyScaleFactor,
      heatmap_daily_intensity_mode: normalizedHeatmapDisplay.dailyIntensityMode,
      heatmap_intensity_metric: normalizedHeatmapDisplay.intensityMetric,
      heatmap_intensity_stop_l1: normalizedHeatmapDisplay.intensityStopL1,
      heatmap_intensity_stop_l2: normalizedHeatmapDisplay.intensityStopL2,
      heatmap_intensity_stop_l3: normalizedHeatmapDisplay.intensityStopL3,
      show_home_title: homeTitleVisible.value,
      home_provider_tabs: homeProviderTabs.value,
      budget_total: legacyBudget.total,
      budget_used_adjustment: legacyBudget.adjustment,
      budget_quota_used_adjustments: normalizedBudgetQuotaUsedAdjustments,
      budget_forecast_method: normalizedBudgetForecastMethod,
      budget_forecast_display: normalizedBudgetForecastDisplay,
      budget_cycle_enabled: legacyBudget.cycleEnabled,
      budget_cycle_mode: legacyBudget.cycleMode,
      budget_refresh_time: legacyBudget.refreshTime,
      budget_refresh_day: legacyBudget.refreshWeekday,
      budget_refresh_month_day: legacyBudget.refreshMonthDay,
      budget_quota_settings: normalizedBudgetQuotaSettings,
      budget_show_countdown: budgetShowCountdown.value,
      budget_show_forecast: budgetShowForecast.value,
      budget_total_codex: legacyBudgetCodex.total,
      budget_used_adjustment_codex: legacyBudgetCodex.adjustment,
      budget_quota_used_adjustments_codex: normalizedBudgetQuotaUsedAdjustmentsCodex,
      budget_forecast_method_codex: normalizedBudgetForecastMethodCodex,
      budget_forecast_display_codex: normalizedBudgetForecastDisplayCodex,
      budget_cycle_enabled_codex: legacyBudgetCodex.cycleEnabled,
      budget_cycle_mode_codex: legacyBudgetCodex.cycleMode,
      budget_refresh_time_codex: legacyBudgetCodex.refreshTime,
      budget_refresh_day_codex: legacyBudgetCodex.refreshWeekday,
      budget_refresh_month_day_codex: legacyBudgetCodex.refreshMonthDay,
      budget_quota_settings_codex: normalizedBudgetQuotaSettingsCodex,
      budget_show_countdown_codex: budgetShowCountdownCodex.value,
      budget_show_forecast_codex: budgetShowForecastCodex.value,
      auto_start: autoStartEnabled.value,
      auto_update: autoUpdateEnabled.value,
      update_history_keep_count: normalizedUpdateHistoryKeepCount,
      logs_refresh_interval_seconds: latestSettings.logs_refresh_interval_seconds,
      main_window_destroy_delay_seconds: normalizedMainWindowDestroyDelaySeconds,
      auto_connectivity_test: autoConnectivityTestEnabled.value,
      provider_quota_auto_disable_enabled: providerQuotaAutoDisableEnabled.value,
      provider_quota_recovery_interval_seconds: normalizedProviderQuotaRecoveryIntervalSeconds,
      provider_quota_recovery_notify_enabled: providerQuotaRecoveryNotifyEnabled.value,
      enable_switch_notify: switchNotifyEnabled.value,
      enable_round_robin: roundRobinEnabled.value,
      claude_model_routing_enabled: claudeModelRoutingEnabled.value,
      claude_model_aggregation_enabled:
        claudeModelRoutingEnabled.value && claudeModelAggregationEnabled.value,
      claude_model_metadata_merge_strategy: claudeModelMetadataMergeStrategy.value,
      claude_proxy_auth_field: claudeProxyAuthField.value,
      preserve_codex_official_auth_on_switch: preserveCodexOfficialAuthOnSwitch.value,
      unify_codex_session_history: unifyCodexSessionHistory.value,
      unify_codex_migrate_existing: unifyCodexMigrateExisting.value,
      provider_concurrency_limits: latestSettings?.provider_concurrency_limits ?? {},
      provider_quota_query_preset_codes: latestSettings?.provider_quota_query_preset_codes ?? {},
      provider_quota_query_presets: latestSettings?.provider_quota_query_presets ?? {},
      capture_request_log_payload: captureRequestLogPayloadEnabled.value,
      sanitize_request_log_payload: sanitizeRequestLogPayloadEnabled.value,
    }
    await saveAppSettings(payload)

    try {
      await setAutoCheckEnabled(autoUpdateEnabled.value)
    } catch (error) {
      console.error('failed to sync auto update setting', error)
    }

    try {
      await Call.ByName(
        'codeswitch/services.HealthCheckService.SetAutoAvailabilityPolling',
        autoConnectivityTestEnabled.value
      )
    } catch (error) {
      console.error('failed to sync auto connectivity setting', error)
    }

    syncAppSettingsCache()

    window.dispatchEvent(new CustomEvent('app-settings-updated'))
  } catch (error) {
    console.error('failed to save app settings', error)
    showToast(t('components.general.label.settingsSaveFailed'), 'error')
    await loadAppSettings()
  } finally {
    saveBusy.value = false
    if (saveQueued) {
      saveQueued = false
      void persistAppSettingsNow()
    } else {
      resolvePersistIdleWaiters()
    }
  }
}

const persistAppSettings = () => {
  if (settingsLoading.value) return
  if (persistTimer) {
    window.clearTimeout(persistTimer)
  }
  persistTimer = window.setTimeout(() => {
    persistTimer = undefined
    void persistAppSettingsNow()
  }, persistDebounceMs)
}

const persistMainWindowDestroyDelay = async (event: Event) => {
  if (settingsLoading.value) return
  const rawValue = (event.target as HTMLInputElement | null)?.value.trim() ?? ''
  if (rawValue === '') return
  const seconds = normalizeMainWindowDestroyDelayInput(Number(rawValue))
  const requestSeq = ++mainWindowDestroyDelayRequestSeq
  mainWindowDestroyDelaySeconds.value = seconds
  try {
    const settings = await saveMainWindowDestroyDelay(seconds)
    if (requestSeq !== mainWindowDestroyDelayRequestSeq) return
    mainWindowDestroyDelaySeconds.value = settings.main_window_destroy_delay_seconds
    localStorage.setItem('app-settings-mainWindowDestroyDelaySeconds', String(mainWindowDestroyDelaySeconds.value))
  } catch (error) {
    if (requestSeq !== mainWindowDestroyDelayRequestSeq) return
    console.error('failed to save main window destroy delay', error)
    showToast(t('components.general.label.settingsSaveFailed'), 'error')
    await loadAppSettings()
  }
}

const flushPendingPersist = async () => {
  if (persistTimer) {
    window.clearTimeout(persistTimer)
    persistTimer = undefined
    await persistAppSettingsNow()
  }
  await waitForPersistIdle()
}

const claudeModelRefreshHint = computed(() => {
  const status = claudeModelRoutingStatus.value
  if (!status) return t('components.general.label.claudeModelRefreshIdle')
  if (status.refreshing) return t('components.general.label.claudeModelRefreshing')
  if (status.lastSuccessAt) {
    return t('components.general.label.claudeModelRefreshStatus', {
      time: formatLocalDateTime(new Date(status.lastSuccessAt)),
      success: status.successCount,
      failure: status.failureCount,
    })
  }
  return t('components.general.label.claudeModelRefreshIdle')
})

const loadClaudeModelRoutingStatus = async () => {
  try {
    claudeModelRoutingStatus.value = await getClaudeModelRoutingStatus()
  } catch (error) {
    console.error('failed to load Claude model routing status', error)
  }
}

const handleClaudeModelRoutingToggle = async () => {
  if (!claudeModelRoutingEnabled.value) {
    claudeModelAggregationEnabled.value = false
  }
  await persistAppSettingsNow()
  await loadClaudeModelRoutingStatus()
}

const handleClaudeModelAggregationToggle = async () => {
  if (!claudeModelRoutingEnabled.value) {
    claudeModelAggregationEnabled.value = false
  }
  await persistAppSettingsNow()
}

const handleClaudeModelStrategyChange = async () => {
  await persistAppSettingsNow()
}

const handleClaudeModelRefresh = async () => {
  if (!claudeModelRoutingEnabled.value || claudeModelRefreshBusy.value) return
  claudeModelRefreshBusy.value = true
  try {
    await persistAppSettingsNow()
    const result = await refreshClaudeModelRoutes()
    await loadClaudeModelRoutingStatus()
    const message = t('components.general.label.claudeModelRefreshResult', {
      success: result.successCount,
      failure: result.failureCount,
    })
    showToast(message, result.failureCount > 0 ? 'warning' : 'success')
  } catch (error) {
    showToast(t('components.general.label.claudeModelRefreshFailed', { error: extractErrorMessage(error) }), 'error')
  } finally {
    claudeModelRefreshBusy.value = false
  }
}

const handleCodexUnifyToggleChange = async (event: Event) => {
  const checked = (event.target as HTMLInputElement | null)?.checked === true
  if (checked) {
    unifyCodexMigrateExisting.value = false
    codexUnifyEnableConfirmOpen.value = true
    return
  }
  try {
    codexUnifyHasBackup.value = await hasCodexUnifiedHistoryBackup()
  } catch (error) {
    console.error('failed to check codex unified history backup', error)
    codexUnifyHasBackup.value = false
  }
  codexUnifyRestoreBackup.value = codexUnifyHasBackup.value
  codexUnifyDisableConfirmOpen.value = true
}

const cancelCodexUnifyEnable = () => {
  codexUnifyEnableConfirmOpen.value = false
  unifyCodexMigrateExisting.value = false
}

const confirmCodexUnifyEnable = async () => {
  unifyCodexSessionHistory.value = true
  codexUnifyEnableConfirmOpen.value = false
  await persistAppSettingsNow()
}

const cancelCodexUnifyDisable = () => {
  if (codexUnifyRestoreBusy.value) return
  codexUnifyDisableConfirmOpen.value = false
  codexUnifyRestoreBackup.value = codexUnifyHasBackup.value
}

const confirmCodexUnifyDisable = async () => {
  codexUnifyRestoreBusy.value = true
  try {
    const shouldRestore = codexUnifyRestoreBackup.value && codexUnifyHasBackup.value
    unifyCodexSessionHistory.value = false
    unifyCodexMigrateExisting.value = false
    await persistAppSettingsNow()
    if (shouldRestore) {
      const result = await restoreCodexUnifiedHistory()
      if (result.skipped_reason) {
        showToast(t('components.general.label.unifyCodexHistoryRestoreSkipped'), 'warning')
      } else {
        showToast(t('components.general.label.unifyCodexHistoryRestoreCompleted'), 'success')
      }
    }
    codexUnifyDisableConfirmOpen.value = false
  } catch (error) {
    console.error('failed to restore codex unified history', error)
    showToast(t('components.general.label.unifyCodexHistoryRestoreFailed'), 'error')
  } finally {
    codexUnifyRestoreBusy.value = false
  }
}

const loadUpdateState = async () => {
  try {
    updateState.value = await getUpdateState()
  } catch (error) {
    console.error('failed to load update state', error)
  }
}

const updateLocalDownloadProgress = (progress: number) => {
  if (!Number.isFinite(progress)) return
  const normalized = Math.max(0, Math.min(100, progress))
  downloadProgress.value = normalized
  if (updateState.value) {
    updateState.value = {
      ...updateState.value,
      download_progress: normalized,
    }
  }
}

const updateErrorLogPath = '.code-switch/updates/update-errors.log'

const isUacCancelledError = (message: string) => {
  const normalized = message.toLowerCase()
  return normalized.includes('err_uac_denied') ||
    normalized.includes('uac') && (normalized.includes('cancel') || normalized.includes('deny')) ||
    message.includes('用户取消') ||
    message.includes('取消 UAC') ||
    message.includes('拒绝 UAC')
}

const buildRestartInstallErrorMessage = (error: unknown) => {
  const message = extractErrorMessage(error)

  if (isUacCancelledError(message)) {
    return `你刚才把管理员权限确认（UAC）取消了，本次更新没安装。\n想更新的话，再点一次“安装并重启”就行。`
  }

  if (message.includes('应用更新失败') || message.includes('更新') || message.includes('安装')) {
    return `安装更新失败：${message}\n详细日志在 ${updateErrorLogPath}`
  }

  return `重启失败：${message}\n详细日志在 ${updateErrorLogPath}`
}

const alertRestartInstallError = (error: unknown) => {
  console.error('restart/install update failed', error)
  updateModalError.value = buildRestartInstallErrorMessage(error)
}

const checkUpdateManually = async () => {
  if (checking.value || downloading.value || installing.value) return

  updateModalOpen.value = true
  updateModalError.value = ''
  updateModalMessage.value = t('components.general.update.checking')
  updateCheckInfo.value = null

  checking.value = true
  try {
    const info = await checkUpdate()
    await loadUpdateState()

    if (!info) {
      updateModalError.value = t('components.general.update.checkEmptyResponse')
      return
    }

    updateCheckInfo.value = info

    if (!info.available) {
      updateModalMessage.value = t('components.general.update.alreadyLatest', {
        version: info.version || appVersion.value || '—',
      })
    } else {
      updateModalMessage.value = t('components.general.update.updateFound', { version: info.version })
    }
  } catch (error) {
    console.error('check update failed', error)
    updateModalError.value = t('components.general.update.checkFailedWithError', {
      error: extractErrorMessage(error),
    })
    await loadUpdateState()
  } finally {
    checking.value = false
  }
}

const downloadAndInstall = async () => {
  if (downloading.value || installing.value || checking.value) return

  updateModalError.value = ''
  updateModalMessage.value = ''

  if (updateState.value?.update_ready) {
    installing.value = true
    try {
      await restartApp()
    } catch (restartError) {
      alertRestartInstallError(restartError)
    } finally {
      installing.value = false
    }
    return
  }

  if (!updateCheckInfo.value?.available) {
    updateModalMessage.value = t('components.general.update.noUpdateToInstall')
    return
  }

  downloading.value = true
  downloadProgress.value = 0
  updateLocalDownloadProgress(0)
  try {
    await downloadUpdate((progress: number) => {
      updateLocalDownloadProgress(progress)
    })
    await loadUpdateState()
    updateLocalDownloadProgress(100)
    updateModalMessage.value = t('components.general.update.downloadReady')
  } catch (error) {
    console.error('download failed', error)
    updateModalError.value = t('components.general.update.downloadFailedWithError', {
      error: extractErrorMessage(error),
    })
    return
  } finally {
    downloading.value = false
    downloadProgress.value = null
  }
}

const formatLastCheckTime = (timeStr?: string) => {
  if (!timeStr) return '从未检查'

  const checkTime = new Date(timeStr)
  const now = new Date()
  const diffMs = now.getTime() - checkTime.getTime()
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60))

  if (diffHours < 1) {
    return '刚刚'
  } else if (diffHours < 24) {
    return `${diffHours} 小时前`
  } else {
    const diffDays = Math.floor(diffHours / 24)
    return `${diffDays} 天前`
  }
}

// 加载拉黑配置
const loadBlacklistSettings = async () => {
  blacklistLoading.value = true
  try {
    const settings = await getBlacklistSettings()
    blacklistThreshold.value = settings.failureThreshold
    blacklistDurationSeconds.value = settings.durationSeconds
    healthBlacklistThreshold.value = await getHealthBlacklistThreshold()

    // 加载拉黑功能总开关
    const enabled = await getBlacklistEnabled()
    blacklistEnabled.value = enabled

    // 加载等级拉黑开关状态
    const levelEnabled = await getLevelBlacklistEnabled()
    levelBlacklistEnabled.value = levelEnabled
  } catch (error) {
    console.error('failed to load blacklist settings', error)
    // 使用默认值
    blacklistEnabled.value = true
    blacklistThreshold.value = 5
    healthBlacklistThreshold.value = 3
    blacklistDurationSeconds.value = 1800
    levelBlacklistEnabled.value = false
  } finally {
    blacklistLoading.value = false
  }
}

// 保存拉黑配置
const saveBlacklistSettings = async () => {
  if (blacklistLoading.value || blacklistSaving.value) return
  blacklistSaving.value = true
  try {
    await updateBlacklistSettingsWithHealthThreshold(
      blacklistThreshold.value,
      blacklistDurationSeconds.value,
      healthBlacklistThreshold.value,
    )
    showToast(t('components.general.toast.blacklistSaveSuccess'), 'success')
  } catch (error) {
    console.error('failed to save blacklist settings', error)
    await loadBlacklistSettings()
    showToast(
      t('components.general.toast.blacklistSaveFailed', { error: extractErrorMessage(error) }),
      'error'
    )
  } finally {
    blacklistSaving.value = false
  }
}

// 切换拉黑功能总开关
const toggleBlacklist = async () => {
  if (blacklistLoading.value || blacklistSaving.value) return
  blacklistSaving.value = true
  try {
    await setBlacklistEnabled(blacklistEnabled.value)
  } catch (error) {
    console.error('failed to toggle blacklist', error)
    // 回滚状态
    blacklistEnabled.value = !blacklistEnabled.value
    alert('切换失败：' + (error as Error).message)
  } finally {
    blacklistSaving.value = false
  }
}

// 切换等级拉黑开关
const toggleLevelBlacklist = async () => {
  if (blacklistLoading.value || blacklistSaving.value) return
  blacklistSaving.value = true
  try {
    await setLevelBlacklistEnabled(levelBlacklistEnabled.value)
  } catch (error) {
    console.error('failed to toggle level blacklist', error)
    // 回滚状态
    levelBlacklistEnabled.value = !levelBlacklistEnabled.value
    alert('切换失败：' + (error as Error).message)
  } finally {
    blacklistSaving.value = false
  }
}

// 加载配置导入状态
const loadImportStatus = async () => {
  importLoading.value = true
  try {
    importStatus.value = await fetchConfigImportStatus()
    // 设置默认路径
    if (importStatus.value?.config_path) {
      importPath.value = importStatus.value.config_path
    }
  } catch (error) {
    console.error('failed to load import status', error)
    importStatus.value = null
  } finally {
    importLoading.value = false
  }
}

// 执行导入
const handleImport = async () => {
  if (importing.value || !importPath.value.trim()) return
  importing.value = true
  try {
    const result = await importFromPath(importPath.value.trim())
    // 无论结果如何，都更新状态
    importStatus.value = result.status
    if (result.status.config_path) {
      importPath.value = result.status.config_path
    }
    if (!result.status.config_exists) {
      alert(t('components.general.import.fileNotFound'))
      return
    }
    const imported = result.imported_providers + result.imported_mcp
    if (imported > 0) {
      alert(t('components.general.import.success', {
        providers: result.imported_providers,
        mcp: result.imported_mcp
      }))
    } else {
      alert(t('components.general.import.nothingToImport'))
    }
  } catch (error) {
    console.error('import failed', error)
    alert(t('components.general.import.failed') + ': ' + (error as Error).message)
  } finally {
    importing.value = false
  }
}

const normalizeWebDAVTimeoutSeconds = (value: unknown): number => {
  const n = Number(value)
  if (!Number.isFinite(n) || n <= 0) return 20
  const rounded = Math.round(n)
  if (rounded < 1) return 1
  if (rounded > 120) return 120
  return rounded
}

const buildWebDAVConfig = (): WebDAVSyncConfig => {
  return {
    endpoint: webdavEndpoint.value?.trim() ?? '',
    username: webdavUsername.value?.trim() ?? '',
    password: webdavPassword.value ?? '',
    remote_dir: webdavRemoteDir.value?.trim() ?? '',
    remote_file: (webdavRemoteFile.value?.trim() || 'codeswitch-config.zip'),
    timeout_seconds: normalizeWebDAVTimeoutSeconds(webdavTimeoutSeconds.value),
  }
}

const loadWebDAV = async () => {
  webdavLoading.value = true
  try {
    const cfg = await fetchWebDAVConfig()
    webdavEndpoint.value = cfg?.endpoint ?? ''
    webdavUsername.value = cfg?.username ?? ''
    webdavPassword.value = cfg?.password ?? ''
    webdavRemoteDir.value = cfg?.remote_dir ?? ''
    webdavRemoteFile.value = cfg?.remote_file ?? 'codeswitch-config.zip'
    webdavTimeoutSeconds.value = normalizeWebDAVTimeoutSeconds(cfg?.timeout_seconds ?? 20)
  } catch (error) {
    console.error('failed to load webdav config', error)
    webdavEndpoint.value = ''
    webdavUsername.value = ''
    webdavPassword.value = ''
    webdavRemoteDir.value = ''
    webdavRemoteFile.value = 'codeswitch-config.zip'
    webdavTimeoutSeconds.value = 20
  } finally {
    webdavLoading.value = false
  }
}

const saveWebDAV = async () => {
  if (webdavLoading.value || webdavSaving.value) return
  webdavSaving.value = true
  try {
    const saved = await saveWebDAVConfig(buildWebDAVConfig())
    webdavEndpoint.value = saved?.endpoint ?? webdavEndpoint.value
    webdavUsername.value = saved?.username ?? webdavUsername.value
    webdavPassword.value = saved?.password ?? webdavPassword.value
    webdavRemoteDir.value = saved?.remote_dir ?? webdavRemoteDir.value
    webdavRemoteFile.value = saved?.remote_file ?? webdavRemoteFile.value
    webdavTimeoutSeconds.value = normalizeWebDAVTimeoutSeconds(saved?.timeout_seconds ?? webdavTimeoutSeconds.value)
    showToast(t('components.general.toast.webdavSaveSuccess'), 'success')
  } catch (error) {
    console.error('failed to save webdav config', error)
    showToast(t('components.general.toast.webdavSaveFailed', { error: extractErrorMessage(error) }), 'error')
  } finally {
    webdavSaving.value = false
  }
}

const testWebDAV = async () => {
  if (webdavLoading.value || webdavTesting.value) return
  webdavTesting.value = true
  try {
    const result = await testWebDAVConfig(buildWebDAVConfig())
    if (result?.ok) {
      showToast(t('components.general.webdav.testOk'), 'success')
    } else {
      showToast(result?.message || t('components.general.webdav.testFailed'), 'error')
    }
  } catch (error) {
    console.error('webdav test failed', error)
    showToast(t('components.general.webdav.testFailed') + ': ' + extractErrorMessage(error), 'error')
  } finally {
    webdavTesting.value = false
  }
}

const openWebdavManageModal = () => {
  webdavManageModalOpen.value = true
}

const closeWebdavManageModal = () => {
  if (webdavUploadModalOpen.value || webdavDownloadModalOpen.value) return
  webdavManageModalOpen.value = false
}

const uploadToWebDAV = async () => {
  await openWebdavUploadModal()
}

const downloadFromWebDAV = () => {
  openWebdavDownloadModal()
}

onMounted(async () => {
  void preloadProviderDisplayIcons(HOME_PROVIDER_TAB_OPTIONS.map((tab) => tab.icon))

  await loadAppSettings()
  await loadClaudeModelRoutingStatus()

  // 加载当前版本号
  try {
    appVersion.value = await fetchCurrentVersion()
  } catch (error) {
    console.error('failed to load app version', error)
  }

  // 加载更新状态
  await loadUpdateState()

  // 加载拉黑配置
  await loadBlacklistSettings()

  // 加载导入状态
  await loadImportStatus()

  // 加载 WebDAV 配置
  await loadWebDAV()

  // WebDAV 同步进度事件
  unsubscribeWebdavSync = Events.On('webdav:sync', handleWebdavSyncEvent as Events.WailsEventCallback)
})

onBeforeUnmount(() => {
  void flushPendingPersist()
  if (unsubscribeWebdavSync) {
    unsubscribeWebdavSync()
    unsubscribeWebdavSync = null
  }
})
</script>

<template>
  <div class="main-shell general-shell">
    <div class="global-actions">
      <p class="global-eyebrow">{{ $t('components.general.title.application') }}</p>
      <button class="ghost-icon" :aria-label="$t('components.general.buttons.back')" @click="goBack">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M15 18l-6-6 6-6"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>
    </div>

    <div class="general-page">
      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.application') }}</h2>
        <div class="mac-panel">
          <ListItem :label="$t('components.general.label.heatmap')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="heatmapEnabled"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem :label="$t('components.general.label.heatmapGranularity')">
            <select
              v-model="heatmapGranularity"
              :disabled="settingsLoading || saveBusy || !heatmapEnabled"
              class="mac-select"
              @change="persistAppSettings">
              <option value="hourly">{{ $t('components.general.label.heatmapGranularityHourly') }}</option>
              <option value="daily">{{ $t('components.general.label.heatmapGranularityDaily') }}</option>
            </select>
          </ListItem>
          <ListItem :label="$t('components.general.label.heatmapDisplayConfig')">
            <div class="toggle-with-hint">
              <button
                type="button"
                class="action-btn"
                :disabled="settingsLoading || saveBusy || !heatmapEnabled"
                @click="openHeatmapDisplayModal"
              >
                {{ $t('components.general.heatmapDisplay.manage') }}
              </button>
              <span class="hint-text hint-text--single-line">{{ heatmapDisplaySummary }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.homeTitle')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="homeTitleVisible"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem :label="$t('components.general.label.homeProviderTabs')">
            <div class="home-provider-tabs-setting">
              <div class="home-provider-tabs-group">
                <span class="home-provider-tabs-group-label">{{ $t('components.general.label.homeProviderTabsVisible') }}</span>
                <div class="home-provider-tabs-options">
                  <label
                    v-for="tab in visibleHomeProviderTabOptions"
                    :key="tab.id"
                    class="provider-tab-option selected"
                    :class="{
                      'is-dragging': draggedHomeProviderTab === tab.id,
                      'is-drop-before': dragOverHomeProviderTab === tab.id && dragOverHomeProviderTabPosition === 'before',
                      'is-drop-after': dragOverHomeProviderTab === tab.id && dragOverHomeProviderTabPosition === 'after',
                    }"
                    :draggable="!settingsLoading && !saveBusy"
                    @dragstart="handleHomeProviderTabDragStart($event, tab.id)"
                    @dragover="handleHomeProviderTabDragOver($event, tab.id)"
                    @dragenter="handleHomeProviderTabDragOver($event, tab.id)"
                    @dragleave="handleHomeProviderTabDragLeave($event, tab.id)"
                    @drop.stop="handleHomeProviderTabDrop($event, tab.id)"
                    @dragend="handleHomeProviderTabDragEnd"
                  >
                    <span class="provider-tab-option-drag-handle" aria-hidden="true"></span>
                    <input
                      class="provider-tab-option-input"
                      type="checkbox"
                      :checked="true"
                      :disabled="settingsLoading || saveBusy || visibleHomeProviderTabOptions.length <= 1"
                      aria-keyshortcuts="Alt+ArrowLeft Alt+ArrowRight Alt+ArrowUp Alt+ArrowDown"
                      @keydown.alt.left.prevent="moveVisibleHomeProviderTab(tab.id, -1)"
                      @keydown.alt.right.prevent="moveVisibleHomeProviderTab(tab.id, 1)"
                      @keydown.alt.up.prevent="moveVisibleHomeProviderTab(tab.id, -1)"
                      @keydown.alt.down.prevent="moveVisibleHomeProviderTab(tab.id, 1)"
                      @change="toggleHomeProviderTab(tab.id, false)"
                    />
                    <span class="provider-tab-option-icon" aria-hidden="true">
                      <span
                        v-if="getHomeProviderTabIconSvg(tab.icon)"
                        class="provider-tab-option-svg"
                        v-html="getHomeProviderTabIconSvg(tab.icon)"
                      ></span>
                      <span v-else class="provider-tab-option-fallback">
                        {{ getHomeProviderTabInitials(tab.label) }}
                      </span>
                    </span>
                    <span>{{ tab.label }}</span>
                  </label>
                </div>
              </div>
              <div v-if="hiddenHomeProviderTabOptions.length" class="home-provider-tabs-group">
                <span class="home-provider-tabs-group-label">{{ $t('components.general.label.homeProviderTabsHidden') }}</span>
                <div class="home-provider-tabs-options">
                  <label
                    v-for="tab in hiddenHomeProviderTabOptions"
                    :key="tab.id"
                    class="provider-tab-option"
                  >
                    <input
                      class="provider-tab-option-input"
                      type="checkbox"
                      :checked="false"
                      :disabled="settingsLoading || saveBusy"
                      @change="toggleHomeProviderTab(tab.id, true)"
                    />
                    <span class="provider-tab-option-icon" aria-hidden="true">
                      <span
                        v-if="getHomeProviderTabIconSvg(tab.icon)"
                        class="provider-tab-option-svg"
                        v-html="getHomeProviderTabIconSvg(tab.icon)"
                      ></span>
                      <span v-else class="provider-tab-option-fallback">
                        {{ getHomeProviderTabInitials(tab.label) }}
                      </span>
                    </span>
                    <span>{{ tab.label }}</span>
                  </label>
                </div>
              </div>
              <span class="hint-text">{{ $t('components.general.label.homeProviderTabsHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.autoStart')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="autoStartEnabled"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem v-if="isMacPlatform" :label="$t('components.general.label.mainWindowDestroyDelay')">
            <div class="toggle-with-hint">
              <div class="budget-input">
                <input
                  v-model.number="mainWindowDestroyDelaySeconds"
                  type="number"
                  inputmode="numeric"
                  :min="MIN_MAIN_WINDOW_DESTROY_DELAY_SECONDS"
                  :max="MAX_MAIN_WINDOW_DESTROY_DELAY_SECONDS"
                  step="1"
                  :disabled="settingsLoading || saveBusy"
                  class="mac-input budget-input-field"
                  @input="persistMainWindowDestroyDelay"
                />
                <span class="budget-unit">{{ $t('components.general.label.seconds') }}</span>
              </div>
              <span class="hint-text">{{ $t('components.general.label.mainWindowDestroyDelayHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.switchNotify')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  type="checkbox"
                  :disabled="settingsLoading || saveBusy"
                  v-model="switchNotifyEnabled"
                  @change="persistAppSettings"
                />
                <span></span>
              </label>
              <span class="hint-text">{{ $t('components.general.label.switchNotifyHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.roundRobin')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  type="checkbox"
                  :disabled="settingsLoading || saveBusy"
                  v-model="roundRobinEnabled"
                  @change="persistAppSettings"
                />
                <span></span>
              </label>
              <span class="hint-text">{{ $t('components.general.label.roundRobinHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.claudeModelRouting')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  v-model="claudeModelRoutingEnabled"
                  type="checkbox"
                  :disabled="settingsLoading || saveBusy"
                  @change="handleClaudeModelRoutingToggle"
                />
                <span></span>
              </label>
              <span class="hint-text">{{ $t('components.general.label.claudeModelRoutingHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.claudeModelAggregation')">
            <div class="claude-model-setting">
              <div class="toggle-with-hint">
                <label class="mac-switch">
                  <input
                    v-model="claudeModelAggregationEnabled"
                    type="checkbox"
                    :disabled="settingsLoading || saveBusy || !claudeModelRoutingEnabled"
                    @change="handleClaudeModelAggregationToggle"
                  />
                  <span></span>
                </label>
                <span class="hint-text">{{ $t('components.general.label.claudeModelAggregationHint') }}</span>
              </div>
              <div v-if="claudeModelRoutingEnabled" class="claude-model-tools">
                <div v-if="claudeModelAggregationEnabled" class="metadata-strategy" role="group" :aria-label="$t('components.general.label.claudeModelMetadataStrategy')">
                  <button
                    v-for="strategy in claudeModelMetadataStrategies"
                    :key="strategy"
                    type="button"
                    :class="{ active: claudeModelMetadataMergeStrategy === strategy }"
                    :disabled="settingsLoading || saveBusy"
                    @click="claudeModelMetadataMergeStrategy = strategy; handleClaudeModelStrategyChange()"
                  >
                    {{ $t(`components.general.label.claudeModelMetadata${strategy === 'aggressive' ? 'Aggressive' : 'Conservative'}`) }}
                  </button>
                </div>
                <button
                  type="button"
                  class="claude-model-refresh"
                  :disabled="settingsLoading || saveBusy || claudeModelRefreshBusy"
                  :title="$t('components.general.label.claudeModelRefresh')"
                  @click="handleClaudeModelRefresh"
                >
                  <svg viewBox="0 0 20 20" aria-hidden="true">
                    <path d="M16 6V2.8M16 2.8h-3.2M16 2.8A7 7 0 104.4 15" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
                  </svg>
                  {{ $t('components.general.label.claudeModelRefresh') }}
                </button>
              </div>
              <span v-if="claudeModelRoutingEnabled" class="hint-text hint-text--single-line">{{ claudeModelRefreshHint }}</span>
            </div>
          </ListItem>
          <p class="panel-title">{{ $t('components.general.label.codexAuthSection') }}</p>
          <ListItem :label="$t('components.general.label.preserveCodexOfficialAuthOnSwitch')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  type="checkbox"
                  :disabled="settingsLoading || saveBusy"
                  v-model="preserveCodexOfficialAuthOnSwitch"
                  @change="persistAppSettings"
                />
                <span></span>
              </label>
              <span class="hint-text">{{ $t('components.general.label.preserveCodexOfficialAuthOnSwitchHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.unifyCodexSessionHistory')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  type="checkbox"
                  :disabled="settingsLoading || saveBusy || codexUnifyRestoreBusy"
                  :checked="unifyCodexSessionHistory"
                  @change="handleCodexUnifyToggleChange"
                />
                <span></span>
              </label>
              <span class="hint-text">{{ $t('components.general.label.unifyCodexSessionHistoryHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.captureRequestLogPayload')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  type="checkbox"
                  :disabled="settingsLoading || saveBusy"
                  v-model="captureRequestLogPayloadEnabled"
                  @change="persistAppSettings"
                />
                <span></span>
              </label>
              <span class="hint-text">{{ $t('components.general.label.captureRequestLogPayloadHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.sanitizeRequestLogPayload')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  type="checkbox"
                  :disabled="settingsLoading || saveBusy || !captureRequestLogPayloadEnabled"
                  v-model="sanitizeRequestLogPayloadEnabled"
                  @change="persistAppSettings"
                />
                <span></span>
              </label>
              <span class="hint-text">
                {{
                  captureRequestLogPayloadEnabled
                    ? $t('components.general.label.sanitizeRequestLogPayloadHint')
                    : $t('components.general.label.sanitizeRequestLogPayloadDisabledHint')
                }}
              </span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.title.webdavSync')">
            <button
              type="button"
              class="action-btn"
              :disabled="webdavLoading || webdavUploading || webdavDownloading"
              @click="openWebdavManageModal"
            >
              {{ $t('components.general.webdav.manage') }}
            </button>
          </ListItem>
        </div>
      </section>

      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.trayPanel') }}</h2>
        <div class="mac-panel">
          <p class="panel-title">{{ $t('components.general.label.trayPanelClaude') }}</p>
          <div class="budget-quota-grid">
            <div
              v-for="definition in budgetQuotaDefinitions"
              :key="`claude-${definition.key}`"
              class="budget-quota-card">
              <div class="budget-quota-card__header">
                <div class="budget-quota-card__heading">
                  <p class="budget-quota-card__title">{{ $t(definition.titleKey) }}</p>
                  <p class="budget-quota-card__hint">{{ $t(definition.hintKey) }}</p>
                </div>
                <span class="budget-quota-card__limit">
                  {{ formatBudgetLimitLabel(budgetQuotaSettings[definition.key].total) }}
                </span>
              </div>
              <div class="budget-quota-card__body">
                <div class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ $t('components.general.label.budgetTotal') }}</span>
                  <div class="budget-input">
                    <input
                      type="number"
                      inputmode="decimal"
                      min="0"
                      step="0.01"
                      :disabled="settingsLoading || saveBusy"
                      v-model.number="budgetQuotaSettings[definition.key].total"
                      @change="handleBudgetQuotaConfigChange('claude')"
                      class="mac-input budget-input-field"
                    />
                    <span class="budget-unit">USD</span>
                  </div>
                  <span class="budget-quota-field__hint">{{ $t('components.general.label.budgetQuotaUnsetHint') }}</span>
                </div>
                <div class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ $t('components.general.label.budgetQuotaUsedAdjustment') }}</span>
                  <div class="budget-input">
                    <input
                      type="number"
                      inputmode="decimal"
                      min="0"
                      step="any"
                      :disabled="settingsLoading || saveBusy || !isBudgetQuotaCurrentUsedEditable('claude', definition.key)"
                      v-model.number="budgetQuotaCurrentUsed[definition.key]"
                      @change="handleBudgetQuotaCurrentUsedChange('claude', definition.key)"
                      class="mac-input budget-input-field"
                    />
                    <span class="budget-unit">USD</span>
                  </div>
                  <span class="budget-quota-field__hint">{{ getBudgetQuotaCurrentUsedHint('claude', definition.key) }}</span>
                </div>
                <div v-if="definition.showWeekday" class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ $t('components.general.label.budgetRefreshWeekday') }}</span>
                  <select
                    v-model.number="budgetQuotaSettings[definition.key].refreshWeekday"
                    :disabled="settingsLoading || saveBusy"
                    class="mac-select budget-select"
                    @change="handleBudgetQuotaConfigChange('claude')">
                    <option
                      v-for="weekday in weekdayOptions"
                      :key="`claude-${definition.key}-${weekday.value}`"
                      :value="weekday.value">
                      {{ $t(weekday.labelKey) }}
                    </option>
                  </select>
                </div>
                <div v-if="definition.showMonthDay" class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ $t('components.general.label.budgetRefreshMonthDay') }}</span>
                  <select
                    v-model.number="budgetQuotaSettings[definition.key].refreshMonthDay"
                    :disabled="settingsLoading || saveBusy"
                    class="mac-select budget-select"
                    @change="handleBudgetQuotaConfigChange('claude')">
                    <option
                      v-for="day in monthDayOptions"
                      :key="`claude-${definition.key}-day-${day}`"
                      :value="day">
                      {{ day }}
                    </option>
                  </select>
                  <span class="budget-quota-field__hint">{{ $t('components.general.label.budgetRefreshMonthDayHint') }}</span>
                </div>
                <div v-if="definition.showTime" class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ $t('components.general.label.budgetRefreshTime') }}</span>
                  <input
                    type="time"
                    :disabled="settingsLoading || saveBusy"
                    v-model="budgetQuotaSettings[definition.key].refreshTime"
                    @change="handleBudgetQuotaConfigChange('claude')"
                    class="mac-input budget-time-input"
                  />
                </div>
              </div>
            </div>
          </div>
          <ListItem :label="$t('components.general.label.budgetShowCountdown')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="budgetShowCountdown"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetShowForecast')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="budgetShowForecast"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetForecastDisplay')">
            <div class="toggle-with-hint">
              <select
                v-model="budgetForecastDisplay"
                :disabled="settingsLoading || saveBusy || !budgetShowForecast"
                class="mac-select budget-select"
                @change="persistAppSettings">
                <option value="datetime">{{ $t('components.general.label.budgetForecastDisplayDatetime') }}</option>
                <option value="remaining">{{ $t('components.general.label.budgetForecastDisplayRemaining') }}</option>
              </select>
              <span class="hint-text">{{ $t('components.general.label.budgetForecastDisplayHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetForecastMethod')">
            <div class="toggle-with-hint">
              <select
                v-model="budgetForecastMethod"
                :disabled="settingsLoading || saveBusy || !budgetShowForecast"
                class="mac-select budget-select"
                @change="persistAppSettings">
                <option value="cycle">{{ $t('components.general.label.budgetForecastMethodCycle') }}</option>
                <option value="10m">{{ $t('components.general.label.budgetForecastMethod10m') }}</option>
                <option value="1h">{{ $t('components.general.label.budgetForecastMethod1h') }}</option>
                <option value="yesterday">{{ $t('components.general.label.budgetForecastMethodYesterday') }}</option>
                <option value="last24h">{{ $t('components.general.label.budgetForecastMethod24h') }}</option>
              </select>
              <span class="hint-text">{{ $t('components.general.label.budgetForecastMethodHint') }}</span>
            </div>
          </ListItem>
        </div>
        <div class="mac-panel">
          <p class="panel-title">{{ $t('components.general.label.trayPanelCodex') }}</p>
          <div class="budget-quota-grid">
            <div
              v-for="definition in budgetQuotaDefinitions"
              :key="`codex-${definition.key}`"
              class="budget-quota-card">
              <div class="budget-quota-card__header">
                <div class="budget-quota-card__heading">
                  <p class="budget-quota-card__title">{{ $t(definition.titleKey) }}</p>
                  <p class="budget-quota-card__hint">{{ $t(definition.hintKey) }}</p>
                </div>
                <span class="budget-quota-card__limit">
                  {{ formatBudgetLimitLabel(budgetQuotaSettingsCodex[definition.key].total) }}
                </span>
              </div>
              <div class="budget-quota-card__body">
                <div class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ $t('components.general.label.budgetTotal') }}</span>
                  <div class="budget-input">
                    <input
                      type="number"
                      inputmode="decimal"
                      min="0"
                      step="0.01"
                      :disabled="settingsLoading || saveBusy"
                      v-model.number="budgetQuotaSettingsCodex[definition.key].total"
                      @change="handleBudgetQuotaConfigChange('codex')"
                      class="mac-input budget-input-field"
                    />
                    <span class="budget-unit">USD</span>
                  </div>
                  <span class="budget-quota-field__hint">{{ $t('components.general.label.budgetQuotaUnsetHint') }}</span>
                </div>
                <div class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ $t('components.general.label.budgetQuotaUsedAdjustment') }}</span>
                  <div class="budget-input">
                    <input
                      type="number"
                      inputmode="decimal"
                      min="0"
                      step="any"
                      :disabled="settingsLoading || saveBusy || !isBudgetQuotaCurrentUsedEditable('codex', definition.key)"
                      v-model.number="budgetQuotaCurrentUsedCodex[definition.key]"
                      @change="handleBudgetQuotaCurrentUsedChange('codex', definition.key)"
                      class="mac-input budget-input-field"
                    />
                    <span class="budget-unit">USD</span>
                  </div>
                  <span class="budget-quota-field__hint">{{ getBudgetQuotaCurrentUsedHint('codex', definition.key) }}</span>
                </div>
                <div v-if="definition.showWeekday" class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ $t('components.general.label.budgetRefreshWeekday') }}</span>
                  <select
                    v-model.number="budgetQuotaSettingsCodex[definition.key].refreshWeekday"
                    :disabled="settingsLoading || saveBusy"
                    class="mac-select budget-select"
                    @change="handleBudgetQuotaConfigChange('codex')">
                    <option
                      v-for="weekday in weekdayOptions"
                      :key="`codex-${definition.key}-${weekday.value}`"
                      :value="weekday.value">
                      {{ $t(weekday.labelKey) }}
                    </option>
                  </select>
                </div>
                <div v-if="definition.showMonthDay" class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ $t('components.general.label.budgetRefreshMonthDay') }}</span>
                  <select
                    v-model.number="budgetQuotaSettingsCodex[definition.key].refreshMonthDay"
                    :disabled="settingsLoading || saveBusy"
                    class="mac-select budget-select"
                    @change="handleBudgetQuotaConfigChange('codex')">
                    <option
                      v-for="day in monthDayOptions"
                      :key="`codex-${definition.key}-day-${day}`"
                      :value="day">
                      {{ day }}
                    </option>
                  </select>
                  <span class="budget-quota-field__hint">{{ $t('components.general.label.budgetRefreshMonthDayHint') }}</span>
                </div>
                <div v-if="definition.showTime" class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ $t('components.general.label.budgetRefreshTime') }}</span>
                  <input
                    type="time"
                    :disabled="settingsLoading || saveBusy"
                    v-model="budgetQuotaSettingsCodex[definition.key].refreshTime"
                    @change="handleBudgetQuotaConfigChange('codex')"
                    class="mac-input budget-time-input"
                  />
                </div>
              </div>
            </div>
          </div>
          <ListItem :label="$t('components.general.label.budgetShowCountdown')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="budgetShowCountdownCodex"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetShowForecast')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="budgetShowForecastCodex"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetForecastDisplay')">
            <div class="toggle-with-hint">
              <select
                v-model="budgetForecastDisplayCodex"
                :disabled="settingsLoading || saveBusy || !budgetShowForecastCodex"
                class="mac-select budget-select"
                @change="persistAppSettings">
                <option value="datetime">{{ $t('components.general.label.budgetForecastDisplayDatetime') }}</option>
                <option value="remaining">{{ $t('components.general.label.budgetForecastDisplayRemaining') }}</option>
              </select>
              <span class="hint-text">{{ $t('components.general.label.budgetForecastDisplayHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetForecastMethod')">
            <div class="toggle-with-hint">
              <select
                v-model="budgetForecastMethodCodex"
                :disabled="settingsLoading || saveBusy || !budgetShowForecastCodex"
                class="mac-select budget-select"
                @change="persistAppSettings">
                <option value="cycle">{{ $t('components.general.label.budgetForecastMethodCycle') }}</option>
                <option value="10m">{{ $t('components.general.label.budgetForecastMethod10m') }}</option>
                <option value="1h">{{ $t('components.general.label.budgetForecastMethod1h') }}</option>
                <option value="yesterday">{{ $t('components.general.label.budgetForecastMethodYesterday') }}</option>
                <option value="last24h">{{ $t('components.general.label.budgetForecastMethod24h') }}</option>
              </select>
              <span class="hint-text">{{ $t('components.general.label.budgetForecastMethodHint') }}</span>
            </div>
          </ListItem>
        </div>
        <div class="mac-panel">
          <p class="panel-title">{{ $t('components.general.label.modelPricingPanel') }}</p>
          <ListItem :label="$t('components.general.label.modelPricing')">
            <div class="toggle-with-hint">
              <button type="button" class="action-btn" @click="modelPricingModalOpen = true">
                {{ $t('components.general.modelPricing.manage') }}
              </button>
              <span class="hint-text">{{ $t('components.general.label.modelPricingHint') }}</span>
            </div>
          </ListItem>
        </div>
      </section>

      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.connectivity') }}</h2>
        <div class="mac-panel">
          <ListItem :label="$t('components.general.label.claudeProxyAuthField')">
            <div class="claude-proxy-auth-setting">
              <div class="auth-field-selector" role="group" :aria-label="$t('components.general.label.claudeProxyAuthField')">
                <button
                  v-for="authField in claudeProxyAuthFields"
                  :key="authField"
                  type="button"
                  :class="{ active: claudeProxyAuthField === authField }"
                  :disabled="settingsLoading || saveBusy"
                  @click="claudeProxyAuthField = authField; persistAppSettings()"
                >
                  {{ $t(`components.general.label.claudeProxyAuth${authField === 'auth_token' ? 'Token' : 'APIKey'}`) }}
                </button>
              </div>
              <span class="hint-text">{{ $t('components.general.label.claudeProxyAuthFieldHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.autoConnectivityTest')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  type="checkbox"
                  :disabled="settingsLoading || saveBusy"
                  v-model="autoConnectivityTestEnabled"
                  @change="persistAppSettings"
                />
                <span></span>
              </label>
              <span class="hint-text">{{ $t('components.general.label.autoConnectivityTestHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.providerQuotaAutoDisable')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  v-model="providerQuotaAutoDisableEnabled"
                  type="checkbox"
                  :disabled="settingsLoading || saveBusy"
                  @change="persistAppSettings"
                />
                <span></span>
              </label>
              <span class="hint-text">{{ $t('components.general.label.providerQuotaAutoDisableHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.providerQuotaRecoveryInterval')">
            <div class="toggle-with-hint">
              <div class="budget-input">
                <input
                  v-model.number="providerQuotaRecoveryIntervalSeconds"
                  type="number"
                  inputmode="numeric"
                  :min="MIN_PROVIDER_QUOTA_RECOVERY_INTERVAL_SECONDS"
                  :max="MAX_PROVIDER_QUOTA_RECOVERY_INTERVAL_SECONDS"
                  step="1"
                  :disabled="settingsLoading || saveBusy || !providerQuotaAutoDisableEnabled"
                  class="mac-input budget-input-field"
                  @change="persistAppSettings"
                />
                <span class="budget-unit">{{ $t('components.general.label.seconds') }}</span>
              </div>
              <span class="hint-text">{{ $t('components.general.label.providerQuotaRecoveryIntervalHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.providerQuotaRecoveryNotify')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  v-model="providerQuotaRecoveryNotifyEnabled"
                  type="checkbox"
                  :disabled="settingsLoading || saveBusy || !providerQuotaAutoDisableEnabled"
                  @change="persistAppSettings"
                />
                <span></span>
              </label>
              <span class="hint-text">{{ $t('components.general.label.providerQuotaRecoveryNotifyHint') }}</span>
            </div>
          </ListItem>
        </div>
      </section>

      <!-- Network & WSL Settings -->
      <NetworkWslSettings />

      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.blacklist') }}</h2>
        <div class="mac-panel">
          <ListItem :label="$t('components.general.label.enableBlacklist')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  type="checkbox"
                  :disabled="blacklistLoading || blacklistSaving"
                  v-model="blacklistEnabled"
                  @change="toggleBlacklist"
                />
                <span></span>
              </label>
              <span class="hint-text">{{ $t('components.general.label.enableBlacklistHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.enableLevelBlacklist')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  type="checkbox"
                  :disabled="blacklistLoading || blacklistSaving"
                  v-model="levelBlacklistEnabled"
                  @change="toggleLevelBlacklist"
                />
                <span></span>
              </label>
              <span class="hint-text">{{ $t('components.general.label.enableLevelBlacklistHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.blacklistThreshold')">
            <div class="toggle-with-hint">
              <select
                v-model.number="blacklistThreshold"
                :disabled="blacklistLoading || blacklistSaving"
                class="mac-select">
                <option :value="1">1 {{ $t('components.general.label.times') }}</option>
                <option :value="2">2 {{ $t('components.general.label.times') }}</option>
                <option :value="3">3 {{ $t('components.general.label.times') }}</option>
                <option :value="4">4 {{ $t('components.general.label.times') }}</option>
                <option :value="5">5 {{ $t('components.general.label.times') }}</option>
                <option :value="6">6 {{ $t('components.general.label.times') }}</option>
                <option :value="7">7 {{ $t('components.general.label.times') }}</option>
                <option :value="8">8 {{ $t('components.general.label.times') }}</option>
                <option :value="9">9 {{ $t('components.general.label.times') }}</option>
              </select>
              <span class="hint-text">{{ $t('components.general.label.blacklistThresholdHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.healthBlacklistThreshold')">
            <div class="toggle-with-hint">
              <select
                v-model.number="healthBlacklistThreshold"
                :disabled="blacklistLoading || blacklistSaving"
                class="mac-select">
                <option v-for="count in 8" :key="count + 1" :value="count + 1">
                  {{ count + 1 }} {{ $t('components.general.label.times') }}
                </option>
              </select>
              <span class="hint-text">{{ $t('components.general.label.healthBlacklistThresholdHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.blacklistDuration')">
            <select
              v-model.number="blacklistDurationSeconds"
              :disabled="blacklistLoading || blacklistSaving"
              class="mac-select">
              <option :value="30">30 {{ $t('components.general.label.seconds') }}</option>
              <option :value="60">1 {{ $t('components.general.label.minutes') }}</option>
              <option :value="180">3 {{ $t('components.general.label.minutes') }}</option>
              <option :value="300">5 {{ $t('components.general.label.minutes') }}</option>
              <option :value="900">15 {{ $t('components.general.label.minutes') }}</option>
              <option :value="1800">30 {{ $t('components.general.label.minutes') }}</option>
              <option :value="3600">60 {{ $t('components.general.label.minutes') }}</option>
            </select>
          </ListItem>
          <ListItem :label="$t('components.general.label.saveBlacklist')">
            <button
              @click="saveBlacklistSettings"
              :disabled="blacklistLoading || blacklistSaving"
              class="primary-btn">
              {{ blacklistSaving ? $t('components.general.label.saving') : $t('components.general.label.save') }}
            </button>
          </ListItem>
        </div>
      </section>

      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.dataImport') }}</h2>
        <div class="mac-panel">
          <ListItem :label="$t('components.general.import.configPath')">
            <input
              type="text"
              v-model="importPath"
              :placeholder="$t('components.general.import.pathPlaceholder')"
              class="mac-input import-path-input"
            />
          </ListItem>
          <ListItem :label="$t('components.general.import.status')">
            <span class="info-text" v-if="importLoading">
              {{ $t('components.general.import.loading') }}
            </span>
            <span class="info-text" v-else-if="importStatus?.config_exists">
              {{ $t('components.general.import.configFound') }}
              <span v-if="importStatus.pending_provider_count > 0 || importStatus.pending_mcp_count > 0">
                ({{ $t('components.general.import.pendingCount', {
                  providers: importStatus.pending_provider_count,
                  mcp: importStatus.pending_mcp_count
                }) }})
              </span>
            </span>
            <span class="info-text warning" v-else-if="importStatus">
              {{ $t('components.general.import.configNotFound') }}
            </span>
          </ListItem>
          <ListItem :label="$t('components.general.import.action')">
            <button
              @click="handleImport"
              :disabled="importing || !importPath.trim()"
              class="action-btn">
              {{ importing ? $t('components.general.import.importing') : $t('components.general.import.importBtn') }}
            </button>
          </ListItem>
        </div>
      </section>

      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.exterior') }}</h2>
        <div class="mac-panel">
          <ListItem :label="$t('components.general.label.language')">
            <LanguageSwitcher />
          </ListItem>
          <ListItem :label="$t('components.general.label.theme')">
            <ThemeSetting />
          </ListItem>
        </div>
      </section>

      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.update') }}</h2>
        <div class="mac-panel">
          <ListItem :label="$t('components.general.label.autoUpdate')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="autoUpdateEnabled"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>

          <ListItem :label="$t('components.general.label.updateHistoryKeepCount')">
            <div class="toggle-with-hint">
              <div class="budget-input">
                <input
                  type="number"
                  inputmode="numeric"
                  min="1"
                  max="20"
                  step="1"
                  :disabled="settingsLoading || saveBusy"
                  v-model.number="updateHistoryKeepCount"
                  @change="persistAppSettings"
                  class="mac-input budget-input-field"
                />
              </div>
              <span class="hint-text">{{ $t('components.general.label.updateHistoryKeepCountHint') }}</span>
            </div>
          </ListItem>

          <ListItem :label="$t('components.general.label.lastCheck')">
            <span class="info-text">{{ formatLastCheckTime(updateState?.last_check_time) }}</span>
            <span v-if="updateState && updateState.consecutive_failures > 0" class="warning-badge">
              ⚠️ {{ $t('components.general.update.checkFailed', { count: updateState.consecutive_failures }) }}
            </span>
          </ListItem>

          <ListItem :label="$t('components.general.label.currentVersion')">
            <span class="version-text">{{ appVersion }}</span>
          </ListItem>

          <ListItem :label="$t('components.general.label.checkNow')">
            <button
              @click="checkUpdateManually"
              :disabled="checking || downloading || installing"
              class="action-btn">
              {{ checking ? $t('components.general.update.checking') : $t('components.general.update.checkNow') }}
            </button>
          </ListItem>
        </div>
      </section>

      <InlineModal
        :open="codexUnifyEnableConfirmOpen"
        :title="$t('components.general.label.unifyCodexHistoryEnableTitle')"
        variant="confirm"
        @close="cancelCodexUnifyEnable"
      >
        <div class="webdav-sync-modal">
          <p class="webdav-sync-hint">{{ $t('components.general.label.unifyCodexHistoryEnableMessage') }}</p>
          <label class="codex-unify-option">
            <input
              v-model="unifyCodexMigrateExisting"
              type="checkbox"
              :disabled="settingsLoading || saveBusy"
            />
            <span>{{ $t('components.general.label.unifyCodexHistoryMigrateExisting') }}</span>
          </label>
        </div>
        <footer class="webdav-sync-actions">
          <button class="action-btn" type="button" :disabled="saveBusy" @click="cancelCodexUnifyEnable">
            {{ $t('common.cancel') }}
          </button>
          <button class="primary-btn" type="button" :disabled="saveBusy" @click="confirmCodexUnifyEnable">
            {{ $t('common.save') }}
          </button>
        </footer>
      </InlineModal>

      <InlineModal
        :open="codexUnifyDisableConfirmOpen"
        :title="$t('components.general.label.unifyCodexHistoryDisableTitle')"
        variant="confirm"
        :close-disabled="codexUnifyRestoreBusy"
        @close="cancelCodexUnifyDisable"
      >
        <div class="webdav-sync-modal">
          <p class="webdav-sync-hint">{{ $t('components.general.label.unifyCodexHistoryDisableMessage') }}</p>
          <label v-if="codexUnifyHasBackup" class="codex-unify-option">
            <input
              v-model="codexUnifyRestoreBackup"
              type="checkbox"
              :disabled="settingsLoading || saveBusy || codexUnifyRestoreBusy"
            />
            <span>{{ $t('components.general.label.unifyCodexHistoryRestoreBackup') }}</span>
          </label>
          <p v-else class="info-text">{{ $t('components.general.label.unifyCodexHistoryRestoreSkipped') }}</p>
        </div>
        <footer class="webdav-sync-actions">
          <button
            class="action-btn"
            type="button"
            :disabled="codexUnifyRestoreBusy"
            @click="cancelCodexUnifyDisable"
          >
            {{ $t('common.cancel') }}
          </button>
          <button
            class="primary-btn"
            type="button"
            :disabled="saveBusy || codexUnifyRestoreBusy"
            @click="confirmCodexUnifyDisable"
          >
            {{ codexUnifyRestoreBusy ? $t('components.general.label.saving') : $t('common.save') }}
          </button>
        </footer>
      </InlineModal>

      <InlineModal
        :open="heatmapDisplayModalOpen"
        :title="$t('components.general.heatmapDisplay.title')"
        @close="closeHeatmapDisplayModal"
      >
        <div class="heatmap-display-modal">
          <p class="heatmap-display-hint">
            {{ $t('components.general.heatmapDisplay.hint') }}
          </p>

          <div class="heatmap-display-fields">
            <label class="heatmap-display-field">
              <span class="heatmap-display-label">{{ $t('components.general.label.heatmapGranularity') }}</span>
              <select v-model="heatmapGranularityDraft" class="mac-select heatmap-display-input">
                <option value="hourly">{{ $t('components.general.label.heatmapGranularityHourly') }}</option>
                <option value="daily">{{ $t('components.general.label.heatmapGranularityDaily') }}</option>
              </select>
            </label>

            <label class="heatmap-display-field">
              <span class="heatmap-display-label">{{ $t('components.general.heatmapDisplay.dailyIntensityMode') }}</span>
              <select v-model="heatmapDisplayDraft.dailyIntensityMode" class="mac-select heatmap-display-input">
                <option value="hourly_scaled">
                  {{ $t('components.general.heatmapDisplay.dailyIntensityModeHourlyScaled') }}
                </option>
                <option value="daily_peak">
                  {{ $t('components.general.heatmapDisplay.dailyIntensityModeDailyPeak') }}
                </option>
              </select>
            </label>

            <label class="heatmap-display-field">
              <span class="heatmap-display-label">{{ $t('components.general.heatmapDisplay.intensityMetric') }}</span>
              <select v-model="heatmapDisplayDraft.intensityMetric" class="mac-select heatmap-display-input">
                <option value="requests">{{ $t('components.general.heatmapDisplay.intensityMetricRequests') }}</option>
                <option value="cost">{{ $t('components.general.heatmapDisplay.intensityMetricCost') }}</option>
                <option value="total_tokens">{{ $t('components.general.heatmapDisplay.intensityMetricTotalTokens') }}</option>
                <option value="input_tokens">{{ $t('components.general.heatmapDisplay.intensityMetricInputTokens') }}</option>
                <option value="output_tokens">{{ $t('components.general.heatmapDisplay.intensityMetricOutputTokens') }}</option>
                <option value="reasoning_tokens">{{ $t('components.general.heatmapDisplay.intensityMetricReasoningTokens') }}</option>
              </select>
              <span class="heatmap-display-note">
                {{ $t('components.general.heatmapDisplay.intensityMetricHint') }}
              </span>
            </label>

            <label class="heatmap-display-field">
              <span class="heatmap-display-label">{{ $t('components.general.heatmapDisplay.dailyScaleFactor') }}</span>
              <input
                v-model.number="heatmapDisplayDraft.dailyScaleFactor"
                type="number"
                min="1"
                max="72"
                step="1"
                class="mac-input heatmap-display-input"
                :disabled="heatmapDisplayDraft.dailyIntensityMode !== 'hourly_scaled'"
              />
              <span class="heatmap-display-note">
                {{
                  heatmapDisplayDraft.dailyIntensityMode === 'hourly_scaled'
                    ? $t('components.general.heatmapDisplay.dailyScaleFactorHint')
                    : $t('components.general.heatmapDisplay.dailyScaleFactorDisabledHint')
                }}
              </span>
            </label>

            <label class="heatmap-display-field">
              <span class="heatmap-display-label">{{ $t('components.general.heatmapDisplay.intensityStopL1') }}</span>
              <input
                v-model.number="heatmapDisplayDraft.intensityStopL1"
                type="number"
                min="1"
                max="99"
                step="1"
                class="mac-input heatmap-display-input"
              />
            </label>

            <label class="heatmap-display-field">
              <span class="heatmap-display-label">{{ $t('components.general.heatmapDisplay.intensityStopL2') }}</span>
              <input
                v-model.number="heatmapDisplayDraft.intensityStopL2"
                type="number"
                min="1"
                max="99"
                step="1"
                class="mac-input heatmap-display-input"
              />
            </label>

            <label class="heatmap-display-field">
              <span class="heatmap-display-label">{{ $t('components.general.heatmapDisplay.intensityStopL3') }}</span>
              <input
                v-model.number="heatmapDisplayDraft.intensityStopL3"
                type="number"
                min="1"
                max="99"
                step="1"
                class="mac-input heatmap-display-input"
              />
            </label>
          </div>

          <p class="heatmap-display-note">
            {{ $t('components.general.heatmapDisplay.intensityStopsHint') }}
          </p>

          <footer class="heatmap-display-actions">
            <button class="action-btn" type="button" @click="resetHeatmapDisplayDraft">
              {{ $t('components.general.heatmapDisplay.reset') }}
            </button>
            <button class="action-btn" type="button" @click="closeHeatmapDisplayModal">
              {{ $t('common.cancel') }}
            </button>
            <button class="primary-btn" type="button" @click="applyHeatmapDisplayDraft">
              {{ $t('components.general.heatmapDisplay.apply') }}
            </button>
          </footer>
        </div>
      </InlineModal>

      <InlineModal
        :open="updateModalOpen"
        :title="$t('components.general.update.modalTitle')"
        @close="updateModalOpen = false"
      >
        <div class="update-modal">
          <div class="update-modal-row">
            <span class="update-modal-label">{{ $t('components.general.label.currentVersion') }}</span>
            <span class="version-text">{{ appVersion || '—' }}</span>
          </div>
          <div class="update-modal-row">
            <span class="update-modal-label">{{ $t('components.general.label.latestVersion') }}</span>
            <span class="version-text highlight">{{ updateModalLatestVersion }}</span>
          </div>

          <div v-if="updateModalMessage" class="info-text update-modal-message">
            {{ updateModalMessage }}
          </div>
          <div v-if="updateModalError" class="alert-error">
            {{ updateModalError }}
          </div>

          <div class="update-modal-block">
            <div class="update-modal-block-title">{{ $t('components.general.update.releaseNotes') }}</div>
            <pre class="update-modal-release-notes">{{ updateModalReleaseNotes }}</pre>
          </div>

          <footer class="update-modal-actions">
            <button
              class="action-btn"
              type="button"
              :disabled="downloading || checking || installing"
              @click="checkUpdateManually"
            >
              {{ $t('components.general.update.recheck') }}
            </button>
            <button
              class="action-btn"
              type="button"
              :disabled="downloading || checking || installing"
              @click="updateModalOpen = false"
            >
              {{ $t('common.cancel') }}
            </button>
            <button
              class="primary-btn"
              type="button"
              :disabled="checking || downloading || installing || !canTriggerUpdateFromModal"
              @click="downloadAndInstall"
            >
              {{ updateModalActionText }}
            </button>
          </footer>
        </div>
      </InlineModal>

      <InlineModal
        :open="webdavManageModalOpen"
        :title="$t('components.general.title.webdavSync')"
        :panel-width="webdavManageModalPanelWidth"
        @close="closeWebdavManageModal"
      >
        <div class="mac-panel webdav-manage-modal">
          <ListItem :label="$t('components.general.webdav.endpoint')">
            <input
              type="text"
              v-model="webdavEndpoint"
              :disabled="webdavLoading"
              :placeholder="$t('components.general.webdav.endpointPlaceholder')"
              class="mac-input webdav-input"
            />
          </ListItem>
          <ListItem :label="$t('components.general.webdav.username')">
            <input
              type="text"
              v-model="webdavUsername"
              :disabled="webdavLoading"
              :placeholder="$t('components.general.webdav.usernamePlaceholder')"
              class="mac-input webdav-input"
            />
          </ListItem>
          <ListItem :label="$t('components.general.webdav.password')">
            <input
              type="password"
              v-model="webdavPassword"
              :disabled="webdavLoading"
              :placeholder="$t('components.general.webdav.passwordPlaceholder')"
              class="mac-input webdav-input"
            />
          </ListItem>
          <ListItem :label="$t('components.general.webdav.remoteDir')">
            <input
              type="text"
              v-model="webdavRemoteDir"
              :disabled="webdavLoading"
              :placeholder="$t('components.general.webdav.remoteDirPlaceholder')"
              class="mac-input webdav-input"
            />
          </ListItem>
          <ListItem :label="$t('components.general.webdav.remoteFile')">
            <input
              type="text"
              v-model="webdavRemoteFile"
              :disabled="webdavLoading"
              :placeholder="$t('components.general.webdav.remoteFilePlaceholder')"
              class="mac-input webdav-input"
            />
          </ListItem>
          <ListItem :label="$t('components.general.webdav.timeoutSeconds')">
            <div class="toggle-with-hint">
              <input
                type="number"
                inputmode="numeric"
                min="1"
                max="120"
                step="1"
                v-model.number="webdavTimeoutSeconds"
                :disabled="webdavLoading"
                :placeholder="$t('components.general.webdav.timeoutPlaceholder')"
                class="mac-input webdav-timeout-input"
              />
              <span class="hint-text">{{ $t('components.general.webdav.timeoutHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.webdav.actions')" class="webdav-actions-row">
            <div class="webdav-actions">
              <button
                @click="saveWebDAV"
                :disabled="webdavLoading || webdavSaving"
                class="action-btn">
                {{ webdavSaving ? $t('components.general.label.saving') : $t('components.general.webdav.save') }}
              </button>
              <button
                @click="testWebDAV"
                :disabled="webdavLoading || webdavTesting || webdavUploading || webdavDownloading"
                class="action-btn">
                {{ webdavTesting ? $t('components.general.webdav.testing') : $t('components.general.webdav.test') }}
              </button>
              <button
                @click="uploadToWebDAV"
                :disabled="webdavLoading || webdavUploading || webdavDownloading"
                class="primary-btn">
                {{ webdavUploading ? $t('components.general.webdav.uploading') : $t('components.general.webdav.upload') }}
              </button>
              <button
                @click="downloadFromWebDAV"
                :disabled="webdavLoading || webdavDownloading || webdavUploading"
                class="action-btn">
                {{ webdavDownloading ? $t('components.general.webdav.downloading') : $t('components.general.webdav.download') }}
              </button>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.webdav.includes')">
            <span class="hint-text">{{ $t('components.general.webdav.includesHint') }}</span>
          </ListItem>
        </div>
      </InlineModal>

      <InlineModal
        :open="webdavUploadModalOpen"
        :title="$t('components.general.webdav.upload')"
        :close-on-backdrop="false"
        @close="closeWebdavUploadModal"
      >
        <div class="webdav-sync-modal">
          <p class="webdav-sync-hint">{{ $t('components.general.webdav.confirmUpload') }}</p>

          <div class="webdav-sync-block">
            <div class="webdav-sync-block-title">
              {{ $t('components.general.webdav.includes') }}
              <span v-if="webdavUploadIncludes.length" class="webdav-sync-count">
                ({{ webdavUploadIncludes.length }})
              </span>
            </div>
            <div v-if="webdavUploadPreviewLoading" class="info-text">
              {{ $t('components.general.import.loading') }}
            </div>
            <div v-else-if="!webdavUploadIncludes.length" class="info-text">
              —
            </div>
            <ul v-else class="webdav-sync-includes">
              <li v-for="item in webdavUploadIncludes" :key="item">{{ item }}</li>
            </ul>
          </div>

          <div class="webdav-sync-block">
            <div class="webdav-sync-block-title">{{ $t('components.general.webdav.progress') }}</div>

            <div class="webdav-sync-progress-row">
              <span class="webdav-sync-stage">{{ webdavUploadMessage || webdavUploadStage }}</span>
              <span class="webdav-sync-percent">{{ webdavUploadPercent }}%</span>
            </div>
            <div
              class="webdav-progress-bar"
              role="progressbar"
              :aria-valuenow="webdavUploadPercent"
              aria-valuemin="0"
              aria-valuemax="100"
            >
              <div class="webdav-progress-fill" :style="{ width: webdavUploadPercent + '%' }"></div>
            </div>
            <div v-if="webdavUploadStage === 'uploading' && webdavUploadTotal > 0" class="webdav-sync-meta">
              {{ formatBytes(webdavUploadSent) }} / {{ formatBytes(webdavUploadTotal) }}
            </div>
            <div v-else-if="webdavUploadStage === 'done' && webdavUploadBytes > 0" class="webdav-sync-meta">
              {{ formatBytes(webdavUploadBytes) }}
            </div>

            <p v-if="webdavUploadRemoteURL" class="webdav-sync-remote">
              {{ $t('components.general.webdav.remoteUrl') }}：<span class="webdav-sync-remote-url">{{ webdavUploadRemoteURL }}</span>
            </p>
            <p v-if="webdavUploadError" class="alert-error">{{ webdavUploadError }}</p>
            <div v-if="webdavUploadLogs.length" class="webdav-sync-logs">
              <p
                v-for="(item, idx) in webdavUploadLogs"
                :key="`${item.ts}-${idx}`"
                class="webdav-sync-log"
                :class="{ 'is-error': item.level === 'error' }"
              >
                {{ item.text }}
              </p>
            </div>
          </div>
        </div>

        <footer class="webdav-sync-actions">
          <button
            v-if="!webdavUploading && (webdavUploadStage === 'ready' || webdavUploadStage === 'idle')"
            class="action-btn"
            type="button"
            @click="closeWebdavUploadModal"
          >
            {{ $t('common.cancel') }}
          </button>

          <button
            v-if="!webdavUploading && (webdavUploadStage === 'ready' || webdavUploadStage === 'idle')"
            class="primary-btn"
            type="button"
            @click="startWebdavUpload"
          >
            {{ $t('components.general.webdav.upload') }}
          </button>

          <button v-else-if="webdavUploading" class="primary-btn" type="button" disabled>
            {{ $t('components.general.webdav.uploading') }}
          </button>

          <button v-else class="action-btn" type="button" @click="closeWebdavUploadModal">
            {{ $t('common.close') }}
          </button>
        </footer>
      </InlineModal>

      <InlineModal
        :open="webdavDownloadModalOpen"
        :title="$t('components.general.webdav.download')"
        :close-on-backdrop="false"
        @close="closeWebdavDownloadModal"
      >
        <div class="webdav-sync-modal">
          <p class="webdav-sync-hint">{{ $t('components.general.webdav.confirmDownload') }}</p>

          <div class="webdav-sync-block">
            <div class="webdav-sync-block-title">{{ $t('components.general.webdav.progress') }}</div>

            <div class="webdav-sync-progress-row">
              <span class="webdav-sync-stage">{{ webdavDownloadMessage || webdavDownloadStage }}</span>
              <span class="webdav-sync-percent">{{ webdavDownloadPercent }}%</span>
            </div>
            <div
              class="webdav-progress-bar"
              role="progressbar"
              :aria-valuenow="webdavDownloadPercent"
              aria-valuemin="0"
              aria-valuemax="100"
            >
              <div class="webdav-progress-fill" :style="{ width: webdavDownloadPercent + '%' }"></div>
            </div>

            <div v-if="webdavDownloadStage === 'downloading' && webdavDownloadTotalCount > 0" class="webdav-sync-meta">
              {{ webdavDownloadDoneCount }} / {{ webdavDownloadTotalCount }}
            </div>
            <div v-else-if="webdavDownloadStage === 'downloading' && webdavDownloadTotal > 0" class="webdav-sync-meta">
              {{ formatBytes(webdavDownloadSent) }} / {{ formatBytes(webdavDownloadTotal) }}
            </div>
            <div v-else-if="webdavDownloadStage === 'downloading' && webdavDownloadSent > 0" class="webdav-sync-meta">
              {{ formatBytes(webdavDownloadSent) }}
            </div>
            <div v-else-if="webdavDownloadStage === 'done' && webdavDownloadBytes > 0" class="webdav-sync-meta">
              {{ formatBytes(webdavDownloadBytes) }}
            </div>

            <p v-if="webdavDownloadCurrentFile" class="webdav-sync-remote">
              {{ $t('components.general.webdav.currentFile') }}：<span class="webdav-sync-remote-url">{{ webdavDownloadCurrentFile }}</span>
            </p>
            <p v-if="webdavDownloadRemoteURL" class="webdav-sync-remote">
              {{ $t('components.general.webdav.remoteUrl') }}：<span class="webdav-sync-remote-url">{{ webdavDownloadRemoteURL }}</span>
            </p>
            <p v-if="webdavDownloadBackupPath" class="webdav-sync-remote">
              {{ $t('components.general.webdav.backupPath') }}：<span class="webdav-sync-remote-url">{{ webdavDownloadBackupPath }}</span>
            </p>
            <p v-if="webdavDownloadError" class="alert-error">
              <span v-if="webdavDownloadErrorFile">
                {{ $t('components.general.webdav.errorFile') }}：{{ webdavDownloadErrorFile }}<br />
              </span>
              {{ webdavDownloadError }}
            </p>
            <div v-if="webdavDownloadLogs.length" class="webdav-sync-logs">
              <p
                v-for="(item, idx) in webdavDownloadLogs"
                :key="`${item.ts}-${idx}`"
                class="webdav-sync-log"
                :class="{ 'is-error': item.level === 'error' }"
              >
                {{ item.text }}
              </p>
            </div>
          </div>
        </div>

        <footer class="webdav-sync-actions">
          <button
            v-if="!webdavDownloading && (webdavDownloadStage === 'ready' || webdavDownloadStage === 'idle')"
            class="action-btn"
            type="button"
            @click="closeWebdavDownloadModal"
          >
            {{ $t('common.cancel') }}
          </button>

          <button
            v-if="!webdavDownloading && (webdavDownloadStage === 'ready' || webdavDownloadStage === 'idle')"
            class="primary-btn"
            type="button"
            @click="startWebdavDownload"
          >
            {{ $t('components.general.webdav.download') }}
          </button>

          <button v-else-if="webdavDownloading" class="primary-btn" type="button" disabled>
            {{ $t('components.general.webdav.downloading') }}
          </button>

          <button v-else class="action-btn" type="button" @click="closeWebdavDownloadModal">
            {{ $t('common.close') }}
          </button>
        </footer>
      </InlineModal>

      <ModelPricingModal :open="modelPricingModalOpen" @close="modelPricingModalOpen = false" />
    </div>
  </div>
</template>

<style scoped>
.mac-input {
  padding: 6px 12px;
  border: 1px solid var(--mac-border);
  border-radius: 6px;
  background: var(--mac-surface);
  color: var(--mac-text);
  font-size: 13px;
  font-family: monospace;
  min-width: 160px;
  transition: border-color 0.2s;
}

.mac-input:focus {
  outline: none;
  border-color: var(--mac-accent);
}

.panel-title {
  margin: 0;
  padding: 12px 18px 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--mac-text-secondary);
  letter-spacing: 0.02em;
  border-bottom: 1px solid var(--mac-divider);
}

.mac-panel + .mac-panel {
  margin-top: 12px;
}

.toggle-with-hint {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
}

.claude-model-setting {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 7px;
  min-width: min(100%, 360px);
}

.claude-model-tools {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
}

.metadata-strategy {
  display: grid;
  grid-template-columns: repeat(2, minmax(54px, 1fr));
  min-width: 122px;
  padding: 2px;
  border: 1px solid var(--mac-border);
  border-radius: 8px;
  background: var(--mac-surface);
}

.metadata-strategy button {
  min-height: 26px;
  padding: 0 9px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--mac-text-secondary);
  cursor: pointer;
  font-size: 11px;
  font-weight: 600;
}

.metadata-strategy button.active {
  background: var(--mac-accent);
  color: #fff;
}

.metadata-strategy button:focus-visible,
.claude-model-refresh:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--mac-accent) 42%, transparent);
  outline-offset: 2px;
}

.claude-model-refresh {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  min-height: 30px;
  padding: 0 10px;
  border: 1px solid var(--mac-border);
  border-radius: 8px;
  background: var(--mac-surface);
  color: var(--mac-text);
  cursor: pointer;
  font-size: 11px;
  font-weight: 600;
}

.claude-model-refresh svg {
  width: 14px;
  height: 14px;
}

.claude-model-refresh:hover:not(:disabled) {
  border-color: color-mix(in srgb, var(--mac-accent) 45%, var(--mac-border));
  color: var(--mac-accent);
}

.claude-model-refresh:disabled,
.metadata-strategy button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.claude-proxy-auth-setting {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 7px;
  min-width: min(100%, 330px);
}

.auth-field-selector {
  display: grid;
  grid-template-columns: repeat(2, minmax(118px, 1fr));
  padding: 2px;
  border: 1px solid var(--mac-border);
  border-radius: 8px;
  background: var(--mac-surface);
}

.auth-field-selector button {
  min-height: 28px;
  padding: 0 10px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--mac-text-secondary);
  cursor: pointer;
  font-size: 11px;
  font-weight: 600;
}

.auth-field-selector button.active {
  background: var(--mac-accent);
  color: #fff;
}

.auth-field-selector button:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--mac-accent) 42%, transparent);
  outline-offset: 2px;
}

.auth-field-selector button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.hint-text {
  font-size: 11px;
  color: var(--mac-text-secondary);
  line-height: 1.4;
  max-width: 320px;
  text-align: right;
  white-space: normal;
  overflow-wrap: anywhere;
}

.hint-text--single-line {
  max-width: none;
  white-space: nowrap;
  overflow-wrap: normal;
}

.home-provider-tabs-setting {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 6px;
  width: min(520px, 100%);
}

.home-provider-tabs-group {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
  width: 100%;
}

.home-provider-tabs-group-label {
  color: var(--mac-text-secondary);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0;
  text-transform: uppercase;
}

.home-provider-tabs-options {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
  width: 100%;
}

.provider-tab-option {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-height: 34px;
  padding: 7px 11px;
  border: 1px solid var(--mac-border);
  border-radius: 9px;
  background: var(--mac-surface);
  color: var(--mac-text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  position: relative;
  transition: border-color 0.2s ease, background 0.2s ease, color 0.2s ease, opacity 0.2s ease;
}

.provider-tab-option[draggable='true'] {
  cursor: grab;
}

.provider-tab-option[draggable='true']:active {
  cursor: grabbing;
}

.provider-tab-option.is-dragging {
  opacity: 0.5;
}

.provider-tab-option.is-drop-before::before,
.provider-tab-option.is-drop-after::after {
  content: '';
  position: absolute;
  top: 4px;
  bottom: 4px;
  width: 3px;
  border-radius: 999px;
  background: var(--mac-accent);
  box-shadow: 0 0 8px color-mix(in srgb, var(--mac-accent) 55%, transparent);
}

.provider-tab-option.is-drop-before::before {
  left: -6px;
}

.provider-tab-option.is-drop-after::after {
  right: -6px;
}

.provider-tab-option-drag-handle {
  width: 10px;
  height: 14px;
  flex-shrink: 0;
  opacity: 0.55;
  background-image: radial-gradient(circle, currentColor 1px, transparent 1.5px);
  background-position: 0 1px;
  background-size: 5px 5px;
}

.provider-tab-option:hover {
  border-color: var(--mac-accent);
  color: var(--mac-text);
}

.provider-tab-option:focus-within {
  outline: 2px solid color-mix(in srgb, var(--mac-accent) 48%, transparent);
  outline-offset: 2px;
}

.provider-tab-option.selected {
  border-color: rgba(59, 130, 246, 0.6);
  background: rgba(59, 130, 246, 0.12);
  color: var(--mac-accent);
}

.provider-tab-option-input {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: -1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.provider-tab-option-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  color: currentColor;
}

.provider-tab-option-svg,
.provider-tab-option-svg :deep(svg) {
  display: block;
  width: 16px;
  height: 16px;
}

.provider-tab-option-fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 5px;
  background: rgba(148, 163, 184, 0.16);
  font-size: 8px;
  font-weight: 700;
}

.provider-tab-option:has(input:disabled) {
  cursor: not-allowed;
  opacity: 0.6;
}

.provider-tab-option[draggable='true']:has(input:disabled) {
  cursor: grab;
  opacity: 1;
}

:global(.dark) .hint-text {
  color: rgba(255, 255, 255, 0.5);
}

:global(.dark) .provider-tab-option {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.1);
}

:global(.dark) .provider-tab-option.selected {
  background: rgba(96, 165, 250, 0.16);
  border-color: rgba(96, 165, 250, 0.46);
}

@media (max-width: 760px) {
  .home-provider-tabs-setting,
  .home-provider-tabs-group {
    align-items: flex-start;
  }

  .home-provider-tabs-options {
    justify-content: flex-start;
  }

  .home-provider-tabs-setting .hint-text {
    text-align: left;
  }
}

@media (prefers-reduced-motion: reduce) {
  .provider-tab-option {
    transition: none;
  }
}

.budget-input {
  display: flex;
  align-items: center;
  gap: 8px;
}

.budget-input-field {
  width: 140px;
}

.budget-time-input {
  width: 140px;
}

.budget-select {
  width: 160px;
}

.budget-quota-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
  padding: 16px 18px 18px;
}

.budget-quota-card {
  border: 1px solid color-mix(in srgb, var(--mac-border) 88%, transparent);
  border-radius: 12px;
  background: color-mix(in srgb, var(--mac-surface-strong) 72%, var(--mac-surface));
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.budget-quota-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.budget-quota-card__heading {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.budget-quota-card__title {
  margin: 0;
  font-size: 13px;
  font-weight: 700;
  color: var(--mac-text);
}

.budget-quota-card__hint {
  margin: 0;
  font-size: 11px;
  line-height: 1.5;
  color: var(--mac-text-secondary);
}

.budget-quota-card__limit {
  flex-shrink: 0;
  padding: 5px 9px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--mac-accent) 14%, transparent);
  color: var(--mac-accent);
  font-size: 12px;
  font-weight: 700;
}

.budget-quota-card__body {
  display: grid;
  gap: 12px;
}

.budget-quota-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.budget-quota-field__label {
  font-size: 11px;
  font-weight: 600;
  color: var(--mac-text-secondary);
}

.budget-quota-field__hint {
  font-size: 11px;
  line-height: 1.4;
  color: var(--mac-text-secondary);
}

.budget-unit {
  font-size: 12px;
  color: var(--mac-text-secondary);
}

:global(.dark) .budget-quota-card {
  background: color-mix(in srgb, rgba(255, 255, 255, 0.04) 70%, rgba(17, 24, 39, 0.92));
  border-color: rgba(255, 255, 255, 0.08);
}

:global(.dark) .budget-quota-card__hint,
:global(.dark) .budget-quota-field__label,
:global(.dark) .budget-quota-field__hint {
  color: rgba(255, 255, 255, 0.58);
}

:global(.dark) .budget-quota-card__limit {
  background: rgba(124, 224, 127, 0.12);
  color: #8be08e;
}

.import-path-input {
  width: 280px;
  font-size: 12px;
}

.webdav-input {
  width: 280px;
  font-size: 12px;
}

.webdav-timeout-input {
  width: 120px;
  font-size: 12px;
}

.webdav-actions {
  display: grid;
  width: 100%;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 8px;
  max-width: 100%;
  min-width: 0;
}

:deep(.webdav-actions-row) {
  align-items: stretch;
}

:deep(.webdav-actions-row .mac-list-text) {
  flex: 1 1 100%;
  min-width: 0;
}

:deep(.webdav-actions-row .mac-list-control) {
  display: flex;
  flex: 1 1 100%;
  width: 100%;
  justify-content: flex-start;
  margin-left: 0;
}

.webdav-actions :is(.action-btn, .primary-btn) {
  width: 100%;
  min-width: 0;
}

@media (max-width: 760px) {
  .webdav-actions {
    grid-template-columns: 1fr;
  }
}

.webdav-manage-modal {
  width: 100%;
  max-width: 100%;
}

.heatmap-display-modal {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.heatmap-display-hint {
  margin: 0;
  font-size: 12px;
  color: var(--mac-text-secondary);
  line-height: 1.5;
}

.heatmap-display-fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}

.heatmap-display-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.heatmap-display-label {
  font-size: 12px;
  color: var(--mac-text-secondary);
}

.heatmap-display-input {
  width: 100%;
  min-width: 0;
}

.heatmap-display-note {
  margin: 0;
  font-size: 11px;
  color: var(--mac-text-secondary);
  line-height: 1.4;
}

.heatmap-display-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  flex-wrap: wrap;
  padding-top: 12px;
  border-top: 1px solid var(--mac-divider);
}

.update-modal {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.update-modal-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.update-modal-label {
  font-size: 12px;
  color: var(--mac-text-secondary);
}

.update-modal-message {
  margin: 2px 0 0;
}

.update-modal-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.update-modal-block-title {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--mac-text);
}

.update-modal-release-notes {
  margin: 0;
  max-height: 220px;
  overflow: auto;
  border-radius: 12px;
  border: 1px solid var(--mac-border);
  background: var(--mac-surface-strong);
  color: var(--mac-text-secondary);
  padding: 10px 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.update-modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  flex-wrap: wrap;
  padding-top: 12px;
  border-top: 1px solid var(--mac-divider);
}

.codex-unify-option {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--mac-text);
  font-size: 13px;
  line-height: 1.4;
}

.webdav-sync-modal {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.webdav-sync-hint {
  margin: 0;
  font-size: 0.875rem;
  color: var(--mac-text-secondary);
  line-height: 1.5;
}

.webdav-sync-block-title {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--mac-text);
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.webdav-sync-count {
  font-size: 0.8rem;
  color: var(--mac-text-secondary);
  font-weight: 500;
}

.webdav-sync-includes {
  margin: 10px 0 0;
  padding-left: 18px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 12px;
  color: var(--mac-text-secondary);
  line-height: 1.5;
}

.webdav-sync-progress-row {
  margin-top: 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.webdav-sync-stage {
  font-size: 0.875rem;
  color: var(--mac-text-secondary);
}

.webdav-sync-percent {
  font-size: 0.85rem;
  color: var(--mac-text-secondary);
  font-variant-numeric: tabular-nums;
}

.webdav-progress-bar {
  margin-top: 8px;
  width: 100%;
  height: 10px;
  background: var(--mac-surface-strong);
  border: 1px solid var(--mac-border);
  border-radius: 999px;
  overflow: hidden;
}

.webdav-progress-fill {
  height: 100%;
  background: #0ea5e9;
  transition: width 0.2s ease;
}

.webdav-sync-meta {
  margin-top: 8px;
  text-align: right;
  font-size: 12px;
  color: var(--mac-text-secondary);
  font-variant-numeric: tabular-nums;
}

.webdav-sync-remote {
  margin: 10px 0 0;
  font-size: 12px;
  color: var(--mac-text-secondary);
  line-height: 1.4;
}

.webdav-sync-remote-url {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  color: var(--mac-text);
  overflow-wrap: anywhere;
}

.alert-error {
  margin: 10px 0 0;
  padding: 0.65rem 0.85rem;
  border-radius: 12px;
  background: rgba(244, 67, 54, 0.12);
  color: #d93025;
  border: 1px solid rgba(244, 67, 54, 0.2);
  font-size: 12px;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

:global(.dark) .alert-error {
  background: rgba(244, 67, 54, 0.15);
  color: #ff9b9b;
  border-color: rgba(244, 67, 54, 0.2);
}

.webdav-sync-logs {
  margin: 10px 0 0;
  padding: 10px 12px;
  border-radius: 12px;
  background: var(--mac-surface-strong);
  border: 1px solid var(--mac-border);
  max-height: 160px;
  overflow: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 12px;
  line-height: 1.5;
  color: var(--mac-text-secondary);
}

.webdav-sync-log {
  margin: 0;
  color: var(--mac-text-secondary);
  overflow-wrap: anywhere;
}

.webdav-sync-log + .webdav-sync-log {
  margin-top: 4px;
}

.webdav-sync-log.is-error {
  color: #d93025;
}

:global(.dark) .webdav-sync-log.is-error {
  color: #ff9b9b;
}

.webdav-sync-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  flex-wrap: wrap;
  padding-top: 16px;
  border-top: 1px solid var(--mac-divider);
}

.info-text.warning {
  color: var(--mac-text-warning, #e67e22);
}

:global(.dark) .info-text.warning {
  color: #f39c12;
}


:global(.dark) .mac-input {
  background: var(--mac-surface-strong);
}
</style>
