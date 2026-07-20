<template>
  <Combobox v-model="selectedValue" :disabled="disabled">
    <div class="searchable-model-input">
      <div class="searchable-model-input__control">
        <ComboboxInput
          ref="inputRef"
          v-bind="forwardedInputAttrs"
          :id="inputId || undefined"
          class="mac-input searchable-model-input__field"
          :display-value="displayValue"
          :placeholder="placeholder"
          autocomplete="off"
          autocorrect="off"
          autocapitalize="off"
          spellcheck="false"
          @change="handleInputChange"
          @keydown="handleInputKeydown"
          @blur="handleInputBlur"
        />
        <ComboboxButton class="searchable-model-input__button">
          <svg viewBox="0 0 20 20" aria-hidden="true">
            <path
              d="M6 8l4 4 4-4"
              fill="none"
              stroke="currentColor"
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.5"
            />
          </svg>
        </ComboboxButton>
      </div>

      <ComboboxOptions class="searchable-model-input__options" :style="{ maxHeight }">
        <ComboboxOption
          v-for="option in filteredOptions"
          :key="option"
          :value="option"
          @mousedown="allowPointerSelection"
          v-slot="{ active, selected }"
        >
          <div :class="['searchable-model-input__option', { active, selected }]">
            <span class="searchable-model-input__option-label">{{ option }}</span>
          </div>
        </ComboboxOption>

        <div v-if="filteredOptions.length === 0" class="searchable-model-input__empty">
          {{ emptyText }}
        </div>
      </ComboboxOptions>
    </div>
  </Combobox>
</template>

<script setup lang="ts">
import { Combobox, ComboboxButton, ComboboxInput, ComboboxOption, ComboboxOptions } from '@headlessui/vue'
import { computed, nextTick, reactive, ref, useAttrs } from 'vue'
import { filterAndSortStringOptions } from '../../utils/fuzzyOptionSearch'
import {
  allowSearchableInputPointerSelection,
  consumeSearchableInputSelection,
  createSearchableInputSelectionState,
  handleSearchableInputEnter,
  handleSearchableInputSelectionKeydown,
  resetSearchableInputSelectionState,
} from './searchableInputKeyboard'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  modelValue?: string
  options?: string[]
  placeholder?: string
  emptyText?: string
  disabled?: boolean
  inputId?: string
  maxHeight?: string
  selectOnly?: boolean
}>(), {
  modelValue: '',
  options: () => [],
  placeholder: '',
  emptyText: '',
  disabled: false,
  inputId: '',
  maxHeight: '280px',
  selectOnly: false,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'select', value: string): void
  (e: 'customEnter'): void
}>()

const attrs = useAttrs()
const searchQuery = ref('')
const inputRef = ref<InstanceType<typeof ComboboxInput> | null>(null)
const selectionState = reactive(createSearchableInputSelectionState())

const forwardedInputAttrs = computed(() => {
  const {
    class: _class,
    style: _style,
    ...rest
  } = attrs
  return rest
})

const filteredOptions = computed(() => (
  filterAndSortStringOptions(props.options || [], searchQuery.value)
))

const selectedValue = computed({
  get: () => props.modelValue ?? '',
  set: (value: string) => {
    if (!consumeSearchableInputSelection(selectionState)) return

    const nextValue = String(value ?? '')
    emit('update:modelValue', nextValue)
    emit('select', nextValue)
    searchQuery.value = ''
  },
})

const displayValue = (value: unknown) => (typeof value === 'string' ? value : '')

const handleInputChange = (event: Event) => {
  const nextValue = (event.target as HTMLInputElement).value
  resetSearchableInputSelectionState(selectionState)
  searchQuery.value = nextValue
  if (!props.selectOnly) {
    emit('update:modelValue', nextValue)
  } else if (!nextValue.trim()) {
    emit('update:modelValue', '')
  }
}

const handleInputKeydown = (event: KeyboardEvent) => {
  if (!handleSearchableInputSelectionKeydown(selectionState, event)) {
    if (event.key === 'Enter') {
      nextTick(() => consumeSearchableInputSelection(selectionState))
    }
    return
  }

  handleSearchableInputEnter(event, () => emit('customEnter'))
}

const allowPointerSelection = (event: MouseEvent) => {
  if (event.button !== 0) return
  allowSearchableInputPointerSelection(selectionState)
  nextTick(() => consumeSearchableInputSelection(selectionState))
}

const clearSearchQuery = () => {
  nextTick(() => {
    nextTick(() => {
      searchQuery.value = ''
    })
  })
}

const handleInputBlur = () => {
  resetSearchableInputSelectionState(selectionState)
  clearSearchQuery()
}

const focus = () => {
  const inputElement = inputRef.value?.$el as HTMLInputElement | undefined
  inputElement?.focus()
}

defineExpose({ focus })
</script>

<style scoped>
.searchable-model-input {
  position: relative;
  width: 100%;
}

.searchable-model-input__control {
  position: relative;
}

.searchable-model-input__field {
  width: 100%;
  min-width: 0;
  padding-right: 40px;
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
}

.searchable-model-input__button {
  position: absolute;
  top: 50%;
  right: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--mac-text-secondary);
  cursor: pointer;
  transform: translateY(-50%);
  transition: background 0.15s ease, color 0.15s ease;
}

.searchable-model-input__button:hover {
  background: rgba(148, 163, 184, 0.12);
  color: var(--mac-text);
}

.searchable-model-input__button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.searchable-model-input__button svg {
  width: 16px;
  height: 16px;
}

.searchable-model-input__options {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  right: 0;
  z-index: 40;
  overflow-y: auto;
  padding: 6px;
  margin: 0;
  list-style: none;
  background: var(--mac-surface);
  border: 1px solid var(--mac-border);
  border-radius: 14px;
  box-shadow: 0 20px 45px rgba(0, 0, 0, 0.18);
}

.searchable-model-input__option {
  display: flex;
  align-items: center;
  min-height: 40px;
  padding: 10px 12px;
  border-radius: 10px;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.searchable-model-input__option.active {
  background: var(--mac-surface-strong);
}

.searchable-model-input__option.selected {
  background: color-mix(in srgb, var(--mac-accent) 12%, transparent);
}

.searchable-model-input__option-label {
  min-width: 0;
  color: var(--mac-text);
  font-size: 0.9rem;
  line-height: 1.4;
  word-break: break-word;
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
}

.searchable-model-input__empty {
  padding: 16px 12px;
  text-align: center;
  color: var(--mac-text-secondary);
  font-size: 0.85rem;
  line-height: 1.45;
}
</style>
