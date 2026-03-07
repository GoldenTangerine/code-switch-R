<template>
  <BaseModal
    :open="open"
    :title="modalTitle"
    :panel-width="'min(1240px, 98vw)'"
    @close="handleClose"
  >
    <div class="provider-model-modal">
      <div class="provider-model-toolbar">
        <div class="provider-model-meta">
          <span v-if="siteType" class="meta-pill">
            {{ t('components.main.modelList.siteType') }}：{{ siteType }}
          </span>
          <span v-if="pricingSource" class="meta-pill">
            {{ t('components.main.modelList.source') }}：{{ pricingSource }}
          </span>
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
      <div v-else-if="error" class="provider-model-state error">
        {{ error }}
      </div>
      <div v-else-if="filteredModels.length === 0" class="provider-model-state">
        {{ t('components.main.modelList.empty') }}
      </div>
      <div v-else class="provider-model-list">
        <p v-if="!pricingAvailable" class="pricing-hint">
          {{ t('components.main.modelList.pricingUnavailable') }}
        </p>
        <p v-else class="pricing-scroll-hint">
          {{ t('components.main.modelList.scrollHint') }}
        </p>
        <div
          v-for="model in filteredModels"
          :key="model.model"
          class="provider-model-item"
          :class="{ 'no-pricing': !pricingAvailable }"
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
            <div v-if="pricingAvailable && model.quotaType === 0" class="model-actions">
              <button
                type="button"
                class="action-btn"
                :disabled="isWorkingModel(model.model)"
                @click="openOverrideEditor(model)"
              >
                {{ t('components.main.modelList.adjustCache') }}
              </button>
            </div>
          </div>

          <div v-if="pricingAvailable" class="pricing-inline-container">
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
                <div class="price-block">
                  <span class="price-label">{{ t('components.main.modelList.cacheCreate') }}</span>
                  <span class="price-value cache-create">
                    {{ formatUSD(resolveCachePrice(model.inputUsdPerM, resolveCacheCreateMultiplier(model))) }}/M
                  </span>
                  <span v-if="resolveCacheCreateHint(model)" class="price-note" :class="cacheHintClass(model.cacheCreateMultiplierSource)">
                    {{ resolveCacheCreateHint(model) }}
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
                <div v-if="model.modelRatio > 0" class="price-block ratio">
                  <span class="price-label">{{ t('components.main.modelList.ratio') }}</span>
                  <span class="price-value">
                    {{ formatRatio(model.modelRatio) }}
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
  </BaseModal>

  <InlineModal
    :open="Boolean(editingTargetModel)"
    :title="overrideModalTitle"
    :panel-width="'min(560px, 92vw)'"
    :body-scrollable="false"
    @close="cancelOverrideEditor"
  >
    <div v-if="editingTargetModel" class="override-editor-modal">
      <p class="override-editor-model" :title="editingTargetModel.model">
        {{ editingTargetModel.model }}
      </p>
      <p class="pricing-hint override-editor-hint">
        {{ t('components.main.modelList.overrideModalHint') }}
      </p>

      <div class="override-editor-grid">
        <div class="override-field">
          <label class="override-label">{{ t('components.main.modelList.cacheCreateMultiplier') }}</label>
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

      <div class="override-actions">
        <button type="button" class="action-btn" :disabled="isWorkingModel(editingTargetModel.model)" @click="cancelOverrideEditor">
          {{ t('common.cancel') }}
        </button>
        <button
          v-if="hasManualCacheOverride(editingTargetModel)"
          type="button"
          class="action-btn"
          :disabled="isWorkingModel(editingTargetModel.model)"
          @click="resetOverride(editingTargetModel)"
        >
          {{ resettingModel === editingTargetModel.model ? t('components.general.modelPricing.removing') : t('components.main.modelList.resetCache') }}
        </button>
        <button
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
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseModal from '../common/BaseModal.vue'
import BaseInput from '../common/BaseInput.vue'
import InlineModal from '../common/InlineModal.vue'
import type { AutomationCard } from '../../data/cards'
import { extractErrorMessage } from '../../utils/error'
import { showToast } from '../../utils/toast'
import {
  deleteProviderModelPricingOverride,
  fetchProviderModelPricing,
  type ProviderModelPerCallPrice,
  type ProviderModelPricingItem,
  type ProviderModelPricingResponse,
  upsertProviderModelPricingOverride,
} from '../../services/providerModelPricing'

