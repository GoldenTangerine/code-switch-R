import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@wailsio/runtime', () => ({
  Call: {
    ByName: vi.fn(),
  },
}))

import { Call } from '@wailsio/runtime'
import { fetchAppSettings, normalizeHeatmapGranularity } from './appSettings'

describe('appSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('falls back to daily when heatmap granularity is missing', async () => {
    vi.mocked(Call.ByName).mockResolvedValueOnce({})

    const settings = await fetchAppSettings()

    expect(settings.heatmap_granularity).toBe('daily')
  })

  it('preserves explicit hourly heatmap granularity', async () => {
    vi.mocked(Call.ByName).mockResolvedValueOnce({
      heatmap_granularity: 'hourly',
    })

    const settings = await fetchAppSettings()

    expect(settings.heatmap_granularity).toBe('hourly')
  })

  it('defaults home provider tabs to Claude Code, Codex, and Gemini', async () => {
    vi.mocked(Call.ByName).mockResolvedValueOnce({})

    const settings = await fetchAppSettings()

    expect(settings.home_provider_tabs).toEqual(['claude', 'codex', 'gemini'])
  })

  it('normalizes invalid and duplicate home provider tabs', async () => {
    vi.mocked(Call.ByName).mockResolvedValueOnce({
      home_provider_tabs: ['opencode', 'weird', 'opencode', 'others'],
    })

    const settings = await fetchAppSettings()

    expect(settings.home_provider_tabs).toEqual(['opencode', 'others'])
  })

  it('normalizes provider concurrency limit switches', async () => {
    vi.mocked(Call.ByName).mockResolvedValueOnce({
      provider_concurrency_limits: {
        claude: true,
        codex: false,
        '  ': true,
      },
    })

    const settings = await fetchAppSettings()

    expect(settings.provider_concurrency_limits).toEqual({
      claude: true,
      codex: false,
    })
  })

  it('disables Claude model aggregation when model routing is off', async () => {
    vi.mocked(Call.ByName).mockResolvedValueOnce({
      claude_model_routing_enabled: false,
      claude_model_aggregation_enabled: true,
    })

    const settings = await fetchAppSettings()

    expect(settings.claude_model_routing_enabled).toBe(false)
    expect(settings.claude_model_aggregation_enabled).toBe(false)
  })

  it('normalizes Claude model metadata strategy', async () => {
    vi.mocked(Call.ByName).mockResolvedValueOnce({
      claude_model_routing_enabled: true,
      claude_model_aggregation_enabled: true,
      claude_model_metadata_merge_strategy: 'invalid',
    })

    const settings = await fetchAppSettings()

    expect(settings.claude_model_aggregation_enabled).toBe(true)
    expect(settings.claude_model_metadata_merge_strategy).toBe('aggressive')
  })

  it('migrates legacy provider quota query preset codes to named presets', async () => {
    vi.mocked(Call.ByName).mockResolvedValueOnce({
      provider_quota_query_preset_codes: {
        general: '({ request: { url: "{{baseUrl}}/usage" }, extractor: function() {} })',
      },
    })

    const settings = await fetchAppSettings()

    expect(settings.provider_quota_query_presets.general).toEqual({
      defaultId: 'legacy-general',
      items: [{
        id: 'legacy-general',
        name: '自定义预设',
        code: '({ request: { url: "{{baseUrl}}/usage" }, extractor: function() {} })',
      }],
    })
  })

  it('uses the provided fallback for invalid granularity values', () => {
    expect(normalizeHeatmapGranularity('weird-value')).toBe('daily')
    expect(normalizeHeatmapGranularity('weird-value', 'hourly')).toBe('hourly')
  })
})
