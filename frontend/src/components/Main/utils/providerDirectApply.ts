import type { AutomationCard } from '../../../data/cards'
import type { ProviderTab, VendorForm } from '../types'

type DirectApplyProviderLike =
  | Pick<AutomationCard, 'apiFormat' | 'connectivityAuthType'>
  | Pick<VendorForm, 'apiFormat' | 'connectivityAuthType'>

const claudeAuthRequiresHostedRouting = (authType: string | undefined): boolean => {
  const normalized = `${authType ?? ''}`.trim().toLowerCase()
  return normalized !== '' && normalized !== 'bearer' && normalized !== 'x-api-key'
}

export const claudeDirectApplyRequiresHostedRouting = (
  provider: DirectApplyProviderLike,
): boolean => (
  (provider.apiFormat || 'anthropic') !== 'anthropic'
  || claudeAuthRequiresHostedRouting(provider.connectivityAuthType)
)

export const isDirectApplyBlockedForProvider = (
  tabId: ProviderTab,
  provider: DirectApplyProviderLike,
): boolean => (
  tabId === 'opencode' ||
  (tabId === 'claude' && claudeDirectApplyRequiresHostedRouting(provider))
)
