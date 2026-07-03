/**
 * @name: Codex OAuth 服务
 * @Descripttion: 封装 ChatGPT Codex OAuth 认证的前端调用
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-03 00:00:00
 * @LastEditTime: 2026-07-03 00:00:00
 * @FilePath: frontend/src/services/codexOAuth.ts
 */
import { Call } from '@wailsio/runtime'

const SERVICE_PATH = 'codeswitch/services.CodexOAuthService'

export type CodexOAuthDeviceCodeResponse = {
  provider: string
  deviceCode: string
  userCode: string
  verificationUri: string
  expiresIn: number
  interval: number
}

export type CodexOAuthAccount = {
  id: string
  provider: string
  login: string
  authenticatedAt: number
  isDefault: boolean
}

export type CodexOAuthStatus = {
  provider: string
  authenticated: boolean
  defaultAccountId?: string
  accounts: CodexOAuthAccount[]
  providerCard?: unknown
}

export const startCodexOAuthLogin = async (): Promise<CodexOAuthDeviceCodeResponse> => {
  return Call.ByName(`${SERVICE_PATH}.StartLogin`) as Promise<CodexOAuthDeviceCodeResponse>
}

export const pollCodexOAuthLogin = async (deviceCode: string): Promise<CodexOAuthAccount | null> => {
  return Call.ByName(`${SERVICE_PATH}.PollLogin`, deviceCode) as Promise<CodexOAuthAccount | null>
}

export const fetchCodexOAuthStatus = async (): Promise<CodexOAuthStatus> => {
  return Call.ByName(`${SERVICE_PATH}.GetStatus`) as Promise<CodexOAuthStatus>
}

export const setDefaultCodexOAuthAccount = async (accountId: string): Promise<void> => {
  await Call.ByName(`${SERVICE_PATH}.SetDefaultAccount`, accountId)
}

export const removeCodexOAuthAccount = async (accountId: string): Promise<void> => {
  await Call.ByName(`${SERVICE_PATH}.RemoveAccount`, accountId)
}

export const logoutCodexOAuth = async (): Promise<void> => {
  await Call.ByName(`${SERVICE_PATH}.Logout`)
}
