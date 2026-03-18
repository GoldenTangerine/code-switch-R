/**
 * 模型价格（input/output/cache）配置 API 封装
 */
import { Call } from '@wailsio/runtime'

const MODEL_PRICING_SERVICE = 'codeswitch/services.ModelPricingService'

export interface ModelPricingRow {
  original_model?: string
  model: string
  input_cost_per_token: number
  output_cost_per_token: number
  output_cost_per_reasoning_token: number
  cache_creation_input_token_cost: number
  cache_read_input_token_cost: number
  ephemeral_1h_cost_per_token: number
  group_multiplier: number
  is_override: boolean
  is_custom: boolean
  source?: 'builtin' | 'manual' | 'claude_sync' | string
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

export const listModelPricing = async (): Promise<ModelPricingRow[]> => {
  const result = await Call.ByName(`${MODEL_PRICING_SERVICE}.ListModelPricing`)
  return (result ?? []) as ModelPricingRow[]
}

export const upsertModelPricing = async (row: ModelPricingRow): Promise<void> => {
  await Call.ByName(`${MODEL_PRICING_SERVICE}.UpsertModelPricing`, row)
}

export const deleteModelPricing = async (model: string): Promise<void> => {
  await Call.ByName(`${MODEL_PRICING_SERVICE}.DeleteModelPricing`, model)
}

export const syncClaudeOfficialPricing = async (): Promise<ModelPricingSyncResult> => {
  return Call.ByName(`${MODEL_PRICING_SERVICE}.SyncClaudeOfficialPricing`)
}

export const previewClaudeOfficialPricing = async (): Promise<ClaudeOfficialPricingPreviewResult> => {
  return Call.ByName(`${MODEL_PRICING_SERVICE}.PreviewClaudeOfficialPricing`)
}
