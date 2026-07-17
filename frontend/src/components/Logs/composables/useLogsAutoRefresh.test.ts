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
})
