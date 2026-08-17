/**
 * @name: Pi 供应商预设
 * @Descripttion: 提供 Pi 原生 ~/.pi/agent/models.json 的官方/第三方接入预设（additive 模式，无推广字段）
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 01:25:00
 * @LastEditTime: 2026-08-17 01:25:00
 * @FilePath: frontend/src/components/Main/config/piProviderPresets.ts
 */
export type PiProviderCategory = 'official' | 'third_party' | 'aggregator' | 'custom'

export interface PiProviderPreset {
  name: string
  websiteUrl: string
  baseUrl?: string
  category: PiProviderCategory
  icon: string
  // 官方默认态预设：无需 API Key，使用 Pi 内置供应商与官方登录态
  isOfficial?: boolean
  description?: string
}

export const piProviderPresets: PiProviderPreset[] = [
  {
    name: 'Pi 官方',
    websiteUrl: 'https://github.com/badlogic/pi-mono',
    category: 'official',
    icon: 'claude',
    isOfficial: true,
    description: 'Pi 内置供应商目录，使用 /login 官方登录态或环境变量密钥，无需在此填写 API Key。',
  },
  {
    name: 'Anthropic 官方直连',
    websiteUrl: 'https://www.anthropic.com',
    baseUrl: 'https://api.anthropic.com',
    category: 'official',
    icon: 'claude',
    description: '使用 Anthropic 官方 API Key 直连 Messages API（覆写内置 anthropic 供应商的接入地址）。',
  },
  {
    name: 'OpenRouter',
    websiteUrl: 'https://openrouter.ai',
    baseUrl: 'https://openrouter.ai/api/v1',
    category: 'aggregator',
    icon: 'openrouter',
    description: '通过 OpenRouter 聚合访问各家模型（OpenAI 兼容端点）。',
  },
]
