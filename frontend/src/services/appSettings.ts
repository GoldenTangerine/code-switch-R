import { Call } from '@wailsio/runtime'
import {
  DEFAULT_HEATMAP_DISPLAY_SETTINGS,
  normalizeHeatmapDisplaySettings,
} from '../data/heatmapDisplaySettings'
import {
  DEFAULT_HOME_PROVIDER_TABS,
  normalizeHomeProviderTabs,
  type HomeProviderTab,
} from '../data/homeProviderTabs'
import {
  createDefaultBudgetQuotaAdjustments,
  createDefaultBudgetQuotaSettings,
  normalizeBudgetQuotaAdjustments,
  normalizeBudgetQuotaSettings,
  projectBudgetQuotaToLegacy,
  serializeBudgetQuotaSettings,
  type BudgetQuotaAdjustments,
  type BudgetQuotaSettings,
} from '../utils/budgetUsage'

export type HeatmapGranularity = 'hourly' | 'daily'

export const normalizeHeatmapGranularity = (
  value?: string | null,
  fallback: HeatmapGranularity = 'daily',
): HeatmapGranularity => {
  if (value === 'daily') return 'daily'
  if (value === 'hourly') return 'hourly'
  return fallback
}

export type AppSettings = {
  show_heatmap: boolean
  heatmap_granularity: HeatmapGranularity
  heatmap_daily_scale_factor: number
  heatmap_daily_intensity_mode: 'hourly_scaled' | 'daily_peak'
  heatmap_intensity_metric: 'requests' | 'cost' | 'total_tokens' | 'input_tokens' | 'output_tokens' | 'reasoning_tokens'
  heatmap_intensity_stop_l1: number
  heatmap_intensity_stop_l2: number
  heatmap_intensity_stop_l3: number
  show_home_title: boolean
  home_provider_tabs: HomeProviderTab[]
  budget_total: number
  budget_used_adjustment: number
  budget_cycle_enabled: boolean
  budget_cycle_mode: string
  budget_refresh_time: string
  budget_refresh_day: number
  budget_refresh_month_day: number
  budget_quota_used_adjustments: BudgetQuotaAdjustments
  budget_quota_settings: BudgetQuotaSettings
  budget_show_countdown: boolean
  budget_show_forecast: boolean
  budget_forecast_method: string
  budget_forecast_display: string
  budget_total_codex: number
  budget_used_adjustment_codex: number
  budget_cycle_enabled_codex: boolean
  budget_cycle_mode_codex: string
  budget_refresh_time_codex: string
  budget_refresh_day_codex: number
  budget_refresh_month_day_codex: number
  budget_quota_used_adjustments_codex: BudgetQuotaAdjustments
  budget_quota_settings_codex: BudgetQuotaSettings
  budget_show_countdown_codex: boolean
  budget_show_forecast_codex: boolean
  budget_forecast_method_codex: string
  budget_forecast_display_codex: string
  auto_start: boolean
  auto_update: boolean
  update_history_keep_count: number
  auto_connectivity_test: boolean
  enable_switch_notify: boolean // 供应商切换通知开关
  enable_round_robin: boolean   // 同 Level 轮询负载均衡开关
  provider_concurrency_limits: Record<string, boolean>
  capture_request_log_payload: boolean
  sanitize_request_log_payload: boolean
}

