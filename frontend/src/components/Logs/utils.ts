import type { ModelUsageStat, RequestLog } from '../../services/logs'
import type { ModelPricingRow } from '../../services/modelPricing'
import type { Chart } from 'chart.js'
import type { CostTooltipDetail, CostTooltipPriceLine, LogInfoTooltipDetail, LogInfoTooltipRow, LogInfoTooltipTone, ModelShareRow } from './types'
import { COST_TOOLTIP_DIFF_EPSILON, PER_MILLION_TOKENS, TOKENS_PER_SECOND_MIN_WINDOW_SEC } from './constants'

export type CacheCreateTokenSplit = {
  totalTokens: number
  tokens5m: number
  tokens1h: number
}

export type CacheCreateTier = '5m' | '1h'

export type CacheCreatePriceRate = {
  tier?: CacheCreateTier
  perToken: number
}

export type TokenRatePriceLineOptions = {
  inputPerToken: number
  outputPerToken: number
  reasoningPerToken: number
  cacheCreateRates: CacheCreatePriceRate[]
  cacheReadPerToken: number
  includeCacheRead: boolean
  includeReasoning: boolean
  suffix?: string
  includeCacheMultiplierHint?: boolean
}

export type CacheCreateCostDetail = {
  tier: CacheCreateTier
  tokens: number
  perToken: number
  cost: number
}

export type TokenRatePriceLineLabels = {
  promptPrice: string
  completionPrice: string
  cacheCreatePriceLabel: (tier?: CacheCreateTier) => string
  cacheCreateMultiplierLabel: (multiplier: number, tier?: CacheCreateTier) => string
  cacheReadPrice: string
  cacheReadMultiplierLabel: (multiplier: number) => string
  reasoningPrice: string
}

export type ProviderApiPerCallPriceLineLabels = {
  perCallUnifiedPrice: string
  perCallInputPrice: string
  perCallOutputPrice: string
  perRequestSuffix: string
}

type LogsTranslateParams = Record<string, string | number>

export type LogsTranslate = (key: string, params?: LogsTranslateParams) => string

export type TokenCostFormulaLabels = {
  usagePrompt: string
  usageCompletion: string
  usageReasoning: string
  usageCacheRead: string
  formulaEmpty: string
  cacheCreateUsageLabel: (tier?: CacheCreateTier) => string
  cacheCreateMultiplierLabel: (multiplier: number, tier?: CacheCreateTier) => string
  cacheReadMultiplierLabel: (multiplier: number) => string
  groupMultiplierLabel: (multiplier: number) => string
}

export type LogsCostTooltipLabels = {
  tokenRatePriceLineLabels: TokenRatePriceLineLabels
  providerApiPerCallPriceLineLabels: ProviderApiPerCallPriceLineLabels
  tokenFormulaLabels: TokenCostFormulaLabels
  observedPriceSuffix: string
  providerApiFormula: string
  providerApiPerCallFormula: string
  providerApiHint: string
  providerApiFallbackHint: string
  providerApiZeroCostHint: string
  noPricingFormula: string
  noPricingHint: string
  recordedCostHint: (cost: string) => string
  matchedModelHint: (model: string) => string
}

export type LogsInfoTooltipLabels = {
  modelTitle: string
  verifyTitle: string
  tooltipValueMissing: string
  pricingSourceLabel: string
  pricingModelLabel: string
  pricingDetailLabel: string
  pricingFormulaLabel: string
  pricingHintLabel: string
  recordedCostLabel: string
  requestedModelLabel: string
  responseModelLabel: string
  userAgentLabel: string
  pricingUnavailableValue: string
  priceSourceLabels: Record<LogPriceSource, string>
}

export type LogsTableTextFormatters = {
  formatStream: (value?: boolean | number) => string
  formatPayloadDetailAriaLabel: (item: RequestLog) => string
  formatModelVerifyStatus: (item: RequestLog) => string
  formatModelInfoAriaLabel: (item: RequestLog) => string
  formatReasoningEffort: (item: RequestLog) => string
  formatReasoningEffortTone: (value?: string) => string
  formatVerifyInfoAriaLabel: (item: RequestLog) => string
  formatCostAriaLabel: (item: RequestLog) => string
}

const cacheCreateTokenSplitCache = new WeakMap<RequestLog, CacheCreateTokenSplit>()

type CostSnapshotPayload = Pick<RequestLog,
  | 'total_cost'
  | 'input_cost'
  | 'output_cost'
  | 'reasoning_cost'
  | 'cache_create_cost'
  | 'cache_read_cost'
  | 'ephemeral_5m_cost'
  | 'ephemeral_1h_cost'
  | 'has_pricing'
>

export const pad2 = (num: number) => String(num).padStart(2, '0')

export const formatDateYmd = (date: Date) =>
  `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`

export const getCurrentYear = () => new Date().getFullYear()

export const LOGS_YEAR_PICKER_SPAN = 10

export const getLogsYearPickerRange = (baseYear = getCurrentYear()): [number, number] => [
  baseYear - LOGS_YEAR_PICKER_SPAN,
  baseYear + LOGS_YEAR_PICKER_SPAN,
]

export const isLogsYearInRange = (year: number, baseYear = getCurrentYear()) => {
  if (!Number.isInteger(year)) return false
  const [minYear, maxYear] = getLogsYearPickerRange(baseYear)
  return year >= minYear && year <= maxYear
}

export const toDateParts = (value: string) => {
  const [y, m, d] = value.split('-').map((item) => Number(item))
  if (!Number.isFinite(y) || !Number.isFinite(m) || !Number.isFinite(d)) return null
  if (y <= 0 || m <= 0 || d <= 0) return null
  return { y, m, d }
}

export const toTimeLayout = (date: Date) => {
  const pad = (num: number) => String(num).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

export const startOfTodayLocal = () => {
  const now = new Date()
  now.setHours(0, 0, 0, 0)
  return now
}

export const formatBytes = (bytes?: number, rows?: number) => {
  const value = Number(bytes ?? 0)
  const count = Number(rows ?? 0)
  if (!Number.isFinite(value) || value < 0) return '—'
  if (value === 0 && Number.isFinite(count) && count > 0) return '—'
  if (value === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let current = value
  let idx = 0
  while (current >= 1024 && idx < units.length - 1) {
    current /= 1024
    idx += 1
  }
  const digits = idx === 0 ? 0 : current >= 10 ? 1 : 2
  return `${current.toFixed(digits)} ${units[idx]}`
}

export const formatStorageHeatmapPayloadValue = (
  bytes: number | undefined,
  capturedRequests: number | undefined,
  requests: number | undefined,
  payloadUnavailableText: string,
) => {
  const totalRequests = Number(requests ?? 0)
  if (!Number.isFinite(totalRequests) || totalRequests <= 0) {
    return '0 B'
  }
  const capturedCount = Number(capturedRequests ?? 0)
  if (!Number.isFinite(capturedCount) || capturedCount <= 0) {
    return payloadUnavailableText
  }
  return formatBytes(bytes)
}

export const parseLogDate = (value?: string) => {
  if (!value) return null
  const normalize = value.replace(' ', 'T')
  const attempts = [value, normalize, `${normalize}Z`]
  for (const candidate of attempts) {
    const parsed = new Date(candidate)
    if (!Number.isNaN(parsed.getTime())) {
      return parsed
    }
  }
  const match = value.match(/^(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2}) ([+-]\d{4}) UTC$/)
  if (match) {
    const [, day, time, zone] = match
    const zoneFormatted = `${zone.slice(0, 3)}:${zone.slice(3)}`
    const parsed = new Date(`${day}T${time}${zoneFormatted}`)
    if (!Number.isNaN(parsed.getTime())) {
      return parsed
    }
  }
  return null
}

export const padHour = (num: number) => String(num).padStart(2, '0')

export const formatTime = (value?: string) => {
  const date = parseLogDate(value)
  if (!date) return value || '—'
  return `${date.getFullYear()}-${padHour(date.getMonth() + 1)}-${padHour(date.getDate())} ${padHour(date.getHours())}:${padHour(date.getMinutes())}:${padHour(date.getSeconds())}`
}

export const isStreamingLog = (value?: boolean | number) => value === true || value === 1

export const formatStream = (value: boolean | number | undefined, streamOnText: string, streamOffText: string) =>
  isStreamingLog(value) ? streamOnText : streamOffText

export const formatDuration = (value?: number) => {
  if (!value || Number.isNaN(value)) return '—'
  return `${value.toFixed(2)}s`
}

export const toPositiveFinite = (value?: number) => {
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric) || numeric <= 0) return 0
  return numeric
}

export const formatFirstTokenMs = (item: RequestLog) => {
  if (!isStreamingLog(item.is_stream)) return '—'
  const seconds = toPositiveFinite(item.first_token_sec)
  if (seconds <= 0) return '—'
  const milliseconds = seconds * 1000
  const precision = milliseconds >= 100 ? 0 : milliseconds >= 10 ? 1 : 2
  return `${milliseconds.toFixed(precision)} ms`
}

