/**
 * @name: 供应商黑名单计数映射测试
 * @Descripttion: 验证黑名单状态与默认阈值的计数展示映射
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-23 15:18:00
 * @LastEditTime: 2026-07-23 15:18:00
 * @FilePath: frontend/src/components/Main/composables/useBlacklistState.test.ts
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { BlacklistStatus } from '../../../services/blacklist'

const { callByNameMock, eventCallbacks } = vi.hoisted(() => ({
  callByNameMock: vi.fn(),
  eventCallbacks: new Map<string, (event: { data: Record<string, unknown> }) => void>(),
}))

vi.mock('@wailsio/runtime', () => ({
  Call: { ByName: callByNameMock },
  Events: {
    On: (name: string, callback: (event: { data: Record<string, unknown> }) => void) => {
      eventCallbacks.set(name, callback)
      return vi.fn()
    },
  },
}))

import { buildProviderSuccessRateTooltip } from '../utils/providerBlacklistDisplay'
import { resolveProviderBlacklistCounters, useBlacklistState } from './useBlacklistState'

beforeEach(() => {
  callByNameMock.mockReset()
  eventCallbacks.clear()
})

const buildStatus = (overrides: Partial<BlacklistStatus> = {}): BlacklistStatus => ({
  platform: 'claude',
  providerName: 'kimi',
  failureCount: 2,
  failureThreshold: 5,
  healthFailureCount: 1,
  healthFailureThreshold: 4,
  isBlacklisted: false,
  remainingSeconds: 0,
  blacklistLevel: 0,
  forgivenessRemaining: 0,
  ...overrides,
})

describe('resolveProviderBlacklistCounters', () => {
  it('uses stored counts and thresholds when a provider has a blacklist record', () => {
    expect(resolveProviderBlacklistCounters(buildStatus(), {
      failureThreshold: 3,
      healthFailureThreshold: 3,
    })).toEqual({
      failureCount: 2,
      failureThreshold: 5,
      healthFailureCount: 1,
      healthFailureThreshold: 4,
    })
  })

  it('fills zero counts with current global thresholds when no record exists', () => {
    expect(resolveProviderBlacklistCounters(null, {
      failureThreshold: 6,
      healthFailureThreshold: 7,
    })).toEqual({
      failureCount: 0,
      failureThreshold: 6,
      healthFailureCount: 0,
      healthFailureThreshold: 7,
    })
  })

  it('keeps thresholds unavailable instead of showing misleading defaults', () => {
    expect(resolveProviderBlacklistCounters(null, {
      failureThreshold: null,
      healthFailureThreshold: null,
    })).toEqual({
      failureCount: 0,
      failureThreshold: null,
      healthFailureCount: 0,
      healthFailureThreshold: null,
    })
  })

  it('falls back to the last valid thresholds when status data is invalid', () => {
    expect(resolveProviderBlacklistCounters(buildStatus({
      failureThreshold: Number.NaN,
      healthFailureThreshold: 0,
    }), {
      failureThreshold: 6,
      healthFailureThreshold: 7,
    })).toEqual({
      failureCount: 2,
      failureThreshold: 6,
      healthFailureCount: 1,
      healthFailureThreshold: 7,
    })
  })
})

describe('buildProviderSuccessRateTooltip', () => {
  it('formats unavailable thresholds as dashes', () => {
    const tooltip = buildProviderSuccessRateTooltip('暂无数据', {
      failureCount: 0,
      failureThreshold: null,
      healthFailureCount: 1,
      healthFailureThreshold: 5,
    }, (key: string, values?: Record<string, unknown>) => {
      if (key.endsWith('requestCount')) return `请求 ${values?.current}/${values?.threshold}`
      if (key.endsWith('healthCount')) return `巡检 ${values?.current}/${values?.threshold}`
      return key
    })

    expect(tooltip).toBe('暂无数据\n请求 0/—\n巡检 1/5')
  })
})

describe('useBlacklistState threshold loading', () => {
  it('deduplicates concurrent threshold requests and reuses the short-lived cache', async () => {
    callByNameMock.mockImplementation((name: string) => {
      if (name.endsWith('.GetBlacklistStatus')) return Promise.resolve([])
      if (name.endsWith('.GetBlacklistSettingsStruct')) {
        return Promise.resolve({ failureThreshold: 6, durationSeconds: 1800 })
      }
      if (name.endsWith('.GetHealthBlacklistThreshold')) return Promise.resolve(7)
      return Promise.resolve(null)
    })

    const state = useBlacklistState({
      t: (key: string) => key,
      getActiveTab: () => 'claude',
      getSelectedToolId: () => null,
    })

    await Promise.all([
      state.loadBlacklistStatus('claude'),
      state.loadBlacklistStatus('codex'),
      state.loadBlacklistStatus('gemini'),
    ])
    await state.loadBlacklistStatus('claude')

    expect(callByNameMock.mock.calls.filter(([name]) => name.endsWith('.GetBlacklistSettingsStruct'))).toHaveLength(1)
    expect(callByNameMock.mock.calls.filter(([name]) => name.endsWith('.GetHealthBlacklistThreshold'))).toHaveLength(1)
  })
})

describe('useBlacklistState provider event visibility', () => {
  const providerEvents = [
    ['provider:switched', {
      platform: 'claude',
      toProvider: 'kimi',
      toProviderId: '101',
      timestamp: 1,
    }],
    ['provider:blacklisted', {
      platform: 'claude',
      providerName: 'kimi',
      providerId: '101',
      timestamp: 1,
    }],
  ] as const

  beforeEach(() => {
    callByNameMock.mockResolvedValue([])
    vi.stubGlobal('window', {
      setInterval: vi.fn(() => 1),
      clearInterval: vi.fn(),
      setTimeout: vi.fn(() => 2),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it.each(providerEvents)('keeps %s on a background tab without switching or highlighting', (eventName, data) => {
    const state = useBlacklistState({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      getSelectedToolId: () => null,
    })
    state.startStatusSync()

    try {
      eventCallbacks.get(eventName)?.({ data })

      expect(state.lastUsedProviders.claude?.provider_name).toBe('kimi')
      expect(state.highlightedProviderName.value).toBeNull()
    } finally {
      state.stopStatusSync()
    }
  })

  it.each(providerEvents)('keeps %s highlighting on the active tab', (eventName, data) => {
    const state = useBlacklistState({
      t: (key: string) => key,
      getActiveTab: () => 'claude',
      getSelectedToolId: () => null,
    })
    state.startStatusSync()

    try {
      eventCallbacks.get(eventName)?.({ data })

      expect(state.highlightedProviderName.value).toBe('kimi')
    } finally {
      state.stopStatusSync()
    }
  })
})
