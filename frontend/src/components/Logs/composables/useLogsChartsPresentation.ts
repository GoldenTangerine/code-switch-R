import { computed, type Ref } from 'vue'
import { Chart, type ChartData, type ChartOptions, type ScriptableContext } from 'chart.js'
import type { LogStats, LogStatsSeries, ModelUsageStat } from '../../../services/logs'
import type { LogDateFilterType, LogsSummaryCard, ModelShareRow } from '../types'
import { MODEL_SHARE_COLORS } from '../constants'
import {
  buildLineAreaGradient,
  buildLogsTableTextFormatters,
  buildModelShareRows,
  durationColor,
  formatCacheHitRate,
  formatCurrency,
  formatDuration,
  formatFirstTokenMs,
  formatTime,
  formatModelShareTooltipLabel,
  formatNumber,
  formatSeriesLabel,
  formatTokenNumber,
  formatTokensPerSecond,
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
  stats: Ref<LogStats | null>
  modelStats: Ref<ModelUsageStat[]>
  statsSeries: Ref<LogStatsSeries[]>
  summaryScopeHint: Ref<string>
  computeDateRange: () => LogsDateRange | null
  dateType: Ref<LogDateFilterType>
}

export function useLogsChartsPresentation(options: UseLogsChartsPresentationOptions) {
  const {
    t,
    isDarkTheme,
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

  const seriesGranularity = computed<'hour' | 'day'>(() => {
    const range = computeDateRange()
    if (!range || (!range.startAt && !range.endAt)) {
      return 'hour'
    }
    const start = parseLogDate(range.startAt)
    const end = parseLogDate(range.endAt)
    if (!start || !end) {
      return dateType.value === 'day' ? 'hour' : 'day'
    }
    const duration = end.getTime() - start.getTime()
    return duration > 48 * 60 * 60 * 1000 ? 'day' : 'hour'
  })

  const chartData = computed<ChartData<'line'>>(() => {
    const series = statsSeries.value
    return {
      labels: series.map((item) => formatSeriesLabel(item.day, seriesGranularity.value)),
      datasets: [
        {
          label: t('components.logs.tokenLabels.cost'),
          data: series.map((item) => Number(((item.total_cost ?? 0)).toFixed(4))),
          borderColor: '#f97316',
          backgroundColor: (context: ScriptableContext<'line'>) => buildLineAreaGradient(context.chart as Chart<'line'>, '#f97316', 0.22),
          tension: 0.3,
          fill: 'origin',
          yAxisID: 'yCost',
        },
        {
          label: t('components.logs.tokenLabels.input'),
          data: series.map((item) => item.input_tokens ?? 0),
          borderColor: '#34d399',
          backgroundColor: (context: ScriptableContext<'line'>) => buildLineAreaGradient(context.chart as Chart<'line'>, '#34d399', 0.34),
          tension: 0.35,
          fill: 'origin',
        },
        {
          label: t('components.logs.tokenLabels.output'),
          data: series.map((item) => item.output_tokens ?? 0),
          borderColor: '#60a5fa',
          backgroundColor: (context: ScriptableContext<'line'>) => buildLineAreaGradient(context.chart as Chart<'line'>, '#60a5fa', 0.3),
          tension: 0.35,
          fill: 'origin',
        },
        {
          label: t('components.logs.tokenLabels.reasoning'),
          data: series.map((item) => item.reasoning_tokens ?? 0),
          borderColor: '#f472b6',
          backgroundColor: (context: ScriptableContext<'line'>) => buildLineAreaGradient(context.chart as Chart<'line'>, '#f472b6', 0.3),
          tension: 0.35,
          fill: 'origin',
        },
        {
          label: t('components.logs.tokenLabels.cacheWrite'),
          data: series.map((item) => item.cache_create_tokens ?? 0),
          borderColor: '#fbbf24',
          backgroundColor: (context: ScriptableContext<'line'>) => buildLineAreaGradient(context.chart as Chart<'line'>, '#fbbf24', 0.28),
          tension: 0.35,
          fill: 'origin',
        },
        {
          label: t('components.logs.tokenLabels.cacheRead'),
          data: series.map((item) => item.cache_read_tokens ?? 0),
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
    const data = stats.value
    const scopeHint = summaryScopeHint.value
    const totalTokens =
      (data?.input_tokens ?? 0) + (data?.output_tokens ?? 0) + (data?.cache_read_tokens ?? 0)
    return [
      {
        key: 'requests',
        label: t('components.logs.summary.total'),
        hint: t('components.logs.summary.requests'),
        value: data ? formatNumber(data.total_requests) : '—',
      },
      {
        key: 'tokens',
        label: t('components.logs.summary.tokens'),
        hint: t('components.logs.summary.tokenHint'),
        value: data ? formatTokenNumber(totalTokens) : '—',
      },
      {
        key: 'cacheReads',
        label: t('components.logs.summary.cache'),
        hint: t('components.logs.summary.cacheHint'),
        value: data ? formatTokenNumber(data.cache_read_tokens) : '—',
        subValue: data ? formatCacheHitRate(data.cache_read_tokens, data.input_tokens) : '',
      },
      {
        key: 'cost',
        label: t('components.logs.tokenLabels.cost'),
        hint: scopeHint,
        value: formatCurrency(data?.cost_total ?? 0),
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
