import { pad2, type BudgetQuotaKey } from './budgetUsage'
import type { TrayBudgetDisplayMode } from './trayBudgetDisplay'

export type TrayCountdownQuota = {
  key: BudgetQuotaKey
  hasBudget: boolean
  nextReset: Date | null
}

export const formatClockCountdown = (remainingMs: number) => {
  if (remainingMs <= 0) {
    return '00:00:00'
  }
  const totalSeconds = Math.max(Math.ceil(remainingMs / 1000), 1)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  return `${pad2(hours)}:${pad2(minutes)}:${pad2(seconds)}`
}

export const shouldUseSecondPrecisionTrayTicker = (
  displayMode: TrayBudgetDisplayMode | 'pending',
  showCountdown: boolean,
  quotas: readonly TrayCountdownQuota[],
) => {
  return showCountdown
    && displayMode === 'quotas'
    && quotas.some((quota) => quota.key === 'five_hour' && quota.hasBudget && Boolean(quota.nextReset))
}

export const updateItemsAndCollectRefresh = <T>(
  items: readonly T[],
  updateItem: (item: T) => boolean,
) => {
  let shouldRefresh = false
  for (const item of items) {
    if (updateItem(item)) {
      shouldRefresh = true
    }
  }
  return shouldRefresh
}
