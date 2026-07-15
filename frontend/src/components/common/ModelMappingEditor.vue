<template>
  <div class="model-mapping-editor">
    <div class="editor-header">
      <div class="editor-heading">
        <label class="editor-label">
          <span>{{ $t('components.provider.modelMapping.label') }}</span>
          <button
            ref="helpIconRef"
            type="button"
            class="help-icon"
            :aria-label="$t('components.provider.modelMapping.tooltip')"
            :aria-describedby="helpTooltip.visible ? helpTooltipId : undefined"
            @mouseenter="showHelpTooltip"
            @mouseleave="hideHelpTooltip"
            @focus="showHelpTooltip"
            @blur="hideHelpTooltip"
            @keydown.esc="hideHelpTooltip"
          >
            <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
              <path
                d="M8 1a7 7 0 100 14A7 7 0 008 1zm0 13A6 6 0 118 2a6 6 0 010 12zm0-9.5a.75.75 0 01.75.75v4a.75.75 0 01-1.5 0v-4A.75.75 0 018 4.5zm0 7.5a1 1 0 100-2 1 1 0 000 2z"
                fill="currentColor"
              />
            </svg>
          </button>
        </label>
        <p class="editor-description">
          {{ $t('components.provider.modelMapping.description') }}
        </p>
      </div>
      <div class="mapping-count">
        {{ $t('components.provider.modelMapping.ruleCount', { count: mappingList.length }) }}
      </div>
    </div>

    <Teleport to="body">
      <div
        v-if="helpTooltip.visible"
        :id="helpTooltipId"
        ref="helpTooltipRef"
        class="model-mapping-help-tooltip"
        :class="[
          `is-${helpTooltip.placement}`,
          { 'is-positioned': helpTooltip.positioned },
        ]"
        :style="{ left: `${helpTooltip.left}px`, top: `${helpTooltip.top}px` }"
        role="tooltip"
      >
        {{ $t('components.provider.modelMapping.tooltip') }}
      </div>
    </Teleport>

    <div class="mapping-list">
      <template v-if="mappingList.length > 0">
        <div
          v-for="mapping in mappingList"
          :key="mapping.key"
          class="mapping-row"
          :class="{ 'is-editing': editingOriginalKey === mapping.key }"
        >
          <div class="mapping-content">
            <span
              class="mapping-type"
              :class="`mapping-type--${getMappingKind(mapping.key)}`"
            >
              {{ getMappingKindLabel(mapping.key) }}
            </span>

            <div class="mapping-models">
              <code class="mapping-key">
                {{ mapping.key }}
              </code>
              <svg class="mapping-arrow" viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
                <path
                  d="M5.5 4.5L9 8l-3.5 3.5M8.5 8H12"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.5"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
              <code class="mapping-value">
                {{ mapping.value }}
              </code>
            </div>
            <span
              class="mapping-effort"
              :class="{ 'is-passthrough': !mapping.reasoningEffort }"
              :title="mapping.reasoningEffort || $t('components.provider.modelMapping.reasoningEffort.passthrough')"
            >
              {{ mapping.reasoningEffort || $t('components.provider.modelMapping.reasoningEffort.passthrough') }}
            </span>
          </div>

          <div class="mapping-row-actions">
            <button
              type="button"
              class="mapping-action"
              :aria-label="$t('components.provider.modelMapping.edit')"
              @click="startEditing(mapping.key, mapping.value, mapping.reasoningEffort)"
            >
              <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
                <path
                  d="M11.5 2.5l2 2L6 12l-3 .5.5-3 7.5-7z"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.35"
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
              <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
                <path
                  d="M3.5 4.5h9M6.25 4.5V3.25h3.5V4.5m-5 2v6m3.25-6v6m3.25-6l-.4 6.25a1 1 0 01-1 .94H6.15a1 1 0 01-1-.94L4.75 6.5"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.25"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
            </button>
          </div>
        </div>
      </template>
      <div v-else class="mapping-empty">
        {{ $t('components.provider.modelMapping.empty') }}
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

        <div class="mapping-field mapping-field--effort">
          <SearchableModelInput
            v-model="newReasoningEffort"
            :options="reasoningEffortOptions"
            :placeholder="$t('components.provider.modelMapping.reasoningEffort.placeholder')"
            :empty-text="$t('components.provider.modelMapping.reasoningEffort.customHint')"
            :aria-label="$t('components.provider.modelMapping.reasoningEffort.label')"
            max-height="220px"
          />
        </div>

        <div class="mapping-actions">
          <button
            type="button"
            class="mapping-submit"
            :disabled="!canSubmitMapping"
            @click="submitMapping"
          >
            <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
              <path
                d="M8 3.5v9M3.5 8h9"
                fill="none"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
              />
            </svg>
            {{ isEditing ? $t('components.provider.modelMapping.save') : $t('components.provider.modelMapping.add') }}
          </button>
          <button
            v-if="isEditing"
            type="button"
            class="mapping-cancel"
            @click="cancelEditing"
          >
            {{ $t('components.provider.modelMapping.cancel') }}
          </button>
        </div>
      </div>

      <div class="mapping-input-meta">
        <div class="mapping-hints">
          <span>
            <svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
              <path
                d="M8 2.5l1.6 3.3 3.6.5-2.6 2.5.6 3.6L8 10.7l-3.2 1.7.6-3.6-2.6-2.5 3.6-.5L8 2.5z"
                fill="none"
                stroke="currentColor"
                stroke-width="1.2"
                stroke-linejoin="round"
              />
            </svg>
            {{ $t('components.provider.modelMapping.examples.exact') }}
          </span>
          <span>
            <svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
              <path
                d="M4 4l8 8M12 4l-8 8"
                fill="none"
                stroke="currentColor"
                stroke-width="1.3"
                stroke-linecap="round"
              />
            </svg>
            {{ $t('components.provider.modelMapping.examples.wildcard') }}
          </span>
          <label class="miss-policy-inline">
            <span>{{ $t('components.provider.modelMapping.missPolicy.label') }}</span>
            <select v-model="selectedMissPolicy" class="miss-policy-select">
              <option value="block">
                {{ $t('components.provider.modelMapping.missPolicy.block') }}
              </option>
              <option value="passthrough">
                {{ $t('components.provider.modelMapping.missPolicy.passthrough') }}
              </option>
            </select>
          </label>
        </div>
        <div class="mapping-status">
          <span v-if="inputError" class="mapping-input-error" role="alert">
            {{ inputError }}
          </span>
          <span v-else-if="isEditing" class="mapping-editing-hint">
            {{ $t('components.provider.modelMapping.editingHint', { key: editingOriginalKey }) }}
          </span>
          <span v-else-if="builtinPickerHint" class="mapping-picker-hint">
            {{ builtinPickerHint }}
          </span>
        </div>
      </div>
    </div>

    <div v-if="selectedMissPolicy === 'passthrough'" class="passthrough-panel">
      <div class="passthrough-heading">
        <span>{{ $t('components.provider.modelMapping.passthroughRules.label') }}</span>
        <span>{{ $t('components.provider.modelMapping.passthroughRules.count', { count: normalizedPassthroughPatterns.length }) }}</span>
      </div>
      <div v-if="normalizedPassthroughPatterns.length > 0" class="passthrough-tags">
        <span v-for="pattern in normalizedPassthroughPatterns" :key="pattern" class="passthrough-tag">
          <code>{{ pattern }}</code>
          <button
            type="button"
            :aria-label="$t('components.provider.modelMapping.passthroughRules.remove')"
            @click="removePassthroughPattern(pattern)"
          >
            <svg viewBox="0 0 12 12" width="10" height="10" aria-hidden="true">
              <path d="M3 3l6 6M9 3l-6 6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
            </svg>
          </button>
        </span>
      </div>
      <div class="passthrough-input-row">
        <BaseInput
          v-model="newPassthroughPattern"
          type="text"
          :placeholder="$t('components.provider.modelMapping.passthroughRules.placeholder')"
          @keydown.enter.prevent="addPassthroughPattern"
        />
        <button
          type="button"
          class="passthrough-add"
          :disabled="!newPassthroughPattern.trim()"
          @click="addPassthroughPattern"
        >
          {{ $t('components.provider.modelMapping.passthroughRules.add') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ModelMappingMissPolicy } from '../../data/cards'
import type { CLIPlatform } from '../../services/cliConfig'
import { listModelPricing, type ModelPricingRow } from '../../services/modelPricing'
import { buildBuiltinModelOptions } from '../../utils/builtinModels'
import BaseInput from './BaseInput.vue'
import { removeModelMappingRule, upsertModelMappingRule } from './modelMappingState'
import SearchableModelInput from './SearchableModelInput.vue'

type MappingKind = 'exact' | 'prefix' | 'wildcard'

interface Props {
  modelValue?: Record<string, string>
  reasoningEfforts?: Record<string, string>
  missPolicy?: ModelMappingMissPolicy
  passthroughPatterns?: string[]
  platform?: CLIPlatform
}

interface Emits {
  (e: 'update:modelValue', value: Record<string, string>): void
  (e: 'update:reasoningEfforts', value: Record<string, string>): void
  (e: 'update:missPolicy', value: ModelMappingMissPolicy): void
  (e: 'update:passthroughPatterns', value: string[]): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const { t } = useI18n()

const mappingList = computed(() => {
  if (!props.modelValue) return []
  return Object.entries(props.modelValue).map(([key, value]) => ({
    key,
    value,
    reasoningEffort: `${props.reasoningEfforts?.[key] ?? ''}`.trim(),
  }))
})

const newKey = ref('')
const newValue = ref('')
const newReasoningEffort = ref('')
const newPassthroughPattern = ref('')
const editingOriginalKey = ref('')
const inputError = ref('')
const builtinModelRows = ref<ModelPricingRow[]>([])
const builtinModelLoading = ref(false)
const builtinModelLoadFailed = ref(false)
const valueInputRef = ref<InstanceType<typeof BaseInput> | null>(null)
const helpIconRef = ref<HTMLButtonElement | null>(null)
const helpTooltipRef = ref<HTMLElement | null>(null)
const helpTooltipId = `model-mapping-help-tooltip-${Math.random().toString(36).slice(2, 9)}`
const helpTooltip = reactive({
  visible: false,
  positioned: false,
  left: 0,
  top: 0,
  placement: 'below' as 'above' | 'below',
})

const HELP_TOOLTIP_DEFAULT_WIDTH = 280
const HELP_TOOLTIP_DEFAULT_HEIGHT = 48
const HELP_TOOLTIP_GAP = 8
const HELP_TOOLTIP_VIEWPORT_MARGIN = 12
let isHelpTooltipListening = false

const isEditing = computed(() => editingOriginalKey.value !== '')
const reasoningEffortOptions = ['low', 'medium', 'high', 'xhigh', 'max']
const canSubmitMapping = computed(() => newKey.value.trim() !== '' && newValue.value.trim() !== '')
const selectedMissPolicy = computed<ModelMappingMissPolicy>({
  get: () => props.missPolicy === 'passthrough' ? 'passthrough' : 'block',
  set: (value) => emit('update:missPolicy', value === 'passthrough' ? 'passthrough' : 'block'),
})
const normalizedPassthroughPatterns = computed(() => Array.from(new Set(
  (props.passthroughPatterns || []).map((pattern) => pattern.trim()).filter(Boolean),
)))
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
  return t('components.provider.modelMapping.builtinCompactHint', {
    count: builtinModelOptions.value.length,
  })
})

