<template>
  <InlineModal
    :open="open"
    :title="modalTitle"
    :panel-width="'min(1040px, 94vw)'"
    :panel-class="modalPanelClass"
    @close="emit('close')"
  >
    <div :class="['provider-concurrency-modal', isDarkTheme ? 'provider-concurrency-modal--dark' : 'provider-concurrency-modal--light']">
      <section class="provider-concurrency-modal__hero">
        <div>
          <span class="provider-concurrency-modal__eyebrow">{{ platformLabel }}</span>
          <h3 class="provider-concurrency-modal__title">{{ providerName }}</h3>
          <p class="provider-concurrency-modal__subtitle">{{ t('components.main.concurrencyDetails.summary') }}</p>
        </div>
        <div class="provider-concurrency-modal__stats">
          <span class="provider-concurrency-modal__stat">{{ t('components.main.concurrencyDetails.activeCount', { count: activeRequests.length }) }}</span>
          <span class="provider-concurrency-modal__stat">{{ t('components.main.concurrencyDetails.historyCount', { count: historyCountDisplay }) }}</span>
        </div>
      </section>

      <div class="provider-concurrency-modal__tabs">
        <button
          type="button"
          class="provider-concurrency-modal__tab"
          :class="{ 'is-active': activeTab === 'active' }"
          :aria-pressed="activeTab === 'active'"
          @click="selectTab('active')"
        >
          {{ t('components.main.concurrencyDetails.activeTab', { count: activeRequests.length }) }}
        </button>
        <button
          type="button"
          class="provider-concurrency-modal__tab"
          :class="{ 'is-active': activeTab === 'history' }"
          :aria-pressed="activeTab === 'history'"
          @click="selectTab('history')"
        >
          {{ t('components.main.concurrencyDetails.historyTab', { count: historyCountDisplay }) }}
        </button>
      </div>

      <section class="provider-concurrency-modal__section">
        <template v-if="activeTab === 'active'">
          <h4 class="provider-concurrency-modal__section-title">{{ t('components.main.concurrencyDetails.activeTitle') }}</h4>
          <div v-if="activeRequests.length === 0" class="provider-concurrency-modal__state">
            {{ t('components.main.concurrencyDetails.emptyActive') }}
          </div>
          <article
            v-for="request in activeRequests"
            v-else
            :key="request.id"
            class="provider-concurrency-row"
          >
            <div
              class="provider-concurrency-row__main"
              :class="{ 'provider-concurrency-row__main--route': showModelRouteDetails }"
            >
              <div v-if="showModelRouteDetails" class="provider-concurrency-row__models">
                <div class="provider-concurrency-row__model-line">
                  <span class="provider-concurrency-row__model-label">{{ t('components.main.concurrencyDetails.requestedModel') }}</span>
                  <span class="provider-concurrency-row__model-value">{{ activeRequestedModel(request) }}</span>
                </div>
                <div class="provider-concurrency-row__model-line">
                  <span class="provider-concurrency-row__model-label">{{ t('components.main.concurrencyDetails.actualModel') }}</span>
                  <span
                    class="provider-concurrency-row__route-trigger"
                    tabindex="0"
                    :aria-label="routeTriggerAriaLabel(activeActualModel(request), activeRouteTooltipLines(request))"
                    @mouseenter="showRouteTooltip($event, activeRouteTooltipLines(request))"
                    @mouseleave="hideRouteTooltip($event)"
                    @focus="showRouteTooltip($event, activeRouteTooltipLines(request))"
                    @blur="hideRouteTooltip($event)"
                    @keydown.esc="hideRouteTooltip()"
                  >
                    <strong>{{ activeActualModel(request) }}</strong>
                  </span>
                </div>
              </div>
              <strong v-else>{{ displayModel(request.model) }}</strong>
              <span class="provider-concurrency-row__context">{{ request.endpoint || '-' }}</span>
            </div>
            <div class="provider-concurrency-row__meta">
              <span>{{ request.isStream ? t('components.main.concurrencyDetails.stream') : t('components.main.concurrencyDetails.nonStream') }}</span>
              <span>{{ formatDurationMs(request.durationMs) }}</span>
            </div>
            <p class="provider-concurrency-row__ua">{{ request.userAgent || t('components.main.concurrencyDetails.emptyUserAgent') }}</p>
          </article>
        </template>

        <template v-else>
          <h4 class="provider-concurrency-modal__section-title">{{ t('components.main.concurrencyDetails.historyTitle') }}</h4>
          <div v-if="historyLoading" class="provider-concurrency-modal__state">
            {{ t('components.main.concurrencyDetails.loadingHistory') }}
          </div>
          <div v-else-if="historyError" class="provider-concurrency-modal__state provider-concurrency-modal__state--error">
            {{ t('components.main.concurrencyDetails.loadHistoryFailed', { error: historyError }) }}
          </div>
          <div v-else-if="historyLogs.length === 0" class="provider-concurrency-modal__state">
            {{ t('components.main.concurrencyDetails.emptyHistory') }}
          </div>
          <article
            v-for="log in historyLogs"
            v-else
            :key="log.id"
            class="provider-concurrency-row"
          >
            <div
              class="provider-concurrency-row__main"
              :class="{ 'provider-concurrency-row__main--route': showModelRouteDetails }"
            >
              <div v-if="showModelRouteDetails" class="provider-concurrency-row__models">
                <div class="provider-concurrency-row__model-line">
                  <span class="provider-concurrency-row__model-label">{{ t('components.main.concurrencyDetails.requestedModel') }}</span>
                  <span class="provider-concurrency-row__model-value">{{ historyRequestedModel(log) }}</span>
                </div>
                <div class="provider-concurrency-row__model-line">
                  <span class="provider-concurrency-row__model-label">{{ t('components.main.concurrencyDetails.actualModel') }}</span>
                  <span
                    class="provider-concurrency-row__route-trigger"
                    tabindex="0"
                    :aria-label="routeTriggerAriaLabel(historyActualModel(log), historyRouteTooltipLines(log))"
                    @mouseenter="showRouteTooltip($event, historyRouteTooltipLines(log))"
                    @mouseleave="hideRouteTooltip($event)"
                    @focus="showRouteTooltip($event, historyRouteTooltipLines(log))"
                    @blur="hideRouteTooltip($event)"
                    @keydown.esc="hideRouteTooltip()"
                  >
                    <strong>{{ historyActualModel(log) }}</strong>
                  </span>
                </div>
              </div>
              <strong v-else>{{ displayModel(log.requested_model || log.model || log.response_model) }}</strong>
              <span class="provider-concurrency-row__context">HTTP {{ log.http_code || 0 }}</span>
            </div>
            <div class="provider-concurrency-row__meta">
              <span>{{ log.is_stream ? t('components.main.concurrencyDetails.stream') : t('components.main.concurrencyDetails.nonStream') }}</span>
              <span>{{ formatDurationSec(log.duration_sec) }}</span>
              <time :datetime="log.created_at">{{ formatDateTime(log.created_at) }}</time>
            </div>
            <p class="provider-concurrency-row__ua">{{ log.user_agent || t('components.main.concurrencyDetails.emptyUserAgent') }}</p>
          </article>
        </template>
      </section>
    </div>

    <Teleport to="body">
      <div
        v-if="routeTooltip.visible"
        ref="routeTooltipRef"
        class="provider-model-route-tooltip"
        :class="{ 'provider-model-route-tooltip--dark': isDarkTheme }"
        :style="routeTooltipStyle"
        role="tooltip"
      >
        <span
          v-for="(line, index) in routeTooltip.lines"
          :key="index"
          class="provider-model-route-tooltip__line"
        >{{ line }}</span>
      </div>
    </Teleport>
  </InlineModal>
