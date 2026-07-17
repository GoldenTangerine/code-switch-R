<template>
  <div ref="rootRef" class="logs-controls">
    <div class="logs-controls-orb logs-controls-orb--primary" aria-hidden="true"></div>
    <div class="logs-controls-orb logs-controls-orb--secondary" aria-hidden="true"></div>

    <form class="logs-filter-form" @submit.prevent="emit('submit')">
      <div class="logs-filter-primary">
        <div class="logs-filter-field logs-filter-field--platform">
          <span class="logs-filter-label">
            <svg class="logs-filter-label-icon" viewBox="0 0 24 24" aria-hidden="true">
              <path
                d="M7.5 4.75h9a2.75 2.75 0 0 1 2.75 2.75v9a2.75 2.75 0 0 1-2.75 2.75h-9a2.75 2.75 0 0 1-2.75-2.75v-9A2.75 2.75 0 0 1 7.5 4.75Z"
                fill="none"
                stroke="currentColor"
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.6"
              />
              <path
                d="M9 9.25h.01M15 9.25h.01M9 14.75h6"
                fill="none"
                stroke="currentColor"
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.8"
              />
            </svg>
            <span>{{ t('components.logs.filters.platform') }}</span>
          </span>

          <div class="logs-select" :class="{ 'is-open': activeSelect === 'platform' }">
            <button
              type="button"
              class="logs-select-button"
              :class="{ 'is-open': activeSelect === 'platform' }"
              :aria-expanded="activeSelect === 'platform'"
              :aria-label="t('components.logs.filters.platform')"
              @click="toggleSelect('platform')"
            >
              <span class="logs-select-value">{{ platformLabel }}</span>
              <svg class="logs-select-chevron" viewBox="0 0 20 20" aria-hidden="true">
                <path
                  d="M6 8.5 10 12.5 14 8.5"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.6"
                />
              </svg>
            </button>

            <Transition name="logs-select-pop">
              <div v-if="activeSelect === 'platform'" class="logs-select-options" role="listbox">
                <button
                  v-for="option in platformOptions"
                  :key="option.value || option.label"
                  type="button"
                  class="logs-select-option"
                  :class="{ 'is-selected': platformValue === option.value }"
                  :aria-selected="platformValue === option.value"
                  @click="selectDropdownOption('platform', option.value)"
                >
                  <span>{{ option.label }}</span>
                  <span v-if="platformValue === option.value" class="logs-select-option-indicator"></span>
                </button>
              </div>
            </Transition>
          </div>
        </div>

        <div class="logs-filter-field logs-filter-field--provider">
          <span class="logs-filter-label">
            <svg class="logs-filter-label-icon" viewBox="0 0 24 24" aria-hidden="true">
              <path
                d="M12 4.5a7.5 7.5 0 1 0 0 15 7.5 7.5 0 0 0 0-15Z"
                fill="none"
                stroke="currentColor"
                stroke-width="1.6"
              />
              <path
                d="M4.9 9.25h14.2M4.9 14.75h14.2M12 4.75c2.15 2.28 3.2 4.69 3.2 7.25 0 2.56-1.05 4.97-3.2 7.25-2.15-2.28-3.2-4.69-3.2-7.25 0-2.56 1.05-4.97 3.2-7.25Z"
                fill="none"
                stroke="currentColor"
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.4"
              />
            </svg>
            <span>{{ t('components.logs.filters.provider') }}</span>
          </span>

          <div
            ref="providerSelectRef"
            class="logs-select"
            :class="{ 'is-open': activeSelect === 'provider' }"
          >
            <button
              type="button"
              class="logs-select-button"
              :class="{ 'is-open': activeSelect === 'provider' }"
              :aria-expanded="activeSelect === 'provider'"
              :aria-label="t('components.logs.filters.provider')"
              @click="toggleSelect('provider')"
            >
              <span class="logs-select-value">{{ providerLabel }}</span>
              <svg class="logs-select-chevron" viewBox="0 0 20 20" aria-hidden="true">
                <path
                  d="M6 8.5 10 12.5 14 8.5"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.6"
                />
              </svg>
            </button>

            <Transition name="logs-select-pop">
              <div
                ref="providerPopupRef"
                v-if="activeSelect === 'provider'"
                class="logs-select-options logs-select-options--provider"
                :style="providerPopupStyle"
              >
                <div class="logs-select-search" @click.stop>
                  <svg class="logs-select-search-icon" viewBox="0 0 24 24" aria-hidden="true">
                    <path
                      d="m17.5 17.5 2.75 2.75M19 11a8 8 0 1 1-16 0 8 8 0 0 1 16 0Z"
                      fill="none"
                      stroke="currentColor"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.8"
                    />
                  </svg>
                  <input
                    ref="providerSearchInputRef"
                    v-model="providerSearchQuery"
                    type="text"
                    class="logs-select-search-input"
                    :placeholder="t('components.logs.filters.providerPlaceholder')"
                    autocomplete="off"
                    autocorrect="off"
                    autocapitalize="off"
                    spellcheck="false"
                    @click.stop
                    @keydown.enter.prevent
                  />
                </div>

                <div class="logs-select-options-scroll logs-select-options-scroll--provider" role="listbox">
                  <button
                    type="button"
                    class="logs-select-option logs-select-option--pinned"
                    :class="{ 'is-selected': providerValue === providerAllOption.value }"
                    :aria-selected="providerValue === providerAllOption.value"
                    @click="selectDropdownOption('provider', providerAllOption.value)"
                  >
                    <span class="logs-select-option-label logs-select-option-label--multiline">
                      {{ providerAllOption.label }}
                    </span>
                    <span
                      v-if="providerValue === providerAllOption.value"
                      class="logs-select-option-indicator"
                    ></span>
                  </button>

                  <button
                    v-for="option in filteredProviderSearchOptions"
                    :key="option.value || option.label"
                    type="button"
                    class="logs-select-option"
                    :class="{ 'is-selected': providerValue === option.value }"
                    :aria-selected="providerValue === option.value"
                    @click="selectDropdownOption('provider', option.value)"
                  >
                    <span class="logs-select-option-label logs-select-option-label--multiline">
                      {{ option.label }}
                    </span>
                    <span v-if="providerValue === option.value" class="logs-select-option-indicator"></span>
                  </button>

                  <div
                    v-if="showProviderEmptyState"
                    class="logs-select-empty"
                  >
                    {{ t('components.logs.filters.providerNoResults') }}
                  </div>
                </div>
              </div>
            </Transition>
          </div>
        </div>

        <div class="logs-filter-field logs-filter-field--model">
          <span class="logs-filter-label">
            <svg class="logs-filter-label-icon" viewBox="0 0 24 24" aria-hidden="true">
              <path
                d="m12 4 7 4-7 4-7-4 7-4Zm-7 8 7 4 7-4M5 16l7 4 7-4"
                fill="none"
                stroke="currentColor"
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.6"
              />
            </svg>
            <span>{{ t('components.logs.filters.model') }}</span>
          </span>

          <div class="logs-select" :class="{ 'is-open': activeSelect === 'model' }">
            <button
              type="button"
              class="logs-select-button"
              :class="{ 'is-open': activeSelect === 'model' }"
              :aria-expanded="activeSelect === 'model'"
              :aria-label="t('components.logs.filters.model')"
              @click="toggleSelect('model')"
            >
              <span class="logs-select-value" :title="modelLabel">{{ modelLabel }}</span>
              <svg class="logs-select-chevron" viewBox="0 0 20 20" aria-hidden="true">
                <path
                  d="M6 8.5 10 12.5 14 8.5"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.6"
                />
              </svg>
            </button>

            <Transition name="logs-select-pop">
              <div v-if="activeSelect === 'model'" class="logs-select-options logs-select-options--model" role="listbox">
                <button
                  v-for="option in modelSelectOptions"
                  :key="option.value || option.label"
                  type="button"
                  class="logs-select-option"
                  :class="{ 'is-selected': modelValue === option.value }"
                  :aria-selected="modelValue === option.value"
                  :title="option.label"
                  @click="selectDropdownOption('model', option.value)"
                >
                  <span class="logs-select-option-label">{{ option.label }}</span>
                  <span v-if="modelValue === option.value" class="logs-select-option-indicator"></span>
                </button>
              </div>
            </Transition>
          </div>
        </div>

        <div class="logs-filter-field logs-filter-field--date-type">
          <span class="logs-filter-label">
            <svg class="logs-filter-label-icon" viewBox="0 0 24 24" aria-hidden="true">
              <path
                d="M7.25 5.25v2.5M16.75 5.25v2.5M5.5 9.5h13"
                fill="none"
                stroke="currentColor"
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.6"
              />
              <rect
                x="4.75"
                y="6.75"
                width="14.5"
                height="12.5"
                rx="2.75"
                fill="none"
                stroke="currentColor"
                stroke-width="1.6"
              />
            </svg>
            <span>{{ t('components.logs.filters.dateType') }}</span>
          </span>

          <div class="logs-select" :class="{ 'is-open': activeSelect === 'dateType' }">
            <button
              type="button"
              class="logs-select-button"
              :class="{ 'is-open': activeSelect === 'dateType' }"
              :aria-expanded="activeSelect === 'dateType'"
              :aria-label="t('components.logs.filters.dateType')"
              @click="toggleSelect('dateType')"
            >
              <span class="logs-select-value">{{ dateTypeLabel }}</span>
              <svg class="logs-select-chevron" viewBox="0 0 20 20" aria-hidden="true">
                <path
                  d="M6 8.5 10 12.5 14 8.5"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.6"
                />
              </svg>
            </button>

            <Transition name="logs-select-pop">
              <div v-if="activeSelect === 'dateType'" class="logs-select-options" role="listbox">
                <button
                  v-for="option in dateTypeOptions"
                  :key="option.value"
                  type="button"
                  class="logs-select-option"
                  :class="{ 'is-selected': dateTypeValue === option.value }"
                  :aria-selected="dateTypeValue === option.value"
                  @click="selectDropdownOption('dateType', option.value)"
                >
                  <span>{{ option.label }}</span>
                  <span v-if="dateTypeValue === option.value" class="logs-select-option-indicator"></span>
                </button>
              </div>
            </Transition>
          </div>
        </div>

        <div class="logs-filter-action">
          <BaseButton class="logs-filter-submit" size="sm" type="submit" :disabled="loading || !isFilterValid">
            <svg class="logs-filter-submit-icon" viewBox="0 0 24 24" aria-hidden="true">
              <path
                d="m17.5 17.5 2.75 2.75M19 11a8 8 0 1 1-16 0 8 8 0 0 1 16 0Z"
                fill="none"
                stroke="currentColor"
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.8"
              />
            </svg>
            <span>{{ t('components.logs.query') }}</span>
          </BaseButton>
        </div>
      </div>

      <Transition name="logs-filter-detail">
        <div v-if="showDateDetail" class="logs-filter-secondary" :class="{ 'is-range': filters.dateType === 'range' }">
          <div class="logs-filter-field logs-filter-field--date-detail">
            <span class="logs-filter-label">
              <svg v-if="filters.dateType === 'range'" class="logs-filter-label-icon" viewBox="0 0 24 24" aria-hidden="true">
                <path
                  d="M12 6.25v5.25l3.25 1.9M12 20a8 8 0 1 0 0-16 8 8 0 0 0 0 16Z"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.6"
                />
              </svg>
              <svg v-else class="logs-filter-label-icon" viewBox="0 0 24 24" aria-hidden="true">
                <path
                  d="M7.25 5.25v2.5M16.75 5.25v2.5M5.5 9.5h13"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.6"
                />
                <rect
                  x="4.75"
                  y="6.75"
                  width="14.5"
                  height="12.5"
                  rx="2.75"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.6"
                />
              </svg>
              <span>{{ dateDetailLabel }}</span>
            </span>

            <div class="logs-sub-filter-shell" :class="{ 'has-clear': showDateClearButton }">
              <VueDatePicker
                v-if="filters.dateType === 'year'"
                :model-value="yearPickerValue"
                class="logs-date-picker"
                :dark="isDarkTheme"
                :locale="dateFnsLocale"
                year-picker
                auto-apply
                :text-input="false"
                :year-range="yearPickerRange"
                :input-attrs="datePickerInputAttrs"
                :ui="datePickerUi"
                :formats="{ input: 'yyyy' }"
                :placeholder="dateDetailPlaceholder"
                @update:model-value="emit('update:year-picker-value', $event as number | null)"
              />

              <VueDatePicker
                v-else-if="filters.dateType === 'month'"
                :model-value="monthPickerValue"
                class="logs-date-picker"
                :dark="isDarkTheme"
                :locale="dateFnsLocale"
                month-picker
                auto-apply
                :text-input="false"
                :year-range="yearPickerRange"
                :input-attrs="datePickerInputAttrs"
                :ui="datePickerUi"
                :formats="{ input: 'yyyy-MM' }"
                :placeholder="dateDetailPlaceholder"
                @update:model-value="emit('update:month-picker-value', $event as MonthModel | null)"
              />

              <VueDatePicker
                v-else-if="filters.dateType === 'day'"
                :model-value="dayPickerValue"
                class="logs-date-picker"
                :dark="isDarkTheme"
                :locale="dateFnsLocale"
                auto-apply
                :text-input="false"
                :input-attrs="datePickerInputAttrs"
                :ui="datePickerUi"
                :formats="{ input: 'yyyy-MM-dd' }"
                :placeholder="dateDetailPlaceholder"
                @update:model-value="emit('update:day-picker-value', $event as Date | null)"
              />

              <VueDatePicker
                v-else-if="filters.dateType === 'range'"
                :model-value="rangePickerValue"
                class="logs-date-picker"
                :dark="isDarkTheme"
                :locale="dateFnsLocale"
                :range="rangePickerConfig"
                :multi-calendars="2"
                auto-apply
                :text-input="false"
                :input-attrs="datePickerInputAttrs"
                :ui="datePickerUi"
                :formats="{ input: formatRangeInput }"
                :placeholder="dateDetailPlaceholder"
                @update:model-value="emit('update:range-picker-value', $event as Date[] | null)"
              />

              <button
                v-if="showDateClearButton"
                type="button"
                class="logs-sub-filter-clear"
                :aria-label="t('components.logs.filters.footer.utilityClear')"
                @click="clearCurrentDateDetail"
              >
                <svg viewBox="0 0 20 20" aria-hidden="true">
                  <path
                    d="M6 6l8 8M14 6l-8 8"
                    fill="none"
                    stroke="currentColor"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="1.7"
                  />
                </svg>
              </button>
            </div>
          </div>
        </div>
      </Transition>

      <div class="logs-filter-footer">
        <div class="logs-filter-footer-meta">
          <span class="logs-filter-footer-chip" :class="`is-${footerStatusTone}`">
            <span class="logs-filter-footer-dot"></span>
            <span>{{ footerStatusText }}</span>
          </span>
          <span class="logs-filter-footer-scope">{{ footerScopeText }}</span>
        </div>

        <div class="logs-filter-footer-tools">
          <button
            type="button"
            class="logs-filter-tool-btn"
            :disabled="!hasUtilityAction"
            :aria-label="utilityActionLabel"
            :title="utilityActionLabel"
            @click="handleUtilityAction"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path
                d="M5.25 6.75h13.5M8.25 12h7.5M10.25 17.25h3.5"
                fill="none"
                stroke="currentColor"
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.8"
              />
            </svg>
          </button>
        </div>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { VueDatePicker, type MonthModel } from '@vuepic/vue-datepicker'
