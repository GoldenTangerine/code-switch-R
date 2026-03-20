import type { RequestLog } from '../../../services/logs'
import {
  isLikelyProviderRequestMetadata,
  parseProviderErrorFromConsoleMessage,
  type ProviderErrorDetail,
} from '../../../utils/providerError'

export type ConsoleLogLike = {
  timestamp?: string | Date | number | null
  level?: string | null
  message?: string | null
}

export type ConsoleProviderErrorCandidate = {
  timestamp: string
  level: string
  message: string
  providerError: ProviderErrorDetail
}

const JSON_CONTINUATION_MAX_GAP_MS = 1500
const JSON_CONTINUATION_MAX_LINES = 12
const JSON_CONTINUATION_MAX_CHARS = 16 * 1024
const logPrefixPattern = /^\[[A-Z]+\]/

const hasStructuredProviderErrorFields = (detail: ProviderErrorDetail | null | undefined) => {
  if (!detail) {
    return false
  }

  return Boolean(
    detail.providerMessage ||
    detail.errorCode ||
    detail.errorType ||
    detail.errorStatus ||
    detail.errorParam,
  )
}

const normalizeLooseText = (value: string) =>
  String(value ?? '')
    .trim()
    .toLowerCase()
    .replace(/[\s_\-:/|()[\]{}]+/g, '')

const toTimestamp = (value: string | Date | number | null | undefined) => {
  if (value instanceof Date) {
    const timestamp = value.getTime()
    return Number.isFinite(timestamp) ? timestamp : Number.NaN
  }
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : Number.NaN
  }
  if (!value) return Number.NaN
  const normalized = value.includes('T') ? value : value.replace(' ', 'T')
  const timestamp = new Date(normalized).getTime()
  return Number.isFinite(timestamp) ? timestamp : Number.NaN
}

const normalizeConsoleTimestamp = (value: string | Date | number | null | undefined) => {
  if (value instanceof Date) {
    return value.toISOString()
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return new Date(value).toISOString()
  }
  return String(value ?? '')
}

const isLikelyJsonBlockStartLine = (value: string) => {
  const normalized = String(value ?? '').trim()
  if (!normalized) {
    return false
  }
  return normalized.startsWith('{') || normalized.startsWith('[')
}

const isLikelyLogBoundary = (value: string) => {
  const normalized = String(value ?? '').trim()
  if (!normalized) {
    return true
  }
  return logPrefixPattern.test(normalized)
}

const shouldAttemptContinuationMerge = (detail: ProviderErrorDetail) => {
  const looksMetadataOnly = isLikelyProviderRequestMetadata(detail.rawPayload)
  const hasStructuredFields = hasStructuredProviderErrorFields(detail)
  return looksMetadataOnly || !hasStructuredFields
}

const tryMergeStructuredContinuation = (
  logs: ConsoleLogLike[],
  startIndex: number,
  baseMessage: string,
  baseDetail: ProviderErrorDetail,
) => {
  if (!shouldAttemptContinuationMerge(baseDetail)) {
    return null
  }

  let continuationBuffer = ''
  let lastConsumedIndex = startIndex
  const baseTimestamp = toTimestamp(logs[startIndex]?.timestamp)

  for (let index = startIndex + 1; index < logs.length && index <= startIndex + JSON_CONTINUATION_MAX_LINES; index += 1) {
    const nextMessage = String(logs[index]?.message ?? '').trim()
    if (!nextMessage) {
      break
    }

    const nextTimestamp = toTimestamp(logs[index]?.timestamp)
    if (
      Number.isFinite(baseTimestamp) &&
      Number.isFinite(nextTimestamp) &&
      Math.abs(nextTimestamp - baseTimestamp) > JSON_CONTINUATION_MAX_GAP_MS
    ) {
      break
    }

    if (index === startIndex + 1) {
      if (!isLikelyJsonBlockStartLine(nextMessage)) {
        break
      }
    } else if (isLikelyLogBoundary(nextMessage)) {
      break
    }

    continuationBuffer = continuationBuffer ? `${continuationBuffer}\n${nextMessage}` : nextMessage
    if (continuationBuffer.length > JSON_CONTINUATION_MAX_CHARS) {
      break
    }

    const mergedMessage = `${baseMessage}\n${continuationBuffer}`
    const mergedDetail = parseProviderErrorFromConsoleMessage(mergedMessage)
    if (!mergedDetail) {
      lastConsumedIndex = index
      continue
    }

    const mergedLooksMetadata = isLikelyProviderRequestMetadata(mergedDetail.rawPayload)
    const mergedHasStructuredFields = hasStructuredProviderErrorFields(mergedDetail)
    lastConsumedIndex = index

    if (mergedHasStructuredFields && !mergedLooksMetadata) {
      return {
        message: mergedMessage,
        providerError: mergedDetail,
        lastConsumedIndex,
      }
    }
  }

  return null
}

