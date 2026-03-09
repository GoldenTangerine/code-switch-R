<template>
  <div class="automation-list" @dragover.prevent>
    <ProviderCard
      v-for="viewModel in cards"
      :key="viewModel.card.id"
      :view-model="viewModel"
      :active-tab="activeTab"
      :active-proxy-state="activeProxyState"
      :resolved-theme="resolvedTheme"
      :format-blacklist-countdown="formatBlacklistCountdown"
      :bind-card-ref="bindCardRef(viewModel.card)"
      @card-click="$emit('card-click', viewModel.card)"
      @dragstart="$emit('dragstart', viewModel.card.id)"
      @dragend="$emit('dragend')"
      @drop="$emit('drop', viewModel.card.id)"
      @open-site="$emit('open-site', viewModel.card)"
      @unblock-and-reset="$emit('unblock-and-reset', viewModel.card)"
      @reset-level="$emit('reset-level', viewModel.card)"
      @toggle-enabled="(enabled) => $emit('toggle-enabled', viewModel.card, enabled)"
      @direct-apply="$emit('direct-apply', viewModel.card)"
      @configure="$emit('configure', viewModel.card)"
      @open-model-list="$emit('open-model-list', viewModel.card)"
      @duplicate="$emit('duplicate', viewModel.card)"
      @remove="$emit('remove', viewModel.card)"
    />
  </div>
</template>

<script setup lang="ts">
import type { ComponentPublicInstance } from 'vue'
import type { AutomationCard } from '../../../data/cards'
import ProviderCard from './ProviderCard.vue'
import type { ProviderCardViewModel, ProviderTab, ResolvedTheme } from '../types'

defineProps<{
  cards: ProviderCardViewModel[]
  activeTab: ProviderTab
  activeProxyState: boolean
  resolvedTheme: ResolvedTheme
  formatBlacklistCountdown: (remainingSeconds: number) => string
  bindCardRef: (card: AutomationCard) => (element: Element | ComponentPublicInstance | null) => void
}>()

defineEmits<{
  'card-click': [card: AutomationCard]
  dragstart: [id: number]
  dragend: []
  drop: [id: number]
  'open-site': [card: AutomationCard]
  'unblock-and-reset': [card: AutomationCard]
  'reset-level': [card: AutomationCard]
  'toggle-enabled': [card: AutomationCard, enabled: boolean]
  'direct-apply': [card: AutomationCard]
  configure: [card: AutomationCard]
  'open-model-list': [card: AutomationCard]
  duplicate: [card: AutomationCard]
  remove: [card: AutomationCard]
}>()
</script>
