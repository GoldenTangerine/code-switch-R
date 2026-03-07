<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import InlineModal from '../common/InlineModal.vue'
import { useI18n } from 'vue-i18n'
import { deleteModelPricing, upsertModelPricing, type ModelPricingRow } from '../../services/modelPricing'
import { extractErrorMessage } from '../../utils/error'
import { showToast } from '../../utils/toast'

type EditMode = 'edit' | 'new'
type CacheFieldMode = 'price' | 'multiplier'

const props = defineProps<{
  open: boolean
  mode: EditMode
  row: ModelPricingRow | null
  rows: ModelPricingRow[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved', model: string): void
  (e: 'removed', model: string): void
}>()

const { t } = useI18n()

const saving = ref(false)
const removing = ref(false)

const form = reactive({
  originalModel: '',
  templateModel: '',
  model: '',
  inputUsdPer1M: '',
  outputUsdPer1M: '',
  reasoningUsdPer1M: '',
  cacheCreateUsdPer1M: '',
  cacheReadUsdPer1M: '',
  cacheCreateMultiplier: '',
  cacheReadMultiplier: '',
  ephemeral1hUsdPer1M: '',
})

const cacheFieldMode = reactive<{ create: CacheFieldMode; read: CacheFieldMode }>({
  create: 'price',
  read: 'price',
})

const templateOptions = computed(() => [...props.rows].sort((a, b) => a.model.localeCompare(b.model)))

const perTokenToPer1M = (value: number) => (Number.isFinite(value) ? value * 1_000_000 : 0)
const per1MToPerToken = (value: number) => (Number.isFinite(value) ? value / 1_000_000 : 0)

const formatEditableNumber = (value: number | null | undefined) => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return ''
  if (value === 0) return '0'
  const digits = value >= 10 ? 2 : value >= 1 ? 3 : value >= 0.1 ? 4 : 6
  return value.toFixed(digits).replace(/\.?0+$/, '')
}

const modalTitle = computed(() => {
  const action =
    props.mode === 'new'
      ? t('components.general.modelPricing.editor.create')
      : t('components.general.modelPricing.editor.edit')
  return `${t('components.general.modelPricing.title')} - ${action}`
})

const badgeText = computed(() => {
  if (!props.row) return ''
  if (props.row.is_custom) return t('components.general.modelPricing.badge.custom')
  if (props.row.is_override) return t('components.general.modelPricing.badge.override')
  return ''
})

const badgeClass = computed(() => {
  if (!props.row) return 'tag-neutral'
  if (props.row.is_custom) return 'tag-custom'
  if (props.row.is_override) return 'tag-override'
  return 'tag-neutral'
})

const canRemove = computed(() => props.mode === 'edit')

const renameHint = computed(() => {
  if (props.mode !== 'edit' || !props.row) return ''
  if (props.row.is_override || props.row.is_custom) {
    return t('components.general.modelPricing.renameHintCustom')
  }
  return t('components.general.modelPricing.renameHintBuiltin')
})

const parseNumber = (raw: string) => {
  const trimmed = String(raw ?? '').trim()
  if (!trimmed) return 0
  const value = Number(trimmed)
  return Number.isFinite(value) ? value : NaN
}

const parseOptionalNumber = (raw: string) => {
  const trimmed = String(raw ?? '').trim()
  if (!trimmed) return null
  const value = Number(trimmed)
  return Number.isFinite(value) ? value : null
}

const calculateMultiplierFromPrices = (cachePriceRaw: string, inputPriceRaw: string) => {
  const cachePrice = parseOptionalNumber(cachePriceRaw)
  const inputPrice = parseOptionalNumber(inputPriceRaw)
  if (cachePrice === null || inputPrice === null || cachePrice < 0 || inputPrice < 0) return null
  if (inputPrice === 0) return cachePrice === 0 ? 0 : null
  return cachePrice / inputPrice
}

const applyMultiplierToPrice = (inputPriceRaw: string, multiplierRaw: string) => {
  const inputPrice = parseOptionalNumber(inputPriceRaw)
  const multiplier = parseOptionalNumber(multiplierRaw)
  if (multiplier === null) return ''
  if (inputPrice === null || inputPrice < 0 || multiplier < 0) return ''
  if (inputPrice === 0) return multiplier === 0 ? '0' : ''
  return formatEditableNumber(inputPrice * multiplier)
}

const syncCacheCreateMultiplierFromPrice = () => {
  form.cacheCreateMultiplier = formatEditableNumber(
    calculateMultiplierFromPrices(form.cacheCreateUsdPer1M, form.inputUsdPer1M),
  )
}

