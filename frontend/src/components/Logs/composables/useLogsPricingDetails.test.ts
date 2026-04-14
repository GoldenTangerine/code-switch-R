import { describe, expect, it, vi, beforeEach } from 'vitest'
import type { RequestLog } from '../../../services/logs'
import { useLogsPricingDetails } from './useLogsPricingDetails'

vi.mock('../../../services/modelPricing', () => ({
  listModelPricing: vi.fn(),
}))

import { listModelPricing } from '../../../services/modelPricing'

function createTranslate() {
  return (key: string, params?: Record<string, unknown>) => {
    switch (key) {
      case 'components.logs.costTooltip.promptPrice':
        return '输入'
      case 'components.logs.costTooltip.completionPrice':
        return '输出'
      case 'components.logs.costTooltip.cacheCreatePrice':
        return '缓存创建'
      case 'components.logs.costTooltip.cacheCreatePriceWithTtl':
        return `缓存创建（${String(params?.ttl ?? '')}）`
      case 'components.logs.costTooltip.cacheReadPrice':
        return '缓存读取'
      case 'components.logs.costTooltip.reasoningPrice':
        return '推理'
      case 'components.logs.costTooltip.perCallUnifiedPrice':
        return '统一按次'
      case 'components.logs.costTooltip.perCallInputPrice':
        return '输入按次'
      case 'components.logs.costTooltip.perCallOutputPrice':
        return '输出按次'
      case 'components.logs.costTooltip.perRequestSuffix':
        return '/ 次请求'
      case 'components.logs.costTooltip.usagePrompt':
        return '输入用量'
      case 'components.logs.costTooltip.usageCompletion':
        return '输出用量'
      case 'components.logs.costTooltip.usageReasoning':
        return '推理用量'
      case 'components.logs.costTooltip.usageCacheRead':
        return '缓存读取用量'
      case 'components.logs.costTooltip.usageCacheCreate':
        return '缓存创建用量'
      case 'components.logs.costTooltip.usageCacheCreateWithTtl':
        return `缓存创建用量（${String(params?.ttl ?? '')}）`
      case 'components.logs.costTooltip.formulaEmpty':
        return '暂无公式'
      case 'components.logs.costTooltip.cacheCreateMultiplierLabel':
        return `缓存创建倍率 ${String(params?.multiplier ?? '')}`
      case 'components.logs.costTooltip.cacheCreateMultiplierLabelWithTtl':
        return `缓存创建倍率（${String(params?.ttl ?? '')}） ${String(params?.multiplier ?? '')}`
      case 'components.logs.costTooltip.cacheReadMultiplierLabel':
        return `缓存读取倍率 ${String(params?.multiplier ?? '')}`
      case 'components.logs.costTooltip.groupMultiplierLabel':
        return `分组倍率 ${String(params?.multiplier ?? '')}`
      case 'components.logs.costTooltip.observedPriceSuffix':
        return '观测值'
      case 'components.logs.costTooltip.providerApiFormula':
        return '接口计价'
      case 'components.logs.costTooltip.providerApiPerCallFormula':
        return '接口按次计价'
      case 'components.logs.costTooltip.providerApiHint':
        return '接口提示'
      case 'components.logs.costTooltip.providerApiFallbackHint':
        return '接口回退提示'
      case 'components.logs.costTooltip.providerApiZeroCostHint':
        return '接口零成本提示'
      case 'components.logs.costTooltip.noPricingFormula':
        return '未命中可用价格表，无法按价格表拆解计算。'
      case 'components.logs.costTooltip.noPricingHint':
        return '当前仅展示日志记录金额。'
      case 'components.logs.costTooltip.recordedCostHint':
        return `日志记录金额：${String(params?.cost ?? '')}`
      case 'components.logs.costTooltip.matchedModelHint':
        return `本条日志按模型 ${String(params?.model ?? '')} 的价格计算。`
      default:
        return key
    }
  }
}

function createRequestLog(model: string): RequestLog {
  return {
    id: 1,
    platform: 'claude',
    model,
    provider: 'test-provider',
    http_code: 200,
    input_tokens: 1000,
    output_tokens: 500,
    cache_create_tokens: 0,
    cache_read_tokens: 0,
    reasoning_tokens: 0,
    created_at: '2026-04-14 16:00:00',
    total_cost: 0.01,
    price_source: 'builtin',
  }
}

describe('useLogsPricingDetails', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('reloads pricing rows after they are marked stale', async () => {
    vi.mocked(listModelPricing)
      .mockResolvedValueOnce([
        {
          model: 'qwen3.6',
          input_cost_per_token: 1.173 / 1_000_000,
          output_cost_per_token: 7 / 1_000_000,
          output_cost_per_reasoning_token: 0,
          cache_creation_input_token_cost: 1.467 / 1_000_000,
          cache_read_input_token_cost: 0.1174 / 1_000_000,
          ephemeral_1h_cost_per_token: 2 / 1_000_000,
          group_multiplier: 1,
          is_override: true,
          is_custom: true,
          source: 'manual',
        },
      ])
      .mockResolvedValueOnce([
        {
          model: 'qwen3.6',
          input_cost_per_token: 1.173 / 1_000_000,
          output_cost_per_token: 8 / 1_000_000,
          output_cost_per_reasoning_token: 0,
          cache_creation_input_token_cost: 1.467 / 1_000_000,
          cache_read_input_token_cost: 0.1174 / 1_000_000,
          ephemeral_1h_cost_per_token: 2 / 1_000_000,
          group_multiplier: 1,
          is_override: true,
          is_custom: true,
          source: 'manual',
        },
      ])

    const { loadModelPricingRows, buildCostTooltipDetail, modelPricingStale, markModelPricingStale } = useLogsPricingDetails({
      t: createTranslate(),
    })
    const item = createRequestLog('qwen3.6')

    await loadModelPricingRows()
    const initialDetail = buildCostTooltipDetail(item)
    expect(initialDetail.hasPricing).toBe(true)
    expect(initialDetail.pricingModel).toBe('qwen3.6')
    expect(initialDetail.priceLines.some((line) => line.value.includes('$7.00'))).toBe(true)
    expect(modelPricingStale.value).toBe(false)

    markModelPricingStale()
    expect(modelPricingStale.value).toBe(true)

    await loadModelPricingRows()
    const refreshedDetail = buildCostTooltipDetail(item)
    expect(refreshedDetail.hasPricing).toBe(true)
    expect(refreshedDetail.pricingModel).toBe('qwen3.6')
    expect(refreshedDetail.priceLines.some((line) => line.value.includes('$8.00'))).toBe(true)
    expect(modelPricingStale.value).toBe(false)
    expect(listModelPricing).toHaveBeenCalledTimes(2)
  })
})
