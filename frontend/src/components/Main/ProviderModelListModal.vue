<template>
  <InlineModal
    :open="open"
    :title="modalTitle"
    :panel-width="'min(1280px, 98vw)'"
    @close="handleClose"
  >
    <div class="provider-model-modal">
      <div class="provider-model-toolbar">
        <div class="provider-model-meta">
          <span v-if="siteType" class="meta-pill">
            {{ t('components.main.modelList.siteType') }}：{{ siteType }}
          </span>
          <span v-if="pricingSourceLabel" class="meta-pill">
            {{ t('components.main.modelList.source') }}：{{ pricingSourceLabel }}
          </span>
          <span v-if="importedData" class="meta-pill meta-pill-accent">
            {{ t('components.main.modelList.importedBadge') }}
          </span>
        </div>

        <div class="provider-model-toolbar-main">
          <div class="provider-model-source-field">
            <span class="provider-model-source-row">
              <span class="provider-model-source-label">{{ t('components.main.modelList.sourcePicker') }}</span>
              <select
                v-model="selectedSource"
                class="mac-select provider-model-source-select"
                @change="handleSourceChange"
              >
                <option
                  v-for="option in sourceOptions"
                  :key="option.value"
                  :value="option.value"
                >
                  {{ option.label }}
                </option>
              </select>
            </span>
            <span class="provider-model-source-hint">
              {{ sourcePickerHintText }}
            </span>
            <span class="provider-model-source-footer">
              <span v-if="pricingSourceLabel" class="provider-model-source-current">
                {{ t('components.main.modelList.sourceResolved', { source: pricingSourceLabel }) }}
              </span>
              <span class="provider-model-source-actions">
                <button
                  type="button"
                  class="provider-model-debug-button"
                  :disabled="loading || !hasDebugDetails"
                  @click="openDebugModal()"
                >
                  {{ t('components.main.modelList.debugButton') }}
                </button>
                <button
                  type="button"
                  class="provider-model-import-button"
                  :disabled="loading"
                  @click="openImportModal"
                >
                  {{ t('components.main.modelList.importButton') }}
                </button>
              </span>
            </span>
          </div>
        </div>

        <div class="provider-model-search">
          <BaseInput
            v-model="searchTerm"
            type="text"
            :placeholder="t('components.main.modelList.searchPlaceholder')"
          />
        </div>

        <div v-if="vendorTabs.length > 1" class="provider-model-vendors">
          <button
            v-for="tab in vendorTabs"
            :key="tab.key"
            type="button"
            class="vendor-pill"
            :class="{ active: selectedVendor === tab.key }"
            @click="selectedVendor = tab.key"
          >
            {{ tab.label }} ({{ tab.count }})
          </button>
        </div>
      </div>

      <div v-if="loading" class="provider-model-state">
        {{ t('components.main.modelList.loading') }}
      </div>
      <div v-else-if="error" class="provider-model-state provider-model-state-stack error">
        <span>{{ error }}</span>
        <p v-if="showChallengeHint" class="provider-model-challenge-hint">
          {{ challengeMessage }}
        </p>
        <button
          v-if="hasDebugDetails"
          type="button"
          class="provider-model-inline-debug-btn"
          @click="openDebugModal()"
        >
          {{ t('components.main.modelList.debugOpenInline') }}
        </button>
      </div>
      <div v-else-if="filteredModels.length === 0" class="provider-model-state">
        {{ t('components.main.modelList.empty') }}
      </div>
      <div v-else class="provider-model-list">
        <p v-if="!pricingAvailable" class="pricing-hint">
          {{ t('components.main.modelList.pricingUnavailable') }}
        </p>
        <p v-else class="pricing-scroll-hint">
          {{ t('components.main.modelList.detailEntryHint') }}
        </p>
        <div
          v-for="model in filteredModels"
          :key="model.model"
          class="provider-model-item"
          :class="{ 'no-pricing': !pricingAvailable, clickable: pricingAvailable }"
          :role="pricingAvailable ? 'button' : undefined"
          :aria-haspopup="pricingAvailable ? 'dialog' : undefined"
          :aria-label="pricingAvailable ? t('components.main.modelList.openDetailAria', { model: model.model }) : undefined"
          :tabindex="pricingAvailable ? 0 : -1"
          @click="handleModelClick(model)"
          @keydown.enter.prevent="handleModelClick(model)"
          @keydown.space.prevent="handleModelClick(model)"
        >
          <div class="model-main">
            <div class="model-name" :title="model.model">{{ model.model }}</div>
            <div class="model-tags">
              <span class="tag" :class="billingTagClass(model.quotaType)">
                {{ billingLabel(model.quotaType) }}
              </span>
              <span v-if="model.ownerBy" class="tag tag-neutral">
                {{ model.ownerBy }}
              </span>
              <span v-if="hasManualCacheOverride(model)" class="tag tag-manual">
                {{ t('components.main.modelList.manualOverride') }}
              </span>
            </div>
          </div>

          <div
            v-if="pricingAvailable"
            class="pricing-inline-container"
            @pointerdown="onPricingPointerDown($event, model.model)"
            @pointermove="onPricingPointerMove"
            @pointerup="onPricingPointerEnd"
            @pointercancel="clearPricingInteraction"
            @click.stop="handlePricingAreaClick($event, model)"
          >
            <div class="model-pricing">
              <template v-if="model.quotaType === 0">
                <div class="price-block">
                  <span class="price-label">{{ t('components.main.modelList.input') }}</span>
                  <span class="price-value input">
                    {{ formatUSD(model.inputUsdPerM) }}/M
                  </span>
                </div>
                <div class="price-block">
                  <span class="price-label">{{ t('components.main.modelList.output') }}</span>
                  <span class="price-value output">
                    {{ formatUSD(model.outputUsdPerM) }}/M
                  </span>
                </div>
                <div
                  v-for="cacheItem in resolveCacheCreatePriceEntries(model)"
                  :key="`${model.model}-${cacheItem.key}`"
                  class="price-block"
                >
                  <span class="price-label">{{ cacheItem.label }}</span>
                  <span class="price-value cache-create">
                    {{ formatUSD(cacheItem.value) }}/M
                  </span>
                  <span v-if="cacheItem.hint" class="price-note" :class="cacheItem.hintClass">
                    {{ cacheItem.hint }}
                  </span>
                </div>
                <div class="price-block">
                  <span class="price-label">{{ t('components.main.modelList.cacheRead') }}</span>
                  <span class="price-value cache-read">
                    {{ formatUSD(resolveCachePrice(model.inputUsdPerM, resolveCacheReadMultiplier(model))) }}/M
                  </span>
                  <span v-if="resolveCacheReadHint(model)" class="price-note" :class="cacheHintClass(model.cacheReadMultiplierSource)">
                    {{ resolveCacheReadHint(model) }}
                  </span>
                </div>
              </template>
              <template v-else>
                <div class="price-block">
                  <span class="price-label">{{ t('components.main.modelList.perCall') }}</span>
                  <span class="price-value">
                    {{ formatPerCall(model.perCallPrice) }}
                  </span>
                </div>
              </template>
            </div>
          </div>

        </div>
      </div>
    </div>
  </InlineModal>

  <InlineModal
    :open="importModalOpen"
    :title="importModalTitle"
    :panel-width="'min(860px, 94vw)'"
    :close-disabled="importingJson"
    @close="requestImportModalClose"
  >
    <div class="provider-import-modal">
      <p class="pricing-hint import-hint">
        {{ t('components.main.modelList.importHint') }}
      </p>
      <BaseTextarea
        ref="importTextareaRef"
        v-model="importJsonInput"
        rows="14"
        class="provider-import-textarea"
        :placeholder="t('components.main.modelList.importPlaceholder')"
      />
      <p v-if="importError" class="provider-import-error">
        {{ importError }}
      </p>
      <div class="override-actions debug-actions">
        <button type="button" class="action-btn" :disabled="importingJson" @click="requestImportModalClose">
          {{ t('common.close') }}
        </button>
        <button
          type="button"
          class="action-btn"
          :disabled="importingJson || !hasImportDebugDetails"
          @click="openDebugModal('import')"
        >
          {{ t('components.main.modelList.debugButton') }}
        </button>
        <button
          type="button"
          class="primary-btn"
          :disabled="importingJson || !importJsonInput.trim()"
          @click="submitImportJson"
        >
          {{ importingJson ? t('components.main.modelList.importSubmitting') : t('components.main.modelList.importSubmit') }}
        </button>
      </div>
    </div>
  </InlineModal>

  <InlineModal
    :open="debugModalOpen"
    :title="debugModalTitle"
    :panel-width="'min(1120px, 96vw)'"
    @close="closeDebugModal"
  >
    <div class="provider-debug-modal">
      <p class="pricing-hint debug-hint">
        {{ t('components.main.modelList.debugHint') }}
      </p>

      <div class="detail-section-title">{{ t('components.main.modelList.debugSummaryTitle') }}</div>

      <div class="debug-summary-grid">
        <div class="debug-summary-card">
          <span class="detail-label">{{ t('components.main.modelList.debugSummaryRequestedSource') }}</span>
          <span class="detail-value detail-value--wrap">{{ debugRequestedSourceLabel }}</span>
        </div>
        <div class="debug-summary-card">
          <span class="detail-label">{{ t('components.main.modelList.debugSummaryResolvedSource') }}</span>
          <span class="detail-value detail-value--wrap">{{ debugResolvedSourceLabel }}</span>
        </div>
        <div class="debug-summary-card">
          <span class="detail-label">{{ t('components.main.modelList.debugSummaryConfiguredAuth') }}</span>
          <span class="detail-value detail-value--wrap">{{ debugConfiguredAuthLabel }}</span>
        </div>
        <div class="debug-summary-card">
          <span class="detail-label">{{ t('components.main.modelList.debugSummaryAuthCandidates') }}</span>
          <span class="detail-value detail-value--wrap">{{ debugAuthCandidatesLabel }}</span>
        </div>
        <div class="debug-summary-card">
          <span class="detail-label">{{ t('components.main.modelList.debugSummaryBaseUrl') }}</span>
          <span class="detail-value detail-value--wrap">{{ debugBaseUrl }}</span>
        </div>
        <div v-if="debugPlatformLabel" class="debug-summary-card">
          <span class="detail-label">{{ t('components.main.modelList.debugSummaryPlatform') }}</span>
          <span class="detail-value detail-value--wrap">{{ debugPlatformLabel }}</span>
        </div>
        <div v-if="debugFetchError" class="debug-summary-card debug-summary-card--error">
          <span class="detail-label">{{ t('components.main.modelList.debugSummaryFetchError') }}</span>
          <span class="detail-value detail-value--wrap">{{ debugFetchError }}</span>
        </div>
      </div>

      <p class="debug-summary-note">
        {{ t('components.main.modelList.debugSummaryMasked') }}
      </p>

      <div class="detail-section-title">{{ t('components.main.modelList.debugAttemptsTitle') }}</div>

      <div v-if="debugAttempts.length === 0" class="provider-model-state">
        {{ t('components.main.modelList.debugUnavailable') }}
      </div>
      <div v-else class="debug-attempts">
        <article
          v-for="(attempt, index) in debugAttempts"
          :key="`${attempt.source}-${attempt.endpoint}-${attempt.authType}-${index}`"
          class="debug-attempt-card"
        >
          <div class="debug-attempt-header">
            <div class="debug-attempt-title-wrap">
              <div class="debug-attempt-title">
                {{ t('components.main.modelList.debugAttemptLabel', { index: index + 1 }) }}
              </div>
              <div class="debug-attempt-meta">
                <span class="debug-pill">
                  {{ formatSourceLabel(attempt.source) }}
                </span>
                <span class="debug-pill">
                  {{ attempt.endpoint || '—' }}
                </span>
                <span class="debug-pill" :class="debugStatusClass(attempt)">
                  {{ formatDebugStatus(attempt) }}
                </span>
              </div>
            </div>

            <div class="debug-attempt-side">
              <span class="debug-attempt-side-item">
                {{ t('components.main.modelList.debugAttemptAuth') }}：{{ formatDebugAuthType(attempt.authType) }}
              </span>
              <span v-if="typeof attempt.durationMs === 'number'" class="debug-attempt-side-item">
                {{ t('components.main.modelList.debugAttemptDuration') }}：{{ formatDebugDuration(attempt.durationMs) }}
              </span>
            </div>
          </div>

          <div class="debug-attempt-grid">
            <section class="debug-block">
              <div class="debug-block-title">{{ t('components.main.modelList.debugRequestTitle') }}</div>
              <div class="debug-kv">
                <span class="debug-kv-label">{{ t('components.main.modelList.debugRequestUrl') }}</span>
                <code class="debug-inline-code">{{ `${attempt.method} ${attempt.url}`.trim() }}</code>
              </div>
              <div class="debug-kv">
                <span class="debug-kv-label">{{ t('components.main.modelList.debugRequestHeaders') }}</span>
                <pre class="debug-code-block">{{ formatDebugMap(attempt.requestHeaders) }}</pre>
              </div>
            </section>

            <section class="debug-block">
              <div class="debug-block-title">{{ t('components.main.modelList.debugResponseTitle') }}</div>
              <div class="debug-response-meta">
                <span>{{ t('components.main.modelList.debugAttemptStatus') }}：{{ formatDebugStatus(attempt) }}</span>
                <span v-if="attempt.contentType">content-type：{{ attempt.contentType }}</span>
                <span v-if="attempt.responseBodyBytes">
                  {{ t('components.main.modelList.debugResponseBodyMeta', { bytes: attempt.responseBodyBytes, truncated: attempt.responseBodyTruncated ? t('components.main.modelList.debugResponseBodyTruncated') : '' }) }}
                </span>
              </div>
              <div class="debug-kv">
                <span class="debug-kv-label">{{ t('components.main.modelList.debugResponseHeaders') }}</span>
                <pre class="debug-code-block">{{ formatDebugMap(attempt.responseHeaders) }}</pre>
              </div>
              <div class="debug-kv">
                <span class="debug-kv-label">{{ t('components.main.modelList.debugResponseBody') }}</span>
                <pre class="debug-code-block debug-code-block--body">{{ formatDebugBody(attempt.responseBody) }}</pre>
              </div>
              <p v-if="attempt.error" class="debug-attempt-error">
                {{ attempt.error }}
              </p>
            </section>
          </div>
        </article>
      </div>

      <div class="override-actions debug-actions">
        <button type="button" class="action-btn" @click="closeDebugModal">
          {{ t('common.close') }}
        </button>
        <button
          type="button"
          class="primary-btn"
          :disabled="debugAttempts.length === 0"
          @click="copyDebugDetails"
        >
          {{ t('components.main.modelList.debugCopy') }}
        </button>
      </div>
    </div>
  </InlineModal>

  <InlineModal
    :open="Boolean(editingTargetModel)"
    :title="detailModalTitle"
    :panel-width="'min(980px, 96vw)'"
    :body-scrollable="false"
    @close="closeModelDetail"
  >
    <div v-if="editingTargetModel" class="model-detail-modal">
      <p class="pricing-hint detail-hint">
        {{ t('components.main.modelList.detailHint') }}
      </p>

      <div class="detail-grid">
        <div class="detail-item detail-item--full">
          <span class="detail-label">{{ t('components.main.modelList.detailModel') }}</span>
          <span class="detail-value detail-value--wrap" :title="editingTargetModel.model">
            {{ editingTargetModel.model }}
          </span>
        </div>

        <div class="detail-item">
          <span class="detail-label">{{ t('components.main.modelList.detailBilling') }}</span>
          <span class="detail-value">{{ billingLabel(editingTargetModel.quotaType) }}</span>
        </div>

        <div v-if="editingTargetModel.ownerBy" class="detail-item">
          <span class="detail-label">{{ t('components.main.modelList.detailOwner') }}</span>
          <span class="detail-value detail-value--wrap">{{ editingTargetModel.ownerBy }}</span>
        </div>

        <template v-if="editingTargetModel.quotaType === 0">
          <div class="detail-item">
            <span class="detail-label">{{ t('components.main.modelList.detailInput') }}</span>
            <span class="detail-value input">{{ formatUSD(editingTargetModel.inputUsdPerM) }}/M</span>
          </div>

          <div class="detail-item">
            <span class="detail-label">{{ t('components.main.modelList.detailOutput') }}</span>
            <span class="detail-value output">{{ formatUSD(editingTargetModel.outputUsdPerM) }}/M</span>
          </div>

          <div
            v-for="cacheItem in resolveCacheCreatePriceEntries(editingTargetModel)"
            :key="`detail-${editingTargetModel.model}-${cacheItem.key}`"
            class="detail-item"
          >
            <span class="detail-label">{{ cacheItem.detailLabel }}</span>
            <span class="detail-value cache-create">{{ formatUSD(cacheItem.value) }}/M</span>
            <span v-if="cacheItem.hint" class="detail-note" :class="cacheItem.hintClass">
              {{ cacheItem.hint }}
            </span>
          </div>

          <div class="detail-item">
            <span class="detail-label">{{ t('components.main.modelList.detailCacheRead') }}</span>
            <span class="detail-value cache-read">
              {{ formatUSD(resolveCachePrice(editingTargetModel.inputUsdPerM, resolveCacheReadMultiplier(editingTargetModel))) }}/M
            </span>
            <span v-if="resolveCacheReadHint(editingTargetModel)" class="detail-note" :class="cacheHintClass(editingTargetModel.cacheReadMultiplierSource)">
              {{ resolveCacheReadHint(editingTargetModel) }}
            </span>
          </div>

          <div v-if="editingTargetModel.modelRatio > 0" class="detail-item">
            <span class="detail-label">{{ t('components.main.modelList.detailRatio') }}</span>
            <span class="detail-value">{{ formatRatio(editingTargetModel.modelRatio) }}</span>
          </div>

          <div class="detail-item">
            <span class="detail-label">{{ resolveCacheCreateMultiplierLabel(editingTargetModel, 'detail') }}</span>
            <span class="detail-value">{{ formatMultiplier(resolveCacheCreateMultiplier(editingTargetModel)) }}</span>
            <span v-if="formatCacheMultiplierSource(editingTargetModel.cacheCreateMultiplierSource)" class="detail-note" :class="cacheHintClass(editingTargetModel.cacheCreateMultiplierSource)">
              {{ formatCacheMultiplierSource(editingTargetModel.cacheCreateMultiplierSource) }}
            </span>
          </div>

          <div class="detail-item">
            <span class="detail-label">{{ t('components.main.modelList.detailCacheReadMultiplier') }}</span>
            <span class="detail-value">{{ formatMultiplier(resolveCacheReadMultiplier(editingTargetModel)) }}</span>
            <span v-if="formatCacheMultiplierSource(editingTargetModel.cacheReadMultiplierSource)" class="detail-note" :class="cacheHintClass(editingTargetModel.cacheReadMultiplierSource)">
              {{ formatCacheMultiplierSource(editingTargetModel.cacheReadMultiplierSource) }}
            </span>
          </div>
        </template>

        <div v-else class="detail-item detail-item--full">
          <span class="detail-label">{{ t('components.main.modelList.detailPerCall') }}</span>
          <span class="detail-value detail-value--wrap">{{ formatPerCall(editingTargetModel.perCallPrice) }}</span>
        </div>
      </div>

      <template v-if="editingTargetModel.quotaType === 0">
        <div class="detail-divider"></div>

        <div class="detail-section-title">{{ t('components.main.modelList.detailOverrideSection') }}</div>

        <div class="override-editor-grid">
          <div class="override-field">
            <label class="override-label">{{ resolveCacheCreateMultiplierLabel(editingTargetModel, 'input') }}</label>
            <input
              v-model="overrideForm.cacheCreateMultiplier"
              type="number"
              step="0.0001"
              min="0"
              class="mac-input override-input"
              :placeholder="resolveOverridePlaceholder(editingTargetModel, 'create')"
            />
            <span class="override-field-hint">{{ resolveOverrideHint(editingTargetModel, 'create') }}</span>
          </div>

          <div class="override-field">
            <label class="override-label">{{ t('components.main.modelList.cacheReadMultiplier') }}</label>
            <input
              v-model="overrideForm.cacheReadMultiplier"
              type="number"
              step="0.0001"
              min="0"
              class="mac-input override-input"
              :placeholder="resolveOverridePlaceholder(editingTargetModel, 'read')"
            />
            <span class="override-field-hint">{{ resolveOverrideHint(editingTargetModel, 'read') }}</span>
          </div>
        </div>
      </template>

      <div class="override-actions">
        <button type="button" class="action-btn" @click="closeModelDetail">
          {{ t('common.cancel') }}
        </button>
        <button
          v-if="editingTargetModel.quotaType === 0 && hasManualCacheOverride(editingTargetModel)"
          type="button"
          class="action-btn"
          :disabled="isWorkingModel(editingTargetModel.model)"
          @click="resetOverride(editingTargetModel)"
        >
          {{ resettingModel === editingTargetModel.model ? t('components.general.modelPricing.removing') : t('components.main.modelList.resetCache') }}
        </button>
        <button
          v-if="editingTargetModel.quotaType === 0"
          type="button"
          class="primary-btn"
          :disabled="isWorkingModel(editingTargetModel.model)"
          @click="saveOverride(editingTargetModel)"
        >
          {{ savingModel === editingTargetModel.model ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </div>
  </InlineModal>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseInput from '../common/BaseInput.vue'
import BaseTextarea from '../common/BaseTextarea.vue'
import InlineModal from '../common/InlineModal.vue'
import type { AutomationCard } from '../../data/cards'
import { extractErrorMessage } from '../../utils/error'
import { showToast } from '../../utils/toast'
import {
  deleteProviderModelPricingOverride,
  fetchProviderModelPricing,
  importProviderModelPricingJSON,
  type ProviderModelPricingDebug,
  type ProviderModelPricingDebugAttempt,
  type ProviderModelPerCallPrice,
  type ProviderModelPricingItem,
  type ProviderModelPricingResponse,
  type ProviderModelPricingSource,
  upsertProviderModelPricingOverride,
} from '../../services/providerModelPricing'

type ProviderVendorKey = 'all' | 'OpenAI' | 'Claude' | 'Gemini' | 'Moonshot' | 'Grok' | 'DeepSeek' | 'Qwen' | 'Mistral' | 'Unknown'

type ProviderVendorTab = {
  key: ProviderVendorKey
  label: string
  count: number
}

type ProviderModelSourceOption = {
  value: ProviderModelPricingSource
  label: string
}

const props = defineProps<{
  open: boolean
  provider: AutomationCard | null
  platform: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()

const loading = ref(false)
const error = ref('')
const response = ref<ProviderModelPricingResponse | null>(null)
const loadRequestSeq = ref(0)
const importRequestSeq = ref(0)

const searchTerm = ref('')
const selectedVendor = ref<ProviderVendorKey>('all')
const selectedSource = ref<ProviderModelPricingSource>('auto')
const editingModel = ref('')
const savingModel = ref('')
const resettingModel = ref('')
const debugModalOpen = ref(false)
const importModalOpen = ref(false)
const importingJson = ref(false)
const importJsonInput = ref('')
const importError = ref('')
const importDebugResponse = ref<ProviderModelPricingResponse | null>(null)
const importTextareaRef = ref<InstanceType<typeof BaseTextarea> | null>(null)
const debugContext = ref<'main' | 'import'>('main')

const pricingInteraction = reactive({
  model: '',
  startX: 0,
  startY: 0,
  dragged: false,
  pointerId: null as number | null,
})

const overrideForm = reactive({
  cacheCreateMultiplier: '',
  cacheReadMultiplier: '',
})

const modalTitle = computed(() => {
  if (!props.provider) return t('components.main.modelList.modalTitleFallback')
  return t('components.main.modelList.modalTitle', { name: props.provider.name })
})

const siteType = computed(() => response.value?.siteType ?? '')
const pricingSource = computed<ProviderModelPricingSource | ''>(() => response.value?.pricingSource ?? '')
const models = computed(() => response.value?.models ?? [])
const pricingAvailable = computed(() => response.value?.pricingSource !== 'v1/models')
const importedData = computed(() => Boolean(response.value?.imported))
const challengeDetected = computed(() => Boolean(response.value?.challengeDetected))
const challengeMessage = computed(() => response.value?.challengeMessage?.trim() || '')
const showChallengeHint = computed(() => Boolean(
  challengeDetected.value &&
  challengeMessage.value &&
  challengeMessage.value !== error.value.trim(),
))
const sourceLabelMap = computed<Record<ProviderModelPricingSource, string>>(() => ({
  auto: t('components.main.modelList.sourceAuto'),
  'api/pricing': t('components.main.modelList.sourceApiPricing'),
  'one-hub': t('components.main.modelList.sourceOneHub'),
  'v1/models': t('components.main.modelList.sourceModels'),
}))

const formatSourceLabel = (source?: ProviderModelPricingSource | string) => {
  if (!source) return ''
  return sourceLabelMap.value[source as ProviderModelPricingSource] || source
}

const pricingSourceLabel = computed(() => formatSourceLabel(pricingSource.value))
const sourcePickerHintText = computed(() => (
  selectedSource.value === 'auto'
    ? t('components.main.modelList.sourcePickerAutoHint')
    : t('components.main.modelList.sourcePickerFixedHint')
))
const sourceOptions = computed<ProviderModelSourceOption[]>(() => [
  { value: 'auto', label: sourceLabelMap.value.auto },
  { value: 'api/pricing', label: sourceLabelMap.value['api/pricing'] },
  { value: 'one-hub', label: sourceLabelMap.value['one-hub'] },
  { value: 'v1/models', label: sourceLabelMap.value['v1/models'] },
])

const activeDebugResponse = computed<ProviderModelPricingResponse | null>(() => (
  debugContext.value === 'import' ? importDebugResponse.value : response.value
))
const debugInfo = computed<ProviderModelPricingDebug | null>(() => activeDebugResponse.value?.debug ?? null)
const debugAttempts = computed<ProviderModelPricingDebugAttempt[]>(() => debugInfo.value?.attempts ?? [])
const hasDebugDetails = computed(() => Boolean(response.value?.debug?.attempts?.length))
const hasImportDebugDetails = computed(() => Boolean(importDebugResponse.value?.debug?.attempts?.length))
const debugFetchError = computed(() => activeDebugResponse.value?.fetchError?.trim() || '')
const debugBaseUrl = computed(() => debugInfo.value?.baseUrl || props.provider?.apiUrl || '—')
const debugPlatformLabel = computed(() => debugInfo.value?.platform?.trim() || props.platform || '')
const debugRequestedSourceLabel = computed(() => (
  formatSourceLabel(debugInfo.value?.requestedSource) ||
  (debugContext.value === 'import' ? formatSourceLabel('api/pricing') : formatSourceLabel(selectedSource.value)) ||
  '—'
))
const debugResolvedSourceLabel = computed(() => (
  formatSourceLabel(debugInfo.value?.resolvedSource) ||
  formatSourceLabel(activeDebugResponse.value?.pricingSource) ||
  '—'
))

const identifyVendor = (modelName: string, ownerBy?: string): ProviderVendorKey => {
  const raw = `${ownerBy || ''} ${modelName || ''}`.toLowerCase()
  if (/\bgpt\b|whisper|text-embedding|\bo\d+/.test(raw)) return 'OpenAI'
  if (/claude|sonnet|haiku|opus|anthropic/.test(raw)) return 'Claude'
  if (/gemini/.test(raw)) return 'Gemini'
  if (/moonshot|kimi/.test(raw)) return 'Moonshot'
  if (/grok/.test(raw)) return 'Grok'
  if (/deepseek/.test(raw)) return 'DeepSeek'
  if (/qwen/.test(raw)) return 'Qwen'
  if (/mistral|mixtral|codestral|pixtral|ministral|magistral/.test(raw)) return 'Mistral'
  return 'Unknown'
}

const vendorLabelMap = computed<Record<ProviderVendorKey, string>>(() => ({
  all: t('components.main.modelList.vendorAll'),
  OpenAI: 'OpenAI',
  Claude: 'Claude',
  Gemini: 'Gemini',
  Moonshot: 'Moonshot',
  Grok: 'Grok',
  DeepSeek: 'DeepSeek',
  Qwen: 'Qwen',
  Mistral: 'Mistral',
  Unknown: t('components.main.modelList.vendorUnknown'),
}))

const vendorTabs = computed<ProviderVendorTab[]>(() => {
  const counts = new Map<ProviderVendorKey, number>()
  for (const item of models.value) {
    const vendor = identifyVendor(item.model, item.ownerBy)
    counts.set(vendor, (counts.get(vendor) || 0) + 1)
  }

  const tabs: ProviderVendorTab[] = [{ key: 'all', label: vendorLabelMap.value.all, count: models.value.length }]
  const entries = [...counts.entries()].filter(([key]) => key !== 'Unknown').sort((a, b) => b[1] - a[1])
  for (const [key, count] of entries) {
    if (count <= 0) continue
    tabs.push({ key, label: vendorLabelMap.value[key], count })
  }

  const unknownCount = counts.get('Unknown') || 0
  if (unknownCount > 0) {
    tabs.push({ key: 'Unknown', label: vendorLabelMap.value.Unknown, count: unknownCount })
  }
  return tabs
})

const filteredModels = computed(() => {
  const term = searchTerm.value.trim().toLowerCase()
  const vendor = selectedVendor.value

  const filtered = models.value.filter((item) => {
    if (vendor !== 'all') {
      const detected = identifyVendor(item.model, item.ownerBy)
      if (detected !== vendor) return false
    }
    if (!term) return true
    return (item.model || '').toLowerCase().includes(term)
  })

  return [...filtered].sort((a, b) => (a.model || '').localeCompare(b.model || ''))
})

const editingTargetModel = computed(() => models.value.find((item) => item.model === editingModel.value) ?? null)

const detailModalTitle = computed(() => {
  const baseTitle = t('components.main.modelList.detailTitle')
  if (!editingTargetModel.value?.model) return baseTitle
  return `${baseTitle} · ${editingTargetModel.value.model}`
})

const importModalTitle = computed(() => {
  if (!props.provider) return t('components.main.modelList.importTitleFallback')
  return t('components.main.modelList.importTitle', { name: props.provider.name })
})

const debugModalTitle = computed(() => {
  if (debugContext.value === 'import') {
    if (!props.provider) return t('components.main.modelList.importDebugTitleFallback')
    return t('components.main.modelList.importDebugTitle', { name: props.provider.name })
  }
  if (!props.provider) return t('components.main.modelList.debugTitleFallback')
  return t('components.main.modelList.debugTitle', { name: props.provider.name })
})

const isLikelyChallengeScript = (value: string) => {
  const raw = String(value ?? '').toLowerCase()
  return raw.includes('acw_sc__v2') ||
    (raw.includes('document.cookie') && raw.includes('location.reload'))
}

const formatDebugAuthType = (authType?: string, kind: 'configured' | 'attempt' = 'attempt') => {
  const trimmed = String(authType ?? '').trim()
  if (!trimmed) {
    return kind === 'configured'
      ? t('components.main.modelList.debugAuthAuto')
      : t('components.main.modelList.debugAuthEmpty')
  }
  const normalized = trimmed.toLowerCase()
  if (normalized === 'bearer') return 'Authorization: Bearer'
  if (normalized === 'x-api-key') return 'x-api-key'
  return trimmed
}

const debugConfiguredAuthLabel = computed(() => formatDebugAuthType(debugInfo.value?.configuredAuthType, 'configured'))

const debugAuthCandidatesLabel = computed(() => {
  const candidates = (debugInfo.value?.authCandidates ?? [])
    .map((candidate) => formatDebugAuthType(candidate))
    .filter(Boolean)
  return candidates.length > 0 ? candidates.join(' / ') : t('components.main.modelList.debugAuthEmpty')
})

const formatDebugMap = (value?: Record<string, string>) => {
  if (!value || Object.keys(value).length === 0) {
    return t('components.main.modelList.debugEmptyBlock')
  }
  return JSON.stringify(value, null, 2)
}

const formatDebugBody = (body?: string) => {
  const raw = String(body ?? '')
  const trimmed = raw.trim()
  if (!trimmed) return t('components.main.modelList.debugEmptyBlock')
  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2)
    } catch {
      return raw
    }
  }
  return raw
}

