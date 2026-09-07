/**
@name: 模型价格与费用展示
@Descripttion: 维护模型价格规则及请求费用展示。
@version: 1.0.0
@Author: sm
@Date: 2026-09-07 11:13:10
@LastEditTime: 2026-09-07 11:13:10
@FilePath: frontend/src/services/modelPricing.ts
*/
/**
 * 模型价格（input/output/cache）配置 API 封装
 */
import { Call, Events } from '@wailsio/runtime'

export const TOKEN_PRICING_CHARGES = ['prompt', 'completion', 'reasoning', 'cache_read', 'cache_write', 'cache_write_1h'] as const
export type TokenPricingCharge = typeof TOKEN_PRICING_CHARGES[number]

export interface CloudPricingTrigger {
  kind: string
  threshold?: number
  inclusive?: boolean
  field?: string
  header?: string
  pattern?: string
}

export interface CloudPricingRules {
  charges: Record<string, number>
  tracks: {
    label: string
    factor: number
    charge_factors?: Record<string, number>
    triggers: CloudPricingTrigger[]
  }[]
}

const MODEL_PRICING_SERVICE = 'codeswitch/services.ModelPricingService'
export const MODEL_PRICING_CHANGED_EVENT = 'model-pricing:changed'

export type ModelPricingChangedAction = 'upsert' | 'delete' | 'sync'

export interface ModelPricingChangedEventPayload {
  action: ModelPricingChangedAction
  model?: string
  syncedAt?: string
  timestamp: number
}

export interface ModelPricingRow {
  cloud_pricing?: CloudPricingRules
  original_model?: string
  model: string
  input_cost_per_token: number
  output_cost_per_token: number
  output_cost_per_reasoning_token: number
  cache_creation_input_token_cost: number
  has_cache_creation_input_token_cost?: boolean
  cache_read_input_token_cost: number
  has_cache_read_input_token_cost?: boolean
  ephemeral_1h_cost_per_token: number
  group_multiplier: number
  is_override: boolean
  is_custom: boolean
  source?: 'builtin' | 'manual' | 'claude_sync' | 'cloud_sync' | string
  source_updated_at?: string
}

export interface ModelPricingSyncResult {
  provider: string
  synced_at: string
  total_models: number
  created_models: number
  updated_models: number
  changed_models: number
  unchanged_models: number
  unrecognized_models?: string[]
  skipped_manual_models?: string[]
}

export interface ClaudeOfficialPricingPreviewRow {
  display_name: string
  target_models?: string[]
  input_cost_per_token: number
  output_cost_per_token: number
  cache_creation_input_token_cost: number
  cache_read_input_token_cost: number
  ephemeral_1h_cost_per_token: number
  group_multiplier?: number
  is_recognized: boolean
}

export interface ClaudeOfficialPricingPreviewResult {
  provider: string
  fetched_at: string
  rows: ClaudeOfficialPricingPreviewRow[]
  unrecognized_models?: string[]
}

export interface CloudPriceTableConflictPricing {
  cloud_pricing?: CloudPricingRules
  input_cost_per_token: number
  output_cost_per_token: number
  output_cost_per_reasoning_token: number
  cache_creation_input_token_cost: number
  cache_read_input_token_cost: number
  ephemeral_1h_cost_per_token: number
  group_multiplier: number
}

export interface CloudPriceTableSyncConflictRow {
  model: string
  display_name?: string
  litellm_provider?: string
  mode?: string
  current: CloudPriceTableConflictPricing
  incoming: CloudPriceTableConflictPricing
}

export interface CloudPriceTableSyncConflictResult {
  provider: string
  fetched_at: string
  conflicts: CloudPriceTableSyncConflictRow[]
}

const emitModelPricingChanged = async (payload: ModelPricingChangedEventPayload) => {
  try {
    await Events.Emit(MODEL_PRICING_CHANGED_EVENT, payload)
  } catch (error) {
    console.warn('failed to emit model pricing changed event', error)
  }
}

export const listModelPricing = async (): Promise<ModelPricingRow[]> => {
  const result = await Call.ByName(`${MODEL_PRICING_SERVICE}.ListModelPricing`)
  return (result ?? []) as ModelPricingRow[]
}

export const upsertModelPricing = async (row: ModelPricingRow): Promise<void> => {
  await Call.ByName(`${MODEL_PRICING_SERVICE}.UpsertModelPricing`, row)
  await emitModelPricingChanged({
    action: 'upsert',
    model: row.model,
    timestamp: Date.now(),
  })
}

export const deleteModelPricing = async (model: string): Promise<void> => {
  await Call.ByName(`${MODEL_PRICING_SERVICE}.DeleteModelPricing`, model)
  await emitModelPricingChanged({
    action: 'delete',
    model,
    timestamp: Date.now(),
  })
}

export const syncClaudeOfficialPricing = async (): Promise<ModelPricingSyncResult> => {
  const result = await Call.ByName(`${MODEL_PRICING_SERVICE}.SyncClaudeOfficialPricing`) as ModelPricingSyncResult
  await emitModelPricingChanged({
    action: 'sync',
    syncedAt: result?.synced_at,
    timestamp: Date.now(),
  })
  return result
}

export const previewCloudPriceTableSyncConflicts = async (): Promise<CloudPriceTableSyncConflictResult> => {
  return Call.ByName(`${MODEL_PRICING_SERVICE}.PreviewCloudPriceTableSyncConflicts`)
}

export const syncCloudPriceTable = async (overwriteManualModels: string[] = []): Promise<ModelPricingSyncResult> => {
  const result = await Call.ByName(
    `${MODEL_PRICING_SERVICE}.SyncCloudPriceTable`,
    overwriteManualModels,
  ) as ModelPricingSyncResult
  await emitModelPricingChanged({
    action: 'sync',
    syncedAt: result?.synced_at,
    timestamp: Date.now(),
  })
  return result
}

export const previewClaudeOfficialPricing = async (): Promise<ClaudeOfficialPricingPreviewResult> => {
  return Call.ByName(`${MODEL_PRICING_SERVICE}.PreviewClaudeOfficialPricing`)
}
