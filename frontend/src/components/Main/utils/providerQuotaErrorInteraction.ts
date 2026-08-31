/**
 * @name: 供应商额度错误悬浮交互
 * @Descripttion: 管理额度错误悬浮窗的延迟打开、关闭和点击锁定状态。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-18 12:51:00
 * @LastEditTime: 2026-08-18 12:51:00
 * @FilePath: frontend/src/components/Main/utils/providerQuotaErrorInteraction.ts
 */

import type { Ref } from 'vue'

export const PROVIDER_QUOTA_ERROR_HOVER_DELAY_MS = 100

type ProviderQuotaErrorPopoverInteractionOptions = {
  open: Ref<boolean>
  pinned: Ref<boolean>
  hovering: Ref<boolean>
  focusOnOpen?: Ref<boolean>
  openDelayMs?: number
  closeDelayMs?: number
  onClose?: () => void
}

export type ProviderQuotaErrorPopoverInteraction = {
  clear: () => void
  close: () => void
  enter: () => void
  leave: () => void
  toggle: () => void
  dispose: () => void
}

export const createProviderQuotaErrorPopoverInteraction = (
  options: ProviderQuotaErrorPopoverInteractionOptions,
): ProviderQuotaErrorPopoverInteraction => {
  let openTimer: ReturnType<typeof globalThis.setTimeout> | undefined
  let closeTimer: ReturnType<typeof globalThis.setTimeout> | undefined
  const openDelayMs = options.openDelayMs ?? PROVIDER_QUOTA_ERROR_HOVER_DELAY_MS
  const closeDelayMs = options.closeDelayMs ?? PROVIDER_QUOTA_ERROR_HOVER_DELAY_MS

  const clear = () => {
    if (openTimer !== undefined) {
      globalThis.clearTimeout(openTimer)
      openTimer = undefined
    }
    if (closeTimer !== undefined) {
      globalThis.clearTimeout(closeTimer)
      closeTimer = undefined
    }
  }

  const close = () => {
    clear()
    options.pinned.value = false
    if (options.focusOnOpen) options.focusOnOpen.value = false
    options.open.value = false
    options.onClose?.()
  }

  const enter = () => {
    options.hovering.value = true
    clear()
    if (options.open.value || options.pinned.value) return

    openTimer = globalThis.setTimeout(() => {
      openTimer = undefined
      if (!options.hovering.value) return
      if (options.focusOnOpen) options.focusOnOpen.value = false
      options.open.value = true
    }, openDelayMs)
  }

  const leave = () => {
    options.hovering.value = false
    clear()
    if (options.pinned.value) return

    closeTimer = globalThis.setTimeout(() => {
      closeTimer = undefined
      if (!options.hovering.value) close()
    }, closeDelayMs)
  }

  const toggle = () => {
    clear()
    if (options.open.value && options.pinned.value) {
      close()
      return
    }

    options.pinned.value = true
    if (options.focusOnOpen) options.focusOnOpen.value = true
    options.open.value = true
  }

  return {
    clear,
    close,
    enter,
    leave,
    toggle,
    dispose: clear,
  }
}

export const buildProviderQuotaErrorCopyPayload = <T>(
  items: T[],
  resolveNote: (item: T) => string,
  fallback: string,
) => items
  .map((item) => resolveNote(item) || fallback)
  .filter(Boolean)
  .join('\n')
