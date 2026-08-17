import { reactive } from 'vue'
import { getLatestResults, HealthStatus, type ProviderTimeline } from '../../../services/healthcheck'
import type { ProviderTab, TranslateFn } from '../types'

const createAvailabilityMap = (): Record<ProviderTab, Record<number, ProviderTimeline>> => ({
  claude: {},
  codex: {},
  gemini: {},
  opencode: {},
  grokbuild: {},
  'claude-desktop': {},
  openclaw: {},
  hermes: {},
  pi: {},
  others: {},
})

export function useAvailabilityState(t: TranslateFn, getActiveTab: () => ProviderTab) {
  const availabilityResultsMap = reactive(createAvailabilityMap())

  const loadAvailabilityResults = async () => {
    try {
      const allResults = await getLatestResults()

      for (const platform of Object.keys(allResults)) {
        const timelines = allResults[platform] || []
        const map: Record<number, ProviderTimeline> = {}
        timelines.forEach((timeline) => {
          map[timeline.providerId] = timeline
        })
        availabilityResultsMap[platform as ProviderTab] = map
      }
    } catch (error) {
      console.error('加载可用性监控结果失败:', error)
    }
  }

  const getProviderAvailabilityResult = (providerId: number): ProviderTimeline | null => {
    return availabilityResultsMap[getActiveTab()][providerId] || null
  }

  const getConnectivityIndicatorClass = (providerId: number): string => {
    const result = getProviderAvailabilityResult(providerId)
    if (!result || !result.latest) return 'connectivity-gray'

    switch (result.latest.status) {
      case HealthStatus.OPERATIONAL:
        return 'connectivity-green'
      case HealthStatus.DEGRADED:
        return 'connectivity-yellow'
      case HealthStatus.FAILED:
      case HealthStatus.VALIDATION_ERROR:
        return 'connectivity-red'
      default:
        return 'connectivity-gray'
    }
  }

  const getConnectivityTooltip = (providerId: number): string => {
    const result = getProviderAvailabilityResult(providerId)
    if (!result || !result.latest) return t('components.main.connectivity.noData')

    let statusText = ''
    switch (result.latest.status) {
      case HealthStatus.OPERATIONAL:
        statusText = t('components.main.connectivity.available')
        break
      case HealthStatus.DEGRADED:
        statusText = t('components.main.connectivity.degraded')
        break
      case HealthStatus.FAILED:
      case HealthStatus.VALIDATION_ERROR:
        statusText = t('components.main.connectivity.unavailable')
        break
      default:
        statusText = t('components.main.connectivity.noData')
    }

    const latencyText = result.latest.latencyMs > 0 ? ` (${result.latest.latencyMs}ms)` : ''
    const uptimeText = result.uptime > 0 ? ` - ${result.uptime.toFixed(1)}%` : ''
    return statusText + latencyText + uptimeText
  }

  return {
    availabilityResultsMap,
    loadAvailabilityResults,
    getProviderAvailabilityResult,
    getConnectivityIndicatorClass,
    getConnectivityTooltip,
  }
}
