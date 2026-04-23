export function resolveProviderQuotaCurrencyCode(unit?: string): string | undefined {
  const normalizedUnit = `${unit ?? ''}`.trim().toUpperCase()
  if (!normalizedUnit) return undefined

  if (normalizedUnit === '¥' || normalizedUnit === '￥' || normalizedUnit === 'RMB') {
    return 'CNY'
  }

  return /^[A-Z]{3}$/.test(normalizedUnit)
    ? normalizedUnit
    : undefined
}
