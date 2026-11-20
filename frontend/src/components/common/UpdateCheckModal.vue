<!--
 * @name: 应用更新弹窗
 * @Descripttion: 打开即检查应用更新，展示版本与更新说明，并负责下载、安装重启的完整流程
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 18:10:00
 * @LastEditTime: 2026-11-20 18:10:00
 * @FilePath: frontend/src/components/common/UpdateCheckModal.vue
-->
<template>
  <InlineModal
    :open="open"
    :title="t('components.general.update.modalTitle')"
    @close="emit('close')"
  >
    <div class="update-modal">
      <div class="update-modal-row">
        <span class="update-modal-label">{{ t('components.general.label.currentVersion') }}</span>
        <span class="version-text">{{ appVersion || '—' }}</span>
      </div>
      <div class="update-modal-row">
        <span class="update-modal-label">{{ t('components.general.label.latestVersion') }}</span>
        <span class="version-text highlight">{{ latestVersion }}</span>
      </div>

      <div v-if="message" class="info-text update-modal-message">
        {{ message }}
      </div>
      <div v-if="errorMessage" class="update-modal-error">
        {{ errorMessage }}
      </div>

      <div class="update-modal-block">
        <div class="update-modal-block-title">{{ t('components.general.update.releaseNotes') }}</div>
        <pre class="update-modal-release-notes">{{ releaseNotes }}</pre>
      </div>

      <footer class="update-modal-actions">
        <button class="action-btn" type="button" :disabled="busy" @click="checkForUpdate">
          {{ t('components.general.update.recheck') }}
        </button>
        <button class="action-btn" type="button" :disabled="busy" @click="emit('close')">
          {{ t('common.cancel') }}
        </button>
        <button
          class="primary-btn"
          type="button"
          :disabled="busy || !canTriggerUpdate"
          @click="downloadAndInstall"
        >
          {{ actionText }}
        </button>
      </footer>
    </div>
  </InlineModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import InlineModal from './InlineModal.vue'
import { checkUpdate, downloadUpdate, getUpdateState, restartApp, type UpdateInfo, type UpdateState } from '../../services/update'
import { fetchCurrentVersion } from '../../services/version'
import { extractErrorMessage } from '../../utils/error'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
  // 检查或下载后更新状态已刷新，父组件可据此同步「上次检查」等展示
  updated: [state: UpdateState]
}>()

const { t } = useI18n()

const UPDATE_ERROR_LOG_PATH = '.code-switch/updates/update-errors.log'

const appVersion = ref('')
const updateState = ref<UpdateState | null>(null)
const updateInfo = ref<UpdateInfo | null>(null)
const checking = ref(false)
const downloading = ref(false)
const installing = ref(false)
const downloadProgress = ref<number | null>(null)
const message = ref('')
const errorMessage = ref('')

const busy = computed(() => checking.value || downloading.value || installing.value)

const latestVersion = computed(() => (
  updateInfo.value?.version || updateState.value?.latest_known_version || appVersion.value || '—'
))

const releaseNotes = computed(() => (
  updateInfo.value?.release_notes?.trim()
  || updateState.value?.latest_release_notes?.trim()
  || t('components.general.update.releaseNotesEmpty')
))

const canTriggerUpdate = computed(() => Boolean(updateState.value?.update_ready || updateInfo.value?.available))

const actionText = computed(() => {
  if (installing.value) return t('components.general.update.installing')
  if (updateState.value?.update_ready) return t('components.general.update.installAndRestart')
  if (downloading.value) {
    const progress = Math.round(downloadProgress.value ?? updateState.value?.download_progress ?? 0)
    return t('components.general.update.downloading', { progress })
  }
  return t('components.general.update.updateNow')
})

const loadUpdateState = async () => {
  try {
    updateState.value = await getUpdateState()
    emit('updated', updateState.value)
  } catch (error) {
    console.error('failed to load update state', error)
  }
}

const loadAppVersion = async () => {
  if (appVersion.value) return
  try {
    appVersion.value = await fetchCurrentVersion()
  } catch (error) {
    console.error('failed to load app version', error)
  }
}

const setDownloadProgress = (progress: number) => {
  if (!Number.isFinite(progress)) return
  const normalized = Math.max(0, Math.min(100, progress))
  downloadProgress.value = normalized
  if (updateState.value) {
    updateState.value = { ...updateState.value, download_progress: normalized }
  }
}

