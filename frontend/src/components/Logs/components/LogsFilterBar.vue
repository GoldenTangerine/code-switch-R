<template>
  <div class="logs-controls">
    <form class="logs-filter-row" @submit.prevent="emit('submit')">
      <div class="filter-fields">
        <label class="filter-field">
          <span>{{ t('components.logs.filters.platform') }}</span>
          <select :value="filters.platform" class="mac-select" @change="updatePlatform">
            <option value="">{{ t('components.logs.filters.allPlatforms') }}</option>
            <option value="claude">Claude</option>
            <option value="codex">Codex</option>
            <option value="gemini">Gemini</option>
          </select>
        </label>
        <label class="filter-field">
          <span>{{ t('components.logs.filters.provider') }}</span>
          <select :value="filters.provider" class="mac-select" @change="updateProvider">
            <option value="">{{ t('components.logs.filters.allProviders') }}</option>
            <option v-for="provider in providerOptions" :key="provider.value" :value="provider.value">
              {{ provider.label }}
            </option>
          </select>
        </label>
        <label class="filter-field">
          <span>{{ t('components.logs.filters.dateType') }}</span>
          <select :value="filters.dateType" class="mac-select" @change="updateDateType">
            <option value="all">{{ t('components.logs.filters.dateTypeAll') }}</option>
            <option value="today">{{ t('components.logs.filters.dateTypeToday') }}</option>
            <option value="year">{{ t('components.logs.filters.dateTypeYear') }}</option>
            <option value="month">{{ t('components.logs.filters.dateTypeMonth') }}</option>
            <option value="day">{{ t('components.logs.filters.dateTypeDay') }}</option>
            <option value="range">{{ t('components.logs.filters.dateTypeRange') }}</option>
          </select>
        </label>
        <div class="filter-query-cell">
          <BaseButton size="sm" type="submit" :disabled="loading || !isFilterValid">
            {{ t('components.logs.query') }}
          </BaseButton>
        </div>

        <label v-if="filters.dateType === 'year'" class="filter-field">
          <span>{{ t('components.logs.filters.year') }}</span>
          <VueDatePicker
            :model-value="yearPickerValue"
            class="logs-date-picker"
            :dark="isDarkTheme"
            :locale="dateFnsLocale"
            year-picker
            auto-apply
            :text-input="false"
            :year-range="yearPickerRange"
            :input-attrs="{ hideInputIcon: true }"
            :ui="datePickerUi"
            :formats="{ input: 'yyyy' }"
            placeholder="YYYY"
            @update:model-value="emit('update:year-picker-value', $event as number | null)"
          />
        </label>
        <label v-else-if="filters.dateType === 'month'" class="filter-field">
          <span>{{ t('components.logs.filters.month') }}</span>
          <VueDatePicker
            :model-value="monthPickerValue"
            class="logs-date-picker"
            :dark="isDarkTheme"
            :locale="dateFnsLocale"
            month-picker
            auto-apply
            :text-input="false"
            :year-range="yearPickerRange"
            :input-attrs="{ hideInputIcon: true }"
            :ui="datePickerUi"
            :formats="{ input: 'yyyy-MM' }"
            placeholder="YYYY-MM"
            @update:model-value="emit('update:month-picker-value', $event as MonthModel | null)"
          />
        </label>
        <label v-else-if="filters.dateType === 'day'" class="filter-field">
          <span>{{ t('components.logs.filters.day') }}</span>
          <VueDatePicker
            :model-value="dayPickerValue"
            class="logs-date-picker"
            :dark="isDarkTheme"
            :locale="dateFnsLocale"
            auto-apply
            :text-input="false"
            :input-attrs="{ hideInputIcon: true }"
            :ui="datePickerUi"
            :formats="{ input: 'yyyy-MM-dd' }"
            placeholder="YYYY-MM-DD"
            @update:model-value="emit('update:day-picker-value', $event as Date | null)"
          />
        </label>
        <label v-else-if="filters.dateType === 'range'" class="filter-field">
          <span>{{ t('components.logs.filters.range') }}</span>
          <VueDatePicker
            :model-value="rangePickerValue"
            class="logs-date-picker"
            :dark="isDarkTheme"
            :locale="dateFnsLocale"
            :range="rangePickerConfig"
            :multi-calendars="2"
            auto-apply
            :text-input="false"
            :input-attrs="{ hideInputIcon: true }"
            :ui="datePickerUi"
            :formats="{ input: formatRangeInput }"
            :placeholder="t('components.logs.filters.range')"
            @update:model-value="emit('update:range-picker-value', $event as Date[] | null)"
          />
        </label>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { VueDatePicker, type MonthModel } from '@vuepic/vue-datepicker'
import { enUS, zhCN } from 'date-fns/locale'
import BaseButton from '../../common/BaseButton.vue'
import type { LogDateFilterType, LogProviderOption, LogsFiltersState } from '../types'
import { getLogsYearPickerRange } from '../utils'

defineProps<{
  filters: LogsFiltersState
  providerOptions: LogProviderOption[]
  loading: boolean
  isFilterValid: boolean
  yearPickerValue: number | null
  monthPickerValue: MonthModel | null
  dayPickerValue: Date | null
  rangePickerValue: Date[] | null
  isDarkTheme: boolean
}>()

const emit = defineEmits<{
  (event: 'submit'): void
  (event: 'update:platform', value: LogsFiltersState['platform']): void
  (event: 'update:provider', value: string): void
  (event: 'update:date-type', value: LogDateFilterType): void
  (event: 'update:year-picker-value', value: number | null): void
  (event: 'update:month-picker-value', value: MonthModel | null): void
  (event: 'update:day-picker-value', value: Date | null): void
  (event: 'update:range-picker-value', value: Date[] | null): void
}>()

const { t, locale } = useI18n()

const dateFnsLocale = computed(() => (locale.value === 'zh' ? zhCN : enUS))

const datePickerUi = {
  input: 'mac-input logs-date-picker-input',
  menu: 'mac-panel logs-date-picker-menu',
}

const yearPickerRange = computed<[number, number]>(() => getLogsYearPickerRange())

const rangePickerConfig = { partialRange: false } as const

const pad2 = (num: number) => num.toString().padStart(2, '0')

const formatDateYmd = (date: Date) =>
  `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`

const formatRangeInput = (dates: Array<Date | null>) => {
  const [start, end] = dates ?? []
  if (!start) return ''
  if (!end) return formatDateYmd(start)
  return `${formatDateYmd(start)} ~ ${formatDateYmd(end)}`
}

const updatePlatform = (event: Event) => {
  emit('update:platform', (event.target as HTMLSelectElement).value as LogsFiltersState['platform'])
}

const updateProvider = (event: Event) => {
  emit('update:provider', (event.target as HTMLSelectElement).value)
}

const updateDateType = (event: Event) => {
  emit('update:date-type', (event.target as HTMLSelectElement).value as LogDateFilterType)
}
</script>
