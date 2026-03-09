<template>
  <section
    ref="heatmapContainerRef"
    class="contrib-wall"
    :aria-label="t('components.main.heatmap.ariaLabel')"
  >
    <div class="contrib-legend">
      <span>{{ t('components.main.heatmap.legendLow') }}</span>
      <span v-for="level in 5" :key="level" :class="['legend-box', intensityClass(level - 1)]" />
      <span>{{ t('components.main.heatmap.legendHigh') }}</span>
    </div>

    <div class="contrib-grid">
      <div
        v-for="(week, weekIndex) in usageHeatmap"
        :key="weekIndex"
        class="contrib-column"
      >
        <div
          v-for="(day, dayIndex) in week"
          :key="dayIndex"
          class="contrib-cell"
          :class="[intensityClass(day.intensity), { 'is-current-time': isCurrentTimeCell(day) }]"
          @mouseenter="showUsageTooltip(day, $event)"
          @mousemove="showUsageTooltip(day, $event)"
          @mouseleave="hideUsageTooltip"
        />
      </div>
    </div>

    <div
      v-if="usageTooltip.visible"
      ref="tooltipRef"
      class="contrib-tooltip"
      :class="[usageTooltip.placement, { 'is-positioned': usageTooltip.positioned }]"
      :style="{ left: `${usageTooltip.left}px`, top: `${usageTooltip.top}px` }"
      role="tooltip"
    >
      <p class="tooltip-heading">{{ formattedTooltipLabel }}</p>
      <div class="tooltip-summary-grid">
        <div
          v-for="card in usageTooltipSummaryCards"
          :key="card.key"
          :class="['tooltip-summary-card', `is-${card.tone}`, { 'is-full': card.fullWidth }]"
        >
          <span class="tooltip-summary-label">{{ card.label }}</span>
          <span class="tooltip-summary-value">{{ card.value }}</span>
        </div>
      </div>
      <div class="tooltip-sections">
        <section
          v-for="section in usageTooltipSections"
          :key="section.key"
          class="tooltip-section"
        >
          <div class="tooltip-section-heading" :class="`is-${section.tone}`">
            {{ section.title }}
          </div>
          <div class="tooltip-section-body">
            <div
              v-for="row in section.rows"
              :key="row.key"
              :class="['tooltip-row', `is-${row.tone}`, { 'is-active': row.active }]"
            >
              <span class="tooltip-row-label">{{ row.label }}</span>
              <span
                :class="[
                  'tooltip-row-value',
                  `is-${row.tone}`,
                  { 'is-emphasis': row.emphasis || row.active },
                ]"
              >
                {{ row.value }}
              </span>
            </div>
          </div>
        </section>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, toRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { type UsageHeatmapDay } from '../../../data/usageHeatmap'
import { useAdaptiveHeatmap } from '../../../composables/useAdaptiveHeatmap'
import type { HeatmapDisplaySettings, HeatmapIntensityMetric } from '../../../data/heatmapDisplaySettings'
import type { HeatmapGranularity } from '../../../services/appSettings'
import {
  buildHeatmapCellMatchKey,
  buildHeatmapCurrentCellMatchKey,
  getMillisecondsUntilNextHeatmapBoundary,
} from '../../../utils/heatmapCurrentCell'

const props = defineProps<{
  granularity: HeatmapGranularity
  displaySettings: HeatmapDisplaySettings
}>()

const { t, locale } = useI18n()

const heatmapContainerRef = ref<HTMLElement | null>(null)
const tooltipRef = ref<HTMLElement | null>(null)
const granularityRef = toRef(props, 'granularity')
const displaySettingsRef = toRef(props, 'displaySettings')

const {
  displayData: usageHeatmap,
  init,
  cleanup,
  reload,
} = useAdaptiveHeatmap(heatmapContainerRef, granularityRef, displaySettingsRef)

defineExpose({
  reload,
})

const intensityClass = (value: number) => `gh-level-${value}`
const currentTime = ref(Date.now())
const currentHeatmapCellMatchKey = computed(() =>
  buildHeatmapCurrentCellMatchKey(granularityRef.value, currentTime.value),
)
const heatmapCellMatchKeyCache = new Map<string, string>()

const syncCurrentTime = () => {
  currentTime.value = Date.now()
}

