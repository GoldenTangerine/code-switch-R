/**
 * @name: 主页面轮询生命周期测试
 * @Descripttion: 验证主页面轮询仅在窗口可见时运行，并正确处理恢复、重复信号和卸载
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-31 19:00:40
 * @LastEditTime: 2026-08-31 19:00:40
 * @FilePath: frontend/src/components/Main/composables/mainPollingLifecycle.test.ts
 */

import { afterEach, describe, expect, it, vi } from 'vitest'
import { bindMainPollingVisibility, createMainPollingLifecycle } from './mainPollingLifecycle'

const flushPromises = async () => {
  await Promise.resolve()
  await Promise.resolve()
}

function createPollingHarness(initialVisible = true, refresh = vi.fn(async () => {})) {
  let pollingRounds = 0
  let timers: Array<ReturnType<typeof globalThis.setInterval>> = []
  const startPolling = vi.fn(() => {
    timers = [2_000, 10_000, 60_000, 30_000, 30_000].map((intervalMs) => (
      globalThis.setInterval(() => {
        pollingRounds++
      }, intervalMs)
    ))
  })
  const stopPolling = vi.fn(() => {
    timers.forEach((timer) => globalThis.clearInterval(timer))
    timers = []
  })
  const lifecycle = createMainPollingLifecycle({
    initialVisible,
    startPolling,
    stopPolling,
    refresh,
  })

  return {
    lifecycle,
    refresh,
    startPolling,
    stopPolling,
    getPollingRounds: () => pollingRounds,
  }
}

afterEach(() => {
  vi.useRealTimers()
})

