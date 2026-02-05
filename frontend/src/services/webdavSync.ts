import { Call } from '@wailsio/runtime'

export type WebDAVSyncConfig = {
  endpoint: string
  username: string
  password: string
  remote_dir: string
  remote_file: string
  timeout_seconds: number
}

export type WebDAVTestResult = {
  ok: boolean
  message: string
  remote_url?: string
}

export type WebDAVSyncResult = {
  ok: boolean
  message: string
  remote_url?: string
  bytes?: number
  backup_path?: string
  includes?: string[]
}

const DEFAULT_CONFIG: WebDAVSyncConfig = {
  endpoint: '',
  username: '',
  password: '',
  remote_dir: '',
  remote_file: 'codeswitch-config.zip',
  timeout_seconds: 20,
}

const SERVICE = 'codeswitch/services.WebDAVSyncService'

export const fetchWebDAVConfig = async (): Promise<WebDAVSyncConfig> => {
  const data = await Call.ByName(`${SERVICE}.GetConfig`)
  return (data as WebDAVSyncConfig) ?? DEFAULT_CONFIG
}

export const saveWebDAVConfig = async (cfg: WebDAVSyncConfig): Promise<WebDAVSyncConfig> => {
  const data = await Call.ByName(`${SERVICE}.SaveConfig`, cfg)
  return data as WebDAVSyncConfig
}

export const testWebDAVConfig = async (cfg: WebDAVSyncConfig): Promise<WebDAVTestResult> => {
  const data = await Call.ByName(`${SERVICE}.TestConfig`, cfg)
  return data as WebDAVTestResult
}

export const previewWebDAVContent = async (): Promise<WebDAVSyncResult> => {
  const data = await Call.ByName(`${SERVICE}.PreviewLocalContent`)
  return data as WebDAVSyncResult
}

export const syncToWebDAV = async (cfg: WebDAVSyncConfig): Promise<WebDAVSyncResult> => {
  const data = await Call.ByName(`${SERVICE}.SyncToWebDAV`, cfg)
  return data as WebDAVSyncResult
}

export const loadFromWebDAV = async (cfg: WebDAVSyncConfig): Promise<WebDAVSyncResult> => {
  const data = await Call.ByName(`${SERVICE}.LoadFromWebDAV`, cfg)
  return data as WebDAVSyncResult
}
