import type { ProviderCostDisplayPart } from '../types'

type ProviderCostDisplay = {
  formatted: string
  parts: ProviderCostDisplayPart[]
}

const currencyFormatterCache = new Map<string, Intl.NumberFormat>()

const isAmountPart = (type: Intl.NumberFormatPart['type']) => (
  type === 'integer'
  || type === 'group'
  || type === 'decimal'
  || type === 'fraction'
)

export const getProviderCostFormatter = (locale: string) => {
  const normalizedLocale = `${locale || 'en'}`.trim() || 'en'
  let formatter = currencyFormatterCache.get(normalizedLocale)
  if (!formatter) {
    formatter = new Intl.NumberFormat(normalizedLocale, {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })
    currencyFormatterCache.set(normalizedLocale, formatter)
  }
  return formatter
}

export const buildProviderCostDisplay = (value: number, locale = 'en'): ProviderCostDisplay => {
  const normalizedValue = Number.isFinite(value) && value > 0 ? value : 0
  const formatter = getProviderCostFormatter(locale)
  const formatted = formatter.format(normalizedValue)
  const rawParts = formatter.formatToParts(normalizedValue)

  const currency = rawParts
    .filter((part) => part.type === 'currency')
    .map((part) => part.value)
    .join('')
    .trim() || 'USD'

  const amount = rawParts
    .filter((part) => isAmountPart(part.type))
    .map((part) => part.value)
    .join('')
    .trim() || '0.00'

  const orderedTypes = rawParts.reduce<ProviderCostDisplayPart['type'][]>((list, part) => {
    const resolvedType = part.type === 'currency'
      ? 'currency'
      : isAmountPart(part.type)
        ? 'amount'
        : null

    if (resolvedType && !list.includes(resolvedType)) {
      list.push(resolvedType)
    }

    return list
  }, [])

  const displayOrder: ProviderCostDisplayPart['type'][] = orderedTypes.length === 2
    ? orderedTypes
    : orderedTypes[0] === 'amount'
      ? ['amount', 'currency']
      : ['currency', 'amount']

  return {
    formatted,
    parts: displayOrder.map((type) => ({
      type,
      value: type === 'currency' ? currency : amount,
    })),
  }
}