function getMappingKind(model: string): MappingKind {
  const starIndex = model.indexOf('*')
  if (starIndex === -1) return 'exact'
  if (starIndex === model.length - 1 && model.lastIndexOf('*') === starIndex) return 'prefix'
  return 'wildcard'
}

function getMappingKindLabel(model: string): string {
  return t(`components.provider.modelMapping.types.${getMappingKind(model)}`)
}

function clamp(value: number, min: number, max: number): number {
  if (max <= min) return min
  return Math.min(Math.max(value, min), max)
}

function getViewportSize(): { width: number; height: number } {
  if (typeof window !== 'undefined') {
    return {
      width: window.innerWidth,
      height: window.innerHeight,
    }
  }
  return {
    width: document.documentElement.clientWidth,
    height: document.documentElement.clientHeight,
  }
}

function updateHelpTooltipPosition(): void {
  const anchor = helpIconRef.value
  if (!anchor || !anchor.isConnected) {
    hideHelpTooltip()
    return
  }

  const anchorRect = anchor.getBoundingClientRect()
  const tooltipRect = helpTooltipRef.value?.getBoundingClientRect()
  const { width: viewportWidth, height: viewportHeight } = getViewportSize()
  const tooltipWidth = tooltipRect?.width ?? HELP_TOOLTIP_DEFAULT_WIDTH
  const tooltipHeight = tooltipRect?.height ?? HELP_TOOLTIP_DEFAULT_HEIGHT
  const minLeft = HELP_TOOLTIP_VIEWPORT_MARGIN
  const maxLeft = viewportWidth - tooltipWidth - HELP_TOOLTIP_VIEWPORT_MARGIN
  const desiredLeft = anchorRect.left + anchorRect.width / 2 - tooltipWidth / 2

  helpTooltip.left = clamp(desiredLeft, minLeft, maxLeft)

  const belowTop = anchorRect.bottom + HELP_TOOLTIP_GAP
  const aboveTop = anchorRect.top - tooltipHeight - HELP_TOOLTIP_GAP
  const canShowBelow = belowTop + tooltipHeight <= viewportHeight - HELP_TOOLTIP_VIEWPORT_MARGIN
  const canShowAbove = aboveTop >= HELP_TOOLTIP_VIEWPORT_MARGIN
  const shouldShowBelow = canShowBelow || !canShowAbove
  const desiredTop = shouldShowBelow ? belowTop : aboveTop
  const maxTop = viewportHeight - tooltipHeight - HELP_TOOLTIP_VIEWPORT_MARGIN

  helpTooltip.placement = shouldShowBelow ? 'below' : 'above'
  helpTooltip.top = clamp(desiredTop, HELP_TOOLTIP_VIEWPORT_MARGIN, maxTop)
  helpTooltip.positioned = true
}