import { enUS, zhCN } from 'date-fns/locale'
import { useI18n } from 'vue-i18n'
import BaseButton from '../../common/BaseButton.vue'
import type { LogDateFilterType, LogProviderOption, LogsFiltersState } from '../types'
import { scoreStringOption } from '../../../utils/fuzzyOptionSearch'
import { getLogsYearPickerRange } from '../utils'

type FilterOption<T extends string = string> = {
  value: T
  label: string
}

type ProviderFilterOption = FilterOption<string> & {
  providerName?: string
}

type SelectName = 'platform' | 'provider' | 'model' | 'dateType'

const props = defineProps<{
  filters: LogsFiltersState
  providerOptions: LogProviderOption[]
  modelOptions: string[]
  loading: boolean
  isFilterValid: boolean
  hasPendingChanges?: boolean
  yearPickerValue: number | null
  monthPickerValue: MonthModel | null
  dayPickerValue: Date | null
  rangePickerValue: Date[] | null
  isDarkTheme: boolean
  summaryScopeHint?: string
}>()

const emit = defineEmits<{
  (event: 'submit'): void
  (event: 'update:platform', value: LogsFiltersState['platform']): void
  (event: 'update:provider', value: string): void
  (event: 'update:model', value: string): void
  (event: 'update:date-type', value: LogDateFilterType): void
  (event: 'update:year-picker-value', value: number | null): void
  (event: 'update:month-picker-value', value: MonthModel | null): void
  (event: 'update:day-picker-value', value: Date | null): void
  (event: 'update:range-picker-value', value: Date[] | null): void
}>()

