<template>
  <BaseModal
    :open="open"
    :title="t('components.logs.storage.title')"
    :panel-width="'min(1320px, 98vw)'"
    @close="emit('close')"
  >
    <div class="logs-storage-modal logs-storage-modal--wide">
      <div class="logs-storage-actions logs-storage-actions--modal">
        <BaseButton
          variant="outline"
          size="sm"
          :disabled="storageLoading || storageHeatmapLoading"
          @click="handlers.refreshStorageOverview"
        >
          {{ t('components.logs.storage.refresh') }}
        </BaseButton>
      </div>
      <p v-if="storageLoading && !storageStats" class="cost-detail-loading">
        {{ t('components.logs.loading') }}
      </p>
      <div v-else-if="storageStats" class="mac-panel logs-storage-panel">
        <div class="logs-storage-db">
          <div class="logs-storage-db-line">
            {{ t('components.logs.storage.db') }}：
            {{ t('components.logs.storage.used') }} {{ formatters.formatBytes(storageStats.database.used_bytes) }}
            /
            {{ formatters.formatBytes(storageStats.database.total_bytes || storageStats.database.file_bytes) }}
            <span v-if="storageStats.database.free_bytes">
              （{{ t('components.logs.storage.free') }} {{ formatters.formatBytes(storageStats.database.free_bytes) }}）
            </span>
            <span v-if="storageStats.database.wal_bytes">
              · {{ t('components.logs.storage.wal') }} {{ formatters.formatBytes(storageStats.database.wal_bytes) }}
            </span>
          </div>
        </div>

        <div class="logs-storage-rows">
          <div class="logs-storage-row">
            <div class="logs-storage-name">{{ t('components.logs.storage.requestLog') }}</div>
            <div class="logs-storage-meta">
              {{ t('components.logs.storage.rows', { count: storageStats.request_log.rows }) }}
              · {{ formatters.formatBytes(storageStats.request_log.bytes, storageStats.request_log.rows) }}
            </div>
            <BaseButton
              variant="outline"
              size="sm"
              :disabled="storageClearing"
              @click="handlers.handleClearRequestLogs"
            >
              {{ storageClearing ? t('components.logs.storage.clearing') : t('components.logs.storage.clearRequestLog') }}
            </BaseButton>
          </div>

          <div class="logs-storage-row">
            <div class="logs-storage-name">{{ t('components.logs.storage.stats') }}</div>
            <div class="logs-storage-meta">
              {{ t('components.logs.storage.rows', { count: storageStats.stats_hour.rows + storageStats.stats_day.rows }) }}
              · {{ formatters.formatBytes(
                storageStats.stats_hour.bytes + storageStats.stats_day.bytes,
                storageStats.stats_hour.rows + storageStats.stats_day.rows,
              ) }}
            </div>
            <BaseButton
              variant="outline"
              size="sm"
              :disabled="storageClearing"
              @click="handlers.handleClearStats"
            >
              {{ storageClearing ? t('components.logs.storage.clearing') : t('components.logs.storage.clearStats') }}
            </BaseButton>
          </div>
        </div>
      </div>
      <p v-else class="cost-detail-empty">
        {{ t('components.logs.storage.empty') }}
      </p>

      <section
        class="contrib-wall logs-storage-heatmap-panel"
        :aria-label="t('components.logs.storage.heatmapAriaLabel', { year: storageHeatmapYear })"
      >
        <div class="logs-storage-heatmap-header">
          <div class="logs-storage-heatmap-header__content">
            <h3 class="logs-storage-heatmap-title">{{ t('components.logs.storage.heatmapTitle') }}</h3>
            <p class="logs-storage-heatmap-subtitle">
              {{ t('components.logs.storage.heatmapHint', { year: storageHeatmapYear }) }}
            </p>
          </div>
          <div class="logs-storage-heatmap-header__meta">
            <label class="logs-storage-heatmap-year-field">
              <span class="logs-storage-heatmap-year-label">{{ t('components.logs.storage.heatmapYearLabel') }}</span>
              <select
                :value="storageHeatmapYear"
                class="mac-select logs-storage-heatmap-year-select"
                :aria-label="t('components.logs.storage.heatmapYearAria')"
                :disabled="storageClearing || storageHeatmapLoading"
                @change="handlers.updateStorageHeatmapYear(($event.target as HTMLSelectElement).value)"
              >
                <option v-for="year in storageHeatmapYears" :key="`storage-heatmap-year-${year}`" :value="year">
                  {{ year }}
                </option>
              </select>
            </label>
            <span v-if="storageHeatmapLoading" class="logs-storage-heatmap-status">
              {{ t('components.logs.loading') }}
            </span>
          </div>
        </div>

        <div class="contrib-legend">
          <span>{{ t('components.main.heatmap.legendLow') }}</span>
          <span
            v-for="level in 5"
            :key="`storage-legend-${level}`"
            :class="['legend-box', formatters.intensityClass(level - 1)]"
          />
          <span>{{ t('components.main.heatmap.legendHigh') }}</span>
        </div>

        <div class="logs-storage-heatmap-grid-shell">
          <div class="logs-storage-heatmap-grid-scroll">
            <div class="contrib-grid logs-storage-heatmap-grid">
              <div
                v-for="(week, weekIndex) in storageHeatmap"
                :key="`storage-week-${weekIndex}`"
                class="contrib-column"
              >
                <div
                  v-for="(day, dayIndex) in week"
                  :key="`storage-day-${weekIndex}-${dayIndex}`"
                  :class="[
                    'contrib-cell',
                    'logs-storage-heatmap-day',
                    formatters.intensityClass(day.intensity),
                    {
                      'logs-storage-heatmap-day--interactive': day.requests > 0,
                      'logs-storage-heatmap-day--selected': formatters.isSelectedStorageHeatmapDay(day),
                    },
                  ]"
                  :role="day.requests > 0 ? 'button' : undefined"
                  :aria-hidden="day.requests > 0 ? undefined : true"
                  :aria-label="day.requests > 0 ? formatters.formatStorageHeatmapAriaLabel(day) : undefined"
                  :aria-pressed="day.requests > 0 ? formatters.isSelectedStorageHeatmapDay(day) : undefined"
                  :tabindex="day.requests > 0 ? 0 : undefined"
                  @mouseenter="handlers.showStorageHeatmapTooltip(day, $event)"
                  @mousemove="handlers.showStorageHeatmapTooltip(day, $event)"
                  @mouseleave="handlers.hideStorageHeatmapTooltip"
                  @focus="day.requests > 0 ? handlers.showStorageHeatmapTooltip(day, $event) : undefined"
                  @blur="day.requests > 0 ? handlers.hideStorageHeatmapTooltip() : undefined"
                  @keydown.esc="day.requests > 0 ? handlers.hideStorageHeatmapTooltip() : undefined"
                  @keydown.enter.prevent="day.requests > 0 ? handlers.selectStorageHeatmapDay(day) : undefined"
                  @keydown.space.prevent="day.requests > 0 ? handlers.selectStorageHeatmapDay(day) : undefined"
                  @click="day.requests > 0 ? handlers.selectStorageHeatmapDay(day) : undefined"
                />
              </div>
            </div>
          </div>
        </div>

        <div class="mac-panel logs-storage-heatmap-detail">
          <template v-if="selectedStorageHeatmapDay">
            <div class="logs-storage-heatmap-detail__header">
              <div class="logs-storage-heatmap-detail__summary">
                <div class="logs-storage-heatmap-detail__eyebrow">
                  {{ t('components.logs.storage.selectedDate') }}
                </div>
                <div class="logs-storage-heatmap-detail__date">{{ selectedStorageHeatmapDateLabel }}</div>
                <div class="logs-storage-heatmap-detail__meta-wrap">
                  <span class="logs-storage-heatmap-detail__meta">
                    {{ t('components.logs.storage.rows', { count: selectedStorageHeatmapDay.requests }) }}
                  </span>
                  <span v-if="storageDayLogsShowingText" class="logs-storage-heatmap-detail__meta">
                    {{ storageDayLogsShowingText }}
                  </span>
                </div>
              </div>
              <BaseButton
                variant="danger"
                size="sm"
                :disabled="storageClearing"
                @click="handlers.handleClearRequestLogsByDate"
              >
                {{ storageClearing ? t('components.logs.storage.clearing') : t('components.logs.storage.clearByDate') }}
              </BaseButton>
            </div>
            <p class="logs-storage-heatmap-detail__hint">
              {{ t('components.logs.storage.dayLogsHint') }}
            </p>

            <p v-if="storageDayLogsLoading" class="logs-storage-day-list__state">
              {{ t('components.logs.loading') }}
            </p>
            <p v-else-if="!storageDayLogs.length" class="logs-storage-day-list__state">
              {{ t('components.logs.storage.dayLogsEmpty', { date: selectedStorageHeatmapDateLabel }) }}
            </p>
            <template v-else>
              <div class="logs-storage-day-list">
                <article
                  v-for="item in pagedStorageDayLogs"
                  :key="`storage-day-log-${item.id}`"
                  class="logs-storage-day-item"
                >
                  <div class="logs-storage-day-item__top">
                    <span class="logs-storage-day-item__time">{{ formatters.formatTime(item.created_at) }}</span>
                    <button
                      type="button"
                      class="logs-storage-day-item__detail-btn"
                      @click="handlers.openPayloadDetailModal(item)"
                    >
                      {{ t('components.logs.payloadDetail.openButton') }}
                    </button>
                  </div>

                  <div class="logs-storage-day-item__main">
                    <div class="logs-storage-day-item__provider">{{ item.provider || '—' }}</div>
                    <div class="logs-storage-day-item__model">{{ item.model || '—' }}</div>
                  </div>

                  <div class="logs-storage-day-item__meta">
                    <span class="logs-storage-day-item__badge logs-storage-day-item__badge--platform">
                      {{ item.platform || '—' }}
                    </span>
                    <span
                      :class="[
                        'logs-storage-day-item__badge',
                        'logs-storage-day-item__badge--code',
                        formatters.httpCodeClass(item.http_code),
                      ]"
                    >
                      HTTP {{ item.http_code }}
                    </span>
                    <span :class="['stream-tag', item.is_stream ? 'on' : 'off']">
                      {{ formatters.formatStream(item.is_stream) }}
                    </span>
                    <span :class="['duration-tag', formatters.durationColor(item.duration_sec)]">
                      {{ formatters.formatDuration(item.duration_sec) }}
                    </span>
                  </div>

                  <div class="logs-storage-day-item__stats">
                    <div class="logs-storage-day-item__stat">
                      <span class="logs-storage-day-item__stat-label">{{ t('components.logs.table.tokens') }}</span>
                      <span class="logs-storage-day-item__stat-value">{{ formatters.formatTokenNumber(formatters.resolveStorageDayLogTotalTokens(item)) }}</span>
                    </div>
                    <div class="logs-storage-day-item__stat">
                      <span class="logs-storage-day-item__stat-label">{{ t('components.logs.table.cost') }}</span>
                      <span class="logs-storage-day-item__stat-value logs-storage-day-item__stat-value--success">{{ formatters.formatCurrency(item.total_cost) }}</span>
                    </div>
                  </div>
                </article>
              </div>

              <div v-if="storageDayLogsTotalPages > 1" class="logs-storage-day-pagination">
                <span>{{ storageDayLogsPage }} / {{ storageDayLogsTotalPages }}</span>
                <div class="pagination-actions">
                  <BaseButton
                    variant="outline"
                    size="sm"
                    :disabled="storageDayLogsPage === 1 || storageDayLogsLoading"
                    @click="handlers.prevStorageDayLogsPage"
                  >
                    ‹
                  </BaseButton>
                  <BaseButton
                    variant="outline"
                    size="sm"
                    :disabled="storageDayLogsPage >= storageDayLogsTotalPages || storageDayLogsLoading"
                    @click="handlers.nextStorageDayLogsPage"
                  >
                    ›
                  </BaseButton>
                </div>
              </div>
            </template>
          </template>
          <p v-else class="logs-storage-heatmap-detail__empty">
            {{ storageHeatmapHasData
              ? t('components.logs.storage.dayLogsPlaceholder')
              : t('components.logs.storage.selectedDateEmpty', { year: storageHeatmapYear }) }}
          </p>
        </div>
      </section>
    </div>
  </BaseModal>

  <Teleport to="body">
    <div
      v-if="storageHeatmapTooltip.visible"
      :ref="bindStorageHeatmapTooltipRef"
      class="contrib-tooltip logs-storage-heatmap-tooltip"
      :class="[
        storageHeatmapTooltip.placement,
        { 'is-positioned': storageHeatmapTooltip.positioned },
      ]"
      :style="{ left: `${storageHeatmapTooltip.left}px`, top: `${storageHeatmapTooltip.top}px` }"
      role="tooltip"
    >
      <p class="tooltip-heading">{{ storageHeatmapTooltip.label }}</p>
      <div class="tooltip-summary-grid logs-storage-heatmap-tooltip__summary-grid">
        <div class="tooltip-summary-card is-info">
          <span class="tooltip-summary-label">{{ t('components.logs.storage.heatmapTooltipRequests') }}</span>
          <span class="tooltip-summary-value">
            {{ t('components.logs.storage.rows', { count: storageHeatmapTooltip.requests }) }}
          </span>
        </div>
        <div class="tooltip-summary-card is-violet">
          <span class="tooltip-summary-label">{{ t('components.logs.storage.heatmapTooltipPayload') }}</span>
          <span class="tooltip-summary-value">
            {{ formatters.formatStorageHeatmapPayloadValue(
              storageHeatmapTooltip.payloadBytes,
              storageHeatmapTooltip.payloadCapturedRequests,
              storageHeatmapTooltip.requests,
            ) }}
          </span>
        </div>
        <div class="tooltip-summary-card" :class="storageHeatmapTooltip.intensity > 0 ? 'is-success' : 'is-neutral'">
          <span class="tooltip-summary-label">{{ t('components.logs.storage.heatmapTooltipLevel') }}</span>
          <span class="tooltip-summary-value">L{{ storageHeatmapTooltip.intensity }}</span>
        </div>
      </div>
      <p class="logs-storage-heatmap-tooltip__hint">
        {{ storageHeatmapTooltip.requests > 0
          ? t('components.logs.storage.heatmapHoverHint')
          : t('components.logs.storage.heatmapZeroHint') }}
      </p>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { Teleport, type ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseButton from '../../common/BaseButton.vue'
import BaseModal from '../../common/BaseModal.vue'
import type { RequestLog, LogStorageStats } from '../../../services/logs'
import type { UsageHeatmapDay } from '../../../data/usageHeatmap'

type StorageHeatmapTooltipPlacement = 'above' | 'below'
type StorageHeatmapTooltipState = {
  visible: boolean
  positioned: boolean
  left: number
  top: number
  placement: StorageHeatmapTooltipPlacement
  label: string
  requests: number
  payloadBytes: number
  payloadCapturedRequests: number
  intensity: number
}

type RefBinder = (element: Element | ComponentPublicInstance | null) => void

type StorageModalFormatters = {
  formatBytes: (bytes?: number, rows?: number) => string
  intensityClass: (value: number) => string
  isSelectedStorageHeatmapDay: (day: UsageHeatmapDay) => boolean
  formatStorageHeatmapAriaLabel: (day: UsageHeatmapDay) => string
  formatTime: (value?: string) => string
  formatTokenNumber: (value?: number) => string
  formatCurrency: (value?: number) => string
  resolveStorageDayLogTotalTokens: (item: RequestLog) => number
  httpCodeClass: (code: number) => string
  formatStream: (value?: boolean | number) => string
  durationColor: (value?: number) => string
  formatDuration: (value?: number) => string
  formatStorageHeatmapPayloadValue: (bytes?: number, capturedRequests?: number, requests?: number) => string
}

type StorageModalHandlers = {
  refreshStorageOverview: () => void | Promise<void>
  handleClearRequestLogs: () => void
  handleClearRequestLogsByDate: () => void
  handleClearStats: () => void
  updateStorageHeatmapYear: (year: number | string) => void | Promise<void>
  showStorageHeatmapTooltip: (day: UsageHeatmapDay, event: MouseEvent | FocusEvent) => void
  hideStorageHeatmapTooltip: () => void
  selectStorageHeatmapDay: (day: UsageHeatmapDay) => void
  prevStorageDayLogsPage: () => void
  nextStorageDayLogsPage: () => void
  openPayloadDetailModal: (item: RequestLog) => void | Promise<void>
}

defineProps<{
  open: boolean
  storageLoading: boolean
  storageClearing: boolean
  storageStats: LogStorageStats | null
  storageHeatmapYear: number
  storageHeatmapYears: number[]
  storageHeatmapLoading: boolean
  storageHeatmap: UsageHeatmapDay[][]
  selectedStorageHeatmapDay: UsageHeatmapDay | null
  selectedStorageHeatmapDateLabel: string
  storageDayLogsShowingText: string
  storageDayLogsLoading: boolean
  storageDayLogs: RequestLog[]
  pagedStorageDayLogs: RequestLog[]
  storageDayLogsPage: number
  storageDayLogsTotalPages: number
  storageHeatmapHasData: boolean
  storageHeatmapTooltip: StorageHeatmapTooltipState
  bindStorageHeatmapTooltipRef: RefBinder
  formatters: StorageModalFormatters
  handlers: StorageModalHandlers
}>()

const emit = defineEmits<{
  (event: 'close'): void
}>()

const { t } = useI18n()
</script>

<style scoped>
.logs-storage-modal--wide {
  gap: 18px;
  min-height: min(78vh, 920px);
}

.logs-storage-heatmap-panel {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.logs-storage-heatmap-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.logs-storage-heatmap-header__content {
  flex: 1 1 auto;
  min-width: 0;
}

.logs-storage-heatmap-header__meta {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 10px 12px;
  flex: 0 0 auto;
}

.logs-storage-heatmap-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  color: var(--contrib-text);
}

.logs-storage-heatmap-subtitle {
  margin: 6px 0 0;
  font-size: 0.84rem;
  line-height: 1.45;
  color: var(--contrib-muted);
}

.logs-storage-heatmap-year-field {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-height: 36px;
  padding: 6px 10px 6px 12px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(15, 23, 42, 0.08);
}

.logs-storage-heatmap-year-label {
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--contrib-muted);
}