const syncCacheReadMultiplierFromPrice = () => {
  form.cacheReadMultiplier = formatEditableNumber(
    calculateMultiplierFromPrices(form.cacheReadUsdPer1M, form.inputUsdPer1M),
  )
}

const syncCacheCreatePriceFromMultiplier = () => {
  form.cacheCreateUsdPer1M = applyMultiplierToPrice(form.inputUsdPer1M, form.cacheCreateMultiplier)
}

const syncCacheReadPriceFromMultiplier = () => {
  form.cacheReadUsdPer1M = applyMultiplierToPrice(form.inputUsdPer1M, form.cacheReadMultiplier)
}

const syncCacheFieldsFromInput = () => {
  if (cacheFieldMode.create === 'multiplier') {
    syncCacheCreatePriceFromMultiplier()
  } else {
    syncCacheCreateMultiplierFromPrice()
  }

  if (cacheFieldMode.read === 'multiplier') {
    syncCacheReadPriceFromMultiplier()
  } else {
    syncCacheReadMultiplierFromPrice()
  }
}

const handleInputUsdChange = () => {
  syncCacheFieldsFromInput()
}

const handleCacheCreatePriceInput = () => {
  cacheFieldMode.create = 'price'
  syncCacheCreateMultiplierFromPrice()
}

const handleCacheReadPriceInput = () => {
  cacheFieldMode.read = 'price'
  syncCacheReadMultiplierFromPrice()
}

const handleCacheCreateMultiplierInput = () => {
  cacheFieldMode.create = 'multiplier'
  syncCacheCreatePriceFromMultiplier()
}

const handleCacheReadMultiplierInput = () => {
  cacheFieldMode.read = 'multiplier'
  syncCacheReadPriceFromMultiplier()
}

const resetForm = () => {
  form.originalModel = ''
  form.templateModel = ''
  form.model = ''
  form.inputUsdPer1M = ''
  form.outputUsdPer1M = ''
  form.reasoningUsdPer1M = ''
  form.cacheCreateUsdPer1M = ''
  form.cacheReadUsdPer1M = ''
  form.cacheCreateMultiplier = ''
  form.cacheReadMultiplier = ''
  form.ephemeral1hUsdPer1M = ''
  cacheFieldMode.create = 'price'
  cacheFieldMode.read = 'price'
}

const assignPricingFields = (row: ModelPricingRow) => {
  form.inputUsdPer1M = formatEditableNumber(perTokenToPer1M(row.input_cost_per_token))
  form.outputUsdPer1M = formatEditableNumber(perTokenToPer1M(row.output_cost_per_token))
  form.reasoningUsdPer1M = formatEditableNumber(perTokenToPer1M(row.output_cost_per_reasoning_token))
  form.cacheCreateUsdPer1M = formatEditableNumber(perTokenToPer1M(row.cache_creation_input_token_cost))
  form.cacheReadUsdPer1M = formatEditableNumber(perTokenToPer1M(row.cache_read_input_token_cost))
  form.ephemeral1hUsdPer1M = formatEditableNumber(perTokenToPer1M(row.ephemeral_1h_cost_per_token))
  cacheFieldMode.create = 'price'
  cacheFieldMode.read = 'price'
  syncCacheCreateMultiplierFromPrice()
  syncCacheReadMultiplierFromPrice()
}

const fillFormFromRow = (row: ModelPricingRow) => {
  form.originalModel = row.model
  form.templateModel = ''
  form.model = row.model
  assignPricingFields(row)
}

const applyTemplate = (modelName: string) => {
  const template = props.rows.find((item) => item.model === modelName)
  if (!template) return
  const currentModel = form.model
  const originalModel = form.originalModel
  assignPricingFields(template)
  form.model = currentModel
  form.originalModel = originalModel
}

const syncFromProps = () => {
  if (props.mode === 'edit' && props.row) {
    fillFormFromRow(props.row)
    return
  }
  resetForm()
}

watch(
  () => props.open,
  (open) => {
    if (!open) return
    syncFromProps()
  },
)

watch(
  () => [props.mode, props.row] as const,
  () => {
    if (!props.open) return
    syncFromProps()
  },
)

watch(
  () => form.templateModel,
  (templateModel, previous) => {
    if (!props.open || props.mode !== 'new') return
    if (!templateModel || templateModel === previous) return
    applyTemplate(templateModel)
  },
)

