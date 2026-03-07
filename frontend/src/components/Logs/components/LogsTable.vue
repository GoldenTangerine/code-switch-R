<template>
  <section class="logs-table-wrapper">
    <table class="logs-table">
      <thead>
        <tr>
          <th class="col-time">{{ t('components.logs.table.time') }}</th>
          <th class="col-platform">{{ t('components.logs.table.platform') }}</th>
          <th class="col-provider">{{ t('components.logs.table.provider') }}</th>
          <th class="col-model">{{ t('components.logs.table.model') }}</th>
          <th class="col-verify">{{ t('components.logs.table.verify') }}</th>
          <th class="col-http">{{ t('components.logs.table.httpCode') }}</th>
          <th class="col-stream">{{ t('components.logs.table.stream') }}</th>
          <th class="col-duration">{{ t('components.logs.table.duration') }}</th>
          <th class="col-performance">{{ t('components.logs.table.performance') }}</th>
          <th class="col-cost">{{ t('components.logs.table.cost') }}</th>
          <th class="col-tokens">{{ t('components.logs.table.tokens') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in items" :key="item.id">
          <td>{{ formatters.formatTime(item.created_at) }}</td>
          <td>{{ item.platform || '—' }}</td>
          <td>{{ item.provider || '—' }}</td>
          <td class="model-cell">
            <span
              class="model-name model-meta-trigger"
              tabindex="0"
              aria-haspopup="true"
              :aria-label="formatters.formatModelInfoAriaLabel(item)"
              :aria-describedby="logInfoTooltipVisible ? 'logs-table-info-tooltip' : undefined"
              @mouseenter="handlers.scheduleShowModelInfoTooltip(item, $event)"
              @mousemove="handlers.moveLogInfoTooltip($event)"
              @mouseleave="handlers.hideLogInfoTooltip"
              @focus="handlers.showModelInfoTooltip(item, $event)"
              @blur="handlers.hideLogInfoTooltip"
              @keydown.esc="handlers.hideLogInfoTooltipImmediately"
            >
              {{ item.model || '—' }}
            </span>
          </td>
          <td class="verify-cell">
            <span
              :class="['verify-tag', `verify-${formatters.resolveModelVerifyStatus(item)}`, 'verify-meta-trigger']"
              tabindex="0"
              aria-haspopup="true"
              :aria-label="formatters.formatVerifyInfoAriaLabel(item)"
              :aria-describedby="logInfoTooltipVisible ? 'logs-table-info-tooltip' : undefined"
              @mouseenter="handlers.scheduleShowVerifyInfoTooltip(item, $event)"
              @mousemove="handlers.moveLogInfoTooltip($event)"
              @mouseleave="handlers.hideLogInfoTooltip"
              @focus="handlers.showVerifyInfoTooltip(item, $event)"
              @blur="handlers.hideLogInfoTooltip"
              @keydown.esc="handlers.hideLogInfoTooltipImmediately"
            >
              {{ formatters.formatModelVerifyStatus(item) }}
            </span>
          </td>
          <td :class="['code', formatters.httpCodeClass(item.http_code)]">{{ item.http_code }}</td>
          <td><span :class="['stream-tag', item.is_stream ? 'on' : 'off']">{{ formatters.formatStream(item.is_stream) }}</span></td>
          <td><span :class="['duration-tag', formatters.durationColor(item.duration_sec)]">{{ formatters.formatDuration(item.duration_sec) }}</span></td>
          <td class="performance-cell">
            <div>
              <span class="performance-badge performance-badge--ttft">首</span>
              <span class="token-value">{{ formatters.formatFirstTokenMs(item) }}</span>
            </div>
            <div>
              <button
                type="button"
                class="performance-trigger"
                :title="t('components.logs.payloadDetail.openHint')"
                :aria-label="formatters.formatPayloadDetailAriaLabel(item)"
                @click="handlers.openPayloadDetailModal(item)"
              >
                <span class="performance-badge performance-badge--tps">速</span>
                <span class="token-value">{{ formatters.formatTokensPerSecond(item) }}</span>
              </button>
            </div>
          </td>
          <td class="cost-cell">
            <span
              class="cost-cell__value"
              tabindex="0"
              aria-haspopup="true"
              :aria-label="formatters.formatCostAriaLabel(item)"
              :aria-describedby="costTooltipVisible ? 'logs-table-cost-tooltip' : undefined"
              @mouseenter="handlers.scheduleShowCostTooltip(item, $event)"
              @mousemove="handlers.moveCostTooltip($event)"
              @mouseleave="handlers.hideCostTooltip"
              @focus="handlers.showCostTooltip(item, $event)"
              @blur="handlers.hideCostTooltip"
              @keydown.esc="handlers.hideCostTooltipImmediately"
            >
              {{ formatters.formatCurrency(item.total_cost) }}
            </span>
          </td>
          <td class="token-cell">
            <div>
              <span class="token-label">{{ t('components.logs.tokenLabels.input') }}</span>
              <span class="token-value">{{ formatters.formatTokenNumber(item.input_tokens) }}</span>
            </div>
            <div>
              <span class="token-label">{{ t('components.logs.tokenLabels.output') }}</span>
              <span class="token-value">{{ formatters.formatTokenNumber(item.output_tokens) }}</span>
            </div>
            <div>
              <span class="token-label">{{ t('components.logs.tokenLabels.reasoning') }}</span>
              <span class="token-value">{{ formatters.formatTokenNumber(item.reasoning_tokens) }}</span>
            </div>
            <div class="token-breakdown-row">
              <span class="token-label">{{ t('components.logs.tokenLabels.cacheWrite') }}</span>
              <span class="token-value">{{ formatters.formatTokenNumber(item.cache_create_tokens) }}</span>
              <span v-if="formatters.hasCacheCreateDetail(item)" class="cache-create-badges">
                <span v-if="formatters.resolveEphemeral5mTokens(item) > 0" class="cache-create-badge cache-create-badge--5m">
                  {{ t('components.logs.tokenLabels.cacheWrite5m') }} {{ formatters.formatTokenNumber(formatters.resolveEphemeral5mTokens(item)) }}
                </span>
                <span v-if="formatters.resolveEphemeral1hTokens(item) > 0" class="cache-create-badge cache-create-badge--1h">
                  {{ t('components.logs.tokenLabels.cacheWrite1h') }} {{ formatters.formatTokenNumber(formatters.resolveEphemeral1hTokens(item)) }}
                </span>
              </span>
            </div>
            <div>
              <span class="token-label">{{ t('components.logs.tokenLabels.cacheRead') }}</span>
              <span class="token-value">{{ formatters.formatTokenNumber(item.cache_read_tokens) }}</span>
            </div>
          </td>
        </tr>
        <tr v-if="!items.length && !loading">
          <td colspan="11" class="empty">{{ t('components.logs.empty') }}</td>
        </tr>
      </tbody>
    </table>
    <p v-if="loading" class="empty">{{ t('components.logs.loading') }}</p>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { RequestLog } from '../../../services/logs'

type TooltipPointerEvent = MouseEvent | FocusEvent

type LogsTableFormatters = {
  formatTime: (value?: string) => string
  formatModelInfoAriaLabel: (item: RequestLog) => string
  formatVerifyInfoAriaLabel: (item: RequestLog) => string
  resolveModelVerifyStatus: (item: RequestLog) => string
  formatModelVerifyStatus: (item: RequestLog) => string
  httpCodeClass: (code: number) => string
  formatStream: (value?: boolean | number) => string
  durationColor: (value?: number) => string
  formatDuration: (value?: number) => string
  formatFirstTokenMs: (item: RequestLog) => string
  formatPayloadDetailAriaLabel: (item: RequestLog) => string
  formatTokensPerSecond: (item: RequestLog) => string
  formatCostAriaLabel: (item: RequestLog) => string
  formatCurrency: (value?: number) => string
  formatTokenNumber: (value?: number) => string
  hasCacheCreateDetail: (item: RequestLog) => boolean
  resolveEphemeral5mTokens: (item: RequestLog) => number
  resolveEphemeral1hTokens: (item: RequestLog) => number
}

type LogsTableHandlers = {
  scheduleShowModelInfoTooltip: (item: RequestLog, event: MouseEvent) => void
  moveLogInfoTooltip: (event: MouseEvent) => void
  hideLogInfoTooltip: () => void
  showModelInfoTooltip: (item: RequestLog, event: TooltipPointerEvent) => void
  hideLogInfoTooltipImmediately: () => void
  scheduleShowVerifyInfoTooltip: (item: RequestLog, event: MouseEvent) => void
  showVerifyInfoTooltip: (item: RequestLog, event: TooltipPointerEvent) => void
  openPayloadDetailModal: (item: RequestLog) => void | Promise<void>
  scheduleShowCostTooltip: (item: RequestLog, event: MouseEvent) => void
  moveCostTooltip: (event: MouseEvent) => void
  hideCostTooltip: () => void
  showCostTooltip: (item: RequestLog, event: TooltipPointerEvent) => void
  hideCostTooltipImmediately: () => void
}

defineProps<{
  items: RequestLog[]
  loading: boolean
  logInfoTooltipVisible: boolean
  costTooltipVisible: boolean
  formatters: LogsTableFormatters
  handlers: LogsTableHandlers
}>()

const { t } = useI18n()
</script>
