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
        <h4 class="provider-concurrency-modal__section-title">{{ sectionTitle }}</h4>
        <div
          v-if="sectionState"
          class="provider-concurrency-modal__state"
          :class="{ 'provider-concurrency-modal__state--error': sectionState.isError }"
        >
          {{ sectionState.message }}
        </div>
        <article
          v-for="entry in displayEntries"
          v-else
          :key="entry.id"
          class="provider-concurrency-row"
          :class="{ 'provider-concurrency-row--switchable': isEntrySwitchable(entry) }"
          :role="isEntrySwitchable(entry) ? 'button' : undefined"
          :tabindex="isEntrySwitchable(entry) ? 0 : undefined"
          @click="handleEntryClick(entry)"
          @keydown.enter.prevent="handleEntryClick(entry)"
          @keydown.space.prevent="handleEntryClick(entry)"
        >
          <div
            class="provider-concurrency-row__main"
            :class="{ 'provider-concurrency-row__main--route': entry.modelRows.length > 1 }"
          >
            <div
              class="provider-concurrency-row__models"
              :class="{ 'provider-concurrency-row__models--single': entry.modelRows.length === 1 }"
            >
              <div
                v-for="modelRow in entry.modelRows"
                :key="modelRow.key"
                class="provider-concurrency-row__model-line"
                :class="{ 'provider-concurrency-row__model-line--single': entry.modelRows.length === 1 }"
              >
                <span v-if="entry.modelRows.length > 1" class="provider-concurrency-row__model-label">
                  {{ modelRow.key === 'requested' ? t('components.main.concurrencyDetails.requestedModel') : t('components.main.concurrencyDetails.actualModel') }}
                </span>
                <span
                  class="provider-concurrency-row__route-trigger provider-concurrency-row__model-details"
                  tabindex="0"
                  :aria-label="modelRowAriaLabel(modelRow, entry.modelRows.length > 1)"
                  @mouseenter="showRouteTooltip($event, modelRow.tooltipLines)"
                  @mouseleave="hideRouteTooltip($event)"
                  @focus="showRouteTooltip($event, modelRow.tooltipLines)"
                  @blur="hideRouteTooltip($event)"
                  @keydown.esc="hideRouteTooltip()"
                >
                  <strong v-if="modelRow.emphasized">{{ modelRow.model }}</strong>
                  <span v-else class="provider-concurrency-row__model-value">{{ modelRow.model }}</span>
                  <span class="provider-concurrency-row__parameters">
                    <span
                      v-for="key in connectionParameterKeys"
                      :key="key"
                      :class="['provider-parameter-chip', `provider-parameter-chip--${parameterTone(modelRow.parameters, key, modelRow.stage)}`]"
                    >
                      <span class="provider-parameter-chip__label">{{ parameterShortLabel(key) }}</span>
                      <span class="provider-parameter-chip__value">{{ parameterValue(modelRow.parameters, key, modelRow.stage) }}</span>
                    </span>
                  </span>
                </span>
              </div>
            </div>
            <span class="provider-concurrency-row__context">{{ entry.context }}</span>
          </div>
          <div class="provider-concurrency-row__meta">
            <template v-for="item in entry.meta" :key="item.key">
              <time v-if="item.dateTime" :datetime="item.dateTime">{{ item.label }}</time>
              <span v-else>{{ item.label }}</span>
            </template>
          </div>
          <p class="provider-concurrency-row__ua">{{ entry.userAgent }}</p>
          <div v-if="entry.sessionNumber && entry.sessionRole" class="provider-concurrency-row__session">
            <span>{{ t('components.main.concurrencyDetails.sessionNumber', { number: entry.sessionNumber }) }}</span>
            <span v-if="entry.sessionRole === 'child'">{{ entry.parentSessionNumber ? t('components.main.concurrencyDetails.parentSession', { number: entry.parentSessionNumber }) : t('components.main.concurrencyDetails.inheritedSession') }}</span>
            <span v-else>{{ t('components.main.concurrencyDetails.rootSession') }}</span>
          </div>
        </article>
      </section>

      <InlineModal
        :open="switchPanelVisible"
        :title="t('components.main.concurrencyDetails.switchCandidates')"
        :panel-width="'min(620px, 92vw)'"
        :panel-class="isDarkTheme ? 'provider-session-switch-modal provider-session-switch-modal--dark' : 'provider-session-switch-modal'"
        @close="closeSwitchPanel"
      >
        <div class="provider-concurrency-switch-panel">
          <p v-if="!props.sessionAffinityEnabled" class="provider-concurrency-modal__state">{{ t('components.main.concurrencyDetails.stickyDisabled') }}</p>
          <p v-else-if="switchLoading" class="provider-concurrency-modal__state">{{ t('components.main.concurrencyDetails.loadingCandidates') }}</p>
          <p v-else-if="selectedSessionNumber === 0" class="provider-concurrency-modal__state">{{ t('components.main.concurrencyDetails.sessionUnavailable') }}</p>
          <div v-else class="provider-concurrency-switch-panel__grid">
            <button
              v-for="candidate in selectedSessionCandidates"
              :key="candidate.providerId"
              type="button"
              class="provider-concurrency-switch-option"
              :class="{ 'is-unavailable': candidate.current || !candidate.available }"
              :disabled="switchLoading || candidate.current || !candidate.switchable || !candidate.available"
              @click="selectedSessionNumber !== null && emit('switch-session-provider', selectedSessionNumber, candidate.providerId)"
            >
              <span>{{ candidate.providerName }}</span>
              <small v-if="candidate.current">{{ t('components.main.concurrencyDetails.currentProvider') }}</small>
              <small v-else-if="candidate.available">L{{ candidate.level }} · {{ candidate.boundSessions }}/{{ candidate.maxSessions }}</small>
              <small v-else>{{ candidate.reason || t('components.main.concurrencyDetails.switchUnavailable') }}</small>
            </button>
          </div>
        </div>
      </InlineModal>
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
import type {
  ProviderConcurrencyRequestParameterView,
  ProviderConcurrencyRequestView,
  ProviderConcurrencyStatusView,
  ProviderSessionSwitchCandidateView,
  ResolvedTheme,
} from '../types'
import {
  buildConnectionModelRows,
  buildModelRouteAriaLabel,
  buildModelRouteTooltipLines,
  connectionParameterKeys,
  connectionParameterTone,
  connectionParameterValue,
  type ConnectionParameterKey,
  type ConnectionParameterSnapshot,
  type ConnectionParameterStage,
  type ConnectionModelRow,
  type ModelRouteSnapshot,
} from './providerModelRoute'

