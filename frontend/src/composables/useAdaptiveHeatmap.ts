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
import { fetchHeatmapStats } from '../services/logs'

// 格子尺寸配置（与 CSS 媒体查询保持一致）
const CELL_SIZES = {
	large: { cell: 14, gap: 4, padding: 32 }, // > 960px
	medium: { cell: 12, gap: 3, padding: 24 }, // 640-960px
	small: { cell: 10, gap: 2, padding: 16 }, // < 640px
} as const

// 边界限制
const MIN_COLUMNS = 9 // 最少显示 3 天 (3×3)
const MAX_COLUMNS = 63 // 最多显示 21 天 (21×3)
const MAX_DAYS_HOURLY = 21
const MAX_DAYS_DAILY = 63
const DEFAULT_DAYS_HOURLY = 14
const DEFAULT_DAYS_DAILY = 21

const bucketsPerDayByGranularity = (granularity: HeatmapGranularity) =>
	granularity === 'daily' ? 1 : BUCKETS_PER_DAY

const maxDaysByGranularity = (granularity: HeatmapGranularity) =>
	granularity === 'daily' ? MAX_DAYS_DAILY : MAX_DAYS_HOURLY

const defaultDaysByGranularity = (granularity: HeatmapGranularity) =>
	granularity === 'daily' ? DEFAULT_DAYS_DAILY : DEFAULT_DAYS_HOURLY

/**
 * 自适应热力图 Composable
 * @param containerRef 热力图容器的 ref 引用
 */
export function useAdaptiveHeatmap(
	containerRef: Ref<HTMLElement | null>,
	granularity: Ref<HeatmapGranularity>,
) {
	// 响应式状态
	const containerWidth = ref(0)
	const visibleColumns = ref(defaultDaysByGranularity(granularity.value) * bucketsPerDayByGranularity(granularity.value))
	const heatmapData = ref<UsageHeatmapWeek[]>(
		generateFallbackUsageHeatmap(defaultDaysByGranularity(granularity.value), granularity.value),
	)
	const isLoading = ref(false)
	const loadedDays = ref(0) // 已加载的天数
	const loadedGranularity = ref<HeatmapGranularity | null>(null)
	const initialized = ref(false)
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
	const calculateColumns = (containerWidth: number, bucketsPerDay: number): number => {
		const { cell, gap, padding } = cellConfig.value
		const availableWidth = containerWidth - padding * 2
		const cellUnit = cell + gap

		// 计算可容纳的列数
		const cols = Math.floor((availableWidth + gap) / cellUnit)

		// 应用边界限制
		const bounded = Math.max(MIN_COLUMNS, Math.min(MAX_COLUMNS, cols))

		// 向下取整到 bucketsPerDay 的倍数，确保天数完整
		return Math.max(
			bucketsPerDay,
			Math.floor(bounded / bucketsPerDay) * bucketsPerDay,
		)
	}

	const calculateDaysFromColumns = (columns: number, bucketsPerDay: number, maxDays: number) => {
		return Math.max(1, Math.min(maxDays, Math.floor(columns / bucketsPerDay)))
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
			const stats = await fetchHeatmapStats(days)
			if (requestToken !== latestRequestToken) {
				return
			}
			if (granularity.value !== currentGranularity) {
				return
			}
			heatmapData.value = buildUsageHeatmapMatrix(stats, days, currentGranularity)
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
		const bucketsPerDay = bucketsPerDayByGranularity(granularity.value)
		const maxDays = maxDaysByGranularity(granularity.value)
		const newColumns = calculateColumns(width, bucketsPerDay)

		// 只有列数变化时才更新
		if (newColumns !== visibleColumns.value) {
			const newDays = calculateDaysFromColumns(newColumns, bucketsPerDay, maxDays)
			visibleColumns.value = newColumns

			// 只在需要更多数据时重新请求
			if (newDays > loadedDays.value || loadedGranularity.value !== granularity.value) {
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

	/**
	 * 初始化热力图
	 */
	const init = async () => {
		const container = containerRef.value
		if (!container) return

		const currentGranularity = granularity.value
		const bucketsPerDay = bucketsPerDayByGranularity(currentGranularity)
		const maxDays = maxDaysByGranularity(currentGranularity)

		// 初始宽度计算
		const initialWidth = container.clientWidth
		containerWidth.value = initialWidth
		const initialColumns = calculateColumns(initialWidth, bucketsPerDay)
		visibleColumns.value = initialColumns

		// 加载初始数据
		const initialDays = calculateDaysFromColumns(initialColumns, bucketsPerDay, maxDays)
		await loadHeatmapData(initialDays)

		// 设置 ResizeObserver
		resizeObserver = new ResizeObserver((entries) => {
			for (const entry of entries) {
				const { width } = entry.contentRect
				handleResize(width)
			}
		})
		resizeObserver.observe(container)
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
		const bucketsPerDay = bucketsPerDayByGranularity(granularity.value)
		const maxDays = maxDaysByGranularity(granularity.value)
		const days = calculateDaysFromColumns(visibleColumns.value, bucketsPerDay, maxDays)
		await loadHeatmapData(days)
	}

	watch(granularity, (nextGranularity) => {
		loadedDays.value = 0
		loadedGranularity.value = null
		const bucketsPerDay = bucketsPerDayByGranularity(nextGranularity)
		const maxDays = maxDaysByGranularity(nextGranularity)
		const defaultDays = defaultDaysByGranularity(nextGranularity)
		if (!initialized.value) {
			visibleColumns.value = defaultDays * bucketsPerDay
			heatmapData.value = generateFallbackUsageHeatmap(defaultDays, nextGranularity)
			return
		}
		const width = containerRef.value?.clientWidth ?? containerWidth.value
		if (width > 0) {
			const columns = calculateColumns(width, bucketsPerDay)
			visibleColumns.value = columns
			const days = calculateDaysFromColumns(columns, bucketsPerDay, maxDays)
			void loadHeatmapData(days)
			return
		}
		visibleColumns.value = defaultDays * bucketsPerDay
		heatmapData.value = generateFallbackUsageHeatmap(defaultDays, nextGranularity)
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
