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

    <div class="contrib-page">
      <section class="contrib-hero">
        <h1>{{ t('components.openclawConfig.hero.title') }}</h1>
        <p class="lead">{{ t('components.openclawConfig.hero.lead') }}</p>
      </section>

      <section class="automation-section">
        <div class="section-header">
          <div class="tab-group" role="tablist" :aria-label="t('components.openclawConfig.tabs.ariaLabel')">
            <button
              v-for="option in subTabs"
              :key="option.id"
              class="tab-pill"
              :class="{ active: activeSubTab === option.id }"
              role="tab"
              type="button"
              :aria-selected="activeSubTab === option.id"
              @click="switchSubTab(option.id)"
            >
              <span class="tab-pill__label">{{ option.label }}</span>
            </button>
          </div>
        </div>

        <div v-if="statusText" class="claw-status-card">
          <span class="claw-status-card__dot" aria-hidden="true"></span>
          <span>{{ statusText }}</span>
        </div>

        <EnvPanel v-if="activeSubTab === 'env'" />
        <ToolsPanel v-else-if="activeSubTab === 'tools'" />
        <AgentsPanel v-else />
      </section>
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

// 原生 settings 落盘状态：展示路径、当前生效供应商与受管条目数
const statusText = computed(() => {
  const current = status.value
  if (!current) return ''
  const parts: string[] = []
  if (current.currentProviderName) {
    parts.push(t('components.openclawConfig.status.currentProvider', { name: current.currentProviderName }))
  } else {
    parts.push(t('components.openclawConfig.status.noCurrentProvider'))
  }
  parts.push(t('components.openclawConfig.status.managedProviders', { count: current.managedProviderCount }))
  if (current.settingsPath) {
    parts.push(t('components.openclawConfig.status.settingsPath', { path: current.settingsPath }))
  }
  return parts.join(' · ')
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
.claw-status-card {
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

.claw-status-card__dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #10b981;
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.45);
  flex-shrink: 0;
}
</style>
