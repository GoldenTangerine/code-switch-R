<template>
  <InlineModal
    :open="open"
    :title="t('components.logs.costDetail.title')"
    :panel-width="'min(760px, 96vw)'"
    :panel-class="modalPanelClass"
    @close="emit('close')"
  >
    <div :class="['logs-cost-modal', isDarkTheme ? 'logs-cost-modal--dark' : 'logs-cost-modal--light']">
      <div class="logs-cost-modal__glow logs-cost-modal__glow--orange" aria-hidden="true"></div>
      <div class="logs-cost-modal__glow logs-cost-modal__glow--blue" aria-hidden="true"></div>

      <section class="logs-cost-modal__hero">
        <div class="logs-cost-modal__hero-header">
          <div class="logs-cost-modal__hero-icon" aria-hidden="true">
            <svg viewBox="0 0 24 24" class="logs-cost-modal__hero-icon-svg">
              <rect x="3.5" y="6.25" width="17" height="11.5" rx="2.8" fill="none" stroke="currentColor" stroke-width="1.7" />
              <path d="M3.5 10h17" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
              <path d="M7.5 14.2h4.2" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
            </svg>
          </div>
          <div class="logs-cost-modal__hero-copy">
            <p class="logs-cost-modal__hero-title">{{ t('components.logs.costDetail.subtitle') }}</p>
            <p v-if="scopeText" class="logs-cost-modal__hero-scope">{{ scopeText }}</p>
          </div>
        </div>

        <div v-if="loading" class="logs-cost-modal__loading-block">
          <article class="logs-cost-modal__summary-card logs-cost-modal__summary-card--loading">
            <span class="logs-cost-modal__skeleton logs-cost-modal__skeleton--label"></span>
            <span class="logs-cost-modal__skeleton logs-cost-modal__skeleton--amount"></span>
            <span class="logs-cost-modal__skeleton logs-cost-modal__skeleton--meta"></span>
          </article>

          <div class="logs-cost-modal__loading-list">
            <div v-for="index in 4" :key="index" class="logs-cost-modal__loading-item">
              <span class="logs-cost-modal__skeleton logs-cost-modal__skeleton--avatar"></span>
              <div class="logs-cost-modal__loading-copy">
                <span class="logs-cost-modal__skeleton logs-cost-modal__skeleton--row-title"></span>
                <span class="logs-cost-modal__skeleton logs-cost-modal__skeleton--row-bar"></span>
              </div>
              <span class="logs-cost-modal__skeleton logs-cost-modal__skeleton--row-amount"></span>
            </div>
          </div>
        </div>

        <template v-else>
          <div v-if="error" class="logs-cost-modal__state logs-cost-modal__state--error">
            <svg viewBox="0 0 24 24" class="logs-cost-modal__state-icon" aria-hidden="true">
              <circle cx="12" cy="12" r="8.2" fill="none" stroke="currentColor" stroke-width="1.7" />
              <path d="M12 8v5.2" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
              <circle cx="12" cy="16.7" r="1" fill="currentColor" />
            </svg>
            <strong>{{ t('components.logs.costDetail.loadFailed', { error }) }}</strong>
          </div>

          <template v-else>
            <article v-if="showSummary" class="logs-cost-modal__summary-card">
              <div class="logs-cost-modal__summary-main">
                <p class="logs-cost-modal__summary-label">
                  <svg viewBox="0 0 24 24" class="logs-cost-modal__summary-label-icon" aria-hidden="true">
                    <path
                      d="M12 3.5 5.75 6.35v4.32c0 4.04 2.46 7.61 6.25 9.08 3.79-1.47 6.25-5.04 6.25-9.08V6.35L12 3.5Z"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="1.65"
                      stroke-linejoin="round"
                    />
                    <path d="m9.65 11.9 1.68 1.68 3.26-3.72" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" />
                  </svg>
                  {{ t('components.logs.costDetail.totalAmount') }}
                </p>

                <div class="logs-cost-modal__summary-value">
                  <span class="logs-cost-modal__currency">{{ totalAmountParts.symbol }}</span>
                  <span class="logs-cost-modal__amount-whole">{{ totalAmountParts.whole }}</span>
                  <span class="logs-cost-modal__amount-decimal">.{{ totalAmountParts.fraction }}</span>
                </div>

                <p class="logs-cost-modal__summary-meta">{{ providerCountLabel }}</p>
              </div>

              <div class="logs-cost-modal__summary-side">
                <span v-if="updatedAtLabel" class="logs-cost-modal__summary-chip">
                  <svg viewBox="0 0 24 24" class="logs-cost-modal__summary-chip-icon" aria-hidden="true">
                    <circle cx="12" cy="12" r="7.1" fill="none" stroke="currentColor" stroke-width="1.55" />
                    <path d="M12 8.4v4l2.7 1.8" fill="none" stroke="currentColor" stroke-width="1.55" stroke-linecap="round" stroke-linejoin="round" />
                  </svg>
                  {{ t('components.logs.costDetail.updatedAt', { time: updatedAtLabel }) }}
                </span>

                <span v-if="scopeText" class="logs-cost-modal__summary-chip logs-cost-modal__summary-chip--muted">
                  {{ scopeText }}
                </span>
              </div>
            </article>

            <div v-if="rows.length === 0" class="logs-cost-modal__state logs-cost-modal__state--empty">
              <svg viewBox="0 0 24 24" class="logs-cost-modal__state-icon" aria-hidden="true">
                <rect x="4.25" y="6" width="15.5" height="12.5" rx="2.6" fill="none" stroke="currentColor" stroke-width="1.6" />
                <path d="M4.25 10.25h15.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
                <path d="M9.4 14.35h5.2" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
              </svg>
              <strong>{{ t('components.logs.costDetail.empty') }}</strong>
            </div>

            <section v-else class="logs-cost-modal__list-section">
              <header class="logs-cost-modal__list-header">
                <span>{{ t('components.logs.costDetail.providerColumn') }}</span>
                <span>{{ t('components.logs.costDetail.currentUsageColumn') }}</span>
              </header>

              <button
                v-for="row in rows"
                :key="row.providerKey"
                type="button"
                class="logs-cost-modal__row"
                :disabled="!row.providerRef"
                :aria-label="t('components.logs.costDetail.providerSelectAria', { provider: row.provider, amount: row.amountLabel })"
                @click="handleProviderSelect(row.providerRef)"
              >
                <span class="logs-cost-modal__row-accent" aria-hidden="true"></span>

                <div class="logs-cost-modal__row-main">
                  <div class="logs-cost-modal__avatar">{{ row.initial }}</div>
                  <div class="logs-cost-modal__provider-copy">
                    <div class="logs-cost-modal__provider-name">{{ row.provider }}</div>
                    <div class="logs-cost-modal__provider-share">
                      <div class="logs-cost-modal__share-track">
                        <span class="logs-cost-modal__share-fill" :style="{ width: row.barWidth }"></span>
                      </div>
                      <span class="logs-cost-modal__share-label">
                        {{ t('components.logs.costDetail.share', { value: row.shareLabel }) }}
                      </span>
                    </div>
                  </div>
                </div>

                <div class="logs-cost-modal__row-side">
                  <div class="logs-cost-modal__row-amount">{{ row.amountLabel }}</div>
                  <div v-if="row.isHigh" class="logs-cost-modal__row-note">
                    <svg viewBox="0 0 24 24" class="logs-cost-modal__row-note-icon" aria-hidden="true">
                      <path d="M6 15.5 10 11l2.85 2.85L18 8.5" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
                      <path d="M14.8 8.5H18v3.2" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
                    </svg>
                    {{ t('components.logs.costDetail.highUsage') }}
                  </div>
                </div>

                <svg viewBox="0 0 24 24" class="logs-cost-modal__row-arrow" aria-hidden="true">
                  <path d="m10 7 5 5-5 5" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
              </button>
            </section>
          </template>
        </template>
      </section>

      <footer v-if="!loading && !error && rows.length" class="logs-cost-modal__footer">
        <span>{{ t('components.logs.costDetail.statementEnd') }}</span>
        <span>{{ t('components.logs.costDetail.filterHint') }}</span>
      </footer>
    </div>
  </InlineModal>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import InlineModal from '../../common/InlineModal.vue'
