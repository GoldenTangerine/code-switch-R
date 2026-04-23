<template>
  <InlineModal
    :open="open"
    :title="modalTitle"
    :panel-width="'min(1180px, 95vw)'"
    :panel-class="modalPanelClass"
    @close="$emit('close')"
  >
    <div
      :class="['provider-data-modal', isDarkTheme ? 'provider-data-modal--dark' : 'provider-data-modal--light']"
      :style="{
        '--provider-data-accent': providerAccent,
      }"
    >
      <section class="provider-data-modal__hero">
        <div class="provider-data-modal__hero-copy">
          <div class="provider-data-modal__eyebrow-row">
            <span class="provider-data-modal__eyebrow">{{ platformLabel }}</span>
            <span class="provider-data-modal__range-pill">{{ t('components.main.providerDataOverview.rangeLabel', { days: LOOKBACK_DAYS }) }}</span>
          </div>
          <h3 class="provider-data-modal__title">{{ providerName }}</h3>
          <p class="provider-data-modal__subtitle">
            {{ t('components.main.providerDataOverview.summary') }}
          </p>
        </div>
        <span class="provider-data-modal__hero-badge">
          {{ quotaConfiguredLabel }}
        </span>
      </section>

      <div v-if="loading" class="provider-data-modal__state">
        {{ t('components.main.providerDataOverview.loading') }}
      </div>
      <div
        v-else-if="error"
        class="provider-data-modal__state provider-data-modal__state--error"
        role="alert"
      >
        {{ t('components.main.providerDataOverview.loadFailed', { error }) }}
      </div>
      <div v-else class="provider-data-modal__content">
        <section class="provider-data-modal__stats">
          <article class="provider-data-stat provider-data-stat--primary">
            <span class="provider-data-stat__label">{{ t('components.main.providerDataOverview.todayCost') }}</span>
            <strong class="provider-data-stat__value">{{ formatCurrency(todayPoint.cost) }}</strong>
            <span class="provider-data-stat__meta">{{ formatCount(todayPoint.requests) }} {{ t('components.main.providerDataOverview.requestUnit') }}</span>
          </article>
          <article class="provider-data-stat">
            <span class="provider-data-stat__label">{{ t('components.main.providerDataOverview.sevenDayCost') }}</span>
            <strong class="provider-data-stat__value">{{ formatCurrency(sevenDayCost) }}</strong>
            <span class="provider-data-stat__meta">{{ formatCount(totalRequests) }} {{ t('components.main.providerDataOverview.requestUnit') }}</span>
          </article>
          <article class="provider-data-stat">
            <span class="provider-data-stat__label">{{ t('components.main.providerDataOverview.sevenDayTokens') }}</span>
            <strong class="provider-data-stat__value">{{ formatCompactCount(totalTokens) }}</strong>
            <span class="provider-data-stat__meta">{{ t('components.main.providerDataOverview.tokenUsageHint') }}</span>
          </article>
          <article class="provider-data-stat">
            <span class="provider-data-stat__label">{{ t('components.main.providerDataOverview.quotaWindows') }}</span>
            <strong class="provider-data-stat__value">{{ quotaItems.length }}</strong>
            <span class="provider-data-stat__meta">{{ highestQuotaUsageLabel }}</span>
          </article>
        </section>

        <div class="provider-data-modal__grid">
          <section class="provider-data-panel provider-data-panel--wide">
            <header class="provider-data-panel__header">
              <div>
                <h4 class="provider-data-panel__title">{{ t('components.main.providerDataOverview.dailyCostTitle') }}</h4>
                <p class="provider-data-panel__hint">{{ t('components.main.providerDataOverview.dailyCostHint') }}</p>
              </div>
              <span class="provider-data-panel__legend">{{ t('components.main.providerDataOverview.dailyCostLegend') }}</span>
            </header>
            <template v-if="hasTrafficData">
              <div ref="costChartRef" class="provider-data-chart"></div>
              <div v-if="overviewFallbackRows.length > 0" class="provider-data-fallback">
                <div class="provider-data-fallback__header">
                  <div>
                    <strong class="provider-data-fallback__title">{{ t('components.main.providerDataOverview.fallbackTitle') }}</strong>
                    <p class="provider-data-fallback__hint">{{ t('components.main.providerDataOverview.fallbackHint') }}</p>
                  </div>
                </div>
                <div class="provider-data-fallback__table-wrap">
                  <table class="provider-data-fallback__table">
                    <thead>
                      <tr>
                        <th scope="col">{{ t('components.main.providerDataOverview.fallbackDate') }}</th>
                        <th
                          v-for="column in costFallbackColumns"
                          :key="`cost-${column.key}`"
                          scope="col"
                          class="provider-data-fallback__cell--numeric"
                        >
                          {{ column.label }}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr
                        v-for="point in overviewFallbackRows"
                        :key="`cost-${point.dayKey}`"
                        :class="{ 'is-latest': isLatestFallbackRow(point) }"
                      >
                        <th scope="row">
                          <div class="provider-data-fallback__date">
                            <span>{{ formatChartDate(point.timestamp) }}</span>
                            <span
                              v-if="isLatestFallbackRow(point)"
                              class="provider-data-fallback__latest-badge"
                            >
                              {{ t('components.main.providerDataOverview.fallbackLatest') }}
                            </span>
                          </div>
                        </th>
                        <td
                          v-for="column in costFallbackColumns"
                          :key="`cost-${point.dayKey}-${column.key}`"
                          class="provider-data-fallback__cell--numeric"
                          :class="{ 'provider-data-fallback__cell--accent': column.tone === 'accent' }"
                        >
                          {{ formatFallbackCell(point, column.key) }}
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </template>
            <div v-else class="provider-data-panel__empty">
              <strong>{{ t('components.main.providerDataOverview.noChartData') }}</strong>
              <p>{{ t('components.main.providerDataOverview.noChartHint') }}</p>
            </div>
          </section>

          <section class="provider-data-panel provider-data-panel--quota">
            <header class="provider-data-panel__header">
              <div>
                <h4 class="provider-data-panel__title">{{ t('components.main.providerDataOverview.quotaTitle') }}</h4>
                <p class="provider-data-panel__hint">{{ t('components.main.providerDataOverview.quotaHint') }}</p>
              </div>
              <span v-if="quotaItems.length > 0" class="provider-data-panel__legend">{{ t('components.main.providerDataOverview.quotaLegend') }}</span>
            </header>

            <template v-if="quotaItems.length > 0">
              <div ref="quotaChartRef" class="provider-data-chart provider-data-chart--quota"></div>
              <div class="provider-data-quotas">
                <article
                  v-for="item in quotaCards"
                  :key="item.key"
                  :class="[
                    'provider-data-quota-card',
                    `provider-data-quota-card--${item.key}`,
                    item.progressClass,
                    { 'is-over': item.isOver, 'is-inactive': item.isInactive },
                  ]"
                  :title="item.detailTooltip || undefined"
                >
                  <header class="provider-data-quota-card__header">
                    <span class="provider-data-quota-card__badge" :class="`provider-data-quota-card__badge--${item.key}`">
                      {{ item.label }}
                    </span>
                    <span class="provider-data-quota-card__percent">{{ item.percentLabel }}</span>
                  </header>
                  <div class="provider-data-quota-card__values">
                    <div>
                      <span class="provider-data-quota-card__meta-label">{{ t('components.main.providerDataOverview.quotaUsed') }}</span>
                      <strong>{{ formatQuotaValue(item, item.used) }}</strong>
                    </div>
                    <div>
                      <span class="provider-data-quota-card__meta-label">{{ t('components.main.providerDataOverview.quotaLimit') }}</span>
                      <strong>{{ formatQuotaValue(item, item.total) }}</strong>
                    </div>
                  </div>
                  <div class="provider-data-quota-card__progress">
                    <span
                      class="provider-data-quota-card__progress-fill"
                      :class="item.progressClass"
                      :style="{ width: `${item.progressWidth}%` }"
                    ></span>
                  </div>
                  <footer class="provider-data-quota-card__footer">
                    <span>{{ item.remainingLabel }}</span>
                    <span>{{ item.countdownLabel }}</span>
                  </footer>
                  <p v-if="item.detailLabel" class="provider-data-quota-card__detail">
                    {{ item.detailLabel }}
                  </p>
                </article>
              </div>
            </template>
            <div v-else class="provider-data-panel__empty provider-data-panel__empty--quota">
              <strong>{{ t('components.main.providerDataOverview.quotaEmpty') }}</strong>
              <p>{{ t('components.main.providerDataOverview.quotaEmptyHint') }}</p>
            </div>
          </section>

          <section class="provider-data-panel provider-data-panel--wide">
            <header class="provider-data-panel__header">
              <div>
                <h4 class="provider-data-panel__title">{{ t('components.main.providerDataOverview.activityTitle') }}</h4>
                <p class="provider-data-panel__hint">{{ t('components.main.providerDataOverview.activityHint') }}</p>
              </div>
              <span class="provider-data-panel__legend">{{ t('components.main.providerDataOverview.activityLegend') }}</span>
            </header>
            <template v-if="hasTrafficData">
              <div ref="activityChartRef" class="provider-data-chart"></div>
              <div v-if="overviewFallbackRows.length > 0" class="provider-data-fallback">
                <div class="provider-data-fallback__header">
                  <div>
                    <strong class="provider-data-fallback__title">{{ t('components.main.providerDataOverview.fallbackTitle') }}</strong>
                    <p class="provider-data-fallback__hint">{{ t('components.main.providerDataOverview.fallbackHint') }}</p>
                  </div>
                </div>
                <div class="provider-data-fallback__table-wrap">
                  <table class="provider-data-fallback__table">
                    <thead>
                      <tr>
                        <th scope="col">{{ t('components.main.providerDataOverview.fallbackDate') }}</th>
                        <th
                          v-for="column in activityFallbackColumns"
                          :key="`activity-${column.key}`"
                          scope="col"
                          class="provider-data-fallback__cell--numeric"
                        >
                          {{ column.label }}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr
                        v-for="point in overviewFallbackRows"
                        :key="`activity-${point.dayKey}`"
                        :class="{ 'is-latest': isLatestFallbackRow(point) }"
                      >
                        <th scope="row">
                          <div class="provider-data-fallback__date">
                            <span>{{ formatChartDate(point.timestamp) }}</span>
                            <span
                              v-if="isLatestFallbackRow(point)"
                              class="provider-data-fallback__latest-badge"
                            >
                              {{ t('components.main.providerDataOverview.fallbackLatest') }}
                            </span>
                          </div>
                        </th>
                        <td
                          v-for="column in activityFallbackColumns"
                          :key="`activity-${point.dayKey}-${column.key}`"
                          class="provider-data-fallback__cell--numeric"
                          :class="{ 'provider-data-fallback__cell--accent': column.tone === 'accent' }"
                        >
                          {{ formatFallbackCell(point, column.key) }}
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </template>
            <div v-else class="provider-data-panel__empty">
              <strong>{{ t('components.main.providerDataOverview.noChartData') }}</strong>
              <p>{{ t('components.main.providerDataOverview.noChartHint') }}</p>
            </div>
          </section>
        </div>
      </div>
    </div>
  </InlineModal>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AutomationCard } from '../../../data/cards'
