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
import {
  removeModelMappingRule,
  resolveSubmittedModelMappingSupportsOneM,
  upsertModelMappingRule,
} from './modelMappingState'

describe('modelMappingState', () => {
  it('1M 选项隐藏时新增规则不继承草稿状态', () => {
    expect(resolveSubmittedModelMappingSupportsOneM(false, true, '', {})).toBe(false)
  })

  it('1M 选项隐藏时编辑规则保留原状态', () => {
    expect(resolveSubmittedModelMappingSupportsOneM(false, false, 'claude-*', {
      'claude-*': true,
    })).toBe(true)
    expect(resolveSubmittedModelMappingSupportsOneM(false, true, 'gpt-*', {})).toBe(false)
  })

  it('1M 选项显示时使用当前草稿状态', () => {
    expect(resolveSubmittedModelMappingSupportsOneM(true, true, '', {})).toBe(true)
    expect(resolveSubmittedModelMappingSupportsOneM(true, false, 'claude-*', {
      'claude-*': true,
    })).toBe(false)
  })

  it('重命名映射时同步迁移思考强度且不修改原对象', () => {
    const modelMappings = { 'claude-*': 'vendor-*', 'gpt-*': 'openai-*' }
    const reasoningEfforts = { 'claude-*': 'high', 'gpt-*': 'medium' }
    const supportsOneM = { 'claude-*': true }

    const result = upsertModelMappingRule(
      modelMappings,
      { 'claude-*': true },
      reasoningEfforts,
      supportsOneM,
      'claude-*',
      'claude-opus-*',
      'vendor-opus-*',
      'xhigh',
      true,
    )

    expect(result).toEqual({
      modelMappings: { 'claude-opus-*': 'vendor-opus-*', 'gpt-*': 'openai-*' },
      disabledRules: { 'claude-opus-*': true },
      reasoningEfforts: { 'claude-opus-*': 'xhigh', 'gpt-*': 'medium' },
      supportsOneM: { 'claude-opus-*': true },
    })
    expect(modelMappings).toEqual({ 'claude-*': 'vendor-*', 'gpt-*': 'openai-*' })
    expect(reasoningEfforts).toEqual({ 'claude-*': 'high', 'gpt-*': 'medium' })
    expect(supportsOneM).toEqual({ 'claude-*': true })
  })

  it('清空思考强度时删除对应配置', () => {
    const result = upsertModelMappingRule(
      { 'claude-*': 'vendor-*' },
      {},
      { 'claude-*': 'high' },
      { 'claude-*': true },
      'claude-*',
      'claude-*',
      'vendor-*',
      '',
      false,
    )

    expect(result.reasoningEfforts).toEqual({})
    expect(result.disabledRules).toEqual({})
    expect(result.supportsOneM).toEqual({})
  })

  it('删除映射时同步删除思考强度且不修改原对象', () => {
    const modelMappings = { 'claude-*': 'vendor-*', 'gpt-*': 'openai-*' }
    const reasoningEfforts = { 'claude-*': 'high', 'gpt-*': 'medium' }
    const supportsOneM = { 'claude-*': true, 'gpt-*': true }

    const result = removeModelMappingRule(
      modelMappings,
      { 'claude-*': true },
      reasoningEfforts,
      supportsOneM,
      'claude-*',
    )

    expect(result).toEqual({
      modelMappings: { 'gpt-*': 'openai-*' },
      disabledRules: {},
      reasoningEfforts: { 'gpt-*': 'medium' },
      supportsOneM: { 'gpt-*': true },
    })
    expect(modelMappings).toEqual({ 'claude-*': 'vendor-*', 'gpt-*': 'openai-*' })
    expect(reasoningEfforts).toEqual({ 'claude-*': 'high', 'gpt-*': 'medium' })
    expect(supportsOneM).toEqual({ 'claude-*': true, 'gpt-*': true })
  })
})
