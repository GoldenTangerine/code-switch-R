// services/healthcheck.ts
// 可用性监控服务前端调用层
// Author: Half open flowers

import { Call } from '@wailsio/runtime'

// 健康状态常量
export const HealthStatus = {
  OPERATIONAL: 'operational',
  DEGRADED: 'degraded',
  FAILED: 'failed',
  VALIDATION_ERROR: 'validation_failed',
} as const

export type HealthStatusValue = typeof HealthStatus[keyof typeof HealthStatus]

export type LogAvailabilityRange = '15min' | '1h' | '6h' | '24h' | '7d'

// 健康检查结果类型
export interface HealthCheckResult {
  id: number
  providerId: number
  providerName: string
  platform: string
  model?: string
  endpoint?: string
  status: string
  latencyMs: number
  errorMessage: string
  checkedAt: string
}

// Provider 时间线类型
export interface ProviderTimeline {
  providerId: number
  providerName: string
  platform: string
  availabilityMonitorEnabled: boolean
  connectivityAutoBlacklist: boolean
  availabilityConfig?: AvailabilityConfig | null // 高级配置
  items: HealthCheckResult[]
  latest: HealthCheckResult | null
  uptime: number
  avgLatencyMs: number
}

// 可用性高级配置
export interface AvailabilityConfig {
  testModel?: string
  testEndpoint?: string
  timeout?: number
}

type BrowserWindowWithWailsBridge = Window & {
  chrome?: {
    webview?: {
      postMessage?: (...args: any[]) => void
    }
  }
  webkit?: {
    messageHandlers?: {
      external?: {
        postMessage?: (...args: any[]) => void
      }
    }
  }
}

type MockProviderSeed = {
  providerId: number
  providerName: string
  platform: string
  availabilityMonitorEnabled: boolean
  connectivityAutoBlacklist: boolean
  availabilityConfig?: AvailabilityConfig | null
  nominalLatencyMs: number
  historyStatuses: HealthStatusValue[]
  checkSequence: HealthStatusValue[]
  failureMessage?: string
  defaultModel?: string
  defaultEndpoint?: string
}

type MockProviderRuntime = {
  timeline: ProviderTimeline
  checkSequence: HealthStatusValue[]
  checkCount: number
  nominalLatencyMs: number
  failureMessage: string
  defaultModel?: string
  defaultEndpoint?: string
}

type MockHealthHistory = {
  providerId: number
  providerName: string
  platform: string
  items: HealthCheckResult[]
  latest: HealthCheckResult | null
  uptime: number
  avgLatencyMs: number
}

const SERVICE_PATH = 'codeswitch/services.HealthCheckService'
const MOCK_HISTORY_LIMIT = 72
const MOCK_POLLING_DEFAULT = true
const MOCK_TIME_STEP_MS = 37 * 60 * 1000
const MOCK_LATENCY_WAVE = [0, 16, -12, 24, -7, 31, 9, -4] as const
const LOG_AVAILABILITY_RANGE_MS: Record<LogAvailabilityRange, number> = {
  '15min': 15 * 60 * 1000,
  '1h': 60 * 60 * 1000,
  '6h': 6 * 60 * 60 * 1000,
  '24h': 24 * 60 * 60 * 1000,
  '7d': 7 * 24 * 60 * 60 * 1000,
}

const buildMockStatusWindow = (
  length: number,
  baseStatus: HealthStatusValue,
  overrides: Record<number, HealthStatusValue> = {},
): HealthStatusValue[] => Array.from({ length }, (_, index) => overrides[index] ?? baseStatus)

