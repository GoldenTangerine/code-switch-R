<template>
  <div class="cli-config-editor">
    <div class="cli-header">
      <div class="cli-header-left">
        <span class="cli-title">{{ t('components.cliConfig.title') }}</span>
        <span class="cli-platform-badge">{{ platformLabel }}</span>
      </div>

      <div class="cli-header-right">
        <button
          class="cli-action-btn cli-action-btn--icon"
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

    <div class="cli-content">
      <div v-if="loading" class="cli-loading">
        {{ t('components.cliConfig.loading') }}
      </div>

      <template v-else-if="config">
        <section class="cli-section">
          <div class="cli-section-header">
            <span class="cli-section-title">{{ t('components.cliConfig.lockedFields') }}</span>
            <span v-if="lockedFields.length" class="cli-section-count">{{ lockedFields.length }}</span>
          </div>

          <div v-if="lockedFields.length" class="cli-fields">
            <div
              v-for="field in lockedFields"
              :key="field.key"
              class="cli-field locked"
            >
              <label class="cli-field-label">{{ field.key }}</label>
              <input
                type="text"
                :value="field.value ?? ''"
                disabled
                class="cli-field-input disabled"
              />
              <span v-if="field.hint" class="cli-field-hint">{{ field.hint }}</span>
            </div>
          </div>

          <p v-else class="cli-empty-state">
            {{ t('components.cliConfig.noLockedFields') }}
          </p>
        </section>

        <section class="cli-section cli-json-section">
          <div class="cli-json-header">
            <div class="cli-json-header-main">
              <span class="cli-json-title">{{ t('components.cliConfig.jsonEditor.title') }}</span>
              <span v-if="cliJsonDirty" class="cli-json-dirty">
                {{ t('components.cliConfig.jsonEditor.dirty') }}
              </span>
            </div>

            <div class="cli-json-actions" @click.stop>
              <button
                type="button"
                class="cli-action-btn"
                :disabled="!cliJsonDirty"
                @click="resetCliJsonFromValues"
              >
                {{ t('components.cliConfig.jsonEditor.reset') }}
              </button>
              <button
                type="button"
                class="cli-action-btn cli-primary-btn"
                :disabled="!cliJsonDirty || !!cliJsonError"
                @click="applyCliJsonToValues"
              >
                {{ t('components.cliConfig.jsonEditor.apply') }}
              </button>
            </div>
          </div>

          <JsonCodeEditor
            ref="cliJsonEditorRef"
            v-model="cliJsonEditingText"
            :rows="14"
            :invalid="!!cliJsonError"
            :placeholder="cliJsonPlaceholder"
            :show-validation="true"
          />

          <p v-if="cliJsonError" class="cli-json-error" role="alert">
            {{ cliJsonError }}
          </p>
          <p class="cli-json-hint">{{ t('components.cliConfig.jsonEditor.hint') }}</p>
        </section>

        <section v-if="previewFiles.length || currentFiles.length" class="cli-section cli-preview-section">
          <div class="cli-section-header">
            <span class="cli-section-title">{{ t('components.cliConfig.previewTitle') }}</span>
            <span class="cli-section-count">{{ currentPreviewCount }}</span>
          </div>

          <TabGroup :selectedIndex="selectedPreviewTab" @change="selectedPreviewTab = $event">
            <TabList class="cli-tabs-list">
              <Tab as="template" v-slot="{ selected }">
                <button :class="['cli-tab-btn', { selected }]">
                  {{ t('components.cliConfig.tabPreview') }}
                </button>
              </Tab>
              <Tab as="template" v-slot="{ selected }">
                <button :class="['cli-tab-btn', { selected }]">
                  {{ t('components.cliConfig.tabCurrent') }}
                </button>
              </Tab>
            </TabList>

            <TabPanels>
              <TabPanel class="cli-preview-list">
                <p v-if="previewFiles.length === 0" class="cli-empty-state">
                  {{ t('components.cliConfig.previewEmpty') }}
                </p>

                <div
                  v-for="(file, index) in previewFiles"
                  :key="getFileKey(file, index)"
                  class="cli-preview-card"
                >
                  <div class="cli-preview-meta">
                    <span class="cli-preview-name">
                      {{ file.path || t('components.cliConfig.previewUnknownPath') }}
                    </span>
                    <span class="cli-preview-format">{{ (file.format || config?.configFormat || '').toUpperCase() }}</span>
                  </div>
                  <pre class="cli-preview-content">{{ file.content || '' }}</pre>
                </div>
              </TabPanel>

              <TabPanel class="cli-preview-list">
                <p v-if="currentFiles.length === 0" class="cli-empty-state">
                  {{ t('components.cliConfig.previewEmpty') }}
                </p>

                <div
                  v-for="(file, index) in currentFiles"
                  :key="`current-${getFileKey(file, index)}`"
                  class="cli-preview-card"
                >
                  <div class="cli-preview-meta">
                    <span class="cli-preview-name">
                      {{ file.path || t('components.cliConfig.previewUnknownPath') }}
                    </span>
                    <span class="cli-preview-format">{{ (file.format || config?.configFormat || '').toUpperCase() }}</span>
                  </div>
                  <pre class="cli-preview-content">{{ file.content || '' }}</pre>
                </div>
              </TabPanel>
            </TabPanels>
          </TabGroup>
        </section>
      </template>

      <div v-else class="cli-error">
        {{ t('components.cliConfig.loadError') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Tab, TabGroup, TabList, TabPanel, TabPanels } from '@headlessui/vue'
import JsonCodeEditor from './JsonCodeEditor.vue'
import {
  fetchCLIConfig,
  fetchCLIConfigSnapshots,
  restoreDefaultConfig,
  type CLIConfig,
  type CLIConfigFile,
  type CLIConfigSnapshots,
  type CLIPlatform,
} from '../../services/cliConfig'
import { showToast } from '../../utils/toast'

const props = defineProps<{
  platform: CLIPlatform
  modelValue?: Record<string, any>
  providerConfig?: {
    apiKey?: string
    baseUrl?: string
  }
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, any>): void
}>()

