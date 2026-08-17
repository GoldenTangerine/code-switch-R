<!--
 * @name: OpenClaw 工具面板
 * @Descripttion: 编辑 OpenClaw settings 的工具配置（profile 档位 + allow/deny 字符串列表），保存时整体写回原生配置
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/components/OpenClaw/ToolsPanel.vue
-->
<template>
  <div class="claw-panel">
    <header class="claw-panel__header">
      <div>
        <h2 class="claw-panel__title">{{ t('components.openclawConfig.tools.title') }}</h2>
        <p class="claw-panel__hint">{{ t('components.openclawConfig.tools.hint') }}</p>
      </div>
      <BaseButton :disabled="loading || saving" type="button" variant="outline" @click="loadConfig">
        {{ t('components.openclawConfig.common.reload') }}
      </BaseButton>
    </header>

    <div v-if="errorMessage" class="alert-error">{{ errorMessage }}</div>
    <div v-if="loading" class="claw-empty">{{ t('components.openclawConfig.common.loading') }}</div>

    <template v-else>
      <label class="claw-field">
        <span class="claw-field__label">{{ t('components.openclawConfig.tools.profile') }}</span>
        <select v-model="profile" class="mac-select" :disabled="saving">
          <option v-for="option in profileOptions" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
        <span class="claw-field__hint">{{ t('components.openclawConfig.tools.profileHint') }}</span>
      </label>

      <label class="claw-field">
        <span class="claw-field__label">{{ t('components.openclawConfig.tools.allow') }}</span>
        <BaseTextarea
          v-model="allowText"
          rows="6"
          :placeholder="t('components.openclawConfig.tools.listPlaceholder')"
          :disabled="saving"
        />
        <span class="claw-field__hint">{{ t('components.openclawConfig.tools.allowHint') }}</span>
      </label>

      <label class="claw-field">
        <span class="claw-field__label">{{ t('components.openclawConfig.tools.deny') }}</span>
        <BaseTextarea
          v-model="denyText"
          rows="6"
          :placeholder="t('components.openclawConfig.tools.listPlaceholder')"
          :disabled="saving"
        />
        <span class="claw-field__hint">{{ t('components.openclawConfig.tools.denyHint') }}</span>
      </label>

      <footer class="claw-panel__actions">
        <BaseButton type="button" :disabled="saving" @click="saveConfig">
          {{ saving ? t('components.openclawConfig.common.saving') : t('components.openclawConfig.common.save') }}
        </BaseButton>
      </footer>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseButton from '../common/BaseButton.vue'
import BaseTextarea from '../common/BaseTextarea.vue'
import { getOpenClawToolsConfig, setOpenClawToolsConfig } from '../../services/openClaw'
import { openClawPanelDirty } from './panelDirty'
import { showToast } from '../../utils/toast'

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const profile = ref('full')
const allowText = ref('')
const denyText = ref('')

// 未保存跟踪：以加载完成时的内容快照为基线，任一字段变更即视为 dirty
const toolsSnapshot = (): string => JSON.stringify([profile.value, allowText.value, denyText.value])
const baselineSnapshot = ref<string | null>(null)
const isDirty = computed(() => (
  baselineSnapshot.value !== null && toolsSnapshot() !== baselineSnapshot.value
))

watch(isDirty, (dirty) => {
  openClawPanelDirty.tools = dirty
})

const profileOptions = computed(() => [
  { value: 'minimal', label: t('components.openclawConfig.tools.profiles.minimal') },
  { value: 'coding', label: t('components.openclawConfig.tools.profiles.coding') },
  { value: 'messaging', label: t('components.openclawConfig.tools.profiles.messaging') },
  { value: 'full', label: t('components.openclawConfig.tools.profiles.full') },
])

// 每行一项：去空白、忽略空行
const parseList = (value: string): string[] => (
  value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
)

const loadConfig = async () => {
  loading.value = true
  errorMessage.value = ''
  try {
    const config = await getOpenClawToolsConfig()
    profile.value = config.profile
    allowText.value = config.allow.join('\n')
    denyText.value = config.deny.join('\n')
  } catch (error) {
    console.error('failed to load openclaw tools config', error)
    errorMessage.value = t('components.openclawConfig.common.loadError')
  } finally {
    baselineSnapshot.value = toolsSnapshot()
    loading.value = false
  }
}

const saveConfig = async () => {
  if (saving.value) return
  errorMessage.value = ''

  saving.value = true
  try {
    await setOpenClawToolsConfig({
      profile: profile.value,
      allow: parseList(allowText.value),
      deny: parseList(denyText.value),
    })
    showToast(t('components.openclawConfig.common.saved'), 'success')
    await loadConfig()
  } catch (error) {
    console.error('failed to save openclaw tools config', error)
    showToast(t('components.openclawConfig.common.saveError'), 'error')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void loadConfig()
})
</script>

<style scoped>
.claw-panel {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 20px;
  border-radius: 22px;
  border: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface) 82%, transparent);
}

.claw-panel__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.claw-panel__title {
  margin: 0;
  font-size: 17px;
  font-weight: 700;
  color: var(--mac-text);
}

.claw-panel__hint {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--mac-text-secondary);
  line-height: 1.5;
}

.claw-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.claw-field__label {
  font-size: 14px;
  font-weight: 600;
  color: var(--mac-text);
}

.claw-field__hint {
  font-size: 12px;
  color: var(--mac-text-secondary);
  line-height: 1.5;
}

.claw-panel__actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 16px;
  border-top: 1px solid var(--mac-divider);
}

.claw-empty {
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
</style>
