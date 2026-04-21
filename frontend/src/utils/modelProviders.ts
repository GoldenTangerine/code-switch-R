export type KnownModelProviderKey =
  | 'anthropic'
  | 'openai'
  | 'vertex'
  | 'deepseek'
  | 'mistral'
  | 'meta'
  | 'cohere'
  | 'xai'
  | 'groq'
  | 'bedrock'
  | 'azure'
  | 'together'
  | 'nvidia'
  | 'zhipuai'
  | 'volcengine'
  | 'minimax'
  | 'qwen'
  | 'fireworks'
  | 'ollama'
  | 'openrouter'
  | 'moonshot'
  | 'perplexity'
  | 'baichuan'
  | 'sensenova'
  | 'spark'
  | 'hunyuan'
  | 'wenxin'
  | 'gemma'
  | 'internlm'
  | 'yi'
  | 'stepfun'
  | 'kimi'

export type ModelProviderKey = KnownModelProviderKey | 'unknown'
export type ModelProviderFilterKey = 'all' | ModelProviderKey

export interface ModelProviderMeta {
  key: KnownModelProviderKey
  label: string
  iconKey: string
  litellmProvider?: string
  explicitPrefixes?: readonly string[]
  familyPrefixes?: readonly string[]
}

export interface ModelProviderCandidate {
  model: string
  ownerBy?: string | null | undefined
}

export interface ModelProviderTab {
  key: ModelProviderFilterKey
  label: string
  count: number
  iconKey?: string
}

interface ProviderPrefixRule {
  prefix: string
  key: KnownModelProviderKey
}

