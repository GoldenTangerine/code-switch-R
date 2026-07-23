/**
 * @name: 供应商黑名单计数映射测试
 * @Descripttion: 验证黑名单状态与默认阈值的计数展示映射
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-23 15:18:00
 * @LastEditTime: 2026-07-23 15:18:00
 * @FilePath: frontend/src/components/Main/composables/useBlacklistState.test.ts
 */

import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { BlacklistStatus } from '../../../services/blacklist'

const { callByNameMock } = vi.hoisted(() => ({
  callByNameMock: vi.fn(),
}))

vi.mock('@wailsio/runtime', () => ({
  Call: { ByName: callByNameMock },
  Events: { On: vi.fn() },
}))

import { buildProviderSuccessRateTooltip } from '../utils/providerBlacklistDisplay'
import { resolveProviderBlacklistCounters, useBlacklistState } from './useBlacklistState'

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
  beforeEach(() => {
    callByNameMock.mockReset()
  })

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
      switchToPlatform: vi.fn(),
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
