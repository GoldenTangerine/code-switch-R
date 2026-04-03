import type { ProviderDragTarget } from '../types'

export type ProviderDragBounds = {
  id: number
  top: number
  height: number
  bottom?: number
}

export function resolveProviderDragTarget(boundsList: ProviderDragBounds[], pointerY: number): ProviderDragTarget | null {
  if (!boundsList.length) return null

  for (const bounds of boundsList) {
    const bottom = bounds.bottom ?? (bounds.top + bounds.height)
    const midpoint = bounds.top + (bottom - bounds.top) / 2

    if (pointerY <= midpoint) {
      return { id: bounds.id, position: 'before' }
    }

    if (pointerY <= bottom) {
      return { id: bounds.id, position: 'after' }
    }
  }

  const last = boundsList[boundsList.length - 1]
  return last ? { id: last.id, position: 'after' } : null
}
