<template>
  <div :class="['logs-charts-panel', isDarkTheme ? 'is-dark' : 'is-light']">
    <section class="logs-model-share">
      <header class="logs-model-share__header">
        <h3 class="logs-model-share__title">{{ t('components.logs.modelShare.title') }}</h3>
      </header>

      <div v-if="modelShareRows.length" class="logs-model-share__body">
        <div class="logs-model-share__chart-panel">
          <div class="logs-model-share__chart-shell">
            <div class="logs-model-share__chart-wrap">
              <Doughnut
                ref="doughnutRef"
                :data="interactiveModelShareChartData"
                :options="interactiveModelShareChartOptions"
                :plugins="doughnutPlugins"
              />
              <div class="logs-model-share__chart-center">
                <span class="logs-model-share__chart-label">{{ totalCallsLabel }}</span>
                <strong class="logs-model-share__chart-value">{{ formatNumber(totalRequests) }}</strong>
                <span class="logs-model-share__chart-hint">{{ centerHint }}</span>
              </div>
            </div>
          </div>
        </div>

        <div class="logs-model-share__table">
          <div class="logs-model-share__table-head">
            <span class="logs-model-share__table-title">{{ t('components.logs.modelShare.detailTitle') }}</span>
            <span class="logs-model-share__table-metric">{{ t('components.logs.modelShare.requests') }}</span>
            <span class="logs-model-share__table-metric">{{ t('components.logs.modelShare.tokens') }}</span>
            <span class="logs-model-share__table-metric">{{ t('components.logs.modelShare.cost') }}</span>
          </div>
          <div class="logs-model-share__table-body">
            <article
              v-for="(item, index) in modelShareRows"
              :key="item.model"
              class="logs-model-share__table-row"
              :class="{ 'is-active': hoveredIndex === index }"
              @mouseenter="setHoveredIndex(index)"
              @mouseleave="setHoveredIndex(null)"
            >
              <div class="logs-model-share__model">
                <div
                  class="logs-model-share__dot"
                  :style="{
                    backgroundColor: item.color,
                    boxShadow: hoveredIndex === index ? `0 0 14px ${item.color}` : `0 0 0 1px ${hexToRgba(item.color, 0.18)}`,
                  }"
                ></div>
                <div class="logs-model-share__model-copy">
                  <span class="logs-model-share__model-name" :title="item.model">{{ item.model }}</span>
                  <div class="logs-model-share__track">
                    <span
                      class="logs-model-share__track-fill"
                      :style="{
                        width: resolveBarWidth(item.requests),
                        backgroundColor: item.color,
                        boxShadow: hoveredIndex === index ? `0 0 18px ${hexToRgba(item.color, 0.4)}` : 'none',
                        animationDelay: `${Math.min(index * 70, 420)}ms`,
                      }"
                    ></span>
                  </div>
                </div>
              </div>

              <span class="logs-model-share__metric">
                {{ formatNumber(item.requests) }}
              </span>

              <span class="logs-model-share__metric logs-model-share__metric--token">
                <svg viewBox="0 0 24 24" aria-hidden="true" class="logs-model-share__token-icon">
                  <path
                    d="M13 2L5 14h5l-1 8 8-12h-5l1-8z"
                    fill="currentColor"
                  />
                </svg>
                {{ formatTokenNumber(item.tokens) }}
              </span>

              <span class="logs-model-share__cost">
                {{ formatCurrency(item.cost) }}
              </span>
            </article>
          </div>
        </div>
      </div>

      <div v-else class="logs-model-share__empty">
        {{ t('components.logs.modelShare.empty') }}
      </div>
    </section>

    <section class="logs-chart">
      <Line :data="chartData" :options="chartOptions" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Chart as ChartJS, ChartData, ChartOptions, Plugin } from 'chart.js'
import { Doughnut, Line } from 'vue-chartjs'
import type { ModelShareRow } from '../types'
import { formatModelShareTooltipLabel } from '../utils'

const props = defineProps<{
  modelShareRows: ModelShareRow[]
  isDarkTheme: boolean
  chartData: ChartData<'line'>
  chartOptions: ChartOptions<'line'>
  formatNumber: (value?: number) => string
  formatTokenNumber: (value?: number) => string
  formatCurrency: (value?: number) => string
}>()