const formatDebugStatus = (attempt: ProviderModelPricingDebugAttempt) => {
  if (typeof attempt.statusCode === 'number' && attempt.statusCode > 0) {
    return `HTTP ${attempt.statusCode}`
  }
  if (attempt.error) {
    return t('common.failed')
  }
  return '—'
}

const debugStatusClass = (attempt: ProviderModelPricingDebugAttempt) => {
  const statusCode = typeof attempt.statusCode === 'number' ? attempt.statusCode : 0
  return {
    'is-success': statusCode >= 200 && statusCode < 300 && !attempt.error,
    'is-error': Boolean(attempt.error) || statusCode >= 400,
    'is-warning': statusCode >= 300 && statusCode < 400,
  }
}

const formatDebugDuration = (durationMs?: number) => {
  if (typeof durationMs !== 'number' || !Number.isFinite(durationMs) || durationMs < 0) return '—'
  return `${Math.round(durationMs)} ms`
}

const copyTextFallback = (text: string) => {
  if (typeof document === 'undefined') return false
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', 'true')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  textarea.style.pointerEvents = 'none'
  document.body.appendChild(textarea)
  textarea.select()
  textarea.setSelectionRange(0, textarea.value.length)
  const copied = document.execCommand('copy')
  document.body.removeChild(textarea)
  return copied
}