import type { ProviderDailyStat } from '../../../services/logs'
import { buildLogsCostDetailViewState } from './logsCostDetailModalState'

const props = withDefaults(defineProps<{
  open: boolean
  loading: boolean
  error?: string
  data: ProviderDailyStat[]
  updatedAt?: number
  scopeHint?: string
  isDarkTheme?: boolean
  formatCurrency: (value?: number) => string
}>(), {
  error: '',
  updatedAt: 0,
  scopeHint: '',
  isDarkTheme: false,
})

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'select-provider', provider: string): void
}>()

const { t, locale } = useI18n()

const modalPanelClass = computed(() => ({
  'logs-cost-modal-shell': true,
  'logs-cost-modal-shell--dark': props.isDarkTheme,
}))

const scopeText = computed(() => {
  const scope = `${props.scopeHint ?? ''}`.trim()
  if (!scope) return ''
  return t('components.logs.costDetail.scope', { scope })
})

const viewState = computed(() => buildLogsCostDetailViewState({
  data: props.data,
  error: props.error,
  formatCurrency: props.formatCurrency,
}))

const updatedAtLabel = computed(() => {
  if (!props.updatedAt) return ''
  return new Intl.DateTimeFormat(locale.value || 'zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(props.updatedAt)
})

const showSummary = computed(() => viewState.value.showSummary)
const totalAmountParts = computed(() => viewState.value.totalAmountParts)
const providerCountLabel = computed(() => t('components.logs.costDetail.totalProviders', { count: viewState.value.providerCount }))
const rows = computed(() => viewState.value.rows)

function handleProviderSelect(provider: string) {
  const nextProvider = `${provider ?? ''}`.trim()
  if (!nextProvider) return
  emit('select-provider', nextProvider)
}
</script>

<style scoped>
:global(.logs-cost-modal-shell) {
  overflow: hidden;
  border-radius: 30px;
  border: 1px solid rgba(226, 232, 240, 0.82);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.985), rgba(248, 250, 252, 0.96));
  box-shadow:
    0 34px 100px rgba(15, 23, 42, 0.18),
    0 16px 34px rgba(15, 23, 42, 0.08);
}

