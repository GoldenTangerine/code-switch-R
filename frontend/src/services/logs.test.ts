/**
 * @name: 日志服务测试
 * @Descripttion: 验证来源统计与会话重导参数正确传递到后端。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-17 17:35:00
 * @LastEditTime: 2026-07-17 17:35:00
 * @FilePath: frontend/src/services/logs.test.ts
 */

import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@wailsio/runtime', () => ({
  Call: {
    ByName: vi.fn(),
  },
}))

import { Call } from '@wailsio/runtime'
import {
  clearRequestLogs,
  fetchLogStatsV2,
  fetchProviderPerformanceTrend15m,
  fetchRequestLogDailyHeatmapStatsByYear,
  fetchRequestLogHeatmapYears,
  fetchRequestLogsPage,
} from './logs'

describe('logs service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('passes the selected source mode to details and aggregations', async () => {
    vi.mocked(Call.ByName).mockResolvedValue({})

    await fetchRequestLogsPage({ sourceMode: 'session' })
    await fetchLogStatsV2({ sourceMode: 'all' })

    expect(Call.ByName).toHaveBeenNthCalledWith(
      1,
      'codeswitch/services.LogService.ListRequestLogsPageV3',
      '',
      '',
      '',
      'session',
      100,
      0,
      '',
      '',
    )
    expect(Call.ByName).toHaveBeenNthCalledWith(
      2,
      'codeswitch/services.LogService.StatsRangeV3',
      '',
      '',
      '',
      'all',
      '',
      '',
    )
  })

  it('passes the history reimport choice when clearing request logs', async () => {
    vi.mocked(Call.ByName).mockResolvedValue(undefined)

    await clearRequestLogs(true)

    expect(Call.ByName).toHaveBeenCalledWith(
      'codeswitch/services.LogService.ClearRequestLogsV2',
      true,
    )
  })

  it('passes the provider and local range to the 15-minute performance trend', async () => {
    vi.mocked(Call.ByName).mockResolvedValue([])

    await fetchProviderPerformanceTrend15m({
      platform: 'codex',
      provider: 'provider-id',
      startAt: '2026-08-07 00:00:00',
      endAt: '2026-08-07 16:05:50',
    })

    expect(Call.ByName).toHaveBeenCalledWith(
      'codeswitch/services.LogService.ProviderPerformanceTrend15m',
      'codex',
      'provider-id',
      '2026-08-07 00:00:00',
      '2026-08-07 16:05:50',
    )
  })

  it('passes the selected source mode to storage heatmap queries', async () => {
    vi.mocked(Call.ByName).mockResolvedValue([])

    await fetchRequestLogDailyHeatmapStatsByYear(2026, 'session')
    await fetchRequestLogHeatmapYears('all')

    expect(Call.ByName).toHaveBeenNthCalledWith(
      1,
      'codeswitch/services.LogService.RequestLogDailyHeatmapStatsByYearV2',
      2026,
      'session',
    )
    expect(Call.ByName).toHaveBeenNthCalledWith(
      2,
      'codeswitch/services.LogService.ListRequestLogHeatmapYearsV2',
      'all',
    )
  })
})