const { t, locale } = useI18n()
const rootRef = ref<HTMLElement | null>(null)
const activeSelect = ref<SelectName | null>(null)
const providerSelectRef = ref<HTMLElement | null>(null)
const providerPopupRef = ref<HTMLElement | null>(null)
const providerSearchInputRef = ref<HTMLInputElement | null>(null)
const providerSearchQuery = ref('')
const providerPopupStyle = ref<Record<string, string>>({})

const dateFnsLocale = computed(() => (locale.value === 'zh' ? zhCN : enUS))

const datePickerUi = {
  input: 'logs-filter-control logs-date-picker-input',
  menu: 'mac-panel logs-filter-date-picker-menu',
}

const datePickerInputAttrs = {
  hideInputIcon: true,
  clearable: false,
  alwaysClearable: false,
} as const

const yearPickerRange = computed<[number, number]>(() => getLogsYearPickerRange())
const rangePickerConfig = { partialRange: false } as const

const platformOptions = computed<Array<FilterOption<LogsFiltersState['platform']>>>(() => [
  { value: '', label: t('components.logs.filters.allPlatforms') },
  { value: 'claude', label: 'Claude' },
  { value: 'codex', label: 'Codex' },
  { value: 'gemini', label: 'Gemini' },
])

const providerAllOption = computed<ProviderFilterOption>(() => ({
  value: '',
  label: t('components.logs.filters.allProviders'),
}))

