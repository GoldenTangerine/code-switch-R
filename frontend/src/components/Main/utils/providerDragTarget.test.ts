import { describe, expect, it } from 'vitest'
import { resolveProviderDragTarget } from './providerDragTarget'

const bounds = [
  { id: 1, top: 100, height: 40 },
  { id: 2, top: 160, height: 40 },
  { id: 3, top: 220, height: 40 },
]

describe('resolveProviderDragTarget', () => {
  it('places above the first card when pointer is near the top', () => {
    expect(resolveProviderDragTarget(bounds, 90)).toEqual({ id: 1, position: 'before' })
  })

  it('uses card midpoint to switch between before and after', () => {
    expect(resolveProviderDragTarget(bounds, 115)).toEqual({ id: 1, position: 'before' })
    expect(resolveProviderDragTarget(bounds, 135)).toEqual({ id: 1, position: 'after' })
  })

  it('maps gaps between cards to the next card before position', () => {
    expect(resolveProviderDragTarget(bounds, 150)).toEqual({ id: 2, position: 'before' })
  })

  it('places after the last card when pointer is below the list', () => {
    expect(resolveProviderDragTarget(bounds, 280)).toEqual({ id: 3, position: 'after' })
  })

  it('returns null for an empty list', () => {
    expect(resolveProviderDragTarget([], 120)).toBeNull()
  })
})
