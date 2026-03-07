import { describe, expect, it } from 'vitest'
import { buildPayloadPreview, escapeHtml, highlightJsonPayload } from './payloadPreview'

describe('payloadPreview', () => {
  it('escapes html for plain text payload', () => {
    const preview = buildPayloadPreview('<script>alert(1)</script>')
    expect(preview.isJson).toBe(false)
    expect(preview.renderedText).toBe('<script>alert(1)</script>')
    expect(preview.html).toBe('&lt;script&gt;alert(1)&lt;/script&gt;')
  })

  it('formats and highlights json payload', () => {
    const preview = buildPayloadPreview('{"name":"coder","ok":true,"count":3}')
    expect(preview.isJson).toBe(true)
    expect(preview.isFormatted).toBe(true)
    expect(preview.renderedText).toContain('\n')
    expect(preview.html).toContain('json-token json-key')
    expect(preview.html).toContain('json-token json-string')
    expect(preview.html).toContain('json-token json-boolean')
    expect(preview.html).toContain('json-token json-number')
  })

  it('skips formatting when payload is too large', () => {
    const raw = '{"a":"' + 'x'.repeat(40) + '"}'
    const preview = buildPayloadPreview(raw, { formatMaxChars: 16, highlightMaxChars: 16 })
    expect(preview.isJson).toBe(false)
    expect(preview.isFormatted).toBe(false)
    expect(preview.formatSkippedLarge).toBe(true)
    expect(preview.renderedText).toBe(raw)
    expect(preview.html).toBe(escapeHtml(raw))
  })

  it('keeps formatted json but skips highlighting when formatted content is large', () => {
    const raw = '{"a":"' + 'x'.repeat(80) + '"}'
    const preview = buildPayloadPreview(raw, { formatMaxChars: 512, highlightMaxChars: 20 })
    expect(preview.isJson).toBe(true)
    expect(preview.isFormatted).toBe(true)
    expect(preview.highlightApplied).toBe(false)
    expect(preview.html).toBe(escapeHtml(preview.renderedText))
  })

  it('highlights primitive tokens correctly', () => {
    const highlighted = highlightJsonPayload('{"n":1,"b":false,"z":null}')
    expect(highlighted).toContain('json-token json-key')
    expect(highlighted).toContain('json-token json-number')
    expect(highlighted).toContain('json-token json-boolean')
    expect(highlighted).toContain('json-token json-null')
  })
})
