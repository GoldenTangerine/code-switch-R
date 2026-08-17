<!--
 * @name: Hermes 记忆管理页
 * @Descripttion: 编辑 ~/.hermes/memories 的 MEMORY.md/USER.md 整文件 Markdown 内容，展示 § 条目数与字符预算，并管理开关/预算设置
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/components/Hermes/MemoryPanel.vue
-->
<template>
  <div class="main-shell">
    <div class="global-actions">
      <p class="global-eyebrow">{{ t('components.hermesMemory.hero.eyebrow') }}</p>
      <button class="ghost-icon" :aria-label="t('components.hermesMemory.controls.back')" @click="goHome">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M15 18l-6-6 6-6"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>
      <button class="ghost-icon" :aria-label="t('components.hermesMemory.controls.settings')" @click="goToSettings">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M12 15a3 3 0 100-6 3 3 0 000 6z"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
            fill="none"
          />
          <path
            d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 01-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09a1.65 1.65 0 00-1-1.51 1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09a1.65 1.65 0 001.51-1 1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
            fill="none"
          />
        </svg>
      </button>
      <UnsavedConfirmModal
        :open="unsavedConfirmOpen"
        :title="t('components.hermesMemory.common.unsavedTitle')"
        :message="t('components.hermesMemory.common.unsavedMessage')"
        :stay-label="t('components.hermesMemory.common.unsavedStay')"
        :leave-label="t('components.hermesMemory.common.unsavedLeave')"
        @close="cancelLeave"
        @confirm="confirmLeave"
      />
    </div>

    <div class="contrib-page">
      <section class="contrib-hero">
        <h1>{{ t('components.hermesMemory.hero.title') }}</h1>
        <p class="lead">{{ t('components.hermesMemory.hero.lead') }}</p>
      </section>

      <section class="automation-section">
        <div class="section-header">
          <div class="tab-group" role="tablist" :aria-label="t('components.hermesMemory.tabs.ariaLabel')">
            <button
              v-for="option in subTabs"
              :key="option.id"
              class="tab-pill"
              :class="{ active: activeKind === option.id }"
              role="tab"
              type="button"
              :aria-selected="activeKind === option.id"
              @click="switchKind(option.id)"
            >
              <span class="tab-pill__label">{{ option.label }}</span>
            </button>
          </div>
        </div>

        <div class="hermes-status-card">
          <span class="hermes-status-card__dot" :class="{ 'is-disabled': !activeSettings.enabled }" aria-hidden="true"></span>
          <span>
            {{ t('components.hermesMemory.status.entries', { count: entryCount }) }}
            · {{ t('components.hermesMemory.status.usage', { current: charCount, limit: activeSettings.charLimit }) }}
          </span>
          <span v-if="overLimit" class="hermes-status-card__warn">
            {{ t('components.hermesMemory.status.overLimit') }}
          </span>
        </div>

        <div class="hermes-panel">
          <header class="hermes-panel__header">
            <div>
              <h2 class="hermes-panel__title">{{ activeTitle }}</h2>
              <p class="hermes-panel__hint">{{ t('components.hermesMemory.editor.hint') }}</p>
            </div>
            <BaseButton :disabled="loading || saving" type="button" variant="outline" @click="loadAll">
              {{ t('components.hermesMemory.common.reload') }}
            </BaseButton>
          </header>

          <div v-if="errorMessage" class="alert-error">{{ errorMessage }}</div>
          <div v-if="loading" class="hermes-empty">{{ t('components.hermesMemory.common.loading') }}</div>

          <template v-else>
            <textarea
              v-model="memoryContents[activeKind]"
              class="hermes-memory-editor"
              :placeholder="t('components.hermesMemory.editor.placeholder')"
              :disabled="saving"
              spellcheck="false"
            ></textarea>

            <section class="hermes-panel__group">
              <h3 class="hermes-panel__group-title">{{ t('components.hermesMemory.settings.title') }}</h3>
              <p class="hermes-panel__group-hint">{{ t('components.hermesMemory.settings.hint') }}</p>

              <div class="hermes-settings-row">
                <div class="hermes-settings-row__main">
                  <span class="hermes-settings-row__label">{{ t('components.hermesMemory.settings.memoryEnabled') }}</span>
                  <label class="mac-switch">
                    <input
                      type="checkbox"
                      :checked="memoryEnabled"
                      :disabled="saving"
                      @change="memoryEnabled = ($event.target as HTMLInputElement).checked"
                    />
                    <span></span>
                  </label>
                </div>
                <label class="hermes-settings-row__budget">
                  <span>{{ t('components.hermesMemory.settings.memoryCharLimit') }}</span>
                  <BaseInput
                    :model-value="memoryCharLimit"
                    type="number"
                    :disabled="saving"
                    @update:model-value="memoryCharLimit = $event"
                  />
                </label>
              </div>

              <div class="hermes-settings-row">
                <div class="hermes-settings-row__main">
                  <span class="hermes-settings-row__label">{{ t('components.hermesMemory.settings.userEnabled') }}</span>
                  <label class="mac-switch">
                    <input
                      type="checkbox"
                      :checked="userEnabled"
                      :disabled="saving"
                      @change="userEnabled = ($event.target as HTMLInputElement).checked"
                    />
                    <span></span>
                  </label>
                </div>
                <label class="hermes-settings-row__budget">
                  <span>{{ t('components.hermesMemory.settings.userCharLimit') }}</span>
                  <BaseInput
                    :model-value="userCharLimit"
                    type="number"
                    :disabled="saving"
                    @update:model-value="userCharLimit = $event"
                  />
                </label>
              </div>
            </section>

            <footer class="hermes-panel__actions">
              <span class="hermes-panel__note">{{ t('components.hermesMemory.settings.runtimeNote') }}</span>
              <BaseButton type="button" :disabled="saving" @click="saveAll">
                {{ saving ? t('components.hermesMemory.common.saving') : t('components.hermesMemory.common.save') }}
              </BaseButton>
            </footer>
          </template>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import BaseButton from '../common/BaseButton.vue'
