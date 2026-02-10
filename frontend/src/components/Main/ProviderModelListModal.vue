<template>
  <BaseModal
    :open="open"
    :title="modalTitle"
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
        <div
          v-for="model in filteredModels"
          :key="model.model"
          class="provider-model-item"
          :class="{ 'no-pricing': !pricingAvailable }"
        >
          <div class="model-main">
            <div class="model-name">{{ model.model }}</div>
            <div class="model-tags">
              <span
                class="tag"
                :class="billingTagClass(model.quotaType)"
              >
                {{ billingLabel(model.quotaType) }}
              </span>
              <span v-if="model.ownerBy" class="tag tag-neutral">
                {{ model.ownerBy }}
              </span>
            </div>
          </div>

          <div v-if="pricingAvailable" class="model-pricing">
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
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseModal from '../common/BaseModal.vue'
import BaseInput from '../common/BaseInput.vue'
import type { AutomationCard } from '../../data/cards'
import { extractErrorMessage } from '../../utils/error'
import {
  fetchProviderModelPricing,
  type ProviderModelPricingResponse,
  type ProviderModelPricingItem,
  type ProviderModelPerCallPrice,
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

  const allCount = models.value.length
  const labels = vendorLabelMap.value
  const tabs: ProviderVendorTab[] = [{ key: 'all', label: labels.all, count: allCount }]

  const entries = [...counts.entries()]
    .filter(([key]) => key !== 'Unknown')
    .sort((a, b) => b[1] - a[1])

  for (const [key, count] of entries) {
    if (count <= 0) continue
    tabs.push({ key, label: labels[key], count })
  }

  const unknownCount = counts.get('Unknown') || 0
  if (unknownCount > 0) {
    tabs.push({ key: 'Unknown', label: labels.Unknown, count: unknownCount })
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

const resetUIState = () => {
  searchTerm.value = ''
  selectedVendor.value = 'all'
  error.value = ''
  response.value = null
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
.provider-model-modal {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.provider-model-toolbar {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.provider-model-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.meta-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-radius: 999px;
  border: 1px solid var(--mac-border);
  background: var(--mac-surface-strong);
  font-size: 0.82rem;
  color: var(--mac-text-secondary);
}

.provider-model-vendors {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.vendor-pill {
  border: 1px solid transparent;
  background: rgba(148, 163, 184, 0.12);
  color: var(--mac-text-secondary);
  padding: 6px 12px;
  border-radius: 999px;
  cursor: pointer;
  font-size: 0.85rem;
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.vendor-pill:hover {
  background: rgba(148, 163, 184, 0.18);
}

.vendor-pill.active {
  border-color: rgba(59, 130, 246, 0.35);
  background: rgba(59, 130, 246, 0.14);
  color: var(--mac-text);
}

.provider-model-state {
  padding: 18px 12px;
  text-align: center;
  font-size: 0.92rem;
  color: var(--mac-text-secondary);
}

.provider-model-state.error {
  color: #ef4444;
}

.provider-model-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.provider-model-item {
  border: 1px solid var(--mac-border);
  background: var(--mac-surface-strong);
  border-radius: 16px;
  padding: 14px 14px;
  display: grid;
  grid-template-columns: 1.6fr 1fr;
  gap: 12px;
  align-items: center;
}

.provider-model-item.no-pricing {
  grid-template-columns: 1fr;
}

.pricing-hint {
  margin: 0;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px dashed rgba(148, 163, 184, 0.35);
  color: var(--mac-text-secondary);
  background: rgba(148, 163, 184, 0.08);
  font-size: 0.85rem;
}

.model-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.model-name {
  font-weight: 600;
  color: var(--mac-text);
  font-size: 0.95rem;
  word-break: break-all;
}

.model-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.tag {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 0.78rem;
  border: 1px solid transparent;
}

.tag-token {
  color: #2563eb;
  background: rgba(37, 99, 235, 0.12);
  border-color: rgba(37, 99, 235, 0.18);
}

.tag-call {
  color: #7c3aed;
  background: rgba(124, 58, 237, 0.12);
  border-color: rgba(124, 58, 237, 0.18);
}

.tag-neutral {
  color: var(--mac-text-secondary);
  background: rgba(148, 163, 184, 0.12);
  border-color: rgba(148, 163, 184, 0.18);
}

.model-pricing {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  justify-content: flex-end;
  align-items: center;
}

.price-block {
  display: flex;
  align-items: baseline;
  gap: 6px;
  min-width: 0;
}

.price-label {
  font-size: 0.82rem;
  color: var(--mac-text-secondary);
  white-space: nowrap;
}

.price-value {
  font-size: 0.86rem;
  color: var(--mac-text);
  white-space: nowrap;
}

.price-value.input {
  color: #2563eb;
}

.price-value.output {
  color: #16a34a;
}

@media (max-width: 640px) {
  .provider-model-item {
    grid-template-columns: 1fr;
  }

  .model-pricing {
    justify-content: flex-start;
  }
}
</style>
