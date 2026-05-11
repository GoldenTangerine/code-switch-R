import { resolveProviderQuotaCurrencyCode } from '../Main/utils/providerQuotaValueFormat'

export type TrayAmountPart = {
  role: 'amount' | 'unit' | 'literal'
  value: string
}

export type TrayQuotaValueMode = 'currency' | 'count'

const getSafeNumber = (value?: number) => {
  const numeric = Number(value ?? 0)
  return Number.isFinite(numeric) ? numeric : 0
}

const mapCurrencyPartRole = (partType: string): TrayAmountPart['role'] => {
  if (partType === 'currency') return 'unit'
  if (partType === 'literal') return 'literal'
  return 'amount'
}

export const joinTrayAmountParts = (parts: readonly TrayAmountPart[]) => (
  parts.map((part) => part.value).join('')
)

export const formatTrayCurrencyParts = (
  value?: number,
  unit?: string,
  locale = 'en',
): TrayAmountPart[] => {
  const safeValue = getSafeNumber(value)
  const currencyCode = resolveProviderQuotaCurrencyCode(unit)
  if (currencyCode) {
    return new Intl.NumberFormat(locale, {
      style: 'currency',
      currency: currencyCode,
      minimumFractionDigits: safeValue >= 1 ? 2 : 4,
      maximumFractionDigits: safeValue >= 1 ? 2 : 4,
    }).formatToParts(safeValue).map((part) => ({
      role: mapCurrencyPartRole(part.type),
      value: part.value,
    }))
  }
  if (safeValue >= 1) {
    return [
      { role: 'unit', value: '$' },
      { role: 'amount', value: safeValue.toFixed(2) },
    ]
  }
  if (safeValue >= 0.01) {
    return [
      { role: 'unit', value: '$' },
      { role: 'amount', value: safeValue.toFixed(3) },
    ]
  }
  return [
    { role: 'unit', value: '$' },
    { role: 'amount', value: safeValue.toFixed(4) },
  ]
}

export const formatTrayQuotaValueParts = (
  value: number | undefined,
  valueMode: TrayQuotaValueMode = 'currency',
  unit?: string,
  locale = 'en',
): TrayAmountPart[] => {
  const safeValue = getSafeNumber(value)
  if (valueMode === 'count') {
    const formatted = new Intl.NumberFormat(locale, {
      maximumFractionDigits: Number.isInteger(safeValue) ? 0 : 2,
    }).format(safeValue)
    const normalizedUnit = unit?.trim()
    return normalizedUnit
      ? [
          { role: 'amount', value: formatted },
          { role: 'literal', value: ' ' },
          { role: 'unit', value: normalizedUnit },
        ]
      : [{ role: 'amount', value: formatted }]
  }
  return formatTrayCurrencyParts(safeValue, unit, locale)
}

export const formatTrayCurrency = (
  value?: number,
  unit?: string,
  locale = 'en',
) => joinTrayAmountParts(formatTrayCurrencyParts(value, unit, locale))

export const formatTrayQuotaValue = (
  value: number | undefined,
  valueMode: TrayQuotaValueMode = 'currency',
  unit?: string,
  locale = 'en',
) => joinTrayAmountParts(formatTrayQuotaValueParts(value, valueMode, unit, locale))