const buildDebugCopyPayload = () => {
  if (!debugInfo.value) return ''

  const lines: string[] = [
    `[${t('components.main.modelList.debugSummaryTitle')}]`,
    `${t('components.main.modelList.debugSummaryRequestedSource')}: ${debugRequestedSourceLabel.value}`,
    `${t('components.main.modelList.debugSummaryResolvedSource')}: ${debugResolvedSourceLabel.value}`,
    `${t('components.main.modelList.debugSummaryConfiguredAuth')}: ${debugConfiguredAuthLabel.value}`,
    `${t('components.main.modelList.debugSummaryAuthCandidates')}: ${debugAuthCandidatesLabel.value}`,
    `${t('components.main.modelList.debugSummaryBaseUrl')}: ${debugBaseUrl.value}`,
  ]

  if (debugPlatformLabel.value) {
    lines.push(`${t('components.main.modelList.debugSummaryPlatform')}: ${debugPlatformLabel.value}`)
  }
  if (debugFetchError.value) {
    lines.push(`${t('components.main.modelList.debugSummaryFetchError')}: ${debugFetchError.value}`)
  }

  for (const [index, attempt] of debugAttempts.value.entries()) {
    lines.push('')
    lines.push(`[${t('components.main.modelList.debugAttemptLabel', { index: index + 1 })}]`)
    lines.push(`${t('components.main.modelList.debugAttemptSource')}: ${formatSourceLabel(attempt.source) || '—'}`)
    lines.push(`${t('components.main.modelList.debugAttemptEndpoint')}: ${attempt.endpoint || '—'}`)
    lines.push(`${t('components.main.modelList.debugAttemptAuth')}: ${formatDebugAuthType(attempt.authType)}`)
    lines.push(`${t('components.main.modelList.debugAttemptStatus')}: ${formatDebugStatus(attempt)}`)
    lines.push(`${t('components.main.modelList.debugAttemptDuration')}: ${formatDebugDuration(attempt.durationMs)}`)
    lines.push(`${t('components.main.modelList.debugRequestUrl')}: ${`${attempt.method} ${attempt.url}`.trim()}`)
    lines.push(`${t('components.main.modelList.debugRequestHeaders')}:\n${formatDebugMap(attempt.requestHeaders)}`)
    lines.push(`${t('components.main.modelList.debugResponseHeaders')}:\n${formatDebugMap(attempt.responseHeaders)}`)
    lines.push(`${t('components.main.modelList.debugResponseBody')}:\n${formatDebugBody(attempt.responseBody)}`)
    if (attempt.error) {
      lines.push(`Error: ${attempt.error}`)
    }
  }

  return lines.join('\n')
}