:global(.logs-cost-modal-shell .modal-header) {
  padding: 22px 24px 16px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.84);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(248, 250, 252, 0.74));
}

:global(.logs-cost-modal-shell .modal-title) {
  color: rgba(15, 23, 42, 0.96);
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.02em;
}

:global(.logs-cost-modal-shell .modal-body) {
  padding: 0 24px 24px;
  background: transparent;
}

:global(.logs-cost-modal-shell .ghost-icon) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.22);
  background: rgba(255, 255, 255, 0.84);
  color: rgba(51, 65, 85, 0.82);
  transition:
    transform 0.18s ease,
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease;
}

:global(.logs-cost-modal-shell .ghost-icon:hover:not(:disabled)),
:global(.logs-cost-modal-shell .ghost-icon:focus-visible) {
  transform: translateY(-1px);
  border-color: rgba(59, 130, 246, 0.24);
  background: rgba(239, 246, 255, 0.98);
  color: #1d4ed8;
}

:global(.logs-cost-modal-shell--dark) {
  border-color: rgba(148, 163, 184, 0.16);
  background:
    linear-gradient(180deg, rgba(8, 13, 23, 0.995), rgba(10, 16, 28, 0.985));
  box-shadow:
    0 38px 104px rgba(2, 6, 23, 0.64),
    inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

:global(.logs-cost-modal-shell--dark .modal-header) {
  border-bottom-color: rgba(148, 163, 184, 0.14);
  background:
    linear-gradient(180deg, rgba(13, 20, 33, 0.95), rgba(8, 14, 24, 0.8));
}

:global(.logs-cost-modal-shell--dark .modal-title) {
  color: rgba(248, 250, 252, 0.96);
}

:global(.logs-cost-modal-shell--dark .ghost-icon) {
  border-color: rgba(148, 163, 184, 0.16);
  background: rgba(148, 163, 184, 0.08);
  color: rgba(226, 232, 240, 0.82);
}

:global(.logs-cost-modal-shell--dark .ghost-icon:hover:not(:disabled)),
:global(.logs-cost-modal-shell--dark .ghost-icon:focus-visible) {
  border-color: rgba(96, 165, 250, 0.26);
  background: rgba(30, 41, 59, 0.92);
  color: rgba(147, 197, 253, 0.96);
}

.logs-cost-modal {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-height: 380px;
  padding-top: 18px;
  overflow: hidden;
}

.logs-cost-modal--light {
  --logs-cost-text-primary: #0f172a;
  --logs-cost-text-secondary: #64748b;
  --logs-cost-text-soft: #94a3b8;
  --logs-cost-border: rgba(148, 163, 184, 0.18);
  --logs-cost-border-strong: rgba(255, 255, 255, 0.85);
  --logs-cost-surface: linear-gradient(160deg, rgba(255, 255, 255, 0.92), rgba(241, 245, 249, 0.78));
  --logs-cost-surface-soft: rgba(255, 255, 255, 0.66);
  --logs-cost-surface-hover: rgba(255, 255, 255, 0.88);
  --logs-cost-track: rgba(148, 163, 184, 0.16);
  --logs-cost-fill: linear-gradient(90deg, rgba(59, 130, 246, 0.54), rgba(37, 99, 235, 0.92));
  --logs-cost-chip: rgba(255, 255, 255, 0.76);
  --logs-cost-chip-muted: rgba(241, 245, 249, 0.72);
  --logs-cost-footer: rgba(100, 116, 139, 0.72);
  --logs-cost-accent: #f97316;
  --logs-cost-accent-soft: rgba(249, 115, 22, 0.14);
  --logs-cost-danger: rgba(220, 38, 38, 0.9);
  --logs-cost-shadow: 0 20px 48px rgba(15, 23, 42, 0.08);
}

.logs-cost-modal--dark {
  --logs-cost-text-primary: #f8fafc;
  --logs-cost-text-secondary: #94a3b8;
  --logs-cost-text-soft: rgba(148, 163, 184, 0.82);
  --logs-cost-border: rgba(255, 255, 255, 0.08);
  --logs-cost-border-strong: rgba(255, 255, 255, 0.06);
  --logs-cost-surface: linear-gradient(155deg, rgba(27, 33, 46, 0.94), rgba(16, 21, 33, 0.88));
  --logs-cost-surface-soft: rgba(255, 255, 255, 0.025);
  --logs-cost-surface-hover: rgba(255, 255, 255, 0.055);
  --logs-cost-track: rgba(255, 255, 255, 0.08);
  --logs-cost-fill: linear-gradient(90deg, rgba(96, 165, 250, 0.58), rgba(59, 130, 246, 0.96));
  --logs-cost-chip: rgba(255, 255, 255, 0.05);
  --logs-cost-chip-muted: rgba(255, 255, 255, 0.04);
  --logs-cost-footer: rgba(148, 163, 184, 0.5);
  --logs-cost-accent: #fb923c;
  --logs-cost-accent-soft: rgba(249, 115, 22, 0.12);
  --logs-cost-danger: rgba(248, 113, 113, 0.92);
  --logs-cost-shadow: 0 24px 54px rgba(2, 6, 23, 0.38);
}

.logs-cost-modal__glow {
  position: absolute;
  width: 240px;
  height: 240px;
  border-radius: 999px;
  filter: blur(90px);
  pointer-events: none;
  opacity: 0.65;
}

.logs-cost-modal__glow--orange {
  top: -120px;
  right: -108px;
  background: rgba(249, 115, 22, 0.22);
}

.logs-cost-modal__glow--blue {
  bottom: -132px;
  left: -112px;
  background: rgba(59, 130, 246, 0.22);
}

.logs-cost-modal__hero {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.logs-cost-modal__hero-header {
  display: flex;
  align-items: center;
  gap: 14px;
}

.logs-cost-modal__hero-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 16px;
  background: var(--logs-cost-accent-soft);
  color: var(--logs-cost-accent);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06);
}

