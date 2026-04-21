import type { ProviderQuotaDisplayItem } from '../types'

const clampPercent = (value: number) => {
  if (!Number.isFinite(value)) return 0
  return Math.min(Math.max(value, 0), 100)
}

export const getQuotaUsagePercentValue = (item: Pick<ProviderQuotaDisplayItem, 'progressRatio'>) => {
  if (!Number.isFinite(item.progressRatio)) return 0
  return Math.max(item.progressRatio, 0) * 100
}

export const getQuotaProgressPercent = (item: Pick<ProviderQuotaDisplayItem, 'progressRatio'>) => {
  return clampPercent(getQuotaUsagePercentValue(item))
}

export const formatQuotaUsagePercent = (item: Pick<ProviderQuotaDisplayItem, 'progressRatio'>) => {
  const percent = getQuotaUsagePercentValue(item)
  if (percent <= 0) return '0%'
  if (percent < 1) return '<1%'
  return `${Math.round(percent)}%`
}

export const getQuotaProgressClass = (item: Pick<ProviderQuotaDisplayItem, 'progressRatio'>) => {
  const percent = getQuotaUsagePercentValue(item)
  if (percent >= 100) return 'quota-progress--over'
  if (percent >= 90) return 'quota-progress--critical'
  if (percent >= 72) return 'quota-progress--hot'
  if (percent >= 45) return 'quota-progress--warm'
  if (percent >= 18) return 'quota-progress--steady'
  return 'quota-progress--fresh'
}
