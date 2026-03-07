import { ref } from 'vue'

type AsyncTask = () => Promise<void>

type UseLogsAutoRefreshOptions = {
  intervalSeconds?: number
}

const DEFAULT_REFRESH_INTERVAL = 30

export function useLogsAutoRefresh(
  loadDashboard: AsyncTask,
  options: UseLogsAutoRefreshOptions = {},
) {
  const refreshInterval = Math.max(1, Math.floor(options.intervalSeconds ?? DEFAULT_REFRESH_INTERVAL))
  const countdown = ref(refreshInterval)
  let timer: number | undefined

  const resetTimer = () => {
    countdown.value = refreshInterval
  }

  const triggerRefresh = async () => {
    resetTimer()
    await loadDashboard()
  }

  const startCountdown = () => {
    stopCountdown()
    if (typeof window === 'undefined') return
    timer = window.setInterval(() => {
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

  const manualRefresh = () => {
    void triggerRefresh()
  }

  return {
    countdown,
    resetTimer,
    startCountdown,
    stopCountdown,
    manualRefresh,
  }
}
