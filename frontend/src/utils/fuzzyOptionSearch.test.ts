/**
 * @name: 模型候选搜索测试
 * @Descripttion: 验证预索引搜索保持原有排序并支持大规模候选
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-21 16:14:23
 * @LastEditTime: 2026-07-21 16:14:23
 * @FilePath: frontend/src/utils/fuzzyOptionSearch.test.ts
 */

import { describe, expect, it } from 'vitest'
import {
  createStringOptionSearchIndex,
  filterAndSortStringOptionIndex,
  filterAndSortStringOptions,
  getCachedStringOptionSearchIndex,
} from './fuzzyOptionSearch'

describe('fuzzyOptionSearch index', () => {
  it('保持精确、前缀、包含和子序列匹配的原有排序', () => {
    const options = [
      'kimi-k2-thinking',
      'kimi-k3',
      'anthropic/claude-sonnet-4',
      'claude-sonnet-4-5',
      'gpt-5-codex',
      ' kimi-k3 ',
    ]
    const index = createStringOptionSearchIndex(options)

    for (const query of ['', 'k3', 'k2think', 'claude4', 'sonnet']) {
      expect(filterAndSortStringOptionIndex(index, query)).toEqual(
        filterAndSortStringOptions(options, query),
      )
    }
  })

  it('在一万条候选中保留完整列表并定位目标模型', () => {
    const options = Array.from({ length: 10_000 }, (_, index) => `vendor-model-${index}`)
    const searchIndex = createStringOptionSearchIndex(options)

    expect(filterAndSortStringOptionIndex(searchIndex, '')).toHaveLength(10_000)
    expect(filterAndSortStringOptionIndex(searchIndex, 'vendor-model-9876')[0]).toBe('vendor-model-9876')
  })

  it('候选数组原地修改后刷新共享索引', () => {
    const options = ['model-a', 'model-b']
    const initialIndex = getCachedStringOptionSearchIndex(options)

    options[1] = 'model-c'
    const updatedIndex = getCachedStringOptionSearchIndex(options)

    expect(updatedIndex).not.toBe(initialIndex)
    expect(filterAndSortStringOptionIndex(updatedIndex, '')).toEqual(['model-a', 'model-c'])
  })
})
