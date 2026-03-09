/**
 * 自适应热力图 Composable
 * @author sm
 * @description 封装热力图自适应逻辑，根据容器宽度动态计算显示的列数
 */
import { ref, computed, watch, type Ref } from 'vue'
import {
	BUCKETS_PER_DAY,
	buildUsageHeatmapMatrix,
	generateFallbackUsageHeatmap,
	type HeatmapGranularity,
	type UsageHeatmapWeek,
} from '../data/usageHeatmap'
import {
	heatmapDisplaySettingsSignature,
	type HeatmapDisplaySettings,
} from '../data/heatmapDisplaySettings'
import { fetchHeatmapStats, type HeatmapStat } from '../services/logs'

// 格子尺寸配置（与 CSS 媒体查询保持一致）
const CELL_SIZES = {
	large: { cell: 14, gap: 4, padding: 32 }, // > 960px
	medium: { cell: 12, gap: 3, padding: 24 }, // 640-960px
	small: { cell: 10, gap: 2, padding: 16 }, // < 640px
} as const

// 边界限制
const MIN_COLUMNS = 9
const MAX_COLUMNS_HOURLY = 63
const DAYS_PER_DAILY_COLUMN = 7
const MAX_DAYS_HOURLY = 21
const MAX_DAYS_DAILY = 365
const DEFAULT_DAYS_HOURLY = 14
const DEFAULT_DAYS_DAILY = 365

const columnGroupSizeByGranularity = (granularity: HeatmapGranularity) =>
	granularity === 'daily' ? 1 : BUCKETS_PER_DAY

const maxDaysByGranularity = (granularity: HeatmapGranularity) =>
	granularity === 'daily' ? MAX_DAYS_DAILY : MAX_DAYS_HOURLY

const defaultDaysByGranularity = (granularity: HeatmapGranularity) =>
	granularity === 'daily' ? DEFAULT_DAYS_DAILY : DEFAULT_DAYS_HOURLY

const maxColumnsByGranularity = (granularity: HeatmapGranularity) =>
	granularity === 'daily'
		? Math.ceil(MAX_DAYS_DAILY / DAYS_PER_DAILY_COLUMN)
		: MAX_COLUMNS_HOURLY

const columnsFromDays = (days: number, granularity: HeatmapGranularity) => {
	const normalizedDays = Math.max(1, Math.floor(days))
	if (granularity === 'daily') {
		return Math.max(1, Math.ceil(normalizedDays / DAYS_PER_DAILY_COLUMN))
	}
	return Math.max(BUCKETS_PER_DAY, normalizedDays * BUCKETS_PER_DAY)
}

export type HeatmapStatsFetcher = (days: number) => Promise<HeatmapStat[]>

type UseAdaptiveHeatmapOptions = {
	fetcher?: HeatmapStatsFetcher
}

/**
 * 自适应热力图 Composable
 * @param containerRef 热力图容器的 ref 引用
 */
