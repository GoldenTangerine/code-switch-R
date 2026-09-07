<!--
@name: 云端计费规则摘要
@Descripttion: 展示有序计费轨道及字段倍率。
@version: 1.0.0
@Author: sm
@Date: 2026-09-07 11:13:10
@LastEditTime: 2026-09-07 11:13:10
@FilePath: frontend/src/components/Setting/CloudPricingRulesSummary.vue
-->
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { TOKEN_PRICING_CHARGES } from '../../services/modelPricing'
import type { CloudPricingRules } from '../../services/modelPricing'

defineProps<{ rules?: CloudPricingRules }>()
const emit = defineEmits<{ (event: 'resize'): void }>()
const { t } = useI18n()

function supportedFactors(factors?: Record<string, number>) {
  return TOKEN_PRICING_CHARGES.flatMap((charge) => factors?.[charge] === undefined ? [] : [{ charge, factor: factors[charge] }])
}
</script>

<template>
  <details v-if="rules?.tracks?.length" class="cloud-rules" @click.stop @toggle="emit('resize')">
    <summary>{{ t('components.logs.table.pricingRules.title', { count: rules.tracks.length }) }}</summary>
    <ol>
      <li v-for="(track, index) in rules.tracks" :key="index">
        <strong>{{ track.label || t('components.logs.table.pricingRules.default') }}</strong>
        <span> &times;{{ track.factor }}</span>
        <div v-for="(trigger, triggerIndex) in track.triggers" :key="triggerIndex">
          {{ trigger.field === 'service_tier' ? t('components.logs.table.pricingRules.serviceTier') : trigger.header || t(`components.logs.table.pricingRules.${trigger.kind}`) }}
          <template v-if="trigger.kind === 'input_tokens_above'">{{ trigger.inclusive ? '≥' : '>' }} {{ (trigger.threshold ?? 0).toLocaleString() }}</template>
          <template v-if="trigger.pattern"> {{ trigger.pattern }}</template>
        </div>
        <div v-for="{ factor, charge } in supportedFactors(track.charge_factors)" :key="charge">
          {{ t(`components.logs.table.pricingRules.${charge}`) }} &times;{{ factor }}
        </div>
      </li>
    </ol>
  </details>
</template>

<style scoped>
.cloud-rules { margin-top: 8px; font-size: 12px; color: var(--text-secondary); overflow-wrap: anywhere; }
summary { cursor: pointer; }
ol { padding-left: 20px; margin: 6px 0; }
li + li { margin-top: 6px; }
</style>
