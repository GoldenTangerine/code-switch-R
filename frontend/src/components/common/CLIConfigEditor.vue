<template>
  <div class="cli-config-editor">
    <div class="cli-header" @click="toggleExpanded">
      <div class="cli-header-left">
        <svg
          class="expand-icon"
          :class="{ expanded }"
          viewBox="0 0 20 20"
          aria-hidden="true"
        >
          <path
            d="M6 8l4 4 4-4"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
            fill="none"
          />
        </svg>
        <span class="cli-title">{{ t('components.cliConfig.title') }}</span>
        <span class="cli-platform-badge">{{ platformLabel }}</span>
      </div>
      <div class="cli-header-right" @click.stop>
        <button
          v-if="expanded"
          class="cli-action-btn"
          type="button"
          :title="t('components.cliConfig.restoreDefault')"
          @click="handleRestoreDefault"
        >
          <svg viewBox="0 0 20 20" aria-hidden="true">
            <path
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
              stroke="currentColor"
              stroke-width="1.5"
              stroke-linecap="round"
              stroke-linejoin="round"
              fill="none"
            />
          </svg>
        </button>
      </div>
    </div>

    <div v-if="expanded" class="cli-content">
      <div v-if="loading" class="cli-loading">
        {{ t('components.cliConfig.loading') }}
      </div>

      <template v-else-if="config">
        <!-- 锁定字段 -->
        <div class="cli-section">
          <div class="cli-section-header">
            <span class="lock-icon">🔒</span>
            <span>{{ t('components.cliConfig.lockedFields') }}</span>
          </div>
          <div class="cli-fields">
            <div
              v-for="field in lockedFields"
              :key="field.key"
              class="cli-field locked"
            >
              <label class="cli-field-label">{{ field.key }}</label>
              <input
                type="text"
                :value="field.value"
                disabled
                class="cli-field-input disabled"
              />
              <span v-if="field.hint" class="cli-field-hint">{{ field.hint }}</span>
            </div>
          </div>
        </div>

        <!-- 可编辑字段 -->
        <div class="cli-section">
          <div class="cli-section-header">
            <span class="edit-icon">✏️</span>
            <span>{{ t('components.cliConfig.editableFields') }}</span>
          </div>
          <div class="cli-fields">
            <div
              v-for="field in editableFields"
              :key="field.key"
              class="cli-field"
            >
              <label class="cli-field-label">{{ field.key }}</label>

              <!-- 布尔类型 -->
              <template v-if="field.type === 'boolean'">
                <label class="cli-switch">
                  <input
                    type="checkbox"
                    :checked="getFieldValue(field.key)"
                    @change="updateField(field.key, ($event.target as HTMLInputElement).checked)"
                  />
                  <span class="cli-switch-slider"></span>
                </label>
              </template>

              <!-- 对象类型（JSON 编辑器） -->
              <template v-else-if="field.type === 'object'">
                <textarea
                  :value="JSON.stringify(getFieldValue(field.key) || {}, null, 2)"
                  class="cli-field-textarea"
                  rows="3"
                  @change="updateFieldJSON(field.key, ($event.target as HTMLTextAreaElement).value)"
                />
              </template>

              <!-- 字符串类型 -->
              <template v-else>
                <input
                  type="text"
                  :value="getFieldValue(field.key) || ''"
                  class="cli-field-input"
                  @input="updateField(field.key, ($event.target as HTMLInputElement).value)"
                />
              </template>
            </div>
          </div>
        </div>

        <!-- 模板选项 -->
        <div class="cli-template-options">
          <label class="cli-checkbox">
            <input
              type="checkbox"
              v-model="isGlobalTemplate"
              @change="handleTemplateChange"
            />
            <span>{{ t('components.cliConfig.setAsTemplate') }}</span>
          </label>
        </div>
      </template>

      <div v-else class="cli-error">
        {{ t('components.cliConfig.loadError') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  fetchCLIConfig,
  fetchCLITemplate,
  setCLITemplate,
  restoreDefaultConfig,
  type CLIPlatform,
  type CLIConfig,
  type CLIConfigField,
} from '../../services/cliConfig'
import { showToast } from '../../utils/toast'

const props = defineProps<{
  platform: CLIPlatform
  modelValue?: Record<string, any>
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, any>): void
}>()

