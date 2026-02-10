<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import InlineModal from '../common/InlineModal.vue'
import BaseInput from '../common/BaseInput.vue'
import ModelPricingEditorModal from './ModelPricingEditorModal.vue'
import { useI18n } from 'vue-i18n'
import { listModelPricing, type ModelPricingRow } from '../../services/modelPricing'
import { extractErrorMessage } from '../../utils/error'
import { showToast } from '../../utils/toast'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()

const loading = ref(false)
const error = ref('')

const rows = ref<ModelPricingRow[]>([])
const search = ref('')
const onlyOverrides = ref(false)
const selectedModel = ref<string>('')

type EditMode = 'edit' | 'new'

const editorOpen = ref(false)
const editorMode = ref<EditMode>('edit')
const editorRow = ref<ModelPricingRow | null>(null)

const perTokenToPer1M = (value: number) => (Number.isFinite(value) ? value * 1_000_000 : 0)

const formatUsdPer1M = (value: number) => {
  if (!Number.isFinite(value)) return '—'
  const per1m = perTokenToPer1M(value)
  if (per1m === 0) return '$0'
  if (per1m < 0) return '—'
  if (per1m < 0.01) return `$${per1m.toFixed(6)}`
  if (per1m < 1) return `$${per1m.toFixed(4)}`
  return `$${per1m.toFixed(2)}`
}

const overrideCount = computed(() => rows.value.filter((item) => item.is_override || item.is_custom).length)

const filteredRows = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  const base = onlyOverrides.value ? rows.value.filter((item) => item.is_override || item.is_custom) : rows.value
  if (!keyword) return base
  return base.filter((item) => item.model.toLowerCase().includes(keyword))
})

const loadRows = async () => {
  loading.value = true
  error.value = ''
  try {
    rows.value = await listModelPricing()
  } catch (err) {
    const message = t('components.general.modelPricing.toast.loadFailed', { error: extractErrorMessage(err) })
    error.value = message
    showToast(message, 'error')
  } finally {
    loading.value = false
  }
}

const openCreateModal = () => {
  editorMode.value = 'new'
  editorRow.value = null
  editorOpen.value = true
}

const openEditModal = (row: ModelPricingRow) => {
  selectedModel.value = row.model
  editorMode.value = 'edit'
  editorRow.value = row
  editorOpen.value = true
}

const resetUIState = () => {
  search.value = ''
  onlyOverrides.value = false
  selectedModel.value = ''
  error.value = ''
}

const closeModal = () => {
  editorOpen.value = false
  emit('close')
}

const onSaved = async (model: string) => {
  selectedModel.value = model
  editorOpen.value = false
  await loadRows()
}

const onRemoved = async (model: string) => {
  selectedModel.value = model
  editorOpen.value = false
  await loadRows()
}

watch(
  () => props.open,
  (open) => {
    if (!open) {
      editorOpen.value = false
      resetUIState()
      return
    }

    resetUIState()
    // 每次打开都刷新一次，避免后端价格表更新后前端显示滞后
    void loadRows()
  },
)
</script>