import { fetchLogStatsV2, type LogPlatform, type LogStats } from '../../../services/logs'
import { extractErrorMessage } from '../../../utils/error'
import { ensureEChartsLoaded, type EChartsInstanceLike, type EChartsStaticLike } from '../../../utils/echarts'
import { hasProviderQuotaQueryType } from '../../../utils/providerQuotaQuery'
import type { ResolvedTheme } from '../types'
import InlineModal from '../../common/InlineModal.vue'
import { cardProviderRef } from '../adapters/providerCardMappers'
import { formatQuotaUsagePercent, getQuotaProgressClass, getQuotaProgressPercent } from '../utils/providerQuotaDisplay'
import { resolveProviderQuotaQueryDisplay } from '../utils/providerQuotaQueryDisplay'
import { resolveProviderQuotaCurrencyCode } from '../utils/providerQuotaValueFormat'
import {
  formatProviderQuotaCountdownLabel,
  hasProviderQuotaCountdownCrossedReset,
  providerQuotaLabelKeyMap,
  resolveProviderQuotaSnapshot,
  type ProviderQuotaSnapshotItem,
} from '../utils/providerQuotaSnapshot'
import { toChartRgba } from './providerCostTrend'
import {
  buildProviderOverviewFallbackRows,
  buildProviderOverviewDays,
  buildProviderOverviewRange,
  sumLogStatsTokens,
  type ProviderOverviewDayPoint,
} from './providerDataOverview'

const LOOKBACK_DAYS = 7
const QUOTA_COUNTDOWN_TICK_INTERVAL_MS = 1_000

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
const aggregatedStats = ref<LogStats | null>(null)
const dailyPoints = ref<ProviderOverviewDayPoint[]>([])
const quotaItems = ref<ProviderQuotaSnapshotItem[]>([])

const costChartRef = ref<HTMLElement | null>(null)
const activityChartRef = ref<HTMLElement | null>(null)
const quotaChartRef = ref<HTMLElement | null>(null)

let costChartInstance: EChartsInstanceLike | null = null
let activityChartInstance: EChartsInstanceLike | null = null
let quotaChartInstance: EChartsInstanceLike | null = null
let activeLoadRequestId = 0
let countdownTimer: ReturnType<typeof globalThis.setInterval> | undefined
let lastCountdownTickAt: Date | null = null

type ProviderOverviewFallbackColumnKey = 'cost' | 'requests' | 'totalTokens'
type ProviderOverviewFallbackColumn = {
  key: ProviderOverviewFallbackColumnKey
  label: string
  tone?: 'accent'
}

