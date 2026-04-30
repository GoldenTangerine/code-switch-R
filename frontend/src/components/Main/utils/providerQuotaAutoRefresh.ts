import type { AutomationCard } from '../../../data/cards'
import type { ProviderTab } from '../types'

export function shouldAutoRefreshProviderQuota(
  tab: ProviderTab,
  card: Pick<AutomationCard, 'enabled' | 'isInConfig'>,
  isCurrentlyActive: boolean,
): boolean {
  if (tab === 'opencode') {
    return card.isInConfig ?? card.enabled
  }

  return isCurrentlyActive
}
