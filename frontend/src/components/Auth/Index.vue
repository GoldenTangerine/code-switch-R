<script setup lang="ts">
/**
 * @name: 认证页面
 * @Descripttion: 管理 Codex 官方 OAuth 认证状态
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-03 00:00:00
 * @LastEditTime: 2026-07-03 00:00:00
 * @FilePath: frontend/src/components/Auth/Index.vue
 */
import { onActivated, onBeforeUnmount, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Browser, Clipboard } from '@wailsio/runtime'
import ListItem from '../Setting/ListRow.vue'
import {
  fetchCodexOAuthStatus,
  logoutCodexOAuth,
  pollCodexOAuthLogin,
  removeCodexOAuthAccount,
  setDefaultCodexOAuthAccount,
  startCodexOAuthLogin,
  type CodexOAuthDeviceCodeResponse,
  type CodexOAuthStatus,
} from '../../services/codexOAuth'
import { writeTextToClipboard } from '../../utils/clipboard'
import { extractErrorMessage } from '../../utils/error'
import { showToast } from '../../utils/toast'

const router = useRouter()
const { t } = useI18n()

const codexOAuthStatus = ref<CodexOAuthStatus | null>(null)
const codexOAuthLoading = ref(false)
const codexOAuthBusy = ref(false)
const codexOAuthDevice = ref<CodexOAuthDeviceCodeResponse | null>(null)
const codexOAuthPollingMessage = ref('')
let codexOAuthPollTimer: number | null = null

const goBack = () => {
  router.push('/')
}

const stopCodexOAuthPolling = () => {
  if (codexOAuthPollTimer) {
    window.clearTimeout(codexOAuthPollTimer)
    codexOAuthPollTimer = null
  }
}

const resetCodexOAuthDevice = () => {
  stopCodexOAuthPolling()
  codexOAuthDevice.value = null
  codexOAuthPollingMessage.value = ''
}

const loadCodexOAuthStatus = async () => {
  codexOAuthLoading.value = true
  try {
    codexOAuthStatus.value = await fetchCodexOAuthStatus()
  } catch (error) {
    console.error('failed to load Codex OAuth status', error)
  } finally {
    codexOAuthLoading.value = false
  }
}

const scheduleCodexOAuthPoll = () => {
  stopCodexOAuthPolling()
  const device = codexOAuthDevice.value
  if (!device) return
  const interval = Math.max(3, Number(device.interval) || 5) * 1000
  codexOAuthPollTimer = window.setTimeout(() => {
    void pollCodexOAuthOnce()
  }, interval)
}

const pollCodexOAuthOnce = async () => {
  const device = codexOAuthDevice.value
  if (!device) return
  try {
    const account = await pollCodexOAuthLogin(device.deviceCode)
    if (account) {
      resetCodexOAuthDevice()
      await loadCodexOAuthStatus()
      window.dispatchEvent(new CustomEvent('providers-updated'))
      showToast(t('components.general.codexOAuth.loginSuccess'), 'success')
      return
    }
  } catch (error) {
    const message = extractErrorMessage(error)
    if (!message.includes('authorization_pending')) {
      resetCodexOAuthDevice()
      showToast(t('components.general.codexOAuth.loginFailed', { error: message }), 'error')
      return
    }
  }
  codexOAuthPollingMessage.value = t('components.general.codexOAuth.waiting')
  scheduleCodexOAuthPoll()
}

const startCodexOAuth = async () => {
  codexOAuthBusy.value = true
  codexOAuthPollingMessage.value = ''
  try {
    const device = await startCodexOAuthLogin()
    codexOAuthDevice.value = device
    codexOAuthPollingMessage.value = t('components.general.codexOAuth.waiting')
    scheduleCodexOAuthPoll()
  } catch (error) {
    showToast(t('components.general.codexOAuth.loginFailed', { error: extractErrorMessage(error) }), 'error')
  } finally {
    codexOAuthBusy.value = false
  }
}

const copyText = async (payload: string) => {
  try {
    await Clipboard.SetText(payload)
  } catch {
    await writeTextToClipboard(payload)
  }
}

const readTextFromClipboard = async () => {
  try {
    return await Clipboard.Text()
  } catch {
    const clipboardReadText = typeof navigator === 'undefined'
      ? undefined
      : navigator.clipboard?.readText?.bind(navigator.clipboard)
    if (clipboardReadText == null) throw new Error('clipboard read unavailable')
    return clipboardReadText()
  }
}

const openCodexOAuthVerification = () => {
  const target = codexOAuthDevice.value?.verificationUri
  if (!target) return

  Browser.OpenURL(target).catch((error) => {
    console.error('failed to open Codex OAuth verification link', error)
    showToast(t('components.general.codexOAuth.openVerifyFailed'), 'error')
  })
}