const getHeatmapCellMatchKey = (day: UsageHeatmapDay) => {
  const cacheKey = `${granularityRef.value}:${day.dateKey}`
  const cached = heatmapCellMatchKeyCache.get(cacheKey)
  if (cached !== undefined) {
    return cached
  }
  const nextKey = buildHeatmapCellMatchKey(day.dateKey, granularityRef.value)
  heatmapCellMatchKeyCache.set(cacheKey, nextKey)
  return nextKey
}

const isCurrentTimeCell = (day: UsageHeatmapDay) => {
  if (!currentHeatmapCellMatchKey.value) {
    return false
  }
  return getHeatmapCellMatchKey(day) === currentHeatmapCellMatchKey.value
}

type TooltipPlacement = 'above' | 'below'
type UsageTooltipTone = 'neutral' | 'info' | 'warning' | 'success' | 'violet' | 'rose'
type UsageTooltipSummaryCard = {
  key: string
  label: string
  value: string
  tone: UsageTooltipTone
  fullWidth?: boolean
}
type UsageTooltipRow = {
  key: string
  label: string
  value: string
  tone: UsageTooltipTone
  emphasis?: boolean
  active?: boolean
}
type UsageTooltipSection = {
  key: string
  title: string
  tone: UsageTooltipTone
  rows: UsageTooltipRow[]
}
type UsageTooltipSectionKey = 'activity' | 'tokens' | 'cost'
type UsageTooltipMetricRowDefinition = {
  key: string
  metric: HeatmapIntensityMetric
  section: UsageTooltipSectionKey
  label: string
  emphasis?: boolean
}

const usageTooltip = reactive({
  visible: false,
  positioned: false,
  label: '',
  dateKey: '',
  left: 0,
  top: 0,
  placement: 'above' as TooltipPlacement,
  requests: 0,
  inputTokens: 0,
  outputTokens: 0,
  totalTokens: 0,
  reasoningTokens: 0,
  cost: 0,
  intensity: 0,
  intensityValue: 0,
  intensityPeakValue: 0,
})

const formatMetric = (value: number) => value.toLocaleString()

