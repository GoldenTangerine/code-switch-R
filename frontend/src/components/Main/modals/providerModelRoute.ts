/**
 * @name: 模型路由展示工具
 * @Descripttion: 生成连接详情中的模型路由提示和无障碍文案。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-15 10:20:00
 * @LastEditTime: 2026-07-15 10:20:00
 * @FilePath: frontend/src/components/Main/modals/providerModelRoute.ts
 */
export interface ModelRouteSnapshot {
  requestedModel?: string
  mappedModel?: string
  modelMappingPattern?: string
  modelMappingTarget?: string
  modelOverride?: string
  modelRouteCaptured?: boolean
}

export type ConnectionParameterKey = 'reasoning_effort' | 'max_output_tokens'
export type ConnectionParameterSource = 'request' | 'request_body_override' | 'model_mapping' | ''
export type ConnectionParameterStage = 'requested' | 'actual'

export interface ConnectionParameterSnapshot {
  key: ConnectionParameterKey
  requestedValue?: string
  actualValue?: string
  source?: ConnectionParameterSource
}

export interface ConnectionModelRow {
  key: 'requested' | 'actual'
  stage: ConnectionParameterStage
  model: string
  parameters: ConnectionParameterSnapshot[]
  emphasized: boolean
  tooltipLines: string[]
}

export interface ConnectionModelRowsInput {
  showModelRouteDetails: boolean
  requestedModel: string
  actualModel: string
  parameters: ConnectionParameterSnapshot[]
  actualRouteLines?: string[]
}

export const connectionParameterKeys: ConnectionParameterKey[] = [
  'reasoning_effort',
  'max_output_tokens',
]

export type ModelRouteTranslate = (
  key: string,
  params?: Record<string, string>,
) => string

function displayModel(value?: string) {
  return `${value ?? ''}`.trim() || '-'
}

function normalizeReasoningEffort(value?: string) {
  const raw = `${value ?? ''}`.trim()
  const normalized = raw.toLowerCase().replace(/[-_\s]/g, '')
  if (normalized === 'extrahigh' || normalized === 'xhigh') return 'xhigh'
  if (['low', 'medium', 'high', 'max'].includes(normalized)) return normalized
  return raw.toLowerCase()
}

function parameterRawValue(
  parameters: ConnectionParameterSnapshot[],
  key: ConnectionParameterKey,
  stage: ConnectionParameterStage,
) {
  const parameter = parameters.find((item) => item.key === key)
  return `${stage === 'requested' ? parameter?.requestedValue ?? '' : parameter?.actualValue ?? ''}`.trim()
}

function parameterLabelKey(key: ConnectionParameterKey) {
  return key === 'reasoning_effort'
    ? 'components.main.concurrencyDetails.parameterReasoningEffort'
    : 'components.main.concurrencyDetails.parameterMaxOutputTokens'
}

function parameterSourceKey(source?: ConnectionParameterSource) {
  switch (source) {
    case 'request':
      return 'components.main.concurrencyDetails.parameterSourceRequest'
    case 'request_body_override':
      return 'components.main.concurrencyDetails.parameterSourceRequestBodyOverride'
    case 'model_mapping':
      return 'components.main.concurrencyDetails.parameterSourceModelMapping'
    default:
      return 'components.main.concurrencyDetails.parameterValueMissing'
  }
}

export function connectionParameterDisplayValue(key: ConnectionParameterKey, value?: string) {
  const raw = `${value ?? ''}`.trim()
  if (!raw) return '-'
  if (key === 'reasoning_effort') return normalizeReasoningEffort(raw) || '-'

  const numeric = Number(raw)
  if (!Number.isFinite(numeric) || numeric < 1024) return raw
  const compact = Math.round((numeric / 1024) * 10) / 10
  return `${Number.isInteger(compact) ? compact.toFixed(0) : compact.toFixed(1)}K`
}

