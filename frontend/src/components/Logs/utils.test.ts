import { describe, expect, it } from 'vitest'
import type { ModelUsageStat, RequestLog } from '../../services/logs'
import {
  buildModelShareRows,
  buildLogsInfoTooltipLabels,
  buildStreamDiagnosticTooltipDetailData,
  formatCurrencyParts,
  formatPreciseCurrency,
  formatFirstTokenDuration,
  formatModelShareTooltipLabel,
  formatReasoningEffortSource,
  formatTokensPerSecond,
  normalizeReasoningEffortDisplay,
  resolveReasoningEffortTone,
} from './utils'

const MODEL_SHARE_COLORS = ['#818cf8', '#fb923c', '#34d399', '#60a5fa'] as const

const createModelUsageStat = (overrides: Partial<ModelUsageStat> = {}): ModelUsageStat => ({
  model: 'model-a',
  total_requests: 0,
  input_tokens: 0,
  output_tokens: 0,
  cache_read_tokens: 0,
  total_tokens: 0,
  cost_total: 0,
  ...overrides,
})

describe('buildModelShareRows', () => {
  it('sorts rows by requests, then tokens, then cost', () => {
    const rows = buildModelShareRows(
      [
        createModelUsageStat({ model: 'alpha', total_requests: 5, total_tokens: 100, cost_total: 1 }),
        createModelUsageStat({ model: 'beta', total_requests: 8, total_tokens: 10, cost_total: 0.2 }),
        createModelUsageStat({ model: 'gamma', total_requests: 5, total_tokens: 120, cost_total: 0.1 }),
        createModelUsageStat({ model: 'delta', total_requests: 5, total_tokens: 120, cost_total: 0.3 }),
      ],
      MODEL_SHARE_COLORS,
    )

    expect(rows.map(item => item.model)).toEqual(['beta', 'delta', 'gamma', 'alpha'])
    expect(rows.map(item => item.color)).toEqual(['#818cf8', '#fb923c', '#34d399', '#60a5fa'])
  })

  it('merges model names case-insensitively and falls back to summed tokens when total_tokens is empty', () => {
    const rows = buildModelShareRows(
      [
        createModelUsageStat({
          model: 'GLM-4-Plus',
          total_requests: 2,
          input_tokens: 10,
          output_tokens: 20,
          cache_read_tokens: 5,
          total_tokens: 0,
          cost_total: 0.1,
        }),
        createModelUsageStat({
          model: 'glm-4-plus',
          total_requests: 3,
          input_tokens: 15,
          output_tokens: 25,
          cache_read_tokens: 10,
          total_tokens: 0,
          cost_total: 0.2,
        }),
      ],
      MODEL_SHARE_COLORS,
    )

    expect(rows).toHaveLength(1)
    expect(rows[0]?.model).toBe('GLM-4-Plus')
    expect(rows[0]?.requests).toBe(5)
    expect(rows[0]?.tokens).toBe(85)
    expect(rows[0]?.cost).toBeCloseTo(0.3, 10)
    expect(rows[0]?.color).toBe('#818cf8')
  })
})

describe('formatModelShareTooltipLabel', () => {
  it('formats request tooltip text with a custom localized unit', () => {
    expect(formatModelShareTooltipLabel('glm-4-plus', 42, 120, '请求')).toBe('glm-4-plus: 42 请求 (35.0%)')
  })

  it('uses the default req suffix and handles empty totals safely', () => {
    expect(formatModelShareTooltipLabel('ark-code-v2', 7, 0)).toBe('ark-code-v2: 7 req (0.0%)')
  })
})

describe('formatCurrencyParts', () => {
  it('keeps small fractional spend precision aligned with the shared currency formatter', () => {
    expect(formatCurrencyParts(0.0042)).toEqual({
      symbol: '$',
      whole: '0',
      fraction: '0042',
      formatted: '$0.0042',
    })
  })
})

describe('formatPreciseCurrency', () => {
  it('keeps six decimal places for small per-request costs', () => {
    expect(formatPreciseCurrency(0.0000124)).toBe('$0.000012')
    expect(formatPreciseCurrency(undefined)).toBe('$0.000000')
  })
})

