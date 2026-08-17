/**
 * @name: Claude Desktop 供应商预设
 * @Descripttion: 提供 claude_desktop_config.json 的官方/第三方接入预设与默认模型路由
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/components/Main/config/claudeDesktopProviderPresets.ts
 */
import type { ClaudeDesktopModelRoute } from '../../../data/cards'

export type ClaudeDesktopProviderCategory = 'official' | 'third_party' | 'aggregator' | 'custom'

export interface ClaudeDesktopProviderPreset {
  name: string
  websiteUrl: string
  baseUrl?: string
  category: ClaudeDesktopProviderCategory
  icon: string
  // 接入模式：direct 直连 API，proxy 走本地 :18100 代理
  mode: 'direct' | 'proxy'
  // 官方登录态预设：无需 API Key，清空环境变量回到官方默认
  isOfficial?: boolean
  description?: string
}

// 官方模型路由默认条目（Sonnet 4.5 支持 1M 上下文）
export const CLAUDE_DESKTOP_DEFAULT_MODEL_ROUTES: ClaudeDesktopModelRoute[] = [
  { name: 'claude-opus-4-5', labelOverride: 'Claude Opus 4.5' },
  { name: 'claude-sonnet-4-5', labelOverride: 'Claude Sonnet 4.5', supports1m: true },
  { name: 'claude-haiku-4-5', labelOverride: 'Claude Haiku 4.5' },
  { name: 'claude-opus-4-1', labelOverride: 'Claude Opus 4.1' },
]

export const claudeDesktopProviderPresets: ClaudeDesktopProviderPreset[] = [
  {
    name: 'Claude 官方',
    websiteUrl: 'https://claude.ai',
    category: 'official',
    icon: 'claude',
    mode: 'direct',
    isOfficial: true,
    description: 'Anthropic 官方直连，使用 Claude 账号登录态，无需 API Key。',
  },
  {
    name: 'Anthropic 官方 API',
    websiteUrl: 'https://www.anthropic.com',
    baseUrl: 'https://api.anthropic.com',
    category: 'official',
    icon: 'claude',
    mode: 'direct',
    description: '使用 Anthropic 官方 API Key 直连 Messages API。',
  },
]

// 创建一条空白的模型路由行
export const createClaudeDesktopModelRouteDraft = (): ClaudeDesktopModelRoute => ({
  name: '',
  labelOverride: '',
  supports1m: false,
})
