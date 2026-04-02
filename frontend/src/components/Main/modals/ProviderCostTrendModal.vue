<template>
  <InlineModal
    :open="open"
    :title="modalTitle"
    :panel-width="'min(1040px, 94vw)'"
    :panel-class="modalPanelClass"
    @close="$emit('close')"
  >
    <div
      :class="['provider-cost-modal', isDarkTheme ? 'provider-cost-modal--dark' : 'provider-cost-modal--light']"
      :style="{
        '--provider-cost-accent': providerAccent,
        '--provider-cost-tint': providerTint,
      }"
    >
      <section class="provider-cost-modal__hero">
        <div class="provider-cost-modal__copy">
          <div class="provider-cost-modal__copy-top">
            <span class="provider-cost-modal__eyebrow">{{ platformLabel }}</span>
          </div>
          <div class="provider-cost-modal__copy-body">
            <h3 class="provider-cost-modal__title">{{ providerName }}</h3>
            <p class="provider-cost-modal__subtitle">
              {{ t('components.main.providerCostTrend.summary') }}
            </p>
          </div>
        </div>

        <div class="provider-cost-modal__stats">
          <article class="provider-cost-modal__stat-card provider-cost-modal__stat-card--primary">
            <span class="provider-cost-modal__stat-label">{{ t('components.main.providerCostTrend.todayTotal') }}</span>
            <strong class="provider-cost-modal__stat-value provider-cost-modal__stat-value--primary">
              {{ formatCurrency(totalCost) }}
            </strong>
            <span v-if="latestRecordLabel" class="provider-cost-modal__stat-note">
              {{ latestRecordLabel }}
            </span>
          </article>
          <article class="provider-cost-modal__stat-card">
            <span class="provider-cost-modal__stat-label">{{ t('components.main.providerCostTrend.timeRange') }}</span>
            <strong class="provider-cost-modal__stat-value provider-cost-modal__stat-value--range">
              <span>{{ timeRange.start }}</span>
              <span class="provider-cost-modal__stat-range-separator" aria-hidden="true">→</span>
              <span>{{ timeRange.end }}</span>
            </strong>
          </article>
          <article class="provider-cost-modal__stat-card">
            <span class="provider-cost-modal__stat-label">{{ t('components.main.providerCostTrend.requestCount') }}</span>
            <strong class="provider-cost-modal__stat-value">{{ recordCountLabel }}</strong>
          </article>
        </div>
      </section>

      <div v-if="loading" class="provider-cost-modal__state">
        {{ t('components.main.providerCostTrend.loading') }}
      </div>
      <div v-else-if="error" class="provider-cost-modal__state provider-cost-modal__state--error">
        {{ t('components.main.providerCostTrend.loadFailed', { error }) }}
      </div>
      <div v-else-if="trendPoints.length === 0" class="provider-cost-modal__state provider-cost-modal__state--empty">
        <strong>{{ t('components.main.providerCostTrend.empty') }}</strong>
        <p>{{ t('components.main.providerCostTrend.emptyHint') }}</p>
      </div>
      <div v-else class="provider-cost-modal__content">
        <section class="provider-cost-chart">
          <header class="provider-cost-chart__header">
            <div>
              <h4 class="provider-cost-chart__title">{{ t('components.main.providerCostTrend.chartTitle') }}</h4>
              <p class="provider-cost-chart__hint">{{ t('components.main.providerCostTrend.chartHint') }}</p>
            </div>
            <span class="provider-cost-chart__legend">{{ t('components.main.providerCostTrend.legend') }}</span>
          </header>
          <div ref="chartRef" class="provider-cost-chart__canvas"></div>
        </section>
      </div>
    </div>
  </InlineModal>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AutomationCard } from '../../../data/cards'
import InlineModal from '../../common/InlineModal.vue'
import { fetchRequestLogsPage, type LogPlatform, type RequestLog } from '../../../services/logs'
import type { ResolvedTheme } from '../types'
import { cardProviderRef } from '../adapters/providerCardMappers'
import { ensureEChartsLoaded, type EChartsInstanceLike, type EChartsStaticLike } from '../../../utils/echarts'
import { buildProviderCostTrend, toChartRgba, type ProviderCostTrendPoint } from './providerCostTrend'
import { extractErrorMessage } from '../../../utils/error'