const { t } = useI18n()

const doughnutRef = ref<{ chart: ChartJS<'doughnut'> | null } | null>(null)
const hoveredIndex = ref<number | null>(null)

const hexToRgba = (hexColor: string, alpha: number) => {
  const normalized = String(hexColor ?? '').trim().replace('#', '')
  if (normalized.length !== 6) {
    return `rgba(129, 140, 248, ${alpha})`
  }
  const red = Number.parseInt(normalized.slice(0, 2), 16)
  const green = Number.parseInt(normalized.slice(2, 4), 16)
  const blue = Number.parseInt(normalized.slice(4, 6), 16)
  return `rgba(${red}, ${green}, ${blue}, ${alpha})`
}

const doughnutActiveGlowPlugin: Plugin<'doughnut'> = {
  id: 'logs-model-share-active-glow',
  afterDatasetsDraw(chart) {
    const activeArc = chart.getActiveElements()[0]
    if (!activeArc) return

    const dataset = chart.data.datasets[activeArc.datasetIndex]
    const colors = dataset?.backgroundColor
    const glowColor = Array.isArray(colors)
      ? String(colors[activeArc.index] ?? '#818cf8')
      : String(colors ?? '#818cf8')
    const element = chart.getDatasetMeta(activeArc.datasetIndex).data[activeArc.index]
    if (!element) return

    const { x, y, innerRadius, outerRadius, startAngle, endAngle } = element.getProps(
      ['x', 'y', 'innerRadius', 'outerRadius', 'startAngle', 'endAngle'],
      true,
    )

    const context = chart.ctx
    const expandedInnerRadius = Math.max(innerRadius - (props.isDarkTheme ? 4 : 3), 0)
    const expandedOuterRadius = outerRadius + (props.isDarkTheme ? 6 : 5)

    context.save()
    context.beginPath()
    context.arc(x, y, expandedOuterRadius, startAngle, endAngle)
    context.arc(x, y, expandedInnerRadius, endAngle, startAngle, true)
    context.closePath()
    context.fillStyle = glowColor
    context.shadowColor = glowColor
    context.shadowBlur = props.isDarkTheme ? 26 : 16
    context.globalAlpha = props.isDarkTheme ? 0.96 : 0.88
    context.fill()
    context.restore()

    context.save()
    context.beginPath()
    context.arc(x, y, expandedOuterRadius, startAngle, endAngle)
    context.arc(x, y, expandedInnerRadius, endAngle, startAngle, true)
    context.closePath()
    context.lineJoin = 'round'
    context.lineWidth = 2
    context.strokeStyle = doughnutHoverBorderColor.value
    context.stroke()
    context.restore()
  },
}

const doughnutPlugins = [doughnutActiveGlowPlugin]

const totalRequests = computed(() =>
  props.modelShareRows.reduce((sum, item) => sum + item.requests, 0),
)

const peakRequests = computed(() =>
  props.modelShareRows.reduce((max, item) => Math.max(max, item.requests), 0),
)

const activeRow = computed(() =>
  hoveredIndex.value == null ? null : props.modelShareRows[hoveredIndex.value] ?? null,
)

const totalCallsLabel = computed(() => t('components.logs.modelShare.totalCalls'))
const requestUnitLabel = computed(() => t('components.logs.modelShare.requestsUnit'))
const doughnutBorderColor = computed(() => (props.isDarkTheme ? '#090b0f' : '#f8fafc'))
const doughnutHoverBorderColor = computed(() => (props.isDarkTheme ? '#111318' : '#ffffff'))

const centerHint = computed(() => {
  if (!activeRow.value || totalRequests.value <= 0) {
    return t('components.logs.modelShare.centerHint')
  }
  const share = (activeRow.value.requests / totalRequests.value) * 100
  return `${activeRow.value.model} · ${share.toFixed(1)}%`
})

