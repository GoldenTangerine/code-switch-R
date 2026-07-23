<!--
/**
 * @name: 日志数据视图
 * @Descripttion: 在请求明细、Provider 统计和模型统计之间切换并提供排序。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-17 13:31:03
 * @LastEditTime: 2026-07-17 13:31:03
 * @FilePath: frontend/src/components/Logs/components/LogsDataTabs.vue
 */
-->
<template>
  <section class="logs-data-tabs">
    <div class="logs-data-tabs__header">
      <div class="logs-data-tabs__list" role="tablist" :aria-label="t('components.logs.tabs.label')">
        <button
          v-for="tab in tabs"
          :id="`logs-tab-${tab.key}`"
          :key="tab.key"
          type="button"
          role="tab"
          :class="['logs-data-tabs__tab', { 'is-active': activeTab === tab.key }]"
          :aria-selected="activeTab === tab.key"
          :aria-controls="`logs-panel-${tab.key}`"
          @click="emit('update:active-tab', tab.key)"
        >
          {{ tab.label }}
        </button>
      </div>
    </div>

    <div
      v-if="activeTab === 'requests'"
      id="logs-panel-requests"
      role="tabpanel"
      aria-labelledby="logs-tab-requests"
    >
      <slot name="requests"></slot>
    </div>

    <div
      v-else-if="activeTab === 'providers'"
      id="logs-panel-providers"
      class="logs-stats-table-wrap"
      role="tabpanel"
      aria-labelledby="logs-tab-providers"
    >
      <table class="logs-stats-table">
        <thead>
          <tr>
            <th scope="col">{{ t('components.logs.stats.provider') }}</th>
            <th v-for="column in providerColumns" :key="column.key" scope="col" :aria-sort="providerAriaSort(column.key)">
              <button type="button" class="logs-sort-button" @click="toggleProviderSort(column.key)">
                <span>{{ column.label }}</span>
                <span class="logs-sort-indicator" aria-hidden="true">{{ providerSortIndicator(column.key) }}</span>
              </button>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in sortedProviderStats" :key="item.provider_id || item.provider">
            <td class="logs-stats-name">{{ item.provider || '—' }}</td>
            <td>{{ formatNumber(item.total_requests) }}</td>
            <td>{{ formatNumber(item.excluded_requests) }}</td>
            <td>{{ formatTokenNumber(providerTotalTokens(item)) }}</td>
            <td>{{ formatCurrency(item.cost_total) }}</td>
            <td>{{ formatSuccessRate(item) }}</td>
            <td>{{ formatAverageDuration(item) }}</td>
          </tr>
          <tr v-if="!sortedProviderStats.length && !loading">
            <td :colspan="providerColumns.length + 1" class="logs-stats-empty">{{ t('components.logs.stats.empty') }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="loading" class="logs-stats-empty">{{ t('components.logs.loading') }}</p>
    </div>

    <div
      v-else
      id="logs-panel-models"
      class="logs-stats-table-wrap"
      role="tabpanel"
      aria-labelledby="logs-tab-models"
    >
      <table class="logs-stats-table">
        <thead>
          <tr>
            <th scope="col">{{ t('components.logs.stats.pricingModel') }}</th>
            <th v-for="column in modelColumns" :key="column.key" scope="col" :aria-sort="modelAriaSort(column.key)">
              <button type="button" class="logs-sort-button" @click="toggleModelSort(column.key)">
                <span>{{ column.label }}</span>
                <span class="logs-sort-indicator" aria-hidden="true">{{ modelSortIndicator(column.key) }}</span>
              </button>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in sortedModelStats" :key="item.model">
            <td class="logs-stats-name logs-stats-name--model" :title="item.model">{{ item.model || '—' }}</td>
            <td>{{ formatNumber(item.total_requests) }}</td>
            <td>{{ formatTokenNumber(item.total_tokens) }}</td>
            <td>{{ formatCurrency(item.cost_total) }}</td>
            <td>{{ formatPreciseCurrency(modelAverageCost(item)) }}</td>
          </tr>
          <tr v-if="!sortedModelStats.length && !loading">
            <td :colspan="modelColumns.length + 1" class="logs-stats-empty">{{ t('components.logs.stats.empty') }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="loading" class="logs-stats-empty">{{ t('components.logs.loading') }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ModelUsageStat, ProviderDailyStat } from '../../../services/logs'
import type { LogsDataTab } from '../types'
import { formatPreciseCurrency } from '../utils'

type SortDirection = 'asc' | 'desc'
type ProviderSortKey = 'requests' | 'excluded' | 'tokens' | 'cost' | 'success' | 'duration'
type ModelSortKey = 'requests' | 'tokens' | 'cost' | 'average'

const props = defineProps<{
  activeTab: LogsDataTab
  providerStats: ProviderDailyStat[]
  modelStats: ModelUsageStat[]
  loading: boolean
  formatNumber: (value?: number) => string
  formatTokenNumber: (value?: number) => string
  formatCurrency: (value?: number) => string
  formatDuration: (value?: number) => string
}>()

const emit = defineEmits<{
  (event: 'update:active-tab', value: LogsDataTab): void
}>()

const { t } = useI18n()
const providerSortKey = ref<ProviderSortKey>('cost')
const providerSortDirection = ref<SortDirection>('desc')
const modelSortKey = ref<ModelSortKey>('cost')
const modelSortDirection = ref<SortDirection>('desc')

const tabs = computed<Array<{ key: LogsDataTab; label: string }>>(() => [
  { key: 'requests', label: t('components.logs.tabs.requests') },
  { key: 'providers', label: t('components.logs.tabs.providers') },
  { key: 'models', label: t('components.logs.tabs.models') },
])

const providerColumns = computed<Array<{ key: ProviderSortKey; label: string }>>(() => [
  { key: 'requests', label: t('components.logs.stats.requests') },
  { key: 'excluded', label: t('components.logs.stats.excludedRequests') },
  { key: 'tokens', label: t('components.logs.stats.tokens') },
  { key: 'cost', label: t('components.logs.stats.cost') },
  { key: 'success', label: t('components.logs.stats.successRate') },
  { key: 'duration', label: t('components.logs.stats.averageDuration') },
])

const modelColumns = computed<Array<{ key: ModelSortKey; label: string }>>(() => [
  { key: 'requests', label: t('components.logs.stats.requests') },
  { key: 'tokens', label: t('components.logs.stats.tokens') },
  { key: 'cost', label: t('components.logs.stats.cost') },
  { key: 'average', label: t('components.logs.stats.averageCost') },
])

const providerTotalTokens = (item: ProviderDailyStat) =>
  Number(item.input_tokens || 0) + Number(item.output_tokens || 0) + Number(item.cache_read_tokens || 0)

const modelAverageCost = (item: ModelUsageStat) =>
  item.total_requests > 0 ? Number(item.cost_total || 0) / item.total_requests : 0

const providerSortValue = (item: ProviderDailyStat, key: ProviderSortKey) => {
  if (key === 'requests') return Number(item.total_requests || 0)
  if (key === 'excluded') return Number(item.excluded_requests || 0)
  if (key === 'tokens') return providerTotalTokens(item)
  if (key === 'success') return Number(item.success_rate || 0)
  if (key === 'duration') return Number(item.avg_duration_sec || 0)
  return Number(item.cost_total || 0)
}

const modelSortValue = (item: ModelUsageStat, key: ModelSortKey) => {
  if (key === 'requests') return Number(item.total_requests || 0)
  if (key === 'tokens') return Number(item.total_tokens || 0)
  if (key === 'average') return modelAverageCost(item)
  return Number(item.cost_total || 0)
}

const sortedProviderStats = computed(() => [...props.providerStats].sort((left, right) => {
  const delta = providerSortValue(left, providerSortKey.value) - providerSortValue(right, providerSortKey.value)
  if (delta !== 0) return providerSortDirection.value === 'asc' ? delta : -delta
  return String(left.provider || '').localeCompare(String(right.provider || ''))
}))

const sortedModelStats = computed(() => [...props.modelStats].sort((left, right) => {
  const delta = modelSortValue(left, modelSortKey.value) - modelSortValue(right, modelSortKey.value)
  if (delta !== 0) return modelSortDirection.value === 'asc' ? delta : -delta
  return String(left.model || '').localeCompare(String(right.model || ''))
}))

const toggleProviderSort = (key: ProviderSortKey) => {
  if (providerSortKey.value === key) {
    providerSortDirection.value = providerSortDirection.value === 'desc' ? 'asc' : 'desc'
    return
  }
  providerSortKey.value = key
  providerSortDirection.value = 'desc'
}

const toggleModelSort = (key: ModelSortKey) => {
  if (modelSortKey.value === key) {
    modelSortDirection.value = modelSortDirection.value === 'desc' ? 'asc' : 'desc'
    return
  }
  modelSortKey.value = key
  modelSortDirection.value = 'desc'
}

const providerAriaSort = (key: ProviderSortKey) =>
  providerSortKey.value === key ? (providerSortDirection.value === 'asc' ? 'ascending' : 'descending') : 'none'

const modelAriaSort = (key: ModelSortKey) =>
  modelSortKey.value === key ? (modelSortDirection.value === 'asc' ? 'ascending' : 'descending') : 'none'

const providerSortIndicator = (key: ProviderSortKey) =>
  providerSortKey.value === key ? (providerSortDirection.value === 'asc' ? '↑' : '↓') : '↕'

const modelSortIndicator = (key: ModelSortKey) =>
  modelSortKey.value === key ? (modelSortDirection.value === 'asc' ? '↑' : '↓') : '↕'

const formatSuccessRate = (item: ProviderDailyStat) => {
  const evaluatedRequests = Number(item.successful_requests || 0) + Number(item.failed_requests || 0)
  if (evaluatedRequests <= 0) return '—'
  return `${Math.max(0, Math.min(100, Number(item.success_rate || 0) * 100)).toFixed(1)}%`
}

const formatAverageDuration = (item: ProviderDailyStat) =>
  Number(item.duration_sample_count || 0) > 0 ? props.formatDuration(item.avg_duration_sec) : '—'
</script>

<style scoped>
.logs-data-tabs {
  overflow: hidden;
  border: 1px solid var(--mac-border);
  border-radius: 8px;
  background: color-mix(in srgb, var(--mac-surface) 96%, transparent);
}

.logs-data-tabs__header {
  display: flex;
  align-items: center;
  min-height: 52px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--mac-divider);
}

.logs-data-tabs__list {
  display: inline-flex;
  gap: 4px;
  padding: 3px;
  border-radius: 7px;
  background: color-mix(in srgb, var(--mac-surface-strong) 88%, transparent);
}

.logs-data-tabs__tab {
  min-height: 34px;
  border: 0;
  border-radius: 6px;
  padding: 0 13px;
  background: transparent;
  color: var(--mac-text-secondary);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
  transition: color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease;
}

.logs-data-tabs__tab:hover,
.logs-data-tabs__tab:focus-visible {
  color: var(--mac-text);
  background: color-mix(in srgb, var(--mac-surface) 90%, transparent);
}

.logs-data-tabs__tab:focus-visible {
  outline: 2px solid rgba(59, 130, 246, 0.55);
  outline-offset: 1px;
}

.logs-data-tabs__tab.is-active {
  color: #2563eb;
  background: var(--mac-surface);
  box-shadow: 0 1px 4px rgba(15, 23, 42, 0.12);
}

.logs-stats-table-wrap {
  overflow-x: auto;
}

.logs-stats-table {
  width: 100%;
  min-width: 760px;
  border-collapse: collapse;
  font-size: 0.82rem;
}

.logs-stats-table th,
.logs-stats-table td {
  padding: 12px 14px;
  border-bottom: 1px solid var(--mac-divider);
  text-align: right;
  white-space: nowrap;
}

.logs-stats-table th:first-child,
.logs-stats-table td:first-child {
  text-align: left;
}

.logs-stats-table tbody tr:hover {
  background: color-mix(in srgb, var(--mac-surface-strong) 82%, transparent);
}

.logs-sort-button {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  width: 100%;
  border: 0;
  padding: 0;
  background: transparent;
  color: var(--mac-text-secondary);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}

.logs-sort-button:hover,
.logs-sort-button:focus-visible {
  color: var(--mac-text);
}

.logs-sort-button:focus-visible {
  outline: 2px solid rgba(59, 130, 246, 0.55);
  outline-offset: 3px;
}

.logs-sort-indicator {
  width: 12px;
  color: #3b82f6;
  text-align: center;
}

.logs-stats-name {
  max-width: 260px;
  overflow: hidden;
  color: var(--mac-text);
  font-weight: 600;
  text-overflow: ellipsis;
}

.logs-stats-name--model {
  font-family: 'SFMono-Regular', Menlo, Consolas, monospace;
  font-size: 0.78rem;
}

.logs-stats-empty {
  margin: 0;
  padding: 24px;
  color: var(--mac-text-secondary);
  text-align: center !important;
}

html.dark .logs-data-tabs__tab.is-active {
  color: #93c5fd;
  box-shadow: 0 1px 5px rgba(0, 0, 0, 0.3);
}

@media (max-width: 640px) {
  .logs-data-tabs__list {
    width: 100%;
  }

  .logs-data-tabs__tab {
    flex: 1 1 0;
    min-width: 0;
    padding: 0 8px;
  }
}
</style>