const openDebugModal = (context: 'main' | 'import' = 'main') => {
  const targetResponse = context === 'import' ? importDebugResponse.value : response.value
  if ((targetResponse?.debug?.attempts ?? []).length === 0) {
    showToast(t('components.main.modelList.debugUnavailable'), 'warning')
    return
  }
  debugContext.value = context
  debugModalOpen.value = true
}

const closeDebugModal = () => {
  debugModalOpen.value = false
  debugContext.value = 'main'
}

const copyDebugDetails = async () => {
  const payload = buildDebugCopyPayload()
  if (!payload) {
    showToast(t('components.main.modelList.debugUnavailable'), 'warning')
    return
  }
  try {
    if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(payload)
    } else if (!copyTextFallback(payload)) {
      throw new Error(t('components.main.modelList.debugCopyUnavailable'))
    }
    showToast(t('components.main.modelList.toast.debugCopied'))
  } catch (err) {
    showToast(
      t('components.main.modelList.toast.debugCopyFailed', { error: extractErrorMessage(err) }),
      'error',
    )
  }
}

const resetImportState = () => {
  importJsonInput.value = ''
  importError.value = ''
  importingJson.value = false
  importDebugResponse.value = null
}

const openImportModal = () => {
  importModalOpen.value = true
  importError.value = ''
  importDebugResponse.value = null
  nextTick(() => importTextareaRef.value?.focus())
}

