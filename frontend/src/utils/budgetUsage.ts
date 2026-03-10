export type BudgetCycleMode = 'daily' | 'weekly' | 'monthly'
export type BudgetQuotaKey = 'five_hour' | 'daily' | 'weekly' | 'monthly'

export type BudgetUsageConfig = {
  cycleEnabled: boolean
  cycleMode: BudgetCycleMode
  refreshTime: string
  refreshWeekday: number
  refreshMonthDay: number
}

export type BudgetQuotaSetting = {
  total: number
  refreshTime: string
  refreshWeekday: number
  refreshMonthDay: number
}

type BudgetQuotaSettingSource = Partial<BudgetQuotaSetting> & {
  refresh_time?: unknown
  refresh_day?: unknown
  refresh_month_day?: unknown
}

export type BudgetQuotaSettings = Record<BudgetQuotaKey, BudgetQuotaSetting>
export type BudgetQuotaAdjustments = Record<BudgetQuotaKey, number>

export type BudgetQuotaWindow = {
  start: Date
  nextReset: Date | null
}

export type LegacyBudgetQuotaSource = {
  total?: unknown
  cycleEnabled?: unknown
  cycleMode?: unknown
  refreshTime?: unknown
  refreshWeekday?: unknown
  refreshMonthDay?: unknown
}

export type LegacyBudgetQuotaAdjustmentSource = {
  adjustment?: unknown
  cycleEnabled?: unknown
  cycleMode?: unknown
}

export type LegacyBudgetQuotaProjection = {
  total: number
  adjustment: number
  cycleEnabled: boolean
  cycleMode: BudgetCycleMode
  refreshTime: string
  refreshWeekday: number
  refreshMonthDay: number
}

export const budgetQuotaOrder: BudgetQuotaKey[] = ['five_hour', 'daily', 'weekly', 'monthly']

export const pad2 = (value: number) => String(value).padStart(2, '0')

