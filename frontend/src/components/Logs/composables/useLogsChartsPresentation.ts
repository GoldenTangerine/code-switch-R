import { computed, type Ref } from 'vue'
import { Chart, type ChartData, type ChartOptions, type ScriptableContext } from 'chart.js'
import type { LogStats, LogStatsSeries, LogSummary, ModelUsageStat } from '../../../services/logs'
import type { LogDateFilterType, LogsSummaryCard, LogsSummaryMicroPoint, ModelShareRow } from '../types'
import { MODEL_SHARE_COLORS } from '../constants'
import {
  buildLineAreaGradient,
  buildLogsTableTextFormatters,
  buildModelShareRows,
  durationColor,
  formatCurrency,
  formatDuration,
  formatFirstTokenMs,
  formatModelShareTooltipLabel,
  formatSeriesLabel,
  formatTime,
  formatTokenNumber,
  formatTokensPerSecond,
  formatNumber,
  hasCacheCreateDetail,
  httpCodeClass,
  parseLogDate,
  resolveChartLegendColor,
  resolveChartTickColor,
  resolveEphemeral1hTokens,
  resolveEphemeral5mTokens,
  resolveModelVerifyStatus,
} from '../utils'

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

type LogsDateRange = {
  startAt: string
  endAt: string
}

type UseLogsChartsPresentationOptions = {
  t: TranslateFn
  isDarkTheme: Ref<boolean>
  summary: Ref<LogSummary | null>
  stats: Ref<LogStats | null>
  modelStats: Ref<ModelUsageStat[]>
  statsSeries: Ref<LogStatsSeries[]>
  summaryScopeHint: Ref<string>
  computeDateRange: () => LogsDateRange | null
  dateType: Ref<LogDateFilterType>
}

const SUCCESS_ALERT_THRESHOLD = 95
const CACHE_PULSE_THRESHOLD = 80
const OUTPUT_DRIFT_THRESHOLD = 55
const COST_GROWTH_ALERT_THRESHOLD = 50

const safeNumber = (value?: number) => {
  const numeric = Number(value ?? 0)
  return Number.isFinite(numeric) ? numeric : 0
}

const clamp = (value: number, min: number, max: number) => Math.min(max, Math.max(min, value))

const clampPercent = (value?: number) => clamp(safeNumber(value), 0, 100)

const formatPercentLabel = (value?: number) => `${clampPercent(value).toFixed(1)}%`
const formatPercentBadgeLabel = (value?: number) => `${Math.round(clampPercent(value))}%`

const formatSignedPercent = (value: number | null) => {
  if (value == null || !Number.isFinite(value)) return '—'
  const normalized = Number(value.toFixed(1))
  if (normalized === 0) return '0%'
  return `${normalized > 0 ? '+' : ''}${normalized.toFixed(1)}%`
}

const formatQps = (value: number) => {
  if (!Number.isFinite(value) || value <= 0) return '0 req/s'
  if (value >= 10) return `${value.toFixed(0)} req/s`
  return `${value.toFixed(1)} req/s`
}

const resolveValueSize = (value: number): LogsSummaryCard['valueSize'] => {
  if (value >= 1_000_000_000) return 'dense'
  if (value >= 1_000_000) return 'compact'
  return 'regular'
}

const resolveCacheHitRateValue = (cacheReadTokens: number, inputTokens: number) => {
  const read = Math.max(0, cacheReadTokens)
  const input = Math.max(0, inputTokens)
  const total = read + input
  if (total <= 0) return 0
  return (read / total) * 100
}

const buildEmptyPoints = (count: number): LogsSummaryMicroPoint[] =>
  Array.from({ length: count }, (_, index) => ({
    label: '',
    value: 0,
    intensity: 0,
    active: index === count - 1,
  }))

const buildActivityPoints = (points: number[] | undefined): LogsSummaryMicroPoint[] => {
  const source = Array.isArray(points) ? points : []
  if (!source.length) return buildEmptyPoints(12)
  const peak = Math.max(...source, 0)
  return source.map((value, index) => ({
    label: `${(index + 1) * 5}s`,
    value,
    intensity: peak > 0 ? value / peak : 0,
    active: index === source.length - 1,
  }))
}

const buildTrendPoints = (
  series: LogStatsSeries[],
  granularity: 'hour' | 'day',
  maxPoints: number,
  valueResolver: (item: LogStatsSeries) => number,
): LogsSummaryMicroPoint[] => {
  const visibleSeries = series.slice(-maxPoints)
  if (!visibleSeries.length) return buildEmptyPoints(maxPoints)
  const values = visibleSeries.map(valueResolver)
  const peak = Math.max(...values, 0)
  return visibleSeries.map((item, index) => ({
    label: formatSeriesLabel(item.day, granularity),
    value: values[index] ?? 0,
    intensity: peak > 0 ? (values[index] ?? 0) / peak : 0,
    active: index === visibleSeries.length - 1,
  }))
}

