import type { ProviderDailyStat } from '../../../services/logs'
import { formatCurrencyParts } from '../utils'

export interface LogsCostDetailRow {
  providerKey: string
  provider: string
  providerRef: string
  initial: string
  amount: number
  amountLabel: string
  shareLabel: string
  barWidth: string
  isHigh: boolean
}

export interface LogsCostDetailViewState {
  rows: LogsCostDetailRow[]
  totalAmount: number
  totalAmountParts: {
    symbol: string
    whole: string
    fraction: string
    formatted: string
  }
  providerCount: number
  showSummary: boolean
}

interface BuildLogsCostDetailViewStateOptions {
  data: ProviderDailyStat[]
  error?: string
  formatCurrency: (value?: number) => string
}

export function buildLogsCostDetailViewState(options: BuildLogsCostDetailViewStateOptions): LogsCostDetailViewState {
  const rows = buildLogsCostDetailRows(options.data, options.formatCurrency)
  const totalAmount = rows.reduce((sum, row) => sum + row.amount, 0)

  return {
    rows,
    totalAmount,
    totalAmountParts: formatCurrencyParts(totalAmount),
    providerCount: rows.length,
    showSummary: !`${options.error ?? ''}`.trim(),
  }
}

export function buildLogsCostDetailRows(
  data: ProviderDailyStat[],
  formatCurrency: (value?: number) => string,
): LogsCostDetailRow[] {
  const normalizedRows = [...(data ?? [])]
    .map(item => ({
      item,
      amount: safePositiveNumber(item.cost_total),
    }))
    .filter(entry => entry.amount > 0)
    .sort((left, right) => right.amount - left.amount)

  const totalAmount = normalizedRows.reduce((sum, entry) => sum + entry.amount, 0)
  const averageAmount = normalizedRows.length > 0 ? totalAmount / normalizedRows.length : 0
  const hasRelativeHighSpend = normalizedRows.length > 1 && totalAmount >= 5

  return normalizedRows.map(({ item, amount }, index) => {
    const provider = resolveProviderLabel(item)
    const sharePercent = totalAmount > 0 ? (amount / totalAmount) * 100 : 0
    const isHigh = amount >= 50
      || (hasRelativeHighSpend
        && index === 0
        && amount >= Math.max(totalAmount * 0.45, averageAmount * 1.85))

    return {
      providerKey: `${item.provider_id ?? ''}-${provider}-${index}`,
      provider,
      providerRef: resolveProviderRef(item),
      initial: resolveProviderInitial(provider),
      amount,
      amountLabel: formatCurrency(amount),
      shareLabel: formatSharePercent(sharePercent),
      barWidth: resolveBarWidth(sharePercent),
      isHigh,
    }
  })
}

function safePositiveNumber(value: number | undefined) {
  const normalized = Number(value ?? 0)
  return Number.isFinite(normalized) && normalized > 0 ? normalized : 0
}

function resolveProviderLabel(item: ProviderDailyStat) {
  const provider = `${item.provider ?? ''}`.trim()
  const providerId = `${item.provider_id ?? ''}`.trim()
  return provider || providerId || '—'
}

function resolveProviderRef(item: ProviderDailyStat) {
  return `${item.provider_id ?? item.provider ?? ''}`.trim()
}

function resolveProviderInitial(value: string) {
  const normalized = `${value ?? ''}`.trim()
  if (!normalized) return '—'
  return normalized.charAt(0).toUpperCase()
}

function formatSharePercent(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '0%'
  if (value < 1) return '<1%'
  if (value >= 99.5) return '100%'
  if (value >= 10) return `${Math.round(value)}%`
  return `${value.toFixed(1)}%`
}

function resolveBarWidth(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '0%'
  if (value < 5) return '5%'
  return `${Math.min(value, 100)}%`
}
