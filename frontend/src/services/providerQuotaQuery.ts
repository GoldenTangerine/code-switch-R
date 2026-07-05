import { Call } from '@wailsio/runtime'
import {
  resolveProviderQuotaQueryType,
  sanitizeProviderQuotaQueryConfigForSave,
  type ProviderQuotaQueryType,
  type ProviderQuotaQueryConfig,
} from '../utils/providerQuotaQuery'

const PROVIDER_QUOTA_QUERY_SERVICE = 'codeswitch/services.ProviderQuotaQueryService'

export type ProviderQuotaQueryItem = {
  key: string
  label?: string
  used: number
  total: number
  nextReset?: string
  active?: boolean
  isValid?: boolean
  valueMode?: 'currency' | 'count'
  unit?: string
  extra?: string
  invalidMessage?: string
}

export type ProviderQuotaQueryResult = {
  success: boolean
  queryType: ProviderQuotaQueryType
  items: ProviderQuotaQueryItem[]
  error?: string
  queriedAt?: number
}

export type ProviderQuotaScriptValidationResult = {
  valid: boolean
  error?: string
}

export async function queryProviderQuota(
  queryTypeOrConfig: ProviderQuotaQueryType | ProviderQuotaQueryConfig,
  apiUrl: string,
  apiKey: string,
): Promise<ProviderQuotaQueryResult> {
  const normalizedQueryType = resolveProviderQuotaQueryType(queryTypeOrConfig)
  const normalizedConfig = sanitizeProviderQuotaQueryConfigForSave(queryTypeOrConfig, normalizedQueryType)
  return Call.ByName(
    `${PROVIDER_QUOTA_QUERY_SERVICE}.QueryQuota`,
    normalizedQueryType,
    apiUrl.trim(),
    apiKey.trim(),
    normalizedConfig ?? null,
  )
}

export async function validateProviderQuotaScriptPreset(
  templateType: string,
  code: string,
): Promise<ProviderQuotaScriptValidationResult> {
  return Call.ByName(
    `${PROVIDER_QUOTA_QUERY_SERVICE}.ValidateScriptPreset`,
    templateType,
    code,
  )
}
