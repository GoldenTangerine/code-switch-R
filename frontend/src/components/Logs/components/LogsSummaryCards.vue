<template>
  <section
    v-if="statsCards.length"
    :class="[
      'logs-summary',
      isDarkTheme ? 'logs-summary--dark' : 'logs-summary--light',
    ]"
  >
    <article
      v-for="card in statsCards"
      :key="card.key"
      :class="[
        'summary-card',
        `summary-card--${card.tone}`,
        `summary-card--${card.valueSize ?? 'regular'}`,
        { 'summary-card--clickable': isClickable(card.key) },
      ]"
      :style="resolveCardStyle(card.tone)"
      @click="handleCardClick(card.key)"
    >
      <div class="summary-card__glow"></div>

      <header class="summary-card__toolbar">
        <div class="summary-card__icon-box">
          <svg class="summary-card__icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <template v-if="card.key === 'requests'">
              <path d="M3 12h4l2-5 4 10 2-5h6" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
            </template>
            <template v-else-if="card.key === 'tokens'">
              <ellipse cx="12" cy="5.5" rx="5.5" ry="2.5" stroke="currentColor" stroke-width="1.7" />
              <path d="M6.5 5.5v5c0 1.4 2.46 2.5 5.5 2.5s5.5-1.1 5.5-2.5v-5" stroke="currentColor" stroke-width="1.7" />
              <path d="M6.5 10.5v5c0 1.4 2.46 2.5 5.5 2.5s5.5-1.1 5.5-2.5v-5" stroke="currentColor" stroke-width="1.7" />
            </template>
            <template v-else-if="card.key === 'cacheReads'">
              <path d="M13.5 3.5 6.8 13h4.6L10.5 20.5 17.2 11h-4.7z" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" />
            </template>
            <template v-else>
              <circle cx="12" cy="12" r="7.5" stroke="currentColor" stroke-width="1.7" />
              <path d="M14.8 9.4c0-1.1-1.07-1.9-2.63-1.9-1.52 0-2.67.78-2.67 1.95 0 1.13.96 1.66 2.42 1.95 1.66.33 3.08.78 3.08 2.47 0 1.39-1.19 2.22-2.8 2.39" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
              <path d="M12 6.2v11.6" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
            </template>
          </svg>
        </div>
        <span v-if="card.statusLabel" class="summary-card__status">{{ card.statusLabel }}</span>
      </header>

      <div class="summary-card__heading">
        <div class="summary-card__label-row">
          <p class="summary-card__label">{{ card.label }}</p>
          <span
            v-if="card.badge"
            :class="['summary-card__badge', `is-${card.badge.tone}`]"
          >
            {{ card.badge.text }}
          </span>
        </div>
        <p v-if="card.subtitle" class="summary-card__subtitle">{{ card.subtitle }}</p>
        <div class="summary-card__value-row">
          <h2 class="summary-card__value">{{ card.value }}</h2>
          <span v-if="card.valueSuffix" class="summary-card__value-suffix">{{ card.valueSuffix }}</span>
        </div>
        <p v-if="card.subValue" class="summary-card__sub-value">{{ card.subValue }}</p>
      </div>

      <div v-if="card.key === 'requests' && card.miniBars && card.progress" class="summary-card__body summary-card__body--requests">
        <div class="summary-card__request-chart">
          <div class="summary-card__spark-bars">
            <span
              v-for="(point, index) in card.miniBars.points"
              :key="`${card.key}-mini-${index}`"
              class="summary-card__spark-bar"
              :class="{ 'is-active': point.active }"
              :title="`${point.label} · ${point.value.toFixed(1)} QPS`"
              :style="{
                '--bar-scale': `${point.intensity}`,
                '--bar-opacity': `${0.22 + point.intensity * 0.68}`,
              }"
            ></span>
          </div>
          <div class="summary-card__request-side">
            <span class="summary-card__section-label">{{ card.miniBars.label }}</span>
            <span class="summary-card__request-rate">{{ extractFooterValue(card.miniBars.footerLeft) }}</span>
          </div>
        </div>
        <div class="summary-card__micro-footer">
          <span>{{ card.miniBars.footerLeft }}</span>
          <span>{{ card.miniBars.footerRight }}</span>
        </div>

        <div class="summary-card__progress-block">
          <div class="summary-card__progress-meta">
            <span>{{ card.progress.label }}</span>
            <strong>{{ card.progress.valueLabel }}</strong>
          </div>
          <div class="summary-card__progress-track">
            <span
              :class="['summary-card__progress-fill', `is-${card.progress.tone}`]"
              :style="{ width: `${card.progress.value}%` }"
            ></span>
          </div>
        </div>
      </div>

      <div v-else-if="card.key === 'tokens' && card.metrics?.length && card.ratio" class="summary-card__body summary-card__body--tokens">
        <div class="summary-card__metrics summary-card__metrics--token">
          <div
            v-for="metric in card.metrics"
            :key="`${card.key}-${metric.label}`"
            class="summary-card__metric"
          >
            <span class="summary-card__metric-label">{{ metric.label }}</span>
            <strong class="summary-card__metric-value">{{ metric.value }}</strong>
          </div>
        </div>

        <div class="summary-card__ratio-block">
          <span class="summary-card__section-label">{{ card.ratio.label }}</span>
          <div class="summary-card__ratio-meta">
            <span
              v-for="(segment, index) in card.ratio.segments"
              :key="`${card.key}-${segment.label}-meta`"
              class="summary-card__ratio-chip"
              :class="{
                'is-first': index === 0,
                'is-last': index === card.ratio.segments.length - 1,
              }"
              :style="{ '--ratio-chip-color': segment.color }"
            >
              <span class="summary-card__ratio-chip-label">{{ segment.label }}</span>
              <span class="summary-card__ratio-chip-value">{{ segment.valueLabel ?? '0%' }}</span>
            </span>
          </div>
          <div class="summary-card__ratio-track">
            <span
              v-for="segment in card.ratio.segments"
              :key="`${card.key}-${segment.label}`"
              class="summary-card__ratio-segment"
              :style="{
                width: `${segment.value}%`,
                '--segment-color': segment.color,
              }"
            ></span>
          </div>
        </div>
      </div>

      <div v-else-if="card.key === 'cacheReads' && card.ring && card.metrics?.[0]" class="summary-card__body summary-card__body--cache">
        <div class="summary-card__cache-layout">
          <div class="summary-card__ring-shell">
            <div class="summary-card__ring" :style="{ '--ring-progress': `${card.ring.value}%` }">
              <span class="summary-card__ring-value">{{ card.ring.valueLabel }}</span>
            </div>
            <span class="summary-card__ring-label">{{ card.ring.label }}</span>
          </div>

          <div class="summary-card__cache-content">
            <span class="summary-card__cache-label">{{ card.metrics[0].label }}</span>
            <strong
              :class="[
                'summary-card__cache-value',
                { 'is-animated': Boolean(card.metrics[0].animated) },
              ]"
            >
              <span v-if="card.metrics[0].icon" class="summary-card__metric-icon">{{ resolveMetricIcon(card.metrics[0].icon) }}</span>
              {{ card.metrics[0].value }}
            </strong>
            <div v-if="card.hint || card.ring.pulse" class="summary-card__cache-note">
              <span class="summary-card__pulse" :class="{ 'is-live': card.ring.pulse }"></span>
              <span>{{ card.hint }}</span>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="card.key === 'cost' && card.trend && card.metrics?.length" class="summary-card__body summary-card__body--cost">
        <span class="summary-card__section-label">{{ card.trend.label }}</span>
        <div
          class="summary-card__step-bars"
          :style="{ gridTemplateColumns: `repeat(${card.trend.points.length}, minmax(0, 1fr))` }"
        >
          <span
            v-for="(point, index) in card.trend.points"
            :key="`${card.key}-trend-${index}`"
            class="summary-card__step-bar"
            :class="{ 'is-active': point.active }"
            :title="`${point.label || '—'} · ${point.value.toFixed(4)}`"
            :style="{ '--bar-scale': `${point.intensity}` }"
          ></span>
        </div>

        <div class="summary-card__cost-grid">
          <div class="summary-card__cost-item">
            <span class="summary-card__cost-label">{{ card.metrics[0]?.label }}</span>
            <strong :class="['summary-card__cost-value', `tone-${card.metrics[0]?.tone ?? 'neutral'}`]">
              <span v-if="card.metrics[0]?.icon" class="summary-card__cost-icon">{{ resolveMetricIcon(card.metrics[0].icon) }}</span>
              {{ card.metrics[0]?.value }}
            </strong>
          </div>
          <div class="summary-card__cost-item summary-card__cost-item--bordered">
            <span class="summary-card__cost-label">{{ card.metrics[1]?.label }}</span>
            <strong class="summary-card__cost-value">{{ card.metrics[1]?.value }}</strong>
          </div>
        </div>
      </div>

      <div v-else class="summary-card__body">
        <div
          v-if="card.metrics?.length"
          class="summary-card__metrics"
          :class="{ 'summary-card__metrics--single': card.metrics.length === 1 }"
        >
          <div
            v-for="metric in card.metrics"
            :key="`${card.key}-${metric.label}`"
            class="summary-card__metric"
          >
            <span class="summary-card__metric-label">{{ metric.label }}</span>
            <strong
              :class="[
                'summary-card__metric-value',
                metric.tone ? `tone-${metric.tone}` : '',
                { 'is-animated': Boolean(metric.animated) },
              ]"
            >
              <span v-if="metric.icon" class="summary-card__metric-icon">{{ resolveMetricIcon(metric.icon) }}</span>
              {{ metric.value }}
            </strong>
          </div>
        </div>
      </div>

      <p
        v-if="card.hint && card.key !== 'cacheReads'"
        class="summary-card__hint"
      >
        {{ card.hint }}
      </p>
    </article>
  </section>
