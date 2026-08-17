<!--
 * @name: OpenClaw Agents 面板
 * @Descripttion: 以 JSON 编辑器形式编辑 OpenClaw settings 的 agents defaults 对象，保存前做 JSON 校验并提供常用字段提示
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/components/OpenClaw/AgentsPanel.vue
-->
<template>
  <div class="claw-panel">
    <header class="claw-panel__header">
      <div>
        <h2 class="claw-panel__title">{{ t('components.openclawConfig.agents.title') }}</h2>
        <p class="claw-panel__hint">{{ t('components.openclawConfig.agents.hint') }}</p>
      </div>
      <BaseButton :disabled="loading || saving" type="button" variant="outline" @click="loadConfig">
        {{ t('components.openclawConfig.common.reload') }}
      </BaseButton>
    </header>

    <div v-if="errorMessage" class="alert-error">{{ errorMessage }}</div>
    <div v-if="loading" class="claw-empty">{{ t('components.openclawConfig.common.loading') }}</div>

    <template v-else>
      <div class="claw-field">
        <span class="claw-field__label">{{ t('components.openclawConfig.agents.editor') }}</span>
        <BaseTextarea
          v-model="agentsText"
          rows="14"
          class="claw-json-textarea"
          :placeholder="defaultJsonPlaceholder"
          :disabled="saving"
        />
        <p v-if="agentsError" class="alert-error">{{ agentsError }}</p>
        <p class="claw-field__hint">{{ t('components.openclawConfig.agents.fieldHints') }}</p>
      </div>

      <footer class="claw-panel__actions">
        <BaseButton variant="outline" type="button" :disabled="saving" @click="formatJson">
          {{ t('components.openclawConfig.agents.format') }}
        </BaseButton>
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
import { getOpenClawAgentsConfig, setOpenClawAgentsConfig, type OpenClawAgentsConfig } from '../../services/openClaw'
import { openClawPanelDirty } from './panelDirty'
import { showToast } from '../../utils/toast'

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const agentsError = ref('')
const agentsText = ref('{}')

// 未保存跟踪：以加载完成时的文本为基线，编辑（含格式化）后偏离即视为 dirty
const baselineText = ref<string | null>(null)
const isDirty = computed(() => baselineText.value !== null && agentsText.value !== baselineText.value)

watch(isDirty, (dirty) => {
  openClawPanelDirty.agents = dirty
})

const defaultJsonPlaceholder = JSON.stringify({
  model: {
    primary: 'claude-sonnet-4-5',
    fallbacks: ['claude-haiku-4-5'],
  },
  timeoutSeconds: 300,
}, null, 2)

// 解析并校验：必须是合法 JSON 且为对象（非数组）
const parseAgentsConfig = (): OpenClawAgentsConfig | null => {
  const trimmed = agentsText.value.trim()
  if (!trimmed) return {}
  try {
    const parsed = JSON.parse(trimmed) as unknown
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      agentsError.value = t('components.openclawConfig.agents.mustBeObject')
      return null
    }
    return parsed as OpenClawAgentsConfig
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    agentsError.value = t('components.openclawConfig.agents.invalidJson', { message })
    return null
  }
}

const formatJson = () => {
  agentsError.value = ''
  const parsed = parseAgentsConfig()
  if (parsed === null) return
  agentsText.value = JSON.stringify(parsed, null, 2)
}

const loadConfig = async () => {
  loading.value = true
  errorMessage.value = ''
  agentsError.value = ''
  try {
    const config = await getOpenClawAgentsConfig()
    agentsText.value = JSON.stringify(config ?? {}, null, 2)
  } catch (error) {
    console.error('failed to load openclaw agents config', error)
    errorMessage.value = t('components.openclawConfig.common.loadError')
  } finally {
    baselineText.value = agentsText.value
    loading.value = false
  }
}

const saveConfig = async () => {
  if (saving.value) return
  errorMessage.value = ''
  agentsError.value = ''

  const parsed = parseAgentsConfig()
  if (parsed === null) return

  saving.value = true
  try {
    await setOpenClawAgentsConfig(parsed)
    agentsText.value = JSON.stringify(parsed, null, 2)
    baselineText.value = agentsText.value
    showToast(t('components.openclawConfig.common.saved'), 'success')
  } catch (error) {
    console.error('failed to save openclaw agents config', error)
    showToast(t('components.openclawConfig.common.saveError'), 'error')
  } finally {
    saving.value = false
  }
}

watch(agentsText, () => {
  if (agentsError.value) agentsError.value = ''
})

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
  margin: 0;
  font-size: 12px;
  color: var(--mac-text-secondary);
  line-height: 1.5;
}

.claw-json-textarea {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 12px;
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
