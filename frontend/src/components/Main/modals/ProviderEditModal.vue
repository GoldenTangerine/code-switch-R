<template>
  <InlineModal
    :open="open"
    :title="modalTitle"
    :body-scrollable="false"
    :panel-width="'min(980px, 96vw)'"
    :close-on-backdrop="false"
    @close="requestClose"
  >
    <form class="vendor-form vendor-form--provider-modal" @submit.prevent="submit()">
      <div class="vendor-form__scroll-body">
        <section v-if="tabId === 'opencode'" class="opencode-preset-panel">
          <div class="opencode-preset-panel__header">
            <div>
              <h3 class="opencode-preset-panel__title">
                {{ t('components.main.form.labels.opencodePreset') }}
              </h3>
              <p class="opencode-preset-panel__hint">
                {{ t('components.main.form.hints.opencodePreset') }}
              </p>
            </div>
            <span v-if="selectedOpenCodePreset" class="opencode-preset-badge">
              {{ openCodeCategoryLabel(selectedOpenCodePreset.category) }}
            </span>
          </div>
          <Listbox
            v-model="selectedOpenCodePresetId"
            v-slot="{ open: presetSelectOpen }"
            class="w-full"
            @update:model-value="handleOpenCodePresetChange"
          >
            <div class="opencode-preset-select">
              <ListboxButton class="opencode-preset-select__button" @click="focusOpenCodePresetSearchInput">
                <span class="opencode-preset-select__label">{{ selectedOpenCodePresetLabel }}</span>
                <span v-if="selectedOpenCodePreset" class="opencode-preset-select__meta">
                  {{ openCodeCategoryLabel(selectedOpenCodePreset.category) }}
                </span>
                <svg viewBox="0 0 20 20" aria-hidden="true">
                  <path d="M6 8l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none" />
                </svg>
              </ListboxButton>
              <ListboxOptions v-if="presetSelectOpen" class="opencode-preset-select__options">
                <div class="opencode-preset-select__search">
                  <input
                    ref="openCodePresetSearchInputRef"
                    v-model="openCodePresetSearchQuery"
                    type="text"
                    class="opencode-preset-select__search-input"
                    :placeholder="t('components.main.form.placeholders.searchOpenCodePreset')"
                    @click.stop
                    @keydown.stop
                  />
                </div>
                <ListboxOption value="custom" v-slot="{ active, selected }">
                  <div :class="['opencode-preset-select__option', { active, selected }]">
                    <span class="opencode-preset-select__option-name">
                      {{ t('components.main.form.options.opencodeCustomPreset') }}
                    </span>
                    <span class="opencode-preset-select__option-meta">
                      {{ openCodeCategoryLabel('custom') }}
                    </span>
                  </div>
                </ListboxOption>
                <ListboxOption
                  v-for="entry in filteredOpenCodePresetEntries"
                  :key="entry.id"
                  :value="entry.id"
                  v-slot="{ active, selected }"
                >
                  <div :class="['opencode-preset-select__option', { active, selected }]">
                    <span class="opencode-preset-select__option-name">
                      {{ openCodePresetLabel(entry.preset) }}
                    </span>
                    <span class="opencode-preset-select__option-meta">
                      {{ openCodeCategoryLabel(entry.preset.category) }}
                      <template v-if="entry.preset.baseUrl"> · {{ entry.preset.baseUrl }}</template>
                    </span>
                  </div>
                </ListboxOption>
                <div v-if="filteredOpenCodePresetEntries.length === 0" class="opencode-preset-select__empty">
                  {{ t('components.main.form.noOpenCodePresetResults') }}
                </div>
              </ListboxOptions>
            </div>
          </Listbox>
          <div v-if="selectedOpenCodePreset" class="opencode-preset-meta">
            <span>{{ selectedOpenCodePreset.description || selectedOpenCodePreset.name }}</span>
            <a
              v-if="selectedOpenCodePreset.websiteUrl"
              :href="selectedOpenCodePreset.websiteUrl"
              target="_blank"
              rel="noreferrer"
            >
              {{ t('components.main.form.actions.openOfficialSite') }}
            </a>
          </div>
          <p class="opencode-preset-category-hint">
            {{ openCodeCategoryHint(selectedOpenCodePreset?.category) }}
          </p>
        </section>

        <section v-if="tabId === 'grokbuild'" class="opencode-preset-panel">
          <div class="opencode-preset-panel__header">
            <div>
              <h3 class="opencode-preset-panel__title">
                {{ t('components.main.form.labels.grokPreset') }}
              </h3>
              <p class="opencode-preset-panel__hint">
                {{ t('components.main.form.hints.grokPreset') }}
              </p>
            </div>
            <span v-if="selectedGrokPreset" class="opencode-preset-badge">
              {{ openCodeCategoryLabel(selectedGrokPreset.category) }}
            </span>
          </div>
          <Listbox
            v-model="selectedGrokPresetId"
            v-slot="{ open: grokPresetSelectOpen }"
            class="w-full"
            @update:model-value="handleGrokPresetChange"
          >
            <div class="opencode-preset-select">
              <ListboxButton class="opencode-preset-select__button">
                <span class="opencode-preset-select__label">{{ selectedGrokPresetLabel }}</span>
                <span v-if="selectedGrokPreset" class="opencode-preset-select__meta">
                  {{ openCodeCategoryLabel(selectedGrokPreset.category) }}
                </span>
                <svg viewBox="0 0 20 20" aria-hidden="true">
                  <path d="M6 8l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none" />
                </svg>
              </ListboxButton>
              <ListboxOptions v-if="grokPresetSelectOpen" class="opencode-preset-select__options">
                <ListboxOption value="custom" v-slot="{ active, selected }">
                  <div :class="['opencode-preset-select__option', { active, selected }]">
                    <span class="opencode-preset-select__option-name">
                      {{ t('components.main.form.options.grokCustomPreset') }}
                    </span>
                    <span class="opencode-preset-select__option-meta">
                      {{ openCodeCategoryLabel('custom') }}
                    </span>
                  </div>
                </ListboxOption>
                <ListboxOption
                  v-for="preset in grokProviderPresets"
                  :key="preset.profile || preset.name"
                  :value="preset.profile || preset.name"
                  v-slot="{ active, selected }"
                >
                  <div :class="['opencode-preset-select__option', { active, selected }]">
                    <span class="opencode-preset-select__option-name">{{ preset.name }}</span>
                    <span class="opencode-preset-select__option-meta">
                      {{ openCodeCategoryLabel(preset.category) }}
                      <template v-if="preset.baseUrl"> · {{ preset.baseUrl }}</template>
                    </span>
                  </div>
                </ListboxOption>
              </ListboxOptions>
            </div>
          </Listbox>
          <div v-if="selectedGrokPreset" class="opencode-preset-meta">
            <span>{{ selectedGrokPreset.description || selectedGrokPreset.name }}</span>
            <a
              v-if="selectedGrokPreset.websiteUrl"
              :href="selectedGrokPreset.websiteUrl"
              target="_blank"
              rel="noreferrer"
            >
              {{ t('components.main.form.actions.openOfficialSite') }}
            </a>
          </div>
          <p class="opencode-preset-category-hint">
            {{ t('components.main.form.hints.grokCategoryHint') }}
          </p>
        </section>

        <section v-if="tabId === 'claude-desktop'" class="opencode-preset-panel">
          <div class="opencode-preset-panel__header">
            <div>
              <h3 class="opencode-preset-panel__title">
                {{ t('components.main.form.labels.claudeDesktopPreset') }}
              </h3>
              <p class="opencode-preset-panel__hint">
                {{ t('components.main.form.hints.claudeDesktopPreset') }}
              </p>
            </div>
            <span v-if="selectedClaudeDesktopPreset" class="opencode-preset-badge">
              {{ openCodeCategoryLabel(selectedClaudeDesktopPreset.category) }}
            </span>
          </div>
          <Listbox
            v-model="selectedClaudeDesktopPresetId"
            v-slot="{ open: claudeDesktopPresetSelectOpen }"
            class="w-full"
            @update:model-value="handleClaudeDesktopPresetChange"
          >
            <div class="opencode-preset-select">
              <ListboxButton class="opencode-preset-select__button">
                <span class="opencode-preset-select__label">{{ selectedClaudeDesktopPresetLabel }}</span>
                <span v-if="selectedClaudeDesktopPreset" class="opencode-preset-select__meta">
                  {{ openCodeCategoryLabel(selectedClaudeDesktopPreset.category) }}
                </span>
                <svg viewBox="0 0 20 20" aria-hidden="true">
                  <path d="M6 8l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none" />
                </svg>
              </ListboxButton>
              <ListboxOptions v-if="claudeDesktopPresetSelectOpen" class="opencode-preset-select__options">
                <ListboxOption value="custom" v-slot="{ active, selected }">
                  <div :class="['opencode-preset-select__option', { active, selected }]">
                    <span class="opencode-preset-select__option-name">
                      {{ t('components.main.form.options.claudeDesktopCustomPreset') }}
                    </span>
                    <span class="opencode-preset-select__option-meta">
                      {{ openCodeCategoryLabel('custom') }}
                    </span>
                  </div>
                </ListboxOption>
                <ListboxOption
                  v-for="preset in claudeDesktopProviderPresets"
                  :key="preset.name"
                  :value="preset.name"
                  v-slot="{ active, selected }"
                >
                  <div :class="['opencode-preset-select__option', { active, selected }]">
                    <span class="opencode-preset-select__option-name">{{ preset.name }}</span>
                    <span class="opencode-preset-select__option-meta">
                      {{ openCodeCategoryLabel(preset.category) }}
                      <template v-if="preset.baseUrl"> · {{ preset.baseUrl }}</template>
                    </span>
                  </div>
                </ListboxOption>
              </ListboxOptions>
            </div>
          </Listbox>
          <div v-if="selectedClaudeDesktopPreset" class="opencode-preset-meta">
            <span>{{ selectedClaudeDesktopPreset.description || selectedClaudeDesktopPreset.name }}</span>
            <a
              v-if="selectedClaudeDesktopPreset.websiteUrl"
              :href="selectedClaudeDesktopPreset.websiteUrl"
              target="_blank"
              rel="noreferrer"
            >
              {{ t('components.main.form.actions.openOfficialSite') }}
            </a>
          </div>
          <p class="opencode-preset-category-hint">
            {{ t('components.main.form.hints.claudeDesktopCategoryHint') }}
          </p>
        </section>

        <section v-if="tabId === 'openclaw'" class="opencode-preset-panel">
          <div class="opencode-preset-panel__header">
            <div>
              <h3 class="opencode-preset-panel__title">
                {{ t('components.main.form.labels.openclawPreset') }}
              </h3>
              <p class="opencode-preset-panel__hint">
                {{ t('components.main.form.hints.openclawPreset') }}
              </p>
            </div>
            <span v-if="selectedOpenClawPreset" class="opencode-preset-badge">
              {{ openCodeCategoryLabel(selectedOpenClawPreset.category) }}
            </span>
          </div>
          <Listbox
            v-model="selectedOpenClawPresetId"
            v-slot="{ open: openClawPresetSelectOpen }"
            class="w-full"
            @update:model-value="handleOpenClawPresetChange"
          >
            <div class="opencode-preset-select">
              <ListboxButton class="opencode-preset-select__button">
                <span class="opencode-preset-select__label">{{ selectedOpenClawPresetLabel }}</span>
                <span v-if="selectedOpenClawPreset" class="opencode-preset-select__meta">
                  {{ openCodeCategoryLabel(selectedOpenClawPreset.category) }}
                </span>
                <svg viewBox="0 0 20 20" aria-hidden="true">
                  <path d="M6 8l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none" />
                </svg>
              </ListboxButton>
              <ListboxOptions v-if="openClawPresetSelectOpen" class="opencode-preset-select__options">
                <ListboxOption value="custom" v-slot="{ active, selected }">
                  <div :class="['opencode-preset-select__option', { active, selected }]">
                    <span class="opencode-preset-select__option-name">
                      {{ t('components.main.form.options.openclawCustomPreset') }}
                    </span>
                    <span class="opencode-preset-select__option-meta">
                      {{ openCodeCategoryLabel('custom') }}
                    </span>
                  </div>
                </ListboxOption>
                <ListboxOption
                  v-for="preset in openClawProviderPresets"
                  :key="preset.name"
                  :value="preset.name"
                  v-slot="{ active, selected }"
                >
                  <div :class="['opencode-preset-select__option', { active, selected }]">
                    <span class="opencode-preset-select__option-name">{{ preset.name }}</span>
                    <span class="opencode-preset-select__option-meta">
                      {{ openCodeCategoryLabel(preset.category) }}
                      <template v-if="preset.baseUrl"> · {{ preset.baseUrl }}</template>
                    </span>
                  </div>
                </ListboxOption>
              </ListboxOptions>
            </div>
          </Listbox>
          <div v-if="selectedOpenClawPreset" class="opencode-preset-meta">
            <span>{{ selectedOpenClawPreset.description || selectedOpenClawPreset.name }}</span>
            <a
              v-if="selectedOpenClawPreset.websiteUrl"
              :href="selectedOpenClawPreset.websiteUrl"
              target="_blank"
              rel="noreferrer"
            >
              {{ t('components.main.form.actions.openOfficialSite') }}
            </a>
          </div>
          <p class="opencode-preset-category-hint">
            {{ t('components.main.form.hints.openclawCategoryHint') }}
          </p>
        </section>

        <section v-if="tabId === 'hermes'" class="opencode-preset-panel">
          <div class="opencode-preset-panel__header">
            <div>
              <h3 class="opencode-preset-panel__title">
                {{ t('components.main.form.labels.hermesPreset') }}
              </h3>
              <p class="opencode-preset-panel__hint">
                {{ t('components.main.form.hints.hermesPreset') }}
              </p>
            </div>
            <span v-if="selectedHermesPreset" class="opencode-preset-badge">
              {{ openCodeCategoryLabel(selectedHermesPreset.category) }}
            </span>
          </div>
          <Listbox
            v-model="selectedHermesPresetId"
            v-slot="{ open: hermesPresetSelectOpen }"
            class="w-full"
            @update:model-value="handleHermesPresetChange"
          >
            <div class="opencode-preset-select">
              <ListboxButton class="opencode-preset-select__button">
                <span class="opencode-preset-select__label">{{ selectedHermesPresetLabel }}</span>
                <span v-if="selectedHermesPreset" class="opencode-preset-select__meta">
                  {{ openCodeCategoryLabel(selectedHermesPreset.category) }}
                </span>
                <svg viewBox="0 0 20 20" aria-hidden="true">
                  <path d="M6 8l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none" />
                </svg>
              </ListboxButton>
              <ListboxOptions v-if="hermesPresetSelectOpen" class="opencode-preset-select__options">
                <ListboxOption value="custom" v-slot="{ active, selected }">
                  <div :class="['opencode-preset-select__option', { active, selected }]">
                    <span class="opencode-preset-select__option-name">
                      {{ t('components.main.form.options.hermesCustomPreset') }}
                    </span>
                    <span class="opencode-preset-select__option-meta">
                      {{ openCodeCategoryLabel('custom') }}
                    </span>
                  </div>
                </ListboxOption>
                <ListboxOption
                  v-for="preset in hermesProviderPresets"
                  :key="preset.name"
                  :value="preset.name"
                  v-slot="{ active, selected }"
                >
                  <div :class="['opencode-preset-select__option', { active, selected }]">
                    <span class="opencode-preset-select__option-name">{{ preset.name }}</span>
                    <span class="opencode-preset-select__option-meta">
                      {{ openCodeCategoryLabel(preset.category) }}
                      <template v-if="preset.baseUrl"> · {{ preset.baseUrl }}</template>
                    </span>
                  </div>
                </ListboxOption>
              </ListboxOptions>
            </div>
          </Listbox>
          <div v-if="selectedHermesPreset" class="opencode-preset-meta">
            <span>{{ selectedHermesPreset.description || selectedHermesPreset.name }}</span>
            <a
              v-if="selectedHermesPreset.websiteUrl"
              :href="selectedHermesPreset.websiteUrl"
              target="_blank"
              rel="noreferrer"
            >
              {{ t('components.main.form.actions.openOfficialSite') }}
            </a>
          </div>
          <p class="opencode-preset-category-hint">
            {{ t('components.main.form.hints.hermesCategoryHint') }}
          </p>
        </section>

        <section v-if="tabId === 'pi'" class="opencode-preset-panel">
          <div class="opencode-preset-panel__header">
            <div>
              <h3 class="opencode-preset-panel__title">
                {{ t('components.main.form.labels.piPreset') }}
              </h3>
              <p class="opencode-preset-panel__hint">
                {{ t('components.main.form.hints.piPreset') }}
              </p>
            </div>
            <span v-if="selectedPiPreset" class="opencode-preset-badge">
              {{ openCodeCategoryLabel(selectedPiPreset.category) }}
            </span>
          </div>
          <Listbox
            v-model="selectedPiPresetId"
            v-slot="{ open: piPresetSelectOpen }"
            class="w-full"
            @update:model-value="handlePiPresetChange"
          >
            <div class="opencode-preset-select">
              <ListboxButton class="opencode-preset-select__button">
                <span class="opencode-preset-select__label">{{ selectedPiPresetLabel }}</span>
                <span v-if="selectedPiPreset" class="opencode-preset-select__meta">
                  {{ openCodeCategoryLabel(selectedPiPreset.category) }}
                </span>
                <svg viewBox="0 0 20 20" aria-hidden="true">
                  <path d="M6 8l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none" />
                </svg>
              </ListboxButton>
              <ListboxOptions v-if="piPresetSelectOpen" class="opencode-preset-select__options">
                <ListboxOption value="custom" v-slot="{ active, selected }">
                  <div :class="['opencode-preset-select__option', { active, selected }]">
                    <span class="opencode-preset-select__option-name">
                      {{ t('components.main.form.options.piCustomPreset') }}
                    </span>
                    <span class="opencode-preset-select__option-meta">
                      {{ openCodeCategoryLabel('custom') }}
                    </span>
                  </div>
                </ListboxOption>
                <ListboxOption
                  v-for="preset in piProviderPresets"
                  :key="preset.name"
                  :value="preset.name"
                  v-slot="{ active, selected }"
                >
                  <div :class="['opencode-preset-select__option', { active, selected }]">
                    <span class="opencode-preset-select__option-name">{{ preset.name }}</span>
                    <span class="opencode-preset-select__option-meta">
                      {{ openCodeCategoryLabel(preset.category) }}
                      <template v-if="preset.baseUrl"> · {{ preset.baseUrl }}</template>
                    </span>
                  </div>
                </ListboxOption>
              </ListboxOptions>
            </div>
          </Listbox>
          <div v-if="selectedPiPreset" class="opencode-preset-meta">
            <span>{{ selectedPiPreset.description || selectedPiPreset.name }}</span>
            <a
              v-if="selectedPiPreset.websiteUrl"
              :href="selectedPiPreset.websiteUrl"
              target="_blank"
              rel="noreferrer"
            >
              {{ t('components.main.form.actions.openOfficialSite') }}
            </a>
          </div>
          <p class="opencode-preset-category-hint">
            {{ t('components.main.form.hints.piCategoryHint') }}
          </p>
        </section>

        <label v-if="tabId === 'opencode'" class="form-field">
          <span class="label-row">
            {{ t('components.main.form.labels.providerKey') }}
            <span v-if="errors.providerRef" class="field-error">
              {{ errors.providerRef }}
            </span>
          </span>
          <BaseInput
            :model-value="form.providerRef || ''"
            type="text"
            :placeholder="t('components.main.form.placeholders.providerKey')"
            :disabled="isOpenCodeProviderKeyInputDisabled"
            :class="{ 'has-error': !!errors.providerRef }"
            @update:model-value="handleOpenCodeProviderKeyInput"
          />
          <span class="field-hint">
            {{ isOpenCodeProviderKeyLocked
              ? t('components.main.form.hints.providerKeyLocked')
              : t('components.main.form.hints.providerKey') }}
          </span>
          <span v-if="isLoadingOpenCodeLiveProviderIds" class="field-hint">
            {{ t('components.main.form.hints.providerKeyChecking') }}
          </span>
          <span v-else-if="openCodeLiveProviderIdsError" class="field-error">
            {{ openCodeLiveProviderIdsError }}
          </span>
          <span v-else-if="openCodeProviderKeyStatus" :class="openCodeProviderKeyStatus.className">
            {{ openCodeProviderKeyStatus.message }}
          </span>
        </label>

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
            :required="tabId !== 'opencode' && tabId !== 'grokbuild' && tabId !== 'claude-desktop' && tabId !== 'openclaw' && tabId !== 'hermes' && tabId !== 'pi' && !isManagedCodexAuthProvider"
            :disabled="isManagedCodexAuthProvider"
            :class="{ 'has-error': !!errors.apiUrl }"
          />
        </label>

        <label v-if="tabId === 'hermes'" class="form-field">
          <span>{{ t('components.main.form.labels.hermesModel') }}</span>
          <BaseInput
            v-model="hermesModelValue"
            type="text"
            :placeholder="t('components.main.form.placeholders.hermesModel')"
          />
          <span class="field-hint">{{ t('components.main.form.hints.hermesModel') }}</span>
        </label>

        <label v-if="tabId === 'pi'" class="form-field">
          <span>{{ t('components.main.form.labels.piModel') }}</span>
          <BaseInput
            v-model="piModelValue"
            type="text"
            :placeholder="t('components.main.form.placeholders.piModel')"
          />
          <span class="field-hint">{{ t('components.main.form.hints.piModel') }}</span>
        </label>

        <label v-if="tabId === 'opencode'" class="form-field">
          <span>{{ t('components.main.form.labels.opencodeNpm') }}</span>
          <select v-model="form.opencodeNpm" class="mac-select" @change="handleOpenCodeNpmChange">
            <option
              v-for="pkg in opencodeNpmPackages"
              :key="pkg.value"
              :value="pkg.value"
            >
              {{ pkg.label }}
            </option>
          </select>
          <span class="field-hint">{{ t('components.main.form.hints.opencodeNpm') }}</span>
        </label>

        <label class="form-field">
          <span>{{ t('components.main.form.labels.officialSite') }}</span>
          <BaseInput
            v-model="form.officialSite"
            type="text"
            :placeholder="t('components.main.form.placeholders.officialSite')"
          />
        </label>

        <label v-if="!isManagedCodexAuthProvider" class="form-field">
          <span class="label-row">
            {{ t('components.main.form.labels.apiKey') }}
            <a
              v-if="tabId === 'opencode' && opencodeApiKeyLink"
              :href="opencodeApiKeyLink"
              target="_blank"
              rel="noreferrer"
              class="opencode-inline-link"
            >
              {{ t('components.main.form.actions.getApiKey') }}
            </a>
          </span>
          <BaseInput
            v-model="form.apiKey"
            type="text"
            :placeholder="t('components.main.form.placeholders.apiKey')"
          />
          <span v-if="tabId === 'opencode' && openCodePartnerPromotionText" class="opencode-partner-hint">
            {{ openCodePartnerPromotionText }}
          </span>
        </label>

        <section v-if="tabId === 'opencode' && openCodeTemplateValueEntries.length > 0" class="opencode-template-panel">
          <div class="opencode-template-panel__header">
            <h3>{{ t('components.main.form.labels.opencodeTemplateValues') }}</h3>
            <span>{{ t('components.main.form.hints.opencodeTemplateValues') }}</span>
          </div>
          <label
            v-for="entry in openCodeTemplateValueEntries"
            :key="entry.key"
            class="form-field"
          >
            <span>{{ entry.config.label }}</span>
            <BaseInput
              :model-value="entry.value"
              type="text"
              :placeholder="entry.config.placeholder || entry.config.defaultValue || ''"
              @update:model-value="updateOpenCodeTemplateValue(entry.key, $event)"
            />
          </label>
        </section>

        <div v-if="tabId === 'opencode'" class="opencode-config-panel">
          <section class="opencode-editor-card">
            <div class="opencode-editor-card__header">
              <div>
                <h3 class="opencode-editor-card__title">
                  {{ t('components.main.form.labels.opencodeExtraOptions') }}
                </h3>
                <p class="opencode-editor-card__hint">
                  {{ t('components.main.form.hints.opencodeExtraOptions') }}
                </p>
              </div>
              <BaseButton type="button" variant="outline" class="opencode-editor-card__action" @click="addOpenCodeExtraOption">
                {{ t('components.main.form.actions.addOpenCodeOption') }}
              </BaseButton>
            </div>

            <p v-if="opencodeExtraOptionEntries.length === 0" class="opencode-empty-state">
              {{ t('components.main.form.hints.opencodeNoExtraOptions') }}
            </p>
            <div v-else class="opencode-kv-list">
              <div class="opencode-kv-row opencode-kv-row--head">
                <span>{{ t('components.main.form.labels.opencodeOptionKey') }}</span>
                <span>{{ t('components.main.form.labels.opencodeOptionValue') }}</span>
                <span />
              </div>
              <div v-for="entry in opencodeExtraOptionEntries" :key="entry.uiKey" class="opencode-kv-row">
                <BaseInput
                  :model-value="entry.key"
                  type="text"
                  :placeholder="t('components.main.form.placeholders.opencodeOptionKey')"
                  @update:model-value="renameOpenCodeExtraOption(entry.key, $event)"
                />
                <BaseInput
                  :model-value="entry.value"
                  type="text"
                  :placeholder="t('components.main.form.placeholders.opencodeOptionValue')"
                  @update:model-value="updateOpenCodeExtraOptionValue(entry.key, $event)"
                />
                <button
                  type="button"
                  class="opencode-row-remove"
                  :aria-label="t('components.main.form.actions.removeOpenCodeOption')"
                  @click="removeOpenCodeExtraOption(entry.key)"
                >
                  ✕
                </button>
              </div>
            </div>
          </section>

          <section class="opencode-editor-card">
            <div class="opencode-editor-card__header">
              <div>
                <h3 class="opencode-editor-card__title">
                  {{ t('components.main.form.labels.opencodeModels') }}
                </h3>
                <p class="opencode-editor-card__hint">
                  {{ t('components.main.form.hints.opencodeModels') }}
                </p>
              </div>
              <div class="opencode-editor-card__actions">
                <BaseButton
                  type="button"
                  variant="outline"
                  class="opencode-editor-card__action"
                  :disabled="isFetchingOpenCodeModels"
                  @click="fetchOpenCodeModels"
                >
                  {{ isFetchingOpenCodeModels ? t('components.main.form.actions.fetchingOpenCodeModels') : t('components.main.form.actions.fetchOpenCodeModels') }}
                </BaseButton>
                <BaseButton type="button" variant="outline" class="opencode-editor-card__action" @click="addOpenCodeModel">
                  {{ t('components.main.form.actions.addOpenCodeModel') }}
                </BaseButton>
              </div>
            </div>
            <p v-if="opencodeModelFetchError" class="field-error opencode-fetch-error">
              {{ opencodeModelFetchError }}
            </p>
            <div v-if="openCodeModelSuggestions.length > 0" class="opencode-model-suggestions">
              <span>{{ t('components.main.form.labels.opencodeModelSuggestions') }}</span>
              <button
                v-for="model in openCodeModelSuggestions.slice(0, 12)"
                :key="`${model.source}:${model.id}`"
                type="button"
                class="opencode-model-chip"
                @click="addSuggestedOpenCodeModel(model)"
              >
                {{ model.name || model.id }}
              </button>
            </div>

            <p v-if="opencodeModelEntries.length === 0" class="opencode-empty-state">
              {{ t('components.main.form.hints.opencodeNoModels') }}
            </p>
            <div v-else class="opencode-model-list">
              <div class="opencode-model-row opencode-model-row--head">
                <span />
                <span>{{ t('components.main.form.labels.opencodeModelId') }}</span>
                <span>{{ t('components.main.form.labels.opencodeModelName') }}</span>
                <span />
              </div>
              <div v-for="entry in opencodeModelEntries" :key="entry.uiKey" class="opencode-model-item">
                <div class="opencode-model-row">
                  <button
                    type="button"
                    class="opencode-row-expand"
                    :aria-expanded="isOpenCodeModelDetailOpen(entry.id)"
                    @pointerdown.stop
                    @click.stop.prevent="openOpenCodeModelDetail(entry.id)"
                  >
                    ›
                  </button>
                  <BaseInput
                    :model-value="entry.id"
                    type="text"
                    :placeholder="t('components.main.form.placeholders.opencodeModelId')"
                    @update:model-value="renameOpenCodeModel(entry.id, $event)"
                  />
                  <BaseInput
                    :model-value="entry.modelName"
                    type="text"
                    :placeholder="t('components.main.form.placeholders.opencodeModelName')"
                    @update:model-value="updateOpenCodeModelName(entry.id, $event)"
                  />
                  <button
                    type="button"
                    class="opencode-row-remove"
                    :aria-label="t('components.main.form.actions.removeOpenCodeModel')"
                    @click="removeOpenCodeModel(entry.id)"
                  >
                    ✕
                  </button>
                </div>

              </div>
            </div>
          </section>
        </div>

        <label v-if="tabId !== 'opencode'" class="form-field">
          <span>{{ t('components.main.form.labels.apiEndpoint') }}</span>
          <BaseInput
            v-model="form.apiEndpoint"
            type="text"
            :placeholder="t('components.main.form.placeholders.apiEndpoint')"
          />
          <span class="field-hint">{{ t('components.main.form.hints.apiEndpoint') }}</span>
        </label>

        <div v-if="tabId === 'claude'" class="form-field provider-advanced-field">
          <button
            type="button"
            class="advanced-toggle"
            :aria-expanded="claudeAdvancedExpanded"
            @click="claudeAdvancedExpanded = !claudeAdvancedExpanded"
          >
            <span class="advanced-toggle__icon">{{ claudeAdvancedExpanded ? '▾' : '▸' }}</span>
            <span>{{ t('components.main.form.labels.advancedOptions') }}</span>
          </button>
          <span v-if="!claudeAdvancedExpanded" class="field-hint">
            {{ t('components.main.form.hints.advancedOptions') }}
          </span>
          <div v-else class="advanced-section">
            <label class="form-field">
              <span>{{ t('components.main.form.labels.apiFormat') }}</span>
              <select v-model="form.apiFormat" class="mac-select">
                <option value="anthropic">
                  {{ t('components.main.form.labels.apiFormatAnthropic') }}
                </option>
                <option value="openai_chat">
                  {{ t('components.main.form.labels.apiFormatOpenAIChat') }}
                </option>
                <option value="openai_responses">
                  {{ t('components.main.form.labels.apiFormatOpenAIResponses') }}
                </option>
              </select>
              <span class="field-hint">{{ t('components.main.form.hints.apiFormat') }}</span>
            </label>
            <div class="form-field">
              <span>{{ t('components.main.form.labels.claudeAuthField') }}</span>
              <Listbox v-model="selectedAuthType" v-slot="{ open: authTypeOpen }">
                <div class="level-select">
                  <ListboxButton class="level-select-button">
                    <span class="level-label">{{ claudeAuthTypeLabel }}</span>
                    <svg viewBox="0 0 20 20" aria-hidden="true">
                      <path d="M6 8l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none" />
                    </svg>
                  </ListboxButton>
                  <ListboxOptions v-if="authTypeOpen" class="level-select-options">
                    <ListboxOption
                      v-for="option in claudeAuthTypeOptions"
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
                v-if="selectedAuthType === 'custom'"
                v-model="customAuthHeader"
                type="text"
                :placeholder="t('components.main.form.placeholders.customAuthHeader')"
                class="mt-2"
              />
              <span class="field-hint">{{ t('components.main.form.hints.claudeAuthField') }}</span>
            </div>
            <label v-if="showAnthropicCacheTTLField" class="form-field">
              <span>{{ t('components.main.form.labels.anthropicCacheTTL') }}</span>
              <select v-model="form.anthropicCacheTTL" class="mac-select">
                <option value="">
                  {{ t('components.main.form.options.anthropicCacheTTLDefault') }}
                </option>
                <option value="5m">
                  {{ t('components.main.form.options.anthropicCacheTTL5m') }}
                </option>
                <option value="1h">
                  {{ t('components.main.form.options.anthropicCacheTTL1h') }}
                </option>
              </select>
              <span class="field-hint">{{ t('components.main.form.hints.anthropicCacheTTL') }}</span>
            </label>
          </div>
        </div>

        <div v-if="tabId === 'opencode'" class="form-field">
          <span class="label-row">
            {{ t('components.main.form.labels.opencodeSettingsConfig') }}
            <span v-if="opencodeSettingsConfigError" class="field-error">
              {{ opencodeSettingsConfigError }}
            </span>
          </span>
          <JsonCodeEditor
            v-model="opencodeSettingsConfigText"
            :invalid="!!opencodeSettingsConfigError"
            :rows="12"
            :surface-height="'260px'"
          />
          <span class="field-hint">{{ t('components.main.form.hints.opencodeSettingsConfig') }}</span>
        </div>

        <div v-if="tabId === 'grokbuild'" class="form-field">
          <span class="label-row">
            {{ t('components.main.form.labels.grokTOML') }}
            <span v-if="grokTOMLError" class="field-error">
              {{ grokTOMLError }}
            </span>
          </span>
          <textarea
            v-model="form.configTOML"
            class="grok-toml-textarea"
            spellcheck="false"
            autocomplete="off"
          />
          <span class="field-hint">{{ t('components.main.form.hints.grokTOML') }}</span>
        </div>

        <div v-if="tabId === 'claude-desktop'" class="form-field">
          <span>{{ t('components.main.form.labels.claudeDesktopMode') }}</span>
          <select v-model="form.claudeDesktopMode" class="mac-select">
            <option value="direct">
              {{ t('components.main.form.labels.claudeDesktopModeDirect') }}
            </option>
            <option value="proxy">
              {{ t('components.main.form.labels.claudeDesktopModeProxy') }}
            </option>
          </select>
          <span class="field-hint">{{ t('components.main.form.hints.claudeDesktopMode') }}</span>
        </div>

        <div v-if="tabId === 'claude-desktop'" class="form-field">
          <section class="opencode-editor-card">
            <div class="opencode-editor-card__header">
              <div>
                <h3 class="opencode-editor-card__title">
                  {{ t('components.main.form.labels.claudeDesktopModelRoutes') }}
                </h3>
                <p class="opencode-editor-card__hint">
                  {{ t('components.main.form.hints.claudeDesktopModelRoutes') }}
                </p>
              </div>
              <BaseButton
                type="button"
                variant="outline"
                class="opencode-editor-card__action"
                @click="addClaudeDesktopModelRoute"
              >
                {{ t('components.main.form.actions.addClaudeDesktopRoute') }}
              </BaseButton>
            </div>
            <p v-if="claudeDesktopRoutesError" class="field-error">{{ claudeDesktopRoutesError }}</p>
            <p v-if="(form.claudeDesktopModelRoutes ?? []).length === 0" class="opencode-empty-state">
              {{ t('components.main.form.hints.claudeDesktopNoRoutes') }}
            </p>
            <div v-else class="claude-desktop-route-list">
              <div class="claude-desktop-route-row claude-desktop-route-row--head">
                <span>{{ t('components.main.form.labels.claudeDesktopRouteName') }}</span>
                <span>{{ t('components.main.form.labels.claudeDesktopRouteLabel') }}</span>
                <span>{{ t('components.main.form.labels.claudeDesktopRoute1M') }}</span>
                <span />
              </div>
              <div
                v-for="(route, index) in form.claudeDesktopModelRoutes"
                :key="index"
                class="claude-desktop-route-row"
              >
                <BaseInput
                  :model-value="route.name"
                  type="text"
                  :placeholder="t('components.main.form.placeholders.claudeDesktopRouteName')"
                  @update:model-value="updateClaudeDesktopRouteField(index, 'name', $event)"
                />
                <BaseInput
                  :model-value="route.labelOverride ?? ''"
                  type="text"
                  :placeholder="t('components.main.form.placeholders.claudeDesktopRouteLabel')"
                  @update:model-value="updateClaudeDesktopRouteField(index, 'labelOverride', $event)"
                />
                <div class="switch-inline">
                  <label class="mac-switch">
                    <input
                      type="checkbox"
                      :checked="route.supports1m === true"
                      @change="toggleClaudeDesktopRoute1M(index, $event)"
                    />
                    <span></span>
                  </label>
                </div>
                <button
                  type="button"
                  class="opencode-row-remove"
                  :aria-label="t('components.main.form.actions.removeClaudeDesktopRoute')"
                  @click="removeClaudeDesktopModelRoute(index)"
                >
                  ✕
                </button>
              </div>
            </div>
          </section>
        </div>

        <div v-if="tabId !== 'opencode' && tabId !== 'claude'" class="form-field">
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

        <div v-if="tabId !== 'opencode'" class="form-field">
          <span>{{ t('components.main.form.labels.providerConcurrencyLimit') }}</span>
          <BaseInput
            v-model="form.providerConcurrencyLimit"
            type="number"
            min="0"
            max="999"
            step="1"
          />
          <span class="field-hint">{{ t('components.main.form.hints.providerConcurrencyLimit') }}</span>
        </div>

        <div v-if="supportsProviderModelRouting" class="form-field">
          <ModelMappingEditor
            :key="cliConfigEditorKey"
            v-model="form.modelMapping"
            v-model:disabled-rules="form.modelMappingDisabled"
            v-model:reasoning-efforts="form.modelMappingReasoningEfforts"
            v-model:supports-one-m="form.modelMappingSupports1M"
            v-model:miss-policy="form.modelMappingMissPolicy"
            v-model:passthrough-patterns="form.modelPassthroughPatterns"
            :platform="builtinModelPlatform"
            :show-supports-one-m="tabId === 'claude' && (form.apiFormat || 'anthropic') === 'anthropic'"
            :show-claude-special-mappings="tabId === 'claude'"
            :toggle-saving="modelMappingToggleSaving"
            @toggle-rule="handleModelMappingRuleToggle"
          />
        </div>

        <div class="form-field">
          <span>{{ t('components.main.form.labels.budgetQuota') }}</span>
          <div v-if="form.budgetQuotaSettings" class="budget-quota-grid provider-budget-quota-grid">
            <div class="budget-quota-card budget-quota-card--query">
              <div class="budget-quota-card__header">
                <div class="budget-quota-card__heading">
                  <p class="budget-quota-card__title">{{ t('components.main.form.labels.providerQuotaQueryConfig') }}</p>
                  <p class="budget-quota-card__hint">{{ t('components.main.form.hints.providerQuotaQueryConfig') }}</p>
                </div>
                <span class="budget-quota-card__limit">
                  {{ providerQuotaQueryTypeLabel }}
                </span>
              </div>
              <div class="budget-quota-card__body">
                <div class="budget-quota-field">
                  <span class="budget-quota-field__label">{{ t('components.main.form.labels.providerQuotaQueryConfig') }}</span>
                  <div class="provider-quota-query-summary">
                    <strong class="provider-quota-query-summary__title">{{ providerQuotaQueryTypeLabel }}</strong>
                    <span class="provider-quota-query-summary__meta">{{ providerQuotaQuerySummary }}</span>
                  </div>
                  <div class="provider-quota-query-actions">
                    <BaseButton variant="outline" type="button" @click="openProviderQuotaQueryConfigModal">
                      {{ t('components.main.form.actions.providerQuotaQueryConfigure') }}
                    </BaseButton>
                  </div>
                  <span
                    :class="[
                      'budget-quota-field__hint',
                      { 'budget-quota-field__hint--warning': providerQuotaQueryMissingCredentials },
                    ]"
                  >
                    {{ providerQuotaQueryHint }}
                  </span>
                </div>
              </div>
            </div>
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

        <div v-if="supportsProviderModelRouting" class="form-field">
          <ModelWhitelistEditor v-model="form.supportedModels" />
        </div>

        <div v-if="tabId !== 'opencode'" class="form-field">
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

        <div v-if="tabId !== 'opencode' && tabId !== 'grokbuild' && tabId !== 'claude-desktop' && tabId !== 'openclaw' && tabId !== 'hermes' && tabId !== 'pi'" class="form-field">
          <CLIConfigEditor
            :key="cliConfigEditorKey"
            ref="cliConfigEditorRef"
            :platform="tabId as CLIPlatform"
            v-model="form.cliConfig"
            :provider-name="form.name"
            :provider-config="{
              apiKey: form.apiKey,
              baseUrl: form.apiUrl,
              authType: effectiveAuthType,
              previewMode: claudeCLIConfigPreviewMode,
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

        <div v-if="tabId !== 'opencode'" class="form-field switch-field">
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

        <div v-if="tabId !== 'opencode' && form.availabilityMonitorEnabled" class="form-field switch-field">
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

        <div v-if="tabId !== 'opencode' && form.availabilityMonitorEnabled" class="form-field">
          <span class="field-hint" style="color: #6b7280;">
            💡 {{ t('components.main.form.hints.availabilityAdvancedConfig') }}
          </span>
        </div>
      </div>

      <footer class="form-actions form-actions--provider-modal">
        <BaseButton
          variant="outline"
          type="button"
          :disabled="modelMappingToggleSaving"
          @click="requestClose"
        >
          {{ t('components.main.form.actions.cancel') }}
        </BaseButton>
        <BaseButton type="submit" :disabled="modelMappingToggleSaving">
          {{ t('components.main.form.actions.save') }}
        </BaseButton>
        <BaseButton
          v-if="isEditing && tabId !== 'others' && tabId !== 'opencode' && !activeProxyState"
          type="button"
          variant="primary"
          :disabled="modelMappingToggleSaving || saveAndApplyBlockedByProvider"
          :title="saveAndApplyTooltip"
          @click="submit(true)"
        >
          {{ t('components.main.form.actions.saveAndApply') }}
        </BaseButton>
      </footer>
    </form>
  </InlineModal>

  <InlineModal
    :open="isOpenCodeModelDetailModalOpen"
    :title="openCodeModelDetailTitle"
    :panel-width="'min(720px, 94vw)'"
    :close-on-backdrop="false"
    @close="closeOpenCodeModelDetail"
  >
    <form class="opencode-model-detail-modal" @submit.prevent="saveOpenCodeModelDetail">
      <p class="field-hint">
        {{ t('components.main.form.hints.opencodeModelDetailJson') }}
      </p>
      <textarea
        v-model="opencodeModelDetailText"
        class="opencode-model-detail-textarea"
        spellcheck="false"
        autocomplete="off"
      />
      <p v-if="opencodeModelDetailError" class="field-error">
        {{ opencodeModelDetailError }}
      </p>
      <footer class="form-actions form-actions--provider-modal">
        <BaseButton variant="outline" type="button" @click="closeOpenCodeModelDetail">
          {{ t('components.main.form.actions.cancel') }}
        </BaseButton>
        <BaseButton type="submit">
          {{ t('components.main.form.actions.save') }}
        </BaseButton>
      </footer>
    </form>
  </InlineModal>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Listbox, ListboxButton, ListboxOption, ListboxOptions } from '@headlessui/vue'