</template>

<script setup lang="ts">
/**
 * @name: 供应商并发连接详情弹窗
 * @Descripttion: 展示供应商当前活跃连接与最近连接历史。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-01 17:16:36
 * @LastEditTime: 2026-07-01 17:16:36
 * @FilePath: frontend/src/components/Main/modals/ProviderConcurrencyDetailsModal.vue
 */
import { computed, nextTick, onBeforeUnmount, ref, watch, type CSSProperties } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AutomationCard } from '../../../data/cards'
import { fetchRequestLogsPage, type RequestLog, type RequestLogPlatform } from '../../../services/logs'
import { extractErrorMessage } from '../../../utils/error'
import InlineModal from '../../common/InlineModal.vue'
import { cardProviderRef } from '../adapters/providerCardMappers'
import type { ProviderConcurrencyRequestView, ProviderConcurrencyStatusView, ResolvedTheme } from '../types'
import {
  buildModelRouteAriaLabel,
  buildModelRouteTooltipLines,
  type ModelRouteSnapshot,
} from './providerModelRoute'

const HISTORY_LIMIT = 10
type ConcurrencyDetailsTab = 'active' | 'history'

const props = defineProps<{
  open: boolean
  provider: AutomationCard | null
  platform: RequestLogPlatform | null
  status: ProviderConcurrencyStatusView | null
  resolvedTheme: ResolvedTheme
  showModelRouteDetails: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const { t, locale } = useI18n()

const historyLoading = ref(false)
const historyError = ref('')
const historyLogs = ref<RequestLog[]>([])
const historyLoaded = ref(false)
const activeTab = ref<ConcurrencyDetailsTab>('active')
const routeTooltipRef = ref<HTMLElement | null>(null)
const routeTooltip = ref({
  visible: false,
  positioned: false,
  left: 0,
  top: 0,
  lines: [] as string[],
})
let activeLoadRequestId = 0
let routeTooltipRequestId = 0
let routeTooltipViewportListenersActive = false

const isDarkTheme = computed(() => props.resolvedTheme === 'dark')
const providerName = computed(() => props.provider?.name || props.status?.providerName || '')
const providerRef = computed(() => props.provider ? cardProviderRef(props.provider) : props.status?.providerId || '')
const activeRequests = computed(() => props.status?.requests ?? [])
const historyCountDisplay = computed(() => {
  if (historyLoading.value) return '…'
  return historyLoaded.value ? `${historyLogs.value.length}` : '-'
})
const modalTitle = computed(() => t('components.main.concurrencyDetails.modalTitle', { name: providerName.value || t('components.main.concurrencyDetails.providerFallback') }))
const modalPanelClass = computed(() => ({
  'provider-concurrency-modal-shell': true,
  'provider-concurrency-modal-shell--dark': isDarkTheme.value,
}))
const routeTooltipStyle = computed<CSSProperties>(() => ({
  left: `${routeTooltip.value.left}px`,
  top: `${routeTooltip.value.top}px`,
  visibility: routeTooltip.value.positioned ? 'visible' : 'hidden',
}))
const platformLabel = computed(() => {
  switch (props.platform) {
    case 'claude':
      return 'Claude Code'
    case 'codex':
      return 'Codex'
    case 'gemini':
      return 'Gemini CLI'
    default:
      return 'Provider'
  }
})

const dateTimeFormatter = computed(() => new Intl.DateTimeFormat(locale.value || 'en', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
}))