const isDarkTheme = computed(() => props.resolvedTheme === 'dark')
const providerName = computed(() => props.provider?.name ?? '')
const providerAccent = computed(() => props.provider?.accent ?? '#2563eb')
const modalTitle = computed(() => t('components.main.providerDataOverview.modalTitle', { name: providerName.value }))
const modalPanelClass = computed(() => ({
  'provider-data-modal-shell': true,
  'provider-data-modal-shell--dark': isDarkTheme.value,
}))

const platformLabel = computed(() => {
  switch (props.platform) {
    case 'claude':
      return t('components.main.providerLogs.platformClaude')
    case 'codex':
      return t('components.main.providerLogs.platformCodex')
    case 'gemini':
      return t('components.main.providerLogs.platformGemini')
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

const countFormatter = computed(() => new Intl.NumberFormat(locale.value || 'en', {
  maximumFractionDigits: 0,
}))

const quotaCountFormatter = computed(() => new Intl.NumberFormat(locale.value || 'en', {
  maximumFractionDigits: 2,
}))

const compactCountFormatter = computed(() => new Intl.NumberFormat(locale.value || 'en', {
  notation: 'compact',
  maximumFractionDigits: 1,
}))

const dateFormatter = computed(() => new Intl.DateTimeFormat(locale.value || 'en', {
  month: 'short',
  day: 'numeric',
}))

const formatCurrency = (value: number) => currencyFormatter.value.format(Number.isFinite(value) ? value : 0)
const formatCount = (value: number) => countFormatter.value.format(Number.isFinite(value) ? value : 0)
const formatCompactCount = (value: number) => compactCountFormatter.value.format(Number.isFinite(value) ? value : 0)
const formatChartDate = (timestamp: number) => dateFormatter.value.format(timestamp)
const resolveQuotaCurrencyFormatter = (unit?: string) => {
  const currencyCode = resolveProviderQuotaCurrencyCode(unit)
  if (currencyCode) {
    return new Intl.NumberFormat(locale.value || 'en', {
      style: 'currency',
      currency: currencyCode,
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })
  }
  return null
}
const formatQuotaValue = (
  item: Pick<ProviderQuotaSnapshotItem, 'valueMode' | 'unit'>,
  value: number,
) => {
  const normalized = Number.isFinite(value) ? value : 0
  if (item.valueMode === 'count') {
    const formatted = quotaCountFormatter.value.format(normalized)
    return item.unit?.trim() ? `${formatted} ${item.unit.trim()}` : formatted
  }

  const currencyFormatter = resolveQuotaCurrencyFormatter(item.unit)
  if (currencyFormatter) {
    return currencyFormatter.format(normalized)
  }

  const fallbackFormatted = quotaCountFormatter.value.format(normalized)
  return item.unit?.trim() ? `${fallbackFormatted} ${item.unit.trim()}` : fallbackFormatted
}

const hasTrafficData = computed(() => dailyPoints.value.some((point) => point.cost > 0 || point.requests > 0 || point.totalTokens > 0))
const todayPoint = computed(() => dailyPoints.value[dailyPoints.value.length - 1] ?? {
  dayKey: '',
  label: '',
  timestamp: Date.now(),
  cost: 0,
  requests: 0,
  totalTokens: 0,
})
const sevenDayCost = computed(() => aggregatedStats.value?.cost_total ?? 0)
const totalTokens = computed(() => sumLogStatsTokens(aggregatedStats.value))
const totalRequests = computed(() => aggregatedStats.value?.total_requests ?? 0)
const overviewFallbackRows = computed(() => buildProviderOverviewFallbackRows(dailyPoints.value, LOOKBACK_DAYS))
const latestFallbackDayKey = computed(() => overviewFallbackRows.value[0]?.dayKey ?? '')
const quotaConfiguredLabel = computed(() => (
  quotaItems.value.length > 0
    ? t('components.main.providerDataOverview.quotaConfiguredCount', { count: quotaItems.value.length })
    : t('components.main.providerDataOverview.quotaConfiguredEmpty')
))

const costFallbackColumns = computed<ProviderOverviewFallbackColumn[]>(() => [
  {
    key: 'cost',
    label: t('components.main.providerDataOverview.dailyCostSeries'),
    tone: 'accent',
  },
  {
    key: 'requests',
    label: t('components.main.providerDataOverview.requestSeries'),
  },
])

const activityFallbackColumns = computed<ProviderOverviewFallbackColumn[]>(() => [
  {
    key: 'requests',
    label: t('components.main.providerDataOverview.requestSeries'),
    tone: 'accent',
  },
  {
    key: 'totalTokens',
    label: t('components.main.providerDataOverview.tokenSeries'),
  },
])

const highestQuotaUsageLabel = computed(() => {
  if (quotaItems.value.length === 0) {
    return t('components.main.providerDataOverview.noQuotaConfigured')
  }

  const highest = quotaItems.value.reduce((max, item) => Math.max(max, item.progressRatio), 0)
  return t('components.main.providerDataOverview.highestQuotaUsage', {
    percent: formatQuotaUsagePercent({ progressRatio: highest }),
  })
})

const quotaCards = computed(() => quotaItems.value.map((item) => ({
  ...item,
  label: providerQuotaLabelKeyMap[item.key] ? t(providerQuotaLabelKeyMap[item.key]) : item.label,
  isOver: item.progressRatio >= 1,
  isInactive: !item.isActive,
  percentLabel: formatQuotaUsagePercent(item),
  progressWidth: getQuotaProgressPercent(item),
  progressClass: getQuotaProgressClass(item),
  countdownLabel: item.key === 'total'
    ? t('components.main.providerDataOverview.quotaNoResetLabel')
    : item.isActive
      ? item.countdownLabel
      : t('components.main.providers.quotaInactive'),
  remainingLabel: !item.isActive
    ? `${item.invalidMessage ?? ''}`.trim() || t('components.main.providerDataOverview.quotaInactiveLabel')
    : item.remaining >= 0
      ? t('components.main.providerDataOverview.quotaRemaining', { amount: formatQuotaValue(item, item.remaining) })
      : t('components.main.providerDataOverview.quotaExceeded', { amount: formatQuotaValue(item, Math.abs(item.remaining)) }),
  detailLabel: `${item.extra ?? ''}`.trim() || undefined,
  detailTooltip: [`${item.invalidMessage ?? ''}`.trim(), `${item.extra ?? ''}`.trim()].filter(Boolean).join('\n'),
})))

const nextLoadRequestId = () => {
  activeLoadRequestId += 1
  return activeLoadRequestId
}

const isCurrentLoad = (requestId: number) => requestId === activeLoadRequestId

const isLatestFallbackRow = (point: ProviderOverviewDayPoint) => point.dayKey === latestFallbackDayKey.value

const formatFallbackCell = (point: ProviderOverviewDayPoint, key: ProviderOverviewFallbackColumnKey) => {
  if (key === 'cost') return formatCurrency(point.cost)
  if (key === 'totalTokens') return formatCount(point.totalTokens)
  return formatCount(point.requests)
}

const disposeCharts = () => {
  costChartInstance?.dispose()
  costChartInstance = null
  activityChartInstance?.dispose()
  activityChartInstance = null
  quotaChartInstance?.dispose()
  quotaChartInstance = null
}

const resizeCharts = () => {
  if (!props.open) return
  costChartInstance?.resize()
  activityChartInstance?.resize()
  quotaChartInstance?.resize()
}

const buildCostChartOption = (echarts: EChartsStaticLike) => ({
  color: [providerAccent.value || '#2563eb'],
  grid: {
    top: 38,
    right: 20,
    bottom: 50,
    left: 64,
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
    formatter(params: Array<{ data: ProviderOverviewDayPoint }>) {
      const point = params[0]?.data
      if (!point) return ''
      return `
        <div style="min-width: 210px;">
          <div style="margin-bottom: 10px; color: #94a3b8; font-size: 12px;">${formatChartDate(point.timestamp)}</div>
          <div style="display: flex; justify-content: space-between; gap: 16px; margin-bottom: 6px;">
            <span>${t('components.main.providerDataOverview.dailyCostSeries')}</span>
            <strong style="color: #ffffff;">${formatCurrency(point.cost)}</strong>
          </div>
          <div style="display: flex; justify-content: space-between; gap: 16px;">
            <span style="color: #cbd5e1;">${t('components.main.providerDataOverview.requestSeries')}</span>
            <span style="color: #bfdbfe;">${formatCount(point.requests)}</span>
          </div>
        </div>
      `
    },
  },
  xAxis: {
    type: 'category',
    boundaryGap: false,
    data: dailyPoints.value.map((point) => point.label),
    axisLine: {
      lineStyle: {
        color: isDarkTheme.value ? 'rgba(148, 163, 184, 0.32)' : 'rgba(100, 116, 139, 0.22)',
      },
    },
    axisLabel: {
      color: isDarkTheme.value ? '#94a3b8' : '#64748b',
    },
  },
  yAxis: {
    type: 'value',
    axisLabel: {
      color: isDarkTheme.value ? '#94a3b8' : '#64748b',
      formatter: (value: number) => formatCurrency(value),
    },
    splitLine: {
      lineStyle: {
        color: isDarkTheme.value ? 'rgba(51, 65, 85, 0.56)' : 'rgba(226, 232, 240, 0.92)',
      },
    },
  },
  series: [
    {
      name: t('components.main.providerDataOverview.dailyCostSeries'),
      type: 'line',
      smooth: true,
      symbolSize: 8,
      data: dailyPoints.value.map((point) => ({
        ...point,
        value: point.cost,
      })),
      lineStyle: {
        width: 3,
      },
      itemStyle: {
        color: providerAccent.value || '#2563eb',
        borderWidth: 2,
        borderColor: isDarkTheme.value ? '#0f172a' : '#ffffff',
      },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: toChartRgba(providerAccent.value, 0.34) },
          { offset: 1, color: toChartRgba(providerAccent.value, 0.02) },
        ]),
      },
    },
  ],
})

