import { describe, expect, it } from 'vitest'
import type { ProviderQuotaDisplayItem } from '../types'
import {
  formatProviderQuotaRelativeUpdatedAt,
  getProviderQuotaBalanceTone,
  getProviderQuotaRemainingValue,
  getProviderQuotaVisibleNote,
  isProviderQuotaBalanceItem,
  isProviderQuotaErrorItem,
  isProviderQuotaRefreshErrored,
} from './providerQuotaCardDisplay'

const createQuotaItem = (overrides: Partial<ProviderQuotaDisplayItem> = {}): ProviderQuotaDisplayItem => ({
  key: 'balance',
  label: 'Balance',
  used: 12,
  total: 40,
  progressRatio: 0.3,
  countdownLabel: '',
  nextReset: null,
  valueMode: 'currency',
  queriedAt: Date.UTC(2026, 3, 9, 10, 0, 0),
  ...overrides,
})

describe('providerQuotaCardDisplay', () => {
  it('treats remote currency quotas without reset time as balance cards', () => {
    expect(isProviderQuotaBalanceItem(createQuotaItem())).toBe(true)
    expect(isProviderQuotaErrorItem(createQuotaItem())).toBe(false)
    expect(isProviderQuotaBalanceItem(createQuotaItem({
      nextReset: new Date('2026-04-10T00:00:00.000Z'),
    }))).toBe(false)
    expect(isProviderQuotaBalanceItem(createQuotaItem({
      valueMode: 'count',
    }))).toBe(false)
    expect(isProviderQuotaBalanceItem(createQuotaItem({
      queriedAt: undefined,
    }))).toBe(false)
    expect(isProviderQuotaBalanceItem(createQuotaItem({
      total: 0,
      used: 0,
      invalidMessage: 'Query failed',
    }))).toBe(false)
    expect(isProviderQuotaErrorItem(createQuotaItem({
      total: 0,
      used: 0,
      invalidMessage: 'Query failed',
    }))).toBe(true)
  })

  it('calculates remaining value and tone from used/total data', () => {
    expect(getProviderQuotaRemainingValue(createQuotaItem())).toBe(28)
    expect(getProviderQuotaBalanceTone(createQuotaItem())).toBe('healthy')
    expect(getProviderQuotaBalanceTone(createQuotaItem({
      used: 37,
      total: 40,
    }))).toBe('warning')
    expect(getProviderQuotaBalanceTone(createQuotaItem({
      used: 40,
      total: 40,
    }))).toBe('danger')
    expect(getProviderQuotaBalanceTone(createQuotaItem({
      used: 0,
      total: 0,
      unlimited: true,
    }))).toBe('healthy')
    expect(getProviderQuotaBalanceTone(createQuotaItem({
      invalidMessage: 'Authentication failed',
      used: 0,
      total: 0,
    }))).toBe('invalid')
  })

  it('formats queriedAt into human-friendly relative labels', () => {
    const t = (key: string, options?: { count?: number }) => {
      switch (key) {
        case 'components.main.providers.quotaUpdatedJustNow':
          return '刚刚更新'
        case 'components.main.providers.quotaUpdatedMinutesAgo':
          return `${options?.count} 分钟前更新`
        case 'components.main.providers.quotaUpdatedHoursAgo':
          return `${options?.count} 小时前更新`
        case 'components.main.providers.quotaUpdatedDaysAgo':
          return `${options?.count} 天前更新`
        case 'components.main.providers.quotaNeverUpdated':
          return '从未更新'
        default:
          return key
      }
    }

    const base = Date.UTC(2026, 3, 9, 10, 0, 0)

    expect(formatProviderQuotaRelativeUpdatedAt(base, base + 20_000, t)).toBe('刚刚更新')
    expect(formatProviderQuotaRelativeUpdatedAt(base, base + 5 * 60_000, t)).toBe('5 分钟前更新')
    expect(formatProviderQuotaRelativeUpdatedAt(base, base + 3 * 3_600_000, t)).toBe('3 小时前更新')
    expect(formatProviderQuotaRelativeUpdatedAt(base, base + 2 * 86_400_000, t)).toBe('2 天前更新')
    expect(formatProviderQuotaRelativeUpdatedAt(undefined, base, t)).toBe('从未更新')
  })

  it('combines visible note text and detects refresh failure markers', () => {
    expect(getProviderQuotaVisibleNote(createQuotaItem({
      refreshErrorMessage: '刷新失败，仍显示旧数据',
      invalidMessage: 'Authentication failed',
      extra: '请检查 API Key',
    }))).toBe('刷新失败，仍显示旧数据 · Authentication failed · 请检查 API Key')
    expect(isProviderQuotaRefreshErrored(createQuotaItem({
      refreshErrorMessage: '刷新失败，仍显示旧数据',
    }))).toBe(true)
    expect(isProviderQuotaRefreshErrored(createQuotaItem())).toBe(false)
  })
})
