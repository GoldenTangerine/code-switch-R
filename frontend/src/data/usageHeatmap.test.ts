import { afterEach, describe, expect, it, vi } from 'vitest'
import { buildUsageHeatmapMatrix } from './usageHeatmap'
import type { HeatmapStat } from '../services/logs'

const makeStat = (day: string, requests: number): HeatmapStat => ({
	day,
	total_requests: requests,
	input_tokens: 0,
	output_tokens: 0,
	reasoning_tokens: 0,
	total_cost: 0,
})

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
		const dailyCells = matrix.map((column) => column[0])
		const jan1 = dailyCells.find((cell) => cell.label === '01-01')
		const jan2 = dailyCells.find((cell) => cell.label === '01-02')

		expect(jan1?.requests).toBe(10)
		expect(jan1?.intensity).toBe(1)
		expect(jan2?.requests).toBe(240)
		expect(jan2?.intensity).toBe(4)
	})
})
