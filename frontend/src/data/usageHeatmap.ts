import type { HeatmapStat } from '../services/logs'
import {
	DEFAULT_HEATMAP_DISPLAY_SETTINGS,
	normalizeHeatmapDisplaySettings,
	type HeatmapDisplaySettings,
	type HeatmapIntensityMetric,
} from './heatmapDisplaySettings'

export type HeatmapGranularity = 'hourly' | 'daily'

export type UsageHeatmapDay = {
	label: string
	dateKey: string
	requests: number
	inputTokens: number
	outputTokens: number
	totalTokens: number
	reasoningTokens: number
	cost: number
	intensity: number
	intensityValue: number
	intensityPeakValue: number
}

export type UsageHeatmapWeek = UsageHeatmapDay[]

export const HEATMAP_ROWS = 8
export const BUCKETS_PER_DAY = 3
export const DEFAULT_HEATMAP_DAYS = 21
const DAYS_PER_WEEK = 7
const LEVELS = 4

const clampDays = (days?: number) => (days && days > 0 ? Math.floor(days) : DEFAULT_HEATMAP_DAYS)
const normalizeGranularity = (granularity?: HeatmapGranularity): HeatmapGranularity =>
	granularity === 'daily' ? 'daily' : 'hourly'

const intensityForValue = (
	value: number,
	maxValue: number,
	displaySettings: HeatmapDisplaySettings,
) => {
	if (value <= 0 || maxValue <= 0) return 0
	const ratioPercent = (value / maxValue) * 100
	if (ratioPercent <= displaySettings.intensityStopL1) return 1
	if (ratioPercent <= displaySettings.intensityStopL2) return 2
	if (ratioPercent <= displaySettings.intensityStopL3) return 3
	return LEVELS
}

const startOfDay = (date: Date) => {
	const start = new Date(date)
	start.setHours(0, 0, 0, 0)
	return start
}

const addDays = (date: Date, days: number) => {
	const result = new Date(date)
	result.setDate(result.getDate() + days)
	return result
}

const startOfWeekMonday = (date: Date) => {
	const dayStart = startOfDay(date)
	const dayOfWeek = dayStart.getDay()
	const distanceToMonday = (dayOfWeek + 6) % 7
	return addDays(dayStart, -distanceToMonday)
}

const formatHourKey = (date: Date) => {
	const year = date.getFullYear()
	const month = `${date.getMonth() + 1}`.padStart(2, '0')
	const day = `${date.getDate()}`.padStart(2, '0')
	const hour = `${date.getHours()}`.padStart(2, '0')
	return `${year}-${month}-${day} ${hour}`
}

const formatDayKey = (date: Date) => {
	const year = date.getFullYear()
	const month = `${date.getMonth() + 1}`.padStart(2, '0')
	const day = `${date.getDate()}`.padStart(2, '0')
	return `${year}-${month}-${day}`
}

const labelForCell = (date: Date) => {
	const month = `${date.getMonth() + 1}`.padStart(2, '0')
	const day = `${date.getDate()}`.padStart(2, '0')
	const hour = `${date.getHours()}`.padStart(2, '0')
	return `${month}-${day} ${hour}`
}

const labelForDay = (date: Date) => {
	const month = `${date.getMonth() + 1}`.padStart(2, '0')
	const day = `${date.getDate()}`.padStart(2, '0')
	return `${month}-${day}`
}

const normalizeStatKey = (value?: string | null) => {
	const trimmed = value?.trim()
	if (!trimmed) return null
	const fullMatch = trimmed.match(/^(\d{4})-(\d{2})-(\d{2}) (\d{2})$/)
	if (fullMatch) {
		const [, yearStr, monthStr, dayStr, hourStr] = fullMatch
		return `${yearStr}-${monthStr}-${dayStr} ${hourStr}`
	}
	const match = trimmed.match(/^(\d{2})-(\d{2}) (\d{2})$/)
	if (!match) {
		return null
	}
	const [, monthStr, dayStr, hourStr] = match
	const now = new Date()
	const year = now.getFullYear()
	return `${year}-${monthStr}-${dayStr} ${hourStr}`
}

