/**
 * @name: 首页供应商 Tab 数据测试
 * @Descripttion: 验证首页供应商 Tab 的显隐、排序与活动项回退规则
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 10:55:39
 * @LastEditTime: 2026-08-17 10:55:39
 * @FilePath: frontend/src/data/homeProviderTabs.test.ts
 */

import { describe, expect, it } from 'vitest'
import {
  moveHomeProviderTab,
  reorderHomeProviderTabs,
  resolveHomeProviderTabOptions,
  resolveHomeProviderTabSelectionIndex,
  setHomeProviderTabVisibility,
} from './homeProviderTabs'

describe('homeProviderTabs', () => {
  it('resolves options in the persisted order', () => {
    expect(resolveHomeProviderTabOptions(['pi', 'claude', 'gemini']).map((tab) => tab.id)).toEqual([
      'pi',
      'claude',
      'gemini',
    ])
  })

  it('keeps at least one visible tab', () => {
    expect(setHomeProviderTabVisibility(['pi'], 'pi', false)).toEqual(['pi'])
    expect(setHomeProviderTabVisibility(['pi', 'claude'], 'pi', false)).toEqual(['claude'])
  })

  it('appends a re-enabled hidden tab to the end without duplicates', () => {
    expect(setHomeProviderTabVisibility(['claude', 'gemini'], 'pi', true)).toEqual([
      'claude',
      'gemini',
      'pi',
    ])
    expect(setHomeProviderTabVisibility(['claude', 'pi'], 'pi', true)).toEqual(['claude', 'pi'])
  })

  it('reorders tabs in both drag directions', () => {
    expect(reorderHomeProviderTabs(['claude', 'codex', 'gemini'], 'claude', 'gemini')).toEqual([
      'codex',
      'gemini',
      'claude',
    ])
    expect(reorderHomeProviderTabs(['claude', 'codex', 'gemini'], 'gemini', 'claude')).toEqual([
      'gemini',
      'claude',
      'codex',
    ])
  })

  it('moves tabs by keyboard offset and clamps list boundaries', () => {
    expect(moveHomeProviderTab(['claude', 'codex', 'gemini'], 'codex', -1)).toEqual([
      'codex',
      'claude',
      'gemini',
    ])
    expect(moveHomeProviderTab(['claude', 'codex', 'gemini'], 'codex', 1)).toEqual([
      'claude',
      'gemini',
      'codex',
    ])
    expect(moveHomeProviderTab(['claude', 'codex'], 'claude', -1)).toEqual(['claude', 'codex'])
  })

  it('preserves the active tab after sorting and selects the first tab after hiding it', () => {
    expect(resolveHomeProviderTabSelectionIndex(
      ['claude', 'codex', 'gemini'],
      ['gemini', 'claude', 'codex'],
      1,
    )).toBe(2)
    expect(resolveHomeProviderTabSelectionIndex(
      ['claude', 'codex', 'gemini'],
      ['claude', 'gemini'],
      1,
    )).toBe(0)
  })
})
