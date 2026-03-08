import { computed, nextTick, reactive, ref, unref, watch, type ComponentPublicInstance, type Ref } from 'vue'
import { fetchRequestLogsPage, type RequestLog } from '../../../services/logs'
import { type UsageHeatmapDay } from '../../../data/usageHeatmap'
import { showToast } from '../../../utils/toast'
import { extractErrorMessage } from '../../../utils/error'
import { formatDateYmd, toDateParts, toTimeLayout } from '../utils'

type StorageHeatmapTooltipPlacement = 'above' | 'below'

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

type UseLogsStorageHeatmapOptions = {
  storageHeatmap: Ref<UsageHeatmapDay[][]>
  storageModalOpen: Ref<boolean>
  locale: Ref<string>
  t: TranslateFn
  formatNumber: (value?: number) => string
}

const STORAGE_DAY_LOGS_PAGE_SIZE = 10
const STORAGE_DAY_LOGS_PAGE_SIZE_OPTIONS = [10, 20, 50]
const STORAGE_HEATMAP_TOOLTIP_DEFAULT_WIDTH = 420
const STORAGE_HEATMAP_TOOLTIP_DEFAULT_HEIGHT = 196
const STORAGE_HEATMAP_TOOLTIP_VERTICAL_OFFSET = 10
const TOOLTIP_HORIZONTAL_MARGIN = 14
const TOOLTIP_VERTICAL_MARGIN = 20

type NormalizeStorageHeatmapSelectionOptions = {
  autoLoad?: boolean
}

const clampToRange = (value: number, min: number, max: number) => {
  if (max <= min) return min
  return Math.min(Math.max(value, min), max)
}

const getViewportSize = () => {
  if (typeof window !== 'undefined') {
    return { width: window.innerWidth, height: window.innerHeight }
  }
  if (typeof document !== 'undefined' && document.documentElement) {
    return {
      width: document.documentElement.clientWidth,
      height: document.documentElement.clientHeight,
    }
  }
  return { width: 0, height: 0 }
}