import lobeIcons from '../../../icons/lobeIconMap'
import BaseButton from '../../common/BaseButton.vue'
import BaseInput from '../../common/BaseInput.vue'
import CLIConfigEditor from '../../common/CLIConfigEditor.vue'
import InlineModal from '../../common/InlineModal.vue'
import JsonCodeEditor from '../../common/JsonCodeEditor.vue'
import ModelMappingEditor from '../../common/ModelMappingEditor.vue'
import ModelWhitelistEditor from '../../common/ModelWhitelistEditor.vue'
import { AUTH_TYPE_OPTIONS, getDefaultAuthType } from '../constants'
import { cardProviderRef } from '../adapters/providerCardMappers'
import {
  buildNormalizedVendorForm,
  cloneProviderValue,
  createDefaultOpenCodeSettingsConfig,
  createDefaultVendorForm,
  createVendorFormFromCard,
  isDefaultOpenCodeModels,
  resolveProviderAuthState,
} from '../adapters/providerFormMappers'
import type { ProviderTab, VendorForm } from '../types'
import type { LogPlatform } from '../../../services/logs'
import { fetchCostByProvider, fetchCostSinceByProvider, fetchFiveHourQuotaStatusByProvider } from '../../../services/logs'
import { getOpenCodeLiveProviderIds } from '../../../services/opencode'
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
import {
  detectProviderQuotaBalanceProvider,
  hasProviderQuotaQueryMissingCredentials,
  normalizeProviderQuotaQueryConfig,
  normalizeProviderQuotaQueryType,
  providerQuotaBalanceProviderOptions,
  providerQuotaTemplateLabelKeyMap,
  providerQuotaQueryTypeLabelKeyMap,
  providerQuotaTokenPlanProviderLabelKeyMap,
  queryTypeToTemplateType,
  resolveProviderQuotaQueryType,
} from '../../../utils/providerQuotaQuery'
import type { AutomationCard } from '../../../data/cards'
import type { CLIPlatform } from '../../../services/cliConfig'
import { fetchProviderModelPricing } from '../../../services/providerModelPricing'
import { isBuiltinModelPlatform } from '../../../utils/builtinModels'
import { showToast } from '../../../utils/toast'
import {
  buildProviderIconOptionKeys,
  getProviderDisplayIconSvg,
  preloadProviderDisplayIcons,
} from '../../../utils/providerIconAssets'
import { isDirectApplyBlockedForProvider } from '../utils/providerDirectApply'
import {
  getPresetModelDefaults,
  opencodeNpmPackages,
  opencodeProviderPresets,
  OPENCODE_PRESET_MODEL_VARIANTS,
  type OpenCodeProviderPreset,
  type PresetModelVariant,
  type TemplateValueConfig,
} from '../config/opencodeProviderPresets'
import {
  buildGrokPresetConfigTOML,
  grokProviderPresets,
  syncGrokCredentialsIntoTOML,
  validateGrokConfigTOML,
  type GrokProviderPreset,
} from '../config/grokProviderPresets'
import {
  CLAUDE_DESKTOP_DEFAULT_MODEL_ROUTES,
  claudeDesktopProviderPresets,
  type ClaudeDesktopProviderPreset,
} from '../config/claudeDesktopProviderPresets'
import {
  openClawProviderPresets,
  type OpenClawProviderPreset,
} from '../config/openClawProviderPresets'
import {
  hermesProviderPresets,
  type HermesProviderPreset,
} from '../config/hermesProviderPresets'
import {
  piProviderPresets,
  type PiProviderPreset,
} from '../config/piProviderPresets'
import { useModelMappingRuleToggle } from './useModelMappingRuleToggle'

