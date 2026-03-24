import { computed, reactive } from 'vue'
import type { MonthModel } from '@vuepic/vue-datepicker'
import type { LogDateFilterType, LogsFiltersState } from '../types'
import {
  formatDateYmd,
  getCurrentYear,
  isLogsYearInRange,
  pad2,
  startOfTodayLocal,
  toDateParts,
  toTimeLayout,
} from '../utils'

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

type LogsDateRange = {
  startAt: string
  endAt: string
}

type UseLogsFiltersOptions = {
  t: TranslateFn
}

export function useLogsFilters(options: UseLogsFiltersOptions) {
  const { t } = options

  const filters = reactive<LogsFiltersState>({
    platform: '',
    provider: '',
    dateType: 'all',
    year: '',
    month: '',
    day: '',
    rangeStart: '',
    rangeEnd: '',
  })

  const yearPickerValue = computed<number | null>({
    get() {
      const year = Number(filters.year)
      if (!isLogsYearInRange(year)) return null
      return year
    },
    set(value) {
      filters.year = value == null ? '' : String(value)
    },
  })

  const monthPickerValue = computed<MonthModel | null>({
    get() {
      const match = String(filters.month || '').match(/^(\d{4})-(\d{2})$/)
      if (!match) return null
      const year = Number(match[1])
      const month = Number(match[2])
      if (!Number.isFinite(year) || !Number.isFinite(month) || month < 1 || month > 12) return null
      return { year, month: month - 1 }
    },
    set(value) {
      if (!value) {
        filters.month = ''
        return
      }
      const year = Number(value.year)
      const monthIndex = Number(value.month)
      if (!Number.isFinite(year) || !Number.isFinite(monthIndex) || monthIndex < 0 || monthIndex > 11) {
        filters.month = ''
        return
      }
      filters.month = `${year}-${pad2(monthIndex + 1)}`
    },
  })

  const dayPickerValue = computed<Date | null>({
    get() {
      if (!filters.day) return null
      const parts = toDateParts(filters.day)
      if (!parts) return null
      return new Date(parts.y, parts.m - 1, parts.d, 0, 0, 0, 0)
    },
    set(value) {
      filters.day = value ? formatDateYmd(value) : ''
    },
  })

  const rangePickerValue = computed<Date[] | null>({
    get() {
      if (!filters.rangeStart || !filters.rangeEnd) return null
      const startParts = toDateParts(filters.rangeStart)
      const endParts = toDateParts(filters.rangeEnd)
      if (!startParts || !endParts) return null
      const start = new Date(startParts.y, startParts.m - 1, startParts.d, 0, 0, 0, 0)
      const end = new Date(endParts.y, endParts.m - 1, endParts.d, 0, 0, 0, 0)
      return [start, end]
    },
    set(value) {
      if (!value || value.length < 2 || !value[0] || !value[1]) {
        filters.rangeStart = ''
        filters.rangeEnd = ''
        return
      }
      filters.rangeStart = formatDateYmd(value[0])
      filters.rangeEnd = formatDateYmd(value[1])
    },
  })

  const updateFilterPlatform = (value: LogsFiltersState['platform']) => {
    filters.platform = value
  }

  const updateFilterProvider = (value: string) => {
    filters.provider = value
  }

  const updateFilterDateType = (value: LogDateFilterType) => {
    filters.dateType = value
    if (value === 'year') {
      filters.year = String(getCurrentYear())
    }
  }

  const updateYearPickerValue = (value: number | null) => {
    yearPickerValue.value = value
  }

  const updateMonthPickerValue = (value: MonthModel | null) => {
    monthPickerValue.value = value
  }

  const updateDayPickerValue = (value: Date | null) => {
    dayPickerValue.value = value
  }

  const updateRangePickerValue = (value: Date[] | null) => {
    rangePickerValue.value = value
  }

  const computeDateRange = (): LogsDateRange | null => {
    switch (filters.dateType) {
      case 'all':
        return { startAt: '', endAt: '' }
      case 'today': {
        const start = startOfTodayLocal()
        const end = new Date(start.getTime())
        end.setDate(end.getDate() + 1)
        return { startAt: toTimeLayout(start), endAt: toTimeLayout(end) }
      }
      case 'year': {
        const year = Number(filters.year)
        if (!isLogsYearInRange(year)) return null
        const start = new Date(year, 0, 1, 0, 0, 0, 0)
        const end = new Date(year + 1, 0, 1, 0, 0, 0, 0)
        return { startAt: toTimeLayout(start), endAt: toTimeLayout(end) }
      }
      case 'month': {
        const match = String(filters.month || '').match(/^(\d{4})-(\d{2})$/)
        if (!match) return null
        const year = Number(match[1])
        const month = Number(match[2])
        if (!Number.isFinite(year) || !Number.isFinite(month) || month < 1 || month > 12) return null
        const start = new Date(year, month - 1, 1, 0, 0, 0, 0)
        const end = new Date(year, month, 1, 0, 0, 0, 0)
        return { startAt: toTimeLayout(start), endAt: toTimeLayout(end) }
      }
      case 'day': {
        if (!filters.day) return null
        const parts = toDateParts(filters.day)
        if (!parts) return null
        const start = new Date(parts.y, parts.m - 1, parts.d, 0, 0, 0, 0)
        const end = new Date(parts.y, parts.m - 1, parts.d + 1, 0, 0, 0, 0)
        return { startAt: toTimeLayout(start), endAt: toTimeLayout(end) }
      }
      case 'range': {
        if (!filters.rangeStart || !filters.rangeEnd) return null
        const startParts = toDateParts(filters.rangeStart)
        const endParts = toDateParts(filters.rangeEnd)
        if (!startParts || !endParts) return null
        const start = new Date(startParts.y, startParts.m - 1, startParts.d, 0, 0, 0, 0)
        const inclusiveEnd = new Date(endParts.y, endParts.m - 1, endParts.d, 0, 0, 0, 0)
        if (start.getTime() > inclusiveEnd.getTime()) return null
        const endExclusive = new Date(endParts.y, endParts.m - 1, endParts.d + 1, 0, 0, 0, 0)
        return { startAt: toTimeLayout(start), endAt: toTimeLayout(endExclusive) }
      }
      default:
        return null
    }
  }

  const isFilterValid = computed(() => {
    if (filters.dateType === 'all') return true
    return computeDateRange() != null
  })

  const summaryScopeHint = computed(() => {
    switch (filters.dateType) {
      case 'all': {
        const today = startOfTodayLocal()
        const date = `${today.getFullYear()}-${pad2(today.getMonth() + 1)}-${pad2(today.getDate())}`
        return t('components.logs.summary.todayScope', { date })
      }
      case 'today': {
        const today = startOfTodayLocal()
        const date = `${today.getFullYear()}-${pad2(today.getMonth() + 1)}-${pad2(today.getDate())}`
        return t('components.logs.summary.todayScope', { date })
      }
      case 'year': {
        const year = filters.year?.trim()
        return year ? t('components.logs.summary.yearScope', { year }) : ''
      }
      case 'month': {
        const month = filters.month?.trim()
        return month ? t('components.logs.summary.monthScope', { month }) : ''
      }
      case 'day': {
        const day = filters.day?.trim()
        return day ? t('components.logs.summary.dayScope', { date: day }) : ''
      }
      case 'range': {
        const start = filters.rangeStart?.trim()
        const end = filters.rangeEnd?.trim()
        if (!start || !end) return ''
        return t('components.logs.summary.rangeScope', { start, end })
      }
      default:
        return ''
    }
  })

  return {
    filters,
    yearPickerValue,
    monthPickerValue,
    dayPickerValue,
    rangePickerValue,
    updateFilterPlatform,
    updateFilterProvider,
    updateFilterDateType,
    updateYearPickerValue,
    updateMonthPickerValue,
    updateDayPickerValue,
    updateRangePickerValue,
    computeDateRange,
    isFilterValid,
    summaryScopeHint,
  }
}
