<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, proxyRefs, ref } from 'vue'
import { Call } from '@wailsio/runtime'
import { fetchCostSince, fetchFiveHourQuotaStatus, fetchLogStats } from '../../services/logs'
import { fetchAppSettings, type AppSettings } from '../../services/appSettings'
import { fetchProxyStatus } from '../../services/claudeSettings'
import {
  getVisibleTrayQuotaKeys,
  resolveTrayBudgetDisplayMode,
  type TrayBudgetDisplayMode,
} from '../../utils/trayBudgetDisplay'
import {
  formatClockCountdown,
  shouldUseSecondPrecisionTrayTicker,
  updateItemsAndCollectRefresh,
} from '../../utils/trayCountdown'
import {
  budgetQuotaOrder,
  createDefaultBudgetQuotaAdjustments,
  formatLocalDateTime,
  pad2,
  resolveBudgetQuotaWindow,
  startOfDay,
  type BudgetQuotaAdjustments,
  type BudgetQuotaKey,
  type BudgetQuotaSetting,
  type BudgetQuotaSettings,
} from '../../utils/budgetUsage'

type Platform = 'claude' | 'codex'
type ForecastMethod = 'cycle' | '10m' | '1h' | 'yesterday' | 'last24h'
type ForecastDisplay = 'datetime' | 'remaining'
type CostSinceFetcher = (start: Date) => Promise<number>

type TrayQuotaState = {
  key: BudgetQuotaKey
  title: string
  rawUsed: number
  used: number
  total: number
  usedLabel: string
  totalLabel: string
  hasBudget: boolean
  progressRatio: number
  progressPercentLabel: string
  countdownLabel: string
  forecastLabel: string
  windowStart: Date | null
  nextReset: Date | null
  forecastRate: number
}

const rootRef = ref<HTMLElement | null>(null)
const FULL_REFRESH_INTERVAL_MS = 60_000
const RESET_REFRESH_COOLDOWN_MS = 5_000
let ticker: number | undefined
let refreshBusy = false
let storageRefreshTimer: number | undefined
let lastWindowHeight = 0
let lastFullRefreshAt = 0
let lastRefreshAttemptAt = 0

const quotaTitles: Record<BudgetQuotaKey, string> = {
  five_hour: '5 小时额度',
  daily: '日额度',
  weekly: '周额度',
  monthly: '月额度',
  total: '总额度',
}

const formatCurrency = (value?: number) => {
  if (value === undefined || value === null || Number.isNaN(value)) {
    return '$0.0000'
  }
  if (value >= 1) {
    return `$${value.toFixed(2)}`
  }
  if (value >= 0.01) {
    return `$${value.toFixed(3)}`
  }
  return `$${value.toFixed(4)}`
}

