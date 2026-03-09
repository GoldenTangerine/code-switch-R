import type { HeatmapGranularity } from '../services/appSettings'

const FALLBACK_REFRESH_DELAY_MS = 60_000

const padNumber = (value: number) => `${value}`.padStart(2, '0')

const toValidDate = (value: Date | number | string) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return null
  }
  return date
}

const buildLocalDateMatchKey = (date: Date) =>
  `${date.getFullYear()}-${padNumber(date.getMonth() + 1)}-${padNumber(date.getDate())}`

const buildLocalHourMatchKey = (date: Date) =>
  `${buildLocalDateMatchKey(date)} ${padNumber(date.getHours())}`

export const buildHeatmapCellMatchKey = (
  dateKey: string,
  granularity: HeatmapGranularity,
) => {
  const cellDate = toValidDate(dateKey)
  if (!cellDate) {
    return ''
  }
  return granularity === 'daily'
    ? buildLocalDateMatchKey(cellDate)
    : buildLocalHourMatchKey(cellDate)
}

export const buildHeatmapCurrentCellMatchKey = (
  granularity: HeatmapGranularity,
  referenceTime: Date | number = Date.now(),
) => {
  const currentDate = toValidDate(referenceTime)
  if (!currentDate) {
    return ''
  }
  return granularity === 'daily'
    ? buildLocalDateMatchKey(currentDate)
    : buildLocalHourMatchKey(currentDate)
}

// 当前格子只需要在粒度边界刷新：小时模式等到整点，天模式等到次日零点。
export const getMillisecondsUntilNextHeatmapBoundary = (
  granularity: HeatmapGranularity,
  referenceTime: Date | number = Date.now(),
) => {
  const currentDate = toValidDate(referenceTime)
  if (!currentDate) {
    return FALLBACK_REFRESH_DELAY_MS
  }

  const nextBoundary = new Date(currentDate)
  if (granularity === 'daily') {
    nextBoundary.setHours(24, 0, 0, 0)
  } else {
    nextBoundary.setHours(currentDate.getHours() + 1, 0, 0, 0)
  }

  return Math.max(1, nextBoundary.getTime() - currentDate.getTime())
}
