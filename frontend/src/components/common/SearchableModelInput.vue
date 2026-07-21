<template>
  <div class="searchable-model-input">
    <div ref="controlRef" class="searchable-model-input__control">
      <input
        ref="inputRef"
        v-bind="forwardedInputAttrs"
        :id="inputId || undefined"
        :value="inputValue"
        :placeholder="placeholder"
        :disabled="disabled"
        :aria-controls="optionsId"
        :aria-expanded="isOpen"
        :aria-activedescendant="activeOptionId"
        class="mac-input searchable-model-input__field"
        type="text"
        role="combobox"
        aria-autocomplete="list"
        aria-haspopup="listbox"
        autocomplete="off"
        autocorrect="off"
        autocapitalize="off"
        spellcheck="false"
        writingsuggestions="false"
        data-gramm="false"
        data-gramm_editor="false"
        data-enable-grammarly="false"
        data-1p-ignore
        data-lpignore="true"
        data-form-type="other"
        @focus="handleInputFocus"
        @click="updateDropdownPosition"
        @beforeinput="handleBeforeInput"
        @input="handleInputChange"
        @keydown="handleInputKeydown"
        @blur="handleInputBlur"
      />
      <button
        type="button"
        class="searchable-model-input__button"
        tabindex="-1"
        :disabled="disabled"
        :aria-label="controlAriaLabel"
        :aria-controls="optionsId"
        :aria-expanded="isOpen"
        aria-haspopup="listbox"
        @mousedown.prevent
        @click="toggleDropdown"
      >
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
      </button>
    </div>

    <Teleport to="body">
      <div
        v-if="isOpen"
        :id="optionsId"
        ref="optionsRef"
        class="searchable-model-input__options"
        :style="dropdownStyle"
        :data-placement="dropdownPosition.placement"
        role="listbox"
        :aria-label="controlAriaLabel"
        @scroll.passive="handleOptionsScroll"
      >
        <div v-if="filteredOptions.length === 0" class="searchable-model-input__empty">
          {{ emptyText }}
        </div>

        <div v-else class="searchable-model-input__virtual-spacer" :style="virtualSpacerStyle">
          <div
            v-for="item in visibleOptions"
            :id="getOptionId(item.index)"
            :key="item.option"
            :class="[
              'searchable-model-input__option',
              {
                active: item.index === activeOptionIndex,
                selected: item.option === modelValue,
              },
            ]"
            :style="{ transform: `translateY(${item.index * MODEL_OPTION_HEIGHT}px)` }"
            role="option"
            :aria-selected="item.option === modelValue"
            :aria-posinset="item.index + 1"
            :aria-setsize="filteredOptions.length"
            @mousemove="activeOptionIndex = item.index"
            @mousedown="handleOptionMousedown($event, item.option)"
          >
            <span class="searchable-model-input__option-label" :title="item.option">
              {{ item.option }}
            </span>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, reactive, ref, useAttrs, useId, watch, type CSSProperties } from 'vue'
import {
  filterAndSortStringOptionIndex,
  getCachedStringOptionSearchIndex,
} from '../../utils/fuzzyOptionSearch'
import {
  allowSearchableInputPointerSelection,
  consumeSearchableInputSelection,
  createSearchableInputSelectionState,
  handleSearchableInputBeforeInput,
  handleSearchableInputEnter,
  handleSearchableInputSelectionKeydown,
  resetSearchableInputSelectionState,
} from './searchableInputKeyboard'
import {
  calculateModelDropdownHeight,
  calculateModelDropdownLayout,
  calculateNextModelOptionIndex,
  calculateVirtualOptionRange,
  MODEL_DROPDOWN_CHROME_HEIGHT,
  MODEL_DROPDOWN_VERTICAL_PADDING,
  MODEL_OPTION_HEIGHT,
} from './searchableModelDropdown'

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
const optionsId = `searchable-model-options-${useId()}`
const searchQuery = ref('')
const inputValue = ref(props.modelValue ?? '')
const isOpen = ref(false)
const activeOptionIndex = ref(-1)
const inputRef = ref<HTMLInputElement | null>(null)
const optionsRef = ref<HTMLElement | null>(null)
const controlRef = ref<HTMLElement | null>(null)
const selectionState = reactive(createSearchableInputSelectionState())
const optionsScrollTop = ref(0)
const optionsViewportHeight = ref(0)
const dropdownPosition = reactive({
  top: 0,
  left: 0,
  width: 0,
  height: 0,
  placement: 'below' as 'above' | 'below',
  ready: false,
})
let isDropdownLifecycleListening = false