const providerSelectOptions = computed<ProviderFilterOption[]>(() => [
  providerAllOption.value,
  ...props.providerOptions.map((provider) => ({
    value: provider.value,
    label: provider.label,
    providerName: provider.providerName,
  })),
])

const modelSelectOptions = computed<Array<FilterOption<string>>>(() => [
  { value: '', label: t('components.logs.filters.allModels') },
  ...props.modelOptions.map((model) => ({ value: model, label: model })),
])

const filteredProviderSearchOptions = computed<ProviderFilterOption[]>(() => {
  const query = providerSearchQuery.value.trim()
  const options = providerSelectOptions.value.filter((option) => option.value)
  if (!query) return options

  return options
    .map((option, index) => {
      const score = Math.max(
        scoreStringOption(option.label, query),
        scoreStringOption(option.providerName || '', query),
        scoreStringOption(option.value, query),
        scoreStringOption(`${option.label} ${option.providerName || ''} ${option.value}`, query),
      )

      return {
        option,
        index,
        score,
      }
    })
    .filter((entry) => Number.isFinite(entry.score))
    .sort((left, right) => {
      if (right.score !== left.score) return right.score - left.score
      return left.index - right.index
    })
    .map((entry) => entry.option)
})

const showProviderEmptyState = computed(() =>
  Boolean(providerSearchQuery.value.trim()) && filteredProviderSearchOptions.value.length === 0,
)

const dateTypeOptions = computed<Array<FilterOption<LogDateFilterType>>>(() => [
  { value: 'all', label: t('components.logs.filters.dateTypeAll') },
  { value: 'today', label: t('components.logs.filters.dateTypeToday') },
  { value: 'year', label: t('components.logs.filters.dateTypeYear') },
  { value: 'month', label: t('components.logs.filters.dateTypeMonth') },
  { value: 'day', label: t('components.logs.filters.dateTypeDay') },
  { value: 'range', label: t('components.logs.filters.dateTypeRange') },
])

const platformValue = computed<LogsFiltersState['platform']>({
  get: () => props.filters.platform,
  set: (value) => emit('update:platform', value),
})

const providerValue = computed<string>({
  get: () => props.filters.provider,
  set: (value) => emit('update:provider', value),
})

const modelValue = computed<string>({
  get: () => props.filters.model,
  set: (value) => emit('update:model', value),
})

const dateTypeValue = computed<LogDateFilterType>({
  get: () => props.filters.dateType,
  set: (value) => emit('update:date-type', value),
})

const resolveOptionLabel = <T extends string>(
  options: Array<FilterOption<T>>,
  value: T,
  fallback = '',
) => options.find((option) => option.value === value)?.label ?? (fallback || options[0]?.label || '')

const platformLabel = computed(() =>
  resolveOptionLabel(platformOptions.value, platformValue.value, platformValue.value),
)

const providerLabel = computed(() =>
  resolveOptionLabel(providerSelectOptions.value, providerValue.value, providerValue.value),
)

const modelLabel = computed(() =>
  resolveOptionLabel(modelSelectOptions.value, modelValue.value, modelValue.value),
)

const dateTypeLabel = computed(() =>
  resolveOptionLabel(dateTypeOptions.value, dateTypeValue.value, dateTypeValue.value),
)

const showDateDetail = computed(() => !['all', 'today'].includes(props.filters.dateType))

const dateDetailLabel = computed(() => {
  switch (props.filters.dateType) {
    case 'year':
      return t('components.logs.filters.year')
    case 'month':
      return t('components.logs.filters.month')
    case 'day':
      return t('components.logs.filters.day')
    case 'range':
      return t('components.logs.filters.range')
    default:
      return ''
  }
})

const dateDetailPlaceholder = computed(() => {
  switch (props.filters.dateType) {
    case 'year':
      return 'YYYY'
    case 'month':
      return 'YYYY-MM'
    case 'day':
      return 'YYYY-MM-DD'
    case 'range':
      return locale.value === 'zh' ? '开始日期 至 结束日期' : 'Start date to end date'
    default:
      return ''
  }
})

const showDateClearButton = computed(() => {
  switch (props.filters.dateType) {
    case 'year':
      return props.yearPickerValue != null
    case 'month':
      return props.monthPickerValue != null
    case 'day':
      return props.dayPickerValue != null
    case 'range':
      return Array.isArray(props.rangePickerValue) && props.rangePickerValue.length > 0
    default:
      return false
  }
})

const footerStatusTone = computed<'ready' | 'loading' | 'invalid' | 'pending'>(() => {
  if (props.loading) return 'loading'
  if (!props.isFilterValid) return 'invalid'
  if (props.hasPendingChanges) return 'pending'
  return 'ready'
})

