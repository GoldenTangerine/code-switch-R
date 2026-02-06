<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import InlineModal from '../common/InlineModal.vue'
import { useI18n } from 'vue-i18n'
import { deleteModelPricing, listModelPricing, upsertModelPricing, type ModelPricingRow } from '../../services/modelPricing'
import { extractErrorMessage } from '../../utils/error'
import { showToast } from '../../utils/toast'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()

type EditMode = 'edit' | 'new'

const loading = ref(false)
const saving = ref(false)
const removing = ref(false)

const rows = ref<ModelPricingRow[]>([])
const search = ref('')
const onlyOverrides = ref(false)

const mode = ref<EditMode>('edit')
const selectedModel = ref<string>('')

const form = reactive({
  model: '',
  inputUsdPer1M: '',
  outputUsdPer1M: '',
  reasoningUsdPer1M: '',
  cacheCreateUsdPer1M: '',
  cacheReadUsdPer1M: '',
  ephemeral1hUsdPer1M: '',
})

const perTokenToPer1M = (value: number) => (Number.isFinite(value) ? value * 1_000_000 : 0)
const per1MToPerToken = (value: number) => (Number.isFinite(value) ? value / 1_000_000 : 0)

const formatPer1M = (value: number) => {
  if (!Number.isFinite(value)) return '—'
  const per1m = perTokenToPer1M(value)
  if (per1m === 0) return '0'
  const digits = per1m >= 10 ? 2 : per1m >= 1 ? 3 : per1m >= 0.1 ? 4 : 6
  return per1m.toFixed(digits).replace(/\.?0+$/, '')
}

const selectedRow = computed(() => rows.value.find((item) => item.model === selectedModel.value) ?? null)

const filteredRows = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  const base = onlyOverrides.value ? rows.value.filter((item) => item.is_override || item.is_custom) : rows.value
  if (!keyword) return base
  return base.filter((item) => item.model.toLowerCase().includes(keyword))
})

const resetForm = () => {
  selectedModel.value = ''
  mode.value = 'new'
  form.model = ''
  form.inputUsdPer1M = ''
  form.outputUsdPer1M = ''
  form.reasoningUsdPer1M = ''
  form.cacheCreateUsdPer1M = ''
  form.cacheReadUsdPer1M = ''
  form.ephemeral1hUsdPer1M = ''
}

const fillFormFromRow = (row: ModelPricingRow) => {
  mode.value = 'edit'
  selectedModel.value = row.model
  form.model = row.model
  form.inputUsdPer1M = formatPer1M(row.input_cost_per_token)
  form.outputUsdPer1M = formatPer1M(row.output_cost_per_token)
  form.reasoningUsdPer1M = formatPer1M(row.output_cost_per_reasoning_token)
  form.cacheCreateUsdPer1M = formatPer1M(row.cache_creation_input_token_cost)
  form.cacheReadUsdPer1M = formatPer1M(row.cache_read_input_token_cost)
  form.ephemeral1hUsdPer1M = formatPer1M(row.ephemeral_1h_cost_per_token)
}

const selectRow = (row: ModelPricingRow) => {
  fillFormFromRow(row)
}

const loadRows = async () => {
  loading.value = true
  try {
    rows.value = await listModelPricing()
    if (mode.value === 'edit' && selectedModel.value) {
      const found = rows.value.find((item) => item.model === selectedModel.value)
      if (found) fillFormFromRow(found)
    }
  } catch (error) {
    showToast(
      t('components.general.modelPricing.toast.loadFailed', { error: extractErrorMessage(error) }),
      'error',
    )
  } finally {
    loading.value = false
  }
}

const parseNumber = (raw: string) => {
  const trimmed = String(raw ?? '').trim()
  if (!trimmed) return 0
  const value = Number(trimmed)
  return Number.isFinite(value) ? value : NaN
}

