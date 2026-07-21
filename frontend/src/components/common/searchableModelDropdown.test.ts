/**
 * @name: 可搜索模型下拉测试
 * @Descripttion: 验证下拉高度、视口定位与虚拟窗口计算
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-21 16:45:19
 * @LastEditTime: 2026-07-21 16:45:19
 * @FilePath: frontend/src/components/common/searchableModelDropdown.test.ts
 */

import { describe, expect, it } from 'vitest'
import {
  calculateModelDropdownHeight,
  calculateModelDropdownLayout,
  calculateNextModelOptionIndex,
  calculateVirtualOptionRange,
} from './searchableModelDropdown'

describe('calculateModelDropdownHeight', () => {
  it('候选充足时完整容纳至少八行', () => {
    expect(calculateModelDropdownHeight(8, '280px')).toBe(334)
    expect(calculateModelDropdownHeight(1000, '280px')).toBe(334)
  })

  it('候选较少时按实际内容收缩', () => {
    expect(calculateModelDropdownHeight(0, '280px')).toBe(60)
    expect(calculateModelDropdownHeight(5, '280px')).toBe(214)
  })
})

describe('calculateModelDropdownLayout', () => {
  it('下方空间不足时向上展开', () => {
    expect(calculateModelDropdownLayout(
      { top: 700, bottom: 740, left: 120, width: 300 },
      { width: 1000, height: 800 },
      334,
    )).toEqual({
      top: 358,
      left: 120,
      width: 300,
      height: 334,
      placement: 'above',
    })
  })

  it('两侧空间都不足时限制高度并保持视口边距', () => {
    expect(calculateModelDropdownLayout(
      { top: 120, bottom: 160, left: -10, width: 500 },
      { width: 360, height: 300 },
      334,
    )).toEqual({
      top: 168,
      left: 12,
      width: 336,
      height: 120,
      placement: 'below',
    })
  })
})

describe('calculateVirtualOptionRange', () => {
  it('长列表仅返回可视区域和缓冲项', () => {
    expect(calculateVirtualOptionRange(10_000, 4000, 320)).toEqual({
      start: 94,
      end: 114,
    })
  })

  it('在列表首尾正确收敛', () => {
    expect(calculateVirtualOptionRange(5, 0, 320)).toEqual({ start: 0, end: 5 })
    expect(calculateVirtualOptionRange(10_000, 399_800, 320)).toEqual({
      start: 9989,
      end: 10_000,
    })
  })
})

describe('calculateNextModelOptionIndex', () => {
  it('支持逐项导航和首尾导航', () => {
    expect(calculateNextModelOptionIndex(-1, 5, 'ArrowDown')).toBe(0)
    expect(calculateNextModelOptionIndex(0, 5, 'ArrowUp')).toBe(0)
    expect(calculateNextModelOptionIndex(-1, 5, 'ArrowUp')).toBe(4)
    expect(calculateNextModelOptionIndex(2, 5, 'Home')).toBe(0)
    expect(calculateNextModelOptionIndex(2, 5, 'End')).toBe(4)
  })

  it('空列表始终不激活候选', () => {
    expect(calculateNextModelOptionIndex(0, 0, 'ArrowDown')).toBe(-1)
    expect(calculateNextModelOptionIndex(0, 0, 'PageDown')).toBe(-1)
  })
})
