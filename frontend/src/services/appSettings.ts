import { Call } from '@wailsio/runtime'

export type AppSettings = {
  show_heatmap: boolean
  show_home_title: boolean
  auto_start: boolean
  auto_update: boolean
  auto_connectivity_test: boolean
}

const DEFAULT_SETTINGS: AppSettings = {
  show_heatmap: true,
  show_home_title: true,
  auto_start: false,
  auto_update: true,
  auto_connectivity_test: false,
}

export const fetchAppSettings = async (): Promise<AppSettings> => {
  const data = await Call.ByName('codeswitch/services.AppSettingsService.GetAppSettings')
  return data ?? DEFAULT_SETTINGS
}

export const saveAppSettings = async (settings: AppSettings): Promise<AppSettings> => {
  return Call.ByName('codeswitch/services.AppSettingsService.SaveAppSettings', settings)
}
