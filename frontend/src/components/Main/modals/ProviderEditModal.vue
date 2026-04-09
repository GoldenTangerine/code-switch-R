<template>
  <BaseModal
    :open="open"
    :title="modalTitle"
    :body-scrollable="false"
    :panel-width="'min(980px, 96vw)'"
    @close="$emit('close')"
  >
    <form class="vendor-form vendor-form--provider-modal" @submit.prevent="submit()">
      <div class="vendor-form__scroll-body">
        <label class="form-field">
          <span>{{ t('components.main.form.labels.name') }}</span>
          <BaseInput
            v-model="form.name"
            type="text"
            :placeholder="t('components.main.form.placeholders.name')"
            required
          />
        </label>

        <label class="form-field">
          <span class="label-row">
            {{ t('components.main.form.labels.apiUrl') }}
            <span v-if="errors.apiUrl" class="field-error">
              {{ errors.apiUrl }}
            </span>
          </span>
          <BaseInput
            v-model="form.apiUrl"
            type="text"
            :placeholder="t('components.main.form.placeholders.apiUrl')"
            required
            :class="{ 'has-error': !!errors.apiUrl }"
          />
        </label>

        <label class="form-field">
          <span>{{ t('components.main.form.labels.officialSite') }}</span>
          <BaseInput
            v-model="form.officialSite"
            type="text"
            :placeholder="t('components.main.form.placeholders.officialSite')"
          />
        </label>

        <label class="form-field">
          <span>{{ t('components.main.form.labels.apiKey') }}</span>
          <BaseInput
            v-model="form.apiKey"
            type="text"
            :placeholder="t('components.main.form.placeholders.apiKey')"
          />
        </label>

        <label class="form-field">
          <span>{{ t('components.main.form.labels.apiEndpoint') }}</span>
          <BaseInput
            v-model="form.apiEndpoint"
            type="text"
            :placeholder="t('components.main.form.placeholders.apiEndpoint')"
          />
          <span class="field-hint">{{ t('components.main.form.hints.apiEndpoint') }}</span>
        </label>

        <div class="form-field">
          <span>{{ t('components.main.form.labels.connectivityAuthType') }}</span>
          <Listbox v-model="selectedAuthType" v-slot="{ open: authTypeOpen }">
            <div class="level-select">
              <ListboxButton class="level-select-button">
                <span class="level-label">
                  {{ authTypeLabel }}
                </span>
                <svg viewBox="0 0 20 20" aria-hidden="true">
                  <path d="M6 8l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none" />
                </svg>
              </ListboxButton>
              <ListboxOptions v-if="authTypeOpen" class="level-select-options">
                <ListboxOption
                  v-for="option in authTypeOptions"
                  :key="option.value"
                  :value="option.value"
                  v-slot="{ active, selected }"
                >
                  <div :class="['level-option', { active, selected }]">
                    <span class="level-name">{{ option.label }}</span>
                  </div>
                </ListboxOption>
              </ListboxOptions>
            </div>
          </Listbox>
          <BaseInput
            v-model="customAuthHeader"
            type="text"
            :placeholder="t('components.main.form.placeholders.customAuthHeader')"
            class="mt-2"
          />
          <span class="field-hint">{{ t('components.main.form.hints.connectivityAuthType') }}</span>
        </div>

        <div class="form-field">
          <span>{{ t('components.main.form.labels.icon') }}</span>
          <Listbox v-model="form.icon" v-slot="{ open: iconSelectOpen }" class="w-full">
            <div class="icon-select">
              <ListboxButton class="icon-select-button">
                <span class="icon-preview" v-html="iconSvg(form.icon)" aria-hidden="true" @mouseenter="warmupIcon(form.icon)"></span>
                <span class="icon-select-label">{{ form.icon }}</span>
                <svg viewBox="0 0 20 20" aria-hidden="true">
                  <path d="M6 8l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none" />
                </svg>
              </ListboxButton>
              <ListboxOptions v-if="iconSelectOpen" class="icon-select-options">
                <div class="icon-search-wrapper">
                  <input
                    v-model="iconSearchQuery"
                    type="text"
                    class="icon-search-input"
                    :placeholder="t('components.main.form.placeholders.searchIcon')"
                    @click.stop
                    @keydown.stop
                  />
                </div>
                <ListboxOption
                  v-for="iconName in filteredIconOptions"
                  :key="iconName"
                  :value="iconName"
                  v-slot="{ active, selected }"
                >
                  <div :class="['icon-option', { active, selected }]" @mouseenter="warmupIcon(iconName)">
                    <span class="icon-preview" v-html="iconSvg(iconName)" aria-hidden="true"></span>
                    <span class="icon-name">{{ iconName }}</span>
                  </div>
                </ListboxOption>
                <div v-if="filteredIconOptions.length === 0" class="icon-no-results">
                  {{ t('components.main.form.noIconResults') }}
                </div>
              </ListboxOptions>
            </div>
          </Listbox>
        </div>

        <div class="form-field">
          <span>{{ t('components.main.form.labels.level') }}</span>
          <Listbox v-model="form.level" v-slot="{ open: levelOpen }">
            <div class="level-select">
              <ListboxButton class="level-select-button">
                <span class="level-badge" :class="`level-${form.level || 1}`">
                  L{{ form.level || 1 }}
                </span>
                <span class="level-label">
                  Level {{ form.level || 1 }} - {{ getLevelDescription(form.level || 1) }}
                </span>
                <svg viewBox="0 0 20 20" aria-hidden="true">
                  <path d="M6 8l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none" />
                </svg>
              </ListboxButton>
              <ListboxOptions v-if="levelOpen" class="level-select-options">
                <ListboxOption
                  v-for="level in 10"
                  :key="level"
                  :value="level"
                  v-slot="{ active, selected }"
                >
                  <div :class="['level-option', { active, selected }]">
                    <span class="level-badge" :class="`level-${level}`">L{{ level }}</span>
                    <span class="level-name">Level {{ level }} - {{ getLevelDescription(level) }}</span>
                  </div>
                </ListboxOption>
              </ListboxOptions>
            </div>
          </Listbox>
          <span class="field-hint">{{ t('components.main.form.hints.level') }}</span>
        </div>

        <div class="form-field">
          <span>{{ t('components.main.form.labels.budgetQuota') }}</span>
          <div v-if="form.budgetQuotaSettings" class="quota-settings-grid">
            <div
              v-for="def in quotaDefinitions"
              :key="def.key"
              class="quota-setting-row"
            >
              <span class="quota-setting-label">{{ def.label }}</span>
              <div class="quota-setting-inputs">
                <div class="quota-total-input">
                  <input
                    v-model.number="form.budgetQuotaSettings[def.key].total"
                    type="number"
                    step="0.01"
                    min="0"
                    class="quota-number-input"
                    :placeholder="t('components.main.form.placeholders.budgetQuotaTotal')"
                  />
                  <span class="quota-unit">USD</span>
                </div>
                <template v-if="def.hasRefreshTime">
                  <input
                    v-model="form.budgetQuotaSettings[def.key].refreshTime"
                    type="time"
                    class="quota-time-input"
                  />
                </template>
                <template v-if="def.key === 'weekly'">
                  <select
                    v-model.number="form.budgetQuotaSettings[def.key].refreshWeekday"
                    class="quota-select-input"
                  >
                    <option v-for="d in weekdayOptions" :key="d.value" :value="d.value">
                      {{ d.label }}
                    </option>
                  </select>
                </template>
                <template v-if="def.key === 'monthly'">
                  <input
                    v-model.number="form.budgetQuotaSettings[def.key].refreshMonthDay"
                    type="number"
                    min="1"
                    max="31"
                    class="quota-day-input"
                    :placeholder="'1'"
                  />
                </template>
              </div>
            </div>
          </div>
          <span class="field-hint">{{ t('components.main.form.hints.budgetQuota') }}</span>
        </div>

        <div class="form-field">
          <ModelWhitelistEditor v-model="form.supportedModels" />
        </div>

        <div class="form-field">
          <ModelMappingEditor
            :key="cliConfigEditorKey"
            v-model="form.modelMapping"
            :platform="builtinModelPlatform"
          />
        </div>

        <div class="form-field">
          <span class="label-row">
            {{ t('components.main.form.labels.requestBodyOverrides') }}
            <span v-if="requestBodyOverridesError" class="field-error">
              {{ requestBodyOverridesError }}
            </span>
          </span>
          <JsonCodeEditor
            v-model="requestBodyOverridesText"
            :invalid="!!requestBodyOverridesError"
            :rows="10"
            :surface-height="'220px'"
          />
          <span class="field-hint">{{ t('components.main.form.hints.requestBodyOverrides') }}</span>
        </div>

        <div class="form-field">
          <CLIConfigEditor
            :key="cliConfigEditorKey"
            ref="cliConfigEditorRef"
            :platform="tabId as CLIPlatform"
            v-model="form.cliConfig"
            :provider-name="form.name"
            :provider-config="{
              apiKey: form.apiKey,
              baseUrl: form.apiUrl,
            }"
          />
        </div>

        <div class="form-field switch-field">
          <span>{{ t('components.main.form.labels.enabled') }}</span>
          <div class="switch-inline">
            <label class="mac-switch">
              <input type="checkbox" v-model="form.enabled" />
              <span></span>
            </label>
            <span class="switch-text">
              {{ form.enabled ? t('components.main.form.switch.on') : t('components.main.form.switch.off') }}
            </span>
          </div>
        </div>

        <div class="form-field switch-field">
          <span>{{ t('components.main.form.labels.availabilityMonitor') }}</span>
          <div class="switch-inline">
            <label class="mac-switch">
              <input type="checkbox" v-model="form.availabilityMonitorEnabled" />
              <span></span>
            </label>
            <span class="switch-text">
              {{ form.availabilityMonitorEnabled ? t('components.main.form.switch.on') : t('components.main.form.switch.off') }}
            </span>
          </div>
          <span class="field-hint">{{ t('components.main.form.hints.availabilityMonitor') }}</span>
        </div>

        <div v-if="form.availabilityMonitorEnabled" class="form-field switch-field">
          <span>{{ t('components.main.form.labels.connectivityAutoBlacklist') }}</span>
          <div class="switch-inline">
            <label class="mac-switch">
              <input type="checkbox" v-model="form.connectivityAutoBlacklist" />
              <span></span>
            </label>
            <span class="switch-text">
              {{ form.connectivityAutoBlacklist ? t('components.main.form.switch.on') : t('components.main.form.switch.off') }}
            </span>
          </div>
          <span class="field-hint">{{ t('components.main.form.hints.connectivityAutoBlacklist') }}</span>
        </div>

        <div v-if="form.availabilityMonitorEnabled" class="form-field">
          <span class="field-hint" style="color: #6b7280;">
            💡 {{ t('components.main.form.hints.availabilityAdvancedConfig') }}
          </span>
        </div>
      </div>

      <footer class="form-actions form-actions--provider-modal">
        <BaseButton variant="outline" type="button" @click="$emit('close')">
          {{ t('components.main.form.actions.cancel') }}
        </BaseButton>
        <BaseButton type="submit">
          {{ t('components.main.form.actions.save') }}
        </BaseButton>
        <BaseButton
          v-if="isEditing && tabId !== 'others' && !activeProxyState"
          type="button"
          variant="primary"
          @click="submit(true)"
        >
          {{ t('components.main.form.actions.saveAndApply') }}
        </BaseButton>
      </footer>
    </form>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Listbox, ListboxButton, ListboxOption, ListboxOptions } from '@headlessui/vue'
