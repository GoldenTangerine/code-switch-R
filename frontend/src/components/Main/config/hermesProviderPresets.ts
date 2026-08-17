/**
 * @name: Hermes 供应商预设
 * @Descripttion: 提供 Hermes 原生 config.yaml 的官方/第三方接入预设（additive 模式，无推广字段）
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/components/Main/config/hermesProviderPresets.ts
 */
export type HermesProviderCategory = 'official' | 'third_party' | 'aggregator' | 'custom'

export interface HermesProviderPreset {
  name: string
  websiteUrl: string
  baseUrl?: string
  category: HermesProviderCategory
  icon: string
  // 官方默认态预设：无需 API Key，保持官方登录态端点
  isOfficial?: boolean
  description?: string
}

export const hermesProviderPresets: HermesProviderPreset[] = [
  {
    name: 'Hermes 官方',
    websiteUrl: 'http://127.0.0.1:9119',
    category: 'official',
    icon: 'claude',
    isOfficial: true,
    description: 'Hermes 官方默认端点，使用官方登录态，无需 API Key；官网指向本地 Web UI。',
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
