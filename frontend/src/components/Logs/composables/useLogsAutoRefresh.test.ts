/**
 * @name: 日志自动刷新测试
 * @Descripttion: 验证日志自动刷新的关闭、切换和计时器清理行为。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-17 14:38:50
 * @LastEditTime: 2026-07-17 14:38:50
 * @FilePath: frontend/src/components/Logs/composables/useLogsAutoRefresh.test.ts
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useLogsAutoRefresh } from './useLogsAutoRefresh'

const flushPromises = async () => {
  await Promise.resolve()
  await Promise.resolve()
}

describe('useLogsAutoRefresh', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.stubGlobal('window', {
      setInterval: globalThis.setInterval,
      clearInterval: globalThis.clearInterval,
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('keeps automatic refresh disabled when the interval is zero', async () => {
    const loadDashboard = vi.fn().mockResolvedValue(undefined)
    const intervalSeconds = ref(0)
    const { countdown, startCountdown } = useLogsAutoRefresh(loadDashboard, { intervalSeconds })

    startCountdown()
    await vi.advanceTimersByTimeAsync(10_000)

    expect(countdown.value).toBe(0)
    expect(loadDashboard).not.toHaveBeenCalled()
  })

  it('restarts immediately with the latest interval', async () => {
    const loadDashboard = vi.fn().mockResolvedValue(undefined)
    const intervalSeconds = ref(5)
    const { countdown, restartCountdown, startCountdown } = useLogsAutoRefresh(loadDashboard, { intervalSeconds })

    startCountdown()
    await vi.advanceTimersByTimeAsync(2_000)
    expect(countdown.value).toBe(3)

    intervalSeconds.value = 10
    restartCountdown()
    expect(countdown.value).toBe(10)

    await vi.advanceTimersByTimeAsync(9_000)
    expect(loadDashboard).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1_000)
    expect(loadDashboard).toHaveBeenCalledTimes(1)
    expect(countdown.value).toBe(10)
  })

  it('stops the active timer during cleanup', async () => {
    const loadDashboard = vi.fn().mockResolvedValue(undefined)
    const { stopCountdown, startCountdown } = useLogsAutoRefresh(loadDashboard, { intervalSeconds: 5 })

    startCountdown()
    stopCountdown()
    await vi.advanceTimersByTimeAsync(10_000)

    expect(loadDashboard).not.toHaveBeenCalled()
  })

  it('does not overlap refresh tasks when the previous load is still running', async () => {
    let resolveFirstLoad!: () => void
    const loadDashboard = vi.fn()
      .mockImplementationOnce(() => new Promise<void>((resolve) => {
        resolveFirstLoad = resolve
      }))
      .mockResolvedValue(undefined)
    const { countdown, startCountdown } = useLogsAutoRefresh(loadDashboard, { intervalSeconds: 5 })

    startCountdown()
    await vi.advanceTimersByTimeAsync(15_000)
    expect(loadDashboard).toHaveBeenCalledTimes(1)

    resolveFirstLoad()
    await Promise.resolve()
    await vi.advanceTimersByTimeAsync(5_000)

    expect(loadDashboard).toHaveBeenCalledTimes(2)
    expect(countdown.value).toBe(5)
  })

  it('pauses while deactivated and refreshes once before restarting the full interval', async () => {
    const loadDashboard = vi.fn().mockResolvedValue(undefined)
    const { setPageActive, startCountdown } = useLogsAutoRefresh(loadDashboard, { intervalSeconds: 30 })

    startCountdown()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(loadDashboard).toHaveBeenCalledTimes(2)

    setPageActive(false)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(loadDashboard).toHaveBeenCalledTimes(2)

    setPageActive(true)
    setPageActive(true)
    await flushPromises()
    expect(loadDashboard).toHaveBeenCalledTimes(3)

    await vi.advanceTimersByTimeAsync(29_000)
    expect(loadDashboard).toHaveBeenCalledTimes(3)
    await vi.advanceTimersByTimeAsync(1_000)
    expect(loadDashboard).toHaveBeenCalledTimes(4)
  })

  it('does not refresh for the initial activation signal or while disabled', async () => {
    const loadDashboard = vi.fn().mockResolvedValue(undefined)
    const intervalSeconds = ref(30)
    const { setPageActive, startCountdown } = useLogsAutoRefresh(loadDashboard, { intervalSeconds })

    setPageActive(true)
    startCountdown()
    expect(loadDashboard).not.toHaveBeenCalled()

    setPageActive(false)
    intervalSeconds.value = 0
    setPageActive(true)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(60_000)

    expect(loadDashboard).not.toHaveBeenCalled()
  })

  it('does not start before initial loading finishes while the page is deactivated', async () => {
    const loadDashboard = vi.fn().mockResolvedValue(undefined)
    const { setPageActive, startCountdown } = useLogsAutoRefresh(loadDashboard, { intervalSeconds: 30 })

    setPageActive(false)
    startCountdown()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(loadDashboard).not.toHaveBeenCalled()

    setPageActive(true)
    await flushPromises()
    expect(loadDashboard).toHaveBeenCalledTimes(1)
  })

  it('does not restart after being deactivated during activation refresh', async () => {
    let resolveRefresh!: () => void
    const loadDashboard = vi.fn(() => new Promise<void>((resolve) => {
      resolveRefresh = resolve
    }))
    const { setPageActive, startCountdown } = useLogsAutoRefresh(loadDashboard, { intervalSeconds: 5 })

    startCountdown()
    setPageActive(false)
    setPageActive(true)
    expect(loadDashboard).toHaveBeenCalledTimes(1)

    setPageActive(false)
    resolveRefresh()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(10_000)

    expect(loadDashboard).toHaveBeenCalledTimes(1)
  })
})
