/**
 * @name: Claude 模型路由服务
 * @Descripttion: 封装 Claude 模型路由刷新与状态查询
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-14 14:39:25
 * @LastEditTime: 2026-07-14 14:39:25
 * @FilePath: frontend/src/services/claudeModelRouting.ts
 */
import { Call } from '@wailsio/runtime'

const SERVICE_PATH = 'codeswitch/services.ClaudeModelRoutingService'

export interface ClaudeModelRoutingStatus {
  refreshing: boolean
  lastSuccessAt?: string
  providerCount: number
  successCount: number
  failureCount: number
  staleCount: number
  lastFailedNames?: string[]
}

export interface ClaudeModelRefreshResult {
  successCount: number
  failureCount: number
  failedProviders?: string[]
  finishedAt: string
}

export async function getClaudeModelRoutingStatus(): Promise<ClaudeModelRoutingStatus> {
  return await Call.ByName(`${SERVICE_PATH}.GetStatus`) as ClaudeModelRoutingStatus
}

export async function refreshClaudeModelRoutes(): Promise<ClaudeModelRefreshResult> {
  return await Call.ByName(`${SERVICE_PATH}.RefreshAll`) as ClaudeModelRefreshResult
}
