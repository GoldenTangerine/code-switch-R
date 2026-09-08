/**
 * @name: 托盘共享快照适配
 * @Descripttion: 将后端统一供应商快照适配为托盘额度与统计展示。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-08 16:53:06
 * @LastEditTime: 2026-09-08 16:53:06
 * @FilePath: frontend/src/components/Tray/traySharedSnapshot.ts
 */
import { Call } from '@wailsio/runtime'
import type { ProviderDailyStat } from '../../services/logs'
import type { ProviderQuotaQueryItem } from '../../services/providerQuotaQuery'
import type { ProviderQuotaSnapshotItem } from '../Main/utils/providerQuotaSnapshot'
import { formatProviderQuotaCountdownLabel, providerQuotaLabelKeyMap } from '../Main/utils/providerQuotaSnapshot'
import type { TranslateFn } from '../Main/types'

export type SharedTrayQuota = ProviderQuotaQueryItem & { displayKind: 'progress' | 'balance' | 'error' }
export type SharedTrayProvider = {
  providerId: string
  providerName: string
  icon: string
  activeRequests: number
  status: 'active' | 'default'
  loading: boolean
  updatedAt: number
  quotas: SharedTrayQuota[]
  stats: ProviderDailyStat | null
}
export type SharedTrayPlatform = {
  platform: string
  name: string
  icon: string
  error: boolean
  providers: SharedTrayProvider[]
}
export type SharedTraySnapshot = {
  version: number
  session: string
  sequence: number
  heartbeatAt: number
  platforms: SharedTrayPlatform[]
}

export const fetchSharedTraySnapshot = (): Promise<SharedTraySnapshot> => Call.ByName(
  'codeswitch/services.TraySnapshotService.GetSnapshot',
)

export function sharedTrayQuotaItem(item: SharedTrayQuota, t: TranslateFn, now: Date): ProviderQuotaSnapshotItem {
  const nextReset = item.nextReset ? new Date(item.nextReset.replace(' ', 'T')) : null
  const validReset = nextReset && Number.isFinite(nextReset.getTime()) ? nextReset : null
  return {
    key: item.key,
    label: providerQuotaLabelKeyMap[item.key] ? t(providerQuotaLabelKeyMap[item.key]) : item.label || item.key,
    used: item.used,
    total: item.total,
    unlimited: item.unlimited,
    valueMode: item.valueMode,
    unit: item.unit,
    nextReset: validReset,
    queriedAt: item.displayKind === 'balance' || item.displayKind === 'error' ? now.getTime() : undefined,
    invalidMessage: item.invalidMessage ? t('tray.queryFailed') : '',
    extra: item.extra,
    countdownLabel: item.active === false ? t('components.main.providers.quotaInactive') : formatProviderQuotaCountdownLabel(validReset, now),
    progressRatio: item.total > 0 ? item.used / item.total : 0,
    trackedUsed: item.used,
    adjustment: 0,
    remaining: Math.max(0, item.total - item.used),
    isActive: item.active !== false,
  }
}