type StatBucket = {
	requests: number
	inputTokens: number
	outputTokens: number
	cacheReadTokens: number
	reasoningTokens: number
	totalTokens: number
	cost: number
}

const emptyBucket = (): StatBucket => ({
	requests: 0,
	inputTokens: 0,
	outputTokens: 0,
	cacheReadTokens: 0,
	reasoningTokens: 0,
	totalTokens: 0,
	cost: 0,
})

const totalTokensForStat = (stat: HeatmapStat) => {
	if (stat?.total_tokens !== undefined && stat?.total_tokens !== null) {
		const explicitTotal = Number(stat.total_tokens)
		if (Number.isFinite(explicitTotal)) {
			return Math.max(0, explicitTotal)
		}
	}
	const inputTokens = Number(stat?.input_tokens) || 0
	const outputTokens = Number(stat?.output_tokens) || 0
	const cacheReadTokens = Number(stat?.cache_read_tokens) || 0
	return inputTokens + outputTokens + cacheReadTokens
}

const totalTokensForBucket = (bucket: StatBucket) => bucket.totalTokens

const intensityMetricValueForBucket = (
	bucket: StatBucket,
	metric: HeatmapIntensityMetric,
) => {
	switch (metric) {
		case 'cost':
			return bucket.cost
		case 'total_tokens':
			return totalTokensForBucket(bucket)
		case 'input_tokens':
			return bucket.inputTokens
		case 'output_tokens':
			return bucket.outputTokens
		case 'reasoning_tokens':
			return bucket.reasoningTokens
		case 'requests':
		default:
			return bucket.requests
	}
}

const buildUsageHeatmapDay = (
	label: string,
	dateKey: string,
	bucket: StatBucket,
	intensityMetric: HeatmapIntensityMetric,
	maxValue: number,
	displaySettings: HeatmapDisplaySettings,
): UsageHeatmapDay => {
	const intensityValue = intensityMetricValueForBucket(bucket, intensityMetric)
	return {
		label,
		dateKey,
		requests: bucket.requests,
		inputTokens: bucket.inputTokens,
		outputTokens: bucket.outputTokens,
		totalTokens: bucket.totalTokens,
		reasoningTokens: bucket.reasoningTokens,
		cost: bucket.cost,
		intensity: intensityForValue(intensityValue, maxValue, displaySettings),
		intensityValue,
		intensityPeakValue: maxValue,
	}
}

const buildHourlyColumns = (
	days: number,
	statsMap: Map<string, StatBucket>,
	startDay: Date,
	maxValue: number,
	displaySettings: HeatmapDisplaySettings,
) => {
	const columns: UsageHeatmapWeek[] = []
	const intensityMetric = displaySettings.intensityMetric
	for (let dayIndex = 0; dayIndex < days; dayIndex++) {
		const dayStart = addDays(startDay, dayIndex)
		for (let bucketIndex = 0; bucketIndex < BUCKETS_PER_DAY; bucketIndex++) {
			const column: UsageHeatmapWeek = []
			for (let rowIndex = 0; rowIndex < HEATMAP_ROWS; rowIndex++) {
				const hour = bucketIndex * HEATMAP_ROWS + rowIndex
				const cellTime = new Date(dayStart)
				cellTime.setHours(hour, 0, 0, 0)
				const key = formatHourKey(cellTime)
				const bucket = statsMap.get(key) ?? emptyBucket()
				column.push(
					buildUsageHeatmapDay(
						labelForCell(cellTime),
						cellTime.toISOString(),
						bucket,
						intensityMetric,
						maxValue,
						displaySettings,
					),
				)
			}
			columns.push(column)
		}
	}
	return columns
}

