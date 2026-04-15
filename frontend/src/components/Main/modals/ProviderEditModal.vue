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
          <div v-if="form.budgetQuotaSettings" class="budget-quota-grid provider-budget-quota-grid">
            <div
              v-for="def in quotaDefinitions"
              :key="def.key"
              class="budget-quota-card"
            >
              <div class="budget-quota-card__header">
                <div class="budget-quota-card__heading">
                  <p class="budget-quota-card__title">{{ t(def.titleKey) }}</p>
                  <p class="budget-quota-card__hint">{{ t(def.hintKey) }}</p>
                </div>
                <span class="budget-quota-card__limit">
                  {{ formatBudgetLimitLabel(form.budgetQuotaSettings[def.key].total) }}
                </span>
              </div>
              <div class="budget-quota-card__body">
                <div class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ t('components.general.label.budgetTotal') }}</span>
                  <div class="budget-input">
                    <input
                      v-model.number="form.budgetQuotaSettings[def.key].total"
                      type="number"
                      inputmode="decimal"
                      step="0.01"
                      min="0"
                      class="mac-input budget-input-field"
                      :placeholder="t('components.main.form.placeholders.budgetQuotaTotal')"
                      @change="handleBudgetQuotaConfigChange"
                    />
                    <span class="budget-unit">USD</span>
                  </div>
                  <span class="budget-quota-field__hint">{{ t('components.general.label.budgetQuotaUnsetHint') }}</span>
                </div>
                <div class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ t('components.general.label.budgetQuotaUsedAdjustment') }}</span>
                  <div class="budget-input">
                    <input
                      v-model.number="budgetQuotaCurrentUsed[def.key]"
                      type="number"
                      inputmode="decimal"
                      step="any"
                      min="0"
                      class="mac-input budget-input-field"
                      :disabled="!isBudgetQuotaCurrentUsedEditable(def.key)"
                      @change="handleBudgetQuotaCurrentUsedChange(def.key)"
                    />
                    <span class="budget-unit">USD</span>
                  </div>
                  <span class="budget-quota-field__hint">{{ getBudgetQuotaCurrentUsedHint(def.key) }}</span>
                </div>
                <div v-if="def.showWeekday" class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ t('components.general.label.budgetRefreshWeekday') }}</span>
                  <select
                    v-model.number="form.budgetQuotaSettings[def.key].refreshWeekday"
                    class="mac-select budget-select"
                    @change="handleBudgetQuotaConfigChange"
                  >
                    <option v-for="weekday in weekdayOptions" :key="weekday.value" :value="weekday.value">
                      {{ weekday.label }}
                    </option>
                  </select>
                </div>
                <div v-if="def.showMonthDay" class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ t('components.general.label.budgetRefreshMonthDay') }}</span>
                  <select
                    v-model.number="form.budgetQuotaSettings[def.key].refreshMonthDay"
                    class="mac-select budget-select"
                    @change="handleBudgetQuotaConfigChange"
                  >
                    <option v-for="day in monthDayOptions" :key="day" :value="day">
                      {{ day }}
                    </option>
                  </select>
                  <span class="budget-quota-field__hint">{{ t('components.general.label.budgetRefreshMonthDayHint') }}</span>
                </div>
                <div v-if="def.showTime" class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ t('components.general.label.budgetRefreshTime') }}</span>
                  <input
                    v-model="form.budgetQuotaSettings[def.key].refreshTime"
                    type="time"
                    class="mac-input budget-time-input"
                    @change="handleBudgetQuotaConfigChange"
                  />
                </div>
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
import { cardProviderRef } from '../adapters/providerCardMappers'
import {
  buildNormalizedVendorForm,
  createDefaultVendorForm,
  createVendorFormFromCard,
  resolveProviderAuthState,
} from '../adapters/providerFormMappers'
import type { ProviderTab, VendorForm } from '../types'
import type { LogPlatform } from '../../../services/logs'
import { fetchCostByProvider, fetchCostSinceByProvider, fetchFiveHourQuotaStatusByProvider } from '../../../services/logs'
import type { BudgetQuotaAdjustments, BudgetQuotaKey, BudgetQuotaSetting } from '../../../utils/budgetUsage'
import {
  cloneBudgetQuotaAdjustments,
  cloneBudgetQuotaSettings,
  createDefaultBudgetQuotaAdjustments,
  createDefaultBudgetQuotaSettings,
  formatLocalDateTime,
  hasConfiguredBudgetQuotaSettings,
  normalizeBudgetAdjustmentPrecision,
  normalizeBudgetEditableAmount,
  normalizeBudgetQuotaAdjustments,
  normalizeBudgetQuotaSettings,
  normalizeBudgetUsedDisplay,
  providerBudgetQuotaOrder,
  resolveBudgetQuotaWindow,
  resolveBudgetCurrentUsedValue,
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
  titleKey: string
  hintKey: string
  showWeekday: boolean
  showMonthDay: boolean
  showTime: boolean
}