.logs-cost-modal__hero-icon-svg {
  width: 20px;
  height: 20px;
}

.logs-cost-modal__hero-copy {
  min-width: 0;
}

.logs-cost-modal__hero-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--logs-cost-text-primary);
  letter-spacing: 0.01em;
}

.logs-cost-modal__hero-scope {
  margin: 4px 0 0;
  font-size: 12px;
  line-height: 1.55;
  color: var(--logs-cost-text-secondary);
}

.logs-cost-modal__summary-card {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  padding: 24px;
  border-radius: 24px;
  border: 1px solid var(--logs-cost-border);
  background: var(--logs-cost-surface);
  box-shadow: var(--logs-cost-shadow);
  backdrop-filter: blur(18px);
}

.logs-cost-modal__summary-card--loading {
  flex-direction: column;
  gap: 16px;
}

.logs-cost-modal__summary-main {
  min-width: 0;
}

.logs-cost-modal__summary-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 12px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--logs-cost-text-secondary);
}

.logs-cost-modal__summary-label-icon {
  width: 13px;
  height: 13px;
  color: #22c55e;
}

.logs-cost-modal__summary-value {
  display: flex;
  align-items: baseline;
  gap: 4px;
  min-width: 0;
  color: var(--logs-cost-text-primary);
}

.logs-cost-modal__currency {
  font-size: 28px;
  font-weight: 500;
  color: var(--logs-cost-accent);
}

