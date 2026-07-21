/**
 * @name: 可搜索输入框键盘交互
 * @Descripttion: 区分组合框候选选择与自定义值提交的 Enter 行为
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-16 02:14:33
 * @LastEditTime: 2026-07-16 02:14:33
 * @FilePath: frontend/src/components/common/searchableInputKeyboard.ts
 */

interface SearchableInputCompositionEvent {
  isComposing?: boolean
  keyCode?: number
}

export interface SearchableInputEnterEvent extends SearchableInputCompositionEvent {
  preventDefault(): void
}

export interface SearchableInputSelectionKeyEvent extends SearchableInputCompositionEvent {
  key: string
  shiftKey?: boolean
}

export interface SearchableInputPointerEvent {
  button: number
}

export interface SearchableInputBeforeInputEvent {
  inputType?: string
  preventDefault(): void
}

export interface SearchableInputSelectionState {
  hasKeyboardNavigation: boolean
  canSelectOption: boolean
}

export function createSearchableInputSelectionState(): SearchableInputSelectionState {
  return {
    hasKeyboardNavigation: false,
    canSelectOption: false,
  }
}

export function resetSearchableInputSelectionState(state: SearchableInputSelectionState): void {
  state.hasKeyboardNavigation = false
  state.canSelectOption = false
}

export function handleSearchableInputSelectionKeydown(
  state: SearchableInputSelectionState,
  event: SearchableInputSelectionKeyEvent,
): boolean {
  if (
    event.key === 'ArrowDown'
    || event.key === 'ArrowUp'
    || event.key === 'PageDown'
    || event.key === 'PageUp'
    || ((event.key === 'Home' || event.key === 'End') && !event.shiftKey)
  ) {
    state.hasKeyboardNavigation = true
    return false
  }

  if (event.key === 'Enter') {
    if (isSearchableInputComposing(event)) {
      resetSearchableInputSelectionState(state)
      return false
    }

    state.canSelectOption = state.hasKeyboardNavigation
    state.hasKeyboardNavigation = false
    return !state.canSelectOption
  }

  if (event.key === 'Tab' || event.key === 'Escape') {
    resetSearchableInputSelectionState(state)
  }

  return false
}

export function allowSearchableInputPointerSelection(
  state: SearchableInputSelectionState,
  event: SearchableInputPointerEvent,
): boolean {
  if (event.button !== 0) return false

  state.canSelectOption = true
  return true
}

export function consumeSearchableInputSelection(state: SearchableInputSelectionState): boolean {
  const canSelectOption = state.canSelectOption
  resetSearchableInputSelectionState(state)
  return canSelectOption
}

function isSearchableInputComposing(event: SearchableInputCompositionEvent): boolean {
  return event.isComposing === true || event.keyCode === 229
}

export function handleSearchableInputEnter(
  event: SearchableInputEnterEvent,
  submit: () => void,
): boolean {
  // Headless UI 会跳过已被前置监听器 preventDefault 的内部按键处理。
  if (isSearchableInputComposing(event)) return false

  event.preventDefault()
  submit()
  return true
}

export function handleSearchableInputBeforeInput(event: SearchableInputBeforeInputEvent): boolean {
  if (event.inputType !== 'insertReplacementText') return false

  event.preventDefault()
  return true
}
