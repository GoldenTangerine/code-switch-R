<template>
  <div class="model-mapping-editor">
    <div class="editor-header">
      <label class="editor-label">
        <span>{{ $t('components.provider.modelMapping.label') }}</span>
        <button
          type="button"
          class="help-icon"
          :data-tooltip="$t('components.provider.modelMapping.tooltip')"
        >
          <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
            <path
              d="M8 1a7 7 0 100 14A7 7 0 008 1zm0 13A6 6 0 118 2a6 6 0 010 12zm0-9.5a.75.75 0 01.75.75v4a.75.75 0 01-1.5 0v-4A.75.75 0 018 4.5zm0 7.5a1 1 0 100-2 1 1 0 000 2z"
              fill="currentColor"
            />
          </svg>
        </button>
      </label>
    </div>

    <div v-if="mappingList.length > 0" class="mapping-list">
      <div
        v-for="mapping in mappingList"
        :key="mapping.key"
        class="mapping-row"
        :class="{ 'is-editing': editingOriginalKey === mapping.key }"
      >
        <div class="mapping-content">
          <code class="mapping-key" :class="{ wildcard: isWildcard(mapping.key) }">
            {{ mapping.key }}
          </code>
          <svg class="mapping-arrow" viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
            <path
              d="M6 4l4 4-4 4"
              fill="none"
              stroke="currentColor"
              stroke-width="1.5"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
          <code class="mapping-value" :class="{ wildcard: isWildcard(mapping.value) }">
            {{ mapping.value }}
          </code>
        </div>

        <div class="mapping-row-actions">
          <button
            type="button"
            class="mapping-action"
            :aria-label="$t('components.provider.modelMapping.edit')"
            @click="startEditing(mapping.key, mapping.value)"
          >
            <svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
              <path
                d="M11.5 2.5l2 2L6 12l-3 .5.5-3 7.5-7z"
                fill="none"
                stroke="currentColor"
                stroke-width="1.25"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </button>
          <button
            type="button"
            class="mapping-remove"
            :aria-label="$t('components.provider.modelMapping.remove')"
            @click="removeMapping(mapping.key)"
          >
            <svg viewBox="0 0 12 12" width="10" height="10" aria-hidden="true">
              <path
                d="M3 3l6 6M9 3l-6 6"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
              />
            </svg>
          </button>
        </div>
      </div>
    </div>

    <div class="mapping-input-panel" :class="{ 'is-editing': isEditing }">
      <div class="mapping-input-row">
        <div class="mapping-field mapping-field--key">
          <SearchableModelInput
            v-model="newKey"
            :placeholder="$t('components.provider.modelMapping.keyPlaceholder')"
            :options="builtinModelOptions"
            :empty-text="$t('components.provider.modelMapping.builtinNoResults')"
            @keydown.enter.prevent="focusValueInput"
            @select="handleBuiltinModelSelect"
          />
        </div>

        <svg class="input-arrow" viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
          <path
            d="M6 4l4 4-4 4"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>

        <div class="mapping-field">
          <BaseInput
            ref="valueInputRef"
            v-model="newValue"
            type="text"
            :placeholder="$t('components.provider.modelMapping.valuePlaceholder')"
            @keydown.enter.prevent="submitMapping"
          />
        </div>

        <div class="mapping-actions">
          <BaseButton type="button" @click="submitMapping">
            {{ isEditing ? $t('components.provider.modelMapping.save') : $t('components.provider.modelMapping.add') }}
          </BaseButton>
          <BaseButton
            v-if="isEditing"
            type="button"
            variant="outline"
            @click="cancelEditing"
          >
            {{ $t('components.provider.modelMapping.cancel') }}
          </BaseButton>
        </div>
      </div>

      <p v-if="builtinPickerHint" class="mapping-picker-hint">
        {{ builtinPickerHint }}
      </p>
      <p v-if="isEditing" class="mapping-editing-hint">
        {{ $t('components.provider.modelMapping.editingHint', { key: editingOriginalKey }) }}
      </p>
      <p v-if="inputError" class="mapping-input-error" role="alert">
        {{ inputError }}
      </p>
    </div>

    <div class="help-text">
      <p class="help-example">
        <strong>{{ $t('components.provider.modelMapping.examples.title') }}</strong>
      </p>
      <ul class="help-list">
        <li>
          <code>claude-sonnet-4</code> → <code>anthropic/claude-sonnet-4</code><br />
          <span class="help-desc">{{ $t('components.provider.modelMapping.examples.exact') }}</span>
        </li>
        <li>
          <code>claude-*</code> → <code>anthropic/claude-*</code><br />
          <span class="help-desc">{{ $t('components.provider.modelMapping.examples.wildcard') }}</span>
        </li>
        <li>
          <code>gpt-*</code> → <code>openai/gpt-*</code><br />
          <span class="help-desc">{{ $t('components.provider.modelMapping.examples.prefix') }}</span>
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CLIPlatform } from '../../services/cliConfig'
import { listModelPricing, type ModelPricingRow } from '../../services/modelPricing'
import { buildBuiltinModelOptions } from '../../utils/builtinModels'
import BaseInput from './BaseInput.vue'
import BaseButton from './BaseButton.vue'
import SearchableModelInput from './SearchableModelInput.vue'