const PAGE_SIZE = 250

const props = defineProps<{
  open: boolean
  provider: AutomationCard | null
  platform: LogPlatform | null
  resolvedTheme: ResolvedTheme
}>()

defineEmits<{
  close: []
}>()

const { t, locale } = useI18n()

const loading = ref(false)
const error = ref('')
const trendPoints = ref<ProviderCostTrendPoint[]>([])
const chartRef = ref<HTMLElement | null>(null)
let chartInstance: EChartsInstanceLike | null = null
let activeLoadRequestId = 0

const isDarkTheme = computed(() => props.resolvedTheme === 'dark')
const providerName = computed(() => props.provider?.name ?? '')
const providerAccent = computed(() => props.provider?.accent ?? '#2563eb')
const providerTint = computed(() => props.provider?.tint ?? 'rgba(37, 99, 235, 0.12)')
const modalPanelClass = computed(() => ({
  'provider-cost-modal-shell': true,
  'provider-cost-modal-shell--dark': isDarkTheme.value,
}))
const modalTitle = computed(() => t('components.main.providerCostTrend.modalTitle', { name: providerName.value }))
const platformLabel = computed(() => {
  switch (props.platform) {
    case 'claude':
      return 'Claude'
    case 'codex':
      return 'Codex'
    case 'gemini':
      return 'Gemini'
    default:
      return 'Provider'
  }
})

const currencyFormatter = computed(() => new Intl.NumberFormat(locale.value || 'en', {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
}))

const compactCurrencyFormatter = computed(() => new Intl.NumberFormat(locale.value || 'en', {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 0,
  maximumFractionDigits: 2,
}))

const countFormatter = computed(() => new Intl.NumberFormat(locale.value || 'en', {
  maximumFractionDigits: 0,
}))

const timeFormatter = computed(() => new Intl.DateTimeFormat(locale.value || 'en', {
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
}))

const dateTimeFormatter = computed(() => new Intl.DateTimeFormat(locale.value || 'en', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
}))

const formatCurrency = (value: number) => currencyFormatter.value.format(Number.isFinite(value) ? value : 0)
const formatAxisCurrency = (value: number) => compactCurrencyFormatter.value.format(Number.isFinite(value) ? value : 0)
const formatCount = (value: number) => countFormatter.value.format(Number.isFinite(value) ? value : 0)

const parseTime = (value: string) => {
  const raw = `${value ?? ''}`.trim()
  if (!raw) return Number.NaN

  const candidates = raw.includes('T')
    ? [raw, raw.replace(/-/g, '/')]
    : [raw.replace(' ', 'T'), raw, raw.replace(/-/g, '/')]

  for (const candidate of candidates) {
    const timestamp = Date.parse(candidate)
    if (Number.isFinite(timestamp)) return timestamp
  }

  return Number.NaN
}

const formatTime = (value: string) => {
  const timestamp = parseTime(value)
  if (!Number.isFinite(timestamp)) return value || '—'
  return timeFormatter.value.format(timestamp)
}

const formatDateTime = (value: string) => {
  const timestamp = parseTime(value)
  if (!Number.isFinite(timestamp)) return value || '—'
  return dateTimeFormatter.value.format(timestamp)
}

const totalCost = computed(() => trendPoints.value[trendPoints.value.length - 1]?.cumulativeCost ?? 0)
const timeRange = computed(() => {
  const first = trendPoints.value[0]?.time
  const last = trendPoints.value[trendPoints.value.length - 1]?.time
  if (!first || !last) {
    return {
      start: '—',
      end: '—',
    }
  }
  return {
    start: formatTime(first),
    end: formatTime(last),
  }
})
const latestRecordLabel = computed(() => {
  const last = trendPoints.value[trendPoints.value.length - 1]?.time
  if (!last) return ''
  return t('components.main.providerCostTrend.updatedAt', { time: formatTime(last) })
})
const recordCountLabel = computed(() => formatCount(trendPoints.value.length))