const interactiveModelShareChartData = computed<ChartData<'doughnut'>>(() => ({
  labels: props.modelShareRows.map(item => item.model),
  datasets: [
    {
      data: props.modelShareRows.map(item => item.requests),
      backgroundColor: props.modelShareRows.map((item, index) =>
        hoveredIndex.value == null || hoveredIndex.value === index
          ? item.color
          : hexToRgba(item.color, props.isDarkTheme ? 0.28 : 0.42),
      ),
      borderColor: props.modelShareRows.map(() => doughnutBorderColor.value),
      borderWidth: 3,
      hoverBorderColor: props.modelShareRows.map(() => doughnutHoverBorderColor.value),
      hoverBorderWidth: 3,
      spacing: 2,
      hoverOffset: 0,
    },
  ],
}))

const interactiveModelShareChartOptions = computed<ChartOptions<'doughnut'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  cutout: '68%',
  radius: '88%',
  animation: {
    duration: 720,
    easing: 'easeOutQuart',
  },
  interaction: {
    mode: 'nearest',
    intersect: true,
  },
  plugins: {
    legend: {
      display: false,
    },
    tooltip: {
      callbacks: {
        label: (context) =>
          formatModelShareTooltipLabel(
            context.label,
            props.modelShareRows[context.dataIndex]?.requests ?? context.raw,
            totalRequests.value,
            requestUnitLabel.value,
          ),
      },
      displayColors: false,
      backgroundColor: props.isDarkTheme ? '#111318' : '#ffffff',
      borderColor: props.isDarkTheme ? 'rgba(255, 255, 255, 0.08)' : 'rgba(148, 163, 184, 0.22)',
      borderWidth: 1,
      titleColor: props.isDarkTheme ? '#f5f7fb' : '#0f172a',
      bodyColor: props.isDarkTheme ? '#d4d7dd' : '#334155',
      bodyFont: {
        size: 11,
      },
      padding: 10,
      cornerRadius: 10,
    },
  },
  onHover: (_event, activeElements, chart) => {
    chart.canvas.style.cursor = activeElements.length ? 'pointer' : 'default'
    hoveredIndex.value = activeElements[0]?.index ?? null
  },
}))

const setHoveredIndex = (index: number | null) => {
  hoveredIndex.value = index
}

const resolveBarWidth = (requests: number) => {
  if (peakRequests.value <= 0) return '0%'
  return `${Math.max((requests / peakRequests.value) * 100, 4)}%`
}

const syncChartActiveState = () => {
  const chart = doughnutRef.value?.chart
  if (!chart) return

  const activeElements = hoveredIndex.value == null
    ? []
    : [{ datasetIndex: 0, index: hoveredIndex.value }]

  chart.setActiveElements(activeElements)

  if (chart.tooltip) {
    if (!activeElements.length) {
      chart.tooltip.setActiveElements([], { x: 0, y: 0 })
    } else {
      const activeElement = chart.getDatasetMeta(0).data[hoveredIndex.value ?? 0]
      if (activeElement) {
        const { x, y } = activeElement.getProps(['x', 'y'], true)
        chart.tooltip.setActiveElements(activeElements, { x, y })
      }
    }
  }

  chart.update('none')
}

watch(
  [hoveredIndex, () => props.modelShareRows.length, () => props.isDarkTheme],
  async () => {
    if (hoveredIndex.value != null && hoveredIndex.value >= props.modelShareRows.length) {
      hoveredIndex.value = null
      return
    }
    await nextTick()
    syncChartActiveState()
  },
  { flush: 'post', immediate: true },
)

watch(
  () => props.modelShareRows.map(item => `${item.model}:${item.requests}`).join('|'),
  async () => {
    await nextTick()
    syncChartActiveState()
  },
  { flush: 'post' },
)
</script>