const MOCK_PROVIDER_SEEDS: MockProviderSeed[] = [
  {
    providerId: 100,
    providerName: '0011',
    platform: 'claude',
    availabilityMonitorEnabled: false,
    connectivityAutoBlacklist: false,
    availabilityConfig: {
      testModel: 'claude-3-5-sonnet-latest',
      testEndpoint: '/v1/messages',
      timeout: 15000,
    },
    nominalLatencyMs: 268,
    historyStatuses: buildMockStatusWindow(MOCK_HISTORY_LIMIT, HealthStatus.OPERATIONAL, {
      4: HealthStatus.DEGRADED,
      12: HealthStatus.DEGRADED,
      19: HealthStatus.FAILED,
    }),
    checkSequence: [HealthStatus.OPERATIONAL, HealthStatus.DEGRADED, HealthStatus.OPERATIONAL],
    defaultModel: 'claude-3-5-sonnet-latest',
    defaultEndpoint: '/v1/messages',
  },
  {
    providerId: 101,
    providerName: 'AICoding.sh',
    platform: 'claude',
    availabilityMonitorEnabled: true,
    connectivityAutoBlacklist: false,
    availabilityConfig: {
      testModel: 'claude-3-7-sonnet-20250219',
      testEndpoint: '/v1/messages',
      timeout: 15000,
    },
    nominalLatencyMs: 138,
    historyStatuses: buildMockStatusWindow(MOCK_HISTORY_LIMIT, HealthStatus.OPERATIONAL, {
      7: HealthStatus.DEGRADED,
      16: HealthStatus.DEGRADED,
    }),
    checkSequence: [HealthStatus.OPERATIONAL, HealthStatus.OPERATIONAL, HealthStatus.DEGRADED, HealthStatus.OPERATIONAL],
    defaultModel: 'claude-3-7-sonnet-20250219',
    defaultEndpoint: '/v1/messages',
  },
  {
    providerId: 102,
    providerName: 'Kimi',
    platform: 'claude',
    availabilityMonitorEnabled: true,
    connectivityAutoBlacklist: true,
    availabilityConfig: {
      testModel: 'kimi-k2-0905',
      testEndpoint: '/v1/chat/completions',
      timeout: 18000,
    },
    nominalLatencyMs: 312,
    historyStatuses: buildMockStatusWindow(MOCK_HISTORY_LIMIT, HealthStatus.OPERATIONAL, {
      0: HealthStatus.DEGRADED,
      1: HealthStatus.DEGRADED,
      5: HealthStatus.DEGRADED,
      10: HealthStatus.DEGRADED,
      17: HealthStatus.FAILED,
      23: HealthStatus.DEGRADED,
    }),
    checkSequence: [HealthStatus.DEGRADED, HealthStatus.OPERATIONAL, HealthStatus.DEGRADED, HealthStatus.OPERATIONAL],
    defaultModel: 'kimi-k2-0905',
    defaultEndpoint: '/v1/chat/completions',
  },
  {
    providerId: 103,
    providerName: 'Deepseek',
    platform: 'claude',
    availabilityMonitorEnabled: true,
    connectivityAutoBlacklist: true,
    availabilityConfig: {
      testModel: 'deepseek-chat',
      testEndpoint: '/responses',
      timeout: 15000,
    },
    nominalLatencyMs: 286,
    historyStatuses: buildMockStatusWindow(MOCK_HISTORY_LIMIT, HealthStatus.OPERATIONAL, {
      0: HealthStatus.FAILED,
      1: HealthStatus.FAILED,
      4: HealthStatus.DEGRADED,
      6: HealthStatus.FAILED,
      11: HealthStatus.DEGRADED,
      12: HealthStatus.FAILED,
      18: HealthStatus.DEGRADED,
      21: HealthStatus.FAILED,
    }),
    checkSequence: [HealthStatus.FAILED, HealthStatus.DEGRADED, HealthStatus.FAILED, HealthStatus.OPERATIONAL],
    failureMessage: '上游供应商返回 529 Overloaded，请检查代理出口与重试策略。',
    defaultModel: 'deepseek-chat',
    defaultEndpoint: '/responses',
  },
  {
    providerId: 201,
    providerName: 'AICoding.sh',
    platform: 'codex',
    availabilityMonitorEnabled: true,
    connectivityAutoBlacklist: false,
    availabilityConfig: {
      testModel: 'gpt-5.1-mini',
      testEndpoint: '/responses',
      timeout: 12000,
    },
    nominalLatencyMs: 164,
    historyStatuses: buildMockStatusWindow(MOCK_HISTORY_LIMIT, HealthStatus.OPERATIONAL, {
      8: HealthStatus.DEGRADED,
      15: HealthStatus.DEGRADED,
    }),
    checkSequence: [HealthStatus.OPERATIONAL, HealthStatus.OPERATIONAL, HealthStatus.OPERATIONAL, HealthStatus.DEGRADED],
    defaultModel: 'gpt-5.1-mini',
    defaultEndpoint: '/responses',
  },
  {
    providerId: 301,
    providerName: 'Gemini 2.5 Pro Relay',
    platform: 'gemini',
    availabilityMonitorEnabled: true,
    connectivityAutoBlacklist: false,
    availabilityConfig: {
      testModel: 'gemini-2.5-pro',
      testEndpoint: '/v1beta/models',
      timeout: 20000,
    },
    nominalLatencyMs: 604,
    historyStatuses: buildMockStatusWindow(MOCK_HISTORY_LIMIT, HealthStatus.OPERATIONAL, {
      0: HealthStatus.DEGRADED,
      3: HealthStatus.DEGRADED,
      6: HealthStatus.DEGRADED,
      10: HealthStatus.DEGRADED,
      15: HealthStatus.DEGRADED,
      22: HealthStatus.FAILED,
    }),
    checkSequence: [HealthStatus.DEGRADED, HealthStatus.OPERATIONAL, HealthStatus.DEGRADED, HealthStatus.OPERATIONAL],
    defaultModel: 'gemini-2.5-pro',
    defaultEndpoint: '/v1beta/models',
  },
  {
    providerId: 401,
    providerName: '自建 OpenAI Proxy',
    platform: 'others',
    availabilityMonitorEnabled: true,
    connectivityAutoBlacklist: false,
    availabilityConfig: {
      testModel: 'gpt-4.1-mini',
      testEndpoint: '/v1/chat/completions',
      timeout: 10000,
    },
    nominalLatencyMs: 344,
    historyStatuses: buildMockStatusWindow(MOCK_HISTORY_LIMIT, HealthStatus.OPERATIONAL, {
      9: HealthStatus.DEGRADED,
      18: HealthStatus.DEGRADED,
      24: HealthStatus.DEGRADED,
    }),
    checkSequence: [HealthStatus.OPERATIONAL, HealthStatus.OPERATIONAL, HealthStatus.DEGRADED, HealthStatus.OPERATIONAL],
    defaultModel: 'gpt-4.1-mini',
    defaultEndpoint: '/v1/chat/completions',
  },
]

