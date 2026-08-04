/**
 * @name: 供应商统计格式化测试
 * @Descripttion: 验证首页与托盘共用的供应商统计格式
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-04 10:37:27
 * @LastEditTime: 2026-08-04 10:37:27
 * @FilePath: frontend/src/utils/providerStatsFormat.test.ts
 */
import { describe, expect, it } from 'vitest'
import {
  formatProviderSuccessRate,
  formatProviderTokenCount,
  formatProviderTokensPerSecond,
} from './providerStatsFormat'

describe('providerStatsFormat', () => {
  it('formats compact token totals', () => {
    expect(formatProviderTokenCount(513_790, 'en-US')).toBe('513.79k')
    expect(formatProviderTokenCount(2_500_000, 'en-US')).toBe('2.50M')
  })

  it('formats success rate thresholds consistently', () => {
    expect(formatProviderSuccessRate(0.905)).toBe('90.5%')
    expect(formatProviderSuccessRate(1)).toBe('100%')
  })

  it('formats speed with a caller-selected unit', () => {
    expect(formatProviderTokensPerSecond(26.17, 't/s')).toBe('26.17 t/s')
    expect(formatProviderTokensPerSecond(126.17)).toBe('126.2 tokens/s')
    expect(formatProviderTokensPerSecond(0)).toBe('—')
  })
})
