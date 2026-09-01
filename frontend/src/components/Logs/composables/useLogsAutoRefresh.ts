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
  let ready = false
  let pageActive = true
  let generation = 0

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
    ready = true
    stopCountdown()
    if (typeof window === 'undefined' || !pageActive) return
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

  const startCountdownForGeneration = (expectedGeneration: number) => {
    if (!pageActive || generation !== expectedGeneration) return
    startCountdown()
  }

  const setPageActive = (nextActive: boolean) => {
    if (pageActive === nextActive) return

    pageActive = nextActive
    const expectedGeneration = ++generation
    if (!pageActive) {
      stopCountdown()
      return
    }
    if (!ready || typeof window === 'undefined' || resolveRefreshInterval() === 0) {
      resetTimer()
      return
    }
    void triggerRefresh().then(
      () => startCountdownForGeneration(expectedGeneration),
      () => startCountdownForGeneration(expectedGeneration),
    )
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
    setPageActive,
    manualRefresh,
    restartCountdown,
  }
}