describe('createMainPollingLifecycle', () => {
  it('keeps the visible schedule and adds no periodic rounds while hidden', async () => {
    vi.useFakeTimers()
    const harness = createPollingHarness()

    harness.lifecycle.start()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(harness.getPollingRounds()).toBe(41)

    harness.lifecycle.setVisible(false)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(harness.getPollingRounds()).toBe(41)

    harness.lifecycle.setVisible(true)
    await flushPromises()
    expect(harness.refresh).toHaveBeenCalledTimes(1)
    expect(harness.startPolling).toHaveBeenCalledTimes(2)

    await vi.advanceTimersByTimeAsync(60_000)
    expect(harness.getPollingRounds()).toBe(82)

    harness.lifecycle.dispose()
  })

  it('adds no periodic rounds while deactivated and refreshes once when activated', async () => {
    vi.useFakeTimers()
    const harness = createPollingHarness()

    harness.lifecycle.start()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(harness.getPollingRounds()).toBe(41)

    harness.lifecycle.setPageActive(false)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(harness.getPollingRounds()).toBe(41)

    harness.lifecycle.setPageActive(true)
    harness.lifecycle.setPageActive(true)
    await flushPromises()
    expect(harness.refresh).toHaveBeenCalledTimes(1)
    expect(harness.startPolling).toHaveBeenCalledTimes(2)

    await vi.advanceTimersByTimeAsync(60_000)
    expect(harness.getPollingRounds()).toBe(82)

    harness.lifecycle.dispose()
  })

  it('does not refresh for the initial activation signal', async () => {
    const harness = createPollingHarness()

    harness.lifecycle.setPageActive(true)
    harness.lifecycle.start()
    harness.lifecycle.setPageActive(true)
    await flushPromises()

    expect(harness.refresh).not.toHaveBeenCalled()
    expect(harness.startPolling).toHaveBeenCalledTimes(1)

    harness.lifecycle.dispose()
  })

  it('does not restart after being deactivated during route activation refresh', async () => {
    let resolveRefresh: (() => void) | undefined
    const refresh = vi.fn(() => new Promise<void>((resolve) => {
      resolveRefresh = resolve
    }))
    const harness = createPollingHarness(true, refresh)

    harness.lifecycle.start()
    harness.lifecycle.setPageActive(false)
    harness.lifecycle.setPageActive(true)
    expect(refresh).toHaveBeenCalledTimes(1)

    harness.lifecycle.setPageActive(false)
    resolveRefresh?.()
    await flushPromises()

    expect(harness.startPolling).toHaveBeenCalledTimes(1)
    expect(harness.lifecycle.isActive()).toBe(false)

    harness.lifecycle.dispose()
  })

  it('requires both visibility and route activation before resuming', async () => {
    const harness = createPollingHarness()

    harness.lifecycle.start()
    harness.lifecycle.setPageActive(false)
    harness.lifecycle.setVisible(false)
    harness.lifecycle.setVisible(true)
    await flushPromises()
    expect(harness.refresh).not.toHaveBeenCalled()
    expect(harness.startPolling).toHaveBeenCalledTimes(1)

    harness.lifecycle.setPageActive(true)
    await flushPromises()
    expect(harness.refresh).toHaveBeenCalledTimes(1)
    expect(harness.startPolling).toHaveBeenCalledTimes(2)

    harness.lifecycle.dispose()
  })

  it('deduplicates visibility signals and does not restart after being hidden during refresh', async () => {
    let resolveRefresh: (() => void) | undefined
    const refresh = vi.fn(() => new Promise<void>((resolve) => {
      resolveRefresh = resolve
    }))
    const harness = createPollingHarness(true, refresh)

    harness.lifecycle.start()
    harness.lifecycle.setVisible(false)
    harness.lifecycle.setVisible(true)
    harness.lifecycle.setVisible(true)

    expect(refresh).toHaveBeenCalledTimes(1)
    expect(harness.startPolling).toHaveBeenCalledTimes(1)

    harness.lifecycle.setVisible(false)
    resolveRefresh?.()
    await flushPromises()

    expect(harness.startPolling).toHaveBeenCalledTimes(1)
    expect(harness.lifecycle.isActive()).toBe(false)

    harness.lifecycle.dispose()
  })

  it('refreshes before starting polling when first shown after hidden initialization', async () => {
    const harness = createPollingHarness(false)

    harness.lifecycle.start()
    expect(harness.startPolling).not.toHaveBeenCalled()

    harness.lifecycle.setVisible(true)
    expect(harness.refresh).toHaveBeenCalledTimes(1)
    expect(harness.startPolling).not.toHaveBeenCalled()

    await flushPromises()
    expect(harness.startPolling).toHaveBeenCalledTimes(1)
    expect(harness.lifecycle.isActive()).toBe(true)

    harness.lifecycle.dispose()
  })

  it('stops once and ignores later visibility changes after disposal', async () => {
    const harness = createPollingHarness()

    harness.lifecycle.start()
    harness.lifecycle.dispose()
    harness.lifecycle.dispose()
    harness.lifecycle.setVisible(false)
    harness.lifecycle.setVisible(true)
    await flushPromises()

    expect(harness.stopPolling).toHaveBeenCalledTimes(1)
    expect(harness.startPolling).toHaveBeenCalledTimes(1)
    expect(harness.refresh).not.toHaveBeenCalled()
    expect(harness.lifecycle.isActive()).toBe(false)
  })

  it('binds Wails and document visibility signals and removes every listener once', async () => {
    const harness = createPollingHarness()
    let hidden = false
    let hideHandler: (() => void) | undefined
    let showHandler: (() => void) | undefined
    let visibilityHandler: (() => void) | undefined
    const unsubscribeHide = vi.fn()
    const unsubscribeShow = vi.fn()
    const removeVisibilityListener = vi.fn()

    harness.lifecycle.start()
    const unbind = bindMainPollingVisibility(harness.lifecycle, {
      onWindowHide: (handler) => {
        hideHandler = handler
        return unsubscribeHide
      },
      onWindowShow: (handler) => {
        showHandler = handler
        return unsubscribeShow
      },
      addVisibilityListener: (handler) => {
        visibilityHandler = handler
      },
      removeVisibilityListener,
      isDocumentHidden: () => hidden,
    })

    hideHandler?.()
    expect(harness.lifecycle.isActive()).toBe(false)

    showHandler?.()
    await flushPromises()
    expect(harness.refresh).toHaveBeenCalledTimes(1)
    expect(harness.lifecycle.isActive()).toBe(true)

    hidden = true
    visibilityHandler?.()
    expect(harness.lifecycle.isActive()).toBe(false)

    unbind()
    unbind()
    expect(unsubscribeHide).toHaveBeenCalledTimes(1)
    expect(unsubscribeShow).toHaveBeenCalledTimes(1)
    expect(removeVisibilityListener).toHaveBeenCalledTimes(1)
    expect(removeVisibilityListener).toHaveBeenCalledWith(visibilityHandler)

    harness.lifecycle.dispose()
  })
})