const { t } = useI18n()

type FormatCliConfigJsonResult =
  | { ok: true; text: string; value: Record<string, any> }
  | { ok: false; error: string }

type SetEditableValuesOptions = {
  emit?: boolean
  forceSyncText?: boolean
}

type LoadConfigOptions = {
  mergeModelValue?: boolean
  emitChanges?: boolean
  forceSyncText?: boolean
}

const loading = ref(false)
const config = ref<CLIConfig | null>(null)
const editableValues = ref<Record<string, any>>({})
const cliJsonEditorRef = ref<InstanceType<typeof JsonCodeEditor> | null>(null)
const cliJsonSyncedText = ref('')
const cliJsonEditingText = ref('')
const cliJsonError = ref('')
const selectedPreviewTab = ref(0)
const snapshotsData = ref<CLIConfigSnapshots | null>(null)

let snapshotDebounceTimer: ReturnType<typeof setTimeout> | null = null
let snapshotRequestSeq = 0
let loadConfigRequestSeq = 0

const clearSnapshotDebounceTimer = () => {
  if (snapshotDebounceTimer) {
    clearTimeout(snapshotDebounceTimer)
    snapshotDebounceTimer = null
  }
}

const cloneCliConfigValue = <T>(value: T): T => {
  if (value == null) return value
  return JSON.parse(JSON.stringify(value))
}

const normalizeCliConfigRecord = (value: Record<string, any> | undefined | null): Record<string, any> =>
  cloneCliConfigValue(value || {})

const toSortedJsonValue = (value: unknown): unknown => {
  if (Array.isArray(value)) {
    return value.map((item) => toSortedJsonValue(item))
  }

  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>)
        .sort(([left], [right]) => left.localeCompare(right))
        .map(([key, entryValue]) => [key, toSortedJsonValue(entryValue)]),
    )
  }

  return value
}

const buildCliJsonText = (value: Record<string, any>) =>
  JSON.stringify(toSortedJsonValue(normalizeCliConfigRecord(value)), null, 2)

const platformLabels: Record<CLIPlatform, string> = {
  claude: 'Claude Code',
  codex: 'Codex',
  gemini: 'Gemini',
}