export function useLogsStorageHeatmap(options: UseLogsStorageHeatmapOptions) {
  const { storageHeatmap, storageModalOpen, locale, t, formatNumber } = options

  const selectedStorageHeatmapDate = ref('')
  const storageDayLogs = ref<RequestLog[]>([])
  const storageDayLogsTotalCount = ref(0)
  const storageDayLogsLoading = ref(false)
  const storageDayLogsPage = ref(1)
  const storageDayLogsPageSize = ref(STORAGE_DAY_LOGS_PAGE_SIZE)
  const storageDayLogsRequestId = ref(0)
  const storageHeatmapTooltipRef = ref<HTMLElement | null>(null)
  const storageHeatmapTooltipRequestId = ref(0)
  let storageHeatmapTooltipAnimationFrame = 0
  let storageHeatmapTooltipPendingRect: DOMRect | null = null

  const storageHeatmapTooltip = reactive<{
    visible: boolean
    positioned: boolean
    left: number
    top: number
    placement: StorageHeatmapTooltipPlacement
    label: string
    requests: number
    payloadBytes: number
    payloadCapturedRequests: number
    intensity: number
  }>({
    visible: false,
    positioned: false,
    left: 0,
    top: 0,
    placement: 'above',
    label: '',
    requests: 0,
    payloadBytes: 0,
    payloadCapturedRequests: 0,
    intensity: 0,
  })

  const bindStorageHeatmapTooltipRef = (element: Element | ComponentPublicInstance | null) => {
    storageHeatmapTooltipRef.value = (element instanceof HTMLElement
      ? element
      : (element as ComponentPublicInstance | null)?.$el instanceof HTMLElement
        ? (element as ComponentPublicInstance).$el
        : null)
  }

  const normalizedStorageDayLogsPageSize = computed(() => {
    const raw = Number(storageDayLogsPageSize.value)
    if (!Number.isFinite(raw) || raw <= 0) return STORAGE_DAY_LOGS_PAGE_SIZE
    const normalized = Math.max(1, Math.floor(raw))
    return STORAGE_DAY_LOGS_PAGE_SIZE_OPTIONS.includes(normalized) ? normalized : STORAGE_DAY_LOGS_PAGE_SIZE
  })

  const storageHeatmapDays = computed(() => storageHeatmap.value.flat())

  const storageHeatmapDayKey = (day: UsageHeatmapDay) => {
    const date = new Date(day.dateKey)
    if (!Number.isNaN(date.getTime())) {
      return formatDateYmd(date)
    }
    return ''
  }

  const storageHeatmapDateFormatter = computed(() =>
    new Intl.DateTimeFormat(locale.value || 'en', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    }),
  )

  const formatStorageHeatmapDateLabel = (value: string) => {
    const parts = toDateParts(value)
    if (!parts) return value
    return storageHeatmapDateFormatter.value.format(new Date(parts.y, parts.m - 1, parts.d, 0, 0, 0, 0))
  }

  const selectedStorageHeatmapDay = computed(
    () =>
      storageHeatmapDays.value.find(
        (day) => day.requests > 0 && storageHeatmapDayKey(day) === selectedStorageHeatmapDate.value,
      ) ?? null,
  )

  const selectedStorageHeatmapDateLabel = computed(() =>
    selectedStorageHeatmapDate.value ? formatStorageHeatmapDateLabel(selectedStorageHeatmapDate.value) : '',
  )

  const storageHeatmapHasData = computed(() => storageHeatmapDays.value.some((day) => day.requests > 0))

  const storageDayLogsTotalPages = computed(() => {
    const total = Math.ceil(storageDayLogsTotalCount.value / normalizedStorageDayLogsPageSize.value)
    return Math.max(total, 1)
  })

  const pagedStorageDayLogs = computed(() => storageDayLogs.value)

  const storageDayLogsRangeStart = computed(() =>
    storageDayLogsTotalCount.value > 0 ? (storageDayLogsPage.value - 1) * normalizedStorageDayLogsPageSize.value + 1 : 0,
  )

  const storageDayLogsRangeEnd = computed(() =>
    storageDayLogsTotalCount.value > 0
      ? storageDayLogsRangeStart.value + Math.max(storageDayLogs.value.length - 1, 0)
      : 0,
  )

  const storageDayLogsShowingText = computed(() => {
    const total = storageDayLogsTotalCount.value
    if (!Number.isFinite(total) || total <= 0 || storageDayLogs.value.length <= 0) return ''
    return t('components.logs.storage.dayLogsShowingRange', {
      start: formatNumber(storageDayLogsRangeStart.value),
      end: formatNumber(storageDayLogsRangeEnd.value),
      total: formatNumber(total),
    })
  })

  const buildStorageDayRange = (value: string) => {
    const parts = toDateParts(value)
    if (!parts) return null
    const start = new Date(parts.y, parts.m - 1, parts.d, 0, 0, 0, 0)
    const end = new Date(start.getTime())
    end.setDate(end.getDate() + 1)
    return {
      startAt: toTimeLayout(start),
      endAt: toTimeLayout(end),
    }
  }

  const resetStorageDayLogs = () => {
    storageDayLogsRequestId.value += 1
    storageDayLogsLoading.value = false
    storageDayLogsPage.value = 1
    storageDayLogsTotalCount.value = 0
    storageDayLogs.value = []
  }

  const loadSelectedStorageDayLogs = async (page = storageDayLogsPage.value) => {
    if (!unref(storageModalOpen) || !selectedStorageHeatmapDate.value) {
      resetStorageDayLogs()
      return
    }
    const range = buildStorageDayRange(selectedStorageHeatmapDate.value)
    if (!range) {
      resetStorageDayLogs()
      return
    }

    const requestId = ++storageDayLogsRequestId.value
    const normalizedPage = Math.max(1, Math.floor(page || 1))
    const pageSize = normalizedStorageDayLogsPageSize.value
    storageDayLogsLoading.value = true
    try {
      const result = await fetchRequestLogsPage({
        limit: pageSize,
        offset: (normalizedPage - 1) * pageSize,
        startAt: range.startAt,
        endAt: range.endAt,
      })
      if (requestId !== storageDayLogsRequestId.value) return
      const total = Math.max(0, Number(result?.total ?? 0))
      const items = result?.items ?? []
      const totalPages = Math.max(1, Math.ceil(total / pageSize))
      if (total > 0 && normalizedPage > totalPages) {
        storageDayLogsPage.value = totalPages
        storageDayLogsTotalCount.value = total
        void loadSelectedStorageDayLogs(totalPages)
        return
      }
      storageDayLogsPage.value = normalizedPage
      storageDayLogsTotalCount.value = total
      storageDayLogs.value = items
    } catch (error) {
      if (requestId !== storageDayLogsRequestId.value) return
      storageDayLogsTotalCount.value = 0
      storageDayLogs.value = []
      showToast(
        t('components.logs.storage.dayLogsLoadFailed', {
          error: extractErrorMessage(error),
        }),
        'warning',
      )
    } finally {
      if (requestId === storageDayLogsRequestId.value) {
        storageDayLogsLoading.value = false
      }
    }
  }

  const resolveStorageDayLogTotalTokens = (item: RequestLog) => {
    const inputTokens = Number(item.input_tokens ?? 0)
    const outputTokens = Number(item.output_tokens ?? 0)
    const cacheReadTokens = Number(item.cache_read_tokens ?? 0)
    const total = inputTokens + outputTokens + cacheReadTokens
    return Number.isFinite(total) && total > 0 ? total : 0
  }

  const formatStorageHeatmapAriaLabel = (day: UsageHeatmapDay) => {
    const key = storageHeatmapDayKey(day)
    const label = key ? formatStorageHeatmapDateLabel(key) : day.label
    return t('components.logs.storage.heatmapCellAria', {
      date: label,
      count: day.requests,
    })
  }

  const hideStorageHeatmapTooltip = () => {
    storageHeatmapTooltipRequestId.value += 1
    cancelStorageHeatmapTooltipAnimation()
    storageHeatmapTooltip.visible = false
    storageHeatmapTooltip.positioned = false
  }

  const selectStorageHeatmapDay = (day: UsageHeatmapDay) => {
    if (day.requests <= 0) return
    const nextDate = storageHeatmapDayKey(day)
    if (!nextDate) return
    if (nextDate === selectedStorageHeatmapDate.value) {
      void loadSelectedStorageDayLogs(storageDayLogsPage.value)
      return
    }
    selectedStorageHeatmapDate.value = nextDate
    storageDayLogsPage.value = 1
    hideStorageHeatmapTooltip()
    void loadSelectedStorageDayLogs(1)
  }

  const isSelectedStorageHeatmapDay = (day: UsageHeatmapDay) => {
    const key = storageHeatmapDayKey(day)
    return key !== '' && key === selectedStorageHeatmapDate.value
  }

  const normalizeStorageHeatmapSelection = (options: NormalizeStorageHeatmapSelectionOptions = {}) => {
    const autoLoad = options.autoLoad !== false
    if (
      selectedStorageHeatmapDate.value &&
      storageHeatmapDays.value.some(
        (day) => day.requests > 0 && storageHeatmapDayKey(day) === selectedStorageHeatmapDate.value,
      )
    ) {
      return false
    }
    const nextDay = [...storageHeatmapDays.value].reverse().find((day) => day.requests > 0)
    const nextDate = nextDay ? storageHeatmapDayKey(nextDay) : ''
    if (selectedStorageHeatmapDate.value === nextDate) {
      if (!nextDate) {
        resetStorageDayLogs()
      }
      return false
    }
    selectedStorageHeatmapDate.value = nextDate
    storageDayLogsPage.value = 1
    hideStorageHeatmapTooltip()
    if (autoLoad) {
      void loadSelectedStorageDayLogs(1)
    }
    return true
  }

  const applyStorageHeatmapTooltipMetrics = (day: UsageHeatmapDay) => {
    const key = storageHeatmapDayKey(day)
    storageHeatmapTooltip.label = key ? formatStorageHeatmapDateLabel(key) : day.label
    storageHeatmapTooltip.requests = day.requests
    storageHeatmapTooltip.payloadBytes = day.payloadBytes
    storageHeatmapTooltip.payloadCapturedRequests = day.payloadCapturedRequests
    storageHeatmapTooltip.intensity = day.intensity
  }

  const getStorageHeatmapTooltipSize = () => {
    const rect = storageHeatmapTooltipRef.value?.getBoundingClientRect()
    return {
      width: rect?.width ?? STORAGE_HEATMAP_TOOLTIP_DEFAULT_WIDTH,
      height: rect?.height ?? STORAGE_HEATMAP_TOOLTIP_DEFAULT_HEIGHT,
    }
  }

  const updateStorageHeatmapTooltipPosition = (anchorRect: DOMRect) => {
    const { width: tooltipWidth, height: tooltipHeight } = getStorageHeatmapTooltipSize()
    const { width: viewportWidth, height: viewportHeight } = getViewportSize()
    const centerX = anchorRect.left + anchorRect.width / 2
    const minLeft = TOOLTIP_HORIZONTAL_MARGIN + tooltipWidth / 2
    const maxLeft =
      viewportWidth > 0 ? viewportWidth - tooltipWidth / 2 - TOOLTIP_HORIZONTAL_MARGIN : centerX
    storageHeatmapTooltip.left = clampToRange(centerX, minLeft, maxLeft)

    const canShowAbove =
      anchorRect.top - tooltipHeight - STORAGE_HEATMAP_TOOLTIP_VERTICAL_OFFSET >= TOOLTIP_VERTICAL_MARGIN
    storageHeatmapTooltip.placement = canShowAbove ? 'above' : 'below'
    const desiredTop = canShowAbove
      ? anchorRect.top - tooltipHeight - STORAGE_HEATMAP_TOOLTIP_VERTICAL_OFFSET
      : anchorRect.bottom + STORAGE_HEATMAP_TOOLTIP_VERTICAL_OFFSET
    const maxTop =
      viewportHeight > 0 ? viewportHeight - tooltipHeight - TOOLTIP_VERTICAL_MARGIN : desiredTop
    storageHeatmapTooltip.top = clampToRange(desiredTop, TOOLTIP_VERTICAL_MARGIN, maxTop)
  }

  const cancelStorageHeatmapTooltipAnimation = () => {
    storageHeatmapTooltipPendingRect = null
    if (storageHeatmapTooltipAnimationFrame && typeof window !== 'undefined') {
      window.cancelAnimationFrame(storageHeatmapTooltipAnimationFrame)
    }
    storageHeatmapTooltipAnimationFrame = 0
  }

  const scheduleStorageHeatmapTooltipPosition = (anchorRect: DOMRect) => {
    storageHeatmapTooltipPendingRect = anchorRect
    if (typeof window === 'undefined') {
      updateStorageHeatmapTooltipPosition(anchorRect)
      storageHeatmapTooltip.positioned = true
      return
    }
    if (storageHeatmapTooltipAnimationFrame) return
    storageHeatmapTooltipAnimationFrame = window.requestAnimationFrame(() => {
      storageHeatmapTooltipAnimationFrame = 0
      const pendingRect = storageHeatmapTooltipPendingRect
      storageHeatmapTooltipPendingRect = null
      if (!storageHeatmapTooltip.visible || !pendingRect) return
      updateStorageHeatmapTooltipPosition(pendingRect)
      storageHeatmapTooltip.positioned = true
    })
  }

  const finalizeStorageHeatmapTooltipPosition = async (anchorRect: DOMRect, requestId: number) => {
    await nextTick()
    if (!storageHeatmapTooltip.visible || requestId !== storageHeatmapTooltipRequestId.value) return
    scheduleStorageHeatmapTooltipPosition(anchorRect)
  }

  const showStorageHeatmapTooltip = (day: UsageHeatmapDay, event: MouseEvent | FocusEvent) => {
    const target = event.currentTarget as HTMLElement | null
    if (!target) return
    const anchorRect = target.getBoundingClientRect()
    const isInitialRender = !storageHeatmapTooltip.visible
    const requestId = ++storageHeatmapTooltipRequestId.value
    applyStorageHeatmapTooltipMetrics(day)

    if (isInitialRender || !storageHeatmapTooltip.positioned) {
      storageHeatmapTooltip.visible = true
      storageHeatmapTooltip.positioned = false
      void finalizeStorageHeatmapTooltipPosition(anchorRect, requestId)
      return
    }

    scheduleStorageHeatmapTooltipPosition(anchorRect)
    void finalizeStorageHeatmapTooltipPosition(anchorRect, requestId)
  }

  const goToStorageDayLogsPage = (page: number) => {
    const normalizedPage = Math.max(1, Math.floor(Number(page) || 1))
    if (normalizedPage === storageDayLogsPage.value) {
      if (!storageDayLogsLoading.value) {
        void loadSelectedStorageDayLogs(normalizedPage)
      }
      return
    }
    void loadSelectedStorageDayLogs(normalizedPage)
  }

  const updateStorageDayLogsPageSize = (pageSize: number) => {
    const normalized = Number(pageSize)
    if (!Number.isFinite(normalized) || normalized <= 0) return
    const nextPageSize = Math.max(1, Math.floor(normalized))
    if (!STORAGE_DAY_LOGS_PAGE_SIZE_OPTIONS.includes(nextPageSize)) return
    if (nextPageSize === normalizedStorageDayLogsPageSize.value) return
    storageDayLogsPageSize.value = nextPageSize
    storageDayLogsPage.value = 1
    if (!unref(storageModalOpen) || !selectedStorageHeatmapDate.value) {
      resetStorageDayLogs()
      return
    }
    void loadSelectedStorageDayLogs(1)
  }

  watch(
    storageHeatmap,
    () => {
      if (!unref(storageModalOpen)) return
      normalizeStorageHeatmapSelection()
    },
    { deep: true },
  )

  return {
    selectedStorageHeatmapDate,
    storageDayLogs,
    storageDayLogsLoading,
    storageDayLogsPage,
    storageDayLogsPageSize: normalizedStorageDayLogsPageSize,
    storageDayLogsPageSizeOptions: STORAGE_DAY_LOGS_PAGE_SIZE_OPTIONS,
    storageHeatmapTooltip,
    bindStorageHeatmapTooltipRef,
    storageHeatmapDays,
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
    goToStorageDayLogsPage,
    updateStorageDayLogsPageSize,
  }
}
