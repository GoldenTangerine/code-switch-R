import type { RequestLog } from '../../../services/logs'

export type ProviderCostTrendPoint = {
  time: string
  cost: number
  cumulativeCost: number
  timestamp: number
}

type ParsedColor = {
  r: number
  g: number
  b: number
  a: number
}

const toFiniteNumber = (value: unknown) => {
  const numeric = Number(value ?? 0)
  return Number.isFinite(numeric) ? numeric : 0
}

const clampChannel = (value: number) => Math.min(255, Math.max(0, Math.round(value)))
const clampAlpha = (value: number) => Math.min(1, Math.max(0, value))

const parseCreatedAt = (value: string) => {
  const raw = String(value ?? '').trim()
  if (!raw) return Number.NaN

  const candidates = raw.includes('T')
    ? [raw, raw.replace(/-/g, '/')]
    : [raw.replace(' ', 'T'), raw, raw.replace(/-/g, '/')]

  for (const candidate of candidates) {
    const timestamp = Date.parse(candidate)
    if (Number.isFinite(timestamp)) return timestamp
  }

  return Number.NaN
}

const expandHex = (value: string) => value.split('').map((char) => `${char}${char}`).join('')

const parseHexColor = (value: string): ParsedColor | null => {
  const raw = String(value ?? '').trim()
  const shortMatch = raw.match(/^#([\da-f]{3,4})$/i)
  if (shortMatch) {
    return parseHexColor(`#${expandHex(shortMatch[1])}`)
  }

  const longMatch = raw.match(/^#([\da-f]{6}|[\da-f]{8})$/i)
  if (!longMatch) return null

  const hex = longMatch[1]
  return {
    r: Number.parseInt(hex.slice(0, 2), 16),
    g: Number.parseInt(hex.slice(2, 4), 16),
    b: Number.parseInt(hex.slice(4, 6), 16),
    a: hex.length === 8 ? Number.parseInt(hex.slice(6, 8), 16) / 255 : 1,
  }
}

const parseRgbColor = (value: string): ParsedColor | null => {
  const match = String(value ?? '').trim().match(/^rgba?\(([^)]+)\)$/i)
  if (!match) return null

  const segments = match[1].split(',').map((segment) => segment.trim())
  if (segments.length < 3) return null

  const r = clampChannel(Number.parseFloat(segments[0]))
  const g = clampChannel(Number.parseFloat(segments[1]))
  const b = clampChannel(Number.parseFloat(segments[2]))
  const a = segments[3] == null ? 1 : clampAlpha(Number.parseFloat(segments[3]))
  if (![r, g, b, a].every((item) => Number.isFinite(item))) return null

  return { r, g, b, a }
}

const parseChartColor = (value: string) => parseHexColor(value) ?? parseRgbColor(value)

export const toChartRgba = (color: string, alpha: number, fallback = '#2563eb') => {
  const parsed = parseChartColor(color) ?? parseChartColor(fallback) ?? { r: 37, g: 99, b: 235, a: 1 }
  const resolvedAlpha = Number((clampAlpha(parsed.a * clampAlpha(alpha))).toFixed(4))
  return `rgba(${parsed.r}, ${parsed.g}, ${parsed.b}, ${resolvedAlpha})`
}

export const buildProviderCostTrend = (logs: RequestLog[]): ProviderCostTrendPoint[] => {
  const normalized = (logs ?? [])
    .map((log) => ({
      time: String(log.created_at ?? '').trim(),
      cost: toFiniteNumber(log.total_cost),
      timestamp: parseCreatedAt(String(log.created_at ?? '')),
    }))
    .filter((item) => item.time)
    .sort((left, right) => {
      const leftValid = Number.isFinite(left.timestamp)
      const rightValid = Number.isFinite(right.timestamp)
      if (leftValid && rightValid) return left.timestamp - right.timestamp
      if (leftValid) return -1
      if (rightValid) return 1
      return left.time.localeCompare(right.time)
    })

  let cumulative = 0

  return normalized.map((item) => {
    cumulative += item.cost
    return {
      time: item.time,
      cost: item.cost,
      cumulativeCost: Number(cumulative.toFixed(6)),
      timestamp: item.timestamp,
    }
  })
}
