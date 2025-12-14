<template>
  <Teleport to="body">
    <Transition name="fullscreen-panel-slide">
      <div v-if="open" class="panel-container">
        <!-- Header -->
        <header class="panel-header">
          <button class="back-button" type="button" aria-label="Close Panel" @click="handleClose">
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="15 18 9 12 15 6"></polyline>
            </svg>
          </button>
          <h2 class="panel-title">{{ title }}</h2>
          <div class="header-spacer"></div>
        </header>

        <!-- Main Content -->
        <main class="panel-content">
          <slot></slot>
        </main>

        <!-- Footer -->
        <footer v-if="$slots.footer" class="panel-footer">
          <slot name="footer"></slot>
        </footer>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { watch, onUnmounted } from 'vue'

const props = defineProps<{
  open: boolean
  title: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const handleClose = () => {
  emit('close')
}

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
    }
  },
)

onUnmounted(() => {
  document.body.style.overflow = ''
})
</script>

<style scoped>
.panel-container {
  position: fixed;
  inset: 0;
  z-index: 2000;
  background-color: var(--mac-surface);
  color: var(--mac-text);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.panel-header {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  padding: 12px 20px;
  border-bottom: 1px solid var(--mac-border);
  flex-shrink: 0;
  text-align: center;
}

.panel-title {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--mac-text);
  grid-column: 2;
  line-height: 1.5;
  margin: 0;
  padding: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.back-button {
  grid-column: 1;
  background: rgba(128, 128, 128, 0.1);
  border: none;
  padding: 8px;
  margin: 0;
  cursor: pointer;
  color: var(--mac-text-secondary);
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  transition: background-color 0.2s ease;
  width: 36px;
  height: 36px;
}

.back-button:hover {
  background-color: rgba(128, 128, 128, 0.2);
}

.header-spacer {
  grid-column: 3;
  width: 36px;
}

.panel-content {
  flex-grow: 1;
  overflow-y: auto;
  padding: 24px;
}

.panel-footer {
  flex-shrink: 0;
  padding: 16px 24px;
  border-top: 1px solid var(--mac-border);
  background-color: var(--mac-surface);
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.fullscreen-panel-slide-enter-active,
.fullscreen-panel-slide-leave-active {
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.fullscreen-panel-slide-enter-from,
.fullscreen-panel-slide-leave-to {
  transform: translateY(100%);
}

.fullscreen-panel-slide-enter-to,
.fullscreen-panel-slide-leave-from {
  transform: translateY(0);
}
</style>