</template>

<script setup lang="ts">
import type { CSSProperties } from 'vue'
import type { LogsSummaryCard, LogsSummaryCardTone } from '../types'

defineProps<{
  statsCards: LogsSummaryCard[]
  isDarkTheme?: boolean
}>()

const emit = defineEmits<{
  (event: 'card-click', key: string): void
}>()

const isClickable = (key: string) => key === 'cost' || key === 'tokens'

const handleCardClick = (key: string) => {
  if (!isClickable(key)) return
  emit('card-click', key)
}

const tonePalette: Record<LogsSummaryCardTone, CSSProperties> = {
  blue: {
    '--summary-accent': '#3B82F6',
    '--summary-accent-soft': 'rgba(59,130,246,0.16)',
    '--summary-glow': '59,130,246',
  },
  purple: {
    '--summary-accent': '#A855F7',
    '--summary-accent-soft': 'rgba(168,85,247,0.16)',
    '--summary-glow': '168,85,247',
  },
  amber: {
    '--summary-accent': '#F59E0B',
    '--summary-accent-soft': 'rgba(245,158,11,0.16)',
    '--summary-glow': '245,158,11',
  },
  green: {
    '--summary-accent': '#10B981',
    '--summary-accent-soft': 'rgba(16,185,129,0.16)',
    '--summary-glow': '16,185,129',
  },
}