export const buildConsoleProviderErrorCandidates = (logs: ConsoleLogLike[]): ConsoleProviderErrorCandidate[] => {
  const candidates: ConsoleProviderErrorCandidate[] = []

  for (let index = 0; index < logs.length; index += 1) {
    const baseMessage = String(logs[index]?.message ?? '').trim()
    if (!baseMessage) {
      continue
    }

    const baseDetail = parseProviderErrorFromConsoleMessage(baseMessage)
    if (!baseDetail) {
      continue
    }

    const merged = tryMergeStructuredContinuation(logs, index, baseMessage, baseDetail)
    const finalMessage = merged?.message ?? baseMessage
    const finalDetail = merged?.providerError ?? baseDetail

    candidates.push({
      timestamp: normalizeConsoleTimestamp(logs[index]?.timestamp),
      level: String(logs[index]?.level ?? ''),
      message: finalMessage,
      providerError: finalDetail,
    })

    if (merged) {
      index = merged.lastConsumedIndex
    }
  }

  return candidates
}

export const getConsoleCandidateQualityScore = (detail: ProviderErrorDetail | null | undefined) => {
  if (!detail) {
    return Number.NEGATIVE_INFINITY
  }

  let score = 0
  if (detail.providerMessage) score += 4
  if (detail.errorCode) score += 1
  if (detail.errorType) score += 1
  if (detail.errorStatus) score += 1
  if (detail.errorParam) score += 1
  if (detail.statusCode != null) score += 1

  const normalizedPayload = detail.rawPayload.trim()
  if (normalizedPayload) {
    score += isLikelyProviderRequestMetadata(normalizedPayload) ? -4 : 2
  }

  return score
}

const displayModel = (log: RequestLog) => log.requested_model || log.model || log.response_model || '-'

const buildProviderTerms = (log: RequestLog, providerHints: string[]) => {
  const providerTerms = new Set<string>()
  ;[
    ...providerHints,
    log.provider,
    log.provider_id,
  ].forEach((value) => {
    const normalized = normalizeLooseText(String(value ?? ''))
    if (normalized.length >= 2) {
      providerTerms.add(normalized)
    }
  })
  return [...providerTerms]
}

const buildModelTerms = (log: RequestLog) => {
  const model = displayModel(log)
  const terms = new Set<string>()
  const normalizedFull = normalizeLooseText(model)
  if (normalizedFull.length >= 3 && normalizedFull !== '-') {
    terms.add(normalizedFull)
  }

  model
    .split(/[\/:,|@_\-\s]+/)
    .map((segment) => normalizeLooseText(segment))
    .filter((segment) => segment.length >= 3)
    .forEach((segment) => terms.add(segment))

  return [...terms]
}

export const matchConsoleProviderCandidate = (
  log: RequestLog,
  candidates: ConsoleProviderErrorCandidate[],
  providerHints: string[] = [],
) => {
  const requestTimestamp = toTimestamp(log.created_at)
  const providerTerms = buildProviderTerms(log, providerHints)
  const modelTerms = buildModelTerms(log)
  const hasValidRequestTimestamp = Number.isFinite(requestTimestamp)

  let bestIndex = -1
  let bestScore = -1

  candidates.forEach((candidate, index) => {
    const messageLoose = normalizeLooseText(candidate.message)
    const providerMatched = providerTerms.some((term) => messageLoose.includes(term))
    if (!providerMatched) {
      return
    }

    const candidateTimestamp = toTimestamp(candidate.timestamp)
    const hasValidCandidateTimestamp = Number.isFinite(candidateTimestamp)
    const deltaMs = Math.abs(candidateTimestamp - requestTimestamp)
    if (hasValidRequestTimestamp && hasValidCandidateTimestamp && deltaMs > 15 * 60 * 1000) {
      return
    }

    const statusMatched = candidate.providerError.statusCode === log.http_code
    const modelMatched = modelTerms.some((term) => messageLoose.includes(term))
    const hasTightTimeProximity =
      hasValidRequestTimestamp &&
      hasValidCandidateTimestamp &&
      deltaMs <= 45 * 1000
    const canUseWithoutTime = statusMatched && modelMatched

    if (!hasValidRequestTimestamp || !hasValidCandidateTimestamp) {
      if (!canUseWithoutTime) {
        return
      }
    } else if (!statusMatched && !modelMatched && !hasTightTimeProximity) {
      return
    }

    let score = 5
    if (statusMatched) score += 4
    if (modelMatched) score += 2
    if (candidate.level.toUpperCase().includes('ERROR')) score += 1

    if (hasValidRequestTimestamp && hasValidCandidateTimestamp) {
      if (deltaMs <= 45 * 1000) score += 5
      else if (deltaMs <= 60 * 1000) score += 4
      else if (deltaMs <= 3 * 60 * 1000) score += 3
      else if (deltaMs <= 5 * 60 * 1000) score += 2
      else score += 1
    } else {
      score -= 2
    }

    score += getConsoleCandidateQualityScore(candidate.providerError)

    if (score > bestScore) {
      bestScore = score
      bestIndex = index
    }
  })

  if (bestIndex < 0 || bestScore < 10) {
    return null
  }

  return {
    index: bestIndex,
    candidate: candidates[bestIndex],
  }
}
