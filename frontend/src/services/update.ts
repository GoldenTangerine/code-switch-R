import { Call } from '@wailsio/runtime'

export interface UpdateInfo {
  available: boolean
  version: string
  download_url: string
  release_notes: string
  file_size: number
  sha256: string
}

export interface UpdateState {
  last_check_time: string
  last_check_success: boolean
  consecutive_failures: number
  latest_known_version: string
  latest_release_notes?: string
  download_progress: number
  update_ready: boolean
  auto_check_enabled?: boolean
}

export const checkUpdate = async (): Promise<UpdateInfo | null> => {
  return Call.ByName('codeswitch/services.UpdateService.CheckUpdate')
}

export const downloadUpdate = async (progressCallback?: ((progress: number) => void) | null): Promise<void> => {
  return Call.ByName('codeswitch/services.UpdateService.DownloadUpdate', progressCallback ?? null)
}

export const restartApp = async (): Promise<void> => {
  return Call.ByName('codeswitch/services.UpdateService.RestartApp')
}

export const getUpdateState = async (): Promise<UpdateState> => {
  return Call.ByName('codeswitch/services.UpdateService.GetUpdateState')
}

export const setAutoCheckEnabled = async (enabled: boolean): Promise<void> => {
  return Call.ByName('codeswitch/services.UpdateService.SetAutoCheckEnabled', enabled)
}
