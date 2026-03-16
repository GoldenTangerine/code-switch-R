<template>
  <div class="cli-config-editor">
    <div class="cli-header">
      <div class="cli-header-left">
        <span class="cli-title">{{ t('components.cliConfig.title') }}</span>
        <span class="cli-platform-badge">{{ platformLabel }}</span>
      </div>

      <div class="cli-header-right">
        <button
          class="cli-action-btn"
          type="button"
          @click="openGlobalTemplateModal"
        >
          {{ t('components.cliConfig.editTemplate') }}
        </button>
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
          <p class="cli-json-hint">{{ cliJsonHint }}</p>

          <label class="cli-template-inject">
            <input
              type="checkbox"
              :checked="sharedTemplateEnabled"
              :disabled="!hasSharedTemplate && !sharedTemplateEnabled"
              @change="handleSharedTemplateToggle(($event.target as HTMLInputElement).checked)"
            />
            <span>{{ t('components.cliConfig.injectTemplate') }}</span>
          </label>
          <p class="cli-json-hint">{{ t('components.cliConfig.injectTemplateHint') }}</p>
        </section>

        <section v-if="previewDisplayFiles.length || currentFiles.length" class="cli-section cli-preview-section">
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
                <p v-if="previewDisplayFiles.length === 0" class="cli-empty-state">
                  {{ t('components.cliConfig.previewEmpty') }}
                </p>

                <div
                  v-for="(file, index) in previewDisplayFiles"
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

  <InlineModal
    :open="globalTemplateModalOpen"
    :title="t('components.cliConfig.templateDialogTitle', { platform: platformLabel })"
    :panel-width="'min(820px, 94vw)'"
    :body-scrollable="false"
    @close="closeGlobalTemplateModal"
  >
    <div class="cli-template-modal">
      <p class="cli-template-hint">{{ t('components.cliConfig.templateHint') }}</p>

      <div v-if="globalTemplateLoading" class="cli-loading">
        {{ t('components.cliConfig.loading') }}
      </div>

      <template v-else>
        <JsonCodeEditor
          v-model="globalTemplateEditingText"
          :rows="16"
          :invalid="!!globalTemplateError"
          :placeholder="cliJsonPlaceholder"
          :show-validation="true"
        />

        <p v-if="globalTemplateError" class="cli-json-error" role="alert">
          {{ globalTemplateError }}
        </p>

        <label class="cli-template-checkbox">
          <input v-model="globalTemplateEnabled" type="checkbox" />
          <span>{{ t('components.cliConfig.writeTemplate') }}</span>
        </label>

        <div class="cli-template-actions">
          <button
            type="button"
            class="cli-action-btn"
            @click="closeGlobalTemplateModal"
          >
            {{ t('components.main.form.actions.cancel') }}
          </button>
          <button
            type="button"
            class="cli-action-btn cli-primary-btn"
            :disabled="globalTemplateSaving || !globalTemplateDirty || !!globalTemplateError"
            @click="saveGlobalTemplate"
          >
            {{ t('components.main.form.actions.save') }}
          </button>
        </div>
      </template>
    </div>
  </InlineModal>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Tab, TabGroup, TabList, TabPanel, TabPanels } from '@headlessui/vue'
import InlineModal from './InlineModal.vue'
import JsonCodeEditor from './JsonCodeEditor.vue'
import {
  fetchCLIConfig,
  fetchEditableCLIConfigSnapshots,
  fetchCLITemplate,
  restoreDefaultConfig,
  setCLITemplate,
  type CLIConfig,
  type CLIConfigFile,
  type CLIConfigSnapshots,
  type CLITemplate,
  type CLIPlatform,
} from '../../services/cliConfig'
import { showToast } from '../../utils/toast'
import { extractErrorMessage } from '../../utils/error'

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
  applyTemplateWhenEmpty?: boolean
}

type TemplateInjectionEntry = {
  path: string[]
  hadValue: boolean
  previousValue?: any
}

