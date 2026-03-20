<template>
  <InlineModal
    :open="open"
    :title="modalTitle"
    :panel-width="'min(1120px, 94vw)'"
    @close="$emit('close')"
  >
    <div
      class="provider-logs-modal"
      :style="{
        '--provider-log-accent': providerAccent,
        '--provider-log-tint': providerTint,
      }"
    >
      <section class="provider-logs-hero">
        <div class="provider-logs-hero__copy">
          <span class="provider-logs-hero__eyebrow">{{ platformLabel }}</span>
          <h3 class="provider-logs-hero__title">{{ providerName }}</h3>
          <p class="provider-logs-hero__subtitle">
            {{ t('components.main.providerLogs.summary') }}
          </p>
        </div>
        <div class="provider-logs-hero__stats">
          <span class="provider-logs-pill provider-logs-pill--accent">
            {{ t('components.main.providerLogs.failureOnly') }}
          </span>
          <span class="provider-logs-pill">
            {{ t('components.main.providerLogs.loadedCount', { count: entries.length }) }}
          </span>
        </div>
      </section>

      <div v-if="loading" class="provider-logs-state">
        {{ t('components.main.providerLogs.loading') }}
      </div>
      <div v-else-if="error" class="provider-logs-state provider-logs-state--error">
        {{ t('components.main.providerLogs.loadFailed', { error }) }}
      </div>
      <div v-else-if="entries.length === 0" class="provider-logs-state provider-logs-state--empty">
        <strong>{{ t('components.main.providerLogs.empty') }}</strong>
        <p>{{ t('components.main.providerLogs.emptyHint') }}</p>
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
              <span v-if="entry.semanticTag" class="provider-log-entry__tag provider-log-entry__tag--semantic">
                {{ entry.semanticTag }}
              </span>
              <span v-if="entry.errorType" class="provider-log-entry__tag">
                {{ entry.errorType }}
              </span>
              <span v-if="entry.errorCode" class="provider-log-entry__tag">
                {{ entry.errorCode }}
              </span>
            </div>
            <time class="provider-log-entry__time" :datetime="entry.log.created_at">
              {{ formatCreatedAt(entry.log.created_at) }}
            </time>
          </header>

          <p class="provider-log-entry__summary">
            {{ entry.errorSummary }}
          </p>

          <div class="provider-log-entry__meta">
            <span>{{ t('components.main.providerLogs.model') }}：{{ displayModel(entry.log) }}</span>
            <span>{{ t('components.main.providerLogs.logId', { id: entry.log.id }) }}</span>
            <span v-if="entry.sourceLabel">{{ entry.sourceLabel }}</span>
          </div>

          <div class="provider-log-entry__toolbar">
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
              {{ copyButtonLabel(entry) }}
            </button>
          </div>

          <div v-if="!entry.detailPreview.rawText" class="provider-log-entry__hint">
            {{ t('components.main.providerLogs.noPayload') }}
          </div>
          <div v-else class="provider-log-code">
            <div class="provider-log-code__header">
              <span>{{ entry.detailLabel }}</span>
              <div class="provider-log-code__badges">
                <span
                  v-if="entry.detailSource === 'console'"
                  class="provider-log-code__badge provider-log-code__badge--console"
                >
                  {{ t('components.main.providerLogs.detailFromConsole') }}
                </span>
                <span
                  v-else-if="entry.detailSource === 'payload'"
                  class="provider-log-code__badge provider-log-code__badge--payload"
                >
                  {{ t('components.main.providerLogs.detailFromRequest') }}
                </span>
                <span
                  v-if="entry.detailSource === 'payload' && entry.log.response_body_truncated"
                  class="provider-log-code__badge"
                >
                  {{ t('components.main.providerLogs.responseTruncated') }}
                </span>
                <span v-if="entry.detailPreview.formatSkippedLarge" class="provider-log-code__badge">
                  {{ t('components.main.providerLogs.payloadLarge') }}
                </span>
              </div>
            </div>
            <pre class="provider-log-code__body" v-html="entry.detailPreview.html"></pre>
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
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AutomationCard } from '../../../data/cards'
import { GetLogs, GetRecentLogs } from '../../../../bindings/codeswitch/services/consoleservice'
import InlineModal from '../../common/InlineModal.vue'
import {
  fetchFailedRequestLogsPage,
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

type DetailSource = 'payload' | 'console' | 'none'

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

type ResolvedEntriesResult = {
  entries: ProviderLogEntry[]
  unmatchedNoPayloadCount: number
}

const DISPLAY_CHUNK_SIZE = 12
const RECENT_CONSOLE_LOG_COUNT = 400
const CONSOLE_MATCH_MAX_WINDOW_MS = 15 * 60 * 1000

const props = defineProps<{
  open: boolean
  provider: AutomationCard | null
  platform: LogPlatform | null
}>()

defineEmits<{
  close: []
}>()

const { t, locale } = useI18n()

const loading = ref(false)
const loadingMore = ref(false)
const error = ref('')
const entries = ref<RequestLog[]>([])
const consoleCandidates = ref<ConsoleProviderErrorCandidate[]>([])
const total = ref(0)
const requestSeq = ref(0)
const copiedEntryKey = ref('')
const consoleCoverageMode = ref<ConsoleCoverageMode>('recent')

const providerName = computed(() => props.provider?.name?.trim() || t('components.main.providerLogs.modalTitleFallback'))
const providerAccent = computed(() => props.provider?.accent || '#ea580c')
const providerTint = computed(() => props.provider?.tint || 'rgba(249, 115, 22, 0.14)')

const providerFilter = computed(() => {
  const ref = props.provider ? cardProviderRef(props.provider) : ''
  if (ref) {
    return ref
  }
  return props.provider?.name?.trim() || ''
})

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

const resetState = () => {
  entries.value = []
  consoleCandidates.value = []
  total.value = 0
  error.value = ''
  loading.value = false
  loadingMore.value = false
  copiedEntryKey.value = ''
  consoleCoverageMode.value = 'recent'
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

  const fallbackSummary = payloadHasMeaningfulDetail ? responseBody : `HTTP ${log.http_code}`
  const detailSource: DetailSource = payloadHasMeaningfulDetail ? 'payload' : consoleParsedError ? 'console' : 'none'
  const activeDetail = detailSource === 'console' ? consoleParsedError : payloadParsedError
  const detailText = detailSource === 'payload'
    ? payloadParsedError?.copyText?.trim() || responseBody
    : detailSource === 'console'
      ? consoleParsedError?.copyText?.trim() || consoleParsedError?.rawPayload?.trim() || consoleParsedError?.summary?.trim() || ''
      : ''
  const summaryText = detailSource === 'console'
    ? consoleParsedError?.summary || (payloadHasMeaningfulDetail ? payloadParsedError?.summary : '') || fallbackSummary
    : payloadHasMeaningfulDetail
      ? payloadParsedError?.summary || fallbackSummary
      : fallbackSummary

  return {
    log,
    detailPreview: buildPayloadPreview(detailText),
    errorSummary: truncateText(summaryText),
    semanticTag: activeDetail?.semanticTag || payloadParsedError?.semanticTag || '',
    errorCode: activeDetail?.errorCode || payloadParsedError?.errorCode || '',
    errorType: activeDetail?.errorType || payloadParsedError?.errorType || '',
    detailSource,
    detailLabel: t('components.main.providerLogs.providerErrorDetail'),
    sourceLabel: detailSource === 'console'
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
    const matched = payloadState.hasMeaningfulDetail ? null : matchConsoleCandidate(item, availableCandidates)
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

const fetchRecentConsoleCandidates = async () => {
  try {
    return await parseConsoleCandidates('recent')
  } catch (err) {
    console.warn('加载最近控制台错误详情失败:', err)
    return []
  }
}

const reloadLogs = async () => {
  const currentSeq = ++requestSeq.value
  resetState()

  if (!props.open || !props.platform || !providerFilter.value) return

  loading.value = true
  try {
    const [page, recentConsoleErrors] = await Promise.all([
      fetchFailedRequestLogsPage({
        platform: props.platform,
        provider: providerFilter.value,
        limit: DISPLAY_CHUNK_SIZE,
        offset: 0,
      }),
      fetchRecentConsoleCandidates(),
    ])
    if (currentSeq !== requestSeq.value) return
    entries.value = page.items
    consoleCandidates.value = recentConsoleErrors
    consoleCoverageMode.value = 'recent'
    total.value = page.total
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
  loadingMore.value = true
  const currentSeq = requestSeq.value
  try {
    const page = await fetchFailedRequestLogsPage({
      platform: props.platform ?? '',
      provider: providerFilter.value,
      limit: DISPLAY_CHUNK_SIZE,
      offset: entries.value.length,
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
  () => [props.open, props.platform, props.provider?.id, props.provider?.name, props.provider?.providerRef] as const,
  ([open]) => {
    if (!open) {
      requestSeq.value += 1
      resetState()
      return
    }
    void reloadLogs()
  },
  { immediate: true },
)
</script>

<style scoped>
.provider-logs-modal {
  display: flex;
  flex-direction: column;
  gap: 18px;
  color: var(--mac-text);
}

.provider-logs-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 22px;
  border-radius: 22px;
  background:
    radial-gradient(circle at top right, color-mix(in srgb, var(--provider-log-accent) 26%, transparent), transparent 38%),
    linear-gradient(135deg, color-mix(in srgb, var(--provider-log-tint) 86%, #ffffff), rgba(255, 255, 255, 0.96));
  border: 1px solid color-mix(in srgb, var(--provider-log-accent) 18%, rgba(15, 23, 42, 0.08));
  box-shadow: 0 24px 48px rgba(15, 23, 42, 0.08);
}

.provider-logs-hero__copy {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.provider-logs-hero__eyebrow {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: color-mix(in srgb, var(--provider-log-accent) 72%, #7c2d12);
}

.provider-logs-hero__title {
  margin: 0;
  font-size: 28px;
  font-weight: 700;
  line-height: 1.1;
}

.provider-logs-hero__subtitle {
  margin: 0;
  max-width: 700px;
  line-height: 1.6;
  color: rgba(15, 23, 42, 0.68);
}

.provider-logs-hero__stats {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.provider-logs-pill {
  display: inline-flex;
  align-items: center;
  min-height: 36px;
  padding: 0 14px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 600;
  color: rgba(15, 23, 42, 0.72);
  background: rgba(255, 255, 255, 0.72);
  border: 1px solid rgba(148, 163, 184, 0.24);
  backdrop-filter: blur(10px);
}

.provider-logs-pill--accent {
  color: color-mix(in srgb, var(--provider-log-accent) 80%, #9a3412);
  background: color-mix(in srgb, var(--provider-log-accent) 10%, rgba(255, 255, 255, 0.84));
  border-color: color-mix(in srgb, var(--provider-log-accent) 22%, transparent);
}

.provider-logs-state {
  padding: 34px 20px;
  text-align: center;
  border-radius: 20px;
  border: 1px dashed rgba(148, 163, 184, 0.36);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.96), rgba(255, 255, 255, 0.92));
  color: rgba(15, 23, 42, 0.72);
}

.provider-logs-state--error {
  color: #b91c1c;
  background: linear-gradient(180deg, rgba(254, 242, 242, 0.98), rgba(255, 255, 255, 0.92));
  border-color: rgba(248, 113, 113, 0.3);
}

.provider-logs-state--empty strong {
  display: block;
  margin-bottom: 8px;
  font-size: 18px;
  color: rgba(15, 23, 42, 0.88);
}

.provider-logs-state--empty p {
  margin: 0;
}

.provider-logs-feed {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.provider-log-entry {
  position: relative;
  padding: 18px 18px 16px;
  border-radius: 22px;
  border: 1px solid rgba(226, 232, 240, 0.92);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.94));
  box-shadow: 0 18px 36px rgba(15, 23, 42, 0.08);
}

.provider-log-entry::before {
  content: '';
  position: absolute;
  left: 0;
  top: 18px;
  bottom: 18px;
  width: 4px;
  border-radius: 999px;
}

.provider-log-entry.is-severe::before {
  background: linear-gradient(180deg, #ef4444, #f97316);
}

.provider-log-entry.is-warning::before {
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
  min-height: 30px;
  padding: 0 11px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
}

.provider-log-entry__status {
  color: #fff7ed;
  background: linear-gradient(135deg, #b91c1c, #ea580c);
}

.provider-log-entry__tag {
  color: rgba(15, 23, 42, 0.7);
  background: rgba(241, 245, 249, 0.98);
  border: 1px solid rgba(203, 213, 225, 0.7);
}

.provider-log-entry__tag--semantic {
  color: #9a3412;
  background: rgba(255, 237, 213, 0.98);
  border-color: rgba(251, 146, 60, 0.34);
}

.provider-log-entry__time {
  color: rgba(100, 116, 139, 0.92);
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
}

.provider-log-entry__summary {
  margin: 14px 0 10px;
  font-size: 17px;
  font-weight: 600;
  line-height: 1.55;
  color: rgba(15, 23, 42, 0.92);
}

.provider-log-entry__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-bottom: 12px;
  font-size: 13px;
  color: rgba(71, 85, 105, 0.92);
}

.provider-log-entry__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.provider-log-entry__source-note {
  font-size: 12px;
  font-weight: 600;
  color: rgba(154, 52, 18, 0.92);
}

.provider-log-entry__copy {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 156px;
  min-height: 34px;
  padding: 0 14px;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--provider-log-accent) 26%, rgba(15, 23, 42, 0.08));
  background: linear-gradient(135deg, rgba(255, 247, 237, 0.98), rgba(255, 255, 255, 0.94));
  color: color-mix(in srgb, var(--provider-log-accent) 76%, #7c2d12);
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
  transition:
    transform 0.18s ease,
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease;
}

.provider-log-entry__copy:hover:not(:disabled) {
  transform: translateY(-1px);
  border-color: color-mix(in srgb, var(--provider-log-accent) 34%, transparent);
  background: linear-gradient(135deg, rgba(255, 237, 213, 0.98), rgba(255, 247, 237, 0.96));
}

.provider-log-entry__copy.is-copied {
  color: #166534;
  border-color: rgba(34, 197, 94, 0.3);
  background: linear-gradient(135deg, rgba(220, 252, 231, 0.96), rgba(240, 253, 244, 0.96));
}

.provider-log-entry__copy.is-disabled,
.provider-log-entry__copy:disabled {
  cursor: not-allowed;
  opacity: 0.58;
  transform: none;
}

.provider-log-entry__hint {
  padding: 14px 16px;
  border-radius: 16px;
  font-size: 13px;
  line-height: 1.65;
  color: rgba(71, 85, 105, 0.96);
  background: rgba(248, 250, 252, 0.98);
  border: 1px dashed rgba(203, 213, 225, 0.72);
}

.provider-log-code {
  overflow: hidden;
  border-radius: 18px;
  border: 1px solid rgba(226, 232, 240, 0.95);
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.98), rgba(30, 41, 59, 0.98));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.provider-log-code__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: rgba(226, 232, 240, 0.78);
  border-bottom: 1px solid rgba(71, 85, 105, 0.36);
  background: rgba(15, 23, 42, 0.42);
}

.provider-log-code__badges {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.provider-log-code__badge {
  padding: 4px 8px;
  border-radius: 999px;
  color: #fdba74;
  background: rgba(249, 115, 22, 0.18);
}

.provider-log-code__badge--console {
  color: #bfdbfe;
  background: rgba(59, 130, 246, 0.18);
}

.provider-log-code__badge--payload {
  color: #fde68a;
  background: rgba(234, 179, 8, 0.16);
}

.provider-log-code__body {
  margin: 0;
  padding: 16px;
  max-height: 300px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.72;
  color: #e2e8f0;
  white-space: pre-wrap;
  word-break: break-word;
}

.provider-log-code__body :deep(.json-token.json-key) {
  color: #fda4af;
}

.provider-log-code__body :deep(.json-token.json-string) {
  color: #86efac;
}

.provider-log-code__body :deep(.json-token.json-number) {
  color: #7dd3fc;
}

.provider-log-code__body :deep(.json-token.json-boolean) {
  color: #c4b5fd;
}

.provider-log-code__body :deep(.json-token.json-null) {
  color: #fcd34d;
}

.provider-logs-actions {
  display: flex;
  justify-content: center;
  padding-top: 4px;
}

.provider-logs-load-more {
  min-width: 160px;
  min-height: 42px;
  padding: 0 18px;
  border: none;
  border-radius: 999px;
  font-size: 14px;
  font-weight: 700;
  color: #fff7ed;
  background: linear-gradient(135deg, color-mix(in srgb, var(--provider-log-accent) 90%, #ea580c), #b91c1c);
  box-shadow: 0 16px 30px rgba(185, 28, 28, 0.22);
  cursor: pointer;
  transition:
    transform 0.2s ease,
    box-shadow 0.2s ease,
    opacity 0.2s ease;
}

.provider-logs-load-more:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 22px 36px rgba(185, 28, 28, 0.28);
}

.provider-logs-load-more:disabled {
  cursor: wait;
  opacity: 0.72;
}

@media (max-width: 860px) {
  .provider-logs-hero,
  .provider-log-entry__header,
  .provider-log-code__header {
    flex-direction: column;
  }

  .provider-log-entry__toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .provider-logs-hero__stats {
    justify-content: flex-start;
  }

  .provider-log-entry__time {
    white-space: normal;
  }

  .provider-log-entry__copy {
    width: 100%;
  }
}

:global(.dark) .provider-logs-modal {
  color: #f3f4f6;
}

:global(.dark) .provider-logs-hero {
  background:
    radial-gradient(circle at top right, color-mix(in srgb, var(--provider-log-accent) 28%, rgba(15, 23, 42, 0)), transparent 42%),
    linear-gradient(145deg, rgba(10, 14, 24, 0.96), rgba(19, 24, 35, 0.94));
  border-color: rgba(255, 255, 255, 0.08);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.04),
    0 28px 64px rgba(0, 0, 0, 0.42);
}

:global(.dark) .provider-logs-hero__eyebrow {
  color: color-mix(in srgb, var(--provider-log-accent) 70%, #fdba74);
}

:global(.dark) .provider-logs-hero__subtitle {
  color: rgba(226, 232, 240, 0.72);
}

:global(.dark) .provider-logs-pill {
  color: rgba(226, 232, 240, 0.84);
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.08);
}

:global(.dark) .provider-logs-pill--accent {
  color: #fed7aa;
  background: color-mix(in srgb, var(--provider-log-accent) 18%, rgba(255, 255, 255, 0.04));
  border-color: color-mix(in srgb, var(--provider-log-accent) 22%, rgba(255, 255, 255, 0.1));
}

:global(.dark) .provider-logs-state {
  color: rgba(226, 232, 240, 0.74);
  border-color: rgba(255, 255, 255, 0.08);
  background: linear-gradient(180deg, rgba(14, 19, 30, 0.92), rgba(20, 26, 38, 0.9));
}

:global(.dark) .provider-logs-state--error {
  color: #fca5a5;
  background: linear-gradient(180deg, rgba(55, 22, 28, 0.94), rgba(34, 18, 24, 0.92));
  border-color: rgba(248, 113, 113, 0.24);
}

:global(.dark) .provider-logs-state--empty strong {
  color: rgba(255, 255, 255, 0.92);
}

:global(.dark) .provider-log-entry {
  border-color: rgba(255, 255, 255, 0.08);
  background:
    linear-gradient(180deg, rgba(11, 16, 27, 0.98), rgba(17, 23, 34, 0.96));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.04),
    0 24px 48px rgba(0, 0, 0, 0.34);
}

:global(.dark) .provider-log-entry__tag {
  color: rgba(226, 232, 240, 0.82);
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(255, 255, 255, 0.08);
}

:global(.dark) .provider-log-entry__tag--semantic {
  color: #fdba74;
  background: rgba(249, 115, 22, 0.16);
  border-color: rgba(249, 115, 22, 0.24);
}

:global(.dark) .provider-log-entry__time {
  color: rgba(148, 163, 184, 0.82);
}

:global(.dark) .provider-log-entry__summary {
  color: rgba(248, 250, 252, 0.95);
}

:global(.dark) .provider-log-entry__meta {
  color: rgba(148, 163, 184, 0.92);
}

:global(.dark) .provider-log-entry__source-note {
  color: #93c5fd;
}

:global(.dark) .provider-log-entry__copy {
  color: rgba(255, 237, 213, 0.92);
  background: linear-gradient(135deg, rgba(41, 27, 22, 0.96), rgba(28, 20, 19, 0.96));
  border-color: rgba(249, 115, 22, 0.24);
}

:global(.dark) .provider-log-entry__copy:hover:not(:disabled) {
  background: linear-gradient(135deg, rgba(61, 36, 26, 0.98), rgba(39, 25, 20, 0.98));
  border-color: rgba(251, 146, 60, 0.34);
}

:global(.dark) .provider-log-entry__copy.is-copied {
  color: #bbf7d0;
  border-color: rgba(34, 197, 94, 0.32);
  background: linear-gradient(135deg, rgba(20, 48, 35, 0.96), rgba(18, 37, 31, 0.96));
}

:global(.dark) .provider-log-entry__hint {
  color: rgba(203, 213, 225, 0.86);
  background: rgba(255, 255, 255, 0.03);
  border-color: rgba(255, 255, 255, 0.08);
}

:global(.dark) .provider-log-code {
  border-color: rgba(255, 255, 255, 0.08);
  background: linear-gradient(180deg, rgba(5, 10, 18, 0.98), rgba(12, 18, 28, 0.98));
}

:global(.dark) .provider-log-code__header {
  color: rgba(226, 232, 240, 0.72);
  border-bottom-color: rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.03);
}

:global(.dark) .provider-log-code__badge {
  color: #fdba74;
  background: rgba(249, 115, 22, 0.16);
}

:global(.dark) .provider-log-code__badge--console {
  color: #bfdbfe;
  background: rgba(59, 130, 246, 0.18);
}

:global(.dark) .provider-log-code__badge--payload {
  color: #fde68a;
  background: rgba(234, 179, 8, 0.16);
}
</style>
