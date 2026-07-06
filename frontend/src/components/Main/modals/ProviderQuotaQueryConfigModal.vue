<template>
  <InlineModal
    :open="open"
    :title="t('components.main.form.labels.providerQuotaQueryConfig')"
    :body-scrollable="false"
    panel-class="provider-quota-query-config-modal-shell"
    :panel-width="'min(960px, 94vw)'"
    @close="$emit('close')"
  >
    <form class="provider-quota-query-config-modal" @submit.prevent="handleSave">
      <div class="provider-quota-query-config-modal__content">
        <section class="provider-quota-query-config-modal__section">
          <div class="provider-quota-query-config-modal__switch-row">
            <div class="provider-quota-query-config-modal__heading-block">
              <p class="provider-quota-query-config-modal__title">
                {{ t('components.main.form.labels.providerQuotaQueryEnabled') }}
              </p>
              <p class="provider-quota-query-config-modal__hint">
                {{ draft.enabled
                  ? t('components.main.form.hints.providerQuotaQueryEnabled')
                  : t('components.main.form.hints.providerQuotaQueryDisabled') }}
              </p>
            </div>
            <label class="mac-switch">
              <input v-model="draft.enabled" type="checkbox" />
              <span></span>
            </label>
          </div>
        </section>

        <section class="provider-quota-query-config-modal__section">
          <div class="provider-quota-query-config-modal__section-header">
            <div>
              <p class="provider-quota-query-config-modal__title">
                {{ t('components.main.form.labels.providerQuotaQueryTemplate') }}
              </p>
              <p class="provider-quota-query-config-modal__hint">
                {{ t('components.main.form.hints.providerQuotaQueryTemplateHint') }}
              </p>
            </div>
          </div>

          <div class="provider-quota-query-config-modal__template-grid">
            <button
              v-for="option in templateOptions"
              :key="option.value"
              type="button"
              :class="[
                'provider-quota-query-config-modal__template-card',
                'provider-quota-query-config-modal__template-card--detail',
                { 'is-selected': selectedTemplate === option.value },
              ]"
              @click="handleSelectTemplate(option.value)"
            >
              <span class="provider-quota-query-config-modal__template-copy">
                <span class="provider-quota-query-config-modal__template-name">{{ option.label }}</span>
                <span class="provider-quota-query-config-modal__template-desc">{{ option.description }}</span>
              </span>
            </button>
          </div>

          <div
            v-if="selectedTemplate === 'balance'"
            :class="[
              'provider-quota-query-config-modal__template-status',
              { 'is-warning': !detectedBalanceProviderOption },
            ]"
          >
            <span class="provider-quota-query-config-modal__template-status-badge">
              {{ detectedBalanceProviderOption
                ? t('components.main.form.hints.providerQuotaQueryBalanceDetectedBadge', {
                  provider: detectedBalanceProviderOption.label,
                })
                : t('components.main.form.hints.providerQuotaQueryBalanceUnsupportedBadge') }}
            </span>
            <p class="provider-quota-query-config-modal__template-status-text">
              {{ detectedBalanceProviderOption
                ? t('components.main.form.hints.providerQuotaQueryBalanceDetected', {
                  provider: detectedBalanceProviderOption.label,
                })
                : t('components.main.form.hints.providerQuotaQueryBalanceUnsupported', {
                  url: `${props.providerApiUrl ?? ''}`.trim() || '-',
                }) }}
            </p>
          </div>
        </section>

        <section class="provider-quota-query-config-modal__section">
          <div class="provider-quota-query-config-modal__section-header">
            <div>
              <p class="provider-quota-query-config-modal__title">
                {{ t('components.main.form.labels.providerQuotaQueryTiming') }}
              </p>
              <p class="provider-quota-query-config-modal__hint">
                {{ t('components.main.form.hints.providerQuotaQueryAutoInterval') }}
              </p>
            </div>
          </div>

          <div class="provider-quota-query-config-modal__field-grid">
            <label class="form-field provider-quota-query-config-modal__field">
              <span>{{ t('components.main.form.labels.providerQuotaQueryTimeout') }}</span>
              <input
                v-model.number="draft.timeout"
                type="number"
                min="2"
                max="30"
                step="1"
                class="mac-input"
                :placeholder="t('components.main.form.placeholders.providerQuotaQueryTimeout')"
              />
              <span class="field-hint">{{ t('components.main.form.hints.providerQuotaQueryTimeout') }}</span>
            </label>

            <label class="form-field provider-quota-query-config-modal__field">
              <span>{{ t('components.main.form.labels.providerQuotaQueryAutoInterval') }}</span>
              <input
                v-model.number="draft.autoQueryInterval"
                type="number"
                min="0"
                max="1440"
                step="1"
                class="mac-input"
                :placeholder="t('components.main.form.placeholders.providerQuotaQueryAutoInterval')"
              />
              <span class="field-hint">{{ t('components.main.form.hints.providerQuotaQueryAutoInterval') }}</span>
            </label>
          </div>
        </section>

        <section
          v-if="selectedTemplate === 'token_plan'"
          class="provider-quota-query-config-modal__section"
        >
          <div class="provider-quota-query-config-modal__section-header">
            <div>
              <p class="provider-quota-query-config-modal__title">
                {{ t('components.main.form.labels.providerQuotaQueryTokenPlanProvider') }}
              </p>
              <p class="provider-quota-query-config-modal__hint">
                {{ t('components.main.form.hints.providerQuotaQueryTemplateTokenPlan') }}
              </p>
            </div>
          </div>

          <div class="provider-quota-query-config-modal__template-grid provider-quota-query-config-modal__template-grid--compact">
            <button
              v-for="provider in tokenPlanProviders"
              :key="provider.value"
              type="button"
              :class="[
                'provider-quota-query-config-modal__template-card',
                'provider-quota-query-config-modal__template-card--compact',
                { 'is-selected': draft.tokenPlanProvider === provider.value },
              ]"
              @click="draft.tokenPlanProvider = provider.value"
            >
              <span class="provider-quota-query-config-modal__template-name">{{ provider.label }}</span>
            </button>
          </div>
        </section>

        <section
          v-if="showCredentialsSection"
          class="provider-quota-query-config-modal__section"
        >
          <div class="provider-quota-query-config-modal__section-header">
            <div>
              <p class="provider-quota-query-config-modal__title">
                {{ t('components.main.form.labels.providerQuotaQueryCredentials') }}
              </p>
              <p class="provider-quota-query-config-modal__hint">
                {{ credentialHint }}
              </p>
            </div>
          </div>

          <div class="provider-quota-query-config-modal__field-grid">
            <label v-if="showBaseUrlField" class="form-field provider-quota-query-config-modal__field">
              <span>{{ t('components.main.form.labels.providerQuotaQueryDedicatedBaseUrl') }}</span>
              <BaseInput
                v-model="draft.baseUrl"
                type="text"
                :placeholder="t('components.main.form.placeholders.providerQuotaQueryBaseUrl')"
              />
            </label>

            <label v-if="showApiKeyField" class="form-field provider-quota-query-config-modal__field">
              <span>{{ t('components.main.form.labels.providerQuotaQueryDedicatedApiKey') }}</span>
              <BaseInput
                v-model="draft.apiKey"
                type="text"
                :placeholder="t('components.main.form.placeholders.providerQuotaQueryApiKey')"
              />
            </label>

            <label v-if="showAccessTokenField" class="form-field provider-quota-query-config-modal__field">
              <span>{{ t('components.main.form.labels.providerQuotaQueryAccessToken') }}</span>
              <BaseInput
                v-model="draft.accessToken"
                type="text"
                :placeholder="t('components.main.form.placeholders.providerQuotaQueryAccessToken')"
              />
              <span class="field-hint">{{ t('components.main.form.hints.providerQuotaQueryAccessTokenHint') }}</span>
            </label>

            <label v-if="showUserIdField" class="form-field provider-quota-query-config-modal__field">
              <span>{{ t('components.main.form.labels.providerQuotaQueryUserId') }}</span>
              <BaseInput
                v-model="draft.userId"
                type="text"
                :placeholder="t('components.main.form.placeholders.providerQuotaQueryUserId')"
              />
              <span class="field-hint">{{ t('components.main.form.hints.providerQuotaQueryUserIdHint') }}</span>
            </label>
          </div>
        </section>

        <section
          v-if="showScriptSection"
          class="provider-quota-query-config-modal__section"
        >
          <div class="provider-quota-query-config-modal__section-header provider-quota-query-config-modal__section-header--with-action">
            <div>
              <p class="provider-quota-query-config-modal__title">
                {{ t('components.main.form.labels.providerQuotaQueryCode') }}
              </p>
              <p class="provider-quota-query-config-modal__hint">
                {{ t('components.main.form.hints.providerQuotaQueryCodeHint') }}
              </p>
            </div>
            <div class="provider-quota-query-config-modal__section-actions">
              <BaseButton variant="outline" type="button" @click="handleLoadPresetCode">
                {{ t('components.main.form.actions.providerQuotaQueryLoadPreset') }}
              </BaseButton>
              <BaseButton variant="outline" type="button" @click="openPresetEditor">
                {{ t('components.main.form.actions.providerQuotaQueryEditPreset') }}
              </BaseButton>
            </div>
          </div>

          <JsonCodeEditor
            v-if="showScriptSection"
            v-model="draft.code"
            mode="plain"
            :rows="20"
            :show-validation="false"
            :surface-height="'360px'"
            :placeholder="t('components.main.form.placeholders.providerQuotaQueryCode')"
          />
        </section>
      </div>

      <footer class="provider-quota-query-config-modal__actions provider-quota-query-config-modal__actions--fixed">
        <BaseButton variant="outline" type="button" @click="$emit('close')">
          {{ t('components.main.form.actions.cancel') }}
        </BaseButton>
        <BaseButton type="button" variant="outline" :disabled="testing" @click="handleTest">
          {{ testing
            ? t('components.main.form.actions.providerQuotaQueryTesting')
            : t('components.main.form.actions.providerQuotaQueryTest') }}
        </BaseButton>
        <BaseButton type="submit">
          {{ t('components.main.form.actions.save') }}
        </BaseButton>
      </footer>
    </form>
  </InlineModal>

  <InlineModal
    :open="presetEditorOpen"
    :title="t('components.main.form.labels.providerQuotaQueryPresetEditor')"
    :body-scrollable="true"
    :panel-width="'min(860px, 92vw)'"
    @close="requestClosePresetEditor"
  >
    <form class="provider-quota-query-preset-editor" @submit.prevent="handleSavePresetCode">
      <section class="provider-quota-query-config-modal__section">
        <div class="provider-quota-query-config-modal__section-header">
          <div>
            <p class="provider-quota-query-config-modal__title">
              {{ selectedTemplateLabel }}
            </p>
            <p class="provider-quota-query-config-modal__hint">
              {{ t('components.main.form.hints.providerQuotaQueryPresetEditor') }}
            </p>
          </div>
          <BaseButton variant="outline" type="button" @click="createPresetDraft">
            {{ t('components.main.form.actions.providerQuotaQueryAddPreset') }}
          </BaseButton>
        </div>

        <div class="provider-quota-query-preset-editor__layout">
          <aside class="provider-quota-query-preset-editor__list">
            <button
              v-for="preset in presetEditorItems"
              :key="preset.id"
              type="button"
              :class="[
                'provider-quota-query-preset-editor__item',
                { 'is-selected': selectedPresetId === preset.id },
              ]"
              @click="selectPresetDraft(preset.id)"
            >
              <span class="provider-quota-query-preset-editor__item-name">{{ preset.name }}</span>
              <span
                v-if="currentPresetGroup.defaultId === preset.id"
                class="provider-quota-query-preset-editor__item-badge"
              >
                {{ t('components.main.form.labels.providerQuotaQueryDefaultPreset') }}
              </span>
            </button>
            <p v-if="presetEditorItems.length === 0" class="provider-quota-query-preset-editor__empty">
              {{ t('components.main.form.hints.providerQuotaQueryNoPresets') }}
            </p>
          </aside>

          <div class="provider-quota-query-preset-editor__body">
            <label class="form-field">
              <span>{{ t('components.main.form.labels.providerQuotaQueryPresetName') }}</span>
              <BaseInput
                v-model="presetEditorName"
                type="text"
                :placeholder="t('components.main.form.placeholders.providerQuotaQueryPresetName')"
              />
            </label>

            <JsonCodeEditor
              v-model="presetEditorText"
              mode="plain"
              :rows="20"
              :show-validation="false"
              :surface-height="'360px'"
              :placeholder="t('components.main.form.placeholders.providerQuotaQueryCode')"
            />
          </div>
        </div>
        <p v-if="presetEditorError" class="provider-quota-query-preset-editor__error">
          {{ presetEditorError }}
        </p>
      </section>

      <footer class="provider-quota-query-config-modal__actions">
        <BaseButton variant="outline" type="button" @click="requestClosePresetEditor">
          {{ t('components.main.form.actions.cancel') }}
        </BaseButton>
        <BaseButton
          type="button"
          variant="outline"
          :disabled="!selectedPresetId || presetEditorSaving || presetEditorDeleting || presetEditorDefaulting"
          @click="handleDeletePresetCode"
        >
          {{ presetEditorDeleting
            ? t('components.main.form.actions.providerQuotaQueryDeletingPreset')
            : t('components.main.form.actions.providerQuotaQueryDeletePreset') }}
        </BaseButton>
        <BaseButton
          type="button"
          variant="outline"
          :disabled="!selectedPresetId || currentPresetGroup.defaultId === selectedPresetId || presetEditorSaving || presetEditorDeleting || presetEditorDefaulting"
          @click="handleSetDefaultPresetCode"
        >
          {{ presetEditorDefaulting
            ? t('components.main.form.actions.providerQuotaQuerySettingDefaultPreset')
            : t('components.main.form.actions.providerQuotaQuerySetDefaultPreset') }}
        </BaseButton>
        <BaseButton type="submit" :disabled="presetEditorSaving || presetEditorDeleting || presetEditorDefaulting">
          {{ presetEditorSaving
            ? t('components.main.form.actions.providerQuotaQuerySavingPreset')
            : t('components.main.form.actions.save') }}
        </BaseButton>
      </footer>
    </form>
  </InlineModal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseButton from '../../common/BaseButton.vue'