type CliConfigSubmitState = {
  value: Record<string, any>
  shouldPersist: boolean
}

const loading = ref(false)
const config = ref<CLIConfig | null>(null)
const baseEditableValues = ref<Record<string, any>>({})
const editableValues = ref<Record<string, any>>({})
const cliJsonEditorRef = ref<InstanceType<typeof JsonCodeEditor> | null>(null)
const cliJsonSyncedText = ref('')
const cliJsonEditingText = ref('')
const cliJsonError = ref('')
const selectedPreviewTab = ref(0)
const snapshotsData = ref<CLIConfigSnapshots | null>(null)
const templateState = ref<CLITemplate | null>(null)
const globalTemplateModalOpen = ref(false)
const globalTemplateLoading = ref(false)
const globalTemplateSaving = ref(false)
const globalTemplateEnabled = ref(false)
const globalTemplateSyncedText = ref('')
const globalTemplateSyncedEnabled = ref(false)
const globalTemplateEditingText = ref('')
const globalTemplateError = ref('')
const sharedTemplateEnabled = ref(false)
const sharedTemplateInjectedEntries = ref<TemplateInjectionEntry[]>([])
const persistBaselineValue = ref<Record<string, any>>({})
const initialModelHasExplicitValue = ref(false)

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

const hasConfigKeys = (value: Record<string, any> | undefined | null) =>
  Object.keys(normalizeCliConfigRecord(value)).length > 0

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

const buildComparableCliConfigText = (value: Record<string, any> | undefined | null) =>
  JSON.stringify(toSortedJsonValue(normalizeCliConfigRecord(value)))

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
const editorUsesEffectiveJson = computed(() => props.platform === 'claude')
const cliJsonDirty = computed(() => cliJsonEditingText.value !== cliJsonSyncedText.value)
const globalTemplateDirty = computed(() => (
  globalTemplateEditingText.value !== globalTemplateSyncedText.value
  || globalTemplateEnabled.value !== globalTemplateSyncedEnabled.value
))
const hasSharedTemplate = computed(() => (
  !!templateState.value?.template
  && Object.keys(templateState.value.template).length > 0
))

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

const cliJsonHint = computed(() => (
  editorUsesEffectiveJson.value
    ? t('components.cliConfig.jsonEditor.fileHint')
    : t('components.cliConfig.jsonEditor.hint')
))

const hasProviderInput = computed(() => {
  const apiKey = props.providerConfig?.apiKey?.trim() || ''
  const baseUrl = props.providerConfig?.baseUrl?.trim() || ''

  if (props.platform === 'gemini') {
    return !!(apiKey || baseUrl)
  }

  return !!(apiKey && baseUrl)
})

const baseLockedFields = computed(() => config.value?.fields.filter((field) => field.locked) || [])