const resolveCardStyle = (tone: LogsSummaryCardTone) => tonePalette[tone]

const resolveMetricIcon = (icon: 'up' | 'alert' | 'spark') => {
  if (icon === 'up') return '↗'
  if (icon === 'alert') return '⚠'
  return '✦'
}

const extractFooterValue = (value: string) => value.split('·').pop()?.trim() ?? value
</script>

<style scoped>
.logs-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.82rem;
  width: 100%;
  margin: 0 0 0.78rem;
  padding: 0.92rem;
  border-radius: 26px;
  border: 1px solid rgba(148, 163, 184, 0.08);
  background:
    radial-gradient(circle at top center, rgba(37, 99, 235, 0.08), transparent 34%),
    linear-gradient(180deg, rgba(6, 10, 28, 0.98), rgba(4, 7, 21, 0.98));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.03),
    0 24px 50px -34px rgba(2, 6, 23, 0.95);
}

.summary-card {
  --summary-accent: #3b82f6;
  --summary-accent-soft: rgba(59, 130, 246, 0.16);
  --summary-glow: 59, 130, 246;
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 248px;
  padding: 1.12rem 1.08rem 1rem;
  border-radius: 28px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background:
    linear-gradient(180deg, rgba(11, 15, 26, 0.98), rgba(13, 18, 31, 0.98)),
    radial-gradient(circle at top right, rgba(var(--summary-glow), 0.08), transparent 40%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.03),
    0 18px 38px -32px rgba(0, 0, 0, 0.95);
}