import BaseInput from '../../common/BaseInput.vue'
import InlineModal from '../../common/InlineModal.vue'
import JsonCodeEditor from '../../common/JsonCodeEditor.vue'
import {
  fetchAppSettings,
  saveAppSettings,
  type ProviderQuotaQueryPresetEntry,
  type ProviderQuotaQueryPresetGroup,
  type ProviderQuotaQueryPresetGroups,
} from '../../../services/appSettings'
import { queryProviderQuota, validateProviderQuotaScriptPreset } from '../../../services/providerQuotaQuery'
import { showToast } from '../../../utils/toast'
import {
  detectProviderQuotaBalanceProvider,
  detectProviderQuotaTokenPlanProvider,
  normalizeProviderQuotaQueryConfig,
  providerQuotaBalanceProviderOptions,
  providerQuotaTemplateLabelKeyMap,
  providerQuotaTemplateTypes,
  providerQuotaTokenPlanProviderLabelKeyMap,
  resetProviderQuotaQueryConfigFieldsOnTemplateSwitch,
  sanitizeProviderQuotaQueryConfigForSave,
  validateProviderQuotaQueryConfigForSave,
  type ProviderQuotaQueryConfig,
  type ProviderQuotaQuerySaveValidationIssue,
  type ProviderQuotaTemplateType,
  type ProviderQuotaTokenPlanProvider,
} from '../../../utils/providerQuotaQuery'

