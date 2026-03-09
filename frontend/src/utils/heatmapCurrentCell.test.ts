import { describe, expect, it } from 'vitest'
import {
  buildHeatmapCellMatchKey,
  buildHeatmapCurrentCellMatchKey,
  getMillisecondsUntilNextHeatmapBoundary,
} from './heatmapCurrentCell'

const withTimezone = (timezone: string, run: () => void) => {
  const previousTimezone = process.env.TZ
  process.env.TZ = timezone
  try {
    run()
  } finally {
    if (previousTimezone === undefined) {
      delete process.env.TZ
      return
    }
    process.env.TZ = previousTimezone
  }
}

describe('heatmapCurrentCell', () => {
  it('builds local match keys from stored ISO date keys', () => {
    withTimezone('Asia/Shanghai', () => {
      expect(buildHeatmapCellMatchKey('2026-03-09T23:00:00.000Z', 'daily')).toBe('2026-03-10')
      expect(buildHeatmapCellMatchKey('2026-03-09T23:00:00.000Z', 'hourly')).toBe('2026-03-10 07')
    })
  })

  it('builds current match keys from local time without reparsing every cell', () => {
    const now = new Date(2026, 2, 9, 15, 45, 30, 250)

    expect(buildHeatmapCurrentCellMatchKey('daily', now)).toBe('2026-03-09')
    expect(buildHeatmapCurrentCellMatchKey('hourly', now)).toBe('2026-03-09 15')
  })

  it('calculates the next hourly boundary precisely', () => {
    const now = new Date(2026, 2, 9, 15, 45, 30, 250)

    expect(getMillisecondsUntilNextHeatmapBoundary('hourly', now)).toBe(869_750)
  })

  it('calculates the next daily boundary precisely', () => {
    const now = new Date(2026, 2, 9, 23, 59, 59, 900)

    expect(getMillisecondsUntilNextHeatmapBoundary('daily', now)).toBe(100)
  })

  it('returns safe fallbacks for invalid dates', () => {
    expect(buildHeatmapCellMatchKey('not-a-date', 'daily')).toBe('')
    expect(buildHeatmapCurrentCellMatchKey('hourly', Number.NaN)).toBe('')
    expect(getMillisecondsUntilNextHeatmapBoundary('daily', Number.NaN)).toBe(60_000)
  })
})
