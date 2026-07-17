<template>
  <section class="logs-table-wrapper">
    <table class="logs-table">
      <thead>
        <tr>
          <th class="col-time">{{ t('components.logs.table.time') }}</th>
          <th class="col-platform">{{ t('components.logs.table.platform') }}</th>
          <th class="col-source">{{ t('components.logs.table.source') }}</th>
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
          <td><span class="source-tag">{{ sourceLabel(item) }}</span></td>
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
              <span class="model-name__text">{{ resolvePricingModel(item) }}</span>
              <template v-for="reasoningEffort in [formatters.formatReasoningEffort(item)]" :key="`${item.id}-reasoning-effort`">
                <span
                  v-if="reasoningEffort"
                  :class="['reasoning-effort', `reasoning-effort--${formatters.formatReasoningEffortTone(reasoningEffort)}`]"
                >
                  {{ reasoningEffort }}
                </span>
              </template>
            </span>
            <span v-if="resolveModelTrail(item)" class="model-route-trail" :title="resolveModelTrail(item)">
              {{ resolveModelTrail(item) }}
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
          <td :class="['code', { 'is-session': isSessionLog(item) }, formatters.httpCodeClass(item.http_code)]">
            <template v-if="isSessionLog(item)">—</template>
            <span
              v-else-if="formatters.hasStreamDiagnosticData(item)"
              class="http-diagnostic-trigger"
              tabindex="0"
              aria-haspopup="true"
              :aria-label="formatters.formatStreamDiagnosticAriaLabel(item)"
              :aria-describedby="logInfoTooltipVisible ? 'logs-table-info-tooltip' : undefined"
              @mouseenter="handlers.scheduleShowStreamInfoTooltip(item, $event)"
              @mousemove="handlers.moveLogInfoTooltip($event)"
              @mouseleave="handlers.hideLogInfoTooltip"
              @focus="handlers.showStreamInfoTooltip(item, $event)"
              @blur="handlers.hideLogInfoTooltip"
              @keydown.esc="handlers.hideLogInfoTooltipImmediately"
            >
              {{ item.http_code }}
            </span>
            <template v-else>{{ item.http_code }}</template>
          </td>
          <td><span :class="['stream-tag', item.is_stream ? 'on' : 'off']">{{ formatters.formatStream(item.is_stream) }}</span></td>
          <td><span :class="['duration-tag', formatters.durationColor(item.duration_sec)]">{{ formatters.formatDuration(item.duration_sec) }}</span></td>
          <td class="performance-cell">
            <div>
              <span class="performance-badge performance-badge--ttft">首</span>
              <span class="token-value">{{ formatters.formatFirstTokenDuration(item) }}</span>
            </div>
            <div>
              <button
                type="button"
                class="performance-trigger"
                :title="t('components.logs.payloadDetail.openHint')"
                :aria-label="formatters.formatPayloadDetailAriaLabel(item)"
                :disabled="isSessionLog(item)"
                @click="handlers.openPayloadDetailModal(item)"
              >
                <span class="performance-badge performance-badge--tps">速</span>
                <span class="token-value">{{ formatters.formatTokensPerSecond(item) }}</span>
              </button>
            </div>
          </td>
          <td class="cost-cell">
            <span
              :class="['cost-cell__value', { 'cost-cell__value--zero': !item.total_cost }]"
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
                <template v-if="formatters.resolveEphemeral5mTokens(item) > 0">
                  <span class="cache-create-badge cache-create-badge--5m">{{ t('components.logs.tokenLabels.cacheWrite5m') }}</span>
                  <span class="cache-create-num cache-create-num--5m">{{ formatters.formatTokenNumber(formatters.resolveEphemeral5mTokens(item)) }}</span>
                </template>
                <template v-if="formatters.resolveEphemeral1hTokens(item) > 0">
                  <span class="cache-create-badge cache-create-badge--1h">{{ t('components.logs.tokenLabels.cacheWrite1h') }}</span>
                  <span class="cache-create-num cache-create-num--1h">{{ formatters.formatTokenNumber(formatters.resolveEphemeral1hTokens(item)) }}</span>
                </template>
              </span>
            </div>
            <div>
              <span class="token-label">{{ t('components.logs.tokenLabels.cacheRead') }}</span>
              <span class="token-value">{{ formatters.formatTokenNumber(item.cache_read_tokens) }}</span>
            </div>
          </td>
        </tr>
        <tr v-if="!items.length && !loading">
          <td colspan="12" class="empty">{{ t('components.logs.empty') }}</td>
        </tr>
      </tbody>
    </table>
    <p v-if="loading" class="empty">{{ t('components.logs.loading') }}</p>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { RequestLog } from '../../../services/logs'
