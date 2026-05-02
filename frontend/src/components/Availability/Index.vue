<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  getLatestResults,
  getLogBasedResults,
  runAllChecks,
  runSingleCheck,
  setAvailabilityMonitorEnabled,
  isPollingRunning,
  saveAvailabilityConfig,
  type ProviderTimeline,
  type LogAvailabilityRange,
  HealthStatus,
} from '../../services/healthcheck'
import { lockScroll, unlockScroll } from '../../utils/scrollLock'

type StatusTone = 'operational' | 'degraded' | 'failed' | 'disabled'
type HistoryTone = StatusTone | 'empty'
type AvailabilityMode = 'probe' | 'logs'

type HistorySegment = {
  key: string
  tone: HistoryTone
  statusLabel: string
  checkedAtDateLabel: string
  checkedAtLabel: string
  checkedAtFullLabel: string
  latencyLabel: string
  isPlaceholder: boolean
}

type ProviderCardViewModel = ProviderTimeline & {
  providerKey: string
  platformLabel: string
  statusTone: StatusTone
  statusLabel: string
  statusDescription: string
  latestLatencyLabel: string
  avgLatencyLabel: string
  uptimeLabel: string
  lastCheckedLabel: string
  lastCheckedFullLabel: string
  historySegments: HistorySegment[]
  latestError: string
}

const REFRESH_INTERVAL_SECONDS = 60
const HISTORY_SEGMENT_LIMIT = 72
const PROVIDERS_UPDATED_EVENT = 'providers-updated'
const AVAILABILITY_MODE_STORAGE_KEY = 'code-switch:availability-mode'
const LOG_AVAILABILITY_RANGE_STORAGE_KEY = 'code-switch:availability-log-range'
const LOG_AVAILABILITY_RANGE_MS: Record<LogAvailabilityRange, number> = {
  '15min': 15 * 60 * 1000,
  '1h': 60 * 60 * 1000,
  '6h': 6 * 60 * 60 * 1000,
  '24h': 24 * 60 * 60 * 1000,
  '7d': 7 * 24 * 60 * 60 * 1000,
}
const PLATFORM_LABELS: Record<string, string> = {
  claude: 'Claude',
  codex: 'Codex',
  gemini: 'Gemini',
  others: 'Other',
}

const { t, locale } = useI18n()

const loading = ref(true)
const refreshing = ref(false)
const timelines = ref<Record<string, ProviderTimeline[]>>({})
const pollingRunning = ref(false)
const activeAvailabilityMode = ref<AvailabilityMode>(resolveInitialAvailabilityMode())
const activeLogRange = ref<LogAvailabilityRange>(resolveInitialLogRange())
const isDarkTheme = ref(document.documentElement.classList.contains('dark'))
const lastUpdated = ref<Date | null>(null)
const nextRefreshIn = ref(REFRESH_INTERVAL_SECONDS)

const showConfigModal = ref(false)
const savingConfig = ref(false)
const activeProvider = ref<ProviderTimeline | null>(null)
const checkingProviderKey = ref<string | null>(null)
const togglingProviderKey = ref<string | null>(null)
const configForm = ref({
  testModel: '',
  testEndpoint: '',
  timeout: 15000,
})

let refreshTimer: ReturnType<typeof setInterval> | null = null
let countdownTimer: ReturnType<typeof setInterval> | null = null
let themeObserver: MutationObserver | null = null
let loadDataRequestId = 0

const localeTag = computed(() => (locale.value?.startsWith('zh') ? 'zh-CN' : 'en-US'))

const providerCards = computed<ProviderCardViewModel[]>(() => {
  const cards = Object.entries(timelines.value).flatMap(([platform, items]) =>
    (items ?? []).map((timeline) => buildProviderCard(platform, timeline)),
  )

  return cards.sort((a, b) => {
    const toneDiff = getStatusOrder(a.statusTone) - getStatusOrder(b.statusTone)
    if (toneDiff !== 0) return toneDiff

    const platformDiff = a.platformLabel.localeCompare(b.platformLabel, localeTag.value)
    if (platformDiff !== 0) return platformDiff

    return a.providerName.localeCompare(b.providerName, localeTag.value)
  })
})

const statusStats = computed(() => {
  const stats = {
    operational: 0,
    degraded: 0,
    failed: 0,
    disabled: 0,
    total: providerCards.value.length,
  }

  for (const card of providerCards.value) {
    switch (card.statusTone) {
      case 'operational':
        stats.operational += 1
        break
      case 'degraded':
        stats.degraded += 1
        break
      case 'failed':
        stats.failed += 1
        break
      case 'disabled':
        stats.disabled += 1
        break
    }
  }

  return stats
})

const summaryCards = computed(() => [
  {
    key: 'operational',
    tone: 'operational' as const,
    label: t('availability.stats.operational'),
    value: statusStats.value.operational,
  },
  {
    key: 'degraded',
    tone: 'degraded' as const,
    label: t('availability.stats.degraded'),
    value: statusStats.value.degraded,
  },
  {
    key: 'failed',
    tone: 'failed' as const,
    label: t('availability.stats.failed'),
    value: statusStats.value.failed,
  },
  {
    key: 'disabled',
    tone: 'disabled' as const,
    label: t('availability.stats.disabled'),
    value: statusStats.value.disabled,
  },
])

const pageThemeClass = computed(() => (
  isDarkTheme.value ? 'availability-page--dark' : 'availability-page--light'
))

const isLogAvailabilityMode = computed(() => activeAvailabilityMode.value === 'logs')

const availabilityModeOptions = computed(() => [
  {
    value: 'logs' as const,
    label: t('availability.mode.logs'),
    title: t('availability.mode.logsHint'),
  },
  {
    value: 'probe' as const,
    label: t('availability.mode.probe'),
    title: t('availability.mode.probeHint'),
  },
])

const logRangeOptions = computed(() => [
  {
    value: '15min' as const,
    label: t('availability.range.last15min'),
  },
  {
    value: '1h' as const,
    label: t('availability.range.last1h'),
  },
  {
    value: '6h' as const,
    label: t('availability.range.last6h'),
  },
  {
    value: '24h' as const,
    label: t('availability.range.last24h'),
  },
  {
    value: '7d' as const,
    label: t('availability.range.last7d'),
  },
])

const activeModeHint = computed(() => (
  isLogAvailabilityMode.value ? t('availability.mode.logsHint') : t('availability.mode.probeHint')
))

const activeLogRangeLabel = computed(() => (
  logRangeOptions.value.find((option) => option.value === activeLogRange.value)?.label ?? t('availability.range.last15min')
))

const historyTitleLabel = computed(() => (
  isLogAvailabilityMode.value
    ? t('availability.history.logTitle', { range: activeLogRangeLabel.value })
    : t('availability.history.title')
))

const historyStartLabel = computed(() => (
  isLogAvailabilityMode.value
    ? t('availability.history.rangeStart', { range: activeLogRangeLabel.value })
    : t('availability.history.start')
))

const historyEndLabel = computed(() => (
  isLogAvailabilityMode.value ? t('availability.history.now') : t('availability.history.end')
))

watch(showConfigModal, (open) => {
  if (open) {
    lockScroll()
    return
  }

  unlockScroll()
})

const handleProvidersUpdated = () => {
  void loadData()
}

const handleEscape = (event: KeyboardEvent) => {
  if (event.key === 'Escape' && showConfigModal.value) {
    closeConfigModal()
  }
}

function getStatusOrder(tone: StatusTone) {
  switch (tone) {
    case 'failed':
      return 0
    case 'degraded':
      return 1
    case 'operational':
      return 2
    case 'disabled':
      return 3
    default:
      return 4
  }
}

function getProviderKey(platform: string, providerId: number) {
  return `${platform}:${providerId}`
}

function resolvePlatformLabel(platform: string) {
  return PLATFORM_LABELS[platform] ?? platform
}

function resolveInitialAvailabilityMode(): AvailabilityMode {
  if (typeof window === 'undefined') {
    return 'logs'
  }
  return window.localStorage.getItem(AVAILABILITY_MODE_STORAGE_KEY) === 'probe' ? 'probe' : 'logs'
}

function isLogAvailabilityRange(value: string | null): value is LogAvailabilityRange {
  return value === '15min' || value === '1h' || value === '6h' || value === '24h' || value === '7d'
}

function resolveInitialLogRange(): LogAvailabilityRange {
  if (typeof window === 'undefined') {
    return '24h'
  }
  const storedRange = window.localStorage.getItem(LOG_AVAILABILITY_RANGE_STORAGE_KEY)
  return isLogAvailabilityRange(storedRange) ? storedRange : '24h'
}

function setAvailabilityMode(mode: AvailabilityMode) {
  if (activeAvailabilityMode.value === mode) return
  activeAvailabilityMode.value = mode
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(AVAILABILITY_MODE_STORAGE_KEY, mode)
  }
  loading.value = true
  nextRefreshIn.value = REFRESH_INTERVAL_SECONDS
  void loadData()
}

function setLogAvailabilityRange(range: LogAvailabilityRange) {
  if (activeLogRange.value === range) return
  activeLogRange.value = range
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(LOG_AVAILABILITY_RANGE_STORAGE_KEY, range)
  }
  if (isLogAvailabilityMode.value) {
    loading.value = true
    nextRefreshIn.value = REFRESH_INTERVAL_SECONDS
    void loadData()
  }
}

function canDisplayTimeline(timeline: ProviderTimeline) {
  return isLogAvailabilityMode.value || timeline.availabilityMonitorEnabled
}

function hasTimelineSamples(timeline: ProviderTimeline) {
  if (!canDisplayTimeline(timeline)) return false
  if (!isLogAvailabilityMode.value) {
    return (timeline.items?.length ?? 0) > 0
  }
  return (timeline.items ?? []).some((item) => Boolean(item.status))
}

