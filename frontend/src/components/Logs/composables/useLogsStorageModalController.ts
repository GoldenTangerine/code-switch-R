import { computed, reactive, ref, type Ref } from 'vue'
import { DEFAULT_HEATMAP_DISPLAY_SETTINGS, type HeatmapDisplaySettings } from '../../../data/heatmapDisplaySettings'
import {
  buildUsageHeatmapMatrixForRange,
  generateFallbackUsageHeatmapForRange,
  type UsageHeatmapDay,
} from '../../../data/usageHeatmap'
import {
  clearLogStats,
  clearRequestLogs,
  deleteRequestLogsByDate,
  fetchLogStorageStats,
  fetchRequestLogDailyHeatmapStatsByYear,
  fetchRequestLogHeatmapYears,
  type LogStorageStats,
  type RequestLog,
} from '../../../services/logs'
import { extractErrorMessage } from '../../../utils/error'
import { showToast } from '../../../utils/toast'
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

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

type UseLogsStorageModalControllerOptions = {
  locale: Ref<string>
  t: TranslateFn
  loadDashboard: () => Promise<void>
  openPayloadDetailModal: (item: RequestLog) => void | Promise<void>
}

type StorageClearTarget = 'requestLogs' | 'requestLogsByDate' | 'stats'

const STORAGE_HEATMAP_GRANULARITY = 'daily'

const getCurrentYear = () => new Date().getFullYear()

const isLeapYear = (year: number) => year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0)

const normalizeStorageHeatmapYear = (value: number | string) => {
  const parsed = typeof value === 'number' ? value : Number.parseInt(String(value ?? '').trim(), 10)
  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : getCurrentYear()
}

const buildStorageHeatmapYearRange = (year: number) => {
  const startDay = new Date(year, 0, 1, 0, 0, 0, 0)
  return {
    startDay,
    days: isLeapYear(year) ? 366 : 365,
  }
}

