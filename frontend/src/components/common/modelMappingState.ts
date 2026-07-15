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
  reasoningEfforts: Record<string, string>
}

export function upsertModelMappingRule(
  modelMappings: Record<string, string>,
  reasoningEfforts: Record<string, string>,
  originalKey: string,
  key: string,
  value: string,
  reasoningEffort: string,
): ModelMappingState {
  const nextModelMappings = { ...modelMappings }
  const nextReasoningEfforts = { ...reasoningEfforts }

  if (originalKey && originalKey !== key) {
    delete nextModelMappings[originalKey]
    delete nextReasoningEfforts[originalKey]
  }

  nextModelMappings[key] = value
  if (reasoningEffort) {
    nextReasoningEfforts[key] = reasoningEffort
  } else {
    delete nextReasoningEfforts[key]
  }

  return {
    modelMappings: nextModelMappings,
    reasoningEfforts: nextReasoningEfforts,
  }
}

export function removeModelMappingRule(
  modelMappings: Record<string, string>,
  reasoningEfforts: Record<string, string>,
  key: string,
): ModelMappingState {
  const nextModelMappings = { ...modelMappings }
  const nextReasoningEfforts = { ...reasoningEfforts }
  delete nextModelMappings[key]
  delete nextReasoningEfforts[key]

  return {
    modelMappings: nextModelMappings,
    reasoningEfforts: nextReasoningEfforts,
  }
}