function toDate(value?: string | Date | null) {
  if (!value) return null
  const date = value instanceof Date ? value : new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

function formatDateTime(
  value?: string | Date | null,
  options: Intl.DateTimeFormatOptions = {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  },
) {
  const date = toDate(value)
  if (!date) return '--'
  return new Intl.DateTimeFormat(localeTag.value, options).format(date)
}

function formatLastUpdated() {
  return formatDateTime(lastUpdated.value)
}

function formatFullDateTime(value?: string | Date | null) {
  return formatDateTime(value, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function formatShortDateTime(value?: string | Date | null) {
  return formatDateTime(value, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatHistoryDate(value?: string | Date | null) {
  const date = toDate(value)
  if (!date) return '--'
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

function getLogBucketSizeMinutes() {
  return (LOG_AVAILABILITY_RANGE_MS[activeLogRange.value] / HISTORY_SEGMENT_LIMIT) / (60 * 1000)
}

function formatLogHistoryDate(value?: string | Date | null) {
  const bucketSizeMinutes = getLogBucketSizeMinutes()
  if (bucketSizeMinutes >= 1440) {
    return formatDateTime(value, {
      month: '2-digit',
      day: '2-digit',
    })
  }
  if (bucketSizeMinutes >= 60) {
    return formatDateTime(value, {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  }
  if (bucketSizeMinutes < 1) {
    return formatDateTime(value, {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  }
  return formatDateTime(value, {
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatHistoryTooltipDate(value?: string | Date | null) {
  return isLogAvailabilityMode.value ? formatLogHistoryDate(value) : formatHistoryDate(value)
}

function formatLatency(value?: number | null) {
  if (!Number.isFinite(value) || Number(value) <= 0) {
    return '--'
  }
  return `${Math.round(Number(value))} ms`
}

function formatUptime(timeline: ProviderTimeline) {
  if (!hasTimelineSamples(timeline)) {
    return '--'
  }

  if (!Number.isFinite(timeline.uptime)) {
    return '--'
  }

  if (timeline.uptime >= 100) {
    return '100%'
  }

  return `${timeline.uptime.toFixed(2)}%`
}

function resolveStatusTone(timeline: ProviderTimeline): StatusTone {
  if (!canDisplayTimeline(timeline)) {
    return 'disabled'
  }

  switch (timeline.latest?.status) {
    case HealthStatus.OPERATIONAL:
      return 'operational'
    case HealthStatus.DEGRADED:
      return 'degraded'
    case HealthStatus.FAILED:
    case HealthStatus.VALIDATION_ERROR:
      return 'failed'
    default:
      return 'disabled'
  }
}

function resolveStatusLabel(timeline: ProviderTimeline) {
  if (!canDisplayTimeline(timeline)) {
    return t('availability.statusChip.disabled')
  }

  switch (timeline.latest?.status) {
    case HealthStatus.OPERATIONAL:
      return t('availability.statusChip.operational')
    case HealthStatus.DEGRADED:
      return t('availability.statusChip.degraded')
    case HealthStatus.FAILED:
    case HealthStatus.VALIDATION_ERROR:
      return t('availability.statusChip.failed')
    default:
      return t('availability.history.noData')
  }
}

function resolveStatusDescription(timeline: ProviderTimeline) {
  if (!canDisplayTimeline(timeline)) {
    return t('availability.statusDescription.disabled')
  }

  switch (timeline.latest?.status) {
    case HealthStatus.OPERATIONAL:
      return t('availability.statusDescription.operational')
    case HealthStatus.DEGRADED:
      return t('availability.statusDescription.degraded')
    case HealthStatus.FAILED:
    case HealthStatus.VALIDATION_ERROR:
      return t('availability.statusDescription.failed')
    default:
      return t('availability.history.noData')
  }
}

function resolveHistoryTone(status?: string): HistoryTone {
  switch (status) {
    case HealthStatus.OPERATIONAL:
      return 'operational'
    case HealthStatus.DEGRADED:
      return 'degraded'
    case HealthStatus.FAILED:
    case HealthStatus.VALIDATION_ERROR:
      return 'failed'
    default:
      return 'empty'
  }
}

function buildHistorySegments(timeline: ProviderTimeline): HistorySegment[] {
  const hasTimelineData = canDisplayTimeline(timeline)
  const recentItems = hasTimelineData
    ? [...(timeline.items ?? [])].slice(0, HISTORY_SEGMENT_LIMIT).reverse()
    : []

  const segments = recentItems.map((item, index) => ({
    key: `${timeline.providerId}-${item.checkedAt}-${index}`,
    tone: resolveHistoryTone(item.status),
    statusLabel: resolveStatusLabel({
      ...timeline,
      latest: item,
      items: [item],
      availabilityMonitorEnabled: true,
    }),
    checkedAtDateLabel: formatHistoryTooltipDate(item.checkedAt),
    checkedAtLabel: formatShortDateTime(item.checkedAt),
    checkedAtFullLabel: formatFullDateTime(item.checkedAt),
    latencyLabel: formatLatency(item.latencyMs),
    isPlaceholder: false,
  }))

  const placeholderCount = Math.max(HISTORY_SEGMENT_LIMIT - segments.length, 0)
  const placeholderLabel = hasTimelineData ? t('availability.history.noSample') : t('availability.notMonitored')
  const placeholderTime = hasTimelineData ? t('availability.history.noData') : t('availability.notMonitored')

  const placeholders = Array.from({ length: placeholderCount }, (_, index) => ({
    key: `${timeline.providerId}-placeholder-${index}`,
    tone: 'empty' as const,
    statusLabel: placeholderLabel,
    checkedAtDateLabel: placeholderTime,
    checkedAtLabel: placeholderTime,
    checkedAtFullLabel: placeholderTime,
    latencyLabel: '--',
    isPlaceholder: true,
  }))

  return [...placeholders, ...segments]
}

function buildProviderCard(platform: string, timeline: ProviderTimeline): ProviderCardViewModel {
  return {
    ...timeline,
    providerKey: getProviderKey(platform, timeline.providerId),
    platformLabel: resolvePlatformLabel(platform),
    statusTone: resolveStatusTone(timeline),
    statusLabel: resolveStatusLabel(timeline),
    statusDescription: resolveStatusDescription(timeline),
    latestLatencyLabel: formatLatency(timeline.latest?.latencyMs),
    avgLatencyLabel: formatLatency(timeline.avgLatencyMs),
    uptimeLabel: formatUptime(timeline),
    lastCheckedLabel: formatDateTime(timeline.latest?.checkedAt),
    lastCheckedFullLabel: formatFullDateTime(timeline.latest?.checkedAt),
    historySegments: buildHistorySegments(timeline),
    latestError: timeline.latest?.errorMessage?.trim() ?? '',
  }
}

function displayConfigValue(value: string | undefined | null, fallbackLabel: string) {
  if (!value || !value.trim()) {
    return fallbackLabel
  }
  return value
}

function displayTimeoutValue(value?: number | null) {
  if (!Number.isFinite(value) || Number(value) <= 0) {
    return '15000 ms'
  }
  return `${Math.round(Number(value))} ms`
}

function findProviderTimeline(platform: string, providerId: number) {
  return timelines.value[platform]?.find((item) => item.providerId === providerId) ?? null
}

function isCheckingCard(providerKey: string) {
  return checkingProviderKey.value === providerKey
}

function isTogglingCard(providerKey: string) {
  return togglingProviderKey.value === providerKey
}

async function loadData() {
  const requestId = ++loadDataRequestId
  const requestedMode = activeAvailabilityMode.value
  const requestedRange = activeLogRange.value

  try {
    const nextTimelines = requestedMode === 'logs'
      ? await getLogBasedResults(requestedRange)
      : await getLatestResults()
    let nextPollingRunning = pollingRunning.value

    try {
      nextPollingRunning = await isPollingRunning()
    } catch (pollingError) {
      if (requestId === loadDataRequestId) {
        console.error('Failed to load polling status:', pollingError)
      }
    }

    if (requestId !== loadDataRequestId) return

    timelines.value = nextTimelines
    pollingRunning.value = nextPollingRunning

    lastUpdated.value = new Date()
    nextRefreshIn.value = REFRESH_INTERVAL_SECONDS
  } catch (error) {
    if (requestId === loadDataRequestId) {
      console.error('Failed to load availability data:', error)
    }
  } finally {
    if (requestId === loadDataRequestId) {
      loading.value = false
    }
  }
}

async function refreshAll() {
  if (refreshing.value) return

  refreshing.value = true
  try {
    if (!isLogAvailabilityMode.value) {
      await runAllChecks()
    }
    await loadData()
  } catch (error) {
    console.error('Failed to refresh availability data:', error)
  } finally {
    refreshing.value = false
  }
}

async function checkSingle(platform: string, providerId: number) {
  const providerKey = getProviderKey(platform, providerId)
  if (checkingProviderKey.value === providerKey) return

  checkingProviderKey.value = providerKey
  try {
    await runSingleCheck(platform, providerId)
    await loadData()
  } catch (error) {
    console.error('Failed to check provider:', error)
  } finally {
    checkingProviderKey.value = null
  }
}

async function toggleMonitor(platform: string, providerId: number, enabled: boolean) {
  const providerKey = getProviderKey(platform, providerId)
  if (togglingProviderKey.value === providerKey) return

  togglingProviderKey.value = providerKey
  try {
    await setAvailabilityMonitorEnabled(platform, providerId, enabled)
    await loadData()

    window.dispatchEvent(new CustomEvent(PROVIDERS_UPDATED_EVENT, {
      detail: { platform, providerId, enabled },
    }))
  } catch (error) {
    console.error('Failed to toggle availability monitor:', error)
  } finally {
    togglingProviderKey.value = null
  }
}

async function enableMonitoringAndEdit(platform: string, timeline: ProviderTimeline) {
  try {
    await toggleMonitor(platform, timeline.providerId, true)
    const latestTimeline = findProviderTimeline(platform, timeline.providerId)
    editConfig(latestTimeline ?? { ...timeline, availabilityMonitorEnabled: true })
  } catch (error) {
    console.error('Failed to enable monitoring and edit config:', error)
  }
}

function editConfig(timeline: ProviderTimeline) {
  activeProvider.value = { ...timeline }
  const cfg = timeline.availabilityConfig || {}
  configForm.value = {
    testModel: cfg.testModel || '',
    testEndpoint: cfg.testEndpoint || '',
    timeout: cfg.timeout || 15000,
  }
  showConfigModal.value = true
}

function closeConfigModal() {
  showConfigModal.value = false
  activeProvider.value = null
}

async function saveConfig() {
  if (!activeProvider.value) return

  savingConfig.value = true
  try {
    await saveAvailabilityConfig(activeProvider.value.platform, activeProvider.value.providerId, {
      testModel: configForm.value.testModel,
      testEndpoint: configForm.value.testEndpoint,
      timeout: Number(configForm.value.timeout) || 15000,
    })
    showConfigModal.value = false
    await loadData()
  } catch (error) {
    console.error('Failed to save availability config:', error)
  } finally {
    savingConfig.value = false
  }
}

function startRefreshTimer() {
  stopTimers()
  nextRefreshIn.value = REFRESH_INTERVAL_SECONDS

  refreshTimer = setInterval(() => {
    void loadData()
  }, REFRESH_INTERVAL_SECONDS * 1000)

  countdownTimer = setInterval(() => {
    nextRefreshIn.value = Math.max(nextRefreshIn.value - 1, 0)
  }, 1000)
}

watch(activeAvailabilityMode, () => {
  nextRefreshIn.value = REFRESH_INTERVAL_SECONDS
})

function startThemeObserver() {
  themeObserver?.disconnect()
  themeObserver = new MutationObserver(() => {
    isDarkTheme.value = document.documentElement.classList.contains('dark')
  })
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class'],
  })
}

function stopTimers() {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }

  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
}

onMounted(async () => {
  startThemeObserver()
  window.addEventListener(PROVIDERS_UPDATED_EVENT, handleProvidersUpdated)
  window.addEventListener('keydown', handleEscape)
  await loadData()
  startRefreshTimer()
})

onUnmounted(() => {
  if (showConfigModal.value) {
    unlockScroll()
  }
  themeObserver?.disconnect()
  themeObserver = null
  window.removeEventListener(PROVIDERS_UPDATED_EVENT, handleProvidersUpdated)
  window.removeEventListener('keydown', handleEscape)
  stopTimers()
})
</script>

<template>
  <div :class="['availability-page', pageThemeClass]">
    <div class="availability-page__grid" aria-hidden="true"></div>
    <div class="availability-page__glow availability-page__glow--primary" aria-hidden="true"></div>
    <div class="availability-page__glow availability-page__glow--secondary" aria-hidden="true"></div>

    <div class="availability-page__shell">
      <header class="availability-hero">
        <div class="availability-hero__copy">
          <div class="availability-hero__title-wrap">
            <div class="availability-hero__title-main">
              <span class="availability-hero__title-icon" aria-hidden="true">
                <svg viewBox="0 0 24 24" fill="none">
                  <path
                    d="M4 12h3l2.2-5.5L13 18l2.5-6H20"
                    stroke="currentColor"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="1.8"
                  />
                </svg>
              </span>

              <div class="availability-hero__title-group">
                <h1>{{ t('availability.title') }}</h1>
                <p>{{ t('availability.subtitle') }}</p>
              </div>
            </div>

            <div class="availability-runtime-card">
              <div class="availability-runtime-card__item">
                <span>{{ t('availability.lastUpdate') }}</span>
                <strong>{{ formatLastUpdated() }}</strong>
              </div>
              <div class="availability-runtime-card__item">
                <span>{{ t('availability.nextRefresh') }}</span>
                <strong>{{ nextRefreshIn }}s</strong>
              </div>
            </div>
          </div>

          <div class="availability-hero__control-row">
            <span class="availability-hero__runtime" :class="{ 'availability-hero__runtime--active': pollingRunning || isLogAvailabilityMode }">
              <span class="availability-hero__runtime-dot"></span>
              {{ isLogAvailabilityMode ? t('availability.runtime.logMode') : (pollingRunning ? t('availability.runtime.pollingRunning') : t('availability.runtime.pollingStopped')) }}
            </span>

            <div class="availability-hero__actions">
              <div class="availability-mode-switch" :title="activeModeHint">
                <span class="availability-mode-switch__label">{{ t('availability.mode.title') }}</span>
                <div class="availability-mode-switch__options" role="group" :aria-label="t('availability.mode.title')">
                  <button
                    v-for="option in availabilityModeOptions"
                    :key="option.value"
                    type="button"
                    class="availability-mode-switch__option"
                    :class="{ 'availability-mode-switch__option--active': activeAvailabilityMode === option.value }"
                    :title="option.title"
                    @click="setAvailabilityMode(option.value)"
                  >
                    {{ option.label }}
                  </button>
                </div>
              </div>

              <div v-if="isLogAvailabilityMode" class="availability-mode-switch availability-range-switch">
                <span class="availability-mode-switch__label">{{ t('availability.range.title') }}</span>
                <div class="availability-mode-switch__options" role="group" :aria-label="t('availability.range.title')">
                  <button
                    v-for="option in logRangeOptions"
                    :key="option.value"
                    type="button"
                    class="availability-mode-switch__option"
                    :class="{ 'availability-mode-switch__option--active': activeLogRange === option.value }"
                    @click="setLogAvailabilityRange(option.value)"
                  >
                    {{ option.label }}
                  </button>
                </div>
              </div>

              <button
                type="button"
                class="availability-primary-button"
                :disabled="refreshing"
                @click="refreshAll"
              >
                <svg
                  class="availability-primary-button__icon"
                  :class="{ 'availability-primary-button__icon--spinning': refreshing }"
                  viewBox="0 0 24 24"
                  fill="none"
                  aria-hidden="true"
                >
                  <path
                    d="M20 12a8 8 0 1 1-2.343-5.657"
                    stroke="currentColor"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="1.8"
                  />
                  <path
                    d="M20 4v5h-5"
                    stroke="currentColor"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="1.8"
                  />
                </svg>
                <span>
                  {{ refreshing ? (isLogAvailabilityMode ? t('availability.loadingLogs') : t('availability.refreshing')) : (isLogAvailabilityMode ? t('availability.reloadLogs') : t('availability.refreshAll')) }}
                </span>
              </button>
            </div>
          </div>
        </div>
      </header>

      <section class="availability-summary-grid" aria-label="summary cards">
        <article
          v-for="card in summaryCards"
          :key="card.key"
          class="availability-summary-card"
          :class="`availability-summary-card--${card.tone}`"
        >
          <span class="availability-summary-card__label">{{ card.label }}</span>
          <strong class="availability-summary-card__value">{{ card.value }}</strong>
        </article>
      </section>

      <div v-if="loading" class="availability-state-panel">
        <span class="availability-loader" aria-hidden="true"></span>
        <p>{{ t('availability.loading') }}</p>
      </div>

      <template v-else-if="providerCards.length > 0">
        <section class="availability-grid">
          <article
            v-for="card in providerCards"
            :key="card.providerKey"
            class="availability-provider-card"
            :class="[
              `availability-provider-card--${card.statusTone}`,
              {
                'availability-provider-card--checking': isCheckingCard(card.providerKey),
                'availability-provider-card--toggling': isTogglingCard(card.providerKey),
              },
            ]"
          >
            <header class="availability-provider-card__header">
              <div class="availability-provider-card__identity">
                <div class="availability-provider-card__status-icon" :class="`availability-provider-card__status-icon--${card.statusTone}`">
                  <svg v-if="card.statusTone === 'operational'" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <path
                      d="M8 12.5l2.7 2.7L16 9.8"
                      stroke="currentColor"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                    />
                    <circle cx="12" cy="12" r="8" stroke="currentColor" stroke-width="1.6" opacity="0.28" />
                  </svg>
                  <svg v-else-if="card.statusTone === 'degraded'" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <path
                      d="M12 7v5"
                      stroke="currentColor"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                    />
                    <circle cx="12" cy="15.5" r="1" fill="currentColor" />
                    <circle cx="12" cy="12" r="8" stroke="currentColor" stroke-width="1.6" opacity="0.28" />
                  </svg>
                  <svg v-else-if="card.statusTone === 'failed'" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <path
                      d="M9 9l6 6M15 9l-6 6"
                      stroke="currentColor"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                    />
                    <circle cx="12" cy="12" r="8" stroke="currentColor" stroke-width="1.6" opacity="0.28" />
                  </svg>
                  <svg v-else viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <path
                      d="M12 7v5l3 2"
                      stroke="currentColor"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                    />
                    <circle cx="12" cy="12" r="8" stroke="currentColor" stroke-width="1.6" opacity="0.28" />
                  </svg>
                </div>

                <div class="availability-provider-card__title-block">
                  <div class="availability-provider-card__name-row">
                    <h2>{{ card.providerName }}</h2>
                    <span v-if="!isLogAvailabilityMode && !card.availabilityMonitorEnabled" class="availability-provider-card__disabled-chip">
                      {{ t('availability.stats.disabled') }}
                    </span>
                    <span v-else-if="isLogAvailabilityMode" class="availability-provider-card__source-chip">
                      {{ t('availability.mode.logs') }}
                    </span>
                  </div>

                  <div class="availability-provider-card__meta-row">
                    <span class="availability-status-chip" :class="`availability-status-chip--${card.statusTone}`">
                      <span class="availability-status-chip__dot"></span>
                      {{ card.statusLabel }}
                    </span>

                    <span class="availability-provider-card__latency">
                      <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                        <path
                          d="M12 7v5l3 2"
                          stroke="currentColor"
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="1.8"
                        />
                        <circle cx="12" cy="12" r="8" stroke="currentColor" stroke-width="1.6" opacity="0.28" />
                      </svg>
                      {{ card.latestLatencyLabel }}
                    </span>
                  </div>

                  <p :title="card.lastCheckedFullLabel">
                    {{ card.platformLabel }}
                    <span aria-hidden="true">·</span>
                    {{ t('availability.card.lastChecked') }}
                    {{ (isLogAvailabilityMode || card.availabilityMonitorEnabled) ? card.lastCheckedLabel : t('availability.history.noData') }}
                  </p>
                </div>
              </div>

              <div v-if="!isLogAvailabilityMode" class="availability-provider-card__actions">
                <label
                  class="availability-toggle"
                  :class="{ 'availability-toggle--on': card.availabilityMonitorEnabled }"
                  :title="t('availability.card.toggleMonitoring')"
                >
                  <input
                    type="checkbox"
                    :checked="card.availabilityMonitorEnabled"
                    :disabled="isTogglingCard(card.providerKey)"
                    @change="toggleMonitor(card.platform, card.providerId, !card.availabilityMonitorEnabled)"
                  />
                  <span class="availability-toggle__track">
                    <span class="availability-toggle__thumb"></span>
                  </span>
                </label>

                <button
                  type="button"
                  class="availability-icon-button"
                  :disabled="isTogglingCard(card.providerKey)"
                  :title="t('availability.editConfig')"
                  @click="editConfig(card)"
                >
                  <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <path
                      d="M12 8.75a3.25 3.25 0 1 0 0 6.5 3.25 3.25 0 0 0 0-6.5Z"
                      stroke="currentColor"
                      stroke-width="1.8"
                    />
                    <path
                      d="M19.4 15a1 1 0 0 0 .2 1.1l.05.05a1.85 1.85 0 0 1-2.62 2.62l-.05-.05a1 1 0 0 0-1.1-.2 1 1 0 0 0-.6.92V19.5a1.85 1.85 0 1 1-3.7 0v-.07a1 1 0 0 0-.6-.92 1 1 0 0 0-1.1.2l-.05.05a1.85 1.85 0 1 1-2.62-2.62l.05-.05a1 1 0 0 0 .2-1.1 1 1 0 0 0-.92-.6H4.5a1.85 1.85 0 1 1 0-3.7h.07a1 1 0 0 0 .92-.6 1 1 0 0 0-.2-1.1l-.05-.05a1.85 1.85 0 1 1 2.62-2.62l.05.05a1 1 0 0 0 1.1.2 1 1 0 0 0 .6-.92V4.5a1.85 1.85 0 1 1 3.7 0v.07a1 1 0 0 0 .6.92 1 1 0 0 0 1.1-.2l.05-.05a1.85 1.85 0 1 1 2.62 2.62l-.05.05a1 1 0 0 0-.2 1.1 1 1 0 0 0 .92.6h.07a1.85 1.85 0 1 1 0 3.7h-.07a1 1 0 0 0-.92.6Z"
                      stroke="currentColor"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.5"
                    />
                  </svg>
                </button>
              </div>
            </header>

            <section class="availability-history-card">
              <div class="availability-history-card__heading">
                <span class="availability-history-card__label">{{ historyTitleLabel }}</span>
                <div class="availability-history-card__uptime">
                  <span>{{ t('availability.card.uptime') }}</span>
                  <strong>{{ card.uptimeLabel }}</strong>
                </div>
              </div>

              <div
                class="availability-history__track"
                :style="{ '--availability-history-segment-count': card.historySegments.length }"
              >
                <div
                  v-for="segment in card.historySegments"
                  :key="segment.key"
                  class="availability-history__segment"
                  :class="[
                    `availability-history__segment--${segment.tone}`,
                    { 'availability-history__segment--placeholder': segment.isPlaceholder },
                  ]"
                  role="img"
                  :aria-label="`${segment.checkedAtDateLabel} ${t('availability.history.tooltip.status')} ${segment.statusLabel} ${t('availability.history.tooltip.latency')} ${segment.latencyLabel}`"
                >
                  <span class="availability-history__tooltip" aria-hidden="true">
                    <span class="availability-history__tooltip-header">
                      <strong class="availability-history__tooltip-date">{{ segment.checkedAtDateLabel }}</strong>
                      <span
                        class="availability-history__tooltip-dot"
                        :class="`availability-history__tooltip-dot--${segment.tone}`"
                      ></span>
                    </span>

                    <span class="availability-history__tooltip-body">
                      <span class="availability-history__tooltip-row">
                        <span class="availability-history__tooltip-label">{{ t('availability.history.tooltip.status') }}</span>
                        <strong
                          class="availability-history__tooltip-value"
                          :class="`availability-history__tooltip-value--${segment.tone}`"
                        >
                          {{ segment.statusLabel }}
                        </strong>
                      </span>

                      <span class="availability-history__tooltip-row">
                        <span class="availability-history__tooltip-label">{{ t('availability.history.tooltip.latency') }}</span>
                        <span class="availability-history__tooltip-value availability-history__tooltip-value--latency">
                          {{ segment.latencyLabel }}
                        </span>
                      </span>
                    </span>
                  </span>
                </div>
              </div>

              <div class="availability-history__legend">
                <span>{{ historyStartLabel }}</span>
                <span class="availability-history__legend-line"></span>
                <span>{{ historyEndLabel }}</span>
              </div>
            </section>

            <p
              v-if="card.statusTone === 'failed' && card.latestError"
              class="availability-provider-card__hint"
              :title="card.latestError"
            >
              {{ card.latestError }}
            </p>

            <footer class="availability-provider-card__footer">
              <span class="availability-provider-card__footer-copy">
                {{ isLogAvailabilityMode ? t('availability.card.logSource') : (card.availabilityMonitorEnabled ? t('availability.card.autoRefresh', { seconds: REFRESH_INTERVAL_SECONDS }) : t('availability.enableToMonitor')) }}
              </span>

              <div v-if="!isLogAvailabilityMode" class="availability-provider-card__footer-actions">
                <button
                  v-if="card.availabilityMonitorEnabled"
                  type="button"
                  class="availability-secondary-button"
                  :disabled="isCheckingCard(card.providerKey)"
                  @click="checkSingle(card.platform, card.providerId)"
                >
                  <svg
                    viewBox="0 0 24 24"
                    fill="none"
                    aria-hidden="true"
                    :class="{ 'availability-secondary-button__icon--spinning': isCheckingCard(card.providerKey) }"
                  >
                    <path
                      d="M4 4v5h5"
                      stroke="currentColor"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.8"
                    />
                    <path
                      d="M20 20v-5h-5"
                      stroke="currentColor"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.8"
                    />
                    <path
                      d="M6.5 14a7 7 0 0 0 12.36 2.05M17.5 10A7 7 0 0 0 5.14 7.95"
                      stroke="currentColor"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.8"
                    />
                  </svg>
                  <span>{{ isCheckingCard(card.providerKey) ? t('availability.card.checking') : t('availability.card.quickCheck') }}</span>
                </button>

                <button
                  v-else
                  type="button"
                  class="availability-primary-link availability-primary-link--block"
                  :disabled="isTogglingCard(card.providerKey)"
                  @click="enableMonitoringAndEdit(card.platform, card)"
                >
                  {{ t('availability.enableMonitoring') }}
                </button>
              </div>
            </footer>
          </article>
        </section>

        <footer class="availability-page__legend">
          <div class="availability-page__legend-items">
            <span class="availability-page__legend-item">
              <i class="availability-page__legend-dot availability-page__legend-dot--operational"></i>
              {{ t('availability.stats.operational') }}
            </span>
            <span class="availability-page__legend-item">
              <i class="availability-page__legend-dot availability-page__legend-dot--degraded"></i>
              {{ t('availability.stats.degraded') }}
            </span>
            <span class="availability-page__legend-item">
              <i class="availability-page__legend-dot availability-page__legend-dot--failed"></i>
              {{ t('availability.stats.failed') }}
            </span>
          </div>
        </footer>
      </template>

      <div v-else class="availability-empty-state">
        <div class="availability-empty-state__icon" aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="none">
            <path
              d="M12 6v6l4 2"
              stroke="currentColor"
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.8"
            />
            <circle cx="12" cy="12" r="8" stroke="currentColor" stroke-width="1.8" />
          </svg>
        </div>
        <h2>{{ t('availability.noProviders') }}</h2>
        <p>{{ t('availability.subtitle') }}</p>
      </div>
    </div>

    <Teleport to="body">
      <div
        v-if="showConfigModal"
        class="availability-modal"
        :class="pageThemeClass"
        role="dialog"
        aria-modal="true"
      >
        <div class="availability-modal__backdrop" @click="closeConfigModal"></div>
        <div class="availability-modal__panel" @click.stop>
          <div class="availability-modal__header">
            <div>
              <h3>{{ t('availability.configTitle') }}</h3>
              <p>{{ activeProvider?.providerName }} · {{ resolvePlatformLabel(activeProvider?.platform || '') }}</p>
            </div>
            <button type="button" class="availability-icon-button" :title="t('common.close')" @click="closeConfigModal">
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path
                  d="M6 6l12 12M18 6L6 18"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.8"
                />
              </svg>
            </button>
          </div>

          <div class="availability-modal__body">
            <label class="availability-field">
              <span>{{ t('availability.field.testModel') }}</span>
              <input
                v-model="configForm.testModel"
                type="text"
                class="base-input availability-field__input"
                :placeholder="t('availability.placeholder.testModel')"
              />
            </label>

            <label class="availability-field">
              <span>{{ t('availability.field.testEndpoint') }}</span>
              <input
                v-model="configForm.testEndpoint"
                type="text"
                class="base-input availability-field__input"
                :placeholder="t('availability.placeholder.testEndpoint')"
              />
            </label>

            <label class="availability-field">
              <span>{{ t('availability.field.timeout') }}</span>
              <input
                v-model.number="configForm.timeout"
                type="number"
                min="1000"
                class="base-input availability-field__input"
                :placeholder="t('availability.placeholder.timeout')"
              />
              <small>{{ t('availability.hint.timeout') }}</small>
            </label>
          </div>

          <div class="availability-modal__footer">
            <button type="button" class="availability-tertiary-button" @click="closeConfigModal">
              {{ t('common.cancel') }}
            </button>
            <button type="button" class="availability-primary-button availability-primary-button--compact" :disabled="savingConfig" @click="saveConfig">
              <span>{{ savingConfig ? t('common.saving') : t('common.save') }}</span>
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.availability-page {
  position: relative;
  min-height: 100%;
  padding: 24px;
  overflow: hidden;
  color: #e2e8f0;
  background:
    radial-gradient(circle at top right, rgba(37, 99, 235, 0.12), transparent 28%),
    radial-gradient(circle at left bottom, rgba(34, 197, 94, 0.08), transparent 24%),
    linear-gradient(180deg, #0f1115 0%, #11161f 52%, #0d1118 100%);
}

.availability-page button {
  margin: 0;
}

.availability-page__grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(148, 163, 184, 0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(148, 163, 184, 0.04) 1px, transparent 1px);
  background-size: 32px 32px;
  mask-image: linear-gradient(180deg, rgba(0, 0, 0, 0.42), transparent 82%);
  pointer-events: none;
}

.availability-page__glow {
  position: absolute;
  border-radius: 999px;
  filter: blur(80px);
  pointer-events: none;
}

.availability-page__glow--primary {
  top: -120px;
  right: -40px;
  width: 280px;
  height: 280px;
  background: rgba(59, 130, 246, 0.16);
}

.availability-page__glow--secondary {
  left: -80px;
  bottom: 80px;
  width: 240px;
  height: 240px;
  background: rgba(56, 189, 248, 0.1);
}

.availability-page__shell {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 22px;
  max-width: 1280px;
  margin: 0 auto;
}

.availability-hero {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: 18px;
}

.availability-hero__copy {
  display: flex;
  flex-direction: column;
  flex: 1 1 100%;
  gap: 14px;
  min-width: 0;
}

.availability-hero__title-wrap {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  width: 100%;
  min-width: 0;
}

.availability-hero__title-main {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

.availability-hero__title-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 14px;
  border: 1px solid rgba(96, 165, 250, 0.18);
  background: rgba(15, 23, 42, 0.74);
  color: #60a5fa;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06);
}

.availability-hero__title-icon svg {
  width: 22px;
  height: 22px;
}

.availability-hero__title-group h1,
.availability-empty-state h2,
.availability-modal__header h3,
.availability-provider-card__title-block h2 {
  margin: 0;
}

.availability-hero__title-group h1 {
  font-size: clamp(1.9rem, 2.8vw, 2.6rem);
  font-weight: 700;
  letter-spacing: -0.04em;
  color: #f8fafc;
}

.availability-hero__title-group p,
.availability-empty-state p,
.availability-modal__header p,
.availability-state-panel p,
.availability-provider-card__title-block p,
.availability-provider-card__footer-copy,
.availability-field small {
  margin: 0;
  color: rgba(148, 163, 184, 0.8);
}

.availability-hero__title-group p {
  margin-top: 4px;
  font-size: 0.96rem;
}

.availability-hero__runtime {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 8px;
  width: fit-content;
  min-height: 32px;
  padding: 0 12px;
  border-radius: 999px;
  border: 1px solid rgba(248, 113, 113, 0.16);
  background: rgba(15, 23, 42, 0.68);
  color: #fca5a5;
  font-size: 0.82rem;
  font-weight: 600;
}

.availability-hero__runtime--active {
  border-color: rgba(74, 222, 128, 0.18);
  color: #86efac;
}

.availability-hero__runtime-dot,
.availability-status-chip__dot,
.availability-page__legend-dot {
  border-radius: 999px;
  background: currentColor;
}

.availability-hero__runtime-dot {
  width: 8px;
  height: 8px;
  box-shadow: 0 0 0 4px rgba(255, 255, 255, 0.04);
}

.availability-hero__control-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-width: 0;
}

.availability-hero__actions {
  display: flex;
  flex-wrap: wrap;
  flex: 1 1 auto;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  min-width: 0;
}

.availability-runtime-card,
.availability-summary-card,
.availability-provider-card,
.availability-state-panel,
.availability-empty-state,
.availability-history-card,
.availability-modal__panel {
  border: 1px solid rgba(148, 163, 184, 0.12);
  background: rgba(15, 23, 42, 0.56);
  backdrop-filter: blur(18px);
}

.availability-runtime-card {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  flex: 0 0 auto;
  gap: 0;
  min-width: 222px;
  overflow: hidden;
  border-radius: 16px;
  box-shadow: 0 12px 24px rgba(2, 6, 23, 0.16);
}

.availability-runtime-card__item {
  display: flex;
  flex-direction: column;
  gap: 5px;
  padding: 10px 14px;
}

.availability-runtime-card__item + .availability-runtime-card__item {
  border-left: 1px solid rgba(148, 163, 184, 0.08);
}

.availability-runtime-card__item span,
.availability-history-card__label,
.availability-summary-card__label {
  font-size: 0.78rem;
  color: rgba(148, 163, 184, 0.82);
  letter-spacing: 0.04em;
}

.availability-runtime-card__item strong,
.availability-history-card__uptime strong {
  color: #93c5fd;
  font-size: 1rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.availability-primary-button,
.availability-secondary-button,
.availability-tertiary-button,
.availability-primary-link,
.availability-icon-button,
.availability-mode-switch__option,
.availability-history__segment,
.availability-toggle {
  transition:
    transform 0.2s ease,
    box-shadow 0.2s ease,
    border-color 0.2s ease,
    background-color 0.2s ease,
    color 0.2s ease,
    opacity 0.2s ease;
}

.availability-primary-button,
.availability-secondary-button,
.availability-tertiary-button,
.availability-primary-link,
.availability-icon-button,
.availability-mode-switch__option {
  cursor: pointer;
}

.availability-primary-button:disabled,
.availability-secondary-button:disabled,
.availability-tertiary-button:disabled,
.availability-primary-link:disabled,
.availability-icon-button:disabled {
  cursor: not-allowed;
  opacity: 0.56;
  transform: none;
  box-shadow: none;
}

.availability-primary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 48px;
  padding: 0 18px;
  border: none;
  border-radius: 16px;
  background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
  color: #eff6ff;
  font-size: 0.92rem;
  font-weight: 700;
  box-shadow: 0 16px 28px rgba(37, 99, 235, 0.22);
}

.availability-primary-button--compact {
  min-height: 44px;
  padding: 0 16px;
}

.availability-primary-button:hover:not(:disabled),
.availability-secondary-button:hover:not(:disabled),
.availability-tertiary-button:hover:not(:disabled),
.availability-primary-link:hover:not(:disabled),
.availability-icon-button:hover:not(:disabled) {
  transform: translateY(-2px);
}

.availability-primary-button__icon,
.availability-secondary-button svg,
.availability-icon-button svg,
.availability-empty-state__icon svg {
  width: 18px;
  height: 18px;
}

.availability-primary-button__icon--spinning,
.availability-secondary-button__icon--spinning {
  animation: availability-spin 0.9s linear infinite;
}

.availability-mode-switch {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-height: 48px;
  padding: 6px 8px 6px 14px;
  border: 1px solid rgba(96, 165, 250, 0.14);
  border-radius: 16px;
  background: rgba(15, 23, 42, 0.58);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.05), 0 14px 26px rgba(2, 6, 23, 0.14);
}

.availability-mode-switch__label {
  color: rgba(191, 219, 254, 0.9);
  font-size: 0.8rem;
  font-weight: 700;
  white-space: nowrap;
}

.availability-mode-switch__options {
  display: inline-flex;
  gap: 4px;
  padding: 3px;
  border-radius: 12px;
  background: rgba(2, 6, 23, 0.22);
}

.availability-mode-switch__option {
  min-height: 32px;
  padding: 0 12px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: transparent;
  color: rgba(203, 213, 225, 0.82);
  font-size: 0.78rem;
  font-weight: 800;
  white-space: nowrap;
}

.availability-mode-switch__option:hover {
  color: #e0f2fe;
  background: rgba(96, 165, 250, 0.12);
}

.availability-mode-switch__option--active {
  border-color: rgba(96, 165, 250, 0.24);
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.9) 0%, rgba(59, 130, 246, 0.86) 100%);
  color: #eff6ff;
  box-shadow: 0 8px 18px rgba(37, 99, 235, 0.24);
}

.availability-range-switch .availability-mode-switch__options {
  flex-wrap: wrap;
}

.availability-range-switch .availability-mode-switch__option {
  min-width: 54px;
  padding-inline: 10px;
}

.availability-summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.availability-summary-card {
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 92px;
  padding: 16px 18px;
  border-radius: 20px;
  box-shadow: 0 14px 30px rgba(2, 6, 23, 0.16);
}

.availability-summary-card::before,
.availability-provider-card::before {
  content: '';
  position: absolute;
  inset: 0 auto 0 0;
  width: 3px;
  border-radius: 999px;
}

.availability-summary-card--operational::before,
.availability-provider-card--operational::before {
  background: #22c55e;
}

.availability-summary-card--degraded::before,
.availability-provider-card--degraded::before {
  background: #f59e0b;
}

.availability-summary-card--failed::before,
.availability-provider-card--failed::before {
  background: #ef4444;
}

.availability-summary-card--disabled::before,
.availability-provider-card--disabled::before {
  background: #64748b;
}

.availability-summary-card--operational {
  border-color: rgba(74, 222, 128, 0.18);
}

.availability-summary-card--degraded {
  border-color: rgba(251, 191, 36, 0.18);
}

.availability-summary-card--failed {
  border-color: rgba(248, 113, 113, 0.18);
}

.availability-summary-card__value {
  font-size: clamp(2rem, 2.6vw, 2.4rem);
  font-weight: 700;
  letter-spacing: -0.05em;
  color: #f8fafc;
}

.availability-summary-card--operational .availability-summary-card__value {
  color: #86efac;
}

.availability-summary-card--degraded .availability-summary-card__value {
  color: #fde68a;
}

.availability-summary-card--failed .availability-summary-card__value {
  color: #fca5a5;
}

.availability-summary-card--disabled .availability-summary-card__value {
  color: #cbd5e1;
}

.availability-state-panel,
.availability-empty-state {
  min-height: 280px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  text-align: center;
  border-radius: 24px;
}

.availability-loader {
  width: 42px;
  height: 42px;
  border-radius: 999px;
  border: 3px solid rgba(148, 163, 184, 0.16);
  border-top-color: #60a5fa;
  animation: availability-spin 0.85s linear infinite;
}

.availability-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}

.availability-provider-card {
  --availability-card-edge: #64748b;
  --availability-card-glow: rgba(100, 116, 139, 0.22);
  position: relative;
  z-index: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 320px;
  padding: 20px;
  border-radius: 24px;
  background: rgba(22, 27, 34, 0.82);
  box-shadow: 0 18px 38px rgba(2, 6, 23, 0.24);
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease,
    background-color 0.2s ease;
}

.availability-provider-card--operational {
  --availability-card-edge: #22c55e;
  --availability-card-glow: rgba(34, 197, 94, 0.24);
  border-color: rgba(74, 222, 128, 0.14);
}

.availability-provider-card--degraded {
  --availability-card-edge: #f59e0b;
  --availability-card-glow: rgba(245, 158, 11, 0.24);
  border-color: rgba(251, 191, 36, 0.16);
}

.availability-provider-card--failed {
  --availability-card-edge: #ef4444;
  --availability-card-glow: rgba(239, 68, 68, 0.26);
  border-color: rgba(248, 113, 113, 0.22);
  animation: availability-breathe 2.6s ease-in-out infinite;
}

.availability-provider-card--disabled {
  --availability-card-edge: #64748b;
  --availability-card-glow: rgba(100, 116, 139, 0.18);
  border-color: rgba(148, 163, 184, 0.1);
  opacity: 0.76;
}

.availability-provider-card::before {
  inset: 0;
  width: auto;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--availability-card-edge) 0 3px, transparent 3px 100%);
  pointer-events: none;
  transition: filter 0.2s ease;
}

.availability-provider-card::after {
  content: '';
  position: absolute;
  inset: -1px;
  border-radius: inherit;
  opacity: 0;
  pointer-events: none;
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.04),
    0 0 24px var(--availability-card-glow);
  transition: opacity 0.2s ease, box-shadow 0.2s ease;
}

