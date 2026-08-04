/**
 * @name: 托盘供应商统计测试
 * @Descripttion: 验证托盘供应商统计的匹配、口径和格式化行为
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-04 10:08:50
 * @LastEditTime: 2026-08-04 10:08:50
 * @FilePath: frontend/src/components/Tray/trayProviderStats.test.ts
 */
import { describe, expect, it } from 'vitest'
import type { AutomationCard } from '../../data/cards'
import type { ProviderDailyStat } from '../../services/logs'
import {
  buildTrayProviderStatsDisplay,
  getTrayProviderStatsKey,
  prepareTrayProviderStatsRefresh,
  resolveTrayProviderDailyStat,
} from './trayProviderStats'

function createProvider(overrides: Partial<AutomationCard> = {}): AutomationCard {
  return {
    id: 102,
    providerRef: '102',
    name: 'Kimi-3',
    apiUrl: 'https://example.com',
    apiKey: 'key',
    officialSite: 'https://example.com',
    icon: 'kimi',
    tint: 'rgba(10, 132, 255, 0.14)',
    accent: '#0a84ff',
    enabled: true,
    ...overrides,
  }
}

function createStat(overrides: Partial<ProviderDailyStat> = {}): ProviderDailyStat {
  return {
    provider_id: '102',
    provider: 'Kimi-3',
    total_requests: 21,
    successful_requests: 19,
    failed_requests: 2,
    excluded_requests: 0,
    success_rate: 0.905,
    input_tokens: 400_000,
    output_tokens: 100_000,
    reasoning_tokens: 50_000,
    cache_create_tokens: 25_000,
    cache_read_tokens: 13_790,
    cost_total: 0.6,
    avg_first_token_sec: 3.92,
    avg_tokens_per_sec: 26.17,
    ...overrides,
  }
}

describe('trayProviderStats', () => {
  it('matches by provider identity before considering the provider name', () => {
    const sameName = createStat({ provider_id: '999' })
    const sameIdentity = createStat({ provider: 'Renamed provider' })

    expect(resolveTrayProviderDailyStat(createProvider(), [sameName, sameIdentity])).toBe(sameIdentity)
  })

  it('falls back to a legacy name only when the stat has no identity', () => {
    const identifiedSameName = createStat({ provider_id: '999' })
    const legacySameName = createStat({ provider_id: undefined, provider: '  kimi-3  ' })

    expect(resolveTrayProviderDailyStat(createProvider(), [identifiedSameName, legacySameName])).toBe(legacySameName)
  })

  it('formats the agreed daily and recent-performance metrics', () => {
    const display = buildTrayProviderStatsDisplay(createProvider(), [createStat()], 'en-US')

    expect(display).toMatchObject({
      successRate: '90.5%',
      successRateTone: 'warning',
      requests: '21',
      tokens: '513.79k',
      firstToken: '3.92s',
      speed: '26.17 t/s',
    })
    expect(display.cost).toContain('0.60')
  })

  it.each([
    [0.95, 'good'],
    [0.8, 'warning'],
    [0.79, 'bad'],
  ] as const)('maps success rate %s to %s', (successRate, tone) => {
    const display = buildTrayProviderStatsDisplay(
      createProvider(),
      [createStat({ success_rate: successRate })],
      'en-US',
    )

    expect(display.successRateTone).toBe(tone)
  })

  it('keeps stable placeholders when no provider stat exists', () => {
    expect(buildTrayProviderStatsDisplay(createProvider(), [], 'en-US')).toMatchObject({
      successRate: '—',
      successRateTone: 'neutral',
      requests: '0',
      tokens: '0',
      firstToken: '—',
      speed: '—',
    })
  })

  it('does not calculate success rate when every request is excluded', () => {
    const display = buildTrayProviderStatsDisplay(createProvider(), [createStat({
      total_requests: 3,
      successful_requests: 0,
      failed_requests: 0,
      excluded_requests: 3,
      success_rate: 0,
    })])

    expect(display.successRate).toBe('—')
    expect(display.successRateTone).toBe('neutral')
    expect(display.requests).toBe('3')
  })

  it('keeps the last valid display while refreshing the same provider', () => {
    const provider = createProvider()
    const previousDisplay = buildTrayProviderStatsDisplay(provider, [createStat()], 'en-US')
    const prepared = prepareTrayProviderStatsRefresh(
      provider,
      getTrayProviderStatsKey(provider),
      previousDisplay,
      'en-US',
    )

    expect(prepared.display).toBe(previousDisplay)
  })

  it('uses placeholders immediately when switching providers', () => {
    const previousProvider = createProvider()
    const nextProvider = createProvider({ id: 103, providerRef: '103', name: 'Kimi-4' })
    const previousDisplay = buildTrayProviderStatsDisplay(previousProvider, [createStat()], 'en-US')
    const prepared = prepareTrayProviderStatsRefresh(
      nextProvider,
      getTrayProviderStatsKey(previousProvider),
      previousDisplay,
      'en-US',
    )

    expect(prepared.providerKey).toBe('id:103')
    expect(prepared.display).toMatchObject({
      successRate: '—',
      requests: '0',
      tokens: '0',
      firstToken: '—',
      speed: '—',
    })
  })
})