const hasDesktopRuntimeBridge = () => {
  if (typeof window === 'undefined') {
    return false
  }
  const browserWindow = window as BrowserWindowWithWailsBridge
  return Boolean(
    browserWindow.chrome?.webview?.postMessage ||
    browserWindow.webkit?.messageHandlers?.external?.postMessage,
  )
}

const shouldUseBrowserPreviewAvailabilityMock = () => (
  import.meta.env.DEV
  && typeof window !== 'undefined'
  && !hasDesktopRuntimeBridge()
)

let mockResultId = 10_000
let mockPollingRunning = MOCK_POLLING_DEFAULT
let mockTimelineState: Record<string, MockProviderRuntime[]> | null = null

const nextMockResultId = () => {
  mockResultId += 1
  return mockResultId
}

const cloneAvailabilityConfig = (config?: AvailabilityConfig | null): AvailabilityConfig | null | undefined => {
  if (config === null) return null
  if (config === undefined) return undefined
  return {
    testModel: config.testModel || '',
    testEndpoint: config.testEndpoint || '',
    timeout: config.timeout,
  }
}

const cloneHealthCheckResult = (result: HealthCheckResult | null): HealthCheckResult | null => {
  if (!result) return null
  return {
    ...result,
  }
}

const cloneProviderTimeline = (timeline: ProviderTimeline): ProviderTimeline => ({
  ...timeline,
  availabilityConfig: cloneAvailabilityConfig(timeline.availabilityConfig),
  items: timeline.items.map((item) => ({ ...item })),
  latest: timeline.availabilityMonitorEnabled ? cloneHealthCheckResult(timeline.latest) : null,
})

