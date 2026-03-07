import { computed, nextTick, reactive, ref, type ComponentPublicInstance, type Ref } from 'vue'
import { useAdaptiveHeatmap } from '../../../composables/useAdaptiveHeatmap'
import { DEFAULT_HEATMAP_DISPLAY_SETTINGS, type HeatmapDisplaySettings } from '../../../data/heatmapDisplaySettings'
import { fetchRequestLogDailyHeatmapStats, fetchLogStorageStats, clearRequestLogs, deleteRequestLogsByDate, clearLogStats, type RequestLog, type LogStorageStats } from '../../../services/logs'
import { showToast } from '../../../utils/toast'
import { extractErrorMessage } from '../../../utils/error'
import { useLogsStorageHeatmap } from './useLogsStorageHeatmap'
import {
  buildLogsTableTextFormatters,
  durationColor,
  formatBytes,
  formatCurrency,
  formatDuration,
  formatNumber,
  formatStorageHeatmapPayloadValue as formatStorageHeatmapPayloadValueText,
  formatTime,
  formatTokenNumber,
  httpCodeClass,
  intensityClass,
} from '../utils'
import { type HeatmapGranularity } from '../../../data/usageHeatmap'

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

type UseLogsStorageModalControllerOptions = {
  locale: Ref<string>
  t: TranslateFn
  loadDashboard: () => Promise<void>
  openPayloadDetailModal: (item: RequestLog) => void | Promise<void>
}

type StorageClearTarget = 'requestLogs' | 'requestLogsByDate' | 'stats'

