import type { AutomationCard } from '../../../data/cards'

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
  if (left.sortOrder !== right.sortOrder) {
    return left.sortOrder - right.sortOrder
  }
  return left.originalIndex - right.originalIndex
}

const buildOrderedEntries = <T extends AutomationCard>(list: T[], enabled: boolean): OrderedEntry<T>[] => (
  list
    .map((card, originalIndex) => ({
      card,
      originalIndex,
      sortOrder: normalizeSortOrder(card.sortOrder, originalIndex + 1),
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
    entry.card.sortOrder = index + 1
  })
  disabledEntries.forEach((entry, index) => {
    entry.card.sortOrder = index + 1
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
    card.sortOrder = index + 1
  })
  disabledCards.forEach((card, index) => {
    card.sortOrder = index + 1
  })

  list.splice(0, list.length, ...enabledCards, ...disabledCards)
}

export const appendProviderToStatusGroup = <T extends AutomationCard>(list: T[], card: T) => {
  applyNormalizedProviderOrder(list)

  card.sortOrder = list.filter((item) => item.enabled === card.enabled).length + 1
  const enabledCount = countEnabledCards(list)
  const insertIndex = card.enabled ? enabledCount : list.length
  list.splice(insertIndex, 0, card)

  commitProviderOrder(list)
}

export const moveProviderToStatusGroupEnd = <T extends AutomationCard>(
  list: T[],
  card: T,
  enabled: boolean,
) => {
  applyNormalizedProviderOrder(list)

  const currentIndex = list.findIndex((item) => item === card || item.id === card.id)
  if (currentIndex < 0) return false

  const [moved] = list.splice(currentIndex, 1)
  moved.enabled = enabled
  moved.sortOrder = list.filter((item) => item.enabled === enabled).length + 1

  const enabledCount = countEnabledCards(list)
  const insertIndex = enabled ? enabledCount : list.length
  list.splice(insertIndex, 0, moved)

  commitProviderOrder(list)
  return true
}