import { isSessionRequestLog } from '../utils'

type TooltipPointerEvent = MouseEvent | FocusEvent

type LogsTableFormatters = {
  formatTime: (value?: string) => string
  formatModelInfoAriaLabel: (item: RequestLog) => string
  formatReasoningEffort: (item: RequestLog) => string
  formatReasoningEffortTone: (value?: string) => string
  formatVerifyInfoAriaLabel: (item: RequestLog) => string
  hasStreamDiagnosticData: (item: RequestLog) => boolean
  formatStreamDiagnosticAriaLabel: (item: RequestLog) => string
  resolveModelVerifyStatus: (item: RequestLog) => string
  formatModelVerifyStatus: (item: RequestLog) => string
  httpCodeClass: (code: number) => string
  formatStream: (value?: boolean | number) => string
  durationColor: (value?: number) => string
  formatDuration: (value?: number) => string
  formatFirstTokenDuration: (item: RequestLog) => string
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
  scheduleShowStreamInfoTooltip: (item: RequestLog, event: MouseEvent) => void
  showStreamInfoTooltip: (item: RequestLog, event: TooltipPointerEvent) => void
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

const resolvePricingModel = (item: RequestLog) =>
  item.effective_pricing_model?.trim() || item.matched_pricing_model?.trim() || item.model?.trim() || '—'

const resolveModelTrail = (item: RequestLog) => {
  const pricingModel = resolvePricingModel(item)
  const route = [item.requested_model, item.mapped_model, item.response_model, item.model]
    .map((value) => value?.trim() || '')
    .filter((value, index, values) => value && value !== pricingModel && values.indexOf(value) === index)
  return route.join(' → ')
}

const isSessionLog = isSessionRequestLog

const sourceLabel = (item: RequestLog) => {
  if (item.data_source === 'session_log') return t('components.logs.source.claudeSession')
  if (item.data_source === 'codex_session') return t('components.logs.source.codexSession')
  if (item.data_source === 'gemini_session') return t('components.logs.source.geminiSession')
  return t('components.logs.source.proxy')
}
</script>

<style scoped>
.source-tag {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  border: 1px solid color-mix(in srgb, var(--mac-border) 82%, transparent);
  border-radius: 6px;
  padding: 1px 7px;
  background: color-mix(in srgb, var(--mac-surface-strong) 78%, transparent);
  color: var(--mac-text-secondary);
  font-size: 0.72rem;
  font-weight: 650;
  white-space: nowrap;
}

.performance-trigger:disabled {
  cursor: default;
  opacity: 0.58;
}

.model-route-trail {
  display: block;
  max-width: 220px;
  margin-top: 3px;
  overflow: hidden;
  color: var(--mac-text-secondary);
  font-size: 0.7rem;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.http-diagnostic-trigger {
  display: inline-flex;
  cursor: help;
  border-radius: 6px;
  padding: 1px 4px;
  margin: -1px -4px;
}

.http-diagnostic-trigger:hover {
  background: rgba(59, 130, 246, 0.12);
}

.http-diagnostic-trigger:focus-visible {
  outline: 2px solid rgba(59, 130, 246, 0.52);
  outline-offset: 1px;
}

:global(html.dark) .http-diagnostic-trigger:hover {
  background: rgba(59, 130, 246, 0.24);
}

:global(html.dark) .http-diagnostic-trigger:focus-visible {
  outline-color: rgba(96, 165, 250, 0.7);
}

.reasoning-effort {
  margin-left: 0.35rem;
  flex: 0 0 auto;
  font-weight: 700;
}

.reasoning-effort--low {
  color: #f5c344;
}

.reasoning-effort--medium {
  color: #6cb86e;
}

.reasoning-effort--high {
  color: #f97316;
}

.reasoning-effort--xhigh {
  color: #ef4444;
}

.reasoning-effort--max {
  color: #442082;
}

.reasoning-effort--unknown {
  color: #94a3b8;
}
</style>
