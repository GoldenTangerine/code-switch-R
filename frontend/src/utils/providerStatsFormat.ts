/**
 * @name: 供应商统计格式化
 * @Descripttion: 统一格式化供应商 Tokens、成功率和生成速度
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-04 10:37:27
 * @LastEditTime: 2026-08-04 10:37:27
 * @FilePath: frontend/src/utils/providerStatsFormat.ts
 */

const EMPTY_VALUE = '—'

function normalizeNonNegativeNumber(value: unknown) {
  const numeric = Number(value ?? 0)
  return Number.isFinite(numeric) ? Math.max(numeric, 0) : 0
}

export function formatProviderTokenCount(value: unknown, locale = '') {
  const tokens = Math.floor(normalizeNonNegativeNumber(value))
  if (tokens >= 1_000_000_000) return `${(tokens / 1_000_000_000).toFixed(2)}B`
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(2)}M`
  if (tokens >= 1_000) return `${(tokens / 1_000).toFixed(2)}k`
  return tokens.toLocaleString(locale || undefined)
}

export function formatProviderSuccessRate(value: unknown) {
  const percent = Math.min(normalizeNonNegativeNumber(value), 1) * 100
  const decimals = percent >= 99.5 || percent === 0 ? 0 : 1
  return `${percent.toFixed(decimals)}%`
}

export function formatProviderTokensPerSecond(value: unknown, unit = 'tokens/s') {
  const tokensPerSecond = normalizeNonNegativeNumber(value)
  if (tokensPerSecond <= 0) return EMPTY_VALUE
  const precision = tokensPerSecond >= 100 ? 1 : 2
  return `${tokensPerSecond.toFixed(precision)} ${unit}`
}
