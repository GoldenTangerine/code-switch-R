export type HeatmapDailyIntensityMode = 'hourly_scaled' | 'daily_peak'

export type HeatmapDisplaySettings = {
	dailyScaleFactor: number
	dailyIntensityMode: HeatmapDailyIntensityMode
	intensityStopL1: number
	intensityStopL2: number
	intensityStopL3: number
}

const DEFAULT_DAILY_SCALE_FACTOR = 24
const MIN_DAILY_SCALE_FACTOR = 1
const MAX_DAILY_SCALE_FACTOR = 72

const DEFAULT_INTENSITY_STOP_L1 = 25
const DEFAULT_INTENSITY_STOP_L2 = 50
const DEFAULT_INTENSITY_STOP_L3 = 75

const MIN_INTENSITY_STOP = 1
const MAX_INTENSITY_STOP = 99

export const DEFAULT_HEATMAP_DISPLAY_SETTINGS: HeatmapDisplaySettings = {
	dailyScaleFactor: DEFAULT_DAILY_SCALE_FACTOR,
	dailyIntensityMode: 'hourly_scaled',
	intensityStopL1: DEFAULT_INTENSITY_STOP_L1,
	intensityStopL2: DEFAULT_INTENSITY_STOP_L2,
	intensityStopL3: DEFAULT_INTENSITY_STOP_L3,
}

const clampInteger = (value: unknown, min: number, max: number, fallback: number) => {
	const numeric = Number(value)
	if (!Number.isFinite(numeric)) return fallback
	return Math.min(max, Math.max(min, Math.round(numeric)))
}

const normalizeIntensityStops = (input: {
	intensityStopL1?: unknown
	intensityStopL2?: unknown
	intensityStopL3?: unknown
}) => {
	let l1 = clampInteger(
		input.intensityStopL1,
		MIN_INTENSITY_STOP,
		MAX_INTENSITY_STOP,
		DEFAULT_INTENSITY_STOP_L1,
	)
	let l2 = clampInteger(
		input.intensityStopL2,
		MIN_INTENSITY_STOP,
		MAX_INTENSITY_STOP,
		DEFAULT_INTENSITY_STOP_L2,
	)
	let l3 = clampInteger(
		input.intensityStopL3,
		MIN_INTENSITY_STOP,
		MAX_INTENSITY_STOP,
		DEFAULT_INTENSITY_STOP_L3,
	)

	if (l2 <= l1) {
		l2 = Math.min(MAX_INTENSITY_STOP, l1 + 1)
	}
	if (l3 <= l2) {
		l3 = Math.min(MAX_INTENSITY_STOP, l2 + 1)
	}
	if (l3 <= l2) {
		l2 = Math.max(MIN_INTENSITY_STOP, l3 - 1)
	}
	if (l2 <= l1) {
		l1 = Math.max(MIN_INTENSITY_STOP, l2 - 1)
	}

	return {
		intensityStopL1: l1,
		intensityStopL2: l2,
		intensityStopL3: l3,
	}
}

export const normalizeHeatmapDailyIntensityMode = (value?: unknown): HeatmapDailyIntensityMode =>
	value === 'daily_peak' ? 'daily_peak' : 'hourly_scaled'

export const normalizeHeatmapDisplaySettings = (
	value?: Partial<HeatmapDisplaySettings> | null,
): HeatmapDisplaySettings => {
	const next = value ?? {}
	const stops = normalizeIntensityStops(next)
	return {
		dailyScaleFactor: clampInteger(
			next.dailyScaleFactor,
			MIN_DAILY_SCALE_FACTOR,
			MAX_DAILY_SCALE_FACTOR,
			DEFAULT_DAILY_SCALE_FACTOR,
		),
		dailyIntensityMode: normalizeHeatmapDailyIntensityMode(next.dailyIntensityMode),
		intensityStopL1: stops.intensityStopL1,
		intensityStopL2: stops.intensityStopL2,
		intensityStopL3: stops.intensityStopL3,
	}
}

export const heatmapDisplaySettingsSignature = (
	value?: Partial<HeatmapDisplaySettings> | null,
) => {
	const normalized = normalizeHeatmapDisplaySettings(value)
	return [
		normalized.dailyScaleFactor,
		normalized.dailyIntensityMode,
		normalized.intensityStopL1,
		normalized.intensityStopL2,
		normalized.intensityStopL3,
	].join('|')
}

export const areHeatmapDisplaySettingsEqual = (
	left?: Partial<HeatmapDisplaySettings> | null,
	right?: Partial<HeatmapDisplaySettings> | null,
) => heatmapDisplaySettingsSignature(left) === heatmapDisplaySettingsSignature(right)
