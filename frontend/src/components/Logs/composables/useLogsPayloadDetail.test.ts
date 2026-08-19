/**
 * @name: 日志载荷详情测试
 * @Descripttion: 验证请求与响应详情统一使用公共剪贴板工具
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-19 20:45:00
 * @LastEditTime: 2026-08-19 20:45:00
 * @FilePath: frontend/src/components/Logs/composables/useLogsPayloadDetail.test.ts
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { RequestLog } from '../../../services/logs'

const { fetchRequestLogPayloadMock, showToastMock, writeTextToClipboardMock } = vi.hoisted(() => ({
  fetchRequestLogPayloadMock: vi.fn(),
  showToastMock: vi.fn(),
  writeTextToClipboardMock: vi.fn(),
}))

vi.mock('../../../services/logs', () => ({
  fetchRequestLogPayload: fetchRequestLogPayloadMock,
}))

vi.mock('../../../utils/clipboard', () => ({
  writeTextToClipboard: writeTextToClipboardMock,
}))

vi.mock('../../../utils/toast', () => ({
  showToast: showToastMock,
}))

import { useLogsPayloadDetail } from './useLogsPayloadDetail'

const requestLog: RequestLog = {
  id: 7,
  platform: 'codex',
  model: 'gpt-5',
  provider: 'provider-1',
  http_code: 200,
  input_tokens: 10,
  output_tokens: 20,
  cache_create_tokens: 0,
  cache_read_tokens: 0,
  reasoning_tokens: 0,
  created_at: '2026-08-19 20:45:00',
}

const t = (key: string, params?: Record<string, unknown>) => (
  params == null ? key : `${key}:${JSON.stringify(params)}`
)

describe('useLogsPayloadDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    fetchRequestLogPayloadMock.mockResolvedValue({
      id: 7,
      request_body: '{"message":"hello"}',
      response_body: '{"ok":true}',
    })
    writeTextToClipboardMock.mockResolvedValue(undefined)
  })

  it('copies request and response payloads through the shared clipboard utility', async () => {
    const { copyPayloadDetail, openPayloadDetailModal } = useLogsPayloadDetail({ t })
    await openPayloadDetailModal(requestLog)

    await copyPayloadDetail('request', 'raw')
    await copyPayloadDetail('request', 'formatted')
    await copyPayloadDetail('response', 'raw')
    await copyPayloadDetail('response', 'formatted')

    expect(writeTextToClipboardMock.mock.calls).toEqual([
      ['{"message":"hello"}'],
      ['{\n  "message": "hello"\n}'],
      ['{"ok":true}'],
      ['{\n  "ok": true\n}'],
    ])
  })

  it('keeps the detailed error message when the shared clipboard utility fails', async () => {
    writeTextToClipboardMock.mockRejectedValueOnce(new Error('native clipboard unavailable'))
    const { copyPayloadDetail, openPayloadDetailModal } = useLogsPayloadDetail({ t })
    await openPayloadDetailModal(requestLog)

    await copyPayloadDetail('request', 'raw')

    expect(showToastMock).toHaveBeenLastCalledWith(
      'components.logs.payloadDetail.copyFailed:{"error":"native clipboard unavailable"}',
      'error',
    )
  })
})
