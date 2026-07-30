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
export type ClaudeModelMetadataMergeStrategy = 'aggressive' | 'conservative'
export type ClaudeProxyAuthField = 'auth_token' | 'api_key'
export type LogsRefreshIntervalSeconds = 0 | 5 | 10 | 30 | 60

export const DEFAULT_MAIN_WINDOW_DESTROY_DELAY_SECONDS = 30
export const MIN_MAIN_WINDOW_DESTROY_DELAY_SECONDS = 0
export const MAX_MAIN_WINDOW_DESTROY_DELAY_SECONDS = 300

let mainWindowDestroyDelayRevision = Date.now() * 1000

const nextMainWindowDestroyDelayRevision = (): number => {
  mainWindowDestroyDelayRevision = Math.max(mainWindowDestroyDelayRevision + 1, Date.now() * 1000)
  return mainWindowDestroyDelayRevision
}

export const normalizeMainWindowDestroyDelaySeconds = (value?: number | null): number => {
  if (!Number.isFinite(value)) return DEFAULT_MAIN_WINDOW_DESTROY_DELAY_SECONDS
  const normalized = Math.floor(value as number)
  if (normalized < MIN_MAIN_WINDOW_DESTROY_DELAY_SECONDS) return MIN_MAIN_WINDOW_DESTROY_DELAY_SECONDS
  if (normalized > MAX_MAIN_WINDOW_DESTROY_DELAY_SECONDS) return MAX_MAIN_WINDOW_DESTROY_DELAY_SECONDS
  return normalized
}

export const LOGS_REFRESH_INTERVAL_OPTIONS: readonly LogsRefreshIntervalSeconds[] = [0, 5, 10, 30, 60]

export const normalizeLogsRefreshIntervalSeconds = (value?: number | null): LogsRefreshIntervalSeconds => (
  LOGS_REFRESH_INTERVAL_OPTIONS.includes(value as LogsRefreshIntervalSeconds)
    ? value as LogsRefreshIntervalSeconds
    : 30
)

export const normalizeClaudeProxyAuthField = (value?: string | null): ClaudeProxyAuthField => (
  value === 'api_key' ? 'api_key' : 'auth_token'
)

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
  logs_refresh_interval_seconds: LogsRefreshIntervalSeconds
  main_window_destroy_delay_seconds: number
  auto_connectivity_test: boolean
  enable_switch_notify: boolean // 供应商切换通知开关
  enable_round_robin: boolean   // 同 Level 轮询负载均衡开关
  claude_model_routing_enabled: boolean
  claude_model_aggregation_enabled: boolean
  claude_model_metadata_merge_strategy: ClaudeModelMetadataMergeStrategy
  claude_proxy_auth_field: ClaudeProxyAuthField
  preserve_codex_official_auth_on_switch: boolean
  unify_codex_session_history: boolean
  unify_codex_migrate_existing: boolean
  provider_concurrency_limits: Record<string, boolean>
  provider_quota_query_preset_codes: Record<string, string>
  provider_quota_query_presets: ProviderQuotaQueryPresetGroups
  provider_quota_auto_disable_enabled: boolean
  capture_request_log_payload: boolean
  sanitize_request_log_payload: boolean
}

export type ProviderQuotaQueryPresetEntry = {
  id: string
  name: string
  code: string
  updatedAt?: number
}

export type ProviderQuotaQueryPresetGroup = {
  defaultId?: string
  items: ProviderQuotaQueryPresetEntry[]
}

export type ProviderQuotaQueryPresetGroups = Record<string, ProviderQuotaQueryPresetGroup>

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
  logs_refresh_interval_seconds: 30,
  main_window_destroy_delay_seconds: DEFAULT_MAIN_WINDOW_DESTROY_DELAY_SECONDS,
  auto_connectivity_test: false,
  enable_switch_notify: true,  // 默认开启
  enable_round_robin: false,   // 默认关闭轮询
  claude_model_routing_enabled: false,
  claude_model_aggregation_enabled: false,
  claude_model_metadata_merge_strategy: 'aggressive',
  claude_proxy_auth_field: 'auth_token',
  preserve_codex_official_auth_on_switch: false,
  unify_codex_session_history: false,
  unify_codex_migrate_existing: false,
  provider_concurrency_limits: {},
  provider_quota_query_preset_codes: {},
  provider_quota_query_presets: {},
  provider_quota_auto_disable_enabled: false,
  capture_request_log_payload: false,
  sanitize_request_log_payload: true,
}