const MODEL_PROVIDER_LIST: readonly ModelProviderMeta[] = [
  {
    key: 'anthropic',
    label: 'Anthropic',
    iconKey: 'claude',
    litellmProvider: 'anthropic',
    explicitPrefixes: ['anthropic'],
    familyPrefixes: ['claude', 'sonnet', 'haiku', 'opus'],
  },
  {
    key: 'openai',
    label: 'OpenAI',
    iconKey: 'openai',
    litellmProvider: 'openai',
    explicitPrefixes: ['openai'],
    familyPrefixes: ['chatgpt', 'gpt', 'o1', 'o3', 'o4', 'text-embedding', 'whisper'],
  },
  {
    key: 'vertex',
    label: 'Vertex',
    iconKey: 'gemini',
    litellmProvider: 'vertex_ai-language-models',
    explicitPrefixes: ['vertex_ai-language-models', 'vertex_ai', 'vertex'],
    familyPrefixes: ['gemini', 'google'],
  },
  {
    key: 'deepseek',
    label: 'DeepSeek',
    iconKey: 'deepseek',
    litellmProvider: 'deepseek',
    explicitPrefixes: ['deepseek'],
    familyPrefixes: ['deepseek'],
  },
  {
    key: 'mistral',
    label: 'Mistral',
    iconKey: 'mistral',
    litellmProvider: 'mistral',
    explicitPrefixes: ['mistral'],
    familyPrefixes: ['mistral', 'mixtral', 'pixtral', 'codestral', 'ministral', 'magistral'],
  },
  {
    key: 'meta',
    label: 'Meta',
    iconKey: 'meta',
    litellmProvider: 'meta',
    explicitPrefixes: ['meta'],
    familyPrefixes: ['llama', 'meta-llama'],
  },
  {
    key: 'cohere',
    label: 'Cohere',
    iconKey: 'cohere',
    litellmProvider: 'cohere_chat',
    explicitPrefixes: ['cohere_chat', 'cohere'],
    familyPrefixes: ['command'],
  },
  {
    key: 'xai',
    label: 'xAI',
    iconKey: 'grok',
    litellmProvider: 'xai',
    explicitPrefixes: ['xai'],
    familyPrefixes: ['grok'],
  },
  {
    key: 'groq',
    label: 'Groq',
    iconKey: 'groq',
    litellmProvider: 'groq',
    explicitPrefixes: ['groq'],
  },
  {
    key: 'bedrock',
    label: 'Bedrock',
    iconKey: 'bedrock',
    litellmProvider: 'bedrock',
    explicitPrefixes: ['bedrock'],
  },
  {
    key: 'azure',
    label: 'Azure',
    iconKey: 'azure',
    litellmProvider: 'azure',
    explicitPrefixes: ['azure'],
  },
  {
    key: 'together',
    label: 'Together',
    iconKey: 'together',
    litellmProvider: 'together_ai',
    explicitPrefixes: ['together_ai', 'together'],
  },
  {
    key: 'nvidia',
    label: 'NVIDIA',
    iconKey: 'nvidia',
    litellmProvider: 'nvidia_nim',
    explicitPrefixes: ['nvidia_nim', 'nvidia'],
    familyPrefixes: ['nemotron'],
  },
  {
    key: 'zhipuai',
    label: 'Zhipu AI',
    iconKey: 'zhipu',
    litellmProvider: 'zhipuai',
    explicitPrefixes: ['zhipuai'],
    familyPrefixes: ['chatglm', 'glm'],
  },
  {
    key: 'volcengine',
    label: 'Volcengine',
    iconKey: 'volcengine',
    litellmProvider: 'volcengine',
    explicitPrefixes: ['volcengine'],
    familyPrefixes: ['doubao', 'seed'],
  },
  {
    key: 'minimax',
    label: 'MiniMax',
    iconKey: 'minimax',
    litellmProvider: 'minimax',
    explicitPrefixes: ['minimax'],
    familyPrefixes: ['abab'],
  },
  {
    key: 'qwen',
    label: 'Qwen',
    iconKey: 'qwen',
    litellmProvider: 'qwen',
    explicitPrefixes: ['qwen'],
    familyPrefixes: ['qwen', 'qvq'],
  },
  {
    key: 'fireworks',
    label: 'Fireworks',
    iconKey: 'fireworks',
    litellmProvider: 'fireworks_ai',
    explicitPrefixes: ['fireworks_ai', 'fireworks'],
  },
  {
    key: 'ollama',
    label: 'Ollama',
    iconKey: 'ollama',
    litellmProvider: 'ollama',
    explicitPrefixes: ['ollama'],
  },
  {
    key: 'openrouter',
    label: 'OpenRouter',
    iconKey: 'openrouter',
    litellmProvider: 'openrouter',
    explicitPrefixes: ['openrouter'],
  },
  {
    key: 'moonshot',
    label: 'Moonshot',
    iconKey: 'moonshot',
    explicitPrefixes: ['moonshot', 'moonshotai'],
    familyPrefixes: ['moonshot'],
  },
  {
    key: 'perplexity',
    label: 'Perplexity',
    iconKey: 'perplexity',
    explicitPrefixes: ['perplexity'],
    familyPrefixes: ['sonar', 'pplx'],
  },
  {
    key: 'baichuan',
    label: 'Baichuan',
    iconKey: 'baichuan',
    explicitPrefixes: ['baichuan'],
    familyPrefixes: ['baichuan'],
  },
  {
    key: 'sensenova',
    label: 'SenseNova',
    iconKey: 'sensenova',
    explicitPrefixes: ['sensenova'],
    familyPrefixes: ['sensenova'],
  },
  {
    key: 'spark',
    label: 'Spark',
    iconKey: 'spark',
    explicitPrefixes: ['spark'],
    familyPrefixes: ['spark'],
  },
  {
    key: 'hunyuan',
    label: 'Hunyuan',
    iconKey: 'hunyuan',
    explicitPrefixes: ['hunyuan'],
    familyPrefixes: ['hunyuan'],
  },
  {
    key: 'wenxin',
    label: 'Wenxin',
    iconKey: 'wenxin',
    explicitPrefixes: ['wenxin'],
    familyPrefixes: ['ernie'],
  },
  {
    key: 'gemma',
    label: 'Gemma',
    iconKey: 'gemma',
    explicitPrefixes: ['gemma'],
    familyPrefixes: ['gemma'],
  },
  {
    key: 'internlm',
    label: 'InternLM',
    iconKey: 'internlm',
    explicitPrefixes: ['internlm'],
    familyPrefixes: ['internlm'],
  },
  {
    key: 'yi',
    label: 'Yi',
    iconKey: 'yi',
    explicitPrefixes: ['yi'],
    familyPrefixes: ['yi'],
  },
  {
    key: 'stepfun',
    label: 'StepFun',
    iconKey: 'stepfun',
    explicitPrefixes: ['stepfun'],
    familyPrefixes: ['step'],
  },
  {
    key: 'kimi',
    label: 'Kimi',
    iconKey: 'kimi',
    explicitPrefixes: ['kimi'],
    familyPrefixes: ['kimi'],
  },
] as const

