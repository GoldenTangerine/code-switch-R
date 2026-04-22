<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  getLatestResults,
  runAllChecks,
  runSingleCheck,
  setAvailabilityMonitorEnabled,
  isPollingRunning,
  saveAvailabilityConfig,
  type ProviderTimeline,
  HealthStatus,
} from '../../services/healthcheck'

type StatusTone = 'operational' | 'degraded' | 'failed' | 'disabled'
type HistoryTone = StatusTone | 'empty'

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

function formatLatency(value?: number | null) {
  if (!Number.isFinite(value) || Number(value) <= 0) {
    return '--'
  }
  return `${Math.round(Number(value))} ms`
}

function formatUptime(timeline: ProviderTimeline) {
  if (!timeline.availabilityMonitorEnabled || (timeline.items?.length ?? 0) === 0) {
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
  if (!timeline.availabilityMonitorEnabled) {
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
  if (!timeline.availabilityMonitorEnabled) {
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
  if (!timeline.availabilityMonitorEnabled) {
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
  const recentItems = timeline.availabilityMonitorEnabled
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
    checkedAtDateLabel: formatHistoryDate(item.checkedAt),
    checkedAtLabel: formatShortDateTime(item.checkedAt),
    checkedAtFullLabel: formatFullDateTime(item.checkedAt),
    latencyLabel: formatLatency(item.latencyMs),
    isPlaceholder: false,
  }))

  const placeholderCount = Math.max(HISTORY_SEGMENT_LIMIT - segments.length, 0)
  const placeholderLabel = timeline.availabilityMonitorEnabled ? t('availability.history.noSample') : t('availability.notMonitored')
  const placeholderTime = timeline.availabilityMonitorEnabled ? t('availability.history.noData') : t('availability.notMonitored')

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
  try {
    timelines.value = await getLatestResults()

    try {
      pollingRunning.value = await isPollingRunning()
    } catch (pollingError) {
      console.error('Failed to load polling status:', pollingError)
    }

    lastUpdated.value = new Date()
    nextRefreshIn.value = REFRESH_INTERVAL_SECONDS
  } catch (error) {
    console.error('Failed to load availability data:', error)
  } finally {
    loading.value = false
  }
}

async function refreshAll() {
  if (refreshing.value) return

  refreshing.value = true
  try {
    await runAllChecks()
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
  window.addEventListener(PROVIDERS_UPDATED_EVENT, handleProvidersUpdated)
  window.addEventListener('keydown', handleEscape)
  await loadData()
  startRefreshTimer()
})

onUnmounted(() => {
  window.removeEventListener(PROVIDERS_UPDATED_EVENT, handleProvidersUpdated)
  window.removeEventListener('keydown', handleEscape)
  stopTimers()
})
</script>

<template>
  <div class="availability-page">
    <div class="availability-page__grid" aria-hidden="true"></div>
    <div class="availability-page__glow availability-page__glow--primary" aria-hidden="true"></div>
    <div class="availability-page__glow availability-page__glow--secondary" aria-hidden="true"></div>

    <div class="availability-page__shell">
      <header class="availability-hero">
        <div class="availability-hero__copy">
          <div class="availability-hero__title-wrap">
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

          <span class="availability-hero__runtime" :class="{ 'availability-hero__runtime--active': pollingRunning }">
            <span class="availability-hero__runtime-dot"></span>
            {{ pollingRunning ? t('availability.runtime.pollingRunning') : t('availability.runtime.pollingStopped') }}
          </span>
        </div>

        <div class="availability-hero__actions">
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
            <span>{{ refreshing ? t('availability.refreshing') : t('availability.refreshAll') }}</span>
          </button>
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
                    <span v-if="!card.availabilityMonitorEnabled" class="availability-provider-card__disabled-chip">
                      {{ t('availability.stats.disabled') }}
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
                    {{ card.availabilityMonitorEnabled ? card.lastCheckedLabel : t('availability.history.noData') }}
                  </p>
                </div>
              </div>

              <div class="availability-provider-card__actions">
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
                <span class="availability-history-card__label">{{ t('availability.history.title') }}</span>
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
                <span>{{ t('availability.history.start') }}</span>
                <span class="availability-history__legend-line"></span>
                <span>{{ t('availability.history.end') }}</span>
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
                {{ card.availabilityMonitorEnabled ? t('availability.card.autoRefresh', { seconds: REFRESH_INTERVAL_SECONDS }) : t('availability.enableToMonitor') }}
              </span>

              <div class="availability-provider-card__footer-actions">
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

    <div v-if="showConfigModal" class="availability-modal" role="dialog" aria-modal="true">
      <div class="availability-modal__backdrop" @click="closeConfigModal"></div>
      <div class="availability-modal__panel">
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
  gap: 14px;
}

.availability-hero__title-wrap {
  display: flex;
  align-items: center;
  gap: 14px;
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

.availability-hero__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: stretch;
  justify-content: flex-end;
  gap: 12px;
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
  gap: 0;
  min-width: 272px;
  overflow: hidden;
  border-radius: 18px;
  box-shadow: 0 16px 32px rgba(2, 6, 23, 0.2);
}

.availability-runtime-card__item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 14px 16px;
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
.availability-icon-button {
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
.availability-icon-button:hover:not(:disabled),
.availability-provider-card:hover {
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
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 320px;
  padding: 20px;
  border-radius: 24px;
  background: rgba(22, 27, 34, 0.82);
  box-shadow: 0 18px 38px rgba(2, 6, 23, 0.24);
}

.availability-provider-card--operational {
  border-color: rgba(74, 222, 128, 0.14);
}

.availability-provider-card--degraded {
  border-color: rgba(251, 191, 36, 0.16);
}

.availability-provider-card--failed {
  border-color: rgba(248, 113, 113, 0.22);
  animation: availability-breathe 2.6s ease-in-out infinite;
}

.availability-provider-card--disabled {
  border-color: rgba(148, 163, 184, 0.1);
  opacity: 0.76;
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
  z-index: 4;
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

  .availability-hero__actions {
    width: 100%;
    justify-content: stretch;
  }

  .availability-runtime-card,
  .availability-primary-button {
    width: 100%;
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
  .availability-history__segment:hover {
    transform: none !important;
  }
}
</style>
