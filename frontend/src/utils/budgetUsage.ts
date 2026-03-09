export type BudgetCycleMode = 'daily' | 'weekly' | 'monthly'

export type BudgetUsageConfig = {
  cycleEnabled: boolean
  cycleMode: BudgetCycleMode
  refreshTime: string
  refreshWeekday: number
  refreshMonthDay: number
}

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
