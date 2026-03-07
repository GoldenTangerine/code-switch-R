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

            <div class="detail-item">
              <span class="detail-label">{{ t('components.main.modelList.detailCacheCreate') }}</span>
              <span class="detail-value cache-create">
                {{ formatUSD(resolveCachePrice(editingTargetModel.inputUsdPerM, resolveCacheCreateMultiplier(editingTargetModel))) }}/M
              </span>
              <span v-if="resolveCacheCreateHint(editingTargetModel)" class="detail-note" :class="cacheHintClass(editingTargetModel.cacheCreateMultiplierSource)">
                {{ resolveCacheCreateHint(editingTargetModel) }}
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
              <span class="detail-label">{{ t('components.main.modelList.detailCacheCreateMultiplier') }}</span>
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
  </InlineModal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
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

const detailModalTitle = computed(() => {
  const baseTitle = t('components.main.modelList.detailTitle')
  if (!editingTargetModel.value?.model) return baseTitle
  return `${baseTitle} · ${editingTargetModel.value.model}`
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
  error.value = ''
  response.value = null
  closeModelDetail()
  clearPricingInteraction()
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
  min-width: 84px;
}

.provider-model-item .pricing-inline-container .price-block.ratio {
  min-width: 64px;
}

.model-detail-modal {
  display: flex;
  flex-direction: column;
  gap: 16px;
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

  .detail-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .override-editor-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .provider-model-item {
    grid-template-columns: 1fr;
  }

  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