function displayModel(value?: string) {
  return `${value ?? ''}`.trim() || '-'
}

function activeRequestedModel(request: ProviderConcurrencyRequestView) {
  return displayModel(request.requestedModel || request.model)
}

function activeActualModel(request: ProviderConcurrencyRequestView) {
  return displayModel(request.model || request.requestedModel)
}

function historyRequestedModel(log: RequestLog) {
  return displayModel(log.requested_model || log.model || log.response_model)
}

function historyActualModel(log: RequestLog) {
  return displayModel(log.model || log.response_model || log.requested_model)
}

function modelRouteTooltipLines(route: ModelRouteSnapshot, unavailableMessage: string) {
  return buildModelRouteTooltipLines(route, unavailableMessage, t)
}

function activeRouteTooltipLines(request: ProviderConcurrencyRequestView) {
  return modelRouteTooltipLines({
    requestedModel: request.requestedModel || request.model,
    mappedModel: request.mappedModel,
    modelMappingPattern: request.modelMappingPattern,
    modelMappingTarget: request.modelMappingTarget,
    modelOverride: request.modelOverride,
    modelRouteCaptured: request.modelRouteCaptured,
  }, t('components.main.concurrencyDetails.activeRouteUnavailable'))
}

function historyRouteTooltipLines(log: RequestLog) {
  return modelRouteTooltipLines({
    requestedModel: log.requested_model || log.model || log.response_model,
    mappedModel: log.mapped_model,
    modelMappingPattern: log.model_mapping_pattern,
    modelMappingTarget: log.model_mapping_target,
    modelOverride: log.model_override,
    modelRouteCaptured: log.model_route_captured,
  }, t('components.main.concurrencyDetails.routeUnavailable'))
}

