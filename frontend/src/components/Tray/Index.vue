<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, proxyRefs, ref } from 'vue'
import { Call } from '@wailsio/runtime'
import { useI18n } from 'vue-i18n'
import { LoadProviders } from '../../../bindings/codeswitch/services/providerservice'
import { GetProviders as GetGeminiProviders } from '../../../bindings/codeswitch/services/geminiservice'
import {
  fetchCostSince,
  fetchFiveHourQuotaStatus,
  fetchLogStats,
  fetchProviderDailyStats,
  type RequestLogPlatform,
} from '../../services/logs'
import { fetchAppSettings, type AppSettings } from '../../services/appSettings'
import { fetchProxyStatus } from '../../services/claudeSettings'
import { getCustomCliProxyStatus, listCustomCliTools, type CustomCliTool } from '../../services/customCliService'
import type { AutomationCard } from '../../data/cards'
import { HOME_PROVIDER_TAB_OPTIONS } from '../../data/homeProviderTabs'
import {
  getVisibleTrayQuotaKeys,
  resolveTrayBudgetDisplayMode,
  type TrayBudgetDisplayMode,
} from '../../utils/trayBudgetDisplay'
import {
  formatClockCountdown,
  shouldUseSecondPrecisionTrayTicker,
  updateItemsAndCollectRefresh,
  type TrayCountdownQuota,
} from '../../utils/trayCountdown'
import {
  budgetQuotaOrder,
  createDefaultBudgetQuotaAdjustments,
  createDefaultBudgetQuotaSettings,
  formatLocalDateTime,
  pad2,
  resolveBudgetQuotaWindow,
  startOfDay,
  type BudgetQuotaAdjustments,
  type BudgetQuotaKey,
  type BudgetQuotaSetting,
  type BudgetQuotaSettings,
} from '../../utils/budgetUsage'
import { hasProviderQuotaQueryType } from '../../utils/providerQuotaQuery'
import {
  deserializeProviders,
  geminiToCard,
  type PersistedProvider,
} from '../Main/adapters/providerCardMappers'
import { resolveProviderQuotaQueryDisplay } from '../Main/utils/providerQuotaQueryDisplay'
import {
  resolveProviderQuotaSnapshot,
  type ProviderQuotaSnapshotItem,
} from '../Main/utils/providerQuotaSnapshot'
import {
  getProviderQuotaRemainingValue,
} from '../Main/utils/providerQuotaCardDisplay'
import {
  hasTrayFallbackProviderQuotaConfig,
  resolveTrayProviderQuotaDisplay,
} from './trayProviderFallback'
import {
  getProviderDisplayIconSvg,
  preloadProviderDisplayIcons,
} from '../../utils/providerIconAssets'
import {
  formatTrayCurrency,
  formatTrayCurrencyParts,
  formatTrayQuotaValue,
  formatTrayQuotaValueParts,
  joinTrayAmountParts,
  type TrayAmountPart,
  type TrayQuotaValueMode,
} from './trayAmountFormatter'
import {
  buildTrayProviderStatsDisplay,
  type TrayProviderStatsDisplay,
} from './trayProviderStats'
import { createTrayRefreshLifecycle } from './trayRefreshLifecycle'
import {
  buildTrayProviderActivityRefreshKey,
  hasTrayProviderActivityChanged,
  loadTrayProviderActivitySnapshot,
  resolveTrayProviderActivity,
  type TrayProviderActivity,
} from './trayProviderActivity'

type Platform = RequestLogPlatform
type ForecastMethod = 'cycle' | '10m' | '1h' | 'yesterday' | 'last24h'
type ForecastDisplay = 'datetime' | 'remaining'
type CostSinceFetcher = (start: Date) => Promise<number>

type TrayQuotaState = {
  key: string
  title: string
  rawUsed: number
  used: number
  total: number
  unlimited: boolean
  usedLabel: string
  totalLabel: string
  remainingParts: TrayAmountPart[]
  valueMode: TrayQuotaValueMode
  unit?: string
  extra: string
  invalidMessage: string
  source: 'global' | 'provider'
  displayKind: 'progress' | 'balance' | 'error'
  hasBudget: boolean
  progressRatio: number
  progressPercentLabel: string
  countdownLabel: string
  forecastLabel: string
  windowStart: Date | null
  nextReset: Date | null
  forecastRate: number
}

type TrayProviderBlock = {
  provider: AutomationCard | null
  providerId: string
  providerName: string
  providerIconKey: string
  providerIconSvg: string
  providerInitials: string
  activeRequests: number
  activityStatus: 'active' | 'default'
  quotas: TrayQuotaState[]
  stats: TrayProviderStatsDisplay | null
}

const rootRef = ref<HTMLElement | null>(null)
const FULL_REFRESH_INTERVAL_MS = 60_000
const RESET_REFRESH_COOLDOWN_MS = 5_000
let refreshBusy = false
let storageRefreshTimer: number | undefined
let lastWindowHeight = 0
let lastFullRefreshAt = 0
let lastRefreshAttemptAt = 0
let refreshLifecycle: ReturnType<typeof createTrayRefreshLifecycle> | undefined

const quotaTitleKeys: Record<BudgetQuotaKey, string> = {
  five_hour: 'tray.quotaFiveHour',
  daily: 'tray.quotaDaily',
  weekly: 'tray.quotaWeekly',
  monthly: 'tray.quotaMonthly',
  total: 'tray.quotaTotal',
}

const { t, locale } = useI18n()
const getTrayPlatformIconSvg = (iconKey: string) => getProviderDisplayIconSvg(iconKey)

const currentLocale = () => locale.value || 'en'
const formatCurrency = (value?: number, unit?: string) => formatTrayCurrency(value, unit, currentLocale())
const formatCurrencyParts = (value?: number, unit?: string) => formatTrayCurrencyParts(value, unit, currentLocale())
const formatQuotaValue = (
  value: number | undefined,
  valueMode: TrayQuotaValueMode = 'currency',
  unit?: string,
) => formatTrayQuotaValue(value, valueMode, unit, currentLocale())
const formatQuotaValueParts = (
  value: number | undefined,
  valueMode: TrayQuotaValueMode = 'currency',
  unit?: string,
) => formatTrayQuotaValueParts(value, valueMode, unit, currentLocale())

