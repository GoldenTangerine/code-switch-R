/**
@name: 模型价格与费用展示
@Descripttion: 维护模型价格规则及请求费用展示。
@version: 1.0.0
@Author: sm
@Date: 2026-09-07 11:13:10
@LastEditTime: 2026-09-07 11:13:10
@FilePath: frontend/src/components/Logs/composables/useLogsPricingDetails.test.ts
*/
import { describe, expect, it, vi, beforeEach } from 'vitest'
import type { RequestLog } from '../../../services/logs'
import { useLogsPricingDetails } from './useLogsPricingDetails'

vi.mock('../../../services/modelPricing', async () => ({
  ...await vi.importActual<typeof import('../../../services/modelPricing')>('../../../services/modelPricing'),
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

function createRequestLog(model: string, overrides: Partial<RequestLog> = {}): RequestLog {
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
    ...overrides,
  }
}

describe('useLogsPricingDetails', () => {
  it('uses persisted mixed field prices including explicit zero without loading current prices', () => {
    const { buildCostTooltipDetail, buildModelInfoTooltipDetail } = useLogsPricingDetails({ t: createTranslate() })
    const item = createRequestLog('cloud-model', {
      price_source: 'mixed',
      pricing_snapshot: {
        track_label: 'Priority > 128K',
        unit_prices: { prompt: 0, completion: 0.000012 },
        field_sources: { prompt: 'provider_api', completion: 'cloud' },
        complete: true,
      },
    })
    const detail = buildCostTooltipDetail(item)
    expect(detail.hasPricing).toBe(true)
    expect(detail.priceLines).toHaveLength(2)
    expect(detail.priceLines[0].value).toContain('$0')
    expect(detail.priceLines[0].value).toContain('providerApi')
    expect(detail.priceLines[1].value).toContain('cloud')
    expect(detail.note).toContain('pricingRules.track')
    expect(buildModelInfoTooltipDetail(item).rows.some(row => row.value.includes('priceSourceValues.mixed'))).toBe(true)
    expect(listModelPricing).not.toHaveBeenCalled()
  })

  it('keeps known field prices visible for incomplete snapshots', () => {
    const { buildCostTooltipDetail } = useLogsPricingDetails({ t: createTranslate() })
    const detail = buildCostTooltipDetail(createRequestLog('partial-model', {
      pricing_snapshot: { unit_prices: { prompt: 0 }, field_sources: { prompt: 'cloud' }, complete: false },
    }))
    expect(detail.hasPricing).toBe(true)
    expect(detail.priceLines).toHaveLength(1)
    expect(detail.note).toContain('pricingRules.incomplete')
    const emptyDetail = buildCostTooltipDetail(createRequestLog('missing-model', {
      pricing_snapshot: { unit_prices: {}, field_sources: {}, complete: false },
    }))
    expect(emptyDetail.hasPricing).toBe(false)
    expect(emptyDetail.priceLines).toEqual([])
  })

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows session provider preference only when the log captured it', () => {
    const { buildModelInfoTooltipDetail } = useLogsPricingDetails({ t: createTranslate() })
    const detail = buildModelInfoTooltipDetail(createRequestLog('vendor-model', {
      session_preferred_provider: 'Provider B',
      session_provider_route: 'fallback',
    }))

    expect(detail.rows.find((row) => row.key === 'session-preferred-provider')).toEqual({
      key: 'session-preferred-provider',
      label: 'components.logs.table.tooltipLabels.sessionPreferredProvider',
      value: 'Provider B',
    })
    expect(detail.rows.find((row) => row.key === 'session-provider-route')).toEqual({
      key: 'session-provider-route',
      label: 'components.logs.table.tooltipLabels.sessionProviderRoute',
      value: 'components.logs.table.sessionProviderRouteValues.fallback',
    })

    const legacyDetail = buildModelInfoTooltipDetail(createRequestLog('vendor-model'))
    expect(legacyDetail.rows.some((row) => row.key.startsWith('session-'))).toBe(false)
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
    const item = createRequestLog('qwen3.6', {
      requested_model: 'qwen3.6',
      total_cost: 0,
    })

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

  it('prefers response model when resolving builtin pricing details', async () => {
    vi.mocked(listModelPricing).mockResolvedValueOnce([
      {
        model: 'ark-code-latest',
        input_cost_per_token: 1 / 1_000_000,
        output_cost_per_token: 2 / 1_000_000,
        output_cost_per_reasoning_token: 0,
        cache_creation_input_token_cost: 1.25 / 1_000_000,
        cache_read_input_token_cost: 0.1 / 1_000_000,
        ephemeral_1h_cost_per_token: 2 / 1_000_000,
        group_multiplier: 1,
        is_override: true,
        is_custom: true,
        source: 'manual',
      },
      {
        model: 'kimi-k2.6',
        input_cost_per_token: 3 / 1_000_000,
        output_cost_per_token: 9 / 1_000_000,
        output_cost_per_reasoning_token: 0,
        cache_creation_input_token_cost: 3.75 / 1_000_000,
        cache_read_input_token_cost: 0.3 / 1_000_000,
        ephemeral_1h_cost_per_token: 6 / 1_000_000,
        group_multiplier: 1,
        is_override: true,
        is_custom: true,
        source: 'manual',
      },
    ])

    const { loadModelPricingRows, buildCostTooltipDetail } = useLogsPricingDetails({
      t: createTranslate(),
    })
    const item = createRequestLog('ark-code-latest', {
      requested_model: 'ark-code-latest',
      response_model: 'kimi-k2.6',
      total_cost: 0,
    })

    await loadModelPricingRows()
    const detail = buildCostTooltipDetail(item)

    expect(detail.hasPricing).toBe(true)
    expect(detail.pricingModel).toBe('kimi-k2.6')
    expect(detail.note).toContain('kimi-k2.6')
    expect(detail.priceLines.some((line) => line.value.includes('$9.00'))).toBe(true)
  })

  it('does not rebuild formula from current pricing when a historical amount has no cost breakdown', async () => {
    vi.mocked(listModelPricing).mockResolvedValueOnce([
      {
        model: 'gpt-5.5',
        input_cost_per_token: 5 / 1_000_000,
        output_cost_per_token: 30 / 1_000_000,
        output_cost_per_reasoning_token: 0,
        cache_creation_input_token_cost: 6.25 / 1_000_000,
        cache_read_input_token_cost: 0.5 / 1_000_000,
        ephemeral_1h_cost_per_token: 6.25 / 1_000_000,
        group_multiplier: 1,
        is_override: true,
        is_custom: true,
        source: 'manual',
      },
    ])

    const { loadModelPricingRows, buildCostTooltipDetail } = useLogsPricingDetails({
      t: createTranslate(),
    })
    const item = createRequestLog('gpt-5.5', {
      requested_model: 'gpt-5.5',
      response_model: 'gpt-5.5',
      input_tokens: 10170,
      output_tokens: 919,
      cache_read_tokens: 16000,
      total_cost: 0.0239025,
      input_cost: 0,
      output_cost: 0,
      cache_read_cost: 0,
    })

    await loadModelPricingRows()
    const detail = buildCostTooltipDetail(item)

    expect(detail.hasPricing).toBe(false)
    expect(detail.priceLines).toHaveLength(0)
    expect(detail.formula).toBe('未命中可用价格表，无法按价格表拆解计算。')
    expect(detail.note).toBe('当前仅展示日志记录金额。')
    expect(detail.recordedCostHint).toBe('日志记录金额：$0.023903')
  })

  it('does not fallback to builtin formula for provider api historical amount without snapshot or breakdown', async () => {
    vi.mocked(listModelPricing).mockResolvedValueOnce([
      {
        model: 'gpt-5.5',
        input_cost_per_token: 5 / 1_000_000,
        output_cost_per_token: 30 / 1_000_000,
        output_cost_per_reasoning_token: 0,
        cache_creation_input_token_cost: 6.25 / 1_000_000,
        cache_read_input_token_cost: 0.5 / 1_000_000,
        ephemeral_1h_cost_per_token: 6.25 / 1_000_000,
        group_multiplier: 1,
        is_override: true,
        is_custom: true,
        source: 'manual',
      },
    ])

    const { loadModelPricingRows, buildCostTooltipDetail } = useLogsPricingDetails({
      t: createTranslate(),
    })
    const item = createRequestLog('gpt-5.5', {
      requested_model: 'gpt-5.5',
      response_model: 'gpt-5.5',
      price_source: 'provider_api',
      provider_pricing_available: false,
      input_tokens: 10170,
      output_tokens: 919,
      cache_read_tokens: 16000,
      total_cost: 0.0239025,
      input_cost: 0,
      output_cost: 0,
      cache_read_cost: 0,
    })

    await loadModelPricingRows()
    const detail = buildCostTooltipDetail(item)

    expect(detail.hasPricing).toBe(false)
    expect(detail.priceLines).toHaveLength(0)
    expect(detail.formula).toBe('接口计价')
    expect(detail.note).toBe('接口提示 当前仅展示日志记录金额。')
    expect(detail.recordedCostHint).toBe('日志记录金额：$0.023903')
  })

  it('keeps provider api negative historical amount as recorded snapshot', async () => {
    vi.mocked(listModelPricing).mockResolvedValueOnce([])

    const { loadModelPricingRows, buildCostTooltipDetail } = useLogsPricingDetails({
      t: createTranslate(),
    })
    const item = createRequestLog('gpt-5.5', {
      requested_model: 'gpt-5.5',
      response_model: 'gpt-5.5',
      price_source: 'provider_api',
      provider_pricing_available: false,
      total_cost: -0.0025,
      input_cost: 0,
      output_cost: 0,
      cache_read_cost: 0,
    })

    await loadModelPricingRows()
    const detail = buildCostTooltipDetail(item)

    expect(detail.hasPricing).toBe(false)
    expect(detail.priceLines).toHaveLength(0)
    expect(detail.formula).toBe('接口计价')
    expect(detail.note).toBe('接口提示 当前仅展示日志记录金额。')
    expect(detail.recordedCostHint).toBe('日志记录金额：$-0.002500')
  })

  it('does not fallback to local model when response/requested pricing model is missing', async () => {
    vi.mocked(listModelPricing).mockResolvedValueOnce([
      {
        model: 'ark-code-latest',
        input_cost_per_token: 1 / 1_000_000,
        output_cost_per_token: 2 / 1_000_000,
        output_cost_per_reasoning_token: 0,
        cache_creation_input_token_cost: 1.25 / 1_000_000,
        cache_read_input_token_cost: 0.1 / 1_000_000,
        ephemeral_1h_cost_per_token: 2 / 1_000_000,
        group_multiplier: 1,
        is_override: true,
        is_custom: true,
        source: 'manual',
      },
    ])

    const { loadModelPricingRows, buildCostTooltipDetail } = useLogsPricingDetails({
      t: createTranslate(),
    })
    const item = createRequestLog('ark-code-latest', {
      total_cost: 0,
      response_model: '',
      requested_model: '',
      matched_pricing_model: '',
    })

    await loadModelPricingRows()
    const detail = buildCostTooltipDetail(item)

    expect(detail.hasPricing).toBe(false)
    expect(detail.pricingModel).toBe('—')
    expect(detail.priceLines).toHaveLength(0)
  })
})