const buildTodayRange = () => {
  const now = new Date()
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0, 0)
  const end = new Date(start)
  end.setDate(end.getDate() + 1)
  return {
    startAt: formatLocalDateTime(start),
    endAt: formatLocalDateTime(end),
  }
}

const formatLocalDateTime = (value: Date) => {
  const year = value.getFullYear()
  const month = `${value.getMonth() + 1}`.padStart(2, '0')
  const day = `${value.getDate()}`.padStart(2, '0')
  const hours = `${value.getHours()}`.padStart(2, '0')
  const minutes = `${value.getMinutes()}`.padStart(2, '0')
  const seconds = `${value.getSeconds()}`.padStart(2, '0')
  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
}

const nextLoadRequestId = () => {
  activeLoadRequestId += 1
  return activeLoadRequestId
}

const isCurrentLoad = (requestId: number) => requestId === activeLoadRequestId

const invalidatePendingLoad = () => {
  activeLoadRequestId += 1
}

const collectTodayRequestLogs = async (
  platform: LogPlatform,
  providerRef: string,
  requestId: number,
) => {
  const { startAt, endAt } = buildTodayRange()
  const items: RequestLog[] = []
  let offset = 0
  let total = Number.POSITIVE_INFINITY

  while (offset < total) {
    if (!isCurrentLoad(requestId)) return null
    const page = await fetchRequestLogsPage({
      platform,
      provider: providerRef,
      limit: PAGE_SIZE,
      offset,
      startAt,
      endAt,
    })
    if (!isCurrentLoad(requestId)) return null

    const batch = page.items ?? []
    if (Number.isFinite(page.total) && page.total >= 0) {
      total = page.total
    }

    if (!batch.length) {
      break
    }

    items.push(...batch)
    const nextOffset = offset + batch.length
    if (nextOffset <= offset) break
    offset = nextOffset
  }

  return items
}

const disposeChart = () => {
  chartInstance?.dispose()
  chartInstance = null
}