export const formatTokensPerSecond = (item: RequestLog) => {
  if (!isStreamingLog(item.is_stream)) return '—'
  const outputTokens = Number(item.output_tokens ?? 0)
  if (!Number.isFinite(outputTokens) || outputTokens <= 0) return '—'
  const totalDuration = toPositiveFinite(item.duration_sec)
  const firstToken = toPositiveFinite(item.first_token_sec)
  if (firstToken <= 0) return '—'
  const generationWindow = totalDuration - firstToken
  if (generationWindow < TOKENS_PER_SECOND_MIN_WINDOW_SEC) return '—'
  const tokensPerSecond = outputTokens / generationWindow
  if (!Number.isFinite(tokensPerSecond) || tokensPerSecond <= 0) return '—'
  const precision = tokensPerSecond >= 100 ? 1 : 2
  return `${tokensPerSecond.toFixed(precision)} tokens/s`
}

export const normalizeReasoningEffortDisplay = (value?: string) => {
  const raw = String(value ?? '').trim()
  const normalized = raw.toLowerCase().replace(/[-_\s]/g, '')
  switch (normalized) {
    case 'low':
    case 'medium':
    case 'high':
    case 'xhigh':
    case 'max':
      return normalized
    case 'extrahigh':
      return 'xhigh'
    default:
      return raw.toLowerCase()
  }
}

export const resolveReasoningEffortTone = (value?: string) => {
  const normalized = normalizeReasoningEffortDisplay(value)
  if (normalized === 'low') return 'low'
  if (normalized === 'medium') return 'medium'
  if (normalized === 'high') return 'high'
  if (normalized === 'xhigh') return 'xhigh'
  if (normalized === 'max') return 'max'
  return 'unknown'
}

export const httpCodeClass = (code: number) => {
  if (code >= 500) return 'http-server-error'
  if (code >= 400) return 'http-client-error'
  if (code >= 300) return 'http-redirect'
  if (code >= 200) return 'http-success'
  return 'http-info'
}

export const durationColor = (value?: number) => {
  if (!value || Number.isNaN(value)) return 'neutral'
  if (value < 2) return 'fast'
  if (value < 5) return 'medium'
  return 'slow'
}

export const safeNumber = (value?: number) => {
  const numeric = Number(value ?? 0)
  return Number.isFinite(numeric) ? numeric : 0
}

export const normalizePricingModelKey = (value: string) =>
  value
    .trim()
    .toLowerCase()
    .replace(/[-_.:/\s]/g, '')

export const commonPrefixLength = (a: string, b: string) => {
  const limit = Math.min(a.length, b.length)
  let cursor = 0
  while (cursor < limit && a[cursor] === b[cursor]) {
    cursor += 1
  }
  return cursor
}

export const stripPricingRegionPrefix = (value: string) => {
  const trimmed = value.trim()
  const lowered = trimmed.toLowerCase()
  for (const prefix of ['us.', 'eu.', 'apac.']) {
    if (lowered.startsWith(prefix)) {
      return trimmed.slice(prefix.length)
    }
  }
  return trimmed
}

export const stripPricingProviderPrefix = (value: string) => {
  const trimmed = value.trim()
  const lowered = trimmed.toLowerCase()
  if (lowered.startsWith('anthropic.')) {
    return trimmed.slice('anthropic.'.length)
  }
  return trimmed
}

export const pricingAliasCandidates = (value: string) => {
  const lowered = value.trim().toLowerCase()
  if (lowered === 'gpt-5-codex') {
    return ['gpt-5']
  }
  return [] as string[]
}

export const buildPricingModelCandidates = (value: string) => {
  const base = value.trim()
  if (!base) return [] as string[]
  const result = new Set<string>()
  const collect = (candidate: string) => {
    const normalized = candidate.trim()
    if (normalized) {
      result.add(normalized)
    }
  }
  const collectWithVariants = (candidate: string) => {
    const trimmed = candidate.trim()
    if (!trimmed) return
    collect(trimmed)
    collect(stripPricingRegionPrefix(trimmed))
    collect(stripPricingProviderPrefix(trimmed))
    collect(stripPricingProviderPrefix(stripPricingRegionPrefix(trimmed)))
    for (const alias of pricingAliasCandidates(trimmed)) {
      collect(alias)
      collect(stripPricingRegionPrefix(alias))
      collect(stripPricingProviderPrefix(alias))
      collect(stripPricingProviderPrefix(stripPricingRegionPrefix(alias)))
    }
  }
  collectWithVariants(base)

  const noLongContextSuffix = base.replace(/\[1m\]/gi, '').trim()
  if (noLongContextSuffix && noLongContextSuffix !== base) {
    collectWithVariants(noLongContextSuffix)
  }
  return Array.from(result)
}

export const formatUsdPrecise = (value: number) => `$${safeNumber(value).toFixed(6)}`

export const formatUsdPerMillion = (perTokenPrice: number) =>
  formatUsdPrecise(safeNumber(perTokenPrice) * PER_MILLION_TOKENS)

export const formatMultiplierValue = (value: number) => {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric < 0) return '1'
  const rounded = Number(numeric.toFixed(4))
  if (Number.isInteger(rounded)) return String(rounded)
  return rounded.toString()
}

export type ModelPricingLookup = {
  byExact: Map<string, ModelPricingRow>
  byLower: Map<string, ModelPricingRow>
  byNormalized: Map<string, ModelPricingRow>
}

export const buildModelPricingLookup = (modelPricingRows: ModelPricingRow[]): ModelPricingLookup => {
  const byExact = new Map<string, ModelPricingRow>()
  const byLower = new Map<string, ModelPricingRow>()
  const byNormalized = new Map<string, ModelPricingRow>()
  for (const row of modelPricingRows) {
    const model = String(row.model ?? '').trim()
    if (!model) continue
    byExact.set(model, row)
    byLower.set(model.toLowerCase(), row)
    byNormalized.set(normalizePricingModelKey(model), row)
  }
  return { byExact, byLower, byNormalized }
}

export const buildLogPricingModelCandidates = (
  item: Pick<RequestLog,
    | 'response_model'
    | 'matched_pricing_model'
    | 'requested_model'
    | 'total_cost'
    | 'input_cost'
    | 'output_cost'
    | 'reasoning_cost'
    | 'cache_create_cost'
    | 'cache_read_cost'
    | 'ephemeral_5m_cost'
    | 'ephemeral_1h_cost'
    | 'has_pricing'
  >,
) => {
  const hasHistoricalPricingSnapshot = hasStoredCostSnapshot(item)
  const candidates = hasHistoricalPricingSnapshot
    ? [item.matched_pricing_model, item.response_model, item.requested_model]
    : [item.response_model, item.matched_pricing_model, item.requested_model]
  const seen = new Set<string>()
  const resolved: string[] = []
  for (const value of candidates) {
    const normalized = String(value ?? '').trim()
    if (!normalized) continue
    const key = normalized.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    resolved.push(normalized)
  }
  return resolved
}

export const resolveLogPricingModelName = (
  item: Parameters<typeof buildLogPricingModelCandidates>[0],
) => buildLogPricingModelCandidates(item)[0] ?? ''

export const resolvePricingRow = (
  item: RequestLog,
  lookup: ModelPricingLookup,
  modelPricingRows: ModelPricingRow[],
) => {
  const candidates = buildLogPricingModelCandidates(item)
  for (const modelName of candidates) {
    const name = String(modelName ?? '').trim()
    if (!name) continue
    for (const candidate of buildPricingModelCandidates(name)) {
      const exact = lookup.byExact.get(candidate)
      if (exact) return exact
      const lower = lookup.byLower.get(candidate.toLowerCase())
      if (lower) return lower
      const normalized = lookup.byNormalized.get(normalizePricingModelKey(candidate))
      if (normalized) return normalized
    }
  }

  for (const modelName of candidates) {
    const name = String(modelName ?? '').trim()
    if (!name) continue
    const targetNorm = normalizePricingModelKey(name)
    if (!targetNorm) continue
    let bestRow: ModelPricingRow | null = null
    let bestScore = -1
    for (const row of modelPricingRows) {
      const rowNorm = normalizePricingModelKey(String(row.model ?? ''))
      if (!rowNorm) continue
      if (!(rowNorm.includes(targetNorm) || targetNorm.includes(rowNorm))) continue
      const maxLen = Math.max(rowNorm.length, targetNorm.length)
      if (maxLen <= 0) continue
      const prefixScore = commonPrefixLength(rowNorm, targetNorm) / maxLen
      const overlapScore = Math.min(rowNorm.length, targetNorm.length) / maxLen
      const score = overlapScore * 0.8 + prefixScore * 0.2
      if (score > bestScore) {
        bestScore = score
        bestRow = row
      }
    }
    if (bestRow) return bestRow
  }
  return null
}

export const isTrueFlag = (value: unknown) => value === true || value === 1

