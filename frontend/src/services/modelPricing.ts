/**
 * 模型价格（input/output/cache）配置 API 封装
 */
import { Call } from '@wailsio/runtime'

const MODEL_PRICING_SERVICE = 'codeswitch/services.ModelPricingService'

export interface ModelPricingRow {
  model: string
  input_cost_per_token: number
  output_cost_per_token: number
  output_cost_per_reasoning_token: number
  cache_creation_input_token_cost: number
  cache_read_input_token_cost: number
  ephemeral_1h_cost_per_token: number
  is_override: boolean
  is_custom: boolean
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

