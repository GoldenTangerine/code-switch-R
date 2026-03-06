<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import InlineModal from '../common/InlineModal.vue'
import { useI18n } from 'vue-i18n'
import { deleteModelPricing, upsertModelPricing, type ModelPricingRow } from '../../services/modelPricing'
import { extractErrorMessage } from '../../utils/error'
import { showToast } from '../../utils/toast'

type EditMode = 'edit' | 'new'

const props = defineProps<{
  open: boolean
  mode: EditMode
  row: ModelPricingRow | null
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
  return per1m.toFixed(digits).replace(/\\.?0+$/, '')
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

const resetForm = () => {
  form.model = ''
  form.inputUsdPer1M = ''
  form.outputUsdPer1M = ''
  form.reasoningUsdPer1M = ''
  form.cacheCreateUsdPer1M = ''
  form.cacheReadUsdPer1M = ''
  form.ephemeral1hUsdPer1M = ''
}

const fillFormFromRow = (row: ModelPricingRow) => {
  form.model = row.model
  form.inputUsdPer1M = formatPer1M(row.input_cost_per_token)
  form.outputUsdPer1M = formatPer1M(row.output_cost_per_token)
  form.reasoningUsdPer1M = formatPer1M(row.output_cost_per_reasoning_token)
  form.cacheCreateUsdPer1M = formatPer1M(row.cache_creation_input_token_cost)
  form.cacheReadUsdPer1M = formatPer1M(row.cache_read_input_token_cost)
  form.ephemeral1hUsdPer1M = formatPer1M(row.ephemeral_1h_cost_per_token)
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
    [t('components.general.modelPricing.fields.input'), input1m],
    [t('components.general.modelPricing.fields.output'), output1m],
    [t('components.general.modelPricing.fields.reasoning'), reasoning1m],
    [t('components.general.modelPricing.fields.cacheCreate'), cacheCreate1m],
    [t('components.general.modelPricing.fields.cacheRead'), cacheRead1m],
    [t('components.general.modelPricing.fields.ephemeral1h'), eph1m],
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
  const model = form.model.trim()
  if (!model) return
  const wasCustom = props.row?.is_custom ?? false

  removing.value = true
  try {
    await deleteModelPricing(model)
    showToast(
      wasCustom
        ? t('components.general.modelPricing.toast.deleteSuccess')
        : t('components.general.modelPricing.toast.resetSuccess'),
    )
    emit('removed', model)
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
    panel-width="min(620px, 95vw)"
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
  min-height: 360px;
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
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 4px;
}
</style>