const buildDailyColumns = (
	days: number,
	dailyStatsMap: Map<string, StatBucket>,
	startDay: Date,
	maxValue: number,
	displaySettings: HeatmapDisplaySettings,
) => {
	const columns: UsageHeatmapWeek[] = []
	const rangeStart = startOfDay(startDay)
	const rangeEnd = addDays(rangeStart, days - 1)
	const firstWeekStart = startOfWeekMonday(rangeStart)
	const lastWeekStart = startOfWeekMonday(rangeEnd)
	const intensityMetric = displaySettings.intensityMetric

	for (
		let weekStart = firstWeekStart;
		weekStart <= lastWeekStart;
		weekStart = addDays(weekStart, DAYS_PER_WEEK)
	) {
		const column: UsageHeatmapWeek = []
		for (let dayOffset = 0; dayOffset < DAYS_PER_WEEK; dayOffset++) {
			const dayStart = addDays(weekStart, dayOffset)
			const inRange = dayStart >= rangeStart && dayStart <= rangeEnd
			const key = formatDayKey(dayStart)
			const bucket = inRange ? (dailyStatsMap.get(key) ?? emptyBucket()) : emptyBucket()
			column.push(
				buildUsageHeatmapDay(
					labelForDay(dayStart),
					dayStart.toISOString(),
					bucket,
					intensityMetric,
					maxValue,
					displaySettings,
				),
			)
		}
		columns.push(column)
	}
	return columns
}

export const generateFallbackUsageHeatmap = (
	days = DEFAULT_HEATMAP_DAYS,
	granularity: HeatmapGranularity = 'hourly',
	displaySettings: HeatmapDisplaySettings = DEFAULT_HEATMAP_DISPLAY_SETTINGS,
): UsageHeatmapWeek[] => {
	const normalizedDays = clampDays(days)
	const startDay = addDays(startOfDay(new Date()), -(normalizedDays - 1))
	const normalizedGranularity = normalizeGranularity(granularity)
	const normalizedDisplaySettings = normalizeHeatmapDisplaySettings(displaySettings)
	if (normalizedGranularity === 'daily') {
		const dailyStatsMap = new Map<string, StatBucket>()
		return buildDailyColumns(normalizedDays, dailyStatsMap, startDay, 0, normalizedDisplaySettings)
	}
	const hourlyStatsMap = new Map<string, StatBucket>()
	return buildHourlyColumns(normalizedDays, hourlyStatsMap, startDay, 0, normalizedDisplaySettings)
}