const closeImportModal = (force = false) => {
  if (importingJson.value && !force) return
  importRequestSeq.value += 1
  importModalOpen.value = false
  resetImportState()
}

const requestImportModalClose = () => {
  closeImportModal()
}

const submitImportJson = async () => {
  if (!props.provider || importingJson.value) return
  const raw = importJsonInput.value.trim()
  if (!raw) {
    importError.value = t('components.main.modelList.importEmpty')
    return
  }
  if (isLikelyChallengeScript(raw)) {
    importError.value = t('components.main.modelList.importChallengeScript')
    return
  }

  const requestSeq = importRequestSeq.value + 1
  importRequestSeq.value = requestSeq
  importError.value = ''
  importDebugResponse.value = null
  importingJson.value = true
  try {
    const data = await importProviderModelPricingJSON(props.provider, props.platform, raw)
    if (requestSeq !== importRequestSeq.value) return
    if (data.fetchError?.trim()) {
      importDebugResponse.value = data
      importError.value = data.fetchError.trim()
      return
    }
    response.value = data
    error.value = data.fetchError?.trim() || ''
    searchTerm.value = ''
    selectedSource.value = 'api/pricing'
    selectedVendor.value = 'all'
    closeDebugModal()
    closeModelDetail()
    clearPricingInteraction()
    closeImportModal(true)
    showToast(t('components.main.modelList.toast.importSuccess'))
  } catch (err) {
    if (requestSeq !== importRequestSeq.value) return
    importError.value = extractErrorMessage(err)
  } finally {
    if (requestSeq !== importRequestSeq.value) return
    importingJson.value = false
  }
}

const formatUSD = (value?: number) => {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '—'
  if (value === 0) return '$0'
  if (value < 0) return '—'
  if (value < 0.01) return `$${value.toFixed(6)}`
  if (value < 1) return `$${value.toFixed(4)}`
  return `$${value.toFixed(2)}`
}

const formatRatio = (value: number) => {
  if (!Number.isFinite(value) || value <= 0) return '—'
  if (Math.abs(value - Math.round(value)) < 1e-9) return `${Math.round(value)}x`
  return `${value.toFixed(2)}x`
}

const formatMultiplier = (value?: number) => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return '—'
  if (Math.abs(value - Math.round(value)) < 1e-9) return `${Math.round(value)}x`
  if (value < 0.01) return `${value.toFixed(4)}x`
  return `${value.toFixed(2)}x`
}

