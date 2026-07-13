import { Clipboard } from '@wailsio/runtime'

interface WailsRuntimeWindow extends Window {
  _wails?: {
    environment?: unknown
  }
}

function hasWailsRuntime() {
  if (typeof window === 'undefined') {
    return false
  }
  return (window as WailsRuntimeWindow)._wails?.environment != null
}

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
  if (hasWailsRuntime()) {
    try {
      await Clipboard.SetText(payload)
      return
    } catch {
      // 浏览器开发环境中 Wails Runtime 不可用时继续降级。
    }
  }

  const clipboardWriteText = typeof navigator === 'undefined'
    ? undefined
    : navigator.clipboard?.writeText?.bind(navigator.clipboard)
  if (clipboardWriteText != null) {
    try {
      await clipboardWriteText(payload)
      return
    } catch {
      // WebView 可能暴露 Clipboard API，但拒绝当前上下文写入。
    }
  }
  copyWithTextareaFallback(payload)
}
