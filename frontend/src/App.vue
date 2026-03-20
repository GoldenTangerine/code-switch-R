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
  display: flex;
  height: 100vh;
  width: 100vw;
  overflow: hidden;
}

.main-content {
  flex: 1;
  overflow-y: auto;
  background: var(--mac-bg);
}
</style>