export const isProviderPerCallValueSet = (value?: number, setFlag?: boolean) => {
  if (isTrueFlag(setFlag)) return true
  if (setFlag === false) return false
  return safeNumber(value) > 0
}

export const withPriceSuffix = (value: string, suffix?: string) => {
  const normalized = String(suffix ?? '').trim()
  return normalized ? `${value} ${normalized}` : value
}

export const normalizeTokenInteger = (value?: number) => {
  const normalized = Number(value ?? 0)
  if (!Number.isFinite(normalized)) return 0
  return Math.max(0, Math.round(normalized))
}

export const normalizeCacheCreateTokenSplit = (
  totalTokens: number | undefined,
  ephemeral5mTokens: number | undefined,
  ephemeral1hTokens: number | undefined,
): CacheCreateTokenSplit => {
  let total = normalizeTokenInteger(totalTokens)
  let tokens5m = normalizeTokenInteger(ephemeral5mTokens)
  let tokens1h = normalizeTokenInteger(ephemeral1hTokens)

  if (total === 0 && (tokens5m > 0 || tokens1h > 0)) {
    total = tokens5m + tokens1h
  }
  if (total <= 0) {
    return {
      totalTokens: 0,
      tokens5m: 0,
      tokens1h: 0,
    }
  }

  if (tokens1h > total) {
    tokens1h = total
  }
  const max5m = Math.max(0, total - tokens1h)
  if (tokens5m > max5m) {
    tokens5m = max5m
  }

  const assigned = tokens5m + tokens1h
  if (assigned < total) {
    tokens5m += total - assigned
  }

  return {
    totalTokens: total,
    tokens5m,
    tokens1h,
  }
}

export const resolveCacheCreateTokenSplit = (item: RequestLog): CacheCreateTokenSplit => {
  const cached = cacheCreateTokenSplitCache.get(item)
  if (cached) return cached
  const split = normalizeCacheCreateTokenSplit(
    item.cache_create_tokens,
    item.ephemeral_5m_tokens,
    item.ephemeral_1h_tokens,
  )
  cacheCreateTokenSplitCache.set(item, split)
  return split
}

export const resolveEphemeral1hTokens = (item: RequestLog) =>
  resolveCacheCreateTokenSplit(item).tokens1h

export const resolveEphemeral5mTokens = (item: RequestLog) =>
  resolveCacheCreateTokenSplit(item).tokens5m

export const hasCacheCreateDetail = (item: RequestLog) => {
  const split = resolveCacheCreateTokenSplit(item)
  if (split.totalTokens <= 0) return false
  return split.tokens5m > 0 || split.tokens1h > 0
}

export const formatCacheCreateTierLabel = (tier: CacheCreateTier) => tier

export const buildCacheCreateCostDetails = ({
  split,
  totalCost,
  ephemeral5mCost,
  ephemeral1hCost,
  fallback5mPerToken,
  fallback1hPerToken,
  fallbackCombinedPerToken,
}: {
  split: CacheCreateTokenSplit
  totalCost: number
  ephemeral5mCost: number
  ephemeral1hCost: number
  fallback5mPerToken: number
  fallback1hPerToken: number
  fallbackCombinedPerToken: number
}): CacheCreateCostDetail[] => {
  const tokens5m = split.tokens5m
  const tokens1h = split.tokens1h
  if (tokens5m <= 0 && tokens1h <= 0) return []

  const normalizedTotalCost = Math.max(0, safeNumber(totalCost))
  const normalized5mCost = tokens5m > 0 ? Math.max(0, safeNumber(ephemeral5mCost)) : 0
  const normalized1hCost = tokens1h > 0 ? Math.max(0, safeNumber(ephemeral1hCost)) : 0
  const fallbackCombined = Math.max(0, safeNumber(fallbackCombinedPerToken))
  const fallback5m = Math.max(0, safeNumber(fallback5mPerToken)) || fallbackCombined
  const fallback1h = Math.max(0, safeNumber(fallback1hPerToken)) || fallbackCombined

  let perToken5m = tokens5m > 0 && normalized5mCost > 0 ? normalized5mCost / tokens5m : 0
  let perToken1h = tokens1h > 0 && normalized1hCost > 0 ? normalized1hCost / tokens1h : 0

  if (tokens5m > 0 && tokens1h <= 0 && perToken5m <= 0) {
    perToken5m = normalizedTotalCost > 0 ? normalizedTotalCost / tokens5m : fallback5m
  }
  if (tokens1h > 0 && tokens5m <= 0 && perToken1h <= 0) {
    perToken1h = normalizedTotalCost > 0 ? normalizedTotalCost / tokens1h : fallback1h
  }

  if (tokens5m > 0 && tokens1h > 0) {
    if (normalizedTotalCost > 0) {
      const has5mCost = perToken5m > 0
      const has1hCost = perToken1h > 0
      if (has5mCost && !has1hCost) {
        const remaining = normalizedTotalCost - normalized5mCost
        if (remaining > 0) perToken1h = remaining / tokens1h
      } else if (!has5mCost && has1hCost) {
        const remaining = normalizedTotalCost - normalized1hCost
        if (remaining > 0) perToken5m = remaining / tokens5m
      } else if (!has5mCost && !has1hCost) {
        const base5mCost = tokens5m * fallback5m
        const base1hCost = tokens1h * fallback1h
        const baseTotalCost = base5mCost + base1hCost
        if (baseTotalCost > 0) {
          const scale = normalizedTotalCost / baseTotalCost
          perToken5m = fallback5m * scale
          perToken1h = fallback1h * scale
        } else {
          const averagePerToken = normalizedTotalCost / (tokens5m + tokens1h)
          perToken5m = averagePerToken
          perToken1h = averagePerToken
        }
      }
    }
    if (perToken5m <= 0) perToken5m = fallback5m
    if (perToken1h <= 0) perToken1h = fallback1h
  }

  const details: CacheCreateCostDetail[] = []
  if (tokens5m > 0 && perToken5m > 0) {
    details.push({
      tier: '5m',
      tokens: tokens5m,
      perToken: perToken5m,
      cost: tokens5m * perToken5m,
    })
  }
  if (tokens1h > 0 && perToken1h > 0) {
    details.push({
      tier: '1h',
      tokens: tokens1h,
      perToken: perToken1h,
      cost: tokens1h * perToken1h,
    })
  }

  if (details.length > 0) return details

  if (tokens5m > 0) {
    details.push({
      tier: '5m',
      tokens: tokens5m,
      perToken: fallback5m,
      cost: tokens5m * fallback5m,
    })
  }
  if (tokens1h > 0) {
    details.push({
      tier: '1h',
      tokens: tokens1h,
      perToken: fallback1h,
      cost: tokens1h * fallback1h,
    })
  }
  return details
}

export const buildTokenRatePriceLines = (
  {
    inputPerToken,
    outputPerToken,
    reasoningPerToken,
    cacheCreateRates,
    cacheReadPerToken,
    includeCacheRead,
    includeReasoning,
    suffix = '',
    includeCacheMultiplierHint = false,
  }: TokenRatePriceLineOptions,
  labels: TokenRatePriceLineLabels,
): CostTooltipPriceLine[] => {
  const completionMultiplier = inputPerToken > 0 ? outputPerToken / inputPerToken : 0
  const cacheReadMultiplier = inputPerToken > 0 ? cacheReadPerToken / inputPerToken : 0
  const tokensUnit = '/ 1M tokens'
  const priceLines: CostTooltipPriceLine[] = [
    {
      key: 'prompt',
      label: labels.promptPrice,
      value: withPriceSuffix(`${formatUsdPerMillion(inputPerToken)} ${tokensUnit}`, suffix),
    },
  ]

  const completionValue =
    completionMultiplier > 0 && inputPerToken > 0
      ? `${formatUsdPerMillion(inputPerToken)} * ${formatMultiplierValue(completionMultiplier)} = ${formatUsdPerMillion(outputPerToken)} ${tokensUnit}`
      : `${formatUsdPerMillion(outputPerToken)} ${tokensUnit}`
  priceLines.push({
    key: 'completion',
    label: labels.completionPrice,
    value: withPriceSuffix(completionValue, suffix),
  })

  const cacheCreateRatesToRender = cacheCreateRates.length > 0
    ? cacheCreateRates
    : [{ perToken: 0 }]
  cacheCreateRatesToRender.forEach((entry, index) => {
    const cacheCreatePerToken = Math.max(0, safeNumber(entry.perToken))
    const cacheCreateMultiplier = inputPerToken > 0 ? cacheCreatePerToken / inputPerToken : 0
    const cacheCreateHint = includeCacheMultiplierHint && cacheCreateMultiplier > 0
      ? ` (${labels.cacheCreateMultiplierLabel(cacheCreateMultiplier, entry.tier)})`
      : ''
    const cacheCreateValue =
      cacheCreateMultiplier > 0 && inputPerToken > 0
        ? `${formatUsdPerMillion(inputPerToken)} * ${formatMultiplierValue(cacheCreateMultiplier)} = ${formatUsdPerMillion(cacheCreatePerToken)} ${tokensUnit}${cacheCreateHint}`
        : `${formatUsdPerMillion(cacheCreatePerToken)} ${tokensUnit}`
    const keySuffix = entry.tier ?? String(index)
    priceLines.push({
      key: `cacheCreate-${keySuffix}`,
      label: labels.cacheCreatePriceLabel(entry.tier),
      value: withPriceSuffix(cacheCreateValue, suffix),
    })
  })

  if (includeCacheRead) {
    const cacheReadHint = includeCacheMultiplierHint
      ? ` (${labels.cacheReadMultiplierLabel(cacheReadMultiplier)})`
      : ''
    const cacheReadValue =
      cacheReadMultiplier > 0 && inputPerToken > 0
        ? `${formatUsdPerMillion(inputPerToken)} * ${formatMultiplierValue(cacheReadMultiplier)} = ${formatUsdPerMillion(cacheReadPerToken)} ${tokensUnit}${cacheReadHint}`
        : `${formatUsdPerMillion(cacheReadPerToken)} ${tokensUnit}`
    priceLines.push({
      key: 'cacheRead',
      label: labels.cacheReadPrice,
      value: withPriceSuffix(cacheReadValue, suffix),
    })
  }

  if (includeReasoning) {
    priceLines.push({
      key: 'reasoning',
      label: labels.reasoningPrice,
      value: withPriceSuffix(`${formatUsdPerMillion(reasoningPerToken)} ${tokensUnit}`, suffix),
    })
  }

  return priceLines
}

