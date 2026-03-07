export type BudgetCycleMode = 'daily' | 'weekly'

export type BudgetUsageConfig = {
  cycleEnabled: boolean
  cycleMode: BudgetCycleMode
  refreshTime: string
  refreshDay: number
}

export const pad2 = (value: number) => String(value).padStart(2, '0')

export const formatLocalDateTime = (date: Date) => {
  return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())} ${pad2(date.getHours())}:${pad2(date.getMinutes())}:${pad2(date.getSeconds())}`
}

export const normalizeBudgetCycleMode = (value: unknown): BudgetCycleMode => {
  return String(value ?? '').trim().toLowerCase() === 'weekly' ? 'weekly' : 'daily'
}

export const normalizeBudgetRefreshDay = (value: unknown) => {
  const numeric = Number(value ?? 1)
  if (!Number.isFinite(numeric)) return 1
  return Math.min(Math.max(Math.floor(numeric), 0), 6)
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

export const resolveCycleStart = (config: BudgetUsageConfig, now: Date) => {
  if (!config.cycleEnabled) {
    return startOfDay(now)
  }
  const { hour, minute } = parseRefreshTime(config.refreshTime)
  if (config.cycleMode === 'weekly') {
    const desiredDay = normalizeBudgetRefreshDay(config.refreshDay)
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
  const start = new Date(now)
  start.setHours(hour, minute, 0, 0)
  if (now < start) {
    start.setDate(start.getDate() - 1)
  }
  return start
}

export const buildBudgetUsageConfig = (
  cycleEnabled: boolean,
  cycleMode: unknown,
  refreshTime: unknown,
  refreshDay: unknown,
): BudgetUsageConfig => {
  return {
    cycleEnabled,
    cycleMode: normalizeBudgetCycleMode(cycleMode),
    refreshTime: normalizeBudgetRefreshTime(refreshTime),
    refreshDay: normalizeBudgetRefreshDay(refreshDay),
  }
}

export const getBudgetUsageConfigKey = (config: BudgetUsageConfig) => {
  const normalized = buildBudgetUsageConfig(
    config.cycleEnabled,
    config.cycleMode,
    config.refreshTime,
    config.refreshDay,
  )
  return `${normalized.cycleEnabled ? '1' : '0'}|${normalized.cycleMode}|${normalized.refreshTime}|${normalized.refreshDay}`
}