function addHelpTooltipListeners(): void {
  if (isHelpTooltipListening || typeof window === 'undefined') return
  window.addEventListener('scroll', updateHelpTooltipPosition, true)
  window.addEventListener('resize', updateHelpTooltipPosition)
  isHelpTooltipListening = true
}

function removeHelpTooltipListeners(): void {
  if (!isHelpTooltipListening || typeof window === 'undefined') return
  window.removeEventListener('scroll', updateHelpTooltipPosition, true)
  window.removeEventListener('resize', updateHelpTooltipPosition)
  isHelpTooltipListening = false
}

async function showHelpTooltip(): Promise<void> {
  helpTooltip.visible = true
  helpTooltip.positioned = false
  addHelpTooltipListeners()
  await nextTick()
  if (!helpTooltip.visible) return
  updateHelpTooltipPosition()
}

function hideHelpTooltip(): void {
  helpTooltip.visible = false
  helpTooltip.positioned = false
  removeHelpTooltipListeners()
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
  newReasoningEffort.value = ''
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

function startEditing(key: string, value: string, reasoningEffort: string): void {
  editingOriginalKey.value = key
  newKey.value = key
  newValue.value = value
  newReasoningEffort.value = reasoningEffort
  inputError.value = ''
}

function cancelEditing(): void {
  resetDraft()
}

function submitMapping(): void {
  const key = newKey.value.trim()
  const value = newValue.value.trim()
  const reasoningEffort = newReasoningEffort.value.trim()

  if (!key || !value) return

  if (hasConflictingKey(key)) {
    inputError.value = t('components.provider.modelMapping.duplicateError')
    return
  }

  const updated = upsertModelMappingRule(
    props.modelValue || {},
    props.reasoningEfforts || {},
    editingOriginalKey.value,
    key,
    value,
    reasoningEffort,
  )
  emit('update:modelValue', updated.modelMappings)
  emit('update:reasoningEfforts', updated.reasoningEfforts)
  resetDraft()
}

function removeMapping(key: string): void {
  const updated = removeModelMappingRule(props.modelValue || {}, props.reasoningEfforts || {}, key)
  emit('update:modelValue', updated.modelMappings)
  emit('update:reasoningEfforts', updated.reasoningEfforts)

  if (editingOriginalKey.value === key) {
    resetDraft()
    return
  }

  if (!hasConflictingKey(newKey.value.trim())) {
    inputError.value = ''
  }
}

function addPassthroughPattern(): void {
  const pattern = newPassthroughPattern.value.trim()
  if (!pattern || normalizedPassthroughPatterns.value.includes(pattern)) {
    newPassthroughPattern.value = ''
    return
  }
  emit('update:passthroughPatterns', [...normalizedPassthroughPatterns.value, pattern])
  newPassthroughPattern.value = ''
}

function removePassthroughPattern(pattern: string): void {
  emit('update:passthroughPatterns', normalizedPassthroughPatterns.value.filter((item) => item !== pattern))
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

watch([newKey, newValue, newReasoningEffort], () => {
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

onBeforeUnmount(() => {
  hideHelpTooltip()
})
</script>

<style scoped>
.model-mapping-editor {
  --mapping-panel: color-mix(in srgb, var(--mac-surface) 76%, transparent);
  --mapping-panel-strong: color-mix(in srgb, var(--mac-surface-strong) 88%, transparent);
  --mapping-row-hover: color-mix(in srgb, var(--mac-accent) 7%, var(--mac-surface));
  --mapping-code-bg: color-mix(in srgb, var(--mac-surface-strong) 76%, transparent);
  --mapping-muted: color-mix(in srgb, var(--mac-text-secondary) 85%, transparent);
  display: flex;
  flex-direction: column;
  gap: 14px;
  color: var(--mac-text);
}

html.dark .model-mapping-editor {
  --mapping-panel: rgba(18, 20, 27, 0.72);
  --mapping-panel-strong: rgba(24, 27, 36, 0.88);
  --mapping-row-hover: rgba(30, 41, 59, 0.44);
  --mapping-code-bg: rgba(7, 10, 16, 0.36);
  --mapping-muted: color-mix(in srgb, var(--mac-text-secondary) 72%, transparent);
}

.editor-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 14px;
  border-bottom: 1px solid color-mix(in srgb, var(--mac-border) 78%, transparent);
}

.editor-heading {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.editor-label {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-weight: 700;
  font-size: 1rem;
  line-height: 1.25;
  color: var(--mac-text);
}

.editor-description {
  margin: 0;
  font-size: 0.75rem;
  line-height: 1.5;
  color: var(--mapping-muted);
}

.mapping-count {
  flex-shrink: 0;
  font-size: 0.6875rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: var(--mapping-muted);
  text-transform: uppercase;
}

.help-icon {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  border: 0;
  background: transparent;
  color: color-mix(in srgb, var(--mac-text-secondary) 85%, transparent);
  cursor: help;
  border-radius: 999px;
  transition: background 0.16s ease, color 0.16s ease;
}

.help-icon:hover,
.help-icon:focus-visible {
  color: var(--mac-accent);
  background: color-mix(in srgb, var(--mac-accent) 10%, transparent);
  outline: none;
}

.model-mapping-help-tooltip {
  position: fixed;
  z-index: 4200;
  box-sizing: border-box;
  width: max-content;
  max-width: min(280px, calc(100vw - 24px));
  padding: 8px 10px;
  border: 1px solid color-mix(in srgb, var(--mac-border) 88%, transparent);
  border-radius: 9px;
  background: color-mix(in srgb, var(--mac-surface) 98%, transparent);
  color: var(--mac-text-secondary);
  box-shadow: 0 18px 36px rgba(15, 23, 42, 0.16);
  font-size: 0.6875rem;
  font-weight: 500;
  line-height: 1.45;
  text-align: left;
  white-space: normal;
  pointer-events: none;
  opacity: 0;
  visibility: hidden;
  transform: translateY(-3px);
  transition: opacity 0.16s ease, transform 0.16s ease, visibility 0.16s ease;
}

.model-mapping-help-tooltip.is-above {
  transform: translateY(3px);
}

.model-mapping-help-tooltip.is-positioned {
  opacity: 1;
  visibility: visible;
  transform: translateY(0);
}

html.dark .model-mapping-help-tooltip {
  background: rgba(18, 20, 27, 0.98);
  box-shadow: 0 18px 36px rgba(0, 0, 0, 0.42);
}

.mapping-list {
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--mac-border) 86%, transparent);
  border-radius: 12px;
  background: var(--mapping-panel);
}

.mapping-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 46px;
  padding: 9px 14px;
  transition: background 0.16s ease, box-shadow 0.16s ease;
}

.mapping-row + .mapping-row {
  border-top: 1px solid color-mix(in srgb, var(--mac-border) 68%, transparent);
}

.mapping-row:hover {
  background: var(--mapping-row-hover);
}

.mapping-row.is-editing {
  background: color-mix(in srgb, var(--mac-accent) 9%, transparent);
  box-shadow: inset 3px 0 0 var(--mac-accent);
}

.mapping-content {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
  flex: 1;
}

.mapping-type {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 50px;
  min-width: 50px;
  min-height: 22px;
  border-radius: 6px;
  border: 1px solid currentColor;
  font-size: 0.6875rem;
  font-weight: 700;
  line-height: 1;
  white-space: nowrap;
}

.mapping-type--exact {
  color: #059669;
  background: rgba(16, 185, 129, 0.1);
  border-color: rgba(16, 185, 129, 0.28);
}

.mapping-type--prefix {
  color: #0284c7;
  background: rgba(14, 165, 233, 0.1);
  border-color: rgba(14, 165, 233, 0.28);
}

.mapping-type--wildcard {
  color: #7c3aed;
  background: rgba(124, 58, 237, 0.1);
  border-color: rgba(124, 58, 237, 0.28);
}

html.dark .mapping-type--exact {
  color: #34d399;
  background: rgba(16, 185, 129, 0.08);
  border-color: rgba(52, 211, 153, 0.32);
}

html.dark .mapping-type--prefix {
  color: #60a5fa;
  background: rgba(59, 130, 246, 0.08);
  border-color: rgba(96, 165, 250, 0.32);
}

html.dark .mapping-type--wildcard {
  color: #c084fc;
  background: rgba(168, 85, 247, 0.08);
  border-color: rgba(192, 132, 252, 0.32);
}

.mapping-models {
  display: grid;
  grid-template-columns: minmax(128px, 0.52fr) auto minmax(120px, 1fr);
  align-items: center;
  gap: 12px;
  min-width: 0;
  flex: 1;
}

.mapping-key,
.mapping-value {
  overflow: hidden;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 0.75rem;
  font-weight: 650;
  line-height: 1.45;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mapping-key {
  color: var(--mac-text);
}

.mapping-value {
  color: var(--mac-accent);
}

.mapping-effort {
  flex-shrink: 0;
  min-width: 68px;
  max-width: min(180px, 35%);
  overflow: hidden;
  padding: 4px 8px;
  border: 1px solid color-mix(in srgb, var(--mac-accent) 34%, var(--mac-border));
  border-radius: 999px;
  background: color-mix(in srgb, var(--mac-accent) 10%, transparent);
  color: var(--mac-accent);
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 0.6875rem;
  font-weight: 700;
  line-height: 1.2;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mapping-effort.is-passthrough {
  border-color: color-mix(in srgb, var(--mac-border) 86%, transparent);
  background: color-mix(in srgb, var(--mac-surface-strong) 72%, transparent);
  color: var(--mapping-muted);
}

.mapping-arrow,
.input-arrow {
  flex-shrink: 0;
  color: color-mix(in srgb, var(--mac-text-secondary) 62%, transparent);
}

.mapping-row-actions {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  opacity: 0;
  transform: translateX(4px);
  transition: opacity 0.16s ease, transform 0.16s ease;
}

.mapping-row:hover .mapping-row-actions,
.mapping-row:focus-within .mapping-row-actions,
.mapping-row.is-editing .mapping-row-actions {
  opacity: 1;
  transform: translateX(0);
}

.mapping-action,
.mapping-remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  padding: 0;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: color-mix(in srgb, var(--mac-text-secondary) 82%, transparent);
  cursor: pointer;
  transition: background 0.16s ease, color 0.16s ease;
}

.mapping-action:hover,
.mapping-action:focus-visible {
  color: var(--mac-accent);
  background: color-mix(in srgb, var(--mac-accent) 12%, transparent);
  outline: none;
}

.mapping-remove:hover,
.mapping-remove:focus-visible {
  color: #ef4444;
  background: rgba(239, 68, 68, 0.12);
  outline: none;
}

.mapping-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 54px;
  padding: 14px;
  color: var(--mapping-muted);
  font-size: 0.8125rem;
}

.mapping-input-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid color-mix(in srgb, var(--mac-border) 86%, transparent);
  border-radius: 12px;
  background: var(--mapping-panel-strong);
  transition: border-color 0.16s ease, box-shadow 0.16s ease;
}