const cloneLogProviderTimeline = (
  timeline: ProviderTimeline,
  range: LogAvailabilityRange = '24h',
): ProviderTimeline => {
  const cloned = cloneProviderTimeline({ ...timeline, availabilityMonitorEnabled: true })
  const bucketDurationMs = LOG_AVAILABILITY_RANGE_MS[range] / MOCK_HISTORY_LIMIT
  const now = Date.now()
  cloned.availabilityMonitorEnabled = true
  cloned.items = cloned.items.slice(0, MOCK_HISTORY_LIMIT).map((item, index) => ({
    ...item,
    checkedAt: new Date(now - index * bucketDurationMs).toISOString(),
  }))
  cloned.latest = cloned.items.find((item) => item.status) ?? null
  return cloned
}

const resolveMockLatency = (
  nominalLatencyMs: number,
  status: HealthStatusValue,
  checkCount: number,
): number => {
  const wave = MOCK_LATENCY_WAVE[checkCount % MOCK_LATENCY_WAVE.length]

  switch (status) {
    case HealthStatus.OPERATIONAL:
      return Math.max(nominalLatencyMs + wave, 42)
    case HealthStatus.DEGRADED:
      return Math.max(nominalLatencyMs + 220 + Math.abs(wave) * 3, nominalLatencyMs + 180)
    case HealthStatus.FAILED:
    case HealthStatus.VALIDATION_ERROR:
      return 0
    default:
      return Math.max(nominalLatencyMs, 42)
  }
}

const recalculateMockTimeline = (timeline: ProviderTimeline) => {
  timeline.items = timeline.items.slice(0, MOCK_HISTORY_LIMIT)
  timeline.latest = timeline.items[0] ?? null

  if (!timeline.items.length) {
    timeline.uptime = 0
    timeline.avgLatencyMs = 0
    return
  }

  let successCount = 0
  let totalLatency = 0

  timeline.items.forEach((item) => {
    if (item.status === HealthStatus.OPERATIONAL || item.status === HealthStatus.DEGRADED) {
      successCount += 1
      totalLatency += item.latencyMs
    }
  })

  timeline.uptime = Number(((successCount / timeline.items.length) * 100).toFixed(2))
  timeline.avgLatencyMs = successCount > 0 ? Math.round(totalLatency / successCount) : 0
}

const createMockResult = (
  runtime: MockProviderRuntime,
  status: HealthStatusValue,
  checkedAt: string,
): HealthCheckResult => {
  const config = runtime.timeline.availabilityConfig || {}
  return {
    id: nextMockResultId(),
    providerId: runtime.timeline.providerId,
    providerName: runtime.timeline.providerName,
    platform: runtime.timeline.platform,
    model: config.testModel || runtime.defaultModel || '',
    endpoint: config.testEndpoint || runtime.defaultEndpoint || '',
    status,
    latencyMs: resolveMockLatency(runtime.nominalLatencyMs, status, runtime.checkCount),
    errorMessage: status === HealthStatus.FAILED || status === HealthStatus.VALIDATION_ERROR
      ? runtime.failureMessage
      : '',
    checkedAt,
  }
}