<style scoped>
.logs-charts-panel {
  --logs-card-border: rgba(148, 163, 184, 0.26);
  --logs-model-share-bg:
    radial-gradient(circle at top left, rgba(99, 102, 241, 0.14), transparent 30%),
    radial-gradient(circle at 84% 18%, rgba(251, 146, 60, 0.1), transparent 24%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.97), rgba(244, 247, 255, 0.98));
  --logs-card-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.88),
    0 20px 48px rgba(15, 23, 42, 0.08);
  --logs-grid-overlay: rgba(15, 23, 42, 0.02);
  --logs-bottom-line: rgba(99, 102, 241, 0.24);
  --logs-divider: rgba(148, 163, 184, 0.28);
  --logs-chart-panel-bg: linear-gradient(180deg, rgba(246, 248, 253, 0.92), rgba(239, 243, 250, 0.68));
  --logs-chart-shell-bg:
    radial-gradient(circle, rgba(255, 255, 255, 0.85), transparent 62%),
    radial-gradient(circle at center, rgba(99, 102, 241, 0.1), transparent 72%);
  --logs-chart-shell-border: rgba(99, 102, 241, 0.14);
  --logs-title: #0f172a;
  --logs-chart-label: #64748b;
  --logs-chart-value: #0f172a;
  --logs-chart-hint: #64748b;
  --logs-table-head: #64748b;
  --logs-table-title: #334155;
  --logs-table-divider: rgba(148, 163, 184, 0.22);
  --logs-row-text: #64748b;
  --logs-row-hover-bg: rgba(15, 23, 42, 0.04);
  --logs-row-hover-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.55);
  --logs-model-name: #334155;
  --logs-model-name-active: #0f172a;
  --logs-track-bg: rgba(148, 163, 184, 0.22);
  --logs-metric: #475569;
  --logs-token-icon: #94a3b8;
  --logs-cost: #111827;
  --logs-empty: #64748b;
  --logs-scrollbar: rgba(148, 163, 184, 0.72);
  --logs-line-chart-bg:
    radial-gradient(circle at top right, rgba(96, 165, 250, 0.1), transparent 30%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(245, 247, 252, 0.98));
  --logs-line-chart-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.88),
    0 18px 42px rgba(15, 23, 42, 0.08);
  --logs-line-chart-overlay: linear-gradient(180deg, rgba(255, 255, 255, 0.4), transparent 18%);
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.logs-charts-panel.is-dark {
  --logs-card-border: rgba(31, 33, 40, 0.92);
  --logs-model-share-bg:
    radial-gradient(circle at top left, rgba(129, 140, 248, 0.16), transparent 26%),
    radial-gradient(circle at 84% 18%, rgba(251, 146, 60, 0.08), transparent 24%),
    linear-gradient(180deg, rgba(11, 12, 16, 0.98), rgba(6, 7, 10, 0.98));
  --logs-card-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.04),
    0 28px 72px rgba(2, 6, 23, 0.34);
  --logs-grid-overlay: rgba(255, 255, 255, 0.015);
  --logs-bottom-line: rgba(129, 140, 248, 0.3);
  --logs-divider: rgba(31, 33, 40, 0.88);
  --logs-chart-panel-bg: linear-gradient(180deg, rgba(13, 14, 20, 0.72), rgba(10, 11, 16, 0.28));
  --logs-chart-shell-bg:
    radial-gradient(circle, rgba(255, 255, 255, 0.012), transparent 62%),
    radial-gradient(circle at center, rgba(129, 140, 248, 0.08), transparent 72%);
  --logs-chart-shell-border: rgba(129, 140, 248, 0.08);
  --logs-title: #f8fafc;
  --logs-chart-label: #63646c;
  --logs-chart-value: #ffffff;
  --logs-chart-hint: #7f828f;
  --logs-table-head: #63646c;
  --logs-table-title: #989aa3;
  --logs-table-divider: rgba(31, 33, 40, 0.8);
  --logs-row-text: #a1a1aa;
  --logs-row-hover-bg: rgba(255, 255, 255, 0.035);
  --logs-row-hover-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.02);
  --logs-model-name: #a7aab4;
  --logs-model-name-active: #ffffff;
  --logs-track-bg: #16181d;
  --logs-metric: #71717a;
  --logs-token-icon: #4a4d57;
  --logs-cost: #e2e2e7;
  --logs-empty: #8d93a5;
  --logs-scrollbar: rgba(84, 88, 101, 0.92);
  --logs-line-chart-bg:
    radial-gradient(circle at top right, rgba(96, 165, 250, 0.08), transparent 30%),
    linear-gradient(180deg, rgba(11, 12, 16, 0.96), rgba(8, 9, 13, 0.96));
  --logs-line-chart-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.03),
    0 22px 48px rgba(2, 6, 23, 0.24);
  --logs-line-chart-overlay: linear-gradient(180deg, rgba(255, 255, 255, 0.018), transparent 18%);
}