export function useLogsStorageModalController(options: UseLogsStorageModalControllerOptions) {
  const { locale, t, loadDashboard, openPayloadDetailModal } = options

  const storageStats = ref<LogStorageStats | null>(null)
  const storageLoading = ref(false)
  const storageClearing = ref(false)
  const storageModal = reactive({
    open: false,
  })

  const storageHeatmapDisplaySettings: HeatmapDisplaySettings = {
    ...DEFAULT_HEATMAP_DISPLAY_SETTINGS,
    intensityMetric: 'requests',
    dailyIntensityMode: 'daily_peak',
  }

  const storageHeatmapYear = ref(getCurrentYear())
  const storageHeatmapFetchedYears = ref<number[]>([])
  const storageHeatmap = ref<UsageHeatmapDay[][]>(
    generateFallbackUsageHeatmapForRange(
      buildStorageHeatmapYearRange(storageHeatmapYear.value),
      STORAGE_HEATMAP_GRANULARITY,
      storageHeatmapDisplaySettings,
    ),
  )
  const storageHeatmapLoading = ref(false)
  const storageHeatmapRequestId = ref(0)
  const storageHeatmapYearsRequestId = ref(0)
  let storageHeatmapReady = false

  const storageHeatmapYears = computed(() => {
    const years = new Set<number>([
      getCurrentYear(),
      storageHeatmapYear.value,
      ...storageHeatmapFetchedYears.value.map((year) => normalizeStorageHeatmapYear(year)),
    ])
    return [...years].sort((left, right) => right - left)
  })

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

  const buildStorageHeatmapFallback = (year = storageHeatmapYear.value) =>
    generateFallbackUsageHeatmapForRange(
      buildStorageHeatmapYearRange(normalizeStorageHeatmapYear(year)),
      STORAGE_HEATMAP_GRANULARITY,
      storageHeatmapDisplaySettings,
    )

  const primeStorageHeatmapState = (year = getCurrentYear()) => {
    const normalizedYear = normalizeStorageHeatmapYear(year)
    storageHeatmapYear.value = normalizedYear
    storageHeatmapFetchedYears.value = []
    storageHeatmap.value = buildStorageHeatmapFallback(normalizedYear)
    storageHeatmapLoading.value = false
  }

  const invalidateStorageHeatmapRequests = () => {
    storageHeatmapRequestId.value += 1
    storageHeatmapYearsRequestId.value += 1
    storageHeatmapLoading.value = false
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

  const loadStorageHeatmapYears = async () => {
    const requestId = ++storageHeatmapYearsRequestId.value
    try {
      const years = await fetchRequestLogHeatmapYears()
      if (requestId !== storageHeatmapYearsRequestId.value) return
      storageHeatmapFetchedYears.value = (Array.isArray(years) ? years : [])
        .map((year) => normalizeStorageHeatmapYear(year))
        .filter((year, index, items) => year > 0 && items.indexOf(year) === index)
        .sort((left, right) => right - left)
    } catch (error) {
      if (requestId !== storageHeatmapYearsRequestId.value) return
      console.error('failed to load request log heatmap years', error)
    }
  }

  const loadStorageHeatmap = async (year = storageHeatmapYear.value) => {
    const normalizedYear = normalizeStorageHeatmapYear(year)
    const range = buildStorageHeatmapYearRange(normalizedYear)
    const requestId = ++storageHeatmapRequestId.value
    storageHeatmapLoading.value = true
    storageHeatmap.value = generateFallbackUsageHeatmapForRange(
      range,
      STORAGE_HEATMAP_GRANULARITY,
      storageHeatmapDisplaySettings,
    )
    try {
      const stats = await fetchRequestLogDailyHeatmapStatsByYear(normalizedYear)
      if (requestId !== storageHeatmapRequestId.value || storageHeatmapYear.value !== normalizedYear) {
        return
      }
      storageHeatmap.value = buildUsageHeatmapMatrixForRange(
        stats,
        range,
        STORAGE_HEATMAP_GRANULARITY,
        storageHeatmapDisplaySettings,
      )
    } catch (error) {
      if (requestId !== storageHeatmapRequestId.value) return
      console.error('failed to load request log heatmap', error)
    } finally {
      if (requestId === storageHeatmapRequestId.value) {
        storageHeatmapLoading.value = false
      }
    }
  }

  const syncStorageHeatmapSelectionAndLogs = async () => {
    const selectionChanged = normalizeStorageHeatmapSelection({ autoLoad: false })
    await loadSelectedStorageDayLogs(selectionChanged ? 1 : storageDayLogsPage.value)
  }

  const ensureStorageHeatmapInitialized = async () => {
    await Promise.all([loadStorageHeatmapYears(), loadStorageHeatmap(storageHeatmapYear.value)])
    storageHeatmapReady = true
    await syncStorageHeatmapSelectionAndLogs()
  }

  const reloadStorageHeatmapIfReady = async () => {
    if (!storageHeatmapReady) return
    await Promise.all([loadStorageHeatmapYears(), loadStorageHeatmap(storageHeatmapYear.value)])
    await syncStorageHeatmapSelectionAndLogs()
  }

  const refreshStorageOverview = async () => {
    await Promise.all([loadStorageStats(), reloadStorageHeatmapIfReady()])
  }

  const updateStorageHeatmapYear = async (value: number | string) => {
    const nextYear = normalizeStorageHeatmapYear(value)
    if (nextYear === storageHeatmapYear.value) return
    storageHeatmapYear.value = nextYear
    selectedStorageHeatmapDate.value = ''
    hideStorageHeatmapTooltip()
    resetStorageDayLogs()
    if (!storageHeatmapReady) return
    await loadStorageHeatmap(nextYear)
    await syncStorageHeatmapSelectionAndLogs()
  }

  const openStorageModal = async () => {
    storageModal.open = true
    primeStorageHeatmapState(getCurrentYear())
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
    invalidateStorageHeatmapRequests()
    primeStorageHeatmapState(getCurrentYear())
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
    updateStorageHeatmapYear,
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
    resetStorageClearConfirm()
    invalidateStorageHeatmapRequests()
    primeStorageHeatmapState(getCurrentYear())
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
    storageHeatmapYear,
    storageHeatmapYears,
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
    bindStorageHeatmapTooltipRef,
    storageModalFormatters,
    storageModalHandlers,
    disposeStorageModalController,
  }
}