const { t } = useI18n()

const expanded = ref(false)
const loading = ref(false)
const config = ref<CLIConfig | null>(null)
const editableValues = ref<Record<string, any>>({})
const isGlobalTemplate = ref(false)

const platformLabels: Record<CLIPlatform, string> = {
  claude: 'Claude Code',
  codex: 'Codex',
  gemini: 'Gemini',
}

const platformLabel = computed(() => platformLabels[props.platform] || props.platform)

const lockedFields = computed(() => {
  return config.value?.fields.filter(f => f.locked) || []
})

const editableFields = computed(() => {
  return config.value?.fields.filter(f => !f.locked) || []
})

// 获取字段值，支持嵌套的 env.* 字段
const getFieldValue = (key: string) => {
  if (key.startsWith('env.')) {
    const envKey = key.slice(4)
    const env = editableValues.value.env as Record<string, any> | undefined
    return env ? env[envKey] : undefined
  }
  return editableValues.value[key]
}

const toggleExpanded = () => {
  expanded.value = !expanded.value
  if (expanded.value && !config.value) {
    loadConfig()
  }
}

const loadConfig = async () => {
  loading.value = true
  try {
    config.value = await fetchCLIConfig(props.platform)
    editableValues.value = { ...(config.value?.editable || {}) }

    // 加载模板状态，并在新供应商时应用默认模板
    const template = await fetchCLITemplate(props.platform)
    isGlobalTemplate.value = template?.isGlobalDefault || false

    // 判断是否为新供应商（modelValue 为空或未定义）
    // 注意：editableValues 可能被后端填充了默认值，所以必须检查 modelValue
    const isNewProvider = !props.modelValue || Object.keys(props.modelValue).length === 0
    if (isNewProvider && template?.isGlobalDefault && template.template) {
      // 将模板值覆盖到当前可编辑值
      editableValues.value = { ...editableValues.value, ...template.template }
      emitChanges()
    }
  } catch (error) {
    console.error('Failed to load CLI config:', error)
    config.value = null
    showToast(t('components.cliConfig.loadError'), 'error')
  } finally {
    loading.value = false
  }
}

const updateField = (key: string, value: any) => {
  if (key.startsWith('env.')) {
    // 处理嵌套的 env.* 字段
    const envKey = key.slice(4)
    const env = { ...(editableValues.value.env as Record<string, any> || {}) }
    env[envKey] = value
    editableValues.value.env = env
  } else {
    editableValues.value[key] = value
  }
  emitChanges()
}

const updateFieldJSON = (key: string, jsonStr: string) => {
  try {
    const parsed = JSON.parse(jsonStr)
    editableValues.value[key] = parsed
    emitChanges()
  } catch {
    showToast(t('components.cliConfig.jsonParseError'), 'error')
  }
}

const emitChanges = () => {
  emit('update:modelValue', { ...editableValues.value })
}

const handleTemplateChange = async () => {
  try {
    // 无论是启用还是禁用模板，都保存状态
    await setCLITemplate(props.platform, editableValues.value, isGlobalTemplate.value)
    showToast(t('components.cliConfig.templateSaved'), 'success')
  } catch (error) {
    console.error('Failed to save template:', error)
    showToast(t('components.cliConfig.templateSaveError'), 'error')
    // 恢复原来的状态
    isGlobalTemplate.value = !isGlobalTemplate.value
  }
}

const handleRestoreDefault = async () => {
  if (!confirm(t('components.cliConfig.restoreConfirm'))) {
    return
  }

  try {
    await restoreDefaultConfig(props.platform)
    await loadConfig()
    showToast(t('components.cliConfig.restoreSuccess'), 'success')
  } catch (error) {
    console.error('Failed to restore default:', error)
    showToast(t('components.cliConfig.restoreError'), 'error')
  }
}

// 监听 modelValue 变化
watch(() => props.modelValue, (newVal) => {
  if (newVal && Object.keys(newVal).length > 0) {
    editableValues.value = { ...newVal }
  }
}, { immediate: true })

// 监听平台变化
watch(() => props.platform, () => {
  if (expanded.value) {
    loadConfig()
  } else {
    config.value = null
  }
})

