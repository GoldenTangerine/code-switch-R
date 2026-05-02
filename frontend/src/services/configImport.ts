import { Call } from '@wailsio/runtime'

export type ConfigImportStatus = {
  config_exists: boolean
  config_path?: string
  pending_providers: boolean
  pending_mcp: boolean
  pending_provider_count: number
  pending_mcp_count: number
}

export type ConfigImportResult = {
  status: ConfigImportStatus
  imported_providers: number
  imported_mcp: number
}

const emptyStatus: ConfigImportStatus = {
  config_exists: false,
  pending_providers: false,
  pending_mcp: false,
  pending_provider_count: 0,
  pending_mcp_count: 0
}

export const fetchConfigImportStatus = async (): Promise<ConfigImportStatus> => {
  const response = await Call.ByName('codeswitch/services.ImportService.GetStatus')
  return (response as ConfigImportStatus) ?? emptyStatus
}

export const importLegacyConfig = async (): Promise<ConfigImportResult> => {
  const response = await Call.ByName('codeswitch/services.ImportService.ImportAll')
  return response as ConfigImportResult
}

// 从指定路径导入配置
export const importFromPath = async (path: string): Promise<ConfigImportResult> => {
  const response = await Call.ByName('codeswitch/services.ImportService.ImportFromPath', path)
  return response as ConfigImportResult
}

// 检查是否首次使用（用于显示导入提示）
export const isFirstRun = async (): Promise<boolean> => {
  const response = await Call.ByName('codeswitch/services.ImportService.IsFirstRun')
  return response as boolean
}

// 标记首次使用已完成（不再显示导入提示）
export const markFirstRunDone = async (): Promise<void> => {
  await Call.ByName('codeswitch/services.ImportService.MarkFirstRunDone')
}