const formatLocalDateTimeLabel = (date: Date) => {
  return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())} ${pad2(date.getHours())}:${pad2(date.getMinutes())}`
}

const parseTrayDateTime = (value?: string | null) => {
  const raw = String(value ?? '').trim()
  if (!raw) return null
  const normalized = raw.replace(' ', 'T')
  const attempts = [raw, normalized, `${normalized}Z`]
  for (const candidate of attempts) {
    const parsed = new Date(candidate)
    if (!Number.isNaN(parsed.getTime())) {
      return parsed
    }
  }
  return null
}

const formatCountdown = (remainingMs: number) => {
  const totalMinutes = Math.max(Math.floor(remainingMs / 60000), 0)
  const days = Math.floor(totalMinutes / (24 * 60))
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60)
  const minutes = totalMinutes % 60
  return `${pad2(days)}天 ${pad2(hours)}:${pad2(minutes)}`
}

const calculateRate = (cost: number, seconds: number) => {
  if (!Number.isFinite(cost) || !Number.isFinite(seconds) || seconds <= 0) return 0
  return Math.max(cost, 0) / seconds
}

const normalizeForecastMethod = (value: unknown): ForecastMethod => {
  const raw = String(value ?? '').trim()
  if (raw === 'cycle' || raw === '10m' || raw === '1h' || raw === 'yesterday' || raw === 'last24h') {
    return raw
  }
  return 'cycle'
}

const normalizeForecastDisplay = (value: unknown): ForecastDisplay => {
  const raw = String(value ?? '').trim()
  if (raw === 'datetime' || raw === 'remaining') {
    return raw
  }
  return 'datetime'
}

const createQuotaState = (key: BudgetQuotaKey): TrayQuotaState => ({
  key,
  title: quotaTitles[key],
  rawUsed: 0,
  used: 0,
  total: 0,
  usedLabel: formatCurrency(0),
  totalLabel: '∞',
  hasBudget: false,
  progressRatio: 0,
  progressPercentLabel: '',
  countdownLabel: '',
  forecastLabel: '',
  windowStart: null,
  nextReset: null,
  forecastRate: 0,
})

const createCostSinceFetcher = (platform: Platform): CostSinceFetcher => {
  const cache = new Map<string, Promise<number>>()
  return async (start: Date) => {
    const key = formatLocalDateTime(start)
    if (!cache.has(key)) {
      cache.set(
        key,
        fetchCostSince(key, platform)
          .then((value) => {
            const numeric = Number(value)
            return Number.isFinite(numeric) ? numeric : 0
          })
          .catch((error) => {
            console.error(`failed to load ${platform} cost since ${key}`, error)
            return 0
          }),
      )
    }
    return cache.get(key)!
  }
}

const createTrayCard = (platform: Platform, brandName: string, brandIcon: string) => {
  const quotas = ref<TrayQuotaState[]>(budgetQuotaOrder.map((key) => createQuotaState(key)))
  const loading = ref(false)
  const showCountdown = ref(false)
  const showForecast = ref(false)
  const forecastMethod = ref<ForecastMethod>('cycle')
  const forecastDisplay = ref<ForecastDisplay>('datetime')
  const usedAdjustments = ref<BudgetQuotaAdjustments>(createDefaultBudgetQuotaAdjustments())
  const totalUsage = ref(0)
  const displayMode = ref<TrayBudgetDisplayMode | 'pending'>('pending')
  const visibleQuotaKeys = ref<BudgetQuotaKey[]>([])
  const hostingEnabled = ref(false)
  const hostingLabel = computed(() => (hostingEnabled.value ? '托管中' : '未托管'))
  const visibleQuotas = computed(() => {
    if (displayMode.value !== 'quotas') return []
    const allowedKeys = new Set(visibleQuotaKeys.value)
    return quotas.value.filter((quota) => quota.hasBudget && allowedKeys.has(quota.key))
  })
  const hasSecondPrecisionCountdown = computed(() => (
    shouldUseSecondPrecisionTrayTicker(displayMode.value, showCountdown.value, quotas.value)
  ))
  const showTotalUsage = computed(() => displayMode.value === 'summary')
  const totalUsageLabel = computed(() => formatCurrency(totalUsage.value))

  const applyUsedAdjustment = (key: BudgetQuotaKey, rawUsed: number) => {
    const adjusted = rawUsed + usedAdjustments.value[key]
    if (!Number.isFinite(adjusted)) return 0
    return Math.max(adjusted, 0)
  }

  const clampStartToWindow = (start: Date, windowStart: Date | null) => {
    if (windowStart && start < windowStart) {
      return windowStart
    }
    return start
  }

  const updateQuotaStaticLabels = (quota: TrayQuotaState) => {
    quota.usedLabel = formatCurrency(quota.used)
    quota.hasBudget = quota.total > 0
    quota.totalLabel = quota.hasBudget ? formatCurrency(quota.total) : '∞'
    quota.progressRatio = quota.hasBudget ? Math.min(Math.max(quota.used / quota.total, 0), 1) : 0
    quota.progressPercentLabel = quota.hasBudget ? `${Math.round(quota.progressRatio * 100)}%` : ''
  }

  const updateQuotaTimeLabels = (quota: TrayQuotaState, now: Date) => {
    if (showCountdown.value && quota.hasBudget && quota.nextReset) {
      const remaining = quota.nextReset.getTime() - now.getTime()
      quota.countdownLabel = remaining > 0
        ? `重置倒计时 ${quota.key === 'five_hour' ? formatClockCountdown(remaining) : formatCountdown(remaining)}`
        : '即将重置'
    } else {
      quota.countdownLabel = ''
    }

    if (showForecast.value && quota.total > 0) {
      const rate = quota.forecastRate
      if (rate > 0 && quota.used < quota.total) {
        const secondsToBudget = (quota.total - quota.used) / rate
        if (!Number.isFinite(secondsToBudget)) {
          quota.forecastLabel = '预计耗尽 —'
        } else if (forecastDisplay.value === 'remaining') {
          quota.forecastLabel = `预计耗尽 ${formatCountdown(secondsToBudget * 1000)}`
        } else {
          quota.forecastLabel = `预计耗尽 ${formatLocalDateTimeLabel(new Date(now.getTime() + secondsToBudget * 1000))}`
        }
      } else if (quota.used >= quota.total) {
        quota.forecastLabel = '已达预算'
      } else {
        quota.forecastLabel = '预计耗尽 —'
      }
    } else {
      quota.forecastLabel = ''
    }

    return Boolean(quota.hasBudget && quota.nextReset && now >= quota.nextReset && !loading.value)
  }

  const computeForecastRate = async (
    quota: TrayQuotaState,
    now: Date,
    fetchCostSinceByStart: CostSinceFetcher,
  ) => {
    const method = forecastMethod.value
    if (method === 'cycle') {
      const start = quota.windowStart ?? startOfDay(now)
      const elapsedSeconds = Math.max((now.getTime() - start.getTime()) / 1000, 1)
      return calculateRate(quota.rawUsed, elapsedSeconds)
    }
    if (method === '10m') {
      const start = clampStartToWindow(new Date(now.getTime() - 10 * 60 * 1000), quota.windowStart)
      const cost = await fetchCostSinceByStart(start)
      return calculateRate(cost, (now.getTime() - start.getTime()) / 1000)
    }
    if (method === '1h') {
      const start = clampStartToWindow(new Date(now.getTime() - 60 * 60 * 1000), quota.windowStart)
      const cost = await fetchCostSinceByStart(start)
      return calculateRate(cost, (now.getTime() - start.getTime()) / 1000)
    }
    if (method === 'yesterday') {
      const todayStart = startOfDay(now)
      const yesterdayStart = new Date(todayStart)
      yesterdayStart.setDate(yesterdayStart.getDate() - 1)
      const [costSinceYesterday, costSinceToday] = await Promise.all([
        fetchCostSinceByStart(yesterdayStart),
        fetchCostSinceByStart(todayStart),
      ])
      return calculateRate(Math.max(costSinceYesterday - costSinceToday, 0), 24 * 60 * 60)
    }
    const windowStart = new Date(now.getTime() - 24 * 60 * 60 * 1000)
    const cost = await fetchCostSinceByStart(windowStart)
    return calculateRate(cost, (now.getTime() - windowStart.getTime()) / 1000)
  }

  const updateHostingState = async () => {
    try {
      const status = await fetchProxyStatus(platform)
      hostingEnabled.value = Boolean(status?.enabled)
    } catch (error) {
      console.error(`failed to load ${platform} proxy status`, error)
    }
  }

  const getQuotaSettings = (settings: AppSettings): BudgetQuotaSettings => {
    return platform === 'codex'
      ? settings.budget_quota_settings_codex
      : settings.budget_quota_settings
  }

  const getQuotaAdjustments = (settings: AppSettings): BudgetQuotaAdjustments => {
    return platform === 'codex'
      ? settings.budget_quota_used_adjustments_codex
      : settings.budget_quota_used_adjustments
  }

  const loadTotalUsage = async () => {
    try {
      const stats = await fetchLogStats(platform)
      const numeric = Number(stats?.cost_total)
      totalUsage.value = Number.isFinite(numeric) ? Math.max(numeric, 0) : 0
    } catch (error) {
      console.error(`failed to load ${platform} total usage`, error)
      totalUsage.value = 0
    }
  }

  const applySettings = (settings: AppSettings) => {
    if (platform === 'codex') {
      showCountdown.value = settings?.budget_show_countdown_codex ?? false
      showForecast.value = settings?.budget_show_forecast_codex ?? false
      forecastMethod.value = normalizeForecastMethod(settings?.budget_forecast_method_codex ?? 'cycle')
      forecastDisplay.value = normalizeForecastDisplay(settings?.budget_forecast_display_codex ?? 'datetime')
      usedAdjustments.value = settings?.budget_quota_used_adjustments_codex ?? createDefaultBudgetQuotaAdjustments()
      return
    }
    showCountdown.value = settings?.budget_show_countdown ?? false
    showForecast.value = settings?.budget_show_forecast ?? false
    forecastMethod.value = normalizeForecastMethod(settings?.budget_forecast_method ?? 'cycle')
    forecastDisplay.value = normalizeForecastDisplay(settings?.budget_forecast_display ?? 'datetime')
    usedAdjustments.value = settings?.budget_quota_used_adjustments ?? createDefaultBudgetQuotaAdjustments()
  }

  const refresh = async (settings: AppSettings) => {
    loading.value = true
    try {
      applySettings(settings)
      const now = new Date()
      const quotaSettings = getQuotaSettings(settings)
      usedAdjustments.value = getQuotaAdjustments(settings)
      await updateHostingState()
      const nextDisplayMode = resolveTrayBudgetDisplayMode(quotaSettings)

      if (nextDisplayMode === 'summary') {
        quotas.value = budgetQuotaOrder.map((key) => createQuotaState(key))
        visibleQuotaKeys.value = []
        displayMode.value = 'summary'
        await loadTotalUsage()
        return
      }

      const nextVisibleQuotaKeys = getVisibleTrayQuotaKeys(quotaSettings)
      const visibleQuotaKeySet = new Set(nextVisibleQuotaKeys)
      const fetchCostSinceByStart = createCostSinceFetcher(platform)
      quotas.value = await Promise.all(
        budgetQuotaOrder.map(async (key) => {
          if (!visibleQuotaKeySet.has(key)) {
            return createQuotaState(key)
          }
          const setting = quotaSettings[key] as BudgetQuotaSetting
          if (key === 'five_hour') {
            const snapshot = await fetchFiveHourQuotaStatus(platform)
            const rawUsed = Number.isFinite(Number(snapshot?.used)) ? Math.max(Number(snapshot.used), 0) : 0
            const nextQuota = createQuotaState(key)
            nextQuota.rawUsed = rawUsed
            nextQuota.used = applyUsedAdjustment(key, rawUsed)
            nextQuota.total = setting.total
            nextQuota.windowStart = snapshot?.active ? parseTrayDateTime(snapshot.window_start) : null
            nextQuota.nextReset = snapshot?.active ? parseTrayDateTime(snapshot.next_reset) : null
            nextQuota.forecastRate = showForecast.value && setting.total > 0
              ? await computeForecastRate(nextQuota, now, fetchCostSinceByStart)
              : 0
            updateQuotaStaticLabels(nextQuota)
            updateQuotaTimeLabels(nextQuota, now)
            return nextQuota
          }
          const window = resolveBudgetQuotaWindow(key, setting, now)
          const rawUsed = await fetchCostSinceByStart(window.start)
          const used = applyUsedAdjustment(key, rawUsed)
          const nextQuota = createQuotaState(key)
          nextQuota.rawUsed = rawUsed
          nextQuota.used = used
          nextQuota.total = setting.total
          nextQuota.windowStart = window.start
          nextQuota.nextReset = window.nextReset
          nextQuota.forecastRate = showForecast.value && setting.total > 0
            ? await computeForecastRate(nextQuota, now, fetchCostSinceByStart)
            : 0
          updateQuotaStaticLabels(nextQuota)
          updateQuotaTimeLabels(nextQuota, now)
          return nextQuota
        }),
      )
      visibleQuotaKeys.value = nextVisibleQuotaKeys
      displayMode.value = 'quotas'
      totalUsage.value = 0
    } catch (error) {
      console.error(`failed to load ${platform} tray stats`, error)
    } finally {
      loading.value = false
    }
  }

  const updateDerivedLabels = (now: Date) => {
    return updateItemsAndCollectRefresh(quotas.value, (quota) => updateQuotaTimeLabels(quota, now))
  }

  return proxyRefs({
    platform,
    brandName,
    brandIcon,
    quotas,
    visibleQuotas,
    showTotalUsage,
    totalUsageLabel,
    hostingEnabled,
    hostingLabel,
    loading,
    refresh,
    hasSecondPrecisionCountdown,
    updateDerivedLabels,
  })
}

const claudeCard = createTrayCard('claude', 'Claude Code', 'C')
const codexCard = createTrayCard('codex', 'Codex', 'X')
const cards = [claudeCard, codexCard]

const getTickerIntervalMs = () => (
  cards.some((card) => card.hasSecondPrecisionCountdown) ? 1000 : FULL_REFRESH_INTERVAL_MS
)

const triggerRefreshAll = () => {
  if (refreshBusy) return
  lastRefreshAttemptAt = Date.now()
  void refreshAll()
}

const refreshAllIfDue = () => {
  if (Date.now() - lastFullRefreshAt < FULL_REFRESH_INTERVAL_MS) return
  triggerRefreshAll()
}

const refreshAllForResetIfDue = () => {
  if (Date.now() - lastRefreshAttemptAt < RESET_REFRESH_COOLDOWN_MS) return
  triggerRefreshAll()
}

const updateAllDerivedLabels = () => {
  const now = new Date()
  const shouldRefresh = updateItemsAndCollectRefresh(cards, (card) => card.updateDerivedLabels(now))
  if (shouldRefresh) {
    refreshAllForResetIfDue()
  }
}

const setupTicker = () => {
  if (ticker) {
    window.clearInterval(ticker)
  }
  ticker = window.setInterval(() => {
    if (document.hidden) return
    updateAllDerivedLabels()
    refreshAllIfDue()
  }, getTickerIntervalMs())
}

const resizeToContent = async () => {
  await nextTick()
  if (!rootRef.value) return
  const height = Math.ceil(Math.max(
    rootRef.value.getBoundingClientRect().height,
    rootRef.value.scrollHeight,
  ))
  if (height <= 0) return
  if (height === lastWindowHeight) return
  try {
    await Call.ByName('main.AppService.SetTrayWindowHeight', height)
    lastWindowHeight = height
  } catch (error) {
    console.error('failed to resize tray window', error)
  }
}

const refreshAll = async () => {
  if (refreshBusy) return
  refreshBusy = true
  lastRefreshAttemptAt = Date.now()
  try {
    const settings = await fetchAppSettings()
    await Promise.all(cards.map((card) => card.refresh(settings)))
  } catch (error) {
    console.error('failed to refresh tray cards', error)
  } finally {
    refreshBusy = false
    lastFullRefreshAt = Date.now()
    lastRefreshAttemptAt = lastFullRefreshAt
    updateAllDerivedLabels()
    setupTicker()
    await resizeToContent()
  }
}

const handleFocus = () => {
  triggerRefreshAll()
}

const handleVisibilityChange = () => {
  if (document.hidden || refreshBusy) return
  triggerRefreshAll()
}

const scheduleRefreshAll = () => {
  if (storageRefreshTimer) {
    window.clearTimeout(storageRefreshTimer)
  }
  storageRefreshTimer = window.setTimeout(() => {
    storageRefreshTimer = undefined
    if (refreshBusy) {
      scheduleRefreshAll()
      return
    }
    triggerRefreshAll()
  }, 80)
}

const handleStorageChange = (event: StorageEvent) => {
  const key = event?.key
  if (!key || !key.startsWith('app-settings-')) return
  scheduleRefreshAll()
}

onMounted(() => {
  lastWindowHeight = 0
  triggerRefreshAll()
  window.addEventListener('focus', handleFocus)
  window.addEventListener('visibilitychange', handleVisibilityChange)
  window.addEventListener('app-settings-updated', handleFocus)
  window.addEventListener('storage', handleStorageChange)
})

onUnmounted(() => {
  if (ticker) {
    window.clearInterval(ticker)
    ticker = undefined
  }
  if (storageRefreshTimer) {
    window.clearTimeout(storageRefreshTimer)
    storageRefreshTimer = undefined
  }
  lastWindowHeight = 0
  window.removeEventListener('focus', handleFocus)
  window.removeEventListener('visibilitychange', handleVisibilityChange)
  window.removeEventListener('app-settings-updated', handleFocus)
  window.removeEventListener('storage', handleStorageChange)
})
</script>

<template>
  <div ref="rootRef" class="tray-root">
    <div class="tray-list">
      <div v-for="card in cards" :key="card.platform" class="tray-panel">
        <div class="tray-header">
          <div class="tray-brand">
            <div class="tray-brand__icon" aria-hidden="true">{{ card.brandIcon }}</div>
            <span class="tray-brand__name">{{ card.brandName }}</span>
          </div>
          <div class="tray-status" :class="{ active: card.hostingEnabled }">
            <span class="tray-status__dot"></span>
            <span class="tray-status__text">{{ card.hostingLabel }}</span>
          </div>
        </div>
        <div class="tray-content">
          <div v-if="card.showTotalUsage" class="tray-item tray-item--summary">
            <div class="tray-item__header">
              <div class="tray-item__title">
                <span class="tray-dot"></span>
                <span>总消耗</span>
              </div>
              <div class="tray-item__value" :class="{ loading: card.loading }">
                <span>{{ card.totalUsageLabel }}</span>
              </div>
            </div>
          </div>
          <div v-for="quota in card.visibleQuotas" :key="`${card.platform}-${quota.key}`" class="tray-item">
            <div class="tray-item__header">
              <div class="tray-item__title">
                <span class="tray-dot"></span>
                <span>{{ quota.title }}</span>
              </div>
              <div class="tray-item__summary">
                <div class="tray-item__value" :class="{ loading: card.loading }">
                  <span>已用 {{ quota.usedLabel }}</span>
                  <span class="tray-divider">/</span>
                  <span>{{ quota.totalLabel }}</span>
                </div>
                <span v-if="quota.hasBudget" class="tray-item__percent">{{ quota.progressPercentLabel }}</span>
              </div>
            </div>
            <div v-if="quota.hasBudget" class="tray-progress">
              <div class="tray-progress__bar" :style="{ width: `${quota.progressRatio * 100}%` }"></div>
            </div>
            <div v-if="quota.countdownLabel || quota.forecastLabel" class="tray-meta">
              <span v-if="quota.countdownLabel">{{ quota.countdownLabel }}</span>
              <span v-if="quota.forecastLabel">{{ quota.forecastLabel }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tray-root {
  padding: 10px;
  color: var(--mac-text);
  box-sizing: border-box;
  max-height: 100vh;
  overflow-y: auto;
  overflow-x: hidden;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
}

.tray-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.tray-panel {
  background: var(--mac-surface);
  border-radius: 16px;
  padding: 12px 14px;
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.12);
  border: 1px solid var(--mac-border);
}

.tray-content {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.tray-item {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.tray-item + .tray-item {
  padding-top: 10px;
  border-top: 1px solid var(--mac-divider);
}

.tray-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 8px;
  margin-bottom: 10px;
  border-bottom: 1px solid var(--mac-divider);
}

.tray-brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.tray-brand__icon {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  background: var(--mac-surface-strong);
  border: 1px solid var(--mac-border);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  font-weight: 700;
  color: var(--mac-text);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08);
}

.tray-brand__name {
  font-size: 13px;
  font-weight: 600;
  color: var(--mac-text);
}

.tray-status {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--mac-text-secondary);
}

.tray-status__dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--mac-text-secondary) 55%, transparent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--mac-text-secondary) 25%, transparent);
}

.tray-status.active {
  color: var(--mac-text);
}

.tray-status.active .tray-status__dot {
  background: #5dbb63;
  box-shadow: 0 0 0 2px rgba(93, 187, 99, 0.25);
}

.tray-item__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.tray-item__summary {
  display: flex;
  align-items: center;
  gap: 10px;
}

.tray-item__title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--mac-text);
}

.tray-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: #5dbb63;
  box-shadow: 0 0 0 2px rgba(93, 187, 99, 0.2);
}

.tray-item__value {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--mac-text-secondary);
}

.tray-item__value.loading {
  opacity: 0.6;
}

.tray-divider {
  opacity: 0.5;
}

.tray-item__percent {
  font-size: 12px;
  font-weight: 600;
  color: #5dbb63;
  min-width: 36px;
  text-align: right;
}

.tray-progress {
  width: 100%;
  height: 8px;
  border-radius: 999px;
  background: var(--mac-divider);
  overflow: hidden;
}

.tray-progress__bar {
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #5dbb63 0%, #6bd36f 100%);
  transition: width 0.2s ease;
}

.tray-meta {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: var(--mac-text-secondary);
}

:global(.dark) .tray-panel {
  background: rgba(28, 30, 36, 0.94);
  border-color: rgba(255, 255, 255, 0.12);
  box-shadow: 0 14px 28px rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(10px);
}

:global(.dark) .tray-header {
  border-bottom-color: var(--mac-divider);
}

:global(.dark) .tray-item + .tray-item {
  border-top-color: var(--mac-divider);
}

:global(.dark) .tray-brand__icon {
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06);
}

:global(.dark) .tray-brand__name {
  color: var(--mac-text);
}

:global(.dark) .tray-status {
  color: var(--mac-text-secondary);
}

:global(.dark) .tray-status__dot {
  background: color-mix(in srgb, var(--mac-text-secondary) 55%, transparent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--mac-text-secondary) 25%, transparent);
}

:global(.dark) .tray-status.active {
  color: var(--mac-text);
}

:global(.dark) .tray-status.active .tray-status__dot {
  background: #7ce07f;
  box-shadow: 0 0 0 2px rgba(124, 224, 127, 0.3);
}

:global(.dark) .tray-item__title {
  color: var(--mac-text);
}

:global(.dark) .tray-item__value {
  color: var(--mac-text-secondary);
}

:global(.dark) .tray-item__percent {
  color: #7ce07f;
}

:global(.dark) .tray-progress {
  background: var(--mac-divider);
}

:global(.dark) .tray-progress__bar {
  background: linear-gradient(90deg, #5dbb63 0%, #7ce07f 100%);
}

:global(.dark) .tray-meta {
  color: var(--mac-text-secondary);
}
</style>
