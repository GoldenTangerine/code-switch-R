/**
 * @name: Grok Build 配置服务封装
 * @Descripttion: 通过 Call.ByName 调用后端 GrokSettingsService，管理 ~/.grok/config.toml 的代理状态、直连应用与首次导入
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/services/grokSettings.ts
 */
import { Call } from '@wailsio/runtime'

// 本地类型定义，避免依赖 CI 生成的绑定文件
export interface GrokProxyStatus {
  enabled: boolean
  baseURL: string
}

export interface GrokLiveStatus {
  official: boolean
  configExists: boolean
  profile: string
  model: string
  baseUrl: string
  apiBackend: string
}

const serviceName = 'codeswitch/services.GrokSettingsService'

// 归一化代理状态字段（兼容 Wails 返回的 Go 导出字段名 Enabled/BaseURL）
// 注意：Wails 绑定会给字段赋默认值，所以用 'in' 检查而非 ??
const normalizeProxyStatus = (raw: any): GrokProxyStatus => {
  const obj = raw ?? {}
  const enabled = 'Enabled' in obj ? obj.Enabled : obj.enabled
  const baseURL = 'BaseURL' in obj ? obj.BaseURL : (obj.baseURL ?? obj.base_url)
  return {
    enabled: enabled === undefined ? false : Boolean(enabled),
    baseURL: typeof baseURL === 'string' ? baseURL : '',
  }
}

// 归一化 GetStatus 返回的 map（后端键为 official/configExists/profile/model/baseUrl/apiBackend）
const normalizeLiveStatus = (raw: any): GrokLiveStatus => {
  const obj = raw ?? {}
  return {
    official: obj.official === true,
    configExists: obj.configExists === true,
    profile: typeof obj.profile === 'string' ? obj.profile : '',
    model: typeof obj.model === 'string' ? obj.model : '',
    baseUrl: typeof obj.baseUrl === 'string' ? obj.baseUrl : '',
    apiBackend: typeof obj.apiBackend === 'string' ? obj.apiBackend : '',
  }
}

export const fetchGrokProxyStatus = async (): Promise<GrokProxyStatus> => {
  const raw = await Call.ByName(`${serviceName}.ProxyStatus`)
  return normalizeProxyStatus(raw)
}

export const enableGrokProxy = async (): Promise<void> => {
  await Call.ByName(`${serviceName}.EnableProxy`)
}

export const disableGrokProxy = async (): Promise<void> => {
  await Call.ByName(`${serviceName}.DisableProxy`)
}

export const applyGrokSingleProvider = async (providerID: number): Promise<void> => {
  await Call.ByName(`${serviceName}.ApplySingleProvider`, providerID)
}

export const getGrokStatus = async (): Promise<GrokLiveStatus> => {
  const raw = await Call.ByName(`${serviceName}.GetStatus`)
  return normalizeLiveStatus(raw)
}

// 首次接入：检测 ~/.grok/config.toml 的自定义模型表并导入为供应商，返回是否实际导入
export const importGrokFromLive = async (): Promise<boolean> => {
  const raw = await Call.ByName(`${serviceName}.ImportFromLive`)
  return raw === true
}