.summary-card::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.03), transparent 24%);
  pointer-events: none;
}

.summary-card__glow,
.summary-card__toolbar,
.summary-card__heading,
.summary-card__body,
.summary-card__hint {
  position: relative;
  z-index: 1;
}

.summary-card__glow {
  position: absolute;
  right: -38px;
  top: -38px;
  width: 120px;
  height: 120px;
  border-radius: 999px;
  background: radial-gradient(circle, rgba(var(--summary-glow), 0.14), transparent 70%);
  pointer-events: none;
}

.summary-card__toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 0.94rem;
}

.summary-card__icon-box {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 14px;
  border: 1px solid rgba(var(--summary-glow), 0.24);
  background: var(--summary-accent-soft);
  color: var(--summary-accent);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.summary-card__icon {
  width: 18px;
  height: 18px;
  stroke: currentColor;
}

.summary-card__status {
  display: inline-flex;
  align-items: center;
  padding: 0.38rem 0.72rem;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.04);
  background: rgba(255, 255, 255, 0.04);
  color: rgba(148, 163, 184, 0.8);
  font-size: 0.58rem;
  font-weight: 800;
  letter-spacing: 0.2em;
  text-transform: uppercase;
}

.summary-card__heading {
  margin-bottom: 0.82rem;
}

.summary-card__label-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.summary-card__label {
  margin: 0;
  font-size: 0.68rem;
  font-weight: 800;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  color: rgba(148, 163, 184, 0.84);
}

.summary-card__badge {
  display: inline-flex;
  align-items: center;
  padding: 0.22rem 0.48rem;
  border-radius: 999px;
  font-size: 0.58rem;
  font-weight: 800;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  border: 1px solid rgba(239, 68, 68, 0.18);
}

.summary-card__badge.is-alert {
  color: #fca5a5;
  background: rgba(127, 29, 29, 0.32);
}

.summary-card__badge.is-success {
  color: #86efac;
  background: rgba(20, 83, 45, 0.32);
}

.summary-card__badge.is-warning {
  color: #fcd34d;
  background: rgba(120, 53, 15, 0.3);
}

.summary-card__badge.is-neutral {
  color: rgba(203, 213, 225, 0.92);
  background: rgba(51, 65, 85, 0.45);
}

.summary-card__value-row {
  display: flex;
  align-items: baseline;
  gap: 0.42rem;
  margin-top: 0.3rem;
}

.summary-card__subtitle {
  display: none;
}

.summary-card__value {
  margin: 0;
  font-size: 2.05rem;
  line-height: 1;
  font-weight: 800;
  letter-spacing: -0.06em;
  color: #f8fafc;
  font-family: 'SFMono-Regular', 'JetBrains Mono', Consolas, 'Liberation Mono', Menlo, monospace;
}