const forwardedInputAttrs = computed(() => {
  const {
    class: _class,
    style: _style,
    ...rest
  } = attrs
  return rest
})

const controlAriaLabel = computed(() => {
  const ariaLabel = attrs['aria-label']
  return typeof ariaLabel === 'string' && ariaLabel.trim() ? ariaLabel : props.placeholder
})
const optionSearchIndex = computed(() => getCachedStringOptionSearchIndex(props.options || []))
const filteredOptions = computed(() => (
  filterAndSortStringOptionIndex(optionSearchIndex.value, searchQuery.value)
))
const desiredDropdownHeight = computed(() => (
  calculateModelDropdownHeight(filteredOptions.value.length, props.maxHeight)
))
const virtualRange = computed(() => calculateVirtualOptionRange(
  filteredOptions.value.length,
  optionsScrollTop.value,
  optionsViewportHeight.value,
))
const visibleOptions = computed(() => (
  filteredOptions.value
    .slice(virtualRange.value.start, virtualRange.value.end)
    .map((option, offset) => ({
      option,
      index: virtualRange.value.start + offset,
    }))
))
const activeOptionId = computed(() => (
  isOpen.value && activeOptionIndex.value >= 0
    ? getOptionId(activeOptionIndex.value)
    : undefined
))
const dropdownStyle = computed<CSSProperties>(() => ({
  top: `${dropdownPosition.top}px`,
  left: `${dropdownPosition.left}px`,
  width: `${dropdownPosition.width}px`,
  height: `${dropdownPosition.height}px`,
  visibility: dropdownPosition.ready ? 'visible' : 'hidden',
}))
const virtualSpacerStyle = computed<CSSProperties>(() => ({
  height: `${filteredOptions.value.length * MODEL_OPTION_HEIGHT}px`,
}))

const getOptionId = (index: number) => `${optionsId}-option-${index}`

const resetSearchQuery = () => {
  searchQuery.value = ''
}

const updateDropdownPosition = () => {
  const control = controlRef.value
  if (!control || typeof window === 'undefined') return

  const rect = control.getBoundingClientRect()
  const layout = calculateModelDropdownLayout(
    {
      top: rect.top,
      bottom: rect.bottom,
      left: rect.left,
      width: rect.width,
    },
    {
      width: window.innerWidth,
      height: window.innerHeight,
    },
    desiredDropdownHeight.value,
  )

  dropdownPosition.top = layout.top
  dropdownPosition.left = layout.left
  dropdownPosition.width = layout.width
  dropdownPosition.height = layout.height
  dropdownPosition.placement = layout.placement
  dropdownPosition.ready = true
  optionsViewportHeight.value = Math.max(0, layout.height - MODEL_DROPDOWN_CHROME_HEIGHT)
}

const syncOptionsViewport = () => {
  const options = optionsRef.value
  if (!options) return
  optionsViewportHeight.value = Math.max(0, options.clientHeight - MODEL_DROPDOWN_VERTICAL_PADDING)
}

const handleExternalScroll = (event: Event) => {
  const target = event.target
  if (target instanceof Node && optionsRef.value?.contains(target)) return
  closeDropdown(true)
}

