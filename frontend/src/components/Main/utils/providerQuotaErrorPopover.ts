/**
 * @name: 额度错误悬浮窗布局
 * @Descripttion: 计算供应商额度错误悬浮窗在视口内的位置和尺寸
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 17:03:00
 * @LastEditTime: 2026-08-17 17:03:00
 * @FilePath: frontend/src/components/Main/utils/providerQuotaErrorPopover.ts
 */

export type ProviderQuotaErrorPopoverPlacement = 'above' | 'below'

export interface ProviderQuotaErrorPopoverRect {
  top: number
  bottom: number
  left: number
  width: number
  height: number
}

export interface ProviderQuotaErrorPopoverLayout {
  top: number
  left: number
  width: number
  maxHeight: number
  placement: ProviderQuotaErrorPopoverPlacement
}

export interface ProviderQuotaErrorPopoverConstraints {
  maxWidth?: number
  maxHeight?: number
}

const POPOVER_MAX_WIDTH = 420
const POPOVER_MAX_HEIGHT = 240
const POPOVER_GAP = 8
const VIEWPORT_MARGIN = 12

function clamp(value: number, min: number, max: number): number {
  if (max <= min) return min
  return Math.min(Math.max(value, min), max)
}

export function calculateProviderQuotaErrorPopoverLayout(
  anchor: ProviderQuotaErrorPopoverRect,
  popover: Pick<ProviderQuotaErrorPopoverRect, 'width' | 'height'>,
  viewport: { width: number; height: number },
  constraints: ProviderQuotaErrorPopoverConstraints = {},
): ProviderQuotaErrorPopoverLayout {
  const viewportWidth = Math.max(viewport.width, VIEWPORT_MARGIN * 2)
  const viewportHeight = Math.max(viewport.height, VIEWPORT_MARGIN * 2)
  const requestedMaxWidth = constraints.maxWidth ?? POPOVER_MAX_WIDTH
  const requestedMaxHeight = constraints.maxHeight ?? POPOVER_MAX_HEIGHT
  const width = Math.min(requestedMaxWidth, viewportWidth - VIEWPORT_MARGIN * 2)
  const viewportMaxHeight = Math.min(requestedMaxHeight, viewportHeight - VIEWPORT_MARGIN * 2)
  const measuredHeight = Math.min(Math.max(popover.height, 0), viewportMaxHeight)
  const belowTop = anchor.bottom + POPOVER_GAP
  const availableBelow = Math.max(0, viewportHeight - VIEWPORT_MARGIN - belowTop)
  const availableAbove = Math.max(0, anchor.top - POPOVER_GAP - VIEWPORT_MARGIN)
  const canShowBelow = availableBelow >= measuredHeight
  const canShowAbove = availableAbove >= measuredHeight
  const placement: ProviderQuotaErrorPopoverPlacement = canShowBelow || (!canShowAbove && availableBelow >= availableAbove)
    ? 'below'
    : 'above'
  const availableHeight = placement === 'below' ? availableBelow : availableAbove
  const maxHeight = Math.min(viewportMaxHeight, availableHeight)
  const displayedHeight = Math.min(measuredHeight, maxHeight)
  const desiredTop = placement === 'below' ? belowTop : anchor.top - displayedHeight - POPOVER_GAP
  const maxTop = viewportHeight - displayedHeight - VIEWPORT_MARGIN
  const desiredLeft = anchor.left + anchor.width / 2 - width / 2
  const maxLeft = viewportWidth - width - VIEWPORT_MARGIN

  return {
    top: clamp(desiredTop, VIEWPORT_MARGIN, maxTop),
    left: clamp(desiredLeft, VIEWPORT_MARGIN, maxLeft),
    width,
    maxHeight,
    placement,
  }
}
