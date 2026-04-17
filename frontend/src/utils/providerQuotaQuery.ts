export type ProviderQuotaQueryType =
  | 'none'
  | 'token_plan_glm'
  | 'token_plan_kimi'
  | 'token_plan_minimax'

export const providerQuotaQueryTypes: ProviderQuotaQueryType[] = [
  'none',
  'token_plan_glm',
  'token_plan_kimi',
  'token_plan_minimax',
]

export const providerQuotaQueryTypeLabelKeyMap: Record<ProviderQuotaQueryType, string> = {
  none: 'components.main.form.options.providerQuotaQueryNone',
  token_plan_glm: 'components.main.form.options.providerQuotaQueryTokenPlanGLM',
  token_plan_kimi: 'components.main.form.options.providerQuotaQueryTokenPlanKimi',
  token_plan_minimax: 'components.main.form.options.providerQuotaQueryTokenPlanMiniMax',
}

export function normalizeProviderQuotaQueryType(value: unknown): ProviderQuotaQueryType {
  const normalized = String(value ?? '').trim().toLowerCase()
  if (
    normalized === 'token_plan_glm'
    || normalized === 'token_plan_kimi'
    || normalized === 'token_plan_minimax'
  ) {
    return normalized
  }
  return 'none'
}

export function hasProviderQuotaQueryType(value: unknown): boolean {
  return normalizeProviderQuotaQueryType(value) !== 'none'
}

export function serializeProviderQuotaQueryType(value: unknown): ProviderQuotaQueryType | undefined {
  const normalized = normalizeProviderQuotaQueryType(value)
  return normalized === 'none' ? undefined : normalized
}
