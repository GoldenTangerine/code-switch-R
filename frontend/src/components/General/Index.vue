<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Call, Events } from '@wailsio/runtime'
import ListItem from '../Setting/ListRow.vue'
import LanguageSwitcher from '../Setting/LanguageSwitcher.vue'
import ThemeSetting from '../Setting/ThemeSetting.vue'
import NetworkWslSettings from '../Setting/NetworkWslSettings.vue'
import InlineModal from '../common/InlineModal.vue'
import { fetchAppSettings, saveAppSettings, type AppSettings } from '../../services/appSettings'
import { checkUpdate, downloadUpdate, restartApp, getUpdateState, setAutoCheckEnabled, type UpdateState } from '../../services/update'
import { fetchCurrentVersion } from '../../services/version'
import { getBlacklistSettings, updateBlacklistSettings, getLevelBlacklistEnabled, setLevelBlacklistEnabled, getBlacklistEnabled, setBlacklistEnabled, type BlacklistSettings } from '../../services/settings'
import { fetchConfigImportStatus, importFromPath, type ConfigImportStatus } from '../../services/configImport'
import { fetchWebDAVConfig, previewWebDAVContent, saveWebDAVConfig, testWebDAVConfig, syncToWebDAV, loadFromWebDAV, type WebDAVSyncConfig } from '../../services/webdavSync'
import { useI18n } from 'vue-i18n'
import { extractErrorMessage } from '../../utils/error'
import { showToast } from '../../utils/toast'

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
const heatmapEnabled = ref(getCachedValue('heatmap', true))
const homeTitleVisible = ref(getCachedValue('homeTitle', true))
const autoStartEnabled = ref(getCachedValue('autoStart', false))
const autoUpdateEnabled = ref(getCachedValue('autoUpdate', true))
const autoConnectivityTestEnabled = ref(getCachedValue('autoConnectivityTest', false))
const switchNotifyEnabled = ref(getCachedValue('switchNotify', true)) // 切换通知开关
const roundRobinEnabled = ref(getCachedValue('roundRobin', false))    // 同 Level 轮询开关
const budgetTotal = ref(getCachedNumber('budgetTotal', 0))
const budgetUsedAdjustment = ref(getCachedNumber('budgetUsedAdjustment', 0))
const budgetForecastMethod = ref(getCachedString('budgetForecastMethod', 'cycle'))
const budgetForecastDisplay = ref(getCachedString('budgetForecastDisplay', 'datetime'))
const budgetCycleEnabled = ref(getCachedValue('budgetCycleEnabled', false))
const budgetCycleMode = ref(getCachedString('budgetCycleMode', 'daily'))
const budgetRefreshTime = ref(getCachedString('budgetRefreshTime', '00:00'))
const budgetRefreshDay = ref(getCachedNumber('budgetRefreshDay', 1))
const budgetShowCountdown = ref(getCachedValue('budgetShowCountdown', false))
const budgetShowForecast = ref(getCachedValue('budgetShowForecast', false))
const budgetTotalCodex = ref(getCachedNumber('budgetTotalCodex', 0))
const budgetUsedAdjustmentCodex = ref(getCachedNumber('budgetUsedAdjustmentCodex', 0))
const budgetForecastMethodCodex = ref(getCachedString('budgetForecastMethodCodex', 'cycle'))
const budgetForecastDisplayCodex = ref(getCachedString('budgetForecastDisplayCodex', 'datetime'))
const budgetCycleEnabledCodex = ref(getCachedValue('budgetCycleEnabledCodex', false))
const budgetCycleModeCodex = ref(getCachedString('budgetCycleModeCodex', 'daily'))
const budgetRefreshTimeCodex = ref(getCachedString('budgetRefreshTimeCodex', '00:00'))
const budgetRefreshDayCodex = ref(getCachedNumber('budgetRefreshDayCodex', 1))
const budgetShowCountdownCodex = ref(getCachedValue('budgetShowCountdownCodex', false))
const budgetShowForecastCodex = ref(getCachedValue('budgetShowForecastCodex', false))
const settingsLoading = ref(true)
const saveBusy = ref(false)

