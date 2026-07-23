<template>
  <InlineModal
    :open="open"
    :title="modalTitle"
    :panel-width="'min(1120px, 94vw)'"
    :panel-class="modalPanelClass"
    @close="emit('close')"
  >
    <div
      :class="['provider-logs-modal', isDarkTheme ? 'provider-logs-modal--dark' : 'provider-logs-modal--light']"
      :style="{
        '--provider-log-accent': providerAccent,
        '--provider-log-tint': providerTint,
      }"
    >
      <section class="provider-logs-hero">
        <div class="provider-logs-hero__glow" aria-hidden="true"></div>
        <div class="provider-logs-hero__copy">
          <span class="provider-logs-hero__eyebrow">{{ platformLabel }}</span>
          <h3 class="provider-logs-hero__title">{{ providerName }}</h3>
          <p class="provider-logs-hero__subtitle">
            {{ t('components.main.providerLogs.summary') }}
          </p>
        </div>
        <div class="provider-logs-hero__side">
          <div class="provider-logs-hero__toolbar">
            <span class="provider-logs-pill provider-logs-pill--count">
              {{ t('components.main.providerLogs.loadedCount', { count: entries.length }) }}
            </span>
            <span class="provider-logs-pill provider-logs-pill--accent">
              <svg viewBox="0 0 24 24" aria-hidden="true" class="provider-logs-pill__icon">
                <path
                  d="M4 6.75h16M7 11.75h10M10.5 16.75h3"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.7"
                  stroke-linecap="round"
                />
              </svg>
              {{ showUnreadOnly ? t('components.main.providerLogs.unreadFailures') : t('components.main.providerLogs.allFailures') }}
            </span>
          </div>
          <div class="provider-logs-hero__actions">
            <div class="provider-logs-scope" role="tablist" :aria-label="t('components.main.providerLogs.scopeLabel')">
              <button
                type="button"
                :class="['provider-logs-scope__button', { 'is-active': showUnreadOnly }]"
                :disabled="loading || loadingMore || markingLogsRead"
                role="tab"
                :aria-selected="showUnreadOnly"
                @click="setLogScope('unread')"
              >
                {{ t('components.main.providerLogs.unreadFailures') }}
              </button>
              <button
                type="button"
                :class="['provider-logs-scope__button', { 'is-active': !showUnreadOnly }]"
                :disabled="loading || loadingMore || markingLogsRead"
                role="tab"
                :aria-selected="!showUnreadOnly"
                @click="setLogScope('all')"
              >
                {{ t('components.main.providerLogs.allFailures') }}
              </button>
            </div>
            <button
              v-if="canMarkProviderLogsRead"
              type="button"
              class="provider-logs-clear"
              :disabled="markingLogsRead"
              @click="openMarkReadConfirm"
            >
              <svg viewBox="0 0 24 24" aria-hidden="true" class="provider-logs-clear__icon">
                <path
                  d="M5 12.5l4.2 4.2L19 6.9"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.8"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
              {{ markingLogsRead ? t('components.main.providerLogs.markingRead') : t('components.main.providerLogs.markRead') }}
            </button>
          </div>
        </div>
      </section>

      <div v-if="loading" class="provider-logs-state">
        {{ t('components.main.providerLogs.loading') }}
      </div>
      <div v-else-if="error" class="provider-logs-state provider-logs-state--error">
        {{ t('components.main.providerLogs.loadFailed', { error }) }}
      </div>
      <div v-else-if="entries.length === 0" class="provider-logs-state provider-logs-state--empty">
        <strong>{{ emptyTitle }}</strong>
        <p>{{ emptyHint }}</p>
      </div>
      <div v-else class="provider-logs-feed">
        <article
          v-for="entry in displayEntries"
          :key="entry.log.id"
          class="provider-log-entry"
          :class="entry.log.http_code >= 500 ? 'is-severe' : 'is-warning'"
        >
          <header class="provider-log-entry__header">
            <div class="provider-log-entry__tags">
              <span class="provider-log-entry__status">
                HTTP {{ entry.log.http_code }}
              </span>
              <span class="provider-log-entry__tag">
                {{ entry.errorType || (entry.log.http_code >= 500 ? 'HTTP 5xx' : 'HTTP 4xx') }}
              </span>
              <span v-if="entry.log.error_source" class="provider-log-entry__tag provider-log-entry__tag--source">
                {{ errorSourceLabel(entry.log.error_source) }}
              </span>
              <span v-if="entry.semanticTag" class="provider-log-entry__tag provider-log-entry__tag--semantic">
                {{ entry.semanticTag }}
              </span>
              <span v-if="entry.errorCode" class="provider-log-entry__tag">
                {{ entry.errorCode }}
              </span>
              <span class="provider-log-entry__tag">
                {{ entry.log.error_read_at?.trim() ? t('components.main.providerLogs.readStatus') : t('components.main.providerLogs.unreadStatus') }}
              </span>
            </div>
            <time class="provider-log-entry__time" :datetime="entry.log.created_at">
              <svg viewBox="0 0 24 24" aria-hidden="true" class="provider-log-entry__time-icon">
                <circle cx="12" cy="12" r="7.25" fill="none" stroke="currentColor" stroke-width="1.6" />
                <path
                  d="M12 8.4v4.2l2.9 1.7"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.6"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
              {{ formatCreatedAt(entry.log.created_at) }}
            </time>
          </header>

          <div class="provider-log-entry__headline-row">
            <h4 class="provider-log-entry__headline">
              HTTP {{ entry.log.http_code }}
              <svg viewBox="0 0 24 24" aria-hidden="true" class="provider-log-entry__headline-icon">
                <circle cx="12" cy="12" r="8.25" fill="none" stroke="currentColor" stroke-width="1.8" />
                <path
                  d="M12 8.25v4.4"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.8"
                  stroke-linecap="round"
                />
                <circle cx="12" cy="16.55" r="0.95" fill="currentColor" />
              </svg>
            </h4>
          </div>

          <div class="provider-log-entry__meta">
            <span class="provider-log-entry__meta-item">
              <svg viewBox="0 0 24 24" aria-hidden="true" class="provider-log-entry__meta-icon">
                <path
                  d="M6.5 8.25h11M6.5 12h11M6.5 15.75h6"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.6"
                  stroke-linecap="round"
                />
                <rect x="4.25" y="5.25" width="15.5" height="13.5" rx="2.5" fill="none" stroke="currentColor" stroke-width="1.4" />
              </svg>
              {{ t('components.main.providerLogs.model') }}：<strong>{{ displayModel(entry.log) }}</strong>
            </span>
            <span class="provider-log-entry__meta-divider" aria-hidden="true">·</span>
            <span class="provider-log-entry__meta-item">
              <svg viewBox="0 0 24 24" aria-hidden="true" class="provider-log-entry__meta-icon">
                <path
                  d="M8 8h8M8 12h8M8 16h5"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.6"
                  stroke-linecap="round"
                />
                <rect x="5" y="4.75" width="14" height="14.5" rx="2.5" fill="none" stroke="currentColor" stroke-width="1.4" />
              </svg>
              {{ t('components.main.providerLogs.logId', { id: entry.log.id }) }}
            </span>
            <template v-if="entry.sourceLabel">
              <span class="provider-log-entry__meta-divider" aria-hidden="true">·</span>
              <span class="provider-log-entry__meta-item">{{ entry.sourceLabel }}</span>
            </template>
          </div>

          <div class="provider-log-terminal">
            <div class="provider-log-terminal__chrome" aria-hidden="true">
              <span></span>
              <span></span>
              <span></span>
            </div>
            <div class="provider-log-terminal__body-wrap">
              <div class="provider-log-terminal__toolbar">
                <span class="provider-log-terminal__label">{{ entry.detailLabel }}</span>
                <div class="provider-log-terminal__badges">
                  <span
                    v-if="entry.detailSource === 'diagnostic'"
                    class="provider-log-terminal__badge provider-log-terminal__badge--diagnostic"
                  >
                    {{ t('components.main.providerLogs.detailFromDiagnostic') }}
                  </span>
                  <span
                    v-else-if="entry.detailSource === 'console'"
                    class="provider-log-terminal__badge provider-log-terminal__badge--console"
                  >
                    {{ t('components.main.providerLogs.detailFromConsole') }}
                  </span>
                  <span
                    v-else-if="entry.detailSource === 'payload'"
                    class="provider-log-terminal__badge provider-log-terminal__badge--payload"
                  >
                    {{ t('components.main.providerLogs.detailFromRequest') }}
                  </span>
                  <span
                    v-if="entry.detailSource === 'payload' && entry.log.response_body_truncated"
                    class="provider-log-terminal__badge"
                  >
                    {{ t('components.main.providerLogs.responseTruncated') }}
                  </span>
                  <span v-if="entry.detailPreview.formatSkippedLarge" class="provider-log-terminal__badge">
                    {{ t('components.main.providerLogs.payloadLarge') }}
                  </span>
                </div>
              </div>
              <pre
                v-if="entry.detailPreview.rawText"
                class="provider-log-terminal__body"
                v-html="entry.detailPreview.html"
              ></pre>
              <div v-else class="provider-log-terminal__body provider-log-terminal__body--fallback">
                <span class="provider-log-terminal__prompt">&gt;</span>
                <span>{{ t('components.main.providerLogs.noPayload') }}</span>
              </div>
            </div>
          </div>

          <div class="provider-log-entry__footer">
            <span
              v-if="entry.detailSource === 'console'"
              class="provider-log-entry__source-note"
            >
              {{ t('components.main.providerLogs.consoleMatched') }}
            </span>
            <button
              type="button"
              class="provider-log-entry__copy"
              :class="{ 'is-disabled': !entry.copyText, 'is-copied': isCopied(entry) }"
              :disabled="!entry.copyText"
              @click="copyProviderDetail(entry)"
            >
              <svg viewBox="0 0 24 24" aria-hidden="true" class="provider-log-entry__copy-icon">
                <path
                  d="M8.5 8.5h8v10h-8a2 2 0 01-2-2v-6a2 2 0 012-2z"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.6"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <path
                  d="M10.5 8.5V7a2 2 0 012-2h5a2 2 0 012 2v7a2 2 0 01-2 2h-1"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.6"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
              {{ copyButtonLabel(entry) }}
            </button>
          </div>
        </article>

        <div v-if="hasMore" class="provider-logs-actions">
          <button
            type="button"
            class="provider-logs-load-more"
            :disabled="loadingMore"
            @click="loadMore"
          >
            {{ loadingMore ? t('components.main.providerLogs.loadingMore') : t('components.main.providerLogs.loadMore') }}
          </button>
        </div>
      </div>
    </div>
  </InlineModal>

  <BaseModal
    :open="markReadConfirmOpen"
    :title="t('components.main.providerLogs.markReadConfirmTitle')"
    variant="confirm"
    @close="closeMarkReadConfirm"
  >
    <div class="confirm-body">
      <p>
        {{ t('components.main.providerLogs.confirmMarkRead', { provider: providerName }) }}
      </p>
    </div>
    <footer class="form-actions confirm-actions provider-logs-confirm-actions">
      <BaseButton variant="outline" type="button" :disabled="markingLogsRead" @click="closeMarkReadConfirm">
        {{ t('common.cancel') }}
      </BaseButton>
      <BaseButton variant="primary" type="button" :disabled="markingLogsRead" @click="markCurrentProviderLogsRead">
        {{ markingLogsRead ? t('components.main.providerLogs.markingRead') : t('components.main.providerLogs.markRead') }}
      </BaseButton>
    </footer>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AutomationCard } from '../../../data/cards'