const buildRowFromForm = (): ModelPricingRow | null => {
  const model = form.model.trim()
  if (!model) {
    showToast(t('components.general.modelPricing.toast.modelRequired'), 'warning')
    return null
  }

  const input1m = parseNumber(form.inputUsdPer1M)
  const output1m = parseNumber(form.outputUsdPer1M)
  const reasoning1m = parseNumber(form.reasoningUsdPer1M)
  const cacheCreate1m = parseNumber(form.cacheCreateUsdPer1M)
  const cacheRead1m = parseNumber(form.cacheReadUsdPer1M)
  const eph1m = parseNumber(form.ephemeral1hUsdPer1M)

  const numbers = [
    ['input', input1m],
    ['output', output1m],
    ['reasoning', reasoning1m],
    ['cacheCreate', cacheCreate1m],
    ['cacheRead', cacheRead1m],
    ['ephemeral1h', eph1m],
  ] as const
  for (const [name, value] of numbers) {
    if (Number.isNaN(value) || value < 0) {
      showToast(t('components.general.modelPricing.toast.invalidNumber', { field: name }), 'warning')
      return null
    }
  }

  return {
    model,
    input_cost_per_token: per1MToPerToken(input1m),
    output_cost_per_token: per1MToPerToken(output1m),
    output_cost_per_reasoning_token: per1MToPerToken(reasoning1m),
    cache_creation_input_token_cost: per1MToPerToken(cacheCreate1m),
    cache_read_input_token_cost: per1MToPerToken(cacheRead1m),
    ephemeral_1h_cost_per_token: per1MToPerToken(eph1m),
    is_override: true,
    is_custom: mode.value === 'new',
  }
}

const save = async () => {
  if (saving.value || removing.value) return
  const payload = buildRowFromForm()
  if (!payload) return

  saving.value = true
  try {
    await upsertModelPricing(payload)
    showToast(t('components.general.modelPricing.toast.saveSuccess'))
    mode.value = 'edit'
    selectedModel.value = payload.model
    await loadRows()
  } catch (error) {
    showToast(
      t('components.general.modelPricing.toast.saveFailed', { error: extractErrorMessage(error) }),
      'error',
    )
  } finally {
    saving.value = false
  }
}

const removeOverride = async () => {
  if (saving.value || removing.value) return
  const model = form.model.trim()
  if (!model) return
  const wasCustom = selectedRow.value?.is_custom ?? false

  removing.value = true
  try {
    await deleteModelPricing(model)
    showToast(
      wasCustom
        ? t('components.general.modelPricing.toast.deleteSuccess')
        : t('components.general.modelPricing.toast.resetSuccess'),
    )
    await loadRows()

    // 如果是自定义模型，删除后就退出编辑态
    if (wasCustom) {
      resetForm()
      return
    }

    const found = rows.value.find((item) => item.model === model)
    if (found) {
      fillFormFromRow(found)
    } else {
      resetForm()
    }
  } catch (error) {
    showToast(
      t('components.general.modelPricing.toast.removeFailed', { error: extractErrorMessage(error) }),
      'error',
    )
  } finally {
    removing.value = false
  }
}

watch(
  () => props.open,
  (open) => {
    if (!open) return
    // 每次打开都刷新一次，避免后端价格表更新后前端显示滞后
    void loadRows()
    if (!selectedModel.value && mode.value !== 'new') {
      resetForm()
    }
  },
)
</script>

