import { Clipboard } from '@wailsio/runtime'
import { extractErrorMessage } from './error'

interface WailsRuntimeWindow extends Window {
  _wails?: {
    environment?: {
      Arch?: string
      OS?: string
    }
  }
}

function hasWailsRuntime() {
  if (typeof window === 'undefined') {
    return false
  }
  const environment = (window as WailsRuntimeWindow)._wails?.environment
  return environment != null && environment.Arch !== 'browser'
}

export const copyWithTextareaFallback = (payload: string) => {
  if (typeof document === 'undefined' || document.body == null) {
    throw new Error('clipboard fallback unavailable')
  }

  const textarea = document.createElement('textarea')
  textarea.value = payload
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  textarea.style.pointerEvents = 'none'
  document.body.appendChild(textarea)
  try {
    textarea.focus()
    textarea.select()
    const copied = document.execCommand('copy')
    if (!copied) {
      throw new Error('execCommand copy failed')
    }
  } finally {
    document.body.removeChild(textarea)
  }
}

export const writeTextToClipboard = async (payload: string) => {
  let lastError: unknown

  if (hasWailsRuntime()) {
    try {
      await Clipboard.SetText(payload)
      return
    } catch (error) {
      lastError = error
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
    } catch (error) {
      lastError = error
      // WebView 可能暴露 Clipboard API，但拒绝当前上下文写入。
    }
  }

  try {
    copyWithTextareaFallback(payload)
  } catch (fallbackError) {
    if (lastError == null) {
      throw fallbackError
    }
    throw new Error(
      `${extractErrorMessage(lastError)}; fallback: ${extractErrorMessage(fallbackError)}`,
    )
  }
}
