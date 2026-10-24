<!--
@name: 供应商工具兼容编辑器
@Descripttion: 配置供应商独立的工具禁用规则和动态工具兼容选项。
@version: 1.0.0
@Author: sm
@Date: 2026-10-24 23:20:14
@LastEditTime: 2026-10-24 23:20:14
@FilePath: frontend/src/components/Main/components/ProviderToolCompatibilityEditor.vue
-->
<template>
  <section class="tool-compatibility" :class="{ 'theme-dark': resolvedTheme === 'dark' }">
    <header class="compatibility-header">
      <div>
        <h3>{{ t(`${prefix}.title`) }}</h3>
        <p>{{ t(`${prefix}.description`) }}</p>
      </div>
      <button type="button" class="preset-button" @click="applyAdvisorPreset">
        {{ t(`${prefix}.advisorPreset`) }}
      </button>
    </header>
    <div class="rule-grid">
      <label class="rule-field">
        <span>{{ t(`${prefix}.names`) }}</span>
        <textarea v-model="namesText" rows="3" spellcheck="false" placeholder="advisor" @input="updateRules('disabledNames', ($event.target as HTMLTextAreaElement).value)" />
      </label>
      <label class="rule-field">
        <span>{{ t(`${prefix}.types`) }}</span>
        <textarea v-model="typesText" rows="3" spellcheck="false" placeholder="advisor_20260301" @input="updateRules('disabledTypes', ($event.target as HTMLTextAreaElement).value)" />
      </label>
    </div>
    <p class="rule-hint">{{ t(`${prefix}.matchHint`) }}</p>
    <label class="dynamic-option">
      <span class="dynamic-copy">
        <strong>{{ t(`${prefix}.dynamicLabel`) }}</strong>
        <span>{{ t(`${prefix}.dynamicHint`) }}</span>
      </span>
      <span class="mac-switch">
        <input type="checkbox" :checked="modelValue?.normalizeDynamicTools === true" @change="updateDynamic(($event.target as HTMLInputElement).checked)" />
        <span class="slider" />
      </span>
    </label>
    <p class="scope-note">{{ t(`${prefix}.scopeHint`) }}</p>
  </section>
</template>

<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProviderToolCompatibility } from '../../../data/cards'
import { getResolvedTheme, onThemeChange } from '../../../utils/ThemeManager'

const props = defineProps<{ modelValue?: ProviderToolCompatibility }>()
const emit = defineEmits<{ 'update:modelValue': [value: ProviderToolCompatibility] }>()
const { t } = useI18n()
const prefix = 'components.main.form.toolCompatibility'
const resolvedTheme = ref(getResolvedTheme())
const stopThemeListener = onThemeChange((detail) => { resolvedTheme.value = detail.resolvedTheme })
onUnmounted(stopThemeListener)
const namesText = ref('')
const typesText = ref('')
const parseRules = (value: string) => [...new Set(value.split(/[,，\n]/).map((item) => item.trim()).filter(Boolean))]

watch(() => props.modelValue, (value) => {
  for (const [key, draft] of [['disabledNames', namesText], ['disabledTypes', typesText]] as const) {
    const rules = value?.[key] || []
    if (JSON.stringify(parseRules(draft.value)) !== JSON.stringify(rules)) draft.value = rules.join('\n')
  }
}, { immediate: true, deep: true })

const updateRules = (key: 'disabledNames' | 'disabledTypes', text: string) => {
  emit('update:modelValue', { ...props.modelValue, [key]: parseRules(text) })
}
const updateDynamic = (enabled: boolean) => {
  emit('update:modelValue', { ...props.modelValue, normalizeDynamicTools: enabled })
}
const applyAdvisorPreset = () => {
  emit('update:modelValue', {
    ...props.modelValue,
    disabledNames: [...new Set([...(props.modelValue?.disabledNames || []), 'advisor'])],
    disabledTypes: [...new Set([...(props.modelValue?.disabledTypes || []), 'advisor_20260301'])],
    normalizeDynamicTools: true,
  })
}
</script>

<style scoped>
.tool-compatibility {
  border: 1px solid var(--mac-border);
  border-radius: 12px;
  padding: 16px;
  color: var(--mac-text);
  background: color-mix(in srgb, var(--mac-surface) 78%, transparent);
}
.tool-compatibility.theme-dark { background: color-mix(in srgb, var(--mac-surface) 90%, transparent); }
.compatibility-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
h3 { margin: 0; font-size: 13px; font-weight: 600; }
p { margin: 6px 0 0; font-size: 12px; line-height: 1.6; color: var(--mac-text-secondary); }
.preset-button { flex-shrink: 0; padding: 6px 10px; border: 1px solid color-mix(in srgb, var(--mac-accent) 35%, var(--mac-border)); border-radius: 7px; background: color-mix(in srgb, var(--mac-accent) 8%, transparent); color: var(--mac-accent); font-size: 12px; cursor: pointer; }
.preset-button:hover { background: color-mix(in srgb, var(--mac-accent) 15%, transparent); }
.rule-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; margin-top: 16px; }
.rule-field { display: flex; flex-direction: column; gap: 8px; min-width: 0; font-size: 12px; }
textarea { box-sizing: border-box; width: 100%; min-height: 76px; resize: vertical; padding: 9px 11px; border: 1px solid var(--mac-border); border-radius: 8px; background: var(--mac-surface); color: var(--mac-text); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; line-height: 1.6; }
textarea::placeholder { color: var(--mac-text-secondary); opacity: 0.65; }
textarea:focus-visible, button:focus-visible { outline: 2px solid var(--mac-accent); outline-offset: 2px; }
.rule-hint { margin-top: 8px; }
.dynamic-option { display: flex; align-items: center; justify-content: space-between; gap: 18px; margin-top: 14px; padding-top: 14px; border-top: 1px solid var(--mac-border); cursor: pointer; }
.dynamic-copy { display: flex; flex-direction: column; gap: 5px; font-size: 12px; line-height: 1.6; }
.dynamic-copy strong { font-weight: 500; }
.dynamic-copy > span { color: var(--mac-text-secondary); }
.mac-switch { flex-shrink: 0; position: relative; }
.mac-switch input { display: block; position: absolute; inset: 0; width: 100%; height: 100%; opacity: 0; margin: 0; }
.mac-switch input:focus-visible + span { outline: 2px solid var(--mac-accent); outline-offset: 3px; }
.scope-note { margin-top: 12px; }
@media (max-width: 600px) {
  .rule-grid { grid-template-columns: 1fr; }
  .compatibility-header { flex-wrap: wrap; gap: 10px; }
}
</style>
