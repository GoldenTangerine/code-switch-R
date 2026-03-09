import { ref } from 'vue'
import { Browser } from '@wailsio/runtime'
import { fetchCurrentVersion } from '../../../services/version'
import { getUpdateState, restartApp } from '../../../services/update'
import { RELEASE_API_POLL_INTERVAL_MS, RELEASE_API_URL, RELEASE_PAGE_URL } from '../constants'
import type { TranslateFn } from '../types'

const normalizeVersion = (value: string) => value.replace(/^v/i, '').trim()

const compareVersions = (current: string, remote: string) => {
  const curParts = normalizeVersion(current).split('.').map((part) => parseInt(part, 10) || 0)
  const remoteParts = normalizeVersion(remote).split('.').map((part) => parseInt(part, 10) || 0)
  const maxLen = Math.max(curParts.length, remoteParts.length)
  for (let i = 0; i < maxLen; i++) {
    const cur = curParts[i] ?? 0
    const rem = remoteParts[i] ?? 0
    if (cur === rem) continue
    return cur < rem ? -1 : 1
  }
  return 0
}

export function useUpdatePolling(t: TranslateFn) {
  const appVersion = ref('')
  const hasUpdateAvailable = ref(false)
  const updateReady = ref(false)
  const downloadProgress = ref(0)
  const autoCheckEnabled = ref(true)

  let lastReleaseApiCheckAt = 0
  let updateTimer: number | undefined

  const checkForUpdates = async (force = false) => {
    try {
      const version = await fetchCurrentVersion()
      appVersion.value = version || ''
    } catch (error) {
      console.error('failed to load app version', error)
    }

    if (!force) {
      if (!autoCheckEnabled.value) return
      const now = Date.now()
      if (now - lastReleaseApiCheckAt < RELEASE_API_POLL_INTERVAL_MS) {
        return
      }
    }

    try {
      const response = await fetch(RELEASE_API_URL, {
        headers: {
          Accept: 'application/vnd.github+json',
        },
      })
      if (!response.ok) return

      lastReleaseApiCheckAt = Date.now()
      const data = await response.json()
      const latestTag = data?.tag_name ?? ''
      if (latestTag) {
        hasUpdateAvailable.value = compareVersions(appVersion.value || '0.0.0', latestTag) < 0
      }
    } catch (error) {
      console.error('failed to fetch release info', error)
    }
  }

  const pollUpdateState = async () => {
    try {
      const state = await getUpdateState()
      autoCheckEnabled.value = state.auto_check_enabled ?? true
      updateReady.value = state.update_ready
      downloadProgress.value = state.download_progress
      if (state.latest_known_version) {
        hasUpdateAvailable.value = compareVersions(appVersion.value || '0.0.0', state.latest_known_version) < 0
      }
    } catch (error) {
      console.error('failed to poll update state', error)
    }
  }

  const startUpdateTimer = () => {
    stopUpdateTimer()
    updateTimer = window.setInterval(() => {
      void pollUpdateState()
      void checkForUpdates()
    }, 30 * 1000)
  }

  const stopUpdateTimer = () => {
    if (updateTimer) {
      clearInterval(updateTimer)
      updateTimer = undefined
    }
  }

  const handleGithubClick = async () => {
    if (updateReady.value) {
      const confirmed = confirm('新版本已准备好，是否立即重启应用？')
      if (!confirmed) return

      try {
        await restartApp()
      } catch (error) {
        console.error('failed to restart app', error)
        alert('重启失败，请手动重启应用')
      }
      return
    }

    Browser.OpenURL(RELEASE_PAGE_URL).catch(() => {
      console.error('failed to open github')
    })
  }

  const getGithubTooltip = () => {
    if (updateReady.value) {
      return t('components.main.controls.updateReady')
    }
    if (hasUpdateAvailable.value) {
      return t('components.main.controls.githubUpdate')
    }
    return t('components.main.controls.github')
  }

  return {
    appVersion,
    hasUpdateAvailable,
    updateReady,
    downloadProgress,
    autoCheckEnabled,
    checkForUpdates,
    pollUpdateState,
    startUpdateTimer,
    stopUpdateTimer,
    handleGithubClick,
    getGithubTooltip,
  }
}