const DEFAULT_TIMEOUT = 10
const DEFAULT_AUTO_QUERY_INTERVAL = 5
const editablePresetTemplateTypes = new Set<ProviderQuotaTemplateType>(['custom', 'general', 'newapi'])

const props = defineProps<{
  open: boolean
  modelValue?: ProviderQuotaQueryConfig
  providerApiUrl?: string
  providerApiKey?: string
}>()

const emit = defineEmits<{
  close: []
  save: [config: ProviderQuotaQueryConfig]
}>()

const { t } = useI18n()

const draft = reactive<ProviderQuotaQueryConfig>({
  enabled: false,
  templateType: 'general',
  code: '',
  timeout: DEFAULT_TIMEOUT,
  apiKey: '',
  baseUrl: '',
  accessToken: '',
  userId: '',
  tokenPlanProvider: 'kimi',
  autoQueryInterval: DEFAULT_AUTO_QUERY_INTERVAL,
})
const testing = ref(false)
const presetGroups = ref<ProviderQuotaQueryPresetGroups>({})
const presetEditorOpen = ref(false)
const selectedPresetId = ref('')
const presetEditorName = ref('')
const presetEditorText = ref('')
const presetEditorSnapshotName = ref('')
const presetEditorSnapshotText = ref('')
const presetEditorError = ref('')
const presetEditorSaving = ref(false)
const presetEditorDeleting = ref(false)
const presetEditorDefaulting = ref(false)

