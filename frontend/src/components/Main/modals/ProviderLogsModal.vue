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
          </div>

          <div v-if="!entry.responsePreview.rawText" class="provider-log-entry__hint">
            {{ t('components.main.providerLogs.noPayload') }}
          </div>
          <div v-else class="provider-log-code">
            <div class="provider-log-code__header">
              <span>{{ t('components.main.providerLogs.responseBody') }}</span>
              <div class="provider-log-code__badges">
                <span v-if="entry.log.response_body_truncated" class="provider-log-code__badge">
                  {{ t('components.main.providerLogs.responseTruncated') }}
                </span>
                <span v-if="entry.responsePreview.formatSkippedLarge" class="provider-log-code__badge">
                  {{ t('components.main.providerLogs.payloadLarge') }}
                </span>
              </div>
            </div>
            <pre class="provider-log-code__body" v-html="entry.responsePreview.html"></pre>
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
import InlineModal from '../../common/InlineModal.vue'
import {
  fetchFailedRequestLogsPage,
  type LogPlatform,
  type RequestLog,
} from '../../../services/logs'
import { cardProviderRef } from '../adapters/providerCardMappers'
import { buildPayloadPreview, type PayloadPreview } from '../../../utils/payloadPreview'
import { parseProviderErrorFromConsoleMessage } from '../../../utils/providerError'
import { extractErrorMessage } from '../../../utils/error'

type ProviderLogEntry = {
  log: RequestLog
  responsePreview: PayloadPreview
  errorSummary: string
  semanticTag: string
  errorCode: string
  errorType: string
}

const DISPLAY_CHUNK_SIZE = 12

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
const total = ref(0)
const requestSeq = ref(0)

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
  total.value = 0
  error.value = ''
  loading.value = false
  loadingMore.value = false
}

const truncateText = (value: string, maxLength = 240) => {
  const normalized = String(value ?? '').replace(/\s+/g, ' ').trim()
  if (!normalized) return ''
  if (normalized.length <= maxLength) return normalized
  return `${normalized.slice(0, maxLength).trimEnd()}...`
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

const buildLogEntry = (log: RequestLog): ProviderLogEntry => {
  const responseBody = log.response_body || ''
  const parsedError = parseProviderErrorFromConsoleMessage(
    responseBody ? `status ${log.http_code}: ${responseBody}` : `status ${log.http_code}:`,
  )
  const fallbackSummary = responseBody || `HTTP ${log.http_code}`

  return {
    log,
    responsePreview: buildPayloadPreview(responseBody),
    errorSummary: truncateText(parsedError?.summary || fallbackSummary),
    semanticTag: parsedError?.semanticTag || '',
    errorCode: parsedError?.errorCode || '',
    errorType: parsedError?.errorType || '',
  }
}

const displayEntries = computed(() => entries.value.map((item) => buildLogEntry(item)))

const reloadLogs = async () => {
  const currentSeq = ++requestSeq.value
  resetState()

  if (!props.open || !props.platform || !providerFilter.value) return

  loading.value = true
  try {
    const page = await fetchFailedRequestLogsPage({
      platform: props.platform,
      provider: providerFilter.value,
      limit: DISPLAY_CHUNK_SIZE,
      offset: 0,
    })
    if (currentSeq !== requestSeq.value) return
    entries.value = page.items
    total.value = page.total
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
    total.value = page.total
    entries.value = mergeLogsById(entries.value, page.items)
  } catch (err) {
    if (currentSeq !== requestSeq.value) return
    error.value = extractErrorMessage(err)
  } finally {
    if (currentSeq === requestSeq.value) {
      loadingMore.value = false
    }
  }
}

const displayModel = (log: RequestLog) => log.requested_model || log.model || log.response_model || '-'

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
    radial-gradient(circle at top right, color-mix(in srgb, var(--provider-log-accent) 24%, transparent), transparent 36%),
    linear-gradient(135deg, color-mix(in srgb, var(--provider-log-tint) 88%, #ffffff), rgba(255, 255, 255, 0.94));
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
  margin-bottom: 14px;
  font-size: 13px;
  color: rgba(71, 85, 105, 0.92);
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

.provider-log-entry__hint--error {
  color: #b91c1c;
  background: rgba(254, 242, 242, 0.98);
  border-color: rgba(248, 113, 113, 0.3);
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

:global(.dark) .provider-logs-hero {
  background:
    radial-gradient(circle at top right, color-mix(in srgb, var(--provider-log-accent) 28%, transparent), transparent 38%),
    linear-gradient(135deg, rgba(15, 23, 42, 0.96), rgba(30, 41, 59, 0.94));
  border-color: rgba(71, 85, 105, 0.42);
  box-shadow: 0 28px 48px rgba(2, 6, 23, 0.34);
}

:global(.dark) .provider-logs-hero__subtitle,
:global(.dark) .provider-logs-pill,
:global(.dark) .provider-log-entry__time,
:global(.dark) .provider-log-entry__meta,
:global(.dark) .provider-logs-state {
  color: rgba(226, 232, 240, 0.76);
}

:global(.dark) .provider-logs-pill {
  background: rgba(15, 23, 42, 0.46);
  border-color: rgba(148, 163, 184, 0.2);
}

:global(.dark) .provider-logs-pill--accent {
  color: #fdba74;
  background: rgba(124, 45, 18, 0.34);
}

:global(.dark) .provider-logs-state {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.94), rgba(30, 41, 59, 0.9));
  border-color: rgba(71, 85, 105, 0.46);
}

:global(.dark) .provider-logs-state--empty strong,
:global(.dark) .provider-log-entry__summary {
  color: rgba(248, 250, 252, 0.94);
}

:global(.dark) .provider-log-entry {
  border-color: rgba(71, 85, 105, 0.46);
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.98), rgba(30, 41, 59, 0.94));
  box-shadow: 0 18px 34px rgba(2, 6, 23, 0.26);
}

:global(.dark) .provider-log-entry__tag {
  color: rgba(226, 232, 240, 0.78);
  background: rgba(30, 41, 59, 0.92);
  border-color: rgba(100, 116, 139, 0.34);
}

:global(.dark) .provider-log-entry__tag--semantic {
  color: #fdba74;
  background: rgba(154, 52, 18, 0.3);
  border-color: rgba(251, 146, 60, 0.34);
}

:global(.dark) .provider-log-entry__hint {
  color: rgba(226, 232, 240, 0.74);
  background: rgba(15, 23, 42, 0.7);
  border-color: rgba(100, 116, 139, 0.34);
}

:global(.dark) .provider-log-entry__hint--error {
  color: #fda4af;
  background: rgba(69, 10, 10, 0.5);
  border-color: rgba(251, 113, 133, 0.24);
}

@media (max-width: 860px) {
  .provider-logs-hero,
  .provider-log-entry__header,
  .provider-log-code__header {
    flex-direction: column;
  }

  .provider-logs-hero__stats {
    justify-content: flex-start;
  }

  .provider-log-entry__time {
    white-space: normal;
  }
}
</style>