.logs-storage-heatmap-year-select {
  width: auto;
  min-width: 104px;
  min-height: 32px;
  padding: 0.35rem 1.95rem 0.35rem 0.8rem;
  border-radius: 999px;
  background-color: rgba(255, 255, 255, 0.08);
  font-size: 0.85rem;
  font-weight: 600;
}

.logs-storage-heatmap-status {
  flex: 0 0 auto;
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--contrib-muted);
}

.logs-storage-heatmap-grid-shell {
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.14), rgba(15, 23, 42, 0.04));
  padding: 14px;
  overflow: hidden;
}

.logs-storage-heatmap-grid-scroll {
  display: flex;
  justify-content: center;
  overflow-x: auto;
  overflow-y: hidden;
  padding: 4px 2px 8px;
}

.logs-storage-heatmap-grid {
  flex: 0 0 auto;
  width: max-content;
  min-width: 100%;
  justify-content: center;
  gap: 4px;
}

.logs-storage-heatmap-grid .contrib-column {
  flex: 0 0 14px;
  min-width: 14px;
  max-width: 14px;
}

.logs-storage-heatmap-grid-scroll::-webkit-scrollbar {
  height: 8px;
}

.logs-storage-heatmap-grid-scroll::-webkit-scrollbar-thumb {
  background: rgba(148, 163, 184, 0.28);
  border-radius: 999px;
}

