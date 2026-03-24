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
