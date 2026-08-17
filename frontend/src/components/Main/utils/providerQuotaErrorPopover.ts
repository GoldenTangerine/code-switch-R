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
): ProviderQuotaErrorPopoverLayout {
  const viewportWidth = Math.max(viewport.width, VIEWPORT_MARGIN * 2)
  const viewportHeight = Math.max(viewport.height, VIEWPORT_MARGIN * 2)
  const width = Math.min(POPOVER_MAX_WIDTH, viewportWidth - VIEWPORT_MARGIN * 2)
  const maxHeight = Math.min(POPOVER_MAX_HEIGHT, viewportHeight - VIEWPORT_MARGIN * 2)
  const measuredHeight = Math.min(Math.max(popover.height, 0), maxHeight)
  const belowTop = anchor.bottom + POPOVER_GAP
  const aboveTop = anchor.top - measuredHeight - POPOVER_GAP
  const availableBelow = viewportHeight - VIEWPORT_MARGIN - belowTop
  const availableAbove = anchor.top - POPOVER_GAP - VIEWPORT_MARGIN
  const canShowBelow = availableBelow >= measuredHeight
  const canShowAbove = availableAbove >= measuredHeight
  const placement: ProviderQuotaErrorPopoverPlacement = canShowBelow || (!canShowAbove && availableBelow >= availableAbove)
    ? 'below'
    : 'above'
  const desiredTop = placement === 'below' ? belowTop : aboveTop
  const maxTop = viewportHeight - measuredHeight - VIEWPORT_MARGIN
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
