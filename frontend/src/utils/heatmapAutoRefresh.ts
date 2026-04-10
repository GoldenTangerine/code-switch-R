type ScheduleFn = (handler: () => void, timeoutMs: number) => ReturnType<typeof setTimeout>
type CancelFn = (handle: ReturnType<typeof setTimeout>) => void

type CreateHeatmapAutoRefreshControllerOptions = {
  intervalMs: number
  reload: () => Promise<unknown> | unknown
  schedule?: ScheduleFn
  cancel?: CancelFn
}

export type HeatmapAutoRefreshController = {
  start: () => void
  stop: () => void
  isActive: () => boolean
}

export const createHeatmapAutoRefreshController = ({
  intervalMs,
  reload,
  schedule = (handler, timeoutMs) => setTimeout(handler, timeoutMs),
  cancel = (handle) => clearTimeout(handle),
}: CreateHeatmapAutoRefreshControllerOptions): HeatmapAutoRefreshController => {
  let active = false
  let cycleToken = 0
  let timer: ReturnType<typeof setTimeout> | null = null

  const clearTimer = () => {
    if (timer === null) return
    cancel(timer)
    timer = null
  }

  const scheduleNext = (token: number) => {
    if (!active || token !== cycleToken) return
    clearTimer()
    timer = schedule(() => {
      timer = null
      void (async () => {
        try {
          await reload()
        } finally {
          if (!active || token !== cycleToken) return
          scheduleNext(token)
        }
      })()
    }, intervalMs)
  }

  return {
    start() {
      active = true
      cycleToken += 1
      scheduleNext(cycleToken)
    },
    stop() {
      active = false
      cycleToken += 1
      clearTimer()
    },
    isActive() {
      return active
    },
  }
}
