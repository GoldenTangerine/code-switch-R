const SEARCH_SPLIT_RE = /[\/:,|@_.\-\s]+/
const SEARCH_STRIP_RE = /[\/:,|@_.\-\s]+/g

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

export const scoreStringOption = (option: string, query: string) => {
  const normalizedOption = normalizeSearchOption(option)
  const normalizedQuery = normalizeSearchOption(query)
  if (!normalizedOption || !normalizedQuery) return 0

  const compactOption = compactSearchOption(option)
  const compactQuery = compactSearchOption(query)
  const optionTerms = splitSearchTerms(option)
  const queryTerms = splitSearchTerms(query)

  const exactNormalized = normalizedOption === normalizedQuery
  const exactCompact = compactQuery !== '' && compactOption === compactQuery
  const startsWithNormalized = normalizedOption.startsWith(normalizedQuery)
  const startsWithCompact = compactQuery !== '' && compactOption.startsWith(compactQuery)
  const includesNormalized = normalizedOption.includes(normalizedQuery)
  const includesCompact = compactQuery !== '' && compactOption.includes(compactQuery)
  const everyTermMatched = queryTerms.length > 0 && queryTerms.every((term) => {
    const compactTerm = compactSearchOption(term)
    return normalizedOption.includes(term) || (compactTerm !== '' && compactOption.includes(compactTerm))
  })
  const matchedSegmentWeight = queryTerms.reduce((total, term) => {
    if (optionTerms.some((segment) => segment.startsWith(term))) return total + 1
    if (optionTerms.some((segment) => segment.includes(term))) return total + 0.4
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

export const dedupeStringOptions = (options: string[]) => {
  const seen = new Set<string>()
  return options.filter((option) => {
    const normalized = String(option ?? '').trim()
    if (!normalized || seen.has(normalized)) return false
    seen.add(normalized)
    return true
  })
}

export const filterAndSortStringOptions = (options: string[], query: string) => {
  const normalizedOptions = dedupeStringOptions(options.map((option) => String(option ?? '').trim()))
  const normalizedQuery = normalizeSearchOption(query)
  if (!normalizedQuery) return normalizedOptions

  return normalizedOptions
    .map((option, index) => ({
      option,
      index,
      score: scoreStringOption(option, normalizedQuery),
    }))
    .filter((entry) => Number.isFinite(entry.score))
    .sort((left, right) => {
      if (right.score !== left.score) return right.score - left.score
      return left.index - right.index
    })
    .map((entry) => entry.option)
}