const DEFAULT_SETTINGS: AppSettings = {
  show_heatmap: true,
  heatmap_granularity: 'daily',
  heatmap_daily_scale_factor: DEFAULT_HEATMAP_DISPLAY_SETTINGS.dailyScaleFactor,
  heatmap_daily_intensity_mode: DEFAULT_HEATMAP_DISPLAY_SETTINGS.dailyIntensityMode,
  heatmap_intensity_metric: DEFAULT_HEATMAP_DISPLAY_SETTINGS.intensityMetric,
  heatmap_intensity_stop_l1: DEFAULT_HEATMAP_DISPLAY_SETTINGS.intensityStopL1,
  heatmap_intensity_stop_l2: DEFAULT_HEATMAP_DISPLAY_SETTINGS.intensityStopL2,
  heatmap_intensity_stop_l3: DEFAULT_HEATMAP_DISPLAY_SETTINGS.intensityStopL3,
  show_home_title: true,
  home_provider_tabs: [...DEFAULT_HOME_PROVIDER_TABS],
  budget_total: 0,
  budget_used_adjustment: 0,
  budget_cycle_enabled: false,
  budget_cycle_mode: 'daily',
  budget_refresh_time: '00:00',
  budget_refresh_day: 1,
  budget_refresh_month_day: 1,
  budget_quota_used_adjustments: createDefaultBudgetQuotaAdjustments(),
  budget_quota_settings: createDefaultBudgetQuotaSettings(),
  budget_show_countdown: false,
  budget_show_forecast: false,
  budget_forecast_method: 'cycle',
  budget_forecast_display: 'datetime',
  budget_total_codex: 0,
  budget_used_adjustment_codex: 0,
  budget_cycle_enabled_codex: false,
  budget_cycle_mode_codex: 'daily',
  budget_refresh_time_codex: '00:00',
  budget_refresh_day_codex: 1,
  budget_refresh_month_day_codex: 1,
  budget_quota_used_adjustments_codex: createDefaultBudgetQuotaAdjustments(),
  budget_quota_settings_codex: createDefaultBudgetQuotaSettings(),
  budget_show_countdown_codex: false,
  budget_show_forecast_codex: false,
  budget_forecast_method_codex: 'cycle',
  budget_forecast_display_codex: 'datetime',
  auto_start: false,
  auto_update: true,
  update_history_keep_count: 3,
  auto_connectivity_test: false,
  enable_switch_notify: true,  // 默认开启
  enable_round_robin: false,   // 默认关闭轮询
  provider_concurrency_limits: {},
  capture_request_log_payload: false,
  sanitize_request_log_payload: true,
}

type AppSettingsResponse = Partial<AppSettings> & {
  budget_quota_used_adjustments?: unknown
  budget_quota_settings?: unknown
  budget_quota_used_adjustments_codex?: unknown
  budget_quota_settings_codex?: unknown
  provider_concurrency_limits?: unknown
}

type SerializedBudgetQuotaAdjustments = {
  five_hour: number
  daily: number
  weekly: number
  monthly: number
  total: number
}

const normalizeBooleanMap = (value: unknown): Record<string, boolean> => {
  if (!value || typeof value !== 'object') return {}
  const normalized: Record<string, boolean> = {}
  Object.entries(value as Record<string, unknown>).forEach(([key, enabled]) => {
    const normalizedKey = key.trim()
    if (!normalizedKey) return
    normalized[normalizedKey] = enabled === true
  })
  return normalized
}

const normalizeAppSettingsResponse = (value: unknown): AppSettings => {
  const data = value && typeof value === 'object'
    ? value as AppSettingsResponse
    : {}
  const normalizedHeatmapDisplay = normalizeHeatmapDisplaySettings({
    dailyScaleFactor: data?.heatmap_daily_scale_factor,
    dailyIntensityMode: data?.heatmap_daily_intensity_mode,
    intensityMetric: data?.heatmap_intensity_metric,
    intensityStopL1: data?.heatmap_intensity_stop_l1,
    intensityStopL2: data?.heatmap_intensity_stop_l2,
    intensityStopL3: data?.heatmap_intensity_stop_l3,
  })
  const normalizedHomeProviderTabs = normalizeHomeProviderTabs(data?.home_provider_tabs)
  return {
    ...DEFAULT_SETTINGS,
    ...data,
    heatmap_granularity: normalizeHeatmapGranularity(
      data?.heatmap_granularity,
      DEFAULT_SETTINGS.heatmap_granularity,
    ),
    heatmap_daily_scale_factor: normalizedHeatmapDisplay.dailyScaleFactor,
    heatmap_daily_intensity_mode: normalizedHeatmapDisplay.dailyIntensityMode,
    heatmap_intensity_metric: normalizedHeatmapDisplay.intensityMetric,
    heatmap_intensity_stop_l1: normalizedHeatmapDisplay.intensityStopL1,
    heatmap_intensity_stop_l2: normalizedHeatmapDisplay.intensityStopL2,
    heatmap_intensity_stop_l3: normalizedHeatmapDisplay.intensityStopL3,
    home_provider_tabs: normalizedHomeProviderTabs,
    budget_quota_used_adjustments: normalizeBudgetQuotaAdjustments(
      data?.budget_quota_used_adjustments,
      {
        adjustment: data?.budget_used_adjustment,
        cycleEnabled: data?.budget_cycle_enabled,
        cycleMode: data?.budget_cycle_mode,
      },
    ),
    budget_quota_settings: normalizeBudgetQuotaSettings(data?.budget_quota_settings, {
      total: data?.budget_total,
      cycleEnabled: data?.budget_cycle_enabled,
      cycleMode: data?.budget_cycle_mode,
      refreshTime: data?.budget_refresh_time,
      refreshWeekday: data?.budget_refresh_day,
      refreshMonthDay: data?.budget_refresh_month_day,
    }),
    budget_quota_used_adjustments_codex: normalizeBudgetQuotaAdjustments(
      data?.budget_quota_used_adjustments_codex,
      {
        adjustment: data?.budget_used_adjustment_codex,
        cycleEnabled: data?.budget_cycle_enabled_codex,
        cycleMode: data?.budget_cycle_mode_codex,
      },
    ),
    budget_quota_settings_codex: normalizeBudgetQuotaSettings(data?.budget_quota_settings_codex, {
      total: data?.budget_total_codex,
      cycleEnabled: data?.budget_cycle_enabled_codex,
      cycleMode: data?.budget_cycle_mode_codex,
      refreshTime: data?.budget_refresh_time_codex,
      refreshWeekday: data?.budget_refresh_day_codex,
      refreshMonthDay: data?.budget_refresh_month_day_codex,
    }),
    provider_concurrency_limits: normalizeBooleanMap(data?.provider_concurrency_limits),
  }
}