.mapping-input-panel.is-editing {
  border-color: color-mix(in srgb, var(--mac-accent) 52%, var(--mac-border));
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--mac-accent) 16%, transparent);
}

.mapping-input-row {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) auto minmax(0, 0.95fr) minmax(128px, 0.55fr) auto;
  gap: 12px;
  align-items: center;
}

.mapping-field {
  min-width: 0;
}

.mapping-field--key :deep(input),
.mapping-field :deep(input) {
  width: 100%;
  min-width: 0;
  height: 36px;
  border-radius: 9px;
  border-color: color-mix(in srgb, var(--mac-border) 92%, transparent);
  background: var(--mapping-code-bg);
  padding-top: 8px;
  padding-bottom: 8px;
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 0.75rem;
}

.mapping-field--key :deep(.searchable-model-input__button) {
  width: 24px;
  height: 24px;
  right: 7px;
}

.mapping-field--key :deep(.searchable-model-input__field) {
  padding-right: 34px;
}

.mapping-field--effort :deep(.searchable-model-input__field) {
  padding-right: 34px;
}

.mapping-field--effort :deep(.searchable-model-input__button) {
  width: 24px;
  height: 24px;
  right: 7px;
}

.mapping-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.mapping-submit,
.mapping-cancel {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 36px;
  padding: 0 14px;
  border-radius: 9px;
  border: 1px solid transparent;
  font-size: 0.75rem;
  font-weight: 700;
  white-space: nowrap;
  cursor: pointer;
  transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease, opacity 0.16s ease;
}

