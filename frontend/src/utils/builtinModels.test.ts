/**
 * @name: 内置模型候选测试
 * @Descripttion: 验证 CLI 与供应商模型下拉候选的筛选规则
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-16 01:45:20
 * @LastEditTime: 2026-07-16 01:45:20
 * @FilePath: frontend/src/utils/builtinModels.test.ts
 */

import { describe, expect, it } from 'vitest'
import type { ModelPricingRow } from '../services/modelPricing'
import { filterAndSortStringOptions } from './fuzzyOptionSearch'
import { buildBuiltinProviderModelOptions } from './builtinModels'

const createRow = (model: string, source: string): ModelPricingRow => ({
  model,
  source,
  input_cost_per_token: 0,
  output_cost_per_token: 0,
  output_cost_per_reasoning_token: 0,
  cache_creation_input_token_cost: 0,
  cache_read_input_token_cost: 0,
  ephemeral_1h_cost_per_token: 0,
  group_multiplier: 1,
  is_override: false,
  is_custom: false,
})

describe('buildBuiltinProviderModelOptions', () => {
  it('collects ordinary model names from builtin and synced sources across platforms', () => {
    const options = buildBuiltinProviderModelOptions([
      createRow('gpt-5-codex', 'builtin'),
      createRow('claude-sonnet-4-5', 'claude_sync'),
      createRow('gemini-2.5-pro', 'cloud_sync'),
      createRow('custom-private-model', 'manual'),
    ])

    expect(options).toEqual([
      'claude-sonnet-4-5',
      'gemini-2.5-pro',
      'gpt-5-codex',
    ])
  })

  it('excludes provider-qualified identifiers and removes duplicates', () => {
    const options = buildBuiltinProviderModelOptions([
      createRow('kimi-k2-thinking', 'builtin'),
      createRow(' kimi-k2-thinking ', 'cloud_sync'),
      createRow('openrouter/moonshotai/kimi-k2', 'builtin'),
      createRow('anthropic.claude-sonnet-4', 'builtin'),
      createRow('bedrock:claude-sonnet-4', 'builtin'),
      createRow('model@provider', 'builtin'),
    ])

    expect(options).toEqual(['kimi-k2-thinking'])
  })

  it('supports fuzzy filtering for provider model options', () => {
    const options = buildBuiltinProviderModelOptions([
      createRow('claude-sonnet-4-5', 'builtin'),
      createRow('gpt-5-codex', 'builtin'),
      createRow('kimi-k2-thinking', 'builtin'),
    ])

    expect(filterAndSortStringOptions(options, 'k2think')).toEqual(['kimi-k2-thinking'])
  })
})
