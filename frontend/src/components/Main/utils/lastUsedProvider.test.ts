import { describe, expect, it } from 'vitest'
import { normalizeLastUsedProvider, shouldUseLastUsedProviderForTool } from './lastUsedProvider'

describe('lastUsedProvider helpers', () => {
  it('normalizes wails payloads and maps custom tool platforms to others', () => {
    expect(normalizeLastUsedProvider({
      Platform: 'codex',
      ProviderID: 'pid-1',
      ProviderName: 'Alpha',
      UpdatedAt: 123,
    })).toEqual({
      platform: 'codex',
      source_platform: 'codex',
      tool_id: undefined,
      provider_id: 'pid-1',
      provider_name: 'Alpha',
      updated_at: 123,
    })

    expect(normalizeLastUsedProvider({
      platform: 'custom:tool-a',
      providerId: 'pid-2',
      providerName: 'Beta',
      timestamp: 456,
    })).toEqual({
      platform: 'others',
      source_platform: 'custom:tool-a',
      tool_id: 'tool-a',
      provider_id: 'pid-2',
      provider_name: 'Beta',
      updated_at: 456,
    })
  })

  it('filters others state by selected tool id', () => {
    const provider = normalizeLastUsedProvider({
      platform: 'custom:tool-a',
      providerId: 'pid-2',
      providerName: 'Beta',
    })

    expect(provider).not.toBeNull()
    expect(shouldUseLastUsedProviderForTool(provider!, 'tool-a')).toBe(true)
    expect(shouldUseLastUsedProviderForTool(provider!, 'tool-b')).toBe(false)
    expect(shouldUseLastUsedProviderForTool(provider!, null)).toBe(false)
  })
})
