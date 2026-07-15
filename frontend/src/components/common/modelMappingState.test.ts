/**
 * @name: 模型映射状态测试
 * @Descripttion: 验证模型映射与思考强度配置保持同步
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-15 16:26:50
 * @LastEditTime: 2026-07-15 16:26:50
 * @FilePath: frontend/src/components/common/modelMappingState.test.ts
 */

import { describe, expect, it } from 'vitest'
import { removeModelMappingRule, upsertModelMappingRule } from './modelMappingState'

describe('modelMappingState', () => {
  it('重命名映射时同步迁移思考强度且不修改原对象', () => {
    const modelMappings = { 'claude-*': 'vendor-*', 'gpt-*': 'openai-*' }
    const reasoningEfforts = { 'claude-*': 'high', 'gpt-*': 'medium' }

    const result = upsertModelMappingRule(
      modelMappings,
      reasoningEfforts,
      'claude-*',
      'claude-opus-*',
      'vendor-opus-*',
      'xhigh',
    )

    expect(result).toEqual({
      modelMappings: { 'claude-opus-*': 'vendor-opus-*', 'gpt-*': 'openai-*' },
      reasoningEfforts: { 'claude-opus-*': 'xhigh', 'gpt-*': 'medium' },
    })
    expect(modelMappings).toEqual({ 'claude-*': 'vendor-*', 'gpt-*': 'openai-*' })
    expect(reasoningEfforts).toEqual({ 'claude-*': 'high', 'gpt-*': 'medium' })
  })

  it('清空思考强度时删除对应配置', () => {
    const result = upsertModelMappingRule(
      { 'claude-*': 'vendor-*' },
      { 'claude-*': 'high' },
      'claude-*',
      'claude-*',
      'vendor-*',
      '',
    )

    expect(result.reasoningEfforts).toEqual({})
  })

  it('删除映射时同步删除思考强度且不修改原对象', () => {
    const modelMappings = { 'claude-*': 'vendor-*', 'gpt-*': 'openai-*' }
    const reasoningEfforts = { 'claude-*': 'high', 'gpt-*': 'medium' }

    const result = removeModelMappingRule(modelMappings, reasoningEfforts, 'claude-*')

    expect(result).toEqual({
      modelMappings: { 'gpt-*': 'openai-*' },
      reasoningEfforts: { 'gpt-*': 'medium' },
    })
    expect(modelMappings).toEqual({ 'claude-*': 'vendor-*', 'gpt-*': 'openai-*' })
    expect(reasoningEfforts).toEqual({ 'claude-*': 'high', 'gpt-*': 'medium' })
  })
})
