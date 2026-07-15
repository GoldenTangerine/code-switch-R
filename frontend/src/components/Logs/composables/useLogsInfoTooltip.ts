import { nextTick, reactive, ref, type Ref } from 'vue'
import type { RequestLog } from '../../../services/logs'
import type { LogInfoTooltipDetail, TooltipPlacement } from '../types'

type UseLogsInfoTooltipOptions = {
  buildModelInfoTooltipDetail: (item: RequestLog) => LogInfoTooltipDetail
  buildVerifyInfoTooltipDetail: (item: RequestLog) => LogInfoTooltipDetail
  buildStreamInfoTooltipDetail: (item: RequestLog) => LogInfoTooltipDetail | null
  ensureModelPricingLoaded: (force?: boolean) => Promise<void>
  modelPricingLoaded: Ref<boolean>
  modelPricingStale: Ref<boolean>
  hidePeerTooltipImmediately?: () => void
}

const LOG_INFO_TOOLTIP_DEFAULT_WIDTH = 340
const LOG_INFO_TOOLTIP_DEFAULT_HEIGHT = 136
const LOG_INFO_TOOLTIP_VERTICAL_OFFSET = 10
const TOOLTIP_HORIZONTAL_MARGIN = 14
const TOOLTIP_VERTICAL_MARGIN = 20
const LOG_TOOLTIP_SHOW_DELAY_MS = 100
const LOG_INFO_TOOLTIP_HIDE_DELAY_MS = 90

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

const resolveTooltipAnchor = (event: MouseEvent | FocusEvent) =>
  event.currentTarget as HTMLElement | null