export const buildObservedCostPriceLines = (
  item: RequestLog,
  labels: TokenRatePriceLineLabels & { suffix: string },
): CostTooltipPriceLine[] => {
  if (!hasBreakdownCostPayload(item)) return []

  const cacheCreateSplit = resolveCacheCreateTokenSplit(item)
  const inputTokens = Math.max(0, Math.round(safeNumber(item.input_tokens)))
  const outputTokens = Math.max(0, Math.round(safeNumber(item.output_tokens)))
  const reasoningTokens = Math.max(0, Math.round(safeNumber(item.reasoning_tokens)))
  const cacheCreateTokens = cacheCreateSplit.totalTokens
  const cacheReadTokens = Math.max(0, Math.round(safeNumber(item.cache_read_tokens)))

  const inputCost = Math.max(0, safeNumber(item.input_cost))
  const outputCost = Math.max(0, safeNumber(item.output_cost))
  const reasoningCost = Math.max(0, safeNumber(item.reasoning_cost))
  const cacheCreateCost = Math.max(0, safeNumber(item.cache_create_cost))
  const cacheReadCost = Math.max(0, safeNumber(item.cache_read_cost))
  const ephemeral5mCost = Math.max(0, safeNumber(item.ephemeral_5m_cost))
  const ephemeral1hCost = Math.max(0, safeNumber(item.ephemeral_1h_cost))

  const inputPerToken = inputTokens > 0 ? inputCost / inputTokens : 0
  const outputPerToken = outputTokens > 0 ? outputCost / outputTokens : 0
  const reasoningPerToken = reasoningTokens > 0 ? reasoningCost / reasoningTokens : 0
  const cacheReadPerToken = cacheReadTokens > 0 ? cacheReadCost / cacheReadTokens : 0
  const cacheCreatePerToken = cacheCreateTokens > 0 ? cacheCreateCost / cacheCreateTokens : 0
  const cacheCreateDetails = buildCacheCreateCostDetails({
    split: cacheCreateSplit,
    totalCost: cacheCreateCost,
    ephemeral5mCost,
    ephemeral1hCost,
    fallback5mPerToken: cacheCreatePerToken,
    fallback1hPerToken: cacheCreatePerToken,
    fallbackCombinedPerToken: cacheCreatePerToken,
  })
  const cacheCreateRates = cacheCreateDetails.length > 0
    ? cacheCreateDetails.map(detail => ({
      tier: detail.tier,
      perToken: detail.perToken,
    }))
    : [{ perToken: cacheCreatePerToken }]

  return buildTokenRatePriceLines({
    inputPerToken,
    outputPerToken,
    reasoningPerToken,
    cacheCreateRates,
    cacheReadPerToken,
    includeCacheRead: cacheReadTokens > 0,
    includeReasoning: reasoningTokens > 0,
    suffix: labels.suffix,
  }, labels)
}

export const buildLogsInfoTooltipLabels = (translate: LogsTranslate): LogsInfoTooltipLabels => ({
  modelTitle: translate('components.logs.table.model'),
  verifyTitle: translate('components.logs.table.verify'),
  tooltipValueMissing: translate('components.logs.table.tooltipValues.missing'),
  pricingSourceLabel: translate('components.logs.table.tooltipLabels.pricingSource'),
  pricingModelLabel: translate('components.logs.table.tooltipLabels.pricingModel'),
  pricingDetailLabel: translate('components.logs.table.tooltipLabels.pricingDetail'),
  pricingFormulaLabel: translate('components.logs.table.tooltipLabels.pricingFormula'),
  pricingHintLabel: translate('components.logs.table.tooltipLabels.pricingHint'),
  recordedCostLabel: translate('components.logs.table.tooltipLabels.recordedCost'),
  requestedModelLabel: translate('components.logs.table.tooltipLabels.requestedModel'),
  responseModelLabel: translate('components.logs.table.tooltipLabels.responseModel'),
  userAgentLabel: translate('components.logs.table.tooltipLabels.userAgent'),
  pricingUnavailableValue: translate('components.logs.table.tooltipValues.pricingUnavailable'),
  priceSourceLabels: {
    provider_api: translate('components.logs.table.priceSourceValues.providerApi'),
    builtin: translate('components.logs.table.priceSourceValues.builtin'),
    none: translate('components.logs.table.priceSourceValues.none'),
  },
})

export const buildLogsTableTextFormatters = (translate: LogsTranslate): LogsTableTextFormatters => ({
  formatStream: (value?: boolean | number) => formatStream(
    value,
    translate('components.logs.streamOn'),
    translate('components.logs.streamOff'),
  ),
  formatPayloadDetailAriaLabel: (item: RequestLog) => translate('components.logs.payloadDetail.openAria', {
    value: formatTokensPerSecond(item),
  }),
  formatModelVerifyStatus: (item: RequestLog) =>
    translate(`components.logs.table.verifyValues.${resolveModelVerifyStatus(item)}`),
  formatModelInfoAriaLabel: (item: RequestLog) =>
    `${translate('components.logs.table.model')}: ${[item.model || '—', normalizeReasoningEffortDisplay(item.reasoning_effort)].filter(Boolean).join(' ')}`,
  formatReasoningEffort: (item: RequestLog) => normalizeReasoningEffortDisplay(item.reasoning_effort),
  formatReasoningEffortTone: (value?: string) => resolveReasoningEffortTone(value),
  formatVerifyInfoAriaLabel: (item: RequestLog) =>
    `${translate('components.logs.table.verify')}: ${translate(`components.logs.table.verifyValues.${resolveModelVerifyStatus(item)}`)}`,
  formatCostAriaLabel: (item: RequestLog) =>
    `${translate('components.logs.table.cost')}: ${formatCurrency(item.total_cost)}`,
})

export const resolveTooltipModelDisplayValue = (
  value: string | null | undefined,
  missingText: string,
): { value: string; tone?: LogInfoTooltipTone } => {
  const normalized = String(value ?? '').trim()
  if (!normalized) {
    return {
      value: missingText,
      tone: 'muted',
    }
  }
  return { value: normalized }
}

export const formatLogPriceSource = (
  source: LogPriceSource,
  labels: LogsInfoTooltipLabels,
) => labels.priceSourceLabels[source]

export const resolveLogInfoTooltipSourceTone = (source: LogPriceSource): LogInfoTooltipTone => {
  if (source === 'provider_api') return 'source-provider-api'
  if (source === 'builtin') return 'source-builtin'
  return 'source-none'
}

export const buildModelInfoTooltipDetailData = ({
  source,
  matchedModel,
  currentModel,
  costDetail,
  recordedCost,
}: {
  source: LogPriceSource
  matchedModel: string
  currentModel: string
  costDetail: CostTooltipDetail
  recordedCost: number
}, labels: LogsInfoTooltipLabels): LogInfoTooltipDetail => {
  const rows: LogInfoTooltipRow[] = [
    {
      key: 'price-source',
      label: labels.pricingSourceLabel,
      value: formatLogPriceSource(source, labels),
      tone: resolveLogInfoTooltipSourceTone(source),
    },
  ]

  if (matchedModel && normalizeModelName(matchedModel) !== normalizeModelName(currentModel)) {
    rows.push({
      key: 'pricing-model',
      label: labels.pricingModelLabel,
      value: matchedModel,
    })
  }

  if (costDetail.priceLines.length > 0) {
    rows.push(
      ...costDetail.priceLines.map((line) => ({
        key: `pricing-line-${line.key}`,
        label: line.label,
        value: line.value,
      })),
    )
  } else {
    rows.push({
      key: 'pricing-line-empty',
      label: labels.pricingDetailLabel,
      value: labels.pricingUnavailableValue,
      tone: 'muted',
    })
  }

  rows.push({
    key: 'pricing-formula',
    label: labels.pricingFormulaLabel,
    value: costDetail.formula,
    tone: costDetail.priceLines.length > 0 ? undefined : 'muted',
  })

  if (costDetail.note) {
    rows.push({
      key: 'pricing-note',
      label: labels.pricingHintLabel,
      value: costDetail.note,
      tone: 'muted',
    })
  }

  rows.push({
    key: 'pricing-recorded-cost',
    label: labels.recordedCostLabel,
    value: formatUsdPrecise(recordedCost),
  })

  return {
    title: labels.modelTitle,
    variant: 'model',
    rows,
  }
}

