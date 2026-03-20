import { describe, expect, it } from 'vitest'
import type { RequestLog } from '../../../services/logs'
import { parseProviderErrorFromConsoleMessage } from '../../../utils/providerError'
import {
  buildConsoleProviderErrorCandidates,
  getConsoleCandidateQualityScore,
  matchConsoleProviderCandidate,
  type ConsoleProviderErrorCandidate,
} from './providerLogsConsoleMatch'

const buildRequestLog = (overrides: Partial<RequestLog> = {}): RequestLog => ({
  id: 25570,
  platform: 'claude',
  model: 'claude-opus-4-6',
  requested_model: 'claude-opus-4-6',
  response_model: '',
  provider_id: 'pinche-pro',
  provider: '拼车-Pro',
  http_code: 503,
  input_tokens: 0,
  output_tokens: 0,
  cache_create_tokens: 0,
  cache_read_tokens: 0,
  reasoning_tokens: 0,
  created_at: '2026-03-20T14:51:29.000Z',
  ...overrides,
})

describe('providerLogsConsoleMatch', () => {
  it('coalesces upstream metadata line with following compact json payload line', () => {
    const candidates = buildConsoleProviderErrorCandidates([
      {
        timestamp: '2026-03-20T14:51:29.000Z',
        level: 'ERROR',
        message: '[ERROR] Upstream claude provider=拼车-Pro status=503 url=https://api-cch.pipidan.xyz/v1/messages content_type=application/json; charset=utf-8',
      },
      {
        timestamp: '2026-03-20T14:51:29.120Z',
        level: 'ERROR',
        message: '{"error":{"message":"No available providers (cch_session_id: 9aeda3c7-77c1-47be-84e0-10f7f49d24ab)","type":"no_available_providers","code":"no_available_providers"}}',
      },
    ])

    expect(candidates).toHaveLength(1)
    expect(candidates[0].providerError.providerMessage).toBe(
      'No available providers (cch_session_id: 9aeda3c7-77c1-47be-84e0-10f7f49d24ab)',
    )
    expect(candidates[0].providerError.errorType).toBe('no_available_providers')
    expect(candidates[0].message).toContain('"code":"no_available_providers"')
  })

  it('coalesces upstream metadata line with pretty-printed multi-line json payload', () => {
    const candidates = buildConsoleProviderErrorCandidates([
      {
        timestamp: '2026-03-20T14:51:29.000Z',
        level: 'ERROR',
        message: '[ERROR] Upstream claude provider=拼车-Pro status=503 url=https://api-cch.pipidan.xyz/v1/messages content_type=application/json; charset=utf-8',
      },
      { timestamp: '2026-03-20T14:51:29.050Z', level: 'ERROR', message: '{' },
      { timestamp: '2026-03-20T14:51:29.080Z', level: 'ERROR', message: '  "error": {' },
      { timestamp: '2026-03-20T14:51:29.100Z', level: 'ERROR', message: '    "message": "No available providers",' },
      { timestamp: '2026-03-20T14:51:29.120Z', level: 'ERROR', message: '    "type": "no_available_providers",' },
      { timestamp: '2026-03-20T14:51:29.140Z', level: 'ERROR', message: '    "code": "no_available_providers"' },
      { timestamp: '2026-03-20T14:51:29.160Z', level: 'ERROR', message: '  }' },
      { timestamp: '2026-03-20T14:51:29.180Z', level: 'ERROR', message: '}' },
      {
        timestamp: '2026-03-20T14:51:31.000Z',
        level: 'INFO',
        message: '[INFO] some unrelated follow-up line',
      },
    ])

    expect(candidates).toHaveLength(1)
    expect(candidates[0].providerError.providerMessage).toBe('No available providers')
    expect(candidates[0].providerError.errorCode).toBe('no_available_providers')
  })

  it('prefers rich provider errors over metadata-only candidates when homepage matches logs', () => {
    const metadataOnly = parseProviderErrorFromConsoleMessage(
      'status 503: url=https://api-cch.pipidan.xyz/v1/messages content_type=application/json; charset=utf-8',
    )
    const richDetail = parseProviderErrorFromConsoleMessage(
      'status 503: {"error":{"message":"No available providers","type":"no_available_providers","code":"no_available_providers"}}',
    )

    expect(metadataOnly).not.toBeNull()
    expect(richDetail).not.toBeNull()

    const candidates: ConsoleProviderErrorCandidate[] = [
      {
        timestamp: '2026-03-20T14:51:29.000Z',
        level: 'ERROR',
        message: '[ERROR] Upstream claude provider=拼车-Pro status=503 url=https://api-cch.pipidan.xyz/v1/messages content_type=application/json; charset=utf-8',
        providerError: metadataOnly!,
      },
      {
        timestamp: '2026-03-20T14:51:29.120Z',
        level: 'ERROR',
        message: '[WARN] ✗ Level 1 失败: 拼车-Pro | 错误: upstream status 503: {"error":{"message":"No available providers","type":"no_available_providers","code":"no_available_providers"}} | 耗时: 0.24s',
        providerError: richDetail!,
      },
    ]

    const matched = matchConsoleProviderCandidate(buildRequestLog(), candidates, ['拼车-Pro', 'pinche-pro'])

    expect(matched).not.toBeNull()
    expect(matched?.index).toBe(1)
    expect(matched?.candidate.providerError.providerMessage).toBe('No available providers')
    expect(getConsoleCandidateQualityScore(richDetail)).toBeGreaterThan(getConsoleCandidateQualityScore(metadataOnly))
  })
})