.logs-model-share {
  position: relative;
  overflow: hidden;
  border-radius: 26px;
  border: 1px solid var(--logs-card-border);
  background: var(--logs-model-share-bg);
  box-shadow: var(--logs-card-shadow);
}

.logs-model-share::before,
.logs-model-share::after {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.logs-model-share::before {
  background:
    linear-gradient(90deg, var(--logs-grid-overlay) 0, var(--logs-grid-overlay) 1px, transparent 1px, transparent 100%);
  background-size: 24px 100%;
  opacity: 1;
}

.logs-model-share::after {
  inset: auto 0 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--logs-bottom-line), transparent);
}

.logs-model-share__header {
  position: relative;
  z-index: 1;
  padding: 1.05rem 1.45rem 0.2rem;
}

.logs-model-share__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--logs-title);
}

.logs-model-share__body {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: minmax(248px, 280px) minmax(0, 1fr);
  min-height: 320px;
}

.logs-model-share__chart-panel {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px 18px 24px;
  border-right: 1px solid var(--logs-divider);
  background: var(--logs-chart-panel-bg);
}

.logs-model-share__chart-shell {
  position: relative;
  display: grid;
  place-items: center;
  width: 100%;
  aspect-ratio: 1 / 1;
  border-radius: 999px;
  background: var(--logs-chart-shell-bg);
}

.logs-model-share__chart-shell::before {
  content: '';
  position: absolute;
  inset: 18px;
  border-radius: 999px;
  border: 1px solid var(--logs-chart-shell-border);
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.015);
}

.logs-model-share__chart-wrap {
  position: relative;
  width: min(100%, 214px);
  height: min(100%, 214px);
}

.logs-model-share__chart-center {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  pointer-events: none;
}

.logs-model-share__chart-label {
  font-size: 0.63rem;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--logs-chart-label);
}

.logs-model-share__chart-value {
  margin-top: 0.22rem;
  font-size: clamp(2rem, 1.6rem + 0.7vw, 2.45rem);
  line-height: 1;
  font-weight: 700;
  letter-spacing: -0.04em;
  color: var(--logs-chart-value);
  font-variant-numeric: tabular-nums;
}

.logs-model-share__chart-hint {
  max-width: 142px;
  margin-top: 0.55rem;
  font-size: 0.72rem;
  line-height: 1.35;
  color: var(--logs-chart-hint);
}

.logs-model-share__table {
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 8px 0 0;
}

.logs-model-share__table-head,
.logs-model-share__table-row {
  display: grid;
  grid-template-columns: minmax(0, 5fr) minmax(68px, 2fr) minmax(104px, 3fr) minmax(88px, 2fr);
  gap: 0.95rem;
  align-items: center;
}

.logs-model-share__table-head {
  position: relative;
  padding: 0.7rem 1.45rem 0.9rem;
  font-size: 0.63rem;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--logs-table-head);
  border-bottom: 1px solid var(--logs-table-divider);
}

.logs-model-share__table-title {
  position: relative;
  display: inline-flex;
  align-items: center;
  min-width: 0;
  color: var(--logs-table-title);
}

.logs-model-share__table-title::after {
  content: '';
  position: absolute;
  left: 0;
  bottom: -0.9rem;
  width: min(52%, 172px);
  height: 3px;
  border-radius: 999px;
  background: linear-gradient(90deg, #fb923c, rgba(251, 146, 60, 0));
}

.logs-model-share__table-metric {
  text-align: right;
}

.logs-model-share__table-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0 6px 0 0;
  scrollbar-width: thin;
  scrollbar-color: var(--logs-scrollbar) transparent;
}

.logs-model-share__table-row {
  position: relative;
  padding: 1rem 1.45rem;
  border-bottom: 1px solid var(--logs-table-divider);
  font-size: 0.78rem;
  color: var(--logs-row-text);
  font-variant-numeric: tabular-nums;
  transition: background-color 0.24s ease, box-shadow 0.24s ease, transform 0.24s ease;
}

.logs-model-share__table-row:hover,
.logs-model-share__table-row.is-active {
  background: var(--logs-row-hover-bg);
  box-shadow: var(--logs-row-hover-shadow);
}

.logs-model-share__model {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  min-width: 0;
}