const HISTORY_LIMIT = 10
type ConcurrencyDetailsTab = 'active' | 'history'
type ConnectionDisplayMetaItem = {
  key: string
  label: string
  dateTime?: string
}
type ConnectionDisplayEntry = {
  id: string | number
  modelRows: ConnectionModelRow[]
  context: string
  meta: ConnectionDisplayMetaItem[]
  userAgent: string
  sessionNumber?: number
  sessionSwitchable?: boolean
  sessionRole?: string
  parentSessionNumber?: number
  inherited?: boolean
}

const props = defineProps<{
  open: boolean
  provider: AutomationCard | null
  platform: RequestLogPlatform | null
  status: ProviderConcurrencyStatusView | null
  resolvedTheme: ResolvedTheme
  showModelRouteDetails: boolean
  sessionAffinityEnabled?: boolean
  switchCandidates?: ProviderSessionSwitchCandidateView[]
  switchLoading?: boolean
  switchCompletedToken?: number
}>()

const emit = defineEmits<{
  close: []
  'request-session-switch': [sessionNumber: number]
  'switch-session-provider': [sessionNumber: number, providerId: string]
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
const selectedSessionNumber = ref<number | null>(null)
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
const sectionTitle = computed(() => activeTab.value === 'active'
  ? t('components.main.concurrencyDetails.activeTitle')
  : t('components.main.concurrencyDetails.historyTitle'))
const sectionState = computed(() => {
  if (activeTab.value === 'active') {
    return activeRequests.value.length === 0
      ? { message: t('components.main.concurrencyDetails.emptyActive'), isError: false }
      : null
  }
  if (historyLoading.value) {
    return { message: t('components.main.concurrencyDetails.loadingHistory'), isError: false }
  }
  if (historyError.value) {
    return {
      message: t('components.main.concurrencyDetails.loadHistoryFailed', { error: historyError.value }),
      isError: true,
    }
  }
  return historyLogs.value.length === 0
    ? { message: t('components.main.concurrencyDetails.emptyHistory'), isError: false }
    : null
})
const displayEntries = computed<ConnectionDisplayEntry[]>(() => {
  if (activeTab.value === 'active') {
    return activeRequests.value.map((request) => {
      const parameters = activeConnectionParameters(request)
      return {
        id: request.id,
        modelRows: buildConnectionModelRows({
          showModelRouteDetails: props.showModelRouteDetails,
          requestedModel: activeRequestedModel(request),
          actualModel: activeActualModel(request),
          parameters,
          actualRouteLines: activeRouteTooltipLines(request),
        }, t),
        context: request.endpoint || '-',
        meta: [
          {
            key: 'stream',
            label: request.isStream
              ? t('components.main.concurrencyDetails.stream')
              : t('components.main.concurrencyDetails.nonStream'),
          },
          { key: 'duration', label: formatDurationMs(request.durationMs) },
        ],
        userAgent: request.userAgent || t('components.main.concurrencyDetails.emptyUserAgent'),
        sessionNumber: request.sessionNumber,
        sessionSwitchable: request.sessionSwitchable,
        sessionRole: request.sessionRole,
        parentSessionNumber: request.parentSessionNumber,
      }
    })
  }

  return historyLogs.value.map((log) => {
    const parameters = historyConnectionParameters(log)
    return {
      id: log.id,
      modelRows: buildConnectionModelRows({
        showModelRouteDetails: props.showModelRouteDetails,
        requestedModel: historyRequestedModel(log),
        actualModel: historyActualModel(log),
        parameters,
        actualRouteLines: historyRouteTooltipLines(log),
      }, t),
      context: `HTTP ${log.http_code || 0}`,
      meta: [
        {
          key: 'stream',
          label: log.is_stream
            ? t('components.main.concurrencyDetails.stream')
            : t('components.main.concurrencyDetails.nonStream'),
        },
        { key: 'duration', label: formatDurationSec(log.duration_sec) },
        { key: 'created-at', label: formatDateTime(log.created_at), dateTime: log.created_at },
      ],
      userAgent: log.user_agent || t('components.main.concurrencyDetails.emptyUserAgent'),
    }
  })
})

const switchPanelVisible = computed(() => selectedSessionNumber.value !== null)
const selectedSessionCandidates = computed(() => props.switchCandidates ?? [])

function handleEntryClick(entry: ConnectionDisplayEntry) {
  if (activeTab.value !== 'active' || !isEntrySwitchable(entry)) return
  selectedSessionNumber.value = entry.sessionNumber ?? 0
  emit('request-session-switch', entry.sessionNumber ?? 0)
}

function isEntrySwitchable(entry: ConnectionDisplayEntry) {
  return activeTab.value === 'active' && (entry.sessionSwitchable === true || props.sessionAffinityEnabled !== true)
}

function closeSwitchPanel() {
  selectedSessionNumber.value = null
}

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

function activeConnectionParameters(request: ProviderConcurrencyRequestView): ConnectionParameterSnapshot[] {
  return Array.isArray(request.parameters)
    ? request.parameters as ProviderConcurrencyRequestParameterView[]
    : []
}

function historyConnectionParameters(log: RequestLog): ConnectionParameterSnapshot[] {
  return [
    {
      key: 'reasoning_effort',
      requestedValue: '',
      actualValue: log.reasoning_effort || '',
      source: log.reasoning_effort_source || '',
    },
    {
      key: 'max_output_tokens',
      requestedValue: '',
      actualValue: '',
      source: '',
    },
  ]
}

function parameterShortLabel(key: ConnectionParameterKey) {
  return key === 'reasoning_effort'
    ? t('components.main.concurrencyDetails.parameterReasoningEffortShort')
    : t('components.main.concurrencyDetails.parameterMaxOutputTokensShort')
}

function parameterValue(
  parameters: ConnectionParameterSnapshot[],
  key: ConnectionParameterKey,
  stage: ConnectionParameterStage,
) {
  return connectionParameterValue(parameters, key, stage)
}

function parameterTone(
  parameters: ConnectionParameterSnapshot[],
  key: ConnectionParameterKey,
  stage: ConnectionParameterStage,
) {
  const parameter = parameters.find((item) => item.key === key)
  const value = stage === 'requested' ? parameter?.requestedValue : parameter?.actualValue
  return connectionParameterTone(key, value)
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
    sessionPreferredProvider: request.sessionPreferredProvider,
    sessionProviderRoute: request.sessionProviderRoute,
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
    sessionPreferredProvider: log.session_preferred_provider,
    sessionProviderRoute: log.session_provider_route,
  }, t('components.main.concurrencyDetails.routeUnavailable'))
}

