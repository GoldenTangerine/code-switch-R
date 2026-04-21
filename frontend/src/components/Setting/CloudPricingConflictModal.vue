<template>
  <InlineModal
    :open="open"
    :body-scrollable="false"
    :title="t('components.general.modelPricing.conflict.title')"
    :panel-width="'min(1320px, 98vw)'"
    @close="emit('close')"
  >
    <div class="provider-model-modal">
      <div class="preview-scroll">
        <div class="provider-model-toolbar">
          <div class="provider-model-meta">
            <span class="meta-pill">
              {{ t('components.general.modelPricing.conflict.fetchedAt') }}：{{ formattedFetchedAt }}
            </span>
            <span class="meta-pill">
              {{ t('components.general.modelPricing.conflict.total') }}：{{ rows.length }}
            </span>
            <span class="meta-pill">
              {{ t('components.general.modelPricing.conflict.selected') }}：{{ selectedModels.length }}
            </span>
          </div>

          <div class="provider-model-search">
            <BaseInput
              v-model="searchTerm"
              type="text"
              :placeholder="t('components.general.modelPricing.conflict.searchPlaceholder')"
            />
          </div>

          <div class="provider-model-vendors">
            <button
              type="button"
              class="vendor-pill"
              :class="{ active: filteredRows.length > 0 && allFilteredSelected }"
              @click="toggleSelectAllFiltered"
            >
              {{ t('components.general.modelPricing.conflict.selectAll') }} ({{ filteredRows.length }})
            </button>
            <button
              type="button"
              class="vendor-pill"
              :disabled="selectedModels.length === 0"
              @click="clearSelection"
            >
              {{ t('components.general.modelPricing.conflict.clear') }}
            </button>
          </div>
        </div>

        <p class="pricing-hint">
          {{ t('components.general.modelPricing.conflict.keepLocalHint') }}
        </p>

        <div v-if="filteredRows.length === 0" class="provider-model-state">
          {{ t('components.general.modelPricing.conflict.empty') }}
        </div>
        <div v-else class="provider-model-list">
          <div
            v-for="row in filteredRows"
            :key="row.model"
            class="provider-model-item provider-model-item--conflict"
          >
            <div class="model-main">
              <label class="model-checkbox-line">
                <input
                  type="checkbox"
                  class="model-checkbox"
                  :checked="selectedModelSet.has(row.model)"
                  @change="toggleModel(row.model)"
                >
                <span class="model-name" :title="row.model">{{ row.model }}</span>
              </label>
              <div class="model-tags">
                <span v-if="row.display_name" class="tag tag-neutral tag-wrap">
                  {{ row.display_name }}
                </span>
                <span v-if="row.litellm_provider" class="tag tag-neutral">
                  {{ row.litellm_provider }}
                </span>
                <span v-if="row.mode" class="tag tag-neutral">
                  {{ row.mode }}
                </span>
              </div>
            </div>

            <div class="conflict-pricing-grid">
              <div class="conflict-pricing-card">
                <div class="conflict-pricing-title">
                  {{ t('components.general.modelPricing.conflict.current') }}
                </div>
                <div class="model-pricing">
                  <div
                    v-for="block in buildPricingBlocks(row.current)"
                    :key="`${row.model}-current-${block.key}`"
                    class="price-block"
                  >
                    <span class="price-label">{{ block.label }}</span>
                    <span class="price-value" :class="block.className">{{ block.value }}</span>
                  </div>
                </div>
              </div>

              <div class="conflict-pricing-card conflict-pricing-card--incoming">
                <div class="conflict-pricing-title">
                  {{ t('components.general.modelPricing.conflict.incoming') }}
                </div>
                <div class="model-pricing">
                  <div
                    v-for="block in buildPricingBlocks(row.incoming)"
                    :key="`${row.model}-incoming-${block.key}`"
                    class="price-block"
                  >
                    <span class="price-label">{{ block.label }}</span>
                    <span class="price-value" :class="block.className">{{ block.value }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="preview-actions">
        <button
          type="button"
          class="action-btn secondary"
          :disabled="syncing"
          @click="emit('close')"
        >
          {{ t('components.general.modelPricing.conflict.close') }}
        </button>
        <button
          type="button"
          class="action-btn sync-btn"
          :disabled="syncing"
          @click="emit('confirm-sync', [...selectedModels])"
        >
          {{ syncing ? t('components.general.modelPricing.syncing') : t('components.general.modelPricing.conflict.apply') }}
        </button>
      </div>
    </div>
  </InlineModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import InlineModal from '../common/InlineModal.vue'
import BaseInput from '../common/BaseInput.vue'
import type {
  CloudPriceTableConflictPricing,
  CloudPriceTableSyncConflictRow,
} from '../../services/modelPricing'

interface DisplayPriceBlock {
  key: string
  label: string
  value: string
  className: string
}

const props = defineProps<{
  open: boolean
  rows: CloudPriceTableSyncConflictRow[]
  fetchedAt: string
  syncing: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'confirm-sync', models: string[]): void
}>()

