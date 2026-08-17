/**
 * @name: Claude Desktop 配置服务封装
 * @Descripttion: 通过 Call.ByName 调用后端 ClaudeDesktopSettingsService，管理 claude_desktop_config.json 的代理状态、直连应用与首次导入
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/services/claudeDesktopSettings.ts
 */
import { Call } from '@wailsio/runtime'

// 本地类型定义，避免依赖 CI 生成的绑定文件
export interface ClaudeDesktopProxyStatus {
  enabled: boolean
  baseURL: string
}

export interface ClaudeDesktopLiveStatus {
  // direct 直连 / proxy 本地代理 / official 官方默认
  mode: string
  baseUrl: string
  // 当前生效供应商的统一存储数值 ID（字符串形式，如 "12"）；direct/official 模式下为空
  providerId: string
}

const serviceName = 'codeswitch/services.ClaudeDesktopSettingsService'

// 归一化代理状态字段（兼容 Wails 返回的 Go 导出字段名 Enabled/BaseURL）
// 注意：Wails 绑定会给字段赋默认值，所以用 'in' 检查而非 ??
const normalizeProxyStatus = (raw: any): ClaudeDesktopProxyStatus => {
  const obj = raw ?? {}
  const enabled = 'Enabled' in obj ? obj.Enabled : obj.enabled
  const baseURL = 'BaseURL' in obj ? obj.BaseURL : (obj.baseURL ?? obj.base_url)
  return {
    enabled: enabled === undefined ? false : Boolean(enabled),
    baseURL: typeof baseURL === 'string' ? baseURL : '',
  }
}

// 归一化 GetStatus 返回的 map（后端键为 mode/baseUrl/providerId，providerId 为统一存储的数值 ID）
const normalizeLiveStatus = (raw: any): ClaudeDesktopLiveStatus => {
  const obj = raw ?? {}
  const mode = typeof obj.mode === 'string' ? obj.mode : `${obj.Mode ?? ''}`
  return {
    mode: mode.trim().toLowerCase(),
    baseUrl: typeof obj.baseUrl === 'string' ? obj.baseUrl : `${obj.base_url ?? obj.BaseURL ?? ''}`,
    providerId: `${obj.providerId ?? obj.provider_id ?? obj.ProviderID ?? ''}`.trim(),
  }
}

export const fetchClaudeDesktopProxyStatus = async (): Promise<ClaudeDesktopProxyStatus> => {
  const raw = await Call.ByName(`${serviceName}.ProxyStatus`)
  return normalizeProxyStatus(raw)
}

export const applyClaudeDesktopSingleProvider = async (providerID: number): Promise<void> => {
  await Call.ByName(`${serviceName}.ApplySingleProvider`, providerID)
}

export const getClaudeDesktopStatus = async (): Promise<ClaudeDesktopLiveStatus> => {
  const raw = await Call.ByName(`${serviceName}.GetStatus`)
  return normalizeLiveStatus(raw)
}

// 首次接入：读取现有 claude_desktop_config.json 的环境变量并导入为供应商，返回是否实际导入
export const importClaudeDesktopFromLive = async (): Promise<boolean> => {
  const raw = await Call.ByName(`${serviceName}.ImportFromLive`)
  return raw === true
}
