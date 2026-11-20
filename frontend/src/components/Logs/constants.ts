export const PER_MILLION_TOKENS = 1_000_000
export const COST_TOOLTIP_DIFF_EPSILON = 0.000001

export const MODEL_SHARE_COLORS = [
  '#818cf8',
  '#fb923c',
  '#f472b6',
  '#34d399',
  '#60a5fa',
  '#a78bfa',
  '#94a3b8',
  '#fbbf24',
  '#fb7185',
  '#22d3ee',
] as const

/** 日志平台标识对应的应用展示名（品牌名不做翻译，与筛选、表格、存储弹窗共用） */
export const LOG_PLATFORM_LABELS: Readonly<Record<string, string>> = {
  claude: 'Claude',
  codex: 'Codex',
  gemini: 'Gemini',
  opencode: 'OpenCode',
  grokbuild: 'Grok Build',
  antigravity: 'Antigravity',
  kimi: 'Kimi Code',
  copilot: 'Copilot CLI',
  workbuddy: 'WorkBuddy',
  'deepseek-harness': 'DeepSeek Harness',
  'minimax-code': 'MiniMax Code',
  omp: 'Oh My Pi',
  pi: 'Pi',
}

/**
 * 平台标识转展示名，未知标识（如 custom:xxx）原样返回
 * @author sm
 */
export const formatLogPlatformLabel = (platform?: string | null) => {
  const key = String(platform ?? '').trim()
  if (!key) return '—'
  return LOG_PLATFORM_LABELS[key] ?? key
}