const { t } = useI18n()

const searchTerm = ref('')
const selectedModels = ref<string[]>([])

const selectedModelSet = computed(() => new Set(selectedModels.value))

const filteredRows = computed(() => {
  const term = searchTerm.value.trim().toLowerCase()
  if (!term) return props.rows
  return props.rows.filter((row) => {
    const target = [row.model, row.display_name, row.litellm_provider, row.mode]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
    return target.includes(term)
  })
})

const allFilteredSelected = computed(() => (
  filteredRows.value.length > 0 && filteredRows.value.every((row) => selectedModelSet.value.has(row.model))
))

const formattedFetchedAt = computed(() => {
  const raw = props.fetchedAt.trim()
  if (!raw) return '-'
  const parsed = new Date(raw)
  if (Number.isNaN(parsed.getTime())) return raw
  return parsed.toLocaleString()
})

function formatUsdPer1M(perToken: number): string {
  if (!Number.isFinite(perToken)) return '—'
  const per1M = perToken * 1_000_000
  if (per1M < 0) return '—'
  if (per1M === 0) return '$0'
  if (per1M < 0.01) return `$${per1M.toFixed(6)}`
  if (per1M < 1) return `$${per1M.toFixed(4)}`
  return `$${per1M.toFixed(2)}`
}

function formatMultiplier(value: number): string {
  if (!Number.isFinite(value) || value < 0) return '—'
  if (Math.abs(value - Math.round(value)) < 1e-9) return `${Math.round(value)}x`
  if (value < 0.01) return `${value.toFixed(4)}x`
  return `${value.toFixed(2)}x`
}

function buildPricingBlocks(pricing: CloudPriceTableConflictPricing): DisplayPriceBlock[] {
  return [
    {
      key: 'input',
      label: t('components.general.modelPricing.columns.input'),
      value: `${formatUsdPer1M(pricing.input_cost_per_token)}/M`,
      className: 'input',
    },
    {
      key: 'output',
      label: t('components.general.modelPricing.columns.output'),
      value: `${formatUsdPer1M(pricing.output_cost_per_token)}/M`,
      className: 'output',
    },
    {
      key: 'reasoning',
      label: t('components.general.modelPricing.fields.reasoning'),
      value: `${formatUsdPer1M(pricing.output_cost_per_reasoning_token)}/M`,
      className: 'output',
    },
    {
      key: 'cache-create',
      label: t('components.general.modelPricing.columns.cacheCreate'),
      value: `${formatUsdPer1M(pricing.cache_creation_input_token_cost)}/M`,
      className: 'cache-create',
    },
    {
      key: 'cache-1h',
      label: t('components.general.modelPricing.columns.cacheCreate1h'),
      value: `${formatUsdPer1M(pricing.ephemeral_1h_cost_per_token)}/M`,
      className: 'cache-create',
    },
    {
      key: 'cache-read',
      label: t('components.general.modelPricing.columns.cacheRead'),
      value: `${formatUsdPer1M(pricing.cache_read_input_token_cost)}/M`,
      className: 'cache-read',
    },
    {
      key: 'group',
      label: t('components.general.modelPricing.columns.groupMultiplier'),
      value: formatMultiplier(pricing.group_multiplier),
      className: '',
    },
  ]
}

