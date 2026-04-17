import type { ClaudeOfficialPricingPreviewRow } from '../services/modelPricing'

export type ClaudePricingPreviewTargetMode = 'mapped' | 'custom' | 'missing'

export function resolveClaudePricingPreviewTargetMode(
  row: ClaudeOfficialPricingPreviewRow,
): ClaudePricingPreviewTargetMode {
  if (row.is_recognized) return 'mapped'

  const hasTargets = (row.target_models ?? []).some(model => String(model ?? '').trim() !== '')
  if (hasTargets) return 'custom'

  return 'missing'
}

export function summarizeClaudePricingPreviewTargets(
  row: ClaudeOfficialPricingPreviewRow,
): string {
  const targets = (row.target_models ?? [])
    .map(model => String(model ?? '').trim())
    .filter(model => model !== '')

  if (targets.length <= 2) return targets.join(', ')
  return `${targets.slice(0, 2).join(', ')} +${targets.length - 2}`
}
