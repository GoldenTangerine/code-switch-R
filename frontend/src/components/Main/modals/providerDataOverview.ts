import type { LogStats, LogStatsSeries } from '../../../services/logs'

const DAY_MS = 24 * 60 * 60 * 1000

const toSafeNumber = (value: unknown) => {
  const numeric = Number(value ?? 0)
  return Number.isFinite(numeric) ? numeric : 0
}

const startOfDay = (value: Date) => new Date(value.getFullYear(), value.getMonth(), value.getDate(), 0, 0, 0, 0)

const parseDay = (value: string) => {
  const raw = String(value ?? '').trim()
  if (!raw) return Number.NaN
  const timestamp = Date.parse(raw.includes('T') ? raw : `${raw}T00:00:00`)
  return Number.isFinite(timestamp) ? timestamp : Number.NaN
}

const formatLocalDateTime = (date: Date) => {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  const hours = `${date.getHours()}`.padStart(2, '0')
  const minutes = `${date.getMinutes()}`.padStart(2, '0')
  const seconds = `${date.getSeconds()}`.padStart(2, '0')
  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
}

const formatDayKey = (date: Date) => formatLocalDateTime(startOfDay(date)).slice(0, 10)

const formatDayLabel = (date: Date) => `${date.getMonth() + 1}/${date.getDate()}`

export type ProviderOverviewDayPoint = {
  dayKey: string
  label: string
  timestamp: number
  cost: number
  requests: number
  totalTokens: number
}

export const sumLogStatsSeriesTokens = (
  stats: Pick<LogStatsSeries, 'input_tokens' | 'output_tokens' | 'reasoning_tokens' | 'cache_create_tokens' | 'cache_read_tokens'>,
) => (
  toSafeNumber(stats.input_tokens)
  + toSafeNumber(stats.output_tokens)
  + toSafeNumber(stats.reasoning_tokens)
  + toSafeNumber(stats.cache_create_tokens)
  + toSafeNumber(stats.cache_read_tokens)
)

export const sumLogStatsTokens = (
  stats: Pick<LogStats, 'input_tokens' | 'output_tokens' | 'reasoning_tokens' | 'cache_create_tokens' | 'cache_read_tokens'> | null | undefined,
) => {
  if (!stats) return 0
  return (
    toSafeNumber(stats.input_tokens)
    + toSafeNumber(stats.output_tokens)
    + toSafeNumber(stats.reasoning_tokens)
    + toSafeNumber(stats.cache_create_tokens)
    + toSafeNumber(stats.cache_read_tokens)
  )
}

export const buildProviderOverviewDays = ({
  series,
  startDate,
  days,
}: {
  series: LogStatsSeries[]
  startDate: Date
  days: number
}): ProviderOverviewDayPoint[] => {
  const normalizedStart = startOfDay(startDate)
  const byDay = new Map<string, ProviderOverviewDayPoint>()

  ;(series ?? []).forEach((entry) => {
    const timestamp = parseDay(entry.day)
    if (!Number.isFinite(timestamp)) return
    const date = new Date(timestamp)
    const dayKey = formatDayKey(date)
    byDay.set(dayKey, {
      dayKey,
      label: formatDayLabel(date),
      timestamp,
      cost: toSafeNumber(entry.total_cost),
      requests: toSafeNumber(entry.total_requests),
      totalTokens: sumLogStatsSeriesTokens(entry),
    })
  })

  return Array.from({ length: Math.max(days, 0) }, (_, index) => {
    const date = new Date(normalizedStart.getTime() + index * DAY_MS)
    const dayKey = formatDayKey(date)
    return byDay.get(dayKey) ?? {
      dayKey,
      label: formatDayLabel(date),
      timestamp: date.getTime(),
      cost: 0,
      requests: 0,
      totalTokens: 0,
    }
  })
}

export const buildProviderOverviewFallbackRows = (
  points: ProviderOverviewDayPoint[],
  maxItems = points.length,
) => {
  const limit = Math.max(Math.floor(maxItems), 0)
  if (limit === 0) return []

  return [...(points ?? [])]
    .sort((left, right) => right.timestamp - left.timestamp)
    .slice(0, limit)
}

export const buildProviderOverviewRange = (days: number, now = new Date()) => {
  const end = new Date(startOfDay(now))
  end.setDate(end.getDate() + 1)

  const start = new Date(end)
  start.setDate(start.getDate() - Math.max(days, 1))

  return {
    start,
    end,
    startAt: formatLocalDateTime(start),
    endAt: formatLocalDateTime(end),
  }
}
