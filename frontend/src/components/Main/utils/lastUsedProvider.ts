import type { LastUsedProvider, ProviderTab } from '../types'

const normalizePlatform = (platformValue: string): { platform: ProviderTab; toolId?: string } | null => {
  const platform = platformValue.trim()
  if (platform === 'claude' || platform === 'codex' || platform === 'gemini' || platform === 'grokbuild' || platform === 'claude-desktop' || platform === 'openclaw' || platform === 'hermes' || platform === 'pi' || platform === 'others') {
    return { platform }
  }

  if (platform.startsWith('custom:')) {
    const toolId = platform.slice('custom:'.length).trim()
    return {
      platform: 'others',
      toolId: toolId || undefined,
    }
  }

  return null
}

export const normalizeLastUsedProvider = (raw: any): LastUsedProvider | null => {
  if (!raw || typeof raw !== 'object') return null

  const rawPlatform = `${raw.platform ?? raw.Platform ?? ''}`.trim()
  const normalizedPlatform = normalizePlatform(rawPlatform)
  const providerId = `${raw.provider_id ?? raw.ProviderID ?? raw.providerId ?? ''}`.trim()
  const providerName = `${raw.provider_name ?? raw.ProviderName ?? raw.providerName ?? ''}`.trim()
  const updatedAt = Number(raw.updated_at ?? raw.UpdatedAt ?? raw.updatedAt ?? raw.timestamp ?? Date.now())

  if (!normalizedPlatform || !providerName) return null

  return {
    platform: normalizedPlatform.platform,
    source_platform: rawPlatform || normalizedPlatform.platform,
    tool_id: normalizedPlatform.toolId,
    provider_id: providerId || undefined,
    provider_name: providerName,
    updated_at: Number.isFinite(updatedAt) ? updatedAt : Date.now(),
  }
}

export const shouldUseLastUsedProviderForTool = (
  provider: LastUsedProvider,
  selectedToolId: string | null,
): boolean => {
  if (provider.platform !== 'others') return true
  if (!provider.tool_id) return selectedToolId === null
  return provider.tool_id === selectedToolId
}
