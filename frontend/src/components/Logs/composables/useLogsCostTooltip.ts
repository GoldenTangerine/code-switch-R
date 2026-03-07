import { nextTick, reactive, ref, type Ref } from 'vue'
import type { RequestLog } from '../../../services/logs'
import type { CostTooltipDetail, TooltipPlacement } from '../types'

type UseLogsCostTooltipOptions = {
  buildCostTooltipDetail: (item: RequestLog) => CostTooltipDetail
  ensureModelPricingLoaded: () => Promise<void>
  modelPricingLoaded: Ref<boolean>
  hidePeerTooltipImmediately?: () => void
}

const COST_TOOLTIP_DEFAULT_WIDTH = 460
const COST_TOOLTIP_DEFAULT_HEIGHT = 236
const COST_TOOLTIP_VERTICAL_OFFSET = 12
const TOOLTIP_HORIZONTAL_MARGIN = 14
const TOOLTIP_VERTICAL_MARGIN = 20
const LOG_TOOLTIP_SHOW_DELAY_MS = 100
const COST_TOOLTIP_HIDE_DELAY_MS = 80

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

export function useLogsCostTooltip(options: UseLogsCostTooltipOptions) {
  const costTooltipRef = ref<HTMLElement | null>(null)
  const costTooltipAnchorRef = ref<HTMLElement | null>(null)
  const costTooltipRequestId = ref(0)
  let costTooltipHideTimer: number | null = null
  let costTooltipShowTimer: number | null = null

  const costTooltip = reactive<{
    visible: boolean
    left: number
    top: number
    placement: TooltipPlacement
    detail: CostTooltipDetail | null
  }>({
    visible: false,
    left: 0,
    top: 0,
    placement: 'above',
    detail: null,
  })

  const getCostTooltipSize = () => {
    const rect = costTooltipRef.value?.getBoundingClientRect()
    return {
      width: rect?.width ?? COST_TOOLTIP_DEFAULT_WIDTH,
      height: rect?.height ?? COST_TOOLTIP_DEFAULT_HEIGHT,
    }
  }

  const clearCostTooltipShowTimer = () => {
    if (costTooltipShowTimer != null) {
      window.clearTimeout(costTooltipShowTimer)
      costTooltipShowTimer = null
    }
  }

  const clearCostTooltipHideTimer = () => {
    if (costTooltipHideTimer != null) {
      window.clearTimeout(costTooltipHideTimer)
      costTooltipHideTimer = null
    }
  }

  const hideCostTooltipImmediately = () => {
    clearCostTooltipShowTimer()
    clearCostTooltipHideTimer()
    costTooltipRequestId.value += 1
    costTooltipAnchorRef.value = null
    costTooltip.visible = false
    costTooltip.detail = null
  }

  const scheduleHideCostTooltip = () => {
    clearCostTooltipShowTimer()
    clearCostTooltipHideTimer()
    costTooltipHideTimer = window.setTimeout(() => {
      hideCostTooltipImmediately()
    }, COST_TOOLTIP_HIDE_DELAY_MS)
  }

  const updateCostTooltipPosition = (anchor: HTMLElement | null) => {
    if (!anchor) return
    const anchorRect = anchor.getBoundingClientRect()
    const { width: tooltipWidth, height: tooltipHeight } = getCostTooltipSize()
    const { width: viewportWidth, height: viewportHeight } = getViewportSize()

    const centerX = anchorRect.left + anchorRect.width / 2
    const minLeft = TOOLTIP_HORIZONTAL_MARGIN + tooltipWidth / 2
    const maxLeft = viewportWidth > 0 ? viewportWidth - tooltipWidth / 2 - TOOLTIP_HORIZONTAL_MARGIN : centerX
    costTooltip.left = clampToRange(centerX, minLeft, maxLeft)

    const canShowAbove = anchorRect.top - tooltipHeight - COST_TOOLTIP_VERTICAL_OFFSET >= TOOLTIP_VERTICAL_MARGIN
    const shouldPlaceBelow = !canShowAbove
    costTooltip.placement = shouldPlaceBelow ? 'below' : 'above'

    const desiredTop = shouldPlaceBelow
      ? anchorRect.bottom + COST_TOOLTIP_VERTICAL_OFFSET
      : anchorRect.top - tooltipHeight - COST_TOOLTIP_VERTICAL_OFFSET
    const maxTop = viewportHeight > 0 ? viewportHeight - tooltipHeight - TOOLTIP_VERTICAL_MARGIN : desiredTop
    costTooltip.top = clampToRange(desiredTop, TOOLTIP_VERTICAL_MARGIN, maxTop)
  }

  const showCostTooltipByAnchor = async (item: RequestLog, target: HTMLElement | null) => {
    if (!target) return
    options.hidePeerTooltipImmediately?.()
    clearCostTooltipShowTimer()
    clearCostTooltipHideTimer()
    costTooltipAnchorRef.value = target
    const requestId = ++costTooltipRequestId.value
    costTooltip.detail = options.buildCostTooltipDetail(item)
    costTooltip.visible = true
    updateCostTooltipPosition(target)
    await nextTick()
    if (requestId !== costTooltipRequestId.value) return
    if (costTooltipAnchorRef.value !== target) return
    updateCostTooltipPosition(target)
    if (options.modelPricingLoaded.value) return
    await options.ensureModelPricingLoaded()
    if (requestId !== costTooltipRequestId.value) return
    if (costTooltipAnchorRef.value !== target) return
    costTooltip.detail = options.buildCostTooltipDetail(item)
    updateCostTooltipPosition(target)
    await nextTick()
    if (requestId !== costTooltipRequestId.value) return
    if (costTooltipAnchorRef.value !== target) return
    updateCostTooltipPosition(target)
  }

  const showCostTooltip = (item: RequestLog, event: MouseEvent | FocusEvent) => {
    const target = resolveTooltipAnchor(event)
    void showCostTooltipByAnchor(item, target)
  }

  const scheduleShowCostTooltip = (item: RequestLog, event: MouseEvent) => {
    const target = resolveTooltipAnchor(event)
    if (!target) return
    clearCostTooltipHideTimer()
    clearCostTooltipShowTimer()
    costTooltipShowTimer = window.setTimeout(() => {
      costTooltipShowTimer = null
      void showCostTooltipByAnchor(item, target)
    }, LOG_TOOLTIP_SHOW_DELAY_MS)
  }

  const moveCostTooltip = (event: MouseEvent) => {
    if (!costTooltip.visible) return
    clearCostTooltipHideTimer()
    const target = event.currentTarget as HTMLElement | null
    if (!target) return
    costTooltipAnchorRef.value = target
    updateCostTooltipPosition(target)
  }

  const hideCostTooltip = () => {
    scheduleHideCostTooltip()
  }

  const handleCostTooltipMouseEnter = () => {
    clearCostTooltipHideTimer()
  }

  const handleCostTooltipMouseLeave = () => {
    scheduleHideCostTooltip()
  }

  return {
    costTooltipRef,
    costTooltip,
    showCostTooltip,
    scheduleShowCostTooltip,
    moveCostTooltip,
    hideCostTooltip,
    hideCostTooltipImmediately,
    handleCostTooltipMouseEnter,
    handleCostTooltipMouseLeave,
  }
}
