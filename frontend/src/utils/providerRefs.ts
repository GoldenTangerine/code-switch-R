/**
 * @name: 供应商引用工具
 * @Descripttion: 供应商 providerRef/id 归一化与卡片引用提取（叶子模块，供 services 与 Main 适配层共用，避免跨 chunk 循环依赖）
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 10:30:00
 * @LastEditTime: 2026-08-17 10:30:00
 * @FilePath: frontend/src/utils/providerRefs.ts
 */
import type { AutomationCard } from '../data/cards'

export const normalizeProviderKey = (value: string) => value?.trim().toLowerCase() ?? ''

export const normalizeProviderRef = (value: string | number | null | undefined) => `${value ?? ''}`.trim()

export const cardProviderRef = (card: AutomationCard): string => {
  const ref = normalizeProviderRef(card.providerRef)
  if (ref) return ref
  if (Number.isFinite(card.id)) return `${card.id}`
  return ''
}