const buildActivityChartOption = (echarts: EChartsStaticLike) => ({
  color: [
    toChartRgba(providerAccent.value, isDarkTheme.value ? 0.74 : 0.56),
    toChartRgba(providerAccent.value, 1),
  ],
  grid: {
    top: 44,
    right: 64,
    bottom: 52,
    left: 58,
  },
  legend: {
    top: 0,
    textStyle: {
      color: isDarkTheme.value ? '#cbd5e1' : '#475569',
    },
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
    formatter(params: Array<{ dataIndex: number }>) {
      const point = dailyPoints.value[params[0]?.dataIndex ?? 0]
      if (!point) return ''
      return `
        <div style="min-width: 220px;">
          <div style="margin-bottom: 10px; color: #94a3b8; font-size: 12px;">${formatChartDate(point.timestamp)}</div>
          <div style="display: flex; justify-content: space-between; gap: 16px; margin-bottom: 6px;">
            <span>${t('components.main.providerDataOverview.requestSeries')}</span>
            <strong style="color: #ffffff;">${formatCount(point.requests)}</strong>
          </div>
          <div style="display: flex; justify-content: space-between; gap: 16px;">
            <span style="color: #cbd5e1;">${t('components.main.providerDataOverview.tokenSeries')}</span>
            <span style="color: #bfdbfe;">${formatCompactCount(point.totalTokens)}</span>
          </div>
        </div>
      `
    },
  },
  xAxis: {
    type: 'category',
    data: dailyPoints.value.map((point) => point.label),
    axisLine: {
      lineStyle: {
        color: isDarkTheme.value ? 'rgba(148, 163, 184, 0.32)' : 'rgba(100, 116, 139, 0.22)',
      },
    },
    axisLabel: {
      color: isDarkTheme.value ? '#94a3b8' : '#64748b',
    },
  },
  yAxis: [
    {
      type: 'value',
      name: t('components.main.providerDataOverview.requestSeries'),
      nameTextStyle: {
        color: isDarkTheme.value ? '#94a3b8' : '#64748b',
      },
      axisLabel: {
        color: isDarkTheme.value ? '#94a3b8' : '#64748b',
      },
      splitLine: {
        lineStyle: {
          color: isDarkTheme.value ? 'rgba(51, 65, 85, 0.56)' : 'rgba(226, 232, 240, 0.92)',
        },
      },
    },
    {
      type: 'value',
      name: t('components.main.providerDataOverview.tokenSeries'),
      nameTextStyle: {
        color: isDarkTheme.value ? '#94a3b8' : '#64748b',
      },
      axisLabel: {
        color: isDarkTheme.value ? '#94a3b8' : '#64748b',
        formatter: (value: number) => formatCompactCount(value),
      },
      splitLine: {
        show: false,
      },
    },
  ],
  series: [
    {
      name: t('components.main.providerDataOverview.requestSeries'),
      type: 'bar',
      barWidth: 18,
      data: dailyPoints.value.map((point) => point.requests),
      itemStyle: {
        borderRadius: [8, 8, 2, 2],
        color: toChartRgba(providerAccent.value, isDarkTheme.value ? 0.72 : 0.54),
      },
    },
    {
      name: t('components.main.providerDataOverview.tokenSeries'),
      type: 'line',
      yAxisIndex: 1,
      smooth: true,
      symbolSize: 7,
      data: dailyPoints.value.map((point) => point.totalTokens),
      lineStyle: {
        width: 2.5,
      },
      itemStyle: {
        color: providerAccent.value || '#2563eb',
      },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: toChartRgba(providerAccent.value, 0.18) },
          { offset: 1, color: toChartRgba(providerAccent.value, 0.01) },
        ]),
      },
    },
  ],
})

