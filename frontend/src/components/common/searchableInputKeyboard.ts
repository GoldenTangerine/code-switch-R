/**
 * @name: 可搜索输入框键盘交互
 * @Descripttion: 区分组合框候选选择与自定义值提交的 Enter 行为
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-16 02:14:33
 * @LastEditTime: 2026-07-16 02:14:33
 * @FilePath: frontend/src/components/common/searchableInputKeyboard.ts
 */

interface SearchableInputTarget {
  getAttribute(name: string): string | null
}

export interface SearchableInputEnterEvent {
  currentTarget: EventTarget | null
  preventDefault(): void
}

function getActiveOptionId(target: EventTarget | null): string {
  const inputTarget = target as SearchableInputTarget | null
  if (!inputTarget || typeof inputTarget.getAttribute !== 'function') return ''
  return inputTarget.getAttribute('aria-activedescendant') || ''
}

export function handleSearchableInputEnter(
  event: SearchableInputEnterEvent,
  submit: () => void,
): boolean {
  // Headless UI 会跳过已被前置监听器 preventDefault 的内部按键处理。
  if (getActiveOptionId(event.currentTarget)) return false

  event.preventDefault()
  submit()
  return true
}