const buildChartOption = (echarts: EChartsStaticLike) => ({
  color: [providerAccent.value || '#2563eb'],
  grid: {
    top: 44,
    right: 22,
    bottom: 76,
    left: 76,
  },
  tooltip: {
    trigger: 'axis',
    backgroundColor: isDarkTheme.value ? 'rgba(2, 6, 23, 0.94)' : 'rgba(15, 23, 42, 0.94)',
    borderWidth: 0,
    padding: 14,
    textStyle: {
      color: '#e2e8f0',
      fontFamily: 'Inter Local, "PingFang SC", "Microsoft YaHei", sans-serif',
    },
    axisPointer: {
      type: 'line',
      lineStyle: {
        color: 'rgba(96, 165, 250, 0.8)',
        width: 1.5,
      },
    },
    formatter(params: Array<{ data: ProviderCostTrendPoint }>) {
      const point = params[0]?.data
      if (!point) return ''
      return `
        <div style="min-width: 220px;">
          <div style="margin-bottom: 10px; color: #94a3b8; font-size: 12px;">${formatDateTime(point.time)}</div>
          <div style="display: flex; justify-content: space-between; gap: 16px; margin-bottom: 6px;">
            <span>${t('components.main.providerCostTrend.todayTotal')}</span>
            <strong style="color: #ffffff;">${formatCurrency(point.cumulativeCost)}</strong>
          </div>
          <div style="display: flex; justify-content: space-between; gap: 16px;">
            <span style="color: #cbd5e1;">${t('components.main.providerCostTrend.singleCost')}</span>
            <span style="color: #bfdbfe;">${formatCurrency(point.cost)}</span>
          </div>
        </div>
      `
    },
  },
  dataZoom: [
    {
      type: 'inside',
      start: 0,
      end: 100,
    },
    {
      type: 'slider',
      start: 0,
      end: 100,
      height: 22,
      bottom: 18,
      borderColor: 'transparent',
      backgroundColor: isDarkTheme.value ? 'rgba(71, 85, 105, 0.18)' : 'rgba(148, 163, 184, 0.14)',
      fillerColor: isDarkTheme.value ? 'rgba(59, 130, 246, 0.24)' : 'rgba(37, 99, 235, 0.16)',
      handleStyle: {
        color: providerAccent.value || '#2563eb',
        borderColor: providerAccent.value || '#1d4ed8',
      },
      moveHandleStyle: {
        color: 'rgba(37, 99, 235, 0.18)',
      },
      textStyle: {
        color: isDarkTheme.value ? '#94a3b8' : '#64748b',
      },
    },
  ],
  xAxis: {
    type: 'category',
    boundaryGap: false,
    data: trendPoints.value.map((item) => item.time),
    axisLine: {
      lineStyle: {
        color: isDarkTheme.value ? 'rgba(100, 116, 139, 0.42)' : 'rgba(148, 163, 184, 0.36)',
      },
    },
    axisLabel: {
      color: isDarkTheme.value ? '#94a3b8' : '#64748b',
      formatter: (value: string) => formatTime(value),
    },
  },
  yAxis: {
    type: 'value',
    name: t('components.main.providerCostTrend.axisName'),
    nameTextStyle: {
      color: isDarkTheme.value ? '#94a3b8' : '#64748b',
      padding: [0, 0, 10, 0],
    },
    axisLabel: {
      color: isDarkTheme.value ? '#94a3b8' : '#64748b',
      formatter: (value: number) => formatAxisCurrency(value),
    },
    splitLine: {
      lineStyle: {
        color: isDarkTheme.value ? 'rgba(51, 65, 85, 0.8)' : 'rgba(148, 163, 184, 0.18)',
        type: 'dashed',
      },
    },
  },
  series: [
    {
      name: t('components.main.providerCostTrend.seriesName'),
      type: 'line',
      smooth: true,
      showSymbol: false,
      symbolSize: 8,
      data: trendPoints.value.map((item) => ({
        ...item,
        value: item.cumulativeCost,
      })),
      lineStyle: {
        width: 3,
        color: providerAccent.value || '#2563eb',
      },
      itemStyle: {
        color: providerAccent.value || '#2563eb',
        borderColor: isDarkTheme.value ? '#0f172a' : '#ffffff',
        borderWidth: 2,
      },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: toChartRgba(providerAccent.value || '#2563eb', 0.32) },
          { offset: 1, color: toChartRgba(providerAccent.value || '#2563eb', 0.04) },
        ]),
      },
      emphasis: {
        focus: 'series',
      },
    },
  ],
})

const renderChart = async (requestId = activeLoadRequestId) => {
  if (!isCurrentLoad(requestId) || !props.open || error.value || trendPoints.value.length === 0 || !chartRef.value) return

  const echarts = await ensureEChartsLoaded()
  if (!isCurrentLoad(requestId)) return
  await nextTick()
  if (!isCurrentLoad(requestId) || !chartRef.value || !props.open) return

  if (!chartInstance) {
    chartInstance = echarts.init(chartRef.value)
  }

  chartInstance.setOption(buildChartOption(echarts), true)
  chartInstance.resize()
}

const safeRenderChart = async (requestId = activeLoadRequestId) => {
  try {
    await renderChart(requestId)
  } catch (chartError) {
    if (!isCurrentLoad(requestId)) return
    disposeChart()
    error.value = extractErrorMessage(chartError, t('components.main.providerCostTrend.loadFailedFallback'))
  }
}

const loadTrend = async () => {
  if (!props.open || !props.provider || !props.platform) return

  const requestId = nextLoadRequestId()
  loading.value = true
  error.value = ''
  trendPoints.value = []
  disposeChart()

  try {
    const providerRef = cardProviderRef(props.provider) || props.provider.name
    const logs = await collectTodayRequestLogs(props.platform, providerRef, requestId)
    if (!logs || !isCurrentLoad(requestId)) return
    trendPoints.value = buildProviderCostTrend(logs)
  } catch (loadError) {
    if (!isCurrentLoad(requestId)) return
    error.value = extractErrorMessage(loadError, t('components.main.providerCostTrend.loadFailedFallback'))
  } finally {
    if (isCurrentLoad(requestId)) {
      loading.value = false
    }
  }

  if (!isCurrentLoad(requestId)) return
  await nextTick()
  if (!error.value && trendPoints.value.length > 0) {
    await safeRenderChart(requestId)
  }
}

