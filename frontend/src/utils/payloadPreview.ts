export type PayloadPreviewOptions = {
  formatMaxChars?: number
  highlightMaxChars?: number
}

export type PayloadPreview = {
  rawText: string
  renderedText: string
  html: string
  isJson: boolean
  isFormatted: boolean
  highlightApplied: boolean
  formatSkippedLarge: boolean
}

export const DEFAULT_PAYLOAD_JSON_FORMAT_MAX_CHARS = 256 * 1024
export const DEFAULT_PAYLOAD_JSON_HIGHLIGHT_MAX_CHARS = 128 * 1024

const jsonTokenPattern =
  /("(?:\\u[\da-fA-F]{4}|\\[^u]|[^\\"])*"(?=\s*:))|("(?:\\u[\da-fA-F]{4}|\\[^u]|[^\\"])*")|\b(?:true|false|null)\b|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/g

export const escapeHtml = (value: string) =>
  value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')

export const highlightJsonPayload = (jsonText: string) => {
  let html = ''
  let lastIndex = 0
  jsonTokenPattern.lastIndex = 0

  let matched: RegExpExecArray | null = jsonTokenPattern.exec(jsonText)
  while (matched) {
    const token = matched[0]
    const offset = matched.index
    html += escapeHtml(jsonText.slice(lastIndex, offset))

    const isKeyToken = Boolean(matched[1])
    let tokenClass = 'json-number'
    if (isKeyToken) {
      tokenClass = 'json-key'
    } else if (token.startsWith('"')) {
      tokenClass = 'json-string'
    } else if (token === 'true' || token === 'false') {
      tokenClass = 'json-boolean'
    } else if (token === 'null') {
      tokenClass = 'json-null'
    }

    html += `<span class="json-token ${tokenClass}">${escapeHtml(token)}</span>`
    lastIndex = offset + token.length
    matched = jsonTokenPattern.exec(jsonText)
  }

  html += escapeHtml(jsonText.slice(lastIndex))
  return html
}

const buildRawPreview = (rawText: string, formatSkippedLarge: boolean): PayloadPreview => ({
  rawText,
  renderedText: rawText,
  html: escapeHtml(rawText),
  isJson: false,
  isFormatted: false,
  highlightApplied: false,
  formatSkippedLarge,
})

export const buildPayloadPreview = (
  rawPayload: string | null | undefined,
  options: PayloadPreviewOptions = {},
): PayloadPreview => {
  const rawText = typeof rawPayload === 'string' ? rawPayload : ''
  if (!rawText) {
    return buildRawPreview('', false)
  }

  const formatMaxChars = Number.isFinite(options.formatMaxChars)
    ? Math.max(0, Math.floor(options.formatMaxChars ?? 0))
    : DEFAULT_PAYLOAD_JSON_FORMAT_MAX_CHARS
  const highlightMaxChars = Number.isFinite(options.highlightMaxChars)
    ? Math.max(0, Math.floor(options.highlightMaxChars ?? 0))
    : DEFAULT_PAYLOAD_JSON_HIGHLIGHT_MAX_CHARS

  if (rawText.length > formatMaxChars) {
    return buildRawPreview(rawText, true)
  }

  try {
    const parsed = JSON.parse(rawText)
    const formattedText = JSON.stringify(parsed, null, 2)
    const shouldHighlight = formattedText.length <= highlightMaxChars
    return {
      rawText,
      renderedText: formattedText,
      html: shouldHighlight ? highlightJsonPayload(formattedText) : escapeHtml(formattedText),
      isJson: true,
      isFormatted: true,
      highlightApplied: shouldHighlight,
      formatSkippedLarge: false,
    }
  } catch {
    return buildRawPreview(rawText, false)
  }
}
