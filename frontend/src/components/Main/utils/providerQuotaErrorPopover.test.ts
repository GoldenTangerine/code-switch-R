/**
 * @name: 额度错误悬浮窗布局测试
 * @Descripttion: 验证额度错误悬浮窗在不同视口边界内的位置和尺寸
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 17:20:00
 * @LastEditTime: 2026-08-17 17:20:00
 * @FilePath: frontend/src/components/Main/utils/providerQuotaErrorPopover.test.ts
 */

import { describe, expect, it } from 'vitest'
import { calculateProviderQuotaErrorPopoverLayout } from './providerQuotaErrorPopover'

describe('calculateProviderQuotaErrorPopoverLayout', () => {
  it('keeps the popover inside the right viewport edge', () => {
    const layout = calculateProviderQuotaErrorPopoverLayout(
      { top: 120, bottom: 144, left: 968, width: 24, height: 24 },
      { width: 320, height: 120 },
      { width: 1_000, height: 700 },
    )

    expect(layout).toMatchObject({
      left: 568,
      width: 420,
      placement: 'below',
    })
    expect(layout.left + layout.width).toBeLessThanOrEqual(988)
  })

  it('places the popover above when the bottom space is insufficient', () => {
    const layout = calculateProviderQuotaErrorPopoverLayout(
      { top: 700, bottom: 724, left: 300, width: 24, height: 24 },
      { width: 420, height: 160 },
      { width: 800, height: 760 },
    )

    expect(layout).toMatchObject({
      top: 532,
      placement: 'above',
    })
  })

  it('limits the popover dimensions within a small viewport', () => {
    const layout = calculateProviderQuotaErrorPopoverLayout(
      { top: 70, bottom: 92, left: 210, width: 20, height: 22 },
      { width: 420, height: 500 },
      { width: 240, height: 160 },
    )

    expect(layout).toMatchObject({
      top: 12,
      left: 12,
      width: 216,
      maxHeight: 136,
      placement: 'above',
    })
    expect(layout.left + layout.width).toBeLessThanOrEqual(228)
    expect(layout.top + layout.maxHeight).toBeLessThanOrEqual(148)
  })

  it('supports custom dimensions for the blacklist details popover', () => {
    const layout = calculateProviderQuotaErrorPopoverLayout(
      { top: 100, bottom: 124, left: 260, width: 120, height: 24 },
      { width: 340, height: 500 },
      { width: 800, height: 700 },
      { maxWidth: 340, maxHeight: 360 },
    )

    expect(layout).toMatchObject({
      top: 132,
      width: 340,
      maxHeight: 360,
      placement: 'below',
    })
  })
})