const handleExternalPointerDown = (event: PointerEvent) => {
  const target = event.target
  if (!(target instanceof Node)) return
  if (controlRef.value?.contains(target) || optionsRef.value?.contains(target)) return
  closeDropdown()
}

const addDropdownLifecycleListeners = () => {
  if (isDropdownLifecycleListening || typeof window === 'undefined') return
  window.addEventListener('scroll', handleExternalScroll, true)
  window.addEventListener('resize', handleWindowResize)
  window.addEventListener('pointerdown', handleExternalPointerDown, true)
  isDropdownLifecycleListening = true
}

const removeDropdownLifecycleListeners = () => {
  if (!isDropdownLifecycleListening || typeof window === 'undefined') return
  window.removeEventListener('scroll', handleExternalScroll, true)
  window.removeEventListener('resize', handleWindowResize)
  window.removeEventListener('pointerdown', handleExternalPointerDown, true)
  isDropdownLifecycleListening = false
}

const closeDropdown = (blurInput = false, restoreSelectOnlyValue = true) => {
  isOpen.value = false
  dropdownPosition.ready = false
  activeOptionIndex.value = -1
  optionsScrollTop.value = 0
  resetSearchableInputSelectionState(selectionState)
  resetSearchQuery()
  removeDropdownLifecycleListeners()

  if (props.selectOnly && restoreSelectOnlyValue) {
    inputValue.value = props.modelValue ?? ''
  }
  if (blurInput) inputRef.value?.blur()
}

const handleWindowResize = () => {
  closeDropdown(true)
}

const openDropdown = () => {
  if (props.disabled) return
  isOpen.value = true
  updateDropdownPosition()
  addDropdownLifecycleListeners()
  nextTick(syncOptionsViewport)
}

const toggleDropdown = () => {
  if (isOpen.value) {
    closeDropdown()
    return
  }

  resetSearchableInputSelectionState(selectionState)
  resetSearchQuery()
  const selectedIndex = filteredOptions.value.indexOf(props.modelValue ?? '')
  activeOptionIndex.value = selectedIndex
  inputRef.value?.focus({ preventScroll: true })
  openDropdown()
  if (selectedIndex >= 0) nextTick(scrollActiveOptionIntoView)
}

const handleBeforeInput = (event: InputEvent) => {
  handleSearchableInputBeforeInput(event)
}

const handleInputChange = (event: Event) => {
  const nextValue = (event.target as HTMLInputElement).value
  inputValue.value = nextValue
  searchQuery.value = nextValue
  activeOptionIndex.value = -1
  optionsScrollTop.value = 0
  resetSearchableInputSelectionState(selectionState)
  openDropdown()
  nextTick(() => {
    if (optionsRef.value) optionsRef.value.scrollTop = 0
    syncOptionsViewport()
  })

  if (!props.selectOnly) {
    emit('update:modelValue', nextValue)
  } else if (!nextValue.trim()) {
    emit('update:modelValue', '')
  }
}

const handleInputKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Enter' && (event.isComposing || event.keyCode === 229)) {
    resetSearchableInputSelectionState(selectionState)
    return
  }

  const shouldSubmitCustomValue = handleSearchableInputSelectionKeydown(selectionState, event)
  if (event.key === 'Enter') {
    if (shouldSubmitCustomValue) {
      handleSearchableInputEnter(event, () => emit('customEnter'))
      return
    }

    event.preventDefault()
    if (consumeSearchableInputSelection(selectionState) && activeOptionIndex.value >= 0) {
      selectOption(filteredOptions.value[activeOptionIndex.value])
    }
    return
  }

  if (event.key === 'Escape') {
    if (!isOpen.value) return
    event.preventDefault()
    closeDropdown()
    return
  }

  if (event.key === 'Tab') {
    closeDropdown()
    return
  }

  if (event.key === 'Home' && event.shiftKey) return
  if (event.key === 'End' && event.shiftKey) return

  if (
    event.key !== 'ArrowDown'
    && event.key !== 'ArrowUp'
    && event.key !== 'Home'
    && event.key !== 'End'
    && event.key !== 'PageDown'
    && event.key !== 'PageUp'
  ) {
    return
  }

  event.preventDefault()
  if (!isOpen.value) openDropdown()
  activeOptionIndex.value = calculateNextModelOptionIndex(
    activeOptionIndex.value,
    filteredOptions.value.length,
    event.key,
  )

  nextTick(scrollActiveOptionIntoView)
}