type CLIConfigEditorExposed = InstanceType<typeof CLIConfigEditor> & {
  applyPendingJsonChanges?: () => boolean | Promise<boolean>
  getCliConfigSubmitState?: () => {
    value: Record<string, any>
    persistValue: Record<string, any>
    shouldPersist: boolean
  }
}

type OpenCodeModel = {
  name?: string
  options?: Record<string, any>
  [key: string]: any
}

type OpenCodeKeyValueEntry = {
  key: string
  value: string
  uiKey: string
}

type OpenCodeModelEntry = {
  id: string
  model: OpenCodeModel
  modelName: string
  extraFieldEntries: OpenCodeKeyValueEntry[]
  optionEntries: OpenCodeKeyValueEntry[]
  uiKey: string
}

type OpenCodeFetchedModel = {
  id: string
  name?: string
  source: 'fetched' | 'preset'
}

type OpenCodeTemplateValueState = Record<string, TemplateValueConfig>

const OPENCODE_DEFAULT_NPM = '@ai-sdk/openai-compatible'
const OPENCODE_KNOWN_OPTION_KEYS = new Set([
  'baseURL',
  'baseUrl',
  'url',
  'apiKey',
  'api_key',
  'APIKey',
  'headers',
])
const OPENCODE_MODEL_RESERVED_KEYS = new Set(['name', 'limit', 'options'])