const buildQuotaChartOption = () => ({
  grid: {
    top: 6,
    right: 10,
    bottom: 0,
    left: 40,
  },
  tooltip: {
    trigger: 'axis',
    axisPointer: {
      type: 'shadow',
    },
    backgroundColor: isDarkTheme.value ? 'rgba(2, 6, 23, 0.94)' : 'rgba(15, 23, 42, 0.94)',
    borderWidth: 0,
    padding: 12,
    textStyle: {
      color: '#e2e8f0',
      fontFamily: 'Inter Local, "PingFang SC", "Microsoft YaHei", sans-serif',
    },
    formatter(params: Array<{ dataIndex: number }>) {
      const item = quotaCards.value[params[0]?.dataIndex ?? 0]
      if (!item) return ''
      return `
        <div style="min-width: 210px;">
          <div style="margin-bottom: 10px; color: #94a3b8; font-size: 12px;">${item.label}</div>
          <div style="display: flex; justify-content: space-between; gap: 16px; margin-bottom: 6px;">
            <span>${t('components.main.providerDataOverview.quotaUsed')}</span>
            <strong style="color: #ffffff;">${formatQuotaValue(item, item.used)}</strong>
          </div>
          <div style="display: flex; justify-content: space-between; gap: 16px;">
            <span style="color: #cbd5e1;">${t('components.main.providerDataOverview.quotaLimit')}</span>
            <span style="color: #bfdbfe;">${formatQuotaValue(item, item.total)}</span>
          </div>
        </div>
      `
    },
  },
  xAxis: {
    type: 'value',
    max: 100,
    axisLabel: {
      color: isDarkTheme.value ? '#94a3b8' : '#64748b',
      formatter: '{value}%',
    },
    splitLine: {
      lineStyle: {
        color: isDarkTheme.value ? 'rgba(51, 65, 85, 0.52)' : 'rgba(226, 232, 240, 0.9)',
      },
    },
  },
  yAxis: {
    type: 'category',
    data: quotaCards.value.map((item) => item.label),
    axisTick: { show: false },
    axisLine: { show: false },
    axisLabel: {
      color: isDarkTheme.value ? '#cbd5e1' : '#475569',
      fontWeight: 600,
    },
  },
  series: [
    {
      type: 'bar',
      data: quotaCards.value.map((item) => ({
        value: item.progressWidth,
        itemStyle: {
          color: item.isOver
            ? '#ef4444'
            : item.progressWidth >= 80
              ? '#f59e0b'
              : providerAccent.value || '#2563eb',
          borderRadius: [0, 999, 999, 0],
        },
      })),
      barWidth: 10,
      showBackground: true,
      backgroundStyle: {
        color: isDarkTheme.value ? 'rgba(30, 41, 59, 0.85)' : 'rgba(226, 232, 240, 0.85)',
        borderRadius: 999,
      },
      label: {
        show: true,
        position: 'right',
        color: isDarkTheme.value ? '#cbd5e1' : '#475569',
        formatter: ({ dataIndex }: { dataIndex: number }) => quotaCards.value[dataIndex]?.percentLabel ?? '',
      },
    },
  ],
})

const renderCharts = async (requestId: number) => {
  if (!isCurrentLoad(requestId) || !props.open || loading.value || error.value) return

  const echarts = await ensureEChartsLoaded()
  if (!isCurrentLoad(requestId) || !props.open) return

  if (hasTrafficData.value && costChartRef.value) {
    if (!costChartInstance) {
      costChartInstance = echarts.init(costChartRef.value)
    }
    costChartInstance.setOption(buildCostChartOption(echarts), true)
    costChartInstance.resize()
  } else {
    costChartInstance?.dispose()
    costChartInstance = null
  }

  if (hasTrafficData.value && activityChartRef.value) {
    if (!activityChartInstance) {
      activityChartInstance = echarts.init(activityChartRef.value)
    }
    activityChartInstance.setOption(buildActivityChartOption(echarts), true)
    activityChartInstance.resize()
  } else {
    activityChartInstance?.dispose()
    activityChartInstance = null
  }

  if (quotaCards.value.length > 0 && quotaChartRef.value) {
    if (!quotaChartInstance) {
      quotaChartInstance = echarts.init(quotaChartRef.value)
    }
    quotaChartInstance.setOption(buildQuotaChartOption(), true)
    quotaChartInstance.resize()
  } else {
    quotaChartInstance?.dispose()
    quotaChartInstance = null
  }
}

const safeRenderCharts = async (requestId: number) => {
  try {
    await renderCharts(requestId)
  } catch (chartError) {
    if (!isCurrentLoad(requestId)) return
    disposeCharts()
    error.value = extractErrorMessage(chartError, t('components.main.providerDataOverview.loadFailedFallback'))
  }
}

const updateQuotaCountdowns = () => {
  if (!props.open || loading.value || error.value || quotaItems.value.length === 0) return

  const now = new Date()
  const previousTickAt = lastCountdownTickAt ?? new Date(now.getTime() - QUOTA_COUNTDOWN_TICK_INTERVAL_MS)
  let needsReload = false

  quotaItems.value.forEach((item) => {
    if (!item.isActive) return
    item.countdownLabel = formatProviderQuotaCountdownLabel(item.nextReset, now)
    if (hasProviderQuotaCountdownCrossedReset(item.nextReset, previousTickAt, now)) {
      needsReload = true
    }
  })

  lastCountdownTickAt = now

  if (needsReload) {
    void loadOverview()
  }
}

const startCountdownTimer = () => {
  stopCountdownTimer()
  lastCountdownTickAt = new Date()
  countdownTimer = globalThis.setInterval(updateQuotaCountdowns, QUOTA_COUNTDOWN_TICK_INTERVAL_MS)
}

const stopCountdownTimer = () => {
  if (countdownTimer !== undefined) {
    globalThis.clearInterval(countdownTimer)
    countdownTimer = undefined
  }
  lastCountdownTickAt = null
}

const loadOverview = async () => {
  if (!props.open || !props.provider || !props.platform) {
    loading.value = false
    error.value = ''
    aggregatedStats.value = null
    dailyPoints.value = []
    quotaItems.value = []
    disposeCharts()
    return
  }

  const requestId = nextLoadRequestId()
  loading.value = true
  error.value = ''

  try {
    const providerRef = cardProviderRef(props.provider) || props.provider.name
    const range = buildProviderOverviewRange(LOOKBACK_DAYS)
    const now = new Date()
    const quotaPromise = hasProviderQuotaQueryType(
      props.provider.providerQuotaQueryConfig ?? props.provider.providerQuotaQueryType,
      props.provider.providerQuotaQueryType,
    )
      ? resolveProviderQuotaQueryDisplay({
          card: props.provider,
          now,
        t,
      })
      : resolveProviderQuotaSnapshot({
        card: props.provider,
        platform: props.platform,
        now,
        t,
      })

    const [stats, quotas] = await Promise.all([
      fetchLogStatsV2({
        platform: props.platform,
        provider: providerRef,
        startAt: range.startAt,
        endAt: range.endAt,
      }),
      quotaPromise,
    ])

    if (!isCurrentLoad(requestId)) return

    aggregatedStats.value = stats
    dailyPoints.value = buildProviderOverviewDays({
      series: stats.series ?? [],
      startDate: range.start,
      days: LOOKBACK_DAYS,
    })
    quotaItems.value = quotas
    lastCountdownTickAt = now
    loading.value = false

    await nextTick()
    void safeRenderCharts(requestId)
  } catch (loadError) {
    if (!isCurrentLoad(requestId)) return
    loading.value = false
    error.value = extractErrorMessage(loadError, t('components.main.providerDataOverview.loadFailedFallback'))
    disposeCharts()
  }
}

