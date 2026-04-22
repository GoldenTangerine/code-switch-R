<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import { computed } from 'vue'
import Sidebar from './components/Sidebar.vue'
import UpdateNotification from './components/common/UpdateNotification.vue'

const route = useRoute()
const isTray = computed(() => route.path === '/tray')
</script>

<template>
  <div v-if="isTray" class="tray-layout">
    <RouterView v-slot="{ Component }">
      <component :is="Component" />
    </RouterView>
  </div>
  <div v-else class="app-layout">
    <div class="app-layout__glow" aria-hidden="true"></div>
    <Sidebar />
    <main class="main-content">
      <RouterView v-slot="{ Component }">
        <keep-alive>
          <component :is="Component" />
        </keep-alive>
      </RouterView>
    </main>
    <!-- 更新通知弹窗 -->
    <UpdateNotification />
  </div>
</template>

<style scoped>
.tray-layout {
  width: 100vw;
  height: 100vh;
  overflow: hidden;
}

.app-layout {
  position: relative;
  display: flex;
  gap: 10px;
  height: 100vh;
  width: 100vw;
  padding: 14px;
  box-sizing: border-box;
  overflow: hidden;
  background: var(--app-background);
}

.app-layout__glow {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    radial-gradient(circle at 14% 16%, rgba(59, 130, 246, 0.12) 0%, rgba(59, 130, 246, 0) 28%),
    radial-gradient(circle at 88% 12%, rgba(56, 189, 248, 0.08) 0%, rgba(56, 189, 248, 0) 24%);
}

.main-content {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  position: relative;
  border-radius: 30px;
  border: 1px solid color-mix(in srgb, var(--mac-border) 95%, transparent);
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--mac-surface) 70%, transparent) 0%, color-mix(in srgb, var(--mac-surface-strong) 78%, transparent) 100%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 24px 54px rgba(2, 6, 23, 0.18);
  backdrop-filter: blur(18px) saturate(140%);
  -webkit-backdrop-filter: blur(18px) saturate(140%);
}

.main-content::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    radial-gradient(circle at top center, rgba(99, 102, 241, 0.12) 0%, rgba(99, 102, 241, 0) 42%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.02) 0%, rgba(255, 255, 255, 0) 18%);
}

@media (max-width: 960px) {
  .app-layout {
    flex-direction: column;
    gap: 12px;
    padding: 12px;
  }
}
</style>