const quotaDefinitions = computed<QuotaDefinition[]>(() => [
  {
    key: 'five_hour',
    titleKey: 'components.general.label.budgetQuotaFiveHour',
    hintKey: 'components.general.label.budgetQuotaFiveHourHint',
    showWeekday: false,
    showMonthDay: false,
    showTime: false,
  },
  {
    key: 'daily',
    titleKey: 'components.general.label.budgetQuotaDaily',
    hintKey: 'components.general.label.budgetQuotaDailyHint',
    showWeekday: false,
    showMonthDay: false,
    showTime: true,
  },
  {
    key: 'weekly',
    titleKey: 'components.general.label.budgetQuotaWeekly',
    hintKey: 'components.general.label.budgetQuotaWeeklyHint',
    showWeekday: true,
    showMonthDay: false,
    showTime: true,
  },
  {
    key: 'monthly',
    titleKey: 'components.general.label.budgetQuotaMonthly',
    hintKey: 'components.general.label.budgetQuotaMonthlyHint',
    showWeekday: false,
    showMonthDay: true,
    showTime: true,
  },
  {
    key: 'total',
    titleKey: 'components.general.label.budgetQuotaTotal',
    hintKey: 'components.general.label.budgetQuotaTotalHint',
    showWeekday: false,
    showMonthDay: false,
    showTime: false,
  },
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
const monthDayOptions = Array.from({ length: 31 }, (_, index) => index + 1)

type BudgetQuotaUsageStatus = 'inactive' | 'loading' | 'ready' | 'error'
type BudgetQuotaUsageStatuses = Record<BudgetQuotaKey, BudgetQuotaUsageStatus>

const createDefaultBudgetQuotaUsageStatuses = (
  status: BudgetQuotaUsageStatus = 'inactive',
): BudgetQuotaUsageStatuses => ({
  five_hour: status,
  daily: status,
  weekly: status,
  monthly: status,
  total: status,
})

const mapBudgetQuotaValues = (resolveValue: (key: BudgetQuotaKey) => number): BudgetQuotaAdjustments => {
  const nextValues = createDefaultBudgetQuotaAdjustments()
  providerBudgetQuotaOrder.forEach((key) => {
    nextValues[key] = resolveValue(key)
  })
  return nextValues
}

const mapBudgetQuotaUsageStatuses = (
  resolveStatus: (key: BudgetQuotaKey) => BudgetQuotaUsageStatus,
): BudgetQuotaUsageStatuses => {
  const nextStatuses = createDefaultBudgetQuotaUsageStatuses()
  providerBudgetQuotaOrder.forEach((key) => {
    nextStatuses[key] = resolveStatus(key)
  })
  return nextStatuses
}

const budgetQuotaTrackedUsage = ref<BudgetQuotaAdjustments>(createDefaultBudgetQuotaAdjustments())
const budgetQuotaCurrentUsed = ref<BudgetQuotaAdjustments>(createDefaultBudgetQuotaAdjustments())
const budgetQuotaUsageStatuses = ref<BudgetQuotaUsageStatuses>(createDefaultBudgetQuotaUsageStatuses())
let budgetQuotaUsageRequestSeq = 0

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

const formatBudgetLimitLabel = (total: number) => {
  if (total <= 0) return '∞'
  if (total >= 1) return `$${total.toFixed(2)}`
  if (total >= 0.01) return `$${total.toFixed(3)}`
  return `$${total.toFixed(4)}`
}

const buildBudgetQuotaCurrentUsed = (
  trackedUsage: BudgetQuotaAdjustments,
  adjustments: BudgetQuotaAdjustments,
  statuses: BudgetQuotaUsageStatuses,
) => {
  return mapBudgetQuotaValues((key) => (
    statuses[key] === 'ready'
      ? resolveBudgetCurrentUsedValue(trackedUsage[key], adjustments[key])
      : 0
  ))
}

const syncBudgetQuotaCurrentUsed = () => {
  budgetQuotaCurrentUsed.value = buildBudgetQuotaCurrentUsed(
    budgetQuotaTrackedUsage.value,
    normalizeBudgetQuotaAdjustments(form.budgetQuotaUsedAdjustments),
    budgetQuotaUsageStatuses.value,
  )
}

const isBudgetQuotaCurrentUsedEditable = (key: BudgetQuotaKey) => budgetQuotaUsageStatuses.value[key] === 'ready'

const getBudgetQuotaCurrentUsedHint = (key: BudgetQuotaKey) => {
  const status = budgetQuotaUsageStatuses.value[key]
  if (status === 'inactive') {
    return t('components.general.label.budgetQuotaUsedInactiveHint')
  }
  if (status === 'loading') {
    return t('components.general.label.budgetQuotaUsedLoadingHint')
  }
  if (status === 'error') {
    return t('components.general.label.budgetQuotaUsedUnavailableHint')
  }
  if (key === 'total') {
    return t('components.general.label.budgetQuotaUsedTotalAdjustmentHint')
  }
  return t('components.general.label.budgetQuotaUsedAdjustmentHint')
}

const resolveQuotaPlatform = (): LogPlatform | '' => {
  if (props.tabId === 'claude' || props.tabId === 'codex' || props.tabId === 'gemini') {
    return props.tabId
  }
  return ''
}

const nextBudgetQuotaUsageRequestId = () => {
  budgetQuotaUsageRequestSeq += 1
  return budgetQuotaUsageRequestSeq
}

const refreshBudgetQuotaUsage = async () => {
  const requestId = nextBudgetQuotaUsageRequestId()
  const quotaSettings = normalizeBudgetQuotaSettings(form.budgetQuotaSettings)
  const activeQuotaKeys = providerBudgetQuotaOrder.filter((key) => quotaSettings[key].total > 0)
  const activeQuotaKeySet = new Set(activeQuotaKeys)

  budgetQuotaUsageStatuses.value = mapBudgetQuotaUsageStatuses((key) => (
    activeQuotaKeySet.has(key) ? 'loading' : 'inactive'
  ))
  budgetQuotaTrackedUsage.value = createDefaultBudgetQuotaAdjustments()
  budgetQuotaCurrentUsed.value = createDefaultBudgetQuotaAdjustments()

  if (activeQuotaKeys.length === 0) {
    return
  }

  const platform = resolveQuotaPlatform()
  const persistedCard = props.card
  const providerRef = persistedCard ? cardProviderRef(persistedCard) : ''
  const providerName = persistedCard?.name?.trim() || form.name.trim()

  if (!persistedCard || !platform || !providerRef || !providerName) {
    if (requestId !== budgetQuotaUsageRequestSeq) return
    budgetQuotaUsageStatuses.value = mapBudgetQuotaUsageStatuses((key) => (
      activeQuotaKeySet.has(key) ? 'ready' : 'inactive'
    ))
    budgetQuotaTrackedUsage.value = createDefaultBudgetQuotaAdjustments()
    syncBudgetQuotaCurrentUsed()
    return
  }

  try {
    const now = new Date()
    const nextTrackedUsage = createDefaultBudgetQuotaAdjustments()
    const nextStatuses = mapBudgetQuotaUsageStatuses((key) => (
      activeQuotaKeySet.has(key) ? 'error' : 'inactive'
    ))
    const results = await Promise.allSettled(
      activeQuotaKeys.map(async (key) => {
        if (key === 'total') {
          const usage = await fetchCostByProvider(platform, providerRef, providerName)
          return {
            key,
            status: 'ready' as const,
            usage: normalizeBudgetUsedDisplay(Number(usage)),
          }
        }
        if (key === 'five_hour') {
          const snapshot = await fetchFiveHourQuotaStatusByProvider(platform, providerRef, providerName)
          return {
            key,
            status: (snapshot.active ? 'ready' : 'inactive') as BudgetQuotaUsageStatus,
            usage: normalizeBudgetUsedDisplay(Number(snapshot?.used ?? 0)),
          }
        }
        const setting = quotaSettings[key] as BudgetQuotaSetting
        const window = resolveBudgetQuotaWindow(key, setting, now)
        const usage = await fetchCostSinceByProvider(formatLocalDateTime(window.start), platform, providerRef, providerName)
        return {
          key,
          status: 'ready' as const,
          usage: normalizeBudgetUsedDisplay(Number(usage)),
        }
      }),
    )
    if (requestId !== budgetQuotaUsageRequestSeq) return
    results.forEach((result) => {
      if (result.status !== 'fulfilled') return
      nextTrackedUsage[result.value.key] = result.value.usage
      nextStatuses[result.value.key] = result.value.status
    })
    budgetQuotaTrackedUsage.value = nextTrackedUsage
    budgetQuotaUsageStatuses.value = nextStatuses
    syncBudgetQuotaCurrentUsed()
  } catch (error) {
    console.error('failed to load provider quota usage', error)
    if (requestId !== budgetQuotaUsageRequestSeq) return
    budgetQuotaUsageStatuses.value = mapBudgetQuotaUsageStatuses((key) => (
      activeQuotaKeySet.has(key) ? 'error' : 'inactive'
    ))
  }
}

const handleBudgetQuotaCurrentUsedChange = (key: BudgetQuotaKey) => {
  if (budgetQuotaUsageStatuses.value[key] !== 'ready') return
  const nextUsed = normalizeBudgetEditableAmount(budgetQuotaCurrentUsed.value[key])
  budgetQuotaCurrentUsed.value[key] = nextUsed
  if (!form.budgetQuotaUsedAdjustments) {
    form.budgetQuotaUsedAdjustments = createDefaultBudgetQuotaAdjustments()
  }
  form.budgetQuotaUsedAdjustments[key] = normalizeBudgetAdjustmentPrecision(
    nextUsed - budgetQuotaTrackedUsage.value[key],
  )
  syncBudgetQuotaCurrentUsed()
}

const handleBudgetQuotaConfigChange = () => {
  void refreshBudgetQuotaUsage()
}

const resetForm = () => {
  errors.apiUrl = ''
  iconSearchQuery.value = ''
  requestBodyOverridesError.value = ''
  cliConfigEditorKey.value += 1

  if (!props.card) {
    Object.assign(form, createDefaultVendorForm(props.tabId, defaultIconKey))
    form.budgetQuotaSettings = createDefaultBudgetQuotaSettings()
    form.budgetQuotaUsedAdjustments = createDefaultBudgetQuotaAdjustments()
    selectedAuthType.value = getDefaultAuthType(props.tabId)
    customAuthHeader.value = ''
    requestBodyOverridesText.value = formatJsonObject(form.requestBodyOverrides)
    void refreshBudgetQuotaUsage()
    return
  }

  Object.assign(form, createVendorFormFromCard(props.card, props.tabId))
  form.budgetQuotaSettings = normalizeBudgetQuotaSettings(props.card.budgetQuotaSettings)
  form.budgetQuotaUsedAdjustments = cloneBudgetQuotaAdjustments(props.card.budgetQuotaUsedAdjustments)
  requestBodyOverridesText.value = formatJsonObject(form.requestBodyOverrides)

  const authState = resolveProviderAuthState(props.card.connectivityAuthType, props.tabId)
  selectedAuthType.value = authState.selectedAuthType
  customAuthHeader.value = authState.customAuthHeader
  void refreshBudgetQuotaUsage()
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
  payload.budgetQuotaUsedAdjustments = hasConfiguredBudgetQuotaSettings(qs)
    ? cloneBudgetQuotaAdjustments(form.budgetQuotaUsedAdjustments)
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