function toggleModel(model: string): void {
  if (selectedModelSet.value.has(model)) {
    selectedModels.value = selectedModels.value.filter((item) => item !== model)
    return
  }
  selectedModels.value = [...selectedModels.value, model]
}

function toggleSelectAllFiltered(): void {
  if (allFilteredSelected.value) {
    const filteredModelSet = new Set(filteredRows.value.map((row) => row.model))
    selectedModels.value = selectedModels.value.filter((model) => !filteredModelSet.has(model))
    return
  }

  const next = new Set(selectedModels.value)
  filteredRows.value.forEach((row) => next.add(row.model))
  selectedModels.value = [...next]
}

function clearSelection(): void {
  selectedModels.value = []
}

watch(
  () => props.open,
  (open) => {
    if (!open) return
    searchTerm.value = ''
    selectedModels.value = []
  },
)
</script>

<style scoped>
@import '../common/provider-model-list-shared.css';

.provider-model-modal {
  height: 100%;
  min-height: 0;
  gap: 12px;
}

.preview-scroll {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  padding-right: 2px;
}

.provider-model-item--conflict {
  grid-template-columns: minmax(280px, 1fr) minmax(0, 1.8fr);
  align-items: start;
  gap: 16px;
}

.model-checkbox-line {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  cursor: pointer;
}

.model-checkbox {
  margin-top: 2px;
}

.model-name {
  white-space: normal;
  overflow: visible;
  text-overflow: clip;
  word-break: break-word;
  overflow-wrap: anywhere;
  line-height: 1.4;
}

.tag-wrap {
  white-space: normal;
  line-height: 1.4;
}

.conflict-pricing-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  min-width: 0;
}

.conflict-pricing-card {
  min-width: 0;
  padding: 12px;
  border: 1px solid var(--mac-border);
  border-radius: 12px;
  background: rgba(148, 163, 184, 0.08);
}

.conflict-pricing-card--incoming {
  border-color: rgba(59, 130, 246, 0.25);
  background: rgba(59, 130, 246, 0.08);
}

.conflict-pricing-title {
  margin-bottom: 10px;
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--mac-text-secondary);
}

.price-value.input {
  color: #2563eb;
}

.price-value.output {
  color: #16a34a;
}

.price-value.cache-create {
  color: #d97706;
}

.price-value.cache-read {
  color: #0f766e;
}

.preview-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
  margin-top: auto;
  padding-top: 12px;
  border-top: 1px solid var(--mac-border);
  background: var(--mac-surface);
}

.action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: max-content;
  max-width: 100%;
  min-width: 88px;
  border: 1px solid rgba(59, 130, 246, 0.35);
  background: rgba(59, 130, 246, 0.12);
  color: var(--mac-text);
  border-radius: 10px;
  padding: 8px 14px;
  min-height: 34px;
  font-size: 0.85rem;
  line-height: 1.2;
  white-space: nowrap;
  flex-shrink: 0;
  box-sizing: border-box;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.sync-btn {
  min-width: 248px;
}

.action-btn:hover:not(:disabled) {
  background: rgba(59, 130, 246, 0.18);
  border-color: rgba(59, 130, 246, 0.45);
}

.action-btn.secondary {
  border-color: var(--mac-border);
  background: var(--mac-surface-strong);
}

.action-btn.secondary:hover:not(:disabled) {
  background: var(--mac-surface-hover);
}

.action-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

@media (max-width: 920px) {
  .provider-model-item--conflict {
    grid-template-columns: 1fr;
  }

  .conflict-pricing-grid {
    grid-template-columns: 1fr;
  }
}
</style>
