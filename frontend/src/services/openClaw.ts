/**
 * @name: OpenClaw 配置服务封装
 * @Descripttion: 通过 Call.ByName 调用后端 OpenClawService，管理 additive 模式供应商、原生 settings 写入状态与 env/tools/agents 三个子配置
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/services/openClaw.ts
 */
import { Call } from '@wailsio/runtime'
import type { AutomationCard } from '../data/cards'
import { normalizeProviderRef, cardProviderRef } from '../utils/providerRefs'

// OpenClawProvider 与后端 OpenClawService.OpenClawProvider 对齐（JSON camelCase）：
// live 条目写入 ~/.openclaw/openclaw.json 的 models.providers.<id> 节（baseUrl/apiKey/model 托管，
// 原生片段由 cliConfig 无损往返）
export interface OpenClawProvider {
  id: string
  name: string
  baseUrl?: string
  apiKey?: string
  model?: string
  enabled: boolean
  level?: number
  category?: string
  cliConfig?: Record<string, any>
}

export interface OpenClawStatus {
  // 原生 settings 文件路径
  settingsPath: string
  // settings 文件是否已存在
  settingsExists: boolean
  // 当前生效供应商 ID（additive 模式下指向 settings 中启用的条目）
  currentProviderId: string
  currentProviderName: string
  // 受管供应商数量
  managedProviderCount: number
}

export interface OpenClawEnvConfig {
  vars: Record<string, string>
  shellEnv: Record<string, string>
}

export interface OpenClawToolsConfig {
  profile: string
  allow: string[]
  deny: string[]
}

export type OpenClawAgentsConfig = Record<string, any>

const serviceName = 'codeswitch/services.OpenClawService'

const asStringRecord = (raw: any): Record<string, string> => {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return {}
  const normalized: Record<string, string> = {}
  Object.entries(raw as Record<string, unknown>).forEach(([key, value]) => {
    if (typeof value === 'string') {
      normalized[key] = value
    } else if (value !== undefined && value !== null) {
      normalized[key] = `${value}`
    }
  })
  return normalized
}

const asStringList = (raw: any): string[] => (
  Array.isArray(raw) ? raw.map((item) => `${item ?? ''}`.trim()).filter(Boolean) : []
)

const asTrimmedString = (raw: any): string => (typeof raw === 'string' ? raw.trim() : '')

// 归一化后端返回的供应商（baseUrl 兼容 apiUrl 键，避免卡片 URL 显示为空）
const normalizeOpenClawProvider = (raw: any): OpenClawProvider => {
  const obj = raw ?? {}
  return {
    id: `${obj.id ?? obj.ID ?? ''}`.trim(),
    name: `${obj.name ?? obj.Name ?? ''}`.trim(),
    baseUrl: asTrimmedString(obj.baseUrl ?? obj.BaseURL ?? obj.apiUrl),
    apiKey: asTrimmedString(obj.apiKey ?? obj.APIKey),
    model: asTrimmedString(obj.model ?? obj.Model),
    enabled: obj.enabled !== false && obj.Enabled !== false,
    level: Number(obj.level ?? obj.Level ?? 1) || 1,
    category: asTrimmedString(obj.category ?? obj.Category) || 'custom',
    cliConfig: obj.cliConfig && typeof obj.cliConfig === 'object' && !Array.isArray(obj.cliConfig)
      ? obj.cliConfig as Record<string, any>
      : {},
  }
}

// 发送给后端的 payload：baseUrl 取 baseUrl ?? apiUrl，多余键后端会忽略
const toOpenClawProviderPayload = (provider: OpenClawProvider): OpenClawProvider => ({
  ...provider,
  id: normalizeProviderRef(provider.id),
  baseUrl: asTrimmedString(provider.baseUrl ?? (provider as any).apiUrl),
})

// 归一化 GetStatus 返回的 map（后端键为 configExists/providerCount/currentProviderId/currentProviderName）
const normalizeStatus = (raw: any): OpenClawStatus => {
  const obj = raw ?? {}
  const configExists = obj.configExists === true || obj.settingsExists === true
  return {
    settingsPath: typeof obj.settingsPath === 'string' ? obj.settingsPath : `${obj.SettingsPath ?? ''}`,
    settingsExists: configExists,
    currentProviderId: `${obj.currentProviderId ?? obj.CurrentProviderID ?? ''}`.trim(),
    currentProviderName: `${obj.currentProviderName ?? obj.CurrentProviderName ?? ''}`.trim(),
    managedProviderCount: Number(obj.providerCount ?? obj.managedProviderCount ?? 0) || 0,
  }
}

export const getOpenClawProviders = async (): Promise<OpenClawProvider[]> => {
  const raw = await Call.ByName(`${serviceName}.GetProviders`)
  return Array.isArray(raw) ? raw.map(normalizeOpenClawProvider) : []
}

// 新增后返回落盘的完整供应商（携带最终 ID），供调用方精确回填卡片 providerRef
export const addOpenClawProvider = async (provider: OpenClawProvider): Promise<OpenClawProvider | null> => {
  const raw = await Call.ByName(`${serviceName}.AddProvider`, toOpenClawProviderPayload(provider))
  return raw && typeof raw === 'object' ? normalizeOpenClawProvider(raw) : null
}

// 更新后返回落盘的完整供应商，供调用方核对最终 ID
export const updateOpenClawProvider = async (provider: OpenClawProvider): Promise<OpenClawProvider | null> => {
  const raw = await Call.ByName(`${serviceName}.UpdateProvider`, toOpenClawProviderPayload(provider))
  return raw && typeof raw === 'object' ? normalizeOpenClawProvider(raw) : null
}

