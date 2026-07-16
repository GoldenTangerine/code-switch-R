/**
 * @name: 模型映射开关异步状态
 * @Descripttion: 管理模型映射开关的乐观更新、即时保存和过期请求隔离
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-16 10:10:27
 * @LastEditTime: 2026-07-16 10:10:27
 * @FilePath: frontend/src/components/Main/modals/useModelMappingRuleToggle.ts
 */

import { ref } from 'vue'
import type { AutomationCard } from '../../../data/cards'
import type { VendorForm } from '../types'

type PersistModelMappingRule = (key: string, enabled: boolean) => Promise<void>

interface UseModelMappingRuleToggleOptions {
  form: VendorForm
  getCard: () => AutomationCard | null
  getPersistRule: () => PersistModelMappingRule | undefined
}

export function useModelMappingRuleToggle(options: UseModelMappingRuleToggleOptions) {
  const isSaving = ref(false)
  let requestSequence = 0

  function invalidatePending(): void {
    requestSequence += 1
    isSaving.value = false
  }

  async function toggleRule(key: string, enabled: boolean): Promise<void> {
    if (isSaving.value) return

    const previousDisabled = { ...(options.form.modelMappingDisabled || {}) }
    const nextDisabled = { ...previousDisabled }
    if (enabled) {
      delete nextDisabled[key]
    } else {
      nextDisabled[key] = true
    }
    options.form.modelMappingDisabled = nextDisabled

    const card = options.getCard()
    const persistRule = options.getPersistRule()
    const isPersistedRule = !!card?.modelMapping
      && Object.prototype.hasOwnProperty.call(card.modelMapping, key)
    if (!isPersistedRule || !persistRule) return

    const requestID = ++requestSequence
    isSaving.value = true
    try {
      await persistRule(key, enabled)
    } catch {
      if (requestID === requestSequence && options.getCard() === card) {
        options.form.modelMappingDisabled = previousDisabled
      }
    } finally {
      if (requestID === requestSequence) {
        isSaving.value = false
      }
    }
  }

  return {
    isSaving,
    invalidatePending,
    toggleRule,
  }
}