const isUacCancelledError = (text: string) => {
  const normalized = text.toLowerCase()
  return normalized.includes('err_uac_denied') ||
    normalized.includes('uac') && (normalized.includes('cancel') || normalized.includes('deny')) ||
    text.includes('用户取消') ||
    text.includes('取消 UAC') ||
    text.includes('拒绝 UAC')
}

const buildRestartInstallErrorMessage = (error: unknown) => {
  const text = extractErrorMessage(error)
  if (isUacCancelledError(text)) {
    return `你刚才把管理员权限确认（UAC）取消了，本次更新没安装。\n想更新的话，再点一次“安装并重启”就行。`
  }
  if (text.includes('应用更新失败') || text.includes('更新') || text.includes('安装')) {
    return `安装更新失败：${text}\n详细日志在 ${UPDATE_ERROR_LOG_PATH}`
  }
  return `重启失败：${text}\n详细日志在 ${UPDATE_ERROR_LOG_PATH}`
}

const checkForUpdate = async () => {
  if (busy.value) return

  errorMessage.value = ''
  message.value = t('components.general.update.checking')
  updateInfo.value = null
  checking.value = true
  try {
    const info = await checkUpdate()
    await loadUpdateState()

    if (!info) {
      message.value = ''
      errorMessage.value = t('components.general.update.checkEmptyResponse')
      return
    }

    updateInfo.value = info
    message.value = info.available
      ? t('components.general.update.updateFound', { version: info.version })
      : t('components.general.update.alreadyLatest', { version: info.version || appVersion.value || '—' })
  } catch (error) {
    console.error('check update failed', error)
    message.value = ''
    errorMessage.value = t('components.general.update.checkFailedWithError', {
      error: extractErrorMessage(error),
    })
    await loadUpdateState()
  } finally {
    checking.value = false
  }
}

const downloadAndInstall = async () => {
  if (busy.value) return

  errorMessage.value = ''
  message.value = ''

  // 更新包已就绪：直接重启安装
  if (updateState.value?.update_ready) {
    installing.value = true
    try {
      await restartApp()
    } catch (error) {
      console.error('restart/install update failed', error)
      errorMessage.value = buildRestartInstallErrorMessage(error)
    } finally {
      installing.value = false
    }
    return
  }

  if (!updateInfo.value?.available) {
    message.value = t('components.general.update.noUpdateToInstall')
    return
  }

  downloading.value = true
  setDownloadProgress(0)
  try {
    await downloadUpdate(setDownloadProgress)
    await loadUpdateState()
    setDownloadProgress(100)
    message.value = t('components.general.update.downloadReady')
  } catch (error) {
    console.error('download failed', error)
    errorMessage.value = t('components.general.update.downloadFailedWithError', {
      error: extractErrorMessage(error),
    })
  } finally {
    downloading.value = false
    downloadProgress.value = null
  }
}

// 每次打开即发起检查；下载/安装进行中重新打开时保留现有进度，不重复检查
watch(
  () => props.open,
  (isOpen) => {
    if (!isOpen || busy.value) return
    void loadAppVersion()
    void checkForUpdate()
  },
  { immediate: true },
)
</script>

<style scoped>
.update-modal {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.update-modal-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.update-modal-label {
  font-size: 12px;
  color: var(--mac-text-secondary);
}

.update-modal-message {
  margin: 2px 0 0;
}

.update-modal-error {
  padding: 0.65rem 0.85rem;
  border-radius: 12px;
  background: rgba(244, 67, 54, 0.12);
  color: #d93025;
  border: 1px solid rgba(244, 67, 54, 0.2);
  font-size: 12px;
  line-height: 1.45;
  white-space: pre-line;
}

html.dark .update-modal-error {
  color: #f87171;
}

.update-modal-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.update-modal-block-title {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--mac-text);
}

.update-modal-release-notes {
  margin: 0;
  max-height: 220px;
  overflow: auto;
  border-radius: 12px;
  border: 1px solid var(--mac-border);
  background: var(--mac-surface-strong);
  color: var(--mac-text-secondary);
  padding: 10px 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.update-modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  flex-wrap: wrap;
  padding-top: 12px;
  border-top: 1px solid var(--mac-divider);
}
</style>
