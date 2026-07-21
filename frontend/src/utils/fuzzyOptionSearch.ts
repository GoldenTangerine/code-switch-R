const SEARCH_SPLIT_RE = /[\/:,|@_.\-\s]+/
const SEARCH_STRIP_RE = /[\/:,|@_.\-\s]+/g

export interface StringOptionSearchEntry {
  option: string
  index: number
  normalizedOption: string
  compactOption: string
  optionTerms: string[]
}

interface StringOptionSearchQuery {
  normalizedQuery: string
  compactQuery: string
  queryTerms: Array<{
    normalized: string
    compact: string
  }>
}

interface CachedStringOptionSearchIndex {
  optionsSnapshot: string[]
  index: StringOptionSearchEntry[]
}

const stringOptionSearchIndexCache = new WeakMap<readonly string[], CachedStringOptionSearchIndex>()

export const normalizeSearchOption = (value: string) => String(value ?? '').trim().toLowerCase()

export const compactSearchOption = (value: string) =>
  normalizeSearchOption(value).replace(SEARCH_STRIP_RE, '')

export const splitSearchTerms = (value: string) =>
  normalizeSearchOption(value)
    .split(SEARCH_SPLIT_RE)
    .map((segment) => segment.trim())
    .filter(Boolean)

const commonPrefixLength = (left: string, right: string) => {
  const limit = Math.min(left.length, right.length)
  let cursor = 0
  while (cursor < limit && left[cursor] === right[cursor]) {
    cursor += 1
  }
  return cursor
}

const isSubsequenceMatch = (source: string, query: string) => {
  if (!query) return true

  let cursor = 0
  for (const char of source) {
    if (char === query[cursor]) {
      cursor += 1
      if (cursor === query.length) {
        return true
      }
    }
  }

  return false
}

function createStringOptionSearchQuery(query: string): StringOptionSearchQuery {
  const normalizedQuery = normalizeSearchOption(query)
  return {
    normalizedQuery,
    compactQuery: compactSearchOption(normalizedQuery),
    queryTerms: splitSearchTerms(normalizedQuery).map((term) => ({
      normalized: term,
      compact: compactSearchOption(term),
    })),
  }
}

function scoreStringOptionEntry(entry: StringOptionSearchEntry, query: StringOptionSearchQuery): number {
  const {
    normalizedOption,
    compactOption,
    optionTerms,
  } = entry
  const {
    normalizedQuery,
    compactQuery,
    queryTerms,
  } = query
  if (!normalizedOption || !normalizedQuery) return 0

  const exactNormalized = normalizedOption === normalizedQuery
  const exactCompact = compactQuery !== '' && compactOption === compactQuery
  const startsWithNormalized = normalizedOption.startsWith(normalizedQuery)
  const startsWithCompact = compactQuery !== '' && compactOption.startsWith(compactQuery)
  const includesNormalized = normalizedOption.includes(normalizedQuery)
  const includesCompact = compactQuery !== '' && compactOption.includes(compactQuery)
  const everyTermMatched = queryTerms.length > 0 && queryTerms.every((term) => {
    return normalizedOption.includes(term.normalized)
      || (term.compact !== '' && compactOption.includes(term.compact))
  })
  const matchedSegmentWeight = queryTerms.reduce((total, term) => {
    if (optionTerms.some((segment) => segment.startsWith(term.normalized))) return total + 1
    if (optionTerms.some((segment) => segment.includes(term.normalized))) return total + 0.4
    return total
  }, 0)
  const subsequenceMatched = compactQuery.length >= 2 && isSubsequenceMatch(compactOption, compactQuery)

  if (
    !exactNormalized
    && !exactCompact
    && !startsWithNormalized
    && !startsWithCompact
    && !includesNormalized
    && !includesCompact
    && !everyTermMatched
    && !subsequenceMatched
  ) {
    return Number.NEGATIVE_INFINITY
  }

  let score = 0
  if (exactNormalized) score += 1400
  if (exactCompact) score += 1320
  if (startsWithNormalized) score += 920
  if (startsWithCompact) score += 860
  if (everyTermMatched) score += 620
  if (includesNormalized) score += 420
  if (includesCompact) score += 360
  if (subsequenceMatched) score += 180

  if (compactQuery) {
    score += commonPrefixLength(compactOption, compactQuery) * 18
    score += Math.max(0, 80 - Math.max(0, compactOption.length - compactQuery.length))
  }

  score += matchedSegmentWeight * 40
  return score
}

export function scoreStringOption(option: string, query: string): number {
  const normalizedOption = normalizeSearchOption(option)
  return scoreStringOptionEntry({
    option,
    index: 0,
    normalizedOption,
    compactOption: compactSearchOption(normalizedOption),
    optionTerms: splitSearchTerms(normalizedOption),
  }, createStringOptionSearchQuery(query))
}

export const dedupeStringOptions = (options: readonly string[]) => {
  const seen = new Set<string>()
  return options.filter((option) => {
    const normalized = String(option ?? '').trim()
    if (!normalized || seen.has(normalized)) return false
    seen.add(normalized)
    return true
  })
}

export function createStringOptionSearchIndex(options: readonly string[]): StringOptionSearchEntry[] {
  return dedupeStringOptions(options.map((option) => String(option ?? '').trim()))
    .map((option, index) => {
      const normalizedOption = normalizeSearchOption(option)
      return {
        option,
        index,
        normalizedOption,
        compactOption: compactSearchOption(normalizedOption),
        optionTerms: splitSearchTerms(normalizedOption),
      }
    })
}

export function getCachedStringOptionSearchIndex(options: readonly string[]): StringOptionSearchEntry[] {
  const cached = stringOptionSearchIndexCache.get(options)
  if (
    cached
    && cached.optionsSnapshot.length === options.length
    && cached.optionsSnapshot.every((option, index) => option === options[index])
  ) {
    return cached.index
  }

  const index = createStringOptionSearchIndex(options)
  stringOptionSearchIndexCache.set(options, {
    optionsSnapshot: Array.from(options),
    index,
  })
  return index
}

export function filterAndSortStringOptionIndex(index: StringOptionSearchEntry[], query: string): string[] {
  const searchQuery = createStringOptionSearchQuery(query)
  if (!searchQuery.normalizedQuery) return index.map((entry) => entry.option)

  return index
    .map((entry) => ({
      entry,
      score: scoreStringOptionEntry(entry, searchQuery),
    }))
    .filter((entry) => Number.isFinite(entry.score))
    .sort((left, right) => {
      if (right.score !== left.score) return right.score - left.score
      return left.entry.index - right.entry.index
    })
    .map(({ entry }) => entry.option)
}

export function filterAndSortStringOptions(options: readonly string[], query: string): string[] {
  return filterAndSortStringOptionIndex(createStringOptionSearchIndex(options), query)
}
