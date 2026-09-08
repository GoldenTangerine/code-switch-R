/**
 * @name: 托盘共享快照适配测试
 * @Descripttion: 验证后端快照在托盘中保持额度类型、时间与统计口径。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-08 17:05:00
 * @LastEditTime: 2026-09-08 17:05:00
 * @FilePath: frontend/src/components/Tray/traySharedSnapshot.test.ts
 */
import { describe, expect, it, vi } from 'vitest'
vi.mock('@wailsio/runtime', () => ({ Call: { ByName: vi.fn() } }))
import { sharedTrayQuotaItem, type SharedTrayQuota } from './traySharedSnapshot'
import { resolveTrayProviderQuotaDisplay } from './trayProviderFallback'

const now = new Date('2026-09-08T12:00:00Z')
const t = (key: string) => key
const base: SharedTrayQuota = { key: 'daily', used: 3, total: 10, active: true, displayKind: 'progress', valueMode: 'currency' }

describe('shared tray snapshot', () => {
  it('keeps configured total budgets as progress rather than remote balances', () => {
    const display = resolveTrayProviderQuotaDisplay(sharedTrayQuotaItem(base, t, now), t)
    expect(display.displayKind).toBe('progress')
    expect(display.used).toBe(3)
    expect(display.total).toBe(10)
  })
  it('preserves balance and unlimited semantics', () => {
    const display = resolveTrayProviderQuotaDisplay(sharedTrayQuotaItem({ ...base, displayKind: 'balance', unlimited: true }, t, now), t)
    expect(display.displayKind).toBe('balance')
    expect(display.unlimited).toBe(true)
  })
  it('localizes failures without rendering raw diagnostics', () => {
    const display = resolveTrayProviderQuotaDisplay(sharedTrayQuotaItem({ ...base, displayKind: 'error', invalidMessage: 'Quota unavailable' }, t, now), t)
    expect(display.displayKind).toBe('error')
    expect(display.invalidMessage).toBe('tray.queryFailed')
  })
  it('retains reset instants and marks inactive windows', () => {
    const item = sharedTrayQuotaItem({ ...base, nextReset: '2026-09-08T23:00:00+08:00' }, t, now)
    expect(item.nextReset?.toISOString()).toBe('2026-09-08T15:00:00.000Z')
    expect(sharedTrayQuotaItem({ ...base, active: false }, t, now).countdownLabel).toBe('components.main.providers.quotaInactive')
  })
})