interface Props {
  modelValue?: Record<string, string>
  platform?: CLIPlatform
}

interface Emits {
  (e: 'update:modelValue', value: Record<string, string>): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const { t } = useI18n()

const mappingList = computed(() => {
  if (!props.modelValue) return []
  return Object.entries(props.modelValue).map(([key, value]) => ({ key, value }))
})

const newKey = ref('')
const newValue = ref('')
const editingOriginalKey = ref('')
const inputError = ref('')
const builtinModelRows = ref<ModelPricingRow[]>([])
const builtinModelLoading = ref(false)
const builtinModelLoadFailed = ref(false)
const valueInputRef = ref<InstanceType<typeof BaseInput> | null>(null)

const isEditing = computed(() => editingOriginalKey.value !== '')
const builtinModelOptions = computed(() => buildBuiltinModelOptions(builtinModelRows.value, props.platform))
const builtinPickerHint = computed(() => {
  if (!props.platform) return ''
  if (builtinModelLoading.value) {
    return t('components.provider.modelMapping.builtinLoadingHint')
  }
  if (builtinModelLoadFailed.value) {
    return t('components.provider.modelMapping.builtinLoadFailedHint')
  }
  if (builtinModelOptions.value.length === 0) {
    return t('components.provider.modelMapping.builtinEmptyHint')
  }
  return t('components.provider.modelMapping.builtinReadyHint', {
    count: builtinModelOptions.value.length,
  })
})

function isWildcard(text: string): boolean {
  return text.includes('*')
}

function hasMappingKey(key: string): boolean {
  return Object.prototype.hasOwnProperty.call(props.modelValue || {}, key)
}

function hasConflictingKey(key: string): boolean {
  if (!key || !hasMappingKey(key)) return false
  return !isEditing.value || key !== editingOriginalKey.value
}

function resetDraft(): void {
  newKey.value = ''
  newValue.value = ''
  editingOriginalKey.value = ''
  inputError.value = ''
}

function focusValueInput(): void {
  const inputElement = (valueInputRef.value as any)?.$el as HTMLInputElement | undefined
  inputElement?.focus()
}

function handleBuiltinModelSelect(): void {
  inputError.value = ''
  focusValueInput()
}

function startEditing(key: string, value: string): void {
  editingOriginalKey.value = key
  newKey.value = key
  newValue.value = value
  inputError.value = ''
}

function cancelEditing(): void {
  resetDraft()
}

function submitMapping(): void {
  const key = newKey.value.trim()
  const value = newValue.value.trim()

  if (!key || !value) return

  if (hasConflictingKey(key)) {
    inputError.value = t('components.provider.modelMapping.duplicateError')
    return
  }

  const updated = { ...(props.modelValue || {}) }
  if (isEditing.value && editingOriginalKey.value !== key) {
    delete updated[editingOriginalKey.value]
  }
  updated[key] = value
  emit('update:modelValue', updated)
  resetDraft()
}

function removeMapping(key: string): void {
  const updated = { ...(props.modelValue || {}) }
  delete updated[key]
  emit('update:modelValue', updated)

  if (editingOriginalKey.value === key) {
    resetDraft()
    return
  }

  if (!hasConflictingKey(newKey.value.trim())) {
    inputError.value = ''
  }
}

async function loadBuiltinModelRows(): Promise<void> {
  if (builtinModelRows.value.length > 0 || builtinModelLoading.value) return

  builtinModelLoading.value = true
  builtinModelLoadFailed.value = false

  try {
    builtinModelRows.value = await listModelPricing()
  } catch (error) {
    builtinModelRows.value = []
    builtinModelLoadFailed.value = true
    console.error('Failed to load builtin model rows for mapping editor:', error)
  } finally {
    builtinModelLoading.value = false
  }
}

watch([newKey, newValue], () => {
  inputError.value = ''
})

watch(() => props.modelValue, () => {
  if (editingOriginalKey.value && !hasMappingKey(editingOriginalKey.value)) {
    resetDraft()
    return
  }

  if (inputError.value && !hasConflictingKey(newKey.value.trim())) {
    inputError.value = ''
  }
}, { deep: true })

watch(() => props.platform, (platform) => {
  if (platform) {
    void loadBuiltinModelRows()
  }
}, { immediate: true })
</script>

<style scoped>
.model-mapping-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.editor-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
  font-size: 0.875rem;
  color: var(--foreground);
}