function routeTriggerAriaLabel(actualModel: string, lines: string[]) {
  return buildModelRouteAriaLabel(actualModel, lines, t)
}

function showRouteTooltip(event: MouseEvent | FocusEvent, lines: string[]) {
  const target = event.currentTarget
  if (!(target instanceof HTMLElement)) return

  const requestId = ++routeTooltipRequestId
  const anchorRect = target.getBoundingClientRect()
  routeTooltip.value = {
    visible: true,
    positioned: false,
    left: anchorRect.left,
    top: anchorRect.bottom + 8,
    lines,
  }

  void nextTick(() => {
    if (requestId !== routeTooltipRequestId || !routeTooltip.value.visible) return
    const tooltipRect = routeTooltipRef.value?.getBoundingClientRect()
    if (!tooltipRect) return

    const viewportMargin = 12
    const preferredLeft = Math.min(
      Math.max(anchorRect.left, viewportMargin),
      Math.max(viewportMargin, window.innerWidth - tooltipRect.width - viewportMargin),
    )
    const belowTop = anchorRect.bottom + 8
    const aboveTop = anchorRect.top - tooltipRect.height - 8
    const preferredTop = belowTop + tooltipRect.height <= window.innerHeight - viewportMargin
      ? belowTop
      : Math.max(viewportMargin, aboveTop)

    routeTooltip.value = {
      ...routeTooltip.value,
      positioned: true,
      left: preferredLeft,
      top: preferredTop,
    }
  })
}

function hideRouteTooltip(event?: MouseEvent | FocusEvent) {
  if (event?.type === 'mouseleave' && document.activeElement === event.currentTarget) {
    return
  }
  routeTooltipRequestId += 1
  routeTooltip.value.visible = false
}

function hideRouteTooltipOnViewportChange() {
  hideRouteTooltip()
}

function setRouteTooltipViewportListeners(active: boolean) {
  if (routeTooltipViewportListenersActive === active) return
  routeTooltipViewportListenersActive = active
  if (active) {
    window.addEventListener('resize', hideRouteTooltipOnViewportChange)
    window.addEventListener('scroll', hideRouteTooltipOnViewportChange, true)
    return
  }
  window.removeEventListener('resize', hideRouteTooltipOnViewportChange)
  window.removeEventListener('scroll', hideRouteTooltipOnViewportChange, true)
}

