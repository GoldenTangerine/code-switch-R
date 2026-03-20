import { Call } from '@wailsio/runtime'

export type LogPlatform = 'claude' | 'codex' | 'gemini'

export type RequestLog = {
  id: number
  platform: LogPlatform | ''
  model: string
  requested_model?: string
  response_model?: string
  provider_id?: string
  provider: string
  http_code: number
  input_tokens: number
  output_tokens: number
  cache_create_tokens: number
  ephemeral_5m_tokens?: number
  ephemeral_1h_tokens?: number
  cache_read_tokens: number
  reasoning_tokens: number
  is_stream?: boolean | number
  duration_sec?: number
  first_token_sec?: number
  created_at: string
  total_cost?: number
  input_cost?: number
  output_cost?: number
  reasoning_cost?: number
  cache_create_cost?: number
  cache_read_cost?: number
  ephemeral_5m_cost?: number
  ephemeral_1h_cost?: number
  group_multiplier?: number
  has_pricing?: boolean
  matched_pricing_model?: string
  price_source?: string
  provider_pricing_available?: boolean
  provider_quota_type?: number
  provider_input_usd_per_m?: number
  provider_output_usd_per_m?: number
  provider_per_call_unified?: number
  provider_per_call_input?: number
  provider_per_call_output?: number
  provider_per_call_unified_set?: boolean
  provider_per_call_input_set?: boolean
  provider_per_call_output_set?: boolean
  response_body?: string
  response_body_truncated?: boolean
}

type RequestLogQuery = {
  platform?: LogPlatform | ''
  provider?: string
  limit?: number
  offset?: number
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

export type RequestLogPageResult = {
  items: RequestLog[]
  total: number
  limit: number
  offset: number
}

export const fetchRequestLogsPage = async (query: RequestLogQuery = {}): Promise<RequestLogPageResult> => {
  const platform = query.platform ?? ''
  const provider = query.provider ?? ''
  const limit = query.limit ?? 100
  const offset = query.offset ?? 0
  const startAt = query.startAt ?? ''
  const endAt = query.endAt ?? ''
  return Call.ByName('codeswitch/services.LogService.ListRequestLogsPageV2', platform, provider, limit, offset, startAt, endAt)
}

export const fetchFailedRequestLogsPage = async (query: RequestLogQuery = {}): Promise<RequestLogPageResult> => {
  const platform = query.platform ?? ''
  const provider = query.provider ?? ''
  const limit = query.limit ?? 100
  const offset = query.offset ?? 0
  const startAt = query.startAt ?? ''
  const endAt = query.endAt ?? ''
  return Call.ByName('codeswitch/services.LogService.ListFailedRequestLogsPageV2', platform, provider, limit, offset, startAt, endAt)
}

export type RequestLogPayloadDetail = {
  id: number
  request_body: string
  response_body: string
  request_body_truncated?: boolean
  response_body_truncated?: boolean
}

export const fetchRequestLogPayload = async (id: number): Promise<RequestLogPayloadDetail> => {
  const normalized = Number.isFinite(id) ? Math.floor(id) : 0
  return Call.ByName('codeswitch/services.LogService.GetRequestLogPayload', normalized)
}

export type LogProviderRef = {
  provider_id?: string
  provider: string
}

export const fetchLogProviderRefs = async (platform: LogPlatform | '' = ''): Promise<LogProviderRef[]> => {
  return Call.ByName('codeswitch/services.LogService.ListProviderRefs', platform)
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

export type FiveHourQuotaStatus = {
  active: boolean
  window_start: string
  next_reset: string
  used: number
}

export const fetchFiveHourQuotaStatus = async (
  platform: LogPlatform | '' = '',
): Promise<FiveHourQuotaStatus> => {
  return Call.ByName('codeswitch/services.LogService.ResolveFiveHourQuotaStatus', platform)
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

export type ProviderLogStorageStat = {
  platform: LogPlatform | ''
  provider_id?: string
  provider: string
  request_log_rows: number
  request_log_bytes: number
  stats_rows: number
  stats_bytes: number
  total_bytes: number
  latest_at: string
}

export const fetchProviderLogStorageStats = async (): Promise<ProviderLogStorageStat[]> => {
  return Call.ByName('codeswitch/services.LogService.ListProviderLogStorageStats')
}

export const clearRequestLogs = async (): Promise<void> => {
  await Call.ByName('codeswitch/services.LogService.ClearRequestLogs')
}

export type DeleteRequestLogsByDateResult = {
  deleted_request_logs: number
  deleted_stats_hour: number
  deleted_stats_day: number
}

export const deleteRequestLogsByDate = async (date: string): Promise<DeleteRequestLogsByDateResult> => {
  const normalized = String(date ?? '').trim()
  return Call.ByName('codeswitch/services.LogService.DeleteRequestLogsByDate', normalized)
}

export const clearProviderLogStorage = async (
  platform: LogPlatform | '',
  providerId: string,
  provider: string,
): Promise<DeleteRequestLogsByDateResult> => {
  return Call.ByName(
    'codeswitch/services.LogService.ClearProviderLogStorage',
    platform ?? '',
    String(providerId ?? '').trim(),
    String(provider ?? '').trim(),
  )
}

export const clearLogStats = async (): Promise<void> => {
  await Call.ByName('codeswitch/services.LogService.ClearLogStats')
}

export type ProviderDailyStat = {
  provider_id?: string
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
  avg_first_token_sec?: number
  avg_tokens_per_sec?: number
  ttft_sample_count?: number
  tps_sample_count?: number
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

export type ModelUsageStat = {
  model: string
  total_requests: number
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  total_tokens: number
  cost_total: number
}

type ModelStatsQuery = {
  platform?: LogPlatform | ''
  provider?: string
  startAt?: string
  endAt?: string
}

export const fetchModelStatsV2 = async (
  query: ModelStatsQuery = {},
): Promise<ModelUsageStat[]> => {
  const platform = query.platform ?? ''
  const provider = query.provider ?? ''
  const startAt = query.startAt ?? ''
  const endAt = query.endAt ?? ''
  return Call.ByName('codeswitch/services.LogService.ModelStatsRangeV2', platform, provider, startAt, endAt)
}

export type HeatmapStat = {
  day: string
  total_requests: number
  input_tokens: number
  output_tokens: number
  cache_read_tokens?: number
  reasoning_tokens: number
  total_tokens?: number
  total_cost: number
  payload_bytes?: number
  payload_captured_requests?: number
}

export const fetchHeatmapStats = async (days: number): Promise<HeatmapStat[]> => {
  const range = Number.isFinite(days) && days > 0 ? Math.floor(days) : 30
  return Call.ByName('codeswitch/services.LogService.HeatmapStats', range)
}

export const fetchRequestLogDailyHeatmapStats = async (days: number): Promise<HeatmapStat[]> => {
  const range = Number.isFinite(days) && days > 0 ? Math.floor(days) : 365
  return Call.ByName('codeswitch/services.LogService.RequestLogDailyHeatmapStats', range)
}

export const fetchRequestLogDailyHeatmapStatsByYear = async (year: number): Promise<HeatmapStat[]> => {
  const normalized = Number.isFinite(year) && year > 0 ? Math.floor(year) : new Date().getFullYear()
  return Call.ByName('codeswitch/services.LogService.RequestLogDailyHeatmapStatsByYear', normalized)
}

export const fetchRequestLogHeatmapYears = async (): Promise<number[]> => {
  return Call.ByName('codeswitch/services.LogService.ListRequestLogHeatmapYears')
}
