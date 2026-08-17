/**
 * @name: Pi 配置服务封装
 * @Descripttion: 通过 Call.ByName 调用后端 PiService，管理 ~/.pi/agent/models.json 的 additive 模式供应商
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 01:25:00
 * @LastEditTime: 2026-08-17 01:25:00
 * @FilePath: frontend/src/services/pi.ts
 */
import { Call } from '@wailsio/runtime'
import type { AutomationCard } from '../data/cards'
import { normalizeProviderRef, cardProviderRef } from '../utils/providerRefs'

// PiProvider 与后端 PiService.PiProvider 对齐（JSON camelCase）：
// live 条目写入 ~/.pi/agent/models.json 顶层 providers.<id>（displayName/baseUrl/apiKey 托管，
// api/models/compat 等原生键由 cliConfig 片段无损往返）
export interface PiProvider {
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

export interface PiStatus {
  // models.json 是否已存在
  configExists: boolean
  // 当前生效供应商 ID（additive 模式下的启用标记）
  currentProviderId: string
  currentProviderName: string
  // live providers 条目数
  providerCount: number
}

const serviceName = 'codeswitch/services.PiService'

const asTrimmedString = (raw: any): string => (typeof raw === 'string' ? raw.trim() : '')

// 归一化后端返回的供应商（baseUrl 兼容 apiUrl 键，避免卡片 URL 显示为空）
const normalizePiProvider = (raw: any): PiProvider => {
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
const toPiProviderPayload = (provider: PiProvider): PiProvider => ({
  ...provider,
  id: normalizeProviderRef(provider.id),
  baseUrl: asTrimmedString(provider.baseUrl ?? (provider as any).apiUrl),
})

export const getPiProviders = async (): Promise<PiProvider[]> => {
  const raw = await Call.ByName(`${serviceName}.GetProviders`)
  return Array.isArray(raw) ? raw.map(normalizePiProvider) : []
}

// 新增后返回落盘的完整供应商（携带最终 ID），供调用方精确回填卡片 providerRef
export const addPiProvider = async (provider: PiProvider): Promise<PiProvider | null> => {
  const raw = await Call.ByName(`${serviceName}.AddProvider`, toPiProviderPayload(provider))
  return raw && typeof raw === 'object' ? normalizePiProvider(raw) : null
}

// 更新后返回落盘的完整供应商，供调用方核对最终 ID
export const updatePiProvider = async (provider: PiProvider): Promise<PiProvider | null> => {
  const raw = await Call.ByName(`${serviceName}.UpdateProvider`, toPiProviderPayload(provider))
  return raw && typeof raw === 'object' ? normalizePiProvider(raw) : null
}

export const deletePiProvider = async (id: string): Promise<void> => {
  await Call.ByName(`${serviceName}.DeleteProvider`, id)
}

export const duplicatePiProvider = async (id: string): Promise<PiProvider | null> => {
  const raw = await Call.ByName(`${serviceName}.DuplicateProvider`, id)
  return raw && typeof raw === 'object' ? normalizePiProvider(raw) : null
}

// additive 模式切换：providers 条目全部共存保留，仅统一存储单选启用标记
export const setCurrentPiProvider = async (id: string): Promise<void> => {
  await Call.ByName(`${serviceName}.SetCurrentProvider`, id)
}

export const getPiStatus = async (): Promise<PiStatus> => {
  const raw = await Call.ByName(`${serviceName}.GetStatus`)
  const obj = raw ?? {}
  return {
    configExists: obj.configExists === true || obj.ConfigExists === true,
    currentProviderId: `${obj.currentProviderId ?? obj.CurrentProviderID ?? ''}`.trim(),
    currentProviderName: `${obj.currentProviderName ?? obj.CurrentProviderName ?? ''}`.trim(),
    providerCount: Number(obj.providerCount ?? obj.ProviderCount ?? 0) || 0,
  }
}

// 首次接入：读取现有 models.json 的 providers 条目并导入，返回是否实际导入（后端返回导入数量）
export const importPiFromLive = async (): Promise<boolean> => {
  const raw = await Call.ByName(`${serviceName}.ImportFromLive`)
  return Number(raw) > 0
}

// ========== 卡片映射（PiProvider ⇄ 首页卡片） ==========

// Pi 卡片默认模型存放在 cliConfig.model（应用侧元数据，不写入 live 条目），随通用表单链路无损往返；
// 数字回退 ID 取负数区间 -(601+index)，与真实正数 ID 及 Date.now() 生成的 providerRef 无碰撞
const PI_CARD_ID_BASE = 601

export const piToCard = (provider: PiProvider, index: number): AutomationCard => {
  const numericId = Number(provider.id)
  const cliConfig = { ...(provider.cliConfig || {}) }
  if (provider.model) {
    cliConfig.model = provider.model
  }
  return {
    id: Number.isFinite(numericId) && numericId > 0 ? numericId : -(PI_CARD_ID_BASE + index),
    providerRef: normalizeProviderRef(provider.id),
    name: provider.name,
    apiUrl: provider.baseUrl || '',
    apiKey: provider.apiKey || '',
    officialSite: '',
    icon: '',
    tint: 'rgba(139, 92, 246, 0.16)',
    accent: '#8b5cf6',
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

// 卡片 → PiProvider（编辑/新增时的保存 payload；original 保留导入时的原生片段快照）
export const cardToPiProvider = (card: AutomationCard, original?: PiProvider): PiProvider => {
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
