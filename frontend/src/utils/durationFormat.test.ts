/**
 * @name: 时长格式化工具测试
 * @Descripttion: 验证自适应时长单位的格式化规则
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-13 20:56:42
 * @LastEditTime: 2026-07-13 20:56:42
 * @FilePath: frontend/src/utils/durationFormat.test.ts
 */

import { describe, expect, it } from 'vitest'
import { formatAdaptiveDurationSeconds } from './durationFormat'

describe('formatAdaptiveDurationSeconds', () => {
  it('formats milliseconds as rounded integers', () => {
    expect(formatAdaptiveDurationSeconds(0.0001)).toBe('1ms')
    expect(formatAdaptiveDurationSeconds(0.00825)).toBe('8ms')
    expect(formatAdaptiveDurationSeconds(0.824)).toBe('824ms')
  })

  it('formats seconds with adaptive precision', () => {
    expect(formatAdaptiveDurationSeconds(1.234)).toBe('1.23s')
    expect(formatAdaptiveDurationSeconds(12.34)).toBe('12.3s')
    expect(formatAdaptiveDurationSeconds(59)).toBe('59s')
  })

  it('rounds decimal half values without binary floating-point drift', () => {
    expect(formatAdaptiveDurationSeconds(1.005)).toBe('1.01s')
    expect(formatAdaptiveDurationSeconds(9.995)).toBe('10s')
    expect(formatAdaptiveDurationSeconds(12.35)).toBe('12.4s')
  })

  it('formats minute and hour combinations with padded components', () => {
    expect(formatAdaptiveDurationSeconds(65)).toBe('1m 05s')
    expect(formatAdaptiveDurationSeconds(3665)).toBe('1h 01m 05s')
  })

  it('promotes rounded values across unit boundaries', () => {
    expect(formatAdaptiveDurationSeconds(0.9996)).toBe('1s')
    expect(formatAdaptiveDurationSeconds(59.96)).toBe('1m 00s')
    expect(formatAdaptiveDurationSeconds(3599.6)).toBe('1h 00m 00s')
  })

  it('returns an empty marker for invalid values', () => {
    expect(formatAdaptiveDurationSeconds(0)).toBe('—')
    expect(formatAdaptiveDurationSeconds(-1)).toBe('—')
    expect(formatAdaptiveDurationSeconds(Number.NaN)).toBe('—')
    expect(formatAdaptiveDurationSeconds(Number.POSITIVE_INFINITY)).toBe('—')
  })
})