.logs-cost-modal__amount-whole {
  font-size: clamp(34px, 5vw, 46px);
  line-height: 1;
  font-weight: 900;
  letter-spacing: -0.04em;
}

.logs-cost-modal__amount-decimal {
  font-size: clamp(20px, 3vw, 28px);
  line-height: 1;
  font-weight: 700;
  color: var(--logs-cost-text-secondary);
}

.logs-cost-modal__summary-meta {
  margin: 12px 0 0;
  font-size: 13px;
  color: var(--logs-cost-text-secondary);
}

.logs-cost-modal__summary-side {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
}

.logs-cost-modal__summary-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 9px 12px;
  border-radius: 999px;
  border: 1px solid var(--logs-cost-border);
  background: var(--logs-cost-chip);
  color: var(--logs-cost-text-secondary);
  font-size: 11px;
  line-height: 1.4;
  letter-spacing: 0.04em;
}

.logs-cost-modal__summary-chip--muted {
  max-width: min(320px, 100%);
  background: var(--logs-cost-chip-muted);
  color: var(--logs-cost-text-soft);
  text-align: right;
}

.logs-cost-modal__summary-chip-icon {
  width: 12px;
  height: 12px;
  flex: 0 0 auto;
}

.logs-cost-modal__state {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 18px 18px 18px 20px;
  border-radius: 20px;
  border: 1px solid var(--logs-cost-border);
  background: var(--logs-cost-surface-soft);
  color: var(--logs-cost-text-secondary);
}

.logs-cost-modal__state strong {
  font-size: 14px;
  font-weight: 600;
  color: inherit;
}

.logs-cost-modal__state--error {
  color: var(--logs-cost-danger);
  background: rgba(239, 68, 68, 0.08);
  border-color: rgba(248, 113, 113, 0.18);
}

.logs-cost-modal__state-icon {
  width: 20px;
  height: 20px;
  flex: 0 0 auto;
}

.logs-cost-modal__list-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.logs-cost-modal__list-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 0 2px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--logs-cost-text-soft);
}

.logs-cost-modal__row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  padding: 16px 18px;
  border-radius: 22px;
  border: 1px solid var(--logs-cost-border);
  background: var(--logs-cost-surface-soft);
  color: inherit;
  text-align: left;
  cursor: pointer;
  overflow: hidden;
  transition:
    transform 0.22s ease,
    border-color 0.22s ease,
    background 0.22s ease,
    box-shadow 0.22s ease;
}

.logs-cost-modal__row:hover:not(:disabled),
.logs-cost-modal__row:focus-visible {
  transform: translateY(-2px);
  border-color: rgba(96, 165, 250, 0.24);
  background: var(--logs-cost-surface-hover);
  box-shadow: 0 20px 42px rgba(15, 23, 42, 0.12);
  outline: none;
}

.logs-cost-modal__row:disabled {
  cursor: default;
}

.logs-cost-modal__row-accent {
  position: absolute;
  left: 0;
  top: 50%;
  width: 4px;
  height: 34px;
  border-radius: 0 999px 999px 0;
  background: var(--logs-cost-accent);
  transform: translateY(-50%) scaleY(0);
  transform-origin: center;
  transition: transform 0.22s ease;
}

.logs-cost-modal__row:hover:not(:disabled) .logs-cost-modal__row-accent,
.logs-cost-modal__row:focus-visible .logs-cost-modal__row-accent {
  transform: translateY(-50%) scaleY(1);
}

.logs-cost-modal__row-main {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
  flex: 1 1 auto;
}

.logs-cost-modal__avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 42px;
  border-radius: 15px;
  border: 1px solid var(--logs-cost-border);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.08), rgba(255, 255, 255, 0.02));
  font-size: 18px;
  font-weight: 700;
  color: var(--logs-cost-text-secondary);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
  flex: 0 0 auto;
}

.logs-cost-modal__provider-copy {
  min-width: 0;
  flex: 1 1 auto;
}

.logs-cost-modal__provider-name {
  font-size: 15px;
  font-weight: 650;
  color: var(--logs-cost-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.logs-cost-modal__provider-share {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
}

.logs-cost-modal__share-track {
  position: relative;
  flex: 0 0 72px;
  height: 6px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--logs-cost-track);
}

.logs-cost-modal__share-fill {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--logs-cost-fill);
}

.logs-cost-modal__share-label {
  font-size: 11px;
  color: var(--logs-cost-text-secondary);
}

