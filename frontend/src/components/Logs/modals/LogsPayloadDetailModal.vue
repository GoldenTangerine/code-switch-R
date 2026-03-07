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
import { useI18n } from 'vue-i18n'
import type { PayloadPreview } from '../../../utils/payloadPreview'
import BaseModal from '../../common/BaseModal.vue'
import type { RequestLogPayloadDetail } from '../../../services/logs'

type PayloadDetailKind = 'request' | 'response'
type PayloadCopyMode = 'raw' | 'formatted'

defineProps<{
  open: boolean
  loading: boolean
  logId: number
  detail: RequestLogPayloadDetail | null
  requestPayloadPreview: PayloadPreview
  responsePayloadPreview: PayloadPreview
  copyPayloadDetail: (kind: PayloadDetailKind, mode: PayloadCopyMode) => void | Promise<void>
}>()

const emit = defineEmits<{
  (event: 'close'): void
}>()

const { t } = useI18n()
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