.mapping-submit {
  min-width: 88px;
  color: #fff;
  background: var(--mac-accent);
  box-shadow: 0 8px 18px color-mix(in srgb, var(--mac-accent) 20%, transparent);
}

.mapping-submit:hover,
.mapping-submit:focus-visible {
  background: color-mix(in srgb, var(--mac-accent) 88%, #ffffff 12%);
  outline: none;
}

.mapping-submit:disabled {
  cursor: not-allowed;
  opacity: 0.48;
  color: color-mix(in srgb, var(--mac-text-secondary) 75%, transparent);
  background: color-mix(in srgb, var(--mac-surface-strong) 78%, var(--mac-text-secondary) 8%);
  box-shadow: none;
}

.mapping-cancel {
  color: var(--mac-text-secondary);
  background: transparent;
  border-color: var(--mac-border);
}

.mapping-cancel:hover,
.mapping-cancel:focus-visible {
  color: var(--mac-text);
  background: color-mix(in srgb, var(--mac-surface) 72%, transparent);
  outline: none;
}

.mapping-input-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  min-height: 24px;
  padding: 0 2px;
}

.passthrough-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid color-mix(in srgb, var(--mac-border) 82%, transparent);
  border-radius: 10px;
  background: var(--mapping-panel);
}

.passthrough-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: var(--mac-text-secondary);
  font-size: 0.75rem;
  font-weight: 650;
}