.logs-cost-modal__row-side {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
  flex: 0 0 auto;
}

.logs-cost-modal__row-amount {
  font-family: "SFMono-Regular", "Roboto Mono", "Consolas", monospace;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--logs-cost-accent);
  font-variant-numeric: tabular-nums;
}

.logs-cost-modal__row-note {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 10px;
  font-weight: 600;
  color: var(--logs-cost-danger);
}

.logs-cost-modal__row-note-icon {
  width: 11px;
  height: 11px;
  flex: 0 0 auto;
}

.logs-cost-modal__row-arrow {
  width: 18px;
  height: 18px;
  color: rgba(148, 163, 184, 0.52);
  flex: 0 0 auto;
  transition:
    color 0.18s ease,
    transform 0.18s ease;
}

.logs-cost-modal__row:hover:not(:disabled) .logs-cost-modal__row-arrow,
.logs-cost-modal__row:focus-visible .logs-cost-modal__row-arrow {
  color: var(--logs-cost-text-secondary);
  transform: translateX(3px);
}

.logs-cost-modal__footer {
  position: relative;
  z-index: 1;
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding-top: 4px;
  border-top: 1px solid var(--logs-cost-border-strong);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--logs-cost-footer);
}

.logs-cost-modal__loading-block {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.logs-cost-modal__loading-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.logs-cost-modal__loading-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 18px;
  border-radius: 20px;
  border: 1px solid var(--logs-cost-border);
  background: var(--logs-cost-surface-soft);
}

.logs-cost-modal__loading-copy {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  gap: 10px;
}

.logs-cost-modal__skeleton {
  position: relative;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.16);
}

.logs-cost-modal--dark .logs-cost-modal__skeleton {
  background: rgba(148, 163, 184, 0.12);
}

.logs-cost-modal__skeleton::after {
  content: '';
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.45), transparent);
  animation: logs-cost-shimmer 1.35s ease-in-out infinite;
}

.logs-cost-modal--dark .logs-cost-modal__skeleton::after {
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.18), transparent);
}

.logs-cost-modal__skeleton--label {
  width: 156px;
  height: 14px;
}

.logs-cost-modal__skeleton--amount {
  width: 240px;
  height: 46px;
  border-radius: 18px;
}

.logs-cost-modal__skeleton--meta {
  width: 200px;
  height: 14px;
}

.logs-cost-modal__skeleton--avatar {
  width: 42px;
  height: 42px;
  border-radius: 15px;
  flex: 0 0 auto;
}

.logs-cost-modal__skeleton--row-title {
  width: 172px;
  height: 14px;
}

.logs-cost-modal__skeleton--row-bar {
  width: 128px;
  height: 10px;
}

.logs-cost-modal__skeleton--row-amount {
  width: 84px;
  height: 16px;
  flex: 0 0 auto;
}

@keyframes logs-cost-shimmer {
  100% {
    transform: translateX(100%);
  }
}

@media (max-width: 720px) {
  :global(.logs-cost-modal-shell .modal-header) {
    padding: 18px 18px 14px;
  }

  :global(.logs-cost-modal-shell .modal-body) {
    padding: 0 18px 18px;
  }

  .logs-cost-modal {
    padding-top: 14px;
  }

  .logs-cost-modal__summary-card {
    flex-direction: column;
    padding: 20px;
  }

  .logs-cost-modal__summary-side {
    align-items: flex-start;
  }

  .logs-cost-modal__summary-chip--muted {
    text-align: left;
  }

  .logs-cost-modal__row {
    align-items: flex-start;
    gap: 12px;
    padding: 15px 16px;
  }

  .logs-cost-modal__row-main {
    align-items: flex-start;
  }

  .logs-cost-modal__provider-share {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }

  .logs-cost-modal__row-side {
    min-width: 78px;
  }

  .logs-cost-modal__footer {
    flex-direction: column;
    gap: 8px;
  }
}

@media (max-width: 560px) {
  .logs-cost-modal__hero-header {
    align-items: flex-start;
  }

  .logs-cost-modal__summary-value {
    flex-wrap: wrap;
  }

  .logs-cost-modal__list-header {
    display: none;
  }

  .logs-cost-modal__row {
    flex-wrap: wrap;
  }

  .logs-cost-modal__row-side {
    align-items: flex-start;
    margin-left: 56px;
  }

  .logs-cost-modal__row-arrow {
    position: absolute;
    top: 18px;
    right: 16px;
  }
}
</style>