export const buildVerifyInfoTooltipDetailData = ({
  requestedModel,
  responseModel,
  userAgent,
}: {
  requestedModel?: string | null
  responseModel?: string | null
  userAgent?: string | null
}, labels: LogsInfoTooltipLabels): LogInfoTooltipDetail => {
  const requested = resolveTooltipModelDisplayValue(requestedModel, labels.tooltipValueMissing)
  const response = resolveTooltipModelDisplayValue(responseModel, labels.tooltipValueMissing)
  const ua = resolveTooltipModelDisplayValue(userAgent, labels.tooltipValueMissing)

  return {
    title: labels.verifyTitle,
    variant: 'verify',
    rows: [
      {
        key: 'requested-model',
        label: labels.requestedModelLabel,
        value: requested.value,
        tone: requested.tone,
      },
      {
        key: 'response-model',
        label: labels.responseModelLabel,
        value: response.value,
        tone: response.tone,
      },
      {
        key: 'user-agent',
        label: labels.userAgentLabel,
        value: ua.value,
        tone: ua.tone,
      },
    ],
  }
}

export const buildProviderApiPerCallPriceLines = (
  item: RequestLog,
  labels: ProviderApiPerCallPriceLineLabels,
): CostTooltipPriceLine[] => {
  if (!hasProviderPricingSnapshot(item)) return []
  if (safeNumber(item.provider_quota_type) !== 1) return []

  const hasUnified = isProviderPerCallValueSet(item.provider_per_call_unified, item.provider_per_call_unified_set)
  const hasInput = isProviderPerCallValueSet(item.provider_per_call_input, item.provider_per_call_input_set)
  const hasOutput = isProviderPerCallValueSet(item.provider_per_call_output, item.provider_per_call_output_set)
  const lines: CostTooltipPriceLine[] = []

  if (hasUnified) {
    lines.push({
      key: 'per-call-unified',
      label: labels.perCallUnifiedPrice,
      value: `${formatUsdPrecise(safeNumber(item.provider_per_call_unified))} ${labels.perRequestSuffix}`,
    })
  }

  if (hasInput) {
    lines.push({
      key: 'per-call-input',
      label: labels.perCallInputPrice,
      value: `${formatUsdPrecise(safeNumber(item.provider_per_call_input))} ${labels.perRequestSuffix}`,
    })
  }

  if (hasOutput) {
    lines.push({
      key: 'per-call-output',
      label: labels.perCallOutputPrice,
      value: `${formatUsdPrecise(safeNumber(item.provider_per_call_output))} ${labels.perRequestSuffix}`,
    })
  }

  return lines
}

export const buildLogsCostTooltipLabels = (
  translate: LogsTranslate,
  formatTierLabel: (tier: CacheCreateTier) => string,
): LogsCostTooltipLabels => {
  const formatCacheCreatePriceLabel = (tier?: CacheCreateTier) =>
    tier
      ? translate('components.logs.costTooltip.cacheCreatePriceWithTtl', {
        ttl: formatTierLabel(tier),
      })
      : translate('components.logs.costTooltip.cacheCreatePrice')

  const formatCacheCreateUsageLabel = (tier?: CacheCreateTier) =>
    tier
      ? translate('components.logs.costTooltip.usageCacheCreateWithTtl', {
        ttl: formatTierLabel(tier),
      })
      : translate('components.logs.costTooltip.usageCacheCreate')

  const formatCacheCreateMultiplierLabel = (
    multiplier: number,
    tier?: CacheCreateTier,
  ) =>
    tier
      ? translate('components.logs.costTooltip.cacheCreateMultiplierLabelWithTtl', {
        ttl: formatTierLabel(tier),
        multiplier: formatMultiplierValue(multiplier),
      })
      : translate('components.logs.costTooltip.cacheCreateMultiplierLabel', {
        multiplier: formatMultiplierValue(multiplier),
      })

  const cacheReadMultiplierLabel = (multiplier: number) => translate('components.logs.costTooltip.cacheReadMultiplierLabel', {
    multiplier: formatMultiplierValue(multiplier),
  })
  const groupMultiplierLabel = (multiplier: number) => translate('components.logs.costTooltip.groupMultiplierLabel', {
    multiplier: formatMultiplierValue(multiplier),
  })

  return {
    tokenRatePriceLineLabels: {
      promptPrice: translate('components.logs.costTooltip.promptPrice'),
      completionPrice: translate('components.logs.costTooltip.completionPrice'),
      cacheCreatePriceLabel: formatCacheCreatePriceLabel,
      cacheCreateMultiplierLabel: formatCacheCreateMultiplierLabel,
      cacheReadPrice: translate('components.logs.costTooltip.cacheReadPrice'),
      cacheReadMultiplierLabel,
      reasoningPrice: translate('components.logs.costTooltip.reasoningPrice'),
    },
    providerApiPerCallPriceLineLabels: {
      perCallUnifiedPrice: translate('components.logs.costTooltip.perCallUnifiedPrice'),
      perCallInputPrice: translate('components.logs.costTooltip.perCallInputPrice'),
      perCallOutputPrice: translate('components.logs.costTooltip.perCallOutputPrice'),
      perRequestSuffix: translate('components.logs.costTooltip.perRequestSuffix'),
    },
    tokenFormulaLabels: {
      usagePrompt: translate('components.logs.costTooltip.usagePrompt'),
      usageCompletion: translate('components.logs.costTooltip.usageCompletion'),
      usageReasoning: translate('components.logs.costTooltip.usageReasoning'),
      usageCacheRead: translate('components.logs.costTooltip.usageCacheRead'),
      formulaEmpty: translate('components.logs.costTooltip.formulaEmpty'),
      cacheCreateUsageLabel: formatCacheCreateUsageLabel,
      cacheCreateMultiplierLabel: formatCacheCreateMultiplierLabel,
      cacheReadMultiplierLabel,
      groupMultiplierLabel,
    },
    observedPriceSuffix: translate('components.logs.costTooltip.observedPriceSuffix'),
    providerApiFormula: translate('components.logs.costTooltip.providerApiFormula'),
    providerApiPerCallFormula: translate('components.logs.costTooltip.providerApiPerCallFormula'),
    providerApiHint: translate('components.logs.costTooltip.providerApiHint'),
    providerApiFallbackHint: translate('components.logs.costTooltip.providerApiFallbackHint'),
    providerApiZeroCostHint: translate('components.logs.costTooltip.providerApiZeroCostHint'),
    noPricingFormula: translate('components.logs.costTooltip.noPricingFormula'),
    noPricingHint: translate('components.logs.costTooltip.noPricingHint'),
    recordedCostHint: (cost: string) => translate('components.logs.costTooltip.recordedCostHint', { cost }),
    matchedModelHint: (model: string) => translate('components.logs.costTooltip.matchedModelHint', { model }),
  }
}


export type TokenCostPricingContext = {
  inputTokens: number
  outputTokens: number
  reasoningTokens: number
  cacheReadTokens: number
  inputPerToken: number
  outputPerToken: number
  reasoningPerToken: number
  cacheReadPerToken: number
  cacheCreateDetails: CacheCreateCostDetail[]
  cacheCreateRates: CacheCreatePriceRate[]
  calculatedTotal: number
  cacheReadMultiplier: number
  groupMultiplier: number
}

export type BuiltinTokenPricingContext = TokenCostPricingContext & {
  modelName: string
  matchedModelChanged: boolean
}

const mapCacheCreateRates = (
  details: CacheCreateCostDetail[],
  fallbackPerToken: number,
): CacheCreatePriceRate[] =>
  details.length > 0
    ? details.map(detail => ({
      tier: detail.tier,
      perToken: detail.perToken,
    }))
    : [{ perToken: fallbackPerToken }]

