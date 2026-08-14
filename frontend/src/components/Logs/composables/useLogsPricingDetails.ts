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
  hasBreakdownCostPayload,
  hasProviderPricingSnapshot,
  hasStoredCostSnapshot,
  isProviderPerCallValueSet,
  mergeCostTooltipNotes,
  resolveLogPricingModelName,
  resolvePriceSource,
  resolveGroupMultiplier,
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
  const modelPricingStale = ref(false)
  let modelPricingLoadingTask: Promise<void> | null = null

  const modelPricingIndex = computed(() => buildModelPricingLookup(modelPricingRows.value))

  const buildInfoTooltipLabels = () => buildLogsInfoTooltipLabels(t)
  const buildCostTooltipLabels = () => buildLogsCostTooltipLabels(t, formatCacheCreateTierLabel)

  const buildProviderAPIPerCallFormula = (item: RequestLog, recordedCost: number) => {
    const labels = buildCostTooltipLabels()
    const parts: string[] = []
    const perRequestSuffix = labels.providerApiPerCallPriceLineLabels.perRequestSuffix
    const groupMultiplier = resolveGroupMultiplier(item)

    if (isProviderPerCallValueSet(item.provider_per_call_unified, item.provider_per_call_unified_set)) {
      parts.push(`${formatUsdPrecise(safeNumber(item.provider_per_call_unified))} ${perRequestSuffix}`)
    } else {
      if (isProviderPerCallValueSet(item.provider_per_call_input, item.provider_per_call_input_set)) {
        parts.push(`${formatUsdPrecise(safeNumber(item.provider_per_call_input))} ${perRequestSuffix}`)
      }
      if (isProviderPerCallValueSet(item.provider_per_call_output, item.provider_per_call_output_set)) {
        parts.push(`${formatUsdPrecise(safeNumber(item.provider_per_call_output))} ${perRequestSuffix}`)
      }
    }

    if (parts.length === 0) return labels.providerApiPerCallFormula
    const baseFormula = parts.length > 1 ? `(${parts.join(' + ')})` : parts[0]
    if (groupMultiplier !== 1) {
      return `${baseFormula} * ${labels.tokenFormulaLabels.groupMultiplierLabel(groupMultiplier)} = ${formatUsdPrecise(recordedCost)}`
    }
    return `${baseFormula} = ${formatUsdPrecise(recordedCost)}`
  }

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
    const labels = buildCostTooltipLabels()

    if (hasStoredCostSnapshot(item) && !hasBreakdownCostPayload(item)) {
      return {
        pricingModel: fallbackModelName,
        hasPricing: false,
        priceLines: [],
        formula: labels.noPricingFormula,
        note: labels.noPricingHint,
        recordedCostHint: labels.recordedCostHint(formatUsdPrecise(recordedCost)),
      }
    }

    const pricingRow = resolvePricingRow(item, modelPricingIndex.value, modelPricingRows.value)
    const modelName = String(pricingRow?.model ?? fallbackModelName).trim() || '—'

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
    const fallbackModelName = resolveLogPricingModelName(item) || '—'
    const recordedCost = safeNumber(item.total_cost)
    const storedCostSnapshotAvailable = hasStoredCostSnapshot(item)
    const providerSnapshotAvailable = hasProviderPricingSnapshot(item)
    const shouldAvoidFallbackEstimate =
      !providerSnapshotAvailable && !storedCostSnapshotAvailable
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
          formula: buildProviderAPIPerCallFormula(item, recordedCost),
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

      if (storedCostSnapshotAvailable) {
        return {
          pricingModel: fallbackModelName,
          hasPricing: false,
          priceLines: [],
          formula: labels.providerApiFormula,
          note: mergeCostTooltipNotes(labels.providerApiHint, labels.noPricingHint),
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
      reasoningEffort: item.reasoning_effort,
      reasoningEffortSource: item.reasoning_effort_source,
      sessionPreferredProvider: item.session_preferred_provider,
      sessionProviderRoute: item.session_provider_route,
    }, buildInfoTooltipLabels())
  }

  const buildVerifyInfoTooltipDetail = (item: RequestLog): LogInfoTooltipDetail =>
    buildVerifyInfoTooltipDetailData({
      requestedModel: item.requested_model,
      responseModel: item.response_model,
      userAgent: item.user_agent,
    }, buildInfoTooltipLabels())

  const markModelPricingStale = () => {
    modelPricingStale.value = true
  }

  const loadModelPricingRows = async (force = false) => {
    if (modelPricingLoaded.value && !modelPricingStale.value && !force) return
    if (modelPricingLoadingTask) {
      await modelPricingLoadingTask
      return
    }

    modelPricingLoadingTask = (async () => {
      try {
        modelPricingRows.value = (await listModelPricing()) ?? []
        modelPricingLoaded.value = true
        modelPricingStale.value = false
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
    modelPricingStale,
    markModelPricingStale,
    loadModelPricingRows,
    buildCostTooltipDetail,
    buildModelInfoTooltipDetail,
    buildVerifyInfoTooltipDetail,
  }
}