.logs-storage-heatmap-day {
  display: block;
  width: 100%;
  min-width: 0;
  margin: 0;
  border: 1px solid rgba(255, 255, 255, 0.04);
  padding: 0;
  cursor: default;
  transition:
    transform 0.14s ease,
    box-shadow 0.14s ease,
    border-color 0.14s ease,
    opacity 0.14s ease;
}

.logs-storage-heatmap-day:hover {
  transform: translateY(-1px) scale(1.06);
  box-shadow: 0 0 0 1px rgba(148, 163, 184, 0.22);
}

.logs-storage-heatmap-day--interactive {
  cursor: pointer;
}

.logs-storage-heatmap-day:not(.logs-storage-heatmap-day--interactive):hover {
  transform: none;
  box-shadow: none;
}

.logs-storage-heatmap-day--selected {
  box-shadow:
    0 0 0 2px rgba(255, 255, 255, 0.9),
    0 0 0 4px rgba(34, 197, 94, 0.36),
    0 0 18px rgba(34, 197, 94, 0.28);
}

html.dark .logs-storage-heatmap-day--selected {
  box-shadow:
    0 0 0 2px rgba(15, 23, 42, 0.92),
    0 0 0 4px rgba(52, 211, 153, 0.44),
    0 0 18px rgba(16, 185, 129, 0.32);
}