.summary-card--compact .summary-card__value {
  font-size: 1.92rem;
}

.summary-card--dense .summary-card__value {
  font-size: 1.68rem;
}

.summary-card__value-suffix {
  font-size: 0.84rem;
  font-weight: 600;
  color: rgba(148, 163, 184, 0.72);
}

.summary-card__sub-value {
  display: none;
}

.summary-card__body {
  display: flex;
  flex: 1;
  flex-direction: column;
  justify-content: flex-end;
}

.summary-card__body--requests,
.summary-card__body--tokens,
.summary-card__body--cost {
  gap: 0.72rem;
}

.summary-card__request-chart {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 0.8rem;
}

.summary-card__request-side {
  display: flex;
  min-width: 66px;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.16rem;
}

.summary-card__request-rate {
  margin-bottom: 0.08rem;
  white-space: nowrap;
  font-size: 0.7rem;
  font-weight: 700;
  color: var(--summary-accent);
}

.summary-card__spark-bars {
  display: flex;
  align-items: flex-end;
  gap: 0.28rem;
  min-height: 34px;
}

.summary-card__spark-bar {
  display: block;
  width: 3px;
  height: calc(6px + var(--bar-scale) * 14px);
  border-radius: 999px 999px 2px 2px;
  background: rgba(var(--summary-glow), var(--bar-opacity, 0.6));
}

.summary-card__spark-bar.is-active {
  background: var(--summary-accent);
  box-shadow: 0 0 0 1px rgba(var(--summary-glow), 0.18), 0 0 10px rgba(var(--summary-glow), 0.2);
}

.summary-card__micro-footer,
.summary-card__section-label {
  font-size: 0.56rem;
  font-weight: 800;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.summary-card__micro-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.56rem;
  color: rgba(148, 163, 184, 0.72);
}

.summary-card__micro-footer span:last-child {
  text-align: right;
}

.summary-card__section-label {
  color: rgba(148, 163, 184, 0.68);
}

.summary-card__progress-block {
  display: flex;
  flex-direction: column;
  gap: 0.36rem;
}

.summary-card__progress-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  font-size: 0.64rem;
  font-weight: 700;
  color: rgba(148, 163, 184, 0.86);
}

.summary-card__progress-meta strong {
  color: #f8fafc;
  font-family: 'SFMono-Regular', 'JetBrains Mono', Consolas, monospace;
}

.summary-card__progress-track {
  height: 3px;
  width: 100%;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(51, 65, 85, 0.72);
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.45);
}

.summary-card__progress-fill {
  display: block;
  height: 100%;
  border-radius: inherit;
}

.summary-card__progress-fill.is-primary {
  background: linear-gradient(90deg, rgba(96, 165, 250, 0.95), var(--summary-accent));
  box-shadow: 0 0 8px rgba(var(--summary-glow), 0.28);
}

