/**
 * @name: 模型路由展示测试
 * @Descripttion: 验证模型路由提示文案与无障碍标签的生成规则。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-15 10:20:00
 * @LastEditTime: 2026-07-15 10:20:00
 * @FilePath: frontend/src/components/Main/modals/providerModelRoute.test.ts
 */
import { describe, expect, it } from 'vitest'
import {
  buildModelRouteAriaLabel,
  buildModelRouteTooltipLines,
  type ModelRouteTranslate,
} from './providerModelRoute'

const translate: ModelRouteTranslate = (key, params = {}) => {
  switch (key) {
    case 'components.main.concurrencyDetails.mappingRule':
      return `mapping:${params.source}->${params.target}`
    case 'components.main.concurrencyDetails.mappingResult':
      return `mapped:${params.model}`
    case 'components.main.concurrencyDetails.modelOverride':
      return `override:${params.source}->${params.target}`
    case 'components.main.concurrencyDetails.routeUnchanged':
      return 'unchanged'
    case 'components.main.concurrencyDetails.routeDetailsAria':
      return `actual:${params.model};details:${params.details}`
    default:
      return key
  }
}

describe('provider model route display', () => {
  it('returns the unavailable message for legacy entries', () => {
    expect(buildModelRouteTooltipLines({
      requestedModel: 'claude-opus-4.8',
      modelRouteCaptured: false,
    }, 'unavailable', translate)).toEqual(['unavailable'])
  })

  it('describes mapping and request body override in order', () => {
    expect(buildModelRouteTooltipLines({
      requestedModel: 'claude-opus-4.8',
      mappedModel: 'vendor-opus-4.8',
      modelMappingPattern: 'claude-opus-*',
      modelMappingTarget: 'vendor-opus-*',
      modelOverride: 'forced-opus-model',
      modelRouteCaptured: true,
    }, 'unavailable', translate)).toEqual([
      'mapping:claude-opus-*->vendor-opus-*',
      'mapped:vendor-opus-4.8',
      'override:vendor-opus-4.8->forced-opus-model',
    ])
  })

  it('marks a captured route without rewrites as unchanged', () => {
    expect(buildModelRouteTooltipLines({
      requestedModel: 'claude-sonnet-4.8',
      modelRouteCaptured: true,
    }, 'unavailable', translate)).toEqual(['unchanged'])
  })

  it('does not mark a route as unchanged when rewrites return to the requested model', () => {
    expect(buildModelRouteTooltipLines({
      requestedModel: 'claude-opus-4.8',
      mappedModel: 'vendor-opus-4.8',
      modelMappingPattern: 'claude-opus-*',
      modelMappingTarget: 'vendor-opus-*',
      modelOverride: 'claude-opus-4.8',
      modelRouteCaptured: true,
    }, 'unavailable', translate)).toEqual([
      'mapping:claude-opus-*->vendor-opus-*',
      'mapped:vendor-opus-4.8',
      'override:vendor-opus-4.8->claude-opus-4.8',
    ])
  })

  it('includes the actual model in the accessible label', () => {
    expect(buildModelRouteAriaLabel('forced-opus-model', ['mapped', 'override'], translate))
      .toBe('actual:forced-opus-model;details:mapped; override')
  })
})
