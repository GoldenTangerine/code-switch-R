import { Call } from '@wailsio/runtime'
import {
  resolveProviderQuotaQueryType,
  sanitizeProviderQuotaQueryConfigForSave,
  type ProviderQuotaQueryType,
  type ProviderQuotaQueryConfig,
} from '../utils/providerQuotaQuery'

const PROVIDER_QUOTA_QUERY_SERVICE = 'codeswitch/services.ProviderQuotaQueryService'
const PROVIDER_QUOTA_AUTOMATION_SERVICE = 'codeswitch/services.ProviderQuotaAutomationService'

export type ProviderQuotaQueryItem = {
  key: string
  label?: string
  used: number
  total: number
  unlimited?: boolean
  nextReset?: string
  active?: boolean
  isValid?: boolean
  valueMode?: 'currency' | 'count'
  unit?: string
  extra?: string
  invalidMessage?: string
  /** 被动采样时间（毫秒） */
  sampledAt?: number
  /** 推算标记：aged 为样本超过 5 分钟，projected 为固定周期窗口越过重置时间后本地顺延，reset 为会话窗口已重置、等待下一次请求开始计时 */
  estimate?: 'aged' | 'projected' | 'reset' | string
}

export type ProviderQuotaQueryResult = {
  success: boolean
  queryType: ProviderQuotaQueryType
  items: ProviderQuotaQueryItem[]
  error?: string
  queriedAt?: number
}

export type ProviderQuotaAutomationResult = ProviderQuotaQueryResult & {
  providerEnabled: boolean
  quotaAutoDisabled: boolean
  quotaAutoDisablePaused: boolean
  stateChanged: boolean
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

export const checkProviderQuota = (
  kind: string,
  providerID: string,
): Promise<ProviderQuotaAutomationResult> => Call.ByName(
  `${PROVIDER_QUOTA_AUTOMATION_SERVICE}.CheckProviderQuota`,
  kind,
  providerID,
)

export const temporarilyEnableQuotaProvider = (
  kind: string,
  providerID: string,
): Promise<ProviderQuotaAutomationResult> => Call.ByName(
  `${PROVIDER_QUOTA_AUTOMATION_SERVICE}.TemporarilyEnableProvider`,
  kind,
  providerID,
)

export const resumeProviderQuotaAutomation = (
  kind: string,
  providerID: string,
): Promise<ProviderQuotaAutomationResult> => Call.ByName(
  `${PROVIDER_QUOTA_AUTOMATION_SERVICE}.ResumeProviderQuotaAutomation`,
  kind,
  providerID,
)
