import type { AutomationCard } from '../../../data/cards'
import type { ProviderTab, VendorForm } from '../types'

type DirectApplyProviderLike = Pick<AutomationCard, 'apiFormat'> | Pick<VendorForm, 'apiFormat'>

export const claudeDirectApplyRequiresHostedRouting = (
  provider: DirectApplyProviderLike,
): boolean => (provider.apiFormat || 'anthropic') !== 'anthropic'

export const isDirectApplyBlockedForProvider = (
  tabId: ProviderTab,
  provider: DirectApplyProviderLike,
): boolean => tabId === 'claude' && claudeDirectApplyRequiresHostedRouting(provider)