import lobeIcons, { preloadLobeIcons } from '../../../icons/lobeIconMap'
import BaseButton from '../../common/BaseButton.vue'
import BaseInput from '../../common/BaseInput.vue'
import BaseModal from '../../common/BaseModal.vue'
import CLIConfigEditor from '../../common/CLIConfigEditor.vue'
import JsonCodeEditor from '../../common/JsonCodeEditor.vue'
import ModelMappingEditor from '../../common/ModelMappingEditor.vue'
import ModelWhitelistEditor from '../../common/ModelWhitelistEditor.vue'
import { AUTH_TYPE_OPTIONS, getDefaultAuthType } from '../constants'
import {
  buildNormalizedVendorForm,
  createDefaultVendorForm,
  createVendorFormFromCard,
  resolveProviderAuthState,
} from '../adapters/providerFormMappers'
import type { ProviderTab, VendorForm } from '../types'
import type { BudgetQuotaKey } from '../../../utils/budgetUsage'
import {
  cloneBudgetQuotaSettings,
  createDefaultBudgetQuotaSettings,
  hasConfiguredBudgetQuotaSettings,
  normalizeBudgetQuotaSettings,
} from '../../../utils/budgetUsage'
import type { AutomationCard } from '../../../data/cards'
import type { CLIPlatform } from '../../../services/cliConfig'
import { isBuiltinModelPlatform } from '../../../utils/builtinModels'