type ProviderVendorKey = 'all' | 'OpenAI' | 'Claude' | 'Gemini' | 'Moonshot' | 'Grok' | 'DeepSeek' | 'Qwen' | 'Mistral' | 'Unknown'

type ProviderVendorTab = {
  key: ProviderVendorKey
  label: string
  count: number
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

const searchTerm = ref('')
const selectedVendor = ref<ProviderVendorKey>('all')
const editingModel = ref('')
const savingModel = ref('')
const resettingModel = ref('')

const overrideForm = reactive({
  cacheCreateMultiplier: '',
  cacheReadMultiplier: '',
})

const modalTitle = computed(() => {
  if (!props.provider) return t('components.main.modelList.modalTitleFallback')
  return t('components.main.modelList.modalTitle', { name: props.provider.name })
})

const siteType = computed(() => response.value?.siteType ?? '')
const pricingSource = computed(() => response.value?.pricingSource ?? '')
const models = computed(() => response.value?.models ?? [])
const pricingAvailable = computed(() => response.value?.pricingSource !== 'v1/models')

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

const overrideModalTitle = computed(() => {
  if (!editingTargetModel.value?.model) return t('components.main.modelList.adjustCache')
  return `${t('components.main.modelList.adjustCache')} · ${editingTargetModel.value.model}`
})

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

const openOverrideEditor = (model: ProviderModelPricingItem) => {
  editingModel.value = model.model
  overrideForm.cacheCreateMultiplier =
    model.cacheCreateMultiplierSource === 'manual' ? formatEditableMultiplier(resolveCacheCreateMultiplier(model)) : ''
  overrideForm.cacheReadMultiplier =
    model.cacheReadMultiplierSource === 'manual' ? formatEditableMultiplier(resolveCacheReadMultiplier(model)) : ''
}

const cancelOverrideEditor = () => {
  editingModel.value = ''
  clearOverrideForm()
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
  loading.value = true
  error.value = ''
  response.value = null

  try {
    response.value = await fetchProviderModelPricing(props.provider, props.platform)
  } catch (err) {
    error.value = extractErrorMessage(err) || t('components.main.modelList.loadFailed')
  } finally {
    loading.value = false
  }
}

const saveOverride = async (model: ProviderModelPricingItem) => {
  if (!props.provider || isWorkingModel(model.model)) return
  const cacheCreateMultiplier = parseOverrideMultiplier(
    overrideForm.cacheCreateMultiplier,
    t('components.main.modelList.cacheCreateMultiplier'),
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
    cancelOverrideEditor()
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
      cancelOverrideEditor()
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
  error.value = ''
  response.value = null
  cancelOverrideEditor()
  savingModel.value = ''
  resettingModel.value = ''
}

const handleClose = () => {
  emit('close')
}

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

.provider-model-item.no-pricing {
  grid-template-columns: 1fr;
}

.provider-model-item {
  --pricing-scroll-fade: var(--mac-surface-strong);
  grid-template-columns: minmax(320px, 1.35fr) minmax(0, 1.65fr);
  align-items: start;
  gap: 16px;
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

.model-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.provider-model-item .price-value.cache-create {
  color: #d97706;
}

.provider-model-item .price-value.cache-read {
  color: #0f766e;
}

.provider-model-item .price-note {
  color: var(--mac-text-secondary);
}

.provider-model-item .price-note.is-estimated {
  color: #b45309;
}

.provider-model-item .price-note.is-manual {
  color: #0f766e;
}

.override-editor-modal {
  display: grid;
  gap: 14px;
}

.override-editor-model {
  margin: 0;
  font-size: 0.96rem;
  font-weight: 600;
  color: var(--mac-text);
  white-space: normal;
  word-break: break-word;
  overflow-wrap: anywhere;
  line-height: 1.45;
}

.override-editor-hint {
  margin: 0;
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
    grid-template-columns: minmax(240px, 1fr) minmax(0, 1.4fr);
  }

  .override-editor-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .provider-model-item {
    grid-template-columns: 1fr;
  }
}
</style>
