/**
 * @name: 模型映射状态
 * @Descripttion: 统一处理模型映射及其思考强度的新增、重命名和删除
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-15 16:26:50
 * @LastEditTime: 2026-07-15 16:26:50
 * @FilePath: frontend/src/components/common/modelMappingState.ts
 */

export interface ModelMappingState {
  modelMappings: Record<string, string>
  disabledRules: Record<string, boolean>
  reasoningEfforts: Record<string, string>
  supportsOneM: Record<string, boolean>
}

export const CLAUDE_SUBAGENT_MODEL_MAPPING_KEY = 'code-switch-r-subagent'
export const DEFAULT_MODEL_MAPPING_KEY = '*'

export function isReservedModelMappingKey(key: string): boolean {
  return key === CLAUDE_SUBAGENT_MODEL_MAPPING_KEY || key === DEFAULT_MODEL_MAPPING_KEY
}

export function filterRegularModelMappings(modelMappings: Record<string, string>): Array<[string, string]> {
  return Object.entries(modelMappings).filter(([key]) => !isReservedModelMappingKey(key))
}

export function resolveSubmittedModelMappingSupportsOneM(
  isOptionVisible: boolean,
  draftValue: boolean,
  originalKey: string,
  supportsOneM: Record<string, boolean>,
): boolean {
  if (isOptionVisible) return draftValue
  return originalKey !== '' && supportsOneM[originalKey] === true
}

export function upsertModelMappingRule(
  modelMappings: Record<string, string>,
  disabledRules: Record<string, boolean>,
  reasoningEfforts: Record<string, string>,
  supportsOneM: Record<string, boolean>,
  originalKey: string,
  key: string,
  value: string,
  reasoningEffort: string,
  declaresOneM: boolean,
): ModelMappingState {
  const nextModelMappings = { ...modelMappings }
  const nextDisabledRules = { ...disabledRules }
  const nextReasoningEfforts = { ...reasoningEfforts }
  const nextSupportsOneM = { ...supportsOneM }

  if (originalKey && originalKey !== key) {
    delete nextModelMappings[originalKey]
    if (nextDisabledRules[originalKey]) {
      nextDisabledRules[key] = true
    }
    delete nextDisabledRules[originalKey]
    delete nextReasoningEfforts[originalKey]
    delete nextSupportsOneM[originalKey]
  }

  nextModelMappings[key] = value
  if (reasoningEffort) {
    nextReasoningEfforts[key] = reasoningEffort
  } else {
    delete nextReasoningEfforts[key]
  }
  if (declaresOneM) {
    nextSupportsOneM[key] = true
  } else {
    delete nextSupportsOneM[key]
  }

  return {
    modelMappings: nextModelMappings,
    disabledRules: nextDisabledRules,
    reasoningEfforts: nextReasoningEfforts,
    supportsOneM: nextSupportsOneM,
  }
}

export function removeModelMappingRule(
  modelMappings: Record<string, string>,
  disabledRules: Record<string, boolean>,
  reasoningEfforts: Record<string, string>,
  supportsOneM: Record<string, boolean>,
  key: string,
): ModelMappingState {
  const nextModelMappings = { ...modelMappings }
  const nextDisabledRules = { ...disabledRules }
  const nextReasoningEfforts = { ...reasoningEfforts }
  const nextSupportsOneM = { ...supportsOneM }
  delete nextModelMappings[key]
  delete nextDisabledRules[key]
  delete nextReasoningEfforts[key]
  delete nextSupportsOneM[key]

  return {
    modelMappings: nextModelMappings,
    disabledRules: nextDisabledRules,
    reasoningEfforts: nextReasoningEfforts,
    supportsOneM: nextSupportsOneM,
  }
}

export function updateFixedModelMappingRule(
  modelMappings: Record<string, string>,
  disabledRules: Record<string, boolean>,
  reasoningEfforts: Record<string, string>,
  supportsOneM: Record<string, boolean>,
  key: string,
  value: string,
  reasoningEffort: string,
  declaresOneM: boolean,
): ModelMappingState {
  if (!value.trim()) {
    return removeModelMappingRule(
      modelMappings,
      disabledRules,
      reasoningEfforts,
      supportsOneM,
      key,
    )
  }

  const updated = upsertModelMappingRule(
    modelMappings,
    disabledRules,
    reasoningEfforts,
    supportsOneM,
    key,
    key,
    value.trim(),
    reasoningEffort.trim(),
    declaresOneM,
  )
  delete updated.disabledRules[key]
  return updated
}