type CLIConfigEditorExposed = InstanceType<typeof CLIConfigEditor> & {
  applyPendingJsonChanges?: () => boolean | Promise<boolean>
  getCliConfigSubmitState?: () => {
    value: Record<string, any>
    persistValue: Record<string, any>
    shouldPersist: boolean
  }
}

const props = defineProps<{
  open: boolean
  tabId: ProviderTab
  card: AutomationCard | null
  activeProxyState: boolean
}>()

const emit = defineEmits<{
  close: []
  submit: [form: VendorForm]
  'submit-and-apply': [form: VendorForm]
}>()

const { t } = useI18n()

const iconOptions = Object.keys(lobeIcons).sort((left, right) => left.localeCompare(right))
const ICON_PRELOAD_BATCH_SIZE = 80
const defaultIconKey = iconOptions[0] ?? 'aicoding'

const form = reactive<VendorForm>(createDefaultVendorForm(props.tabId, defaultIconKey))
const cliConfigEditorRef = ref<CLIConfigEditorExposed | null>(null)
const cliConfigEditorKey = ref(0)
const errors = reactive({
  apiUrl: '',
})
const selectedAuthType = ref<string>(getDefaultAuthType(props.tabId))
const customAuthHeader = ref('')
const iconSearchQuery = ref('')
const requestBodyOverridesText = ref('{}')
const requestBodyOverridesError = ref('')