.availability-provider-card:hover {
  z-index: 10;
  border-color: color-mix(in srgb, var(--availability-card-edge) 36%, rgba(148, 163, 184, 0.18));
  box-shadow:
    0 18px 40px rgba(2, 6, 23, 0.24),
    0 0 26px var(--availability-card-glow);
}

.availability-provider-card:focus-within {
  z-index: 10;
}

.availability-provider-card:hover::before {
  filter: drop-shadow(0 0 8px var(--availability-card-glow));
}

.availability-provider-card:hover::after {
  opacity: 1;
}

.availability-provider-card--checking,
.availability-provider-card--toggling {
  border-color: rgba(96, 165, 250, 0.24);
}

.availability-provider-card__header,
.availability-provider-card__footer,
.availability-modal__header,
.availability-modal__footer,
.availability-history-card__heading,
.availability-history__legend {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.availability-provider-card__header {
  align-items: flex-start;
}

.availability-provider-card__footer {
  flex-wrap: wrap;
}

.availability-provider-card__identity {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  min-width: 0;
}

.availability-provider-card__status-icon {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 14px;
  border: 1px solid rgba(148, 163, 184, 0.12);
  background: rgba(15, 23, 42, 0.68);
}

.availability-provider-card__status-icon svg {
  width: 22px;
  height: 22px;
}

.availability-provider-card__status-icon--operational {
  color: #86efac;
  background: rgba(34, 197, 94, 0.12);
}

.availability-provider-card__status-icon--degraded {
  color: #fde68a;
  background: rgba(245, 158, 11, 0.14);
}

.availability-provider-card__status-icon--failed {
  color: #fca5a5;
  background: rgba(239, 68, 68, 0.16);
}

.availability-provider-card__status-icon--disabled {
  color: #94a3b8;
  background: rgba(71, 85, 105, 0.18);
}

.availability-provider-card__title-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.availability-provider-card__name-row,
.availability-provider-card__meta-row,
.availability-page__legend,
.availability-page__legend-items,
.availability-page__legend-item,
.availability-provider-card__actions,
.availability-provider-card__footer-actions {
  display: flex;
  align-items: center;
}

.availability-provider-card__name-row,
.availability-provider-card__meta-row {
  flex-wrap: wrap;
  gap: 10px;
}

.availability-provider-card__title-block h2 {
  font-size: 1.08rem;
  line-height: 1.28;
  color: #f8fafc;
  word-break: break-word;
}

.availability-provider-card__title-block p {
  font-size: 0.82rem;
  line-height: 1.5;
}

.availability-provider-card__title-block p span {
  margin: 0 4px;
}

.availability-provider-card__disabled-chip,
.availability-provider-card__source-chip,
.availability-status-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 26px;
  padding: 0 10px;
  border-radius: 999px;
  font-size: 0.74rem;
  font-weight: 700;
}