function modelRowAriaLabel(row: ConnectionModelRow, isDual: boolean) {
  if (isDual && row.key === 'actual') {
    return buildModelRouteAriaLabel(row.model, row.tooltipLines, t)
  }
  return t('components.main.concurrencyDetails.modelDetailsAria', {
    model: row.model,
    details: row.tooltipLines.join('; '),
  })
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
    if (!open) {
      hideRouteTooltip()
      selectedSessionNumber.value = null
    }
  },
)

watch(
  () => props.switchCompletedToken,
  (token, previousToken) => {
    if (token !== undefined && token !== previousToken) {
      closeSwitchPanel()
    }
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

.provider-concurrency-row--switchable {
  cursor: pointer;
}

.provider-concurrency-row--switchable:hover {
  border-color: rgba(20, 184, 166, 0.56);
}

.provider-concurrency-row__session {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  color: #0f766e;
  font-size: 12px;
  font-weight: 700;
}

.provider-concurrency-switch-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border: 1px solid rgba(20, 184, 166, 0.3);
  border-radius: 14px;
  background: rgba(20, 184, 166, 0.06);
}

.provider-concurrency-switch-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.provider-concurrency-switch-panel__close {
  border: 0;
  color: inherit;
  background: transparent;
  font-size: 20px;
  cursor: pointer;
}

.provider-concurrency-switch-panel__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 8px;
}