export function useAdaptiveHeatmap(
	containerRef: Ref<HTMLElement | null>,
	granularity: Ref<HeatmapGranularity>,
	displaySettings: Ref<HeatmapDisplaySettings>,
	options: UseAdaptiveHeatmapOptions = {},
) {
	const heatmapStatsFetcher = options.fetcher ?? fetchHeatmapStats
	// 响应式状态
	const containerWidth = ref(0)
	const visibleColumns = ref(columnsFromDays(defaultDaysByGranularity(granularity.value), granularity.value))
	const heatmapData = ref<UsageHeatmapWeek[]>(
		generateFallbackUsageHeatmap(
			defaultDaysByGranularity(granularity.value),
			granularity.value,
			displaySettings.value,
		),
	)
	const isLoading = ref(false)
	const loadedDays = ref(0) // 已加载的天数
	const loadedGranularity = ref<HeatmapGranularity | null>(null)
	const initialized = ref(false)
	const displaySettingsKey = computed(() => heatmapDisplaySettingsSignature(displaySettings.value))
	let latestRequestToken = 0

	/**
	 * 获取当前视口下的格子尺寸配置
	 */
	const cellConfig = computed(() => {
		const width = containerWidth.value
		if (width > 960) return CELL_SIZES.large
		if (width > 640) return CELL_SIZES.medium
		return CELL_SIZES.small
	})

	/**
	 * 计算可显示的列数
	 * @param containerWidth 容器宽度
	 */
	const calculateColumns = (
		containerWidth: number,
		columnGroupSize: number,
		maxColumns: number,
	): number => {
		const { cell, gap, padding } = cellConfig.value
		const availableWidth = containerWidth - padding * 2
		const cellUnit = cell + gap

		// 计算可容纳的列数
		const cols = Math.floor((availableWidth + gap) / cellUnit)

		// 应用边界限制
		const bounded = Math.max(MIN_COLUMNS, Math.min(maxColumns, cols))

		// 向下取整到分组倍数，确保小时模式下按整天显示
		return Math.max(
			columnGroupSize,
			Math.floor(bounded / columnGroupSize) * columnGroupSize,
		)
	}

	const calculateDaysFromColumns = (
		columns: number,
		currentGranularity: HeatmapGranularity,
		maxDays: number,
	) => {
		if (currentGranularity === 'daily') {
			return maxDays
		}
		return Math.max(1, Math.min(maxDays, Math.floor(columns / BUCKETS_PER_DAY)))
	}

	/**
	 * 加载热力图数据
	 * @param days 需要加载的天数
	 */
	const loadHeatmapData = async (days: number) => {
		const currentGranularity = granularity.value
		// 如果已加载的天数足够，不重复请求
		if (
			loadedGranularity.value === currentGranularity &&
			loadedDays.value >= days &&
			heatmapData.value.length > 0
		) {
			return
		}

		const requestToken = ++latestRequestToken
		isLoading.value = true
		try {
			const stats = await heatmapStatsFetcher(days)
			if (requestToken !== latestRequestToken) {
				return
			}
			if (granularity.value !== currentGranularity) {
				return
			}
			heatmapData.value = buildUsageHeatmapMatrix(
				stats,
				days,
				currentGranularity,
				displaySettings.value,
			)
			loadedDays.value = days
			loadedGranularity.value = currentGranularity
		} catch (error) {
			console.error('Failed to load heatmap data:', error)
		} finally {
			if (requestToken === latestRequestToken) {
				isLoading.value = false
			}
		}
	}

	/**
	 * 节流函数
	 */
	const throttle = <T extends (...args: any[]) => void>(fn: T, delay: number) => {
		let lastCall = 0
		let timeoutId: ReturnType<typeof setTimeout> | null = null
		return (...args: Parameters<T>) => {
			const now = Date.now()
			const remaining = delay - (now - lastCall)
			if (remaining <= 0) {
				if (timeoutId) {
					clearTimeout(timeoutId)
					timeoutId = null
				}
				lastCall = now
				fn(...args)
			} else if (!timeoutId) {
				timeoutId = setTimeout(() => {
					lastCall = Date.now()
					timeoutId = null
					fn(...args)
				}, remaining)
			}
		}
	}

	/**
	 * 处理尺寸变化
	 */
	const handleResize = throttle((width: number) => {
		containerWidth.value = width
		const currentGranularity = granularity.value
		const columnGroupSize = columnGroupSizeByGranularity(currentGranularity)
		const maxColumns = maxColumnsByGranularity(currentGranularity)
		const maxDays = maxDaysByGranularity(currentGranularity)
		const newColumns = calculateColumns(width, columnGroupSize, maxColumns)

		// 只有列数变化时才更新
		if (newColumns !== visibleColumns.value) {
			const newDays = calculateDaysFromColumns(newColumns, currentGranularity, maxDays)
			visibleColumns.value = newColumns

			// 只在需要更多数据时重新请求
			if (newDays > loadedDays.value || loadedGranularity.value !== currentGranularity) {
				void loadHeatmapData(newDays)
			}
		}
	}, 150) // 150ms 节流

	/**
	 * 裁剪显示的数据（只显示最新的 N 列）
	 */
	const displayData = computed(() => {
		const data = heatmapData.value
		if (data.length <= visibleColumns.value) {
			return data
		}
		// 从最新的数据开始显示（数组末尾是最新的）
		return data.slice(data.length - visibleColumns.value)
	})

	// ResizeObserver 实例
	let resizeObserver: ResizeObserver | null = null

	const getContainerWidth = (container: HTMLElement) => {
		const rectWidth = Math.round(container.getBoundingClientRect().width || 0)
		if (rectWidth > 0) {
			return rectWidth
		}
		return container.clientWidth
	}

	const probeContainerWidthOnNextFrame = (container: HTMLElement) => {
		if (typeof window === 'undefined' || typeof window.requestAnimationFrame !== 'function') {
			return
		}
		window.requestAnimationFrame(() => {
			const nextWidth = getContainerWidth(container)
			if (nextWidth > 0 && nextWidth !== containerWidth.value) {
				handleResize(nextWidth)
			}
		})
	}

	/**
	 * 初始化热力图
	 */
	const init = async () => {
		const container = containerRef.value
		if (!container) return

		const currentGranularity = granularity.value
		const columnGroupSize = columnGroupSizeByGranularity(currentGranularity)
		const maxColumns = maxColumnsByGranularity(currentGranularity)
		const maxDays = maxDaysByGranularity(currentGranularity)

		// 先挂尺寸监听，避免首屏布局还没稳定时漏掉真正的宽度变化。
		resizeObserver = new ResizeObserver((entries) => {
			for (const entry of entries) {
				const { width } = entry.contentRect
				handleResize(width)
			}
		})
		resizeObserver.observe(container)

		// 初始宽度计算
		const initialWidth = getContainerWidth(container)
		containerWidth.value = initialWidth
		const initialColumns = calculateColumns(initialWidth, columnGroupSize, maxColumns)
		visibleColumns.value = initialColumns
		probeContainerWidthOnNextFrame(container)

		// 加载初始数据
		const initialDays = calculateDaysFromColumns(initialColumns, currentGranularity, maxDays)
		await loadHeatmapData(initialDays)
		initialized.value = true
	}

	/**
	 * 清理 ResizeObserver
	 */
	const cleanup = () => {
		if (resizeObserver) {
			resizeObserver.disconnect()
			resizeObserver = null
		}
	}

	/**
	 * 重新加载数据
	 */
	const reload = async () => {
		loadedDays.value = 0 // 重置已加载天数，强制重新请求
		loadedGranularity.value = null
		const currentGranularity = granularity.value
		const maxDays = maxDaysByGranularity(currentGranularity)
		const days = calculateDaysFromColumns(visibleColumns.value, currentGranularity, maxDays)
		await loadHeatmapData(days)
	}

	watch([granularity, displaySettingsKey], ([nextGranularity, nextDisplaySettingsKey], previous) => {
		const [prevGranularity, prevDisplaySettingsKey] = previous ?? []
		const granularityChanged = prevGranularity !== undefined && nextGranularity !== prevGranularity
		const displaySettingsChanged =
			prevDisplaySettingsKey !== undefined && nextDisplaySettingsKey !== prevDisplaySettingsKey

		if (!granularityChanged && !displaySettingsChanged) {
			return
		}

		loadedDays.value = 0
		loadedGranularity.value = null
		const defaultDays = defaultDaysByGranularity(nextGranularity)

		if (!initialized.value) {
			if (granularityChanged) {
				visibleColumns.value = columnsFromDays(defaultDays, nextGranularity)
			}
			heatmapData.value = generateFallbackUsageHeatmap(
				defaultDays,
				nextGranularity,
				displaySettings.value,
			)
			return
		}

		if (!granularityChanged) {
			void reload()
			return
		}

		const columnGroupSize = columnGroupSizeByGranularity(nextGranularity)
		const maxColumns = maxColumnsByGranularity(nextGranularity)
		const maxDays = maxDaysByGranularity(nextGranularity)
		const width = containerRef.value?.clientWidth ?? containerWidth.value
		if (width > 0) {
			const columns = calculateColumns(width, columnGroupSize, maxColumns)
			visibleColumns.value = columns
			const days = calculateDaysFromColumns(columns, nextGranularity, maxDays)
			void loadHeatmapData(days)
			return
		}

		visibleColumns.value = columnsFromDays(defaultDays, nextGranularity)
		heatmapData.value = generateFallbackUsageHeatmap(
			defaultDays,
			nextGranularity,
			displaySettings.value,
		)
	})

	return {
		containerWidth,
		visibleColumns,
		displayData,
		cellConfig,
		isLoading,
		init,
		cleanup,
		reload,
	}
}
