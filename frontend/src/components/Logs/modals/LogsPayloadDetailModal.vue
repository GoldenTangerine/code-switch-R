<template>
  <BaseModal
    :open="open"
    :title="t('components.logs.payloadDetail.title')"
    panel-width="min(1080px, 96vw)"
    :body-scrollable="false"
    @close="emit('close')"
  >
    <div class="payload-detail-modal">
      <p v-if="loading" class="cost-detail-loading">
        {{ t('components.logs.loading') }}
      </p>
      <template v-else>
        <div class="payload-detail-meta">
          {{ t('components.logs.payloadDetail.logId', { id: logId || '—' }) }}
        </div>
        <section v-if="hasPerformanceMetrics" class="payload-performance" :aria-label="t('components.logs.payloadDetail.performance')">
          <div class="payload-performance__header">
            <span>{{ t('components.logs.payloadDetail.performance') }}</span>
            <span class="payload-performance__connection">
              {{ log?.connection_reused
                ? t('components.logs.payloadDetail.connectionReused')
                : t('components.logs.payloadDetail.connectionNew') }}
            </span>
          </div>
          <dl class="payload-performance__grid">
            <div v-for="metric in performanceMetrics" :key="metric.key" class="payload-performance__metric">
              <dt>{{ metric.label }}</dt>
              <dd>{{ metric.value }}</dd>
            </div>
          </dl>
        </section>
        <p v-else class="payload-performance-empty">
          {{ t('components.logs.payloadDetail.performanceUnavailable') }}
        </p>
        <div class="payload-detail-grid">
          <section class="payload-detail-panel">
            <header class="payload-detail-panel__header">
              <span>{{ t('components.logs.payloadDetail.requestBody') }}</span>
              <div class="payload-detail-panel__actions">
                <span v-if="requestPayloadPreview.isJson" class="payload-detail-format-tag">JSON</span>
                <span v-if="detail?.request_body_truncated" class="payload-detail-truncated">
                  {{ t('components.logs.payloadDetail.truncated') }}
                </span>
                <button
                  type="button"
                  class="payload-detail-copy-btn"
                  :disabled="!requestPayloadPreview.rawText"
                  @click="copyPayloadDetail('request', 'raw')"
                >
                  {{ t('components.logs.payloadDetail.copyRaw') }}
                </button>
                <button
                  v-if="requestPayloadPreview.isFormatted && requestPayloadPreview.renderedText !== requestPayloadPreview.rawText"
                  type="button"
                  class="payload-detail-copy-btn"
                  @click="copyPayloadDetail('request', 'formatted')"
                >
                  {{ t('components.logs.payloadDetail.copyFormatted') }}
                </button>
              </div>
            </header>
            <pre
              v-if="requestPayloadPreview.renderedText"
              class="payload-detail-pre"
              :class="{ 'payload-detail-pre--json': requestPayloadPreview.isJson }"
            ><code class="payload-detail-code" v-html="requestPayloadPreview.html"></code></pre>
            <p v-if="requestPayloadPreview.formatSkippedLarge" class="payload-detail-note">
              {{ t('components.logs.payloadDetail.formatSkippedLarge') }}
            </p>
            <p v-else-if="!requestPayloadPreview.renderedText" class="cost-detail-empty">
              {{ t('components.logs.payloadDetail.emptyRequest') }}
            </p>
          </section>

          <section class="payload-detail-panel">
            <header class="payload-detail-panel__header">
              <span>{{ t('components.logs.payloadDetail.responseBody') }}</span>
              <div class="payload-detail-panel__actions">
                <span v-if="responsePayloadPreview.isJson" class="payload-detail-format-tag">JSON</span>
                <span v-if="detail?.response_body_truncated" class="payload-detail-truncated">
                  {{ t('components.logs.payloadDetail.truncated') }}
                </span>
                <button
                  type="button"
                  class="payload-detail-copy-btn"
                  :disabled="!responsePayloadPreview.rawText"
                  @click="copyPayloadDetail('response', 'raw')"
                >
                  {{ t('components.logs.payloadDetail.copyRaw') }}
                </button>
                <button
                  v-if="responsePayloadPreview.isFormatted && responsePayloadPreview.renderedText !== responsePayloadPreview.rawText"
                  type="button"
                  class="payload-detail-copy-btn"
                  @click="copyPayloadDetail('response', 'formatted')"
                >
                  {{ t('components.logs.payloadDetail.copyFormatted') }}
                </button>
              </div>
            </header>
            <pre
              v-if="responsePayloadPreview.renderedText"
              class="payload-detail-pre"
              :class="{ 'payload-detail-pre--json': responsePayloadPreview.isJson }"
            ><code class="payload-detail-code" v-html="responsePayloadPreview.html"></code></pre>
            <p v-if="responsePayloadPreview.formatSkippedLarge" class="payload-detail-note">
              {{ t('components.logs.payloadDetail.formatSkippedLarge') }}
            </p>
            <p v-else-if="!responsePayloadPreview.renderedText" class="cost-detail-empty">
              {{ t('components.logs.payloadDetail.emptyResponse') }}
            </p>
          </section>
        </div>
      </template>
    </div>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PayloadPreview } from '../../../utils/payloadPreview'
