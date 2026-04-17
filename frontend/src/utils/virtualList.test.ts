import { describe, expect, it } from 'vitest'
import { buildVariableHeightVirtualList } from './virtualList'

describe('virtualList', () => {
  it('uses estimated heights when no measurements are available', () => {
    const result = buildVariableHeightVirtualList({
      items: ['a', 'b', 'c', 'd', 'e'],
      getItemKey: (item) => item,
      estimatedItemHeight: 100,
      gap: 10,
      scrollTop: 0,
      viewportHeight: 220,
      overscan: 0,
    })

    expect(result.totalHeight).toBe(540)
    expect(result.items.map((item) => item.item)).toEqual(['a', 'b', 'c'])
    expect(result.items.map((item) => item.top)).toEqual([0, 110, 220])
  })

  it('respects measured heights and overscan range', () => {
    const result = buildVariableHeightVirtualList({
      items: ['a', 'b', 'c', 'd'],
      getItemKey: (item) => item,
      estimatedItemHeight: 100,
      measuredHeights: {
        a: 80,
        b: 120,
        c: 160,
      },
      gap: 8,
      scrollTop: 120,
      viewportHeight: 100,
      overscan: 40,
    })

    expect(result.totalHeight).toBe(484)
    expect(result.items.map((item) => item.item)).toEqual(['a', 'b', 'c'])
    expect(result.items.map((item) => item.top)).toEqual([0, 88, 216])
    expect(result.items.map((item) => item.height)).toEqual([80, 120, 160])
  })

  it('falls back to a minimal viewport window before the container is measured', () => {
    const result = buildVariableHeightVirtualList({
      items: Array.from({ length: 12 }, (_, index) => `item-${index + 1}`),
      getItemKey: (item) => item,
      estimatedItemHeight: 90,
      gap: 6,
      scrollTop: 0,
      viewportHeight: 0,
      overscan: 0,
    })

    expect(result.items.length).toBeGreaterThanOrEqual(6)
    expect(result.items[0]?.item).toBe('item-1')
  })
})