const resolveSeriesGranularityFromData = (
  range: LogsDateRange | null,
  series: LogStatsSeries[],
  dateType: LogDateFilterType,
) => {
  const start = parseLogDate(range?.startAt)
  const end = parseLogDate(range?.endAt)
  if (start && end) {
    return end.getTime() - start.getTime() > 48 * 60 * 60 * 1000 ? 'day' : 'hour'
  }

  const first = parseLogDate(series[0]?.day)
  const last = parseLogDate(series[series.length - 1]?.day)
  if (first && last) {
    return last.getTime() - first.getTime() > 48 * 60 * 60 * 1000 ? 'day' : 'hour'
  }

  return dateType === 'day' || dateType === 'today' ? 'hour' : 'day'
}

const resolveCostGrowth = (
  currentCost: number | null,
  comparisonCost: number | null,
  comparisonAvailable: boolean,
) => {
  if (currentCost == null || !Number.isFinite(currentCost) || !comparisonAvailable) {
    return null
  }
  const normalizedComparisonCost = comparisonCost == null || !Number.isFinite(comparisonCost) ? 0 : comparisonCost
  if (normalizedComparisonCost > 0) {
    return ((currentCost - normalizedComparisonCost) / normalizedComparisonCost) * 100
  }
  if (normalizedComparisonCost === 0 && currentCost > 0) {
    return 100
  }
  return 0
}

