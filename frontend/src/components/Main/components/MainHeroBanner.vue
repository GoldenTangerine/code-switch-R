<template>
  <div>
    <div v-if="showFirstRunPrompt" class="first-run-banner">
      <div class="banner-content">
        <span class="banner-icon">💡</span>
        <span class="banner-text">{{ t('components.main.firstRun.message') }}</span>
      </div>
      <div class="banner-actions">
        <button class="banner-btn primary" type="button" @click="$emit('go-to-import-settings')">
          {{ t('components.main.firstRun.goToSettings') }}
        </button>
        <button class="banner-btn" type="button" @click="$emit('dismiss-first-run')">
          {{ t('components.main.firstRun.dismiss') }}
        </button>
      </div>
    </div>

    <section class="contrib-hero">
      <h1 v-if="showHomeTitle">{{ t('components.main.hero.title') }}</h1>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  showFirstRunPrompt: boolean
  showHomeTitle: boolean
}>()

defineEmits<{
  'go-to-import-settings': []
  'dismiss-first-run': []
}>()

const { t } = useI18n()
</script>

<style scoped>
.first-run-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  margin-bottom: 16px;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.1) 0%, rgba(147, 51, 234, 0.1) 100%);
  border: 1px solid rgba(59, 130, 246, 0.2);
  border-radius: 12px;
  gap: 16px;
}

:global(.dark) .first-run-banner {
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.15) 0%, rgba(147, 51, 234, 0.15) 100%);
  border-color: rgba(59, 130, 246, 0.3);
}

.banner-content {
  display: flex;
  align-items: center;
  gap: 10px;
}

.banner-icon {
  font-size: 18px;
}

.banner-text {
  font-size: 13px;
  color: var(--mac-text-primary);
  line-height: 1.4;
}

.banner-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.banner-btn {
  padding: 6px 12px;
  font-size: 12px;
  border-radius: 6px;
  border: 1px solid rgba(0, 0, 0, 0.1);
  background: rgba(255, 255, 255, 0.8);
  color: var(--mac-text-primary);
  cursor: pointer;
  transition: all 0.15s ease;
}

.banner-btn:hover {
  background: rgba(255, 255, 255, 1);
}

.banner-btn.primary {
  background: linear-gradient(135deg, #3b82f6 0%, #8b5cf6 100%);
  border-color: transparent;
  color: white;
}

.banner-btn.primary:hover {
  filter: brightness(1.1);
}

:global(.dark) .banner-btn {
  background: rgba(255, 255, 255, 0.1);
  border-color: rgba(255, 255, 255, 0.1);
}

:global(.dark) .banner-btn:hover {
  background: rgba(255, 255, 255, 0.15);
}

:global(.dark) .banner-btn.primary {
  background: linear-gradient(135deg, #3b82f6 0%, #8b5cf6 100%);
}
</style>