import BaseInput from '../common/BaseInput.vue'
import UnsavedConfirmModal from '../common/UnsavedConfirmModal.vue'
import {
  getHermesMemoryContent,
  getHermesMemoryEntries,
  getHermesMemorySettings,
  setHermesMemorySettings,
  writeHermesMemoryContent,
  type HermesMemoryKind,
  type HermesMemorySettings,
} from '../../services/hermes'
import { showToast } from '../../utils/toast'

type PendingLeave = { type: 'kind'; kind: HermesMemoryKind } | { type: 'route'; path: string }

const { t } = useI18n()
const router = useRouter()

const activeKind = ref<HermesMemoryKind>('memory')
const loading = ref(false)
const saving = ref(false)
const errorMessage = ref('')
// 两个文件的编辑内容分别暂存，切换子 tab 不丢稿
const memoryContents = reactive<Record<HermesMemoryKind, string>>({
  memory: '',
  user: '',
})
const entryCounts = reactive<Record<HermesMemoryKind, number>>({
  memory: 0,
  user: 0,
})

const memoryEnabled = ref(true)
const memoryCharLimit = ref('50000')
const userEnabled = ref(true)
const userCharLimit = ref('10000')

// 未保存跟踪：以加载完成时的内容与设置快照为基线，任一字段偏离即视为 dirty
const memorySnapshot = (): string => JSON.stringify({
  memory: memoryContents.memory,
  user: memoryContents.user,
  memoryEnabled: memoryEnabled.value,
  memoryCharLimit: memoryCharLimit.value,
  userEnabled: userEnabled.value,
  userCharLimit: userCharLimit.value,
})
const baselineSnapshot = ref<string | null>(null)
const isDirty = computed(() => (
  baselineSnapshot.value !== null && memorySnapshot() !== baselineSnapshot.value
))

const unsavedConfirmOpen = ref(false)
const pendingLeave = ref<PendingLeave | null>(null)

// 字符预算转数字（非法输入回退 0，交由后端按缺省处理）
const normalizeCharLimit = (value: string): number => {
  const numeric = Number(value)
  return Number.isFinite(numeric) && numeric > 0 ? Math.floor(numeric) : 0
}

const subTabs = computed(() => [
  { id: 'memory' as HermesMemoryKind, label: t('components.hermesMemory.tabs.memory') },
  { id: 'user' as HermesMemoryKind, label: t('components.hermesMemory.tabs.user') },
])

const activeTitle = computed(() => (
  activeKind.value === 'memory'
    ? t('components.hermesMemory.tabs.memoryTitle')
    : t('components.hermesMemory.tabs.userTitle')
))

const entryCount = computed(() => entryCounts[activeKind.value] ?? 0)

const activeSettings = computed(() => ({
  enabled: activeKind.value === 'memory' ? memoryEnabled.value : userEnabled.value,
  charLimit: normalizeCharLimit(activeKind.value === 'memory' ? memoryCharLimit.value : userCharLimit.value),
}))

const charCount = computed(() => Array.from(memoryContents[activeKind.value] ?? '').length)
const overLimit = computed(() => charCount.value > activeSettings.value.charLimit)

const applySettings = (settings: HermesMemorySettings) => {
  memoryEnabled.value = settings.memoryEnabled
  memoryCharLimit.value = `${settings.memoryCharLimit}`
  userEnabled.value = settings.userProfileEnabled
  userCharLimit.value = `${settings.userCharLimit}`
}

const loadKind = async (kind: HermesMemoryKind) => {
  const [content, entries] = await Promise.all([
    getHermesMemoryContent(kind),
    getHermesMemoryEntries(kind),
  ])
  memoryContents[kind] = content
  entryCounts[kind] = entries.length
}

const loadAll = async () => {
  loading.value = true
  errorMessage.value = ''
  try {
    const [settings] = await Promise.all([
      getHermesMemorySettings(),
      loadKind('memory'),
      loadKind('user'),
    ])
    applySettings(settings)
  } catch (error) {
    console.error('failed to load hermes memory', error)
    errorMessage.value = t('components.hermesMemory.common.loadError')
  } finally {
    baselineSnapshot.value = memorySnapshot()
    loading.value = false
  }
}