.logs-storage-heatmap-day:focus-visible {
  outline: 2px solid rgba(56, 189, 248, 0.76);
  outline-offset: 2px;
}

.logs-storage-heatmap-detail {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 16px;
  padding: 18px;
}

.logs-storage-heatmap-detail__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.logs-storage-heatmap-detail__summary {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.logs-storage-heatmap-detail__eyebrow {
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--mac-text-secondary);
}

.logs-storage-heatmap-detail__date {
  font-size: 1.08rem;
  font-weight: 700;
  color: var(--mac-text);
}

.logs-storage-heatmap-detail__meta-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.logs-storage-heatmap-detail__meta,
.logs-storage-heatmap-detail__hint,
.logs-storage-heatmap-detail__empty {
  font-size: 0.88rem;
  color: var(--mac-text-secondary);
}

.logs-storage-heatmap-detail__meta {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 0 10px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.08);
}

.logs-storage-heatmap-detail__meta--warning {
  color: #f59e0b;
  border-color: rgba(245, 158, 11, 0.24);
  background: rgba(245, 158, 11, 0.12);
}

.logs-storage-heatmap-detail__hint {
  margin: -6px 0 0;
  line-height: 1.5;
}

.logs-storage-heatmap-detail__empty {
  margin: 0;
}

.logs-storage-day-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.logs-storage-day-list__state {
  margin: 0;
  padding: 18px;
  border-radius: 16px;
  border: 1px dashed rgba(148, 163, 184, 0.22);
  background: rgba(148, 163, 184, 0.05);
  color: var(--mac-text-secondary);
  text-align: center;
}

