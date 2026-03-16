<template>
  <div class="cli-config-editor">
    <div class="cli-header">
      <div class="cli-header-left">
        <span class="cli-title">{{ t('components.cliConfig.title') }}</span>
        <span class="cli-platform-badge">{{ platformLabel }}</span>
      </div>

      <div class="cli-header-right">
        <label
          class="cli-template-toggle"
          :class="{ 'is-disabled': !hasSharedTemplate && !sharedTemplateEnabled }"
          :title="t('components.cliConfig.injectTemplateHint')"
        >
          <input
            type="checkbox"
            :checked="sharedTemplateEnabled"
            :disabled="!hasSharedTemplate && !sharedTemplateEnabled"
            @change="handleSharedTemplateToggle(($event.target as HTMLInputElement).checked)"
          />
          <span>{{ t('components.cliConfig.injectTemplate') }}</span>
        </label>
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
            :mode="cliEditorMode"
            :show-validation="cliEditorMode === 'json'"
            @format="handleCliEditorFormat"
          />

          <p v-if="cliJsonError" class="cli-json-error" role="alert">
            {{ cliJsonError }}
          </p>
          <p class="cli-json-hint">{{ cliJsonHint }}</p>
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
          :placeholder="templateJsonPlaceholder"
          :mode="templateEditorMode"
          :show-validation="templateEditorMode === 'json'"
          @format="handleGlobalTemplateFormat"
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
import InlineModal from './InlineModal.vue'
import JsonCodeEditor from './JsonCodeEditor.vue'
import {
  fetchCLIConfig,
  fetchCLITemplate,
  normalizeCLIConfigEditorContent,
  normalizeCLITemplateEditorContent,
  renderCLIConfigEditorContent,
  renderCLITemplateEditorContent,
  restoreDefaultConfig,
  setCLITemplate,
  type CLIConfig,
  type CLIConfigField,
  type CLIEditorContent,
  type CLINormalizedEditorContent,
  type CLITemplate,
  type CLIPlatform,
} from '../../services/cliConfig'
import { showToast } from '../../utils/toast'
import { extractErrorMessage } from '../../utils/error'

const CLI_CONFIG_FULL_SOURCE_MARKER = '__code_switch_cli_full__'

const props = defineProps<{
  platform: CLIPlatform
  modelValue?: Record<string, any>
  providerName?: string
  providerConfig?: {
    apiKey?: string
    baseUrl?: string
  }
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, any>): void
}>()

const { t } = useI18n()

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
  persistValue: Record<string, any>
  shouldPersist: boolean
}

const loading = ref(false)
const config = ref<CLIConfig | null>(null)
const baseEditableValues = ref<Record<string, any>>({})
const editableValues = ref<Record<string, any>>({})
const editorLockedFields = ref<CLIConfigField[]>([])
const cliJsonEditorRef = ref<InstanceType<typeof JsonCodeEditor> | null>(null)
const cliJsonSyncedText = ref('')
const cliJsonEditingText = ref('')
const cliJsonError = ref('')
const cliEditorFormat = ref<'json' | 'toml' | 'env' | string>('json')
const templateState = ref<CLITemplate | null>(null)
const globalTemplateModalOpen = ref(false)
const globalTemplateLoading = ref(false)
const globalTemplateSaving = ref(false)
const globalTemplateEnabled = ref(false)
const globalTemplateSyncedText = ref('')
const globalTemplateSyncedEnabled = ref(false)
const globalTemplateEditingText = ref('')
const globalTemplateError = ref('')
const globalTemplateFormat = ref<'json' | 'toml' | 'env' | string>('json')
const sharedTemplateEnabled = ref(false)
const sharedTemplateInjectedEntries = ref<TemplateInjectionEntry[]>([])
const persistBaselineValue = ref<Record<string, any>>({})
const initialModelHasExplicitValue = ref(false)

let loadConfigRequestSeq = 0
let cliEditorRenderRequestSeq = 0
let cliEditorValidateRequestSeq = 0
let cliEditorValidationTimer: ReturnType<typeof setTimeout> | null = null
let globalTemplateFormatRequestSeq = 0
let globalTemplateValidateRequestSeq = 0
let globalTemplateValidationTimer: ReturnType<typeof setTimeout> | null = null