const createMockRuntime = (seed: MockProviderSeed): MockProviderRuntime => {
  const historyItems = seed.historyStatuses.map((status, index) => {
    const checkedAt = new Date(Date.now() - index * MOCK_TIME_STEP_MS).toISOString()
    return {
      id: nextMockResultId(),
      providerId: seed.providerId,
      providerName: seed.providerName,
      platform: seed.platform,
      model: seed.availabilityConfig?.testModel || seed.defaultModel || '',
      endpoint: seed.availabilityConfig?.testEndpoint || seed.defaultEndpoint || '',
      status,
      latencyMs: resolveMockLatency(seed.nominalLatencyMs, status, index),
      errorMessage: status === HealthStatus.FAILED || status === HealthStatus.VALIDATION_ERROR
        ? seed.failureMessage || '请求失败，请检查上游节点状态。'
        : '',
      checkedAt,
    }
  })

  const timeline: ProviderTimeline = {
    providerId: seed.providerId,
    providerName: seed.providerName,
    platform: seed.platform,
    availabilityMonitorEnabled: seed.availabilityMonitorEnabled,
    connectivityAutoBlacklist: seed.connectivityAutoBlacklist,
    availabilityConfig: cloneAvailabilityConfig(seed.availabilityConfig),
    items: historyItems,
    latest: historyItems[0] ?? null,
    uptime: 0,
    avgLatencyMs: 0,
  }

  recalculateMockTimeline(timeline)

  return {
    timeline,
    checkSequence: seed.checkSequence.length ? [...seed.checkSequence] : [seed.historyStatuses[0] ?? HealthStatus.OPERATIONAL],
    checkCount: historyItems.length,
    nominalLatencyMs: seed.nominalLatencyMs,
    failureMessage: seed.failureMessage || '请求失败，请检查上游节点状态。',
    defaultModel: seed.defaultModel,
    defaultEndpoint: seed.defaultEndpoint,
  }
}

const createMockTimelineState = (): Record<string, MockProviderRuntime[]> => {
  const state: Record<string, MockProviderRuntime[]> = {
    claude: [],
    codex: [],
    gemini: [],
    opencode: [],
    others: [],
  }

  MOCK_PROVIDER_SEEDS.forEach((seed) => {
    const runtime = createMockRuntime(seed)
    state[seed.platform] = state[seed.platform] || []
    state[seed.platform].push(runtime)
  })

  return state
}

const shouldRebuildMockTimelineState = (state: Record<string, MockProviderRuntime[]> | null) => {
  if (!state) return true

  return Object.values(state).some((runtimes) =>
    runtimes.some((runtime) => (runtime.timeline.items?.length ?? 0) < MOCK_HISTORY_LIMIT),
  )
}

const getMockTimelineState = (): Record<string, MockProviderRuntime[]> => {
  if (shouldRebuildMockTimelineState(mockTimelineState)) {
    mockTimelineState = createMockTimelineState()
  }
  return mockTimelineState as Record<string, MockProviderRuntime[]>
}

const findMockProvider = (platform: string, providerId: number): MockProviderRuntime => {
  const runtime = getMockTimelineState()[platform]?.find((item) => item.timeline.providerId === providerId)
  if (!runtime) {
    throw new Error(`Mock provider not found: ${platform}#${providerId}`)
  }
  return runtime
}

const runMockProviderCheck = (runtime: MockProviderRuntime): HealthCheckResult => {
  const nextStatus = runtime.checkSequence[runtime.checkCount % runtime.checkSequence.length] || HealthStatus.OPERATIONAL
  const result = createMockResult(runtime, nextStatus, new Date().toISOString())
  runtime.checkCount += 1
  runtime.timeline.items.unshift(result)
  recalculateMockTimeline(runtime.timeline)
  return { ...result }
}

const getMockLatestResults = (): Record<string, ProviderTimeline[]> => {
  const state = getMockTimelineState()
  return Object.fromEntries(
    Object.entries(state).map(([platform, runtimes]) => [
      platform,
      runtimes.map((runtime) => cloneProviderTimeline(runtime.timeline)),
    ]),
  )
}

const getMockLogBasedResults = (range: LogAvailabilityRange = '24h'): Record<string, ProviderTimeline[]> => {
  const state = getMockTimelineState()
  return Object.fromEntries(
    Object.entries(state).map(([platform, runtimes]) => [
      platform,
      runtimes.map((runtime) => cloneLogProviderTimeline(runtime.timeline, range)),
    ]),
  )
}

