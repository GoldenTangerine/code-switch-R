/**
 * @name: 供应商额度错误悬浮交互测试
 * @Descripttion: 验证额度错误悬浮窗的延迟、锁定、关闭和复制内容规则。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-18 12:53:00
 * @LastEditTime: 2026-08-18 12:53:00
 * @FilePath: frontend/src/components/Main/utils/providerQuotaErrorInteraction.test.ts
 */

import { ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  buildProviderQuotaErrorCopyPayload,
  createProviderQuotaErrorPopoverInteraction,
  PROVIDER_QUOTA_ERROR_HOVER_DELAY_MS,
} from './providerQuotaErrorInteraction'

const createInteraction = () => {
  const open = ref(false)
  const pinned = ref(false)
  const hovering = ref(false)
  const focusOnOpen = ref(false)
  const onClose = vi.fn()
  const interaction = createProviderQuotaErrorPopoverInteraction({
    open,
    pinned,
    hovering,
    focusOnOpen,
    onClose,
  })
  return { open, pinned, hovering, focusOnOpen, onClose, interaction }
}

describe('provider quota error interaction', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('opens and closes after the 100ms hover delay', () => {
    const { open, interaction } = createInteraction()

    interaction.enter()
    vi.advanceTimersByTime(PROVIDER_QUOTA_ERROR_HOVER_DELAY_MS - 1)
    expect(open.value).toBe(false)

    vi.advanceTimersByTime(1)
    expect(open.value).toBe(true)

    interaction.leave()
    vi.advanceTimersByTime(PROVIDER_QUOTA_ERROR_HOVER_DELAY_MS - 1)
    expect(open.value).toBe(true)
    vi.advanceTimersByTime(1)
    expect(open.value).toBe(false)
  })

  it('cancels a pending hover open when the pointer leaves early', () => {
    const { open, interaction } = createInteraction()

    interaction.enter()
    interaction.leave()
    vi.advanceTimersByTime(PROVIDER_QUOTA_ERROR_HOVER_DELAY_MS)

    expect(open.value).toBe(false)
  })

  it('keeps a clicked or touch-opened popover pinned until toggled closed', () => {
    const { open, pinned, focusOnOpen, interaction } = createInteraction()

    interaction.toggle()
    expect(open.value).toBe(true)
    expect(pinned.value).toBe(true)
    expect(focusOnOpen.value).toBe(true)

    interaction.leave()
    vi.advanceTimersByTime(PROVIDER_QUOTA_ERROR_HOVER_DELAY_MS)
    expect(open.value).toBe(true)

    interaction.toggle()
    expect(open.value).toBe(false)
    expect(pinned.value).toBe(false)
  })

  it('builds a complete newline-separated copy payload', () => {
    const payload = buildProviderQuotaErrorCopyPayload(
      [{ message: 'timeout' }, { message: '' }, { message: 'unauthorized' }],
      (item) => item.message,
      'Quota query failed',
    )

    expect(payload).toBe('timeout\nQuota query failed\nunauthorized')
  })
})
