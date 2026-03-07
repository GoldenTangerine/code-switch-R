import { afterEach, describe, expect, it, vi } from 'vitest'
import { buildUsageHeatmapMatrix } from './usageHeatmap'
import type { HeatmapStat } from '../services/logs'
import { DEFAULT_HEATMAP_DISPLAY_SETTINGS } from './heatmapDisplaySettings'

const makeStat = (
	day: string,
	requests: number,
	overrides: Partial<Omit<HeatmapStat, 'day' | 'total_requests'>> = {},
): HeatmapStat => ({
	day,
	total_requests: requests,
	input_tokens: 0,
	output_tokens: 0,
	cache_read_tokens: 0,
	reasoning_tokens: 0,
	total_cost: 0,
	...overrides,
})

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

describe('usageHeatmap', () => {
	afterEach(() => {
		vi.useRealTimers()
	})

	it('parses full-year hour keys across year boundary correctly', () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 0, 2, 12, 0, 0))

		const matrix = buildUsageHeatmapMatrix(
			[
				makeStat('2025-12-31 23', 3),
			],
			5,
			'hourly',
		)
		const targetCell = matrix
			.flat()
			.find((cell) => cell.label === '12-31 23')

		expect(targetCell?.requests).toBe(3)
	})

	it('scales daily intensity using hourly baseline * 24', () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 0, 3, 12, 0, 0))

		const stats: HeatmapStat[] = [
			makeStat('2026-01-01 10', 10),
		]
		for (let hour = 0; hour < 24; hour++) {
			stats.push(makeStat(`2026-01-02 ${String(hour).padStart(2, '0')}`, 10))
		}

		const matrix = buildUsageHeatmapMatrix(stats, 3, 'daily')
		expect(matrix).toHaveLength(1)
		expect(matrix.every((column) => column.length === 7)).toBe(true)
		expect(new Date(matrix[0][0].dateKey).getDay()).toBe(1)
		const dailyCells = matrix.flat()
		const jan1 = dailyCells.find((cell) => cell.label === '01-01')
		const jan2 = dailyCells.find((cell) => cell.label === '01-02')

		expect(jan1?.requests).toBe(10)
		expect(jan1?.intensity).toBe(1)
		expect(jan2?.requests).toBe(240)
		expect(jan2?.intensity).toBe(4)
		expect(dailyCells.filter((cell) => cell.label === '01-01')).toHaveLength(1)
	})

	it('groups daily data into monday-first weekly columns', () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 0, 8, 12, 0, 0))

		const matrix = buildUsageHeatmapMatrix([], 10, 'daily')

		expect(matrix).toHaveLength(2)
		expect(matrix.every((column) => column.length === 7)).toBe(true)
		expect(new Date(matrix[0][0].dateKey).getDay()).toBe(1)
		expect(new Date(matrix[1][0].dateKey).getDay()).toBe(1)
		expect(matrix[0][0].label).toBe('12-29')
		expect(matrix[1][0].label).toBe('01-05')
	})

	it('keeps weekly grouping correct across DST transition weeks', () => {
		withTimezone('America/New_York', () => {
			vi.useFakeTimers()
			vi.setSystemTime(new Date('2026-03-14T12:00:00-04:00'))

			const matrix = buildUsageHeatmapMatrix([], 10, 'daily')

			expect(matrix).toHaveLength(2)
			expect(matrix.every((column) => column.length === 7)).toBe(true)
			expect(new Date(matrix[0][0].dateKey).getDay()).toBe(1)
			expect(new Date(matrix[1][0].dateKey).getDay()).toBe(1)
		})
	})

	it('supports daily peak intensity mode for daily granularity', () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 0, 3, 12, 0, 0))

		const stats: HeatmapStat[] = [makeStat('2026-01-01 10', 10)]
		for (let hour = 0; hour < 10; hour++) {
			stats.push(makeStat(`2026-01-02 ${String(hour).padStart(2, '0')}`, 10))
		}

		const scaled = buildUsageHeatmapMatrix(
			stats,
			3,
			'daily',
			{
				...DEFAULT_HEATMAP_DISPLAY_SETTINGS,
				dailyIntensityMode: 'hourly_scaled',
				dailyScaleFactor: 24,
			},
		)
		const dailyPeak = buildUsageHeatmapMatrix(
			stats,
			3,
			'daily',
			{
				...DEFAULT_HEATMAP_DISPLAY_SETTINGS,
				dailyIntensityMode: 'daily_peak',
			},
		)

		const jan2Scaled = scaled.flat().find((cell) => cell.label === '01-02')
		const jan2DailyPeak = dailyPeak.flat().find((cell) => cell.label === '01-02')
		expect(jan2Scaled?.requests).toBe(100)
		expect(jan2Scaled?.intensity).toBe(2)
		expect(jan2Scaled?.intensityValue).toBe(100)
		expect(jan2Scaled?.intensityPeakValue).toBe(240)
		expect(jan2DailyPeak?.requests).toBe(100)
		expect(jan2DailyPeak?.intensity).toBe(4)
		expect(jan2DailyPeak?.intensityValue).toBe(100)
		expect(jan2DailyPeak?.intensityPeakValue).toBe(100)
	})

	it('applies custom intensity stops for hourly granularity', () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 0, 3, 12, 0, 0))

		const stats: HeatmapStat[] = [
			makeStat('2026-01-03 00', 100),
			makeStat('2026-01-03 01', 30),
		]

		const matrixDefault = buildUsageHeatmapMatrix(stats, 1, 'hourly')
		const matrixCustom = buildUsageHeatmapMatrix(
			stats,
			1,
			'hourly',
			{
				...DEFAULT_HEATMAP_DISPLAY_SETTINGS,
				intensityStopL1: 40,
				intensityStopL2: 70,
				intensityStopL3: 90,
			},
		)

		const hour01Default = matrixDefault.flat().find((cell) => cell.label === '01-03 01')
		const hour01Custom = matrixCustom.flat().find((cell) => cell.label === '01-03 01')
		const hour00Custom = matrixCustom.flat().find((cell) => cell.label === '01-03 00')

		expect(hour01Default?.intensity).toBe(2)
		expect(hour01Custom?.intensity).toBe(1)
		expect(hour00Custom?.intensity).toBe(4)
	})

	it('uses total tokens as intensity metric when configured', () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 0, 3, 12, 0, 0))

		const stats: HeatmapStat[] = [
			makeStat('2026-01-03 00', 100, {
				input_tokens: 5,
				output_tokens: 5,
				cache_read_tokens: 0,
				reasoning_tokens: 90,
				total_tokens: 10,
			}),
			makeStat('2026-01-03 01', 1, {
				input_tokens: 30,
				output_tokens: 30,
				cache_read_tokens: 10,
				reasoning_tokens: 0,
				total_tokens: 70,
			}),
		]

		const matrix = buildUsageHeatmapMatrix(
			stats,
			1,
			'hourly',
			{
				...DEFAULT_HEATMAP_DISPLAY_SETTINGS,
				intensityMetric: 'total_tokens',
			},
		)

		const hour00 = matrix.flat().find((cell) => cell.label === '01-03 00')
		const hour01 = matrix.flat().find((cell) => cell.label === '01-03 01')

		expect(hour00?.requests).toBe(100)
		expect(hour00?.intensity).toBe(1)
		expect(hour00?.intensityValue).toBe(10)
		expect(hour00?.intensityPeakValue).toBe(70)
		expect(hour01?.requests).toBe(1)
		expect(hour01?.intensity).toBe(4)
		expect(hour01?.intensityValue).toBe(70)
		expect(hour01?.intensityPeakValue).toBe(70)
	})

	it('falls back to input + output + cache read when total tokens field is absent', () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 0, 3, 12, 0, 0))

		const stats: HeatmapStat[] = [
			makeStat('2026-01-03 00', 5, {
				input_tokens: 5,
				output_tokens: 5,
				cache_read_tokens: 0,
				reasoning_tokens: 100,
				total_tokens: undefined,
			}),
			makeStat('2026-01-03 01', 5, {
				input_tokens: 20,
				output_tokens: 20,
				cache_read_tokens: 10,
				reasoning_tokens: 0,
				total_tokens: undefined,
			}),
		]

		const matrix = buildUsageHeatmapMatrix(
			stats,
			1,
			'hourly',
			{
				...DEFAULT_HEATMAP_DISPLAY_SETTINGS,
				intensityMetric: 'total_tokens',
			},
		)

		const hour00 = matrix.flat().find((cell) => cell.label === '01-03 00')
		const hour01 = matrix.flat().find((cell) => cell.label === '01-03 01')

		expect(hour00?.intensity).toBe(1)
		expect(hour01?.intensity).toBe(4)
	})


	it('tracks scaled peak values for daily cost intensity mode', () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 0, 3, 12, 0, 0))

		const stats: HeatmapStat[] = [
			makeStat('2026-01-01 10', 1, { total_cost: 1 }),
		]
		for (let hour = 0; hour < 24; hour++) {
			stats.push(makeStat(`2026-01-02 ${String(hour).padStart(2, '0')}`, 1, { total_cost: 1 }))
		}

		const matrix = buildUsageHeatmapMatrix(
			stats,
			3,
			'daily',
			{
				...DEFAULT_HEATMAP_DISPLAY_SETTINGS,
				intensityMetric: 'cost',
			},
		)
		const dailyCells = matrix.flat()
		const jan2 = dailyCells.find((cell) => cell.label === '01-02')

		expect(jan2?.cost).toBe(24)
		expect(jan2?.intensityValue).toBe(24)
		expect(jan2?.intensityPeakValue).toBe(24)
	})
	it('uses cost as scaled baseline metric for daily granularity', () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 0, 3, 12, 0, 0))

		const stats: HeatmapStat[] = [
			makeStat('2026-01-01 10', 1, { total_cost: 1 }),
		]
		for (let hour = 0; hour < 24; hour++) {
			stats.push(makeStat(`2026-01-02 ${String(hour).padStart(2, '0')}`, 1, { total_cost: 1 }))
		}

		const matrix = buildUsageHeatmapMatrix(
			stats,
			3,
			'daily',
			{
				...DEFAULT_HEATMAP_DISPLAY_SETTINGS,
				intensityMetric: 'cost',
			},
		)
		const dailyCells = matrix.flat()
		const jan1 = dailyCells.find((cell) => cell.label === '01-01')
		const jan2 = dailyCells.find((cell) => cell.label === '01-02')

		expect(jan1?.cost).toBe(1)
		expect(jan1?.intensity).toBe(1)
		expect(jan2?.cost).toBe(24)
		expect(jan2?.intensity).toBe(4)
	})
})