<template>
  <InlineModal
    :open="open"
    :title="$t('components.general.modelPricing.title')"
    @close="closeModal"
  >
    <div class="model-pricing-modal">
      <div class="model-pricing-toolbar">
        <div class="model-pricing-header">
          <div class="model-pricing-actions">
            <button type="button" class="action-btn" @click="openCreateModal">
              {{ $t('components.general.modelPricing.add') }}
            </button>
            <button type="button" class="action-btn" :disabled="loading" @click="loadRows">
              {{ loading ? $t('components.general.modelPricing.loading') : $t('components.general.modelPricing.refresh') }}
            </button>
          </div>
        </div>

        <div class="model-pricing-search">
          <BaseInput
            v-model="search"
            type="text"
            :placeholder="$t('components.general.modelPricing.searchPlaceholder')"
          />
        </div>

        <div class="model-pricing-filters">
          <button
            type="button"
            class="vendor-pill"
            :class="{ active: !onlyOverrides }"
            @click="onlyOverrides = false"
          >
            {{ $t('components.general.modelPricing.filterAll') }} ({{ rows.length }})
          </button>
          <button
            type="button"
            class="vendor-pill"
            :class="{ active: onlyOverrides }"
            @click="onlyOverrides = true"
          >
            {{ $t('components.general.modelPricing.onlyOverrides') }} ({{ overrideCount }})
          </button>
        </div>
      </div>

      <div v-if="loading && rows.length === 0" class="model-pricing-state">
        {{ $t('components.general.modelPricing.loading') }}
      </div>
      <div v-else-if="error" class="model-pricing-state error">
        {{ error }}
      </div>
      <div v-else-if="filteredRows.length === 0" class="model-pricing-state">
        {{ $t('components.general.modelPricing.empty') }}
      </div>
      <div v-else class="model-pricing-list">
        <p class="pricing-hint">
          {{ $t('components.general.modelPricing.unitHint') }}
        </p>

        <div
          v-for="item in filteredRows"
          :key="item.model"
          class="model-pricing-item"
          :class="{ selected: item.model === selectedModel }"
          @click="openEditModal(item)"
        >
          <div class="model-main">
            <div class="model-name">{{ item.model }}</div>
            <div class="model-tags">
              <span v-if="item.is_custom" class="tag tag-custom">
                {{ $t('components.general.modelPricing.badge.custom') }}
              </span>
              <span v-else-if="item.is_override" class="tag tag-override">
                {{ $t('components.general.modelPricing.badge.override') }}
              </span>
            </div>
          </div>

          <div class="model-pricing">
            <div class="price-block">
              <span class="price-label">{{ $t('components.general.modelPricing.columns.input') }}</span>
              <span class="price-value input">{{ formatUsdPer1M(item.input_cost_per_token) }}/M</span>
            </div>
            <div class="price-block">
              <span class="price-label">{{ $t('components.general.modelPricing.columns.output') }}</span>
              <span class="price-value output">{{ formatUsdPer1M(item.output_cost_per_token) }}/M</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <ModelPricingEditorModal
      :open="editorOpen"
      :mode="editorMode"
      :row="editorRow"
      @close="editorOpen = false"
      @saved="onSaved"
      @removed="onRemoved"
    />
  </InlineModal>
</template>

<style scoped>
.model-pricing-modal {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.model-pricing-toolbar {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.model-pricing-header {
  display: flex;
  justify-content: flex-end;
  align-items: center;
}

.model-pricing-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.model-pricing-filters {
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

.model-pricing-state {
  padding: 18px 12px;
  text-align: center;
  font-size: 0.92rem;
  color: var(--mac-text-secondary);
}

.model-pricing-state.error {
  color: #ef4444;
}

.model-pricing-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.pricing-hint {
  margin: 0;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px dashed rgba(148, 163, 184, 0.35);
  color: var(--mac-text-secondary);
  background: rgba(148, 163, 184, 0.08);
  font-size: 0.85rem;
  line-height: 1.5;
}

.model-pricing-item {
  border: 1px solid var(--mac-border);
  background: var(--mac-surface-strong);
  border-radius: 16px;
  padding: 14px 14px;
  display: grid;
  grid-template-columns: 1.6fr 1fr;
  gap: 12px;
  align-items: center;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.model-pricing-item:hover {
  background: var(--mac-surface-hover);
}

.model-pricing-item.selected {
  border-color: rgba(59, 130, 246, 0.35);
  background: rgba(59, 130, 246, 0.12);
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
  white-space: nowrap;
}

.tag-custom {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
  border-color: rgba(16, 185, 129, 0.3);
}

.tag-override {
  background: rgba(245, 158, 11, 0.12);
  color: #f59e0b;
  border-color: rgba(245, 158, 11, 0.25);
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
  .model-pricing-item {
    grid-template-columns: 1fr;
  }

  .model-pricing {
    justify-content: flex-start;
  }
}
</style>