const getMockHistoryByProviderName = (platform: string, providerName: string, limit: number): MockHealthHistory => {
  const runtime = getMockTimelineState()[platform]?.find((item) => item.timeline.providerName === providerName)
  if (!runtime) {
    throw new Error(`Mock provider not found: ${platform}/${providerName}`)
  }

  const items = runtime.timeline.items.slice(0, Math.max(limit, 0)).map((item) => ({ ...item }))
  const latest = runtime.timeline.availabilityMonitorEnabled ? (items[0] ?? null) : null

  return {
    providerId: runtime.timeline.providerId,
    providerName: runtime.timeline.providerName,
    platform: runtime.timeline.platform,
    items,
    latest,
    uptime: runtime.timeline.uptime,
    avgLatencyMs: runtime.timeline.avgLatencyMs,
  }
}

/**
 * 获取所有 Provider 的最新状态（按平台分组）
 */
export async function getLatestResults(): Promise<Record<string, ProviderTimeline[]>> {
  if (shouldUseBrowserPreviewAvailabilityMock()) {
    return getMockLatestResults()
  }
  return Call.ByName(`${SERVICE_PATH}.GetLatestResults`)
}

/**
 * 获取基于请求日志聚合的 Provider 可用性状态（按平台分组）
 */
export async function getLogBasedResults(range: LogAvailabilityRange = '24h'): Promise<Record<string, ProviderTimeline[]>> {
  if (shouldUseBrowserPreviewAvailabilityMock()) {
    return getMockLogBasedResults(range)
  }
  return Call.ByName(`${SERVICE_PATH}.GetLogBasedResults`, range)
}

/**
 * 获取单个 Provider 的历史记录
 */
export async function getHistory(platform: string, providerName: string, limit: number = 20): Promise<any> {
  if (shouldUseBrowserPreviewAvailabilityMock()) {
    return getMockHistoryByProviderName(platform, providerName, limit)
  }
  return Call.ByName(`${SERVICE_PATH}.GetHistory`, platform, providerName, limit)
}

/**
 * 手动触发单个 Provider 检测
 */
export async function runSingleCheck(platform: string, providerId: number): Promise<HealthCheckResult> {
  if (shouldUseBrowserPreviewAvailabilityMock()) {
    const runtime = findMockProvider(platform, providerId)
    return runMockProviderCheck(runtime)
  }
  return Call.ByName(`${SERVICE_PATH}.RunSingleCheck`, platform, providerId)
}

/**
 * 手动触发全部检测
 */
export async function runAllChecks(): Promise<Record<string, HealthCheckResult[]>> {
  if (shouldUseBrowserPreviewAvailabilityMock()) {
    const state = getMockTimelineState()
    return Object.fromEntries(
      Object.entries(state).map(([platform, runtimes]) => [
        platform,
        runtimes
          .filter((runtime) => runtime.timeline.availabilityMonitorEnabled)
          .map((runtime) => runMockProviderCheck(runtime)),
      ]),
    )
  }
  return Call.ByName(`${SERVICE_PATH}.RunAllChecks`)
}

/**
 * 启动后台定时巡检
 */
export async function startBackgroundPolling(): Promise<void> {
  if (shouldUseBrowserPreviewAvailabilityMock()) {
    mockPollingRunning = true
    return
  }
  return Call.ByName(`${SERVICE_PATH}.StartBackgroundPolling`)
}

/**
 * 停止后台巡检
 */
export async function stopBackgroundPolling(): Promise<void> {
  if (shouldUseBrowserPreviewAvailabilityMock()) {
    mockPollingRunning = false
    return
  }
  return Call.ByName(`${SERVICE_PATH}.StopBackgroundPolling`)
}

/**
 * 检查后台巡检是否运行中
 */
export async function isPollingRunning(): Promise<boolean> {
  if (shouldUseBrowserPreviewAvailabilityMock()) {
    return mockPollingRunning
  }
  return Call.ByName(`${SERVICE_PATH}.IsPollingRunning`)
}

/**
 * 启用/禁用指定 Provider 的可用性监控
 */