const copyCodexOAuthVerificationURL = async () => {
  const target = codexOAuthDevice.value?.verificationUri
  if (!target) return

  try {
    await copyText(target)
    showToast(t('components.general.codexOAuth.copyVerifySuccess'), 'success')
  } catch (error) {
    console.error('failed to copy Codex OAuth verification link', error)
    showToast(t('components.general.codexOAuth.copyVerifyFailed'), 'error')
  }
}

const completeCodexOAuthFromClipboard = async () => {
  const device = codexOAuthDevice.value
  if (!device) return

  codexOAuthBusy.value = true
  try {
    const callbackURL = (await readTextFromClipboard()).trim()
    if (!callbackURL) {
      showToast(t('components.general.codexOAuth.callbackURLMissing'), 'warning')
      return
    }
    try {
      new URL(callbackURL)
    } catch {
      showToast(t('components.general.codexOAuth.callbackURLInvalid'), 'warning')
      return
    }
    await pollCodexOAuthOnce()
  } catch (error) {
    console.error('failed to complete Codex OAuth from clipboard', error)
    showToast(t('components.general.codexOAuth.callbackURLReadFailed'), 'error')
  } finally {
    codexOAuthBusy.value = false
  }
}

const setCodexOAuthDefault = async (accountId: string) => {
  codexOAuthBusy.value = true
  try {
    await setDefaultCodexOAuthAccount(accountId)
    await loadCodexOAuthStatus()
    window.dispatchEvent(new CustomEvent('providers-updated'))
  } catch (error) {
    showToast(extractErrorMessage(error), 'error')
  } finally {
    codexOAuthBusy.value = false
  }
}

const removeCodexOAuth = async (accountId: string) => {
  codexOAuthBusy.value = true
  try {
    await removeCodexOAuthAccount(accountId)
    await loadCodexOAuthStatus()
    window.dispatchEvent(new CustomEvent('providers-updated'))
  } catch (error) {
    showToast(extractErrorMessage(error), 'error')
  } finally {
    codexOAuthBusy.value = false
  }
}

const logoutAllCodexOAuth = async () => {
  codexOAuthBusy.value = true
  try {
    resetCodexOAuthDevice()
    await logoutCodexOAuth()
    await loadCodexOAuthStatus()
    window.dispatchEvent(new CustomEvent('providers-updated'))
  } catch (error) {
    showToast(extractErrorMessage(error), 'error')
  } finally {
    codexOAuthBusy.value = false
  }
}

onActivated(() => {
  void loadCodexOAuthStatus()
})

onBeforeUnmount(() => {
  resetCodexOAuthDevice()
})
</script>

<template>
  <div class="main-shell auth-shell">
    <div class="global-actions">
      <p class="global-eyebrow">{{ t('components.auth.hero.eyebrow') }}</p>
      <button class="ghost-icon" :aria-label="t('components.auth.actions.back')" @click="goBack">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M15 18l-6-6 6-6"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>
    </div>

    <div class="general-page auth-page">
      <section class="auth-hero">
        <div>
          <span class="auth-hero__badge">{{ t('components.auth.hero.badge') }}</span>
          <h1>{{ t('components.auth.hero.title') }}</h1>
          <p>{{ t('components.auth.hero.lead') }}</p>
        </div>
      </section>

      <section>
        <h2 class="mac-section-title">{{ t('components.auth.codex.title') }}</h2>
        <p class="mac-section-description">{{ t('components.auth.codex.description') }}</p>
        <div class="mac-panel auth-panel">
          <ListItem :label="t('components.general.codexOAuth.title')">
            <div class="codex-oauth-panel">
              <div class="codex-oauth-panel__status">
                <span class="codex-oauth-status" :class="{ active: codexOAuthStatus?.authenticated }">
                  {{ codexOAuthStatus?.authenticated ? t('components.general.codexOAuth.loggedIn') : t('components.general.codexOAuth.notLoggedIn') }}
                </span>
                <div class="codex-oauth-actions">
                  <button
                    class="action-btn"
                    type="button"
                    :disabled="codexOAuthBusy || codexOAuthLoading"
                    @click="startCodexOAuth"
                  >
                    {{ t('components.general.codexOAuth.login') }}
                  </button>
                  <button
                    v-if="codexOAuthStatus?.authenticated"
                    class="action-btn"
                    type="button"
                    :disabled="codexOAuthBusy || codexOAuthLoading"
                    @click="logoutAllCodexOAuth"
                  >
                    {{ t('components.general.codexOAuth.logout') }}
                  </button>
                </div>
              </div>

              <div v-if="codexOAuthDevice" class="codex-oauth-device">
                <span>{{ t('components.general.codexOAuth.userCode') }}：<strong>{{ codexOAuthDevice.userCode }}</strong></span>
                <div class="codex-oauth-device__actions">
                  <a :href="codexOAuthDevice.verificationUri" target="_blank" rel="noreferrer" @click.prevent="openCodexOAuthVerification">
                    {{ t('components.general.codexOAuth.openVerify') }}
                  </a>
                  <button class="codex-oauth-link-btn" type="button" @click="copyCodexOAuthVerificationURL">
                    {{ t('components.general.codexOAuth.copyVerify') }}
                  </button>
                  <button class="codex-oauth-link-btn" type="button" :disabled="codexOAuthBusy" @click="completeCodexOAuthFromClipboard">
                    {{ t('components.general.codexOAuth.pasteCallback') }}
                  </button>
                </div>
                <span class="auth-hint">{{ codexOAuthPollingMessage }}</span>
              </div>

              <div v-if="codexOAuthStatus?.accounts?.length" class="codex-oauth-accounts">
                <div v-for="account in codexOAuthStatus.accounts" :key="account.id" class="codex-oauth-account">
                  <span>
                    {{ account.login }}
                    <em v-if="account.isDefault">{{ t('components.general.codexOAuth.defaultAccount') }}</em>
                  </span>
                  <div class="codex-oauth-account__actions">
                    <button
                      v-if="!account.isDefault"
                      class="action-btn"
                      type="button"
                      :disabled="codexOAuthBusy"
                      @click="setCodexOAuthDefault(account.id)"
                    >
                      {{ t('components.general.codexOAuth.setDefault') }}
                    </button>
                    <button
                      class="action-btn"
                      type="button"
                      :disabled="codexOAuthBusy"
                      @click="removeCodexOAuth(account.id)"
                    >
                      {{ t('components.general.codexOAuth.remove') }}
                    </button>
                  </div>
                </div>
              </div>

              <span class="auth-hint">{{ t('components.general.codexOAuth.hint') }}</span>
            </div>
          </ListItem>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.auth-page {
  gap: 24px;
}

