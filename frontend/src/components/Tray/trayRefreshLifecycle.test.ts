/**
 * @name: 托盘刷新生命周期测试
 * @Descripttion: 验证托盘仅在打开期间创建定时器并触发刷新
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-04 10:33:31
 * @LastEditTime: 2026-08-04 10:33:31
 * @FilePath: frontend/src/components/Tray/trayRefreshLifecycle.test.ts
 */
import { describe, expect, it, vi } from 'vitest'
import { createTrayRefreshLifecycle } from './trayRefreshLifecycle'

describe('trayRefreshLifecycle', () => {
  it('refreshes on activation and owns no ticker while inactive', () => {
    const onActivate = vi.fn()
    const onTick = vi.fn()
    const scheduledCallbacks = new Map<number, () => void>()
    const clearedTimerIds: number[] = []
    let nextTimerId = 1

    const lifecycle = createTrayRefreshLifecycle({
      onActivate,
      onTick,
      getIntervalMs: () => 60_000,
      scheduleInterval: (callback) => {
        const timerId = nextTimerId++
        scheduledCallbacks.set(timerId, callback)
        return timerId
      },
      cancelInterval: (timerId) => {
        clearedTimerIds.push(timerId)
      },
    })

    lifecycle.restartTicker()
    expect(scheduledCallbacks.size).toBe(0)

    lifecycle.activate()
    expect(onActivate).toHaveBeenCalledTimes(1)
    expect(lifecycle.isActive()).toBe(true)

    lifecycle.restartTicker()
    expect(scheduledCallbacks.size).toBe(1)
    scheduledCallbacks.get(1)?.()
    expect(onTick).toHaveBeenCalledTimes(1)

    lifecycle.deactivate()
    scheduledCallbacks.get(1)?.()
    expect(clearedTimerIds).toEqual([1])
    expect(onTick).toHaveBeenCalledTimes(1)
    expect(lifecycle.isActive()).toBe(false)
  })

  it('refreshes again when reopened and replaces the active ticker', () => {
    const onActivate = vi.fn()
    const scheduledIntervals: number[] = []
    const clearedTimerIds: number[] = []
    let intervalMs = 1_000
    let nextTimerId = 1

    const lifecycle = createTrayRefreshLifecycle({
      onActivate,
      onTick: vi.fn(),
      getIntervalMs: () => intervalMs,
      scheduleInterval: (_callback, nextIntervalMs) => {
        scheduledIntervals.push(nextIntervalMs)
        return nextTimerId++
      },
      cancelInterval: (timerId) => {
        clearedTimerIds.push(timerId)
      },
    })

    lifecycle.activate()
    lifecycle.restartTicker()
    intervalMs = 60_000
    lifecycle.restartTicker()
    lifecycle.deactivate()
    lifecycle.activate()

    expect(onActivate).toHaveBeenCalledTimes(2)
    expect(scheduledIntervals).toEqual([1_000, 60_000])
    expect(clearedTimerIds).toEqual([1, 2])
  })
})