const props = defineProps<{
  open: boolean
  tabId: ProviderTab
  card: AutomationCard | null
  cards: AutomationCard[]
  activeProxyState: boolean
  persistModelMappingRuleEnabled?: (key: string, enabled: boolean) => Promise<void>
}>()

const emit = defineEmits<{
  close: []
  submit: [form: VendorForm]
  'submit-and-apply': [form: VendorForm]
  'open-provider-quota-query-config': [payload: {
    modelValue: VendorForm['providerQuotaQueryConfig']
    providerApiUrl: string
    providerApiKey: string
  }]
}>()

const { t } = useI18n()

const iconOptions = buildProviderIconOptionKeys(Object.keys(lobeIcons))
const ICON_PRELOAD_BATCH_SIZE = 80
const defaultIconKey = iconOptions.find((iconKey) => iconKey === 'aicoding') ?? iconOptions[0] ?? 'aicoding'

const form = reactive<VendorForm>(createDefaultVendorForm(props.tabId, defaultIconKey))
const {
  isSaving: modelMappingToggleSaving,
  invalidatePending: invalidatePendingModelMappingToggle,
  toggleRule: handleModelMappingRuleToggle,
} = useModelMappingRuleToggle({
  form,
  getCard: () => props.card,
  getPersistRule: () => props.persistModelMappingRuleEnabled,
})
const cliConfigEditorRef = ref<CLIConfigEditorExposed | null>(null)
const cliConfigEditorKey = ref(0)
const errors = reactive({
  apiUrl: '',
  providerRef: '',
})
const selectedAuthType = ref<string>(getDefaultAuthType(props.tabId))
const customAuthHeader = ref('')
const effectiveAuthType = computed(() => {
  if (props.tabId === 'claude') {
    return selectedAuthType.value === 'custom'
      ? customAuthHeader.value.trim()
      : selectedAuthType.value || 'bearer'
  }
  return customAuthHeader.value.trim() || selectedAuthType.value || getDefaultAuthType(props.tabId)
})
const iconSearchQuery = ref('')
const openCodePresetSearchQuery = ref('')
const openCodePresetSearchInputRef = ref<HTMLInputElement | null>(null)
const requestBodyOverridesText = ref('{}')
const requestBodyOverridesError = ref('')
const opencodeSettingsConfigText = ref('{}')
const opencodeSettingsConfigError = ref('')
const opencodeModels = ref<Record<string, OpenCodeModel>>({})
const opencodeExtraOptions = ref<Record<string, string>>({})
const opencodeExtraOptionUiKeys = ref<Record<string, string>>({})
const opencodeModelUiKeys = ref<Record<string, string>>({})
const opencodeModelExtraFieldUiKeys = ref<Record<string, Record<string, string>>>({})
const opencodeModelOptionUiKeys = ref<Record<string, Record<string, string>>>({})
const expandedOpenCodeModelIds = ref<string[]>([])
const opencodeModelDetailId = ref('')
const opencodeModelDetailText = ref('{}')
const opencodeModelDetailError = ref('')
const selectedOpenCodePresetId = ref('custom')
const selectedGrokPresetId = ref('custom')
const grokTOMLError = ref('')
const selectedClaudeDesktopPresetId = ref('custom')
const claudeDesktopRoutesError = ref('')
const selectedOpenClawPresetId = ref('custom')
const selectedHermesPresetId = ref('custom')
const selectedPiPresetId = ref('custom')
const opencodeTemplateValues = ref<OpenCodeTemplateValueState>({})
const openCodeLiveProviderIds = ref<string[]>([])
const isLoadingOpenCodeLiveProviderIds = ref(false)
const openCodeLiveProviderIdsError = ref('')
const fetchedOpenCodeModels = ref<OpenCodeFetchedModel[]>([])
const isSyncingOpenCodeConfigText = ref(false)
const opencodeConfigTextSyncSeq = ref(0)
const isFetchingOpenCodeModels = ref(false)
const opencodeModelFetchError = ref('')
const claudeAdvancedExpanded = ref(false)
const saveAndApplyBlockedByProvider = computed(() => (
  isDirectApplyBlockedForProvider(props.tabId, {
    apiFormat: form.apiFormat,
    connectivityAuthType: effectiveAuthType.value,
    quotaAutoDisabled: form.quotaAutoDisabled,
  })
))
const saveAndApplyTooltip = computed(() => (
  saveAndApplyBlockedByProvider.value
    ? form.quotaAutoDisabled
      ? t('components.main.providers.quotaAutoDisabledHint')
      : t('components.main.directApply.requiresHostedRouting')
    : t('components.main.directApply.title')
))
const isCodexOfficialProvider = computed(() => (
  props.tabId === 'codex' && props.card?.id === 200 && form.category === 'official'
))
const isCodexOAuthProvider = computed(() => (
  props.tabId === 'codex' && `${form.authProvider || props.card?.authProvider || ''}`.trim() === 'codex_oauth'
))
const isManagedCodexAuthProvider = computed(() => isCodexOfficialProvider.value || isCodexOAuthProvider.value)
const CODEX_OAUTH_API_URL = 'https://chatgpt.com/backend-api/codex'
const normalizedProviderQuotaQueryType = computed(() => normalizeProviderQuotaQueryType(form.providerQuotaQueryType))
const normalizedProviderQuotaQueryConfig = computed(() => (
  normalizeProviderQuotaQueryConfig(form.providerQuotaQueryConfig, form.providerQuotaQueryType)
))
const providerQuotaQueryTemplateType = computed(() => (
  normalizedProviderQuotaQueryConfig.value?.templateType
    ?? queryTypeToTemplateType(form.providerQuotaQueryType)
))
const providerQuotaQueryTypeLabel = computed(() => (
  normalizedProviderQuotaQueryConfig.value?.enabled
    ? providerQuotaQueryTemplateType.value === 'token_plan'
      ? t(providerQuotaTokenPlanProviderLabelKeyMap[
          normalizedProviderQuotaQueryConfig.value?.tokenPlanProvider ?? 'kimi'
        ])
      : providerQuotaQueryTemplateType.value
        ? t(providerQuotaTemplateLabelKeyMap[providerQuotaQueryTemplateType.value])
        : t(providerQuotaQueryTypeLabelKeyMap[normalizedProviderQuotaQueryType.value])
    : t(providerQuotaQueryTypeLabelKeyMap.none)
))
const providerQuotaQueryMissingCredentials = computed(() => {
  return hasProviderQuotaQueryMissingCredentials(normalizedProviderQuotaQueryConfig.value, {
    fallbackQueryType: form.providerQuotaQueryType,
    fallbackBaseUrl: form.apiUrl,
    fallbackApiKey: form.apiKey,
  })
})
const providerQuotaQuerySummary = computed(() => {
  const config = normalizedProviderQuotaQueryConfig.value
  if (!config?.enabled) {
    return t('components.main.form.hints.providerQuotaQueryDisabled')
  }

  const summaryParts: string[] = []
  if (providerQuotaQueryTemplateType.value) {
    summaryParts.push(t(providerQuotaTemplateLabelKeyMap[providerQuotaQueryTemplateType.value]))
  }

  if (providerQuotaQueryTemplateType.value === 'token_plan') {
    summaryParts.push(t(providerQuotaTokenPlanProviderLabelKeyMap[config.tokenPlanProvider ?? 'kimi']))
  } else if (providerQuotaQueryTemplateType.value === 'balance') {
    const balanceProvider = detectProviderQuotaBalanceProvider(config.baseUrl || form.apiUrl)
    const providerLabel = providerQuotaBalanceProviderOptions.find((option) => option.value === balanceProvider)?.label
    if (providerLabel) {
      summaryParts.push(providerLabel)
    }
  }

  const interval = Number(config.autoQueryInterval ?? config.autoIntervalMinutes ?? 5)
  summaryParts.push(
    interval > 0
      ? t('components.main.form.hints.providerQuotaQueryAutoEvery', { minutes: interval })
      : t('components.main.form.labels.providerQuotaQueryManualOnly'),
  )

  return summaryParts.join(' · ')
})
const providerQuotaQueryHint = computed(() => (
  providerQuotaQueryMissingCredentials.value
    ? t('components.main.form.hints.providerQuotaQueryMissingCredentials')
    : !normalizedProviderQuotaQueryConfig.value?.enabled
      ? t('components.main.form.hints.providerQuotaQueryConfig')
      : providerQuotaQueryTemplateType.value === 'balance'
        ? t('components.main.form.hints.providerQuotaQueryTemplateBalance')
        : providerQuotaQueryTemplateType.value === 'custom'
          ? t('components.main.form.hints.providerQuotaQueryTemplateCustom')
          : providerQuotaQueryTemplateType.value === 'general'
            ? t('components.main.form.hints.providerQuotaQueryTemplateGeneral')
            : providerQuotaQueryTemplateType.value === 'newapi'
              ? t('components.main.form.hints.providerQuotaQueryTemplateNewApi')
              : providerQuotaQueryTemplateType.value === 'token_plan'
                ? t('components.main.form.hints.providerQuotaQueryTemplateTokenPlan')
                : t('components.main.form.hints.providerQuotaQueryConfig')
))

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
const claudeAuthTypeOptions = computed(() => [
  { value: 'bearer', label: t('components.main.form.options.claudeAuthToken') },
  { value: 'x-api-key', label: t('components.main.form.options.claudeAPIKey') },
  { value: 'custom', label: t('components.main.form.options.claudeCustomHeader') },
])

const isEditing = computed(() => props.card !== null)
const modalTitle = computed(() => (
  isEditing.value
    ? t('components.main.form.editTitle')
    : t('components.main.form.createTitle')
))
const authTypeLabel = computed(() => (
  authTypeOptions.find((option) => option.value === selectedAuthType.value)?.label || selectedAuthType.value
))
const claudeAuthTypeLabel = computed(() => (
  claudeAuthTypeOptions.value.find((option) => option.value === selectedAuthType.value)?.label
  || selectedAuthType.value
))
const claudeCLIConfigPreviewMode = computed<'current' | 'direct' | 'proxy' | undefined>(() => {
  if (props.tabId !== 'claude') return undefined
  if ((form.apiFormat || 'anthropic') !== 'anthropic' || selectedAuthType.value === 'custom') {
    return 'proxy'
  }
  return form.apiUrl.trim() && form.apiKey.trim() ? 'direct' : 'current'
})
const builtinModelPlatform = computed<CLIPlatform | undefined>(() => (
  isBuiltinModelPlatform(props.tabId) ? props.tabId : undefined
))
const supportsProviderModelRouting = computed(() => (
  props.tabId === 'claude' || props.tabId === 'codex' || props.tabId === 'others'
))
const filteredIconOptions = computed(() => {
  const query = iconSearchQuery.value.toLowerCase().trim()
  if (!query) return iconOptions
  return iconOptions.filter((name) => name.toLowerCase().includes(query))
})
const showAnthropicCacheTTLField = computed(() => (
  props.tabId === 'claude' && (form.apiFormat || 'anthropic') === 'anthropic'
))
const hasClaudeAdvancedValue = computed(() => (
  props.tabId === 'claude' && (
    (form.apiFormat || 'anthropic') !== 'anthropic' ||
    !!form.anthropicCacheTTL ||
    selectedAuthType.value !== 'bearer'
  )
))
const iconPreviewOptions = computed(() => {
  const preferred = iconSearchQuery.value.trim() ? 120 : ICON_PRELOAD_BATCH_SIZE
  return Array.from(new Set([form.icon, ...filteredIconOptions.value.slice(0, preferred)]))
})
const opencodeExtraOptionEntries = computed<OpenCodeKeyValueEntry[]>(() => (
  Object.entries(opencodeExtraOptions.value).map(([key, value]) => ({
    key,
    value,
    uiKey: getOpenCodeUiKey(opencodeExtraOptionUiKeys, key, 'extra-option'),
  }))
))
const opencodeModelEntries = computed<OpenCodeModelEntry[]>(() => (
  Object.entries(opencodeModels.value).map(([id, model]) => {
    const modelRecord = getOpenCodeModelRecord(model)
    return {
      id,
      model: modelRecord,
      modelName: typeof modelRecord.name === 'string' ? modelRecord.name : stringifyOpenCodeEditableValue(modelRecord.name),
      extraFieldEntries: getOpenCodeModelExtraFieldEntries(id, modelRecord),
      optionEntries: getOpenCodeModelOptionEntries(id, modelRecord),
      uiKey: getOpenCodeUiKey(opencodeModelUiKeys, id, 'model'),
    }
  })
))
const isOpenCodeModelDetailModalOpen = computed(() => !!opencodeModelDetailId.value)
const openCodeModelDetailTitle = computed(() => (
  opencodeModelDetailId.value
    ? `${t('components.main.form.labels.opencodeModelOptions')} · ${opencodeModelDetailId.value}`
    : t('components.main.form.labels.opencodeModelOptions')
))
const openCodePresetEntries = computed(() => (
  opencodeProviderPresets.map((preset, index) => ({ id: `opencode-${index}`, preset }))
))
const selectedOpenCodePreset = computed<OpenCodeProviderPreset | null>(() => (
  openCodePresetEntries.value.find((entry) => entry.id === selectedOpenCodePresetId.value)?.preset ?? null
))
const normalizeOpenCodePresetSearchText = (value: unknown) => `${value ?? ''}`
  .normalize('NFKD')
  .replace(/[\u0300-\u036f]/g, '')
  .toLowerCase()
  .replace(/\s+/g, ' ')
  .trim()
