import { describe, expect, it } from 'vitest'
import {
  buildModelProviderTabs,
  collectModelProviderIconKeys,
  resolveModelProviderKey,
  resolveModelProviderMeta,
} from './modelProviders'

describe('modelProviders', () => {
  it.each([
    ['claude-sonnet-4-5-20250929', 'anthropic'],
    ['anthropic.claude-3-7-sonnet', 'anthropic'],
    ['gpt-4o-mini', 'openai'],
    ['o3-mini', 'openai'],
    ['gemini-2.5-pro', 'vertex'],
    ['deepseek/deepseek-r1', 'deepseek'],
    ['mixtral-8x7b-instruct', 'mistral'],
    ['command-r-plus', 'cohere'],
    ['grok-3-beta', 'xai'],
    ['qwen-max-latest', 'qwen'],
    ['moonshot-v1-8k', 'moonshot'],
    ['kimi-k1.5', 'kimi'],
    ['sonar-pro', 'perplexity'],
    ['glm-4-plus', 'zhipuai'],
    ['abab-6.5-chat', 'minimax'],
    ['baichuan-4-turbo', 'baichuan'],
    ['sensenova-5.5', 'sensenova'],
    ['spark-4.0-ultra', 'spark'],
    ['hunyuan-turbo', 'hunyuan'],
    ['ernie-4.0-8k', 'wenxin'],
    ['gemma-2-27b', 'gemma'],
    ['nvidia-nemotron-4-340b', 'nvidia'],
    ['internlm2-20b', 'internlm'],
    ['yi-lightning', 'yi'],
    ['step-2-16k', 'stepfun'],
    ['custom-provider/my-private-model', 'unknown'],
  ])('should resolve %s -> %s', (model, expected) => {
    expect(resolveModelProviderKey(model)).toBe(expected)
  })

  it.each([
    ['azure/gpt-4o', 'azure'],
    ['azure/eu/gpt-4o-2024-08-06', 'azure'],
    ['openrouter/anthropic/claude-sonnet-4', 'openrouter'],
    ['openrouter/openai/gpt-4o', 'openrouter'],
    ['bedrock/us-east-1/anthropic.claude-v2:1', 'bedrock'],
    ['bedrock/eu-west-3/mistral.mixtral-8x7b-instruct-v0:1', 'bedrock'],
    ['fireworks_ai/accounts/fireworks/models/deepseek-r1', 'fireworks'],
    ['groq/openai/gpt-oss-120b', 'groq'],
    ['together_ai/meta-llama/Llama-3.3-70B-Instruct-Turbo', 'together'],
    ['nvidia_nim/meta/llama-3.1-405b-instruct', 'nvidia'],
    ['vertex_ai-language-models/gemini-2.5-pro', 'vertex'],
    ['cohere_chat/command-r-plus', 'cohere'],
    ['volcengine/doubao-pro-32k', 'volcengine'],
  ])('should prefer explicit provider prefix for %s -> %s', (model, expected) => {
    expect(resolveModelProviderKey(model)).toBe(expected)
  })

  it('should fall back to ownerBy when model name is not informative', () => {
    expect(resolveModelProviderKey('my-team-model', 'openai')).toBe('openai')
    expect(resolveModelProviderKey('experimental-preview', 'anthropic')).toBe('anthropic')
    expect(resolveModelProviderKey('preview-build', 'openrouter')).toBe('openrouter')
  })

  it('should expose provider meta for known models', () => {
    const meta = resolveModelProviderMeta('claude-sonnet-4')
    expect(meta).not.toBeNull()
    expect(meta?.label).toBe('Anthropic')
    expect(meta?.iconKey).toBe('claude')
  })

  it('should build provider tabs in the same core order as the reference implementation', () => {
    const tabs = buildModelProviderTabs([
      { model: 'openrouter/anthropic/claude-sonnet-4' },
      { model: 'azure/gpt-4o' },
      { model: 'gpt-4o' },
      { model: 'custom/private-model' },
    ], {
      allLabel: 'All vendors',
      unknownLabel: 'Unknown',
    })

    expect(tabs).toEqual([
      { key: 'all', label: 'All vendors', count: 4 },
      { key: 'openai', label: 'OpenAI', count: 1, iconKey: 'openai' },
      { key: 'azure', label: 'Azure', count: 1, iconKey: 'azure' },
      { key: 'openrouter', label: 'OpenRouter', count: 1, iconKey: 'openrouter' },
      { key: 'unknown', label: 'Unknown', count: 1 },
    ])
  })

  it('should collect unique icon keys only for recognized providers', () => {
    const iconKeys = collectModelProviderIconKeys([
      { model: 'openrouter/anthropic/claude-sonnet-4' },
      { model: 'openrouter/openai/gpt-4o' },
      { model: 'azure/gpt-4o' },
      { model: 'custom/private-model' },
    ])

    expect(iconKeys.sort()).toEqual(['azure', 'openrouter'])
  })
})