type QuotaDefinition = {
  key: BudgetQuotaKey
  label: string
  hasRefreshTime: boolean
}

const quotaDefinitions = computed<QuotaDefinition[]>(() => [
  { key: 'five_hour', label: t('components.main.form.labels.budgetQuotaFiveHour'), hasRefreshTime: false },
  { key: 'daily', label: t('components.main.form.labels.budgetQuotaDaily'), hasRefreshTime: true },
  { key: 'weekly', label: t('components.main.form.labels.budgetQuotaWeekly'), hasRefreshTime: true },
  { key: 'monthly', label: t('components.main.form.labels.budgetQuotaMonthly'), hasRefreshTime: true },
])

const weekdayOptions = computed(() => [
  { value: 1, label: t('components.general.label.weekdayMon') },
  { value: 2, label: t('components.general.label.weekdayTue') },
  { value: 3, label: t('components.general.label.weekdayWed') },
  { value: 4, label: t('components.general.label.weekdayThu') },
  { value: 5, label: t('components.general.label.weekdayFri') },
  { value: 6, label: t('components.general.label.weekdaySat') },
  { value: 0, label: t('components.general.label.weekdaySun') },
])

const authTypeOptions = AUTH_TYPE_OPTIONS

const isEditing = computed(() => props.card !== null)
const modalTitle = computed(() => (
  isEditing.value
    ? t('components.main.form.editTitle')
    : t('components.main.form.createTitle')
))
const authTypeLabel = computed(() => (
  authTypeOptions.find((option) => option.value === selectedAuthType.value)?.label || selectedAuthType.value
))
const builtinModelPlatform = computed<CLIPlatform | undefined>(() => (
  isBuiltinModelPlatform(props.tabId) ? props.tabId : undefined
))
const filteredIconOptions = computed(() => {
  const query = iconSearchQuery.value.toLowerCase().trim()
  if (!query) return iconOptions
  return iconOptions.filter((name) => name.toLowerCase().includes(query))
})
const iconPreviewOptions = computed(() => {
  const preferred = iconSearchQuery.value.trim() ? 120 : ICON_PRELOAD_BATCH_SIZE
  return Array.from(new Set([form.icon, ...filteredIconOptions.value.slice(0, preferred)]))
})

const resetForm = () => {
  errors.apiUrl = ''
  iconSearchQuery.value = ''
  requestBodyOverridesError.value = ''
  cliConfigEditorKey.value += 1

  if (!props.card) {
    Object.assign(form, createDefaultVendorForm(props.tabId, defaultIconKey))
    form.budgetQuotaSettings = createDefaultBudgetQuotaSettings()
    selectedAuthType.value = getDefaultAuthType(props.tabId)
    customAuthHeader.value = ''
    requestBodyOverridesText.value = formatJsonObject(form.requestBodyOverrides)
    return
  }

  Object.assign(form, createVendorFormFromCard(props.card, props.tabId))
  form.budgetQuotaSettings = normalizeBudgetQuotaSettings(props.card.budgetQuotaSettings)
  requestBodyOverridesText.value = formatJsonObject(form.requestBodyOverrides)

  const authState = resolveProviderAuthState(props.card.connectivityAuthType, props.tabId)
  selectedAuthType.value = authState.selectedAuthType
  customAuthHeader.value = authState.customAuthHeader
}

