import { ref, unref, type Ref } from 'vue'

type AsyncTask = () => Promise<void>

type UseLogsAutoRefreshOptions = {
  intervalSeconds?: number | Ref<number>
}

const DEFAULT_REFRESH_INTERVAL = 30

export function useLogsAutoRefresh(
  loadDashboard: AsyncTask,
  options: UseLogsAutoRefreshOptions = {},
) {
  const resolveRefreshInterval = () => Math.max(
    0,
    Math.floor(Number(unref(options.intervalSeconds ?? DEFAULT_REFRESH_INTERVAL)) || 0),
  )
  const countdown = ref(resolveRefreshInterval())
  let timer: number | undefined
  let isRefreshing = false

  const resetTimer = () => {
    countdown.value = resolveRefreshInterval()
  }

  const triggerRefresh = async () => {
    if (isRefreshing) return
    isRefreshing = true
    resetTimer()
    try {
      await loadDashboard()
    } finally {
      isRefreshing = false
      resetTimer()
    }
  }

  const startCountdown = () => {
    stopCountdown()
    if (typeof window === 'undefined') return
    if (resolveRefreshInterval() === 0) {
      resetTimer()
      return
    }
    timer = window.setInterval(() => {
      if (isRefreshing) return
      if (countdown.value <= 1) {
        void triggerRefresh()
      } else {
        countdown.value -= 1
      }
    }, 1000)
  }

  const stopCountdown = () => {
    if (timer) {
      window.clearInterval(timer)
      timer = undefined
    }
  }

  const manualRefresh = () => triggerRefresh()

  const restartCountdown = () => {
    resetTimer()
    startCountdown()
  }

  return {
    countdown,
    resetTimer,
    startCountdown,
    stopCountdown,
    manualRefresh,
    restartCountdown,
  }
}