import BaseModal from '../../common/BaseModal.vue'
import type { RequestLog, RequestLogPayloadDetail } from '../../../services/logs'

type PayloadDetailKind = 'request' | 'response'
type PayloadCopyMode = 'raw' | 'formatted'

const emit = defineEmits<{
  (event: 'close'): void
}>()

const { t } = useI18n()

const props = defineProps<{
  open: boolean
  loading: boolean
  logId: number
  log: RequestLog | null
  detail: RequestLogPayloadDetail | null
  requestPayloadPreview: PayloadPreview
  responsePayloadPreview: PayloadPreview
  copyPayloadDetail: (kind: PayloadDetailKind, mode: PayloadCopyMode) => void | Promise<void>
}>()

function formatMilliseconds(value?: number) {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return '—'
  if (value === 0) return '0 ms'
  if (value < 1) return `${value.toFixed(2)} ms`
  return `${value.toFixed(1)} ms`
}

const hasPerformanceMetrics = computed(() => [
  props.log?.proxy_prepare_ms,
  props.log?.dns_ms,
  props.log?.connect_ms,
  props.log?.tls_ms,
  props.log?.upstream_ttfb_ms,
  props.log?.proxy_stream_delay_ms,
].some(value => typeof value === 'number' && Number.isFinite(value) && value > 0))

const performanceMetrics = computed(() => [
  { key: 'prepare', label: t('components.logs.payloadDetail.proxyPrepare'), value: formatMilliseconds(props.log?.proxy_prepare_ms) },
  { key: 'dns', label: t('components.logs.payloadDetail.dns'), value: formatMilliseconds(props.log?.dns_ms) },
  { key: 'connect', label: t('components.logs.payloadDetail.connect'), value: formatMilliseconds(props.log?.connect_ms) },
  { key: 'tls', label: t('components.logs.payloadDetail.tls'), value: formatMilliseconds(props.log?.tls_ms) },
  { key: 'ttfb', label: t('components.logs.payloadDetail.upstreamTtfb'), value: formatMilliseconds(props.log?.upstream_ttfb_ms) },
  { key: 'stream', label: t('components.logs.payloadDetail.streamDelay'), value: formatMilliseconds(props.log?.proxy_stream_delay_ms) },
])
</script>

<style scoped>
.payload-detail-modal {
  min-height: 280px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.payload-detail-meta {
  font-size: 0.8rem;
  color: #64748b;
}

.payload-performance {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid rgba(148, 163, 184, 0.28);
  border-radius: 8px;
  background: rgba(248, 250, 252, 0.78);
}

.payload-performance__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: #334155;
  font-size: 0.8rem;
  font-weight: 700;
}

.payload-performance__connection {
  color: #0f766e;
  font-size: 0.72rem;
  font-weight: 600;
}

.payload-performance__grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 8px;
  margin: 0;
}

.payload-performance__metric {
  min-width: 0;
}

.payload-performance__metric dt {
  color: #64748b;
  font-size: 0.68rem;
  line-height: 1.35;
}

.payload-performance__metric dd {
  margin: 2px 0 0;
  color: #0f172a;
  font-size: 0.78rem;
  font-variant-numeric: tabular-nums;
  font-weight: 650;
}

.payload-performance-empty {
  margin: 0;
  color: #64748b;
  font-size: 0.76rem;
}

html.dark .payload-performance {
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(15, 23, 42, 0.42);
}

html.dark .payload-performance__header,
html.dark .payload-performance__metric dd {
  color: #e2e8f0;
}

html.dark .payload-performance__connection {
  color: #5eead4;
}

@media (max-width: 900px) {
  .payload-performance__grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .payload-performance__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.payload-detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  min-height: 0;
}

