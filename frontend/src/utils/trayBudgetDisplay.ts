import {
  budgetQuotaOrder,
  normalizeBudgetQuotaSettings,
  type BudgetQuotaKey,
} from './budgetUsage'

export type TrayBudgetDisplayMode = 'summary' | 'quotas'

export const getVisibleTrayQuotaKeys = (settings: unknown): BudgetQuotaKey[] => {
  const normalizedSettings = normalizeBudgetQuotaSettings(settings)
  return budgetQuotaOrder.filter((key) => normalizedSettings[key].total > 0)
}

export const resolveTrayBudgetDisplayMode = (settings: unknown): TrayBudgetDisplayMode => {
  return getVisibleTrayQuotaKeys(settings).length > 0 ? 'quotas' : 'summary'
}
