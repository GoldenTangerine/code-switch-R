import { Call } from '@wailsio/runtime'

// CLI 平台类型
export type CLIPlatform = 'claude' | 'codex' | 'gemini'

// 配置字段信息
export interface CLIConfigField {
  key: string
  value: string
  locked: boolean
  hint?: string
  type: 'string' | 'boolean' | 'object'
  required?: boolean
}

// CLI 配置数据
export interface CLIConfig {
  platform: CLIPlatform
  fields: CLIConfigField[]
  rawContent?: string
  configFormat?: 'json' | 'toml' | 'env'
  envContent?: Record<string, string>
  filePath?: string
  editable?: Record<string, any>
}

// CLI 配置模板
export interface CLITemplate {
  template: Record<string, any>
  isGlobalDefault: boolean
}

const SERVICE_PATH = 'codeswitch/services.CliConfigService'

// 获取指定平台的 CLI 配置
export async function fetchCLIConfig(platform: CLIPlatform): Promise<CLIConfig> {
  return Call.ByName(`${SERVICE_PATH}.GetConfig`, platform)
}

// 保存 CLI 配置
export async function saveCLIConfig(platform: CLIPlatform, editable: Record<string, any>): Promise<void> {
  return Call.ByName(`${SERVICE_PATH}.SaveConfig`, platform, editable)
}

// 获取指定平台的全局模板
export async function fetchCLITemplate(platform: CLIPlatform): Promise<CLITemplate> {
  return Call.ByName(`${SERVICE_PATH}.GetTemplate`, platform)
}

// 设置指定平台的全局模板
export async function setCLITemplate(
  platform: CLIPlatform,
  template: Record<string, any>,
  isGlobalDefault: boolean
): Promise<void> {
  return Call.ByName(`${SERVICE_PATH}.SetTemplate`, platform, template, isGlobalDefault)
}

// 获取指定平台的锁定字段列表
export async function fetchLockedFields(platform: CLIPlatform): Promise<string[]> {
  return Call.ByName(`${SERVICE_PATH}.GetLockedFields`, platform)
}

// 恢复默认配置
export async function restoreDefaultConfig(platform: CLIPlatform): Promise<void> {
  return Call.ByName(`${SERVICE_PATH}.RestoreDefault`, platform)
}
