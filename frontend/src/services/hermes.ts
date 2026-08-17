/**
 * @name: Hermes 配置服务封装
 * @Descripttion: 通过 Call.ByName 调用后端 HermesService，管理 additive 模式供应商、原生 config.yaml 写入状态与 MEMORY/USER 记忆文件
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/services/hermes.ts
 */
import { Call } from '@wailsio/runtime'
import type { AutomationCard } from '../data/cards'
import { normalizeProviderRef, cardProviderRef } from '../utils/providerRefs'

// HermesProvider 与后端 HermesService.HermesProvider 对齐（JSON camelCase）：
// live 条目写入 ~/.hermes/config.yaml 顶层 custom_providers 数组（snake_case 托管字段）
export interface HermesProvider {
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

export interface HermesStatus {
  // config.yaml 是否已存在
  configExists: boolean
  // 当前生效供应商 ID（additive 模式下指向顶层 model 节指向的条目）
  currentProviderId: string
  currentProviderName: string
  // live custom_providers 条目数
  providerCount: number
}

export type HermesMemoryKind = 'memory' | 'user'

export interface HermesMemorySettings {
  memoryEnabled: boolean
  memoryCharLimit: number
  userProfileEnabled: boolean
  userCharLimit: number
}

const serviceName = 'codeswitch/services.HermesService'

const asTrimmedString = (raw: any): string => (typeof raw === 'string' ? raw.trim() : '')

// 后端 HermesMemorySettings 的 JSON 键为 snake_case（memory_enabled 等），这里做兼容归一
const normalizeMemorySettings = (raw: any): HermesMemorySettings => {
  const obj = raw ?? {}
  const charLimit = (value: unknown, fallback: number) => {
    const numeric = Number(value)
    return Number.isFinite(numeric) && numeric > 0 ? Math.floor(numeric) : fallback
  }
  return {
    memoryEnabled: obj.memory_enabled !== false && obj.MemoryEnabled !== false,
    memoryCharLimit: charLimit(obj.memory_char_limit ?? obj.MemoryCharLimit, 50000),
    userProfileEnabled: obj.user_profile_enabled !== false && obj.UserProfileEnabled !== false,
    userCharLimit: charLimit(obj.user_char_limit ?? obj.UserCharLimit, 10000),
  }
}

// 归一化后端返回的供应商（baseUrl 兼容 apiUrl 键，避免卡片 URL 显示为空）
const normalizeHermesProvider = (raw: any): HermesProvider => {
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
const toHermesProviderPayload = (provider: HermesProvider): HermesProvider => ({
  ...provider,
  id: normalizeProviderRef(provider.id),
  baseUrl: asTrimmedString(provider.baseUrl ?? (provider as any).apiUrl),
})

export const getHermesProviders = async (): Promise<HermesProvider[]> => {
  const raw = await Call.ByName(`${serviceName}.GetProviders`)
  return Array.isArray(raw) ? raw.map(normalizeHermesProvider) : []
}

// 新增后返回落盘的完整供应商（携带最终 ID），供调用方精确回填卡片 providerRef
export const addHermesProvider = async (provider: HermesProvider): Promise<HermesProvider | null> => {
  const raw = await Call.ByName(`${serviceName}.AddProvider`, toHermesProviderPayload(provider))
  return raw && typeof raw === 'object' ? normalizeHermesProvider(raw) : null
}

// 更新后返回落盘的完整供应商，供调用方核对最终 ID
export const updateHermesProvider = async (provider: HermesProvider): Promise<HermesProvider | null> => {
  const raw = await Call.ByName(`${serviceName}.UpdateProvider`, toHermesProviderPayload(provider))
  return raw && typeof raw === 'object' ? normalizeHermesProvider(raw) : null
}

export const deleteHermesProvider = async (id: string): Promise<void> => {
  await Call.ByName(`${serviceName}.DeleteProvider`, id)
}

export const duplicateHermesProvider = async (id: string): Promise<HermesProvider | null> => {
  const raw = await Call.ByName(`${serviceName}.DuplicateProvider`, id)
  return raw && typeof raw === 'object' ? normalizeHermesProvider(raw) : null
}

// additive 模式切换：把顶层 model 节指向指定条目（custom_providers 全部共存保留）
export const setCurrentHermesProvider = async (id: string): Promise<void> => {
  await Call.ByName(`${serviceName}.SetCurrentProvider`, id)
}

export const getHermesStatus = async (): Promise<HermesStatus> => {
  const raw = await Call.ByName(`${serviceName}.GetStatus`)
  const obj = raw ?? {}
  return {
    configExists: obj.configExists === true || obj.ConfigExists === true,
    currentProviderId: `${obj.currentProviderId ?? obj.CurrentProviderID ?? ''}`.trim(),
    currentProviderName: `${obj.currentProviderName ?? obj.CurrentProviderName ?? ''}`.trim(),
    providerCount: Number(obj.providerCount ?? obj.ProviderCount ?? 0) || 0,
  }
}

// 首次接入：读取现有 config.yaml 的 custom_providers 条目并导入，返回是否实际导入（后端返回导入数量）
export const importHermesFromLive = async (): Promise<boolean> => {
  const raw = await Call.ByName(`${serviceName}.ImportFromLive`)
  return Number(raw) > 0
}

// ========== Memory 子页（~/.hermes/memories/MEMORY.md 与 USER.md） ==========

export const getHermesMemoryContent = async (kind: HermesMemoryKind): Promise<string> => {
  const raw = await Call.ByName(`${serviceName}.GetMemoryContent`, kind)
  return typeof raw === 'string' ? raw : ''
}

export const writeHermesMemoryContent = async (kind: HermesMemoryKind, content: string): Promise<void> => {
  await Call.ByName(`${serviceName}.WriteMemoryContent`, kind, content)
}

// 条目按单独一行的 § 切分（后端已过滤空白条目）
export const getHermesMemoryEntries = async (kind: HermesMemoryKind): Promise<string[]> => {
  const raw = await Call.ByName(`${serviceName}.GetMemoryEntries`, kind)
  return Array.isArray(raw) ? raw.map((entry) => `${entry ?? ''}`) : []
}

export const getHermesMemorySettings = async (): Promise<HermesMemorySettings> => {
  const raw = await Call.ByName(`${serviceName}.GetMemorySettings`)
  return normalizeMemorySettings(raw)
}

// 后端签名为 SetMemorySettings(enabled bool, charLimit int, userEnabled bool, userCharLimit int)，四个参数拆开传
export const setHermesMemorySettings = async (settings: HermesMemorySettings): Promise<void> => {
  await Call.ByName(
    `${serviceName}.SetMemorySettings`,
    settings.memoryEnabled === true,
    Number(settings.memoryCharLimit) || 0,
    settings.userProfileEnabled === true,
    Number(settings.userCharLimit) || 0,
  )
}

// ========== 卡片映射（HermesProvider ⇄ 首页卡片） ==========

// Hermes 卡片默认模型存放在 cliConfig.model（与 Gemini 的 GEMINI_MODEL 同法），随通用表单链路无损往返；
// 数字回退 ID 取负数区间 -(501+index)，与真实正数 ID 及 Date.now() 生成的 providerRef 无碰撞
const HERMES_CARD_ID_BASE = 501

export const hermesToCard = (provider: HermesProvider, index: number): AutomationCard => {
  const numericId = Number(provider.id)
  const cliConfig = { ...(provider.cliConfig || {}) }
  if (provider.model) {
    cliConfig.model = provider.model
  }
  return {
    id: Number.isFinite(numericId) && numericId > 0 ? numericId : -(HERMES_CARD_ID_BASE + index),
    providerRef: normalizeProviderRef(provider.id),
    name: provider.name,
    apiUrl: provider.baseUrl || '',
    apiKey: provider.apiKey || '',
    officialSite: '',
    icon: '',
    tint: 'rgba(14, 165, 233, 0.16)',
    accent: '#0ea5e9',
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

// 卡片 → HermesProvider（编辑/新增时的保存 payload；original 保留导入时的原生片段快照）
export const cardToHermesProvider = (card: AutomationCard, original?: HermesProvider): HermesProvider => {
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
