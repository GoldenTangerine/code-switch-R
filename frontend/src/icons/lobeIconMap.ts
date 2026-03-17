import { shallowReactive } from 'vue'
import fallbackIcons from './fallbackLobeIcons'

const globIcons = import.meta.glob('../../node_modules/@lobehub/icons-static-svg/icons/*.svg', {
  import: 'default',
  query: '?raw',
}) as Record<string, () => Promise<string>>

const normalizeIconKey = (value: string | null | undefined) => {
  return String(value ?? '').trim().toLowerCase()
}

const normalizeFallback = (source: Record<string, string>) => {
  return Object.entries(source).reduce<Record<string, string>>((acc, [key, value]) => {
    const name = normalizeIconKey(key)
    if (name) {
      acc[name] = value
    }
    return acc
  }, {})
}

const normalizeIconPath = (path: string) => {
  return normalizeIconKey(
    path
      .split('/')
      .pop()
      ?.replace('.svg', ''),
  )
}

const normalizedFallback = normalizeFallback(fallbackIcons)
const iconLoaders = Object.entries(globIcons).reduce<Record<string, () => Promise<string>>>((acc, [path, loader]) => {
  const name = normalizeIconPath(path)
  if (name) {
    acc[name] = loader
  }
  return acc
}, {})
const iconKeys = Array.from(new Set([
  ...Object.keys(normalizedFallback),
  ...Object.keys(iconLoaders),
])).sort((left, right) => left.localeCompare(right))
const lobeIconMap = shallowReactive<Record<string, string>>(
  Object.fromEntries(iconKeys.map((key) => [key, normalizedFallback[key] ?? ''])),
)
const pendingIconLoads = new Map<string, Promise<string>>()

export const getLobeIconSvg = (name: string) => {
  const key = normalizeIconKey(name)
  if (!key) return ''
  return lobeIconMap[key] ?? ''
}

export const loadLobeIcon = async (name: string) => {
  const key = normalizeIconKey(name)
  if (!key) return ''
  if (lobeIconMap[key]) return lobeIconMap[key]

  const pending = pendingIconLoads.get(key)
  if (pending) {
    return pending
  }

  const loader = iconLoaders[key]
  if (!loader) {
    return normalizedFallback[key] ?? ''
  }

  const task = loader()
    .then((svg) => {
      const normalized = typeof svg === 'string' ? svg : ''
      if (normalized) {
        lobeIconMap[key] = normalized
      }
      return lobeIconMap[key] ?? ''
    })
    .finally(() => {
      pendingIconLoads.delete(key)
    })

  pendingIconLoads.set(key, task)
  return task
}

export const preloadLobeIcons = async (names: string[]) => {
  const keys = Array.from(new Set(
    names
      .map((name) => normalizeIconKey(name))
      .filter(Boolean),
  ))

  if (keys.length === 0) {
    return
  }

  await Promise.all(keys.map((name) => loadLobeIcon(name)))
}

export default lobeIconMap
