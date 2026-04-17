export interface VirtualListItem<T> {
  item: T
  index: number
  top: number
  height: number
}

export interface VirtualListState<T> {
  items: VirtualListItem<T>[]
  totalHeight: number
}

export interface BuildVariableHeightVirtualListOptions<T> {
  items: T[]
  getItemKey: (item: T) => string
  measuredHeights?: Record<string, number>
  scrollTop?: number
  viewportHeight?: number
  estimatedItemHeight?: number
  overscan?: number
  gap?: number
}

export function buildVariableHeightVirtualList<T>(
  options: BuildVariableHeightVirtualListOptions<T>,
): VirtualListState<T> {
  const {
    items,
    getItemKey,
    measuredHeights = {},
    scrollTop = 0,
    viewportHeight = 0,
    estimatedItemHeight = 0,
    overscan = 0,
    gap = 0,
  } = options

  if (items.length === 0) {
    return {
      items: [],
      totalHeight: 0,
    }
  }

  const safeEstimatedHeight = Math.max(1, estimatedItemHeight)
  const safeScrollTop = Math.max(0, scrollTop)
  const safeViewportHeight = viewportHeight > 0 ? viewportHeight : safeEstimatedHeight * 6
  const safeOverscan = Math.max(0, overscan)
  const safeGap = Math.max(0, gap)
  const visibleStart = Math.max(0, safeScrollTop - safeOverscan)
  const visibleEnd = safeScrollTop + safeViewportHeight + safeOverscan

  const virtualItems: VirtualListItem<T>[] = []
  let cursor = 0

  for (let index = 0; index < items.length; index += 1) {
    const item = items[index]
    const key = getItemKey(item)
    const measuredHeight = measuredHeights[key]
    const itemHeight = Number.isFinite(measuredHeight) && measuredHeight > 0
      ? measuredHeight
      : safeEstimatedHeight
    const itemTop = cursor
    const itemBottom = itemTop + itemHeight

    if (itemBottom >= visibleStart && itemTop <= visibleEnd) {
      virtualItems.push({
        item,
        index,
        top: itemTop,
        height: itemHeight,
      })
    }

    cursor = itemBottom + (index < items.length - 1 ? safeGap : 0)
  }

  return {
    items: virtualItems,
    totalHeight: cursor,
  }
}
