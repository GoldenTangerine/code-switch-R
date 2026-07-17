import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useLogsFilters } from './useLogsFilters'
import { getLogsYearPickerRange } from '../utils'

const createFiltersComposable = () =>
  useLogsFilters({
    t: (key, params) => (params ? `${key}:${JSON.stringify(params)}` : key),
  })

describe('useLogsFilters', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-24T12:00:00Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('resets year filter to the current year whenever switching to year mode', () => {
    const { filters, updateFilterDateType } = createFiltersComposable()

    updateFilterDateType('year')
    expect(filters.year).toBe('2026')

    filters.year = '2020'
    updateFilterDateType('month')
    updateFilterDateType('year')

    expect(filters.dateType).toBe('year')
    expect(filters.year).toBe('2026')
  })

  it('accepts only years within the current year window for picker value and date range', () => {
    const { filters, yearPickerValue, computeDateRange } = createFiltersComposable()

    filters.dateType = 'year'
    filters.year = '2016'
    expect(yearPickerValue.value).toBe(2016)
    expect(computeDateRange()).toEqual({
      startAt: '2016-01-01 00:00:00',
      endAt: '2017-01-01 00:00:00',
    })

    filters.year = '2037'
    expect(yearPickerValue.value).toBeNull()
    expect(computeDateRange()).toBeNull()
  })

  it('builds the selectable year range around the current year', () => {
    expect(getLogsYearPickerRange()).toEqual([2016, 2036])
  })

  it('clears the model filter when the platform or provider changes', () => {
    const { filters, updateFilterPlatform, updateFilterProvider } = createFiltersComposable()

    filters.model = 'claude-sonnet-4-5'
    updateFilterPlatform('codex')
    expect(filters.model).toBe('')

    filters.model = 'gpt-5.1-codex'
    updateFilterProvider('provider-1')
    expect(filters.model).toBe('')
  })
})