const formatEditableMultiplier = (value?: number) => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return ''
  if (value === 0) return '0'
  const digits = value >= 10 ? 2 : value >= 1 ? 3 : value >= 0.1 ? 4 : 6
  return value.toFixed(digits).replace(/\.?0+$/, '')
}

const formatMultiplierHint = (value?: number) => {
  const formatted = formatMultiplier(value)
  if (formatted === '—') return ''
  return `${t('components.main.modelList.multiplier')} ${formatted}`
}

type CacheCreatePriceEntry = {
  key: string
  label: string
  detailLabel: string
  value: number
  hint: string
  hintClass?: Record<string, boolean>
}

const calculatePriceMultiplier = (value?: number, inputUsdPerM?: number) => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return undefined
  if (typeof inputUsdPerM !== 'number' || !Number.isFinite(inputUsdPerM) || inputUsdPerM < 0) return undefined
  if (inputUsdPerM === 0) return value === 0 ? 0 : undefined
  return value / inputUsdPerM
}

const resolveCachePrice = (inputUsdPerM?: number, multiplier?: number) => {
  if (typeof inputUsdPerM !== 'number' || !Number.isFinite(inputUsdPerM) || inputUsdPerM < 0) return undefined
  if (typeof multiplier !== 'number' || !Number.isFinite(multiplier) || multiplier < 0) return undefined
  return inputUsdPerM * multiplier
}

const resolveCacheCreateMultiplier = (model: ProviderModelPricingItem) => {
  if (typeof model.resolvedCacheCreateMultiplier === 'number') return model.resolvedCacheCreateMultiplier
  return model.cacheCreateMultiplier
}

const resolveCacheReadMultiplier = (model: ProviderModelPricingItem) => {
  if (typeof model.resolvedCacheReadMultiplier === 'number') return model.resolvedCacheReadMultiplier
  return model.cacheReadMultiplier
}

const formatCacheMultiplierSource = (source?: string) => {
  if (source === 'manual') return t('components.main.modelList.cacheSourceManual')
  if (source === 'provider') return t('components.main.modelList.cacheSourceProvider')
  if (source === 'builtin') return t('components.main.modelList.cacheSourceBuiltin')
  if (source === 'fallback') return t('components.main.modelList.cacheSourceFallback')
  return ''
}

const buildCacheHint = (multiplier?: number, source?: string) => {
  const multiplierHint = formatMultiplierHint(multiplier)
  const sourceHint = formatCacheMultiplierSource(source)
  if (multiplierHint && sourceHint) return `${multiplierHint} · ${sourceHint}`
  return multiplierHint || sourceHint || ''
}

const resolveCacheCreateHint = (model: ProviderModelPricingItem) =>
  buildCacheHint(resolveCacheCreateMultiplier(model), model.cacheCreateMultiplierSource)

const resolveCacheReadHint = (model: ProviderModelPricingItem) =>
  buildCacheHint(resolveCacheReadMultiplier(model), model.cacheReadMultiplierSource)

const cacheHintClass = (source?: string) => ({
  'is-estimated': source === 'builtin' || source === 'fallback',
  'is-manual': source === 'manual',
})

const hasExplicit1hCacheCreate = (model: ProviderModelPricingItem) =>
  typeof model.cacheCreate1hUsdPerM === 'number' && Number.isFinite(model.cacheCreate1hUsdPerM) && model.cacheCreate1hUsdPerM > 0

const resolveCacheCreateMultiplierLabel = (
  model: ProviderModelPricingItem,
  kind: 'detail' | 'input',
) => {
  if (!hasExplicit1hCacheCreate(model)) {
    return kind === 'detail'
      ? t('components.main.modelList.detailCacheCreateMultiplier')
      : t('components.main.modelList.cacheCreateMultiplier')
  }
  return kind === 'detail'
    ? t('components.main.modelList.detailCacheCreate5mMultiplier')
    : t('components.main.modelList.cacheCreate5mMultiplier')
}

const resolveCacheCreatePriceEntries = (model: ProviderModelPricingItem): CacheCreatePriceEntry[] => {
  const entries: CacheCreatePriceEntry[] = []
  const cacheCreatePrice = resolveCachePrice(model.inputUsdPerM, resolveCacheCreateMultiplier(model))
  const cacheCreate1hPrice = hasExplicit1hCacheCreate(model) ? model.cacheCreate1hUsdPerM : undefined
  const createHint = resolveCacheCreateHint(model)
  const createHintClass = cacheHintClass(model.cacheCreateMultiplierSource)

  const has1hCache = typeof cacheCreate1hPrice === 'number' && cacheCreate1hPrice > 0
  const shouldShowBaseCache = typeof cacheCreatePrice === 'number' && (cacheCreatePrice > 0 || !has1hCache)

  if (shouldShowBaseCache) {
    if (has1hCache) {
      entries.push({
        key: 'cache-create-5m',
        label: t('components.main.modelList.cacheCreate5m'),
        detailLabel: t('components.main.modelList.detailCacheCreate5m'),
        value: cacheCreatePrice,
        hint: createHint,
        hintClass: createHintClass,
      })
    } else {
      entries.push({
        key: 'cache-create',
        label: t('components.main.modelList.cacheCreate'),
        detailLabel: t('components.main.modelList.detailCacheCreate'),
        value: cacheCreatePrice,
        hint: createHint,
        hintClass: createHintClass,
      })
    }
  }

  if (has1hCache) {
    entries.push({
      key: 'cache-create-1h',
      label: t('components.main.modelList.cacheCreate1h'),
      detailLabel: t('components.main.modelList.detailCacheCreate1h'),
      value: cacheCreate1hPrice,
      hint: formatMultiplierHint(calculatePriceMultiplier(cacheCreate1hPrice, model.inputUsdPerM)),
    })
  }

  return entries
}

const formatPerCall = (value?: ProviderModelPerCallPrice) => {
  if (!value) return '—'
  if (typeof value.unified === 'number' && Number.isFinite(value.unified)) {
    return formatUSD(value.unified)
  }
  const input = typeof value.input === 'number' ? value.input : undefined
  const output = typeof value.output === 'number' ? value.output : undefined
  if (typeof input === 'number' && typeof output === 'number') {
    return `${t('components.main.modelList.input')}${formatUSD(input)} · ${t('components.main.modelList.output')}${formatUSD(output)}`
  }
  return '—'
}

const billingLabel = (quotaType: number) => {
  if (quotaType === 0) return t('components.main.modelList.billingToken')
  if (quotaType === 1) return t('components.main.modelList.billingCall')
  return t('components.main.modelList.billingUnknown')
}

const billingTagClass = (quotaType: number) => {
  if (quotaType === 0) return 'tag-token'
  if (quotaType === 1) return 'tag-call'
  return 'tag-neutral'
}

const clearOverrideForm = () => {
  overrideForm.cacheCreateMultiplier = ''
  overrideForm.cacheReadMultiplier = ''
}

const isWorkingModel = (modelName: string) => savingModel.value === modelName || resettingModel.value === modelName

const hasManualCacheOverride = (model: ProviderModelPricingItem) =>
  model.cacheCreateMultiplierSource === 'manual' || model.cacheReadMultiplierSource === 'manual'

const openModelDetail = (model: ProviderModelPricingItem) => {
  editingModel.value = model.model
  if (model.quotaType !== 0) {
    clearOverrideForm()
    return
  }
  overrideForm.cacheCreateMultiplier =
    model.cacheCreateMultiplierSource === 'manual' ? formatEditableMultiplier(resolveCacheCreateMultiplier(model)) : ''
  overrideForm.cacheReadMultiplier =
    model.cacheReadMultiplierSource === 'manual' ? formatEditableMultiplier(resolveCacheReadMultiplier(model)) : ''
}

const closeModelDetail = () => {
  editingModel.value = ''
  clearOverrideForm()
}

const clearPricingInteraction = () => {
  pricingInteraction.model = ''
  pricingInteraction.startX = 0
  pricingInteraction.startY = 0
  pricingInteraction.dragged = false
  pricingInteraction.pointerId = null
}

const onPricingPointerDown = (event: PointerEvent, modelName: string) => {
  if (!pricingAvailable.value || !event.isPrimary) return
  if (event.pointerType === 'mouse' && event.button !== 0) return
  pricingInteraction.model = modelName
  pricingInteraction.startX = event.clientX
  pricingInteraction.startY = event.clientY
  pricingInteraction.dragged = false
  pricingInteraction.pointerId = event.pointerId
}

