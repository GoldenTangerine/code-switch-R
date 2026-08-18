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
  normalizeTrayDefaultProvider,
  normalizeTrayProviderActivityStatus,
  resolveTrayProviderActivity,
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
      { providerId: 'provider-2', providerName: 'Second', activeRequests: 2, status: 'active' },
      { providerId: 'provider-1', providerName: 'First', activeRequests: 1, status: 'active' },
    ])
  })

  it('falls back to the default provider when no request is active', () => {
    expect(resolveTrayProviderActivity([], {
      providerId: 'provider-1',
      providerName: 'First',
    })).toEqual([
      { providerId: 'provider-1', providerName: 'First', activeRequests: 0, status: 'default' },
    ])
  })

  it('prefers every active provider over the default provider', () => {
    expect(resolveTrayProviderActivity([
      { providerId: 'provider-2', providerName: 'Second', activeRequests: 1 },
      { providerId: 'provider-3', providerName: 'Third', activeRequests: 2 },
    ], {
      providerId: 'provider-1',
      providerName: 'First',
    })).toEqual([
      { providerId: 'provider-2', providerName: 'Second', activeRequests: 1, status: 'active' },
      { providerId: 'provider-3', providerName: 'Third', activeRequests: 2, status: 'active' },
    ])
  })

  it('drops statuses without a provider or active request', () => {
    expect(resolveTrayProviderActivity([
      { providerId: '', providerName: 'Missing ID', activeRequests: 3 },
      { providerId: 'idle', providerName: 'Idle', activeRequests: 0 },
    ], null)).toEqual([])
  })

  it('returns no activity when neither an active nor default provider is available', () => {
    expect(resolveTrayProviderActivity([], null)).toEqual([])
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

  it('normalizes Wails field casing for status and default provider snapshots', () => {
    expect(normalizeTrayProviderActivityStatus({
      ProviderID: ' provider-1 ',
      ProviderName: ' First ',
      ActiveRequests: 2,
    })).toEqual({
      providerId: 'provider-1',
      providerName: 'First',
      activeRequests: 2,
    })
    expect(normalizeTrayDefaultProvider({
      ProviderID: 'provider-1',
      ProviderName: 'First',
    })).toEqual({
      providerId: 'provider-1',
      providerName: 'First',
    })
  })

  it('normalizes a batched runtime snapshot', async () => {
    const snapshot = await loadTrayProviderActivitySnapshot(['claude'], {
      loadStates: async () => ({
        claude: {
          Statuses: [{ ProviderID: 'provider-2', ProviderName: 'Second', ActiveRequests: 1 }],
          DefaultProvider: { ProviderID: 'provider-1', ProviderName: 'First' },
        },
      }),
    })

    expect(snapshot.statesByPlatform?.claude).toEqual({
      statuses: [{ providerId: 'provider-2', providerName: 'Second', activeRequests: 1 }],
      defaultProvider: { providerId: 'provider-1', providerName: 'First' },
      error: false,
    })
    expect(snapshot.error).toBeUndefined()
  })

  it('returns an error snapshot when the batched runtime query fails', async () => {
    const error = new Error('runtime unavailable')
    const snapshot = await loadTrayProviderActivitySnapshot(['claude'], {
      loadStates: async () => Promise.reject(error),
    })

    expect(snapshot.statesByPlatform).toBeNull()
    expect(snapshot.error).toBe(error)
  })

  it('normalizes a per-platform runtime error without treating it as no providers', async () => {
    const snapshot = await loadTrayProviderActivitySnapshot(['claude'], {
      loadStates: async () => ({
        claude: { Error: true },
      }),
    })

    expect(snapshot.statesByPlatform?.claude).toEqual({
      statuses: [],
      defaultProvider: null,
      error: true,
    })
  })

  it('treats a missing platform state as a query error', async () => {
    const snapshot = await loadTrayProviderActivitySnapshot(['claude', 'codex'], {
      loadStates: async () => ({
        claude: { Statuses: [] },
      }),
    })

    expect(snapshot.statesByPlatform?.claude?.error).toBe(false)
    expect(snapshot.statesByPlatform?.codex).toEqual({
      statuses: [],
      defaultProvider: null,
      error: true,
    })
  })
})