export const buildProviderApiTokenPricingContext = (item: RequestLog): TokenCostPricingContext | null => {
  if (!hasProviderPricingSnapshot(item)) return null
  if (safeNumber(item.provider_quota_type) !== 0) return null

  const cacheCreateSplit = resolveCacheCreateTokenSplit(item)
  const inputTokens = Math.max(0, Math.round(safeNumber(item.input_tokens)))
  const outputTokens = Math.max(0, Math.round(safeNumber(item.output_tokens)))
  const reasoningTokens = Math.max(0, Math.round(safeNumber(item.reasoning_tokens)))
  const cacheCreateTokens = cacheCreateSplit.totalTokens
  const cacheReadTokens = Math.max(0, Math.round(safeNumber(item.cache_read_tokens)))

  const breakdownInputCost = Math.max(0, safeNumber(item.input_cost))
  const breakdownOutputCost = Math.max(0, safeNumber(item.output_cost))
  const breakdownReasoningCost = Math.max(0, safeNumber(item.reasoning_cost))
  const breakdownCacheCreateCost = Math.max(0, safeNumber(item.cache_create_cost))
  const breakdownCacheReadCost = Math.max(0, safeNumber(item.cache_read_cost))
  const breakdownEphemeral5mCost = Math.max(0, safeNumber(item.ephemeral_5m_cost))
  const breakdownEphemeral1hCost = Math.max(0, safeNumber(item.ephemeral_1h_cost))
  const groupMultiplier = resolveGroupMultiplier(item)

  const inputPerTokenSnapshot = Math.max(0, safeNumber(item.provider_input_usd_per_m)) / PER_MILLION_TOKENS
  const outputPerTokenSnapshot = Math.max(0, safeNumber(item.provider_output_usd_per_m)) / PER_MILLION_TOKENS

  const unscaleCost = (cost: number) => (groupMultiplier > 0 ? cost / groupMultiplier : cost)
  const baseInputCost = unscaleCost(breakdownInputCost)
  const baseOutputCost = unscaleCost(breakdownOutputCost)
  const baseReasoningCost = unscaleCost(breakdownReasoningCost)
  const baseCacheCreateCost = unscaleCost(breakdownCacheCreateCost)
  const baseCacheReadCost = unscaleCost(breakdownCacheReadCost)
  const baseEphemeral5mCost = unscaleCost(breakdownEphemeral5mCost)
  const baseEphemeral1hCost = unscaleCost(breakdownEphemeral1hCost)

  const inputPerToken =
    inputPerTokenSnapshot > 0
      ? inputPerTokenSnapshot
      : inputTokens > 0 && baseInputCost > 0
        ? baseInputCost / inputTokens
        : 0
  const outputPerToken =
    outputPerTokenSnapshot > 0
      ? outputPerTokenSnapshot
      : outputTokens > 0 && baseOutputCost > 0
        ? baseOutputCost / outputTokens
        : 0
  const reasoningPerToken =
    reasoningTokens > 0 && baseReasoningCost > 0
      ? baseReasoningCost / reasoningTokens
      : outputPerToken
  const cacheCreateCombinedPerToken =
    cacheCreateTokens > 0 && baseCacheCreateCost > 0
      ? baseCacheCreateCost / cacheCreateTokens
      : inputPerToken
  const cacheReadPerToken =
    cacheReadTokens > 0 && baseCacheReadCost > 0
      ? baseCacheReadCost / cacheReadTokens
      : inputPerToken
  const cacheCreateDetails = buildCacheCreateCostDetails({
    split: cacheCreateSplit,
    totalCost: baseCacheCreateCost,
    ephemeral5mCost: baseEphemeral5mCost,
    ephemeral1hCost: baseEphemeral1hCost,
    fallback5mPerToken: inputPerToken > 0 ? inputPerToken : cacheCreateCombinedPerToken,
    fallback1hPerToken: cacheCreateCombinedPerToken > 0 ? cacheCreateCombinedPerToken : inputPerToken,
    fallbackCombinedPerToken: cacheCreateCombinedPerToken,
  })
  const cacheCreateRates = mapCacheCreateRates(cacheCreateDetails, cacheCreateCombinedPerToken)

  const hasAnyTokenRate =
    inputPerToken > 0 ||
    outputPerToken > 0 ||
    reasoningPerToken > 0 ||
    cacheCreateRates.some(rate => safeNumber(rate.perToken) > 0) ||
    (cacheReadTokens > 0 && cacheReadPerToken > 0)
  if (!hasAnyTokenRate) return null

  const inputCost = inputTokens > 0 && baseInputCost > 0 ? baseInputCost : inputTokens * inputPerToken
  const outputCost = outputTokens > 0 && baseOutputCost > 0 ? baseOutputCost : outputTokens * outputPerToken
  const reasoningCost = reasoningTokens > 0 && baseReasoningCost > 0
    ? baseReasoningCost
    : reasoningTokens * reasoningPerToken
  const cacheCreateCost = cacheCreateDetails.length > 0
    ? cacheCreateDetails.reduce((sum, detail) => sum + detail.cost, 0)
    : cacheCreateTokens > 0 && baseCacheCreateCost > 0
      ? baseCacheCreateCost
      : cacheCreateTokens * cacheCreateCombinedPerToken
  const cacheReadCost = cacheReadTokens > 0 && baseCacheReadCost > 0
    ? baseCacheReadCost
    : cacheReadTokens * cacheReadPerToken

  const calculatedTotal = (inputCost + outputCost + reasoningCost + cacheCreateCost + cacheReadCost) * groupMultiplier
  const cacheReadMultiplier = inputPerToken > 0 ? cacheReadPerToken / inputPerToken : 0

  return {
    inputTokens,
    outputTokens,
    reasoningTokens,
    cacheReadTokens,
    inputPerToken,
    outputPerToken,
    reasoningPerToken,
    cacheReadPerToken,
    cacheCreateDetails,
    cacheCreateRates,
    calculatedTotal,
    cacheReadMultiplier,
    groupMultiplier,
  }
}

export const buildBuiltinTokenPricingContext = (
  item: RequestLog,
  pricingRow: ModelPricingRow,
  fallbackModelName: string,
): BuiltinTokenPricingContext => {
  const modelName = String(pricingRow.model ?? fallbackModelName).trim() || '—'
  const cacheCreateSplit = resolveCacheCreateTokenSplit(item)
  const inputTokens = Math.max(0, Math.round(safeNumber(item.input_tokens)))
  const outputTokens = Math.max(0, Math.round(safeNumber(item.output_tokens)))
  const reasoningTokens = Math.max(0, Math.round(safeNumber(item.reasoning_tokens)))
  const cacheReadTokens = Math.max(0, Math.round(safeNumber(item.cache_read_tokens)))
  const cacheCreateTokens = cacheCreateSplit.totalTokens

  const inputPerTokenBase = Math.max(0, safeNumber(pricingRow.input_cost_per_token))
  const outputPerTokenBase = Math.max(0, safeNumber(pricingRow.output_cost_per_token))
  const reasoningPerTokenBase = Math.max(0, safeNumber(pricingRow.output_cost_per_reasoning_token))
  const cacheCreate5mRaw = Math.max(0, safeNumber(pricingRow.cache_creation_input_token_cost))
  const cacheCreate1hRaw = Math.max(0, safeNumber(pricingRow.ephemeral_1h_cost_per_token))
  const cacheReadRaw = Math.max(0, safeNumber(pricingRow.cache_read_input_token_cost))

  const cacheCreate5mPerTokenBase = cacheCreate5mRaw > 0 ? cacheCreate5mRaw : inputPerTokenBase * 1.25
  const cacheCreate1hPerTokenBase = cacheCreate1hRaw > 0 ? cacheCreate1hRaw : cacheCreate5mPerTokenBase
  const cacheReadPerTokenBase = cacheReadRaw > 0 ? cacheReadRaw : inputPerTokenBase * 0.1

  const breakdownPayload = hasBreakdownCostPayload(item)
  const breakdownInputCost = Math.max(0, safeNumber(item.input_cost))
  const breakdownOutputCost = Math.max(0, safeNumber(item.output_cost))
  const breakdownReasoningCost = Math.max(0, safeNumber(item.reasoning_cost))
  const breakdownCacheCreateCost = Math.max(0, safeNumber(item.cache_create_cost))
  const breakdownCacheReadCost = Math.max(0, safeNumber(item.cache_read_cost))
  const breakdownEphemeral5mCost = Math.max(0, safeNumber(item.ephemeral_5m_cost))
  const breakdownEphemeral1hCost = Math.max(0, safeNumber(item.ephemeral_1h_cost))
  const groupMultiplier = resolveGroupMultiplier(item)
  const unscaleCost = (cost: number) => (groupMultiplier > 0 ? cost / groupMultiplier : cost)
  const baseInputCost = unscaleCost(breakdownInputCost)
  const baseOutputCost = unscaleCost(breakdownOutputCost)
  const baseReasoningCost = unscaleCost(breakdownReasoningCost)
  const baseCacheCreateCost = unscaleCost(breakdownCacheCreateCost)
  const baseCacheReadCost = unscaleCost(breakdownCacheReadCost)
  const baseEphemeral5mCost = unscaleCost(breakdownEphemeral5mCost)
  const baseEphemeral1hCost = unscaleCost(breakdownEphemeral1hCost)
  const cacheCreateFallback5mPerToken = breakdownPayload ? 0 : cacheCreate5mPerTokenBase
  const cacheCreateFallback1hPerToken = breakdownPayload ? 0 : cacheCreate1hPerTokenBase
  const cacheCreateFallbackCombinedPerToken = breakdownPayload ? 0 : cacheCreate5mPerTokenBase

  const inputCost = breakdownPayload ? baseInputCost : inputTokens * inputPerTokenBase
  const outputCost = breakdownPayload ? baseOutputCost : outputTokens * outputPerTokenBase
  const reasoningCost = breakdownPayload ? baseReasoningCost : reasoningTokens * reasoningPerTokenBase
  const cacheCreateDetails = buildCacheCreateCostDetails({
    split: cacheCreateSplit,
    totalCost: breakdownPayload ? baseCacheCreateCost : 0,
    ephemeral5mCost: breakdownPayload ? baseEphemeral5mCost : 0,
    ephemeral1hCost: breakdownPayload ? baseEphemeral1hCost : 0,
    fallback5mPerToken: cacheCreateFallback5mPerToken,
    fallback1hPerToken: cacheCreateFallback1hPerToken,
    fallbackCombinedPerToken: cacheCreateFallbackCombinedPerToken,
  })
  const cacheCreateRates = mapCacheCreateRates(cacheCreateDetails, cacheCreateFallbackCombinedPerToken)
  const cacheCreateCost = cacheCreateDetails.length > 0
    ? cacheCreateDetails.reduce((sum, detail) => sum + detail.cost, 0)
    : breakdownPayload
      ? baseCacheCreateCost
      : cacheCreateTokens * cacheCreate5mPerTokenBase
  const cacheReadCost = breakdownPayload ? baseCacheReadCost : cacheReadTokens * cacheReadPerTokenBase

  const inputPerToken = inputTokens > 0 ? inputCost / inputTokens : inputPerTokenBase
  const outputPerToken = outputTokens > 0 ? outputCost / outputTokens : outputPerTokenBase
  const reasoningPerToken = reasoningTokens > 0 ? reasoningCost / reasoningTokens : reasoningPerTokenBase
  const cacheReadPerToken = cacheReadTokens > 0 ? cacheReadCost / cacheReadTokens : cacheReadPerTokenBase

  const cacheReadMultiplier = inputPerToken > 0 ? cacheReadPerToken / inputPerToken : 0
  const calculatedTotal = (inputCost + cacheCreateCost + cacheReadCost + outputCost + reasoningCost) * groupMultiplier

  const rowModel = modelName.toLowerCase()
  const logModel = String(item.model ?? '').trim().toLowerCase()
  const matchedModelChanged = Boolean(rowModel && logModel && rowModel !== logModel)

  return {
    modelName,
    matchedModelChanged,
    groupMultiplier,
    inputTokens,
    outputTokens,
    reasoningTokens,
    cacheReadTokens,
    inputPerToken,
    outputPerToken,
    reasoningPerToken,
    cacheReadPerToken,
    cacheCreateDetails,
    cacheCreateRates,
    calculatedTotal,
    cacheReadMultiplier,
  }
}