watch(
  () => [props.open, props.provider?.id, props.platform] as const,
  async ([open]) => {
    if (!open) {
      invalidatePendingLoad()
      loading.value = false
      error.value = ''
      trendPoints.value = []
      disposeChart()
      return
    }
    await loadTrend()
  },
  { immediate: true },
)

watch(
  () => [trendPoints.value.length, props.resolvedTheme, locale.value] as const,
  async ([length]) => {
    if (!props.open || length === 0 || error.value || loading.value) return
    await safeRenderChart(activeLoadRequestId)
  },
)

watch(
  () => props.open,
  (open) => {
    if (!open) return
    window.requestAnimationFrame(() => {
      chartInstance?.resize()
    })
  },
)

const handleWindowResize = () => {
  if (!props.open) return
  chartInstance?.resize()
}

window.addEventListener('resize', handleWindowResize)

onBeforeUnmount(() => {
  invalidatePendingLoad()
  window.removeEventListener('resize', handleWindowResize)
  disposeChart()
})
</script>

<style scoped>
:global(.provider-cost-modal-shell) {
  border-radius: 28px;
  border: 1px solid rgba(226, 232, 240, 0.88);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.99), rgba(248, 250, 252, 0.97));
  box-shadow:
    0 32px 96px rgba(15, 23, 42, 0.18),
    0 12px 28px rgba(15, 23, 42, 0.08);
}

:global(.provider-cost-modal-shell .modal-header) {
  padding: 22px 24px 16px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.86);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(248, 250, 252, 0.72));
}

:global(.provider-cost-modal-shell .modal-title) {
  color: rgba(15, 23, 42, 0.96);
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.02em;
}

:global(.provider-cost-modal-shell .modal-body) {
  padding: 0 24px 24px;
  background: transparent;
}

:global(.provider-cost-modal-shell .ghost-icon) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.22);
  background: rgba(255, 255, 255, 0.82);
  color: rgba(51, 65, 85, 0.84);
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease,
    transform 0.18s ease;
}

:global(.provider-cost-modal-shell .ghost-icon:hover:not(:disabled)),
:global(.provider-cost-modal-shell .ghost-icon:focus-visible) {
  transform: translateY(-1px);
  border-color: rgba(59, 130, 246, 0.24);
  background: rgba(239, 246, 255, 0.98);
  color: #1d4ed8;
}

:global(.provider-cost-modal-shell--dark) {
  border-color: rgba(148, 163, 184, 0.18);
  background:
    linear-gradient(180deg, rgba(7, 12, 21, 0.99), rgba(11, 18, 31, 0.98));
  box-shadow:
    0 36px 92px rgba(0, 0, 0, 0.58),
    inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

:global(.provider-cost-modal-shell--dark .modal-header) {
  border-bottom-color: rgba(148, 163, 184, 0.14);
  background:
    linear-gradient(180deg, rgba(12, 19, 32, 0.94), rgba(8, 14, 24, 0.78));
}

:global(.provider-cost-modal-shell--dark .modal-title) {
  color: rgba(248, 250, 252, 0.96);
}

:global(.provider-cost-modal-shell--dark .ghost-icon) {
  border-color: rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.1);
  color: rgba(226, 232, 240, 0.82);
}

:global(.provider-cost-modal-shell--dark .ghost-icon:hover:not(:disabled)),
:global(.provider-cost-modal-shell--dark .ghost-icon:focus-visible) {
  border-color: rgba(96, 165, 250, 0.28);
  background: rgba(37, 99, 235, 0.18);
  color: #bfdbfe;
}

.provider-cost-modal {
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-height: 0;
}

.provider-cost-modal__hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(320px, 380px);
  align-items: stretch;
  gap: 18px;
  padding: 24px;
  border-radius: 24px;
  background:
    radial-gradient(circle at top left, var(--provider-cost-tint), transparent 42%),
    linear-gradient(145deg, rgba(255, 255, 255, 0.98), rgba(241, 245, 249, 0.92));
  border: 1px solid rgba(191, 219, 254, 0.24);
  box-shadow: 0 20px 44px rgba(15, 23, 42, 0.08);
}

