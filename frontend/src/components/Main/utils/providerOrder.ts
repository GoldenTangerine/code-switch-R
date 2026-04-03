import type { AutomationCard } from '../../../data/cards'

type ProviderStatusGroup = 'enabled' | 'disabled'
type ProviderGroupOrderKey = 'enabledSortOrder' | 'disabledSortOrder'

type OrderedEntry<T extends AutomationCard> = {
  card: T
  originalIndex: number
  sortOrder: number
}

const normalizeSortOrder = (value: number | undefined, fallback: number) => {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric < 1) return fallback
  return Math.floor(numeric)
}

const normalizeStoredGroupSortOrder = (value: number | undefined): number | null => {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric < 1) return null
  return Math.floor(numeric)
}

const compareOrderedEntries = <T extends AutomationCard>(left: OrderedEntry<T>, right: OrderedEntry<T>) => {
  if (left.sortOrder !== right.sortOrder) {
    return left.sortOrder - right.sortOrder
  }
  return left.originalIndex - right.originalIndex
}

const resolveStatusGroup = (enabled: boolean): ProviderStatusGroup => (enabled ? 'enabled' : 'disabled')

const getGroupOrderKey = (group: ProviderStatusGroup): ProviderGroupOrderKey => (
  group === 'enabled' ? 'enabledSortOrder' : 'disabledSortOrder'
)

const getDisplaySortOrder = <T extends AutomationCard>(card: T, group: ProviderStatusGroup, fallback: number) => {
  const key = getGroupOrderKey(group)
  return normalizeSortOrder(card[key] ?? card.sortOrder, fallback)
}

const getRememberedGroupSortOrder = <T extends AutomationCard>(card: T, group: ProviderStatusGroup) => {
  const key = getGroupOrderKey(group)
  return normalizeStoredGroupSortOrder(card[key])
}

const setGroupSortOrder = <T extends AutomationCard>(card: T, group: ProviderStatusGroup, value: number) => {
  const key = getGroupOrderKey(group)
  card[key] = value
  if (resolveStatusGroup(card.enabled) === group) {
    card.sortOrder = value
  }
}

const buildOrderedEntries = <T extends AutomationCard>(list: T[], enabled: boolean): OrderedEntry<T>[] => (
  list
    .map((card, originalIndex) => ({
      card,
      originalIndex,
      sortOrder: getDisplaySortOrder(card, resolveStatusGroup(enabled), originalIndex + 1),
    }))
    .filter((entry) => entry.card.enabled === enabled)
    .sort(compareOrderedEntries)
)

const countEnabledCards = <T extends AutomationCard>(list: T[]) => list.filter((card) => card.enabled).length

const splitProviderGroupsPreservingOrder = <T extends AutomationCard>(list: T[]) => ({
  enabledCards: list.filter((card) => card.enabled),
  disabledCards: list.filter((card) => !card.enabled),
})

export const applyNormalizedProviderOrder = <T extends AutomationCard>(list: T[]) => {
  if (!Array.isArray(list) || list.length === 0) return

  const enabledEntries = buildOrderedEntries(list, true)
  const disabledEntries = buildOrderedEntries(list, false)

  enabledEntries.forEach((entry, index) => {
    setGroupSortOrder(entry.card, 'enabled', index + 1)
  })
  disabledEntries.forEach((entry, index) => {
    setGroupSortOrder(entry.card, 'disabled', index + 1)
  })

  list.splice(
    0,
    list.length,
    ...enabledEntries.map((entry) => entry.card),
    ...disabledEntries.map((entry) => entry.card),
  )
}

export const commitProviderOrder = <T extends AutomationCard>(list: T[]) => {
  if (!Array.isArray(list) || list.length === 0) return

  const { enabledCards, disabledCards } = splitProviderGroupsPreservingOrder(list)

  enabledCards.forEach((card, index) => {
    setGroupSortOrder(card, 'enabled', index + 1)
  })
  disabledCards.forEach((card, index) => {
    setGroupSortOrder(card, 'disabled', index + 1)
  })

  list.splice(0, list.length, ...enabledCards, ...disabledCards)
}

export const appendProviderToStatusGroup = <T extends AutomationCard>(list: T[], card: T) => {
  applyNormalizedProviderOrder(list)

  setGroupSortOrder(
    card,
    resolveStatusGroup(card.enabled),
    list.filter((item) => item.enabled === card.enabled).length + 1,
  )

  const enabledCount = countEnabledCards(list)
  const insertIndex = card.enabled ? enabledCount : list.length
  list.splice(insertIndex, 0, card)

  commitProviderOrder(list)
}

export const moveProviderToStatusGroup = <T extends AutomationCard>(
  list: T[],
  card: T,
  enabled: boolean,
) => {
  applyNormalizedProviderOrder(list)

  const currentIndex = list.findIndex((item) => item === card || item.id === card.id)
  if (currentIndex < 0) return false

  const [moved] = list.splice(currentIndex, 1)
  const targetGroup = resolveStatusGroup(enabled)
  const targetCards = list.filter((item) => item.enabled === enabled)
  const rememberedSortOrder = getRememberedGroupSortOrder(moved, targetGroup)
  const targetIndexWithinGroup = rememberedSortOrder === null
    ? targetCards.length
    : Math.min(Math.max(rememberedSortOrder - 1, 0), targetCards.length)

  moved.enabled = enabled

  const enabledCount = countEnabledCards(list)
  const insertIndex = enabled
    ? targetIndexWithinGroup
    : enabledCount + targetIndexWithinGroup

  list.splice(insertIndex, 0, moved)

  commitProviderOrder(list)
  return true
}