.availability-provider-card__disabled-chip {
  border: 1px solid rgba(148, 163, 184, 0.14);
  background: rgba(51, 65, 85, 0.44);
  color: #94a3b8;
}

.availability-provider-card__source-chip {
  border: 1px solid rgba(96, 165, 250, 0.18);
  background: rgba(37, 99, 235, 0.12);
  color: #bfdbfe;
  font-weight: 800;
}

.availability-status-chip {
  border: 1px solid transparent;
}

.availability-status-chip__dot {
  width: 7px;
  height: 7px;
  box-shadow: 0 0 0 4px rgba(255, 255, 255, 0.04);
}

.availability-status-chip--operational {
  background: rgba(34, 197, 94, 0.12);
  border-color: rgba(74, 222, 128, 0.18);
  color: #86efac;
}

.availability-status-chip--degraded {
  background: rgba(245, 158, 11, 0.14);
  border-color: rgba(251, 191, 36, 0.2);
  color: #fde68a;
}

.availability-status-chip--failed {
  background: rgba(239, 68, 68, 0.14);
  border-color: rgba(248, 113, 113, 0.2);
  color: #fca5a5;
}

.availability-status-chip--disabled {
  background: rgba(71, 85, 105, 0.18);
  border-color: rgba(148, 163, 184, 0.16);
  color: #cbd5e1;
}

