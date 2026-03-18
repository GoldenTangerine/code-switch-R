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

// 配置文件预览（用于前端显示原始内容）
export interface CLIConfigFile {
  path: string
  format?: 'json' | 'toml' | 'env' | string
  content: string
}

// CLI 配置数据
export interface CLIConfig {
  platform: CLIPlatform
  fields: CLIConfigField[]
  rawContent?: string
  rawFiles?: CLIConfigFile[]
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

export interface CLIEditorContent {
  format?: 'json' | 'toml' | 'env' | string
  content: string
  lockedFields?: CLIConfigField[]
}

export interface CLINormalizedEditorContent extends CLIEditorContent {
  editable: Record<string, any>
}

const SERVICE_PATH = 'codeswitch/services.CliConfigService'

// 获取指定平台的 CLI 配置
export async function fetchCLIConfig(platform: CLIPlatform): Promise<CLIConfig> {
  return Call.ByName(`${SERVICE_PATH}.GetConfig`, platform)
}

// 保存 CLI 配置
export async function saveCLIConfig(
  platform: CLIPlatform,
  editable: Record<string, any>,
  apiUrl: string = '',
  apiKey: string = '',
  providerName: string = '',
): Promise<void> {
  return Call.ByName(`${SERVICE_PATH}.SaveConfig`, platform, editable, apiUrl, apiKey, providerName)
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

export async function renderCLITemplateEditorContent(
  platform: CLIPlatform,
  template: Record<string, any> = {},
): Promise<CLIEditorContent> {
  return Call.ByName(
    `${SERVICE_PATH}.RenderTemplateEditorContent`,
    platform,
    template,
  )
}

export async function normalizeCLITemplateEditorContent(
  platform: CLIPlatform,
  content: string,
): Promise<CLINormalizedEditorContent> {
  return Call.ByName(
    `${SERVICE_PATH}.NormalizeTemplateEditorContent`,
    platform,
    content,
  )
}

export async function renderCLIConfigEditorContent(
  platform: CLIPlatform,
  editable: Record<string, any> = {},
  apiUrl: string = '',
  apiKey: string = '',
  providerName: string = '',
): Promise<CLIEditorContent> {
  return Call.ByName(
    `${SERVICE_PATH}.RenderEditorContent`,
    platform,
    editable,
    apiUrl,
    apiKey,
    providerName,
  )
}

export async function normalizeCLIConfigEditorContent(
  platform: CLIPlatform,
  content: string,
  apiUrl: string = '',
  apiKey: string = '',
  providerName: string = '',
): Promise<CLINormalizedEditorContent> {
  return Call.ByName(
    `${SERVICE_PATH}.NormalizeEditorContent`,
    platform,
    content,
    apiUrl,
    apiKey,
    providerName,
  )
}

// 恢复默认配置
export async function restoreDefaultConfig(platform: CLIPlatform): Promise<void> {
  return Call.ByName(`${SERVICE_PATH}.RestoreDefault`, platform)
}