const onPricingPointerMove = (event: PointerEvent) => {
  if (pricingInteraction.pointerId !== event.pointerId || pricingInteraction.dragged) return
  const deltaX = Math.abs(event.clientX - pricingInteraction.startX)
  const deltaY = Math.abs(event.clientY - pricingInteraction.startY)
  if (deltaX > 6 || deltaY > 6) {
    pricingInteraction.dragged = true
  }
}

const onPricingPointerEnd = (event: PointerEvent) => {
  if (pricingInteraction.pointerId !== event.pointerId) return
  pricingInteraction.pointerId = null
}

const handleModelClick = (model: ProviderModelPricingItem) => {
  if (!pricingAvailable.value) return
  openModelDetail(model)
}

const handlePricingAreaClick = (event: MouseEvent, model: ProviderModelPricingItem) => {
  const suppressOpen = pricingInteraction.model === model.model && pricingInteraction.dragged
  clearPricingInteraction()
  if (suppressOpen) return
  handleModelClick(model)
}

const parseOverrideMultiplier = (raw: string, fieldLabel: string) => {
  const trimmed = String(raw ?? '').trim()
  if (!trimmed) {
    return { isSet: false, value: 0 }
  }
  const value = Number(trimmed)
  if (!Number.isFinite(value) || value < 0) {
    showToast(t('components.main.modelList.toast.invalidMultiplier', { field: fieldLabel }), 'warning')
    return null
  }
  return { isSet: true, value }
}

const buildOverrideAutoHint = (multiplier?: number, source?: string) => {
  const formatted = formatMultiplier(multiplier)
  const sourceLabel = formatCacheMultiplierSource(source)
  if (formatted === '—') return t('components.main.modelList.followAutoEmpty')
  if (!sourceLabel) return t('components.main.modelList.followAuto', { value: formatted })
  return t('components.main.modelList.followAutoWithSource', { value: formatted, source: sourceLabel })
}

const resolveOverridePlaceholder = (model: ProviderModelPricingItem, kind: 'create' | 'read') => {
  const multiplier = kind === 'create' ? resolveCacheCreateMultiplier(model) : resolveCacheReadMultiplier(model)
  return formatEditableMultiplier(multiplier)
}

const resolveOverrideHint = (model: ProviderModelPricingItem, kind: 'create' | 'read') => {
  const source = kind === 'create' ? model.cacheCreateMultiplierSource : model.cacheReadMultiplierSource
  const multiplier = kind === 'create' ? resolveCacheCreateMultiplier(model) : resolveCacheReadMultiplier(model)
  if (source === 'manual') {
    return t('components.main.modelList.manualFieldHint', { value: formatMultiplier(multiplier) })
  }
  return buildOverrideAutoHint(multiplier, source)
}

const loadModels = async () => {
  if (!props.provider) return
  const requestSeq = loadRequestSeq.value + 1
  loadRequestSeq.value = requestSeq
  loading.value = true
  error.value = ''
  response.value = null
  closeDebugModal()
  closeImportModal(true)

  try {
    const data = await fetchProviderModelPricing(props.provider, props.platform, selectedSource.value)
    if (requestSeq !== loadRequestSeq.value) return
    response.value = data
    error.value = data.fetchError?.trim() || ''
  } catch (err) {
    if (requestSeq !== loadRequestSeq.value) return
    error.value = extractErrorMessage(err) || t('components.main.modelList.loadFailed')
  } finally {
    if (requestSeq !== loadRequestSeq.value) return
    loading.value = false
  }
}

const handleSourceChange = () => {
  selectedVendor.value = 'all'
  closeModelDetail()
  closeDebugModal()
  closeImportModal(true)
  clearPricingInteraction()
  void loadModels()
}

const saveOverride = async (model: ProviderModelPricingItem) => {
  if (!props.provider || isWorkingModel(model.model)) return
  const cacheCreateMultiplier = parseOverrideMultiplier(
    overrideForm.cacheCreateMultiplier,
    resolveCacheCreateMultiplierLabel(model, 'input'),
  )
  if (cacheCreateMultiplier === null) return

  const cacheReadMultiplier = parseOverrideMultiplier(
    overrideForm.cacheReadMultiplier,
    t('components.main.modelList.cacheReadMultiplier'),
  )
  if (cacheReadMultiplier === null) return

  const hadManualOverride = hasManualCacheOverride(model)
  if (!hadManualOverride && !cacheCreateMultiplier.isSet && !cacheReadMultiplier.isSet) {
    showToast(t('components.main.modelList.toast.overrideRequired'), 'warning')
    return
  }

  savingModel.value = model.model
  try {
    await upsertProviderModelPricingOverride(
      props.provider,
      model.model,
      cacheCreateMultiplier.value,
      cacheCreateMultiplier.isSet,
      cacheReadMultiplier.value,
      cacheReadMultiplier.isSet,
    )
    showToast(
      !cacheCreateMultiplier.isSet && !cacheReadMultiplier.isSet
        ? t('components.main.modelList.toast.resetSuccess')
        : t('components.main.modelList.toast.saveSuccess'),
    )
    closeModelDetail()
    await loadModels()
  } catch (err) {
    showToast(
      t('components.main.modelList.toast.saveFailed', { error: extractErrorMessage(err) }),
      'error',
    )
  } finally {
    savingModel.value = ''
  }
}

const resetOverride = async (model: ProviderModelPricingItem) => {
  if (!props.provider || isWorkingModel(model.model)) return
  resettingModel.value = model.model
  try {
    await deleteProviderModelPricingOverride(props.provider, model.model)
    showToast(t('components.main.modelList.toast.resetSuccess'))
    if (editingModel.value === model.model) {
      closeModelDetail()
    }
    await loadModels()
  } catch (err) {
    showToast(
      t('components.main.modelList.toast.resetFailed', { error: extractErrorMessage(err) }),
      'error',
    )
  } finally {
    resettingModel.value = ''
  }
}

const resetUIState = () => {
  searchTerm.value = ''
  selectedVendor.value = 'all'
  selectedSource.value = 'auto'
  loading.value = false
  error.value = ''
  response.value = null
  loadRequestSeq.value += 1
  closeDebugModal()
  closeImportModal(true)
  closeModelDetail()
  clearPricingInteraction()
  savingModel.value = ''
  resettingModel.value = ''
}

const handleClose = () => {
  emit('close')
}

watch(
  importJsonInput,
  (value, previousValue) => {
    if (value === previousValue || importingJson.value) return
    if (importError.value) {
      importError.value = ''
    }
    if (importDebugResponse.value) {
      importDebugResponse.value = null
    }
  },
)

watch(
  () => [props.open, props.provider?.id],
  ([isOpen]) => {
    if (!isOpen) {
      resetUIState()
      return
    }
    resetUIState()
    void loadModels()
  },
)
</script>

<style scoped>
@import '../common/provider-model-list-shared.css';

.provider-model-state.error {
  color: #ef4444;
}

.provider-model-toolbar-main {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.provider-model-source-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: min(100%, 420px);
}

.provider-model-source-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.provider-model-source-label {
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--mac-text-secondary);
  white-space: nowrap;
  flex: 0 0 auto;
}

.provider-model-source-select {
  flex: 1 1 260px;
  min-width: 0;
  width: auto;
}

.provider-model-source-hint {
  font-size: 0.76rem;
  line-height: 1.45;
  color: var(--mac-text-secondary);
}

.provider-model-source-current {
  font-size: 0.76rem;
  line-height: 1.45;
  color: var(--mac-text);
}

.provider-model-source-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.provider-model-source-actions {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex: 0 0 auto;
  flex-wrap: wrap;
}

.meta-pill-accent {
  border-color: rgba(16, 185, 129, 0.26);
  background: rgba(16, 185, 129, 0.12);
  color: #047857;
}

.provider-model-search {
  width: 100%;
  min-width: 0;
}