onMounted(() => {
  // 如果有初始值，自动展开
  if (props.modelValue && Object.keys(props.modelValue).length > 0) {
    expanded.value = true
    loadConfig()
  }
})
</script>

<style scoped>
.cli-config-editor {
  border: 1px solid var(--mac-border);
  border-radius: 8px;
  overflow: hidden;
  margin-top: 16px;
}

.cli-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: var(--mac-surface);
  cursor: pointer;
  user-select: none;
  transition: background 0.2s;
}

.cli-header:hover {
  background: var(--mac-surface-strong);
}

.cli-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.expand-icon {
  width: 16px;
  height: 16px;
  transition: transform 0.2s;
  opacity: 0.6;
}

.expand-icon.expanded {
  transform: rotate(180deg);
}

.cli-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--mac-text);
}

.cli-platform-badge {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 4px;
  background: var(--mac-accent);
  color: white;
  font-weight: 500;
}

.cli-header-right {
  display: flex;
  gap: 8px;
}

.cli-action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  transition: background 0.2s;
}

.cli-action-btn:hover {
  background: var(--mac-surface-strong);
}

.cli-action-btn svg {
  width: 16px;
  height: 16px;
  color: var(--mac-text-secondary);
}

.cli-content {
  padding: 16px;
  border-top: 1px solid var(--mac-border);
  background: var(--mac-surface);
}

.cli-loading {
  text-align: center;
  padding: 24px;
  color: var(--mac-text-secondary);
  font-size: 14px;
}

.cli-error {
  text-align: center;
  padding: 24px;
  color: var(--mac-error);
  font-size: 14px;
}

.cli-section {
  margin-bottom: 20px;
}

.cli-section:last-child {
  margin-bottom: 0;
}

.cli-section-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 500;
  color: var(--mac-text-secondary);
  margin-bottom: 12px;
}

.lock-icon,
.edit-icon {
  font-size: 14px;
}

.cli-fields {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.cli-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.cli-field-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--mac-text);
  font-family: monospace;
}

.cli-field-input {
  padding: 8px 12px;
  border: 1px solid var(--mac-border);
  border-radius: 6px;
  font-size: 13px;
  background: var(--mac-bg);
  color: var(--mac-text);
  transition: border-color 0.2s;
}

.cli-field-input:focus {
  outline: none;
  border-color: var(--mac-accent);
}

.cli-field-input.disabled {
  background: var(--mac-surface-strong);
  color: var(--mac-text-secondary);
  cursor: not-allowed;
}

.cli-field-textarea {
  padding: 8px 12px;
  border: 1px solid var(--mac-border);
  border-radius: 6px;
  font-size: 12px;
  font-family: monospace;
  background: var(--mac-bg);
  color: var(--mac-text);
  resize: vertical;
  min-height: 60px;
}

.cli-field-textarea:focus {
  outline: none;
  border-color: var(--mac-accent);
}

.cli-field-hint {
  font-size: 11px;
  color: var(--mac-text-tertiary);
}

.cli-switch {
  position: relative;
  display: inline-block;
  width: 40px;
  height: 22px;
}

.cli-switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.cli-switch-slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--mac-border);
  border-radius: 22px;
  transition: 0.2s;
}

.cli-switch-slider:before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 2px;
  bottom: 2px;
  background-color: white;
  border-radius: 50%;
  transition: 0.2s;
}

.cli-switch input:checked + .cli-switch-slider {
  background-color: var(--mac-accent);
}

.cli-switch input:checked + .cli-switch-slider:before {
  transform: translateX(18px);
}

.cli-template-options {
  padding-top: 16px;
  border-top: 1px solid var(--mac-border);
  margin-top: 16px;
}

.cli-checkbox {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--mac-text);
  cursor: pointer;
}

.cli-checkbox input {
  width: 16px;
  height: 16px;
  cursor: pointer;
}

/* 深色模式适配 */
:global(.dark) .cli-field-input {
  background: var(--mac-surface-strong);
}

:global(.dark) .cli-field-textarea {
  background: var(--mac-surface-strong);
}

:global(.dark) .cli-field-input.disabled {
  background: var(--mac-bg);
}
</style>