const stripJsonErrorMessage = (message: string) => message
  .replace(/^JSON\.parse:\s*/i, '')
  .replace(/\s+of the JSON data$/i, '')
  .trim()

const parseJsonError = (error: unknown): string => {
  if (!(error instanceof SyntaxError)) {
    return t('components.cliConfig.jsonEditor.errors.invalidJsonGeneric')
  }

  const rawMessage = error.message
  const positionMatch = rawMessage.match(/at position (\d+)/i)
  if (positionMatch) {
    const detail = stripJsonErrorMessage(rawMessage.replace(/\s+in JSON at position \d+/i, ''))
    return t('components.cliConfig.jsonEditor.errors.invalidJsonAtPosition', {
      message: detail || t('components.cliConfig.jsonEditor.errors.invalidJsonGeneric'),
      position: positionMatch[1],
    })
  }

  const lineColumnMatch = rawMessage.match(/line (\d+) column (\d+)/i)
  if (lineColumnMatch) {
    return t('components.cliConfig.jsonEditor.errors.invalidJsonAtLineColumn', {
      line: lineColumnMatch[1],
      column: lineColumnMatch[2],
    })
  }

  return t('components.cliConfig.jsonEditor.errors.invalidJson', {
    message: stripJsonErrorMessage(rawMessage) || t('components.cliConfig.jsonEditor.errors.invalidJsonGeneric'),
  })
}

const platformLabel = computed(() => platformLabels[props.platform] || props.platform)

const cliJsonDirty = computed(() => cliJsonEditingText.value !== cliJsonSyncedText.value)

const cliJsonPlaceholder = computed(() => {
  if (props.platform === 'claude') {
    return `{
  "env": {
    "ANTHROPIC_BASE_URL": "https://api.example.com",
    "ANTHROPIC_AUTH_TOKEN": "your-api-key"
  }
}`
  }

  if (props.platform === 'codex') {
    return `{
  "model": "gpt-4.1",
  "providers": {
    "default": {
      "base_url": "https://api.example.com/v1"
    }
  }
}`
  }

  return `{
  "GEMINI_MODEL": "gemini-2.5-pro",
  "HTTP_PROXY": "http://127.0.0.1:7890"
}`
})

const hasProviderInput = computed(() => {
  const apiKey = props.providerConfig?.apiKey?.trim() || ''
  const baseUrl = props.providerConfig?.baseUrl?.trim() || ''

  if (props.platform === 'gemini') {
    return !!(apiKey || baseUrl)
  }

  return !!(apiKey && baseUrl)
})

const lockedFields = computed(() => {
  const fields = config.value?.fields.filter((field) => field.locked) || []
  if (!hasProviderInput.value) return fields

  const apiKey = props.providerConfig?.apiKey?.trim() || ''
  const baseUrl = props.providerConfig?.baseUrl?.trim() || ''

  return fields.map((field) => {
    const nextField = { ...field }

    if (props.platform === 'gemini') {
      if (field.key === 'GEMINI_API_KEY' && apiKey) {
        nextField.value = apiKey
      }
      if (field.key === 'GOOGLE_GEMINI_BASE_URL' && baseUrl) {
        nextField.value = baseUrl
      }
    }

    if (props.platform === 'claude') {
      if (field.key === 'env.ANTHROPIC_BASE_URL' && baseUrl) {
        nextField.value = baseUrl
      }
      if (field.key === 'env.ANTHROPIC_AUTH_TOKEN' && apiKey) {
        nextField.value = apiKey
      }
    }

    if (props.platform === 'codex') {
      if (field.key.includes('.base_url') && baseUrl) {
        nextField.value = baseUrl
      }
      if (field.key === 'OPENAI_API_KEY' && apiKey) {
        nextField.value = apiKey
      }
    }

    return nextField
  })
})

const previewFiles = computed((): CLIConfigFile[] => snapshotsData.value?.previewFiles || [])
const currentFiles = computed((): CLIConfigFile[] => snapshotsData.value?.currentFiles || [])

const currentPreviewCount = computed(() => (
  selectedPreviewTab.value === 0 ? previewFiles.value.length : currentFiles.value.length
))

const getFileKey = (file: CLIConfigFile, index: number) => file.path || `${file.format || 'file'}-${index}`