.availability-provider-card__latency {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: rgba(148, 163, 184, 0.86);
  font-size: 0.78rem;
  font-weight: 600;
}

.availability-provider-card__latency svg {
  width: 14px;
  height: 14px;
}

.availability-provider-card__actions,
.availability-provider-card__footer-actions {
  gap: 10px;
}

.availability-toggle {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.availability-toggle input {
  position: absolute;
  inset: 0;
  opacity: 0;
}

.availability-toggle__track {
  position: relative;
  width: 42px;
  height: 24px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: rgba(71, 85, 105, 0.72);
  box-shadow: inset 0 2px 6px rgba(2, 6, 23, 0.34);
}

.availability-toggle__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 18px;
  height: 18px;
  border-radius: 999px;
  background: linear-gradient(180deg, #ffffff 0%, #dbeafe 100%);
  box-shadow: 0 4px 12px rgba(2, 6, 23, 0.32);
}

.availability-toggle--on .availability-toggle__track {
  border-color: rgba(96, 165, 250, 0.24);
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.9) 0%, rgba(59, 130, 246, 0.9) 100%);
}

.availability-toggle--on .availability-toggle__thumb {
  transform: translateX(18px);
}

.availability-icon-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: 1px solid rgba(148, 163, 184, 0.12);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.04);
  color: #cbd5e1;
}

