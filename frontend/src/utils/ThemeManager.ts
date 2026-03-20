// src/utils/ThemeManager.ts
const THEME_KEY = 'theme'
const THEME_CHANNEL = 'code-switch-theme'
const THEME_CHANGE_EVENT = 'code-switch-theme-change'
let themeChannel: BroadcastChannel | null = null

export type ThemeMode = 'light' | 'dark' | 'systemdefault'
export type ResolvedTheme = 'light' | 'dark'
export type ThemeChangeDetail = {
  mode: ThemeMode
  resolvedTheme: ResolvedTheme
}

const normalizeThemeMode = (value: string | null): ThemeMode => {
  if (value === 'light' || value === 'dark' || value === 'systemdefault') {
    return value
  }
  return 'systemdefault'
}

const ensureThemeChannel = () => {
  if (themeChannel || typeof BroadcastChannel === 'undefined') return
  themeChannel = new BroadcastChannel(THEME_CHANNEL)
  themeChannel.addEventListener('message', (event) => {
    const payload = event?.data
    const next = normalizeThemeMode(typeof payload === 'string' ? payload : payload?.mode ?? null)
    const current = getCurrentTheme()
    if (next === current) {
      applyTheme(next)
      return
    }
    localStorage.setItem(THEME_KEY, next)
    applyTheme(next)
  })
}

export function getResolvedTheme(mode: ThemeMode = getCurrentTheme()): ResolvedTheme {
  if (mode === 'systemdefault') {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }
  return mode
}

const emitThemeChange = (detail: ThemeChangeDetail) => {
  window.dispatchEvent(new CustomEvent<ThemeChangeDetail>(THEME_CHANGE_EVENT, { detail }))
}

export function applyTheme(mode: ThemeMode) {
  const resolvedTheme = getResolvedTheme(mode)

  document.documentElement.classList.remove('dark', 'light')
  document.documentElement.classList.add(resolvedTheme)
  emitThemeChange({ mode, resolvedTheme })
}

export function initTheme() {
  const savedTheme = normalizeThemeMode(localStorage.getItem(THEME_KEY))
  applyTheme(savedTheme)
  ensureThemeChannel()
  themeChannel?.postMessage({ mode: savedTheme })

  // 监听系统变化，仅在 systemdefault 时响应
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    const current = getCurrentTheme()
    if (current === 'systemdefault') {
      applyTheme('systemdefault')
    }
  })

  // 同步其他窗口主题变化（托盘弹窗等）
  window.addEventListener('storage', (event) => {
    if (event.key && event.key !== THEME_KEY) return
    applyTheme(normalizeThemeMode(event.newValue))
  })
}

export function setTheme(mode: ThemeMode) {
  localStorage.setItem(THEME_KEY, mode)
  applyTheme(mode)
  ensureThemeChannel()
  themeChannel?.postMessage({ mode })
}

export function getCurrentTheme(): ThemeMode {
  return normalizeThemeMode(localStorage.getItem(THEME_KEY))
}

export function onThemeChange(listener: (detail: ThemeChangeDetail) => void) {
  const handleThemeChange = (event: Event) => {
    const detail = (event as CustomEvent<ThemeChangeDetail>).detail
    if (!detail) return
    listener(detail)
  }

  window.addEventListener(THEME_CHANGE_EVENT, handleThemeChange)
  return () => {
    window.removeEventListener(THEME_CHANGE_EVENT, handleThemeChange)
  }
}