export function useLogsChartsPresentation(options: UseLogsChartsPresentationOptions) {
  const {
    t,
    isDarkTheme,
    summary,
    stats,
    modelStats,
    statsSeries,
    summaryScopeHint,
    computeDateRange,
    dateType,
  } = options

  const modelShareRows = computed<ModelShareRow[]>(() =>
    buildModelShareRows(modelStats.value, MODEL_SHARE_COLORS),
  )

  const modelShareTotalTokens = computed(() =>
    modelShareRows.value.reduce((sum, item) => sum + item.tokens, 0),
  )

  const modelShareChartData = computed<ChartData<'doughnut'>>(() => ({
    labels: modelShareRows.value.map(item => item.model),
    datasets: [
      {
        data: modelShareRows.value.map(item => item.tokens),
        backgroundColor: modelShareRows.value.map(item => item.color),
        borderWidth: 0,
        hoverOffset: 6,
      },
    ],
  }))

  const modelShareChartOptions: ChartOptions<'doughnut'> = {
    responsive: true,
    maintainAspectRatio: false,
    cutout: '50%',
    plugins: {
      legend: {
        display: false,
      },
      tooltip: {
        callbacks: {
          label: (context) => formatModelShareTooltipLabel(context.label, context.raw, modelShareTotalTokens.value),
        },
      },
    },
  }

  const seriesGranularity = computed<'hour' | 'day'>(() =>
    resolveSeriesGranularityFromData(computeDateRange(), statsSeries.value, dateType.value),
  )

  const chartData = computed<ChartData<'line'>>(() => {
    const series = statsSeries.value
    return {
      labels: series.map((item) => formatSeriesLabel(item.day, seriesGranularity.value)),
      datasets: [
        {
          label: t('components.logs.tokenLabels.cost'),
          data: series.map((item) => Number((safeNumber(item.total_cost)).toFixed(4))),
          borderColor: '#f97316',
          backgroundColor: (context: ScriptableContext<'line'>) => buildLineAreaGradient(context.chart as Chart<'line'>, '#f97316', 0.22),
          tension: 0.3,
          fill: 'origin',
          yAxisID: 'yCost',
        },
        {
          label: t('components.logs.tokenLabels.input'),
          data: series.map((item) => safeNumber(item.input_tokens)),
          borderColor: '#34d399',
          backgroundColor: (context: ScriptableContext<'line'>) => buildLineAreaGradient(context.chart as Chart<'line'>, '#34d399', 0.34),
          tension: 0.35,
          fill: 'origin',
        },
        {
          label: t('components.logs.tokenLabels.output'),
          data: series.map((item) => safeNumber(item.output_tokens)),
          borderColor: '#60a5fa',
          backgroundColor: (context: ScriptableContext<'line'>) => buildLineAreaGradient(context.chart as Chart<'line'>, '#60a5fa', 0.3),
          tension: 0.35,
          fill: 'origin',
        },
        {
          label: t('components.logs.tokenLabels.reasoning'),
          data: series.map((item) => safeNumber(item.reasoning_tokens)),
          borderColor: '#f472b6',
          backgroundColor: (context: ScriptableContext<'line'>) => buildLineAreaGradient(context.chart as Chart<'line'>, '#f472b6', 0.3),
          tension: 0.35,
          fill: 'origin',
        },
        {
          label: t('components.logs.tokenLabels.cacheWrite'),
          data: series.map((item) => safeNumber(item.cache_create_tokens)),
          borderColor: '#fbbf24',
          backgroundColor: (context: ScriptableContext<'line'>) => buildLineAreaGradient(context.chart as Chart<'line'>, '#fbbf24', 0.28),
          tension: 0.35,
          fill: 'origin',
        },
        {
          label: t('components.logs.tokenLabels.cacheRead'),
          data: series.map((item) => safeNumber(item.cache_read_tokens)),
          borderColor: '#38bdf8',
          backgroundColor: (context: ScriptableContext<'line'>) => buildLineAreaGradient(context.chart as Chart<'line'>, '#38bdf8', 0.26),
          tension: 0.35,
          fill: 'origin',
        },
      ],
    }
  })

  const chartOptions = computed<ChartOptions<'line'>>(() => ({
    responsive: true,
    maintainAspectRatio: false,
    interaction: {
      mode: 'index',
      intersect: false,
    },
    plugins: {
      legend: {
        position: 'top',
        align: 'start',
        labels: {
          color: resolveChartLegendColor(isDarkTheme.value),
          usePointStyle: true,
          pointStyle: 'circle',
          boxWidth: 8,
          boxHeight: 8,
          padding: 14,
          font: {
            size: 12,
            weight: 500,
          },
          generateLabels: (chart) => {
            const base = Chart.defaults.plugins.legend.labels.generateLabels(chart)
            return base.map((item) => {
              const datasetIndex = typeof item.datasetIndex === 'number' ? item.datasetIndex : -1
              const dataset =
                datasetIndex >= 0
                  ? (chart.data.datasets[datasetIndex] as { borderColor?: unknown } | undefined)
                  : undefined
              const borderColorValue = dataset?.borderColor
              const color = Array.isArray(borderColorValue)
                ? String(borderColorValue[0] ?? '#94a3b8')
                : String(borderColorValue ?? '#94a3b8')
              return {
                ...item,
                fillStyle: color,
                strokeStyle: color,
                lineWidth: 0,
                hidden: datasetIndex >= 0 ? !chart.isDatasetVisible(datasetIndex) : true,
              }
            })
          },
        },
      },
      tooltip: {
        callbacks: {
          labelColor: (context) => {
            const borderColorValue = context.dataset?.borderColor
            const color = Array.isArray(borderColorValue)
              ? String(borderColorValue[0] ?? '#94a3b8')
              : String(borderColorValue ?? '#94a3b8')
            return {
              borderColor: color,
              backgroundColor: color,
              borderWidth: 0,
            }
          },
          label: (context) => {
            const labelPrefix = context.dataset.label ? `${context.dataset.label}: ` : ''
            const rawValue = Number(context.parsed?.y ?? context.raw ?? 0)
            if (context.dataset.yAxisID === 'yCost') {
              if (!Number.isFinite(rawValue)) return `${labelPrefix}$0`
              return `${labelPrefix}${rawValue >= 1 ? `$${rawValue.toFixed(2)}` : `$${rawValue.toFixed(4)}`}`
            }
            return `${labelPrefix}${formatTokenNumber(rawValue)}`
          },
        },
      },
    },
    elements: {
      point: {
        radius: 0,
        hoverRadius: 0,
        hitRadius: 10,
        borderWidth: 0,
      },
    },
    scales: {
      x: {
        grid: { display: false },
        ticks: { color: resolveChartTickColor(isDarkTheme.value) },
      },
      y: {
        beginAtZero: true,
        ticks: {
          color: resolveChartTickColor(isDarkTheme.value),
          callback: (value: string | number) => {
            const numeric = typeof value === 'number' ? value : Number(value)
            if (!Number.isFinite(numeric)) return '0'
            return formatTokenNumber(numeric)
          },
        },
        grid: { color: 'rgba(148, 163, 184, 0.2)' },
      },
      yCost: {
        position: 'right',
        beginAtZero: true,
        grid: { drawOnChartArea: false },
        ticks: {
          color: resolveChartTickColor(isDarkTheme.value),
          callback: (value: string | number) => {
            const numeric = typeof value === 'number' ? value : Number(value)
            if (Number.isNaN(numeric)) return '$0'
            if (numeric >= 1) return `$${numeric.toFixed(2)}`
            return `$${numeric.toFixed(4)}`
          },
        },
      },
    },
  }))

  const statsCards = computed<LogsSummaryCard[]>(() => {
    const summaryData = summary.value
    const scopeHint = summaryScopeHint.value
    const totalRequests = Math.max(0, Math.round(safeNumber(summaryData?.total_requests)))
    const totalTokens = Math.max(0, Math.round(safeNumber(summaryData?.total_tokens)))
    const recentActivityPoints = buildActivityPoints(summaryData?.activity_points)
    const successRate = totalRequests > 0
      ? clamp(safeNumber(summaryData?.success_rate) * 100, 0, 100)
      : null
    const inputTokens = Math.max(0, safeNumber(summaryData?.input_tokens))
    const outputTokens = Math.max(0, safeNumber(summaryData?.output_tokens))
    const peakTokenUsage = Math.max(0, Math.round(safeNumber(summaryData?.peak_tokens)))
    const avgTokenUsage = totalRequests > 0
      ? Math.max(0, Math.round(safeNumber(summaryData?.avg_tokens_per_request)))
      : 0
    const cacheReadTokens = Math.max(0, safeNumber(summaryData?.cache_read_tokens))
    const tokenStructureTotal = inputTokens + cacheReadTokens + outputTokens
    const inputRatio = tokenStructureTotal > 0 ? (inputTokens / tokenStructureTotal) * 100 : 0
    const cacheReadRatio = tokenStructureTotal > 0 ? (cacheReadTokens / tokenStructureTotal) * 100 : 0
    const outputRatio = tokenStructureTotal > 0 ? (outputTokens / tokenStructureTotal) * 100 : 0
    const cacheHitRate = resolveCacheHitRateValue(cacheReadTokens, inputTokens)
    const cacheSavings = Math.max(0, safeNumber(summaryData?.saved_cost_estimate))
    const costTotal = summaryData ? safeNumber(summaryData.cost_total) : null
    const costGrowth = resolveCostGrowth(
      costTotal,
      summaryData ? safeNumber(summaryData.previous_cost_total) : null,
      Boolean(summaryData?.comparison_available),
    )
    const projectedDailyCost = summaryData ? safeNumber(summaryData.projected_daily_cost) : null
    const costTrendPoints = buildTrendPoints(
      statsSeries.value,
      seriesGranularity.value,
      seriesGranularity.value === 'day' ? 7 : 8,
      item => safeNumber(item.total_cost),
    )
    const costGrowthAlert = costGrowth != null && costGrowth > COST_GROWTH_ALERT_THRESHOLD

    return [
      {
        key: 'requests',
        label: t('components.logs.summaryCards.requests.title'),
        subtitle: t('components.logs.summaryCards.requests.subtitle'),
        statusLabel: t('components.logs.summaryCards.requests.status'),
        value: summaryData ? formatNumber(totalRequests) : '—',
        subValue: t('components.logs.summaryCards.requests.totalHint'),
        hint: scopeHint || '',
        tone: 'blue',
        badge: successRate != null && successRate < SUCCESS_ALERT_THRESHOLD
          ? {
              text: t('components.logs.summaryCards.requests.alert'),
              tone: 'alert',
            }
          : undefined,
        miniBars: {
          label: t('components.logs.summaryCards.requests.liveQps'),
          points: recentActivityPoints,
          footerLeft: `${t('components.logs.summaryCards.requests.avgQps')} · ${summaryData ? formatQps(safeNumber(summaryData.activity_avg_qps)) : '—'}`,
          footerRight: `${t('components.logs.summaryCards.requests.peakQps')} · ${summaryData ? formatQps(safeNumber(summaryData.activity_peak_qps)) : '—'}`,
        },
        progress: {
          label: t('components.logs.summaryCards.requests.successRate'),
          value: successRate ?? 0,
          valueLabel: successRate == null ? '—' : formatPercentLabel(successRate),
          tone: successRate != null && successRate < SUCCESS_ALERT_THRESHOLD ? 'alert' : 'primary',
        },
      },
      {
        key: 'tokens',
        label: t('components.logs.summaryCards.tokens.title'),
        subtitle: t('components.logs.summaryCards.tokens.subtitle'),
        statusLabel: t('components.logs.summaryCards.tokens.status'),
        value: summaryData ? formatTokenNumber(totalTokens) : '—',
        subValue: t('components.logs.summaryCards.tokens.totalHint'),
        hint: scopeHint || '',
        tone: 'purple',
        valueSize: resolveValueSize(totalTokens),
        ratio: {
          label: t('components.logs.summaryCards.tokens.ratioLabel'),
          segments: [
            {
              label: t('components.logs.tokenLabels.input'),
              value: inputRatio,
              valueLabel: formatPercentBadgeLabel(inputRatio),
              color: '#818CF8',
            },
            {
              label: t('components.logs.tokenLabels.cacheRead'),
              value: cacheReadRatio,
              valueLabel: formatPercentBadgeLabel(cacheReadRatio),
              color: '#FBBF24',
            },
            {
              label: t('components.logs.tokenLabels.output'),
              value: outputRatio,
              valueLabel: formatPercentBadgeLabel(outputRatio),
              color: outputRatio >= OUTPUT_DRIFT_THRESHOLD ? '#7c3aed' : '#a855f7',
            },
          ],
        },
        metrics: [
          {
            label: t('components.logs.summaryCards.tokens.avgPerRequest'),
            value: summaryData ? formatTokenNumber(avgTokenUsage) : '—',
          },
          {
            label: t('components.logs.summaryCards.tokens.peak'),
            value: summaryData ? formatTokenNumber(peakTokenUsage) : '—',
          },
        ],
      },
      {
        key: 'cacheReads',
        label: t('components.logs.summaryCards.cache.title'),
        subtitle: t('components.logs.summaryCards.cache.subtitle'),
        statusLabel: t('components.logs.summaryCards.cache.status'),
        value: summaryData ? formatTokenNumber(cacheReadTokens) : '—',
        valueSuffix: 'Tokens',
        subValue: t('components.logs.summaryCards.cache.totalHint'),
        hint: !summaryData || totalRequests <= 0
          ? ''
          : cacheHitRate > CACHE_PULSE_THRESHOLD
            ? t('components.logs.summaryCards.cache.pulseStrong')
            : t('components.logs.summaryCards.cache.pulseNormal'),
        tone: 'amber',
        ring: {
          label: t('components.logs.summaryCards.cache.hitRate'),
          value: cacheHitRate,
          valueLabel: summaryData && inputTokens + cacheReadTokens > 0 ? formatPercentLabel(cacheHitRate) : '—',
          pulse: totalRequests > 0 && cacheHitRate > CACHE_PULSE_THRESHOLD,
        },
        metrics: [
          {
            label: t('components.logs.summaryCards.cache.savedCost'),
            value: summaryData ? formatCurrency(cacheSavings) : '—',
            tone: 'success',
            icon: 'spark',
            animated: cacheSavings > 0,
          },
        ],
      },
      {
        key: 'cost',
        label: t('components.logs.summaryCards.cost.title'),
        subtitle: t('components.logs.summaryCards.cost.subtitle'),
        statusLabel: t('components.logs.summaryCards.cost.status'),
        value: costTotal == null ? '—' : formatCurrency(costTotal),
        subValue: t('components.logs.summaryCards.cost.totalHint'),
        hint: scopeHint || '',
        tone: 'green',
        trend: {
          label: seriesGranularity.value === 'day'
            ? t('components.logs.summaryCards.cost.weekTrend')
            : t('components.logs.summaryCards.cost.dayTrend'),
          points: costTrendPoints,
        },
        metrics: [
          {
            label: t('components.logs.summaryCards.cost.compareYesterday'),
            value: formatSignedPercent(costGrowth),
            tone: costGrowth == null ? 'neutral' : costGrowthAlert ? 'warning' : 'success',
            icon: costGrowthAlert ? 'alert' : 'up',
          },
          {
            label: t('components.logs.summaryCards.cost.projectedDaily'),
            value: projectedDailyCost == null ? '—' : formatCurrency(projectedDailyCost),
          },
        ],
      },
    ]
  })

  const logsTableTextFormatters = buildLogsTableTextFormatters(t)

  const logsTableFormatters = {
    formatTime,
    ...logsTableTextFormatters,
    resolveModelVerifyStatus,
    httpCodeClass,
    durationColor,
    formatDuration,
    formatFirstTokenMs,
    formatTokensPerSecond,
    formatCurrency,
    formatTokenNumber,
    hasCacheCreateDetail,
    resolveEphemeral5mTokens,
    resolveEphemeral1hTokens,
  }

  return {
    statsCards,
    modelShareRows,
    modelShareChartData,
    modelShareChartOptions,
    chartData,
    chartOptions,
    logsTableFormatters,
  }
}
