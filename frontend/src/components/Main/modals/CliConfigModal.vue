<template>
  <BaseModal
    :open="open"
    :title="modalTitle"
    @close="$emit('close')"
  >
    <form class="vendor-form cli-tool-form" @submit.prevent="$emit('submit', buildDraft())">
      <label class="form-field">
        <span>{{ t('components.main.customCli.toolName') }}</span>
        <BaseInput
          v-model="form.name"
          type="text"
          :placeholder="t('components.main.customCli.toolNamePlaceholder')"
          required
        />
      </label>

      <div class="form-field">
        <div class="field-header">
          <span>{{ t('components.main.customCli.configFiles') }}</span>
          <button type="button" class="add-btn" @click="addConfigFile">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" fill="none" />
            </svg>
          </button>
        </div>
        <div class="config-files-list">
          <div
            v-for="(configFile, index) in form.configFiles"
            :key="configFile.id"
            class="config-file-item"
          >
            <div class="config-file-row">
              <BaseInput
                v-model="configFile.label"
                class="config-label-input"
                :placeholder="t('components.main.customCli.labelPlaceholder')"
              />
              <select v-model="configFile.format" class="config-format-select">
                <option value="json">JSON</option>
                <option value="toml">TOML</option>
                <option value="env">ENV</option>
              </select>
              <label class="primary-checkbox">
                <input type="checkbox" v-model="configFile.isPrimary" />
                <span>{{ t('components.main.customCli.primary') }}</span>
              </label>
              <button
                type="button"
                class="remove-btn"
                :disabled="form.configFiles.length <= 1"
                @click="removeConfigFile(index)"
              >
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M6 18L18 6M6 6l12 12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" fill="none" />
                </svg>
              </button>
            </div>
            <BaseInput
              v-model="configFile.path"
              class="config-path-input"
              :placeholder="t('components.main.customCli.pathPlaceholder')"
            />
          </div>
        </div>
      </div>

      <div class="form-field">
        <div class="field-header">
          <span>{{ t('components.main.customCli.proxySettings') }}</span>
          <button type="button" class="add-btn" @click="addProxyInjection">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" fill="none" />
            </svg>
          </button>
        </div>
        <div class="proxy-injection-list">
          <div
            v-for="(proxyInjection, index) in form.proxyInjection"
            :key="index"
            class="proxy-injection-item"
          >
            <div class="proxy-injection-row">
              <select v-model="proxyInjection.targetFileId" class="target-file-select">
                <option value="">{{ t('components.main.customCli.selectConfigFile') }}</option>
                <option
                  v-for="configFile in form.configFiles"
                  :key="configFile.id"
                  :value="configFile.id"
                >
                  {{ configFile.label || configFile.path || t('components.main.customCli.unnamed') }}
                </option>
              </select>
              <button
                type="button"
                class="remove-btn"
                :disabled="form.proxyInjection.length <= 1"
                @click="removeProxyInjection(index)"
              >
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M6 18L18 6M6 6l12 12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" fill="none" />
                </svg>
              </button>
            </div>
            <div class="proxy-fields-row">
              <BaseInput
                v-model="proxyInjection.baseUrlField"
                class="proxy-field-input"
                :placeholder="t('components.main.customCli.baseUrlFieldPlaceholder')"
              />
              <BaseInput
                v-model="proxyInjection.authTokenField"
                class="proxy-field-input"
                :placeholder="t('components.main.customCli.authTokenFieldPlaceholder')"
              />
            </div>
          </div>
        </div>
        <p class="field-hint">{{ t('components.main.customCli.proxyHint') }}</p>
      </div>

      <footer class="form-actions">
        <BaseButton variant="outline" type="button" @click="$emit('close')">
          {{ t('components.main.form.actions.cancel') }}
        </BaseButton>
        <BaseButton type="submit">
          {{ t('components.main.form.actions.save') }}
        </BaseButton>
      </footer>
    </form>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseButton from '../../common/BaseButton.vue'
import BaseInput from '../../common/BaseInput.vue'
import BaseModal from '../../common/BaseModal.vue'
import type { CustomCliTool } from '../../../services/customCliService'
import type { CustomCliToolDraft } from '../types'

const props = defineProps<{
  open: boolean
  tool: CustomCliTool | null
}>()

defineEmits<{
  close: []
  submit: [draft: CustomCliToolDraft]
}>()

const { t } = useI18n()

const form = reactive<CustomCliToolDraft>(createEmptyDraft())

const modalTitle = computed(() => (
  props.tool
    ? t('components.main.customCli.editTitle')
    : t('components.main.customCli.createTitle')
))

function createEmptyDraft(): CustomCliToolDraft {
  return {
    name: '',
    configFiles: [{
      id: `cfg-${Date.now()}`,
      label: t('components.main.customCli.primaryConfig'),
      path: '',
      format: 'json',
      isPrimary: true,
    }],
    proxyInjection: [{
      targetFileId: '',
      baseUrlField: '',
      authTokenField: '',
    }],
  }
}

const resetForm = () => {
  if (!props.tool) {
    Object.assign(form, createEmptyDraft())
    return
  }

  Object.assign(form, {
    name: props.tool.name,
    configFiles: props.tool.configFiles.length > 0
      ? props.tool.configFiles.map((configFile) => ({
          id: configFile.id,
          label: configFile.label,
          path: configFile.path,
          format: configFile.format,
          isPrimary: configFile.isPrimary ?? false,
        }))
      : createEmptyDraft().configFiles,
    proxyInjection: props.tool.proxyInjection && props.tool.proxyInjection.length > 0
      ? props.tool.proxyInjection.map((proxyInjection) => ({
          targetFileId: proxyInjection.targetFileId ?? '',
          baseUrlField: proxyInjection.baseUrlField ?? '',
          authTokenField: proxyInjection.authTokenField ?? '',
        }))
      : createEmptyDraft().proxyInjection,
  })
}

watch(() => props.open, (open) => {
  if (open) {
    resetForm()
  }
})

watch(() => props.tool, () => {
  if (props.open) {
    resetForm()
  }
})

const getAutoSelectedProxyTargetFileId = () => {
  if (form.configFiles.length === 1) {
    return form.configFiles[0].id
  }
  return ''
}

const addConfigFile = () => {
  form.configFiles.push({
    id: `cfg-${Date.now()}`,
    label: '',
    path: '',
    format: 'json',
    isPrimary: false,
  })
}

const removeConfigFile = (index: number) => {
  if (form.configFiles.length <= 1) return
  form.configFiles.splice(index, 1)
}

const addProxyInjection = () => {
  form.proxyInjection.push({
    targetFileId: getAutoSelectedProxyTargetFileId(),
    baseUrlField: '',
    authTokenField: '',
  })
}

const removeProxyInjection = (index: number) => {
  if (form.proxyInjection.length <= 1) return
  form.proxyInjection.splice(index, 1)
}

const buildDraft = (): CustomCliToolDraft => ({
  name: form.name,
  configFiles: form.configFiles.map((configFile) => ({
    ...configFile,
  })),
  proxyInjection: form.proxyInjection.map((proxyInjection) => ({
    ...proxyInjection,
  })),
})
</script>

<style scoped src="../styles/cli-config-modal.css"></style>