watch(
  () => [props.open, props.provider, props.platform],
  () => {
    void loadOverview()
  },
  { immediate: true },
)

watch(
  () => [props.resolvedTheme, locale.value] as const,
  async () => {
    if (!props.open || loading.value || error.value) return
    await nextTick()
    void safeRenderCharts(activeLoadRequestId)
  },
)

watch(
  () => props.open,
  (open) => {
    if (!open) {
      stopCountdownTimer()
      return
    }
    startCountdownTimer()
    window.requestAnimationFrame(() => {
      resizeCharts()
    })
  },
  { immediate: true },
)

const handleWindowResize = () => {
  resizeCharts()
}

window.addEventListener('resize', handleWindowResize)

onBeforeUnmount(() => {
  activeLoadRequestId += 1
  stopCountdownTimer()
  window.removeEventListener('resize', handleWindowResize)
  disposeCharts()
})
</script>

<style scoped>
.provider-data-modal {
  display: flex;
  flex-direction: column;
  gap: 18px;
  color: #0f172a;
}

.provider-data-modal__hero,
.provider-data-stat,
.provider-data-panel {
  border-radius: 22px;
  border: 1px solid rgba(226, 232, 240, 0.92);
  background:
    linear-gradient(160deg, rgba(255, 255, 255, 0.96), rgba(248, 250, 252, 0.92));
  box-shadow:
    0 14px 32px rgba(15, 23, 42, 0.06),
    inset 0 1px 0 rgba(255, 255, 255, 0.62);
}

.provider-data-modal__hero {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  padding: 22px 24px;
  background:
    radial-gradient(circle at top left, color-mix(in srgb, var(--provider-data-accent) 16%, white) 0%, transparent 46%),
    linear-gradient(160deg, rgba(255, 255, 255, 0.96), rgba(248, 250, 252, 0.92));
}

.provider-data-modal__hero-copy {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.provider-data-modal__eyebrow-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.provider-data-modal__eyebrow,
.provider-data-modal__range-pill,
.provider-data-modal__hero-badge {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 0 12px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.provider-data-modal__eyebrow {
  color: #0f766e;
  background: rgba(204, 251, 241, 0.9);
}

.provider-data-modal__range-pill {
  color: #1d4ed8;
  background: rgba(219, 234, 254, 0.9);
}

.provider-data-modal__hero-badge {
  align-self: flex-start;
  color: #4338ca;
  background: rgba(224, 231, 255, 0.9);
}

.provider-data-modal__title {
  margin: 0;
  font-size: 1.6rem;
  font-weight: 800;
  letter-spacing: -0.03em;
}

.provider-data-modal__subtitle {
  margin: 0;
  max-width: 680px;
  line-height: 1.65;
  color: #475569;
}

.provider-data-modal__state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 220px;
  border-radius: 22px;
  background: rgba(248, 250, 252, 0.88);
  border: 1px solid rgba(226, 232, 240, 0.92);
  color: #475569;
}

.provider-data-modal__state--error {
  color: #b91c1c;
  background: rgba(254, 242, 242, 0.95);
  border-color: rgba(248, 113, 113, 0.36);
}

.provider-data-modal__content {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.provider-data-modal__stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.provider-data-stat {
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.provider-data-stat--primary {
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--provider-data-accent) 14%, white), rgba(255, 255, 255, 0.96));
}

.provider-data-stat__label {
  font-size: 0.76rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #64748b;
}

.provider-data-stat__value {
  font-size: 1.42rem;
  font-weight: 800;
  letter-spacing: -0.03em;
}

.provider-data-stat__meta {
  font-size: 0.82rem;
  color: #475569;
}

.provider-data-modal__grid {
  display: grid;
  grid-template-columns: minmax(0, 1.55fr) minmax(320px, 0.95fr);
  gap: 16px;
}

.provider-data-panel {
  padding: 18px;
  min-width: 0;
}

.provider-data-panel--wide {
  min-height: 330px;
}

.provider-data-panel--quota {
  grid-row: span 2;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.provider-data-panel__header,
.provider-data-quota-card__header,
.provider-data-quota-card__footer,
.provider-data-quota-card__values {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
}

.provider-data-panel__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 800;
  letter-spacing: -0.02em;
}

.provider-data-panel__hint,
.provider-data-panel__legend,
.provider-data-quota-card__meta-label,
.provider-data-quota-card__footer,
.provider-data-quota-card__detail {
  font-size: 0.82rem;
  color: #64748b;
}

.provider-data-panel__legend {
  white-space: nowrap;
}

.provider-data-chart {
  width: 100%;
  min-height: 242px;
}

.provider-data-chart--quota {
  min-height: 168px;
}

.provider-data-fallback {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid rgba(226, 232, 240, 0.82);
}

.provider-data-fallback__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.provider-data-fallback__title {
  display: block;
  font-size: 0.84rem;
  font-weight: 700;
  color: #334155;
}

.provider-data-fallback__hint {
  margin: 4px 0 0;
  font-size: 0.78rem;
  line-height: 1.55;
  color: #64748b;
}

.provider-data-fallback__table-wrap {
  overflow-x: auto;
  border-radius: 16px;
  border: 1px solid rgba(226, 232, 240, 0.92);
  background: rgba(248, 250, 252, 0.72);
}

.provider-data-fallback__table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  min-width: 360px;
}

.provider-data-fallback__table thead th {
  padding: 10px 12px;
  font-size: 0.74rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #64748b;
  background: rgba(241, 245, 249, 0.92);
}

.provider-data-fallback__table thead th:first-child {
  text-align: left;
}

.provider-data-fallback__table tbody th,
.provider-data-fallback__table tbody td {
  padding: 11px 12px;
  border-top: 1px solid rgba(226, 232, 240, 0.88);
  font-size: 0.84rem;
  color: #334155;
}

.provider-data-fallback__table tbody th {
  text-align: left;
  font-weight: 600;
}