function formatDurationMs(value?: number) {
  const ms = Number(value)
  if (!Number.isFinite(ms) || ms <= 0) return t('components.main.concurrencyDetails.durationNow')
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

function formatDurationSec(value?: number) {
  const sec = Number(value)
  if (!Number.isFinite(sec) || sec <= 0) return '-'
  return `${sec.toFixed(2)}s`
}

function formatDateTime(value?: string) {
  const raw = `${value ?? ''}`.trim()
  if (!raw) return '-'
  const date = new Date(raw.includes('T') ? raw : raw.replace(' ', 'T'))
  if (Number.isNaN(date.getTime())) return raw
  return dateTimeFormatter.value.format(date)
}

function resetHistoryState() {
  activeLoadRequestId += 1
  historyLoading.value = false
  historyError.value = ''
  historyLogs.value = []
  historyLoaded.value = false
}

function selectTab(tab: ConcurrencyDetailsTab) {
  activeTab.value = tab
}

async function loadHistory() {
  if (!props.open || activeTab.value !== 'history' || !props.platform || !providerRef.value) {
    return
  }
  const requestId = ++activeLoadRequestId
  historyLoading.value = true
  historyError.value = ''
  try {
    const page = await fetchRequestLogsPage({
      platform: props.platform,
      provider: providerRef.value,
      limit: HISTORY_LIMIT,
      offset: 0,
    })
    if (requestId !== activeLoadRequestId) return
    historyLogs.value = Array.isArray(page.items) ? page.items : []
    historyLoaded.value = true
  } catch (error) {
    if (requestId !== activeLoadRequestId) return
    historyError.value = extractErrorMessage(error)
    historyLogs.value = []
    historyLoaded.value = false
  } finally {
    if (requestId === activeLoadRequestId) {
      historyLoading.value = false
    }
  }
}

watch(
  () => [props.open, props.platform, providerRef.value],
  () => {
    activeTab.value = 'active'
    resetHistoryState()
  },
  { immediate: true },
)

watch(
  () => [props.open, activeTab.value, props.platform, providerRef.value],
  () => {
    if (activeTab.value === 'history') {
      void loadHistory()
    }
  },
  { immediate: true },
)

watch(
  () => props.open,
  (open) => {
    if (!open) hideRouteTooltip()
  },
)

watch(
  () => routeTooltip.value.visible,
  setRouteTooltipViewportListeners,
)

onBeforeUnmount(() => {
  setRouteTooltipViewportListeners(false)
})
</script>

<style scoped>
.provider-concurrency-modal {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.provider-concurrency-modal__hero {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 18px;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 22px;
  background: rgba(248, 250, 252, 0.82);
}

.provider-concurrency-modal--dark .provider-concurrency-modal__hero {
  background: rgba(15, 23, 42, 0.72);
  border-color: rgba(148, 163, 184, 0.18);
}

.provider-concurrency-modal__eyebrow {
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
}

.provider-concurrency-modal__title {
  margin: 4px 0;
  color: #0f172a;
  font-size: 22px;
}

.provider-concurrency-modal--dark .provider-concurrency-modal__title {
  color: #f8fafc;
}

.provider-concurrency-modal__subtitle {
  margin: 0;
  color: #64748b;
  font-size: 13px;
}

.provider-concurrency-modal__stats {
  display: flex;
  flex-wrap: wrap;
  align-content: flex-start;
  justify-content: flex-end;
  gap: 8px;
}

.provider-concurrency-modal__stat {
  padding: 6px 10px;
  border-radius: 999px;
  color: #0f766e;
  background: rgba(20, 184, 166, 0.1);
  font-size: 12px;
  font-weight: 700;
}

.provider-concurrency-modal__tabs {
  display: inline-flex;
  align-self: flex-start;
  gap: 6px;
  padding: 4px;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.1);
}

.provider-concurrency-modal--dark .provider-concurrency-modal__tabs {
  border-color: rgba(148, 163, 184, 0.16);
  background: rgba(15, 23, 42, 0.66);
}

.provider-concurrency-modal__tab {
  border: 0;
  border-radius: 999px;
  padding: 8px 14px;
  color: #64748b;
  background: transparent;
  font-size: 13px;
  font-weight: 800;
  cursor: pointer;
}

.provider-concurrency-modal__tab:hover,
.provider-concurrency-modal__tab.is-active {
  color: #0f172a;
  background: rgba(255, 255, 255, 0.86);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.08);
}