const saveAll = async () => {
  if (saving.value) return
  errorMessage.value = ''
  saving.value = true
  try {
    // 两个 kind 的编辑稿分别暂存于本地，保存时一并写盘，避免只落当前子 tab 的半套修改
    await writeHermesMemoryContent('memory', memoryContents.memory ?? '')
    await writeHermesMemoryContent('user', memoryContents.user ?? '')
    await setHermesMemorySettings({
      memoryEnabled: memoryEnabled.value === true,
      memoryCharLimit: normalizeCharLimit(memoryCharLimit.value),
      userProfileEnabled: userEnabled.value === true,
      userCharLimit: normalizeCharLimit(userCharLimit.value),
    })
    showToast(t('components.hermesMemory.common.saved'), 'success')
    await loadKind('memory')
    await loadKind('user')
    await applySettings(await getHermesMemorySettings())
    baselineSnapshot.value = memorySnapshot()
  } catch (error) {
    console.error('failed to save hermes memory', error)
    showToast(t('components.hermesMemory.common.saveError'), 'error')
  } finally {
    saving.value = false
  }
}

// 有未保存修改时先弹确认，避免切换/离开导致静默丢稿
const applyLeave = (leave: PendingLeave) => {
  if (leave.type === 'kind') {
    activeKind.value = leave.kind
  } else {
    router.push(leave.path)
  }
}

const requestLeave = (leave: PendingLeave) => {
  if (isDirty.value) {
    pendingLeave.value = leave
    unsavedConfirmOpen.value = true
    return
  }
  applyLeave(leave)
}

const confirmLeave = () => {
  const leave = pendingLeave.value
  unsavedConfirmOpen.value = false
  pendingLeave.value = null
  if (leave) applyLeave(leave)
}

const cancelLeave = () => {
  unsavedConfirmOpen.value = false
  pendingLeave.value = null
}

const switchKind = (kind: HermesMemoryKind) => {
  if (kind === activeKind.value) return
  requestLeave({ type: 'kind', kind })
}

const goHome = () => {
  requestLeave({ type: 'route', path: '/' })
}

const goToSettings = () => {
  requestLeave({ type: 'route', path: '/settings' })
}

onMounted(() => {
  void loadAll()
})
</script>

<style scoped>
.hermes-status-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border-radius: 16px;
  border: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface) 72%, transparent);
  color: var(--mac-text-secondary);
  font-size: 13px;
  line-height: 1.5;
  flex-wrap: wrap;
}

.hermes-status-card__dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #10b981;
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.45);
  flex-shrink: 0;
}

.hermes-status-card__dot.is-disabled {
  background: #9ca3af;
  box-shadow: none;
}

.hermes-status-card__warn {
  color: #f59e0b;
  font-weight: 600;
}

.hermes-panel {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 20px;
  border-radius: 22px;
  border: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface) 82%, transparent);
}

.hermes-panel__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.hermes-panel__title {
  margin: 0;
  font-size: 17px;
  font-weight: 700;
  color: var(--mac-text);
}

.hermes-panel__hint {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--mac-text-secondary);
  line-height: 1.5;
}

.hermes-memory-editor {
  width: 100%;
  min-height: 320px;
  padding: 14px 16px;
  border-radius: 16px;
  border: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface) 92%, transparent);
  color: var(--mac-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
  line-height: 1.7;
  resize: vertical;
  outline: none;
}

.hermes-memory-editor:focus {
  border-color: #0a84ff;
}

.hermes-memory-editor:disabled {
  opacity: 0.6;
}

.hermes-panel__group {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px solid var(--mac-divider);
}

.hermes-panel__group-title {
  margin: 0;
  font-size: 14px;
  font-weight: 700;
  color: var(--mac-text);
}

.hermes-panel__group-hint {
  margin: 0;
  font-size: 12px;
  color: var(--mac-text-secondary);
  line-height: 1.5;
}

.hermes-settings-row {
  display: grid;
  grid-template-columns: 1fr 220px;
  gap: 12px;
  align-items: center;
  padding: 10px 14px;
  border-radius: 14px;
  border: 1px solid var(--mac-border);
}

.hermes-settings-row__main {
  display: flex;
  align-items: center;
  gap: 12px;
}

.hermes-settings-row__label {
  font-size: 13px;
  color: var(--mac-text);
}

.hermes-settings-row__budget {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: var(--mac-text-secondary);
}

.hermes-panel__actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px solid var(--mac-divider);
  flex-wrap: wrap;
}

.hermes-panel__note {
  font-size: 12px;
  color: var(--mac-text-secondary);
}

.hermes-empty {
  text-align: center;
  padding: 24px;
  border: 1px dashed var(--mac-border);
  border-radius: 16px;
  color: var(--mac-text-secondary);
}

.alert-error {
  margin: 0;
  padding: 10px 14px;
  border-radius: 12px;
  border: 1px solid rgba(244, 67, 54, 0.35);
  background: rgba(244, 67, 54, 0.12);
  color: #ef4444;
  font-size: 13px;
}

@media (max-width: 720px) {
  .hermes-settings-row {
    grid-template-columns: 1fr;
  }
}
</style>
