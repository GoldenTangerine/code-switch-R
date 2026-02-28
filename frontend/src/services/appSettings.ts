import { Call } from '@wailsio/runtime'
import {
  DEFAULT_HEATMAP_DISPLAY_SETTINGS,
  normalizeHeatmapDisplaySettings,
} from '../data/heatmapDisplaySettings'

export type HeatmapGranularity = 'hourly' | 'daily'

export const normalizeHeatmapGranularity = (
  value?: string | null,
): HeatmapGranularity => (value === 'daily' ? 'daily' : 'hourly')

export type AppSettings = {
  show_heatmap: boolean
  heatmap_granularity: HeatmapGranularity
  heatmap_daily_scale_factor: number
  heatmap_daily_intensity_mode: 'hourly_scaled' | 'daily_peak'
  heatmap_intensity_stop_l1: number
  heatmap_intensity_stop_l2: number
  heatmap_intensity_stop_l3: number
  show_home_title: boolean
  budget_total: number
  budget_used_adjustment: number
  budget_cycle_enabled: boolean
  budget_cycle_mode: string
  budget_refresh_time: string
  budget_refresh_day: number
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
  capture_request_log_payload: boolean
}

const DEFAULT_SETTINGS: AppSettings = {
  show_heatmap: true,
  heatmap_granularity: 'hourly',
  heatmap_daily_scale_factor: DEFAULT_HEATMAP_DISPLAY_SETTINGS.dailyScaleFactor,
  heatmap_daily_intensity_mode: DEFAULT_HEATMAP_DISPLAY_SETTINGS.dailyIntensityMode,
  heatmap_intensity_stop_l1: DEFAULT_HEATMAP_DISPLAY_SETTINGS.intensityStopL1,
  heatmap_intensity_stop_l2: DEFAULT_HEATMAP_DISPLAY_SETTINGS.intensityStopL2,
  heatmap_intensity_stop_l3: DEFAULT_HEATMAP_DISPLAY_SETTINGS.intensityStopL3,
  show_home_title: true,
  budget_total: 0,
  budget_used_adjustment: 0,
  budget_cycle_enabled: false,
  budget_cycle_mode: 'daily',
  budget_refresh_time: '00:00',
  budget_refresh_day: 1,
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
  capture_request_log_payload: false,
}

export const fetchAppSettings = async (): Promise<AppSettings> => {
  const data = await Call.ByName('codeswitch/services.AppSettingsService.GetAppSettings')
  const normalizedHeatmapDisplay = normalizeHeatmapDisplaySettings({
    dailyScaleFactor: data?.heatmap_daily_scale_factor,
    dailyIntensityMode: data?.heatmap_daily_intensity_mode,
    intensityStopL1: data?.heatmap_intensity_stop_l1,
    intensityStopL2: data?.heatmap_intensity_stop_l2,
    intensityStopL3: data?.heatmap_intensity_stop_l3,
  })
  return {
    ...DEFAULT_SETTINGS,
    ...data,
    heatmap_granularity: normalizeHeatmapGranularity(data?.heatmap_granularity),
    heatmap_daily_scale_factor: normalizedHeatmapDisplay.dailyScaleFactor,
    heatmap_daily_intensity_mode: normalizedHeatmapDisplay.dailyIntensityMode,
    heatmap_intensity_stop_l1: normalizedHeatmapDisplay.intensityStopL1,
    heatmap_intensity_stop_l2: normalizedHeatmapDisplay.intensityStopL2,
    heatmap_intensity_stop_l3: normalizedHeatmapDisplay.intensityStopL3,
  }
}

export const saveAppSettings = async (settings: AppSettings): Promise<AppSettings> => {
  return Call.ByName('codeswitch/services.AppSettingsService.SaveAppSettings', settings)
}