import type { ResolvedTheme } from '../types'
import { GetLogs, GetRecentLogs } from '../../../../bindings/codeswitch/services/consoleservice'
import BaseButton from '../../common/BaseButton.vue'
import BaseModal from '../../common/BaseModal.vue'
import InlineModal from '../../common/InlineModal.vue'
import {
  countProviderUnreadFailedRequestLogs,
  fetchFailedRequestLogsPage,
  markProviderFailedRequestLogsRead,
  type LogPlatform,
  type RequestLog,
} from '../../../services/logs'
import { cardProviderRef } from '../adapters/providerCardMappers'
import { buildPayloadPreview, type PayloadPreview } from '../../../utils/payloadPreview'
import {
  hasMeaningfulProviderErrorPayload,
  parseProviderErrorFromConsoleMessage,
  type ProviderErrorDetail,
} from '../../../utils/providerError'
import { extractErrorMessage } from '../../../utils/error'
import { showToast } from '../../../utils/toast'
import { writeTextToClipboard } from '../../../utils/clipboard'
import {
  buildConsoleProviderErrorCandidates,
  matchConsoleProviderCandidate,
  type ConsoleProviderErrorCandidate,
} from './providerLogsConsoleMatch'

type DetailSource = 'diagnostic' | 'payload' | 'console' | 'none'

type ProviderLogEntry = {
  log: RequestLog
  detailPreview: PayloadPreview
  errorSummary: string
  semanticTag: string
  errorCode: string
  errorType: string
  detailSource: DetailSource
  detailLabel: string
  sourceLabel: string
  copyText: string
}

type PayloadErrorState = {
  responseBody: string
  parsedError: ProviderErrorDetail | null
  hasMeaningfulDetail: boolean
}

type ConsoleCoverageMode = 'recent' | 'all'
type ProviderLogsScope = 'unread' | 'all'

type ResolvedEntriesResult = {
  entries: ProviderLogEntry[]
  unmatchedNoPayloadCount: number
}

type BrowserWindowWithWailsBridge = Window & {
  chrome?: {
    webview?: {
      postMessage?: (...args: any[]) => void
    }
  }
  webkit?: {
    messageHandlers?: {
      external?: {
        postMessage?: (...args: any[]) => void
      }
    }
  }
}

const DISPLAY_CHUNK_SIZE = 12
const RECENT_CONSOLE_LOG_COUNT = 400
const CONSOLE_MATCH_MAX_WINDOW_MS = 15 * 60 * 1000
const DEV_PROVIDER_LOGS_PLATFORM: LogPlatform = 'claude'
const DEV_PROVIDER_LOGS_PROVIDER = '0011'

function hasDesktopRuntimeBridge() {
  if (typeof window === 'undefined') {
    return false
  }
  const browserWindow = window as BrowserWindowWithWailsBridge
  return Boolean(
    browserWindow.chrome?.webview?.postMessage
    || browserWindow.webkit?.messageHandlers?.external?.postMessage,
  )
}

function shouldUseFrontendDevProviderLogsMock() {
  return import.meta.env.DEV
    && typeof window !== 'undefined'
    && !hasDesktopRuntimeBridge()
}