.help-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px;
  border: none;
  background: none;
  color: var(--foreground-muted);
  cursor: help;
  border-radius: 4px;
  transition: all 0.2s;
}

.help-icon:hover {
  color: var(--foreground);
  background-color: var(--background-hover);
}

.mapping-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  background-color: var(--background-secondary);
  border-radius: 8px;
}

.mapping-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  background-color: var(--background);
  border: 1px solid var(--border);
  border-radius: 6px;
  transition: all 0.2s;
}

.mapping-row:hover {
  background-color: var(--background-hover);
}

.mapping-row.is-editing {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent-primary) 28%, transparent);
}

.mapping-content {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

.mapping-key,
.mapping-value {
  padding: 3px 7px;
  background-color: var(--background-secondary);
  border: 1px solid var(--border);
  border-radius: 4px;
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 0.75rem;
  color: var(--foreground);
  word-break: break-all;
}

.mapping-key.wildcard,
.mapping-value.wildcard {
  color: var(--accent-primary);
  font-weight: 500;
}

.mapping-arrow,
.input-arrow {
  flex-shrink: 0;
  color: var(--foreground-muted);
}

.mapping-row-actions {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.mapping-action,
.mapping-remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 4px;
  border: none;
  background: none;
  color: var(--foreground-muted);
  cursor: pointer;
  border-radius: 3px;
  transition: all 0.2s;
}

.mapping-action:hover {
  color: var(--accent-primary);
  background-color: color-mix(in srgb, var(--accent-primary) 10%, transparent);
}

.mapping-remove:hover {
  color: var(--error);
  background-color: var(--error-bg);
}

.mapping-input-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background-color: color-mix(in srgb, var(--background-secondary) 92%, transparent);
}

.mapping-input-panel.is-editing {
  border-color: var(--accent-primary);
  background-color: color-mix(in srgb, var(--accent-primary) 6%, var(--background-secondary));
}

.mapping-input-row {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) auto minmax(0, 1fr) auto;
  gap: 8px;
  align-items: flex-start;
}

.mapping-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.mapping-field--key :deep(input),
.mapping-field :deep(input) {
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
}

.mapping-actions {
  display: flex;
  gap: 8px;
  align-self: flex-end;
}

.mapping-picker-hint,
.mapping-editing-hint,
.mapping-input-error {
  margin: 0;
  font-size: 0.75rem;
  line-height: 1.5;
}

.mapping-picker-hint,
.mapping-editing-hint {
  color: var(--foreground-muted);
}

.mapping-editing-hint {
  color: var(--accent-primary);
}

.mapping-input-error {
  color: var(--error);
}

.help-text {
  padding: 12px;
  background-color: var(--background-secondary);
  border-radius: 8px;
  font-size: 0.8125rem;
  color: var(--foreground-muted);
}

.help-example {
  margin-bottom: 8px;
  color: var(--foreground);
}

.help-list {
  margin: 0;
  padding-left: 20px;
  list-style: disc;
}

.help-list li {
  margin-bottom: 8px;
  line-height: 1.5;
}

.help-list code {
  padding: 2px 6px;
  background-color: var(--background);
  border: 1px solid var(--border);
  border-radius: 4px;
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 0.75rem;
  color: var(--accent-primary);
}

.help-desc {
  font-size: 0.75rem;
  color: var(--foreground-muted);
  font-style: italic;
}

@media (max-width: 768px) {
  .mapping-row {
    align-items: flex-start;
    flex-direction: column;
  }

  .mapping-row-actions {
    align-self: flex-end;
  }

  .mapping-input-row {
    grid-template-columns: 1fr;
  }

  .input-arrow {
    display: none;
  }

  .mapping-actions {
    align-self: stretch;
  }

  .mapping-actions > * {
    flex: 1;
  }
}
</style>