const MODEL_PROVIDER_META_MAP = MODEL_PROVIDER_LIST.reduce<Record<KnownModelProviderKey, ModelProviderMeta>>((acc, item) => {
  acc[item.key] = item
  return acc
}, {} as Record<KnownModelProviderKey, ModelProviderMeta>)

const PRIMARY_MODEL_PROVIDER_ORDER: readonly KnownModelProviderKey[] = [
  'anthropic',
  'openai',
  'vertex',
  'deepseek',
  'mistral',
  'meta',
  'cohere',
  'xai',
  'groq',
  'bedrock',
  'azure',
  'together',
  'nvidia',
  'zhipuai',
  'volcengine',
  'minimax',
  'qwen',
  'fireworks',
  'ollama',
  'openrouter',
] as const

const SECONDARY_MODEL_PROVIDER_ORDER: readonly KnownModelProviderKey[] = [
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

const MODEL_PROVIDER_FILTER_ORDER: readonly KnownModelProviderKey[] = [
  ...PRIMARY_MODEL_PROVIDER_ORDER,
  ...SECONDARY_MODEL_PROVIDER_ORDER,
] as const

const EXPLICIT_PROVIDER_RULES = MODEL_PROVIDER_LIST
  .flatMap<ProviderPrefixRule>((item) => (
    (item.explicitPrefixes ?? []).map((prefix) => ({
      prefix: normalizeProviderInput(prefix),
      key: item.key,
    }))
  ))
  .sort((left, right) => right.prefix.length - left.prefix.length || left.prefix.localeCompare(right.prefix))

const FAMILY_PROVIDER_RULES = MODEL_PROVIDER_LIST
  .flatMap<ProviderPrefixRule>((item) => (
    (item.familyPrefixes ?? []).map((prefix) => ({
      prefix: normalizeProviderInput(prefix),
      key: item.key,
    }))
  ))
  .sort((left, right) => right.prefix.length - left.prefix.length || left.prefix.localeCompare(right.prefix))

const SEGMENT_SPLIT_RE = /[/:|]+/
const TOKEN_SPLIT_RE = /[\s._-]+/

function normalizeProviderInput(value: string | null | undefined) {
  return String(value ?? '').trim().toLowerCase()
}

function appendProviderCandidate(target: string[], seen: Set<string>, value: string) {
  const normalized = normalizeProviderInput(value)
  if (!normalized || seen.has(normalized)) return
  seen.add(normalized)
  target.push(normalized)
}

function appendSegmentTokens(target: string[], seen: Set<string>, value: string) {
  const normalized = normalizeProviderInput(value)
  if (!normalized) return
  appendProviderCandidate(target, seen, normalized)

  const tokens = normalized
    .split(TOKEN_SPLIT_RE)
    .map((segment) => segment.trim())
    .filter(Boolean)

  for (const token of tokens) {
    appendProviderCandidate(target, seen, token)
  }
}

function buildFallbackCandidates(value: string | null | undefined) {
  const normalized = normalizeProviderInput(value)
  if (!normalized) return []

  const candidates: string[] = []
  const seen = new Set<string>()
  const segments = normalized
    .split(SEGMENT_SPLIT_RE)
    .map((segment) => segment.trim())
    .filter(Boolean)

  const tail = segments.at(-1) ?? normalized
  appendSegmentTokens(candidates, seen, tail)
  appendProviderCandidate(candidates, seen, normalized)

  for (let index = segments.length - 1; index >= 0; index -= 1) {
    appendSegmentTokens(candidates, seen, segments[index])
  }

  return candidates
}

function resolveExplicitModelProviderKey(...values: Array<string | null | undefined>): KnownModelProviderKey | null {
  for (const value of values) {
    const normalized = normalizeProviderInput(value)
    if (!normalized) continue

    const firstSegment = normalized
      .split('/')
      .map((segment) => segment.trim())
      .find(Boolean) ?? ''

    for (const candidate of [firstSegment, normalized]) {
      if (!candidate) continue
      const matchedRule = EXPLICIT_PROVIDER_RULES.find((rule) => candidate === rule.prefix)
      if (matchedRule) {
        return matchedRule.key
      }
    }
  }

  return null
}

function resolveFallbackModelProviderKey(...values: Array<string | null | undefined>): KnownModelProviderKey | null {
  const candidates = values.flatMap((value) => buildFallbackCandidates(value))

  for (const candidate of candidates) {
    const matchedRule = FAMILY_PROVIDER_RULES.find((rule) => candidate.startsWith(rule.prefix))
    if (matchedRule) {
      return matchedRule.key
    }
  }

  return null
}

function resolveKnownModelProviderKey(model: string, ownerBy?: string | null | undefined): KnownModelProviderKey | null {
  return resolveExplicitModelProviderKey(model, ownerBy) ?? resolveFallbackModelProviderKey(model, ownerBy)
}

export function resolveModelProviderKey(model: string, ownerBy?: string | null | undefined): ModelProviderKey {
  return resolveKnownModelProviderKey(model, ownerBy) ?? 'unknown'
}

export function resolveModelProviderMeta(model: string, ownerBy?: string | null | undefined): ModelProviderMeta | null {
  const key = resolveKnownModelProviderKey(model, ownerBy)
  return key ? MODEL_PROVIDER_META_MAP[key] : null
}

export function getModelProviderMeta(key: ModelProviderKey): ModelProviderMeta | null {
  if (key === 'unknown') return null
  return MODEL_PROVIDER_META_MAP[key]
}

export function matchesModelProvider(
  provider: ModelProviderFilterKey,
  model: string,
  ownerBy?: string | null | undefined,
) {
  if (provider === 'all') return true
  return resolveModelProviderKey(model, ownerBy) === provider
}

export function buildModelProviderTabs(
  items: ModelProviderCandidate[],
  options: {
    allLabel: string
    unknownLabel: string
  },
): ModelProviderTab[] {
  const counts = new Map<ModelProviderKey, number>()

  for (const item of items) {
    const provider = resolveModelProviderKey(item.model, item.ownerBy)
    counts.set(provider, (counts.get(provider) ?? 0) + 1)
  }

  const tabs: ModelProviderTab[] = [{
    key: 'all',
    label: options.allLabel,
    count: items.length,
  }]

  for (const key of MODEL_PROVIDER_FILTER_ORDER) {
    const count = counts.get(key) ?? 0
    if (count <= 0) continue
    const meta = MODEL_PROVIDER_META_MAP[key]
    tabs.push({
      key,
      label: meta.label,
      count,
      iconKey: meta.iconKey,
    })
  }

  const unknownCount = counts.get('unknown') ?? 0
  if (unknownCount > 0) {
    tabs.push({
      key: 'unknown',
      label: options.unknownLabel,
      count: unknownCount,
    })
  }

  return tabs
}

export function collectModelProviderIconKeys(items: ModelProviderCandidate[]) {
  const iconKeys = new Set<string>()

  for (const item of items) {
    const meta = resolveModelProviderMeta(item.model, item.ownerBy)
    if (!meta?.iconKey) continue
    iconKeys.add(meta.iconKey)
  }

  return Array.from(iconKeys)
}
