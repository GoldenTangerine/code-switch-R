import { Call } from '@wailsio/runtime'

export type LogPlatform = 'claude' | 'codex' | 'gemini'

export type RequestLog = {
  id: number
  platform: LogPlatform | ''
  model: string
  provider: string
  http_code: number
  input_tokens: number
  output_tokens: number
  cache_create_tokens: number
  cache_read_tokens: number
  reasoning_tokens: number
  is_stream?: boolean | number
  duration_sec?: number
  created_at: string
  total_cost?: number
  input_cost?: number
  output_cost?: number
  cache_create_cost?: number
  cache_read_cost?: number
  ephemeral_5m_cost?: number
  ephemeral_1h_cost?: number
  has_pricing?: boolean
  matched_pricing_model?: string
}

type RequestLogQuery = {
  platform?: LogPlatform | ''
  provider?: string
  limit?: number
  startAt?: string
  endAt?: string
}

export const fetchRequestLogs = async (query: RequestLogQuery = {}): Promise<RequestLog[]> => {
  const platform = query.platform ?? ''
  const provider = query.provider ?? ''
  const limit = query.limit ?? 100
  const startAt = query.startAt ?? ''
  const endAt = query.endAt ?? ''
  return Call.ByName('codeswitch/services.LogService.ListRequestLogsV2', platform, provider, limit, startAt, endAt)
}

export const fetchLogProviders = async (platform: LogPlatform | '' = ''): Promise<string[]> => {
  return Call.ByName('codeswitch/services.LogService.ListProviders', platform)
}

export type LogStatsSeries = {
  day: string
  total_requests: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  cache_create_tokens: number
  cache_read_tokens: number
  total_cost: number
}

export type LogStats = {
  total_requests: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  cache_create_tokens: number
  cache_read_tokens: number
  cost_total: number
  cost_input: number
  cost_output: number
  cost_cache_create: number
  cost_cache_read: number
  series: LogStatsSeries[]
}

export const fetchLogStats = async (platform: LogPlatform | '' = ''): Promise<LogStats> => {
  return Call.ByName('codeswitch/services.LogService.StatsSince', platform)
}

type LogStatsQuery = {
  platform?: LogPlatform | ''
  provider?: string
  startAt?: string
  endAt?: string
}

export const fetchLogStatsV2 = async (query: LogStatsQuery = {}): Promise<LogStats> => {
  const platform = query.platform ?? ''
  const provider = query.provider ?? ''
  const startAt = query.startAt ?? ''
  const endAt = query.endAt ?? ''
  return Call.ByName('codeswitch/services.LogService.StatsRangeV2', platform, provider, startAt, endAt)
}

export const fetchCostSince = async (start: string, platform: LogPlatform | '' = ''): Promise<number> => {
  return Call.ByName('codeswitch/services.LogService.CostSince', start, platform)
}

export type LogTableStorageStat = {
  name: string
  rows: number
  bytes: number
}

export type LogDatabaseStorageStat = {
  file_bytes: number
  wal_bytes: number
  shm_bytes: number
  total_bytes: number
  used_bytes: number
  free_bytes: number
}

export type LogStorageStats = {
  database: LogDatabaseStorageStat
  request_log: LogTableStorageStat
  stats_hour: LogTableStorageStat
  stats_day: LogTableStorageStat
}

export const fetchLogStorageStats = async (): Promise<LogStorageStats> => {
  return Call.ByName('codeswitch/services.LogService.GetLogStorageStats')
}

export const clearRequestLogs = async (): Promise<void> => {
  await Call.ByName('codeswitch/services.LogService.ClearRequestLogs')
}

export const clearLogStats = async (): Promise<void> => {
  await Call.ByName('codeswitch/services.LogService.ClearLogStats')
}

export type ProviderDailyStat = {
  provider: string
  total_requests: number
  successful_requests: number
  failed_requests: number
  success_rate: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  cache_create_tokens: number
  cache_read_tokens: number
  cost_total: number
}

export const fetchProviderDailyStats = async (
  platform: LogPlatform | '' = '',
): Promise<ProviderDailyStat[]> => {
  return Call.ByName('codeswitch/services.LogService.ProviderDailyStats', platform)
}

type ProviderStatsQuery = {
  platform?: LogPlatform | ''
  provider?: string
  startAt?: string
  endAt?: string
}

export const fetchProviderStatsV2 = async (
  query: ProviderStatsQuery = {},
): Promise<ProviderDailyStat[]> => {
  const platform = query.platform ?? ''
  const provider = query.provider ?? ''
  const startAt = query.startAt ?? ''
  const endAt = query.endAt ?? ''
  return Call.ByName('codeswitch/services.LogService.ProviderStatsRangeV2', platform, provider, startAt, endAt)
}

export type HeatmapStat = {
  day: string
  total_requests: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  total_cost: number
}

export const fetchHeatmapStats = async (days: number): Promise<HeatmapStat[]> => {
  const range = Number.isFinite(days) && days > 0 ? Math.floor(days) : 30
  return Call.ByName('codeswitch/services.LogService.HeatmapStats', range)
}