.provider-model-debug-button,
.provider-model-import-button,
.provider-model-inline-debug-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border-radius: 999px;
  border: 1px solid rgba(59, 130, 246, 0.22);
  background: rgba(59, 130, 246, 0.08);
  color: var(--mac-text);
  padding: 6px 12px;
  font-size: 0.78rem;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.provider-model-debug-button:hover,
.provider-model-import-button:hover,
.provider-model-inline-debug-btn:hover {
  background: rgba(59, 130, 246, 0.14);
  border-color: rgba(59, 130, 246, 0.3);
}

.provider-model-debug-button:disabled,
.provider-model-import-button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.provider-model-state-stack {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.provider-model-challenge-hint {
  margin: 0;
  padding: 10px 12px;
  max-width: min(780px, 100%);
  border-radius: 14px;
  border: 1px solid rgba(245, 158, 11, 0.26);
  background: rgba(245, 158, 11, 0.12);
  color: #b45309;
  font-size: 0.82rem;
  line-height: 1.55;
}

.provider-model-item.no-pricing {
  grid-template-columns: 1fr;
}

.provider-model-item {
  --pricing-scroll-fade: var(--mac-surface-strong);
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: start;
  gap: 16px;
}

.provider-model-item.clickable {
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.provider-model-item.clickable:focus-visible {
  outline: 2px solid rgba(59, 130, 246, 0.42);
  outline-offset: 2px;
  border-color: rgba(59, 130, 246, 0.35);
}

.provider-model-item.clickable:hover {
  --pricing-scroll-fade: var(--mac-surface-hover);
  background: var(--mac-surface-hover);
}

.provider-model-item .model-name {
  white-space: normal;
  overflow: visible;
  text-overflow: clip;
  word-break: break-word;
  overflow-wrap: anywhere;
  line-height: 1.4;
}

.tag-manual {
  color: #f59e0b;
  background: rgba(245, 158, 11, 0.12);
  border-color: rgba(245, 158, 11, 0.25);
}

.provider-model-item .pricing-inline-container {
  justify-self: end;
  width: fit-content;
  min-width: 0;
  max-width: 100%;
}

.provider-model-item .pricing-inline-container::after {
  display: none;
}

.provider-model-item .pricing-inline-container .model-pricing {
  width: fit-content;
  max-width: 100%;
  min-width: 0;
  justify-content: flex-end;
  overflow-x: auto;
  padding-right: 0;
  gap: 10px;
  cursor: pointer;
  overscroll-behavior-x: contain;
}

.provider-model-item .pricing-inline-container .price-block {
  min-width: 88px;
}

.provider-model-item .price-value.cache-create {
  color: #d97706;
}

.provider-model-item .price-value.cache-read {
  color: #0f766e;
}

.provider-model-item .price-note.is-estimated {
  color: #b45309;
}

.provider-model-item .price-note.is-manual {
  color: #0f766e;
}

.model-detail-modal {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.provider-import-modal {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.import-hint {
  margin: 0;
}

.provider-import-textarea {
  width: 100%;
  min-height: 320px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 0.8rem;
  line-height: 1.65;
}

.provider-import-error {
  margin: 0;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px solid rgba(239, 68, 68, 0.22);
  background: rgba(239, 68, 68, 0.08);
  color: #b91c1c;
  font-size: 0.82rem;
  line-height: 1.5;
}

.provider-debug-modal {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
}

.debug-hint {
  margin: 0;
}

.debug-summary-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.debug-summary-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px;
  border-radius: 14px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.08);
  min-width: 0;
}

.debug-summary-card--error {
  grid-column: 1 / -1;
  border-color: rgba(239, 68, 68, 0.22);
  background: rgba(239, 68, 68, 0.08);
}

.debug-summary-note {
  margin: 0;
  font-size: 0.74rem;
  line-height: 1.45;
  color: var(--mac-text-secondary);
}

.debug-attempts {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.debug-attempt-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.86), rgba(241, 245, 249, 0.72));
}

.debug-attempt-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.debug-attempt-title-wrap {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.debug-attempt-title {
  font-size: 0.92rem;
  font-weight: 700;
  color: var(--mac-text);
}

.debug-attempt-meta,
.debug-attempt-side {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.debug-attempt-side-item {
  font-size: 0.78rem;
  line-height: 1.4;
  color: var(--mac-text-secondary);
}

.debug-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(255, 255, 255, 0.72);
  font-size: 0.76rem;
  line-height: 1;
  color: var(--mac-text-secondary);
}

.debug-pill.is-success {
  color: #047857;
  border-color: rgba(16, 185, 129, 0.26);
  background: rgba(16, 185, 129, 0.12);
}

.debug-pill.is-warning {
  color: #b45309;
  border-color: rgba(245, 158, 11, 0.26);
  background: rgba(245, 158, 11, 0.12);
}

.debug-pill.is-error {
  color: #b91c1c;
  border-color: rgba(239, 68, 68, 0.26);
  background: rgba(239, 68, 68, 0.12);
}

.debug-attempt-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.debug-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
  padding: 14px;
  border-radius: 14px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(255, 255, 255, 0.72);
}

.debug-block-title {
  font-size: 0.82rem;
  font-weight: 700;
  color: var(--mac-text);
}

.debug-kv {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.debug-kv-label {
  font-size: 0.74rem;
  line-height: 1.35;
  color: var(--mac-text-secondary);
}

.debug-inline-code {
  display: block;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.06);
  color: var(--mac-text);
  font-size: 0.78rem;
  line-height: 1.55;
  word-break: break-word;
  overflow-wrap: anywhere;
}

.debug-response-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  font-size: 0.75rem;
  line-height: 1.45;
  color: var(--mac-text-secondary);
}

.debug-code-block {
  margin: 0;
  padding: 12px 14px;
  border-radius: 14px;
  background: #0f172a;
  color: #e2e8f0;
  font-size: 0.76rem;
  line-height: 1.6;
  font-family: ui-monospace, SFMono-Regular, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  white-space: pre-wrap;
  word-break: break-word;
  overflow: auto;
  max-height: 220px;
}

.debug-code-block--body {
  max-height: 320px;
}

.debug-attempt-error {
  margin: 0;
  font-size: 0.78rem;
  line-height: 1.5;
  color: #b91c1c;
}

.debug-actions {
  margin-top: 4px;
}

.detail-hint {
  margin: 0;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px;
  border-radius: 14px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.08);
  min-width: 0;
}

.detail-item--full {
  grid-column: 1 / -1;
}

.detail-label {
  font-size: 0.78rem;
  line-height: 1.35;
  color: var(--mac-text-secondary);
}

.detail-value {
  font-size: 1rem;
  font-weight: 700;
  line-height: 1.35;
  color: var(--mac-text);
}

.detail-value--wrap {
  white-space: normal;
  word-break: break-word;
  overflow-wrap: anywhere;
}

.detail-note {
  font-size: 0.74rem;
  line-height: 1.35;
  color: var(--mac-text-secondary);
}

.detail-divider {
  height: 1px;
  background: var(--mac-border);
}

.detail-section-title {
  font-size: 0.86rem;
  font-weight: 600;
  color: var(--mac-text);
}

.override-editor-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.override-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.override-field-hint {
  font-size: 0.74rem;
  line-height: 1.35;
  color: var(--mac-text-secondary);
}

.override-label {
  font-size: 12px;
  color: var(--mac-text-secondary);
}

.override-input {
  width: 100%;
  min-width: 0;
}

.override-actions {
  grid-column: 1 / -1;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

@media (max-width: 720px) {
  .provider-model-item {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .debug-summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .debug-attempt-grid {
    grid-template-columns: 1fr;
  }

  .detail-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .override-editor-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .provider-model-source-field {
    width: 100%;
  }

  .provider-model-source-row {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }

  .provider-model-source-select {
    width: 100%;
  }

  .provider-model-source-footer {
    align-items: stretch;
  }

  .provider-model-source-actions {
    width: 100%;
  }

  .provider-model-debug-button,
  .provider-model-import-button,
  .provider-model-inline-debug-btn {
    width: 100%;
  }

  .provider-model-item {
    grid-template-columns: 1fr;
  }

  .debug-summary-grid {
    grid-template-columns: 1fr;
  }

  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
