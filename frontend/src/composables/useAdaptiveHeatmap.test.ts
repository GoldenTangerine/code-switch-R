import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import type { HeatmapStat } from '../services/logs'

vi.mock('../services/logs', () => ({
	fetchHeatmapStats: vi.fn(),
}))

import { fetchHeatmapStats } from '../services/logs'
import { useAdaptiveHeatmap } from './useAdaptiveHeatmap'

const flushPromises = () => new Promise((resolve) => {
	setTimeout(resolve, 0)
})

const makeStat = (day: string, requests: number): HeatmapStat => ({
	day,
	total_requests: requests,
	input_tokens: 0,
	output_tokens: 0,
	reasoning_tokens: 0,
	total_cost: 0,
})

class MockResizeObserver {
	observe() {}
	disconnect() {}
}

describe('useAdaptiveHeatmap', () => {
	beforeEach(() => {
		vi.clearAllMocks()
		;(globalThis as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = MockResizeObserver as unknown as typeof ResizeObserver
	})

	afterEach(() => {
		vi.useRealTimers()
	})

	it('keeps latest granularity data when previous request resolves later', async () => {
		const mockedFetch = vi.mocked(fetchHeatmapStats)
		mockedFetch.mockResolvedValueOnce([makeStat('2026-01-02 10', 1)])

		const containerRef = ref({ clientWidth: 900 } as HTMLElement)
		const granularity = ref<'hourly' | 'daily'>('hourly')
		const { init, reload, displayData } = useAdaptiveHeatmap(containerRef, granularity)

		await init()

		let resolveHourly!: (value: HeatmapStat[]) => void
		const pendingHourly = new Promise<HeatmapStat[]>((resolve) => {
			resolveHourly = resolve
		})
		mockedFetch.mockReturnValueOnce(pendingHourly)
		void reload()

		let resolveDaily!: (value: HeatmapStat[]) => void
		const pendingDaily = new Promise<HeatmapStat[]>((resolve) => {
			resolveDaily = resolve
		})
		mockedFetch.mockReturnValueOnce(pendingDaily)

		granularity.value = 'daily'
		await nextTick()

		resolveDaily([makeStat('2026-01-02 10', 24)])
		await flushPromises()
		const dailyPeak = Math.max(...displayData.value.flat().map((cell) => cell.requests))
		expect(dailyPeak).toBe(24)

		resolveHourly([makeStat('2026-01-02 10', 1)])
		await flushPromises()
		const finalPeak = Math.max(...displayData.value.flat().map((cell) => cell.requests))
		expect(finalPeak).toBe(24)
	})
})