const TOKEN_FORMULA_UNIT = 'tokens / 1M tokens'

const buildTokenCostFormulaResult = (
  formulaParts: string[],
  calculatedTotal: number,
  emptyFormula: string,
  groupMultiplier: number,
  labels: TokenCostFormulaLabels,
) =>
  formulaParts.length > 0
    ? (() => {
      const baseFormula = formulaParts.join(' + ')
      if (groupMultiplier !== 1) {
        return `(${baseFormula}) * ${labels.groupMultiplierLabel(groupMultiplier)} = ${formatUsdPrecise(calculatedTotal)}`
      }
      return `${baseFormula} = ${formatUsdPrecise(calculatedTotal)}`
    })()
    : emptyFormula

export const buildProviderApiTokenFormula = (
  context: TokenCostPricingContext,
  labels: TokenCostFormulaLabels,
  emptyFormula: string,
) => {
  const {
    inputTokens,
    outputTokens,
    reasoningTokens,
    cacheReadTokens,
    inputPerToken,
    outputPerToken,
    reasoningPerToken,
    cacheReadPerToken,
    cacheCreateDetails,
    calculatedTotal,
    cacheReadMultiplier,
    groupMultiplier,
  } = context

  const formulaParts: string[] = []
  if (inputTokens > 0 && inputPerToken > 0) {
    formulaParts.push(
      `${labels.usagePrompt} ${formatTokenFormulaValue(inputTokens)} ${TOKEN_FORMULA_UNIT} * ${formatUsdPerMillion(inputPerToken)}`,
    )
  }

  cacheCreateDetails
    .filter(detail => detail.tokens > 0 && detail.perToken > 0)
    .forEach((detail) => {
      const multiplier = inputPerToken > 0 ? detail.perToken / inputPerToken : 0
      const multiplierSuffix = multiplier > 0
        ? ` (${labels.cacheCreateMultiplierLabel(multiplier, detail.tier)})`
        : ''
      formulaParts.push(
        `${labels.cacheCreateUsageLabel(detail.tier)} ${formatTokenFormulaValue(detail.tokens)} ${TOKEN_FORMULA_UNIT} * ${formatUsdPerMillion(detail.perToken)}${multiplierSuffix}`,
      )
    })

  if (cacheReadTokens > 0 && cacheReadPerToken > 0) {
    const multiplierSuffix = cacheReadMultiplier > 0
      ? ` (${labels.cacheReadMultiplierLabel(cacheReadMultiplier)})`
      : ''
    formulaParts.push(
      `${labels.usageCacheRead} ${formatTokenFormulaValue(cacheReadTokens)} ${TOKEN_FORMULA_UNIT} * ${formatUsdPerMillion(cacheReadPerToken)}${multiplierSuffix}`,
    )
  }

  if (outputTokens > 0 && outputPerToken > 0) {
    formulaParts.push(
      `${labels.usageCompletion} ${formatTokenFormulaValue(outputTokens)} ${TOKEN_FORMULA_UNIT} * ${formatUsdPerMillion(outputPerToken)}`,
    )
  }

  if (reasoningTokens > 0 && reasoningPerToken > 0) {
    formulaParts.push(
      `${labels.usageReasoning} ${formatTokenFormulaValue(reasoningTokens)} ${TOKEN_FORMULA_UNIT} * ${formatUsdPerMillion(reasoningPerToken)}`,
    )
  }

  return buildTokenCostFormulaResult(formulaParts, calculatedTotal, emptyFormula, groupMultiplier, labels)
}

export const buildBuiltinTokenFormula = (
  context: BuiltinTokenPricingContext,
  labels: TokenCostFormulaLabels,
) => {
  const {
    inputTokens,
    outputTokens,
    reasoningTokens,
    cacheReadTokens,
    inputPerToken,
    outputPerToken,
    reasoningPerToken,
    cacheReadPerToken,
    cacheCreateDetails,
    calculatedTotal,
    cacheReadMultiplier,
    groupMultiplier,
  } = context

  const formulaParts: string[] = []
  if (inputTokens > 0 && inputPerToken > 0) {
    formulaParts.push(
      `${labels.usagePrompt} ${formatTokenFormulaValue(inputTokens)} ${TOKEN_FORMULA_UNIT} * ${formatUsdPerMillion(inputPerToken)}`,
    )
  }

  cacheCreateDetails
    .filter(detail => detail.tokens > 0 && detail.perToken > 0)
    .forEach((detail) => {
      const multiplier = inputPerToken > 0 ? detail.perToken / inputPerToken : 0
      const multiplierSuffix = multiplier > 0
        ? ` (${labels.cacheCreateMultiplierLabel(multiplier, detail.tier)})`
        : ''
      formulaParts.push(
        `${labels.cacheCreateUsageLabel(detail.tier)} ${formatTokenFormulaValue(detail.tokens)} ${TOKEN_FORMULA_UNIT} * ${formatUsdPerMillion(detail.perToken)}${multiplierSuffix}`,
      )
    })

  if (cacheReadTokens > 0 && cacheReadPerToken > 0) {
    formulaParts.push(
      `${labels.usageCacheRead} ${formatTokenFormulaValue(cacheReadTokens)} ${TOKEN_FORMULA_UNIT} * ${formatUsdPerMillion(cacheReadPerToken)} (${labels.cacheReadMultiplierLabel(cacheReadMultiplier)})`,
    )
  }

  if (outputTokens > 0 && outputPerToken > 0) {
    formulaParts.push(
      `${labels.usageCompletion} ${formatTokenFormulaValue(outputTokens)} ${TOKEN_FORMULA_UNIT} * ${formatUsdPerMillion(outputPerToken)}`,
    )
  }

  if (reasoningTokens > 0 && reasoningPerToken > 0) {
    formulaParts.push(
      `${labels.usageReasoning} ${formatTokenFormulaValue(reasoningTokens)} ${TOKEN_FORMULA_UNIT} * ${formatUsdPerMillion(reasoningPerToken)}`,
    )
  }

  return buildTokenCostFormulaResult(formulaParts, calculatedTotal, labels.formulaEmpty, groupMultiplier, labels)
}