const buildRowFromForm = (): ModelPricingRow | null => {
  const model = form.model.trim()
  if (!model) {
    showToast(t('components.general.modelPricing.toast.modelRequired'), 'warning')
    return null
  }

  if (
    props.mode === 'edit' &&
    form.originalModel.trim() !== '' &&
    form.originalModel.trim() !== model &&
    props.rows.some((item) => item.model === model && item.model !== form.originalModel.trim())
  ) {
    showToast(t('components.general.modelPricing.toast.modelConflict', { model }), 'warning')
    return null
  }

  const input1m = parseNumber(form.inputUsdPer1M)
  const output1m = parseNumber(form.outputUsdPer1M)
  const reasoning1m = parseNumber(form.reasoningUsdPer1M)
  const cacheCreate1m = parseNumber(form.cacheCreateUsdPer1M)
  const cacheRead1m = parseNumber(form.cacheReadUsdPer1M)
  const eph1m = parseNumber(form.ephemeral1hUsdPer1M)
  const cacheCreateMultiplier = parseNumber(form.cacheCreateMultiplier)
  const cacheReadMultiplier = parseNumber(form.cacheReadMultiplier)

  const numbers = [
    [t('components.general.modelPricing.fields.input'), input1m],
    [t('components.general.modelPricing.fields.output'), output1m],
    [t('components.general.modelPricing.fields.reasoning'), reasoning1m],
    [t('components.general.modelPricing.fields.cacheCreate'), cacheCreate1m],
    [t('components.general.modelPricing.fields.cacheRead'), cacheRead1m],
    [t('components.general.modelPricing.fields.cacheCreateMultiplier'), cacheCreateMultiplier],
    [t('components.general.modelPricing.fields.cacheReadMultiplier'), cacheReadMultiplier],
    [t('components.general.modelPricing.fields.ephemeral1h'), eph1m],
  ] as const

  for (const [name, value] of numbers) {
    if (Number.isNaN(value) || value < 0) {
      showToast(t('components.general.modelPricing.toast.invalidNumber', { field: name }), 'warning')
      return null
    }
  }

  if (input1m <= 0 && cacheCreateMultiplier > 0) {
    showToast(t('components.general.modelPricing.toast.multiplierRequiresInput'), 'warning')
    return null
  }
  if (input1m <= 0 && cacheReadMultiplier > 0) {
    showToast(t('components.general.modelPricing.toast.multiplierRequiresInput'), 'warning')
    return null
  }

  const finalCacheCreate1m =
    cacheFieldMode.create === 'multiplier' ? input1m * cacheCreateMultiplier : cacheCreate1m
  const finalCacheRead1m =
    cacheFieldMode.read === 'multiplier' ? input1m * cacheReadMultiplier : cacheRead1m

  return {
    original_model: props.mode === 'edit' ? form.originalModel.trim() : undefined,
    model,
    input_cost_per_token: per1MToPerToken(input1m),
    output_cost_per_token: per1MToPerToken(output1m),
    output_cost_per_reasoning_token: per1MToPerToken(reasoning1m),
    cache_creation_input_token_cost: per1MToPerToken(finalCacheCreate1m),
    cache_read_input_token_cost: per1MToPerToken(finalCacheRead1m),
    ephemeral_1h_cost_per_token: per1MToPerToken(eph1m),
    is_override: true,
    is_custom: props.mode === 'new',
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
    emit('saved', payload.model)
    emit('close')
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
  const targetModel = (props.row?.model ?? form.originalModel ?? form.model).trim()
  if (!targetModel) return
  const wasCustom = props.row?.is_custom ?? false

  removing.value = true
  try {
    await deleteModelPricing(targetModel)
    showToast(
      wasCustom
        ? t('components.general.modelPricing.toast.deleteSuccess')
        : t('components.general.modelPricing.toast.resetSuccess'),
    )
    emit('removed', targetModel)
    emit('close')
  } catch (error) {
    showToast(
      t('components.general.modelPricing.toast.removeFailed', { error: extractErrorMessage(error) }),
      'error',
    )
  } finally {
    removing.value = false
  }
}
</script>

<template>
  <InlineModal
    :open="open"
    :title="modalTitle"
    panel-width="min(920px, 96vw)"
    :close-on-backdrop="false"
    @close="emit('close')"
  >
    <div class="model-pricing-editor-modal">
      <div class="editor-title">
        <span>{{ mode === 'new' ? $t('components.general.modelPricing.editor.create') : $t('components.general.modelPricing.editor.edit') }}</span>
        <span v-if="badgeText" class="tag" :class="badgeClass">{{ badgeText }}</span>
      </div>

      <p class="pricing-hint">
        {{ $t('components.general.modelPricing.unitHint') }}
      </p>

      <p v-if="renameHint" class="rename-hint">
        {{ renameHint }}
      </p>

      <div class="editor-form">
        <div v-if="mode === 'new' && templateOptions.length > 0" class="editor-field full-width">
          <label class="editor-label">{{ $t('components.general.modelPricing.fields.template') }}</label>
          <select v-model="form.templateModel" class="mac-select editor-input">
            <option value="">{{ $t('components.general.modelPricing.templateEmpty') }}</option>
            <option v-for="item in templateOptions" :key="item.model" :value="item.model">
              {{ item.model }}
            </option>
          </select>
        </div>

        <div class="editor-field full-width">
          <label class="editor-label">{{ $t('components.general.modelPricing.fields.model') }}</label>
          <input v-model="form.model" class="mac-input editor-input" />
        </div>

        <div class="editor-field">
          <label class="editor-label">{{ $t('components.general.modelPricing.fields.input') }}</label>
          <input v-model="form.inputUsdPer1M" type="number" step="0.0001" min="0" class="mac-input editor-input" @input="handleInputUsdChange" />
        </div>

        <div class="editor-field">
          <label class="editor-label">{{ $t('components.general.modelPricing.fields.output') }}</label>
          <input v-model="form.outputUsdPer1M" type="number" step="0.0001" min="0" class="mac-input editor-input" />
        </div>

        <div class="editor-field">
          <label class="editor-label">{{ $t('components.general.modelPricing.fields.reasoning') }}</label>
          <input v-model="form.reasoningUsdPer1M" type="number" step="0.0001" min="0" class="mac-input editor-input" />
        </div>

        <div class="editor-field">
          <label class="editor-label">{{ $t('components.general.modelPricing.fields.ephemeral1h') }}</label>
          <input v-model="form.ephemeral1hUsdPer1M" type="number" step="0.0001" min="0" class="mac-input editor-input" />
        </div>

        <div class="editor-field">
          <label class="editor-label">{{ $t('components.general.modelPricing.fields.cacheCreate') }}</label>
          <input v-model="form.cacheCreateUsdPer1M" type="number" step="0.0001" min="0" class="mac-input editor-input" @input="handleCacheCreatePriceInput" />
        </div>

        <div class="editor-field">
          <label class="editor-label">{{ $t('components.general.modelPricing.fields.cacheCreateMultiplier') }}</label>
          <input v-model="form.cacheCreateMultiplier" type="number" step="0.0001" min="0" class="mac-input editor-input" @input="handleCacheCreateMultiplierInput" />
        </div>

        <div class="editor-field">
          <label class="editor-label">{{ $t('components.general.modelPricing.fields.cacheRead') }}</label>
          <input v-model="form.cacheReadUsdPer1M" type="number" step="0.0001" min="0" class="mac-input editor-input" @input="handleCacheReadPriceInput" />
        </div>

        <div class="editor-field">
          <label class="editor-label">{{ $t('components.general.modelPricing.fields.cacheReadMultiplier') }}</label>
          <input v-model="form.cacheReadMultiplier" type="number" step="0.0001" min="0" class="mac-input editor-input" @input="handleCacheReadMultiplierInput" />
        </div>
      </div>

      <footer class="editor-actions">
        <button
          v-if="canRemove"
          type="button"
          class="action-btn"
          :disabled="saving || removing || (!row?.is_override && !row?.is_custom)"
          @click="removeOverride"
        >
          {{
            removing
              ? $t('components.general.modelPricing.removing')
              : row?.is_custom
                ? $t('components.general.modelPricing.delete')
                : $t('components.general.modelPricing.reset')
          }}
        </button>

        <button type="button" class="action-btn" :disabled="saving || removing" @click="emit('close')">
          {{ $t('common.cancel') }}
        </button>

        <button type="button" class="primary-btn" :disabled="saving || removing" @click="save">
          {{ saving ? $t('common.saving') : $t('common.save') }}
        </button>
      </footer>
    </div>
  </InlineModal>
</template>

<style scoped>
.model-pricing-editor-modal {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 420px;
}

.editor-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-weight: 700;
  color: var(--mac-text);
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

.rename-hint {
  margin: 0;
  font-size: 0.78rem;
  line-height: 1.5;
  color: var(--mac-text-secondary);
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

.tag-neutral {
  color: var(--mac-text-secondary);
  background: rgba(148, 163, 184, 0.12);
  border-color: rgba(148, 163, 184, 0.18);
}

.editor-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.editor-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.editor-field.full-width {
  grid-column: 1 / -1;
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
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 4px;
}

@media (max-width: 720px) {
  .editor-form {
    grid-template-columns: 1fr;
  }
}
</style>
