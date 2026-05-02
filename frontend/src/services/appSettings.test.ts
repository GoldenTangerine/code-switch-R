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

  it('uses the provided fallback for invalid granularity values', () => {
    expect(normalizeHeatmapGranularity('weird-value')).toBe('daily')
    expect(normalizeHeatmapGranularity('weird-value', 'hourly')).toBe('hourly')
  })
})
