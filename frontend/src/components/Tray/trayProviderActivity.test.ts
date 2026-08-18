/**
 * @name: 托盘供应商调用状态测试
 * @Descripttion: 验证活跃供应商归并、最近调用回退与状态变化检测
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-18 15:08:00
 * @LastEditTime: 2026-08-18 15:08:00
 * @FilePath: frontend/src/components/Tray/trayProviderActivity.test.ts
 */
import { describe, expect, it } from 'vitest'
import {
  buildTrayProviderActivityRefreshKey,
  hasTrayProviderActivityChanged,
  loadTrayProviderActivitySnapshot,
  normalizeTrayProviderActivityStatus,
  normalizeTrayRecentProvider,
  resolveTrayProviderActivity,
  resolveTrayProviderActivityWithFallback,
} from './trayProviderActivity'

describe('trayProviderActivity', () => {
  it('changes the refresh key when platforms or lifecycle generation changes', () => {
    const current = buildTrayProviderActivityRefreshKey(['claude', 'codex'], 1)

    expect(buildTrayProviderActivityRefreshKey(['claude', 'codex'], 1)).toBe(current)
    expect(buildTrayProviderActivityRefreshKey(['codex', 'claude'], 1)).not.toBe(current)
    expect(buildTrayProviderActivityRefreshKey(['claude', 'codex', 'gemini'], 1)).not.toBe(current)
    expect(buildTrayProviderActivityRefreshKey(['claude', 'codex'], 2)).not.toBe(current)
  })

  it('keeps every active provider in the supplied route order', () => {
    expect(resolveTrayProviderActivity([
      { providerId: 'provider-2', providerName: 'Second', activeRequests: 2 },
      { providerId: 'provider-1', providerName: 'First', activeRequests: 1 },
    ], null)).toEqual([
      { providerId: 'provider-2', providerName: 'Second', activeRequests: 2, isActive: true },
      { providerId: 'provider-1', providerName: 'First', activeRequests: 1, isActive: true },
    ])
  })

  it('falls back to the most recent provider when no request is active', () => {
    expect(resolveTrayProviderActivity([], {
      provider_id: 'provider-1',
      provider_name: 'First',
    })).toEqual([
      { providerId: 'provider-1', providerName: 'First', activeRequests: 0, isActive: false },
    ])
  })

  it('drops statuses without a provider or active request', () => {
    expect(resolveTrayProviderActivity([
      { providerId: '', providerName: 'Missing ID', activeRequests: 3 },
      { providerId: 'idle', providerName: 'Idle', activeRequests: 0 },
    ], null)).toEqual([])
  })

  it('detects active provider set transitions without reacting to count changes', () => {
    const previous = resolveTrayProviderActivity([
      { providerId: 'provider-1', providerName: 'First', activeRequests: 1 },
    ], null)
    const countChanged = resolveTrayProviderActivity([
      { providerId: 'provider-1', providerName: 'First', activeRequests: 2 },
    ], null)
    const providerChanged = resolveTrayProviderActivity([
      { providerId: 'provider-2', providerName: 'Second', activeRequests: 1 },
    ], null)

    expect(hasTrayProviderActivityChanged(previous, countChanged)).toBe(false)
    expect(hasTrayProviderActivityChanged(previous, providerChanged)).toBe(true)
  })

  it('normalizes Wails field casing for status and recent provider snapshots', () => {
    expect(normalizeTrayProviderActivityStatus({
      ProviderID: ' provider-1 ',
      ProviderName: ' First ',
      ActiveRequests: 2,
    })).toEqual({
      providerId: 'provider-1',
      providerName: 'First',
      activeRequests: 2,
    })
    expect(normalizeTrayRecentProvider({
      ProviderID: 'provider-1',
      ProviderName: 'First',
      UpdatedAt: 123,
    })).toEqual({
      provider_id: 'provider-1',
      provider_name: 'First',
      updated_at: 123,
    })
  })

  it('keeps live statuses available when the recent provider query fails', async () => {
    const recentError = new Error('recent unavailable')
    const snapshot = await loadTrayProviderActivitySnapshot(['claude'], {
      loadStatuses: async () => ({
        claude: [{ providerId: 'provider-2', providerName: 'Second', activeRequests: 1 }],
      }),
      loadRecent: async () => Promise.reject(recentError),
    })

    expect(snapshot.statusesByPlatform?.claude).toEqual([
      { providerId: 'provider-2', providerName: 'Second', activeRequests: 1 },
    ])
    expect(snapshot.hasRecentSnapshot).toBe(false)
    expect(snapshot.recentError).toBe(recentError)
  })

  it('turns the previous provider idle when the recent provider query fails', () => {
    expect(resolveTrayProviderActivityWithFallback([], null, [{
      providerId: 'provider-1',
      providerName: 'First',
      activeRequests: 2,
      isActive: true,
    }], false)).toEqual([{
      providerId: 'provider-1',
      providerName: 'First',
      activeRequests: 0,
      isActive: false,
    }])
  })
})
