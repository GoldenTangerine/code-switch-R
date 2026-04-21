import type { ModelPricingRow } from '../services/modelPricing'

export type ModelPricingSource = 'builtin' | 'manual' | 'claude_sync' | 'cloud_sync' | ''
export type ModelPricingSourceFilter = 'all' | 'manual'

export function normalizeModelPricingSource(source: string | undefined): ModelPricingSource {
  const normalized = String(source ?? '').trim().toLowerCase()
  if (normalized === 'builtin') return 'builtin'
  if (normalized === 'manual') return 'manual'
  if (normalized === 'claude_sync') return 'claude_sync'
  if (normalized === 'cloud_sync') return 'cloud_sync'
  return ''
}

export function isManualModelPricingRow(row: Pick<ModelPricingRow, 'source'>) {
  return normalizeModelPricingSource(row.source) === 'manual'
}

export function matchesModelPricingSourceFilter(
  filter: ModelPricingSourceFilter,
  row: Pick<ModelPricingRow, 'source'>,
) {
  if (filter === 'all') return true
  return isManualModelPricingRow(row)
}