function createTodayMockTimestamp(hour: number, minute: number, second: number) {
  const now = new Date()
  const value = new Date(
    now.getFullYear(),
    now.getMonth(),
    now.getDate(),
    hour,
    minute,
    second,
    0,
  )
  const pad = (input: number) => String(input).padStart(2, '0')
  return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())} ${pad(value.getHours())}:${pad(value.getMinutes())}:${pad(value.getSeconds())}`
}

function createDevMockRequestLogs(): RequestLog[] {
  return [
    {
      id: 40784,
      platform: 'claude',
      provider_id: DEV_PROVIDER_LOGS_PROVIDER,
      provider: DEV_PROVIDER_LOGS_PROVIDER,
      model: 'claude-opus-4-1',
      requested_model: 'claude-opus-4-1',
      response_model: '',
      http_code: 499,
      input_tokens: 318,
      output_tokens: 0,
      cache_create_tokens: 0,
      cache_read_tokens: 0,
      reasoning_tokens: 0,
      created_at: createTodayMockTimestamp(18, 20, 19),
      response_body: '',
      response_body_truncated: false,
    },
    {
      id: 40761,
      platform: 'claude',
      provider_id: DEV_PROVIDER_LOGS_PROVIDER,
      provider: DEV_PROVIDER_LOGS_PROVIDER,
      model: 'claude-opus-4-1',
      requested_model: 'claude-opus-4-1',
      response_model: '',
      http_code: 429,
      input_tokens: 286,
      output_tokens: 0,
      cache_create_tokens: 0,
      cache_read_tokens: 0,
      reasoning_tokens: 0,
      created_at: createTodayMockTimestamp(16, 31, 36),
      response_body: JSON.stringify({
        error: {
          message: 'Rate limit exceeded for demo provider 0011. Please wait a moment before retrying.',
          type: 'rate_limit_error',
          code: 'too_many_requests',
        },
      }),
      response_body_truncated: false,
    },
    {
      id: 40637,
      platform: 'claude',
      provider_id: DEV_PROVIDER_LOGS_PROVIDER,
      provider: DEV_PROVIDER_LOGS_PROVIDER,
      model: 'claude-opus-4-1',
      requested_model: 'claude-opus-4-1',
      response_model: '',
      http_code: 503,
      input_tokens: 452,
      output_tokens: 0,
      cache_create_tokens: 0,
      cache_read_tokens: 0,
      reasoning_tokens: 0,
      created_at: createTodayMockTimestamp(14, 24, 48),
      response_body: JSON.stringify({
        error: {
          message: 'The upstream model is overloaded for provider 0011. Please retry in a few seconds.',
          type: 'overloaded_error',
          code: 'overloaded',
        },
      }),
      response_body_truncated: false,
    },
    {
      id: 40592,
      platform: 'claude',
      provider_id: DEV_PROVIDER_LOGS_PROVIDER,
      provider: DEV_PROVIDER_LOGS_PROVIDER,
      model: 'claude-sonnet-4',
      requested_model: 'claude-sonnet-4',
      response_model: '',
      http_code: 401,
      input_tokens: 132,
      output_tokens: 0,
      cache_create_tokens: 0,
      cache_read_tokens: 0,
      reasoning_tokens: 0,
      created_at: createTodayMockTimestamp(11, 9, 12),
      response_body: JSON.stringify({
        error: {
          message: 'Invalid demo API key for frontend-only preview provider 0011.',
          type: 'authentication_error',
          code: 'invalid_api_key',
        },
      }),
      response_body_truncated: false,
    },
  ]
}

const props = defineProps<{
  open: boolean
  provider: AutomationCard | null
  platform: LogPlatform | null
  resolvedTheme: ResolvedTheme
}>()

const emit = defineEmits<{
  close: []
  markedRead: []
}>()

const { t, locale } = useI18n()

const loading = ref(false)
const loadingMore = ref(false)
const error = ref('')
const entries = ref<RequestLog[]>([])
const consoleCandidates = ref<ConsoleProviderErrorCandidate[]>([])
const total = ref(0)
const unreadTotal = ref(0)
const requestSeq = ref(0)
const copiedEntryKey = ref('')
const consoleCoverageMode = ref<ConsoleCoverageMode>('recent')
const logScope = ref<ProviderLogsScope>('unread')
const markingLogsRead = ref(false)
const markReadConfirmOpen = ref(false)
const devMockLogs = ref<RequestLog[]>(createDevMockRequestLogs())

const providerName = computed(() => props.provider?.name?.trim() || t('components.main.providerLogs.modalTitleFallback'))
const providerAccent = computed(() => props.provider?.accent || '#ea580c')
const providerTint = computed(() => props.provider?.tint || 'rgba(249, 115, 22, 0.14)')
const isDarkTheme = computed(() => props.resolvedTheme === 'dark')
const modalPanelClass = computed(() => [
  'provider-logs-inline-modal',
  isDarkTheme.value ? 'provider-logs-inline-modal--dark' : 'provider-logs-inline-modal--light',
])

const providerFilter = computed(() => {
  const ref = props.provider ? cardProviderRef(props.provider) : ''
  if (ref) {
    return ref
  }
  return props.provider?.name?.trim() || ''
})

function isDevMockProviderTarget() {
  const providerName = props.provider?.name?.trim() || ''
  const providerId = Number(props.provider?.id)
  const providerRef = providerFilter.value.trim()

  return providerName === DEV_PROVIDER_LOGS_PROVIDER
    || providerRef === DEV_PROVIDER_LOGS_PROVIDER
    || (Number.isFinite(providerId) && providerId === 100)
    || providerRef === '100'
}

const isDevMockProviderLogs = computed(() => (
  shouldUseFrontendDevProviderLogsMock()
  && props.platform === DEV_PROVIDER_LOGS_PLATFORM
  && isDevMockProviderTarget()
))

const platformLabel = computed(() => {
  if (props.platform === 'claude') return t('components.main.providerLogs.platformClaude')
  if (props.platform === 'codex') return t('components.main.providerLogs.platformCodex')
  if (props.platform === 'gemini') return t('components.main.providerLogs.platformGemini')
  return t('components.main.providerLogs.modalTitleFallback')
})

const modalTitle = computed(() => {
  if (props.provider?.name) {
    return t('components.main.providerLogs.modalTitle', { name: props.provider.name })
  }
  return t('components.main.providerLogs.modalTitleFallback')
})

const hasMore = computed(() => entries.value.length < total.value)
const showUnreadOnly = computed(() => logScope.value === 'unread')
const isUnreadLog = (log: RequestLog) => !log.error_read_at?.trim()
const canMarkProviderLogsRead = computed(() => {
  return !loading.value
    && !loadingMore.value
    && !markingLogsRead.value
    && (unreadTotal.value > 0 || entries.value.some(isUnreadLog))
})

function buildDevMockLogsPage(limit: number, offset: number) {
  const normalizedLimit = Math.max(1, Math.floor(limit || DISPLAY_CHUNK_SIZE))
  const normalizedOffset = Math.max(0, Math.floor(offset || 0))
  const source = showUnreadOnly.value
    ? devMockLogs.value.filter(isUnreadLog)
    : devMockLogs.value
  const items = source.slice(normalizedOffset, normalizedOffset + normalizedLimit)
  return {
    items,
    total: source.length,
    limit: normalizedLimit,
    offset: normalizedOffset,
  }
}

function countDevMockUnreadLogs() {
  return devMockLogs.value.filter(isUnreadLog).length
}

const resetState = () => {
  entries.value = []
  consoleCandidates.value = []
  total.value = 0
  unreadTotal.value = 0
  error.value = ''
  loading.value = false
  loadingMore.value = false
  copiedEntryKey.value = ''
  consoleCoverageMode.value = 'recent'
  markingLogsRead.value = false
  markReadConfirmOpen.value = false
}

const truncateText = (value: string, maxLength = 240) => {
  const normalized = String(value ?? '').replace(/\s+/g, ' ').trim()
  if (!normalized) return ''
  if (normalized.length <= maxLength) return normalized
  return `${normalized.slice(0, maxLength).trimEnd()}...`
}

const toTimestamp = (value: string | null | undefined) => {
  if (!value) return Number.NaN
  const normalized = value.includes('T') ? value : value.replace(' ', 'T')
  const timestamp = new Date(normalized).getTime()
  return Number.isFinite(timestamp) ? timestamp : Number.NaN
}

const mergeLogsById = (current: RequestLog[], incoming: RequestLog[]) => {
  const merged = [...current]
  const seenIds = new Set(current.map((item) => Number(item.id ?? 0)))
  incoming.forEach((item) => {
    const logId = Number(item.id ?? 0)
    if (seenIds.has(logId)) return
    seenIds.add(logId)
    merged.push(item)
  })
  return merged
}

const displayModel = (log: RequestLog) => log.requested_model || log.model || log.response_model || '-'

const errorSourceLabel = (source: string) => {
  if (source === 'provider_response' || source === 'upstream_network' || source === 'upstream_stream' || source === 'proxy' || source === 'client_abort') {
    return t(`components.main.providerLogs.errorSource.${source}`)
  }
  return t('components.main.providerLogs.errorSource.unknown')
}

const getPayloadErrorState = (log: RequestLog): PayloadErrorState => {
  const responseBody = log.response_body?.trim() || ''
  const parsedError = parseProviderErrorFromConsoleMessage(
    responseBody ? `status ${log.http_code}: ${responseBody}` : `status ${log.http_code}:`,
  )

  return {
    responseBody,
    parsedError,
    hasMeaningfulDetail: hasMeaningfulProviderErrorPayload(responseBody, parsedError),
  }
}

const matchConsoleCandidate = (log: RequestLog, candidates: ConsoleProviderErrorCandidate[]) => {
  return matchConsoleProviderCandidate(log, candidates, [
    props.provider?.name ?? '',
    providerFilter.value,
  ])
}

const buildLogEntry = (
  log: RequestLog,
  payloadState: PayloadErrorState,
  matchedConsoleCandidate?: ConsoleProviderErrorCandidate | null,
): ProviderLogEntry => {
  const {
    responseBody,
    parsedError: payloadParsedError,
    hasMeaningfulDetail: payloadHasMeaningfulDetail,
  } = payloadState
  const consoleParsedError = matchedConsoleCandidate?.providerError ?? null
  const diagnosticMessage = log.error_message?.trim() || ''
  const diagnosticParsedError = diagnosticMessage
    ? parseProviderErrorFromConsoleMessage(`status ${log.http_code}: ${diagnosticMessage}`)
    : null

  const fallbackSummary = diagnosticMessage || (payloadHasMeaningfulDetail ? responseBody : `HTTP ${log.http_code}`)
  const detailSource: DetailSource = diagnosticMessage
    ? 'diagnostic'
    : payloadHasMeaningfulDetail
      ? 'payload'
      : consoleParsedError
        ? 'console'
        : 'none'
  const activeDetail = detailSource === 'diagnostic'
    ? diagnosticParsedError
    : detailSource === 'console'
      ? consoleParsedError
      : payloadParsedError
  const detailText = detailSource === 'diagnostic'
    ? diagnosticParsedError?.copyText?.trim() || diagnosticMessage
    : detailSource === 'payload'
      ? payloadParsedError?.copyText?.trim() || responseBody
      : detailSource === 'console'
        ? consoleParsedError?.copyText?.trim() || consoleParsedError?.rawPayload?.trim() || consoleParsedError?.summary?.trim() || ''
        : ''
  const summaryText = activeDetail?.summary || fallbackSummary

  return {
    log,
    detailPreview: buildPayloadPreview(detailText),
    errorSummary: truncateText(summaryText),
    semanticTag: activeDetail?.semanticTag || payloadParsedError?.semanticTag || '',
    errorCode: activeDetail?.errorCode || payloadParsedError?.errorCode || '',
    errorType: activeDetail?.errorType || payloadParsedError?.errorType || log.stream_error_kind || '',
    detailSource,
    detailLabel: t('components.main.providerLogs.providerErrorDetail'),
    sourceLabel: detailSource === 'diagnostic'
      ? t('components.main.providerLogs.detailFromDiagnostic')
      : detailSource === 'console'
        ? t('components.main.providerLogs.detailFromConsole')
        : detailSource === 'payload'
          ? t('components.main.providerLogs.detailFromRequest')
          : '',
    copyText: detailText,
  }
}

const resolveEntries = (logs: RequestLog[], candidates: ConsoleProviderErrorCandidate[]): ResolvedEntriesResult => {
  const availableCandidates = [...candidates]
  let unmatchedNoPayloadCount = 0

  const resolvedEntries = logs.map((item) => {
    const payloadState = getPayloadErrorState(item)
    const matched = item.error_message?.trim() || payloadState.hasMeaningfulDetail
      ? null
      : matchConsoleCandidate(item, availableCandidates)
    if (matched) {
      availableCandidates.splice(matched.index, 1)
    }

    const entry = buildLogEntry(item, payloadState, matched?.candidate ?? null)
    if (!entry.copyText) {
      unmatchedNoPayloadCount += 1
    }
    return entry
  })

  return {
    entries: resolvedEntries,
    unmatchedNoPayloadCount,
  }
}

const getOldestLogTimestamp = (logs: RequestLog[]) => {
  let oldest = Number.POSITIVE_INFINITY
  logs.forEach((log) => {
    const timestamp = toTimestamp(log.created_at)
    if (Number.isFinite(timestamp)) {
      oldest = Math.min(oldest, timestamp)
    }
  })
  return Number.isFinite(oldest) ? oldest : Number.NaN
}

const getEarliestCandidateTimestamp = (candidates: ConsoleProviderErrorCandidate[]) => {
  let oldest = Number.POSITIVE_INFINITY
  candidates.forEach((candidate) => {
    const timestamp = toTimestamp(candidate.timestamp)
    if (Number.isFinite(timestamp)) {
      oldest = Math.min(oldest, timestamp)
    }
  })
  return Number.isFinite(oldest) ? oldest : Number.NaN
}

const parseConsoleCandidates = async (mode: ConsoleCoverageMode) => {
  const logs = mode === 'all'
    ? await GetLogs()
    : await GetRecentLogs(RECENT_CONSOLE_LOG_COUNT)

  return buildConsoleProviderErrorCandidates(logs)
}

const shouldExpandConsoleCoverage = (
  logs: RequestLog[],
  candidates: ConsoleProviderErrorCandidate[],
  resolved: ResolvedEntriesResult,
) => {
  if (consoleCoverageMode.value === 'all' || logs.length === 0) {
    return false
  }

  const oldestLogTimestamp = getOldestLogTimestamp(logs)
  const earliestCandidateTimestamp = getEarliestCandidateTimestamp(candidates)
  const timeCoverageInsufficient =
    Number.isFinite(oldestLogTimestamp) &&
    Number.isFinite(earliestCandidateTimestamp) &&
    oldestLogTimestamp < earliestCandidateTimestamp - CONSOLE_MATCH_MAX_WINDOW_MS
  const recentPoolLikelyTruncated = candidates.length >= RECENT_CONSOLE_LOG_COUNT

  return timeCoverageInsufficient || (recentPoolLikelyTruncated && resolved.unmatchedNoPayloadCount > 0)
}

const ensureConsoleCoverage = async (
  logs: RequestLog[],
  currentSeq: number,
  recentCandidates?: ConsoleProviderErrorCandidate[],
) => {
  const baseCandidates = recentCandidates ?? consoleCandidates.value
  if (currentSeq !== requestSeq.value) return

  const resolved = resolveEntries(logs, baseCandidates)
  if (!shouldExpandConsoleCoverage(logs, baseCandidates, resolved)) {
    if (recentCandidates) {
      consoleCandidates.value = baseCandidates
      consoleCoverageMode.value = 'recent'
    }
    return
  }

  try {
    const fullCandidates = await parseConsoleCandidates('all')
    if (currentSeq !== requestSeq.value) return
    consoleCandidates.value = fullCandidates
    consoleCoverageMode.value = 'all'
  } catch (err) {
    console.warn('扩展控制台日志覆盖范围失败:', err)
    if (recentCandidates && currentSeq === requestSeq.value) {
      consoleCandidates.value = baseCandidates
      consoleCoverageMode.value = 'recent'
    }
  }
}

const displayResolution = computed(() => {
  locale.value
  return resolveEntries(entries.value, consoleCandidates.value)
})

const displayEntries = computed(() => {
  return displayResolution.value.entries
})

const getEntryKey = (entry: ProviderLogEntry) => `${entry.log.id}:${entry.detailSource}`

const isCopied = (entry: ProviderLogEntry) => copiedEntryKey.value === getEntryKey(entry)

const markCopiedState = (entryKey: string) => {
  copiedEntryKey.value = entryKey
  window.setTimeout(() => {
    if (copiedEntryKey.value === entryKey) {
      copiedEntryKey.value = ''
    }
  }, 1500)
}

const copyProviderDetail = async (entry: ProviderLogEntry) => {
  const payload = entry.copyText.trim()
  if (!payload) {
    showToast(t('components.main.providerLogs.copyUnavailable'), 'warning')
    return
  }

  try {
    await writeTextToClipboard(payload)
    markCopiedState(getEntryKey(entry))
    showToast(t('components.main.providerLogs.copySuccess'), 'success')
  } catch (err) {
    showToast(t('components.main.providerLogs.copyFailed', { error: extractErrorMessage(err) }), 'error')
  }
}

const copyButtonLabel = (entry: ProviderLogEntry) => {
  if (isCopied(entry)) {
    return t('components.main.providerLogs.copied')
  }
  if (!entry.copyText) {
    return t('components.main.providerLogs.copyUnavailableShort')
  }
  return t('components.main.providerLogs.copyDetail')
}

const emptyTitle = computed(() => (
  showUnreadOnly.value
    ? t('components.main.providerLogs.emptyUnread')
    : t('components.main.providerLogs.emptyAll')
))

const emptyHint = computed(() => (
  showUnreadOnly.value
    ? t('components.main.providerLogs.emptyUnreadHint')
    : t('components.main.providerLogs.emptyAllHint')
))

const setLogScope = (scope: ProviderLogsScope) => {
  if (logScope.value === scope || loading.value || loadingMore.value || markingLogsRead.value) return
  logScope.value = scope
}

const openMarkReadConfirm = () => {
  if (!canMarkProviderLogsRead.value) return
  markReadConfirmOpen.value = true
}

const closeMarkReadConfirm = () => {
  if (markingLogsRead.value) return
  markReadConfirmOpen.value = false
}

const markCurrentProviderLogsRead = async () => {
  if (!canMarkProviderLogsRead.value) return

  markingLogsRead.value = true
  markReadConfirmOpen.value = false
  try {
    if (entries.value.length === 0) {
      showToast(
        t('components.main.providerLogs.markReadEmpty', {
          provider: providerName.value,
        }),
        'warning',
      )
      return
    }

    if (isDevMockProviderLogs.value) {
      const readAt = createTodayMockTimestamp(new Date().getHours(), new Date().getMinutes(), new Date().getSeconds())
      let markedLogs = 0
      devMockLogs.value = devMockLogs.value.map((log) => {
        if (!isUnreadLog(log)) return log
        markedLogs += 1
        return {
          ...log,
          error_read_at: readAt,
        }
      })

      if (markedLogs > 0) {
        showToast(
          t('components.main.providerLogs.markReadSuccess', {
            provider: providerName.value,
            logs: markedLogs,
          }),
          'success',
        )
        emit('markedRead')
      } else {
        showToast(
          t('components.main.providerLogs.markReadEmpty', {
            provider: providerName.value,
          }),
          'warning',
        )
      }

      await reloadLogs()
      return
    }

    const result = await markProviderFailedRequestLogsRead(
      props.platform ?? '',
      providerFilter.value,
      providerName.value,
    )
    const markedLogs = Number(result?.marked_request_logs ?? 0)

    if (markedLogs > 0) {
      showToast(
        t('components.main.providerLogs.markReadSuccess', {
          provider: providerName.value,
          logs: markedLogs,
        }),
        'success',
      )
      emit('markedRead')
    } else {
      showToast(
        t('components.main.providerLogs.markReadEmpty', {
          provider: providerName.value,
        }),
        'warning',
      )
    }

    await reloadLogs()
  } catch (err) {
    showToast(t('components.main.providerLogs.markReadFailed', { error: extractErrorMessage(err) }), 'error')
  } finally {
    markingLogsRead.value = false
  }
}

const fetchRecentConsoleCandidates = async () => {
  try {
    return await parseConsoleCandidates('recent')
  } catch (err) {
    console.warn('加载最近控制台错误详情失败:', err)
    return []
  }
}

const fetchUnreadFailureCount = async () => {
  if (isDevMockProviderLogs.value) {
    return countDevMockUnreadLogs()
  }
  const result = await countProviderUnreadFailedRequestLogs(
    props.platform ?? '',
    providerFilter.value,
    providerName.value,
  )
  const count = Number(result?.unread_failed_requests ?? 0)
  return Number.isFinite(count) ? Math.max(0, Math.floor(count)) : 0
}

const fetchUnreadFailureCountSafely = async (fallbackCount: number) => {
  try {
    return await fetchUnreadFailureCount()
  } catch (err) {
    console.warn('加载供应商未读失败日志计数失败:', err)
    return fallbackCount
  }
}

const reloadLogs = async () => {
  const currentSeq = ++requestSeq.value
  const fallbackUnreadTotal = unreadTotal.value
  resetState()

  if (!props.open || !props.platform || !providerFilter.value) return

  if (isDevMockProviderLogs.value) {
    const page = buildDevMockLogsPage(DISPLAY_CHUNK_SIZE, 0)
    entries.value = page.items
    total.value = page.total
    unreadTotal.value = countDevMockUnreadLogs()
    consoleCandidates.value = []
    consoleCoverageMode.value = 'recent'
    return
  }

  loading.value = true
  try {
    const [page, recentConsoleErrors, unreadCount] = await Promise.all([
      fetchFailedRequestLogsPage({
        platform: props.platform,
        provider: providerFilter.value,
        limit: DISPLAY_CHUNK_SIZE,
        offset: 0,
        unreadOnly: showUnreadOnly.value,
      }),
      fetchRecentConsoleCandidates(),
      fetchUnreadFailureCountSafely(fallbackUnreadTotal),
    ])
    if (currentSeq !== requestSeq.value) return
    entries.value = page.items
    consoleCandidates.value = recentConsoleErrors
    consoleCoverageMode.value = 'recent'
    total.value = page.total
    unreadTotal.value = Math.max(unreadCount, page.items.filter(isUnreadLog).length)
    await ensureConsoleCoverage(page.items, currentSeq, recentConsoleErrors)
  } catch (err) {
    if (currentSeq !== requestSeq.value) return
    error.value = extractErrorMessage(err)
  } finally {
    if (currentSeq === requestSeq.value) {
      loading.value = false
    }
  }
}

const loadMore = async () => {
  if (loading.value || loadingMore.value || !hasMore.value) return

  if (isDevMockProviderLogs.value) {
    const page = buildDevMockLogsPage(DISPLAY_CHUNK_SIZE, entries.value.length)
    entries.value = mergeLogsById(entries.value, page.items)
    total.value = page.total
    return
  }

  loadingMore.value = true
  const currentSeq = requestSeq.value
  try {
    const page = await fetchFailedRequestLogsPage({
      platform: props.platform ?? '',
      provider: providerFilter.value,
      limit: DISPLAY_CHUNK_SIZE,
      offset: entries.value.length,
      unreadOnly: showUnreadOnly.value,
    })
    if (currentSeq !== requestSeq.value) return
    const mergedEntries = mergeLogsById(entries.value, page.items)
    total.value = page.total
    entries.value = mergedEntries
    await ensureConsoleCoverage(mergedEntries, currentSeq)
  } catch (err) {
    if (currentSeq !== requestSeq.value) return
    error.value = extractErrorMessage(err)
  } finally {
    if (currentSeq === requestSeq.value) {
      loadingMore.value = false
    }
  }
}

const formatCreatedAt = (value: string) => {
  if (!value) return '-'
  const normalized = value.includes('T') ? value : value.replace(' ', 'T')
  const date = new Date(normalized)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale.value, {
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(date)
}

watch(
  () => [props.open, props.platform, props.provider?.id, props.provider?.name, props.provider?.providerRef, logScope.value] as const,
  ([open]) => {
    if (!open) {
      requestSeq.value += 1
      logScope.value = 'unread'
      resetState()
      return
    }
    void reloadLogs()
  },
  { immediate: true },
)
</script>

<style scoped>
:global(.provider-logs-inline-modal) {
  overflow: hidden;
  border-radius: 24px;
  border: 1px solid rgba(226, 232, 240, 0.82);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(246, 249, 255, 0.96));
  box-shadow:
    0 36px 90px rgba(15, 23, 42, 0.16),
    0 12px 28px rgba(15, 23, 42, 0.08);
}

:global(.provider-logs-inline-modal .modal-header) {
  padding: 16px 20px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(246, 248, 252, 0.88));
}

:global(.provider-logs-inline-modal .modal-title) {
  color: rgba(15, 23, 42, 0.92);
  font-size: 15px;
  font-weight: 600;
  letter-spacing: -0.01em;
}

:global(.provider-logs-inline-modal .modal-body) {
  padding: 14px 18px 20px;
  background: transparent;
}

:global(.provider-logs-inline-modal .ghost-icon) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 10px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: rgba(255, 255, 255, 0.56);
  color: rgba(71, 85, 105, 0.82);
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease,
    transform 0.18s ease;
}

:global(.provider-logs-inline-modal .ghost-icon:hover:not(:disabled)),
:global(.provider-logs-inline-modal .ghost-icon:focus-visible) {
  transform: translateY(-1px);
  border-color: rgba(99, 102, 241, 0.26);
  background: rgba(99, 102, 241, 0.08);
  color: #4338ca;
}

:global(.provider-logs-inline-modal button),
.provider-logs-confirm-actions :deep(button) {
  margin: 0;
}

:global(.provider-logs-inline-modal--dark) {
  border-color: rgba(255, 255, 255, 0.08);
  background: linear-gradient(180deg, rgba(16, 17, 24, 0.995), rgba(11, 13, 19, 0.985));
  box-shadow:
    0 42px 110px rgba(0, 0, 0, 0.62),
    inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

:global(.provider-logs-inline-modal--dark .modal-header) {
  border-bottom-color: rgba(255, 255, 255, 0.05);
  background: linear-gradient(180deg, rgba(23, 24, 31, 0.98), rgba(19, 20, 28, 0.94));
}

:global(.provider-logs-inline-modal--dark .modal-title) {
  color: rgba(241, 245, 249, 0.96);
}

:global(.provider-logs-inline-modal--dark .ghost-icon) {
  border-color: rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.03);
  color: rgba(148, 163, 184, 0.82);
}

:global(.provider-logs-inline-modal--dark .ghost-icon:hover:not(:disabled)),
:global(.provider-logs-inline-modal--dark .ghost-icon:focus-visible) {
  border-color: rgba(129, 140, 248, 0.24);
  background: rgba(99, 102, 241, 0.12);
  color: #e0e7ff;
}

.provider-logs-modal {
  --provider-log-page-bg: linear-gradient(180deg, rgba(249, 250, 255, 0.9), rgba(243, 246, 252, 0.96));
  --provider-log-heading: rgba(15, 23, 42, 0.96);
  --provider-log-subtitle: rgba(51, 65, 85, 0.74);
  --provider-log-toolbar-bg: rgba(255, 255, 255, 0.62);
  --provider-log-toolbar-border: rgba(148, 163, 184, 0.2);
  --provider-log-pill-text: rgba(51, 65, 85, 0.82);
  --provider-log-pill-bg: rgba(255, 255, 255, 0.7);
  --provider-log-pill-border: rgba(148, 163, 184, 0.18);
  --provider-log-state-bg: linear-gradient(180deg, rgba(248, 250, 252, 0.96), rgba(255, 255, 255, 0.92));
  --provider-log-state-border: rgba(148, 163, 184, 0.3);
  --provider-log-state-text: rgba(51, 65, 85, 0.74);
  --provider-log-card-bg: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(245, 247, 251, 0.96));
  --provider-log-card-border: rgba(203, 213, 225, 0.56);
  --provider-log-card-hover-border: rgba(148, 163, 184, 0.8);
  --provider-log-card-shadow: 0 18px 36px rgba(15, 23, 42, 0.08);
  --provider-log-card-tag-text: rgba(51, 65, 85, 0.76);
  --provider-log-card-tag-bg: rgba(248, 250, 252, 0.94);
  --provider-log-card-tag-border: rgba(203, 213, 225, 0.64);
  --provider-log-time-text: rgba(100, 116, 139, 0.82);
  --provider-log-headline-text: rgba(15, 23, 42, 0.96);
  --provider-log-meta-text: rgba(71, 85, 105, 0.92);
  --provider-log-source-text: rgba(37, 99, 235, 0.92);
  --provider-log-terminal-bg: linear-gradient(180deg, rgba(12, 18, 30, 0.98), rgba(17, 24, 39, 0.98));
  --provider-log-terminal-border: rgba(15, 23, 42, 0.08);
  --provider-log-terminal-toolbar-bg: rgba(255, 255, 255, 0.02);
  --provider-log-terminal-toolbar-border: rgba(148, 163, 184, 0.12);
  --provider-log-terminal-text: #cbd5e1;
  --provider-log-terminal-muted: rgba(148, 163, 184, 0.58);
  --provider-log-copy-bg: linear-gradient(135deg, rgba(255, 255, 255, 0.92), rgba(248, 250, 252, 0.94));
  --provider-log-copy-bg-hover: linear-gradient(135deg, rgba(255, 247, 237, 0.96), rgba(255, 255, 255, 0.98));
  --provider-log-copy-border: rgba(148, 163, 184, 0.26);
  --provider-log-copy-border-hover: rgba(249, 115, 22, 0.3);
  --provider-log-copy-text: rgba(51, 65, 85, 0.82);
  --provider-log-copy-glow: rgba(249, 115, 22, 0.18);
  display: flex;
  flex-direction: column;
  gap: 18px;
  min-height: min(76vh, 860px);
  padding: 4px 2px 2px;
  color: var(--mac-text);
  background: var(--provider-log-page-bg);
}

.provider-logs-modal--dark {
  --provider-log-page-bg: linear-gradient(180deg, rgba(18, 19, 24, 0.76), rgba(11, 13, 19, 0.98));
  --provider-log-heading: rgba(248, 250, 252, 0.98);
  --provider-log-subtitle: rgba(148, 163, 184, 0.82);
  --provider-log-toolbar-bg: rgba(7, 10, 16, 0.46);
  --provider-log-toolbar-border: rgba(255, 255, 255, 0.08);
  --provider-log-pill-text: rgba(226, 232, 240, 0.88);
  --provider-log-pill-bg: rgba(255, 255, 255, 0.04);
  --provider-log-pill-border: rgba(255, 255, 255, 0.08);
  --provider-log-state-bg: linear-gradient(180deg, rgba(16, 18, 24, 0.96), rgba(13, 15, 22, 0.94));
  --provider-log-state-border: rgba(255, 255, 255, 0.08);
  --provider-log-state-text: rgba(203, 213, 225, 0.78);
  --provider-log-card-bg: linear-gradient(180deg, rgba(24, 24, 31, 0.96), rgba(19, 20, 28, 0.94));
  --provider-log-card-border: rgba(255, 255, 255, 0.05);
  --provider-log-card-hover-border: rgba(255, 255, 255, 0.1);
  --provider-log-card-shadow: 0 24px 48px rgba(0, 0, 0, 0.28);
  --provider-log-card-tag-text: rgba(203, 213, 225, 0.84);
  --provider-log-card-tag-bg: rgba(255, 255, 255, 0.04);
  --provider-log-card-tag-border: rgba(255, 255, 255, 0.08);
  --provider-log-time-text: rgba(100, 116, 139, 0.92);
  --provider-log-headline-text: rgba(248, 250, 252, 0.98);
  --provider-log-meta-text: rgba(148, 163, 184, 0.82);
  --provider-log-source-text: #93c5fd;
  --provider-log-terminal-bg: linear-gradient(180deg, rgba(7, 8, 12, 0.94), rgba(4, 6, 10, 0.98));
  --provider-log-terminal-border: rgba(255, 255, 255, 0.06);
  --provider-log-terminal-toolbar-bg: rgba(255, 255, 255, 0.02);
  --provider-log-terminal-toolbar-border: rgba(255, 255, 255, 0.05);
  --provider-log-terminal-text: rgba(203, 213, 225, 0.9);
  --provider-log-terminal-muted: rgba(100, 116, 139, 0.58);
  --provider-log-copy-bg: rgba(255, 255, 255, 0.04);
  --provider-log-copy-bg-hover: rgba(249, 115, 22, 0.12);
  --provider-log-copy-border: rgba(255, 255, 255, 0.08);
  --provider-log-copy-border-hover: rgba(249, 115, 22, 0.24);
  --provider-log-copy-text: rgba(226, 232, 240, 0.9);
  --provider-log-copy-glow: rgba(249, 115, 22, 0.18);
  color: #f3f4f6;
}

.provider-logs-hero {
  position: relative;
  overflow: hidden;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 28px;
  padding: 20px 22px;
  border-radius: 18px;
  background: linear-gradient(145deg, rgba(255, 255, 255, 0.92), color-mix(in srgb, var(--provider-log-tint) 24%, rgba(248, 250, 252, 0.96)));
  border: 1px solid color-mix(in srgb, var(--provider-log-accent) 16%, rgba(148, 163, 184, 0.18));
  box-shadow: 0 22px 44px rgba(15, 23, 42, 0.08);
}

.provider-logs-hero__glow {
  position: absolute;
  inset: -40px auto auto 55%;
  width: 320px;
  height: 220px;
  border-radius: 999px;
  pointer-events: none;
  background: radial-gradient(circle, color-mix(in srgb, var(--provider-log-accent) 22%, rgba(99, 102, 241, 0.18)) 0%, transparent 72%);
  filter: blur(28px);
  opacity: 0.9;
}

.provider-logs-hero__copy {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}

.provider-logs-hero__eyebrow {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: color-mix(in srgb, var(--provider-log-accent) 70%, #6366f1);
}

.provider-logs-hero__title {
  margin: 0;
  font-size: clamp(1.7rem, 2vw, 2rem);
  font-weight: 700;
  line-height: 1.08;
  letter-spacing: -0.02em;
  color: var(--provider-log-heading);
}

.provider-logs-hero__subtitle {
  margin: 0;
  max-width: 760px;
  font-size: 13px;
  line-height: 1.72;
  color: var(--provider-log-subtitle);
}

.provider-logs-hero__side {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
  flex-shrink: 0;
}

.provider-logs-hero__toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 4px;
  border-radius: 14px;
  border: 1px solid var(--provider-log-toolbar-border);
  background: var(--provider-log-toolbar-bg);
  backdrop-filter: blur(12px);
}

.provider-logs-hero__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.provider-logs-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 34px;
  padding: 0 13px;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 600;
  color: var(--provider-log-pill-text);
  background: var(--provider-log-pill-bg);
  border: 1px solid var(--provider-log-pill-border);
  font-variant-numeric: tabular-nums;
}

.provider-logs-pill--count {
  min-width: 110px;
}

.provider-logs-pill__icon {
  width: 13px;
  height: 13px;
}

.provider-logs-pill--accent {
  color: color-mix(in srgb, var(--provider-log-accent) 76%, #6366f1);
  background: color-mix(in srgb, var(--provider-log-accent) 11%, rgba(255, 255, 255, 0.72));
  border-color: color-mix(in srgb, var(--provider-log-accent) 24%, transparent);
}

.provider-logs-scope {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-height: 36px;
  padding: 4px;
  border-radius: 12px;
  border: 1px solid var(--provider-log-toolbar-border);
  background: var(--provider-log-toolbar-bg);
}

.provider-logs-scope__button {
  min-height: 28px;
  padding: 0 11px;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: var(--provider-log-pill-text);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition:
    background 0.18s ease,
    color 0.18s ease,
    opacity 0.18s ease;
}

.provider-logs-scope__button.is-active {
  background: color-mix(in srgb, var(--provider-log-accent) 14%, rgba(255, 255, 255, 0.86));
  color: color-mix(in srgb, var(--provider-log-accent) 80%, #4338ca);
}

.provider-logs-scope__button:disabled {
  cursor: wait;
  opacity: 0.64;
}

.provider-logs-clear {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-height: 36px;
  padding: 0 14px;
  border-radius: 12px;
  border: 1px solid rgba(34, 197, 94, 0.18);
  background: linear-gradient(135deg, rgba(236, 253, 245, 0.9), rgba(240, 253, 244, 0.88));
  color: #047857;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition:
    transform 0.18s ease,
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease,
    opacity 0.18s ease;
}

.provider-logs-clear__icon {
  width: 14px;
  height: 14px;
}

.provider-logs-clear:hover:not(:disabled) {
  transform: translateY(-1px);
  border-color: rgba(34, 197, 94, 0.3);
  background: linear-gradient(135deg, rgba(236, 253, 245, 0.96), rgba(220, 252, 231, 0.92));
  color: #065f46;
}

.provider-logs-clear:disabled {
  cursor: wait;
  opacity: 0.68;
  transform: none;
}

.provider-logs-state {
  padding: 40px 24px;
  text-align: center;
  border-radius: 18px;
  border: 1px dashed var(--provider-log-state-border);
  background: var(--provider-log-state-bg);
  color: var(--provider-log-state-text);
}

.provider-logs-state--error {
  color: #b91c1c;
  background: linear-gradient(180deg, rgba(254, 242, 242, 0.96), rgba(255, 255, 255, 0.92));
  border-color: rgba(248, 113, 113, 0.3);
}

.provider-logs-state--empty strong {
  display: block;
  margin-bottom: 8px;
  font-size: 18px;
  color: var(--provider-log-heading);
}

.provider-logs-state--empty p {
  margin: 0;
}

.provider-logs-feed {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.provider-log-entry {
  position: relative;
  overflow: hidden;
  padding: 16px 18px 16px 22px;
  border-radius: 16px;
  border: 1px solid var(--provider-log-card-border);
  background: var(--provider-log-card-bg);
  box-shadow: var(--provider-log-card-shadow);
  transition:
    transform 0.18s ease,
    border-color 0.18s ease,
    box-shadow 0.18s ease;
}

.provider-log-entry:hover {
  transform: translateY(-1px);
  border-color: var(--provider-log-card-hover-border);
}

.provider-log-entry::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 4px;
  border-radius: 16px 0 0 16px;
  box-shadow:
    0 0 16px currentColor,
    1px 0 0 rgba(255, 255, 255, 0.04);
  pointer-events: none;
}

.provider-log-entry.is-severe::before {
  color: rgba(244, 63, 94, 0.52);
  background: linear-gradient(180deg, #ef4444, #f97316);
}

.provider-log-entry.is-warning::before {
  color: rgba(249, 115, 22, 0.46);
  background: linear-gradient(180deg, #f59e0b, #fb923c);
}

.provider-log-entry__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.provider-log-entry__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.provider-log-entry__status,
.provider-log-entry__tag {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
}

.provider-log-entry.is-warning .provider-log-entry__status {
  color: #fdba74;
  background: rgba(249, 115, 22, 0.12);
  border: 1px solid rgba(249, 115, 22, 0.28);
  box-shadow: 0 0 12px rgba(249, 115, 22, 0.1);
}

.provider-log-entry.is-severe .provider-log-entry__status {
  color: #fda4af;
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.26);
  box-shadow: 0 0 12px rgba(244, 63, 94, 0.1);
}

.provider-log-entry__tag {
  color: var(--provider-log-card-tag-text);
  background: var(--provider-log-card-tag-bg);
  border: 1px solid var(--provider-log-card-tag-border);
}

.provider-log-entry__tag--semantic {
  color: #fb923c;
  background: rgba(249, 115, 22, 0.12);
  border-color: rgba(249, 115, 22, 0.18);
}

.provider-log-entry__time {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--provider-log-time-text);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  font-family: 'SFMono-Regular', Menlo, Consolas, monospace;
}

.provider-log-entry__time-icon {
  width: 13px;
  height: 13px;
}

.provider-log-entry__headline-row {
  margin: 12px 0 8px;
}

.provider-log-entry__headline {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  line-height: 1.25;
  color: var(--provider-log-headline-text);
  letter-spacing: -0.01em;
}

.provider-log-entry__headline-icon {
  width: 15px;
  height: 15px;
}

.provider-log-entry.is-warning .provider-log-entry__headline-icon {
  color: #fb923c;
}

.provider-log-entry.is-severe .provider-log-entry__headline-icon {
  color: #fb7185;
}

.provider-log-entry__meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
  font-size: 13px;
  color: var(--provider-log-meta-text);
}

.provider-log-entry__meta-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.provider-log-entry__meta-item strong {
  color: var(--provider-log-headline-text);
  font-weight: 600;
}

.provider-log-entry__meta-divider {
  color: rgba(100, 116, 139, 0.6);
}

.provider-log-entry__meta-icon {
  width: 13px;
  height: 13px;
  color: rgba(100, 116, 139, 0.82);
}

.provider-log-terminal {
  overflow: hidden;
  border-radius: 12px;
  border: 1px solid var(--provider-log-terminal-border);
  background: var(--provider-log-terminal-bg);
}

.provider-log-terminal__chrome {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 22px;
  padding: 0 12px;
  background: var(--provider-log-terminal-toolbar-bg);
  border-bottom: 1px solid var(--provider-log-terminal-toolbar-border);
}

.provider-log-terminal__chrome span {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--provider-log-terminal-muted);
}

.provider-log-terminal__body-wrap {
  padding: 0;
}

.provider-log-terminal__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px 0;
}

.provider-log-terminal__label {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: rgba(148, 163, 184, 0.78);
}

.provider-log-terminal__badges {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.provider-log-terminal__badge {
  padding: 4px 8px;
  border-radius: 999px;
  color: #fdba74;
  font-size: 10px;
  font-weight: 700;
  background: rgba(249, 115, 22, 0.14);
}

.provider-log-terminal__badge--console {
  color: #bfdbfe;
  background: rgba(59, 130, 246, 0.16);
}

.provider-log-terminal__badge--diagnostic {
  color: #fef3c7;
  border-color: rgba(245, 158, 11, 0.38);
  background: rgba(180, 83, 9, 0.28);
}

.provider-log-terminal__badge--payload {
  color: #fde68a;
  background: rgba(234, 179, 8, 0.14);
}

.provider-log-entry__source-note {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  padding: 0 12px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  color: var(--provider-log-source-text);
  background: rgba(59, 130, 246, 0.1);
  border: 1px solid rgba(59, 130, 246, 0.18);
}

.provider-log-terminal__body {
  margin: 0;
  padding: 14px 14px 16px;
  max-height: 280px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.72;
  color: var(--provider-log-terminal-text);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: 'SFMono-Regular', Menlo, Consolas, monospace;
}

.provider-log-terminal__body--fallback {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.provider-log-terminal__prompt {
  color: rgba(148, 163, 184, 0.46);
}

.provider-log-terminal__body :deep(.json-token.json-key) {
  color: #fda4af;
}

.provider-log-terminal__body :deep(.json-token.json-string) {
  color: #86efac;
}

.provider-log-terminal__body :deep(.json-token.json-number) {
  color: #7dd3fc;
}

.provider-log-terminal__body :deep(.json-token.json-boolean) {
  color: #c4b5fd;
}

.provider-log-terminal__body :deep(.json-token.json-null) {
  color: #fcd34d;
}

.provider-log-entry__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 14px;
}

.provider-log-entry__copy {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 34px;
  padding: 0 12px;
  border-radius: 10px;
  border: 1px solid var(--provider-log-copy-border);
  background: var(--provider-log-copy-bg);
  color: var(--provider-log-copy-text);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition:
    transform 0.18s ease,
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease,
    box-shadow 0.18s ease;
}

.provider-log-entry__copy-icon {
  width: 14px;
  height: 14px;
}

.provider-log-entry__copy:hover:not(:disabled) {
  transform: translateY(-1px);
  border-color: var(--provider-log-copy-border-hover);
  background: var(--provider-log-copy-bg-hover);
  box-shadow: 0 10px 18px var(--provider-log-copy-glow);
}

.provider-log-entry__copy.is-copied {
  color: #22c55e;
  border-color: rgba(34, 197, 94, 0.26);
  background: rgba(34, 197, 94, 0.1);
}

.provider-log-entry__copy.is-disabled,
.provider-log-entry__copy:disabled {
  cursor: not-allowed;
  opacity: 0.42;
  transform: none;
  box-shadow: none;
}

.provider-logs-actions {
  display: flex;
  justify-content: center;
  padding: 4px 0 0;
}

.provider-logs-load-more {
  min-width: 160px;
  min-height: 40px;
  padding: 0 18px;
  border: none;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 700;
  color: #eef2ff;
  background: linear-gradient(135deg, #4f46e5 0%, #6366f1 100%);
  box-shadow: 0 14px 28px rgba(79, 70, 229, 0.28);
  cursor: pointer;
  transition:
    transform 0.2s ease,
    box-shadow 0.2s ease,
    opacity 0.2s ease;
}

.provider-logs-load-more:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 18px 32px rgba(79, 70, 229, 0.32);
}

.provider-logs-load-more:disabled {
  cursor: wait;
  opacity: 0.72;
}

:global(.provider-logs-inline-modal .modal-body::-webkit-scrollbar),
.provider-log-terminal__body::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

:global(.provider-logs-inline-modal .modal-body::-webkit-scrollbar-track),
.provider-log-terminal__body::-webkit-scrollbar-track {
  background: transparent;
}

:global(.provider-logs-inline-modal .modal-body::-webkit-scrollbar-thumb),
.provider-log-terminal__body::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.22);
}

:global(.provider-logs-inline-modal .modal-body::-webkit-scrollbar-thumb:hover),
.provider-log-terminal__body::-webkit-scrollbar-thumb:hover {
  background: rgba(148, 163, 184, 0.34);
}

@media (max-width: 860px) {
  .provider-logs-hero,
  .provider-log-entry__header,
  .provider-log-terminal__toolbar {
    flex-direction: column;
  }

  .provider-logs-hero__side,
  .provider-logs-hero__toolbar,
  .provider-logs-hero__actions {
    justify-content: flex-start;
    align-items: flex-start;
  }

  .provider-log-entry__footer {
    align-items: stretch;
  }

  .provider-log-entry__copy {
    width: 100%;
  }
}

.provider-logs-modal--dark .provider-logs-hero {
  background:
    linear-gradient(145deg, rgba(20, 22, 32, 0.96), rgba(15, 16, 24, 0.95));
  border-color: rgba(99, 102, 241, 0.18);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.04),
    0 24px 56px rgba(0, 0, 0, 0.34);
}

.provider-logs-modal--dark .provider-logs-pill--accent {
  color: #c7d2fe;
  background: rgba(99, 102, 241, 0.14);
  border-color: rgba(129, 140, 248, 0.22);
}

.provider-logs-modal--dark .provider-logs-scope__button.is-active {
  background: rgba(99, 102, 241, 0.18);
  color: #e0e7ff;
}

.provider-logs-modal--dark .provider-logs-clear {
  border-color: rgba(74, 222, 128, 0.2);
  background: rgba(20, 83, 45, 0.18);
  color: #bbf7d0;
}

.provider-logs-modal--dark .provider-logs-clear:hover:not(:disabled) {
  border-color: rgba(74, 222, 128, 0.3);
  background: rgba(20, 83, 45, 0.28);
  color: #dcfce7;
}

.provider-logs-modal--dark .provider-logs-state--error {
  color: #fda4af;
  background: linear-gradient(180deg, rgba(55, 22, 28, 0.94), rgba(34, 18, 24, 0.92));
  border-color: rgba(248, 113, 113, 0.18);
}

.provider-logs-modal--dark .provider-log-entry__copy.is-copied {
  color: #bbf7d0;
  border-color: rgba(34, 197, 94, 0.22);
  background: rgba(20, 83, 45, 0.18);
}
</style>