const lockedFields = computed(() => {
  const fields = baseLockedFields.value
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

const previewDisplayFiles = computed((): CLIConfigFile[] => {
  if (!editorUsesEffectiveJson.value || previewFiles.value.length === 0) {
    return previewFiles.value
  }

  return previewFiles.value.map((file, index) => (
    index === 0 && (file.format || 'json').toLowerCase() === 'json'
      ? { ...file, content: cliJsonEditingText.value || file.content }
      : file
  ))
})

const currentPreviewCount = computed(() => (
  selectedPreviewTab.value === 0 ? previewDisplayFiles.value.length : currentFiles.value.length
))

const getFileKey = (file: CLIConfigFile, index: number) => file.path || `${file.format || 'file'}-${index}`

const focusCliJsonEditor = () => {
  requestAnimationFrame(() => {
    cliJsonEditorRef.value?.focus()
  })
}

const getLockedFieldValue = (key: string, useEffectiveValues = true) => {
  const source = useEffectiveValues ? lockedFields.value : baseLockedFields.value
  return source.find((field) => field.key === key)?.value ?? ''
}

const stripLockedKeysFromEditable = (value: Record<string, any>) => {
  const nextValue = normalizeCliConfigRecord(value)

  baseLockedFields.value.forEach((field) => {
    if (field.key.startsWith('env.')) {
      const envKey = field.key.slice(4)
      const env = nextValue.env
      if (env && typeof env === 'object' && !Array.isArray(env)) {
        delete (env as Record<string, any>)[envKey]
        if (Object.keys(env as Record<string, any>).length === 0) {
          delete nextValue.env
        }
      }
      return
    }

    delete nextValue[field.key]
  })

  return nextValue
}

const buildClaudeFileJsonValue = (
  value: Record<string, any>,
  useEffectiveLockedValues = true,
) => {
  const nextValue = normalizeCliConfigRecord(value)
  const envValue = nextValue.env
  const env = envValue && typeof envValue === 'object' && !Array.isArray(envValue)
    ? normalizeCliConfigRecord(envValue as Record<string, any>)
    : {}

  const anthropicBaseUrl = getLockedFieldValue('env.ANTHROPIC_BASE_URL', useEffectiveLockedValues)
  const anthropicAuthToken = getLockedFieldValue('env.ANTHROPIC_AUTH_TOKEN', useEffectiveLockedValues)

  if (anthropicBaseUrl) {
    env.ANTHROPIC_BASE_URL = anthropicBaseUrl
  } else {
    delete env.ANTHROPIC_BASE_URL
  }

  if (anthropicAuthToken) {
    env.ANTHROPIC_AUTH_TOKEN = anthropicAuthToken
  } else {
    delete env.ANTHROPIC_AUTH_TOKEN
  }

  if (Object.keys(env).length > 0) {
    nextValue.env = env
  } else {
    delete nextValue.env
  }

  return nextValue
}

const buildEditorJsonText = (value: Record<string, any>) => (
  editorUsesEffectiveJson.value
    ? JSON.stringify(toSortedJsonValue(buildClaudeFileJsonValue(value, true)), null, 2)
    : buildCliJsonText(value)
)

const buildTemplateJsonText = (value: Record<string, any>) => (
  props.platform === 'claude'
    ? JSON.stringify(toSortedJsonValue(buildClaudeFileJsonValue(value, false)), null, 2)
    : buildCliJsonText(value)
)

const isPlainObjectRecord = (value: unknown): value is Record<string, any> => (
  !!value
  && typeof value === 'object'
  && !Array.isArray(value)
)

const mergeMissingTemplateKeys = (
  currentValue: Record<string, any>,
  templateValue: Record<string, any>,
): Record<string, any> => {
  const nextValue = normalizeCliConfigRecord(currentValue)

  Object.entries(templateValue).forEach(([key, value]) => {
    if (!(key in nextValue) || nextValue[key] == null) {
      nextValue[key] = cloneCliConfigValue(value)
      return
    }

    if (isPlainObjectRecord(nextValue[key]) && isPlainObjectRecord(value)) {
      nextValue[key] = mergeMissingTemplateKeys(
        nextValue[key] as Record<string, any>,
        value,
      )
    }
  })

  return nextValue
}

const mergeMissingTemplateKeysWithTracking = (
  currentValue: Record<string, any>,
  templateValue: Record<string, any>,
  path: string[] = [],
): { value: Record<string, any>; injectedEntries: TemplateInjectionEntry[] } => {
  const nextValue = normalizeCliConfigRecord(currentValue)
  const injectedEntries: TemplateInjectionEntry[] = []

  Object.entries(templateValue).forEach(([key, value]) => {
    const nextPath = [...path, key]

    if (!(key in nextValue)) {
      if (isPlainObjectRecord(value)) {
        const nestedResult = mergeMissingTemplateKeysWithTracking({}, value, nextPath)
        nextValue[key] = nestedResult.value
        if (Object.keys(value).length === 0) {
          injectedEntries.push({ path: nextPath, hadValue: false })
        }
        injectedEntries.push(...nestedResult.injectedEntries)
        return
      }

      nextValue[key] = cloneCliConfigValue(value)
      injectedEntries.push({ path: nextPath, hadValue: false })
      return
    }

    if (nextValue[key] == null) {
      const previousValue = cloneCliConfigValue(nextValue[key])
      nextValue[key] = cloneCliConfigValue(value)
      injectedEntries.push({
        path: nextPath,
        hadValue: true,
        previousValue,
      })
      return
    }

    if (isPlainObjectRecord(nextValue[key]) && isPlainObjectRecord(value)) {
      const nestedResult = mergeMissingTemplateKeysWithTracking(
        nextValue[key] as Record<string, any>,
        value,
        nextPath,
      )
      nextValue[key] = nestedResult.value
      injectedEntries.push(...nestedResult.injectedEntries)
    }
  })

  return { value: nextValue, injectedEntries }
}

const setValueAtPath = (target: Record<string, any>, path: string[], value: unknown) => {
  if (path.length === 0) return

  let current = target
  path.slice(0, -1).forEach((segment) => {
    if (!isPlainObjectRecord(current[segment])) {
      current[segment] = {}
    }
    current = current[segment] as Record<string, any>
  })
  current[path[path.length - 1]] = cloneCliConfigValue(value)
}

const deleteValueAtPath = (target: Record<string, any>, path: string[]) => {
  if (path.length === 0) return

  const parents: Array<{ record: Record<string, any>; key: string }> = []
  let current = target

  for (let index = 0; index < path.length - 1; index += 1) {
    const segment = path[index]
    const next = current[segment]
    if (!isPlainObjectRecord(next)) {
      return
    }
    parents.push({ record: current, key: segment })
    current = next
  }

  delete current[path[path.length - 1]]

  for (let index = parents.length - 1; index >= 0; index -= 1) {
    const { record, key } = parents[index]
    const child = record[key]
    if (isPlainObjectRecord(child) && Object.keys(child).length === 0) {
      delete record[key]
      continue
    }
    break
  }
}

const revertTemplateInjectedEntries = (
  currentValue: Record<string, any>,
  entries: TemplateInjectionEntry[],
): Record<string, any> => {
  const nextValue = normalizeCliConfigRecord(currentValue)
  const orderedEntries = [...entries].sort((left, right) => right.path.length - left.path.length)

  orderedEntries.forEach((entry) => {
    if (entry.hadValue) {
      setValueAtPath(nextValue, entry.path, entry.previousValue)
      return
    }
    deleteValueAtPath(nextValue, entry.path)
  })

  return nextValue
}

const applyTemplateToEditableValue = (
  currentValue: Record<string, any>,
  templateValue: Record<string, any>,
) => mergeMissingTemplateKeysWithTracking(currentValue, templateValue)

const resetSharedTemplateState = () => {
  sharedTemplateEnabled.value = false
  sharedTemplateInjectedEntries.value = []
}

const emitChanges = () => {
  emit('update:modelValue', normalizeCliConfigRecord(editableValues.value))
}

const composeEditableValues = (incomingValue: Record<string, any> | undefined | null) => {
  const nextValue = normalizeCliConfigRecord(baseEditableValues.value)
  const incoming = normalizeCliConfigRecord(incomingValue)

  if (Object.keys(incoming).length > 0) {
    return {
      ...nextValue,
      ...incoming,
    }
  }

  return nextValue
}

const editableValuesChangedFromPersistBaseline = computed(() => (
  buildComparableCliConfigText(editableValues.value) !== buildComparableCliConfigText(persistBaselineValue.value)
))

const shouldPersistCliConfig = computed(() => (
  initialModelHasExplicitValue.value
  || sharedTemplateInjectedEntries.value.length > 0
  || editableValuesChangedFromPersistBaseline.value
))

const syncCliJsonFromValues = (forceSyncText = false) => {
  const previousSynced = cliJsonSyncedText.value
  const nextSynced = buildEditorJsonText(editableValues.value)
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

const formatCliConfigJson = (
  input: string,
  target: 'editor' | 'template' = 'editor',
): FormatCliConfigJsonResult => {
  const trimmed = input.trim()
  if (!trimmed) {
    const emptyValue = {}
    return {
      ok: true,
      text: target === 'template' ? buildTemplateJsonText(emptyValue) : buildEditorJsonText(emptyValue),
      value: emptyValue,
    }
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

  const value = stripLockedKeysFromEditable(normalizeCliConfigRecord(parsed as Record<string, any>))
  return {
    ok: true,
    text: target === 'template' ? buildTemplateJsonText(value) : buildEditorJsonText(value),
    value,
  }
}

const applyCliJsonToValues = () => {
  cliJsonError.value = ''
  const formatted = formatCliConfigJson(cliJsonEditingText.value, 'editor')
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
  loadSnapshotsDebounced()
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
  getCliConfigSubmitState: (): CliConfigSubmitState => ({
    value: normalizeCliConfigRecord(editableValues.value),
    shouldPersist: shouldPersistCliConfig.value,
  }),
})

const setSharedTemplateEnabled = (
  nextEnabled: boolean,
  options: SetEditableValuesOptions & { skipPendingApply?: boolean } = {},
) => {
  if (nextEnabled) {
    if (sharedTemplateEnabled.value) return true
    if (!hasSharedTemplate.value || !templateState.value?.template) {
      return false
    }

    if (!(options.skipPendingApply ?? false)) {
      const ready = applyPendingJsonChanges()
      if (!ready) return false
    }

    const baseline = normalizeCliConfigRecord(editableValues.value)
    const merged = applyTemplateToEditableValue(
      baseline,
      normalizeCliConfigRecord(templateState.value.template),
    )
    sharedTemplateEnabled.value = true
    sharedTemplateInjectedEntries.value = merged.injectedEntries
    setEditableValues(
      merged.value,
      {
        emit: options.emit ?? true,
        forceSyncText: options.forceSyncText ?? true,
      },
    )
    return true
  }

  if (!sharedTemplateEnabled.value) return true

  const baseline = revertTemplateInjectedEntries(
    editableValues.value,
    sharedTemplateInjectedEntries.value,
  )
  resetSharedTemplateState()
  setEditableValues(baseline, {
    emit: options.emit ?? true,
    forceSyncText: options.forceSyncText ?? true,
  })
  return true
}

const handleSharedTemplateToggle = (nextEnabled: boolean) => {
  if (nextEnabled && !hasSharedTemplate.value) {
    showToast(t('components.cliConfig.injectTemplateUnavailable'), 'warning')
    return
  }

  const success = setSharedTemplateEnabled(nextEnabled)
  if (!success) {
    return
  }
  loadSnapshotsDebounced()
}

const loadSnapshots = async () => {
  const currentSeq = ++snapshotRequestSeq

  try {
    const previewMode = hasProviderInput.value ? 'direct' : 'current'
    const apiUrl = props.providerConfig?.baseUrl?.trim() || ''
    const apiKey = props.providerConfig?.apiKey?.trim() || ''

    const result = await fetchEditableCLIConfigSnapshots(
      props.platform,
      editableValues.value,
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
  }, 220)
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
  const shouldApplyTemplateWhenEmpty = options.applyTemplateWhenEmpty ?? true

  try {
    const nextConfig = await fetchCLIConfig(props.platform)
    if (currentSeq !== loadConfigRequestSeq) {
      return
    }

    config.value = nextConfig
    baseEditableValues.value = normalizeCliConfigRecord(nextConfig.editable || {})

    try {
      templateState.value = await fetchCLITemplate(props.platform)
    } catch (error) {
      console.error('Failed to load CLI template:', error)
      templateState.value = null
      resetSharedTemplateState()
    }

    const incomingValue = shouldMergeModelValue ? props.modelValue : undefined
    initialModelHasExplicitValue.value = hasConfigKeys(incomingValue)
    const nextEditableBase = composeEditableValues(incomingValue)
    const shouldEnableSharedTemplate = (
      shouldApplyTemplateWhenEmpty
      && !hasConfigKeys(incomingValue)
      && !!templateState.value?.isGlobalDefault
      && hasSharedTemplate.value
    )

    const nextEditableValues = shouldEnableSharedTemplate && templateState.value?.template
      ? applyTemplateToEditableValue(nextEditableBase, normalizeCliConfigRecord(templateState.value.template))
      : null

    sharedTemplateEnabled.value = !!nextEditableValues
    sharedTemplateInjectedEntries.value = nextEditableValues?.injectedEntries || []
    persistBaselineValue.value = normalizeCliConfigRecord(nextEditableBase)

    setEditableValues(nextEditableValues?.value || nextEditableBase, {
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
    templateState.value = null
    resetSharedTemplateState()
    persistBaselineValue.value = {}
    initialModelHasExplicitValue.value = false
    snapshotsData.value = null
    showToast(t('components.cliConfig.loadError'), 'error')
  } finally {
    if (currentSeq === loadConfigRequestSeq) {
      loading.value = false
    }
  }
}

const openGlobalTemplateModal = async () => {
  globalTemplateModalOpen.value = true
  globalTemplateLoading.value = true
  globalTemplateError.value = ''

  try {
    const nextTemplate = await fetchCLITemplate(props.platform)
    templateState.value = nextTemplate

    const nextValue = nextTemplate?.template && Object.keys(nextTemplate.template).length > 0
      ? normalizeCliConfigRecord(nextTemplate.template)
      : normalizeCliConfigRecord(editableValues.value)

    globalTemplateEnabled.value = nextTemplate?.isGlobalDefault ?? false
    globalTemplateSyncedEnabled.value = globalTemplateEnabled.value
    globalTemplateSyncedText.value = buildTemplateJsonText(nextValue)
    globalTemplateEditingText.value = globalTemplateSyncedText.value
  } catch (error) {
    console.error('Failed to load CLI template:', error)
    globalTemplateEnabled.value = false
    globalTemplateSyncedEnabled.value = false
    globalTemplateSyncedText.value = buildTemplateJsonText(editableValues.value)
    globalTemplateEditingText.value = globalTemplateSyncedText.value
  } finally {
    globalTemplateLoading.value = false
  }
}

const closeGlobalTemplateModal = () => {
  globalTemplateModalOpen.value = false
  globalTemplateError.value = ''
}

const saveGlobalTemplate = async () => {
  globalTemplateError.value = ''
  const formatted = formatCliConfigJson(globalTemplateEditingText.value, 'template')
  if (!formatted.ok) {
    globalTemplateError.value = formatted.error
    return
  }

  globalTemplateSaving.value = true
  try {
    await setCLITemplate(props.platform, formatted.value, globalTemplateEnabled.value)
    templateState.value = {
      template: normalizeCliConfigRecord(formatted.value),
      isGlobalDefault: globalTemplateEnabled.value,
    }
    globalTemplateSyncedEnabled.value = globalTemplateEnabled.value
    globalTemplateSyncedText.value = formatted.text
    globalTemplateEditingText.value = formatted.text
    globalTemplateError.value = ''
    showToast(t('components.cliConfig.templateSaved'), 'success')

    if (!hasConfigKeys(props.modelValue)) {
      const nextBase = composeEditableValues(undefined)
      const shouldEnableShared = globalTemplateEnabled.value && Object.keys(formatted.value).length > 0
      const nextEditableValues = shouldEnableShared
        ? applyTemplateToEditableValue(nextBase, normalizeCliConfigRecord(formatted.value))
        : null

      sharedTemplateEnabled.value = !!nextEditableValues
      sharedTemplateInjectedEntries.value = nextEditableValues?.injectedEntries || []

      setEditableValues(
        nextEditableValues?.value || nextBase,
        { emit: true, forceSyncText: true },
      )
      loadSnapshotsDebounced()
    } else if (sharedTemplateEnabled.value) {
      if (Object.keys(formatted.value).length === 0) {
        setSharedTemplateEnabled(false, {
          emit: true,
          forceSyncText: true,
          skipPendingApply: true,
        })
      } else {
        const nextBase = revertTemplateInjectedEntries(
          editableValues.value,
          sharedTemplateInjectedEntries.value,
        )
        const nextEditableValues = applyTemplateToEditableValue(
          nextBase,
          normalizeCliConfigRecord(formatted.value),
        )
        sharedTemplateInjectedEntries.value = nextEditableValues.injectedEntries
        setEditableValues(
          nextEditableValues.value,
          { emit: true, forceSyncText: true },
        )
      }
      loadSnapshotsDebounced()
    }

    globalTemplateModalOpen.value = false
  } catch (error) {
    console.error('Failed to save CLI template:', error)
    showToast(
      `${t('components.cliConfig.templateSaveError')}：${extractErrorMessage(error)}`,
      'error',
    )
  } finally {
    globalTemplateSaving.value = false
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
      applyTemplateWhenEmpty: false,
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
  const nextBase = composeEditableValues(newValue)
  const nextValue = sharedTemplateEnabled.value && templateState.value?.template
    ? mergeMissingTemplateKeys(nextBase, normalizeCliConfigRecord(templateState.value.template))
    : nextBase

  setEditableValues(nextValue, { forceSyncText: true })
}, { immediate: true, deep: true })

watch(cliJsonEditingText, (value) => {
  const formatted = formatCliConfigJson(value, 'editor')
  cliJsonError.value = formatted.ok ? '' : formatted.error
})

watch(globalTemplateEditingText, (value) => {
  const formatted = formatCliConfigJson(value, 'template')
  globalTemplateError.value = formatted.ok ? '' : formatted.error
})

watch(() => props.platform, () => {
  selectedPreviewTab.value = 0
  loadConfig()
})

watch(
  () => [props.providerConfig?.apiKey, props.providerConfig?.baseUrl],
  () => {
    if (config.value) {
      syncCliJsonFromValues(false)
      if (editorUsesEffectiveJson.value && cliJsonDirty.value) {
        const formatted = formatCliConfigJson(cliJsonEditingText.value, 'editor')
        if (formatted.ok) {
          cliJsonEditingText.value = formatted.text
          cliJsonError.value = ''
        }
      }
      loadSnapshotsDebounced()
    }
  },
  { deep: true },
)

watch(
  editableValues,
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
  resetSharedTemplateState()
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
.cli-preview-meta,
.cli-template-actions {
  display: flex;
  align-items: center;
}

.cli-header-left,
.cli-json-header-main {
  gap: 8px;
}

.cli-header-right,
.cli-json-actions,
.cli-template-actions {
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

.cli-section,
.cli-template-modal {
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
.cli-json-hint,
.cli-template-hint {
  font-size: 12px;
  line-height: 1.6;
  color: var(--mac-text-secondary);
}

.cli-template-inject {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--mac-border);
  border-radius: 12px;
  background: color-mix(in srgb, var(--mac-surface-strong) 90%, transparent);
  color: var(--mac-text);
  font-size: 13px;
}

.cli-template-inject input {
  flex-shrink: 0;
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

.cli-template-checkbox {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid var(--mac-border);
  border-radius: 12px;
  background: color-mix(in srgb, var(--mac-surface-strong) 92%, transparent);
  color: var(--mac-text);
  font-size: 13px;
}

.cli-template-checkbox input {
  margin-top: 2px;
}

.cli-template-actions {
  justify-content: flex-end;
  padding-top: 4px;
}

@media (max-width: 720px) {
  .cli-header,
  .cli-content {
    padding-left: 14px;
    padding-right: 14px;
  }

  .cli-header {
    flex-direction: column;
    align-items: stretch;
  }

  .cli-header-right,
  .cli-json-actions,
  .cli-template-actions {
    width: 100%;
    justify-content: flex-end;
  }

  .cli-fields {
    grid-template-columns: 1fr;
  }
}
</style>