.payload-detail-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.72);
  overflow: hidden;
}

.payload-detail-panel__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.26);
  font-size: 0.8rem;
  font-weight: 600;
  color: #334155;
}

.payload-detail-panel__actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.payload-detail-format-tag {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 2px 7px;
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.02em;
  color: #0f766e;
  background: rgba(20, 184, 166, 0.16);
  border: 1px solid rgba(15, 118, 110, 0.24);
}

.payload-detail-truncated {
  font-size: 0.7rem;
  color: #b45309;
  font-weight: 600;
}

.payload-detail-copy-btn {
  border: 1px solid rgba(148, 163, 184, 0.42);
  background: rgba(255, 255, 255, 0.85);
  color: #334155;
  border-radius: 7px;
  padding: 3px 10px;
  font-size: 0.72rem;
  font-weight: 600;
  line-height: 1.3;
  cursor: pointer;
  transition: border-color 0.16s ease, background 0.16s ease, color 0.16s ease;
}

.payload-detail-copy-btn:hover:not(:disabled) {
  border-color: rgba(14, 165, 233, 0.52);
  background: rgba(224, 242, 254, 0.75);
  color: #0c4a6e;
}

.payload-detail-copy-btn:disabled {
  opacity: 0.48;
  cursor: not-allowed;
}

.payload-detail-note {
  margin: 0;
  padding: 0 12px 10px;
  font-size: 0.72rem;
  color: #64748b;
}

.payload-detail-pre {
  flex: 1 1 auto;
  min-height: 220px;
  max-height: 52vh;
  margin: 0;
  padding: 12px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: 'SFMono-Regular', Menlo, Consolas, monospace;
  font-size: 0.78rem;
  line-height: 1.45;
  color: #0f172a;
  background: rgba(15, 23, 42, 0.03);
  -webkit-user-select: text;
  user-select: text;
}

.payload-detail-pre--json {
  white-space: pre;
  word-break: normal;
}

.payload-detail-code {
  display: block;
  min-width: 100%;
}

.payload-detail-pre--json :deep(.json-token.json-key) {
  color: #7c3aed;
}

.payload-detail-pre--json :deep(.json-token.json-string) {
  color: #0f766e;
}

.payload-detail-pre--json :deep(.json-token.json-number) {
  color: #0c4a6e;
}

.payload-detail-pre--json :deep(.json-token.json-boolean) {
  color: #9a3412;
}

.payload-detail-pre--json :deep(.json-token.json-null) {
  color: #64748b;
  font-style: italic;
}

html.dark .payload-detail-meta {
  color: #94a3b8;
}

html.dark .payload-detail-panel {
  border-color: rgba(148, 163, 184, 0.32);
  background: rgba(15, 23, 42, 0.4);
}

html.dark .payload-detail-panel__header {
  border-bottom-color: rgba(148, 163, 184, 0.24);
  color: #e2e8f0;
}

html.dark .payload-detail-truncated {
  color: #fbbf24;
}

html.dark .payload-detail-format-tag {
  color: #5eead4;
  background: rgba(15, 118, 110, 0.24);
  border-color: rgba(94, 234, 212, 0.36);
}

html.dark .payload-detail-copy-btn {
  border-color: rgba(148, 163, 184, 0.38);
  background: rgba(15, 23, 42, 0.72);
  color: #cbd5e1;
}

html.dark .payload-detail-copy-btn:hover:not(:disabled) {
  border-color: rgba(56, 189, 248, 0.64);
  background: rgba(12, 74, 110, 0.45);
  color: #e0f2fe;
}

html.dark .payload-detail-note {
  color: #94a3b8;
}

html.dark .payload-detail-pre {
  color: #e2e8f0;
  background: rgba(15, 23, 42, 0.55);
}

html.dark .payload-detail-pre--json :deep(.json-token.json-key) {
  color: #c4b5fd;
}

html.dark .payload-detail-pre--json :deep(.json-token.json-string) {
  color: #6ee7b7;
}

html.dark .payload-detail-pre--json :deep(.json-token.json-number) {
  color: #7dd3fc;
}

html.dark .payload-detail-pre--json :deep(.json-token.json-boolean) {
  color: #fdba74;
}

html.dark .payload-detail-pre--json :deep(.json-token.json-null) {
  color: #94a3b8;
}

@media (max-width: 960px) {
  .payload-detail-grid {
    grid-template-columns: 1fr;
  }

  .payload-detail-pre {
    max-height: 34vh;
  }
}
</style>