.availability-history-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px 16px;
  border-radius: 18px;
  background: rgba(9, 14, 22, 0.58);
}

.availability-history-card__uptime {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.availability-history-card__uptime span,
.availability-history__legend {
  color: rgba(148, 163, 184, 0.78);
  font-size: 0.78rem;
}

.availability-history__track {
  --availability-history-segment-gap: 2px;
  --availability-history-segment-count: 72;
  --availability-history-segment-width: calc(
    (100% - (var(--availability-history-segment-count) - 1) * var(--availability-history-segment-gap)) /
      var(--availability-history-segment-count)
  );
  position: relative;
  display: flex;
  align-items: stretch;
  gap: var(--availability-history-segment-gap);
  height: 28px;
  width: 100%;
  overflow: visible;
  isolation: isolate;
}

.availability-history__track > * {
  flex: 0 0 var(--availability-history-segment-width);
  align-items: stretch;
  width: var(--availability-history-segment-width);
  max-width: var(--availability-history-segment-width);
  min-width: 0;
}

.availability-history__segment {
  position: relative;
  display: block;
  width: 100%;
  min-width: 0;
  height: 100%;
  min-height: 100%;
  flex: 0 0 auto;
  margin: 0;
  border-radius: 4px;
  background: rgba(148, 163, 184, 0.08);
  box-shadow: 0 0 0 1px rgba(8, 12, 18, 0.82);
  cursor: help;
  overflow: visible;
}

.availability-history__segment--operational {
  background: #22c55e;
}

.availability-history__segment--degraded {
  background: #f59e0b;
}

.availability-history__segment--failed {
  background: #ef4444;
}

.availability-history__segment--empty,
.availability-provider-card--disabled .availability-history__segment {
  background: rgba(71, 85, 105, 0.52);
}

.availability-history__segment--placeholder {
  cursor: default;
}

.availability-history__segment:hover {
  z-index: 3;
  transform: translateY(-1px);
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.08),
    0 6px 16px rgba(2, 6, 23, 0.22);
}

.availability-history__tooltip {
  --availability-history-tooltip-bg: linear-gradient(180deg, rgba(30, 38, 53, 0.98) 0%, rgba(22, 28, 40, 0.98) 100%);
  --availability-history-tooltip-border: rgba(94, 111, 144, 0.48);
  position: absolute;
  left: 50%;
  bottom: calc(100% + 10px);
  z-index: 20;
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 154px;
  padding: 13px 14px 12px;
  border-radius: 16px;
  border: 1px solid var(--availability-history-tooltip-border);
  background: var(--availability-history-tooltip-bg);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 18px 40px rgba(2, 6, 23, 0.46);
  color: #e2e8f0;
  font-size: 0.76rem;
  white-space: nowrap;
  transform: translate(-50%, 8px);
  opacity: 0;
  pointer-events: none;
  backdrop-filter: blur(18px);
}

.availability-history__tooltip::after {
  content: '';
  position: absolute;
  left: 50%;
  top: calc(100% - 1px);
  width: 12px;
  height: 12px;
  background: var(--availability-history-tooltip-bg);
  border-right: 1px solid var(--availability-history-tooltip-border);
  border-bottom: 1px solid var(--availability-history-tooltip-border);
  transform: translateX(-50%) rotate(45deg);
}

.availability-history__tooltip-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.availability-history__tooltip-date {
  color: #f8fafc;
  font-size: 0.98rem;
  font-weight: 800;
  letter-spacing: 0.02em;
  font-variant-numeric: tabular-nums;
}

.availability-history__tooltip-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  flex: 0 0 auto;
  box-shadow: 0 0 0 4px rgba(255, 255, 255, 0.04);
}

.availability-history__tooltip-dot--operational {
  background: #64d67a;
}

.availability-history__tooltip-dot--degraded {
  background: #f4be42;
}

.availability-history__tooltip-dot--failed {
  background: #fb6b66;
}

.availability-history__tooltip-dot--disabled,
.availability-history__tooltip-dot--empty {
  background: #64748b;
}

.availability-history__tooltip-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.availability-history__tooltip-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
}

.availability-history__tooltip-label {
  color: rgba(148, 163, 184, 0.82);
  font-size: 0.76rem;
  font-weight: 600;
}

.availability-history__tooltip-value {
  color: #e5e7eb;
  font-size: 0.76rem;
  font-weight: 700;
  text-align: right;
}

.availability-history__tooltip-value--operational {
  color: #64d67a;
}