const footerStatusText = computed(() => {
  switch (footerStatusTone.value) {
    case 'loading':
      return t('components.logs.filters.footer.statusLoading')
    case 'pending':
      return t('components.logs.filters.footer.statusPending')
    case 'invalid':
      return t('components.logs.filters.footer.statusInvalid')
    default:
      return t('components.logs.filters.footer.statusReady')
  }
})

const footerScopeText = computed(() =>
  t('components.logs.filters.footer.scope', {
    scope: props.summaryScopeHint || dateTypeLabel.value,
  }),
)

const hasUtilityAction = computed(() => Boolean(activeSelect.value) || showDateClearButton.value)

const utilityActionLabel = computed(() => {
  if (activeSelect.value) return t('components.logs.filters.footer.utilityClose')
  if (showDateClearButton.value) return t('components.logs.filters.footer.utilityClear')
  return t('components.logs.filters.footer.utilityIdle')
})

const toggleSelect = (name: SelectName) => {
  activeSelect.value = activeSelect.value === name ? null : name
}

const closeActiveSelect = () => {
  activeSelect.value = null
}

const resetProviderPopupLayout = () => {
  providerPopupStyle.value = {}
}

const syncProviderPopupLayout = () => {
  if (activeSelect.value !== 'provider') return

  const selectEl = providerSelectRef.value
  const popupEl = providerPopupRef.value
  if (!selectEl || !popupEl) return

  const viewportPadding = window.innerWidth <= 640 ? 18 : 28
  const selectRect = selectEl.getBoundingClientRect()
  const maxWidth = Math.max(selectRect.width, window.innerWidth - viewportPadding * 2)

  providerPopupStyle.value = {
    '--logs-provider-popup-max-width': `${Math.floor(maxWidth)}px`,
    '--logs-provider-popup-shift': '0px',
  }

  const popupRect = popupEl.getBoundingClientRect()
  let shift = 0

  const overflowRight = popupRect.right - (window.innerWidth - viewportPadding)
  if (overflowRight > 0) {
    shift -= overflowRight
  }

  const overflowLeft = viewportPadding - (popupRect.left + shift)
  if (overflowLeft > 0) {
    shift += overflowLeft
  }

  providerPopupStyle.value = {
    '--logs-provider-popup-max-width': `${Math.floor(maxWidth)}px`,
    '--logs-provider-popup-shift': `${Math.round(shift)}px`,
  }
}

const selectDropdownOption = (name: SelectName, value: string) => {
  if (name === 'platform') {
    emit('update:platform', value as LogsFiltersState['platform'])
  } else if (name === 'provider') {
    providerSearchQuery.value = ''
    emit('update:provider', value)
  } else if (name === 'model') {
    emit('update:model', value)
  } else {
    emit('update:date-type', value as LogDateFilterType)
  }
  closeActiveSelect()
}

const clearCurrentDateDetail = () => {
  switch (props.filters.dateType) {
    case 'year':
      emit('update:year-picker-value', null)
      break
    case 'month':
      emit('update:month-picker-value', null)
      break
    case 'day':
      emit('update:day-picker-value', null)
      break
    case 'range':
      emit('update:range-picker-value', null)
      break
    default:
      break
  }
}

const handleUtilityAction = () => {
  if (activeSelect.value) {
    closeActiveSelect()
    return
  }
  if (showDateClearButton.value) {
    clearCurrentDateDetail()
  }
}

const pad2 = (num: number) => num.toString().padStart(2, '0')

const formatDateYmd = (date: Date) =>
  `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`

const formatRangeInput = (dates: Array<Date | null>) => {
  const [start, end] = dates ?? []
  if (!start) return ''
  if (!end) return formatDateYmd(start)
  return `${formatDateYmd(start)} ~ ${formatDateYmd(end)}`
}

const handleDocumentPointerDown = (event: Event) => {
  const target = event.target as Node | null
  if (!target) return
  if (rootRef.value?.contains(target)) return
  closeActiveSelect()
}

const handleWindowKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') {
    closeActiveSelect()
  }
}

const handleWindowResize = () => {
  syncProviderPopupLayout()
}

watch(
  () => activeSelect.value,
  async (value, previousValue) => {
    if (previousValue === 'provider' && value !== 'provider') {
      providerSearchQuery.value = ''
      resetProviderPopupLayout()
    }

    if (value === 'provider') {
      providerSearchQuery.value = ''
      await nextTick()
      providerSearchInputRef.value?.focus()
      providerSearchInputRef.value?.select()
      syncProviderPopupLayout()
    }
  },
)

watch(
  () => filteredProviderSearchOptions.value.map((option) => option.value).join('|'),
  async () => {
    if (activeSelect.value !== 'provider') return
    await nextTick()
    syncProviderPopupLayout()
  },
)

onMounted(() => {
  document.addEventListener('pointerdown', handleDocumentPointerDown)
  window.addEventListener('keydown', handleWindowKeydown)
  window.addEventListener('resize', handleWindowResize)
})

onUnmounted(() => {
  document.removeEventListener('pointerdown', handleDocumentPointerDown)
  window.removeEventListener('keydown', handleWindowKeydown)
  window.removeEventListener('resize', handleWindowResize)
})
</script>

<style>
.logs-controls {
  --logs-filter-action-width: 144px;
  position: relative;
  z-index: 20;
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: clamp(18px, 2vw, 26px);
  border-radius: 24px;
  border: 1px solid color-mix(in srgb, var(--mac-border) 82%, transparent);
  background:
    linear-gradient(
      180deg,
      color-mix(in srgb, var(--mac-surface) 97%, rgba(89, 106, 160, 0.08) 3%) 0%,
      color-mix(in srgb, var(--mac-surface) 92%, rgba(89, 106, 160, 0.04) 8%) 100%
    );
  box-shadow:
    0 22px 48px rgba(15, 23, 42, 0.12),
    inset 0 1px 0 rgba(255, 255, 255, 0.08);
  overflow: visible;
  isolation: isolate;
}