.auth-hero {
  position: relative;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--mac-border) 88%, transparent);
  border-radius: 24px;
  padding: 28px;
  background:
    radial-gradient(circle at 12% 18%, rgba(16, 185, 129, 0.16), transparent 32%),
    radial-gradient(circle at 88% 0%, rgba(59, 130, 246, 0.12), transparent 30%),
    linear-gradient(180deg, color-mix(in srgb, var(--mac-surface) 78%, transparent), color-mix(in srgb, var(--mac-surface-strong) 86%, transparent));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08), 0 20px 48px rgba(2, 6, 23, 0.08);
}

.auth-hero__badge {
  display: inline-flex;
  margin-bottom: 12px;
  padding: 5px 10px;
  border: 1px solid color-mix(in srgb, var(--mac-accent) 24%, transparent);
  border-radius: 999px;
  color: var(--mac-accent);
  background: color-mix(in srgb, var(--mac-accent) 10%, transparent);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.auth-hero h1 {
  margin: 0;
  color: var(--mac-text);
  font-size: clamp(1.8rem, 2.6vw, 2.5rem);
  letter-spacing: -0.04em;
}

.auth-hero p {
  max-width: 680px;
  margin: 10px 0 0;
  color: var(--mac-text-secondary);
  line-height: 1.7;
}

.auth-panel {
  overflow: visible;
}

.codex-oauth-panel,
.codex-oauth-device,
.codex-oauth-accounts {
  display: grid;
  gap: 10px;
}

.codex-oauth-panel__status,
.codex-oauth-account,
.codex-oauth-account__actions,
.codex-oauth-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.codex-oauth-panel__status {
  justify-content: space-between;
}

.codex-oauth-status {
  color: var(--mac-text-secondary);
}

.codex-oauth-status.active {
  color: #10b981;
}

.codex-oauth-device {
  justify-items: start;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--mac-accent) 20%, var(--mac-border));
  border-radius: 14px;
  background: color-mix(in srgb, var(--mac-accent) 8%, var(--mac-surface));
}

.codex-oauth-device strong {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  letter-spacing: 0.08em;
}

.codex-oauth-device a {
  color: var(--mac-accent);
  text-decoration: none;
}

.codex-oauth-device__actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.codex-oauth-link-btn {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--mac-accent);
  cursor: pointer;
  font: inherit;
}

.codex-oauth-link-btn:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.codex-oauth-account {
  justify-content: space-between;
  padding: 8px 10px;
  border: 1px solid var(--mac-border);
  border-radius: 12px;
  background: var(--mac-surface-strong);
}

.codex-oauth-account em {
  margin-left: 6px;
  color: var(--mac-text-secondary);
  font-style: normal;
  font-size: 12px;
}

.auth-hint {
  color: var(--mac-text-secondary);
  font-size: 11px;
  line-height: 1.4;
}

@media (max-width: 720px) {
  .auth-hero {
    padding: 22px;
  }

  .codex-oauth-panel__status,
  .codex-oauth-account {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
