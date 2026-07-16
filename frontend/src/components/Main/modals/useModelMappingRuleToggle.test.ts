/**
 * @name: 模型映射开关异步状态测试
 * @Descripttion: 验证模型映射开关即时保存、回滚和过期请求隔离
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-16 10:10:27
 * @LastEditTime: 2026-07-16 10:10:27
 * @FilePath: frontend/src/components/Main/modals/useModelMappingRuleToggle.test.ts
 */

import { reactive } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import type { AutomationCard } from '../../../data/cards'
import type { VendorForm } from '../types'
import { useModelMappingRuleToggle } from './useModelMappingRuleToggle'

function createCard(id: number, mappingKey: string): AutomationCard {
  return {
    id,
    name: `Provider ${id}`,
    apiUrl: 'https://example.com',
    apiKey: 'key',
    officialSite: '',
    icon: 'openai',
    tint: '',
    accent: '',
    enabled: true,
    modelMapping: { [mappingKey]: 'vendor-*' },
    modelMappingDisabled: {},
  }
}

function createForm(mappingKey: string): VendorForm {
  return reactive({
    name: 'Provider',
    apiUrl: 'https://example.com',
    apiKey: 'key',
    officialSite: '',
    icon: 'openai',
    enabled: true,
    level: 1,
    modelMapping: { [mappingKey]: 'vendor-*' },
    modelMappingDisabled: {},
  })
}

function createDeferred(): {
  promise: Promise<void>
  resolve: () => void
  reject: (error: Error) => void
} {
  let resolve!: () => void
  let reject!: (error: Error) => void
  const promise = new Promise<void>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

describe('useModelMappingRuleToggle', () => {
  it('即时更新并在保存期间拒绝并发切换', async () => {
    const card = createCard(1, 'claude-*')
    const form = createForm('claude-*')
    const deferred = createDeferred()
    const persistRule = vi.fn(() => deferred.promise)
    const controller = useModelMappingRuleToggle({
      form,
      getCard: () => card,
      getPersistRule: () => persistRule,
    })

    const pending = controller.toggleRule('claude-*', false)
    await controller.toggleRule('claude-*', true)

    expect(form.modelMappingDisabled).toEqual({ 'claude-*': true })
    expect(controller.isSaving.value).toBe(true)
    expect(persistRule).toHaveBeenCalledOnce()

    deferred.resolve()
    await pending
    expect(controller.isSaving.value).toBe(false)
  })

  it('保存失败时回滚当前供应商状态', async () => {
    const card = createCard(1, 'claude-*')
    const form = createForm('claude-*')
    const persistRule = vi.fn().mockRejectedValue(new Error('save failed'))
    const controller = useModelMappingRuleToggle({
      form,
      getCard: () => card,
      getPersistRule: () => persistRule,
    })

    await controller.toggleRule('claude-*', false)

    expect(form.modelMappingDisabled).toEqual({})
    expect(controller.isSaving.value).toBe(false)
  })

  it('忽略弹窗切换后到达的旧请求失败结果', async () => {
    const firstCard = createCard(1, 'claude-*')
    const secondCard = createCard(2, 'gpt-*')
    const form = createForm('claude-*')
    const deferred = createDeferred()
    let currentCard = firstCard
    const controller = useModelMappingRuleToggle({
      form,
      getCard: () => currentCard,
      getPersistRule: () => () => deferred.promise,
    })

    const pending = controller.toggleRule('claude-*', false)
    currentCard = secondCard
    form.modelMapping = { 'gpt-*': 'openai-*' }
    form.modelMappingDisabled = { 'gpt-*': true }
    controller.invalidatePending()
    deferred.reject(new Error('save failed'))
    await pending

    expect(form.modelMappingDisabled).toEqual({ 'gpt-*': true })
    expect(controller.isSaving.value).toBe(false)
  })

  it('草稿规则只更新表单，不触发即时保存', async () => {
    const card = createCard(1, 'claude-*')
    const form = createForm('draft-*')
    const persistRule = vi.fn().mockResolvedValue(undefined)
    const controller = useModelMappingRuleToggle({
      form,
      getCard: () => card,
      getPersistRule: () => persistRule,
    })

    await controller.toggleRule('draft-*', false)

    expect(form.modelMappingDisabled).toEqual({ 'draft-*': true })
    expect(persistRule).not.toHaveBeenCalled()
    expect(controller.isSaving.value).toBe(false)
  })
})