.logs-storage-day-item {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.04), rgba(148, 163, 184, 0.04));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
  transition:
    transform 0.16s ease,
    border-color 0.16s ease,
    box-shadow 0.16s ease,
    background 0.16s ease;
}

.logs-storage-day-item:hover,
.logs-storage-day-item:focus-within {
  transform: translateY(-1px);
  border-color: rgba(96, 165, 250, 0.24);
  box-shadow:
    0 12px 24px rgba(15, 23, 42, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.05);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.06), rgba(96, 165, 250, 0.05));
}

.logs-storage-day-item__top,
.logs-storage-day-item__stats {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.logs-storage-day-item__time {
  font-size: 0.86rem;
  font-weight: 600;
  color: var(--mac-text-secondary);
}

.logs-storage-day-item__detail-btn {
  border: 1px solid rgba(96, 165, 250, 0.24);
  background: rgba(59, 130, 246, 0.08);
  color: #60a5fa;
  border-radius: 999px;
  padding: 0.35rem 0.75rem;
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.14s ease, border-color 0.14s ease, color 0.14s ease;
}

.logs-storage-day-item__detail-btn:hover {
  background: rgba(59, 130, 246, 0.14);
  border-color: rgba(96, 165, 250, 0.36);
}

.logs-storage-day-item__detail-btn:focus-visible {
  outline: 2px solid rgba(96, 165, 250, 0.56);
  outline-offset: 2px;
}

.logs-storage-day-item__main {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.logs-storage-day-item__provider,
.logs-storage-day-item__model {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.logs-storage-day-item__provider {
  font-size: 0.92rem;
  font-weight: 700;
  color: var(--mac-text);
}

.logs-storage-day-item__model {
  font-size: 0.84rem;
  color: var(--mac-text-secondary);
}

.logs-storage-day-item__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.logs-storage-day-item__badge {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  padding: 0 10px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.08);
  color: var(--mac-text-secondary);
  font-size: 0.78rem;
  font-weight: 600;
}

.logs-storage-day-item__badge--platform {
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.logs-storage-day-item__badge--code.http-success {
  color: #16a34a;
  border-color: rgba(34, 197, 94, 0.24);
  background: rgba(34, 197, 94, 0.12);
}

.logs-storage-day-item__badge--code.http-client-error,
.logs-storage-day-item__badge--code.http-server-error {
  color: #ef4444;
  border-color: rgba(239, 68, 68, 0.24);
  background: rgba(239, 68, 68, 0.12);
}

.logs-storage-day-item__badge--code.http-redirect,
.logs-storage-day-item__badge--code.http-info {
  color: #f59e0b;
  border-color: rgba(245, 158, 11, 0.24);
  background: rgba(245, 158, 11, 0.12);
}

.logs-storage-day-item__stats {
  padding-top: 4px;
  border-top: 1px solid rgba(148, 163, 184, 0.14);
}

.logs-storage-day-item__stat {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.logs-storage-day-item__stat-label {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--mac-text-secondary);
}

.logs-storage-day-item__stat-value {
  font-size: 0.92rem;
  font-weight: 700;
  color: var(--mac-text);
}

.logs-storage-day-item__stat-value--success {
  color: #22c55e;
}

.logs-storage-day-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 4px;
}

.logs-storage-heatmap-tooltip__summary-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin-bottom: 10px;
}

.logs-storage-heatmap-tooltip {
  width: min(420px, calc(100vw - 32px));
  z-index: 2105;
}

.logs-storage-heatmap-tooltip__hint {
  margin: 0;
  font-size: 0.82rem;
  line-height: 1.5;
  color: var(--contrib-tooltip-text);
}

.cost-detail-loading,
.cost-detail-empty {
  text-align: center;
  color: #64748b;
  padding: 2rem 0;
}

html.dark .cost-detail-loading,
html.dark .cost-detail-empty {
  color: #94a3b8;
}

html.dark .logs-storage-day-item {
  border-color: rgba(148, 163, 184, 0.18);
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.74), rgba(15, 23, 42, 0.5));
}

