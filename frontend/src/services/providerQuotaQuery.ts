import { Call } from '@wailsio/runtime'
import type { BudgetQuotaKey } from '../utils/budgetUsage'
import {
  normalizeProviderQuotaQueryType,
  type ProviderQuotaQueryType,
} from '../utils/providerQuotaQuery'

const PROVIDER_QUOTA_QUERY_SERVICE = 'codeswitch/services.ProviderQuotaQueryService'

export type ProviderQuotaQueryItem = {
  key: BudgetQuotaKey
  used: number
  total: number
  nextReset?: string
  active?: boolean
}

export type ProviderQuotaQueryResult = {
  success: boolean
  queryType: ProviderQuotaQueryType
  items: ProviderQuotaQueryItem[]
  error?: string
  queriedAt?: number
}

export async function queryProviderQuota(
  queryType: ProviderQuotaQueryType,
  apiUrl: string,
  apiKey: string,
): Promise<ProviderQuotaQueryResult> {
  const normalizedQueryType = normalizeProviderQuotaQueryType(queryType)
  return Call.ByName(
    `${PROVIDER_QUOTA_QUERY_SERVICE}.QueryQuota`,
    normalizedQueryType,
    apiUrl.trim(),
    apiKey.trim(),
  )
}