export async function setAvailabilityMonitorEnabled(
  platform: string,
  providerId: number,
  enabled: boolean,
): Promise<void> {
  if (shouldUseBrowserPreviewAvailabilityMock()) {
    const runtime = findMockProvider(platform, providerId)
    runtime.timeline.availabilityMonitorEnabled = enabled
    return
  }
  return Call.ByName(`${SERVICE_PATH}.SetAvailabilityMonitorEnabled`, platform, providerId, enabled)
}

/**
 * 启用/禁用指定 Provider 的连通性自动拉黑
 */
export async function setConnectivityAutoBlacklist(
  platform: string,
  providerId: number,
  enabled: boolean,
): Promise<void> {
  if (shouldUseBrowserPreviewAvailabilityMock()) {
    const runtime = findMockProvider(platform, providerId)
    runtime.timeline.connectivityAutoBlacklist = enabled
    return
  }
  return Call.ByName(`${SERVICE_PATH}.SetConnectivityAutoBlacklist`, platform, providerId, enabled)
}

/**
 * 保存 Provider 的可用性高级配置
 */
export async function saveAvailabilityConfig(
  platform: string,
  providerId: number,
  config: AvailabilityConfig,
): Promise<void> {
  if (shouldUseBrowserPreviewAvailabilityMock()) {
    const runtime = findMockProvider(platform, providerId)
    runtime.timeline.availabilityConfig = {
      testModel: config.testModel || '',
      testEndpoint: config.testEndpoint || '',
      timeout: Number(config.timeout) || 15000,
    }
    return
  }
  return Call.ByName(`${SERVICE_PATH}.SaveAvailabilityConfig`, platform, providerId, config)
}

/**
 * 清理过期的历史记录
 */
export async function cleanupOldRecords(daysToKeep: number = 7): Promise<number> {
  if (shouldUseBrowserPreviewAvailabilityMock()) {
    const cutoff = Date.now() - Math.max(daysToKeep, 0) * 24 * 60 * 60 * 1000
    let removed = 0

    Object.values(getMockTimelineState()).forEach((runtimes) => {
      runtimes.forEach((runtime) => {
        const before = runtime.timeline.items.length
        runtime.timeline.items = runtime.timeline.items.filter((item) => {
          const checkedAt = new Date(item.checkedAt).getTime()
          return Number.isFinite(checkedAt) && checkedAt >= cutoff
        })
        removed += before - runtime.timeline.items.length
        recalculateMockTimeline(runtime.timeline)
      })
    })

    return removed
  }
  return Call.ByName(`${SERVICE_PATH}.CleanupOldRecords`, daysToKeep)
}

/**
 * 格式化状态为中文
 */
export function formatStatus(status: string): string {
  switch (status) {
    case HealthStatus.OPERATIONAL:
      return '正常'
    case HealthStatus.DEGRADED:
      return '延迟'
    case HealthStatus.FAILED:
      return '故障'
    case HealthStatus.VALIDATION_ERROR:
      return '验证失败'
    default:
      return status
  }
}

/**
 * 获取状态对应的颜色类
 */
export function getStatusColor(status: string): string {
  switch (status) {
    case HealthStatus.OPERATIONAL:
      return 'text-green-500'
    case HealthStatus.DEGRADED:
      return 'text-yellow-500'
    case HealthStatus.FAILED:
      return 'text-red-500'
    case HealthStatus.VALIDATION_ERROR:
      return 'text-red-500'
    default:
      return 'text-gray-500'
  }
}

/**
 * 获取状态对应的图标
 */
export function getStatusIcon(status: string): string {
  switch (status) {
    case HealthStatus.OPERATIONAL:
      return '\u{1F7E2}' // green circle
    case HealthStatus.DEGRADED:
      return '\u{1F7E1}' // yellow circle
    case HealthStatus.FAILED:
      return '\u{1F534}' // red circle
    case HealthStatus.VALIDATION_ERROR:
      return '\u{1F534}' // red circle
    default:
      return '\u{26AB}' // black circle
  }
}
