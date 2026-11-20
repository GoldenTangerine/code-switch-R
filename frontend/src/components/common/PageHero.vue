<!--
 * @name: 页面标题区
 * @Descripttion: 工作区页面统一的左对齐标题与说明文案，控制字号、行宽与换行节奏
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 16:20:00
 * @LastEditTime: 2026-11-20 16:20:00
 * @FilePath: frontend/src/components/common/PageHero.vue
-->
<template>
  <header class="page-hero">
    <h1 class="page-hero__title">{{ title }}</h1>
    <p v-if="lead" class="page-hero__lead">{{ lead }}</p>
  </header>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  lead?: string
}>()
</script>

<style scoped>
.page-hero {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 4px 0 4px;
}

.page-hero__title {
  margin: 0;
  font-size: clamp(26px, 2.6vw, 32px);
  font-weight: 700;
  letter-spacing: -0.01em;
  line-height: 1.2;
  color: var(--mac-text);
}

/* 以 em 计行宽（中文字宽约 1em），常规窗口下单行展示；换行时用 balance 均分两行，避免末行孤字 */
.page-hero__lead {
  margin: 0;
  max-width: 56em;
  font-size: 14px;
  line-height: 1.65;
  color: var(--mac-text-secondary);
  text-wrap: balance;
}
</style>
