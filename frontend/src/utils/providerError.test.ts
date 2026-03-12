import { describe, expect, it } from 'vitest'
import { parseProviderErrorFromConsoleMessage } from './providerError'

describe('providerError', () => {
  it('returns null for empty or undefined-like input', () => {
    expect(parseProviderErrorFromConsoleMessage('')).toBeNull()
    expect(parseProviderErrorFromConsoleMessage(undefined as unknown as string)).toBeNull()
  })

  it('extracts upstream json error details from console logs', () => {
    const detail = parseProviderErrorFromConsoleMessage(
      '[WARN] ✗ 失败: acme | 错误: upstream status 503: {"error":{"message":"The model is overloaded. Please try again later.","type":"overloaded_error","code":"overloaded","param":"model"}} | 耗时: 1.23s',
    )

    expect(detail).not.toBeNull()
    expect(detail?.statusCode).toBe(503)
    expect(detail?.providerMessage).toBe('The model is overloaded. Please try again later.')
    expect(detail?.errorType).toBe('overloaded_error')
    expect(detail?.errorCode).toBe('overloaded')
    expect(detail?.errorParam).toBe('model')
    expect(detail?.semanticTag).toBe('模型负载过高')
    expect(detail?.copyText).toContain('"message": "The model is overloaded. Please try again later."')
  })

  it('extracts google style upstream status message', () => {
    const detail = parseProviderErrorFromConsoleMessage(
      '[ERROR] upstream status 500: {"error":{"code":500,"message":"Internal server error","status":"INTERNAL"}}',
    )

    expect(detail).not.toBeNull()
    expect(detail?.statusCode).toBe(500)
    expect(detail?.providerMessage).toBe('Internal server error')
    expect(detail?.errorStatus).toBe('INTERNAL')
    expect(detail?.semanticTag).toBe('接口 5xx')
  })

  it('extracts real provider payload with folded request id from console logs', () => {
    const detail = parseProviderErrorFromConsoleMessage(
      '[ERROR] Upstream codex provider=Any Router status=503 url=https://example.com/responses content_type=application/json\n{"error":{"type":"new_api_error","message":"当前模型 claude-opus-4-6 负载已经达到上限，请稍后重试 (request id:\\n     202603121813186562816676KXPmIKr)"},"type":"error"}',
    )

    expect(detail).not.toBeNull()
    expect(detail?.statusCode).toBe(503)
    expect(detail?.providerMessage).toBe(
      '当前模型 claude-opus-4-6 负载已经达到上限，请稍后重试 (request id: 202603121813186562816676KXPmIKr)',
    )
    expect(detail?.errorType).toBe('new_api_error')
    expect(detail?.summary).toContain('request id: 202603121813186562816676KXPmIKr')
    expect(detail?.semanticTag).toBe('模型负载过高')
    expect(detail?.copyText).toContain('"type": "new_api_error"')
  })

  it('treats bare http 500 log as provider error summary', () => {
    const detail = parseProviderErrorFromConsoleMessage(
      '[Gemini] ✗ 失败: fallback-provider | HTTP 500 | 耗时: 0.87s',
    )

    expect(detail).not.toBeNull()
    expect(detail?.statusCode).toBe(500)
    expect(detail?.summary).toBe('上游返回 HTTP 500')
    expect(detail?.copyText).toBe('上游返回 HTTP 500')
  })

  it('ignores upstream status 200 logs even if they include json payload', () => {
    const detail = parseProviderErrorFromConsoleMessage(
      '[INFO] 请求完成: upstream status 200: {"message":"ok","result":{"id":"resp_123"}} | 耗时: 0.21s',
    )

    expect(detail).toBeNull()
  })

  it('keeps malformed upstream payload as fallback summary', () => {
    const detail = parseProviderErrorFromConsoleMessage(
      '[ERROR] upstream status 500: {broken json | 耗时: 0.91s',
    )

    expect(detail).not.toBeNull()
    expect(detail?.statusCode).toBe(500)
    expect(detail?.summary).toBe('{broken json')
    expect(detail?.copyText).toBe('{broken json')
  })

  it('classifies rate limit errors', () => {
    const detail = parseProviderErrorFromConsoleMessage(
      '[WARN] upstream status 429: {"error":{"message":"Rate limit exceeded","type":"rate_limit_error","code":"too_many_requests"}}',
    )

    expect(detail).not.toBeNull()
    expect(detail?.semanticTag).toBe('限流 / 配额')
  })

  it('classifies authentication failures', () => {
    const detail = parseProviderErrorFromConsoleMessage(
      '[WARN] upstream status 401: {"error":{"message":"Unauthorized","type":"authentication_error","code":"invalid_api_key"}}',
    )

    expect(detail).not.toBeNull()
    expect(detail?.semanticTag).toBe('鉴权失败')
  })

  it('ignores non-error console lines', () => {
    const detail = parseProviderErrorFromConsoleMessage(
      '[INFO] ✓ 成功: acme | HTTP 200 | 耗时: 0.21s',
    )

    expect(detail).toBeNull()
  })
})
