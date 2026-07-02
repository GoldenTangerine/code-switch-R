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
            <div class="provider-concurrency-row__main">
              <strong>{{ displayModel(request.model) }}</strong>
              <span>{{ request.endpoint || '-' }}</span>
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
            <div class="provider-concurrency-row__main">
              <strong>{{ displayModel(log.requested_model || log.model || log.response_model) }}</strong>
              <span>HTTP {{ log.http_code || 0 }}</span>
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
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AutomationCard } from '../../../data/cards'
import { fetchRequestLogsPage, type RequestLog, type RequestLogPlatform } from '../../../services/logs'
import { extractErrorMessage } from '../../../utils/error'
import InlineModal from '../../common/InlineModal.vue'
import { cardProviderRef } from '../adapters/providerCardMappers'
import type { ProviderConcurrencyStatusView, ResolvedTheme } from '../types'

const HISTORY_LIMIT = 10
type ConcurrencyDetailsTab = 'active' | 'history'

const props = defineProps<{
  open: boolean
  provider: AutomationCard | null
  platform: RequestLogPlatform | null
  status: ProviderConcurrencyStatusView | null
  resolvedTheme: ResolvedTheme
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
let activeLoadRequestId = 0

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

.provider-concurrency-row__main strong {
  color: #0f172a;
}

.provider-concurrency-modal--dark .provider-concurrency-row__main strong {
  color: #f8fafc;
}

.provider-concurrency-row__main span,
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