export function useLogsStorageModalController(options: UseLogsStorageModalControllerOptions) {
  const { locale, t, loadDashboard, openPayloadDetailModal } = options

  const storageStats = ref<LogStorageStats | null>(null)
  const storageLoading = ref(false)
  const storageClearing = ref(false)
  const storageModal = reactive({
    open: false,
  })
  const storageHeatmapContainerRef = ref<HTMLElement | null>(null)
  const storageHeatmapGranularity = ref<HeatmapGranularity>('daily')
  const storageHeatmapDisplaySettings = ref<HeatmapDisplaySettings>({
    ...DEFAULT_HEATMAP_DISPLAY_SETTINGS,
    intensityMetric: 'requests',
    dailyIntensityMode: 'daily_peak',
  })
  const {
    displayData: storageHeatmap,
    isLoading: storageHeatmapLoading,
    init: initStorageHeatmap,
    cleanup: cleanupStorageHeatmap,
    reload: reloadStorageHeatmap,
  } = useAdaptiveHeatmap(storageHeatmapContainerRef, storageHeatmapGranularity, storageHeatmapDisplaySettings, {
    fetcher: fetchRequestLogDailyHeatmapStats,
  })
  let storageHeatmapReady = false

  const storageModalOpen = computed(() => storageModal.open)
  const {
    selectedStorageHeatmapDate,
    storageDayLogs,
    storageDayLogsLoading,
    storageDayLogsPage,
    storageHeatmapTooltip,
    bindStorageHeatmapTooltipRef,
    selectedStorageHeatmapDay,
    selectedStorageHeatmapDateLabel,
    storageHeatmapHasData,
    storageDayLogsTotalPages,
    pagedStorageDayLogs,
    storageDayLogsShowingText,
    formatStorageHeatmapDateLabel,
    resolveStorageDayLogTotalTokens,
    formatStorageHeatmapAriaLabel,
    isSelectedStorageHeatmapDay,
    loadSelectedStorageDayLogs,
    resetStorageDayLogs,
    selectStorageHeatmapDay,
    normalizeStorageHeatmapSelection,
    showStorageHeatmapTooltip,
    hideStorageHeatmapTooltip,
    nextStorageDayLogsPage,
    prevStorageDayLogsPage,
  } = useLogsStorageHeatmap({
    storageHeatmap,
    storageModalOpen,
    locale,
    t,
    formatNumber,
  })

  const bindStorageHeatmapContainerRef = (element: Element | ComponentPublicInstance | null) => {
    storageHeatmapContainerRef.value = (element instanceof HTMLElement
      ? element
      : (element as ComponentPublicInstance | null)?.$el instanceof HTMLElement
        ? (element as ComponentPublicInstance).$el
        : null)
  }

  const formatStorageHeatmapPayloadValue = (
    bytes?: number,
    capturedRequests?: number,
    requests?: number,
  ) => formatStorageHeatmapPayloadValueText(
    bytes,
    capturedRequests,
    requests,
    t('components.logs.storage.heatmapTooltipPayloadUnavailable'),
  )

  const loadStorageStats = async () => {
    storageLoading.value = true
    try {
      storageStats.value = await fetchLogStorageStats()
    } catch (error) {
      console.error('failed to load log storage stats', error)
    } finally {
      storageLoading.value = false
    }
  }

  const ensureStorageHeatmapInitialized = async () => {
    await nextTick()
    cleanupStorageHeatmap()
    await initStorageHeatmap()
    storageHeatmapReady = true
    normalizeStorageHeatmapSelection()
  }

  const reloadStorageHeatmapIfReady = async () => {
    if (!storageHeatmapReady) return
    await reloadStorageHeatmap()
    normalizeStorageHeatmapSelection()
  }

  const refreshStorageOverview = async () => {
    await Promise.all([loadStorageStats(), reloadStorageHeatmapIfReady(), loadSelectedStorageDayLogs()])
  }

  const openStorageModal = async () => {
    storageModal.open = true
    await Promise.all([loadStorageStats(), ensureStorageHeatmapInitialized()])
  }

  const storageClearConfirm = reactive<{
    open: boolean
    target: StorageClearTarget | null
    date: string
  }>({
    open: false,
    target: null,
    date: '',
  })

  const resetStorageClearConfirm = () => {
    storageClearConfirm.open = false
    storageClearConfirm.target = null
    storageClearConfirm.date = ''
  }

  const closeStorageModal = () => {
    if (storageClearing.value) return
    storageModal.open = false
    storageHeatmapReady = false
    selectedStorageHeatmapDate.value = ''
    resetStorageClearConfirm()
    hideStorageHeatmapTooltip()
    resetStorageDayLogs()
    cleanupStorageHeatmap()
  }

  const closeStorageClearConfirm = () => {
    if (storageClearing.value) return
    resetStorageClearConfirm()
  }

  const storageClearConfirmMessage = computed(() => {
    switch (storageClearConfirm.target) {
      case 'requestLogs':
        return t('components.logs.storage.confirmClearRequestLog')
      case 'requestLogsByDate':
        return t('components.logs.storage.confirmClearByDate', {
          date: formatStorageHeatmapDateLabel(storageClearConfirm.date),
        })
      case 'stats':
        return t('components.logs.storage.confirmClearStats')
      default:
        return ''
    }
  })

  const storageClearConfirmActionLabel = computed(() => {
    switch (storageClearConfirm.target) {
      case 'requestLogs':
        return t('components.logs.storage.clearRequestLog')
      case 'requestLogsByDate':
        return t('components.logs.storage.clearByDate')
      case 'stats':
        return t('components.logs.storage.clearStats')
      default:
        return t('components.logs.storage.clearRequestLog')
    }
  })

  const handleClearRequestLogs = () => {
    if (storageClearing.value) return
    storageClearConfirm.target = 'requestLogs'
    storageClearConfirm.open = true
  }

  const handleClearRequestLogsByDate = () => {
    if (storageClearing.value || !selectedStorageHeatmapDate.value) return
    storageClearConfirm.target = 'requestLogsByDate'
    storageClearConfirm.date = selectedStorageHeatmapDate.value
    storageClearConfirm.open = true
  }

  const handleClearStats = () => {
    if (storageClearing.value) return
    storageClearConfirm.target = 'stats'
    storageClearConfirm.open = true
  }

  const confirmStorageClear = async () => {
    if (storageClearing.value || !storageClearConfirm.target) return
    const target = storageClearConfirm.target
    const targetDate = storageClearConfirm.date
    storageClearing.value = true
    try {
      let successMessage = t('components.logs.storage.success')
      let successTone: 'success' | 'warning' = 'success'
      if (target === 'requestLogs') {
        await clearRequestLogs()
      } else if (target === 'requestLogsByDate') {
        const result = await deleteRequestLogsByDate(targetDate)
        const deletedStats = Number(result?.deleted_stats_hour ?? 0) + Number(result?.deleted_stats_day ?? 0)
        const deletedLogs = Number(result?.deleted_request_logs ?? 0)
        successMessage =
          deletedLogs > 0 || deletedStats > 0
            ? t('components.logs.storage.clearByDateSuccess', {
              date: formatStorageHeatmapDateLabel(targetDate),
              logs: formatNumber(deletedLogs),
              stats: formatNumber(deletedStats),
            })
            : t('components.logs.storage.clearByDateEmpty', {
              date: formatStorageHeatmapDateLabel(targetDate),
            })
        successTone = deletedLogs > 0 || deletedStats > 0 ? 'success' : 'warning'
      } else {
        await clearLogStats()
      }
      showToast(successMessage, successTone)
      await Promise.all([loadStorageStats(), loadDashboard(), reloadStorageHeatmapIfReady()])
      await loadSelectedStorageDayLogs()
      resetStorageClearConfirm()
    } catch (error) {
      console.error('failed to clear log storage', error)
      showToast(t('components.logs.storage.failed', { error: extractErrorMessage(error) }), 'error')
      resetStorageClearConfirm()
    } finally {
      storageClearing.value = false
    }
  }

  const logsTableTextFormatters = buildLogsTableTextFormatters(t)

  const storageModalFormatters = {
    formatBytes,
    intensityClass,
    isSelectedStorageHeatmapDay,
    formatStorageHeatmapAriaLabel,
    formatTime,
    formatTokenNumber,
    formatCurrency,
    resolveStorageDayLogTotalTokens,
    httpCodeClass,
    formatStream: logsTableTextFormatters.formatStream,
    durationColor,
    formatDuration,
    formatStorageHeatmapPayloadValue,
  }

  const storageModalHandlers = {
    refreshStorageOverview,
    handleClearRequestLogs,
    handleClearRequestLogsByDate,
    handleClearStats,
    showStorageHeatmapTooltip,
    hideStorageHeatmapTooltip,
    selectStorageHeatmapDay,
    prevStorageDayLogsPage,
    nextStorageDayLogsPage,
    openPayloadDetailModal,
  }

  const disposeStorageModalController = () => {
    storageHeatmapReady = false
    hideStorageHeatmapTooltip()
    resetStorageDayLogs()
    cleanupStorageHeatmap()
    resetStorageClearConfirm()
  }

  return {
    storageStats,
    storageLoading,
    storageClearing,
    storageModal,
    openStorageModal,
    closeStorageModal,
    storageClearConfirm,
    storageClearConfirmMessage,
    storageClearConfirmActionLabel,
    closeStorageClearConfirm,
    confirmStorageClear,
    storageHeatmapLoading,
    storageHeatmap,
    selectedStorageHeatmapDay,
    selectedStorageHeatmapDateLabel,
    storageDayLogsShowingText,
    storageDayLogsLoading,
    storageDayLogs,
    pagedStorageDayLogs,
    storageDayLogsPage,
    storageDayLogsTotalPages,
    storageHeatmapHasData,
    storageHeatmapTooltip,
    bindStorageHeatmapContainerRef,
    bindStorageHeatmapTooltipRef,
    storageModalFormatters,
    storageModalHandlers,
    disposeStorageModalController,
  }
}
