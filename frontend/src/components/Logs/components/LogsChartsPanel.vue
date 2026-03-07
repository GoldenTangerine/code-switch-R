<template>
  <section class="logs-model-share">
    <div class="logs-model-share__title">{{ t('components.logs.modelShare.title') }}</div>
    <div v-if="modelShareRows.length" class="logs-model-share__body">
      <div class="logs-model-share__chart-wrap">
        <Doughnut :data="modelShareChartData" :options="modelShareChartOptions" />
      </div>

      <div class="logs-model-share__table">
        <div class="logs-model-share__table-head">
          <span>{{ t('components.logs.modelShare.model') }}</span>
          <span>{{ t('components.logs.modelShare.requests') }}</span>
          <span>{{ t('components.logs.modelShare.tokens') }}</span>
          <span>{{ t('components.logs.modelShare.cost') }}</span>
        </div>
        <div class="logs-model-share__table-body">
          <div v-for="item in modelShareRows" :key="item.model" class="logs-model-share__table-row">
            <span class="logs-model-share__model">
              <span class="logs-model-share__dot" :style="{ backgroundColor: item.color }"></span>
              <span class="logs-model-share__model-name">{{ item.model }}</span>
            </span>
            <span>{{ formatNumber(item.requests) }}</span>
            <span>{{ formatTokenNumber(item.tokens) }}</span>
            <span class="logs-model-share__cost">{{ formatCurrency(item.cost) }}</span>
          </div>
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
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ChartData, ChartOptions } from 'chart.js'
import { Doughnut, Line } from 'vue-chartjs'
import type { ModelShareRow } from '../types'

defineProps<{
  modelShareRows: ModelShareRow[]
  modelShareChartData: ChartData<'doughnut'>
  modelShareChartOptions: ChartOptions<'doughnut'>
  chartData: ChartData<'line'>
  chartOptions: ChartOptions<'line'>
  formatNumber: (value?: number) => string
  formatTokenNumber: (value?: number) => string
  formatCurrency: (value?: number) => string
}>()

const { t } = useI18n()
</script>

<style scoped>
.logs-model-share {
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 16px;
  padding: 1rem 1.2rem;
  background: radial-gradient(circle at top left, rgba(79, 126, 227, 0.08), rgba(15, 23, 42, 0));
}

.logs-model-share__title {
  font-size: 1rem;
  font-weight: 600;
  color: #0f172a;
  margin-bottom: 0.9rem;
}

.logs-model-share__body {
  display: grid;
  grid-template-columns: minmax(170px, 220px) minmax(0, 1fr);
  gap: 1.25rem;
  align-items: center;
}

.logs-model-share__chart-wrap {
  width: min(100%, 190px);
  height: 190px;
  margin: 0 auto;
}

.logs-model-share__table {
  min-width: 0;
}

.logs-model-share__table-head,
.logs-model-share__table-row {
  display: grid;
  grid-template-columns: minmax(0, 2.2fr) minmax(70px, 0.8fr) minmax(90px, 1fr) minmax(90px, 0.9fr);
  gap: 0.75rem;
  align-items: center;
}

.logs-model-share__table-head {
  padding: 0.35rem 0.25rem 0.55rem;
  font-size: 0.83rem;
  font-weight: 600;
  color: #64748b;
}

.logs-model-share__table-body {
  max-height: 210px;
  overflow-y: auto;
}

.logs-model-share__table-row {
  padding: 0.7rem 0.25rem;
  border-top: 1px solid rgba(148, 163, 184, 0.2);
  font-size: 0.92rem;
  color: #334155;
  font-variant-numeric: tabular-nums;
}

.logs-model-share__model {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}

.logs-model-share__dot {
  width: 0.6rem;
  height: 0.6rem;
  border-radius: 999px;
  flex-shrink: 0;
}

.logs-model-share__model-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.logs-model-share__cost {
  color: #0f766e;
  font-weight: 600;
}

.logs-model-share__empty {
  font-size: 0.9rem;
  color: #64748b;
  padding: 0.35rem 0.1rem;
}

html.dark .logs-model-share {
  border-color: rgba(255, 255, 255, 0.12);
  background: radial-gradient(circle at top left, rgba(96, 165, 250, 0.16), rgba(15, 23, 42, 0.36));
}

html.dark .logs-model-share__title {
  color: rgba(248, 250, 252, 0.95);
}

html.dark .logs-model-share__table-head {
  color: rgba(186, 194, 210, 0.86);
}

html.dark .logs-model-share__table-row {
  border-top-color: rgba(148, 163, 184, 0.24);
  color: rgba(226, 232, 240, 0.92);
}

html.dark .logs-model-share__cost {
  color: #6ee7b7;
}

html.dark .logs-model-share__empty {
  color: #94a3b8;
}

@media (max-width: 768px) {
  .logs-model-share {
    padding: 0.9rem 1rem;
  }

  .logs-model-share__body {
    grid-template-columns: 1fr;
    gap: 0.9rem;
  }

  .logs-model-share__chart-wrap {
    height: 170px;
  }

  .logs-model-share__table-head,
  .logs-model-share__table-row {
    grid-template-columns: minmax(0, 1.9fr) minmax(0, 0.8fr) minmax(0, 0.95fr) minmax(0, 0.85fr);
    gap: 0.55rem;
    font-size: 0.82rem;
  }

  .logs-model-share__table-body {
    max-height: 230px;
  }
}
</style>
