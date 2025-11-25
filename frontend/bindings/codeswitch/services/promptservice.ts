// Prompt 数据结构
export interface Prompt {
  id: string
  name: string
  content: string
  description?: string
  enabled: boolean
  createdAt?: number
  updatedAt?: number
}

import { Call } from '@wailsio/runtime'

// GetPrompts 获取指定平台的所有提示词
export function GetPrompts(platform: string): Promise<Record<string, Prompt>> {
  return Call.ByName('codeswitch/services.PromptService.GetPrompts', platform)
}

// UpsertPrompt 添加或更新提示词
export function UpsertPrompt(platform: string, id: string, prompt: Prompt): Promise<void> {
  return Call.ByName('codeswitch/services.PromptService.UpsertPrompt', platform, id, prompt)
}

// DeletePrompt 删除提示词
export function DeletePrompt(platform: string, id: string): Promise<void> {
  return Call.ByName('codeswitch/services.PromptService.DeletePrompt', platform, id)
}

// EnablePrompt 启用指定提示词（会禁用同平台其他提示词）
export function EnablePrompt(platform: string, id: string): Promise<void> {
  return Call.ByName('codeswitch/services.PromptService.EnablePrompt', platform, id)
}

// ImportFromFile 从现有文件导入提示词
export function ImportFromFile(platform: string): Promise<string> {
  return Call.ByName('codeswitch/services.PromptService.ImportFromFile', platform)
}

// GetCurrentFileContent 获取当前提示词文件内容
export function GetCurrentFileContent(platform: string): Promise<string | null> {
  return Call.ByName('codeswitch/services.PromptService.GetCurrentFileContent', platform)
}