const detectedTokenPlanProvider = computed(() => (
  detectProviderQuotaTokenPlanProvider(props.providerApiUrl)
))
const detectedBalanceProvider = computed(() => (
  detectProviderQuotaBalanceProvider(props.providerApiUrl)
))
const templateOptions = computed(() => providerQuotaTemplateTypes.map((value) => ({
  value,
  label: t(providerQuotaTemplateLabelKeyMap[value]),
  description: t(templateDescriptionKeyMap[value]),
})))
const tokenPlanProviders = computed(() => (['glm', 'kimi', 'minimax'] as ProviderQuotaTokenPlanProvider[]).map((value) => ({
  value,
  label: t(providerQuotaTokenPlanProviderLabelKeyMap[value]),
})))
const detectedBalanceProviderOption = computed(() => (
  providerQuotaBalanceProviderOptions.find((option) => option.value === detectedBalanceProvider.value) ?? null
))
const selectedTemplate = computed<ProviderQuotaTemplateType>(() => (
  draft.templateType ?? 'general'
))
const selectedTemplateLabel = computed(() => t(providerQuotaTemplateLabelKeyMap[selectedTemplate.value]))
const currentPresetGroup = computed<ProviderQuotaQueryPresetGroup>(() => (
  presetGroups.value[selectedTemplate.value] ?? { items: [] }
))
const presetEditorItems = computed(() => currentPresetGroup.value.items ?? [])
const hasPresetEditorUnsavedChanges = computed(() => (
  presetEditorName.value !== presetEditorSnapshotName.value
    || presetEditorText.value !== presetEditorSnapshotText.value
))
const showScriptSection = computed(() => (
  selectedTemplate.value === 'custom'
    || selectedTemplate.value === 'general'
    || selectedTemplate.value === 'newapi'
))
const showCredentialsSection = computed(() => showScriptSection.value)
const showBaseUrlField = computed(() => showCredentialsSection.value)
const showApiKeyField = computed(() => (
  selectedTemplate.value === 'custom' || selectedTemplate.value === 'general'
))
const showAccessTokenField = computed(() => (
  selectedTemplate.value === 'custom' || selectedTemplate.value === 'newapi'
))
const showUserIdField = computed(() => (
  selectedTemplate.value === 'custom' || selectedTemplate.value === 'newapi'
))
const credentialHint = computed(() => {
  if (selectedTemplate.value === 'newapi') {
    return t('components.main.form.hints.providerQuotaQueryNewApiCredentialHint')
  }
  if (selectedTemplate.value === 'custom') {
    return t('components.main.form.hints.providerQuotaQueryCustomCredentialHint')
  }
  return t('components.main.form.hints.providerQuotaQueryWillFallbackToProvider')
})

const templateDescriptionKeyMap: Record<ProviderQuotaTemplateType, string> = {
  balance: 'components.main.form.hints.providerQuotaQueryTemplateBalance',
  custom: 'components.main.form.hints.providerQuotaQueryTemplateCustom',
  general: 'components.main.form.hints.providerQuotaQueryTemplateGeneral',
  newapi: 'components.main.form.hints.providerQuotaQueryTemplateNewApi',
  token_plan: 'components.main.form.hints.providerQuotaQueryTemplateTokenPlan',
}

const saveValidationMessageKeyMap: Record<ProviderQuotaQuerySaveValidationIssue, string> = {
  missing_script: 'components.main.form.toast.providerQuotaQueryScriptRequired',
  missing_newapi_credentials: 'components.main.form.toast.providerQuotaQueryNewApiCredentialsRequired',
  missing_provider_credentials: 'components.main.form.toast.providerQuotaQueryProviderCredentialsRequired',
  unsupported_balance_provider: 'components.main.form.toast.providerQuotaQueryBalanceProviderUnsupported',
}

function buildPresetCode(templateType: ProviderQuotaTemplateType): string {
  switch (templateType) {
    case 'custom':
      return `({
  request: {
    url: '',
    method: 'GET',
    headers: {},
  },
  extractor: function(response) {
    return {
      label: 'Quota',
      remaining: 0,
      unit: 'USD',
      valueMode: 'currency',
    };
  },
})`
    case 'general':
      return `({
  request: {
    url: '{{baseUrl}}/user/balance',
    method: 'GET',
    headers: {
      'Authorization': 'Bearer {{apiKey}}',
      'User-Agent': 'code-switch-R/1.0',
    },
  },
  extractor: function(response) {
    return {
      label: 'Balance',
      remaining: response.balance,
      unit: 'USD',
      valueMode: 'currency',
    };
  },
})`
    case 'newapi':
      return `({
  request: {
    url: '{{baseUrl}}/api/user/self',
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer {{accessToken}}',
      'User-Agent': 'code-switch-R/1.0',
      'New-Api-User': '{{userId}}',
    },
  },
  extractor: function(response) {
    if (response.success && response.data) {
      return {
        label: response.data.group || 'Default Plan',
        remaining: response.data.quota / 500000,
        used: response.data.used_quota / 500000,
        total: (response.data.quota + response.data.used_quota) / 500000,
        unit: 'USD',
        valueMode: 'currency',
      };
    }
    return {
      label: 'NewAPI',
      isValid: false,
      invalidMessage: response.message || 'Query failed',
    };
  },
})`
    default:
      return ''
  }
}

function resolvePresetCode(templateType: ProviderQuotaTemplateType): string {
  if (!editablePresetTemplateTypes.has(templateType)) {
    return buildPresetCode(templateType)
  }

  const group = presetGroups.value[templateType]
  const defaultPreset = group?.items?.find((item) => item.id === group.defaultId)
  const customCode = `${defaultPreset?.code ?? ''}`.trim()
  if (customCode) return customCode

  return buildPresetCode(templateType)
}

function createDefaultConfig(): ProviderQuotaQueryConfig {
  const tokenPlanProvider = detectedTokenPlanProvider.value
  if (tokenPlanProvider) {
    return {
      enabled: false,
      templateType: 'token_plan',
      tokenPlanProvider,
      timeout: DEFAULT_TIMEOUT,
      autoQueryInterval: DEFAULT_AUTO_QUERY_INTERVAL,
      code: '',
      apiKey: '',
      baseUrl: '',
      accessToken: '',
      userId: '',
    }
  }

  if (detectedBalanceProvider.value) {
    return {
      enabled: false,
      templateType: 'balance',
      timeout: DEFAULT_TIMEOUT,
      autoQueryInterval: DEFAULT_AUTO_QUERY_INTERVAL,
      code: '',
      apiKey: '',
      baseUrl: '',
      accessToken: '',
      userId: '',
    }
  }

  return {
    enabled: false,
    templateType: 'general',
    code: buildPresetCode('general'),
    timeout: DEFAULT_TIMEOUT,
    autoQueryInterval: DEFAULT_AUTO_QUERY_INTERVAL,
    apiKey: '',
    baseUrl: '',
    accessToken: '',
    userId: '',
    tokenPlanProvider: 'kimi',
  }
}

