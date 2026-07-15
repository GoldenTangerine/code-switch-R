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

export type ModelRouteTranslate = (
  key: string,
  params?: Record<string, string>,
) => string

function displayModel(value?: string) {
  return `${value ?? ''}`.trim() || '-'
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