.passthrough-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
}

.passthrough-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 26px;
  padding: 3px 6px 3px 9px;
  border: 1px solid color-mix(in srgb, var(--mac-accent) 24%, var(--mac-border));
  border-radius: 7px;
  background: color-mix(in srgb, var(--mac-accent) 7%, transparent);
  color: var(--mac-text);
}

.passthrough-tag code {
  font-size: 0.6875rem;
}

.passthrough-tag button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--mac-text-secondary);
  cursor: pointer;
}

.passthrough-tag button:hover,
.passthrough-tag button:focus-visible {
  color: #ef4444;
  background: color-mix(in srgb, #ef4444 10%, transparent);
  outline: none;
}

.passthrough-input-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
}

.passthrough-add {
  min-width: 72px;
  border: 1px solid var(--mac-border);
  border-radius: 8px;
  background: var(--mapping-panel-strong);
  color: var(--mac-text);
  cursor: pointer;
  font-size: 0.75rem;
  font-weight: 650;
}

.passthrough-add:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.mapping-hints {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px 14px;
  min-width: 0;
}

.mapping-hints > span,
.miss-policy-inline {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--mapping-muted);
  font-size: 0.6875rem;
  line-height: 1.4;
  white-space: nowrap;
}

.miss-policy-inline {
  gap: 7px;
}