const cloneCliConfigValue = <T>(value: T): T => {
  if (value == null) return value
  return JSON.parse(JSON.stringify(value))
}

const isCliConfigFullSource = (value: Record<string, any> | undefined | null) => (
  !!value
  && typeof value === 'object'
  && !Array.isArray(value)
  && (value as Record<string, any>)[CLI_CONFIG_FULL_SOURCE_MARKER] === true
)

const normalizeCliConfigRecord = (value: Record<string, any> | undefined | null): Record<string, any> => {
  const nextValue = cloneCliConfigValue(value || {})
  if (nextValue && typeof nextValue === 'object' && !Array.isArray(nextValue)) {
    delete (nextValue as Record<string, any>)[CLI_CONFIG_FULL_SOURCE_MARKER]
  }
  return nextValue
}

const mergeCliConfigRecords = (
  baseValue: Record<string, any>,
  overrideValue: Record<string, any>,
): Record<string, any> => {
  const nextValue = normalizeCliConfigRecord(baseValue)

  Object.entries(overrideValue).forEach(([key, value]) => {
    if (isPlainObjectRecord(nextValue[key]) && isPlainObjectRecord(value)) {
      nextValue[key] = mergeCliConfigRecords(
        nextValue[key] as Record<string, any>,
        value,
      )
      return
    }

    nextValue[key] = cloneCliConfigValue(value)
  })

  return nextValue
}

const attachCliConfigMetadata = (value: Record<string, any> | undefined | null): Record<string, any> => {
  const nextValue = normalizeCliConfigRecord(value)
  if (Object.keys(nextValue).length === 0) {
    return {}
  }
  return {
    ...nextValue,
    [CLI_CONFIG_FULL_SOURCE_MARKER]: true,
  }
}

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

const buildComparableCliConfigText = (value: Record<string, any> | undefined | null) =>
  JSON.stringify(toSortedJsonValue(normalizeCliConfigRecord(value)))

const platformLabels: Record<CLIPlatform, string> = {
  claude: 'Claude Code',
  codex: 'Codex',
  gemini: 'Gemini',
}

const platformLabel = computed(() => platformLabels[props.platform] || props.platform)
const providerName = computed(() => props.providerName?.trim() || '')
const providerApiKey = computed(() => props.providerConfig?.apiKey?.trim() || '')
const providerApiUrl = computed(() => props.providerConfig?.baseUrl?.trim() || '')
const cliEditorMode = computed<'json' | 'plain'>(() => (
  cliEditorFormat.value === 'json' ? 'json' : 'plain'
))
const templateEditorMode = computed<'json' | 'plain'>(() => (
  globalTemplateFormat.value === 'json' ? 'json' : 'plain'
))
const lockedFields = computed(() => editorLockedFields.value)
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
  },
  "model": "claude-sonnet-4-5"
}`
  }

  if (props.platform === 'codex') {
    return `model = "gpt-5-codex"
disable_response_storage = true
model_reasoning_effort = "xhigh"

[features]
parallel = true`
  }

  return `GOOGLE_GEMINI_BASE_URL=https://api.example.com
GEMINI_API_KEY=your-api-key
GEMINI_MODEL=gemini-2.5-pro`
})

const templateJsonPlaceholder = computed(() => {
  if (props.platform === 'claude') {
    return `{
  "model": "claude-sonnet-4-5",
  "env": {
    "ANTHROPIC_CUSTOM_HEADER": "value"
  }
}`
  }

  if (props.platform === 'codex') {
    return `model = "gpt-5-codex"
disable_response_storage = true
model_reasoning_effort = "high"

