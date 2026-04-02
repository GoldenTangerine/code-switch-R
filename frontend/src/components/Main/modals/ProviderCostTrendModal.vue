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
          <span class="provider-cost-modal__eyebrow">{{ platformLabel }}</span>
          <h3 class="provider-cost-modal__title">{{ providerName }}</h3>
          <p class="provider-cost-modal__subtitle">
            {{ t('components.main.providerCostTrend.summary') }}
          </p>
        </div>

        <div class="provider-cost-modal__stats">
          <article class="provider-cost-modal__stat-card">
            <span class="provider-cost-modal__stat-label">{{ t('components.main.providerCostTrend.todayTotal') }}</span>
            <strong class="provider-cost-modal__stat-value">{{ formatCurrency(totalCost) }}</strong>
          </article>
          <article class="provider-cost-modal__stat-card">
            <span class="provider-cost-modal__stat-label">{{ t('components.main.providerCostTrend.timeRange') }}</span>
            <strong class="provider-cost-modal__stat-value provider-cost-modal__stat-value--small">
              {{ timeRangeLabel }}
            </strong>
          </article>
          <article class="provider-cost-modal__stat-card">
            <span class="provider-cost-modal__stat-label">{{ t('components.main.providerCostTrend.requestCount') }}</span>
            <strong class="provider-cost-modal__stat-value">{{ trendPoints.length }}</strong>
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
const timeRangeLabel = computed(() => {
  const first = trendPoints.value[0]?.time
  const last = trendPoints.value[trendPoints.value.length - 1]?.time
  if (!first || !last) return '—'
  return `${formatTime(first)} - ${formatTime(last)}`
})

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
.provider-cost-modal {
  display: flex;
  flex-direction: column;
  gap: 18px;
  min-height: 0;
}

.provider-cost-modal__hero {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(320px, 0.85fr);
  gap: 16px;
  padding: 22px;
  border-radius: 22px;
  background:
    radial-gradient(circle at top right, rgba(37, 99, 235, 0.16), transparent 34%),
    linear-gradient(145deg, var(--provider-cost-tint), rgba(255, 255, 255, 0.92));
  border: 1px solid rgba(148, 163, 184, 0.18);
}

.provider-cost-modal__eyebrow {
  display: inline-flex;
  align-items: center;
  padding: 7px 11px;
  border-radius: 999px;
  color: var(--provider-cost-accent);
  background: rgba(255, 255, 255, 0.74);
  border: 1px solid rgba(148, 163, 184, 0.18);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.provider-cost-modal__title {
  margin: 14px 0 10px;
  font-size: 28px;
  line-height: 1.05;
  letter-spacing: -0.04em;
}

.provider-cost-modal__subtitle {
  margin: 0;
  color: var(--mac-text-secondary);
  font-size: 14px;
  line-height: 1.7;
}

.provider-cost-modal__stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.provider-cost-modal__stat-card {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 10px;
  min-height: 112px;
  padding: 16px 18px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.82);
  border: 1px solid rgba(226, 232, 240, 0.88);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.26);
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
}

.provider-cost-modal__stat-value--small {
  font-size: 19px;
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
  padding: 16px;
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.94);
  border: 1px solid rgba(226, 232, 240, 0.9);
}

.provider-cost-chart__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 12px;
}

.provider-cost-chart__title {
  margin: 0;
  font-size: 18px;
  font-weight: 800;
  letter-spacing: -0.03em;
}

.provider-cost-chart__hint {
  margin: 8px 0 0;
  color: var(--mac-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.provider-cost-chart__legend {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  color: var(--mac-text-secondary);
  font-size: 13px;
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
  border-color: rgba(71, 85, 105, 0.56);
  background:
    radial-gradient(circle at top right, rgba(59, 130, 246, 0.22), transparent 34%),
    linear-gradient(145deg, rgba(15, 23, 42, 0.92), rgba(30, 41, 59, 0.94));
}

.provider-cost-modal--dark .provider-cost-modal__subtitle,
.provider-cost-modal--dark .provider-cost-modal__stat-label,
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
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .provider-cost-modal__hero {
    padding: 16px;
    border-radius: 18px;
  }

  .provider-cost-modal__title {
    font-size: 24px;
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