export const buildUsageHeatmapMatrix = (
	stats: HeatmapStat[] = [],
	days = DEFAULT_HEATMAP_DAYS,
	granularity: HeatmapGranularity = 'hourly',
	displaySettings: HeatmapDisplaySettings = DEFAULT_HEATMAP_DISPLAY_SETTINGS,
): UsageHeatmapWeek[] => {
	const normalizedDays = clampDays(days)
	const startDay = addDays(startOfDay(new Date()), -(normalizedDays - 1))
	const normalizedGranularity = normalizeGranularity(granularity)
	const normalizedDisplaySettings = normalizeHeatmapDisplaySettings(displaySettings)
	const intensityMetric = normalizedDisplaySettings.intensityMetric
	if (normalizedGranularity === 'daily') {
		const dailyStatsMap = new Map<string, StatBucket>()
		const hourlyMetricValues = new Map<string, number>()
		let maxHourlyValue = 0
		let maxDailyValue = 0

		stats.forEach((stat) => {
			if (!stat) return
			const update: StatBucket = {
				requests: Number(stat.total_requests) || 0,
				inputTokens: Number(stat.input_tokens) || 0,
				outputTokens: Number(stat.output_tokens) || 0,
				cacheReadTokens: Number(stat.cache_read_tokens) || 0,
				reasoningTokens: Number(stat.reasoning_tokens) || 0,
				totalTokens: totalTokensForStat(stat),
				cost: Number(stat.total_cost) || 0,
			}
			const hourKey = normalizeStatKey(stat.day)
			if (!hourKey) return

			const nextHourlyValue =
				(hourlyMetricValues.get(hourKey) ?? 0) + intensityMetricValueForBucket(update, intensityMetric)
			hourlyMetricValues.set(hourKey, nextHourlyValue)
			if (nextHourlyValue > maxHourlyValue) {
				maxHourlyValue = nextHourlyValue
			}

			const dayKey = hourKey.slice(0, 10)
			const dayBucket = dailyStatsMap.get(dayKey)
			if (dayBucket) {
				dayBucket.requests += update.requests
				dayBucket.inputTokens += update.inputTokens
				dayBucket.outputTokens += update.outputTokens
				dayBucket.cacheReadTokens += update.cacheReadTokens
				dayBucket.reasoningTokens += update.reasoningTokens
				dayBucket.totalTokens += update.totalTokens
				dayBucket.cost += update.cost
				const nextDailyValue = intensityMetricValueForBucket(dayBucket, intensityMetric)
				if (nextDailyValue > maxDailyValue) {
					maxDailyValue = nextDailyValue
				}
			} else {
				dailyStatsMap.set(dayKey, { ...update })
				const nextDailyValue = intensityMetricValueForBucket(update, intensityMetric)
				if (nextDailyValue > maxDailyValue) {
					maxDailyValue = nextDailyValue
				}
			}
		})

		const dailyScaleMax = Math.max(maxHourlyValue * normalizedDisplaySettings.dailyScaleFactor, 0)
		const intensityMax =
			normalizedDisplaySettings.dailyIntensityMode === 'daily_peak'
				? maxDailyValue
				: dailyScaleMax
		return buildDailyColumns(
			normalizedDays,
			dailyStatsMap,
			startDay,
			Math.max(intensityMax, 0),
			normalizedDisplaySettings,
		)
	}

	const hourlyStatsMap = new Map<string, StatBucket>()
	let maxHourlyValue = 0
	stats.forEach((stat) => {
		if (!stat) return
		const update: StatBucket = {
			requests: Number(stat.total_requests) || 0,
			inputTokens: Number(stat.input_tokens) || 0,
			outputTokens: Number(stat.output_tokens) || 0,
			cacheReadTokens: Number(stat.cache_read_tokens) || 0,
			reasoningTokens: Number(stat.reasoning_tokens) || 0,
			totalTokens: totalTokensForStat(stat),
			cost: Number(stat.total_cost) || 0,
		}
		const hourKey = normalizeStatKey(stat.day)
		if (!hourKey) return

		const hourBucket = hourlyStatsMap.get(hourKey)
		if (hourBucket) {
			hourBucket.requests += update.requests
			hourBucket.inputTokens += update.inputTokens
			hourBucket.outputTokens += update.outputTokens
			hourBucket.cacheReadTokens += update.cacheReadTokens
			hourBucket.reasoningTokens += update.reasoningTokens
			hourBucket.totalTokens += update.totalTokens
			hourBucket.cost += update.cost
			const nextHourlyValue = intensityMetricValueForBucket(hourBucket, intensityMetric)
			if (nextHourlyValue > maxHourlyValue) {
				maxHourlyValue = nextHourlyValue
			}
			return
		}
		hourlyStatsMap.set(hourKey, { ...update })
		const nextHourlyValue = intensityMetricValueForBucket(update, intensityMetric)
		if (nextHourlyValue > maxHourlyValue) {
			maxHourlyValue = nextHourlyValue
		}
	})

	return buildHourlyColumns(
		normalizedDays,
		hourlyStatsMap,
		startDay,
		maxHourlyValue,
		normalizedDisplaySettings,
	)
}

export const calculateHeatmapDayRange = (days = DEFAULT_HEATMAP_DAYS) => {
	return clampDays(days)
}