.provider-data-fallback__cell--numeric {
  text-align: right;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.provider-data-fallback__cell--accent {
  color: color-mix(in srgb, var(--provider-data-accent) 72%, #0f172a);
  font-weight: 700;
}

.provider-data-fallback__table tbody tr.is-latest {
  background: color-mix(in srgb, var(--provider-data-accent) 6%, rgba(255, 255, 255, 0.96));
}

.provider-data-fallback__date {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.provider-data-fallback__latest-badge {
  display: inline-flex;
  align-items: center;
  min-height: 20px;
  padding: 0 8px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.03em;
  color: color-mix(in srgb, var(--provider-data-accent) 74%, #0f172a);
  background: color-mix(in srgb, var(--provider-data-accent) 14%, rgba(255, 255, 255, 0.94));
}

.provider-data-panel__empty {
  display: flex;
  flex-direction: column;
  justify-content: center;
  min-height: 242px;
  padding: 18px;
  border-radius: 18px;
  background: rgba(241, 245, 249, 0.82);
  border: 1px dashed rgba(148, 163, 184, 0.35);
  color: #475569;
}

.provider-data-panel__empty strong {
  font-size: 0.96rem;
}

.provider-data-panel__empty p {
  margin: 6px 0 0;
  line-height: 1.6;
}

.provider-data-panel__empty--quota {
  min-height: 100%;
}

.provider-data-quotas {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.provider-data-quota-card {
  padding: 14px;
  border-radius: 18px;
  border: 1px solid rgba(226, 232, 240, 0.92);
  background: rgba(248, 250, 252, 0.84);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.provider-data-quota-card.is-over {
  border-color: rgba(248, 113, 113, 0.4);
  background: rgba(254, 242, 242, 0.84);
}

.provider-data-quota-card.is-inactive {
  opacity: 0.72;
}

.provider-data-quota-card__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 42px;
  min-height: 24px;
  padding: 0 10px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.provider-data-quota-card__badge--five_hour {
  color: #7c3aed;
  background: rgba(139, 92, 246, 0.14);
}

.provider-data-quota-card__badge--daily {
  color: #2563eb;
  background: rgba(59, 130, 246, 0.14);
}

.provider-data-quota-card__badge--weekly {
  color: #059669;
  background: rgba(16, 185, 129, 0.14);
}

.provider-data-quota-card__badge--monthly {
  color: #d97706;
  background: rgba(245, 158, 11, 0.16);
}

.provider-data-quota-card__badge--total {
  color: #db2777;
  background: rgba(236, 72, 153, 0.16);
}

.provider-data-quota-card__percent {
  font-size: 0.85rem;
  font-weight: 700;
  color: #334155;
}

.provider-data-quota-card__values strong {
  display: block;
  margin-top: 2px;
  font-size: 0.98rem;
  font-weight: 800;
}

.provider-data-quota-card__progress {
  height: 8px;
  border-radius: 999px;
  background: rgba(226, 232, 240, 0.95);
  overflow: hidden;
}

.provider-data-quota-card__progress-fill {
  display: block;
  height: 100%;
  border-radius: inherit;
  transition:
    width 220ms ease,
    background 220ms ease,
    box-shadow 220ms ease;
}

.provider-data-quota-card__detail {
  margin: 0;
  line-height: 1.45;
}

.provider-data-quota-card {
  --quota-fill-fresh-from: #818cf8;
  --quota-fill-fresh-to: #93c5fd;
  --quota-fill-steady-from: #6366f1;
  --quota-fill-steady-to: #60a5fa;
  --quota-fill-glow: rgba(99, 102, 241, 0.34);
}

.provider-data-quota-card--five_hour {
  --quota-fill-fresh-from: #818cf8;
  --quota-fill-fresh-to: #93c5fd;
  --quota-fill-steady-from: #6366f1;
  --quota-fill-steady-to: #60a5fa;
  --quota-fill-glow: rgba(99, 102, 241, 0.34);
}

.provider-data-quota-card--daily {
  --quota-fill-fresh-from: #60a5fa;
  --quota-fill-fresh-to: #7dd3fc;
  --quota-fill-steady-from: #3b82f6;
  --quota-fill-steady-to: #38bdf8;
  --quota-fill-glow: rgba(56, 189, 248, 0.32);
}

.provider-data-quota-card--weekly {
  --quota-fill-fresh-from: #6ee7b7;
  --quota-fill-fresh-to: #86efac;
  --quota-fill-steady-from: #10b981;
  --quota-fill-steady-to: #4ade80;
  --quota-fill-glow: rgba(16, 185, 129, 0.3);
}

.provider-data-quota-card--monthly {
  --quota-fill-fresh-from: #fbbf24;
  --quota-fill-fresh-to: #fde68a;
  --quota-fill-steady-from: #f59e0b;
  --quota-fill-steady-to: #fbbf24;
  --quota-fill-glow: rgba(245, 158, 11, 0.3);
}

.provider-data-quota-card--total {
  --quota-fill-fresh-from: #f9a8d4;
  --quota-fill-fresh-to: #fbcfe8;
  --quota-fill-steady-from: #ec4899;
  --quota-fill-steady-to: #f472b6;
  --quota-fill-glow: rgba(236, 72, 153, 0.3);
}

.provider-data-quota-card.quota-progress--fresh .provider-data-quota-card__progress-fill {
  background: linear-gradient(90deg, var(--quota-fill-fresh-from) 0%, var(--quota-fill-fresh-to) 100%);
  box-shadow: 0 0 12px color-mix(in srgb, var(--quota-fill-glow) 58%, transparent);
}

.provider-data-quota-card.quota-progress--steady .provider-data-quota-card__progress-fill {
  background: linear-gradient(90deg, var(--quota-fill-steady-from) 0%, var(--quota-fill-steady-to) 100%);
  box-shadow: 0 0 14px color-mix(in srgb, var(--quota-fill-glow) 62%, transparent);
}

.provider-data-quota-card.quota-progress--warm .provider-data-quota-card__progress-fill {
  background: linear-gradient(
    90deg,
    color-mix(in srgb, var(--quota-fill-steady-from) 28%, #f59e0b 72%) 0%,
    #f59e0b 52%,
    #fbbf24 100%
  );
  box-shadow: 0 0 16px rgba(245, 158, 11, 0.28);
}

.provider-data-quota-card.quota-progress--hot .provider-data-quota-card__progress-fill {
  background: linear-gradient(90deg, #f97316 0%, #fb7185 100%);
  box-shadow: 0 0 18px rgba(249, 115, 22, 0.28);
}

.provider-data-quota-card.quota-progress--critical .provider-data-quota-card__progress-fill {
  background: linear-gradient(90deg, #ef4444 0%, #fb7185 100%);
  box-shadow: 0 0 20px rgba(239, 68, 68, 0.32);
}

.provider-data-quota-card.quota-progress--over .provider-data-quota-card__progress-fill {
  background: linear-gradient(90deg, #dc2626 0%, #f43f5e 100%);
  box-shadow: 0 0 22px rgba(244, 63, 94, 0.34);
}

.provider-data-quota-card.quota-progress--fresh .provider-data-quota-card__percent {
  color: color-mix(in srgb, var(--quota-fill-fresh-to) 58%, currentColor 42%);
}

.provider-data-quota-card.quota-progress--steady .provider-data-quota-card__percent {
  color: color-mix(in srgb, var(--quota-fill-steady-to) 62%, currentColor 38%);
}

.provider-data-quota-card.quota-progress--warm .provider-data-quota-card__percent {
  color: #d97706;
}

.provider-data-quota-card.quota-progress--hot .provider-data-quota-card__percent {
  color: #ea580c;
}

.provider-data-quota-card.quota-progress--critical .provider-data-quota-card__percent,
.provider-data-quota-card.quota-progress--over .provider-data-quota-card__percent {
  color: #dc2626;
}

.provider-data-modal--dark {
  color: #e2e8f0;
}

.provider-data-modal--dark .provider-data-modal__hero,
.provider-data-modal--dark .provider-data-stat,
.provider-data-modal--dark .provider-data-panel {
  background:
    linear-gradient(160deg, rgba(15, 23, 42, 0.92), rgba(15, 23, 42, 0.86));
  border-color: rgba(51, 65, 85, 0.88);
  box-shadow:
    0 16px 36px rgba(2, 6, 23, 0.28),
    inset 0 1px 0 rgba(148, 163, 184, 0.06);
}

.provider-data-modal--dark .provider-data-modal__hero {
  background:
    radial-gradient(circle at top left, color-mix(in srgb, var(--provider-data-accent) 24%, transparent) 0%, transparent 46%),
    linear-gradient(160deg, rgba(15, 23, 42, 0.94), rgba(15, 23, 42, 0.88));
}

.provider-data-modal--dark .provider-data-stat--primary {
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--provider-data-accent) 22%, rgba(15, 23, 42, 0.92)), rgba(15, 23, 42, 0.9));
}

.provider-data-modal--dark .provider-data-modal__subtitle,
.provider-data-modal--dark .provider-data-stat__label,
.provider-data-modal--dark .provider-data-stat__meta,
.provider-data-modal--dark .provider-data-panel__hint,
.provider-data-modal--dark .provider-data-panel__legend,
.provider-data-modal--dark .provider-data-quota-card__meta-label,
.provider-data-modal--dark .provider-data-quota-card__footer,
.provider-data-modal--dark .provider-data-fallback__hint,
.provider-data-modal--dark .provider-data-fallback__table thead th {
  color: #94a3b8;
}

.provider-data-modal--dark .provider-data-fallback {
  border-top-color: rgba(51, 65, 85, 0.82);
}

.provider-data-modal--dark .provider-data-fallback__title,
.provider-data-modal--dark .provider-data-fallback__table tbody th,
.provider-data-modal--dark .provider-data-fallback__table tbody td {
  color: #e2e8f0;
}

.provider-data-modal--dark .provider-data-fallback__table-wrap {
  border-color: rgba(51, 65, 85, 0.82);
  background: rgba(2, 6, 23, 0.42);
}

.provider-data-modal--dark .provider-data-fallback__table thead th {
  background: rgba(15, 23, 42, 0.92);
}

.provider-data-modal--dark .provider-data-fallback__table tbody th,
.provider-data-modal--dark .provider-data-fallback__table tbody td {
  border-top-color: rgba(51, 65, 85, 0.76);
}

.provider-data-modal--dark .provider-data-fallback__cell--accent {
  color: color-mix(in srgb, var(--provider-data-accent) 64%, #f8fafc);
}

.provider-data-modal--dark .provider-data-fallback__table tbody tr.is-latest {
  background: color-mix(in srgb, var(--provider-data-accent) 10%, rgba(15, 23, 42, 0.9));
}

.provider-data-modal--dark .provider-data-fallback__latest-badge {
  color: #ffffff;
  background: color-mix(in srgb, var(--provider-data-accent) 24%, rgba(15, 23, 42, 0.9));
}

.provider-data-modal--dark .provider-data-modal__state {
  background: rgba(15, 23, 42, 0.9);
  border-color: rgba(51, 65, 85, 0.88);
  color: #cbd5e1;
}

.provider-data-modal--dark .provider-data-modal__state--error {
  color: #fecaca;
  background: rgba(69, 10, 10, 0.7);
  border-color: rgba(248, 113, 113, 0.34);
}

.provider-data-modal--dark .provider-data-panel__empty {
  background: rgba(15, 23, 42, 0.72);
  border-color: rgba(71, 85, 105, 0.54);
  color: #cbd5e1;
}

.provider-data-modal--dark .provider-data-quota-card {
  background: rgba(15, 23, 42, 0.72);
  border-color: rgba(51, 65, 85, 0.76);
}

.provider-data-modal--dark .provider-data-quota-card.is-over {
  background: rgba(69, 10, 10, 0.55);
  border-color: rgba(248, 113, 113, 0.34);
}

.provider-data-modal--dark .provider-data-quota-card__percent {
  color: #e2e8f0;
}

.provider-data-modal--dark .provider-data-quota-card__progress {
  background: rgba(30, 41, 59, 0.92);
}

:global(.provider-data-modal-shell .ghost-icon) {
  border: none;
  background: rgba(15, 23, 42, 0.08);
  color: #0f172a;
  width: 34px;
  height: 34px;
  border-radius: 12px;
  transition:
    background 0.2s ease,
    color 0.2s ease,
    transform 0.2s ease;
}

:global(.provider-data-modal-shell .ghost-icon:hover:not(:disabled)),
:global(.provider-data-modal-shell .ghost-icon:focus-visible) {
  background: color-mix(in srgb, var(--provider-data-accent) 12%, rgba(255, 255, 255, 0.94));
  color: color-mix(in srgb, var(--provider-data-accent) 74%, #0f172a);
  transform: translateY(-1px);
}

:global(.provider-data-modal-shell--dark .ghost-icon) {
  background: rgba(255, 255, 255, 0.08);
  color: #e2e8f0;
}

:global(.provider-data-modal-shell--dark .ghost-icon:hover:not(:disabled)),
:global(.provider-data-modal-shell--dark .ghost-icon:focus-visible) {
  background: color-mix(in srgb, var(--provider-data-accent) 20%, rgba(15, 23, 42, 0.92));
  color: #ffffff;
}

@media (max-width: 1100px) {
  .provider-data-modal__stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .provider-data-modal__grid {
    grid-template-columns: 1fr;
  }

  .provider-data-panel--quota {
    grid-row: auto;
  }
}

@media (max-width: 720px) {
  .provider-data-modal__hero {
    flex-direction: column;
    padding: 18px;
  }

  .provider-data-modal__hero-badge {
    align-self: flex-start;
  }

  .provider-data-modal__stats {
    grid-template-columns: 1fr;
  }

  .provider-data-panel,
  .provider-data-stat {
    padding: 16px;
  }

  .provider-data-quota-card__values {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
