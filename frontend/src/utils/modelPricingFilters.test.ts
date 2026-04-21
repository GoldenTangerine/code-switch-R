import { describe, expect, it } from 'vitest'
import { isManualModelPricingRow, matchesModelPricingSourceFilter } from './modelPricingFilters'

describe('modelPricingFilters', () => {
  it('should treat manual source as local', () => {
    expect(isManualModelPricingRow({ source: 'manual' })).toBe(true)
  })

  it('should not treat synced overrides as local', () => {
    expect(matchesModelPricingSourceFilter('manual', { source: 'cloud_sync' })).toBe(false)
    expect(matchesModelPricingSourceFilter('manual', { source: 'claude_sync' })).toBe(false)
  })

  it('should allow all rows when filter is all', () => {
    expect(matchesModelPricingSourceFilter('all', { source: 'cloud_sync' })).toBe(true)
    expect(matchesModelPricingSourceFilter('all', { source: 'manual' })).toBe(true)
  })
})