const formatTokenNumber = (value: number) => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  }
  if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  }
  if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}k`
  }
  return value.toLocaleString()
}

const toPositiveFiniteNumber = (value: unknown) => {
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric) || numeric <= 0) return 0
  return numeric
}

const formatTooltipTokenMetricValue = (value: number) => {
  const normalized = Math.max(0, Math.round(value || 0))
  const compact = formatTokenNumber(normalized)
  if (normalized < 1_000) return compact
  return `${compact} (${normalized.toLocaleString()})`
}

const formatTooltipCostMetricValue = (value: number) => {
  const normalized = Math.max(0, Number(value || 0))
  if (!Number.isFinite(normalized)) return '$0.00'
  const maximumFractionDigits =
    normalized >= 100 ? 2 : normalized >= 1 ? 4 : normalized >= 0.01 ? 4 : 6
  return new Intl.NumberFormat(locale.value || 'en', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits,
  }).format(normalized)
}

const formatTooltipPercentValue = (value: number) => {
  const normalized = Number.isFinite(value) ? Math.min(Math.max(value, 0), 1) : 0
  return new Intl.NumberFormat(locale.value || 'en', {
    style: 'percent',
    minimumFractionDigits: 0,
    maximumFractionDigits:
      normalized * 100 > 0 && normalized * 100 < 10
        ? 1
        : Number.isInteger(normalized * 100)
          ? 0
          : 1,
  }).format(normalized)
}

const tooltipDateFormatter = computed(() =>
  new Intl.DateTimeFormat(locale.value || 'en', {
    month: 'short',
    day: 'numeric',
    ...(granularityRef.value === 'daily'
      ? {}
      : {
          hour: '2-digit',
          minute: '2-digit',
        }),
  }),
)

const formattedTooltipLabel = computed(() => {
  if (!usageTooltip.dateKey) return usageTooltip.label
  const date = new Date(usageTooltip.dateKey)
  if (Number.isNaN(date.getTime())) {
    return usageTooltip.label
  }
  return tooltipDateFormatter.value.format(date)
})

const getHeatmapMetricLabel = (metric: HeatmapIntensityMetric) => {
  switch (metric) {
    case 'cost':
      return t('components.general.heatmapDisplay.intensityMetricCost')
    case 'total_tokens':
      return t('components.general.heatmapDisplay.intensityMetricTotalTokens')
    case 'input_tokens':
      return t('components.general.heatmapDisplay.intensityMetricInputTokens')
    case 'output_tokens':
      return t('components.general.heatmapDisplay.intensityMetricOutputTokens')
    case 'reasoning_tokens':
      return t('components.general.heatmapDisplay.intensityMetricReasoningTokens')
    case 'requests':
    default:
      return t('components.general.heatmapDisplay.intensityMetricRequests')
  }
}

const getUsageTooltipMetricRawValue = (metric: HeatmapIntensityMetric) => {
  switch (metric) {
    case 'cost':
      return usageTooltip.cost
    case 'total_tokens':
      return usageTooltip.totalTokens
    case 'input_tokens':
      return usageTooltip.inputTokens
    case 'output_tokens':
      return usageTooltip.outputTokens
    case 'reasoning_tokens':
      return usageTooltip.reasoningTokens
    case 'requests':
    default:
      return usageTooltip.requests
  }
}

const formatHeatmapMetricValue = (metric: HeatmapIntensityMetric, value: number) => {
  switch (metric) {
    case 'cost':
      return formatTooltipCostMetricValue(value)
    case 'total_tokens':
    case 'input_tokens':
    case 'output_tokens':
    case 'reasoning_tokens':
      return formatTooltipTokenMetricValue(value)
    case 'requests':
    default:
      return formatMetric(Math.max(0, Math.round(value || 0)))
  }
}

const heatmapIntensityMetric = computed<HeatmapIntensityMetric>(() => displaySettingsRef.value.intensityMetric)
const heatmapIntensityMetricLabel = computed(() => getHeatmapMetricLabel(heatmapIntensityMetric.value))
const heatmapIntensityMetricValue = computed(() =>
  formatHeatmapMetricValue(
    heatmapIntensityMetric.value,
    getUsageTooltipMetricRawValue(heatmapIntensityMetric.value),
  ),
)
const heatmapIntensityLevelRatioValue = computed(() => {
  const normalizedLevel = Math.max(0, Math.round(usageTooltip.intensity || 0))
  const peakRatio =
    usageTooltip.intensityPeakValue > 0
      ? usageTooltip.intensityValue / usageTooltip.intensityPeakValue
      : 0
  return t('components.main.heatmap.metrics.brightnessLevelRatioValue', {
    level: normalizedLevel,
    ratio: formatTooltipPercentValue(peakRatio),
  })
})

const HEATMAP_TOOLTIP_SECTION_ORDER: UsageTooltipSectionKey[] = ['activity', 'tokens', 'cost']

const getHeatmapMetricTone = (metric: HeatmapIntensityMetric): UsageTooltipTone => {
  switch (metric) {
    case 'cost':
      return 'success'
    case 'total_tokens':
      return 'warning'
    case 'input_tokens':
      return 'info'
    case 'output_tokens':
      return 'violet'
    case 'reasoning_tokens':
      return 'rose'
    case 'requests':
    default:
      return 'info'
  }
}

const getHeatmapIntensityTone = (intensity: number): UsageTooltipTone => {
  if (intensity >= 4) return 'success'
  if (intensity >= 2) return 'warning'
  if (intensity >= 1) return 'info'
  return 'neutral'
}

const isActiveHeatmapMetric = (metric: HeatmapIntensityMetric) => heatmapIntensityMetric.value === metric

const usageTooltipSectionMeta = computed<Record<UsageTooltipSectionKey, { title: string; tone: UsageTooltipTone }>>(() => ({
  activity: {
    title: t('components.main.heatmap.sections.activity'),
    tone: 'info',
  },
  tokens: {
    title: t('components.main.heatmap.sections.tokens'),
    tone: 'warning',
  },
  cost: {
    title: t('components.main.heatmap.sections.cost'),
    tone: 'success',
  },
}))

const usageTooltipMetricRowDefinitions = computed<UsageTooltipMetricRowDefinition[]>(() => [
  {
    key: 'requests',
    metric: 'requests',
    section: 'activity',
    label: t('components.main.heatmap.metrics.requests'),
    emphasis: true,
  },
  {
    key: 'totalTokens',
    metric: 'total_tokens',
    section: 'tokens',
    label: t('components.main.heatmap.metrics.totalTokens'),
    emphasis: true,
  },
  {
    key: 'inputTokens',
    metric: 'input_tokens',
    section: 'tokens',
    label: t('components.main.heatmap.metrics.inputTokens'),
  },
  {
    key: 'outputTokens',
    metric: 'output_tokens',
    section: 'tokens',
    label: t('components.main.heatmap.metrics.outputTokens'),
  },
  {
    key: 'reasoningTokens',
    metric: 'reasoning_tokens',
    section: 'tokens',
    label: t('components.main.heatmap.metrics.reasoningTokens'),
  },
  {
    key: 'cost',
    metric: 'cost',
    section: 'cost',
    label: t('components.main.heatmap.metrics.cost'),
    emphasis: true,
  },
])

const usageTooltipSummaryCards = computed<UsageTooltipSummaryCard[]>(() => [
  {
    key: 'brightnessMetric',
    label: t('components.main.heatmap.metrics.brightnessMetric'),
    value: heatmapIntensityMetricLabel.value,
    tone: 'neutral',
  },
  {
    key: 'brightnessValue',
    label: t('components.main.heatmap.metrics.brightnessValue'),
    value: heatmapIntensityMetricValue.value,
    tone: getHeatmapMetricTone(heatmapIntensityMetric.value),
  },
  {
    key: 'brightnessLevelRatio',
    label: t('components.main.heatmap.metrics.brightnessLevelRatio'),
    value: heatmapIntensityLevelRatioValue.value,
    tone: getHeatmapIntensityTone(usageTooltip.intensity),
    fullWidth: true,
  },
])

const usageTooltipSections = computed<UsageTooltipSection[]>(() =>
  HEATMAP_TOOLTIP_SECTION_ORDER.map((sectionKey) => {
    const sectionMeta = usageTooltipSectionMeta.value[sectionKey]
    const rows = usageTooltipMetricRowDefinitions.value
      .filter((item) => item.section === sectionKey)
      .map<UsageTooltipRow>((item) => ({
        key: item.key,
        label: item.label,
        value: formatHeatmapMetricValue(item.metric, getUsageTooltipMetricRawValue(item.metric)),
        tone: getHeatmapMetricTone(item.metric),
        emphasis: item.emphasis,
        active: isActiveHeatmapMetric(item.metric),
      }))
    const activeRow = rows.find((row) => row.active)
    return {
      key: sectionKey,
      title: sectionMeta.title,
      tone: activeRow?.tone ?? sectionMeta.tone,
      rows,
    }
  }),
)

const clamp = (value: number, min: number, max: number) => {
  if (max <= min) return min
  return Math.min(Math.max(value, min), max)
}

const TOOLTIP_DEFAULT_WIDTH = 336
const TOOLTIP_DEFAULT_HEIGHT = 384
const TOOLTIP_VERTICAL_OFFSET = 12
const TOOLTIP_HORIZONTAL_MARGIN = 20
const TOOLTIP_VERTICAL_MARGIN = 24

const getTooltipSize = () => {
  const rect = tooltipRef.value?.getBoundingClientRect()
  return {
    width: rect?.width ?? TOOLTIP_DEFAULT_WIDTH,
    height: rect?.height ?? TOOLTIP_DEFAULT_HEIGHT,
  }
}

const viewportSize = () => {
  if (typeof window !== 'undefined') {
    return { width: window.innerWidth, height: window.innerHeight }
  }
  if (typeof document !== 'undefined' && document.documentElement) {
    return {
      width: document.documentElement.clientWidth,
      height: document.documentElement.clientHeight,
    }
  }
  return {
    width: heatmapContainerRef.value?.clientWidth ?? 0,
    height: heatmapContainerRef.value?.clientHeight ?? 0,
  }
}

const applyUsageTooltipMetrics = (day: UsageHeatmapDay) => {
  usageTooltip.label = day.label
  usageTooltip.dateKey = day.dateKey
  usageTooltip.requests = day.requests
  usageTooltip.inputTokens = day.inputTokens
  usageTooltip.outputTokens = day.outputTokens
  usageTooltip.totalTokens = day.totalTokens
  usageTooltip.reasoningTokens = day.reasoningTokens
  usageTooltip.cost = day.cost
  usageTooltip.intensity = day.intensity
  usageTooltip.intensityValue = day.intensityValue
  usageTooltip.intensityPeakValue = day.intensityPeakValue
}

const updateUsageTooltipPosition = (cellRect: DOMRect | ReturnType<HTMLElement['getBoundingClientRect']>) => {
  const { width: tooltipWidth, height: tooltipHeight } = getTooltipSize()
  const { width: viewportWidth, height: viewportHeight } = viewportSize()
  const centerX = cellRect.left + cellRect.width / 2
  const halfWidth = tooltipWidth / 2
  const minLeft = TOOLTIP_HORIZONTAL_MARGIN + halfWidth
  const maxLeft = viewportWidth > 0 ? viewportWidth - halfWidth - TOOLTIP_HORIZONTAL_MARGIN : centerX
  usageTooltip.left = clamp(centerX, minLeft, maxLeft)

  const anchorTop = cellRect.top
  const anchorBottom = cellRect.bottom
  const canShowAbove = anchorTop - tooltipHeight - TOOLTIP_VERTICAL_OFFSET >= TOOLTIP_VERTICAL_MARGIN
  const viewportBottomLimit = viewportHeight > 0 ? viewportHeight - tooltipHeight - TOOLTIP_VERTICAL_MARGIN : anchorBottom
  const shouldPlaceBelow = !canShowAbove
  usageTooltip.placement = shouldPlaceBelow ? 'below' : 'above'
  const desiredTop = shouldPlaceBelow
    ? anchorBottom + TOOLTIP_VERTICAL_OFFSET
    : anchorTop - tooltipHeight - TOOLTIP_VERTICAL_OFFSET
  usageTooltip.top = clamp(desiredTop, TOOLTIP_VERTICAL_MARGIN, viewportBottomLimit)
}

let usageTooltipPositionRequestId = 0
let currentTimeRefreshTimer: number | null = null

const clearCurrentTimeRefreshTimer = () => {
  if (!currentTimeRefreshTimer) return
  clearTimeout(currentTimeRefreshTimer)
  currentTimeRefreshTimer = null
}

// 只在当前粒度的边界刷新，避免页面常驻时高亮停在旧的小时或日期上。
const scheduleCurrentTimeRefresh = () => {
  clearCurrentTimeRefreshTimer()
  if (typeof window === 'undefined') return
  currentTimeRefreshTimer = window.setTimeout(() => {
    syncCurrentTime()
    scheduleCurrentTimeRefresh()
  }, getMillisecondsUntilNextHeatmapBoundary(granularityRef.value, currentTime.value))
}

const finalizeUsageTooltipPosition = async (
  cellRect: DOMRect | ReturnType<HTMLElement['getBoundingClientRect']>,
  positionRequestId: number,
) => {
  await nextTick()
  if (!usageTooltip.visible || positionRequestId !== usageTooltipPositionRequestId) return
  updateUsageTooltipPosition(cellRect)
  usageTooltip.positioned = true
}

const showUsageTooltip = (day: UsageHeatmapDay, event: MouseEvent) => {
  const target = event.currentTarget as HTMLElement | null
  if (!target) return

  const cellRect = target.getBoundingClientRect()
  const isInitialRender = !usageTooltip.visible
  const positionRequestId = ++usageTooltipPositionRequestId
  applyUsageTooltipMetrics(day)

  if (isInitialRender || !usageTooltip.positioned) {
    usageTooltip.visible = true
    usageTooltip.positioned = false
    void finalizeUsageTooltipPosition(cellRect, positionRequestId)
    return
  }

  updateUsageTooltipPosition(cellRect)
  usageTooltip.positioned = true
  void finalizeUsageTooltipPosition(cellRect, positionRequestId)
}

const hideUsageTooltip = () => {
  usageTooltipPositionRequestId += 1
  usageTooltip.positioned = false
  usageTooltip.visible = false
}

onMounted(() => {
  syncCurrentTime()
  scheduleCurrentTimeRefresh()
  void init()
})

watch(granularityRef, () => {
  syncCurrentTime()
  scheduleCurrentTimeRefresh()
})

onUnmounted(() => {
  clearCurrentTimeRefreshTimer()
  cleanup()
})
</script>