const focusCliJsonEditor = () => {
  requestAnimationFrame(() => {
    cliJsonEditorRef.value?.focus()
  })
}

const emitChanges = () => {
  emit('update:modelValue', normalizeCliConfigRecord(editableValues.value))
}

const syncCliJsonFromValues = (forceSyncText = false) => {
  const previousSynced = cliJsonSyncedText.value
  const nextSynced = buildCliJsonText(editableValues.value)
  cliJsonSyncedText.value = nextSynced

  const editingWasSynced = cliJsonEditingText.value === previousSynced
  if (forceSyncText || editingWasSynced || !cliJsonEditingText.value) {
    cliJsonEditingText.value = nextSynced
  }

  if (forceSyncText) {
    cliJsonError.value = ''
  }
}

const setEditableValues = (
  value: Record<string, any> | undefined | null,
  options: SetEditableValuesOptions = {},
) => {
  editableValues.value = normalizeCliConfigRecord(value)
  syncCliJsonFromValues(options.forceSyncText ?? false)
  if (options.emit) {
    emitChanges()
  }
}

const formatCliConfigJson = (input: string): FormatCliConfigJsonResult => {
  const trimmed = input.trim()
  if (!trimmed) {
    return { ok: true, text: buildCliJsonText({}), value: {} }
  }

  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch (error) {
    return {
      ok: false,
      error: parseJsonError(error),
    }
  }

  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return {
      ok: false,
      error: t('components.cliConfig.jsonEditor.errors.mustBeObject'),
    }
  }

  const value = normalizeCliConfigRecord(parsed as Record<string, any>)
  return {
    ok: true,
    text: buildCliJsonText(value),
    value,
  }
}

const applyCliJsonToValues = () => {
  cliJsonError.value = ''
  const formatted = formatCliConfigJson(cliJsonEditingText.value)
  if (!formatted.ok) {
    cliJsonError.value = formatted.error
    focusCliJsonEditor()
    return false
  }

  setEditableValues(formatted.value, {
    emit: true,
    forceSyncText: true,
  })
  cliJsonEditingText.value = formatted.text
  cliJsonError.value = ''
  return true
}

const resetCliJsonFromValues = () => {
  cliJsonError.value = ''
  cliJsonEditingText.value = cliJsonSyncedText.value
}

const applyPendingJsonChanges = () => {
  if (!cliJsonDirty.value) return true
  return applyCliJsonToValues()
}

defineExpose({
  applyPendingJsonChanges,
})

const loadSnapshots = async () => {
  const currentSeq = ++snapshotRequestSeq

  try {
    const previewMode = hasProviderInput.value ? 'direct' : 'current'
    const apiUrl = props.providerConfig?.baseUrl?.trim() || ''
    const apiKey = props.providerConfig?.apiKey?.trim() || ''

    const result = await fetchCLIConfigSnapshots(
      props.platform,
      apiUrl,
      apiKey,
      previewMode,
    )

    if (currentSeq !== snapshotRequestSeq) {
      return
    }

    snapshotsData.value = result
  } catch (error) {
    if (currentSeq !== snapshotRequestSeq) {
      return
    }

    console.error('Failed to load CLI config snapshots:', error)
    snapshotsData.value = null
  }
}

const loadSnapshotsDebounced = () => {
  clearSnapshotDebounceTimer()

  snapshotDebounceTimer = setTimeout(() => {
    loadSnapshots()
  }, 300)
}

const loadConfig = async () => {
  await loadConfigWithOptions()
}