[features]
parallel = true`
  }

  return `{
  "GEMINI_MODEL": "gemini-2.5-pro",
  "HTTP_PROXY": "http://127.0.0.1:7890"
}`
})

const cliJsonHint = computed(() => t('components.cliConfig.jsonEditor.fileHint'))

const focusCliJsonEditor = () => {
  requestAnimationFrame(() => {
    cliJsonEditorRef.value?.focus()
  })
}

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

const clearCliEditorValidationTimer = () => {
  if (cliEditorValidationTimer) {
    clearTimeout(cliEditorValidationTimer)
    cliEditorValidationTimer = null
  }
}

const clearGlobalTemplateValidationTimer = () => {
  if (globalTemplateValidationTimer) {
    clearTimeout(globalTemplateValidationTimer)
    globalTemplateValidationTimer = null
  }
}

const emitChanges = () => {
  emit('update:modelValue', attachCliConfigMetadata(editableValues.value))
}

const composeEditableValues = (incomingValue: Record<string, any> | undefined | null) => {
  const nextValue = normalizeCliConfigRecord(baseEditableValues.value)
  const incoming = normalizeCliConfigRecord(incomingValue)

  if (Object.keys(incoming).length === 0) {
    return nextValue
  }

  if (isCliConfigFullSource(incomingValue)) {
    return incoming
  }

  return mergeCliConfigRecords(nextValue, incoming)
}

const editableValuesChangedFromPersistBaseline = computed(() => (
  buildComparableCliConfigText(editableValues.value) !== buildComparableCliConfigText(persistBaselineValue.value)
))

const shouldPersistCliConfig = computed(() => (
  initialModelHasExplicitValue.value
  || sharedTemplateInjectedEntries.value.length > 0
  || editableValuesChangedFromPersistBaseline.value
))

const applyRenderedCliEditorContent = (rendered: CLIEditorContent, forceSyncText = false) => {
  const previousSynced = cliJsonSyncedText.value
  const nextSynced = rendered.content || ''
  cliEditorFormat.value = rendered.format || 'json'
  editorLockedFields.value = (rendered.lockedFields || []).map((field) => ({ ...field }))
  cliJsonSyncedText.value = nextSynced

  const editingWasSynced = cliJsonEditingText.value === previousSynced
  if (forceSyncText || editingWasSynced || !cliJsonEditingText.value) {
    cliJsonEditingText.value = nextSynced
  }

  if (forceSyncText) {
    cliJsonError.value = ''
  }
}

const renderCliEditorContent = async (value: Record<string, any>) => (
  renderCLIConfigEditorContent(
    props.platform,
    normalizeCliConfigRecord(value),
    providerApiUrl.value,
    providerApiKey.value,
    providerName.value,
  )
)

const normalizeCliEditorContent = async (content: string) => (
  normalizeCLIConfigEditorContent(
    props.platform,
    content,
    providerApiUrl.value,
    providerApiKey.value,
    providerName.value,
  )
)

const renderTemplateEditorContent = async (value: Record<string, any>) => (
  renderCLITemplateEditorContent(
    props.platform,
    normalizeCliConfigRecord(value),
  )
)

const normalizeTemplateEditorContent = async (content: string): Promise<CLINormalizedEditorContent> => (
  normalizeCLITemplateEditorContent(props.platform, content)
)

const syncCliJsonFromValues = async (forceSyncText = false) => {
  const currentSeq = ++cliEditorRenderRequestSeq

  try {
    const rendered = await renderCliEditorContent(editableValues.value)
    if (currentSeq !== cliEditorRenderRequestSeq) {
      return
    }

    applyRenderedCliEditorContent(rendered, forceSyncText)
  } catch (error) {
    if (currentSeq !== cliEditorRenderRequestSeq) {
      return
    }
    console.error('Failed to render CLI editor content:', error)
    if (forceSyncText) {
      cliJsonError.value = extractErrorMessage(error)
    }
  }
}

const setEditableValues = async (
  value: Record<string, any> | undefined | null,
  options: SetEditableValuesOptions = {},
) => {
  editableValues.value = normalizeCliConfigRecord(value)
  if (options.emit) {
    emitChanges()
  }
  await syncCliJsonFromValues(options.forceSyncText ?? false)
}

const applyNormalizedCliEditor = (
  normalized: CLINormalizedEditorContent,
  options: SetEditableValuesOptions = {},
) => {
  editableValues.value = normalizeCliConfigRecord(normalized.editable)
  if (options.emit) {
    emitChanges()
  }
  applyRenderedCliEditorContent(normalized, options.forceSyncText ?? true)
}

const applyCliJsonToValues = async () => {
  cliJsonError.value = ''

  try {
    const normalized = await normalizeCliEditorContent(cliJsonEditingText.value)
    applyNormalizedCliEditor(normalized, {
      emit: true,
      forceSyncText: true,
    })
    cliJsonError.value = ''
    return true
  } catch (error) {
    cliJsonError.value = extractErrorMessage(error)
    focusCliJsonEditor()
    return false
  }
}

const handleCliEditorFormat = async () => {
  try {
    const normalized = await normalizeCliEditorContent(cliJsonEditingText.value)
    cliEditorFormat.value = normalized.format || cliEditorFormat.value
    editorLockedFields.value = (normalized.lockedFields || []).map((field) => ({ ...field }))
    cliJsonEditingText.value = normalized.content
    cliJsonError.value = ''
  } catch (error) {
    cliJsonError.value = extractErrorMessage(error)
    focusCliJsonEditor()
  }
}

const handleGlobalTemplateFormat = () => {
  void (async () => {
    const currentSeq = ++globalTemplateFormatRequestSeq
    const currentText = globalTemplateEditingText.value

    try {
      const normalized = await normalizeTemplateEditorContent(currentText)
      if (currentSeq !== globalTemplateFormatRequestSeq || globalTemplateEditingText.value !== currentText) {
        return
      }
      globalTemplateFormat.value = normalized.format || globalTemplateFormat.value
      globalTemplateEditingText.value = normalized.content
      globalTemplateError.value = ''
    } catch (error) {
      if (currentSeq !== globalTemplateFormatRequestSeq || globalTemplateEditingText.value !== currentText) {
        return
      }
      globalTemplateError.value = extractErrorMessage(error)
    }
  })()
}

const applyRenderedGlobalTemplateContent = (rendered: CLIEditorContent) => {
  globalTemplateFormat.value = rendered.format || 'json'
  globalTemplateSyncedText.value = rendered.content || ''
  globalTemplateEditingText.value = globalTemplateSyncedText.value
}

const normalizeGlobalTemplateDraft = async () => {
  const normalized = await normalizeTemplateEditorContent(globalTemplateEditingText.value)
  globalTemplateFormat.value = normalized.format || globalTemplateFormat.value
  return {
    value: normalizeCliConfigRecord(normalized.editable),
    text: normalized.content,
  }
}

const resetCliJsonFromValues = () => {
  cliJsonError.value = ''
  cliJsonEditingText.value = cliJsonSyncedText.value
}

const applyPendingJsonChanges = async () => {
  if (!cliJsonDirty.value) return true
  return applyCliJsonToValues()
}

defineExpose({
  applyPendingJsonChanges,
  getCliConfigSubmitState: (): CliConfigSubmitState => ({
    value: attachCliConfigMetadata(editableValues.value),
    persistValue: normalizeCliConfigRecord(editableValues.value),
    shouldPersist: shouldPersistCliConfig.value,
  }),
})

const setSharedTemplateEnabled = async (
  nextEnabled: boolean,
  options: SetEditableValuesOptions & { skipPendingApply?: boolean } = {},
) => {
  if (nextEnabled) {
    if (sharedTemplateEnabled.value) return true
    if (!hasSharedTemplate.value || !templateState.value?.template) {
      return false
    }

    if (!(options.skipPendingApply ?? false)) {
      const ready = await applyPendingJsonChanges()
      if (!ready) return false
    }

    const baseline = normalizeCliConfigRecord(editableValues.value)
    const merged = applyTemplateToEditableValue(
      baseline,
      normalizeCliConfigRecord(templateState.value.template),
    )
    sharedTemplateEnabled.value = true
    sharedTemplateInjectedEntries.value = merged.injectedEntries
    await setEditableValues(
      merged.value,
      {
        emit: options.emit ?? true,
        forceSyncText: options.forceSyncText ?? true,
      },
    )
    return true
  }

  if (!sharedTemplateEnabled.value) return true

  if (!(options.skipPendingApply ?? false)) {
    const ready = await applyPendingJsonChanges()
    if (!ready) return false
  }

  const baseline = revertTemplateInjectedEntries(
    editableValues.value,
    sharedTemplateInjectedEntries.value,
  )
  resetSharedTemplateState()
  await setEditableValues(baseline, {
    emit: options.emit ?? true,
    forceSyncText: options.forceSyncText ?? true,
  })
  return true
}

const handleSharedTemplateToggle = async (nextEnabled: boolean) => {
  if (nextEnabled && !hasSharedTemplate.value) {
    showToast(t('components.cliConfig.injectTemplateUnavailable'), 'warning')
    return
  }

  const success = await setSharedTemplateEnabled(nextEnabled)
  if (!success) {
    return
  }
}

const scheduleCliEditorValidation = () => {
  clearCliEditorValidationTimer()

  if (!cliJsonEditingText.value.trim() || cliJsonEditingText.value === cliJsonSyncedText.value) {
    cliJsonError.value = ''
    return
  }

  const currentSeq = ++cliEditorValidateRequestSeq
  const currentText = cliJsonEditingText.value

  cliEditorValidationTimer = setTimeout(async () => {
    try {
      await normalizeCliEditorContent(currentText)
      if (currentSeq !== cliEditorValidateRequestSeq || cliJsonEditingText.value !== currentText) {
        return
      }
      cliJsonError.value = ''
    } catch (error) {
      if (currentSeq !== cliEditorValidateRequestSeq || cliJsonEditingText.value !== currentText) {
        return
      }
      cliJsonError.value = extractErrorMessage(error)
    }
  }, 180)
}

const scheduleGlobalTemplateValidation = () => {
  clearGlobalTemplateValidationTimer()

  if (!globalTemplateModalOpen.value) {
    globalTemplateError.value = ''
    return
  }

  if (!globalTemplateEditingText.value.trim() || globalTemplateEditingText.value === globalTemplateSyncedText.value) {
    globalTemplateError.value = ''
    return
  }

  const currentSeq = ++globalTemplateValidateRequestSeq
  const currentText = globalTemplateEditingText.value

  globalTemplateValidationTimer = setTimeout(async () => {
    try {
      await normalizeTemplateEditorContent(currentText)
      if (currentSeq !== globalTemplateValidateRequestSeq || globalTemplateEditingText.value !== currentText) {
        return
      }
      globalTemplateError.value = ''
    } catch (error) {
      if (currentSeq !== globalTemplateValidateRequestSeq || globalTemplateEditingText.value !== currentText) {
        return
      }
      globalTemplateError.value = extractErrorMessage(error)
    }
  }, 180)
}

const loadConfig = async () => {
  await loadConfigWithOptions()
}

const loadConfigWithOptions = async (options: LoadConfigOptions = {}) => {
  const currentSeq = ++loadConfigRequestSeq
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

    await setEditableValues(nextEditableValues?.value || nextEditableBase, {
      emit: shouldEmitChanges,
      forceSyncText: shouldForceSyncText,
    })
  } catch (error) {
    if (currentSeq !== loadConfigRequestSeq) {
      return
    }

    console.error('Failed to load CLI config:', error)
    config.value = null
    editorLockedFields.value = []
    templateState.value = null
    resetSharedTemplateState()
    persistBaselineValue.value = {}
    initialModelHasExplicitValue.value = false
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
    applyRenderedGlobalTemplateContent(await renderTemplateEditorContent(nextValue))
  } catch (error) {
    console.error('Failed to load CLI template:', error)
    globalTemplateEnabled.value = false
    globalTemplateSyncedEnabled.value = false
    applyRenderedGlobalTemplateContent(await renderTemplateEditorContent(editableValues.value))
  } finally {
    globalTemplateLoading.value = false
  }
}

const closeGlobalTemplateModal = () => {
  clearGlobalTemplateValidationTimer()
  globalTemplateFormatRequestSeq += 1
  globalTemplateValidateRequestSeq += 1
  globalTemplateModalOpen.value = false
  globalTemplateError.value = ''
}

const saveGlobalTemplate = async () => {
  globalTemplateError.value = ''
  let normalizedTemplate: { value: Record<string, any>; text: string }
  try {
    normalizedTemplate = await normalizeGlobalTemplateDraft()
  } catch (error) {
    globalTemplateError.value = extractErrorMessage(error)
    return
  }

  if ((!shouldPersistCliConfig.value || sharedTemplateEnabled.value) && !await applyPendingJsonChanges()) {
    return
  }

  globalTemplateSaving.value = true
  try {
    await setCLITemplate(props.platform, normalizedTemplate.value, globalTemplateEnabled.value)
    templateState.value = {
      template: normalizeCliConfigRecord(normalizedTemplate.value),
      isGlobalDefault: globalTemplateEnabled.value,
    }
    globalTemplateSyncedEnabled.value = globalTemplateEnabled.value
    globalTemplateSyncedText.value = normalizedTemplate.text
    globalTemplateEditingText.value = normalizedTemplate.text
    globalTemplateError.value = ''
    showToast(t('components.cliConfig.templateSaved'), 'success')

    if (!shouldPersistCliConfig.value) {
      const nextBase = composeEditableValues(undefined)
      const shouldEnableShared = globalTemplateEnabled.value && Object.keys(normalizedTemplate.value).length > 0
      const nextEditableValues = shouldEnableShared
        ? applyTemplateToEditableValue(nextBase, normalizeCliConfigRecord(normalizedTemplate.value))
        : null

      sharedTemplateEnabled.value = !!nextEditableValues
      sharedTemplateInjectedEntries.value = nextEditableValues?.injectedEntries || []

      await setEditableValues(
        nextEditableValues?.value || nextBase,
        { emit: true, forceSyncText: true },
      )
    } else if (sharedTemplateEnabled.value) {
      if (Object.keys(normalizedTemplate.value).length === 0) {
        await setSharedTemplateEnabled(false, {
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
          normalizeCliConfigRecord(normalizedTemplate.value),
        )
        sharedTemplateInjectedEntries.value = nextEditableValues.injectedEntries
        await setEditableValues(
          nextEditableValues.value,
          { emit: true, forceSyncText: true },
        )
      }
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

watch(() => props.modelValue, async (newValue) => {
  const nextBase = composeEditableValues(newValue)
  const nextValue = sharedTemplateEnabled.value && templateState.value?.template
    ? mergeMissingTemplateKeys(nextBase, normalizeCliConfigRecord(templateState.value.template))
    : nextBase

  await setEditableValues(nextValue, { forceSyncText: true })
}, { immediate: true, deep: true })

watch(cliJsonEditingText, () => {
  scheduleCliEditorValidation()
})

watch(globalTemplateEditingText, () => {
  scheduleGlobalTemplateValidation()
})

watch(() => props.platform, () => {
  void loadConfig()
})

watch(
  () => [props.providerName, props.providerConfig?.apiKey, props.providerConfig?.baseUrl],
  async () => {
    if (!config.value) {
      return
    }

    await syncCliJsonFromValues(false)
    scheduleCliEditorValidation()
  },
  { deep: true },
)

onMounted(() => {
  void loadConfig()
})

onUnmounted(() => {
  clearCliEditorValidationTimer()
  clearGlobalTemplateValidationTimer()
  loadConfigRequestSeq += 1
  cliEditorRenderRequestSeq += 1
  cliEditorValidateRequestSeq += 1
  globalTemplateFormatRequestSeq += 1
  globalTemplateValidateRequestSeq += 1
  editorLockedFields.value = []
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

.cli-template-toggle {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 32px;
  padding: 0 12px;
  border: 1px solid var(--mac-border);
  border-radius: 10px;
  background: color-mix(in srgb, var(--mac-surface) 94%, transparent);
  color: var(--mac-text);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.cli-template-toggle:hover {
  border-color: color-mix(in srgb, var(--mac-accent) 45%, var(--mac-border));
  color: var(--mac-accent);
}

.cli-template-toggle.is-disabled {
  opacity: 0.55;
}

.cli-template-toggle input {
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

  .cli-template-toggle {
    width: 100%;
    justify-content: center;
  }

  .cli-fields {
    grid-template-columns: 1fr;
  }
}
</style>