.summary-card__progress-fill.is-success {
  background: linear-gradient(90deg, #34d399, #10b981);
}

.summary-card__progress-fill.is-alert {
  background: linear-gradient(90deg, #fb7185, #ef4444);
}

.summary-card__metrics {
  display: grid;
  gap: 0.52rem;
}

.summary-card__metrics--token {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.summary-card__metric {
  display: flex;
  flex-direction: column;
  gap: 0.18rem;
  padding: 0.7rem 0.62rem;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.05);
  background: rgba(255, 255, 255, 0.05);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.02);
}

.summary-card__metric-label {
  font-size: 0.54rem;
  font-weight: 800;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  text-align: center;
  color: rgba(148, 163, 184, 0.7);
}

.summary-card__metric-value {
  text-align: center;
  font-size: 0.92rem;
  font-weight: 700;
  color: var(--summary-accent);
  font-family: 'SFMono-Regular', 'JetBrains Mono', Consolas, monospace;
}

.summary-card__ratio-block {
  display: flex;
  flex-direction: column;
  gap: 0.38rem;
}

.summary-card__ratio-meta {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.32rem;
}

.summary-card__ratio-chip {
  display: flex;
  align-items: center;
  gap: 0.16rem;
  min-width: 0;
  color: var(--ratio-chip-color);
  font-size: 0.52rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.summary-card__ratio-chip.is-first {
  justify-content: flex-start;
}

.summary-card__ratio-chip.is-last {
  justify-content: flex-end;
}

.summary-card__ratio-chip:not(.is-first):not(.is-last) {
  justify-content: center;
}

.summary-card__ratio-chip-label,
.summary-card__ratio-chip-value {
  white-space: nowrap;
}

.summary-card__ratio-chip-value {
  color: color-mix(in srgb, var(--ratio-chip-color) 82%, white 18%);
}

.summary-card__ratio-track {
  display: flex;
  overflow: hidden;
  height: 5px;
  border-radius: 999px;
  background: rgba(51, 65, 85, 0.72);
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.45);
}

.summary-card__ratio-segment {
  display: block;
  height: 100%;
  background: linear-gradient(90deg, color-mix(in srgb, var(--segment-color) 76%, white 24%), var(--segment-color));
}

.summary-card__body--cache {
  justify-content: center;
}

.summary-card__cache-layout {
  display: flex;
  align-items: center;
  gap: 0.86rem;
}

.summary-card__ring-shell {
  flex: none;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.34rem;
}

.summary-card__ring {
  --ring-progress: 0%;
  position: relative;
  display: grid;
  place-items: center;
  width: 58px;
  height: 58px;
  border-radius: 999px;
  background: conic-gradient(var(--summary-accent) 0 var(--ring-progress), rgba(51, 65, 85, 0.78) var(--ring-progress) 100%);
}

.summary-card__ring::before {
  content: '';
  position: absolute;
  inset: 4px;
  border-radius: inherit;
  background: #0f1423;
}

.summary-card__ring-value {
  position: relative;
  z-index: 1;
  font-size: 0.72rem;
  font-weight: 800;
  color: var(--summary-accent);
  font-family: 'SFMono-Regular', 'JetBrains Mono', Consolas, monospace;
}

.summary-card__ring-label {
  max-width: 68px;
  text-align: center;
  font-size: 0.52rem;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: rgba(148, 163, 184, 0.7);
}

.summary-card__cache-content {
  display: flex;
  flex-direction: column;
  gap: 0.24rem;
}

.summary-card__cache-label {
  font-size: 0.58rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgba(148, 163, 184, 0.74);
}

.summary-card__cache-value {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 1.38rem;
  line-height: 1;
  font-weight: 800;
  color: #6ee7b7;
  font-family: 'SFMono-Regular', 'JetBrains Mono', Consolas, monospace;
}

.summary-card__cache-value.is-animated,
.summary-card__metric-value.is-animated {
  animation: metric-pop 1.9s ease-in-out infinite;
}

.summary-card__cache-note {
  display: inline-flex;
  align-items: center;
  gap: 0.42rem;
  font-size: 0.6rem;
  color: rgba(148, 163, 184, 0.72);
}

.summary-card__pulse {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.35);
}

.summary-card__pulse.is-live {
  background: #22c55e;
  box-shadow: 0 0 0 rgba(34, 197, 94, 0.42);
  animation: pulse-dot 1.8s infinite;
}

.summary-card__step-bars {
  display: flex;
  align-items: flex-end;
  gap: 0.28rem;
  height: 30px;
  padding: 0 0.12rem;
}

.summary-card__step-bar {
  display: block;
  flex: 1;
  min-width: 0;
  height: calc(6px + var(--bar-scale) * 18px);
  border-radius: 3px 3px 0 0;
  background: rgba(var(--summary-glow), 0.22);
  transition: filter 0.18s ease, box-shadow 0.18s ease;
}