export function useLogsInfoTooltip(options: UseLogsInfoTooltipOptions) {
  const logInfoTooltipRef = ref<HTMLElement | null>(null)
  const logInfoTooltipAnchorRef = ref<HTMLElement | null>(null)
  const logInfoTooltipRequestId = ref(0)
  let logInfoTooltipHideTimer: number | null = null
  let logInfoTooltipShowTimer: number | null = null

  const logInfoTooltip = reactive<{
    visible: boolean
    left: number
    top: number
    placement: TooltipPlacement
    detail: LogInfoTooltipDetail | null
  }>({
    visible: false,
    left: 0,
    top: 0,
    placement: 'above',
    detail: null,
  })

  const getLogInfoTooltipSize = () => {
    const rect = logInfoTooltipRef.value?.getBoundingClientRect()
    return {
      width: rect?.width ?? LOG_INFO_TOOLTIP_DEFAULT_WIDTH,
      height: rect?.height ?? LOG_INFO_TOOLTIP_DEFAULT_HEIGHT,
    }
  }

  const clearLogInfoTooltipShowTimer = () => {
    if (logInfoTooltipShowTimer != null) {
      window.clearTimeout(logInfoTooltipShowTimer)
      logInfoTooltipShowTimer = null
    }
  }

  const clearLogInfoTooltipHideTimer = () => {
    if (logInfoTooltipHideTimer != null) {
      window.clearTimeout(logInfoTooltipHideTimer)
      logInfoTooltipHideTimer = null
    }
  }

  const hideLogInfoTooltipImmediately = () => {
    clearLogInfoTooltipShowTimer()
    clearLogInfoTooltipHideTimer()
    logInfoTooltipRequestId.value += 1
    logInfoTooltipAnchorRef.value = null
    logInfoTooltip.visible = false
    logInfoTooltip.detail = null
  }

  const scheduleHideLogInfoTooltip = () => {
    clearLogInfoTooltipShowTimer()
    clearLogInfoTooltipHideTimer()
    logInfoTooltipHideTimer = window.setTimeout(() => {
      hideLogInfoTooltipImmediately()
    }, LOG_INFO_TOOLTIP_HIDE_DELAY_MS)
  }

  const updateLogInfoTooltipPosition = (anchor: HTMLElement | null) => {
    if (!anchor) return
    const anchorRect = anchor.getBoundingClientRect()
    const { width: tooltipWidth, height: tooltipHeight } = getLogInfoTooltipSize()
    const { width: viewportWidth, height: viewportHeight } = getViewportSize()

    const centerX = anchorRect.left + anchorRect.width / 2
    const minLeft = TOOLTIP_HORIZONTAL_MARGIN + tooltipWidth / 2
    const maxLeft =
      viewportWidth > 0 ? viewportWidth - tooltipWidth / 2 - TOOLTIP_HORIZONTAL_MARGIN : centerX
    logInfoTooltip.left = clampToRange(centerX, minLeft, maxLeft)

    const canShowAbove = anchorRect.top - tooltipHeight - LOG_INFO_TOOLTIP_VERTICAL_OFFSET >= TOOLTIP_VERTICAL_MARGIN
    const shouldPlaceBelow = !canShowAbove
    logInfoTooltip.placement = shouldPlaceBelow ? 'below' : 'above'

    const desiredTop = shouldPlaceBelow
      ? anchorRect.bottom + LOG_INFO_TOOLTIP_VERTICAL_OFFSET
      : anchorRect.top - tooltipHeight - LOG_INFO_TOOLTIP_VERTICAL_OFFSET
    const maxTop = viewportHeight > 0 ? viewportHeight - tooltipHeight - TOOLTIP_VERTICAL_MARGIN : desiredTop
    logInfoTooltip.top = clampToRange(desiredTop, TOOLTIP_VERTICAL_MARGIN, maxTop)
  }

  const showLogInfoTooltip = async (detail: LogInfoTooltipDetail, target: HTMLElement | null) => {
    if (!target) return
    options.hidePeerTooltipImmediately?.()
    clearLogInfoTooltipShowTimer()
    clearLogInfoTooltipHideTimer()
    logInfoTooltipAnchorRef.value = target
    logInfoTooltip.detail = detail
    logInfoTooltip.visible = true
    updateLogInfoTooltipPosition(target)
    await nextTick()
    if (logInfoTooltipAnchorRef.value !== target) return
    updateLogInfoTooltipPosition(target)
  }

  const refreshModelInfoTooltipAfterPricingLoad = async (
    item: RequestLog,
    target: HTMLElement,
    requestId: number,
  ) => {
    const shouldRefreshPricing = !options.modelPricingLoaded.value || options.modelPricingStale.value
    if (!shouldRefreshPricing) return
    await options.ensureModelPricingLoaded(options.modelPricingStale.value)
    if (requestId !== logInfoTooltipRequestId.value) return
    if (!logInfoTooltip.visible) return
    if (logInfoTooltipAnchorRef.value !== target) return
    logInfoTooltip.detail = options.buildModelInfoTooltipDetail(item)
    updateLogInfoTooltipPosition(target)
    await nextTick()
    if (requestId !== logInfoTooltipRequestId.value) return
    if (logInfoTooltipAnchorRef.value !== target) return
    updateLogInfoTooltipPosition(target)
  }

  const showModelInfoTooltip = (item: RequestLog, event: MouseEvent | FocusEvent) => {
    const target = resolveTooltipAnchor(event)
    if (!target) return
    const requestId = ++logInfoTooltipRequestId.value
    void (async () => {
      await showLogInfoTooltip(options.buildModelInfoTooltipDetail(item), target)
      if (requestId !== logInfoTooltipRequestId.value) return
      await refreshModelInfoTooltipAfterPricingLoad(item, target, requestId)
    })()
  }

  const showVerifyInfoTooltip = (item: RequestLog, event: MouseEvent | FocusEvent) => {
    const target = resolveTooltipAnchor(event)
    logInfoTooltipRequestId.value += 1
    void showLogInfoTooltip(options.buildVerifyInfoTooltipDetail(item), target)
  }

  const showStreamInfoTooltip = (item: RequestLog, event: MouseEvent | FocusEvent) => {
    const detail = options.buildStreamInfoTooltipDetail(item)
    if (!detail) return
    const target = resolveTooltipAnchor(event)
    logInfoTooltipRequestId.value += 1
    void showLogInfoTooltip(detail, target)
  }

  const scheduleShowModelInfoTooltip = (item: RequestLog, event: MouseEvent) => {
    const target = resolveTooltipAnchor(event)
    if (!target) return
    clearLogInfoTooltipHideTimer()
    clearLogInfoTooltipShowTimer()
    logInfoTooltipShowTimer = window.setTimeout(() => {
      logInfoTooltipShowTimer = null
      const requestId = ++logInfoTooltipRequestId.value
      void (async () => {
        await showLogInfoTooltip(options.buildModelInfoTooltipDetail(item), target)
        if (requestId !== logInfoTooltipRequestId.value) return
        await refreshModelInfoTooltipAfterPricingLoad(item, target, requestId)
      })()
    }, LOG_TOOLTIP_SHOW_DELAY_MS)
  }

  const scheduleShowVerifyInfoTooltip = (item: RequestLog, event: MouseEvent) => {
    const target = resolveTooltipAnchor(event)
    if (!target) return
    clearLogInfoTooltipHideTimer()
    clearLogInfoTooltipShowTimer()
    logInfoTooltipShowTimer = window.setTimeout(() => {
      logInfoTooltipShowTimer = null
      logInfoTooltipRequestId.value += 1
      void showLogInfoTooltip(options.buildVerifyInfoTooltipDetail(item), target)
    }, LOG_TOOLTIP_SHOW_DELAY_MS)
  }

  const scheduleShowStreamInfoTooltip = (item: RequestLog, event: MouseEvent) => {
    const detail = options.buildStreamInfoTooltipDetail(item)
    const target = resolveTooltipAnchor(event)
    if (!detail || !target) return
    clearLogInfoTooltipHideTimer()
    clearLogInfoTooltipShowTimer()
    logInfoTooltipShowTimer = window.setTimeout(() => {
      logInfoTooltipShowTimer = null
      logInfoTooltipRequestId.value += 1
      void showLogInfoTooltip(detail, target)
    }, LOG_TOOLTIP_SHOW_DELAY_MS)
  }

  const moveLogInfoTooltip = (event: MouseEvent) => {
    if (!logInfoTooltip.visible) return
    clearLogInfoTooltipHideTimer()
    const target = event.currentTarget as HTMLElement | null
    if (!target) return
    logInfoTooltipAnchorRef.value = target
    updateLogInfoTooltipPosition(target)
  }

  const hideLogInfoTooltip = () => {
    scheduleHideLogInfoTooltip()
  }

  const handleLogInfoTooltipMouseEnter = () => {
    clearLogInfoTooltipHideTimer()
  }

  const handleLogInfoTooltipMouseLeave = () => {
    scheduleHideLogInfoTooltip()
  }

  return {
    logInfoTooltipRef,
    logInfoTooltip,
    showModelInfoTooltip,
    showVerifyInfoTooltip,
    showStreamInfoTooltip,
    scheduleShowModelInfoTooltip,
    scheduleShowVerifyInfoTooltip,
    scheduleShowStreamInfoTooltip,
    moveLogInfoTooltip,
    hideLogInfoTooltip,
    hideLogInfoTooltipImmediately,
    handleLogInfoTooltipMouseEnter,
    handleLogInfoTooltipMouseLeave,
  }
}