<template>
  <InlineModal :open="open" :title="$t('components.general.modelPricing.title')" @close="emit('close')">
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
        <button type="button" class="action-btn" @click="resetForm">
          {{ $t('components.general.modelPricing.add') }}
        </button>
        <button type="button" class="action-btn" :disabled="loading" @click="loadRows">
          {{ loading ? $t('components.general.modelPricing.loading') : $t('components.general.modelPricing.refresh') }}
        </button>
      </div>

      <div class="model-pricing-grid">
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
                @click="selectRow(item)"
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

        <div class="model-pricing-editor">
          <div class="editor-title">
            {{ mode === 'new' ? $t('components.general.modelPricing.editor.create') : $t('components.general.modelPricing.editor.edit') }}
            <span v-if="selectedRow?.is_override || selectedRow?.is_custom" class="editor-badge">
              {{ selectedRow?.is_custom ? $t('components.general.modelPricing.badge.custom') : $t('components.general.modelPricing.badge.override') }}
            </span>
          </div>

          <div class="editor-hint">
            {{ $t('components.general.modelPricing.unitHint') }}
          </div>

          <div class="editor-form">
            <label class="editor-label">{{ $t('components.general.modelPricing.fields.model') }}</label>
            <input v-model="form.model" class="mac-input editor-input" :disabled="mode === 'edit'" />

            <label class="editor-label">{{ $t('components.general.modelPricing.fields.input') }}</label>
            <input v-model="form.inputUsdPer1M" type="number" step="0.0001" min="0" class="mac-input editor-input" />

            <label class="editor-label">{{ $t('components.general.modelPricing.fields.output') }}</label>
            <input v-model="form.outputUsdPer1M" type="number" step="0.0001" min="0" class="mac-input editor-input" />

            <label class="editor-label">{{ $t('components.general.modelPricing.fields.reasoning') }}</label>
            <input v-model="form.reasoningUsdPer1M" type="number" step="0.0001" min="0" class="mac-input editor-input" />

            <label class="editor-label">{{ $t('components.general.modelPricing.fields.cacheCreate') }}</label>
            <input v-model="form.cacheCreateUsdPer1M" type="number" step="0.0001" min="0" class="mac-input editor-input" />

            <label class="editor-label">{{ $t('components.general.modelPricing.fields.cacheRead') }}</label>
            <input v-model="form.cacheReadUsdPer1M" type="number" step="0.0001" min="0" class="mac-input editor-input" />

            <label class="editor-label">{{ $t('components.general.modelPricing.fields.ephemeral1h') }}</label>
            <input v-model="form.ephemeral1hUsdPer1M" type="number" step="0.0001" min="0" class="mac-input editor-input" />
          </div>

          <div class="editor-actions">
            <button
              v-if="mode === 'edit'"
              type="button"
              class="action-btn"
              :disabled="saving || removing || (!selectedRow?.is_override && !selectedRow?.is_custom)"
              @click="removeOverride"
            >
              {{
                removing
                  ? $t('components.general.modelPricing.removing')
                  : selectedRow?.is_custom
                    ? $t('components.general.modelPricing.delete')
                    : $t('components.general.modelPricing.reset')
              }}
            </button>

            <button type="button" class="primary-btn" :disabled="saving || removing" @click="save">
              {{ saving ? $t('common.saving') : $t('common.save') }}
            </button>
          </div>
        </div>
      </div>
    </div>
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

.model-pricing-grid {
  display: grid;
  grid-template-columns: minmax(280px, 1fr) minmax(260px, 360px);
  gap: 14px;
  align-items: start;
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

.model-pricing-editor {
  border: 1px solid var(--mac-border);
  border-radius: 10px;
  padding: 14px;
  background: var(--mac-surface);
}

.editor-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-weight: 700;
  color: var(--mac-text);
}

.editor-badge {
  font-size: 12px;
  font-weight: 600;
  color: var(--mac-text-secondary);
}

.editor-hint {
  margin-top: 6px;
  font-size: 12px;
  color: var(--mac-text-secondary);
}

.editor-form {
  margin-top: 12px;
  display: grid;
  grid-template-columns: 1fr;
  gap: 10px;
}

.editor-label {
  font-size: 12px;
  color: var(--mac-text-secondary);
}

.editor-input {
  width: 100%;
  min-width: 0;
}

.editor-actions {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

@media (max-width: 860px) {
  .model-pricing-grid {
    grid-template-columns: 1fr;
  }
}
</style>