.logs-model-share__dot {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 999px;
  flex-shrink: 0;
  transition: transform 0.24s ease, box-shadow 0.24s ease, opacity 0.24s ease;
}

.logs-model-share__model-copy {
  min-width: 0;
  flex: 1;
}

.logs-model-share__model-name {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.84rem;
  font-weight: 600;
  color: var(--logs-model-name);
  transition: color 0.24s ease;
}

.logs-model-share__table-row:hover .logs-model-share__model-name,
.logs-model-share__table-row.is-active .logs-model-share__model-name {
  color: var(--logs-model-name-active);
}

.logs-model-share__table-row:hover .logs-model-share__dot,
.logs-model-share__table-row.is-active .logs-model-share__dot {
  transform: scale(1.12);
}

.logs-model-share__track {
  position: relative;
  width: 100%;
  height: 3px;
  margin-top: 0.58rem;
  overflow: hidden;
  border-radius: 999px;
  background: var(--logs-track-bg);
}

.logs-model-share__track-fill {
  position: absolute;
  inset: 0 auto 0 0;
  border-radius: inherit;
  transform-origin: left center;
  animation: logs-model-share-bar-enter 0.96s cubic-bezier(0.16, 1, 0.3, 1) both;
}

.logs-model-share__metric,
.logs-model-share__cost {
  justify-self: end;
  color: var(--logs-metric);
  font-size: 0.82rem;
}

.logs-model-share__metric--token {
  display: inline-flex;
  align-items: center;
  gap: 0.38rem;
}

.logs-model-share__token-icon {
  width: 0.9rem;
  height: 0.9rem;
  color: var(--logs-token-icon);
  transition: color 0.24s ease;
}

.logs-model-share__table-row:hover .logs-model-share__token-icon,
.logs-model-share__table-row.is-active .logs-model-share__token-icon {
  color: #f59e0b;
}

.logs-model-share__cost {
  font-weight: 700;
  color: var(--logs-cost);
}

.logs-model-share__empty {
  display: grid;
  place-items: center;
  min-height: 280px;
  padding: 1.5rem;
  font-size: 0.95rem;
  color: var(--logs-empty);
}

.logs-model-share__table-body::-webkit-scrollbar {
  width: 6px;
}

.logs-model-share__table-body::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: var(--logs-scrollbar);
}

.logs-model-share__table-body::-webkit-scrollbar-track {
  background: transparent;
}

.logs-chart {
  position: relative;
  overflow: hidden;
  min-height: 260px;
  border: 1px solid var(--logs-card-border);
  border-radius: 24px;
  padding: 18px 18px 16px;
  background: var(--logs-line-chart-bg);
  box-shadow: var(--logs-line-chart-shadow);
}

.logs-chart::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: var(--logs-line-chart-overlay);
}

:deep(canvas) {
  max-width: 100%;
}

@keyframes logs-model-share-bar-enter {
  0% {
    transform: scaleX(0.18);
    opacity: 0.45;
  }
  100% {
    transform: scaleX(1);
    opacity: 1;
  }
}

@media (max-width: 768px) {
  .logs-model-share__body {
    grid-template-columns: 1fr;
  }

  .logs-model-share__chart-panel {
    padding: 20px 18px 10px;
    border-right: none;
    border-bottom: 1px solid var(--logs-divider);
  }

  .logs-model-share__chart-wrap {
    width: min(100%, 198px);
    height: 198px;
  }

  .logs-model-share__table-head,
  .logs-model-share__table-row {
    grid-template-columns: minmax(0, 2.4fr) minmax(56px, 0.8fr) minmax(88px, 1fr) minmax(76px, 0.85fr);
    gap: 0.6rem;
  }

  .logs-model-share__table-body {
    max-height: 260px;
  }

  .logs-chart {
    min-height: 220px;
  }
}

@media (max-width: 560px) {
  .logs-model-share__header,
  .logs-model-share__table-head,
  .logs-model-share__table-row {
    padding-left: 1rem;
    padding-right: 1rem;
  }

  .logs-model-share__table-title::after {
    width: 84px;
  }

  .logs-model-share__model {
    gap: 0.62rem;
  }

  .logs-model-share__model-name,
  .logs-model-share__metric,
  .logs-model-share__cost {
    font-size: 0.75rem;
  }
}
</style>