html.dark .logs-controls {
  border-color: rgba(255, 255, 255, 0.08);
  background: linear-gradient(180deg, rgba(20, 22, 33, 0.96) 0%, rgba(13, 15, 24, 0.97) 100%);
  box-shadow:
    0 28px 64px rgba(0, 0, 0, 0.48),
    inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.logs-controls::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background-image:
    linear-gradient(rgba(79, 102, 168, 0.12) 1px, transparent 1px),
    linear-gradient(90deg, rgba(79, 102, 168, 0.12) 1px, transparent 1px);
  background-size: 36px 36px;
  opacity: 0.08;
  pointer-events: none;
}

html.dark .logs-controls::before {
  opacity: 0.12;
}

.logs-controls-orb {
  position: absolute;
  border-radius: 999px;
  pointer-events: none;
  filter: blur(32px);
}

.logs-controls-orb--primary {
  inset: -16% auto auto -6%;
  width: 320px;
  height: 220px;
  background: radial-gradient(circle, rgba(74, 109, 255, 0.18) 0%, rgba(74, 109, 255, 0) 72%);
}

.logs-controls-orb--secondary {
  inset: auto -8% -20% auto;
  width: 300px;
  height: 240px;
  background: radial-gradient(circle, rgba(168, 85, 247, 0.18) 0%, rgba(168, 85, 247, 0) 74%);
}

.logs-filter-form {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.logs-filter-primary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr)) var(--logs-filter-action-width);
  gap: 18px;
  align-items: end;
}

.logs-filter-secondary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr)) var(--logs-filter-action-width);
  gap: 18px;
  align-items: start;
}

.logs-filter-secondary.is-range {
  grid-template-columns: repeat(3, minmax(0, 1fr)) var(--logs-filter-action-width);
}

.logs-filter-field--date-detail {
  grid-column: 1 / 2;
}

.logs-filter-field {
  --logs-filter-accent: 95 150 255;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.logs-filter-field--provider {
  --logs-filter-accent: 168 85 247;
}

.logs-filter-field--model {
  --logs-filter-accent: 59 130 246;
}

.logs-filter-field--date-type,
.logs-filter-field--date-detail {
  --logs-filter-accent: 16 185 129;
}

.logs-filter-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 20px;
  font-size: 0.82rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: color-mix(in srgb, var(--mac-text-secondary) 92%, transparent);
}

html.dark .logs-filter-label {
  color: rgba(226, 232, 240, 0.88);
}

.logs-filter-label-icon {
  width: 14px;
  height: 14px;
  flex: 0 0 auto;
  color: rgb(var(--logs-filter-accent));
}

.logs-select {
  position: relative;
}

.logs-select.is-open {
  z-index: 24;
}

.logs-select-button,
.logs-filter-control {
  width: 100%;
  min-height: 44px;
  padding: 0 14px;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--mac-border) 90%, transparent);
  background: color-mix(in srgb, var(--mac-surface) 92%, transparent);
  color: var(--mac-text);
  font-size: 0.94rem;
  box-sizing: border-box;
  transition:
    border-color 0.24s ease,
    background 0.24s ease,
    box-shadow 0.24s ease,
    transform 0.24s ease;
}

html.dark .logs-select-button,
html.dark .logs-filter-control {
  border-color: rgba(255, 255, 255, 0.08);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.045) 0%, rgba(255, 255, 255, 0.02) 100%);
  color: rgba(241, 245, 249, 0.96);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.03);
}

.logs-select-button {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  justify-content: space-between;
  margin: 0;
  cursor: pointer;
  text-align: left;
}

.logs-select-button:hover,
.logs-filter-control:hover,
.logs-sub-filter-shell:hover .logs-filter-control {
  border-color: rgba(var(--logs-filter-accent), 0.42);
  background: rgba(var(--logs-filter-accent), 0.05);
}

.logs-select-button:focus-visible,
.logs-select-button.is-open,
.logs-filter-control:focus,
.logs-sub-filter-shell:focus-within .logs-filter-control {
  outline: none;
  border-color: rgba(var(--logs-filter-accent), 0.88);
  background: rgba(var(--logs-filter-accent), 0.07);
  box-shadow:
    0 0 0 4px rgba(var(--logs-filter-accent), 0.14),
    0 14px 28px rgba(15, 23, 42, 0.12);
}

html.dark .logs-select-button:focus-visible,
html.dark .logs-select-button.is-open,
html.dark .logs-filter-control:focus,
html.dark .logs-sub-filter-shell:focus-within .logs-filter-control {
  box-shadow:
    0 0 0 4px rgba(var(--logs-filter-accent), 0.16),
    0 18px 36px rgba(0, 0, 0, 0.34);
}

.logs-select-value {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.logs-select-chevron {
  width: 18px;
  height: 18px;
  margin-left: auto;
  color: color-mix(in srgb, var(--mac-text-secondary) 90%, transparent);
  transition: transform 0.24s ease, color 0.24s ease;
}

html.dark .logs-select-chevron {
  color: rgba(148, 163, 184, 0.9);
}

.logs-select-button.is-open .logs-select-chevron {
  transform: rotate(180deg);
  color: rgb(var(--logs-filter-accent));
}

.logs-select-options {
  position: absolute;
  inset: calc(100% + 10px) 0 auto;
  z-index: 30;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 0;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--mac-border) 84%, transparent);
  background: color-mix(in srgb, var(--mac-surface) 96%, transparent);
  box-shadow:
    0 20px 38px rgba(15, 23, 42, 0.16),
    inset 0 1px 0 rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(18px);
  overflow: hidden;
}