.availability-history__tooltip-value--degraded {
  color: #f4be42;
}

.availability-history__tooltip-value--failed {
  color: #fb6b66;
}

.availability-history__tooltip-value--disabled,
.availability-history__tooltip-value--empty {
  color: #94a3b8;
}

.availability-history__tooltip-value--latency {
  color: #e5e7eb;
  font-size: 0.95rem;
  font-variant-numeric: tabular-nums;
}

.availability-history__segment:hover .availability-history__tooltip {
  opacity: 1;
  transform: translate(-50%, 0);
}

.availability-history__legend {
  gap: 12px;
}

.availability-history__legend-line {
  flex: 1;
  height: 1px;
  background: rgba(148, 163, 184, 0.16);
}

.availability-provider-card__hint {
  margin: -4px 0 0;
  color: rgba(252, 165, 165, 0.9);
  font-size: 0.78rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.availability-provider-card__footer {
  margin-top: auto;
  padding-top: 14px;
  border-top: 1px solid rgba(148, 163, 184, 0.08);
}

.availability-provider-card__footer-copy {
  flex: 1 1 220px;
  font-size: 0.8rem;
}

.availability-secondary-button,
.availability-primary-link,
.availability-tertiary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 40px;
  padding: 0 14px;
  border-radius: 14px;
  font-size: 0.84rem;
  font-weight: 700;
}

.availability-secondary-button {
  border: 1px solid rgba(96, 165, 250, 0.16);
  background: rgba(37, 99, 235, 0.1);
  color: #bfdbfe;
}

.availability-primary-link {
  border: 1px solid rgba(96, 165, 250, 0.18);
  background: rgba(37, 99, 235, 0.14);
  color: #dbeafe;
}

.availability-primary-link--block {
  min-width: 120px;
}

.availability-tertiary-button {
  border: 1px solid rgba(148, 163, 184, 0.14);
  background: rgba(255, 255, 255, 0.04);
  color: #e2e8f0;
}

.availability-empty-state__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 72px;
  height: 72px;
  border-radius: 22px;
  background: rgba(37, 99, 235, 0.12);
  color: #93c5fd;
}

.availability-page__legend {
  justify-content: center;
  padding: 10px 0 4px;
}

.availability-page__legend-items {
  flex-wrap: wrap;
  justify-content: center;
  gap: 18px;
  color: rgba(148, 163, 184, 0.86);
  font-size: 0.82rem;
}

.availability-page__legend-item {
  gap: 8px;
}

.availability-page__legend-dot {
  display: inline-block;
  width: 10px;
  height: 10px;
}

.availability-page__legend-dot--operational {
  color: #22c55e;
}

.availability-page__legend-dot--degraded {
  color: #f59e0b;
}

.availability-page__legend-dot--failed {
  color: #ef4444;
}

.availability-modal {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.availability-modal__backdrop {
  position: absolute;
  inset: 0;
  background: rgba(2, 6, 23, 0.72);
  backdrop-filter: blur(8px);
}

.availability-modal__panel {
  position: relative;
  z-index: 1;
  width: min(560px, 100%);
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 24px;
  border-radius: 24px;
  background: rgba(11, 16, 24, 0.94);
  box-shadow: 0 28px 70px rgba(2, 6, 23, 0.48);
}

.availability-modal__header,
.availability-field {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.availability-modal__header {
  flex-direction: row;
  align-items: flex-start;
}

.availability-modal__body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.availability-field span {
  color: #f8fafc;
  font-size: 0.92rem;
  font-weight: 600;
}

.availability-field__input {
  background: rgba(15, 23, 42, 0.72);
}

.availability-primary-button:focus-visible,
.availability-secondary-button:focus-visible,
.availability-tertiary-button:focus-visible,
.availability-primary-link:focus-visible,
.availability-icon-button:focus-visible,
.availability-toggle:focus-within {
  outline: 2px solid rgba(96, 165, 250, 0.78);
  outline-offset: 2px;
}

.availability-page--light {
  color: #111827;
  background:
    radial-gradient(circle at 14% 2%, rgba(96, 165, 250, 0.12), transparent 28%),
    radial-gradient(circle at 86% 12%, rgba(59, 130, 246, 0.08), transparent 30%),
    linear-gradient(180deg, #f8fafc 0%, #f3f6fb 46%, #eef2f7 100%);
}

.availability-page--light .availability-page__grid {
  background-image:
    linear-gradient(rgba(148, 163, 184, 0.08) 1px, transparent 1px),
    linear-gradient(90deg, rgba(148, 163, 184, 0.08) 1px, transparent 1px);
  mask-image: linear-gradient(180deg, rgba(255, 255, 255, 0.38), transparent 82%);
}

.availability-page--light .availability-page__glow--primary {
  background: rgba(96, 165, 250, 0.16);
}

.availability-page--light .availability-page__glow--secondary {
  background: rgba(52, 211, 153, 0.12);
}

.availability-page--light .availability-hero__title-icon {
  border-color: rgba(147, 197, 253, 0.56);
  background: linear-gradient(180deg, rgba(239, 246, 255, 0.98), rgba(219, 234, 254, 0.76));
  color: #2563eb;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.9),
    0 10px 24px rgba(37, 99, 235, 0.1);
}

.availability-page--light .availability-hero__title-group h1,
.availability-page--light .availability-empty-state h2,
.availability-page--light .availability-modal__header h3,
.availability-page--light .availability-provider-card__title-block h2 {
  color: #111827;
}

.availability-modal.availability-page--light .availability-modal__header h3 {
  color: #111827;
}

.availability-page--light .availability-hero__title-group p,
.availability-page--light .availability-empty-state p,
.availability-page--light .availability-modal__header p,
.availability-page--light .availability-state-panel p,
.availability-page--light .availability-provider-card__title-block p,
.availability-page--light .availability-provider-card__footer-copy,
.availability-page--light .availability-field small {
  color: #64748b;
}

.availability-modal.availability-page--light .availability-modal__header p,
.availability-modal.availability-page--light .availability-field small {
  color: #64748b;
}

.availability-page--light .availability-hero__runtime {
  border-color: rgba(248, 113, 113, 0.22);
  background: rgba(254, 242, 242, 0.92);
  color: #dc2626;
}

.availability-page--light .availability-hero__runtime--active {
  border-color: rgba(34, 197, 94, 0.22);
  background: rgba(240, 253, 244, 0.96);
  color: #16a34a;
}

.availability-page--light .availability-hero__runtime-dot,
.availability-page--light .availability-status-chip__dot,
.availability-page--light .availability-history__tooltip-dot {
  box-shadow: 0 0 0 4px rgba(15, 23, 42, 0.05);
}

.availability-page--light .availability-runtime-card,
.availability-page--light .availability-summary-card,
.availability-page--light .availability-provider-card,
.availability-page--light .availability-state-panel,
.availability-page--light .availability-empty-state,
.availability-page--light .availability-history-card,
.availability-page--light .availability-modal__panel {
  border-color: rgba(203, 213, 225, 0.82);
  background: rgba(255, 255, 255, 0.95);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.9),
    0 16px 36px rgba(15, 23, 42, 0.08);
}

.availability-page--light .availability-runtime-card__item + .availability-runtime-card__item {
  border-left-color: rgba(203, 213, 225, 0.76);
}

.availability-page--light .availability-runtime-card__item span,
.availability-page--light .availability-history-card__label,
.availability-page--light .availability-summary-card__label {
  color: #64748b;
}

.availability-page--light .availability-runtime-card__item strong,
.availability-page--light .availability-history-card__uptime strong {
  color: #2563eb;
}

.availability-page--light .availability-summary-card {
  background: #ffffff;
  box-shadow: 0 14px 28px rgba(15, 23, 42, 0.08);
}

.availability-page--light .availability-summary-card--operational {
  border-color: rgba(34, 197, 94, 0.2);
  box-shadow: 0 14px 28px rgba(15, 23, 42, 0.08), inset 0 -4px 0 rgba(34, 197, 94, 0.68);
}

.availability-page--light .availability-summary-card--degraded {
  border-color: rgba(245, 158, 11, 0.24);
  box-shadow: 0 14px 28px rgba(15, 23, 42, 0.08), inset 0 -4px 0 rgba(245, 158, 11, 0.72);
}

.availability-page--light .availability-summary-card--failed {
  border-color: rgba(239, 68, 68, 0.24);
  box-shadow: 0 14px 28px rgba(15, 23, 42, 0.08), inset 0 -4px 0 rgba(239, 68, 68, 0.68);
}

.availability-page--light .availability-summary-card--disabled {
  border-color: rgba(148, 163, 184, 0.36);
  box-shadow: 0 14px 28px rgba(15, 23, 42, 0.08), inset 0 -4px 0 rgba(148, 163, 184, 0.56);
}

.availability-page--light .availability-summary-card--operational .availability-summary-card__value {
  color: #16a34a;
}

.availability-page--light .availability-summary-card--degraded .availability-summary-card__value {
  color: #c0841a;
}

.availability-page--light .availability-summary-card--failed .availability-summary-card__value {
  color: #dc2626;
}

.availability-page--light .availability-summary-card--disabled .availability-summary-card__value {
  color: #334155;
}

.availability-page--light .availability-loader {
  border-color: rgba(203, 213, 225, 0.76);
  border-top-color: #3b82f6;
}

.availability-page--light .availability-provider-card {
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 18px 38px rgba(15, 23, 42, 0.08);
}

.availability-page--light .availability-provider-card::after {
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.92),
    0 0 24px var(--availability-card-glow);
}

.availability-page--light .availability-provider-card:hover {
  border-color: color-mix(in srgb, var(--availability-card-edge) 38%, rgba(148, 163, 184, 0.34));
  box-shadow:
    0 18px 40px rgba(15, 23, 42, 0.09),
    0 0 24px var(--availability-card-glow);
}

.availability-page--light .availability-provider-card--operational {
  border-color: rgba(34, 197, 94, 0.22);
}

.availability-page--light .availability-provider-card--degraded {
  border-color: rgba(245, 158, 11, 0.24);
}

.availability-page--light .availability-provider-card--failed {
  border-color: rgba(239, 68, 68, 0.28);
}

.availability-page--light .availability-provider-card--disabled {
  border-color: rgba(203, 213, 225, 0.82);
  opacity: 0.82;
}

.availability-page--light .availability-provider-card--checking,
.availability-page--light .availability-provider-card--toggling {
  border-color: rgba(59, 130, 246, 0.32);
}

.availability-page--light .availability-provider-card__status-icon {
  border-color: rgba(203, 213, 225, 0.82);
  background: #f8fafc;
}

.availability-page--light .availability-provider-card__status-icon--operational {
  color: #16a34a;
  background: rgba(240, 253, 244, 0.98);
}

.availability-page--light .availability-provider-card__status-icon--degraded {
  color: #c0841a;
  background: rgba(255, 251, 235, 0.98);
}

.availability-page--light .availability-provider-card__status-icon--failed {
  color: #dc2626;
  background: rgba(254, 242, 242, 0.98);
}

.availability-page--light .availability-provider-card__status-icon--disabled {
  color: #94a3b8;
  background: #f1f5f9;
}

.availability-page--light .availability-provider-card__disabled-chip {
  border-color: rgba(203, 213, 225, 0.9);
  background: #f1f5f9;
  color: #64748b;
}

.availability-page--light .availability-provider-card__source-chip {
  border-color: rgba(59, 130, 246, 0.2);
  background: rgba(239, 246, 255, 0.98);
  color: #2563eb;
}

.availability-page--light .availability-mode-switch {
  border-color: rgba(191, 219, 254, 0.88);
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.08);
}

