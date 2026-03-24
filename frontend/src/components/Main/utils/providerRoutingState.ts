import type { AutomationCard } from '../../../data/cards'

type HostedRouteStateInput = {
  activeProxyState: boolean
  isLastUsed: boolean
  enabled: boolean
  apiUrl: string
  apiKey: string
  isBlacklisted: boolean
}

export const isHostedRouteActive = ({
  activeProxyState,
  isLastUsed,
  enabled,
  apiUrl,
  apiKey,
  isBlacklisted,
}: HostedRouteStateInput): boolean => {
  if (!activeProxyState || !isLastUsed || !enabled || isBlacklisted) {
    return false
  }

  return apiUrl.trim() !== '' && apiKey.trim() !== ''
}

type HostedProviderCandidate = Pick<AutomationCard, 'id' | 'providerRef' | 'level' | 'enabled' | 'apiUrl' | 'apiKey'>

const normalizeLevel = (level: number | string | undefined): number => {
  const numeric = Number(level)
  if (!Number.isFinite(numeric) || numeric < 1) return 1
  return Math.min(10, Math.floor(numeric))
}

const providerIdentity = (provider: HostedProviderCandidate): string =>
  `${provider.providerRef ?? provider.id ?? ''}`.trim()

export const isHostedProviderRoutable = (
  provider: HostedProviderCandidate,
  isBlacklisted: boolean,
): boolean => (
  provider.enabled
  && provider.apiUrl.trim() !== ''
  && provider.apiKey.trim() !== ''
  && !isBlacklisted
)

export const getDefaultHostedProviderRef = <T extends HostedProviderCandidate>(
  providers: T[],
  isBlacklisted: (provider: T) => boolean,
): string | null => {
  let selected: T | null = null
  let selectedLevel = Number.MAX_SAFE_INTEGER

  providers.forEach((provider) => {
    if (!isHostedProviderRoutable(provider, isBlacklisted(provider))) {
      return
    }

    const level = normalizeLevel(provider.level)
    if (selected === null || level < selectedLevel) {
      selected = provider
      selectedLevel = level
    }
  })

  return selected ? providerIdentity(selected) : null
}