.logs-select-options--provider {
  inset: calc(100% + 10px) auto auto 0;
  min-width: 100%;
  width: max-content;
  max-width: var(--logs-provider-popup-max-width, min(720px, calc(100vw - 56px)));
  padding: 10px;
  gap: 8px;
  overflow-x: hidden;
  transform: translateX(var(--logs-provider-popup-shift, 0px));
}

html.dark .logs-select-options {
  border-color: rgba(255, 255, 255, 0.08);
  background: rgba(28, 31, 46, 0.94);
  box-shadow:
    0 24px 48px rgba(0, 0, 0, 0.48),
    inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.logs-select-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  min-width: 0;
  margin: 0;
  gap: 12px;
  padding: 11px 14px;
  border: none;
  border-radius: 0;
  background: transparent;
  cursor: pointer;
  font-size: 0.92rem;
  color: color-mix(in srgb, var(--mac-text) 94%, transparent);
  transition: background 0.18s ease, color 0.18s ease, transform 0.18s ease;
}

.logs-select-option--pinned {
  margin-bottom: 2px;
}

html.dark .logs-select-option {
  color: rgba(226, 232, 240, 0.94);
}

.logs-select-option-label {
  flex: 1 1 auto;
  min-width: 0;
  text-align: left;
}

.logs-select-option-label--multiline {
  white-space: normal;
  overflow-wrap: anywhere;
  word-break: break-word;
  line-height: 1.45;
}

.logs-select-option:hover,
.logs-select-option:focus-visible {
  outline: none;
  background: rgba(var(--logs-filter-accent), 0.14);
}

.logs-select-option.is-selected {
  background: rgba(var(--logs-filter-accent), 0.12);
  color: rgb(var(--logs-filter-accent));
  font-weight: 600;
}

.logs-select-option-indicator {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: rgb(var(--logs-filter-accent));
  box-shadow: 0 0 0 6px rgba(var(--logs-filter-accent), 0.12);
  flex: 0 0 auto;
}

.logs-select-search {
  position: relative;
  display: flex;
  align-items: center;
  min-width: 0;
}

.logs-select-search-icon {
  position: absolute;
  left: 12px;
  width: 16px;
  height: 16px;
  color: rgba(148, 163, 184, 0.82);
  pointer-events: none;
}

.logs-select-search-input {
  width: 100%;
  min-width: 0;
  min-height: 40px;
  padding: 0 12px 0 38px;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(255, 255, 255, 0.04);
  color: var(--mac-text);
  font-size: 0.9rem;
  box-sizing: border-box;
  transition: border-color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease;
}

html.dark .logs-select-search-input {
  color: rgba(241, 245, 249, 0.96);
}

.logs-select-search-input::placeholder {
  color: rgba(148, 163, 184, 0.78);
}

.logs-select-search-input:focus {
  outline: none;
  border-color: rgba(168, 85, 247, 0.42);
  background: rgba(168, 85, 247, 0.08);
  box-shadow: 0 0 0 3px rgba(168, 85, 247, 0.12);
}

.logs-select-options-scroll {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: min(280px, 48vh);
  overflow-y: auto;
  overflow-x: hidden;
}

.logs-select-options-scroll--provider {
  min-width: 100%;
}

.logs-select-empty {
  padding: 14px 12px;
  border-radius: 12px;
  text-align: center;
  font-size: 0.86rem;
  color: rgba(148, 163, 184, 0.86);
  background: rgba(255, 255, 255, 0.03);
}

.logs-filter-action {
  display: flex;
  align-items: flex-end;
}

.logs-filter-submit.btn {
  min-width: 144px;
  min-height: 44px;
  padding: 0 28px;
  border-radius: 14px;
  border: 1px solid rgba(109, 132, 255, 0.24);
  background: linear-gradient(135deg, #4d72ff 0%, #4563f0 45%, #3f5fe9 100%);
  color: #ffffff;
  gap: 10px;
  box-shadow:
    0 18px 34px rgba(69, 99, 240, 0.24),
    0 0 30px rgba(77, 114, 255, 0.16);
  transition:
    transform 0.22s ease,
    box-shadow 0.22s ease,
    filter 0.22s ease,
    opacity 0.22s ease;
}

.logs-filter-submit.btn:hover:not(:disabled),
.logs-filter-submit.btn:focus-visible {
  transform: translateY(-1px);
  filter: brightness(1.04);
  box-shadow:
    0 20px 38px rgba(69, 99, 240, 0.28),
    0 0 34px rgba(77, 114, 255, 0.22);
}

.logs-filter-submit.btn:disabled {
  opacity: 0.58;
  cursor: not-allowed;
  box-shadow: none;
  transform: none;
}

.logs-filter-submit-icon {
  width: 18px;
  height: 18px;
  transition: transform 0.22s ease;
}

.logs-filter-submit.btn:hover:not(:disabled) .logs-filter-submit-icon {
  transform: scale(1.08);
}

.logs-date-picker {
  width: 100%;
  min-width: 0;
}

.logs-date-picker .dp__input_wrap {
  width: 100%;
}

.logs-sub-filter-shell {
  position: relative;
  width: 100%;
}

.logs-sub-filter-shell.has-clear .logs-filter-control,
.logs-sub-filter-shell.has-clear .logs-date-picker-input {
  padding-inline-end: 46px;
}

.logs-filter-control {
  padding-inline: 14px;
}

.logs-filter-control::placeholder {
  color: color-mix(in srgb, var(--mac-text-secondary) 92%, transparent);
}

html.dark .logs-filter-control::placeholder {
  color: rgba(148, 163, 184, 0.78);
}

.logs-date-picker-input {
  padding-inline-end: 14px;
}

.logs-sub-filter-clear {
  position: absolute;
  top: 50%;
  right: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.05);
  color: rgba(148, 163, 184, 0.9);
  cursor: pointer;
  transform: translateY(-50%);
  transition: background 0.18s ease, color 0.18s ease, border-color 0.18s ease;
}

