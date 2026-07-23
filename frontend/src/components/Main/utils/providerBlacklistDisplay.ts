/**
 * @name: 供应商黑名单展示工具
 * @Descripttion: 统一生成供应商成功率提示中的黑名单计数信息
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-23 16:00:00
 * @LastEditTime: 2026-07-23 16:00:00
 * @FilePath: frontend/src/components/Main/utils/providerBlacklistDisplay.ts
 */

import type { ProviderBlacklistCounters, TranslateFn } from '../types'

const formatThreshold = (value: number | null) => value ?? '—'

export function buildProviderSuccessRateTooltip(
  summary: string,
  counters: ProviderBlacklistCounters,
  t: TranslateFn,
): string {
  return [
    summary,
    t('components.main.blacklist.requestCount', {
      current: counters.failureCount,
      threshold: formatThreshold(counters.failureThreshold),
    }),
    t('components.main.blacklist.healthCount', {
      current: counters.healthFailureCount,
      threshold: formatThreshold(counters.healthFailureThreshold),
    }),
  ].filter(Boolean).join('\n')
}