.provider-concurrency-switch-option {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  min-height: 54px;
  padding: 10px 12px;
  border: 1px solid rgba(20, 184, 166, 0.28);
  border-radius: 10px;
  color: #0f172a;
  background: rgba(255, 255, 255, 0.82);
  text-align: left;
  cursor: pointer;
}

.provider-session-switch-modal--dark .provider-concurrency-modal__state,
.provider-session-switch-modal--dark .provider-concurrency-switch-option {
  color: #f8fafc;
  background: rgba(15, 23, 42, 0.76);
}

.provider-concurrency-switch-option.is-unavailable {
  opacity: 0.48;
  cursor: not-allowed;
}

.provider-concurrency-switch-option small {
  color: #64748b;
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

.provider-concurrency-row__models--single {
  flex: 0 1 auto;
}

.provider-concurrency-row__model-line {
  display: grid;
  grid-template-columns: minmax(56px, auto) minmax(0, 1fr);
  align-items: baseline;
  gap: 10px;
}

.provider-concurrency-row__model-line--single {
  display: flex;
  min-width: 0;
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

.provider-concurrency-row__model-details {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  width: fit-content;
  max-width: 100%;
  gap: 7px;
}

.provider-concurrency-row__parameters {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  min-width: 0;
  gap: 5px;
}

.provider-parameter-chip {
  display: inline-grid;
  grid-template-columns: auto minmax(0, auto);
  align-items: center;
  max-width: 150px;
  min-height: 22px;
  padding: 2px 6px;
  border: 1px solid rgba(100, 116, 139, 0.24);
  border-radius: 5px;
  color: #334155;
  background: rgba(241, 245, 249, 0.94);
  font-size: 10px;
  font-weight: 700;
  line-height: 1.2;
}

.provider-parameter-chip__label {
  margin-right: 4px;
  color: #64748b;
  font-weight: 600;
}

.provider-parameter-chip__value {
  min-width: 0;
  max-width: 84px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-parameter-chip--low {
  color: #166534;
  border-color: rgba(34, 197, 94, 0.3);
  background: rgba(220, 252, 231, 0.72);
}

.provider-parameter-chip--medium,
.provider-parameter-chip--output {
  color: #1d4ed8;
  border-color: rgba(59, 130, 246, 0.28);
  background: rgba(219, 234, 254, 0.72);
}

.provider-parameter-chip--high {
  color: #a16207;
  border-color: rgba(234, 179, 8, 0.32);
  background: rgba(254, 249, 195, 0.76);
}

.provider-parameter-chip--xhigh,
.provider-parameter-chip--max {
  color: #be123c;
  border-color: rgba(244, 63, 94, 0.3);
  background: rgba(255, 228, 230, 0.78);
}

.provider-parameter-chip--custom {
  color: #6d28d9;
  border-color: rgba(139, 92, 246, 0.28);
  background: rgba(237, 233, 254, 0.74);
}

.provider-parameter-chip--empty {
  color: #64748b;
  border-color: rgba(148, 163, 184, 0.24);
  background: rgba(241, 245, 249, 0.76);
}

.provider-concurrency-modal--dark .provider-parameter-chip {
  color: #cbd5e1;
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(30, 41, 59, 0.86);
}

.provider-concurrency-modal--dark .provider-parameter-chip__label {
  color: #94a3b8;
}

.provider-concurrency-modal--dark .provider-parameter-chip--low {
  color: #86efac;
  border-color: rgba(74, 222, 128, 0.28);
  background: rgba(20, 83, 45, 0.38);
}

.provider-concurrency-modal--dark .provider-parameter-chip--medium,
.provider-concurrency-modal--dark .provider-parameter-chip--output {
  color: #93c5fd;
  border-color: rgba(96, 165, 250, 0.3);
  background: rgba(30, 64, 175, 0.3);
}

.provider-concurrency-modal--dark .provider-parameter-chip--high {
  color: #fde047;
  border-color: rgba(250, 204, 21, 0.28);
  background: rgba(113, 63, 18, 0.34);
}

.provider-concurrency-modal--dark .provider-parameter-chip--xhigh,
.provider-concurrency-modal--dark .provider-parameter-chip--max {
  color: #fda4af;
  border-color: rgba(251, 113, 133, 0.3);
  background: rgba(136, 19, 55, 0.3);
}

.provider-concurrency-modal--dark .provider-parameter-chip--custom {
  color: #c4b5fd;
  border-color: rgba(167, 139, 250, 0.3);
  background: rgba(76, 29, 149, 0.3);
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
  color: #0f172a;
  background: #f8fafc;
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
  color: #e2e8f0;
  background: #0f172a;
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

@media (max-width: 640px) {
  .provider-concurrency-modal__hero {
    flex-direction: column;
  }

  .provider-concurrency-modal__stats {
    justify-content: flex-start;
  }

  .provider-concurrency-row__model-line {
    grid-template-columns: minmax(0, 1fr);
    gap: 4px;
  }

  .provider-concurrency-row__models--single {
    flex-basis: 100%;
    width: 100%;
  }

  .provider-concurrency-row__model-details,
  .provider-concurrency-row__parameters {
    width: 100%;
  }
}
</style>
