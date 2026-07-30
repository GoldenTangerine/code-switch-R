import type { AutomationCard } from '../../../data/cards'
import type { ProviderTab } from '../types'
import { applyNormalizedProviderOrder } from './providerOrder'

export type ProviderQuotaStateChange = {
  providerId: string
  enabled: boolean
  quotaAutoDisabled: boolean
  quotaAutoDisablePaused: boolean
}

export function applyProviderQuotaStateChange(
  cards: AutomationCard[],
  change: ProviderQuotaStateChange,
): boolean {
  const providerId = `${change?.providerId ?? ''}`.trim()
  if (
    !providerId
    || typeof change.enabled !== 'boolean'
    || typeof change.quotaAutoDisabled !== 'boolean'
    || typeof change.quotaAutoDisablePaused !== 'boolean'
  ) {
    return false
  }

  const card = cards.find((item) => (
    (`${item.providerRef ?? ''}`.trim() || `${item.id}`) === providerId
  ))
  if (!card) return false

  card.enabled = change.enabled
  card.quotaAutoDisabled = change.quotaAutoDisabled
  card.quotaAutoDisablePaused = change.quotaAutoDisablePaused
  applyNormalizedProviderOrder(cards)
  return true
}

export function shouldAutoRefreshProviderQuota(
  tab: ProviderTab,
  card: Pick<AutomationCard, 'enabled' | 'isInConfig' | 'quotaAutoDisabled' | 'quotaAutoDisablePaused'>,
  isCurrentlyActive: boolean,
): boolean {
  if (card.quotaAutoDisabled || card.quotaAutoDisablePaused) return true
  if (tab === 'opencode') {
    return card.isInConfig ?? card.enabled
  }

  return isCurrentlyActive
}
