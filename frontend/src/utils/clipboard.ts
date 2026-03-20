export const copyWithTextareaFallback = (payload: string) => {
  const textarea = document.createElement('textarea')
  textarea.value = payload
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  textarea.style.pointerEvents = 'none'
  document.body.appendChild(textarea)
  textarea.focus()
  textarea.select()
  const copied = document.execCommand('copy')
  document.body.removeChild(textarea)
  if (!copied) {
    throw new Error('execCommand copy failed')
  }
}

export const writeTextToClipboard = async (payload: string) => {
  const clipboardWriteText = typeof navigator === 'undefined'
    ? undefined
    : navigator.clipboard?.writeText?.bind(navigator.clipboard)
  if (clipboardWriteText != null) {
    await clipboardWriteText(payload)
    return
  }
  copyWithTextareaFallback(payload)
}
