import { getLobeIconSvg, preloadLobeIcons } from '../icons/lobeIconMap'
import { PLATFORM_BRAND_ICONS } from '../icons/platformBrandIcons'

const INLINE_PROVIDER_ICONS: Record<string, string> = {
  ...PLATFORM_BRAND_ICONS,
  opencode: '<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M12 3.25 4.75 7.5v9L12 20.75l7.25-4.25v-9L12 3.25Z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/><path d="m9.25 9-2.5 3 2.5 3M14.75 9l2.5 3-2.5 3M13.25 7.75l-2.5 8.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  others: '<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M6.75 8.25h10.5M6.75 12h10.5M6.75 15.75h10.5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/><path d="M4.75 4.75h14.5v14.5H4.75z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></svg>',
}

const PROVIDER_COLOR_ICON_ALIASES: Record<string, string> = {
  claude: 'claude-color',
  gemini: 'gemini-color',
  deepseek: 'deepseek-color',
  mistral: 'mistral-color',
  meta: 'meta-color',
  cohere: 'cohere-color',
  bedrock: 'bedrock-color',
  azure: 'azure-color',
  together: 'together-color',
  nvidia: 'nvidia-color',
  zhipu: 'zhipu-color',
  volcengine: 'doubao-color',
  minimax: 'minimax-color',
  qwen: 'qwen-color',
  fireworks: 'fireworks-color',
  perplexity: 'perplexity-color',
  baichuan: 'baichuan-color',
  sensenova: 'sensenova-color',
  spark: 'spark-color',
  hunyuan: 'hunyuan-color',
  wenxin: 'wenxin-color',
  gemma: 'gemma-color',
  internlm: 'internlm-color',
  yi: 'yi-color',
  stepfun: 'stepfun-color',
  kimi: 'kimi-color',
}

const PROVIDER_PRIORITY_ICON_KEYS = [
  'claude',
  'openai',
  'opencode',
  'gemini',
  'deepseek',
  'mistral',
  'meta',
  'cohere',
  'grok',
  'groq',
  'bedrock',
  'azure',
  'together',
  'nvidia',
  'zhipu',
  'volcengine',
  'minimax',
  'qwen',
  'fireworks',
  'ollama',
  'openrouter',
  'moonshot',
  'perplexity',
  'baichuan',
  'sensenova',
  'spark',
  'hunyuan',
  'wenxin',
  'gemma',
  'internlm',
  'yi',
  'stepfun',
  'kimi',
] as const

function normalizeIconKey(value: string | null | undefined) {
  return String(value ?? '').trim().toLowerCase()
}

export function resolveProviderDisplayIconKey(iconKey: string | null | undefined) {
  const normalized = normalizeIconKey(iconKey)
  if (!normalized) return ''
  return PROVIDER_COLOR_ICON_ALIASES[normalized] ?? normalized
}

export function collectProviderDisplayIconKeys(iconKeys: readonly string[]) {
  const seen = new Set<string>()

  for (const iconKey of iconKeys) {
    const resolved = resolveProviderDisplayIconKey(iconKey)
    if (!resolved) continue
    seen.add(resolved)
  }

  return Array.from(seen)
}

export function getProviderDisplayIconSvg(iconKey: string | null | undefined) {
  const normalized = normalizeIconKey(iconKey)
  if (!normalized) return ''

  const preferred = resolveProviderDisplayIconKey(normalized)
  return INLINE_PROVIDER_ICONS[preferred] || INLINE_PROVIDER_ICONS[normalized] || getLobeIconSvg(preferred) || getLobeIconSvg(normalized)
}

export async function preloadProviderDisplayIcons(iconKeys: readonly string[]) {
  const displayKeys = collectProviderDisplayIconKeys(iconKeys)
  if (displayKeys.length === 0) return
  await preloadLobeIcons(displayKeys)
}

export function buildProviderIconOptionKeys(iconKeys: readonly string[]) {
  const uniqueKeys = Array.from(new Set(
    iconKeys
      .map((iconKey) => normalizeIconKey(iconKey))
      .filter(Boolean),
  ))

  const remaining = new Set(uniqueKeys)
  const ordered: string[] = []

  for (const iconKey of PROVIDER_PRIORITY_ICON_KEYS) {
    if (!remaining.delete(iconKey)) continue
    ordered.push(iconKey)
  }

  return [
    ...ordered,
    ...Object.keys(INLINE_PROVIDER_ICONS).filter((iconKey) => !ordered.includes(iconKey) && !remaining.has(iconKey)),
    ...Array.from(remaining).sort((left, right) => left.localeCompare(right)),
  ]
}