const getTrayProviderInitials = (name: string) => {
  if (!name) return 'AI'
  return name
    .split(/\s+/)
    .filter(Boolean)
    .map((word) => word[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()
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
  return t('tray.countdownDays', {
    days: pad2(days),
    time: `${pad2(hours)}:${pad2(minutes)}`,
  })
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
  title: t(quotaTitleKeys[key]),
  rawUsed: 0,
  used: 0,
  total: 0,
  unlimited: false,
  usedLabel: formatCurrency(0),
  totalLabel: '∞',
  remainingParts: formatCurrencyParts(0),
  valueMode: 'currency',
  unit: undefined,
  extra: '',
  invalidMessage: '',
  source: 'global',
  displayKind: 'progress',
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
  const providerBlocks = ref<TrayProviderBlock[]>([])
  const providerActivities = ref<TrayProviderActivity[]>([])
  const providerActivityLoaded = ref(false)
  const providerActivityError = ref(false)
  const providerByRef = new Map<string, AutomationCard>()
  const loading = ref(false)
  const showCountdown = ref(false)
  const showForecast = ref(false)
  const forecastMethod = ref<ForecastMethod>('cycle')
  const forecastDisplay = ref<ForecastDisplay>('datetime')
  const usedAdjustments = ref<BudgetQuotaAdjustments>(createDefaultBudgetQuotaAdjustments())
  const totalUsage = ref(0)
  const displayMode = ref<TrayBudgetDisplayMode | 'pending'>('pending')
  const visibleQuotaKeys = ref<BudgetQuotaKey[]>([])
  const brandIconSvg = computed(() => getTrayPlatformIconSvg(brandIcon))
  const hostingEnabled = ref(false)
  const hostingLabel = computed(() => (
    hostingEnabled.value ? t('tray.hosted') : t('tray.notHosted')
  ))
  const visibleQuotas = computed(() => {
    if (displayMode.value !== 'quotas') return []
    const allowedKeys = new Set(visibleQuotaKeys.value)
    return quotas.value.filter((quota) => quota.hasBudget && allowedKeys.has(quota.key as BudgetQuotaKey))
  })
  const providerBlockQuotas = computed(() => (
    providerBlocks.value.flatMap((block) => block.quotas)
  ))
  const hasSecondPrecisionCountdown = computed(() => (
    shouldUseSecondPrecisionTrayTicker(
      displayMode.value,
      showCountdown.value,
      quotas.value as TrayCountdownQuota[],
    )
    || shouldUseSecondPrecisionTrayTicker(
      'provider-quotas',
      true,
      providerBlockQuotas.value as TrayCountdownQuota[],
    )
  ))
  const showTotalUsage = computed(() => displayMode.value === 'summary')
  const totalUsageLabel = computed(() => formatCurrency(totalUsage.value))

  const providerActivityLabel = (block: TrayProviderBlock) => block.activityStatus === 'active'
    ? t('tray.calling', { count: block.activeRequests })
    : t('tray.defaultProvider')

  const updateProviderBlocksFromActivity = (activities: readonly TrayProviderActivity[]) => {
    const previous = providerBlocks.value
    const previousByRef = new Map(previous.map((block) => [block.providerId, block]))
    providerBlocks.value = activities.map((activity) => {
      const provider = providerByRef.get(activity.providerId) ?? null
      const previousBlock = previousByRef.get(activity.providerId)
      const providerName = provider?.name || activity.providerName || previousBlock?.providerName || activity.providerId
      const providerIconKey = provider?.icon || previousBlock?.providerIconKey || 'openai'
      return {
        provider,
        providerId: activity.providerId,
        providerName,
        providerIconKey,
        providerIconSvg: getProviderDisplayIconSvg(providerIconKey),
        providerInitials: getTrayProviderInitials(providerName),
        activeRequests: activity.activeRequests,
        activityStatus: activity.status,
        quotas: previousBlock?.quotas ?? [],
        stats: previousBlock?.stats ?? null,
      }
    })
  }

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
    const remaining = getProviderQuotaRemainingValue(quota)
    quota.usedLabel = formatQuotaValue(quota.used, quota.valueMode, quota.unit)
    quota.remainingParts = quota.unlimited
      ? [{ role: 'amount', value: '∞' }]
      : formatQuotaValueParts(remaining, quota.valueMode, quota.unit)
    quota.hasBudget = quota.displayKind === 'balance'
      ? true
      : quota.displayKind === 'error'
        ? false
        : quota.total > 0
    quota.totalLabel = quota.hasBudget && quota.displayKind !== 'balance'
      ? formatQuotaValue(quota.total, quota.valueMode, quota.unit)
      : '∞'
    quota.progressRatio = quota.displayKind === 'progress' && quota.hasBudget
      ? Math.min(Math.max(quota.used / quota.total, 0), 1)
      : 0
    quota.progressPercentLabel = quota.displayKind === 'progress' && quota.hasBudget
      ? `${Math.round(quota.progressRatio * 100)}%`
      : ''
  }

  const updateQuotaTimeLabels = (quota: TrayQuotaState, now: Date) => {
    if (quota.source === 'provider') {
      if (quota.displayKind === 'progress' && quota.hasBudget && quota.nextReset) {
        const remaining = quota.nextReset.getTime() - now.getTime()
        quota.countdownLabel = remaining > 0
          ? t('tray.resetCountdown', {
            countdown: quota.key === 'five_hour' ? formatClockCountdown(remaining) : formatCountdown(remaining),
          })
          : t('tray.resetSoon')
      }
      return Boolean(quota.displayKind === 'progress' && quota.hasBudget && quota.nextReset && now >= quota.nextReset && !loading.value)
    }

    if (showCountdown.value && quota.hasBudget && quota.nextReset) {
      const remaining = quota.nextReset.getTime() - now.getTime()
      quota.countdownLabel = remaining > 0
        ? t('tray.resetCountdown', {
          countdown: quota.key === 'five_hour' ? formatClockCountdown(remaining) : formatCountdown(remaining),
        })
        : t('tray.resetSoon')
    } else {
      quota.countdownLabel = ''
    }

    if (showForecast.value && quota.total > 0) {
      const rate = quota.forecastRate
      if (rate > 0 && quota.used < quota.total) {
        const secondsToBudget = (quota.total - quota.used) / rate
        if (!Number.isFinite(secondsToBudget)) {
          quota.forecastLabel = t('tray.forecastUnavailable')
        } else if (forecastDisplay.value === 'remaining') {
          quota.forecastLabel = t('tray.forecastDepletion', {
            value: formatCountdown(secondsToBudget * 1000),
          })
        } else {
          quota.forecastLabel = t('tray.forecastDepletion', {
            value: formatLocalDateTimeLabel(new Date(now.getTime() + secondsToBudget * 1000)),
          })
        }
      } else if (quota.used >= quota.total) {
        quota.forecastLabel = t('tray.budgetReached')
      } else {
        quota.forecastLabel = t('tray.forecastUnavailable')
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
      if (platform === 'claude' || platform === 'codex') {
        const status = await fetchProxyStatus(platform)
        hostingEnabled.value = Boolean(status?.enabled)
        return
      }
      if (platform === 'gemini') {
        const status = await Call.ByName('codeswitch/services.GeminiService.ProxyStatus') as { enabled?: boolean; Enabled?: boolean } | null
        hostingEnabled.value = Boolean(status?.enabled ?? status?.Enabled)
        return
      }
      if (platform === 'grokbuild') {
        const status = await Call.ByName('codeswitch/services.GrokSettingsService.ProxyStatus') as { enabled?: boolean; Enabled?: boolean } | null
        hostingEnabled.value = Boolean(status?.enabled ?? status?.Enabled)
        return
      }
      if (platform.startsWith('custom:')) {
        const status = await getCustomCliProxyStatus(platform.slice('custom:'.length))
        hostingEnabled.value = Boolean(status?.enabled)
        return
      }
      hostingEnabled.value = false
    } catch (error) {
      console.error(`failed to load ${platform} proxy status`, error)
    }
  }

  const getQuotaSettings = (settings: AppSettings): BudgetQuotaSettings => {
    if (platform === 'codex') return settings.budget_quota_settings_codex
    if (platform === 'claude') return settings.budget_quota_settings
    return createDefaultBudgetQuotaSettings()
  }

  const getQuotaAdjustments = (settings: AppSettings): BudgetQuotaAdjustments => {
    if (platform === 'codex') return settings.budget_quota_used_adjustments_codex
    if (platform === 'claude') return settings.budget_quota_used_adjustments
    return createDefaultBudgetQuotaAdjustments()
  }

  const loadProviders = async (): Promise<AutomationCard[]> => {
    providerByRef.clear()
    try {
      let providers: AutomationCard[] = []
      if (platform === 'gemini') {
        providers = (await GetGeminiProviders()).map(geminiToCard)
      } else {
        const saved = await LoadProviders(platform)
        providers = Array.isArray(saved)
          ? deserializeProviders(saved as PersistedProvider[], platform)
          : []
      }
      const enabledProviders = providers.filter((provider) => provider.enabled)
      enabledProviders.forEach((provider) => {
        const providerRef = String(provider.providerRef ?? provider.id).trim()
        if (providerRef) providerByRef.set(providerRef, provider)
      })
      return enabledProviders
    } catch (error) {
      console.error(`failed to load ${platform} providers`, error)
      return []
    }
  }

  const createProviderQuotaState = (item: ProviderQuotaSnapshotItem): TrayQuotaState => {
    const display = resolveTrayProviderQuotaDisplay(item, t)
    const nextQuota: TrayQuotaState = {
      key: display.key,
      title: display.title,
      rawUsed: display.used,
      used: display.used,
      total: display.total,
      unlimited: display.unlimited,
      usedLabel: '',
      totalLabel: '',
      remainingParts: [],
      valueMode: display.valueMode,
      unit: display.unit,
      extra: display.extra,
      invalidMessage: display.invalidMessage,
      source: 'provider',
      displayKind: display.displayKind,
      hasBudget: display.displayKind === 'error' ? false : display.total > 0,
      progressRatio: 0,
      progressPercentLabel: '',
      countdownLabel: display.countdownLabel,
      forecastLabel: '',
      windowStart: null,
      nextReset: display.nextReset,
      forecastRate: 0,
    }
    updateQuotaStaticLabels(nextQuota)
    return nextQuota
  }

  const loadProviderQuotas = async (provider: AutomationCard, now: Date): Promise<TrayQuotaState[]> => {
    if (!hasTrayFallbackProviderQuotaConfig(provider)) return []
    if (hasProviderQuotaQueryType(provider.providerQuotaQueryConfig ?? provider.providerQuotaQueryType, provider.providerQuotaQueryType)) {
      const result = await resolveProviderQuotaQueryDisplay({
        card: provider,
        now,
        t,
      })
      if (result.items.length > 0) {
        return result.items.map(createProviderQuotaState)
      }
      if (result.failureMessage) {
        return [{
          key: 'provider_quota_error',
          title: t('tray.providerQuotaQuery'),
          rawUsed: 0,
          used: 0,
          total: 0,
          unlimited: false,
          usedLabel: formatQuotaValue(0),
          totalLabel: '∞',
          remainingParts: formatQuotaValueParts(0),
          valueMode: 'currency',
          unit: undefined,
          extra: '',
          invalidMessage: result.failureMessage,
          source: 'provider',
          displayKind: 'error',
          hasBudget: false,
          progressRatio: 0,
          progressPercentLabel: '',
          countdownLabel: '',
          forecastLabel: '',
          windowStart: null,
          nextReset: null,
          forecastRate: 0,
        }]
      }
    }

    const snapshots = await resolveProviderQuotaSnapshot({
      card: provider,
      platform,
      now,
      t,
    })
    return snapshots.map(createProviderQuotaState)
  }

  const loadProviderBlocks = async (
    activities: readonly TrayProviderActivity[],
    now: Date,
    providers: readonly AutomationCard[],
  ) => {
    if (activities.length === 0) {
      providerBlocks.value = []
      return
    }
    const statsPromise = fetchProviderDailyStats(platform).catch((error) => {
      console.error(`failed to load ${platform} tray provider stats`, error)
      return []
    })
    const providerByName = new Map(providers.map((provider) => [provider.name, provider]))
    const stats = await statsPromise
    providerBlocks.value = await Promise.all(activities.map(async (activity) => {
      const provider = providerByRef.get(activity.providerId) ?? providerByName.get(activity.providerName) ?? null
      const providerName = provider?.name || activity.providerName || activity.providerId
      const providerIconKey = provider?.icon || 'openai'
      const quotas = provider ? await loadProviderQuotas(provider, now) : []
      return {
        provider,
        providerId: activity.providerId || String(provider?.providerRef ?? provider?.id ?? '').trim(),
        providerName,
        providerIconKey,
        providerIconSvg: getProviderDisplayIconSvg(providerIconKey),
        providerInitials: getTrayProviderInitials(providerName),
        activeRequests: activity.activeRequests,
        activityStatus: activity.status,
        quotas,
        stats: provider ? buildTrayProviderStatsDisplay(provider, stats, currentLocale()) : null,
      }
    }))
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

  const setProviderActivities = (activities: readonly TrayProviderActivity[], error = false) => {
    providerActivities.value = [...activities]
    providerActivityLoaded.value = true
    providerActivityError.value = error
    updateProviderBlocksFromActivity(activities)
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
    if (platform === 'claude') {
      showCountdown.value = settings?.budget_show_countdown ?? false
      showForecast.value = settings?.budget_show_forecast ?? false
      forecastMethod.value = normalizeForecastMethod(settings?.budget_forecast_method ?? 'cycle')
      forecastDisplay.value = normalizeForecastDisplay(settings?.budget_forecast_display ?? 'datetime')
      usedAdjustments.value = settings?.budget_quota_used_adjustments ?? createDefaultBudgetQuotaAdjustments()
      return
    }
    showCountdown.value = false
    showForecast.value = false
    forecastMethod.value = 'cycle'
    forecastDisplay.value = 'datetime'
    usedAdjustments.value = createDefaultBudgetQuotaAdjustments()
  }

  const refresh = async (settings: AppSettings) => {
    loading.value = true
    try {
      applySettings(settings)
      const now = new Date()
      const quotaSettings = getQuotaSettings(settings)
      usedAdjustments.value = getQuotaAdjustments(settings)
      await updateHostingState()
      const providers = await loadProviders()
      void preloadProviderDisplayIcons(providers.map((provider) => provider.icon))
      await loadProviderBlocks(providerActivities.value, now, providers)
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
    const shouldRefreshGlobalQuotas = updateItemsAndCollectRefresh(
      quotas.value,
      (quota) => updateQuotaTimeLabels(quota, now),
    )
    const shouldRefreshProviderQuotas = updateItemsAndCollectRefresh(
      providerBlockQuotas.value,
      (quota) => updateQuotaTimeLabels(quota, now),
    )
    return shouldRefreshGlobalQuotas || shouldRefreshProviderQuotas
  }

  return proxyRefs({
    platform,
    brandName,
    brandIcon,
    brandIconSvg,
    quotas,
    visibleQuotas,
    providerBlocks,
    providerActivityLoaded,
    providerActivityError,
    providerActivityLabel,
    showTotalUsage,
    totalUsageLabel,
    hostingEnabled,
    hostingLabel,
    loading,
    refresh,
    setProviderActivities,
    hasSecondPrecisionCountdown,
    updateDerivedLabels,
  })
}

type TrayPlatformDescriptor = {
  platform: Platform
  brandName: string
  brandIcon: string
}

const platformOptions = new Map(
  HOME_PROVIDER_TAB_OPTIONS.map((option) => [option.id, option]),
)
const visiblePlatformDescriptors = ref<TrayPlatformDescriptor[]>([])
const customCliTools = ref<CustomCliTool[]>([])
const cardCache = new Map<string, ReturnType<typeof createTrayCard>>()
const activityByPlatform = new Map<string, TrayProviderActivity[]>()
let activityRefreshTask: { key: string; promise: Promise<void> } | null = null
let activityRefreshGeneration = 0
let activityRefreshTimer: number | undefined
let activityNeedsFullRefresh = false

const getOrCreateCard = (descriptor: TrayPlatformDescriptor) => {
  const cached = cardCache.get(descriptor.platform)
  if (cached) return cached
  const card = createTrayCard(descriptor.platform, descriptor.brandName, descriptor.brandIcon)
  cardCache.set(descriptor.platform, card)
  return card
}

const cards = computed(() => visiblePlatformDescriptors.value.map(getOrCreateCard))

const updateVisiblePlatformDescriptors = async (settings: AppSettings) => {
  const descriptors: TrayPlatformDescriptor[] = []
  for (const tabId of settings.home_provider_tabs) {
    if (tabId === 'others') continue
    if (tabId !== 'claude' && tabId !== 'codex' && tabId !== 'gemini' && tabId !== 'grokbuild') continue
    const option = platformOptions.get(tabId)
    if (!option) continue
    descriptors.push({
      platform: tabId,
      brandName: option.label,
      brandIcon: option.icon,
    })
  }

  if (settings.home_provider_tabs.includes('others')) {
    try {
      customCliTools.value = await listCustomCliTools()
    } catch (error) {
      console.error('failed to load tray custom cli tools', error)
      customCliTools.value = []
    }
    customCliTools.value.forEach((tool) => {
      if (!tool.id) return
      descriptors.push({
        platform: `custom:${tool.id}`,
        brandName: tool.name || tool.id,
        brandIcon: 'others',
      })
    })
  } else {
    customCliTools.value = []
  }

  const nextPlatforms = new Set(descriptors.map((descriptor) => descriptor.platform))
  visiblePlatformDescriptors.value.forEach((descriptor) => {
    if (nextPlatforms.has(descriptor.platform)) return
    activityByPlatform.delete(descriptor.platform)
    cardCache.get(descriptor.platform)?.setProviderActivities([])
  })
  visiblePlatformDescriptors.value = descriptors
  void preloadProviderDisplayIcons(descriptors.map((descriptor) => descriptor.brandIcon))
}

const refreshProviderActivity = (): Promise<void> => {
  if (!isTrayWindowActive() || !refreshLifecycle?.isActive()) return Promise.resolve()

  const targetCards = [...cards.value]
  const platforms = targetCards.map((card) => card.platform)
  if (platforms.length === 0) return Promise.resolve()
  const refreshKey = buildTrayProviderActivityRefreshKey(platforms, activityRefreshGeneration)
  if (activityRefreshTask) {
    if (activityRefreshTask.key === refreshKey) return activityRefreshTask.promise
    return activityRefreshTask.promise.then(refreshProviderActivity, refreshProviderActivity)
  }

  const nextRefresh = (async () => {
    const snapshot = await loadTrayProviderActivitySnapshot(platforms, {
      loadStates: () => Call.ByName(
        'codeswitch/services.ProviderConcurrencyService.GetTrayProviderRuntimeStatesBatch',
        platforms,
      ),
    })

    const currentPlatforms = cards.value.map((card) => card.platform)
    const currentRefreshKey = buildTrayProviderActivityRefreshKey(
      currentPlatforms,
      activityRefreshGeneration,
    )
    if (
      refreshKey !== currentRefreshKey
      || !isTrayWindowActive()
      || !refreshLifecycle?.isActive()
    ) return

    if (snapshot.error || !snapshot.statesByPlatform) {
      console.error('failed to load tray provider activity', snapshot.error)
    }

    targetCards.forEach((card) => {
      const state = snapshot.statesByPlatform?.[card.platform]
      const activityError = Boolean(snapshot.error || !snapshot.statesByPlatform || state?.error)
      if (state?.error) {
        console.error(`failed to load ${card.platform} tray provider activity`)
      }
      const previous = activityByPlatform.get(card.platform) ?? []
      const next = activityError
        ? []
        : resolveTrayProviderActivity(
          state?.statuses ?? [],
          state?.defaultProvider ?? null,
        )
      activityByPlatform.set(card.platform, next)
      card.setProviderActivities(next, activityError)
      if (activityError) return
      if (!hasTrayProviderActivityChanged(previous, next)) return
      if (refreshBusy) {
        activityNeedsFullRefresh = true
      } else {
        scheduleRefreshAll()
      }
    })
  })()

  const promise = nextRefresh.finally(() => {
    if (activityRefreshTask?.promise === promise) {
      activityRefreshTask = null
    }
  })
  activityRefreshTask = { key: refreshKey, promise }
  return promise
}

const getTickerIntervalMs = () => (
  cards.value.some((card) => card.hasSecondPrecisionCountdown) ? 1000 : FULL_REFRESH_INTERVAL_MS
)

const isTrayWindowActive = () => !document.hidden && document.hasFocus()

const triggerRefreshAll = () => {
  if (!isTrayWindowActive()) {
    refreshLifecycle?.deactivate()
    return
  }
  if (!refreshLifecycle?.isActive() || refreshBusy) return
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
  const shouldRefresh = updateItemsAndCollectRefresh(cards.value, (card) => card.updateDerivedLabels(now))
  if (shouldRefresh) {
    refreshAllForResetIfDue()
  }
}

const setupTicker = () => refreshLifecycle?.restartTicker()

const resizeToContent = async () => {
  if (!isTrayWindowActive() || !refreshLifecycle?.isActive()) return
  await nextTick()
  if (!isTrayWindowActive() || !refreshLifecycle?.isActive()) return
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
  if (!refreshLifecycle?.isActive() || refreshBusy) return
  refreshBusy = true
  lastRefreshAttemptAt = Date.now()
  try {
    const settings = await fetchAppSettings()
    await updateVisiblePlatformDescriptors(settings)
    await refreshProviderActivity()
    if (!isTrayWindowActive() || !refreshLifecycle?.isActive()) return
    activityNeedsFullRefresh = false
    await Promise.all(cards.value.map((card) => card.refresh(settings)))
  } catch (error) {
    console.error('failed to refresh tray cards', error)
  } finally {
    refreshBusy = false
    lastFullRefreshAt = Date.now()
    lastRefreshAttemptAt = lastFullRefreshAt
    updateAllDerivedLabels()
    setupTicker()
    await resizeToContent()
    if (activityNeedsFullRefresh) {
      activityNeedsFullRefresh = false
      scheduleRefreshAll()
    }
  }
}

const clearActivityRefreshTimer = () => {
  if (activityRefreshTimer === undefined) return
  window.clearInterval(activityRefreshTimer)
  activityRefreshTimer = undefined
}

const startActivityRefreshTimer = () => {
  clearActivityRefreshTimer()
  if (!isTrayWindowActive()) return
  if (cards.value.length > 0) void refreshProviderActivity()
  activityRefreshTimer = window.setInterval(() => {
    if (!isTrayWindowActive()) {
      clearActivityRefreshTimer()
      return
    }
    void refreshProviderActivity()
  }, 1000)
}

refreshLifecycle = createTrayRefreshLifecycle({
  onActivate: triggerRefreshAll,
  onTick: () => {
    if (!isTrayWindowActive()) {
      refreshLifecycle?.deactivate()
      return
    }
    updateAllDerivedLabels()
    refreshAllIfDue()
  },
  getIntervalMs: getTickerIntervalMs,
})

const clearStorageRefreshTimer = () => {
  if (storageRefreshTimer === undefined) return
  window.clearTimeout(storageRefreshTimer)
  storageRefreshTimer = undefined
}

const activateTray = () => {
  if (!isTrayWindowActive()) return
  if (!refreshLifecycle?.isActive()) lastWindowHeight = 0
  refreshLifecycle?.activate()
  startActivityRefreshTimer()
  void resizeToContent()
}

const deactivateTray = () => {
  activityRefreshGeneration += 1
  refreshLifecycle?.deactivate()
  clearActivityRefreshTimer()
  clearStorageRefreshTimer()
}

const handleVisibilityChange = () => {
  if (document.hidden) {
    deactivateTray()
    return
  }
  activateTray()
}

const scheduleRefreshAll = () => {
  if (!isTrayWindowActive() || !refreshLifecycle?.isActive()) return
  clearStorageRefreshTimer()
  storageRefreshTimer = window.setTimeout(() => {
    storageRefreshTimer = undefined
    if (!refreshLifecycle?.isActive()) return
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

const handleExternalRefresh = () => scheduleRefreshAll()

onMounted(() => {
  lastWindowHeight = 0
  window.addEventListener('focus', activateTray)
  window.addEventListener('blur', deactivateTray)
  document.addEventListener('visibilitychange', handleVisibilityChange)
  window.addEventListener('app-settings-updated', handleExternalRefresh)
  window.addEventListener('storage', handleStorageChange)
  activateTray()
})

onUnmounted(() => {
  activityRefreshGeneration += 1
  refreshLifecycle?.dispose()
  clearActivityRefreshTimer()
  clearStorageRefreshTimer()
  lastWindowHeight = 0
  window.removeEventListener('focus', activateTray)
  window.removeEventListener('blur', deactivateTray)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  window.removeEventListener('app-settings-updated', handleExternalRefresh)
  window.removeEventListener('storage', handleStorageChange)
})
</script>

<template>
  <div ref="rootRef" class="tray-root">
    <div class="tray-list">
      <div v-for="card in cards" :key="card.platform" class="tray-panel">
        <div class="tray-header">
          <div class="tray-brand">
            <div class="tray-brand__icon" aria-hidden="true">
              <span
                v-if="card.brandIconSvg"
                class="tray-brand__icon-svg"
                v-html="card.brandIconSvg"
              ></span>
              <span v-else class="tray-brand__icon-fallback">{{ card.brandIcon }}</span>
            </div>
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
                <span>{{ t('tray.totalUsage') }}</span>
              </div>
              <div class="tray-item__value" :class="{ loading: card.loading }">
                <span>{{ card.totalUsageLabel }}</span>
              </div>
            </div>
          </div>
          <div v-for="quota in card.visibleQuotas" :key="`${card.platform}-${quota.key}`" class="tray-item">
            <div class="tray-item__header">
              <div class="tray-item__title" :title="quota.title">
                <span class="tray-dot"></span>
                <span class="tray-item__title-text">{{ quota.title }}</span>
              </div>
              <div class="tray-item__summary">
                <div class="tray-item__value" :class="{ loading: card.loading }">
                  <span>{{ t('tray.used', { amount: quota.usedLabel }) }}</span>
                  <span class="tray-divider">/</span>
                  <span>{{ quota.totalLabel }}</span>
                </div>
                <span v-if="quota.hasBudget" class="tray-item__percent">{{ quota.progressPercentLabel }}</span>
              </div>
            </div>
            <div
              v-if="quota.hasBudget"
              class="tray-progress"
              role="progressbar"
              :aria-label="quota.title"
              aria-valuemin="0"
              aria-valuemax="100"
              :aria-valuenow="Math.round(quota.progressRatio * 100)"
              :aria-valuetext="quota.progressPercentLabel"
            >
              <div class="tray-progress__bar" :style="{ width: `${quota.progressRatio * 100}%` }"></div>
            </div>
            <div v-if="quota.countdownLabel || quota.forecastLabel" class="tray-meta">
              <span v-if="quota.countdownLabel">{{ quota.countdownLabel }}</span>
              <span v-if="quota.forecastLabel">{{ quota.forecastLabel }}</span>
            </div>
          </div>
          <div
            v-if="card.providerActivityLoaded && card.providerActivityError"
            class="tray-provider-empty tray-provider-empty--error"
            role="status"
          >
            <span class="tray-provider-empty__dot" aria-hidden="true"></span>
            <span>{{ t('tray.queryFailed') }}</span>
          </div>
          <div
            v-else-if="card.providerActivityLoaded && card.providerBlocks.length === 0"
            class="tray-provider-empty"
            role="status"
          >
            <span class="tray-provider-empty__dot" aria-hidden="true"></span>
            <span>{{ t('tray.noAvailableProviders') }}</span>
          </div>
          <div
            v-for="block in card.providerBlocks"
            :key="`${card.platform}-${block.activityStatus}-${block.providerId || block.providerName}`"
            class="tray-provider-block"
          >
            <div class="tray-provider-source">
              <span class="tray-provider-source__status" :class="{ active: block.activityStatus === 'active' }">
                <span class="tray-provider-source__status-dot"></span>
                <span>{{ card.providerActivityLabel(block) }}</span>
              </span>
              <strong class="tray-provider-source__provider" :title="block.providerName">
                <span class="tray-provider-source__icon" aria-hidden="true">
                  <span
                    v-if="block.providerIconSvg"
                    class="tray-provider-source__icon-svg"
                    v-html="block.providerIconSvg"
                  ></span>
                  <span v-else class="tray-provider-source__icon-fallback">{{ block.providerInitials }}</span>
                </span>
                <span class="tray-provider-source__name">{{ block.providerName }}</span>
              </strong>
            </div>
            <div
              v-if="block.stats"
              class="tray-provider-metrics"
              :class="{ loading: card.loading }"
            >
              <dl class="tray-provider-metrics__grid">
                <div class="tray-provider-metric">
                  <dt>{{ t('tray.successRate') }}</dt>
                  <dd
                    :class="`tray-provider-metric__value--${block.stats.successRateTone}`"
                    :title="block.stats.successRate"
                  >{{ block.stats.successRate }}</dd>
                </div>
                <div class="tray-provider-metric">
                  <dt>{{ t('tray.requests') }}</dt>
                  <dd :title="block.stats.requests">{{ block.stats.requests }}</dd>
                </div>
                <div class="tray-provider-metric">
                  <dt>{{ t('tray.tokens') }}</dt>
                  <dd :title="block.stats.tokens">{{ block.stats.tokens }}</dd>
                </div>
                <div class="tray-provider-metric tray-provider-metric--cost">
                  <dt>{{ t('tray.cost') }}</dt>
                  <dd :title="block.stats.cost">{{ block.stats.cost }}</dd>
                </div>
              </dl>
              <div class="tray-provider-performance">
                <div class="tray-provider-performance__item tray-provider-performance__item--first-token">
                  <span class="tray-provider-performance__icon" aria-hidden="true">{{ t('tray.firstTokenShort') }}</span>
                  <span class="tray-provider-performance__label">{{ t('tray.firstToken') }}</span>
                  <strong :title="block.stats.firstToken">{{ block.stats.firstToken }}</strong>
                </div>
                <div class="tray-provider-performance__item tray-provider-performance__item--speed">
                  <span class="tray-provider-performance__icon" aria-hidden="true">{{ t('tray.speedShort') }}</span>
                  <span class="tray-provider-performance__label">{{ t('tray.speed') }}</span>
                  <strong :title="block.stats.speed">{{ block.stats.speed }}</strong>
                </div>
              </div>
            </div>
            <div
              v-for="quota in block.quotas"
              :key="`${card.platform}-${block.providerId}-quota-${quota.key}`"
              class="tray-item"
              :class="{
                'tray-item--balance': quota.displayKind === 'balance',
                'tray-item--error': quota.displayKind === 'error',
              }"
            >
              <div class="tray-item__header">
                <div class="tray-item__title" :title="quota.title">
                  <span class="tray-dot"></span>
                  <span class="tray-item__title-text">{{ quota.title }}</span>
                </div>
                <div class="tray-item__summary">
                  <div class="tray-item__value" :class="{ loading: card.loading }">
                    <template v-if="quota.displayKind === 'balance'">
                      <span class="tray-remaining">
                        <span class="sr-only">{{ t('tray.remaining', { amount: joinTrayAmountParts(quota.remainingParts) }) }}</span>
                        <span class="tray-remaining__label" aria-hidden="true">{{ t('tray.remainingLabel') }}</span>
                        <span class="tray-remaining__amount" aria-hidden="true">
                          <span
                            v-for="(part, index) in quota.remainingParts"
                            :key="`${quota.key}-remaining-${index}`"
                            class="tray-remaining__part"
                            :class="`tray-remaining__part--${part.role}`"
                          >{{ part.value }}</span>
                        </span>
                      </span>
                    </template>
                    <template v-else-if="quota.displayKind === 'error'">
                      <span>{{ t('tray.queryFailed') }}</span>
                    </template>
                    <template v-else>
                      <span>{{ t('tray.used', { amount: quota.usedLabel }) }}</span>
                      <span class="tray-divider">/</span>
                      <span>{{ quota.totalLabel }}</span>
                    </template>
                  </div>
                  <span v-if="quota.displayKind === 'progress' && quota.hasBudget" class="tray-item__percent">
                    {{ quota.progressPercentLabel }}
                  </span>
                </div>
              </div>
              <div
                v-if="quota.displayKind === 'progress' && quota.hasBudget"
                class="tray-progress"
                role="progressbar"
                :aria-label="quota.title"
                aria-valuemin="0"
                aria-valuemax="100"
                :aria-valuenow="Math.round(quota.progressRatio * 100)"
                :aria-valuetext="quota.progressPercentLabel"
              >
                <div class="tray-progress__bar" :style="{ width: `${quota.progressRatio * 100}%` }"></div>
              </div>
              <div v-if="quota.countdownLabel || quota.extra || quota.invalidMessage" class="tray-meta">
                <span v-if="quota.countdownLabel">{{ quota.countdownLabel }}</span>
                <span v-if="quota.invalidMessage">{{ quota.invalidMessage }}</span>
                <span v-if="quota.extra">{{ quota.extra }}</span>
              </div>
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
  scrollbar-width: thin;
  scrollbar-color: color-mix(in srgb, var(--mac-text-secondary) 42%, transparent) transparent;
}

.tray-root::-webkit-scrollbar {
  width: 5px;
  height: 5px;
}

.tray-root::-webkit-scrollbar-track {
  background: transparent;
}

.tray-root::-webkit-scrollbar-thumb {
  border: 1px solid transparent;
  border-radius: 999px;
  background: color-mix(in srgb, var(--mac-text-secondary) 42%, transparent);
  background-clip: content-box;
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

.tray-provider-block + .tray-provider-block {
  padding-top: 12px;
  border-top: 1px solid var(--mac-divider);
}

.tray-provider-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-height: 28px;
  color: var(--mac-text-secondary);
  font-size: 11px;
}

.tray-provider-empty__dot {
  width: 7px;
  height: 7px;
  flex: 0 0 7px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--mac-text-secondary) 55%, transparent);
}

.tray-provider-empty--error {
  color: var(--mac-text-secondary);
}

.tray-provider-source {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 6px 0 0;
  font-size: 11px;
  color: var(--mac-text-secondary);
}

.tray-provider-source strong {
  overflow: hidden;
  color: var(--mac-text);
  font-size: 12px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tray-provider-source__provider {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  min-width: 0;
  max-width: 62%;
  flex: 0 1 62%;
  gap: 5px;
}

.tray-provider-source__status {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  color: var(--mac-text-secondary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tray-provider-source__status.active {
  color: var(--mac-text);
}

.tray-provider-source__status-dot {
  width: 7px;
  height: 7px;
  flex: 0 0 7px;
  border-radius: 999px;
  background: var(--mac-text-secondary);
  opacity: 0.6;
}

.tray-provider-source__status.active .tray-provider-source__status-dot {
  background: #5dbb63;
  opacity: 1;
  box-shadow: 0 0 0 2px rgba(93, 187, 99, 0.2);
}

.tray-provider-source__icon {
  width: 15px;
  height: 15px;
  flex: 0 0 15px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--mac-text);
}

.tray-provider-source__icon-svg,
.tray-provider-source__icon-svg :deep(svg) {
  display: block;
  width: 14px;
  height: 14px;
}

.tray-provider-source__icon-fallback {
  overflow: hidden;
  font-size: 8px;
  font-weight: 700;
  line-height: 1;
  max-width: 15px;
  text-align: center;
}

.tray-provider-source__name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tray-provider-source + .tray-provider-metrics,
.tray-provider-metrics + .tray-item {
  padding-top: 10px;
  border-top: 1px solid var(--mac-divider);
}

.tray-provider-metrics {
  display: flex;
  flex-direction: column;
  gap: 9px;
  min-width: 0;
  transition: opacity 0.2s ease;
}

.tray-provider-metrics.loading {
  opacity: 0.6;
}

.tray-provider-metrics__grid {
  display: grid;
  grid-template-columns: minmax(0, 0.85fr) minmax(0, 0.85fr) minmax(0, 1.1fr) minmax(0, 1.3fr);
  margin: 0;
}

.tray-provider-metric {
  min-width: 0;
  padding: 2px 4px;
  text-align: center;
}

.tray-provider-metric + .tray-provider-metric {
  border-left: 1px solid var(--mac-divider);
}

.tray-provider-metric dt {
  overflow: hidden;
  color: var(--mac-text-secondary);
  font-size: 10px;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tray-provider-metric dd {
  overflow: hidden;
  margin: 5px 0 0;
  color: var(--mac-text);
  font-size: 12px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tray-provider-metric dd.tray-provider-metric__value--good {
  color: #11745a;
}

.tray-provider-metric dd.tray-provider-metric__value--warning {
  color: #925500;
}

.tray-provider-metric dd.tray-provider-metric__value--bad {
  color: #c93632;
}

.tray-provider-metric dd.tray-provider-metric__value--neutral {
  color: var(--mac-text-secondary);
}

.tray-provider-metric--cost dd {
  color: #925500;
  font-size: 11px;
}

.tray-provider-performance {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.tray-provider-performance__item {
  min-width: 0;
  min-height: 28px;
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr) auto;
  align-items: center;
  gap: 5px;
  padding: 0 7px;
  border: 1px solid currentColor;
  border-radius: 6px;
  font-size: 10px;
}

.tray-provider-performance__item--first-token {
  --tray-performance-accent: #7950b8;
  color: var(--tray-performance-accent);
  background: rgba(121, 80, 184, 0.07);
}

.tray-provider-performance__item--speed {
  --tray-performance-accent: #0f7466;
  color: var(--tray-performance-accent);
  background: rgba(15, 116, 102, 0.07);
}

.tray-provider-performance__icon {
  width: 18px;
  height: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  background: var(--tray-performance-accent);
  color: white;
  font-size: 9px;
  font-weight: 700;
  line-height: 1;
}

.tray-provider-performance__label {
  overflow: hidden;
  color: currentColor;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tray-provider-performance strong {
  overflow: hidden;
  color: var(--mac-text);
  font-size: 11px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  text-overflow: ellipsis;
  white-space: nowrap;
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

.tray-brand__icon-svg,
.tray-brand__icon-svg :deep(svg) {
  display: block;
  width: 16px;
  height: 16px;
}

.tray-brand__icon-fallback {
  font-size: 13px;
  font-weight: 700;
  line-height: 1;
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
  min-width: 0;
}

.tray-item__summary {
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  gap: 10px;
  min-width: 0;
}

.tray-item__title {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 8px;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--mac-text);
}

.tray-item__title-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tray-dot {
  width: 10px;
  height: 10px;
  flex: 0 0 10px;
  border-radius: 999px;
  background: #5dbb63;
  box-shadow: 0 0 0 2px rgba(93, 187, 99, 0.2);
}

.tray-item__value {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  font-size: 12px;
  color: var(--mac-text-secondary);
  text-align: right;
  white-space: nowrap;
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
  flex-wrap: wrap;
  gap: 6px 10px;
  justify-content: space-between;
  font-size: 11px;
  color: var(--mac-text-secondary);
}

.tray-item--balance .tray-dot {
  background: #0a84ff;
  box-shadow: 0 0 0 2px rgba(10, 132, 255, 0.18);
}

.tray-item--balance .tray-item__value {
  color: var(--mac-text);
  font-weight: 600;
}

.tray-remaining {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
}

.tray-remaining__amount {
  display: inline-flex;
  align-items: baseline;
  font-variant-numeric: tabular-nums;
}

.tray-remaining__part--unit {
  color: color-mix(in srgb, var(--mac-accent, #0a84ff) 90%, var(--mac-text) 10%);
  font-weight: 700;
}

.tray-remaining__part--unit + .tray-remaining__part--amount {
  margin-left: 3px;
}

.tray-remaining__part--amount {
  color: #fbbf24;
  font-weight: 700;
}

.tray-remaining__part--literal {
  color: var(--mac-text-secondary);
}

.tray-item--error .tray-dot {
  background: #ff5f57;
  box-shadow: 0 0 0 2px rgba(255, 95, 87, 0.2);
}

.tray-item--error .tray-item__value,
.tray-item--error .tray-meta {
  color: #bf2b24;
}

:global(html.light) .tray-remaining__part--amount {
  color: #c2410c;
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

:global(.dark) .tray-provider-block + .tray-provider-block {
  border-top-color: var(--mac-divider);
}

:global(.dark) .tray-provider-source + .tray-provider-metrics,
:global(.dark) .tray-provider-metrics + .tray-item {
  border-top-color: var(--mac-divider);
}

:global(.dark) .tray-provider-metric dd.tray-provider-metric__value--good {
  color: #65d6ad;
}

:global(.dark) .tray-provider-metric dd.tray-provider-metric__value--warning,
:global(.dark) .tray-provider-metric--cost dd {
  color: #f6c453;
}

:global(.dark) .tray-provider-metric dd.tray-provider-metric__value--bad {
  color: #ff8a82;
}

:global(.dark) .tray-provider-performance__item--first-token {
  --tray-performance-accent: #b794f4;
  background: rgba(183, 148, 244, 0.08);
}

:global(.dark) .tray-provider-performance__item--speed {
  --tray-performance-accent: #55d6c2;
  background: rgba(85, 214, 194, 0.08);
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

:global(.dark) .tray-item--balance .tray-dot {
  background: #64b5ff;
  box-shadow: 0 0 0 2px rgba(100, 181, 255, 0.24);
}

:global(.dark) .tray-remaining__part--unit {
  color: color-mix(in srgb, var(--mac-accent, #0a84ff) 72%, white 28%);
}

:global(.dark) .tray-remaining__part--amount {
  color: #fbbf24;
}

:global(.dark) .tray-item--error .tray-dot {
  background: #ff6b63;
  box-shadow: 0 0 0 2px rgba(255, 107, 99, 0.26);
}

:global(.dark) .tray-item--error .tray-item__value,
:global(.dark) .tray-item--error .tray-meta {
  color: #ff8a82;
}
</style>
