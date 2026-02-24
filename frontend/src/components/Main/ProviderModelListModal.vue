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
@import '../common/provider-model-list-shared.css';

.provider-model-state.error {
  color: #ef4444;
}

.provider-model-item.no-pricing {
  grid-template-columns: 1fr;
}
</style>