watch(() => props.open, (open) => {
  if (open) {
    resetForm()
  }
})

watch(() => props.card, () => {
  if (props.open) {
    resetForm()
  }
})

watch(() => props.tabId, () => {
  if (props.open) {
    resetForm()
  }
})

watch(iconPreviewOptions, (icons) => {
  if (!props.open) return
  void preloadLobeIcons(icons)
}, { immediate: true })

watch(requestBodyOverridesText, () => {
  requestBodyOverridesError.value = ''
})

const resolveEffectiveAuthType = () =>
  customAuthHeader.value.trim() || selectedAuthType.value || getDefaultAuthType(props.tabId)

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

const formatJsonObject = (value: Record<string, any> | undefined) => (
  JSON.stringify(toSortedJsonValue(value || {}), null, 2)
)

const parseRequestBodyOverrides = (): Record<string, any> | null => {
  requestBodyOverridesError.value = ''

  const raw = requestBodyOverridesText.value.trim()
  if (!raw) {
    requestBodyOverridesText.value = formatJsonObject({})
    return {}
  }

  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      requestBodyOverridesError.value = t('components.main.form.errors.requestBodyOverridesMustBeObject')
      return null
    }

    const normalized = parsed as Record<string, any>
    requestBodyOverridesText.value = formatJsonObject(normalized)
    return normalized
  } catch (error) {
    requestBodyOverridesError.value = t('components.main.form.errors.requestBodyOverridesInvalid', {
      error: error instanceof Error ? error.message : String(error),
    })
    return null
  }
}

const iconSvg = (name: string) => {
  if (!name) return ''
  return lobeIcons[name.toLowerCase()] ?? ''
}

const warmupIcon = (name: string) => {
  void preloadLobeIcons([name])
}

const getLevelDescription = (level: number) => {
  const descriptions: Record<number, string> = {
    1: t('components.main.levelDesc.highest'),
    2: t('components.main.levelDesc.high'),
    3: t('components.main.levelDesc.mediumHigh'),
    4: t('components.main.levelDesc.medium'),
    5: t('components.main.levelDesc.normal'),
    6: t('components.main.levelDesc.mediumLow'),
    7: t('components.main.levelDesc.low'),
    8: t('components.main.levelDesc.lower'),
    9: t('components.main.levelDesc.veryLow'),
    10: t('components.main.levelDesc.lowest'),
  }
  return descriptions[level] || t('components.main.levelDesc.normal')
}

const buildFormPayload = async (): Promise<VendorForm | null> => {
  const cliConfigReady = await (cliConfigEditorRef.value?.applyPendingJsonChanges?.() ?? true)
  if (!cliConfigReady) return null

  const apiUrl = form.apiUrl.trim()
  errors.apiUrl = ''

  try {
    const parsed = new URL(apiUrl)
    if (!/^https?:/.test(parsed.protocol)) throw new Error('protocol')
  } catch {
    errors.apiUrl = t('components.main.form.errors.invalidUrl')
    return null
  }

  form.apiUrl = apiUrl
  const requestBodyOverrides = parseRequestBodyOverrides()
  if (!requestBodyOverrides) return null

  const payload = buildNormalizedVendorForm({
    form,
    tabId: props.tabId,
    defaultIconKey,
    resolveAuthType: resolveEffectiveAuthType,
  })
  payload.requestBodyOverrides = requestBodyOverrides

  // 处理预算额度：仅保存 total > 0 的配置
  const qs = form.budgetQuotaSettings
  payload.budgetQuotaSettings = hasConfiguredBudgetQuotaSettings(qs)
    ? cloneBudgetQuotaSettings(qs)
    : undefined

  const cliConfigSubmitState = cliConfigEditorRef.value?.getCliConfigSubmitState?.()
  if (cliConfigSubmitState) {
    payload.cliConfig = cliConfigSubmitState.shouldPersist ? cliConfigSubmitState.value : {}
    payload.cliConfigPersistValue = cliConfigSubmitState.persistValue
    payload.cliConfigShouldPersist = cliConfigSubmitState.shouldPersist
  }

  return payload
}

const submit = async (applyAfterSave = false) => {
  const payload = await buildFormPayload()
  if (!payload) return

  if (applyAfterSave) {
    emit('submit-and-apply', payload)
    return
  }
  emit('submit', payload)
}
</script>

<style scoped src="../styles/provider-edit-modal.css"></style>