.summary-card__step-bar.is-active {
  background: var(--summary-accent);
  box-shadow: 0 -2px 8px rgba(var(--summary-glow), 0.28);
}

.summary-card__cost-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  padding-top: 0.68rem;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
}

.summary-card__cost-item {
  display: flex;
  flex-direction: column;
  gap: 0.18rem;
}

.summary-card__cost-item--bordered {
  padding-left: 0.74rem;
  border-left: 1px solid rgba(255, 255, 255, 0.05);
}

.summary-card__cost-label {
  font-size: 0.54rem;
  font-weight: 800;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: rgba(148, 163, 184, 0.72);
}

.summary-card__cost-value {
  display: inline-flex;
  align-items: center;
  gap: 0.28rem;
  font-size: 0.92rem;
  font-weight: 700;
  color: #f8fafc;
  font-family: 'SFMono-Regular', 'JetBrains Mono', Consolas, monospace;
}

.summary-card__cost-value.tone-success {
  color: #6ee7b7;
}

.summary-card__cost-value.tone-warning {
  color: #fbbf24;
}

.summary-card__cost-value.tone-danger {
  color: #fca5a5;
}

.summary-card__cost-icon {
  font-size: 0.92rem;
}

.summary-card__metric-icon {
  font-size: 0.9rem;
  line-height: 1;
}

.summary-card__metric-value {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.24rem;
}

.summary-card__metric-value.tone-success {
  color: #6ee7b7;
}

.summary-card__metric-value.tone-warning {
  color: #fbbf24;
}

.summary-card__metric-value.tone-danger {
  color: #fca5a5;
}

.summary-card__hint {
  display: none;
}

.summary-card--clickable {
  cursor: pointer;
  transition: transform 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease;
}

.summary-card--clickable:hover {
  transform: translateY(-2px);
  border-color: rgba(255, 255, 255, 0.24);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.04),
    0 24px 44px -34px rgba(var(--summary-glow), 0.48);
}

.logs-summary--light {
  border-color: rgba(203, 213, 225, 0.72);
  background:
    radial-gradient(circle at 16% 0%, rgba(99, 102, 241, 0.13), transparent 34%),
    radial-gradient(circle at 88% 12%, rgba(14, 165, 233, 0.1), transparent 32%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.96));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.9),
    0 24px 52px -38px rgba(15, 23, 42, 0.32);
}

.logs-summary--light .summary-card {
  border-color: rgba(203, 213, 225, 0.78);
  background:
    radial-gradient(circle at top right, rgba(var(--summary-glow), 0.11), transparent 42%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.95));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.86),
    0 18px 38px -30px rgba(15, 23, 42, 0.26);
}

.logs-summary--light .summary-card::after {
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.72), transparent 30%);
}

.logs-summary--light .summary-card__glow {
  background: radial-gradient(circle, rgba(var(--summary-glow), 0.16), transparent 72%);
}

.logs-summary--light .summary-card__icon-box {
  border-color: rgba(var(--summary-glow), 0.2);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.82);
}

.logs-summary--light .summary-card__status {
  border-color: rgba(148, 163, 184, 0.18);
  background: rgba(15, 23, 42, 0.04);
  color: rgba(71, 85, 105, 0.74);
}

.logs-summary--light .summary-card__label,
.logs-summary--light .summary-card__micro-footer,
.logs-summary--light .summary-card__section-label,
.logs-summary--light .summary-card__metric-label,
.logs-summary--light .summary-card__ring-label,
.logs-summary--light .summary-card__cache-label,
.logs-summary--light .summary-card__cache-note,
.logs-summary--light .summary-card__cost-label,
.logs-summary--light .summary-card__sub-value {
  color: rgba(71, 85, 105, 0.74);
}

