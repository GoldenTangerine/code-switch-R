<!--
 * @name: 分段切换标签
 * @Descripttion: 轻量分段式 Tab，使用 grid 自适应宽度，选中项由父组件控制，便于切换前做未保存拦截
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 16:20:00
 * @LastEditTime: 2026-11-20 16:20:00
 * @FilePath: frontend/src/components/common/SegmentedTabs.vue
-->
<template>
  <div class="segmented" role="tablist" :aria-label="ariaLabel">
    <button
      v-for="option in options"
      :key="option.id"
      type="button"
      role="tab"
      :class="['segmented-tab', { active: option.id === active }]"
      :aria-selected="option.id === active"
      @click="emit('select', option.id)"
    >{{ option.label }}</button>
  </div>
</template>

<script setup lang="ts">
export interface SegmentedOption {
  id: string
  label: string
}

defineProps<{
  options: readonly SegmentedOption[]
  active: string
  ariaLabel?: string
}>()

const emit = defineEmits<{
  select: [id: string]
}>()
</script>

<style scoped>
/* grid 列宽按内容计算，规避 WebKit 下 flex + max-content 导致末项溢出的问题 */
.segmented {
  display: inline-grid;
  grid-auto-flow: column;
  grid-auto-columns: max-content;
  gap: 2px;
  padding: 3px;
  border-radius: 12px;
  border: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface) 72%, transparent);
  max-width: 100%;
}

/* 显式声明宽高与行高，覆盖 public/style.css 中 button 的固定 60×30 尺寸 */
.segmented-tab {
  appearance: none;
  width: auto;
  height: auto;
  border: 0;
  margin: 0;
  padding: 7px 16px;
  border-radius: 9px;
  background: transparent;
  color: var(--mac-text-secondary);
  font-size: 13px;
  font-weight: 600;
  line-height: 1.25;
  white-space: nowrap;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease, box-shadow 0.18s ease;
}

.segmented-tab:hover:not(.active) {
  color: var(--mac-text);
  background: color-mix(in srgb, var(--mac-text) 6%, transparent);
}

.segmented-tab.active {
  background: var(--mac-accent);
  color: #fff;
  box-shadow:
    0 1px 2px rgba(15, 23, 42, 0.18),
    0 4px 12px color-mix(in srgb, var(--mac-accent) 28%, transparent);
}

.segmented-tab:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--mac-accent) 60%, transparent);
  outline-offset: 2px;
}
</style>
