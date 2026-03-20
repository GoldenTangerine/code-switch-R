export type ProviderErrorDetail = {
  statusCode?: number
  providerMessage: string
  errorCode: string
  errorType: string
  errorStatus: string
  errorParam: string
  rawPayload: string
  summary: string
  semanticTag: string
  copyText: string
}

type JsonRecord = Record<string, unknown>

const upstreamStatusPattern = /\bupstream status\s+(\d{3})\b/i
const inlineStatusPattern = /\bstatus(?:=|:|\s)(\d{3})\b/i
const httpStatusPattern = /\bHTTP\s+(\d{3})\b/i
const payloadTailMarkers = [' | 耗时:', ' | 重试 ', ' | Model:', ' | mode:', ' | hint:']
const metadataSignalPatterns = [
  /\burl=https?:\/\/[^\s]+/i,
  /\bcontent_type=[^;\s]+/i,
  /\bcharset=[^;\s]+/i,
  /\bmethod=(?:GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\b/i,
]
const metadataErrorSignalPatterns = [
  /\berror\b/i,
  /\bmessage\b/i,
  /\bdetail\b/i,
  /\breason\b/i,
  /\bno_available_providers\b/i,
  /\bno available providers\b/i,
  /\brate limit\b/i,
  /\boverloaded\b/i,
  /\bunauthorized\b/i,
  /\bforbidden\b/i,
  /\bquota\b/i,
  /\btimeout\b/i,
  /\binternal\b/i,
  /\bservice unavailable\b/i,
]

const normalizeWhitespace = (value: string) => value.replace(/\s+/g, ' ').trim()

const toNonEmptyString = (value: unknown): string => {
  if (typeof value === 'string') {
    return value.trim()
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  return ''
}

const getPathValue = (source: unknown, path: string): unknown => {
  if (source == null || typeof source !== 'object') {
    return undefined
  }

  const segments = path.split('.')
  let cursor: unknown = source
  for (const segment of segments) {
    if (cursor == null || typeof cursor !== 'object' || Array.isArray(cursor)) {
      return undefined
    }
    cursor = (cursor as JsonRecord)[segment]
  }
  return cursor
}

const firstStringFromPaths = (source: unknown, paths: string[]) => {
  for (const path of paths) {
    const value = toNonEmptyString(getPathValue(source, path))
    if (value) {
      return value
    }
  }
  return ''
}

const firstNumberFromPaths = (source: unknown, paths: string[]) => {
  for (const path of paths) {
    const value = getPathValue(source, path)
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value
    }
    if (typeof value === 'string') {
      const parsed = Number.parseInt(value, 10)
      if (Number.isFinite(parsed)) {
        return parsed
      }
    }
  }
  return undefined
}

const extractBalancedJsonSegment = (input: string): string => {
  const trimmed = input.trim()
  if (!trimmed) {
    return ''
  }

  const firstChar = trimmed[0]
  const closingChar = firstChar === '{' ? '}' : firstChar === '[' ? ']' : ''
  if (!closingChar) {
    return ''
  }

  let depth = 0
  let inString = false
  let escaped = false
  for (let index = 0; index < trimmed.length; index += 1) {
    const char = trimmed[index]
    if (inString) {
      if (escaped) {
        escaped = false
        continue
      }
      if (char === '\\') {
        escaped = true
        continue
      }
      if (char === '"') {
        inString = false
      }
      continue
    }

    if (char === '"') {
      inString = true
      continue
    }

    if (char === firstChar) {
      depth += 1
      continue
    }

    if (char === closingChar) {
      depth -= 1
      if (depth === 0) {
        return trimmed.slice(0, index + 1)
      }
    }
  }

  return ''
}

const trimPayloadTail = (value: string) => {
  let result = value.trim()
  for (const marker of payloadTailMarkers) {
    const markerIndex = result.indexOf(marker)
    if (markerIndex >= 0) {
      result = result.slice(0, markerIndex).trim()
    }
  }
  return result
}

const extractFirstEmbeddedJsonSegment = (input: string): string => {
  const trimmed = input.trim()
  if (!trimmed) {
    return ''
  }

  const firstJsonStart = trimmed.search(/[{\[]/)
  if (firstJsonStart < 0) {
    return ''
  }

  return extractBalancedJsonSegment(trimmed.slice(firstJsonStart))
}

const extractStructuredPayloadCandidate = (candidate: string) => {
  const trimmed = candidate.trim()
  if (!trimmed) {
    return ''
  }

  const balancedSegment = extractBalancedJsonSegment(trimmed)
  if (balancedSegment) {
    return balancedSegment
  }

  const embeddedSegment = extractFirstEmbeddedJsonSegment(trimmed)
  if (!embeddedSegment) {
    return ''
  }

  return tryParseJson(embeddedSegment) != null ? embeddedSegment : ''
}

const extractPayloadText = (message: string, statusMatch: RegExpMatchArray | null) => {
  if (statusMatch == null || statusMatch.index == null) {
    return ''
  }

  let remainder = message.slice(statusMatch.index + statusMatch[0].length).trim()
  if (remainder.startsWith(':')) {
    remainder = remainder.slice(1).trim()
  }
  if (!remainder) {
    return ''
  }

  const newlinePayload = remainder.includes('\n')
    ? remainder.slice(remainder.indexOf('\n') + 1).trim()
    : ''

  for (const candidate of [remainder, newlinePayload]) {
    if (!candidate) {
      continue
    }

    const jsonSegment = extractStructuredPayloadCandidate(candidate)
    if (jsonSegment) {
      return jsonSegment
    }
  }

  return trimPayloadTail(newlinePayload || remainder)
}

const tryParseJson = (value: string): unknown => {
  if (!value) {
    return null
  }
  try {
    return JSON.parse(value)
  } catch {
    return null
  }
}

const includesAny = (source: string, candidates: string[]) => {
  for (const candidate of candidates) {
    if (source.includes(candidate)) {
      return true
    }
  }
  return false
}

const classifyProviderError = (detail: Pick<ProviderErrorDetail, 'statusCode' | 'providerMessage' | 'errorCode' | 'errorType' | 'errorStatus'>) => {
  const normalized = [
    detail.providerMessage,
    detail.errorCode,
    detail.errorType,
    detail.errorStatus,
    detail.statusCode ? String(detail.statusCode) : '',
  ]
    .join(' ')
    .toLowerCase()

  if (
    includesAny(normalized, [
      'overloaded',
      'overload',
      'server overloaded',
      'service unavailable',
      'model unavailable',
      'temporarily unavailable',
      'currently unavailable',
      '负载已经达到上限',
      '负载达到上限',
      '模型负载过高',
      '模型繁忙',
      '服务繁忙',
    ])
  ) {
    return '模型负载过高'
  }

  if (
    normalized.includes('rate limit') ||
    normalized.includes('too many requests') ||
    normalized.includes('resource exhausted') ||
    normalized.includes('quota')
  ) {
    return '限流 / 配额'
  }

  if (
    normalized.includes('unauthorized') ||
    normalized.includes('forbidden') ||
    normalized.includes('authentication') ||
    normalized.includes('permission') ||
    normalized.includes('invalid api key')
  ) {
    return '鉴权失败'
  }

  if (
    normalized.includes('timeout') ||
    normalized.includes('timed out') ||
    normalized.includes('deadline exceeded')
  ) {
    return '请求超时'
  }

  if (detail.statusCode != null && detail.statusCode >= 500) {
    return '接口 5xx'
  }

  if (detail.statusCode != null && detail.statusCode >= 400) {
    return '接口 4xx'
  }

  return ''
}

const extractProviderMessage = (payload: unknown) => {
  const directError = getPathValue(payload, 'error')
  if (typeof directError === 'string' && directError.trim()) {
    return directError.trim()
  }

  const message = firstStringFromPaths(payload, [
    'error.message',
    'message',
    'detail',
    'details',
    'error.detail',
    'error.details',
    'error.reason',
  ])
  return normalizeWhitespace(message)
}

const buildCopyText = (rawPayload: string, fallback: string) => {
  const parsed = tryParseJson(rawPayload)
  if (parsed != null) {
    return JSON.stringify(parsed, null, 2)
  }
  if (rawPayload) {
    return rawPayload
  }
  return fallback
}

const hasValidEmbeddedJsonPayload = (value: string) => {
  const embeddedSegment = extractFirstEmbeddedJsonSegment(value)
  if (!embeddedSegment) {
    return false
  }
  return tryParseJson(embeddedSegment) != null
}

export const isLikelyProviderRequestMetadata = (value: string) => {
  const normalized = String(value ?? '').trim()
  if (!normalized) {
    return false
  }

  if (hasValidEmbeddedJsonPayload(normalized)) {
    return false
  }

  const metadataSignalCount = metadataSignalPatterns.reduce((count, pattern) => {
    return count + (pattern.test(normalized) ? 1 : 0)
  }, 0)

  if (metadataSignalCount < 2) {
    return false
  }

  return !metadataErrorSignalPatterns.some((pattern) => pattern.test(normalized))
}

export const hasMeaningfulProviderErrorPayload = (
  rawPayload: string | null | undefined,
  parsedDetail?: ProviderErrorDetail | null,
) => {
  const normalized = String(rawPayload ?? '').trim()
  if (!normalized) {
    return false
  }

  if (isLikelyProviderRequestMetadata(normalized)) {
    return false
  }

  return Boolean(
    parsedDetail?.providerMessage ||
    parsedDetail?.errorCode ||
    parsedDetail?.errorType ||
    parsedDetail?.errorStatus ||
    parsedDetail?.errorParam ||
    normalized,
  )
}

export const parseProviderErrorFromConsoleMessage = (message: string): ProviderErrorDetail | null => {
  const normalizedMessage = String(message ?? '').trim()
  if (!normalizedMessage) {
    return null
  }

  const upstreamMatch = normalizedMessage.match(upstreamStatusPattern)
  const inlineStatusMatch = normalizedMessage.match(inlineStatusPattern)
  const statusMatch = upstreamMatch ?? inlineStatusMatch
  const httpStatusMatch = normalizedMessage.match(httpStatusPattern)
  const upstreamStatusCode = statusMatch ? Number.parseInt(statusMatch[1], 10) : undefined
  const httpStatusCode = httpStatusMatch ? Number.parseInt(httpStatusMatch[1], 10) : undefined
  const rawPayload = extractPayloadText(normalizedMessage, statusMatch)
  const parsedPayload = tryParseJson(rawPayload)

  const providerMessage = extractProviderMessage(parsedPayload)
  const errorCode = firstStringFromPaths(parsedPayload, ['error.code', 'code'])
  const errorType = firstStringFromPaths(parsedPayload, ['error.type', 'type'])
  const errorStatus = firstStringFromPaths(parsedPayload, ['error.status', 'status'])
  const errorParam = firstStringFromPaths(parsedPayload, ['error.param', 'param'])

  const statusCode = upstreamStatusCode ?? firstNumberFromPaths(parsedPayload, ['error.code', 'code']) ?? httpStatusCode
  if (statusCode != null && statusCode < 400) {
    return null
  }
  if (statusCode == null && !rawPayload && !providerMessage) {
    return null
  }

  const fallbackSummary = rawPayload || (statusCode != null ? `上游返回 HTTP ${statusCode}` : '')
  const summary = providerMessage || normalizeWhitespace(fallbackSummary)
  const semanticTag = classifyProviderError({
    statusCode,
    providerMessage,
    errorCode,
    errorType,
    errorStatus,
  })

  return {
    statusCode,
    providerMessage,
    errorCode,
    errorType,
    errorStatus,
    errorParam,
    rawPayload,
    summary,
    semanticTag,
    copyText: buildCopyText(rawPayload, summary),
  }
}