.logs-summary--light .summary-card__value,
.logs-summary--light .summary-card__progress-meta strong,
.logs-summary--light .summary-card__cost-value {
  color: #0f172a;
}

.logs-summary--light .summary-card__value-suffix,
.logs-summary--light .summary-card__progress-meta {
  color: rgba(51, 65, 85, 0.78);
}

.logs-summary--light .summary-card__badge.is-alert {
  color: #dc2626;
  background: rgba(254, 226, 226, 0.92);
}

.logs-summary--light .summary-card__badge.is-success {
  color: #047857;
  background: rgba(209, 250, 229, 0.92);
}

.logs-summary--light .summary-card__badge.is-warning {
  color: #b45309;
  background: rgba(254, 243, 199, 0.95);
}

.logs-summary--light .summary-card__badge.is-neutral {
  color: #475569;
  background: rgba(241, 245, 249, 0.95);
}

.logs-summary--light .summary-card__progress-track,
.logs-summary--light .summary-card__ratio-track {
  background: rgba(226, 232, 240, 0.96);
  box-shadow: inset 0 1px 2px rgba(15, 23, 42, 0.08);
}

.logs-summary--light .summary-card__metric {
  border-color: rgba(203, 213, 225, 0.68);
  background: rgba(248, 250, 252, 0.9);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.82);
}

.logs-summary--light .summary-card__ratio-chip-value {
  color: color-mix(in srgb, var(--ratio-chip-color) 72%, #0f172a 28%);
}

.logs-summary--light .summary-card__ring {
  background: conic-gradient(var(--summary-accent) 0 var(--ring-progress), rgba(226, 232, 240, 0.95) var(--ring-progress) 100%);
}

.logs-summary--light .summary-card__ring::before {
  background: rgba(255, 255, 255, 0.98);
}

.logs-summary--light .summary-card__cache-value,
.logs-summary--light .summary-card__cost-value.tone-success,
.logs-summary--light .summary-card__metric-value.tone-success {
  color: #047857;
}

.logs-summary--light .summary-card__cost-value.tone-warning,
.logs-summary--light .summary-card__metric-value.tone-warning {
  color: #b45309;
}

.logs-summary--light .summary-card__cost-value.tone-danger,
.logs-summary--light .summary-card__metric-value.tone-danger {
  color: #dc2626;
}

.logs-summary--light .summary-card__pulse {
  background: rgba(100, 116, 139, 0.28);
}

.logs-summary--light .summary-card__step-bar {
  background: rgba(var(--summary-glow), 0.18);
}

.logs-summary--light .summary-card__cost-grid {
  border-top-color: rgba(203, 213, 225, 0.72);
}

.logs-summary--light .summary-card__cost-item--bordered {
  border-left-color: rgba(203, 213, 225, 0.72);
}

.logs-summary--light .summary-card--clickable:hover {
  border-color: rgba(var(--summary-glow), 0.32);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.9),
    0 26px 48px -34px rgba(var(--summary-glow), 0.44);
}

@keyframes pulse-dot {
  0% {
    transform: scale(0.95);
    box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.38);
  }
  70% {
    transform: scale(1.08);
    box-shadow: 0 0 0 8px rgba(34, 197, 94, 0);
  }
  100% {
    transform: scale(0.95);
    box-shadow: 0 0 0 0 rgba(34, 197, 94, 0);
  }
}

@keyframes metric-pop {
  0%,
  100% {
    transform: translateY(0);
    text-shadow: 0 0 0 rgba(110, 231, 183, 0);
  }
  50% {
    transform: translateY(-1px);
    text-shadow: 0 0 14px rgba(110, 231, 183, 0.18);
  }
}

@media (max-width: 1180px) {
  .logs-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .logs-summary {
    grid-template-columns: minmax(0, 1fr);
    padding: 0.9rem;
  }

  .summary-card {
    min-height: 236px;
    padding: 1rem 0.95rem 0.92rem;
    border-radius: 24px;
  }
}
</style>