function applyDraft(nextConfig: ProviderQuotaQueryConfig) {
  draft.enabled = !!nextConfig.enabled
  draft.templateType = nextConfig.templateType ?? 'general'
  draft.code = nextConfig.code ?? ''
  draft.timeout = clampInteger(nextConfig.timeout, DEFAULT_TIMEOUT, 2, 30)
  draft.apiKey = `${nextConfig.apiKey ?? ''}`
  draft.baseUrl = `${nextConfig.baseUrl ?? ''}`
  draft.accessToken = `${nextConfig.accessToken ?? ''}`
  draft.userId = `${nextConfig.userId ?? ''}`
  draft.tokenPlanProvider = nextConfig.tokenPlanProvider ?? detectedTokenPlanProvider.value ?? 'kimi'
  draft.autoQueryInterval = clampInteger(
    nextConfig.autoQueryInterval,
    DEFAULT_AUTO_QUERY_INTERVAL,
    0,
    1440,
  )
}

function resetDraft() {
  const normalized = normalizeProviderQuotaQueryConfig(props.modelValue)
  const nextConfig = normalized
    ? {
        ...createDefaultConfig(),
        ...normalized,
      }
    : createDefaultConfig()

  if (showScriptTemplate(nextConfig.templateType) && !`${nextConfig.code ?? ''}`.trim()) {
    nextConfig.code = buildPresetCode(nextConfig.templateType ?? 'general')
  }
  applyDraft(nextConfig)
}

function showScriptTemplate(templateType: ProviderQuotaTemplateType | undefined): templateType is Extract<ProviderQuotaTemplateType, 'custom' | 'general' | 'newapi'> {
  return templateType === 'custom' || templateType === 'general' || templateType === 'newapi'
}

function clampInteger(value: unknown, fallback: number, min: number, max: number): number {
  const numericValue = Number(value)
  if (!Number.isFinite(numericValue)) return fallback
  return Math.min(Math.max(Math.floor(numericValue), min), max)
}

function buildSaveConfig(forceEnable = false): ProviderQuotaQueryConfig {
  const normalized = sanitizeProviderQuotaQueryConfigForSave({
    enabled: forceEnable ? true : !!draft.enabled,
    templateType: selectedTemplate.value,
    code: draft.code ?? '',
    timeout: clampInteger(draft.timeout, DEFAULT_TIMEOUT, 2, 30),
    apiKey: `${draft.apiKey ?? ''}`.trim(),
    baseUrl: `${draft.baseUrl ?? ''}`.trim(),
    accessToken: `${draft.accessToken ?? ''}`.trim(),
    userId: `${draft.userId ?? ''}`.trim(),
    tokenPlanProvider: draft.tokenPlanProvider ?? detectedTokenPlanProvider.value ?? 'kimi',
    autoQueryInterval: clampInteger(draft.autoQueryInterval, DEFAULT_AUTO_QUERY_INTERVAL, 0, 1440),
  })

  return normalized ?? createDefaultConfig()
}

function formatTestRemaining(item: { used: number, total: number, unit?: string }) {
  const remaining = Math.max(Number(item.total) - Number(item.used), 0)
  const formatter = new Intl.NumberFormat(undefined, {
    maximumFractionDigits: remaining >= 100 ? 0 : 2,
  })
  return `${formatter.format(remaining)}${item.unit ? ` ${item.unit}` : ''}`
}

function formatTestItemLabel(item: { key: string, label?: string }) {
  return `${item.label ?? ''}`.trim() || `${item.key ?? ''}`.trim() || 'Quota'
}

function formatTestItemSummary(item: {
  key: string
  label?: string
  used: number
  total: number
  unit?: string
  active?: boolean
  isValid?: boolean
  extra?: string
  invalidMessage?: string
}) {
  const label = formatTestItemLabel(item)
  const invalidMessage = `${item.invalidMessage ?? ''}`.trim()
  const extra = `${item.extra ?? ''}`.trim()
  const summaryParts = [
    item.active === false || item.isValid === false || invalidMessage
      ? `${label}: ${invalidMessage || t('components.main.providers.quotaInactive')}`
      : `${label}: ${formatTestRemaining(item)}`,
  ]

  if (extra) {
    summaryParts.push(extra)
  }

  return summaryParts.join(' · ')
}

function validateCurrentDraft(forceEnable = false): ProviderQuotaQuerySaveValidationIssue | null {
  return validateProviderQuotaQueryConfigForSave(buildSaveConfig(forceEnable), {
    fallbackBaseUrl: props.providerApiUrl,
    fallbackApiKey: props.providerApiKey,
  })
}

function handleSelectTemplate(templateType: ProviderQuotaTemplateType) {
  const nextConfig = resetProviderQuotaQueryConfigFieldsOnTemplateSwitch(
    {
      enabled: !!draft.enabled,
      templateType: draft.templateType,
      code: draft.code ?? '',
      timeout: clampInteger(draft.timeout, DEFAULT_TIMEOUT, 2, 30),
      apiKey: `${draft.apiKey ?? ''}`,
      baseUrl: `${draft.baseUrl ?? ''}`,
      accessToken: `${draft.accessToken ?? ''}`,
      userId: `${draft.userId ?? ''}`,
      tokenPlanProvider: draft.tokenPlanProvider ?? detectedTokenPlanProvider.value ?? 'kimi',
      autoQueryInterval: clampInteger(draft.autoQueryInterval, DEFAULT_AUTO_QUERY_INTERVAL, 0, 1440),
    },
    templateType,
    {
      defaultTokenPlanProvider: detectedTokenPlanProvider.value ?? 'kimi',
    },
  )

  if (showScriptTemplate(templateType)) {
    nextConfig.code = `${nextConfig.code ?? ''}`.trim()
      ? nextConfig.code ?? ''
      : buildPresetCode(templateType)
  }

  applyDraft({
    ...createDefaultConfig(),
    ...nextConfig,
  })
}

function handleLoadPresetCode() {
  if (!showScriptTemplate(selectedTemplate.value)) return
  draft.code = resolvePresetCode(selectedTemplate.value)
  showToast(t('components.main.form.toast.providerQuotaQueryPresetLoaded'), 'success')
}

