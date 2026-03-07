<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { getUpdateState, restartApp, type UpdateState } from '../../services/update'

const { t } = useI18n()

const visible = ref(false)
const updateState = ref<UpdateState | null>(null)
const isRestarting = ref(false)
let pollInterval: ReturnType<typeof setInterval> | null = null

const version = computed(() => {
  return updateState.value?.latest_known_version || ''
})

async function checkUpdateReady() {
  try {
    const state = await getUpdateState()
    updateState.value = state

    // 当更新准备好时显示通知
    if (state.update_ready && state.latest_known_version) {
      visible.value = true
    }
  } catch (err) {
    console.error('[UpdateNotification] Failed to get update state:', err)
  }
}

async function installNow() {
  if (isRestarting.value) return

  isRestarting.value = true
  try {
    await restartApp()
  } catch (err) {
    console.error('[UpdateNotification] Failed to restart app:', err)
    isRestarting.value = false
  }
}

function dismiss() {
  visible.value = false
}

onMounted(() => {
  // 初始检查
  checkUpdateReady()

  // 每 30 秒轮询一次更新状态
  pollInterval = setInterval(checkUpdateReady, 30000)
})

onUnmounted(() => {
  if (pollInterval) {
    clearInterval(pollInterval)
    pollInterval = null
  }
})
</script>

<template>
  <Teleport to="body">
    <Transition name="update-notification-slide">
      <div
        v-if="visible"
        class="update-notification"
      >
        <div class="update-notification-card">
          <!-- 图标 -->
          <div class="update-notification-icon" aria-hidden="true">
            🎉
          </div>

          <!-- 文本内容 -->
          <div class="update-notification-content">
            <div class="update-notification-title">
              {{ t('update.newVersionReady') }}
            </div>
            <div class="update-notification-version">
              {{ version }}
            </div>
          </div>

          <!-- 操作按钮 -->
          <div class="update-notification-actions">
            <button
              type="button"
              :disabled="isRestarting"
              class="update-notification-btn update-notification-btn-primary"
              @click="installNow"
            >
              <span v-if="isRestarting" class="update-notification-loading">
                <svg class="update-notification-spinner" viewBox="0 0 24 24" fill="none">
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  />
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  />
                </svg>
                {{ t('update.installing') }}
              </span>
              <span v-else>{{ t('update.installNow') }}</span>
            </button>
            <button
              type="button"
              :disabled="isRestarting"
              class="update-notification-btn update-notification-btn-secondary"
              @click="dismiss"
            >
              {{ t('update.later') }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.update-notification {
  position: fixed;
  right: 16px;
  bottom: 16px;
  z-index: 9999;
  max-width: min(420px, calc(100vw - 32px));
}

.update-notification-card {
  display: flex;
  align-items: center;
  gap: 12px;
  border-radius: 14px;
  border: 1px solid rgba(15, 23, 42, 0.1);
  background: rgba(255, 255, 255, 0.92);
  padding: 14px;
  box-shadow: 0 18px 44px rgba(15, 23, 42, 0.2);
  backdrop-filter: blur(10px);
}

.update-notification-icon {
  width: 40px;
  height: 40px;
  flex-shrink: 0;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  background: rgba(16, 185, 129, 0.18);
}

.update-notification-content {
  min-width: 0;
  flex: 1 1 auto;
}

.update-notification-title {
  font-size: 15px;
  font-weight: 600;
  color: #111827;
}

.update-notification-version {
  margin-top: 2px;
  font-size: 12px;
  color: #6b7280;
}

.update-notification-actions {
  display: inline-flex;
  flex-shrink: 0;
  gap: 8px;
}

.update-notification-btn {
  min-height: 32px;
  border-radius: 8px;
  border: 1px solid transparent;
  padding: 0 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
  cursor: pointer;
  appearance: none;
  transition: background-color 0.2s ease, border-color 0.2s ease, color 0.2s ease, opacity 0.2s ease;
}

.update-notification-btn:focus-visible {
  outline: 2px solid rgba(14, 165, 233, 0.45);
  outline-offset: 2px;
}

.update-notification-btn-primary {
  background: #0ea5e9;
  color: #f8fafc;
}

.update-notification-btn-primary:hover:not(:disabled) {
  background: #0284c7;
}

.update-notification-btn-secondary {
  border-color: rgba(15, 23, 42, 0.14);
  background: #f3f4f6;
  color: #374151;
}

.update-notification-btn-secondary:hover:not(:disabled) {
  background: #e5e7eb;
}

.update-notification-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.update-notification-loading {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.update-notification-spinner {
  width: 12px;
  height: 12px;
  animation: update-notification-spin 0.8s linear infinite;
}

@keyframes update-notification-spin {
  to {
    transform: rotate(360deg);
  }
}

.update-notification-slide-enter-active,
.update-notification-slide-leave-active {
  transition: transform 0.24s ease, opacity 0.24s ease;
}

.update-notification-slide-enter-from,
.update-notification-slide-leave-to {
  transform: translateY(18px);
  opacity: 0;
}

.update-notification-slide-enter-to,
.update-notification-slide-leave-from {
  transform: translateY(0);
  opacity: 1;
}

:global(.dark) .update-notification-card {
  border-color: rgba(255, 255, 255, 0.08);
  background: rgba(28, 30, 36, 0.94);
  background: color-mix(in srgb, var(--mac-surface) 88%, rgba(0, 0, 0, 0.65));
  box-shadow: 0 22px 54px rgba(0, 0, 0, 0.5);
}

:global(.dark) .update-notification-icon {
  background: rgba(16, 185, 129, 0.24);
}

:global(.dark) .update-notification-title {
  color: #f8fafc;
}

:global(.dark) .update-notification-version {
  color: #b7bac7;
}

:global(.dark) .update-notification-btn-secondary {
  border-color: rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.1);
  color: #e5e7eb;
}

:global(.dark) .update-notification-btn-secondary:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.16);
}
</style>