// 更新相关状态
const updateState = ref<UpdateState | null>(null)
const checking = ref(false)
const downloading = ref(false)
const appVersion = ref('')

// 拉黑配置相关状态
const blacklistEnabled = ref(true)  // 拉黑功能总开关
const blacklistThreshold = ref(3)
const blacklistDurationSeconds = ref(1800)
const levelBlacklistEnabled = ref(false)
const blacklistLoading = ref(false)
const blacklistSaving = ref(false)

// cc-switch 导入相关状态
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

let unsubscribeWebdavSync: (() => void) | null = null

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

const handleWebdavSyncEvent = (event: { data: Record<string, any> }) => {
  const data = event?.data ?? {}
  if (data?.type !== 'upload') return

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
}

const goBack = () => {
  router.push('/')
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

const loadAppSettings = async () => {
  settingsLoading.value = true
  try {
    const data = await fetchAppSettings()
    heatmapEnabled.value = data?.show_heatmap ?? true
    homeTitleVisible.value = data?.show_home_title ?? true
    budgetTotal.value = Number(data?.budget_total ?? 0)
    budgetUsedAdjustment.value = Number(data?.budget_used_adjustment ?? 0)
    budgetForecastMethod.value = normalizeBudgetForecastMethod(data?.budget_forecast_method ?? 'cycle')
    budgetForecastDisplay.value = normalizeBudgetForecastDisplay(data?.budget_forecast_display ?? 'datetime')
    budgetCycleEnabled.value = data?.budget_cycle_enabled ?? false
    budgetCycleMode.value = data?.budget_cycle_mode === 'weekly' ? 'weekly' : 'daily'
    budgetRefreshTime.value = data?.budget_refresh_time || '00:00'
    budgetRefreshDay.value = Number.isFinite(data?.budget_refresh_day) ? data?.budget_refresh_day : 1
    budgetShowCountdown.value = data?.budget_show_countdown ?? false
    budgetShowForecast.value = data?.budget_show_forecast ?? false
    budgetTotalCodex.value = Number(data?.budget_total_codex ?? 0)
    budgetUsedAdjustmentCodex.value = Number(data?.budget_used_adjustment_codex ?? 0)
    budgetForecastMethodCodex.value = normalizeBudgetForecastMethod(data?.budget_forecast_method_codex ?? 'cycle')
    budgetForecastDisplayCodex.value = normalizeBudgetForecastDisplay(data?.budget_forecast_display_codex ?? 'datetime')
    budgetCycleEnabledCodex.value = data?.budget_cycle_enabled_codex ?? false
    budgetCycleModeCodex.value = data?.budget_cycle_mode_codex === 'weekly' ? 'weekly' : 'daily'
    budgetRefreshTimeCodex.value = data?.budget_refresh_time_codex || '00:00'
    budgetRefreshDayCodex.value = Number.isFinite(data?.budget_refresh_day_codex) ? data?.budget_refresh_day_codex : 1
    budgetShowCountdownCodex.value = data?.budget_show_countdown_codex ?? false
    budgetShowForecastCodex.value = data?.budget_show_forecast_codex ?? false
    autoStartEnabled.value = data?.auto_start ?? false
    autoUpdateEnabled.value = data?.auto_update ?? true
    autoConnectivityTestEnabled.value = data?.auto_connectivity_test ?? false
    switchNotifyEnabled.value = data?.enable_switch_notify ?? true
    roundRobinEnabled.value = data?.enable_round_robin ?? false

    // 缓存到 localStorage，下次打开时直接显示正确状态
    localStorage.setItem('app-settings-heatmap', String(heatmapEnabled.value))
    localStorage.setItem('app-settings-homeTitle', String(homeTitleVisible.value))
    localStorage.setItem('app-settings-budgetTotal', String(budgetTotal.value))
    localStorage.setItem('app-settings-budgetUsedAdjustment', String(budgetUsedAdjustment.value))
    localStorage.setItem('app-settings-budgetForecastMethod', budgetForecastMethod.value)
    localStorage.setItem('app-settings-budgetForecastDisplay', budgetForecastDisplay.value)
    localStorage.setItem('app-settings-budgetCycleEnabled', String(budgetCycleEnabled.value))
    localStorage.setItem('app-settings-budgetCycleMode', budgetCycleMode.value)
    localStorage.setItem('app-settings-budgetRefreshTime', budgetRefreshTime.value)
    localStorage.setItem('app-settings-budgetRefreshDay', String(budgetRefreshDay.value))
    localStorage.setItem('app-settings-budgetShowCountdown', String(budgetShowCountdown.value))
    localStorage.setItem('app-settings-budgetShowForecast', String(budgetShowForecast.value))
    localStorage.setItem('app-settings-budgetTotalCodex', String(budgetTotalCodex.value))
    localStorage.setItem('app-settings-budgetUsedAdjustmentCodex', String(budgetUsedAdjustmentCodex.value))
    localStorage.setItem('app-settings-budgetForecastMethodCodex', budgetForecastMethodCodex.value)
    localStorage.setItem('app-settings-budgetForecastDisplayCodex', budgetForecastDisplayCodex.value)
    localStorage.setItem('app-settings-budgetCycleEnabledCodex', String(budgetCycleEnabledCodex.value))
    localStorage.setItem('app-settings-budgetCycleModeCodex', budgetCycleModeCodex.value)
    localStorage.setItem('app-settings-budgetRefreshTimeCodex', budgetRefreshTimeCodex.value)
    localStorage.setItem('app-settings-budgetRefreshDayCodex', String(budgetRefreshDayCodex.value))
    localStorage.setItem('app-settings-budgetShowCountdownCodex', String(budgetShowCountdownCodex.value))
    localStorage.setItem('app-settings-budgetShowForecastCodex', String(budgetShowForecastCodex.value))
    localStorage.setItem('app-settings-autoStart', String(autoStartEnabled.value))
    localStorage.setItem('app-settings-autoUpdate', String(autoUpdateEnabled.value))
    localStorage.setItem('app-settings-autoConnectivityTest', String(autoConnectivityTestEnabled.value))
    localStorage.setItem('app-settings-switchNotify', String(switchNotifyEnabled.value))
    localStorage.setItem('app-settings-roundRobin', String(roundRobinEnabled.value))
  } catch (error) {
    console.error('failed to load app settings', error)
    heatmapEnabled.value = true
    homeTitleVisible.value = true
    budgetTotal.value = 0
    budgetUsedAdjustment.value = 0
    budgetForecastMethod.value = 'cycle'
    budgetForecastDisplay.value = 'datetime'
    budgetCycleEnabled.value = false
    budgetCycleMode.value = 'daily'
    budgetRefreshTime.value = '00:00'
    budgetRefreshDay.value = 1
    budgetShowCountdown.value = false
    budgetShowForecast.value = false
    budgetTotalCodex.value = 0
    budgetUsedAdjustmentCodex.value = 0
    budgetForecastMethodCodex.value = 'cycle'
    budgetForecastDisplayCodex.value = 'datetime'
    budgetCycleEnabledCodex.value = false
    budgetCycleModeCodex.value = 'daily'
    budgetRefreshTimeCodex.value = '00:00'
    budgetRefreshDayCodex.value = 1
    budgetShowCountdownCodex.value = false
    budgetShowForecastCodex.value = false
    autoStartEnabled.value = false
    autoUpdateEnabled.value = true
    autoConnectivityTestEnabled.value = false
    switchNotifyEnabled.value = true
    roundRobinEnabled.value = false
  } finally {
    settingsLoading.value = false
  }
}

const persistAppSettings = async () => {
  if (settingsLoading.value || saveBusy.value) return
  saveBusy.value = true
  try {
    const normalizedBudgetTotal = Number.isFinite(budgetTotal.value) ? Math.max(0, budgetTotal.value) : 0
    budgetTotal.value = normalizedBudgetTotal
    const normalizedBudgetUsedAdjustment = Number.isFinite(budgetUsedAdjustment.value)
      ? budgetUsedAdjustment.value
      : 0
    budgetUsedAdjustment.value = normalizedBudgetUsedAdjustment
    const normalizedBudgetForecastMethod = normalizeBudgetForecastMethod(budgetForecastMethod.value)
    budgetForecastMethod.value = normalizedBudgetForecastMethod
    const normalizedBudgetForecastDisplay = normalizeBudgetForecastDisplay(budgetForecastDisplay.value)
    budgetForecastDisplay.value = normalizedBudgetForecastDisplay
    const normalizedBudgetTotalCodex = Number.isFinite(budgetTotalCodex.value)
      ? Math.max(0, budgetTotalCodex.value)
      : 0
    budgetTotalCodex.value = normalizedBudgetTotalCodex
    const normalizedBudgetUsedAdjustmentCodex = Number.isFinite(budgetUsedAdjustmentCodex.value)
      ? budgetUsedAdjustmentCodex.value
      : 0
    budgetUsedAdjustmentCodex.value = normalizedBudgetUsedAdjustmentCodex
    const normalizedBudgetForecastMethodCodex = normalizeBudgetForecastMethod(budgetForecastMethodCodex.value)
    budgetForecastMethodCodex.value = normalizedBudgetForecastMethodCodex
    const normalizedBudgetForecastDisplayCodex = normalizeBudgetForecastDisplay(budgetForecastDisplayCodex.value)
    budgetForecastDisplayCodex.value = normalizedBudgetForecastDisplayCodex
    const normalizedBudgetRefreshDay = Number.isFinite(budgetRefreshDay.value)
      ? Math.min(Math.max(Math.floor(budgetRefreshDay.value), 0), 6)
      : 1
    budgetRefreshDay.value = normalizedBudgetRefreshDay
    const normalizedBudgetCycleMode = budgetCycleMode.value === 'weekly' ? 'weekly' : 'daily'
    budgetCycleMode.value = normalizedBudgetCycleMode
    const normalizedBudgetRefreshDayCodex = Number.isFinite(budgetRefreshDayCodex.value)
      ? Math.min(Math.max(Math.floor(budgetRefreshDayCodex.value), 0), 6)
      : 1
    budgetRefreshDayCodex.value = normalizedBudgetRefreshDayCodex
    const normalizedBudgetCycleModeCodex = budgetCycleModeCodex.value === 'weekly' ? 'weekly' : 'daily'
    budgetCycleModeCodex.value = normalizedBudgetCycleModeCodex
    const payload: AppSettings = {
      show_heatmap: heatmapEnabled.value,
      show_home_title: homeTitleVisible.value,
      budget_total: normalizedBudgetTotal,
      budget_used_adjustment: normalizedBudgetUsedAdjustment,
      budget_forecast_method: normalizedBudgetForecastMethod,
      budget_forecast_display: normalizedBudgetForecastDisplay,
      budget_cycle_enabled: budgetCycleEnabled.value,
      budget_cycle_mode: normalizedBudgetCycleMode,
      budget_refresh_time: budgetRefreshTime.value || '00:00',
      budget_refresh_day: normalizedBudgetRefreshDay,
      budget_show_countdown: budgetShowCountdown.value,
      budget_show_forecast: budgetShowForecast.value,
      budget_total_codex: normalizedBudgetTotalCodex,
      budget_used_adjustment_codex: normalizedBudgetUsedAdjustmentCodex,
      budget_forecast_method_codex: normalizedBudgetForecastMethodCodex,
      budget_forecast_display_codex: normalizedBudgetForecastDisplayCodex,
      budget_cycle_enabled_codex: budgetCycleEnabledCodex.value,
      budget_cycle_mode_codex: normalizedBudgetCycleModeCodex,
      budget_refresh_time_codex: budgetRefreshTimeCodex.value || '00:00',
      budget_refresh_day_codex: normalizedBudgetRefreshDayCodex,
      budget_show_countdown_codex: budgetShowCountdownCodex.value,
      budget_show_forecast_codex: budgetShowForecastCodex.value,
      auto_start: autoStartEnabled.value,
      auto_update: autoUpdateEnabled.value,
      auto_connectivity_test: autoConnectivityTestEnabled.value,
      enable_switch_notify: switchNotifyEnabled.value,
      enable_round_robin: roundRobinEnabled.value,
    }
    await saveAppSettings(payload)

    // 同步自动更新设置到 UpdateService
    await setAutoCheckEnabled(autoUpdateEnabled.value)

    // 同步自动可用性监控设置到 HealthCheckService（复用旧字段名）
    await Call.ByName(
      'codeswitch/services.HealthCheckService.SetAutoAvailabilityPolling',
      autoConnectivityTestEnabled.value
    )

    // 更新缓存
    localStorage.setItem('app-settings-heatmap', String(heatmapEnabled.value))
    localStorage.setItem('app-settings-homeTitle', String(homeTitleVisible.value))
    localStorage.setItem('app-settings-budgetTotal', String(budgetTotal.value))
    localStorage.setItem('app-settings-budgetUsedAdjustment', String(budgetUsedAdjustment.value))
    localStorage.setItem('app-settings-budgetForecastMethod', budgetForecastMethod.value)
    localStorage.setItem('app-settings-budgetForecastDisplay', budgetForecastDisplay.value)
    localStorage.setItem('app-settings-budgetCycleEnabled', String(budgetCycleEnabled.value))
    localStorage.setItem('app-settings-budgetCycleMode', budgetCycleMode.value)
    localStorage.setItem('app-settings-budgetRefreshTime', budgetRefreshTime.value)
    localStorage.setItem('app-settings-budgetRefreshDay', String(budgetRefreshDay.value))
    localStorage.setItem('app-settings-budgetShowCountdown', String(budgetShowCountdown.value))
    localStorage.setItem('app-settings-budgetShowForecast', String(budgetShowForecast.value))
    localStorage.setItem('app-settings-budgetTotalCodex', String(budgetTotalCodex.value))
    localStorage.setItem('app-settings-budgetUsedAdjustmentCodex', String(budgetUsedAdjustmentCodex.value))
    localStorage.setItem('app-settings-budgetForecastMethodCodex', budgetForecastMethodCodex.value)
    localStorage.setItem('app-settings-budgetForecastDisplayCodex', budgetForecastDisplayCodex.value)
    localStorage.setItem('app-settings-budgetCycleEnabledCodex', String(budgetCycleEnabledCodex.value))
    localStorage.setItem('app-settings-budgetCycleModeCodex', budgetCycleModeCodex.value)
    localStorage.setItem('app-settings-budgetRefreshTimeCodex', budgetRefreshTimeCodex.value)
    localStorage.setItem('app-settings-budgetRefreshDayCodex', String(budgetRefreshDayCodex.value))
    localStorage.setItem('app-settings-budgetShowCountdownCodex', String(budgetShowCountdownCodex.value))
    localStorage.setItem('app-settings-budgetShowForecastCodex', String(budgetShowForecastCodex.value))
    localStorage.setItem('app-settings-autoStart', String(autoStartEnabled.value))
    localStorage.setItem('app-settings-autoUpdate', String(autoUpdateEnabled.value))
    localStorage.setItem('app-settings-autoConnectivityTest', String(autoConnectivityTestEnabled.value))
    localStorage.setItem('app-settings-switchNotify', String(switchNotifyEnabled.value))
    localStorage.setItem('app-settings-roundRobin', String(roundRobinEnabled.value))

    window.dispatchEvent(new CustomEvent('app-settings-updated'))
  } catch (error) {
    console.error('failed to save app settings', error)
  } finally {
    saveBusy.value = false
  }
}

const loadUpdateState = async () => {
  try {
    updateState.value = await getUpdateState()
  } catch (error) {
    console.error('failed to load update state', error)
  }
}

const checkUpdateManually = async () => {
  checking.value = true
  try {
    const info = await checkUpdate()
    await loadUpdateState()

    if (!info.available) {
      alert('已是最新版本')
    } else {
      // 发现新版本，提示用户并开始下载
      const confirmed = confirm(`发现新版本 ${info.version}，是否立即下载？`)
      if (confirmed) {
        downloading.value = true
        checking.value = false
        try {
          await downloadUpdate()
          await loadUpdateState()

          // 下载完成，提示重启
          const restart = confirm('新版本已下载完成，是否立即重启应用？')
          if (restart) {
            await restartApp()
          }
        } catch (downloadError) {
          console.error('download failed', downloadError)
          alert('下载失败: ' + extractErrorMessage(downloadError))
        } finally {
          downloading.value = false
        }
      }
    }
  } catch (error) {
    console.error('check update failed', error)
    alert('检查更新失败，请检查网络连接')
  } finally {
    checking.value = false
  }
}

const downloadAndInstall = async () => {
  downloading.value = true
  try {
    await downloadUpdate()
    await loadUpdateState()

    // 弹窗确认重启
    const confirmed = confirm('新版本已下载完成，是否立即重启应用？')
    if (confirmed) {
      await restartApp()
    }
  } catch (error) {
    console.error('download failed', error)
    alert('下载失败: ' + extractErrorMessage(error))
  } finally {
    downloading.value = false
  }
}

// 当更新已下载完成时，直接安装并重启（无需再次下载）
const installAndRestart = async () => {
  const confirmed = confirm('是否立即安装更新并重启应用？')
  if (confirmed) {
    try {
      await restartApp()
    } catch (error) {
      console.error('restart failed', error)
      alert('重启失败，请手动重启应用')
    }
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
    blacklistThreshold.value = 3
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
    await updateBlacklistSettings(blacklistThreshold.value, blacklistDurationSeconds.value)
    showToast(t('components.general.toast.blacklistSaveSuccess'), 'success')
  } catch (error) {
    console.error('failed to save blacklist settings', error)
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

// 加载 cc-switch 导入状态
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

const uploadToWebDAV = async () => {
  await openWebdavUploadModal()
}

const downloadFromWebDAV = async () => {
  if (webdavLoading.value || webdavDownloading.value || webdavUploading.value) return
  const confirmed = confirm(t('components.general.webdav.confirmDownload'))
  if (!confirmed) return
  webdavDownloading.value = true
  try {
    const result = await loadFromWebDAV(buildWebDAVConfig())
    showToast(t('components.general.webdav.downloadOk'), 'success')
    await loadAppSettings()
    await loadBlacklistSettings()
    await loadImportStatus()
    alert(t('components.general.webdav.downloadPostHint', { backup: result?.backup_path || '' }))
  } catch (error) {
    console.error('webdav download failed', error)
    showToast(t('components.general.webdav.downloadFailed') + ': ' + extractErrorMessage(error), 'error')
  } finally {
    webdavDownloading.value = false
  }
}

onMounted(async () => {
  await loadAppSettings()

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
  unsubscribeWebdavSync = Events.On('webdav:sync', handleWebdavSyncEvent as Events.Callback)
})

onBeforeUnmount(() => {
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
        </div>
      </section>

      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.trayPanel') }}</h2>
        <div class="mac-panel">
          <p class="panel-title">{{ $t('components.general.label.trayPanelClaude') }}</p>
          <ListItem :label="$t('components.general.label.budgetTotal')">
            <div class="toggle-with-hint">
              <div class="budget-input">
                <input
                  type="number"
                  inputmode="decimal"
                  min="0"
                  step="0.01"
                  :disabled="settingsLoading || saveBusy"
                  v-model.number="budgetTotal"
                  @change="persistAppSettings"
                  class="mac-input budget-input-field"
                />
                <span class="budget-unit">USD</span>
              </div>
              <span class="hint-text">{{ $t('components.general.label.budgetTotalHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetUsedAdjustment')">
            <div class="toggle-with-hint">
              <div class="budget-input">
                <input
                  type="number"
                  inputmode="decimal"
                  step="0.01"
                  :disabled="settingsLoading || saveBusy"
                  v-model.number="budgetUsedAdjustment"
                  @change="persistAppSettings"
                  class="mac-input budget-input-field"
                />
                <span class="budget-unit">USD</span>
              </div>
              <span class="hint-text">{{ $t('components.general.label.budgetUsedAdjustmentHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetCycle')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  type="checkbox"
                  :disabled="settingsLoading || saveBusy"
                  v-model="budgetCycleEnabled"
                  @change="persistAppSettings"
                />
                <span></span>
              </label>
              <span class="hint-text">{{ $t('components.general.label.budgetCycleHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetCycleMode')">
            <select
              v-model="budgetCycleMode"
              :disabled="settingsLoading || saveBusy || !budgetCycleEnabled"
              class="mac-select budget-select"
              @change="persistAppSettings">
              <option value="daily">{{ $t('components.general.label.budgetCycleModeDaily') }}</option>
              <option value="weekly">{{ $t('components.general.label.budgetCycleModeWeekly') }}</option>
            </select>
          </ListItem>
          <ListItem
            v-if="budgetCycleMode === 'weekly'"
            :label="$t('components.general.label.budgetRefreshDay')">
            <select
              v-model.number="budgetRefreshDay"
              :disabled="settingsLoading || saveBusy || !budgetCycleEnabled"
              class="mac-select budget-select"
              @change="persistAppSettings">
              <option :value="1">{{ $t('components.general.label.weekdayMon') }}</option>
              <option :value="2">{{ $t('components.general.label.weekdayTue') }}</option>
              <option :value="3">{{ $t('components.general.label.weekdayWed') }}</option>
              <option :value="4">{{ $t('components.general.label.weekdayThu') }}</option>
              <option :value="5">{{ $t('components.general.label.weekdayFri') }}</option>
              <option :value="6">{{ $t('components.general.label.weekdaySat') }}</option>
              <option :value="0">{{ $t('components.general.label.weekdaySun') }}</option>
            </select>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetRefreshTime')">
            <input
              type="time"
              :disabled="settingsLoading || saveBusy || !budgetCycleEnabled"
              v-model="budgetRefreshTime"
              @change="persistAppSettings"
              class="mac-input budget-time-input"
            />
          </ListItem>
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
          <ListItem :label="$t('components.general.label.budgetTotal')">
            <div class="toggle-with-hint">
              <div class="budget-input">
                <input
                  type="number"
                  inputmode="decimal"
                  min="0"
                  step="0.01"
                  :disabled="settingsLoading || saveBusy"
                  v-model.number="budgetTotalCodex"
                  @change="persistAppSettings"
                  class="mac-input budget-input-field"
                />
                <span class="budget-unit">USD</span>
              </div>
              <span class="hint-text">{{ $t('components.general.label.budgetTotalHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetUsedAdjustment')">
            <div class="toggle-with-hint">
              <div class="budget-input">
                <input
                  type="number"
                  inputmode="decimal"
                  step="0.01"
                  :disabled="settingsLoading || saveBusy"
                  v-model.number="budgetUsedAdjustmentCodex"
                  @change="persistAppSettings"
                  class="mac-input budget-input-field"
                />
                <span class="budget-unit">USD</span>
              </div>
              <span class="hint-text">{{ $t('components.general.label.budgetUsedAdjustmentHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetCycle')">
            <div class="toggle-with-hint">
              <label class="mac-switch">
                <input
                  type="checkbox"
                  :disabled="settingsLoading || saveBusy"
                  v-model="budgetCycleEnabledCodex"
                  @change="persistAppSettings"
                />
                <span></span>
              </label>
              <span class="hint-text">{{ $t('components.general.label.budgetCycleHint') }}</span>
            </div>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetCycleMode')">
            <select
              v-model="budgetCycleModeCodex"
              :disabled="settingsLoading || saveBusy || !budgetCycleEnabledCodex"
              class="mac-select budget-select"
              @change="persistAppSettings">
              <option value="daily">{{ $t('components.general.label.budgetCycleModeDaily') }}</option>
              <option value="weekly">{{ $t('components.general.label.budgetCycleModeWeekly') }}</option>
            </select>
          </ListItem>
          <ListItem
            v-if="budgetCycleModeCodex === 'weekly'"
            :label="$t('components.general.label.budgetRefreshDay')">
            <select
              v-model.number="budgetRefreshDayCodex"
              :disabled="settingsLoading || saveBusy || !budgetCycleEnabledCodex"
              class="mac-select budget-select"
              @change="persistAppSettings">
              <option :value="1">{{ $t('components.general.label.weekdayMon') }}</option>
              <option :value="2">{{ $t('components.general.label.weekdayTue') }}</option>
              <option :value="3">{{ $t('components.general.label.weekdayWed') }}</option>
              <option :value="4">{{ $t('components.general.label.weekdayThu') }}</option>
              <option :value="5">{{ $t('components.general.label.weekdayFri') }}</option>
              <option :value="6">{{ $t('components.general.label.weekdaySat') }}</option>
              <option :value="0">{{ $t('components.general.label.weekdaySun') }}</option>
            </select>
          </ListItem>
          <ListItem :label="$t('components.general.label.budgetRefreshTime')">
            <input
              type="time"
              :disabled="settingsLoading || saveBusy || !budgetCycleEnabledCodex"
              v-model="budgetRefreshTimeCodex"
              @change="persistAppSettings"
              class="mac-input budget-time-input"
            />
          </ListItem>
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
      </section>

      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.connectivity') }}</h2>
        <div class="mac-panel">
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
        <h2 class="mac-section-title">{{ $t('components.general.title.webdavSync') }}</h2>
        <div class="mac-panel">
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
          <ListItem :label="$t('components.general.webdav.actions')">
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

          <ListItem :label="$t('components.general.label.lastCheck')">
            <span class="info-text">{{ formatLastCheckTime(updateState?.last_check_time) }}</span>
            <span v-if="updateState && updateState.consecutive_failures > 0" class="warning-badge">
              ⚠️ {{ $t('components.general.update.checkFailed', { count: updateState.consecutive_failures }) }}
            </span>
          </ListItem>

          <ListItem :label="$t('components.general.label.currentVersion')">
            <span class="version-text">{{ appVersion }}</span>
          </ListItem>

          <ListItem
            v-if="updateState?.latest_known_version && updateState.latest_known_version !== appVersion"
            :label="$t('components.general.label.latestVersion')">
            <span class="version-text highlight">{{ updateState.latest_known_version }} 🆕</span>
          </ListItem>

          <ListItem :label="$t('components.general.label.checkNow')">
            <button
              @click="checkUpdateManually"
              :disabled="checking"
              class="action-btn">
              {{ checking ? $t('components.general.update.checking') : $t('components.general.update.checkNow') }}
            </button>
          </ListItem>

          <ListItem
            v-if="updateState?.update_ready"
            :label="$t('components.general.label.manualUpdate')">
            <button
              @click="installAndRestart"
              class="primary-btn">
              {{ $t('components.general.update.installAndRestart') }}
            </button>
          </ListItem>
        </div>
      </section>

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

.hint-text {
  font-size: 11px;
  color: var(--mac-text-secondary);
  line-height: 1.4;
  max-width: 320px;
  text-align: right;
  white-space: normal;
  overflow-wrap: anywhere;
}

:global(.dark) .hint-text {
  color: rgba(255, 255, 255, 0.5);
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

.budget-unit {
  font-size: 12px;
  color: var(--mac-text-secondary);
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
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
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
