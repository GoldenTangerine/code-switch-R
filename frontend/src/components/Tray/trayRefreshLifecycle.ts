/**
 * @name: 托盘刷新生命周期
 * @Descripttion: 管理托盘打开期间的刷新定时器与激活状态
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-04 10:33:31
 * @LastEditTime: 2026-08-04 10:33:31
 * @FilePath: frontend/src/components/Tray/trayRefreshLifecycle.ts
 */

export interface TrayRefreshLifecycleOptions {
  onActivate: () => void
  onTick: () => void
  getIntervalMs: () => number
  scheduleInterval?: (callback: () => void, intervalMs: number) => number
  cancelInterval?: (timerId: number) => void
}

export interface TrayRefreshLifecycle {
  activate: () => void
  deactivate: () => void
  restartTicker: () => void
  isActive: () => boolean
  dispose: () => void
}

export function createTrayRefreshLifecycle(options: TrayRefreshLifecycleOptions): TrayRefreshLifecycle {
  const scheduleInterval = options.scheduleInterval
    ?? ((callback, intervalMs) => window.setInterval(callback, intervalMs))
  const cancelInterval = options.cancelInterval
    ?? ((timerId) => window.clearInterval(timerId))
  let active = false
  let tickerId: number | undefined

  function stopTicker() {
    if (tickerId === undefined) return
    cancelInterval(tickerId)
    tickerId = undefined
  }

  function restartTicker() {
    stopTicker()
    if (!active) return
    const intervalMs = Math.max(1_000, Math.floor(options.getIntervalMs()))
    tickerId = scheduleInterval(() => {
      if (active) options.onTick()
    }, intervalMs)
  }

  function activate() {
    active = true
    options.onActivate()
  }

  function deactivate() {
    active = false
    stopTicker()
  }

  return {
    activate,
    deactivate,
    restartTicker,
    isActive: () => active,
    dispose: deactivate,
  }
}
