import { getLobeIconSvg, preloadLobeIcons } from '../icons/lobeIconMap'

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
  return getLobeIconSvg(preferred) || getLobeIconSvg(normalized)
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
    ...Array.from(remaining).sort((left, right) => left.localeCompare(right)),
  ]
}
