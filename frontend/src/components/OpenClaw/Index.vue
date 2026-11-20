<!--
 * @name: OpenClaw 配置页
 * @Descripttion: OpenClaw 专属配置入口，提供环境变量 / 工具 / Agents 三个子配置面板的切换与原生 settings 状态展示
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/components/OpenClaw/Index.vue
-->
<template>
  <div class="main-shell">
    <div class="global-actions">
      <p class="global-eyebrow">{{ t('components.openclawConfig.hero.eyebrow') }}</p>
      <button class="ghost-icon" :aria-label="t('components.openclawConfig.controls.back')" @click="goHome">
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
      <button class="ghost-icon" :aria-label="t('components.openclawConfig.controls.settings')" @click="goToSettings">
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
        :title="t('components.openclawConfig.common.unsavedTitle')"
        :message="t('components.openclawConfig.common.unsavedMessage')"
        :stay-label="t('components.openclawConfig.common.unsavedStay')"
        :leave-label="t('components.openclawConfig.common.unsavedLeave')"
        @close="cancelLeave"
        @confirm="confirmLeave"
      />
    </div>

    <div class="contrib-page claw-page">
      <PageHero
        :title="t('components.openclawConfig.hero.title')"
        :lead="t('components.openclawConfig.hero.lead')"
      />

      <div class="claw-toolbar">
        <SegmentedTabs
          :options="subTabs"
          :active="activeSubTab"
          :aria-label="t('components.openclawConfig.tabs.ariaLabel')"
          @select="switchSubTab($event as OpenClawSubTab)"
        />
        <div v-if="status" class="claw-meta">
          <span class="claw-meta__item">
            <span class="claw-meta__dot" :class="{ 'is-idle': !status.currentProviderName }" aria-hidden="true"></span>
            {{ currentProviderText }}
          </span>
          <span class="claw-meta__item">{{ t('components.openclawConfig.status.managedProviders', { count: status.managedProviderCount }) }}</span>
        </div>
      </div>

      <p v-if="status?.settingsPath" class="claw-path" :title="status.settingsPath">
        {{ t('components.openclawConfig.status.settingsPath', { path: status.settingsPath }) }}
      </p>

      <EnvPanel v-if="activeSubTab === 'env'" />
      <ToolsPanel v-else-if="activeSubTab === 'tools'" />
      <AgentsPanel v-else />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import EnvPanel from './EnvPanel.vue'
import ToolsPanel from './ToolsPanel.vue'
import AgentsPanel from './AgentsPanel.vue'
import UnsavedConfirmModal from '../common/UnsavedConfirmModal.vue'
import PageHero from '../common/PageHero.vue'
import SegmentedTabs from '../common/SegmentedTabs.vue'
import { hasOpenClawDirtyPanel, resetOpenClawPanelDirty } from './panelDirty'
import { getOpenClawStatus, type OpenClawStatus } from '../../services/openClaw'

type OpenClawSubTab = 'env' | 'tools' | 'agents'

// 待执行的离开动作：切换目标子 tab 或路由跳转，dirty 确认通过后统一放行
type PendingLeave = { type: 'subtab'; tab: OpenClawSubTab } | { type: 'route'; path: string }

const { t } = useI18n()
const router = useRouter()

const activeSubTab = ref<OpenClawSubTab>('env')
const status = ref<OpenClawStatus | null>(null)
const unsavedConfirmOpen = ref(false)
const pendingLeave = ref<PendingLeave | null>(null)

const subTabs = computed(() => [
  { id: 'env' as OpenClawSubTab, label: t('components.openclawConfig.tabs.env') },
  { id: 'tools' as OpenClawSubTab, label: t('components.openclawConfig.tabs.tools') },
  { id: 'agents' as OpenClawSubTab, label: t('components.openclawConfig.tabs.agents') },
])

// 原生 settings 落盘状态：当前生效供应商单独成项，路径另起一行展示，避免长路径挤压
const currentProviderText = computed(() => {
  const name = status.value?.currentProviderName
  return name
    ? t('components.openclawConfig.status.currentProvider', { name })
    : t('components.openclawConfig.status.noCurrentProvider')
})

const loadStatus = async () => {
  try {
    status.value = await getOpenClawStatus()
  } catch (error) {
    console.error('failed to load openclaw status', error)
    status.value = null
  }
}

// 任意子面板有未保存修改时先弹确认，避免切换/离开导致静默丢稿
const applyLeave = (leave: PendingLeave) => {
  resetOpenClawPanelDirty()
  if (leave.type === 'subtab') {
    activeSubTab.value = leave.tab
  } else {
    router.push(leave.path)
  }
}

const requestLeave = (leave: PendingLeave) => {
  if (hasOpenClawDirtyPanel()) {
    pendingLeave.value = leave
    unsavedConfirmOpen.value = true
    return
  }
  applyLeave(leave)
}

const switchSubTab = (tab: OpenClawSubTab) => {
  if (tab === activeSubTab.value) return
  requestLeave({ type: 'subtab', tab })
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

const goHome = () => {
  requestLeave({ type: 'route', path: '/' })
}

const goToSettings = () => {
  requestLeave({ type: 'route', path: '/settings' })
}

onMounted(() => {
  void loadStatus()
})
</script>

<style scoped>
.claw-page {
  gap: 16px;
}

.claw-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px 16px;
  flex-wrap: wrap;
  margin-top: 8px;
}

.claw-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.claw-meta__item {
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
  white-space: nowrap;
}

.claw-meta__dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: #10b981;
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.45);
  flex-shrink: 0;
}

.claw-meta__dot.is-idle {
  background: #9ca3af;
  box-shadow: none;
}

.claw-path {
  margin: -4px 0 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--mac-text-secondary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}
</style>