html.dark .logs-storage-day-item:hover,
html.dark .logs-storage-day-item:focus-within {
  border-color: rgba(96, 165, 250, 0.38);
  box-shadow:
    0 14px 28px rgba(2, 6, 23, 0.32),
    inset 0 1px 0 rgba(148, 163, 184, 0.06);
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.82), rgba(30, 41, 59, 0.7));
}

html.dark .logs-storage-heatmap-grid-shell {
  border-color: rgba(148, 163, 184, 0.16);
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.55), rgba(15, 23, 42, 0.3));
}

html.dark .logs-storage-heatmap-year-field {
  background: rgba(15, 23, 42, 0.54);
  border-color: rgba(148, 163, 184, 0.22);
}

html.dark .logs-storage-heatmap-year-select {
  background-color: rgba(15, 23, 42, 0.82);
}

@media (max-width: 768px) {
  .logs-storage-heatmap-header,
  .logs-storage-heatmap-detail__header {
    flex-direction: column;
    align-items: stretch;
  }

  .logs-storage-heatmap-header__meta {
    justify-content: space-between;
  }

  .logs-storage-heatmap-year-field {
    justify-content: space-between;
    width: 100%;
  }

  .logs-storage-heatmap-year-select {
    min-width: 0;
    flex: 1 1 auto;
  }

  .logs-storage-heatmap-status {
    align-self: flex-start;
  }

  .logs-storage-day-list {
    grid-template-columns: 1fr;
  }

  .logs-storage-day-item__top,
  .logs-storage-day-item__stats {
    flex-direction: column;
    align-items: flex-start;
  }

  .logs-storage-day-pagination {
    flex-direction: column;
    align-items: stretch;
  }

  .logs-storage-heatmap-grid .contrib-column {
    flex-basis: 12px;
    min-width: 12px;
    max-width: 12px;
  }
}

@media (max-width: 640px) {
  .logs-storage-heatmap-tooltip__summary-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