.logs-sub-filter-clear:hover,
.logs-sub-filter-clear:focus-visible {
  outline: none;
  border-color: rgba(16, 185, 129, 0.24);
  background: rgba(16, 185, 129, 0.12);
  color: rgb(52, 211, 153);
}

.logs-sub-filter-clear svg {
  width: 14px;
  height: 14px;
}

.logs-filter-date-picker-menu {
  z-index: 28;
  border-radius: 18px;
  border: 1px solid color-mix(in srgb, var(--mac-border) 84%, transparent);
  box-shadow:
    0 22px 46px rgba(15, 23, 42, 0.18),
    inset 0 1px 0 rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

html.dark .logs-filter-date-picker-menu {
  border-color: rgba(255, 255, 255, 0.08);
  background: rgba(20, 23, 34, 0.96);
  box-shadow:
    0 26px 54px rgba(0, 0, 0, 0.54),
    inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.logs-filter-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
}

.logs-filter-footer-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  min-width: 0;
}

.logs-filter-footer-chip,
.logs-filter-footer-scope {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 24px;
  padding: 0 10px;
  border-radius: 999px;
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.logs-filter-footer-chip {
  border: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(255, 255, 255, 0.03);
  color: rgba(226, 232, 240, 0.86);
}

.logs-filter-footer-chip.is-ready {
  border-color: rgba(16, 185, 129, 0.18);
  background: rgba(16, 185, 129, 0.08);
}

.logs-filter-footer-chip.is-loading {
  border-color: rgba(59, 130, 246, 0.18);
  background: rgba(59, 130, 246, 0.08);
}

.logs-filter-footer-chip.is-invalid {
  border-color: rgba(245, 158, 11, 0.18);
  background: rgba(245, 158, 11, 0.08);
}

.logs-filter-footer-chip.is-pending {
  border-color: rgba(168, 85, 247, 0.22);
  background: rgba(168, 85, 247, 0.1);
}

.logs-filter-footer-dot {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  flex: 0 0 auto;
}

.logs-filter-footer-chip.is-ready .logs-filter-footer-dot {
  background: rgb(16, 185, 129);
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.65);
}

.logs-filter-footer-chip.is-loading .logs-filter-footer-dot {
  background: rgb(59, 130, 246);
  box-shadow: 0 0 10px rgba(59, 130, 246, 0.65);
  animation: logs-footer-pulse 1.4s ease-in-out infinite;
}

.logs-filter-footer-chip.is-invalid .logs-filter-footer-dot {
  background: rgb(245, 158, 11);
  box-shadow: 0 0 10px rgba(245, 158, 11, 0.48);
}

.logs-filter-footer-chip.is-pending .logs-filter-footer-dot {
  background: rgb(168, 85, 247);
  box-shadow: 0 0 10px rgba(168, 85, 247, 0.5);
}

.logs-filter-footer-scope {
  max-width: min(100%, 460px);
  border: 1px solid rgba(255, 255, 255, 0.04);
  background: rgba(255, 255, 255, 0.02);
  color: rgba(148, 163, 184, 0.82);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.logs-filter-footer-tools {
  display: flex;
  align-items: center;
  gap: 8px;
}

.logs-filter-tool-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.03);
  color: rgba(148, 163, 184, 0.9);
  cursor: pointer;
  transition: background 0.18s ease, border-color 0.18s ease, color 0.18s ease, transform 0.18s ease;
}

.logs-filter-tool-btn:hover:not(:disabled),
.logs-filter-tool-btn:focus-visible:not(:disabled) {
  outline: none;
  transform: translateY(-1px);
  border-color: rgba(77, 114, 255, 0.24);
  background: rgba(77, 114, 255, 0.1);
  color: rgba(191, 219, 254, 0.96);
}

.logs-filter-tool-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.logs-filter-tool-btn svg {
  width: 14px;
  height: 14px;
}

@keyframes logs-footer-pulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 1;
  }
  50% {
    transform: scale(1.2);
    opacity: 0.72;
  }
}

.logs-select-pop-enter-active,
.logs-select-pop-leave-active,
.logs-filter-detail-enter-active,
.logs-filter-detail-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.logs-select-pop-enter-from,
.logs-select-pop-leave-to,
.logs-filter-detail-enter-from,
.logs-filter-detail-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

.logs-select-options--provider.logs-select-pop-enter-from,
.logs-select-options--provider.logs-select-pop-leave-to {
  transform: translateX(var(--logs-provider-popup-shift, 0px)) translateY(-6px);
}

.logs-select-options--model {
  max-height: 280px;
  overflow-y: auto;
}

@media (max-width: 1080px) {
  .logs-filter-primary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .logs-filter-secondary,
  .logs-filter-secondary.is-range {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .logs-filter-action {
    grid-column: 1 / -1;
  }

  .logs-filter-submit.btn {
    width: 100%;
  }
}

@media (max-width: 640px) {
  .logs-controls {
    padding: 18px;
    border-radius: 20px;
  }

  .logs-filter-primary,
  .logs-filter-secondary,
  .logs-filter-secondary.is-range {
    grid-template-columns: 1fr;
  }

  .logs-select-options {
    inset-block-start: calc(100% + 8px);
  }

  .logs-select-options--provider {
    max-width: var(--logs-provider-popup-max-width, calc(100vw - 36px));
  }

  .logs-filter-submit.btn {
    width: 100%;
    justify-content: center;
  }

  .logs-filter-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .logs-filter-footer-meta {
    flex-direction: column;
    align-items: flex-start;
  }

  .logs-filter-footer-scope {
    max-width: 100%;
  }

  .logs-filter-footer-tools {
    justify-content: flex-end;
  }
}
</style>
