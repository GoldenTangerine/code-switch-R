/**
 * @name: 可搜索模型下拉布局
 * @Descripttion: 计算模型下拉的稳定高度、视口位置与虚拟窗口
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-21 16:45:19
 * @LastEditTime: 2026-07-21 16:45:19
 * @FilePath: frontend/src/components/common/searchableModelDropdown.ts
 */

export const MODEL_DROPDOWN_GAP = 8
export const MODEL_DROPDOWN_VIEWPORT_MARGIN = 12
export const MODEL_OPTION_HEIGHT = 40
export const MODEL_DROPDOWN_VERTICAL_PADDING = 12
export const MODEL_DROPDOWN_CHROME_HEIGHT = MODEL_DROPDOWN_VERTICAL_PADDING + 2
export const MODEL_DROPDOWN_MIN_VISIBLE_OPTIONS = 8
export const MODEL_DROPDOWN_EMPTY_HEIGHT = 60
export const MODEL_DROPDOWN_OVERSCAN = 6

const DEFAULT_MAX_HEIGHT = 280

export interface ModelDropdownAnchorRect {
  top: number
  bottom: number
  left: number
  width: number
}

export interface ModelDropdownViewport {
  width: number
  height: number
}

export interface ModelDropdownLayout {
  top: number
  left: number
  width: number
  height: number
  placement: 'above' | 'below'
}

export interface VirtualOptionRange {
  start: number
  end: number
}

export type ModelOptionNavigationKey =
  | 'ArrowDown'
  | 'ArrowUp'
  | 'Home'
  | 'End'
  | 'PageDown'
  | 'PageUp'

function parsePixelHeight(value: string): number {
  const match = value.trim().match(/^(\d+(?:\.\d+)?)px$/)
  if (!match) return DEFAULT_MAX_HEIGHT

  const parsed = Number(match[1])
  return Number.isFinite(parsed) && parsed > 0 ? parsed : DEFAULT_MAX_HEIGHT
}

export function calculateModelDropdownHeight(optionCount: number, maxHeight: string): number {
  if (optionCount <= 0) return MODEL_DROPDOWN_EMPTY_HEIGHT

  const contentHeight = optionCount * MODEL_OPTION_HEIGHT + MODEL_DROPDOWN_CHROME_HEIGHT
  const configuredMaxHeight = parsePixelHeight(maxHeight)
  const minimumHeight = MODEL_DROPDOWN_MIN_VISIBLE_OPTIONS * MODEL_OPTION_HEIGHT
    + MODEL_DROPDOWN_CHROME_HEIGHT
  const effectiveMaxHeight = optionCount >= MODEL_DROPDOWN_MIN_VISIBLE_OPTIONS
    ? Math.max(configuredMaxHeight, minimumHeight)
    : configuredMaxHeight

  return Math.min(contentHeight, effectiveMaxHeight)
}

export function calculateModelDropdownLayout(
  anchor: ModelDropdownAnchorRect,
  viewport: ModelDropdownViewport,
  desiredHeight: number,
): ModelDropdownLayout {
  const availableBelow = Math.max(
    0,
    viewport.height - MODEL_DROPDOWN_VIEWPORT_MARGIN - anchor.bottom - MODEL_DROPDOWN_GAP,
  )
  const availableAbove = Math.max(
    0,
    anchor.top - MODEL_DROPDOWN_GAP - MODEL_DROPDOWN_VIEWPORT_MARGIN,
  )
  const placement = availableBelow < desiredHeight && availableAbove > availableBelow
    ? 'above'
    : 'below'
  const availableHeight = placement === 'above' ? availableAbove : availableBelow
  const height = Math.min(desiredHeight, availableHeight)
  const maximumWidth = Math.max(0, viewport.width - MODEL_DROPDOWN_VIEWPORT_MARGIN * 2)
  const width = Math.min(anchor.width, maximumWidth)
  const left = Math.min(
    Math.max(anchor.left, MODEL_DROPDOWN_VIEWPORT_MARGIN),
    viewport.width - MODEL_DROPDOWN_VIEWPORT_MARGIN - width,
  )
  const top = placement === 'above'
    ? anchor.top - MODEL_DROPDOWN_GAP - height
    : anchor.bottom + MODEL_DROPDOWN_GAP

  return { top, left, width, height, placement }
}

export function calculateVirtualOptionRange(
  optionCount: number,
  scrollTop: number,
  viewportHeight: number,
): VirtualOptionRange {
  const start = Math.max(
    0,
    Math.floor(Math.max(0, scrollTop) / MODEL_OPTION_HEIGHT) - MODEL_DROPDOWN_OVERSCAN,
  )
  const end = Math.min(
    optionCount,
    Math.ceil((Math.max(0, scrollTop) + Math.max(0, viewportHeight)) / MODEL_OPTION_HEIGHT)
      + MODEL_DROPDOWN_OVERSCAN,
  )

  return { start, end: Math.max(start, end) }
}

export function calculateNextModelOptionIndex(
  currentIndex: number,
  optionCount: number,
  key: ModelOptionNavigationKey,
): number {
  if (optionCount <= 0) return -1

  const lastIndex = optionCount - 1
  if (key === 'ArrowDown') return Math.min(lastIndex, currentIndex + 1)
  if (key === 'ArrowUp') return currentIndex < 0 ? lastIndex : Math.max(0, currentIndex - 1)
  if (key === 'Home' || key === 'PageUp') return 0
  return lastIndex
}