export const deleteOpenClawProvider = async (id: string): Promise<void> => {
  await Call.ByName(`${serviceName}.DeleteProvider`, id)
}

export const duplicateOpenClawProvider = async (id: string): Promise<OpenClawProvider | null> => {
  const raw = await Call.ByName(`${serviceName}.DuplicateProvider`, id)
  return raw && typeof raw === 'object' ? raw as OpenClawProvider : null
}

// additive 模式切换：把指定条目设为 settings 中当前生效的供应商
export const setCurrentOpenClawProvider = async (id: string): Promise<void> => {
  await Call.ByName(`${serviceName}.SetCurrentProvider`, id)
}

export const getOpenClawStatus = async (): Promise<OpenClawStatus> => {
  const raw = await Call.ByName(`${serviceName}.GetStatus`)
  return normalizeStatus(raw)
}

// 首次接入：读取现有 OpenClaw settings 的供应商条目并导入，返回是否实际导入（后端返回导入数量）
export const importOpenClawFromLive = async (): Promise<boolean> => {
  const raw = await Call.ByName(`${serviceName}.ImportFromLive`)
  return Number(raw) > 0
}

export const getOpenClawEnvConfig = async (): Promise<OpenClawEnvConfig> => {
  const raw = await Call.ByName(`${serviceName}.GetEnvConfig`)
  const obj = raw ?? {}
  return {
    vars: asStringRecord(obj.vars ?? obj.Vars),
    shellEnv: asStringRecord(obj.shellEnv ?? obj.ShellEnv),
  }
}

export const setOpenClawEnvConfig = async (config: OpenClawEnvConfig): Promise<void> => {
  // 后端签名为 SetEnvConfig(vars map[string]string, shellEnv map[string]string)，需拆两参传
  await Call.ByName(`${serviceName}.SetEnvConfig`, config.vars, config.shellEnv)
}

export const getOpenClawToolsConfig = async (): Promise<OpenClawToolsConfig> => {
  const raw = await Call.ByName(`${serviceName}.GetToolsConfig`)
  const obj = raw ?? {}
  const profile = `${obj.profile ?? obj.Profile ?? ''}`.trim()
  return {
    profile: ['minimal', 'coding', 'messaging', 'full'].includes(profile) ? profile : 'full',
    allow: asStringList(obj.allow ?? obj.Allow),
    deny: asStringList(obj.deny ?? obj.Deny),
  }
}

export const setOpenClawToolsConfig = async (config: OpenClawToolsConfig): Promise<void> => {
  // 后端签名为 SetToolsConfig(profile string, allow []string, deny []string)，需拆三参传
  await Call.ByName(`${serviceName}.SetToolsConfig`, config.profile, config.allow, config.deny)
}

export const getOpenClawAgentsConfig = async (): Promise<OpenClawAgentsConfig> => {
  const raw = await Call.ByName(`${serviceName}.GetAgentsConfig`)
  return raw && typeof raw === 'object' && !Array.isArray(raw)
    ? raw as OpenClawAgentsConfig
    : {}
}

export const setOpenClawAgentsConfig = async (config: OpenClawAgentsConfig): Promise<void> => {
  await Call.ByName(`${serviceName}.SetAgentsConfig`, config)
}

// ========== 卡片映射（OpenClawProvider ⇄ 首页卡片） ==========

// OpenClaw 卡片默认模型存放在 cliConfig.model（应用侧元数据，不写入 live 托管字段），随通用表单链路无损往返；
// 数字回退 ID 取负数区间 -(401+index)，与真实正数 ID 及 Date.now() 生成的 providerRef 无碰撞
export const openClawToCard = (provider: OpenClawProvider, index: number): AutomationCard => {
  const numericId = Number(provider.id)
  const cliConfig = { ...(provider.cliConfig || {}) }
  if (provider.model) {
    cliConfig.model = provider.model
  }
  return {
    id: Number.isFinite(numericId) && numericId > 0 ? numericId : -(401 + index),
    providerRef: normalizeProviderRef(provider.id),
    name: provider.name,
    apiUrl: provider.baseUrl || '',
    apiKey: provider.apiKey || '',
    officialSite: '',
    icon: '',
    tint: 'rgba(16, 185, 129, 0.16)',
    accent: '#10b981',
    enabled: provider.enabled,
    sortOrder: index + 1,
    enabledSortOrder: provider.enabled ? index + 1 : undefined,
    disabledSortOrder: provider.enabled ? undefined : index + 1,
    level: provider.level || 1,
    category: provider.category || 'custom',
    cliConfig,
    availabilityMonitorEnabled: false,
    connectivityAutoBlacklist: false,
    availabilityConfig: undefined,
  }
}

// 卡片 → OpenClawProvider（编辑/新增时的保存 payload；original 保留导入时的原生片段快照）
export const cardToOpenClawProvider = (card: AutomationCard, original?: OpenClawProvider): OpenClawProvider => {
  const cliConfig = { ...(card.cliConfig || {}) }
  const model = `${cliConfig.model ?? ''}`.trim()
  // model 已映射到结构化字段，从片段中移除避免与托管字段重复
  delete cliConfig.model
  return {
    id: cardProviderRef(card),
    name: card.name,
    baseUrl: card.apiUrl || '',
    apiKey: card.apiKey || '',
    model,
    enabled: card.enabled,
    level: card.level || 1,
    category: card.category || 'custom',
    cliConfig: Object.keys(cliConfig).length > 0 ? cliConfig : (original?.cliConfig ?? {}),
  }
}