export const formatLocalDateTime = (date: Date) => {
  return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())} ${pad2(date.getHours())}:${pad2(date.getMinutes())}:${pad2(date.getSeconds())}`
}

export const normalizeBudgetCycleMode = (value: unknown): BudgetCycleMode => {
  const normalized = String(value ?? '').trim().toLowerCase()
  if (normalized === 'weekly' || normalized === 'monthly') {
    return normalized
  }
  return 'daily'
}

export const normalizeBudgetRefreshWeekday = (value: unknown) => {
  const numeric = Number(value ?? 1)
  if (!Number.isFinite(numeric)) return 1
  return Math.min(Math.max(Math.floor(numeric), 0), 6)
}

export const normalizeBudgetRefreshMonthDay = (value: unknown) => {
  const numeric = Number(value ?? 1)
  if (!Number.isFinite(numeric)) return 1
  return Math.min(Math.max(Math.floor(numeric), 1), 31)
}

export const normalizeBudgetRefreshTime = (value: unknown) => {
  const normalized = String(value ?? '').trim()
  return normalized || '00:00'
}

export const normalizeBudgetUsedDisplay = (value: number) => {
  if (!Number.isFinite(value)) return 0
  return Math.max(value, 0)
}

export const normalizeBudgetQuotaTotal = (value: unknown) => {
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric)) return 0
  return Math.max(numeric, 0)
}

export const normalizeBudgetQuotaAdjustment = (value: unknown) => {
  const numeric = Number(value ?? 0)
  return Number.isFinite(numeric) ? numeric : 0
}

export const parseRefreshTime = (value: string) => {
  const [rawHour, rawMinute] = normalizeBudgetRefreshTime(value).split(':')
  const hour = Number(rawHour)
  const minute = Number(rawMinute)
  return {
    hour: Number.isFinite(hour) ? Math.min(Math.max(Math.floor(hour), 0), 23) : 0,
    minute: Number.isFinite(minute) ? Math.min(Math.max(Math.floor(minute), 0), 59) : 0,
  }
}

export const startOfDay = (date: Date) => {
  const base = new Date(date)
  base.setHours(0, 0, 0, 0)
  return base
}

const resolveMonthlyRefreshPoint = (
  year: number,
  monthIndex: number,
  desiredDay: number,
  hour: number,
  minute: number,
) => {
  const lastDay = new Date(year, monthIndex + 1, 0).getDate()
  const target = new Date(year, monthIndex, Math.min(desiredDay, lastDay))
  target.setHours(hour, minute, 0, 0)
  return target
}

export const resolveCycleStart = (config: BudgetUsageConfig, now: Date) => {
  if (!config.cycleEnabled) {
    return startOfDay(now)
  }
  const { hour, minute } = parseRefreshTime(config.refreshTime)
  if (config.cycleMode === 'weekly') {
    const desiredDay = normalizeBudgetRefreshWeekday(config.refreshWeekday)
    const target = new Date(now)
    const currentDay = target.getDay()
    const diff = desiredDay - currentDay
    target.setDate(target.getDate() + diff)
    target.setHours(hour, minute, 0, 0)
    if (now < target) {
      target.setDate(target.getDate() - 7)
    }
    return target
  }
  if (config.cycleMode === 'monthly') {
    const desiredDay = normalizeBudgetRefreshMonthDay(config.refreshMonthDay)
    const currentMonthTarget = resolveMonthlyRefreshPoint(
      now.getFullYear(),
      now.getMonth(),
      desiredDay,
      hour,
      minute,
    )
    if (now >= currentMonthTarget) {
      return currentMonthTarget
    }
    const previousMonth = new Date(now.getFullYear(), now.getMonth() - 1, 1)
    return resolveMonthlyRefreshPoint(
      previousMonth.getFullYear(),
      previousMonth.getMonth(),
      desiredDay,
      hour,
      minute,
    )
  }
  const start = new Date(now)
  start.setHours(hour, minute, 0, 0)
  if (now < start) {
    start.setDate(start.getDate() - 1)
  }
  return start
}

export const resolveNextCycleStart = (config: BudgetUsageConfig, currentStart: Date) => {
  const normalized = buildBudgetUsageConfig(
    config.cycleEnabled,
    config.cycleMode,
    config.refreshTime,
    config.refreshWeekday,
    config.refreshMonthDay,
  )
  if (normalized.cycleMode === 'weekly') {
    const next = new Date(currentStart)
    next.setDate(next.getDate() + 7)
    return next
  }
  if (normalized.cycleMode === 'monthly') {
    const { hour, minute } = parseRefreshTime(normalized.refreshTime)
    const nextMonth = new Date(currentStart.getFullYear(), currentStart.getMonth() + 1, 1)
    return resolveMonthlyRefreshPoint(
      nextMonth.getFullYear(),
      nextMonth.getMonth(),
      normalized.refreshMonthDay,
      hour,
      minute,
    )
  }
  const next = new Date(currentStart)
  next.setDate(next.getDate() + 1)
  return next
}

export const buildBudgetUsageConfig = (
  cycleEnabled: boolean,
  cycleMode: unknown,
  refreshTime: unknown,
  refreshWeekday: unknown,
  refreshMonthDay: unknown,
): BudgetUsageConfig => {
  return {
    cycleEnabled,
    cycleMode: normalizeBudgetCycleMode(cycleMode),
    refreshTime: normalizeBudgetRefreshTime(refreshTime),
    refreshWeekday: normalizeBudgetRefreshWeekday(refreshWeekday),
    refreshMonthDay: normalizeBudgetRefreshMonthDay(refreshMonthDay),
  }
}

export const getBudgetUsageConfigKey = (config: BudgetUsageConfig) => {
  const normalized = buildBudgetUsageConfig(
    config.cycleEnabled,
    config.cycleMode,
    config.refreshTime,
    config.refreshWeekday,
    config.refreshMonthDay,
  )
  return `${normalized.cycleEnabled ? '1' : '0'}|${normalized.cycleMode}|${normalized.refreshTime}|${normalized.refreshWeekday}|${normalized.refreshMonthDay}`
}

export const createDefaultBudgetQuotaSetting = (): BudgetQuotaSetting => ({
  total: 0,
  refreshTime: '00:00',
  refreshWeekday: 1,
  refreshMonthDay: 1,
})

export const createDefaultBudgetQuotaSettings = (): BudgetQuotaSettings => ({
  five_hour: createDefaultBudgetQuotaSetting(),
  daily: createDefaultBudgetQuotaSetting(),
  weekly: createDefaultBudgetQuotaSetting(),
  monthly: createDefaultBudgetQuotaSetting(),
})

export const createDefaultBudgetQuotaAdjustments = (): BudgetQuotaAdjustments => ({
  five_hour: 0,
  daily: 0,
  weekly: 0,
  monthly: 0,
})

export const normalizeBudgetQuotaSetting = (value: unknown): BudgetQuotaSetting => {
  const source = value && typeof value === 'object'
    ? value as BudgetQuotaSettingSource
    : {}
  return {
    total: normalizeBudgetQuotaTotal(source.total),
    refreshTime: normalizeBudgetRefreshTime(source.refreshTime ?? source.refresh_time),
    refreshWeekday: normalizeBudgetRefreshWeekday(source.refreshWeekday ?? source.refresh_day),
    refreshMonthDay: normalizeBudgetRefreshMonthDay(source.refreshMonthDay ?? source.refresh_month_day),
  }
}

const isBudgetQuotaAdjustmentsEmpty = (adjustments: BudgetQuotaAdjustments) => {
  return budgetQuotaOrder.every((key) => adjustments[key] === 0)
}

const resolveLegacyBudgetQuotaKey = (
  legacy?: Pick<LegacyBudgetQuotaSource, 'cycleEnabled' | 'cycleMode'>,
): Exclude<BudgetQuotaKey, 'five_hour'> => {
  return legacy?.cycleEnabled
    ? normalizeBudgetCycleMode(legacy.cycleMode)
    : 'daily'
}

const applyLegacyBudgetQuotaAdjustment = (
  adjustments: BudgetQuotaAdjustments,
  legacy?: LegacyBudgetQuotaAdjustmentSource,
) => {
  const normalizedLegacy = normalizeBudgetQuotaAdjustment(legacy?.adjustment)
  if (normalizedLegacy === 0) return adjustments
  adjustments[resolveLegacyBudgetQuotaKey(legacy)] = normalizedLegacy
  return adjustments
}

export const normalizeBudgetQuotaAdjustments = (
  value: unknown,
  legacy?: LegacyBudgetQuotaAdjustmentSource,
): BudgetQuotaAdjustments => {
  const source = value && typeof value === 'object'
    ? value as Partial<Record<BudgetQuotaKey, unknown>>
    : {}
  const normalized: BudgetQuotaAdjustments = {
    five_hour: normalizeBudgetQuotaAdjustment(source.five_hour),
    daily: normalizeBudgetQuotaAdjustment(source.daily),
    weekly: normalizeBudgetQuotaAdjustment(source.weekly),
    monthly: normalizeBudgetQuotaAdjustment(source.monthly),
  }
  if (isBudgetQuotaAdjustmentsEmpty(normalized)) {
    return applyLegacyBudgetQuotaAdjustment(normalized, legacy)
  }
  return normalized
}

export const cloneBudgetQuotaAdjustments = (adjustments: unknown): BudgetQuotaAdjustments => {
  return normalizeBudgetQuotaAdjustments(adjustments)
}

const isBudgetQuotaSettingsEmpty = (settings: BudgetQuotaSettings) => {
  return budgetQuotaOrder.every((key) => settings[key].total <= 0)
}

const applyLegacyBudgetQuotaSource = (
  settings: BudgetQuotaSettings,
  legacy?: LegacyBudgetQuotaSource,
) => {
  if (!legacy) return settings
  const total = normalizeBudgetQuotaTotal(legacy.total)
  if (total <= 0) return settings
  const cycleMode = legacy.cycleEnabled ? normalizeBudgetCycleMode(legacy.cycleMode) : 'daily'
  settings[cycleMode] = {
    total,
    refreshTime: normalizeBudgetRefreshTime(legacy.refreshTime),
    refreshWeekday: normalizeBudgetRefreshWeekday(legacy.refreshWeekday),
    refreshMonthDay: normalizeBudgetRefreshMonthDay(legacy.refreshMonthDay),
  }
  return settings
}

export const normalizeBudgetQuotaSettings = (
  value: unknown,
  legacy?: LegacyBudgetQuotaSource,
): BudgetQuotaSettings => {
  const source = value && typeof value === 'object'
    ? value as Partial<Record<BudgetQuotaKey, BudgetQuotaSetting>>
    : {}
  const normalized: BudgetQuotaSettings = {
    five_hour: normalizeBudgetQuotaSetting(source.five_hour),
    daily: normalizeBudgetQuotaSetting(source.daily),
    weekly: normalizeBudgetQuotaSetting(source.weekly),
    monthly: normalizeBudgetQuotaSetting(source.monthly),
  }
  if (isBudgetQuotaSettingsEmpty(normalized)) {
    return applyLegacyBudgetQuotaSource(normalized, legacy)
  }
  return normalized
}

export const cloneBudgetQuotaSettings = (settings: unknown): BudgetQuotaSettings => {
  return normalizeBudgetQuotaSettings(settings)
}

export const projectBudgetQuotaToLegacy = (
  settings: unknown,
  adjustments: unknown,
): LegacyBudgetQuotaProjection => {
  const normalizedSettings = normalizeBudgetQuotaSettings(settings)
  const normalizedAdjustments = normalizeBudgetQuotaAdjustments(adjustments)

  for (const key of ['daily', 'weekly', 'monthly'] as const) {
    const setting = normalizedSettings[key]
    if (setting.total <= 0) continue
    return {
      total: setting.total,
      adjustment: normalizedAdjustments[key],
      cycleEnabled: true,
      cycleMode: key,
      refreshTime: setting.refreshTime,
      refreshWeekday: setting.refreshWeekday,
      refreshMonthDay: setting.refreshMonthDay,
    }
  }

  return {
    total: 0,
    adjustment: 0,
    cycleEnabled: false,
    cycleMode: 'daily',
    refreshTime: '00:00',
    refreshWeekday: 1,
    refreshMonthDay: 1,
  }
}

export const resolveBudgetQuotaWindow = (
  key: BudgetQuotaKey,
  setting: BudgetQuotaSetting,
  now: Date,
): BudgetQuotaWindow => {
  if (key === 'five_hour') {
    return {
      start: new Date(now.getTime() - 5 * 60 * 60 * 1000),
      nextReset: null,
    }
  }
  const config = buildBudgetUsageConfig(
    true,
    key,
    setting.refreshTime,
    setting.refreshWeekday,
    setting.refreshMonthDay,
  )
  const start = resolveCycleStart(config, now)
  return {
    start,
    nextReset: resolveNextCycleStart(config, start),
  }
}

export const getBudgetQuotaConfigKey = (key: BudgetQuotaKey, setting: BudgetQuotaSetting) => {
  const normalized = normalizeBudgetQuotaSetting(setting)
  return `${key}|${normalized.total}|${normalized.refreshTime}|${normalized.refreshWeekday}|${normalized.refreshMonthDay}`
}
