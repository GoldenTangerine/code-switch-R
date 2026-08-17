/**
 * @name: Grok Build 供应商预设
 * @Descripttion: 提供 ~/.grok/config.toml 的 [model.<profile>] 头部预设与 TOML 生成/同步工具
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/components/Main/config/grokProviderPresets.ts
 */

export type GrokProviderCategory = 'official' | 'third_party' | 'aggregator' | 'custom'

export interface GrokProviderPreset {
  name: string
  websiteUrl: string
  // 官方 OAuth 态预设不填 baseUrl（config.toml 无自定义模型表）
  baseUrl?: string
  // api_backend 决定 CLI 在 base_url 后拼接的子路径：responses -> /responses
  apiBackend: string
  // 默认模型 ID（写入 [model.<profile>].model）
  model: string
  // [model.<profile>] 表名，同时写入 [models].default
  profile: string
  contextWindow?: number
  category: GrokProviderCategory
  icon: string
  iconColor?: string
  // 官方登录态：TOML 留空，直连应用时写空文件回到官方态
  isOfficial?: boolean
  description?: string
}

export const GROK_DEFAULT_API_BACKEND = 'responses'

export const grokProviderPresets: GrokProviderPreset[] = [
  {
    name: 'Grok Official',
    websiteUrl: 'https://x.ai',
    apiBackend: GROK_DEFAULT_API_BACKEND,
    model: '',
    profile: '',
    category: 'official',
    icon: 'grok',
    isOfficial: true,
    description: 'xAI 官方登录态（OAuth），无需 API Key，config.toml 保持空配置。',
  },
  {
    name: 'xAI 官方直连',
    websiteUrl: 'https://x.ai',
    baseUrl: 'https://api.x.ai/v1',
    apiBackend: GROK_DEFAULT_API_BACKEND,
    model: 'grok-4-fast',
    profile: 'xai',
    contextWindow: 2000000,
    category: 'official',
    icon: 'grok',
    description: '使用 xAI 官方 API Key 直连，走 Responses API。',
  },
  {
    name: 'OpenRouter',
    websiteUrl: 'https://openrouter.ai',
    baseUrl: 'https://openrouter.ai/api/v1',
    apiBackend: GROK_DEFAULT_API_BACKEND,
    model: 'x-ai/grok-4-fast',
    profile: 'openrouter',
    contextWindow: 2000000,
    category: 'aggregator',
    icon: 'openrouter',
    description: '通过 OpenRouter 聚合访问 Grok 系列模型。',
  },
  {
    name: 'AiHubMix',
    websiteUrl: 'https://aihubmix.com',
    baseUrl: 'https://api.aihubmix.com/v1',
    apiBackend: GROK_DEFAULT_API_BACKEND,
    model: 'grok-4-fast',
    profile: 'aihubmix',
    contextWindow: 2000000,
    category: 'aggregator',
    icon: 'generic',
    description: '通过 AiHubMix 聚合访问 Grok 系列模型。',
  },
]

// TOML 字符串值转义（密钥可能含引号或反斜杠）
const escapeGrokTOMLString = (value: string) => value
  .replace(/\\/g, '\\\\')
  .replace(/"/g, '\\"')

/**
 * 根据预设生成完整 config.toml 片段
 * 字段顺序与后端 go-toml 结构化重写的输出保持一致，便于往返对比
 */
export const buildGrokPresetConfigTOML = (preset: GrokProviderPreset): string => {
  if (preset.isOfficial) return ''

  const lines = [
    '[models]',
    `default = "${preset.profile}"`,
    '',
    `[model.${preset.profile}]`,
    `model = "${preset.model}"`,
    `base_url = "${preset.baseUrl ?? ''}"`,
    `name = "${preset.name}"`,
    `api_backend = "${preset.apiBackend}"`,
  ]
  if (preset.contextWindow && preset.contextWindow > 0) {
    lines.push(`context_window = ${preset.contextWindow}`)
  }
  return `${lines.join('\n')}\n`
}

// 解析 [models].default 指向的 profile，缺省取首个 [model.*] 表
const resolveGrokSelectedProfile = (toml: string): string => {
  const defaultMatch = toml.match(/^\s*default\s*=\s*"([^"]+)"/m)
  if (defaultMatch?.[1]) return defaultMatch[1]

  const modelTableMatch = toml.match(/^\s*\[model\.([^\]\s]+)\]/m)
  return modelTableMatch?.[1] ?? ''
}

/**
 * 将表单 API Key 与 baseUrl 同步进 TOML 选中 profile 的对应字段
 * - 空值不改动对应字段（避免误删手填的 env_key 方案 / 官方 OAuth 态的空 base_url）
 * - 已有属性行则原位替换，否则追加在 [model.<profile>] 表内末尾
 */
export const syncGrokCredentialsIntoTOML = (toml: string, apiKey: string, baseUrl: string): string => {
  const trimmed = toml.trim()
  // 双空值时原样返回，避免 trim 重建丢掉原文的首尾空白
  if (!trimmed || (!apiKey.trim() && !baseUrl.trim())) return toml

  const profile = resolveGrokSelectedProfile(trimmed)
  if (!profile) return toml

  const headerPattern = new RegExp(`^(\\s*\\[model\\.${profile.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\]\\s*)$`, 'm')
  const headerMatch = trimmed.match(headerPattern)
  if (!headerMatch) return toml

  const sectionStart = (headerMatch.index ?? 0) + headerMatch[0].length
  const nextHeader = trimmed.slice(sectionStart).search(/^\s*\[/m)
  const sectionEnd = nextHeader === -1 ? trimmed.length : sectionStart + nextHeader
  let section = trimmed.slice(sectionStart, sectionEnd)

  const upsertLine = (key: 'api_key' | 'base_url', value: string) => {
    const normalized = value.trim()
    if (!normalized) return
    const line = `${key} = "${escapeGrokTOMLString(normalized)}"`
    const linePattern = new RegExp(`^(\\s*)${key}\\s*=.*$`, 'm')
    if (linePattern.test(section)) {
      section = section.replace(linePattern, `$1${line}`)
    } else {
      section = `${section.replace(/\n*$/, '\n')}${line}\n`
    }
  }

  upsertLine('api_key', apiKey)
  upsertLine('base_url', baseUrl)

  return `${trimmed.slice(0, sectionStart)}${section}${trimmed.slice(sectionEnd)}`
}

// 校验 TOML 片段：非空时必须包含至少一个 [model.*] 表（后端直连应用依赖它）
export const validateGrokConfigTOML = (toml: string): boolean => (
  !toml.trim() || /^\s*\[model\.[^\]\s]+\]/m.test(toml)
)
