import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import type { HeatmapStat } from '../services/logs'

vi.mock('../services/logs', () => ({
	fetchHeatmapStats: vi.fn(),
}))

import { fetchHeatmapStats } from '../services/logs'
import { useAdaptiveHeatmap } from './useAdaptiveHeatmap'
import { DEFAULT_HEATMAP_DISPLAY_SETTINGS } from '../data/heatmapDisplaySettings'

const flushPromises = async () => {
	await Promise.resolve()
	await Promise.resolve()
}

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
		vi.useFakeTimers()
		const now = new Date(2026, 1, 11, 12, 0, 0)
		vi.setSystemTime(now)
		const nowKey = '2026-02-11 10'

		const mockedFetch = vi.mocked(fetchHeatmapStats)
		mockedFetch.mockResolvedValueOnce([makeStat(nowKey, 1)])

		const containerRef = ref({ clientWidth: 900 } as HTMLElement)
		const granularity = ref<'hourly' | 'daily'>('hourly')
		const displaySettings = ref({ ...DEFAULT_HEATMAP_DISPLAY_SETTINGS })
		const { init, reload, displayData } = useAdaptiveHeatmap(containerRef, granularity, displaySettings)

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

		resolveDaily([makeStat(nowKey, 24)])
		await flushPromises()
		const dailyPeak = Math.max(...displayData.value.flat().map((cell) => cell.requests))
		expect(dailyPeak).toBe(24)

		resolveHourly([makeStat(nowKey, 1)])
		await flushPromises()
		const finalPeak = Math.max(...displayData.value.flat().map((cell) => cell.requests))
		expect(finalPeak).toBe(24)
	})

	it('loads week-based day range in daily granularity', async () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 1, 11, 12, 0, 0))
		const mockedFetch = vi.mocked(fetchHeatmapStats)
		mockedFetch.mockResolvedValueOnce([])

		const containerRef = ref({ clientWidth: 900 } as HTMLElement)
		const granularity = ref<'hourly' | 'daily'>('daily')
		const displaySettings = ref({ ...DEFAULT_HEATMAP_DISPLAY_SETTINGS })
		const { init, visibleColumns } = useAdaptiveHeatmap(containerRef, granularity, displaySettings)

		await init()

		expect(mockedFetch).toHaveBeenCalledTimes(1)
		expect(mockedFetch.mock.calls[0]?.[0]).toBe(365)
		expect(visibleColumns.value).toBe(53)
	})

	it('keeps daily mode request range as one year on narrow screens', async () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 1, 11, 12, 0, 0))
		const mockedFetch = vi.mocked(fetchHeatmapStats)
		mockedFetch.mockResolvedValueOnce([])

		const containerRef = ref({ clientWidth: 360 } as HTMLElement)
		const granularity = ref<'hourly' | 'daily'>('daily')
		const displaySettings = ref({ ...DEFAULT_HEATMAP_DISPLAY_SETTINGS })
		const { init, visibleColumns } = useAdaptiveHeatmap(containerRef, granularity, displaySettings)

		await init()

		expect(mockedFetch).toHaveBeenCalledTimes(1)
		expect(mockedFetch.mock.calls[0]?.[0]).toBe(365)
		expect(visibleColumns.value).toBe(27)
	})

	it('reloads data when display settings change', async () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 1, 11, 12, 0, 0))
		const nowKey = '2026-02-11 10'
		const mockedFetch = vi.mocked(fetchHeatmapStats)
		mockedFetch.mockResolvedValueOnce([makeStat(nowKey, 1)])
		mockedFetch.mockResolvedValueOnce([makeStat(nowKey, 9)])

		const containerRef = ref({ clientWidth: 900 } as HTMLElement)
		const granularity = ref<'hourly' | 'daily'>('hourly')
		const displaySettings = ref({ ...DEFAULT_HEATMAP_DISPLAY_SETTINGS })
		const { init, displayData } = useAdaptiveHeatmap(containerRef, granularity, displaySettings)

		await init()
		expect(mockedFetch).toHaveBeenCalledTimes(1)

		displaySettings.value = {
			...displaySettings.value,
			intensityStopL1: 10,
		}
		await nextTick()
		await flushPromises()

		expect(mockedFetch).toHaveBeenCalledTimes(2)
		const peak = Math.max(...displayData.value.flat().map((cell) => cell.requests))
		expect(peak).toBe(9)
	})

	it('does not reload when display settings signature is unchanged', async () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 1, 11, 12, 0, 0))
		const nowKey = '2026-02-11 10'
		const mockedFetch = vi.mocked(fetchHeatmapStats)
		mockedFetch.mockResolvedValueOnce([makeStat(nowKey, 1)])

		const containerRef = ref({ clientWidth: 900 } as HTMLElement)
		const granularity = ref<'hourly' | 'daily'>('hourly')
		const displaySettings = ref({ ...DEFAULT_HEATMAP_DISPLAY_SETTINGS })
		const { init } = useAdaptiveHeatmap(containerRef, granularity, displaySettings)

		await init()
		expect(mockedFetch).toHaveBeenCalledTimes(1)

		displaySettings.value = {
			...displaySettings.value,
		}
		await nextTick()
		await flushPromises()

		expect(mockedFetch).toHaveBeenCalledTimes(1)
	})

	it('requests once when granularity and display settings change together', async () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 1, 11, 12, 0, 0))
		const nowKey = '2026-02-11 10'
		const mockedFetch = vi.mocked(fetchHeatmapStats)
		mockedFetch.mockResolvedValueOnce([makeStat(nowKey, 1)])
		mockedFetch.mockResolvedValueOnce([makeStat(nowKey, 24)])

		const containerRef = ref({ clientWidth: 900 } as HTMLElement)
		const granularity = ref<'hourly' | 'daily'>('hourly')
		const displaySettings = ref({ ...DEFAULT_HEATMAP_DISPLAY_SETTINGS })
		const { init, displayData } = useAdaptiveHeatmap(containerRef, granularity, displaySettings)

		await init()
		expect(mockedFetch).toHaveBeenCalledTimes(1)

		granularity.value = 'daily'
		displaySettings.value = {
			...displaySettings.value,
			intensityStopL1: 10,
		}
		await nextTick()
		await flushPromises()

		expect(mockedFetch).toHaveBeenCalledTimes(2)
		expect(mockedFetch.mock.calls[1]?.[0]).toBe(365)
		const peak = Math.max(...displayData.value.flat().map((cell) => cell.requests))
		expect(peak).toBe(24)
	})

	it('supports custom heatmap fetchers', async () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 0, 3, 12, 0, 0))
		const fetcher = vi.fn().mockResolvedValue([makeStat('2026-01-03', 6)])

		const containerRef = ref({ clientWidth: 900 } as HTMLElement)
		const granularity = ref<'hourly' | 'daily'>('daily')
		const displaySettings = ref({ ...DEFAULT_HEATMAP_DISPLAY_SETTINGS })
		const { init, displayData } = useAdaptiveHeatmap(containerRef, granularity, displaySettings, { fetcher })

		await init()
		await flushPromises()

		expect(fetcher).toHaveBeenCalledTimes(1)
		expect(vi.mocked(fetchHeatmapStats)).not.toHaveBeenCalled()
		expect(displayData.value.flat().find((cell) => cell.requests === 6)?.label).toBe('01-03')
	})
})