const loadConfigWithOptions = async (options: LoadConfigOptions = {}) => {
  const currentSeq = ++loadConfigRequestSeq
  clearSnapshotDebounceTimer()
  snapshotRequestSeq += 1
  snapshotsData.value = null
  loading.value = true

  const shouldMergeModelValue = options.mergeModelValue ?? true
  const shouldEmitChanges = options.emitChanges ?? false
  const shouldForceSyncText = options.forceSyncText ?? true

  try {
    const nextConfig = await fetchCLIConfig(props.platform)
    if (currentSeq !== loadConfigRequestSeq) {
      return
    }

    config.value = nextConfig

    let nextEditableValues = normalizeCliConfigRecord(nextConfig.editable || {})
    if (
      shouldMergeModelValue
      && props.modelValue
      && Object.keys(props.modelValue).length > 0
    ) {
      nextEditableValues = {
        ...nextEditableValues,
        ...normalizeCliConfigRecord(props.modelValue),
      }
    }

    setEditableValues(nextEditableValues, {
      emit: shouldEmitChanges,
      forceSyncText: shouldForceSyncText,
    })
    await loadSnapshots()
  } catch (error) {
    if (currentSeq !== loadConfigRequestSeq) {
      return
    }

    console.error('Failed to load CLI config:', error)
    config.value = null
    snapshotsData.value = null
    showToast(t('components.cliConfig.loadError'), 'error')
  } finally {
    if (currentSeq === loadConfigRequestSeq) {
      loading.value = false
    }
  }
}

const handleRestoreDefault = async () => {
  if (!confirm(t('components.cliConfig.restoreConfirm'))) {
    return
  }

  const currentPlatform = props.platform

  try {
    await restoreDefaultConfig(currentPlatform)
    if (currentPlatform !== props.platform) {
      return
    }

    await loadConfigWithOptions({
      mergeModelValue: false,
      emitChanges: true,
      forceSyncText: true,
    })
    showToast(t('components.cliConfig.restoreSuccess'), 'success')
  } catch (error) {
    if (currentPlatform !== props.platform) {
      return
    }

    console.error('Failed to restore default:', error)
    showToast(t('components.cliConfig.restoreError'), 'error')
  }
}

watch(() => props.modelValue, (newValue) => {
  if (newValue && Object.keys(newValue).length > 0) {
    setEditableValues(newValue, { forceSyncText: true })
    return
  }

  setEditableValues({}, { forceSyncText: true })
}, { immediate: true, deep: true })

watch(cliJsonEditingText, (value) => {
  const formatted = formatCliConfigJson(value)
  cliJsonError.value = formatted.ok ? '' : formatted.error
})

watch(() => props.platform, () => {
  selectedPreviewTab.value = 0
  loadConfig()
})

watch(
  () => [props.providerConfig?.apiKey, props.providerConfig?.baseUrl],
  () => {
    if (config.value) {
      loadSnapshotsDebounced()
    }
  },
  { deep: true },
)

onMounted(() => {
  loadConfig()
})

onUnmounted(() => {
  clearSnapshotDebounceTimer()
  loadConfigRequestSeq += 1
  snapshotRequestSeq += 1
})
</script>

<style scoped>
.cli-config-editor {
  margin-top: 16px;
  border: 1px solid var(--mac-border);
  border-radius: 12px;
  overflow: hidden;
  background: color-mix(in srgb, var(--mac-surface) 94%, transparent);
}

.cli-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  background: color-mix(in srgb, var(--mac-surface) 88%, transparent);
  border-bottom: 1px solid var(--mac-border);
}

.cli-header-left,
.cli-header-right,
.cli-section-header,
.cli-json-header,
.cli-json-header-main,
.cli-json-actions,
.cli-preview-meta {
  display: flex;
  align-items: center;
}

.cli-header-left,
.cli-json-header-main {
  gap: 8px;
}

.cli-header-right,
.cli-json-actions {
  gap: 10px;
}

.cli-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--mac-text);
}

.cli-platform-badge {
  padding: 2px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--mac-accent) 16%, transparent);
  color: var(--mac-accent);
  font-size: 11px;
  font-weight: 700;
}

.cli-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 18px 16px 16px;
}

.cli-loading,
.cli-error,
.cli-empty-state {
  padding: 14px 0;
  font-size: 13px;
  color: var(--mac-text-secondary);
}

.cli-error {
  color: var(--mac-error, #ff453a);
}

.cli-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.cli-section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--mac-text);
}

.cli-section-count {
  margin-left: auto;
  min-width: 24px;
  padding: 2px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--mac-surface-strong) 90%, transparent);
  color: var(--mac-text-secondary);
  font-size: 11px;
  font-weight: 600;
  text-align: center;
}

.cli-fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 12px;
}