function generatePresetId(templateType: ProviderQuotaTemplateType): string {
  return `${templateType}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

function createDefaultPresetGroup(): ProviderQuotaQueryPresetGroup {
  return { items: [] }
}

function resolvePresetGroup(templateType: ProviderQuotaTemplateType): ProviderQuotaQueryPresetGroup {
  return presetGroups.value[templateType] ?? createDefaultPresetGroup()
}

function updatePresetDraftFromId(presetId: string) {
  const preset = resolvePresetGroup(selectedTemplate.value).items.find((item) => item.id === presetId)
  selectedPresetId.value = preset?.id ?? ''
  presetEditorName.value = preset?.name ?? t('components.main.form.placeholders.providerQuotaQueryPresetName')
  presetEditorText.value = preset?.code ?? buildPresetCode(selectedTemplate.value)
  presetEditorSnapshotName.value = presetEditorName.value
  presetEditorSnapshotText.value = presetEditorText.value
}

async function loadPresetGroups() {
  try {
    const settings = await fetchAppSettings()
    presetGroups.value = {
      ...(settings.provider_quota_query_presets ?? {}),
    }
  } catch (error) {
    console.error('failed to load provider quota query presets', error)
    presetGroups.value = {}
  }
}

async function openPresetEditor() {
  if (!showScriptTemplate(selectedTemplate.value)) return
  presetEditorError.value = ''
  await loadPresetGroups()
  const group = resolvePresetGroup(selectedTemplate.value)
  const preset = group.items.find((item) => item.id === group.defaultId) ?? group.items[0]
  updatePresetDraftFromId(preset?.id ?? '')
  presetEditorOpen.value = true
}

function closePresetEditor() {
  presetEditorOpen.value = false
  presetEditorError.value = ''
}

function confirmDiscardPresetEditorChanges(): boolean {
  if (!hasPresetEditorUnsavedChanges.value) return true
  return confirm(t('components.main.form.confirmProviderQuotaQueryPresetDiscard'))
}

function requestClosePresetEditor() {
  if (!confirmDiscardPresetEditorChanges()) return
  closePresetEditor()
}

function createPresetDraft() {
  if (!confirmDiscardPresetEditorChanges()) return
  selectedPresetId.value = ''
  presetEditorName.value = t('components.main.form.placeholders.providerQuotaQueryPresetName')
  presetEditorText.value = buildPresetCode(selectedTemplate.value)
  presetEditorSnapshotName.value = presetEditorName.value
  presetEditorSnapshotText.value = presetEditorText.value
  presetEditorError.value = ''
}

function selectPresetDraft(presetId: string) {
  if (selectedPresetId.value === presetId) return
  if (!confirmDiscardPresetEditorChanges()) return
  presetEditorError.value = ''
  updatePresetDraftFromId(presetId)
}

async function persistPresetGroups(nextGroups: ProviderQuotaQueryPresetGroups) {
  const settings = await fetchAppSettings()
  const savedSettings = await saveAppSettings({
    ...settings,
    provider_quota_query_preset_codes: {},
    provider_quota_query_presets: nextGroups,
  })
  presetGroups.value = {
    ...(savedSettings.provider_quota_query_presets ?? {}),
  }
}

async function handleSavePresetCode() {
  if (!showScriptTemplate(selectedTemplate.value)) return
  presetEditorError.value = ''
  presetEditorSaving.value = true
  try {
    const nextName = presetEditorName.value.trim()
    const nextCode = presetEditorText.value.trim()
    if (!nextName) {
      presetEditorError.value = t('components.main.form.errors.providerQuotaQueryPresetNameRequired')
      return
    }
    const validation = await validateProviderQuotaScriptPreset(selectedTemplate.value, nextCode)
    if (!validation.valid) {
      presetEditorError.value = validation.error || t('components.main.form.errors.providerQuotaQueryPresetInvalid')
      return
    }
    const group = resolvePresetGroup(selectedTemplate.value)
    const presetId = selectedPresetId.value || generatePresetId(selectedTemplate.value)
    const nextItem: ProviderQuotaQueryPresetEntry = {
      id: presetId,
      name: nextName,
      code: nextCode,
      updatedAt: Date.now(),
    }
    const nextItems = group.items.some((item) => item.id === presetId)
      ? group.items.map((item) => item.id === presetId ? nextItem : item)
      : [...group.items, nextItem]
    await persistPresetGroups({
      ...presetGroups.value,
      [selectedTemplate.value]: {
        defaultId: group.defaultId || presetId,
        items: nextItems,
      },
    })
    selectedPresetId.value = presetId
    presetEditorName.value = nextName
    presetEditorText.value = nextCode
    presetEditorSnapshotName.value = nextName
    presetEditorSnapshotText.value = nextCode
    showToast(t('components.main.form.toast.providerQuotaQueryPresetSaved'), 'success')
  } catch (error) {
    presetEditorError.value = error instanceof Error ? error.message : String(error)
  } finally {
    presetEditorSaving.value = false
  }
}

async function handleDeletePresetCode() {
  if (!showScriptTemplate(selectedTemplate.value)) return
  if (!selectedPresetId.value) return
  if (!confirm(t('components.main.form.confirmProviderQuotaQueryPresetDelete'))) return
  presetEditorError.value = ''
  presetEditorDeleting.value = true
  try {
    const group = resolvePresetGroup(selectedTemplate.value)
    const nextItems = group.items.filter((item) => item.id !== selectedPresetId.value)
    const nextDefaultId = group.defaultId === selectedPresetId.value ? nextItems[0]?.id ?? '' : group.defaultId
    const nextGroups = {
      ...presetGroups.value,
      [selectedTemplate.value]: {
        defaultId: nextDefaultId,
        items: nextItems,
      },
    }
    if (nextItems.length === 0) {
      delete nextGroups[selectedTemplate.value]
    }
    await persistPresetGroups(nextGroups)
    const nextPreset = nextItems.find((item) => item.id === nextDefaultId) ?? nextItems[0]
    updatePresetDraftFromId(nextPreset?.id ?? '')
    showToast(t('components.main.form.toast.providerQuotaQueryPresetDeleted'), 'success')
  } catch (error) {
    presetEditorError.value = error instanceof Error ? error.message : String(error)
  } finally {
    presetEditorDeleting.value = false
  }
}

async function handleSetDefaultPresetCode() {
  if (!showScriptTemplate(selectedTemplate.value)) return
  if (!selectedPresetId.value) return
  presetEditorError.value = ''
  presetEditorDefaulting.value = true
  try {
    const group = resolvePresetGroup(selectedTemplate.value)
    await persistPresetGroups({
      ...presetGroups.value,
      [selectedTemplate.value]: {
        defaultId: selectedPresetId.value,
        items: group.items,
      },
    })
    showToast(t('components.main.form.toast.providerQuotaQueryPresetDefaultSet'), 'success')
  } catch (error) {
    presetEditorError.value = error instanceof Error ? error.message : String(error)
  } finally {
    presetEditorDefaulting.value = false
  }
}

async function handleTest() {
  const validationIssue = validateCurrentDraft(true)
  if (validationIssue) {
    showToast(t(saveValidationMessageKeyMap[validationIssue]), 'error')
    return
  }

  testing.value = true
  try {
    const config = buildSaveConfig(true)
    const result = await queryProviderQuota(
      config,
      `${props.providerApiUrl ?? ''}`,
      `${props.providerApiKey ?? ''}`,
    )
    const resultItems = Array.isArray(result.items) ? result.items : []

    if (resultItems.length === 0) {
      showToast(
        `${t('components.main.form.toast.providerQuotaQueryTestFailed')}: ${result.error || 'No result'}`,
        'error',
      )
      return
    }

    const invalidItems = resultItems.filter((item) => (
      item
      && (
        item.active === false
        || item.isValid === false
        || !!`${item.invalidMessage ?? ''}`.trim()
      )
    ))
    const summary = resultItems
      .map((item) => formatTestItemSummary(item))
      .join('，')

    if (invalidItems.length === resultItems.length) {
      showToast(`${t('components.main.form.toast.providerQuotaQueryTestFailed')}: ${summary}`, 'error')
      return
    }

    showToast(
      `${t('components.main.form.toast.providerQuotaQueryTestSuccess')}${summary}`,
      invalidItems.length > 0 ? 'warning' : 'success',
    )
  } catch (error) {
    showToast(
      `${t('components.main.form.toast.providerQuotaQueryTestFailed')}: ${error instanceof Error ? error.message : String(error)}`,
      'error',
    )
  } finally {
    testing.value = false
  }
}

function handleSave() {
  const validationIssue = validateCurrentDraft()
  if (validationIssue) {
    showToast(t(saveValidationMessageKeyMap[validationIssue]), 'error')
    return
  }

  emit('save', buildSaveConfig())
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      resetDraft()
      void loadPresetGroups()
    } else {
      closePresetEditor()
    }
  },
)
</script>

<style scoped>
.provider-quota-query-config-modal {
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1 1 auto;
  height: 100%;
  width: 100%;
  min-height: 0;
  min-width: 0;
}

.provider-quota-query-config-modal > * {
  min-width: 0;
}

:global(.provider-quota-query-config-modal-shell) {
  height: min(860px, calc(100vh - 48px));
  max-height: calc(100vh - 48px);
}

:global(.provider-quota-query-config-modal-shell .modal-body) {
  overflow: hidden;
  min-height: 0;
}

.provider-quota-query-config-modal__content {
  display: flex;
  flex: 1 1 0;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
  max-height: 100%;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 0 4px 2px 0;
}

.provider-quota-query-config-modal__section {
  padding: 16px;
  border: 1px solid color-mix(in srgb, var(--mac-border) 88%, transparent);
  border-radius: 16px;
  background: color-mix(in srgb, var(--mac-surface-strong) 72%, var(--mac-surface));
}

.provider-quota-query-config-modal__section-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.provider-quota-query-config-modal__section-header > div,
.provider-quota-query-config-modal__heading-block {
  min-width: 0;
}

.provider-quota-query-config-modal__section-header--with-action {
  align-items: center;
}

.provider-quota-query-config-modal__section-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.provider-quota-query-config-modal__switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.provider-quota-query-config-modal__heading-block {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.provider-quota-query-config-modal__title {
  margin: 0;
  font-size: 14px;
  font-weight: 700;
  color: var(--mac-text);
}

.provider-quota-query-config-modal__hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--mac-text-secondary);
}

.provider-quota-query-config-modal__template-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  align-items: stretch;
}

.provider-quota-query-config-modal__template-grid--compact {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.provider-quota-query-config-modal__template-card {
  appearance: none;
  display: flex !important;
  flex-direction: column !important;
  align-items: flex-start !important;
  justify-content: flex-start !important;
  align-self: stretch;
  width: 100% !important;
  min-width: 0 !important;
  margin: 0 !important;
  gap: 8px !important;
  padding: 14px !important;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--mac-border) 88%, transparent);
  background: color-mix(in srgb, var(--mac-surface) 90%, transparent);
  text-align: left;
  cursor: pointer;
  box-sizing: border-box;
  white-space: normal !important;
  line-height: 1.4 !important;
  overflow: hidden;
  transition: border-color 0.18s ease, transform 0.18s ease, background 0.18s ease;
}

.provider-quota-query-config-modal__template-card--detail {
  min-height: 104px;
}

.provider-quota-query-config-modal__template-card--compact {
  min-height: 56px;
}

.provider-quota-query-config-modal__template-copy {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  min-width: 0;
}

.provider-quota-query-config-modal__template-card:hover {
  transform: translateY(-1px);
  border-color: color-mix(in srgb, var(--mac-accent) 48%, var(--mac-border));
}

.provider-quota-query-config-modal__template-card.is-selected {
  border-color: color-mix(in srgb, var(--mac-accent) 72%, var(--mac-border));
  background: color-mix(in srgb, var(--mac-accent) 10%, var(--mac-surface));
  box-shadow: 0 12px 26px rgba(15, 23, 42, 0.08);
}

.provider-quota-query-config-modal__template-name {
  display: block;
  width: 100%;
  min-width: 0;
  font-size: 13px;
  font-weight: 700;
  color: var(--mac-text);
  white-space: normal;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.provider-quota-query-config-modal__template-desc {
  display: block;
  width: 100%;
  min-width: 0;
  font-size: 12px;
  line-height: 1.55;
  color: var(--mac-text-secondary);
  white-space: normal;
  overflow-wrap: anywhere;
}

.provider-quota-query-config-modal__template-status {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 14px;
  padding: 12px 14px;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--mac-accent) 24%, var(--mac-border));
  background: color-mix(in srgb, var(--mac-accent) 7%, var(--mac-surface));
}

.provider-quota-query-config-modal__template-status.is-warning {
  border-color: color-mix(in srgb, #f59e0b 34%, var(--mac-border));
  background: color-mix(in srgb, #f59e0b 10%, var(--mac-surface));
}

.provider-quota-query-config-modal__template-status-badge {
  display: inline-flex;
  align-items: center;
  align-self: flex-start;
  max-width: 100%;
  padding: 4px 10px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--mac-accent) 12%, transparent);
  color: var(--mac-accent);
  font-size: 11px;
  font-weight: 700;
  line-height: 1.4;
  white-space: normal;
  overflow-wrap: anywhere;
}

.provider-quota-query-config-modal__template-status.is-warning .provider-quota-query-config-modal__template-status-badge {
  background: rgba(245, 158, 11, 0.14);
  color: #b45309;
}

.provider-quota-query-config-modal__template-status-text {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--mac-text-secondary);
  overflow-wrap: anywhere;
}

.provider-quota-query-config-modal__field-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 12px;
}

.provider-quota-query-config-modal__field {
  min-width: 0;
}

.provider-quota-query-config-modal__actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 12px;
  margin-top: 4px;
  border-top: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface) 94%, transparent);
  backdrop-filter: blur(6px);
  flex-shrink: 0;
}

.provider-quota-query-config-modal__actions--fixed {
  margin: 0 -24px -24px;
  padding: 12px 24px max(14px, env(safe-area-inset-bottom));
  box-shadow: 0 -16px 28px rgba(15, 23, 42, 0.08);
}

.provider-quota-query-preset-editor {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
  min-width: 0;
}

.provider-quota-query-preset-editor__layout {
  display: grid;
  grid-template-columns: minmax(180px, 240px) minmax(0, 1fr);
  gap: 14px;
  align-items: stretch;
}

.provider-quota-query-preset-editor__list,
.provider-quota-query-preset-editor__body {
  min-width: 0;
}

.provider-quota-query-preset-editor__list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  border: 1px solid color-mix(in srgb, var(--mac-border) 88%, transparent);
  border-radius: 14px;
  background: color-mix(in srgb, var(--mac-surface) 82%, transparent);
}

.provider-quota-query-preset-editor__item {
  appearance: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: transparent;
  color: var(--mac-text);
  cursor: pointer;
  text-align: left;
  transition: background 0.16s ease, border-color 0.16s ease;
}

.provider-quota-query-preset-editor__item:hover,
.provider-quota-query-preset-editor__item.is-selected {
  border-color: color-mix(in srgb, var(--mac-accent) 38%, var(--mac-border));
  background: color-mix(in srgb, var(--mac-accent) 9%, transparent);
}

.provider-quota-query-preset-editor__item-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 650;
}

.provider-quota-query-preset-editor__item-badge {
  flex-shrink: 0;
  padding: 2px 7px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--mac-accent) 12%, transparent);
  color: var(--mac-accent);
  font-size: 11px;
  font-weight: 700;
}

.provider-quota-query-preset-editor__empty {
  margin: auto 0;
  padding: 14px 8px;
  color: var(--mac-text-secondary);
  font-size: 12px;
  line-height: 1.6;
  text-align: center;
}

.provider-quota-query-preset-editor__body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.provider-quota-query-preset-editor__error {
  margin: 10px 0 0;
  color: #dc2626;
  font-size: 12px;
  line-height: 1.6;
  overflow-wrap: anywhere;
}

:global(.dark) .provider-quota-query-config-modal__section {
  background: color-mix(in srgb, rgba(255, 255, 255, 0.04) 70%, rgba(17, 24, 39, 0.92));
  border-color: rgba(255, 255, 255, 0.08);
}

:global(.dark) .provider-quota-query-config-modal__template-card {
  background: rgba(15, 23, 42, 0.64);
  border-color: rgba(255, 255, 255, 0.08);
}

:global(.dark) .provider-quota-query-config-modal__template-card.is-selected {
  background: color-mix(in srgb, rgba(59, 130, 246, 0.16) 74%, rgba(15, 23, 42, 0.92));
  box-shadow: 0 14px 34px rgba(2, 6, 23, 0.3);
}

:global(.dark) .provider-quota-query-config-modal__hint,
:global(.dark) .provider-quota-query-config-modal__template-desc {
  color: rgba(255, 255, 255, 0.62);
}

:global(.dark) .provider-quota-query-config-modal__template-status {
  background: rgba(10, 132, 255, 0.08);
  border-color: rgba(10, 132, 255, 0.22);
}

:global(.dark) .provider-quota-query-config-modal__template-status.is-warning {
  background: rgba(245, 158, 11, 0.12);
  border-color: rgba(245, 158, 11, 0.26);
}

:global(.dark) .provider-quota-query-config-modal__template-status-badge {
  background: rgba(10, 132, 255, 0.16);
  color: #7dc2ff;
}

:global(.dark) .provider-quota-query-config-modal__template-status.is-warning .provider-quota-query-config-modal__template-status-badge {
  background: rgba(245, 158, 11, 0.18);
  color: #fbbf24;
}

:global(.dark) .provider-quota-query-config-modal__template-status-text {
  color: rgba(255, 255, 255, 0.66);
}

:global(.dark) .provider-quota-query-config-modal__actions--fixed {
  box-shadow: 0 -18px 34px rgba(2, 6, 23, 0.36);
}

@media (max-width: 980px) {
  .provider-quota-query-config-modal__template-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .provider-quota-query-config-modal__switch-row,
  .provider-quota-query-config-modal__section-header--with-action {
    flex-direction: column;
    align-items: stretch;
  }

  .provider-quota-query-config-modal__section-actions {
    justify-content: stretch;
  }

  .provider-quota-query-config-modal__template-grid,
  .provider-quota-query-config-modal__template-grid--compact {
    grid-template-columns: minmax(0, 1fr);
  }

  .provider-quota-query-config-modal__actions--fixed {
    flex-wrap: wrap;
    justify-content: stretch;
  }

  .provider-quota-query-config-modal__actions--fixed > * {
    flex: 1 1 120px;
  }

  .provider-quota-query-preset-editor__layout {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