.provider-cost-modal__copy {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 18px;
  min-width: 0;
}

.provider-cost-modal__copy-top {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.provider-cost-modal__copy-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.provider-cost-modal__eyebrow {
  display: inline-flex;
  align-items: center;
  padding: 7px 12px;
  border-radius: 999px;
  color: var(--provider-cost-accent);
  background: rgba(255, 255, 255, 0.82);
  border: 1px solid rgba(148, 163, 184, 0.2);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.provider-cost-modal__title {
  margin: 0;
  max-width: 18ch;
  font-size: 30px;
  line-height: 1.05;
  letter-spacing: -0.05em;
}

.provider-cost-modal__subtitle {
  margin: 0;
  color: var(--mac-text-secondary);
  font-size: 14px;
  line-height: 1.68;
  max-width: 48ch;
}

.provider-cost-modal__stats {
  display: grid;
  grid-template-columns: minmax(0, 1.08fr) minmax(0, 1fr);
  grid-template-rows: repeat(2, minmax(0, 1fr));
  gap: 12px;
  min-width: 0;
}

.provider-cost-modal__stat-card {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 10px;
  min-height: 100px;
  padding: 16px 18px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.78);
  border: 1px solid rgba(226, 232, 240, 0.9);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.26),
    0 10px 24px rgba(15, 23, 42, 0.06);
}

.provider-cost-modal__stat-card--primary {
  grid-row: 1 / span 2;
  justify-content: space-between;
  padding: 18px 20px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.9), rgba(239, 246, 255, 0.76));
  border-color: rgba(96, 165, 250, 0.22);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.3),
    0 16px 32px rgba(37, 99, 235, 0.1);
}

.provider-cost-modal__stat-label {
  color: var(--mac-text-secondary);
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.provider-cost-modal__stat-value {
  font-family: 'SFMono-Regular', Menlo, Consolas, monospace;
  font-size: 24px;
  font-weight: 700;
  color: var(--mac-text);
  line-height: 1.2;
  letter-spacing: -0.03em;
}

.provider-cost-modal__stat-value--primary {
  font-size: 34px;
}

.provider-cost-modal__stat-value--range {
  display: inline-flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: 8px;
  font-size: 17px;
  white-space: nowrap;
}

.provider-cost-modal__stat-range-separator {
  color: var(--mac-text-secondary);
}

.provider-cost-modal__stat-note {
  color: var(--mac-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.provider-cost-modal__state {
  padding: 24px 20px;
  border-radius: 18px;
  background: rgba(248, 250, 252, 0.9);
  border: 1px solid rgba(226, 232, 240, 0.9);
  color: var(--mac-text-secondary);
  text-align: center;
  line-height: 1.7;
}

.provider-cost-modal__state strong {
  display: block;
  margin-bottom: 6px;
  color: var(--mac-text);
}

.provider-cost-modal__state--error {
  color: #b91c1c;
  background: rgba(254, 242, 242, 0.96);
  border-color: rgba(248, 113, 113, 0.34);
}

.provider-cost-chart {
  padding: 18px 20px 16px;
  border-radius: 22px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(248, 250, 252, 0.92));
  border: 1px solid rgba(226, 232, 240, 0.9);
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.08);
}

.provider-cost-chart__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 14px;
}

.provider-cost-chart__title {
  margin: 0;
  font-size: 17px;
  font-weight: 800;
  letter-spacing: -0.03em;
}

.provider-cost-chart__hint {
  margin: 6px 0 0;
  color: var(--mac-text-secondary);
  font-size: 13px;
  line-height: 1.6;
  max-width: 52ch;
}

.provider-cost-chart__legend {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  padding: 8px 12px;
  border-radius: 999px;
  border: 1px solid rgba(191, 219, 254, 0.46);
  background: rgba(239, 246, 255, 0.78);
  color: var(--mac-text-secondary);
  font-size: 12px;
  font-weight: 700;
}