.cli-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 14px;
  border: 1px solid var(--mac-border);
  border-radius: 14px;
  background: color-mix(in srgb, var(--mac-surface-strong) 92%, transparent);
}

.cli-field-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--mac-text-secondary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  word-break: break-all;
}

.cli-field-input {
  width: 100%;
  min-height: 38px;
  padding: 0 12px;
  border: 1px solid var(--mac-border);
  border-radius: 10px;
  background: var(--mac-surface);
  color: var(--mac-text);
  font-size: 13px;
}

.cli-field-input.disabled {
  background: color-mix(in srgb, var(--mac-surface) 88%, transparent);
  color: color-mix(in srgb, var(--mac-text) 92%, var(--mac-text-secondary));
  cursor: default;
}

.cli-field-hint,
.cli-json-hint {
  font-size: 12px;
  line-height: 1.6;
  color: var(--mac-text-secondary);
}

.cli-json-section {
  gap: 10px;
}

.cli-json-header {
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.cli-json-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--mac-text);
}

.cli-json-dirty {
  padding: 2px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, #f59e0b 16%, transparent);
  color: #b45309;
  font-size: 11px;
  font-weight: 700;
}

.cli-json-error {
  margin: 0;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, #ff453a 22%, transparent);
  border-radius: 12px;
  background: color-mix(in srgb, #ff453a 10%, transparent);
  color: var(--mac-error, #ff453a);
  font-size: 12px;
  line-height: 1.5;
}

.cli-preview-section {
  padding-top: 4px;
}

.cli-tabs-list {
  display: flex;
  gap: 4px;
  padding: 4px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--mac-surface-strong) 92%, transparent);
}

.cli-tab-btn {
  flex: 1;
  min-height: 34px;
  padding: 0 12px;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--mac-text-secondary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.cli-tab-btn:hover:not(.selected) {
  color: var(--mac-text);
  background: color-mix(in srgb, var(--mac-surface) 88%, transparent);
}

.cli-tab-btn.selected {
  background: var(--mac-accent);
  color: #ffffff;
}

.cli-preview-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 12px;
}

.cli-preview-card {
  border: 1px solid var(--mac-border);
  border-radius: 14px;
  overflow: hidden;
  background: color-mix(in srgb, var(--mac-surface-strong) 92%, transparent);
}

.cli-preview-meta {
  justify-content: space-between;
  gap: 8px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface) 86%, transparent);
}

.cli-preview-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
  color: var(--mac-text-secondary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}

.cli-preview-format {
  flex-shrink: 0;
  padding: 2px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--mac-accent) 16%, transparent);
  color: var(--mac-accent);
  font-size: 10px;
  font-weight: 700;
}

.cli-preview-content {
  margin: 0;
  padding: 12px;
  max-height: 220px;
  overflow: auto;
  background: transparent;
  color: var(--mac-text);
  font-size: 11px;
  line-height: 1.6;
  white-space: pre;
  word-break: normal;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}

.cli-action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 32px;
  padding: 0 12px;
  border: 1px solid var(--mac-border);
  border-radius: 10px;
  background: var(--mac-surface);
  color: var(--mac-text);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.cli-action-btn:hover:not(:disabled) {
  border-color: color-mix(in srgb, var(--mac-accent) 45%, var(--mac-border));
  color: var(--mac-accent);
}

.cli-action-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.cli-action-btn svg {
  width: 16px;
  height: 16px;
}

.cli-action-btn--icon {
  width: 32px;
  padding: 0;
  background: transparent;
}

.cli-action-btn--icon:hover:not(:disabled) {
  background: color-mix(in srgb, var(--mac-surface-strong) 92%, transparent);
  color: var(--mac-text);
}

.cli-primary-btn {
  border-color: color-mix(in srgb, var(--mac-accent) 42%, var(--mac-border));
  background: var(--mac-accent);
  color: #ffffff;
}

.cli-primary-btn:hover:not(:disabled) {
  color: #ffffff;
  opacity: 0.92;
}

@media (max-width: 720px) {
  .cli-content {
    padding: 16px 14px 14px;
  }

  .cli-fields {
    grid-template-columns: 1fr;
  }

  .cli-json-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
