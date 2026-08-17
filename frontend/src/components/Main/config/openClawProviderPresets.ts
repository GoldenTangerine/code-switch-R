/**
 * @name: OpenClaw 供应商预设
 * @Descripttion: 提供 OpenClaw 原生 settings 的官方/第三方接入预设（additive 模式，无推广字段）
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/components/Main/config/openClawProviderPresets.ts
 */
export type OpenClawProviderCategory = 'official' | 'third_party' | 'aggregator' | 'custom'

export interface OpenClawProviderPreset {
  name: string
  websiteUrl: string
  baseUrl?: string
  category: OpenClawProviderCategory
  icon: string
  // 官方默认态预设：无需 API Key，settings 保持官方端点
  isOfficial?: boolean
  description?: string
}

export const openClawProviderPresets: OpenClawProviderPreset[] = [
  {
    name: 'OpenClaw 官方',
    websiteUrl: 'https://openclaw.ai',
    category: 'official',
    icon: 'claude',
    isOfficial: true,
    description: 'OpenClaw 官方默认端点，使用官方登录态，无需 API Key。',
  },
  {
    name: 'Anthropic 官方直连',
    websiteUrl: 'https://www.anthropic.com',
    baseUrl: 'https://api.anthropic.com',
    category: 'official',
    icon: 'claude',
    description: '使用 Anthropic 官方 API Key 直连 Messages API。',
  },
  {
    name: 'OpenRouter',
    websiteUrl: 'https://openrouter.ai',
    baseUrl: 'https://openrouter.ai/api/v1',
    category: 'aggregator',
    icon: 'openrouter',
    description: '通过 OpenRouter 聚合访问 Claude 系列模型。',
  },
]
