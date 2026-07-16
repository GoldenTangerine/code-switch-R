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
  handleSearchableInputEnter,
  type SearchableInputEnterEvent,
} from './searchableInputKeyboard'

function createEnterEvent(activeOptionId: string | null): {
  event: SearchableInputEnterEvent
  preventDefault: ReturnType<typeof vi.fn>
} {
  const preventDefault = vi.fn()
  const currentTarget = {
    getAttribute: vi.fn(() => activeOptionId),
  } as unknown as EventTarget

  return {
    event: { currentTarget, preventDefault },
    preventDefault,
  }
}

describe('handleSearchableInputEnter', () => {
  it('活动候选存在时保留 Headless UI 的 Enter 选择行为', () => {
    const { event, preventDefault } = createEnterEvent('headlessui-combobox-option-1')
    const submit = vi.fn()

    expect(handleSearchableInputEnter(event, submit)).toBe(false)
    expect(preventDefault).not.toHaveBeenCalled()
    expect(submit).not.toHaveBeenCalled()
  })

  it('没有活动候选时阻止默认行为并提交自定义值', () => {
    const { event, preventDefault } = createEnterEvent(null)
    const submit = vi.fn()

    expect(handleSearchableInputEnter(event, submit)).toBe(true)
    expect(preventDefault).toHaveBeenCalledOnce()
    expect(submit).toHaveBeenCalledOnce()
  })
})