const isOpenCodePresetFuzzyMatch = (source: string, query: string) => {
  if (source.includes(query)) return true
  let sourceIndex = 0
  for (const character of query) {
    sourceIndex = source.indexOf(character, sourceIndex)
    if (sourceIndex === -1) return false
    sourceIndex += 1
  }
  return true
}
const matchesOpenCodePresetSearchQuery = (source: string, query: string) => {
  const normalizedSource = normalizeOpenCodePresetSearchText(source)
  const tokens = normalizeOpenCodePresetSearchText(query).split(' ').filter(Boolean)
  return tokens.every((token) => isOpenCodePresetFuzzyMatch(normalizedSource, token))
}
const filteredOpenCodePresetEntries = computed(() => {
  const query = openCodePresetSearchQuery.value
  if (!query) return openCodePresetEntries.value
  return openCodePresetEntries.value.filter(({ preset }) => {
    const searchableText = [
      preset.name,
      preset.description,
      preset.category,
      preset.baseUrl,
      preset.websiteUrl,
      preset.apiKeyUrl,
      preset.settingsConfig?.npm,
      openCodePresetLabel(preset),
      openCodeCategoryLabel(preset.category),
    ]
      .filter(Boolean)
      .join(' ')
    return matchesOpenCodePresetSearchQuery(searchableText, query)
  })
})
const selectedOpenCodePresetLabel = computed(() => (
  selectedOpenCodePreset.value
    ? openCodePresetLabel(selectedOpenCodePreset.value)
    : t('components.main.form.options.opencodeCustomPreset')
))
const selectedGrokPreset = computed<GrokProviderPreset | null>(() => (
  grokProviderPresets.find((preset) => (preset.profile || preset.name) === selectedGrokPresetId.value) ?? null
))
const selectedGrokPresetLabel = computed(() => (
  selectedGrokPreset.value?.name ?? t('components.main.form.options.grokCustomPreset')
))
const selectedClaudeDesktopPreset = computed<ClaudeDesktopProviderPreset | null>(() => (
  claudeDesktopProviderPresets.find((preset) => preset.name === selectedClaudeDesktopPresetId.value) ?? null
))
const selectedClaudeDesktopPresetLabel = computed(() => (
  selectedClaudeDesktopPreset.value?.name ?? t('components.main.form.options.claudeDesktopCustomPreset')
))
const selectedOpenClawPreset = computed<OpenClawProviderPreset | null>(() => (
  openClawProviderPresets.find((preset) => preset.name === selectedOpenClawPresetId.value) ?? null
))
const selectedOpenClawPresetLabel = computed(() => (
  selectedOpenClawPreset.value?.name ?? t('components.main.form.options.openclawCustomPreset')
))
const selectedHermesPreset = computed<HermesProviderPreset | null>(() => (
  hermesProviderPresets.find((preset) => preset.name === selectedHermesPresetId.value) ?? null
))
const selectedHermesPresetLabel = computed(() => (
  selectedHermesPreset.value?.name ?? t('components.main.form.options.hermesCustomPreset')
))
// Hermes 默认模型：存放在 cliConfig.model，随通用表单链路写入 HermesProvider.Model
const hermesModelValue = computed({
  get: () => `${form.cliConfig?.model ?? ''}`,
  set: (value: string) => {
    form.cliConfig = { ...(form.cliConfig || {}), model: value }
  },
})
const selectedPiPreset = computed<PiProviderPreset | null>(() => (
  piProviderPresets.find((preset) => preset.name === selectedPiPresetId.value) ?? null
))
const selectedPiPresetLabel = computed(() => (
  selectedPiPreset.value?.name ?? t('components.main.form.options.piCustomPreset')
))
// Pi 默认模型：存放在 cliConfig.model（应用侧元数据，不写入 live 条目），随通用表单链路写入 PiProvider.Model
const piModelValue = computed({
  get: () => `${form.cliConfig?.model ?? ''}`,
  set: (value: string) => {
    form.cliConfig = { ...(form.cliConfig || {}), model: value }
  },
})
const shouldShowOpenCodeApiKeyLink = computed(() => {
  if (props.tabId !== 'opencode') return false
  const category = `${form.category || selectedOpenCodePreset.value?.category || ''}`.trim()
  return ['cn_official', 'aggregator', 'third_party'].includes(category)
})
const opencodeApiKeyLink = computed(() => (
  shouldShowOpenCodeApiKeyLink.value
    ? `${form.apiKeyUrl ?? ''}`.trim()
      || selectedOpenCodePreset.value?.apiKeyUrl
      || selectedOpenCodePreset.value?.websiteUrl
      || ''
    : ''
))
const openCodePartnerPromotionText = computed(() => {
  if (!form.partnerPromotionKey) return ''
  const key = `providerForm.partnerPromotion.${form.partnerPromotionKey}`
  const fallback = t('components.main.form.hints.opencodePartnerPromotion', {
    provider: form.name || selectedOpenCodePreset.value?.name || 'OpenCode',
  })
  const translated = t(key)
  return translated === key ? fallback : translated
})
const openCodeTemplateValueEntries = computed(() => (
  Object.entries(selectedOpenCodePreset.value?.templateValues ?? {}).map(([key, config]) => ({
    key,
    config,
    value: opencodeTemplateValues.value[key]?.editorValue
      ?? opencodeTemplateValues.value[key]?.defaultValue
      ?? config.defaultValue
      ?? '',
  }))
))
const existingOpenCodeProviderKeys = computed(() => (
  new Set(
    props.cards
      .map((card) => `${card.providerRef ?? ''}`.trim())
      .filter((providerRef) => providerRef && providerRef !== `${props.card?.providerRef ?? ''}`.trim()),
  )
))
const currentOpenCodeProviderKey = computed(() => (
  isEditing.value ? `${form.providerRef ?? ''}`.trim() : normalizeNewOpenCodeProviderKey(form.providerRef || form.name)
))
const isOpenCodeProviderKeyLocked = computed(() => (
  props.tabId === 'opencode'
    && isEditing.value
    && openCodeLiveProviderIds.value.includes(`${props.card?.providerRef ?? ''}`.trim())
))
const isOpenCodeProviderKeyInputDisabled = computed(() => (
  isEditing.value || (isEditing.value && isLoadingOpenCodeLiveProviderIds.value)
))
const isOpenCodeProviderKeyDuplicate = computed(() => {
  const providerKey = currentOpenCodeProviderKey.value
  if (!providerKey) return false
  if (existingOpenCodeProviderKeys.value.has(providerKey)) return true
  if (isEditing.value && providerKey === `${props.card?.providerRef ?? ''}`.trim()) return false
  return openCodeLiveProviderIds.value.includes(providerKey)
})
const openCodeProviderKeyStatus = computed(() => {
  if (props.tabId !== 'opencode') return null
  const providerKey = currentOpenCodeProviderKey.value
  if (!providerKey) return null
  if (isEditing.value) return null
  if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(providerKey)) {
    return {
      className: 'field-error',
      message: t('components.main.form.errors.providerKeyInvalid'),
    }
  }
  if (isOpenCodeProviderKeyDuplicate.value) {
    return {
      className: 'field-error',
      message: t('components.main.form.errors.providerKeyDuplicate'),
    }
  }
  return {
    className: 'field-hint opencode-key-ok',
    message: t('components.main.form.hints.providerKeyAvailable'),
  }
})
const openCodePresetModelSuggestions = computed<OpenCodeFetchedModel[]>(() => {
  const npm = form.opencodeNpm || OPENCODE_DEFAULT_NPM
  return (OPENCODE_PRESET_MODEL_VARIANTS[npm] ?? []).map((model) => ({
    id: model.id,
    name: model.name,
    source: 'preset' as const,
  }))
})
const openCodeModelSuggestions = computed<OpenCodeFetchedModel[]>(() => {
  const configured = new Set(Object.keys(opencodeModels.value))
  const merged = [...fetchedOpenCodeModels.value, ...openCodePresetModelSuggestions.value]
  const seen = new Set<string>()
  return merged.filter((model) => {
    if (!model.id || configured.has(model.id) || seen.has(model.id)) return false
    seen.add(model.id)
    return true
  })
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

const handleProviderQuotaQueryConfigSave = (nextConfig: VendorForm['providerQuotaQueryConfig']) => {
  form.providerQuotaQueryConfig = nextConfig
  form.providerQuotaQueryType = normalizeProviderQuotaQueryType(
    resolveProviderQuotaQueryType(nextConfig, form.providerQuotaQueryType),
  )
}

const openProviderQuotaQueryConfigModal = () => {
  emit('open-provider-quota-query-config', {
    modelValue: form.providerQuotaQueryConfig
      ? { ...form.providerQuotaQueryConfig }
      : undefined,
    providerApiUrl: `${form.apiUrl ?? ''}`,
    providerApiKey: `${form.apiKey ?? ''}`,
  })
}

const loadOpenCodeLiveProviderIds = async () => {
  if (props.tabId !== 'opencode') return
  isLoadingOpenCodeLiveProviderIds.value = true
  openCodeLiveProviderIdsError.value = ''
  try {
    openCodeLiveProviderIds.value = await getOpenCodeLiveProviderIds()
  } catch (error) {
    openCodeLiveProviderIds.value = []
    openCodeLiveProviderIdsError.value = t('components.main.form.errors.providerKeyStatusFailed', {
      error: error instanceof Error ? error.message : String(error),
    })
  } finally {
    isLoadingOpenCodeLiveProviderIds.value = false
  }
}

const openCodeCategoryLabel = (category?: string) => {
  const normalized = `${category ?? 'custom'}`.trim() || 'custom'
  const keyMap: Record<string, string> = {
    official: 'components.main.form.options.opencodeCategoryOfficial',
    cn_official: 'components.main.form.options.opencodeCategoryCnOfficial',
    aggregator: 'components.main.form.options.opencodeCategoryAggregator',
    third_party: 'components.main.form.options.opencodeCategoryThirdParty',
    cloud_provider: 'components.main.form.options.opencodeCategoryCloudProvider',
    custom: 'components.main.form.options.opencodeCategoryCustom',
    omo: 'components.main.form.options.opencodeCategoryOmo',
    'omo-slim': 'components.main.form.options.opencodeCategoryOmoSlim',
  }
  return t(keyMap[normalized] || keyMap.custom)
}

const openCodeCategoryHint = (category?: string) => {
  const normalized = `${category ?? 'custom'}`.trim() || 'custom'
  const keyMap: Record<string, string> = {
    official: 'components.main.form.hints.opencodeCategoryOfficial',
    cn_official: 'components.main.form.hints.opencodeCategoryCnOfficial',
    aggregator: 'components.main.form.hints.opencodeCategoryAggregator',
    third_party: 'components.main.form.hints.opencodeCategoryThirdParty',
    cloud_provider: 'components.main.form.hints.opencodeCategoryCloudProvider',
    custom: 'components.main.form.hints.opencodeCategoryCustom',
    omo: 'components.main.form.hints.opencodeCategoryOmo',
    'omo-slim': 'components.main.form.hints.opencodeCategoryOmoSlim',
  }
  return t(keyMap[normalized] || keyMap.custom)
}

const openCodePresetLabel = (preset: OpenCodeProviderPreset) => (
  preset.nameKey && t(preset.nameKey) !== preset.nameKey ? t(preset.nameKey) : preset.name
)

const handleOpenCodeProviderKeyInput = (value: string) => {
  if (props.tabId !== 'opencode' || isOpenCodeProviderKeyLocked.value) return
  form.providerRef = normalizeNewOpenCodeProviderKey(value)
}

const createTemplateValueState = (preset: OpenCodeProviderPreset | null): OpenCodeTemplateValueState => {
  const values: OpenCodeTemplateValueState = {}
  Object.entries(preset?.templateValues ?? {}).forEach(([key, config]) => {
    const currentValue = key === 'apiKey'
      ? form.apiKey
      : key === 'baseURL'
        ? form.apiUrl
        : `${config.editorValue ?? config.defaultValue ?? ''}`
    values[key] = {
      ...config,
      editorValue: currentValue || config.editorValue || config.defaultValue || '',
    }
  })
  return values
}

const applyOpenCodeTemplateValues = (
  source: unknown,
  values: OpenCodeTemplateValueState = opencodeTemplateValues.value,
): any => {
  if (typeof source === 'string') {
    return Object.entries(values).reduce((current, [key, config]) => {
      const value = config.editorValue ?? config.defaultValue ?? ''
      return current.split('${' + key + '}').join(value)
    }, source)
  }
  if (Array.isArray(source)) {
    return source.map((item) => applyOpenCodeTemplateValues(item, values))
  }
  if (source && typeof source === 'object') {
    return Object.fromEntries(
      Object.entries(source as Record<string, unknown>).map(([key, value]) => [
        key,
        applyOpenCodeTemplateValues(value, values),
      ]),
    )
  }
  return source
}

const applyOpenCodePreset = (preset: OpenCodeProviderPreset) => {
  const config = cloneProviderValue(applyOpenCodeTemplateValues(preset.settingsConfig || {}))
  const options = (config.options && typeof config.options === 'object' && !Array.isArray(config.options))
    ? config.options as Record<string, any>
    : {}

  form.name = preset.name
  form.officialSite = preset.websiteUrl || ''
  form.apiKeyUrl = preset.apiKeyUrl || ''
  form.category = preset.category || ''
  form.partnerPromotionKey = preset.partnerPromotionKey || ''
  form.icon = preset.icon || 'opencode'
  form.opencodeNpm = `${config.npm ?? preset.settingsConfig?.npm ?? OPENCODE_DEFAULT_NPM}`.trim() || OPENCODE_DEFAULT_NPM
  form.apiUrl = `${options.baseURL ?? options.baseUrl ?? options.url ?? preset.baseUrl ?? ''}`
  form.apiKey = `${options.apiKey ?? options.api_key ?? options.APIKey ?? ''}`
  form.opencodeSettingsConfig = config
  syncOpenCodeSettingsConfigText()
}

const handleOpenCodePresetChange = () => {
  openCodePresetSearchQuery.value = ''
  const preset = selectedOpenCodePreset.value
  if (!preset) {
    opencodeTemplateValues.value = {}
    return
  }
  opencodeTemplateValues.value = createTemplateValueState(preset)
  applyOpenCodePreset(preset)
}

// Grok 预设应用：填充基础字段并生成完整 [model.<profile>] TOML 片段
const handleGrokPresetChange = () => {
  grokTOMLError.value = ''
  const preset = selectedGrokPreset.value
  if (!preset) return

  form.name = preset.name
  form.officialSite = preset.websiteUrl || ''
  form.apiUrl = preset.baseUrl ?? ''
  form.category = preset.category
  form.icon = preset.icon || 'grok'
  form.configTOML = buildGrokPresetConfigTOML(preset)
}

// Claude Desktop 预设应用：填充基础字段、接入模式与默认官方模型路由
const handleClaudeDesktopPresetChange = () => {
  claudeDesktopRoutesError.value = ''
  const preset = selectedClaudeDesktopPreset.value
  if (!preset) return

  form.name = preset.name
  form.officialSite = preset.websiteUrl || ''
  form.apiUrl = preset.baseUrl ?? ''
  form.category = preset.category
  form.icon = preset.icon || 'claude'
  form.claudeDesktopMode = preset.mode
  form.claudeDesktopModelRoutes = CLAUDE_DESKTOP_DEFAULT_MODEL_ROUTES.map((route) => ({ ...route }))
}

// OpenClaw 预设应用：填充基础字段（additive 模式，官方预设可留空地址）
const handleOpenClawPresetChange = () => {
  const preset = selectedOpenClawPreset.value
  if (!preset) return

  form.name = preset.name
  form.officialSite = preset.websiteUrl || ''
  form.apiUrl = preset.baseUrl ?? ''
  form.category = preset.category
  form.icon = preset.icon || 'claude'
}

// Hermes 预设应用：填充基础字段（additive 模式，官方预设可留空地址）
const handleHermesPresetChange = () => {
  const preset = selectedHermesPreset.value
  if (!preset) return

  form.name = preset.name
  form.officialSite = preset.websiteUrl || ''
  form.apiUrl = preset.baseUrl ?? ''
  form.category = preset.category
  form.icon = preset.icon || 'claude'
}

// Pi 预设应用：填充基础字段（additive 模式，官方预设可留空地址）
const handlePiPresetChange = () => {
  const preset = selectedPiPreset.value
  if (!preset) return

  form.name = preset.name
  form.officialSite = preset.websiteUrl || ''
  form.apiUrl = preset.baseUrl ?? ''
  form.category = preset.category
  form.icon = preset.icon || 'claude'
}

// Claude Desktop 模型路由行编辑：name 必填、labelOverride 可选、supports1m 开关
const updateClaudeDesktopRouteField = (index: number, field: 'name' | 'labelOverride', value: string) => {
  const route = form.claudeDesktopModelRoutes?.[index]
  if (!route) return
  if (field === 'name') {
    route.name = value
  } else {
    route.labelOverride = value
  }
  if (claudeDesktopRoutesError.value) claudeDesktopRoutesError.value = ''
}

const toggleClaudeDesktopRoute1M = (index: number, event: Event) => {
  const route = form.claudeDesktopModelRoutes?.[index]
  if (!route) return
  route.supports1m = (event.target as HTMLInputElement).checked
}

const addClaudeDesktopModelRoute = () => {
  if (!form.claudeDesktopModelRoutes) form.claudeDesktopModelRoutes = []
  form.claudeDesktopModelRoutes.push({ name: '', labelOverride: '', supports1m: false })
}

const removeClaudeDesktopModelRoute = (index: number) => {
  form.claudeDesktopModelRoutes?.splice(index, 1)
}

const focusOpenCodePresetSearchInput = () => {
  void nextTick(() => {
    void nextTick(() => {
      openCodePresetSearchInputRef.value?.focus()
      openCodePresetSearchInputRef.value?.select()
    })
  })
}

const updateOpenCodeTemplateValue = (key: string, value: string) => {
  const preset = selectedOpenCodePreset.value
  const config = preset?.templateValues?.[key]
  if (!preset || !config) return
  opencodeTemplateValues.value = {
    ...opencodeTemplateValues.value,
    [key]: {
      ...config,
      ...(opencodeTemplateValues.value[key] ?? {}),
      editorValue: value,
    },
  }
  applyOpenCodePreset(preset)
}

const resetForm = () => {
  invalidatePendingModelMappingToggle()
  errors.apiUrl = ''
  errors.providerRef = ''
  iconSearchQuery.value = ''
  requestBodyOverridesError.value = ''
  opencodeSettingsConfigError.value = ''
  opencodeModelFetchError.value = ''
  opencodeExtraOptionUiKeys.value = {}
  opencodeModelUiKeys.value = {}
  opencodeModelExtraFieldUiKeys.value = {}
  opencodeModelOptionUiKeys.value = {}
  expandedOpenCodeModelIds.value = []
  closeOpenCodeModelDetail()
  selectedOpenCodePresetId.value = 'custom'
  openCodePresetSearchQuery.value = ''
  opencodeTemplateValues.value = {}
  selectedGrokPresetId.value = 'custom'
  grokTOMLError.value = ''
  selectedClaudeDesktopPresetId.value = 'custom'
  claudeDesktopRoutesError.value = ''
  selectedOpenClawPresetId.value = 'custom'
  selectedHermesPresetId.value = 'custom'
  selectedPiPresetId.value = 'custom'
  fetchedOpenCodeModels.value = []
  cliConfigEditorKey.value += 1

  if (!props.card) {
    Object.assign(form, createDefaultVendorForm(props.tabId, defaultIconKey))
    form.budgetQuotaSettings = createDefaultBudgetQuotaSettings()
    form.budgetQuotaUsedAdjustments = createDefaultBudgetQuotaAdjustments()
    selectedAuthType.value = getDefaultAuthType(props.tabId)
    customAuthHeader.value = ''
    claudeAdvancedExpanded.value = props.tabId === 'claude' && hasClaudeAdvancedValue.value
    requestBodyOverridesText.value = formatJsonObject(form.requestBodyOverrides)
    if (props.tabId === 'opencode') {
      form.icon = 'opencode'
      form.opencodeNpm = form.opencodeNpm || '@ai-sdk/openai-compatible'
      syncOpenCodeSettingsConfigText()
      void loadOpenCodeLiveProviderIds()
    }
    if (props.tabId === 'grokbuild') {
      form.icon = 'grok'
    }
    if (props.tabId === 'claude-desktop') {
      form.icon = 'claude'
      form.claudeDesktopMode = 'direct'
      form.claudeDesktopModelRoutes = CLAUDE_DESKTOP_DEFAULT_MODEL_ROUTES.map((route) => ({ ...route }))
    }
    if (props.tabId === 'openclaw') {
      form.icon = 'claude'
      form.category = form.category || 'custom'
    }
    if (props.tabId === 'hermes') {
      form.icon = 'claude'
      form.category = form.category || 'custom'
    }
    if (props.tabId === 'pi') {
      form.icon = 'claude'
      form.category = form.category || 'custom'
    }
    void refreshBudgetQuotaUsage()
    return
  }

  Object.assign(form, createVendorFormFromCard(props.card, props.tabId))
  form.budgetQuotaSettings = normalizeBudgetQuotaSettings(props.card.budgetQuotaSettings)
  form.budgetQuotaUsedAdjustments = cloneBudgetQuotaAdjustments(props.card.budgetQuotaUsedAdjustments)
  requestBodyOverridesText.value = formatJsonObject(form.requestBodyOverrides)
  if (props.tabId === 'opencode') {
    syncOpenCodeSettingsConfigText()
    void loadOpenCodeLiveProviderIds()
  }

  const authState = resolveProviderAuthState(props.card.connectivityAuthType, props.tabId)
  selectedAuthType.value = authState.selectedAuthType
  customAuthHeader.value = authState.customAuthHeader
  if (props.tabId === 'claude' && customAuthHeader.value) {
    selectedAuthType.value = 'custom'
  }
  claudeAdvancedExpanded.value = props.tabId === 'claude' && hasClaudeAdvancedValue.value
  void refreshBudgetQuotaUsage()
}

watch(() => props.open, (open) => {
  if (open) {
    resetForm()
  } else {
    invalidatePendingModelMappingToggle()
    closeOpenCodeModelDetail()
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
  void preloadProviderDisplayIcons(icons)
}, { immediate: true })

watch(requestBodyOverridesText, () => {
  requestBodyOverridesError.value = ''
})

watch(opencodeSettingsConfigText, () => {
  opencodeSettingsConfigError.value = ''
  if (isSyncingOpenCodeConfigText.value || props.tabId !== 'opencode') return
  const parsedConfig = parseOpenCodeSettingsConfigTextObject()
  if (!parsedConfig) return
  form.opencodeSettingsConfig = cloneProviderValue(parsedConfig)
  syncOpenCodeStructuredStateFromConfig(parsedConfig, { syncBasicFields: true })
})

// Grok：TOML 内容变化时清除上一次的校验错误，避免提示滞留
watch(() => form.configTOML, () => {
  grokTOMLError.value = ''
})

watch(hasClaudeAdvancedValue, (value) => {
  if (value) {
    claudeAdvancedExpanded.value = true
  }
})

watch(() => form.apiFormat, (value) => {
  if (props.tabId === 'claude' && (value || 'anthropic') !== 'anthropic') {
    form.anthropicCacheTTL = ''
  }
})

defineExpose<{
  applyProviderQuotaQueryConfig: (nextConfig: VendorForm['providerQuotaQueryConfig']) => void
}>({
  applyProviderQuotaQueryConfig: handleProviderQuotaQueryConfigSave,
})

const resolveEffectiveAuthType = () => effectiveAuthType.value

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

const normalizeNewOpenCodeProviderKey = (value: string | undefined) => `${value ?? ''}`
  .trim()
  .toLowerCase()
  .replace(/[^a-z0-9-]+/g, '-')
  .replace(/^-+|-+$/g, '')

const isRecordValue = (value: unknown): value is Record<string, any> => (
  !!value && typeof value === 'object' && !Array.isArray(value)
)

const parseOpenCodeEditableValue = (value: string): any => {
  const trimmed = value.trim()
  if (!trimmed) return ''
  try {
    return JSON.parse(trimmed)
  } catch {
    return value
  }
}

const stringifyOpenCodeEditableValue = (value: unknown): string => {
  if (typeof value === 'string') return value
  if (value === undefined) return ''
  try {
    return JSON.stringify(value)
  } catch {
    return `${value ?? ''}`
  }
}

const parseOpenCodeSettingsConfigTextObject = (): Record<string, any> | null => {
  const raw = opencodeSettingsConfigText.value.trim()
  if (!raw) return null

  try {
    const parsed = JSON.parse(raw)
    return isRecordValue(parsed) ? parsed : null
  } catch {
    return null
  }
}

const getOpenCodeConfigOptions = (config?: Record<string, any> | null): Record<string, any> => {
  const options = config?.options
  return isRecordValue(options) ? { ...options } : {}
}

const getOpenCodeConfigModels = (config?: Record<string, any> | null): Record<string, OpenCodeModel> => {
  const models = config?.models
  return isRecordValue(models) ? cloneProviderValue(models as Record<string, OpenCodeModel>) : {}
}

const getOpenCodeModelRecord = (model: unknown): OpenCodeModel => (
  isRecordValue(model) ? model as OpenCodeModel : {}
)

const safeBuildOpenCodeEntries = (
  buildEntries: () => OpenCodeKeyValueEntry[],
  label: string,
): OpenCodeKeyValueEntry[] => {
  try {
    return buildEntries()
  } catch (error) {
    console.warn('failed to render OpenCode model editor entries', label, error)
    return []
  }
}

const toOpenCodeExtraOptions = (options: Record<string, any>): Record<string, string> => {
  const extra: Record<string, string> = {}
  Object.entries(options).forEach(([key, value]) => {
    if (OPENCODE_KNOWN_OPTION_KEYS.has(key)) return
    extra[key] = stringifyOpenCodeEditableValue(value)
  })
  return extra
}

const createOpenCodeDraftKey = (prefix: string, record: Record<string, unknown>) => {
  const seed = `${prefix}-${Date.now()}`
  if (!(seed in record)) return seed
  let index = 2
  while (`${seed}-${index}` in record) index += 1
  return `${seed}-${index}`
}

let openCodeUiKeySeed = 0

const createOpenCodeUiKey = (prefix: string) => `${prefix}-${Date.now()}-${++openCodeUiKeySeed}`

const getOpenCodeUiKey = (
  store: Ref<Record<string, string>>,
  key: string,
  prefix: string,
) => store.value[key] || `${prefix}:${key}`

const ensureOpenCodeUiKey = (
  store: Ref<Record<string, string>>,
  key: string,
  prefix: string,
) => {
  if (!store.value[key]) {
    store.value = { ...store.value, [key]: createOpenCodeUiKey(prefix) }
  }
  return store.value[key]
}

const ensureNestedOpenCodeUiKey = (
  store: Ref<Record<string, Record<string, string>>>,
  parentKey: string,
  key: string,
  prefix: string,
) => {
  const parent = store.value[parentKey] || {}
  if (!parent[key]) {
    store.value = {
      ...store.value,
      [parentKey]: { ...parent, [key]: createOpenCodeUiKey(prefix) },
    }
  }
  return store.value[parentKey]?.[key] || parent[key]
}

const getNestedOpenCodeUiKey = (
  store: Ref<Record<string, Record<string, string>>>,
  parentKey: string,
  key: string,
  prefix: string,
) => store.value[parentKey]?.[key] || `${prefix}:${parentKey}:${key}`

const reconcileOpenCodeUiKeys = (
  store: Ref<Record<string, string>>,
  keys: string[],
  prefix: string,
) => {
  const nextKeys = new Set(keys)
  const nextStore: Record<string, string> = {}
  keys.forEach((key) => {
    nextStore[key] = store.value[key] || createOpenCodeUiKey(prefix)
  })
  if (Object.keys(store.value).some((key) => !nextKeys.has(key)) || Object.keys(store.value).length !== keys.length) {
    store.value = nextStore
  }
}

const reconcileNestedOpenCodeUiKeys = (
  store: Ref<Record<string, Record<string, string>>>,
  entries: Record<string, string[]>,
  prefix: string,
) => {
  const nextStore: Record<string, Record<string, string>> = {}
  Object.entries(entries).forEach(([parentKey, keys]) => {
    const previous = store.value[parentKey] || {}
    nextStore[parentKey] = {}
    keys.forEach((key) => {
      nextStore[parentKey][key] = previous[key] || createOpenCodeUiKey(prefix)
    })
  })
  store.value = nextStore
}

const renameOpenCodeUiKey = (
  store: Ref<Record<string, string>>,
  oldKey: string,
  newKey: string,
) => {
  const uiKey = store.value[oldKey]
  if (!uiKey || oldKey === newKey) return
  const nextStore = { ...store.value }
  delete nextStore[oldKey]
  nextStore[newKey] = uiKey
  store.value = nextStore
}

const renameNestedOpenCodeUiKey = (
  store: Ref<Record<string, Record<string, string>>>,
  parentKey: string,
  oldKey: string,
  newKey: string,
) => {
  const parent = store.value[parentKey]
  const uiKey = parent?.[oldKey]
  if (!parent || !uiKey || oldKey === newKey) return
  store.value = {
    ...store.value,
    [parentKey]: Object.fromEntries(
      Object.entries(parent).map(([key, value]) => [key === oldKey ? newKey : key, value]),
    ),
  }
}

const renameOpenCodeModelScopedUiKeys = (oldModelId: string, newModelId: string) => {
  const migrate = (store: Ref<Record<string, Record<string, string>>>) => {
    const scoped = store.value[oldModelId]
    if (!scoped || oldModelId === newModelId) return
    const nextStore = { ...store.value }
    delete nextStore[oldModelId]
    nextStore[newModelId] = scoped
    store.value = nextStore
  }
  migrate(opencodeModelExtraFieldUiKeys)
  migrate(opencodeModelOptionUiKeys)
}

const removeOpenCodeUiKey = (store: Ref<Record<string, string>>, key: string) => {
  if (!store.value[key]) return
  const nextStore = { ...store.value }
  delete nextStore[key]
  store.value = nextStore
}

const removeNestedOpenCodeUiKey = (
  store: Ref<Record<string, Record<string, string>>>,
  parentKey: string,
  key: string,
) => {
  const parent = store.value[parentKey]
  if (!parent?.[key]) return
  const nextParent = { ...parent }
  delete nextParent[key]
  store.value = {
    ...store.value,
    [parentKey]: nextParent,
  }
}

const removeOpenCodeModelScopedUiKeys = (modelId: string) => {
  const removeScoped = (store: Ref<Record<string, Record<string, string>>>) => {
    if (!store.value[modelId]) return
    const nextStore = { ...store.value }
    delete nextStore[modelId]
    store.value = nextStore
  }
  removeScoped(opencodeModelExtraFieldUiKeys)
  removeScoped(opencodeModelOptionUiKeys)
}

const reconcileOpenCodeEditorUiKeys = () => {
  reconcileOpenCodeUiKeys(opencodeExtraOptionUiKeys, Object.keys(opencodeExtraOptions.value), 'extra-option')
  reconcileOpenCodeUiKeys(opencodeModelUiKeys, Object.keys(opencodeModels.value), 'model')

  const modelExtraFields: Record<string, string[]> = {}
  const modelOptions: Record<string, string[]> = {}
  Object.entries(opencodeModels.value).forEach(([modelId, model]) => {
    const modelRecord = getOpenCodeModelRecord(model)
    modelExtraFields[modelId] = Object.keys(modelRecord).filter((key) => !OPENCODE_MODEL_RESERVED_KEYS.has(key))
    modelOptions[modelId] = isRecordValue(modelRecord.options) ? Object.keys(modelRecord.options) : []
  })
  reconcileNestedOpenCodeUiKeys(opencodeModelExtraFieldUiKeys, modelExtraFields, 'model-field')
  reconcileNestedOpenCodeUiKeys(opencodeModelOptionUiKeys, modelOptions, 'model-option')
}

const renameRecordKey = <T,>(record: Record<string, T>, oldKey: string, newKey: string): Record<string, T> => {
  const normalizedKey = newKey.trim()
  if (!normalizedKey || normalizedKey === oldKey) return record
  if (!(oldKey in record)) return record
  if (normalizedKey in record && normalizedKey !== oldKey) return record

  const renamed: Record<string, T> = {}
  Object.entries(record).forEach(([key, value]) => {
    renamed[key === oldKey ? normalizedKey : key] = value
  })
  return renamed
}

const getOpenCodeModelExtraFieldEntries = (modelId: string, model: OpenCodeModel): OpenCodeKeyValueEntry[] => (
  safeBuildOpenCodeEntries(() => (
    Object.entries(getOpenCodeModelRecord(model))
      .filter(([key]) => !OPENCODE_MODEL_RESERVED_KEYS.has(key))
      .map(([key, value]) => ({
        key,
        value: stringifyOpenCodeEditableValue(value),
        uiKey: getNestedOpenCodeUiKey(opencodeModelExtraFieldUiKeys, modelId, key, 'model-field'),
      }))
  ), `model extra fields:${modelId}`)
)

const getOpenCodeModelOptionEntries = (modelId: string, model: OpenCodeModel): OpenCodeKeyValueEntry[] => {
  return safeBuildOpenCodeEntries(() => {
    const modelRecord = getOpenCodeModelRecord(model)
    const options = isRecordValue(modelRecord.options) ? modelRecord.options : {}
    return Object.entries(options).map(([key, value]) => ({
      key,
      value: stringifyOpenCodeEditableValue(value),
      uiKey: getNestedOpenCodeUiKey(opencodeModelOptionUiKeys, modelId, key, 'model-option'),
    }))
  }, `model options:${modelId}`)
}

const buildOpenCodeSettingsConfigFromStructuredState = (baseConfig?: Record<string, any> | null): Record<string, any> => {
  const source = cloneProviderValue(
    baseConfig && Object.keys(baseConfig).length > 0
      ? baseConfig
      : buildDefaultOpenCodeSettingsConfig(),
  )
  const config: Record<string, any> = {
    ...source,
    npm: `${form.opencodeNpm || source.npm || OPENCODE_DEFAULT_NPM}`.trim() || OPENCODE_DEFAULT_NPM,
    name: form.name || source.name || 'OpenCode Provider',
  }
  const options = getOpenCodeConfigOptions(source)

  delete options.baseURL
  delete options.baseUrl
  delete options.url
  delete options.apiKey
  delete options.api_key
  delete options.APIKey
  Object.keys(options).forEach((key) => {
    if (!OPENCODE_KNOWN_OPTION_KEYS.has(key)) {
      delete options[key]
    }
  })

  Object.entries(opencodeExtraOptions.value).forEach(([rawKey, rawValue]) => {
    const key = rawKey.trim()
    if (!key || OPENCODE_KNOWN_OPTION_KEYS.has(key)) return
    options[key] = parseOpenCodeEditableValue(rawValue)
  })

  const baseUrl = form.apiUrl.trim()
  const apiKey = form.apiKey.trim()
  if (baseUrl) options.baseURL = baseUrl
  if (apiKey) options.apiKey = apiKey

  if (Object.keys(options).length > 0) {
    config.options = options
  } else {
    delete config.options
  }

  config.models = cloneProviderValue(opencodeModels.value)
  return config
}

const syncOpenCodeSettingsConfigTextFromStructuredState = (baseConfig?: Record<string, any> | null) => {
  const config = buildOpenCodeSettingsConfigFromStructuredState(baseConfig ?? parseOpenCodeSettingsConfigTextObject())
  form.opencodeSettingsConfig = cloneProviderValue(config)
  const syncSeq = opencodeConfigTextSyncSeq.value + 1
  opencodeConfigTextSyncSeq.value = syncSeq
  isSyncingOpenCodeConfigText.value = true
  opencodeSettingsConfigText.value = formatJsonObject(config)
  void nextTick(() => {
    if (opencodeConfigTextSyncSeq.value === syncSeq) {
      isSyncingOpenCodeConfigText.value = false
    }
  })
}

const syncOpenCodeStructuredStateFromConfig = (
  config: Record<string, any>,
  options: { syncBasicFields?: boolean } = {},
) => {
  const normalizedNpm = `${config.npm ?? form.opencodeNpm ?? OPENCODE_DEFAULT_NPM}`.trim() || OPENCODE_DEFAULT_NPM
  form.opencodeNpm = normalizedNpm

  const providerOptions = getOpenCodeConfigOptions(config)
  if (options.syncBasicFields) {
    if (typeof config.name === 'string') form.name = config.name
    form.apiUrl = `${providerOptions.baseURL ?? providerOptions.baseUrl ?? providerOptions.url ?? ''}`
    form.apiKey = `${providerOptions.apiKey ?? providerOptions.api_key ?? providerOptions.APIKey ?? ''}`
  }

  opencodeExtraOptions.value = toOpenCodeExtraOptions(providerOptions)
  opencodeModels.value = getOpenCodeConfigModels(config)
  reconcileOpenCodeEditorUiKeys()
  const modelIds = new Set(Object.keys(opencodeModels.value))
  expandedOpenCodeModelIds.value = expandedOpenCodeModelIds.value.filter((modelId) => modelIds.has(modelId))
  if (opencodeModelDetailId.value && !modelIds.has(opencodeModelDetailId.value)) {
    closeOpenCodeModelDetail()
  }
}

const buildDefaultOpenCodeSettingsConfig = (): Record<string, any> => createDefaultOpenCodeSettingsConfig(
  form.opencodeNpm || OPENCODE_DEFAULT_NPM,
  form.name || 'OpenCode Provider',
  form.apiUrl,
  form.apiKey,
)

const syncOpenCodeSettingsConfigText = () => {
  const config = form.opencodeSettingsConfig && Object.keys(form.opencodeSettingsConfig).length > 0
    ? form.opencodeSettingsConfig
    : buildDefaultOpenCodeSettingsConfig()
  syncOpenCodeStructuredStateFromConfig(config)
  form.opencodeSettingsConfig = cloneProviderValue(config)
  const syncSeq = opencodeConfigTextSyncSeq.value + 1
  opencodeConfigTextSyncSeq.value = syncSeq
  isSyncingOpenCodeConfigText.value = true
  opencodeSettingsConfigText.value = formatJsonObject(config)
  void nextTick(() => {
    if (opencodeConfigTextSyncSeq.value === syncSeq) {
      isSyncingOpenCodeConfigText.value = false
    }
  })
}

const isOpenCodeModelExpanded = (modelId: string) => expandedOpenCodeModelIds.value.includes(modelId)

const toggleOpenCodeModelExpansion = (modelId: string) => {
  expandedOpenCodeModelIds.value = isOpenCodeModelExpanded(modelId)
    ? expandedOpenCodeModelIds.value.filter((id) => id !== modelId)
    : [...expandedOpenCodeModelIds.value, modelId]
}

const isOpenCodeModelDetailOpen = (modelId: string) => opencodeModelDetailId.value === modelId

const openOpenCodeModelDetail = (modelId: string) => {
  const model = opencodeModels.value[modelId]
  if (!model) return
  opencodeModelDetailError.value = ''
  opencodeModelDetailText.value = formatJsonObject(getOpenCodeModelRecord(model))
  opencodeModelDetailId.value = modelId
}

const closeOpenCodeModelDetail = () => {
  opencodeModelDetailId.value = ''
  opencodeModelDetailText.value = '{}'
  opencodeModelDetailError.value = ''
}

const saveOpenCodeModelDetail = () => {
  const modelId = opencodeModelDetailId.value
  if (!modelId || !(modelId in opencodeModels.value)) {
    closeOpenCodeModelDetail()
    return
  }

  try {
    const parsed = JSON.parse(opencodeModelDetailText.value.trim() || '{}')
    if (!isRecordValue(parsed)) {
      opencodeModelDetailError.value = t('components.main.form.errors.opencodeModelDetailMustBeObject')
      return
    }

    opencodeModels.value = {
      ...opencodeModels.value,
      [modelId]: cloneProviderValue(parsed as OpenCodeModel),
    }
    reconcileOpenCodeEditorUiKeys()
    syncOpenCodeSettingsConfigTextFromStructuredState()
    closeOpenCodeModelDetail()
  } catch (error) {
    opencodeModelDetailError.value = error instanceof Error
      ? error.message
      : t('components.main.form.errors.opencodeModelDetailInvalidJson')
  }
}

const handleOpenCodeNpmChange = () => {
  const baseConfig = parseOpenCodeSettingsConfigTextObject() || form.opencodeSettingsConfig || buildDefaultOpenCodeSettingsConfig()
  if (isDefaultOpenCodeModels(opencodeModels.value)) {
    const nextDefault = createDefaultOpenCodeSettingsConfig(
      form.opencodeNpm || OPENCODE_DEFAULT_NPM,
      form.name || 'OpenCode Provider',
      form.apiUrl,
      form.apiKey,
    ).models as Record<string, OpenCodeModel>
    opencodeModels.value = cloneProviderValue(nextDefault)
    reconcileOpenCodeEditorUiKeys()
  }
  syncOpenCodeSettingsConfigTextFromStructuredState(baseConfig)
}

const addOpenCodeExtraOption = () => {
  const key = createOpenCodeDraftKey('option', opencodeExtraOptions.value)
  opencodeExtraOptions.value = { ...opencodeExtraOptions.value, [key]: '' }
  ensureOpenCodeUiKey(opencodeExtraOptionUiKeys, key, 'extra-option')
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const renameOpenCodeExtraOption = (oldKey: string, newKey: string) => {
  const normalizedKey = newKey.trim()
  if (!normalizedKey || normalizedKey === oldKey) return
  const nextOptions = renameRecordKey(opencodeExtraOptions.value, oldKey, normalizedKey)
  if (nextOptions === opencodeExtraOptions.value || !(normalizedKey in nextOptions)) return
  opencodeExtraOptions.value = nextOptions
  renameOpenCodeUiKey(opencodeExtraOptionUiKeys, oldKey, normalizedKey)
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const updateOpenCodeExtraOptionValue = (key: string, value: string) => {
  opencodeExtraOptions.value = { ...opencodeExtraOptions.value, [key]: value }
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const removeOpenCodeExtraOption = (key: string) => {
  const nextOptions = { ...opencodeExtraOptions.value }
  delete nextOptions[key]
  opencodeExtraOptions.value = nextOptions
  removeOpenCodeUiKey(opencodeExtraOptionUiKeys, key)
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const addOpenCodeModel = () => {
  const id = createOpenCodeDraftKey('model', opencodeModels.value)
  opencodeModels.value = { ...opencodeModels.value, [id]: { name: '' } }
  ensureOpenCodeUiKey(opencodeModelUiKeys, id, 'model')
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const renameOpenCodeModel = (oldId: string, newId: string) => {
  const normalizedId = newId.trim()
  if (!normalizedId || normalizedId === oldId) return
  if (normalizedId in opencodeModels.value && normalizedId !== oldId) return
  const nextModels = renameRecordKey(opencodeModels.value, oldId, normalizedId)
  if (nextModels === opencodeModels.value || !(normalizedId in nextModels)) return
  opencodeModels.value = nextModels
  renameOpenCodeUiKey(opencodeModelUiKeys, oldId, normalizedId)
  renameOpenCodeModelScopedUiKeys(oldId, normalizedId)
  if (isOpenCodeModelExpanded(oldId)) {
    expandedOpenCodeModelIds.value = expandedOpenCodeModelIds.value.map((id) => (id === oldId ? normalizedId : id))
  }
  if (opencodeModelDetailId.value === oldId) {
    opencodeModelDetailId.value = normalizedId
  }
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const updateOpenCodeModelName = (modelId: string, name: string) => {
  const model = opencodeModels.value[modelId] || {}
  opencodeModels.value = {
    ...opencodeModels.value,
    [modelId]: { ...model, name },
  }
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const removeOpenCodeModel = (modelId: string) => {
  const nextModels = { ...opencodeModels.value }
  delete nextModels[modelId]
  opencodeModels.value = nextModels
  expandedOpenCodeModelIds.value = expandedOpenCodeModelIds.value.filter((id) => id !== modelId)
  if (opencodeModelDetailId.value === modelId) {
    closeOpenCodeModelDetail()
  }
  removeOpenCodeUiKey(opencodeModelUiKeys, modelId)
  removeOpenCodeModelScopedUiKeys(modelId)
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const buildOpenCodeModelFromSuggestion = (model: OpenCodeFetchedModel | PresetModelVariant): OpenCodeModel => {
  const defaults = getPresetModelDefaults(form.opencodeNpm || OPENCODE_DEFAULT_NPM, model.id)
  const modelName = 'name' in model ? model.name : undefined
  const nextModel: OpenCodeModel = {
    name: modelName || defaults?.name || model.id,
  }
  const limit: Record<string, unknown> = {}
  if (defaults?.contextLimit) limit.context = defaults.contextLimit
  if (defaults?.outputLimit) limit.output = defaults.outputLimit
  if (Object.keys(limit).length > 0) nextModel.limit = limit
  if (defaults?.modalities) nextModel.modalities = cloneProviderValue(defaults.modalities)
  if (defaults?.variants) nextModel.variants = cloneProviderValue(defaults.variants)
  if (defaults?.options) nextModel.options = cloneProviderValue(defaults.options)
  return nextModel
}

const addSuggestedOpenCodeModel = (model: OpenCodeFetchedModel) => {
  if (!model.id) return
  opencodeModels.value = {
    ...opencodeModels.value,
    [model.id]: {
      ...(opencodeModels.value[model.id] || {}),
      ...buildOpenCodeModelFromSuggestion(model),
    },
  }
  ensureOpenCodeUiKey(opencodeModelUiKeys, model.id, 'model')
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const fetchOpenCodeModels = async () => {
  opencodeModelFetchError.value = ''
  const apiUrl = form.apiUrl.trim()
  const apiKey = form.apiKey.trim()
  if (!apiUrl || !apiKey) {
    opencodeModelFetchError.value = t('components.main.form.errors.opencodeModelFetchMissingCredentials')
    return
  }

  isFetchingOpenCodeModels.value = true
  try {
    const response = await fetchProviderModelPricing({
      id: Number(form.providerRef) || 0,
      providerRef: form.providerRef || 'opencode-model-fetch',
      name: form.name || 'OpenCode Provider',
      apiUrl,
      apiKey,
      officialSite: form.officialSite || '',
      icon: form.icon || 'opencode',
      tint: '',
      accent: '',
      enabled: form.enabled,
      connectivityAuthType: '',
    }, 'opencode', 'v1/models')
    const modelIds = Array.from(new Set(
      (response.models || [])
        .map((item) => `${item.model ?? ''}`.trim())
        .filter(Boolean),
    ))
    if (modelIds.length === 0) {
      opencodeModelFetchError.value = response.fetchError?.trim() || t('components.main.form.errors.opencodeModelFetchEmpty')
      return
    }

    fetchedOpenCodeModels.value = modelIds.map((modelId) => ({
      id: modelId,
      name: modelId,
      source: 'fetched',
    }))
  } catch (error) {
    opencodeModelFetchError.value = t('components.main.form.errors.opencodeModelFetchFailed', {
      error: error instanceof Error ? error.message : String(error),
    })
  } finally {
    isFetchingOpenCodeModels.value = false
  }
}

const addOpenCodeModelExtraField = (modelId: string) => {
  const model = opencodeModels.value[modelId] || { name: '' }
  const extraFields = Object.fromEntries(
    Object.keys(model)
      .filter((key) => !OPENCODE_MODEL_RESERVED_KEYS.has(key))
      .map((key) => [key, true]),
  )
  const key = createOpenCodeDraftKey('field', extraFields)
  opencodeModels.value = {
    ...opencodeModels.value,
    [modelId]: { ...model, [key]: '' },
  }
  ensureNestedOpenCodeUiKey(opencodeModelExtraFieldUiKeys, modelId, key, 'model-field')
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const renameOpenCodeModelExtraField = (modelId: string, oldKey: string, newKey: string) => {
  const model = opencodeModels.value[modelId]
  const normalizedKey = newKey.trim()
  if (!model || !normalizedKey || normalizedKey === oldKey) return
  if (!(oldKey in model)) return
  if (OPENCODE_MODEL_RESERVED_KEYS.has(normalizedKey) || (normalizedKey in model && normalizedKey !== oldKey)) return

  const renamedModel: OpenCodeModel = {}
  Object.entries(model).forEach(([key, value]) => {
    renamedModel[key === oldKey ? normalizedKey : key] = value
  })
  opencodeModels.value = { ...opencodeModels.value, [modelId]: renamedModel }
  renameNestedOpenCodeUiKey(opencodeModelExtraFieldUiKeys, modelId, oldKey, normalizedKey)
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const updateOpenCodeModelExtraFieldValue = (modelId: string, key: string, value: string) => {
  const model = opencodeModels.value[modelId] || { name: '' }
  if (OPENCODE_MODEL_RESERVED_KEYS.has(key)) return
  opencodeModels.value = {
    ...opencodeModels.value,
    [modelId]: { ...model, [key]: parseOpenCodeEditableValue(value) },
  }
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const removeOpenCodeModelExtraField = (modelId: string, key: string) => {
  const model = opencodeModels.value[modelId]
  if (!model || OPENCODE_MODEL_RESERVED_KEYS.has(key)) return
  const nextModel = { ...model }
  delete nextModel[key]
  opencodeModels.value = { ...opencodeModels.value, [modelId]: nextModel }
  removeNestedOpenCodeUiKey(opencodeModelExtraFieldUiKeys, modelId, key)
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const addOpenCodeModelOption = (modelId: string) => {
  const model = opencodeModels.value[modelId] || { name: '' }
  const options = isRecordValue(model.options) ? model.options : {}
  const key = createOpenCodeDraftKey('option', options)
  opencodeModels.value = {
    ...opencodeModels.value,
    [modelId]: {
      ...model,
      options: { ...options, [key]: '' },
    },
  }
  ensureNestedOpenCodeUiKey(opencodeModelOptionUiKeys, modelId, key, 'model-option')
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const renameOpenCodeModelOption = (modelId: string, oldKey: string, newKey: string) => {
  const model = opencodeModels.value[modelId]
  const normalizedKey = newKey.trim()
  if (!model || !normalizedKey || normalizedKey === oldKey) return
  const options = isRecordValue(model.options) ? model.options : {}
  if (normalizedKey in options && normalizedKey !== oldKey) return
  const renamedOptions = renameRecordKey(options, oldKey, normalizedKey)
  if (renamedOptions === options || !(normalizedKey in renamedOptions)) return
  opencodeModels.value = {
    ...opencodeModels.value,
    [modelId]: {
      ...model,
      options: renamedOptions,
    },
  }
  renameNestedOpenCodeUiKey(opencodeModelOptionUiKeys, modelId, oldKey, normalizedKey)
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const updateOpenCodeModelOptionValue = (modelId: string, key: string, value: string) => {
  const model = opencodeModels.value[modelId] || { name: '' }
  const options = isRecordValue(model.options) ? model.options : {}
  opencodeModels.value = {
    ...opencodeModels.value,
    [modelId]: {
      ...model,
      options: { ...options, [key]: parseOpenCodeEditableValue(value) },
    },
  }
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const removeOpenCodeModelOption = (modelId: string, key: string) => {
  const model = opencodeModels.value[modelId]
  if (!model) return
  const options = isRecordValue(model.options) ? { ...model.options } : {}
  delete options[key]
  const nextModel: OpenCodeModel = { ...model }
  if (Object.keys(options).length > 0) {
    nextModel.options = options
  } else {
    delete nextModel.options
  }
  opencodeModels.value = { ...opencodeModels.value, [modelId]: nextModel }
  removeNestedOpenCodeUiKey(opencodeModelOptionUiKeys, modelId, key)
  syncOpenCodeSettingsConfigTextFromStructuredState()
}

const parseOpenCodeSettingsConfig = (): Record<string, any> | null => {
  opencodeSettingsConfigError.value = ''
  const raw = opencodeSettingsConfigText.value.trim()
  if (!raw) {
    const fallback = buildDefaultOpenCodeSettingsConfig()
    opencodeSettingsConfigText.value = formatJsonObject(fallback)
    return fallback
  }

  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      opencodeSettingsConfigError.value = t('components.main.form.errors.opencodeSettingsConfigMustBeObject')
      return null
    }

    const normalized = parsed as Record<string, any>
    normalized.npm = form.opencodeNpm || normalized.npm || '@ai-sdk/openai-compatible'
    normalized.name = form.name || normalized.name
    const options = normalized.options && typeof normalized.options === 'object' && !Array.isArray(normalized.options)
      ? { ...normalized.options }
      : {}
    delete options.baseURL
    delete options.baseUrl
    delete options.url
    delete options.apiKey
    delete options.api_key
    delete options.APIKey
    if (form.apiUrl) options.baseURL = form.apiUrl
    if (form.apiKey) options.apiKey = form.apiKey
    if (Object.keys(options).length > 0) {
      normalized.options = options
    } else {
      delete normalized.options
    }
    if (!normalized.models || typeof normalized.models !== 'object' || Array.isArray(normalized.models) || isDefaultOpenCodeModels(normalized.models)) {
      normalized.models = buildDefaultOpenCodeSettingsConfig().models
    }
    opencodeSettingsConfigText.value = formatJsonObject(normalized)
    return normalized
  } catch (error) {
    opencodeSettingsConfigError.value = t('components.main.form.errors.opencodeSettingsConfigInvalid', {
      error: error instanceof Error ? error.message : String(error),
    })
    return null
  }
}

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
  return getProviderDisplayIconSvg(name)
}

const requestClose = () => {
  if (modelMappingToggleSaving.value) return
  emit('close')
}

const warmupIcon = (name: string) => {
  void preloadProviderDisplayIcons([name])
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
  if (props.tabId === 'claude' && selectedAuthType.value === 'custom') {
    const headerName = customAuthHeader.value.trim()
    if (!headerName || !/^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/.test(headerName)) {
      showToast(t('components.main.form.errors.invalidCustomAuthHeader'), 'error')
      return null
    }
  }
  const cliConfigReady = await (cliConfigEditorRef.value?.applyPendingJsonChanges?.() ?? true)
  if (!cliConfigReady) return null

  let apiUrl = form.apiUrl.trim()
  if (isCodexOAuthProvider.value && !apiUrl) {
    apiUrl = CODEX_OAUTH_API_URL
  }
  errors.apiUrl = ''
  errors.providerRef = ''

  if (props.tabId === 'opencode') {
    const providerRef = isEditing.value
      ? normalizeNewOpenCodeProviderKey(form.providerRef || props.card?.providerRef || '')
      : normalizeNewOpenCodeProviderKey(form.providerRef || form.name)
    if (!providerRef) {
      errors.providerRef = t('components.main.form.errors.providerKeyRequired')
      return null
    }
    if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(providerRef)) {
      errors.providerRef = t('components.main.form.errors.providerKeyInvalid')
      return null
    }
    if (isLoadingOpenCodeLiveProviderIds.value) {
      errors.providerRef = t('components.main.form.errors.providerKeyStatusLoading')
      return null
    }
    if (isOpenCodeProviderKeyDuplicate.value) {
      errors.providerRef = t('components.main.form.errors.providerKeyDuplicate')
      return null
    }
    form.providerRef = providerRef
  }

  if (apiUrl) {
    try {
      const parsed = new URL(apiUrl)
      if (!/^https?:/.test(parsed.protocol)) throw new Error('protocol')
    } catch {
      errors.apiUrl = t('components.main.form.errors.invalidUrl')
      return null
    }
  } else if (props.tabId !== 'opencode' && props.tabId !== 'grokbuild' && props.tabId !== 'claude-desktop' && props.tabId !== 'openclaw' && props.tabId !== 'hermes' && props.tabId !== 'pi' && !isManagedCodexAuthProvider.value) {
    errors.apiUrl = t('components.main.form.errors.invalidUrl')
    return null
  }

  form.apiUrl = apiUrl
  const requestBodyOverrides = props.tabId === 'opencode'
    ? cloneProviderValue(form.requestBodyOverrides || props.card?.requestBodyOverrides || {})
    : parseRequestBodyOverrides()
  if (!requestBodyOverrides) return null

  const opencodeSettingsConfig = props.tabId === 'opencode'
    ? parseOpenCodeSettingsConfig()
    : undefined
  if (props.tabId === 'opencode' && !opencodeSettingsConfig) return null
  if (props.tabId === 'opencode' && Object.keys(opencodeModels.value).length === 0) {
    opencodeModelFetchError.value = t('components.main.form.errors.opencodeModelsRequired')
    return null
  }

  // Grok：非空 TOML 必须包含 [model.<profile>] 表，否则后端直连应用无法定位 profile
  if (props.tabId === 'grokbuild' && !validateGrokConfigTOML(`${form.configTOML ?? ''}`)) {
    grokTOMLError.value = t('components.main.form.errors.grokTOMLInvalid')
    return null
  }

  // Claude Desktop：模型路由行的模型 ID 必填
  if (props.tabId === 'claude-desktop' && (form.claudeDesktopModelRoutes ?? []).some((route) => !route.name.trim())) {
    claudeDesktopRoutesError.value = t('components.main.form.errors.claudeDesktopRouteNameRequired')
    return null
  }

  const payload = buildNormalizedVendorForm({
    form,
    tabId: props.tabId,
    defaultIconKey,
    resolveAuthType: resolveEffectiveAuthType,
  })
  payload.requestBodyOverrides = requestBodyOverrides
  if (props.tabId === 'opencode') {
    payload.providerRef = form.providerRef
    payload.opencodeNpm = form.opencodeNpm || opencodeSettingsConfig?.npm || '@ai-sdk/openai-compatible'
    payload.opencodeSettingsConfig = opencodeSettingsConfig ?? undefined
    payload.apiKeyUrl = form.apiKeyUrl || ''
    payload.category = form.category || ''
    payload.partnerPromotionKey = form.partnerPromotionKey || ''
  }
  if (props.tabId === 'grokbuild') {
    // 保存前把表单 API Key 与 baseUrl 同步进 TOML 的选中 profile，保证直连应用后 CLI 可直接鉴权与寻址
    payload.configTOML = syncGrokCredentialsIntoTOML(`${form.configTOML ?? ''}`, form.apiKey, form.apiUrl)
    payload.category = form.category || 'custom'
  }
  if (props.tabId === 'claude-desktop') {
    // 模式与路由已由 buildNormalizedVendorForm 归一化，这里仅兜底分类默认值
    payload.category = form.category || 'custom'
  }
  if (props.tabId === 'openclaw') {
    // additive 模式无特殊字段，仅兜底分类默认值
    payload.category = form.category || 'custom'
  }
  if (props.tabId === 'hermes') {
    // additive 模式：仅兜底分类默认值，并把 cliConfig.model 归一（空值移除键）
    payload.category = form.category || 'custom'
    const nextCliConfig = { ...(payload.cliConfig || {}) }
    const hermesModel = `${nextCliConfig.model ?? ''}`.trim()
    if (hermesModel) {
      nextCliConfig.model = hermesModel
    } else {
      delete nextCliConfig.model
    }
    payload.cliConfig = nextCliConfig
  }
  if (props.tabId === 'pi') {
    // additive 模式：仅兜底分类默认值，并把 cliConfig.model 归一（空值移除键，应用侧元数据不写入 live）
    payload.category = form.category || 'custom'
    const nextCliConfig = { ...(payload.cliConfig || {}) }
    const piModel = `${nextCliConfig.model ?? ''}`.trim()
    if (piModel) {
      nextCliConfig.model = piModel
    } else {
      delete nextCliConfig.model
    }
    payload.cliConfig = nextCliConfig
  }

  // 处理预算额度：仅保存 total > 0 的配置
  const qs = form.budgetQuotaSettings
  payload.budgetQuotaSettings = hasConfiguredBudgetQuotaSettings(qs)
    ? cloneBudgetQuotaSettings(qs)
    : undefined
  payload.budgetQuotaUsedAdjustments = hasConfiguredBudgetQuotaSettings(qs)
    ? cloneBudgetQuotaAdjustments(form.budgetQuotaUsedAdjustments)
    : undefined

  const cliConfigSubmitState = props.tabId === 'opencode'
    ? undefined
    : cliConfigEditorRef.value?.getCliConfigSubmitState?.()
  if (cliConfigSubmitState) {
    payload.cliConfig = cliConfigSubmitState.shouldPersist ? cliConfigSubmitState.value : {}
    payload.cliConfigPersistValue = cliConfigSubmitState.persistValue
    payload.cliConfigShouldPersist = cliConfigSubmitState.shouldPersist
  }

  return payload
}

const submit = async (applyAfterSave = false) => {
  if (modelMappingToggleSaving.value) return
  if (applyAfterSave && saveAndApplyBlockedByProvider.value) return
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