const selectOption = (option: string | undefined) => {
  if (typeof option !== 'string') return
  inputValue.value = option
  emit('update:modelValue', option)
  emit('select', option)
  closeDropdown(false, false)
  nextTick(() => inputRef.value?.focus({ preventScroll: true }))
}

const handleOptionMousedown = (event: MouseEvent, option: string) => {
  if (!allowSearchableInputPointerSelection(selectionState, event)) return
  event.preventDefault()
  if (!consumeSearchableInputSelection(selectionState)) return
  selectOption(option)
}

const handleOptionsScroll = (event: Event) => {
  const options = event.currentTarget as HTMLElement
  optionsScrollTop.value = options.scrollTop
  optionsViewportHeight.value = Math.max(0, options.clientHeight - MODEL_DROPDOWN_VERTICAL_PADDING)
}

const scrollActiveOptionIntoView = () => {
  const options = optionsRef.value
  const index = activeOptionIndex.value
  if (!options || index < 0) return

  const viewportHeight = Math.max(0, options.clientHeight - MODEL_DROPDOWN_VERTICAL_PADDING)
  const optionTop = index * MODEL_OPTION_HEIGHT
  const optionBottom = optionTop + MODEL_OPTION_HEIGHT
  let nextScrollTop = options.scrollTop

  if (optionTop < options.scrollTop) {
    nextScrollTop = optionTop
  } else if (optionBottom > options.scrollTop + viewportHeight) {
    nextScrollTop = optionBottom - viewportHeight
  }

  if (nextScrollTop !== options.scrollTop) options.scrollTop = nextScrollTop
  optionsScrollTop.value = nextScrollTop
}

const handleInputFocus = () => {
  updateDropdownPosition()
}

const handleInputBlur = () => {
  nextTick(() => {
    if (document.activeElement === inputRef.value) return
    closeDropdown()
  })
}

const focus = () => inputRef.value?.focus()

watch(() => props.modelValue, (value) => {
  const nextValue = value ?? ''
  if (isOpen.value && nextValue === inputValue.value) return

  inputValue.value = nextValue
  if (!isOpen.value) return

  searchQuery.value = nextValue
  activeOptionIndex.value = -1
  optionsScrollTop.value = 0
  nextTick(() => {
    if (optionsRef.value) optionsRef.value.scrollTop = 0
  })
})

watch(filteredOptions, (options) => {
  if (!isOpen.value) return
  if (activeOptionIndex.value >= options.length) activeOptionIndex.value = -1
  nextTick(updateDropdownPosition)
})

onBeforeUnmount(() => {
  removeDropdownLifecycleListeners()
})

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
  position: fixed;
  z-index: 4300;
  box-sizing: border-box;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 6px;
  margin: 0;
  list-style: none;
  background: var(--mac-surface);
  border: 1px solid var(--mac-border);
  border-radius: 14px;
  box-shadow: 0 20px 45px rgba(0, 0, 0, 0.18);
}

.searchable-model-input__virtual-spacer {
  position: relative;
  width: 100%;
}

.searchable-model-input__option {
  position: absolute;
  top: 0;
  left: 0;
  display: flex;
  align-items: center;
  box-sizing: border-box;
  width: 100%;
  height: 40px;
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
  overflow: hidden;
  min-width: 0;
  color: var(--mac-text);
  font-size: 0.9rem;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
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