.miss-policy-select {
  min-width: 104px;
  height: 26px;
  appearance: none;
  border: 1px solid color-mix(in srgb, var(--mac-border) 88%, transparent);
  border-radius: 7px;
  background: var(--mapping-code-bg);
  color: var(--mac-text);
  padding: 0 24px 0 9px;
  font-size: 0.6875rem;
  font-weight: 650;
  cursor: pointer;
}

.miss-policy-select:focus {
  outline: none;
  border-color: var(--mac-accent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--mac-accent) 18%, transparent);
}

.mapping-status {
  min-width: 160px;
  max-width: 48%;
  text-align: right;
}

.mapping-picker-hint,
.mapping-editing-hint,
.mapping-input-error {
  display: inline-block;
  overflow: hidden;
  max-width: 100%;
  font-size: 0.6875rem;
  line-height: 1.45;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mapping-picker-hint {
  color: var(--mapping-muted);
}

.mapping-editing-hint {
  color: var(--mac-accent);
}

.mapping-input-error {
  color: #ef4444;
}

@media (max-width: 860px) {
  .editor-header,
  .mapping-input-meta {
    align-items: flex-start;
    flex-direction: column;
  }

  .mapping-count,
  .mapping-status {
    max-width: 100%;
    text-align: left;
  }

  .mapping-row {
    align-items: flex-start;
  }

  .mapping-content {
    align-items: flex-start;
    flex-direction: column;
    gap: 8px;
  }

  .mapping-effort {
    max-width: 100%;
  }

  .mapping-models {
    width: 100%;
    grid-template-columns: minmax(0, 1fr);
    gap: 6px;
  }

  .mapping-arrow {
    display: none;
  }

  .mapping-row-actions {
    opacity: 1;
    transform: none;
  }
}

@media (max-width: 720px) {
  .mapping-input-row {
    grid-template-columns: 1fr;
  }

  .input-arrow {
    display: none;
  }

  .mapping-actions,
  .mapping-submit,
  .mapping-cancel,
  .miss-policy-inline,
  .miss-policy-select {
    width: 100%;
  }

  .passthrough-input-row {
    grid-template-columns: 1fr;
  }

  .mapping-submit,
  .mapping-cancel {
    flex: 1;
  }
}
</style>