.provider-concurrency-modal--dark .provider-concurrency-modal__tab:hover,
.provider-concurrency-modal--dark .provider-concurrency-modal__tab.is-active {
  color: #f8fafc;
  background: rgba(30, 41, 59, 0.9);
  box-shadow: none;
}

.provider-concurrency-modal__section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.provider-concurrency-modal__section-title {
  margin: 0;
  color: #0f172a;
  font-size: 15px;
}

.provider-concurrency-modal--dark .provider-concurrency-modal__section-title {
  color: #f8fafc;
}

.provider-concurrency-modal__state {
  padding: 18px;
  border: 1px dashed rgba(148, 163, 184, 0.32);
  border-radius: 16px;
  color: #64748b;
  text-align: center;
}

.provider-concurrency-modal__state--error {
  color: #b91c1c;
  border-color: rgba(248, 113, 113, 0.32);
  background: rgba(254, 242, 242, 0.72);
}

.provider-concurrency-row {
  display: grid;
  gap: 8px;
  padding: 14px;
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.72);
}

.provider-concurrency-modal--dark .provider-concurrency-row {
  background: rgba(15, 23, 42, 0.58);
  border-color: rgba(148, 163, 184, 0.16);
}

.provider-concurrency-row__main,
.provider-concurrency-row__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.provider-concurrency-row__main--route {
  justify-content: space-between;
}

.provider-concurrency-row__models {
  display: grid;
  flex: 1 1 520px;
  min-width: 0;
  max-width: 100%;
  gap: 6px;
}

.provider-concurrency-row__model-line {
  display: grid;
  grid-template-columns: minmax(56px, auto) minmax(0, 1fr);
  align-items: baseline;
  gap: 10px;
}

.provider-concurrency-row__model-label {
  color: #64748b;
  font-size: 11px;
  font-weight: 700;
}

.provider-concurrency-row__model-value,
.provider-concurrency-row__route-trigger strong {
  color: #0f172a;
  overflow-wrap: anywhere;
}

.provider-concurrency-modal--dark .provider-concurrency-row__model-value,
.provider-concurrency-modal--dark .provider-concurrency-row__route-trigger strong {
  color: #f8fafc;
}

.provider-concurrency-row__route-trigger {
  min-width: 0;
  border-radius: 6px;
  outline: none;
  cursor: help;
}

.provider-concurrency-row__route-trigger:focus-visible {
  box-shadow: 0 0 0 2px rgba(20, 184, 166, 0.35);
}

.provider-model-route-tooltip {
  position: fixed;
  z-index: 10000;
  display: grid;
  width: max-content;
  max-width: min(440px, 72vw);
  gap: 6px;
  padding: 10px 12px;
  border: 1px solid rgba(15, 23, 42, 0.12);
  border-radius: 10px;
  color: #e2e8f0;
  background: #0f172a;
  box-shadow: 0 12px 30px rgba(15, 23, 42, 0.2);
  font-size: 12px;
  font-weight: 500;
  line-height: 1.45;
  pointer-events: none;
}

.provider-model-route-tooltip__line {
  overflow-wrap: anywhere;
}

.provider-model-route-tooltip--dark {
  border-color: rgba(148, 163, 184, 0.24);
  color: #0f172a;
  background: #f8fafc;
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.32);
}

.provider-concurrency-row__main strong {
  color: #0f172a;
}

.provider-concurrency-modal--dark .provider-concurrency-row__main strong {
  color: #f8fafc;
}

.provider-concurrency-row__context,
.provider-concurrency-row__meta {
  color: #64748b;
  font-size: 12px;
}

.provider-concurrency-row__ua {
  margin: 0;
  padding: 10px;
  border-radius: 12px;
  color: #334155;
  background: rgba(148, 163, 184, 0.1);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
  font-size: 12px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.provider-concurrency-modal--dark .provider-concurrency-row__ua {
  color: #cbd5e1;
  background: rgba(30, 41, 59, 0.82);
}
</style>
