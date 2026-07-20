/**
 * @name: 可搜索输入框键盘交互测试
 * @Descripttion: 验证 Enter 在候选选择与自定义值提交之间正确分流
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-16 02:14:33
 * @LastEditTime: 2026-07-16 02:14:33
 * @FilePath: frontend/src/components/common/searchableInputKeyboard.test.ts
 */

import { describe, expect, it, vi } from 'vitest'
import {
  allowSearchableInputPointerSelection,
  consumeSearchableInputSelection,
  createSearchableInputSelectionState,
  handleSearchableInputEnter,
  handleSearchableInputSelectionKeydown,
  resetSearchableInputSelectionState,
  type SearchableInputEnterEvent,
} from './searchableInputKeyboard'

function createEnterEvent(options: Partial<SearchableInputEnterEvent> = {}): {
  event: SearchableInputEnterEvent
  preventDefault: ReturnType<typeof vi.fn>
} {
  const preventDefault = vi.fn()

  return {
    event: { preventDefault, ...options },
    preventDefault,
  }
}

describe('handleSearchableInputEnter', () => {
  it('直接 Enter 时阻止 Headless UI 并提交自定义值', () => {
    const { event, preventDefault } = createEnterEvent()
    const submit = vi.fn()

    expect(handleSearchableInputEnter(event, submit)).toBe(true)
    expect(preventDefault).toHaveBeenCalledOnce()
    expect(submit).toHaveBeenCalledOnce()
  })

  it('输入法合成 Enter 时不提交自定义值', () => {
    const { event, preventDefault } = createEnterEvent({ isComposing: true })
    const submit = vi.fn()

    expect(handleSearchableInputEnter(event, submit)).toBe(false)
    expect(preventDefault).not.toHaveBeenCalled()
    expect(submit).not.toHaveBeenCalled()
  })

  it('兼容 keyCode 229 的输入法合成 Enter', () => {
    const { event, preventDefault } = createEnterEvent({ keyCode: 229 })
    const submit = vi.fn()

    expect(handleSearchableInputEnter(event, submit)).toBe(false)
    expect(preventDefault).not.toHaveBeenCalled()
    expect(submit).not.toHaveBeenCalled()
  })
})

describe('SearchableModelInput selection intent', () => {
  it('直接按 Enter 时保留输入值', () => {
    const state = createSearchableInputSelectionState()

    expect(handleSearchableInputSelectionKeydown(state, { key: 'Enter' })).toBe(true)
    expect(consumeSearchableInputSelection(state)).toBe(false)
  })

  it('方向键导航后按 Enter 时允许选择候选', () => {
    const state = createSearchableInputSelectionState()

    expect(handleSearchableInputSelectionKeydown(state, { key: 'ArrowDown' })).toBe(false)
    expect(handleSearchableInputSelectionKeydown(state, { key: 'Enter' })).toBe(false)
    expect(consumeSearchableInputSelection(state)).toBe(true)
    expect(consumeSearchableInputSelection(state)).toBe(false)
  })

  it('支持 Headless UI 的完整候选导航键', () => {
    for (const key of ['Home', 'End', 'PageUp', 'PageDown']) {
      const state = createSearchableInputSelectionState()

      expect(handleSearchableInputSelectionKeydown(state, { key })).toBe(false)
      expect(handleSearchableInputSelectionKeydown(state, { key: 'Enter' })).toBe(false)
      expect(consumeSearchableInputSelection(state)).toBe(true)
    }
  })

  it('Shift 与 Home 或 End 组合时不视为候选导航', () => {
    for (const key of ['Home', 'End']) {
      const state = createSearchableInputSelectionState()

      expect(handleSearchableInputSelectionKeydown(state, { key, shiftKey: true })).toBe(false)
      expect(handleSearchableInputSelectionKeydown(state, { key: 'Enter' })).toBe(true)
    }
  })

  it('方向键导航后按 Tab 时仍保留输入值', () => {
    const state = createSearchableInputSelectionState()

    handleSearchableInputSelectionKeydown(state, { key: 'ArrowUp' })
    handleSearchableInputSelectionKeydown(state, { key: 'Tab' })

    expect(consumeSearchableInputSelection(state)).toBe(false)
  })

  it('鼠标左键点击时允许选择候选', () => {
    const state = createSearchableInputSelectionState()

    handleSearchableInputSelectionKeydown(state, { key: 'ArrowDown' })
    allowSearchableInputPointerSelection(state)

    expect(consumeSearchableInputSelection(state)).toBe(true)
    expect(handleSearchableInputSelectionKeydown(state, { key: 'Enter' })).toBe(true)
  })

  it('失焦重置后直接 Enter 仍保留输入值', () => {
    const state = createSearchableInputSelectionState()

    handleSearchableInputSelectionKeydown(state, { key: 'ArrowDown' })
    resetSearchableInputSelectionState(state)

    expect(handleSearchableInputSelectionKeydown(state, { key: 'Enter' })).toBe(true)
  })

  it('输入法合成 Enter 不产生选择或提交意图', () => {
    const state = createSearchableInputSelectionState()
    handleSearchableInputSelectionKeydown(state, { key: 'ArrowDown' })

    expect(handleSearchableInputSelectionKeydown(state, { key: 'Enter', isComposing: true })).toBe(false)
    expect(consumeSearchableInputSelection(state)).toBe(false)
  })

  it('输入内容变化后清除之前的选择许可', () => {
    const state = createSearchableInputSelectionState()
    allowSearchableInputPointerSelection(state)

    resetSearchableInputSelectionState(state)

    expect(consumeSearchableInputSelection(state)).toBe(false)
  })
})