export const formatTokenNumber = (value?: number) => {
  if (value === undefined || value === null) return '—'
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  }
  if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  }
  if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}k`
  }
  return value.toLocaleString()
}

export const formatTokenFormulaValue = (value?: number) => {
  const compact = formatTokenNumber(value)
  if (compact === '—') return '0'
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric) || Math.abs(numeric) < 1_000) return compact
  const exact = Number.isInteger(numeric)
    ? numeric.toLocaleString()
    : numeric.toLocaleString(undefined, { maximumFractionDigits: 2 })
  return `${compact} (${exact})`
}

export const formatCacheHitRate = (cacheRead?: number, inputTokens?: number) => {
  const read = cacheRead ?? 0
  const input = inputTokens ?? 0
  const total = read + input
  if (total === 0) return '0%'
  const rate = (read / total) * 100
  return `${rate.toFixed(1)}%`
}

export const formatCurrency = (value?: number) => {
  if (value === undefined || value === null || Number.isNaN(value)) {
    return '$0.0000'
  }
  if (value >= 1) {
    return `$${value.toFixed(2)}`
  }
  if (value >= 0.01) {
    return `$${value.toFixed(3)}`
  }
  return `$${value.toFixed(4)}`
}

export const formatCurrencyParts = (value?: number) => {
  const formatted = formatCurrency(value)
  const normalized = formatted.startsWith('$') ? formatted.slice(1) : formatted
  const [whole = '0', fraction = ''] = normalized.split('.')
  return {
    symbol: '$',
    whole,
    fraction,
    formatted,
  }
}

export const hasProviderPricingSnapshot = (item: RequestLog) =>
  isTrueFlag(item.provider_pricing_available)

export const normalizeModelShareKey = (value?: string) => String(value ?? '').trim().toLowerCase()

export const buildModelShareRows = (
  modelStats: ModelUsageStat[],
  colors: readonly string[],
): ModelShareRow[] => {
  const grouped = new Map<string, { model: string; requests: number; tokens: number; cost: number }>()
  for (const item of modelStats) {
    const model = String(item.model ?? '').trim() || '—'
    const normalizedKey = normalizeModelShareKey(model) || '—'
    const requests = Math.max(0, Math.round(safeNumber(item.total_requests)))
    const fallbackTokens = safeNumber(item.input_tokens) + safeNumber(item.output_tokens) + safeNumber(item.cache_read_tokens)
    const tokens = Math.max(0, Math.round(safeNumber(item.total_tokens) || fallbackTokens))
    const cost = safeNumber(item.cost_total)

    const current = grouped.get(normalizedKey) ?? { model, requests: 0, tokens: 0, cost: 0 }
    if (current.model === '—' && model !== '—') {
      current.model = model
    }
    current.requests += requests
    current.tokens += tokens
    current.cost += cost
    grouped.set(normalizedKey, current)
  }

  const rows = Array.from(grouped.values()).sort((a, b) => {
    if (b.requests !== a.requests) return b.requests - a.requests
    if (b.tokens !== a.tokens) return b.tokens - a.tokens
    return b.cost - a.cost
  })

  return rows.map((item, index) => ({
    ...item,
    color: colors[index % colors.length] ?? '#94a3b8',
  }))
}

export const formatModelShareTooltipLabel = (
  label: string | number | undefined,
  rawValue: unknown,
  totalRequests: number,
  requestUnitLabel: string = 'req',
) => {
  const requestValue = Math.max(0, Math.round(Number(rawValue ?? 0)))
  const ratio = totalRequests > 0 ? (requestValue / totalRequests) * 100 : 0
  const resolvedLabel = String(label ?? '').trim() || '—'
  const resolvedUnit = String(requestUnitLabel ?? '').trim()
  const valueLabel = resolvedUnit
    ? `${formatNumber(requestValue)} ${resolvedUnit}`
    : formatNumber(requestValue)
  return `${resolvedLabel}: ${valueLabel} (${ratio.toFixed(1)}%)`
}

const hexToRgb = (hexColor: string) => {
  const normalized = String(hexColor ?? '').trim().replace('#', '')
  if (normalized.length !== 6) return null
  const red = Number.parseInt(normalized.slice(0, 2), 16)
  const green = Number.parseInt(normalized.slice(2, 4), 16)
  const blue = Number.parseInt(normalized.slice(4, 6), 16)
  if ([red, green, blue].some(channel => Number.isNaN(channel))) return null
  return { red, green, blue }
}

export const buildAlphaColor = (hexColor: string, alpha: number) => {
  const normalizedAlpha = Number.isFinite(alpha)
    ? Math.max(0, Math.min(0.5, alpha))
    : 0
  const rgb = hexToRgb(hexColor)
  if (!rgb) return `rgba(148, 163, 184, ${normalizedAlpha})`
  return `rgba(${rgb.red}, ${rgb.green}, ${rgb.blue}, ${normalizedAlpha})`
}

export type ModelVerifyStatus = 'match' | 'mismatch' | 'unknown'

export const normalizeModelName = (value?: string) => String(value ?? '').trim().toLowerCase()

export const resolveModelVerifyStatus = (item: RequestLog): ModelVerifyStatus => {
  const requestedModel = normalizeModelName(item.requested_model)
  const responseModel = normalizeModelName(item.response_model)
  if (!requestedModel || !responseModel) return 'unknown'
  return requestedModel === responseModel ? 'match' : 'mismatch'
}

export type LogPriceSource = 'provider_api' | 'builtin' | 'none'

export const resolvePriceSource = (item: RequestLog): LogPriceSource => {
  const source = String(item.price_source ?? '').trim().toLowerCase()
  if (source === 'provider_api') return 'provider_api'
  if (source === 'builtin') return 'builtin'
  if (source === 'none') return hasStoredCostSnapshot(item) ? 'builtin' : 'none'
  if (hasStoredCostSnapshot(item)) return 'builtin'
  return 'none'
}

export const priceSourceClass = (item: RequestLog) => {
  const source = resolvePriceSource(item)
  if (source === 'provider_api') return 'provider-api'
  return source
}

export const resolveGroupMultiplier = (item: RequestLog) => {
  const candidate = (item as RequestLog & { group_multiplier?: number }).group_multiplier
  if (typeof candidate !== 'number' || !Number.isFinite(candidate) || candidate < 0) return 1
  return candidate
}

export const hasNonZeroCostValue = (value?: number) => safeNumber(value) !== 0

export const hasBreakdownCostPayload = (item: CostSnapshotPayload) =>
  [
    item.input_cost,
    item.output_cost,
    item.reasoning_cost,
    item.cache_create_cost,
    item.cache_read_cost,
    item.ephemeral_5m_cost,
    item.ephemeral_1h_cost,
  ]
    .some(hasNonZeroCostValue)

export const hasStoredCostSnapshot = (item: CostSnapshotPayload) =>
  hasNonZeroCostValue(item.total_cost) || hasBreakdownCostPayload(item) || isTrueFlag(item.has_pricing)

export const mergeCostTooltipNotes = (...notes: Array<string | undefined>) =>
  notes
    .map(note => String(note ?? '').trim())
    .filter(note => note.length > 0)
    .join(' ')

export const buildLineAreaGradient = (chart: Chart<'line'>, hexColor: string, alpha = 0.28) => {
  const area = chart.chartArea
  if (!area) return buildAlphaColor(hexColor, alpha)
  const gradient = chart.ctx.createLinearGradient(0, area.top, 0, area.bottom)
  gradient.addColorStop(0, buildAlphaColor(hexColor, alpha))
  gradient.addColorStop(1, buildAlphaColor(hexColor, 0))
  return gradient
}

export const resolveChartLegendColor = (isDarkTheme: boolean) =>
  isDarkTheme ? '#e2e8f0' : '#0f172a'

export const resolveChartTickColor = (isDarkTheme: boolean) =>
  isDarkTheme ? '#94a3b8' : '#64748b'

export const formatSeriesLabel = (value: string | undefined, granularity: 'day' | 'hour') => {
  if (!value) return ''
  const parsed = parseLogDate(value)
  if (parsed) {
    if (granularity === 'day') {
      return `${padHour(parsed.getMonth() + 1)}-${padHour(parsed.getDate())}`
    }
    return `${padHour(parsed.getHours())}:00`
  }
  const match = value.match(/(\d{2}):(\d{2})/)
  if (match) {
    return `${match[1]}:${match[2]}`
  }
  return value
}

export const formatNumber = (value?: number) => {
  if (value === undefined || value === null) return '—'
  return value.toLocaleString()
}

export const intensityClass = (value: number) => `gh-level-${value}`

export { COST_TOOLTIP_DIFF_EPSILON, PER_MILLION_TOKENS }
