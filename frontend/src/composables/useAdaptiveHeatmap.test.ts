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

const makeContentRect = (width: number): DOMRectReadOnly => ({
	width,
	height: 0,
	x: 0,
	y: 0,
	top: 0,
	right: width,
	bottom: 0,
	left: 0,
	toJSON: () => ({}),
})

const makeStat = (day: string, requests: number): HeatmapStat => ({
	day,
	total_requests: requests,
	input_tokens: 0,
	output_tokens: 0,
	reasoning_tokens: 0,
	total_cost: 0,
})

const originalRequestAnimationFrame = globalThis.requestAnimationFrame
const originalCancelAnimationFrame = globalThis.cancelAnimationFrame

class MockResizeObserver {
	callback: ResizeObserverCallback

	constructor(callback: ResizeObserverCallback) {
		this.callback = callback
	}

	observe() {}
	disconnect() {}

	trigger(width: number) {
		this.callback(
			[
				{
					contentRect: makeContentRect(width),
				} as ResizeObserverEntry,
			],
			this as unknown as ResizeObserver,
		)
	}
}

describe('useAdaptiveHeatmap', () => {
	beforeEach(() => {
		vi.clearAllMocks()
		;(globalThis as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = MockResizeObserver as unknown as typeof ResizeObserver
		;(globalThis as unknown as { requestAnimationFrame: typeof requestAnimationFrame }).requestAnimationFrame =
			((callback: FrameRequestCallback) =>
				setTimeout(() => callback(Date.now()), 0) as unknown as number) as typeof requestAnimationFrame
		;(globalThis as unknown as { cancelAnimationFrame: typeof cancelAnimationFrame }).cancelAnimationFrame =
			((handle: number) => clearTimeout(handle)) as typeof cancelAnimationFrame
	})

	afterEach(() => {
		vi.useRealTimers()
		if (originalRequestAnimationFrame) {
			globalThis.requestAnimationFrame = originalRequestAnimationFrame
		} else {
			delete (globalThis as { requestAnimationFrame?: typeof requestAnimationFrame }).requestAnimationFrame
		}
		if (originalCancelAnimationFrame) {
			globalThis.cancelAnimationFrame = originalCancelAnimationFrame
		} else {
			delete (globalThis as { cancelAnimationFrame?: typeof cancelAnimationFrame }).cancelAnimationFrame
		}
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
		expect(visibleColumns.value).toBe(26)
	})

	it('loads default hourly range when container width is unavailable during init', async () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 1, 11, 12, 0, 0))
		const mockedFetch = vi.mocked(fetchHeatmapStats)
		mockedFetch.mockResolvedValueOnce([])

		const containerRef = ref({
			clientWidth: 0,
			getBoundingClientRect: () => makeContentRect(0),
		} as HTMLElement)
		const granularity = ref<'hourly' | 'daily'>('hourly')
		const displaySettings = ref({ ...DEFAULT_HEATMAP_DISPLAY_SETTINGS })
		const { init, visibleColumns } = useAdaptiveHeatmap(containerRef, granularity, displaySettings)

		await init()

		expect(mockedFetch).toHaveBeenCalledTimes(1)
		expect(mockedFetch.mock.calls[0]?.[0]).toBe(14)
		expect(visibleColumns.value).toBe(42)
	})

	it('re-probes width across animation frames and expands request range after layout settles', async () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 1, 11, 12, 0, 0))
		const mockedFetch = vi.mocked(fetchHeatmapStats)
		mockedFetch.mockResolvedValueOnce([])
		mockedFetch.mockResolvedValueOnce([])

		let measuredWidth = 0
		const containerRef = ref({
			get clientWidth() {
				return measuredWidth
			},
			getBoundingClientRect: () => makeContentRect(measuredWidth),
		} as HTMLElement)
		const granularity = ref<'hourly' | 'daily'>('hourly')
		const displaySettings = ref({ ...DEFAULT_HEATMAP_DISPLAY_SETTINGS })
		const { init, visibleColumns } = useAdaptiveHeatmap(containerRef, granularity, displaySettings)

		await init()
		expect(mockedFetch.mock.calls[0]?.[0]).toBe(14)

		measuredWidth = 1200
		await vi.runAllTimersAsync()
		await flushPromises()

		expect(mockedFetch).toHaveBeenCalledTimes(2)
		expect(mockedFetch.mock.calls[1]?.[0]).toBe(20)
		expect(visibleColumns.value).toBe(60)
	})

	it('pauses layout sync before hidden-page probes can trigger extra requests', async () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 1, 11, 12, 0, 0))
		const mockedFetch = vi.mocked(fetchHeatmapStats)
		mockedFetch.mockResolvedValueOnce([])
		mockedFetch.mockResolvedValueOnce([])

		let measuredWidth = 0
		const containerRef = ref({
			get clientWidth() {
				return measuredWidth
			},
			getBoundingClientRect: () => makeContentRect(measuredWidth),
		} as HTMLElement)
		const granularity = ref<'hourly' | 'daily'>('hourly')
		const displaySettings = ref({ ...DEFAULT_HEATMAP_DISPLAY_SETTINGS })
		const { init, pauseLayoutSync, syncLayout } = useAdaptiveHeatmap(containerRef, granularity, displaySettings)

		await init()
		expect(mockedFetch.mock.calls[0]?.[0]).toBe(14)

		measuredWidth = 1200
		pauseLayoutSync()
		await vi.runAllTimersAsync()
		await flushPromises()

		expect(mockedFetch).toHaveBeenCalledTimes(1)

		await syncLayout()
		await flushPromises()

		expect(mockedFetch).toHaveBeenCalledTimes(2)
		expect(mockedFetch.mock.calls[1]?.[0]).toBe(20)
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

	it('forces reload to override a pending request for the same range', async () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 1, 11, 12, 0, 0))
		const nowKey = '2026-02-11 10'
		const mockedFetch = vi.mocked(fetchHeatmapStats)

		let resolveInitial!: (value: HeatmapStat[]) => void
		const pendingInitial = new Promise<HeatmapStat[]>((resolve) => {
			resolveInitial = resolve
		})
		mockedFetch.mockReturnValueOnce(pendingInitial)

		const containerRef = ref({
			clientWidth: 900,
			getBoundingClientRect: () => makeContentRect(900),
		} as HTMLElement)
		const granularity = ref<'hourly' | 'daily'>('hourly')
		const displaySettings = ref({ ...DEFAULT_HEATMAP_DISPLAY_SETTINGS })
		const { init, reload, displayData } = useAdaptiveHeatmap(containerRef, granularity, displaySettings)

		const initPromise = init()
		await flushPromises()

		let resolveReload!: (value: HeatmapStat[]) => void
		const pendingReload = new Promise<HeatmapStat[]>((resolve) => {
			resolveReload = resolve
		})
		mockedFetch.mockReturnValueOnce(pendingReload)

		const reloadPromise = reload()
		await flushPromises()

		expect(mockedFetch).toHaveBeenCalledTimes(2)
		expect(mockedFetch.mock.calls[0]?.[0]).toBe(18)
		expect(mockedFetch.mock.calls[1]?.[0]).toBe(18)

		resolveReload([makeStat(nowKey, 7)])
		await flushPromises()

		resolveInitial([makeStat(nowKey, 1)])
		await flushPromises()
		await Promise.all([initPromise, reloadPromise])

		const peak = Math.max(...displayData.value.flat().map((cell) => cell.requests))
		expect(peak).toBe(7)
	})

	it('reloads with latest granularity when props change before initial request resolves', async () => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date(2026, 1, 11, 12, 0, 0))
		const nowKey = '2026-02-11 10'
		const mockedFetch = vi.mocked(fetchHeatmapStats)

		let resolveInitial!: (value: HeatmapStat[]) => void
		const pendingInitial = new Promise<HeatmapStat[]>((resolve) => {
			resolveInitial = resolve
		})
		mockedFetch.mockReturnValueOnce(pendingInitial)

		const containerRef = ref({
			clientWidth: 900,
			getBoundingClientRect: () => makeContentRect(900),
		} as HTMLElement)
		const granularity = ref<'hourly' | 'daily'>('hourly')
		const displaySettings = ref({ ...DEFAULT_HEATMAP_DISPLAY_SETTINGS })
		const { init, displayData } = useAdaptiveHeatmap(containerRef, granularity, displaySettings)

		const initPromise = init()
		await flushPromises()

		let resolveDaily!: (value: HeatmapStat[]) => void
		const pendingDaily = new Promise<HeatmapStat[]>((resolve) => {
			resolveDaily = resolve
		})
		mockedFetch.mockReturnValueOnce(pendingDaily)

		granularity.value = 'daily'
		await nextTick()
		await flushPromises()

		expect(mockedFetch).toHaveBeenCalledTimes(2)
		expect(mockedFetch.mock.calls[0]?.[0]).toBe(18)
		expect(mockedFetch.mock.calls[1]?.[0]).toBe(365)

		resolveDaily([makeStat(nowKey, 24)])
		await flushPromises()

		resolveInitial([makeStat(nowKey, 1)])
		await flushPromises()
		await initPromise

		const peak = Math.max(...displayData.value.flat().map((cell) => cell.requests))
		expect(peak).toBe(24)
	})
})
