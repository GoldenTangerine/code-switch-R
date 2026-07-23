/**
 * @name: 设置服务测试
 * @Descripttion: 验证黑名单双阈值配置调用正确传递到后端。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-22 22:04:00
 * @LastEditTime: 2026-07-22 22:04:00
 * @FilePath: frontend/src/services/settings.test.ts
 */

import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@wailsio/runtime', () => ({
  Call: {
    ByName: vi.fn(),
  },
}))

import { Call } from '@wailsio/runtime'
import {
  getHealthBlacklistThreshold,
  updateBlacklistSettingsWithHealthThreshold,
  updateHealthBlacklistThreshold,
} from './settings'

describe('settings service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('reads and updates the independent health blacklist threshold', async () => {
    vi.mocked(Call.ByName).mockResolvedValueOnce(3).mockResolvedValueOnce(undefined)

    await expect(getHealthBlacklistThreshold()).resolves.toBe(3)
    await updateHealthBlacklistThreshold(7)

    expect(Call.ByName).toHaveBeenNthCalledWith(
      1,
      'codeswitch/services.SettingsService.GetHealthBlacklistThreshold',
    )
    expect(Call.ByName).toHaveBeenNthCalledWith(
      2,
      'codeswitch/services.SettingsService.UpdateHealthBlacklistThreshold',
      7,
    )
  })

  it('saves both blacklist thresholds through one backend call', async () => {
    vi.mocked(Call.ByName).mockResolvedValueOnce(undefined)

    await updateBlacklistSettingsWithHealthThreshold(9, 1800, 5)

    expect(Call.ByName).toHaveBeenCalledWith(
      'codeswitch/services.SettingsService.UpdateBlacklistSettingsWithHealthThreshold',
      9,
      1800,
      5,
    )
  })
})