.provider-cost-chart__legend::before {
  content: '';
  width: 11px;
  height: 11px;
  border-radius: 999px;
  background: var(--provider-cost-accent);
  box-shadow: 0 0 0 4px rgba(37, 99, 235, 0.14);
}

.provider-cost-chart__canvas {
  width: 100%;
  height: 430px;
}

.provider-cost-modal--dark .provider-cost-modal__hero {
  border-color: rgba(71, 85, 105, 0.52);
  background:
    radial-gradient(circle at top left, rgba(59, 130, 246, 0.2), transparent 40%),
    linear-gradient(145deg, rgba(11, 18, 32, 0.94), rgba(15, 23, 42, 0.96));
  box-shadow: 0 22px 52px rgba(2, 6, 23, 0.36);
}

.provider-cost-modal--dark .provider-cost-modal__eyebrow {
  background: rgba(148, 163, 184, 0.12);
  border-color: rgba(148, 163, 184, 0.18);
}

.provider-cost-modal--dark .provider-cost-modal__subtitle,
.provider-cost-modal--dark .provider-cost-modal__stat-label,
.provider-cost-modal--dark .provider-cost-modal__stat-note,
.provider-cost-modal--dark .provider-cost-modal__stat-range-separator,
.provider-cost-modal--dark .provider-cost-chart__hint,
.provider-cost-modal--dark .provider-cost-chart__legend,
.provider-cost-modal--dark .provider-cost-modal__state {
  color: #94a3b8;
}

.provider-cost-modal--dark .provider-cost-modal__title,
.provider-cost-modal--dark .provider-cost-modal__stat-value,
.provider-cost-modal--dark .provider-cost-chart__title,
.provider-cost-modal--dark .provider-cost-modal__state strong {
  color: #e2e8f0;
}

.provider-cost-modal--dark .provider-cost-modal__stat-card,
.provider-cost-modal--dark .provider-cost-chart,
.provider-cost-modal--dark .provider-cost-modal__state {
  background: rgba(15, 23, 42, 0.82);
  border-color: rgba(51, 65, 85, 0.84);
}

.provider-cost-modal--dark .provider-cost-modal__stat-card--primary {
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.9), rgba(30, 41, 59, 0.82));
  border-color: rgba(59, 130, 246, 0.32);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.04),
    0 18px 34px rgba(2, 6, 23, 0.34);
}

.provider-cost-modal--dark .provider-cost-chart__legend {
  border-color: rgba(59, 130, 246, 0.24);
  background: rgba(37, 99, 235, 0.12);
}

.provider-cost-modal--dark .provider-cost-modal__state--error {
  color: #fecaca;
  background: rgba(69, 10, 10, 0.72);
  border-color: rgba(153, 27, 27, 0.9);
}

@media (max-width: 900px) {
  .provider-cost-modal__hero {
    grid-template-columns: 1fr;
  }

  .provider-cost-modal__stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    grid-template-rows: auto;
  }

  .provider-cost-modal__stat-card--primary {
    grid-row: auto;
    grid-column: 1 / -1;
  }
}

@media (max-width: 640px) {
  :global(.provider-cost-modal-shell .modal-header) {
    padding: 18px 18px 14px;
  }

  :global(.provider-cost-modal-shell .modal-body) {
    padding: 0 18px 18px;
  }

  .provider-cost-modal__hero {
    padding: 18px;
    border-radius: 18px;
  }

  .provider-cost-modal__title {
    font-size: 24px;
  }

  .provider-cost-modal__stats {
    grid-template-columns: 1fr;
  }

  .provider-cost-modal__stat-card,
  .provider-cost-modal__stat-card--primary {
    grid-column: auto;
    min-height: 0;
  }

  .provider-cost-modal__stat-value--primary {
    font-size: 30px;
  }

  .provider-cost-modal__stat-value--range {
    font-size: 16px;
  }

  .provider-cost-chart {
    padding: 12px;
  }

  .provider-cost-chart__header {
    flex-direction: column;
  }

  .provider-cost-chart__canvas {
    height: 340px;
  }
}
</style>