describe('reasoning effort helpers', () => {
  it('normalizes and tones reasoning effort values', () => {
    expect(normalizeReasoningEffortDisplay('x-high')).toBe('xhigh')
    expect(normalizeReasoningEffortDisplay('MAX')).toBe('max')
    expect(resolveReasoningEffortTone('low')).toBe('low')
    expect(resolveReasoningEffortTone('unknown')).toBe('unknown')
  })

  it('formats reasoning effort sources and historical missing values', () => {
    const labels = buildLogsInfoTooltipLabels((key) => key)
    expect(formatReasoningEffortSource('model_mapping', labels)).toBe(
      'components.logs.table.reasoningEffortSourceValues.modelMapping',
    )
    expect(formatReasoningEffortSource('', labels)).toBe(
      'components.logs.table.tooltipValues.missing',
    )
  })
})

describe('formatTokensPerSecond', () => {
  it('uses total request duration without subtracting first-token latency', () => {
    const item = {
      is_stream: true,
      output_tokens: 100,
      duration_sec: 2,
      first_token_sec: 1.5,
    } as RequestLog

    expect(formatTokensPerSecond(item)).toBe('50.00 tokens/s')
  })

  it('does not require first-token latency but rejects non-streaming or invalid duration rows', () => {
    expect(formatTokensPerSecond({
      is_stream: true,
      output_tokens: 30,
      duration_sec: 1.5,
    } as RequestLog)).toBe('20.00 tokens/s')
    expect(formatTokensPerSecond({
      is_stream: false,
      output_tokens: 30,
      duration_sec: 1.5,
    } as RequestLog)).toBe('—')
    expect(formatTokensPerSecond({
      is_stream: true,
      output_tokens: 30,
      duration_sec: 0,
    } as RequestLog)).toBe('—')
  })
})

describe('formatFirstTokenDuration', () => {
  it('uses adaptive duration units for streaming logs', () => {
    expect(formatFirstTokenDuration({
      is_stream: true,
      first_token_sec: 65,
    } as RequestLog)).toBe('1m 05s')
  })

  it('keeps non-streaming logs empty', () => {
    expect(formatFirstTokenDuration({
      is_stream: false,
      first_token_sec: 1.23,
    } as RequestLog)).toBe('—')
  })
})

describe('buildStreamDiagnosticTooltipDetailData', () => {
  const labels = {
    title: 'Stream diagnostics',
    statusLabel: 'Status',
    lastEventLabel: 'Last event',
    compactionLabel: 'Compaction',
    protocolLabel: 'Protocol',
    bytesLabel: 'Bytes',
    missingValue: 'Not recorded',
    compactionRequested: 'Requested',
    compactionObserved: 'Observed',
    compactionNotObserved: 'Not observed',
    errorKindLabels: {
      missing_terminal: 'Missing terminal event',
      empty_stream: 'Empty stream',
    },
  }

  it('builds a diagnostic tooltip for an incomplete compaction stream', () => {
    expect(buildStreamDiagnosticTooltipDetailData({
      is_stream: true,
      stream_error_kind: 'missing_terminal',
      stream_last_event: 'response.output_item.done',
      stream_compaction_requested: true,
      stream_compaction_observed: true,
      stream_bytes: 2048,
      upstream_protocol: 'HTTP/2.0',
    } as RequestLog, labels)).toEqual({
      title: 'Stream diagnostics',
      variant: 'stream',
      rows: [
        { key: 'stream-status', label: 'Status', value: 'Missing terminal event' },
        { key: 'stream-last-event', label: 'Last event', value: 'response.output_item.done' },
        { key: 'stream-compaction', label: 'Compaction', value: 'Requested · Observed' },
        { key: 'stream-protocol', label: 'Protocol', value: 'HTTP/2.0' },
        { key: 'stream-bytes', label: 'Bytes', value: '2,048' },
      ],
    })
  })

  it('returns null for legacy rows without stream diagnostics', () => {
    expect(buildStreamDiagnosticTooltipDetailData({} as RequestLog, labels)).toBeNull()
  })
})
