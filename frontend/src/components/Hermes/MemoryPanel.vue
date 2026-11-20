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

    <div class="contrib-page hermes-page">
      <PageHero
        :title="t('components.hermesMemory.hero.title')"
        :lead="t('components.hermesMemory.hero.lead')"
      />

      <div class="hermes-toolbar">
        <SegmentedTabs
          :options="subTabs"
          :active="activeKind"
          :aria-label="t('components.hermesMemory.tabs.ariaLabel')"
          @select="switchKind($event as HermesMemoryKind)"
        />
        <div class="hermes-meta">
          <span class="hermes-meta__item">
            <span class="hermes-meta__dot" :class="{ 'is-disabled': !activeSettings.enabled }" aria-hidden="true"></span>
            {{ t('components.hermesMemory.status.entries', { count: entryCount }) }}
          </span>
          <span class="hermes-meta__item" :class="{ 'is-warn': overLimit }">
            {{ t('components.hermesMemory.status.usage', { current: charCount, limit: activeSettings.charLimit }) }}
            <template v-if="overLimit"> · {{ t('components.hermesMemory.status.overLimit') }}</template>
          </span>
        </div>
      </div>

      <div
        class="hermes-usage"
        role="progressbar"
        :aria-valuenow="charCount"
        aria-valuemin="0"
        :aria-valuemax="activeSettings.charLimit"
      >
        <span class="hermes-usage__bar" :class="{ 'is-warn': overLimit }" :style="{ width: `${usagePercent}%` }"></span>
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
            <div>
              <h3 class="hermes-panel__group-title">{{ t('components.hermesMemory.settings.title') }}</h3>
              <p class="hermes-panel__group-hint">{{ t('components.hermesMemory.settings.hint') }}</p>
            </div>

            <div class="hermes-settings-grid">
              <div v-for="item in settingItems" :key="item.kind" class="hermes-setting-card" :class="{ 'is-current': item.kind === activeKind }">
                <div class="hermes-setting-card__head">
                  <span class="hermes-setting-card__label">{{ item.enabledLabel }}</span>
                  <label class="mac-switch">
                    <input
                      type="checkbox"
                      :checked="item.enabled.value"
                      :disabled="saving"
                      :aria-label="item.enabledLabel"
                      @change="item.enabled.value = ($event.target as HTMLInputElement).checked"
                    />
                    <span></span>
                  </label>
                </div>
                <label class="hermes-setting-card__budget">
                  <span>{{ item.limitLabel }}</span>
                  <BaseInput
                    :model-value="item.limit.value"
                    type="number"
                    :disabled="saving"
                    @update:model-value="item.limit.value = $event"
                  />
                </label>
              </div>
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
import PageHero from '../common/PageHero.vue'
import SegmentedTabs from '../common/SegmentedTabs.vue'
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

// 两组记忆设置共用同一卡片结构，ref 直接下发以便模板内双向写回
const settingItems = computed(() => [
  {
    kind: 'memory' as HermesMemoryKind,
    enabled: memoryEnabled,
    limit: memoryCharLimit,
    enabledLabel: t('components.hermesMemory.settings.memoryEnabled'),
    limitLabel: t('components.hermesMemory.settings.memoryCharLimit'),
  },
  {
    kind: 'user' as HermesMemoryKind,
    enabled: userEnabled,
    limit: userCharLimit,
    enabledLabel: t('components.hermesMemory.settings.userEnabled'),
    limitLabel: t('components.hermesMemory.settings.userCharLimit'),
  },
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
// 字符预算占用比例，用于状态条下方的进度细线（预算为 0 时视为未设置，不显示进度）
const usagePercent = computed(() => {
  const limit = activeSettings.value.charLimit
  if (!limit) return 0
  return Math.min(100, Math.round((charCount.value / limit) * 1000) / 10)
})

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
.hermes-page {
  gap: 16px;
}

.hermes-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px 16px;
  flex-wrap: wrap;
  margin-top: 8px;
}

.hermes-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.hermes-meta__item {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 5px 12px;
  border-radius: 999px;
  border: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface) 60%, transparent);
  color: var(--mac-text-secondary);
  font-size: 12px;
  line-height: 1.4;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.hermes-meta__item.is-warn {
  border-color: rgba(245, 158, 11, 0.4);
  background: rgba(245, 158, 11, 0.1);
  color: #d97706;
  font-weight: 600;
}

html.dark .hermes-meta__item.is-warn {
  color: #fbbf24;
}

.hermes-meta__dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: #10b981;
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.45);
  flex-shrink: 0;
}

.hermes-meta__dot.is-disabled {
  background: #9ca3af;
  box-shadow: none;
}

/* 字符预算细进度条：紧贴工具栏下方，直观提示占用情况 */
.hermes-usage {
  height: 3px;
  margin-top: -6px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--mac-text) 8%, transparent);
  overflow: hidden;
}

.hermes-usage__bar {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--mac-accent);
  transition: width 0.25s ease, background 0.2s ease;
}

.hermes-usage__bar.is-warn {
  background: #f59e0b;
}

.hermes-panel {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 20px 22px 22px;
  border-radius: 18px;
  border: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface) 82%, transparent);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.04) inset;
}

.hermes-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.hermes-panel__header :deep(.btn) {
  min-width: 0;
  height: 30px;
  padding: 0 14px;
  line-height: 1;
  font-size: 13px;
}

.hermes-panel__title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--mac-text);
}

.hermes-panel__hint {
  margin: 4px 0 0;
  font-size: 12.5px;
  color: var(--mac-text-secondary);
  line-height: 1.5;
}

.hermes-memory-editor {
  display: block;
  width: 100%;
  min-height: 320px;
  padding: 14px 16px;
  border-radius: 12px;
  border: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface-strong) 70%, transparent);
  color: var(--mac-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
  line-height: 1.7;
  resize: vertical;
  outline: none;
  transition: border-color 0.18s ease, box-shadow 0.18s ease;
}

.hermes-memory-editor::placeholder {
  color: color-mix(in srgb, var(--mac-text-secondary) 70%, transparent);
}

.hermes-memory-editor:focus {
  border-color: var(--mac-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--mac-accent) 22%, transparent);
}

.hermes-memory-editor:disabled {
  opacity: 0.6;
}

.hermes-panel__group {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 18px;
  border-top: 1px solid var(--mac-divider);
}

.hermes-panel__group-title {
  margin: 0;
  font-size: 14px;
  font-weight: 700;
  color: var(--mac-text);
}

.hermes-panel__group-hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--mac-text-secondary);
  line-height: 1.5;
}

.hermes-settings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 300px), 1fr));
  gap: 12px;
}

.hermes-setting-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px 16px;
  border-radius: 14px;
  border: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface-strong) 45%, transparent);
  transition: border-color 0.18s ease;
}

/* 当前所编辑文件对应的设置卡片高亮，帮助对应上方 Tab */
.hermes-setting-card.is-current {
  border-color: color-mix(in srgb, var(--mac-accent) 45%, var(--mac-border));
}

.hermes-setting-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.hermes-setting-card__label {
  font-size: 13px;
  font-weight: 600;
  color: var(--mac-text);
  line-height: 1.4;
}

.hermes-setting-card__head .mac-switch {
  flex-shrink: 0;
}

.hermes-setting-card__budget {
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
  border-radius: 14px;
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
</style>
