/**
 * @name: 供应商日志红点测试
 * @Descripttion: 验证日志红点默认值与关闭后的显示规则
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-23 12:43:49
 * @LastEditTime: 2026-07-23 12:43:49
 * @FilePath: frontend/src/components/Main/utils/providerLogBadge.test.ts
 */

import { describe, expect, it } from 'vitest'
import type { AutomationCard } from '../../../data/cards'
import { cardToGemini, geminiToCard, providerToCard, serializeProviders } from '../adapters/providerCardMappers'
import { shouldShowProviderLogBadge } from './providerLogBadge'

describe('shouldShowProviderLogBadge', () => {
  it('旧配置存在未读日志时默认显示红点', () => {
    expect(shouldShowProviderLogBadge(undefined, true)).toBe(true)
  })

  it('供应商关闭红点后隐藏未读提示', () => {
    expect(shouldShowProviderLogBadge(true, true)).toBe(false)
  })

  it('没有未读日志时不显示红点', () => {
    expect(shouldShowProviderLogBadge(false, false)).toBe(false)
  })

  it('兼容旧供应商配置并持久化关闭状态', () => {
    const provider = {
      id: 1,
      name: 'Codex Provider',
      apiUrl: 'https://example.com',
      apiKey: 'key',
      officialSite: '',
      icon: 'openai',
      tint: '',
      accent: '',
      enabled: true,
    }

    const legacyCard = providerToCard(provider)
    expect(legacyCard.hideLogBadge).toBe(false)

    legacyCard.hideLogBadge = true
    expect(serializeProviders([legacyCard], 'codex')[0]?.hideLogBadge).toBe(true)
  })

  it('在 Gemini 配置映射中保留关闭状态', () => {
    const provider = {
      id: 'gemini-provider',
      name: 'Gemini Provider',
      enabled: true,
      hideLogBadge: true,
    }
    const card = geminiToCard(provider, 0)

    expect(card.hideLogBadge).toBe(true)
    expect(cardToGemini(card as AutomationCard, provider).hideLogBadge).toBe(true)
  })
})
