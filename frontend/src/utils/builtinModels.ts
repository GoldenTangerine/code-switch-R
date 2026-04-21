import type { CLIPlatform } from '../services/cliConfig'
import type { ModelPricingRow } from '../services/modelPricing'

export type BuiltinModelPlatform = CLIPlatform

export function isBuiltinModelPlatform(
  value: string | null | undefined,
): value is BuiltinModelPlatform {
  return value === 'claude' || value === 'codex' || value === 'gemini'
}

export function isDirectCliModelCandidate(model: string): boolean {
  const normalized = model.trim()
  return normalized !== '' && !/[/:@]/.test(normalized) && !/^[a-z]+\./i.test(normalized)
}

export function matchesBuiltinModelPlatform(
  platform: BuiltinModelPlatform,
  model: string,
): boolean {
  const normalized = model.trim().toLowerCase()
  if (!normalized) return false

  if (platform === 'claude') {
    return /claude|sonnet|haiku|opus|anthropic/.test(normalized)
  }

  if (platform === 'gemini') {
    return /gemini/.test(normalized)
  }

  return /\bgpt\b|\bo\d+|codex|whisper|text-embedding|openai/.test(normalized)
}

export function scoreBuiltinModel(model: string): number {
  const normalized = model.toLowerCase()
  let score = 0

  if (!/[/:@]/.test(model)) score += 4
  if (!normalized.includes('azure/')) score += 2
  if (!normalized.includes('openrouter/')) score += 2
  if (!normalized.includes('vertex_ai/')) score += 1

  return score
}

export function buildBuiltinModelOptions(
  rows: ModelPricingRow[],
  platform?: BuiltinModelPlatform | null,
): string[] {
  if (!platform) return []

  const seen = new Set<string>()

  return rows
    .filter((row) => {
      const source = `${row.source || ''}`.trim().toLowerCase()
      if (source && source !== 'builtin' && source !== 'claude_sync' && source !== 'cloud_sync') {
        return false
      }

      return isDirectCliModelCandidate(row.model) && matchesBuiltinModelPlatform(platform, row.model)
    })
    .map((row) => row.model.trim())
    .filter((model) => {
      if (!model || seen.has(model)) return false
      seen.add(model)
      return true
    })
    .sort((left, right) => {
      const scoreDiff = scoreBuiltinModel(right) - scoreBuiltinModel(left)
      if (scoreDiff !== 0) return scoreDiff
      return left.localeCompare(right)
    })
}