.availability-page--light .availability-mode-switch__label {
  color: #475569;
}

.availability-page--light .availability-mode-switch__options {
  background: #eff6ff;
}

.availability-page--light .availability-mode-switch__option {
  color: #64748b;
}

.availability-page--light .availability-mode-switch__option:hover {
  color: #2563eb;
  background: rgba(191, 219, 254, 0.62);
}

.availability-page--light .availability-mode-switch__option--active {
  color: #eff6ff;
}

.availability-page--light .availability-status-chip--operational {
  background: rgba(240, 253, 244, 0.98);
  border-color: rgba(34, 197, 94, 0.22);
  color: #16a34a;
}

.availability-page--light .availability-status-chip--degraded {
  background: rgba(255, 251, 235, 0.98);
  border-color: rgba(245, 158, 11, 0.24);
  color: #c0841a;
}

.availability-page--light .availability-status-chip--failed {
  background: rgba(254, 242, 242, 0.98);
  border-color: rgba(239, 68, 68, 0.24);
  color: #dc2626;
}

.availability-page--light .availability-status-chip--disabled {
  background: #f1f5f9;
  border-color: rgba(203, 213, 225, 0.9);
  color: #64748b;
}

.availability-page--light .availability-provider-card__latency,
.availability-page--light .availability-history-card__uptime span,
.availability-page--light .availability-history__legend,
.availability-page--light .availability-page__legend-items {
  color: #94a3b8;
}

.availability-page--light .availability-toggle__track {
  border-color: rgba(203, 213, 225, 0.9);
  background: #e5e7eb;
  box-shadow: inset 0 2px 5px rgba(15, 23, 42, 0.08);
}

.availability-page--light .availability-toggle__thumb {
  background: #ffffff;
  box-shadow: 0 3px 10px rgba(15, 23, 42, 0.18);
}

.availability-page--light .availability-toggle--on .availability-toggle__track {
  border-color: rgba(59, 130, 246, 0.4);
  background: linear-gradient(135deg, #2563eb 0%, #4f7df3 100%);
}

.availability-page--light .availability-icon-button {
  border-color: rgba(203, 213, 225, 0.82);
  background: #f8fafc;
  color: #94a3b8;
}

.availability-modal.availability-page--light .availability-icon-button {
  border-color: rgba(203, 213, 225, 0.82);
  background: #f8fafc;
  color: #94a3b8;
}

.availability-page--light .availability-history-card {
  background: #f8fafc;
}

.availability-page--light .availability-history__segment {
  background: #e5e7eb;
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.92);
}

.availability-page--light .availability-history__segment--operational {
  background: #4ade80;
}

.availability-page--light .availability-history__segment--degraded {
  background: #f59e0b;
}

.availability-page--light .availability-history__segment--failed {
  background: #ef4444;
}

.availability-page--light .availability-history__segment--empty,
.availability-page--light .availability-provider-card--disabled .availability-history__segment {
  background: #e5e7eb;
}

.availability-page--light .availability-history__segment:hover {
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.95),
    0 8px 18px rgba(15, 23, 42, 0.16);
}

.availability-page--light .availability-history__tooltip {
  --availability-history-tooltip-bg: linear-gradient(180deg, rgba(255, 255, 255, 0.99) 0%, rgba(248, 250, 252, 0.99) 100%);
  --availability-history-tooltip-border: rgba(203, 213, 225, 0.92);
  color: #0f172a;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.9),
    0 18px 36px rgba(15, 23, 42, 0.14);
}

.availability-page--light .availability-history__tooltip::after {
  border-right-color: var(--availability-history-tooltip-border);
  border-bottom-color: var(--availability-history-tooltip-border);
}

.availability-page--light .availability-history__tooltip-date,
.availability-page--light .availability-history__tooltip-value,
.availability-page--light .availability-history__tooltip-value--latency {
  color: #0f172a;
}

.availability-page--light .availability-history__tooltip-label {
  color: #64748b;
}

.availability-page--light .availability-history__tooltip-value--operational {
  color: #16a34a;
}

.availability-page--light .availability-history__tooltip-value--degraded {
  color: #c0841a;
}

.availability-page--light .availability-history__tooltip-value--failed {
  color: #dc2626;
}

.availability-page--light .availability-history__tooltip-value--disabled,
.availability-page--light .availability-history__tooltip-value--empty {
  color: #64748b;
}

.availability-page--light .availability-history__legend-line,
.availability-page--light .availability-provider-card__footer {
  border-color: rgba(203, 213, 225, 0.76);
}

.availability-page--light .availability-history__legend-line {
  background: rgba(203, 213, 225, 0.78);
}

.availability-page--light .availability-provider-card__hint {
  color: #dc2626;
}

.availability-page--light .availability-secondary-button {
  border-color: rgba(147, 197, 253, 0.72);
  background: #eff6ff;
  color: #2563eb;
}

.availability-page--light .availability-primary-link {
  border-color: rgba(191, 219, 254, 0.9);
  background: #f1f5f9;
  color: #475569;
}

.availability-page--light .availability-tertiary-button {
  border-color: rgba(203, 213, 225, 0.88);
  background: #f8fafc;
  color: #334155;
}

.availability-modal.availability-page--light .availability-tertiary-button {
  border-color: rgba(203, 213, 225, 0.88);
  background: #f8fafc;
  color: #334155;
}

.availability-page--light .availability-secondary-button:hover:not(:disabled),
.availability-page--light .availability-primary-link:hover:not(:disabled),
.availability-page--light .availability-tertiary-button:hover:not(:disabled),
.availability-page--light .availability-icon-button:hover:not(:disabled) {
  border-color: rgba(96, 165, 250, 0.44);
  background: #eff6ff;
  box-shadow: 0 10px 22px rgba(37, 99, 235, 0.12);
}

.availability-page--light .availability-empty-state__icon {
  background: #eff6ff;
  color: #2563eb;
}

.availability-page--light .availability-modal__backdrop {
  background: rgba(15, 23, 42, 0.26);
}

.availability-page--light .availability-modal__panel,
.availability-modal.availability-page--light .availability-modal__panel {
  background: rgba(255, 255, 255, 0.98);
  box-shadow: 0 28px 70px rgba(15, 23, 42, 0.18);
}

.availability-page--light .availability-field span,
.availability-modal.availability-page--light .availability-field span {
  color: #0f172a;
}

.availability-page--light .availability-field__input,
.availability-modal.availability-page--light .availability-field__input {
  border-color: rgba(203, 213, 225, 0.9);
  background: #ffffff;
  color: #0f172a;
}

@keyframes availability-spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

@keyframes availability-breathe {
  0%,
  100% {
    box-shadow: 0 18px 38px rgba(2, 6, 23, 0.24), 0 0 0 0 rgba(248, 113, 113, 0.04);
  }
  50% {
    box-shadow: 0 22px 44px rgba(2, 6, 23, 0.28), 0 0 0 6px rgba(248, 113, 113, 0.04);
  }
}

@media (max-width: 1180px) {
  .availability-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 920px) {
  .availability-page {
    padding: 22px;
  }

  .availability-summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .availability-hero__title-wrap {
    align-items: flex-start;
    flex-direction: column;
    gap: 14px;
  }

  .availability-hero__control-row {
    align-items: stretch;
    flex-direction: column;
  }

  .availability-hero__runtime {
    width: fit-content;
  }

  .availability-hero__actions {
    width: 100%;
    justify-content: flex-start;
  }

  .availability-mode-switch,
  .availability-primary-button {
    width: 100%;
  }

  .availability-runtime-card {
    width: 100%;
    min-width: 0;
  }

  .availability-mode-switch {
    justify-content: space-between;
  }
}

@media (max-width: 720px) {
  .availability-page {
    padding: 18px;
  }

  .availability-summary-grid {
    grid-template-columns: 1fr;
  }

  .availability-hero__title-wrap,
  .availability-provider-card__header,
  .availability-provider-card__footer,
  .availability-modal__header,
  .availability-modal__footer {
    flex-direction: column;
    align-items: stretch;
  }

  .availability-provider-card__actions {
    justify-content: space-between;
  }

  .availability-provider-card__footer-actions {
    width: 100%;
  }

  .availability-secondary-button,
  .availability-primary-link--block {
    width: 100%;
  }

  .availability-history__track {
    gap: 3px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .availability-provider-card,
  .availability-primary-button,
  .availability-secondary-button,
  .availability-tertiary-button,
  .availability-primary-link,
  .availability-icon-button,
  .availability-mode-switch__option,
  .availability-history__segment,
  .availability-toggle,
  .availability-loader,
  .availability-primary-button__icon,
  .availability-secondary-button svg {
    animation: none !important;
    transition: none !important;
  }

  .availability-provider-card:hover,
  .availability-primary-button:hover,
  .availability-secondary-button:hover,
  .availability-tertiary-button:hover,
  .availability-primary-link:hover,
  .availability-icon-button:hover,
  .availability-mode-switch__option:hover,
  .availability-history__segment:hover {
    transform: none !important;
  }
}
</style>
