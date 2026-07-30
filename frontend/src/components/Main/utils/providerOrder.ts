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

const compareOrderedEntries = <T extends AutomationCard>(left: OrderedEntry<T>, right: OrderedEntry<T>) => {
  const leftAutoDisabled = left.card.quotaAutoDisabled === true
  const rightAutoDisabled = right.card.quotaAutoDisabled === true
  if (leftAutoDisabled !== rightAutoDisabled) {
    return leftAutoDisabled ? 1 : -1
  }
  if (left.sortOrder !== right.sortOrder) {
    return left.sortOrder - right.sortOrder
  }
  return left.originalIndex - right.originalIndex
}

const resolveStatusGroup = (card: AutomationCard): ProviderStatusGroup => (
  card.enabled || card.quotaAutoDisabled ? 'enabled' : 'disabled'
)

const getGroupOrderKey = (group: ProviderStatusGroup): ProviderGroupOrderKey => (
  group === 'enabled' ? 'enabledSortOrder' : 'disabledSortOrder'
)

const getDisplaySortOrder = <T extends AutomationCard>(card: T, group: ProviderStatusGroup, fallback: number) => {
  const key = getGroupOrderKey(group)
  return normalizeSortOrder(card[key] ?? card.sortOrder, fallback)
}

const setGroupSortOrder = <T extends AutomationCard>(card: T, group: ProviderStatusGroup, value: number) => {
  const key = getGroupOrderKey(group)
  card[key] = value
  if (resolveStatusGroup(card) === group) {
    card.sortOrder = value
  }
}

const buildOrderedEntries = <T extends AutomationCard>(list: T[], group: ProviderStatusGroup): OrderedEntry<T>[] => (
  list
    .map((card, originalIndex) => ({
      card,
      originalIndex,
      sortOrder: getDisplaySortOrder(card, group, originalIndex + 1),
    }))
    .filter((entry) => resolveStatusGroup(entry.card) === group)
    .sort(compareOrderedEntries)
)

const countEnabledCards = <T extends AutomationCard>(list: T[]) => list.filter((card) => resolveStatusGroup(card) === 'enabled').length

const splitProviderGroupsPreservingOrder = <T extends AutomationCard>(list: T[]) => ({
  enabledCards: list.filter((card) => card.enabled && !card.quotaAutoDisabled),
  autoDisabledCards: list.filter((card) => card.quotaAutoDisabled),
  disabledCards: list.filter((card) => resolveStatusGroup(card) === 'disabled'),
})

const assignEnabledOrderPreservingAutoDisabled = <T extends AutomationCard>(
  enabledCards: T[],
  autoDisabledCards: T[],
) => {
  const reservedOrders = new Set(
    autoDisabledCards.map((card, index) => getDisplaySortOrder(card, 'enabled', enabledCards.length + index + 1)),
  )
  let nextOrder = 1
  enabledCards.forEach((card) => {
    while (reservedOrders.has(nextOrder)) {
      nextOrder += 1
    }
    setGroupSortOrder(card, 'enabled', nextOrder)
    nextOrder += 1
  })
}

export const applyNormalizedProviderOrder = <T extends AutomationCard>(list: T[]) => {
  if (!Array.isArray(list) || list.length === 0) return

  const enabledEntries = buildOrderedEntries(list, 'enabled')
  const disabledEntries = buildOrderedEntries(list, 'disabled')

  if (!enabledEntries.some((entry) => entry.card.quotaAutoDisabled)) {
    enabledEntries.forEach((entry, index) => {
      setGroupSortOrder(entry.card, 'enabled', index + 1)
    })
  }
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

  const { enabledCards, autoDisabledCards, disabledCards } = splitProviderGroupsPreservingOrder(list)

  assignEnabledOrderPreservingAutoDisabled(enabledCards, autoDisabledCards)
  disabledCards.forEach((card, index) => {
    setGroupSortOrder(card, 'disabled', index + 1)
  })

  list.splice(0, list.length, ...enabledCards, ...autoDisabledCards, ...disabledCards)
}

export const insertProviderToStatusGroup = <T extends AutomationCard>(list: T[], card: T) => {
  applyNormalizedProviderOrder(list)

  const enabledCount = countEnabledCards(list)
  const targetGroup = resolveStatusGroup(card)
  const insertIndex = enabledCount
  const nextSortOrder = card.enabled
    ? list.filter((item) => item.enabled).length + 1
    : 1

  setGroupSortOrder(card, targetGroup, nextSortOrder)
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
  moved.enabled = enabled

  const enabledCount = countEnabledCards(list)
  const insertIndex = enabled ? 0 : enabledCount

  list.splice(insertIndex, 0, moved)

  commitProviderOrder(list)
  return true
}
