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
  buildConnectionParameterTooltipLines,
  buildConnectionModelRows,
  buildModelRouteAriaLabel,
  buildModelRouteTooltipLines,
  connectionParameterDisplayValue,
  type ConnectionParameterSnapshot,
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
    case 'components.main.concurrencyDetails.sessionPreferredProvider':
      return `preferred:${params.provider}`
    case 'components.main.concurrencyDetails.sessionProviderRoute':
      return `selection:${params.result}`
    case 'components.main.concurrencyDetails.sessionProviderRouteValues.preferred':
      return 'followed'
    case 'components.main.concurrencyDetails.sessionProviderRouteValues.fallback':
      return 'fallback'
    case 'components.main.concurrencyDetails.routeUnchanged':
      return 'unchanged'
    case 'components.main.concurrencyDetails.routeDetailsAria':
      return `actual:${params.model};details:${params.details}`
    case 'components.main.concurrencyDetails.parameterReasoningEffort':
      return 'reasoning'
    case 'components.main.concurrencyDetails.parameterMaxOutputTokens':
      return 'output'
    case 'components.main.concurrencyDetails.parameterValue':
      return `${params.label}:${params.value}`
    case 'components.main.concurrencyDetails.parameterSource':
      return `${params.label}-source:${params.source}`
    case 'components.main.concurrencyDetails.parameterSourceRequest':
      return 'request'
    case 'components.main.concurrencyDetails.parameterSourceRequestBodyOverride':
      return 'override'
    case 'components.main.concurrencyDetails.parameterSourceModelMapping':
      return 'mapping'
    case 'components.main.concurrencyDetails.parameterValueMissing':
      return '-'
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

  it('describes the preferred session provider and fallback result', () => {
    expect(buildModelRouteTooltipLines({
      requestedModel: 'code-switch-r-subagent',
      modelRouteCaptured: true,
      sessionPreferredProvider: 'Provider B',
      sessionProviderRoute: 'fallback',
    }, 'unavailable', translate)).toEqual([
      'preferred:Provider B',
      'selection:fallback',
      'unchanged',
    ])
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

  it('formats connection parameter chips with stable empty values', () => {
    expect(connectionParameterDisplayValue('reasoning_effort', 'Extra-High')).toBe('xhigh')
    expect(connectionParameterDisplayValue('max_output_tokens', '16384')).toBe('16K')
    expect(connectionParameterDisplayValue('max_output_tokens', '')).toBe('-')
  })

  it('builds exact values and sources for the actual model tooltip', () => {
    const parameters: ConnectionParameterSnapshot[] = [
      {
        key: 'reasoning_effort',
        requestedValue: 'low',
        actualValue: 'vendor-ultra',
        source: 'model_mapping',
      },
      {
        key: 'max_output_tokens',
        requestedValue: '8192',
        actualValue: '16384',
        source: 'request_body_override',
      },
    ]

    expect(buildConnectionParameterTooltipLines(parameters, 'actual', translate)).toEqual([
      'reasoning:vendor-ultra',
      'reasoning-source:mapping',
      'output:16384',
      'output-source:override',
    ])
  })

  it('keeps requested tooltip values without final sources', () => {
    const parameters: ConnectionParameterSnapshot[] = []
    expect(buildConnectionParameterTooltipLines(parameters, 'requested', translate)).toEqual([
      'reasoning:-',
      'output:-',
    ])
  })

  it('builds dual model rows with route details only on the actual model', () => {
    const parameters: ConnectionParameterSnapshot[] = [
      {
        key: 'reasoning_effort',
        requestedValue: 'low',
        actualValue: 'high',
        source: 'model_mapping',
      },
    ]
    const rows = buildConnectionModelRows({
      showModelRouteDetails: true,
      requestedModel: 'claude-opus-4.8',
      actualModel: 'vendor-opus-4.8',
      parameters,
      actualRouteLines: ['mapped-route'],
    }, translate)

    expect(rows.map((row) => [row.key, row.stage, row.model, row.emphasized])).toEqual([
      ['requested', 'requested', 'claude-opus-4.8', false],
      ['actual', 'actual', 'vendor-opus-4.8', true],
    ])
    expect(rows[0].tooltipLines).not.toContain('mapped-route')
    expect(rows[1].tooltipLines[0]).toBe('mapped-route')
  })

  it('builds one actual row without route details for single model mode', () => {
    const rows = buildConnectionModelRows({
      showModelRouteDetails: false,
      requestedModel: 'gpt-5.4',
      actualModel: 'gpt-5.4',
      parameters: [],
      actualRouteLines: ['must-not-render'],
    }, translate)

    expect(rows).toHaveLength(1)
    expect(rows[0].key).toBe('actual')
    expect(rows[0].tooltipLines).not.toContain('must-not-render')
  })
})
