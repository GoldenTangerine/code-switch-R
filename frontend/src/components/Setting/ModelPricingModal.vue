<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import InlineModal from '../common/InlineModal.vue'
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

const rows = ref<ModelPricingRow[]>([])
const search = ref('')
const onlyOverrides = ref(false)
const selectedModel = ref<string>('')

type EditMode = 'edit' | 'new'

const editorOpen = ref(false)
const editorMode = ref<EditMode>('edit')
const editorRow = ref<ModelPricingRow | null>(null)

const perTokenToPer1M = (value: number) => (Number.isFinite(value) ? value * 1_000_000 : 0)

const formatPer1M = (value: number) => {
  if (!Number.isFinite(value)) return '—'
  const per1m = perTokenToPer1M(value)
  if (per1m === 0) return '0'
  const digits = per1m >= 10 ? 2 : per1m >= 1 ? 3 : per1m >= 0.1 ? 4 : 6
  return per1m.toFixed(digits).replace(/\\.?0+$/, '')
}

const filteredRows = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  const base = onlyOverrides.value ? rows.value.filter((item) => item.is_override || item.is_custom) : rows.value
  if (!keyword) return base
  return base.filter((item) => item.model.toLowerCase().includes(keyword))
})

const loadRows = async () => {
  loading.value = true
  try {
    rows.value = await listModelPricing()
  } catch (error) {
    showToast(
      t('components.general.modelPricing.toast.loadFailed', { error: extractErrorMessage(error) }),
      'error',
    )
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
      return
    }

    // 每次打开都刷新一次，避免后端价格表更新后前端显示滞后
    void loadRows()
  },
)
</script>

<template>
  <InlineModal
    :open="open"
    :title="$t('components.general.modelPricing.title')"
    panel-width="min(980px, 95vw)"
    @close="closeModal"
  >
    <div class="model-pricing-modal">
      <div class="model-pricing-toolbar">
        <input
          v-model="search"
          type="text"
          class="mac-input model-pricing-search"
          :placeholder="$t('components.general.modelPricing.searchPlaceholder')"
        />
        <label class="model-pricing-toggle">
          <input v-model="onlyOverrides" type="checkbox" />
          <span>{{ $t('components.general.modelPricing.onlyOverrides') }}</span>
        </label>
        <button type="button" class="action-btn" @click="openCreateModal">
          {{ $t('components.general.modelPricing.add') }}
        </button>
        <button type="button" class="action-btn" :disabled="loading" @click="loadRows">
          {{ loading ? $t('components.general.modelPricing.loading') : $t('components.general.modelPricing.refresh') }}
        </button>
      </div>

      <div class="model-pricing-list">
        <div v-if="loading && rows.length === 0" class="info-text">
          {{ $t('components.general.modelPricing.loading') }}
        </div>
        <div v-else-if="filteredRows.length === 0" class="info-text">
          —
        </div>
        <table v-else class="model-pricing-table">
          <thead>
            <tr>
              <th class="col-model">{{ $t('components.general.modelPricing.columns.model') }}</th>
              <th class="col-num">{{ $t('components.general.modelPricing.columns.input') }}</th>
              <th class="col-num">{{ $t('components.general.modelPricing.columns.output') }}</th>
              <th class="col-meta">{{ $t('components.general.modelPricing.columns.flags') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="item in filteredRows"
              :key="item.model"
              class="model-pricing-row"
              :class="{ selected: item.model === selectedModel }"
              @click="openEditModal(item)"
            >
              <td class="cell-model">
                <span class="model-name">{{ item.model }}</span>
              </td>
              <td class="cell-num">{{ formatPer1M(item.input_cost_per_token) }}</td>
              <td class="cell-num">{{ formatPer1M(item.output_cost_per_token) }}</td>
              <td class="cell-meta">
                <span v-if="item.is_custom" class="pricing-badge badge-custom">
                  {{ $t('components.general.modelPricing.badge.custom') }}
                </span>
                <span v-else-if="item.is_override" class="pricing-badge badge-override">
                  {{ $t('components.general.modelPricing.badge.override') }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
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
  gap: 14px;
  min-height: 420px;
}

.model-pricing-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.model-pricing-search {
  flex: 1 1 240px;
  min-width: 220px;
  font-family: monospace;
}

.model-pricing-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--mac-text-secondary);
  user-select: none;
}

.model-pricing-toggle input {
  width: 14px;
  height: 14px;
}

.model-pricing-list {
  border: 1px solid var(--mac-border);
  border-radius: 10px;
  overflow: hidden;
  background: var(--mac-surface);
  min-height: 320px;
}

.model-pricing-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.model-pricing-table thead {
  background: var(--mac-surface-strong);
}

.model-pricing-table th,
.model-pricing-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--mac-border);
  text-align: left;
  vertical-align: middle;
}

.col-model {
  width: 55%;
}

.col-num {
  width: 15%;
  text-align: right;
}

.col-meta {
  width: 15%;
}

.cell-num {
  text-align: right;
  font-family: monospace;
  white-space: nowrap;
}

.model-pricing-row {
  cursor: pointer;
}

.model-pricing-row:hover {
  background: var(--mac-surface-hover);
}

.model-pricing-row.selected {
  background: rgba(14, 165, 233, 0.12);
}

.model-name {
  font-family: monospace;
  font-size: 12px;
  overflow-wrap: anywhere;
}

.pricing-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 11px;
  line-height: 1.2;
  border: 1px solid transparent;
  white-space: nowrap;
}

.badge-custom {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
  border-color: rgba(16, 185, 129, 0.3);
}

.badge-override {
  background: rgba(245, 158, 11, 0.12);
  color: #f59e0b;
  border-color: rgba(245, 158, 11, 0.25);
}
</style>