const serializeBudgetQuotaAdjustments = (value: unknown): SerializedBudgetQuotaAdjustments => {
  const normalized = normalizeBudgetQuotaAdjustments(value)
  return {
    five_hour: normalized.five_hour,
    daily: normalized.daily,
    weekly: normalized.weekly,
    monthly: normalized.monthly,
    total: normalized.total,
  }
}

const serializeAppSettings = (settings: AppSettings) => {
  const budgetLegacy = projectBudgetQuotaToLegacy(
    settings.budget_quota_settings,
    settings.budget_quota_used_adjustments,
  )
  const budgetLegacyCodex = projectBudgetQuotaToLegacy(
    settings.budget_quota_settings_codex,
    settings.budget_quota_used_adjustments_codex,
  )

  return {
    ...settings,
    budget_total: budgetLegacy.total,
    budget_used_adjustment: budgetLegacy.adjustment,
    budget_cycle_enabled: budgetLegacy.cycleEnabled,
    budget_cycle_mode: budgetLegacy.cycleMode,
    budget_refresh_time: budgetLegacy.refreshTime,
    budget_refresh_day: budgetLegacy.refreshWeekday,
    budget_refresh_month_day: budgetLegacy.refreshMonthDay,
    budget_quota_used_adjustments: serializeBudgetQuotaAdjustments(settings.budget_quota_used_adjustments),
    budget_quota_settings: serializeBudgetQuotaSettings(settings.budget_quota_settings),
    budget_total_codex: budgetLegacyCodex.total,
    budget_used_adjustment_codex: budgetLegacyCodex.adjustment,
    budget_cycle_enabled_codex: budgetLegacyCodex.cycleEnabled,
    budget_cycle_mode_codex: budgetLegacyCodex.cycleMode,
    budget_refresh_time_codex: budgetLegacyCodex.refreshTime,
    budget_refresh_day_codex: budgetLegacyCodex.refreshWeekday,
    budget_refresh_month_day_codex: budgetLegacyCodex.refreshMonthDay,
    home_provider_tabs: normalizeHomeProviderTabs(settings.home_provider_tabs),
    provider_concurrency_limits: normalizeBooleanMap(settings.provider_concurrency_limits),
    budget_quota_used_adjustments_codex: serializeBudgetQuotaAdjustments(settings.budget_quota_used_adjustments_codex),
    budget_quota_settings_codex: serializeBudgetQuotaSettings(settings.budget_quota_settings_codex),
  }
}

export const fetchAppSettings = async (): Promise<AppSettings> => {
  const data = await Call.ByName('codeswitch/services.AppSettingsService.GetAppSettings')
  return normalizeAppSettingsResponse(data)
}

export const saveAppSettings = async (settings: AppSettings): Promise<AppSettings> => {
  const data = await Call.ByName(
    'codeswitch/services.AppSettingsService.SaveAppSettings',
    serializeAppSettings(settings),
  )
  return normalizeAppSettingsResponse(data)
}