type AppSettingsResponse = Partial<AppSettings> & {
  budget_quota_used_adjustments?: unknown
  budget_quota_settings?: unknown
  budget_quota_used_adjustments_codex?: unknown
  budget_quota_settings_codex?: unknown
  provider_concurrency_limits?: unknown
  provider_quota_query_preset_codes?: unknown
  provider_quota_query_presets?: unknown
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

const normalizeProviderQuotaQueryPresetCodes = (value: unknown): Record<string, string> => {
  if (!value || typeof value !== 'object') return {}
  const allowedKeys = new Set(['custom', 'general', 'newapi'])
  const normalized: Record<string, string> = {}
  Object.entries(value as Record<string, unknown>).forEach(([key, code]) => {
    const normalizedKey = key.trim().toLowerCase()
    const normalizedCode = `${code ?? ''}`.trim()
    if (!allowedKeys.has(normalizedKey) || !normalizedCode) return
    normalized[normalizedKey] = normalizedCode
  })
  return normalized
}

const providerQuotaQueryPresetTypeSet = new Set(['custom', 'general', 'newapi'])

const normalizeProviderQuotaQueryPresetGroups = (
  value: unknown,
  legacyCodes: Record<string, string> = {},
): ProviderQuotaQueryPresetGroups => {
  const normalized: ProviderQuotaQueryPresetGroups = {}
  const source = value && typeof value === 'object'
    ? value as Record<string, unknown>
    : {}

  Object.entries(source).forEach(([key, groupValue]) => {
    const normalizedKey = key.trim().toLowerCase()
    if (!providerQuotaQueryPresetTypeSet.has(normalizedKey) || !groupValue || typeof groupValue !== 'object') return

    const rawGroup = groupValue as Record<string, unknown>
    const seenIds = new Set<string>()
    const rawItems = Array.isArray(rawGroup.items) ? rawGroup.items : []
    const items: ProviderQuotaQueryPresetEntry[] = rawItems.flatMap((item, index) => {
      if (!item || typeof item !== 'object') return []
      const rawItem = item as Record<string, unknown>
      const code = `${rawItem.code ?? ''}`.trim()
      if (!code) return []
      const fallbackId = `${normalizedKey}-${index + 1}`
      const id = `${rawItem.id ?? fallbackId}`.trim() || fallbackId
      if (seenIds.has(id)) return []
      seenIds.add(id)
      return [{
        id,
        name: `${rawItem.name ?? ''}`.trim() || '自定义预设',
        code,
        updatedAt: Number.isFinite(Number(rawItem.updatedAt)) ? Number(rawItem.updatedAt) : undefined,
      }]
    })

    const defaultId = `${rawGroup.defaultId ?? ''}`.trim()
    if (items.length > 0) {
      normalized[normalizedKey] = {
        defaultId: defaultId && seenIds.has(defaultId) ? defaultId : items[0]?.id,
        items,
      }
    }
  })

  Object.entries(legacyCodes).forEach(([key, code]) => {
    if (normalized[key]?.items.length) return
    const id = `legacy-${key}`
    normalized[key] = {
      defaultId: id,
      items: [{
        id,
        name: '自定义预设',
        code,
      }],
    }
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
  const normalizedProviderQuotaQueryPresetCodes = normalizeProviderQuotaQueryPresetCodes(
    data?.provider_quota_query_preset_codes,
  )
  const routingEnabled = data?.claude_model_routing_enabled === true
  const mergeStrategy: ClaudeModelMetadataMergeStrategy = data?.claude_model_metadata_merge_strategy === 'conservative'
    ? 'conservative'
    : 'aggressive'
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
    claude_model_routing_enabled: routingEnabled,
    claude_model_aggregation_enabled: routingEnabled && data?.claude_model_aggregation_enabled === true,
    claude_model_metadata_merge_strategy: mergeStrategy,
    claude_proxy_auth_field: normalizeClaudeProxyAuthField(data?.claude_proxy_auth_field),
    logs_refresh_interval_seconds: normalizeLogsRefreshIntervalSeconds(data?.logs_refresh_interval_seconds),
    main_window_destroy_delay_seconds: normalizeMainWindowDestroyDelaySeconds(data?.main_window_destroy_delay_seconds),
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
    provider_quota_query_preset_codes: normalizedProviderQuotaQueryPresetCodes,
    provider_quota_query_presets: normalizeProviderQuotaQueryPresetGroups(
      data?.provider_quota_query_presets,
      normalizedProviderQuotaQueryPresetCodes,
    ),
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
    claude_model_aggregation_enabled:
      settings.claude_model_routing_enabled && settings.claude_model_aggregation_enabled,
    claude_model_metadata_merge_strategy:
      settings.claude_model_metadata_merge_strategy === 'conservative' ? 'conservative' : 'aggressive',
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
    provider_quota_query_preset_codes: normalizeProviderQuotaQueryPresetCodes(
      settings.provider_quota_query_preset_codes,
    ),
    provider_quota_query_presets: normalizeProviderQuotaQueryPresetGroups(
      settings.provider_quota_query_presets,
    ),
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

export const saveLogsRefreshInterval = async (
  seconds: LogsRefreshIntervalSeconds,
): Promise<AppSettings> => {
  const data = await Call.ByName(
    'codeswitch/services.AppSettingsService.SetLogsRefreshInterval',
    normalizeLogsRefreshIntervalSeconds(seconds),
  )
  return normalizeAppSettingsResponse(data)
}

export const saveMainWindowDestroyDelay = async (seconds: number): Promise<AppSettings> => {
  const data = await Call.ByName(
    'codeswitch/services.AppSettingsService.SetMainWindowDestroyDelay',
    normalizeMainWindowDestroyDelaySeconds(seconds),
    nextMainWindowDestroyDelayRevision(),
  )
  return normalizeAppSettingsResponse(data)
}