export function connectionParameterTone(key: ConnectionParameterKey, value?: string) {
  if (!`${value ?? ''}`.trim()) return 'empty'
  if (key !== 'reasoning_effort') return 'output'
  const normalized = normalizeReasoningEffort(value)
  if (['low', 'medium', 'high', 'xhigh', 'max'].includes(normalized)) return normalized
  return normalized ? 'custom' : 'empty'
}

export function connectionParameterValue(
  parameters: ConnectionParameterSnapshot[],
  key: ConnectionParameterKey,
  stage: ConnectionParameterStage,
) {
  return connectionParameterDisplayValue(key, parameterRawValue(parameters, key, stage))
}

export function buildConnectionParameterTooltipLines(
  parameters: ConnectionParameterSnapshot[],
  stage: ConnectionParameterStage,
  translate: ModelRouteTranslate,
) {
  const lines: string[] = []
  connectionParameterKeys.forEach((key) => {
    const parameter = parameters.find((item) => item.key === key)
    const rawValue = parameterRawValue(parameters, key, stage)
    const value = key === 'reasoning_effort'
      ? connectionParameterDisplayValue(key, rawValue)
      : rawValue || translate('components.main.concurrencyDetails.parameterValueMissing')
    const label = translate(parameterLabelKey(key))
    lines.push(translate('components.main.concurrencyDetails.parameterValue', { label, value }))
    if (stage === 'actual') {
      lines.push(translate('components.main.concurrencyDetails.parameterSource', {
        label,
        source: translate(parameterSourceKey(parameter?.source)),
      }))
    }
  })
  return lines
}

export function buildConnectionModelRows(
  input: ConnectionModelRowsInput,
  translate: ModelRouteTranslate,
): ConnectionModelRow[] {
  const actualRow: ConnectionModelRow = {
    key: 'actual',
    stage: 'actual',
    model: displayModel(input.actualModel),
    parameters: input.parameters,
    emphasized: true,
    tooltipLines: [
      ...(input.showModelRouteDetails ? input.actualRouteLines ?? [] : []),
      ...buildConnectionParameterTooltipLines(input.parameters, 'actual', translate),
    ],
  }
  if (!input.showModelRouteDetails) {
    return [actualRow]
  }
  return [
    {
      key: 'requested',
      stage: 'requested',
      model: displayModel(input.requestedModel),
      parameters: input.parameters,
      emphasized: false,
      tooltipLines: buildConnectionParameterTooltipLines(input.parameters, 'requested', translate),
    },
    actualRow,
  ]
}

export function buildModelRouteTooltipLines(
  route: ModelRouteSnapshot,
  unavailableMessage: string,
  translate: ModelRouteTranslate,
) {
  if (!route.modelRouteCaptured) {
    return [unavailableMessage]
  }

  const requestedModel = displayModel(route.requestedModel)
  const mappedModel = `${route.mappedModel ?? ''}`.trim()
  const mappingPattern = `${route.modelMappingPattern ?? ''}`.trim()
  const mappingTarget = `${route.modelMappingTarget ?? ''}`.trim()
  const modelOverride = `${route.modelOverride ?? ''}`.trim()
  const lines: string[] = []

  if (mappingPattern && mappingTarget) {
    lines.push(translate('components.main.concurrencyDetails.mappingRule', {
      source: mappingPattern,
      target: mappingTarget,
    }))
    if (mappedModel) {
      lines.push(translate('components.main.concurrencyDetails.mappingResult', { model: mappedModel }))
    }
  }

  if (modelOverride) {
    lines.push(translate('components.main.concurrencyDetails.modelOverride', {
      source: mappedModel || requestedModel,
      target: modelOverride,
    }))
  }

  return lines.length > 0
    ? lines
    : [translate('components.main.concurrencyDetails.routeUnchanged')]
}

export function buildModelRouteAriaLabel(
  actualModel: string,
  lines: string[],
  translate: ModelRouteTranslate,
) {
  return translate('components.main.concurrencyDetails.routeDetailsAria', {
    model: displayModel(actualModel),
    details: lines.join('; '),
  })
}
