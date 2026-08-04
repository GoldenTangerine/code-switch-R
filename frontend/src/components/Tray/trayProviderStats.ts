/**
 * @name: 托盘供应商统计
 * @Descripttion: 匹配并格式化托盘当前供应商的今日统计与性能数据
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-04 10:08:50
 * @LastEditTime: 2026-08-04 10:08:50
 * @FilePath: frontend/src/components/Tray/trayProviderStats.ts
 */
import type { AutomationCard } from '../../data/cards'
import type { ProviderDailyStat } from '../../services/logs'
import { formatAdaptiveDurationSeconds } from '../../utils/durationFormat'
import {
  formatProviderSuccessRate,
  formatProviderTokenCount,
  formatProviderTokensPerSecond,
} from '../../utils/providerStatsFormat'
import {
  cardProviderRef,
  normalizeProviderKey,
  normalizeProviderRef,
} from '../Main/adapters/providerCardMappers'
import { SUCCESS_RATE_THRESHOLDS } from '../Main/constants'
import { buildProviderCostDisplay } from '../Main/utils/providerCostDisplay'

const EMPTY_VALUE = '—'

export type TraySuccessRateTone = 'good' | 'warning' | 'bad' | 'neutral'

export interface TrayProviderStatsDisplay {
  successRate: string
  successRateTone: TraySuccessRateTone
  requests: string
  tokens: string
  cost: string
  firstToken: string
  speed: string
}

export interface PreparedTrayProviderStats {
  providerKey: string
  display: TrayProviderStatsDisplay
}

function normalizeNumber(value: unknown) {
  const numeric = Number(value ?? 0)
  return Number.isFinite(numeric) ? Math.max(numeric, 0) : 0
}

function normalizeInteger(value: unknown) {
  return Math.floor(normalizeNumber(value))
}

function formatMetric(value: number, locale: string) {
  return value.toLocaleString(locale || 'en')
}

function resolveSuccessRateTone(value: number): TraySuccessRateTone {
  if (value >= SUCCESS_RATE_THRESHOLDS.healthy) return 'good'
  if (value >= SUCCESS_RATE_THRESHOLDS.warning) return 'warning'
  return 'bad'
}

function formatCost(value: unknown, locale: string) {
  const display = buildProviderCostDisplay(normalizeNumber(value), locale || 'en')
  return display.parts.map((part) => part.value).join(' ')
}

export function getTrayProviderStatsKey(provider: AutomationCard) {
  const providerRef = cardProviderRef(provider)
  if (providerRef) return `id:${providerRef}`
  return `name:${normalizeProviderKey(provider.name)}`
}

export function prepareTrayProviderStatsRefresh(
  provider: AutomationCard,
  previousProviderKey: string,
  previousDisplay: TrayProviderStatsDisplay | null,
  locale = 'en',
): PreparedTrayProviderStats {
  const providerKey = getTrayProviderStatsKey(provider)
  return {
    providerKey,
    display: providerKey === previousProviderKey && previousDisplay
      ? previousDisplay
      : buildTrayProviderStatsDisplay(provider, [], locale),
  }
}

export function resolveTrayProviderDailyStat(
  provider: AutomationCard,
  stats: readonly ProviderDailyStat[],
): ProviderDailyStat | null {
  const providerRef = cardProviderRef(provider)
  if (providerRef) {
    const matchedByRef = stats.find((stat) => normalizeProviderRef(stat.provider_id) === providerRef)
    if (matchedByRef) return matchedByRef
  }

  const providerName = normalizeProviderKey(provider.name)
  if (!providerName) return null

  return stats.find((stat) => (
    !normalizeProviderRef(stat.provider_id)
    && normalizeProviderKey(stat.provider) === providerName
  )) ?? null
}

export function buildTrayProviderStatsDisplay(
  provider: AutomationCard,
  stats: readonly ProviderDailyStat[],
  locale = 'en',
): TrayProviderStatsDisplay {
  const stat = resolveTrayProviderDailyStat(provider, stats)
  if (!stat) {
    return {
      successRate: EMPTY_VALUE,
      successRateTone: 'neutral',
      requests: '0',
      tokens: '0',
      cost: formatCost(0, locale),
      firstToken: EMPTY_VALUE,
      speed: EMPTY_VALUE,
    }
  }

  const successfulRequests = normalizeInteger(stat.successful_requests)
  const failedRequests = normalizeInteger(stat.failed_requests)
  const evaluatedRequests = successfulRequests + failedRequests
  const successRate = Math.min(Math.max(normalizeNumber(stat.success_rate), 0), 1)
  const totalTokens = normalizeInteger(stat.input_tokens)
    + normalizeInteger(stat.output_tokens)
    + normalizeInteger(stat.cache_read_tokens)

  return {
    successRate: evaluatedRequests > 0 ? formatProviderSuccessRate(successRate) : EMPTY_VALUE,
    successRateTone: evaluatedRequests > 0 ? resolveSuccessRateTone(successRate) : 'neutral',
    requests: formatMetric(normalizeInteger(stat.total_requests), locale),
    tokens: formatProviderTokenCount(totalTokens, locale),
    cost: formatCost(stat.cost_total, locale),
    firstToken: formatAdaptiveDurationSeconds(stat.avg_first_token_sec),
    speed: formatProviderTokensPerSecond(stat.avg_tokens_per_sec, 't/s'),
  }
}
