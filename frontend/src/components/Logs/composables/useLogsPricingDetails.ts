import { computed, ref } from 'vue'
import { listModelPricing, type ModelPricingRow } from '../../../services/modelPricing'
import type { RequestLog } from '../../../services/logs'
import type { CostTooltipDetail, LogInfoTooltipDetail } from '../types'
import { COST_TOOLTIP_DIFF_EPSILON } from '../constants'
import {
  buildBuiltinTokenFormula,
  buildBuiltinTokenPricingContext,
  buildLogsCostTooltipLabels,
  buildLogsInfoTooltipLabels,
  buildModelInfoTooltipDetailData,
  buildModelPricingLookup,
  buildObservedCostPriceLines,
  buildProviderApiPerCallPriceLines,
  buildProviderApiTokenFormula,
  buildProviderApiTokenPricingContext,
  buildTokenRatePriceLines,
  buildVerifyInfoTooltipDetailData,
  formatCacheCreateTierLabel,
  formatUsdPrecise,
  hasProviderPricingSnapshot,
  mergeCostTooltipNotes,
  resolvePriceSource,
  resolvePricingRow,
  safeNumber,
} from '../utils'

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

type UseLogsPricingDetailsOptions = {
  t: TranslateFn
}

export function useLogsPricingDetails(options: UseLogsPricingDetailsOptions) {
  const { t } = options

  const modelPricingRows = ref<ModelPricingRow[]>([])
  const modelPricingLoaded = ref(false)
  let modelPricingLoadingTask: Promise<void> | null = null

  const modelPricingIndex = computed(() => buildModelPricingLookup(modelPricingRows.value))

  const buildInfoTooltipLabels = () => buildLogsInfoTooltipLabels(t)
  const buildCostTooltipLabels = () => buildLogsCostTooltipLabels(t, formatCacheCreateTierLabel)

  const buildProviderAPITokenTooltipDetail = (
    item: RequestLog,
    fallbackModelName: string,
    recordedCost: number,
  ): CostTooltipDetail | null => {
    const pricingContext = buildProviderApiTokenPricingContext(item)
    if (!pricingContext) return null

    const {
      reasoningTokens,
      cacheReadTokens,
      inputPerToken,
      outputPerToken,
      reasoningPerToken,
      cacheReadPerToken,
      cacheCreateRates,
      calculatedTotal,
    } = pricingContext

    const labels = buildCostTooltipLabels()
    const priceLines = buildTokenRatePriceLines({
      inputPerToken,
      outputPerToken,
      reasoningPerToken,
      cacheCreateRates,
      cacheReadPerToken,
      includeCacheRead: cacheReadTokens > 0,
      includeReasoning: reasoningTokens > 0,
      includeCacheMultiplierHint: true,
    }, labels.tokenRatePriceLineLabels)

    const formula = buildProviderApiTokenFormula(
      pricingContext,
      labels.tokenFormulaLabels,
      labels.providerApiFormula,
    )
    const recordedCostHint =
      Math.abs(calculatedTotal - recordedCost) > COST_TOOLTIP_DIFF_EPSILON
        ? labels.recordedCostHint(formatUsdPrecise(recordedCost))
        : ''

    return {
      pricingModel: fallbackModelName,
      hasPricing: true,
      priceLines,
      formula,
      note: labels.providerApiHint,
      recordedCostHint,
    }
  }

  const buildBuiltinCostTooltipDetail = (
    item: RequestLog,
    fallbackModelName: string,
    recordedCost: number,
  ): CostTooltipDetail => {
    const pricingRow = resolvePricingRow(item, modelPricingIndex.value, modelPricingRows.value)
    const modelName = String(pricingRow?.model ?? fallbackModelName).trim() || '—'
    const labels = buildCostTooltipLabels()

    if (!pricingRow) {
      return {
        pricingModel: modelName,
        hasPricing: false,
        priceLines: [],
        formula: labels.noPricingFormula,
        note: labels.noPricingHint,
        recordedCostHint: labels.recordedCostHint(formatUsdPrecise(recordedCost)),
      }
    }

    const pricingContext = buildBuiltinTokenPricingContext(item, pricingRow, fallbackModelName)
    const {
      modelName: resolvedModelName,
      matchedModelChanged,
      reasoningTokens,
      cacheReadTokens,
      inputPerToken,
      outputPerToken,
      reasoningPerToken,
      cacheReadPerToken,
      cacheCreateRates,
      calculatedTotal,
    } = pricingContext

    const priceLines = buildTokenRatePriceLines({
      inputPerToken,
      outputPerToken,
      reasoningPerToken,
      cacheCreateRates,
      cacheReadPerToken,
      includeCacheRead: cacheReadTokens > 0,
      includeReasoning: reasoningTokens > 0,
      includeCacheMultiplierHint: true,
    }, labels.tokenRatePriceLineLabels)

    const formula = buildBuiltinTokenFormula(pricingContext, labels.tokenFormulaLabels)
    const note = matchedModelChanged ? labels.matchedModelHint(resolvedModelName) : ''
    const recordedCostHint =
      Math.abs(calculatedTotal - recordedCost) > COST_TOOLTIP_DIFF_EPSILON
        ? labels.recordedCostHint(formatUsdPrecise(recordedCost))
        : ''

    return {
      pricingModel: resolvedModelName,
      hasPricing: true,
      priceLines,
      formula,
      note,
      recordedCostHint,
    }
  }

  const buildCostTooltipDetail = (item: RequestLog): CostTooltipDetail => {
    const source = resolvePriceSource(item)
    const fallbackModelName = String(item.matched_pricing_model ?? item.model ?? '').trim() || '—'
    const recordedCost = safeNumber(item.total_cost)
    const providerSnapshotAvailable = hasProviderPricingSnapshot(item)
    const shouldAvoidFallbackEstimate =
      !providerSnapshotAvailable && recordedCost <= COST_TOOLTIP_DIFF_EPSILON
    const labels = buildCostTooltipLabels()
    const recordedCostHint = labels.recordedCostHint(formatUsdPrecise(recordedCost))

    if (source === 'provider_api') {
      const providerTokenDetail = buildProviderAPITokenTooltipDetail(item, fallbackModelName, recordedCost)
      if (providerTokenDetail) {
        return providerTokenDetail
      }

      const providerPerCallLines = buildProviderApiPerCallPriceLines(item, labels.providerApiPerCallPriceLineLabels)
      if (providerPerCallLines.length > 0) {
        return {
          pricingModel: fallbackModelName,
          hasPricing: true,
          priceLines: providerPerCallLines,
          formula: labels.providerApiPerCallFormula,
          note: labels.providerApiHint,
          recordedCostHint,
        }
      }

      if (shouldAvoidFallbackEstimate) {
        return {
          pricingModel: fallbackModelName,
          hasPricing: false,
          priceLines: [],
          formula: labels.providerApiFormula,
          note: mergeCostTooltipNotes(labels.providerApiHint, labels.providerApiZeroCostHint),
          recordedCostHint,
        }
      }

      const observedPriceLines = buildObservedCostPriceLines(item, {
        ...labels.tokenRatePriceLineLabels,
        suffix: labels.observedPriceSuffix,
      })
      if (observedPriceLines.length > 0) {
        return {
          pricingModel: fallbackModelName,
          hasPricing: true,
          priceLines: observedPriceLines,
          formula: labels.providerApiFormula,
          note: providerSnapshotAvailable
            ? labels.providerApiHint
            : mergeCostTooltipNotes(labels.providerApiHint, labels.providerApiFallbackHint),
          recordedCostHint,
        }
      }

      const builtinFallbackDetail = buildBuiltinCostTooltipDetail(item, fallbackModelName, recordedCost)
      if (builtinFallbackDetail.hasPricing) {
        return {
          ...builtinFallbackDetail,
          note: mergeCostTooltipNotes(labels.providerApiFallbackHint, builtinFallbackDetail.note),
          recordedCostHint,
        }
      }

      return {
        pricingModel: fallbackModelName,
        hasPricing: false,
        priceLines: [],
        formula: labels.providerApiFormula,
        note: labels.providerApiHint,
        recordedCostHint,
      }
    }

    return buildBuiltinCostTooltipDetail(item, fallbackModelName, recordedCost)
  }

  const buildModelInfoTooltipDetail = (item: RequestLog): LogInfoTooltipDetail => {
    const costDetail = buildCostTooltipDetail(item)
    const matchedModel = String(costDetail.pricingModel ?? item.matched_pricing_model ?? '').trim()
    const currentModel = String(item.model ?? '').trim()
    return buildModelInfoTooltipDetailData({
      source: resolvePriceSource(item),
      matchedModel,
      currentModel,
      costDetail,
      recordedCost: safeNumber(item.total_cost),
    }, buildInfoTooltipLabels())
  }

  const buildVerifyInfoTooltipDetail = (item: RequestLog): LogInfoTooltipDetail =>
    buildVerifyInfoTooltipDetailData({
      requestedModel: item.requested_model,
      responseModel: item.response_model,
    }, buildInfoTooltipLabels())

  const loadModelPricingRows = async () => {
    if (modelPricingLoaded.value) return
    if (modelPricingLoadingTask) {
      await modelPricingLoadingTask
      return
    }

    modelPricingLoadingTask = (async () => {
      try {
        modelPricingRows.value = (await listModelPricing()) ?? []
        modelPricingLoaded.value = true
      } catch (error) {
        console.error('failed to load model pricing rows', error)
      } finally {
        modelPricingLoadingTask = null
      }
    })()

    await modelPricingLoadingTask
  }

  return {
    modelPricingLoaded,
    loadModelPricingRows,
    buildCostTooltipDetail,
    buildModelInfoTooltipDetail,
    buildVerifyInfoTooltipDetail,
  }
}
