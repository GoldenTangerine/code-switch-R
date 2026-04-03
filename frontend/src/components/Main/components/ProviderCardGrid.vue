<template>
  <TransitionGroup
    ref="listRef"
    tag="div"
    name="provider-card-list"
    class="automation-list"
    :class="{ 'is-sorting': isSorting }"
    @dragover.prevent="handleListDragOver"
    @dragleave="handleListDragLeave"
    @drop.prevent="handleListDrop"
  >
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
      @dragend="$emit('dragend', $event)"
      @open-site="$emit('open-site', viewModel.card)"
      @unblock-and-reset="$emit('unblock-and-reset', viewModel.card)"
      @reset-level="$emit('reset-level', viewModel.card)"
      @toggle-enabled="(enabled) => $emit('toggle-enabled', viewModel.card, enabled)"
      @direct-apply="$emit('direct-apply', viewModel.card)"
      @configure="$emit('configure', viewModel.card)"
      @open-model-list="$emit('open-model-list', viewModel.card)"
      @open-provider-logs="$emit('open-provider-logs', viewModel.card)"
      @open-provider-cost-trend="$emit('open-provider-cost-trend', viewModel.card)"
      @duplicate="$emit('duplicate', viewModel.card)"
      @remove="$emit('remove', viewModel.card)"
    />
  </TransitionGroup>
</template>

<script setup lang="ts">
import { ref, TransitionGroup, type ComponentPublicInstance } from 'vue'
import type { AutomationCard } from '../../../data/cards'
import ProviderCard from './ProviderCard.vue'
import type { ProviderCardViewModel, ProviderDragTarget, ProviderTab, ResolvedTheme } from '../types'
import { resolveProviderDragTarget } from '../utils/providerDragTarget'

defineProps<{
  cards: ProviderCardViewModel[]
  activeTab: ProviderTab
  activeProxyState: boolean
  resolvedTheme: ResolvedTheme
  isSorting: boolean
  formatBlacklistCountdown: (remainingSeconds: number) => string
  bindCardRef: (card: AutomationCard) => (element: Element | ComponentPublicInstance | null) => void
}>()

const emit = defineEmits<{
  'card-click': [card: AutomationCard]
  dragstart: [id: number]
  'dragover-card': [target: ProviderDragTarget]
  'dragleave-list': []
  dragend: [dropEffect: DataTransfer['dropEffect'] | 'none']
  drop: [target: ProviderDragTarget]
  'open-site': [card: AutomationCard]
  'unblock-and-reset': [card: AutomationCard]
  'reset-level': [card: AutomationCard]
  'toggle-enabled': [card: AutomationCard, enabled: boolean]
  'direct-apply': [card: AutomationCard]
  configure: [card: AutomationCard]
  'open-model-list': [card: AutomationCard]
  'open-provider-logs': [card: AutomationCard]
  'open-provider-cost-trend': [card: AutomationCard]
  duplicate: [card: AutomationCard]
  remove: [card: AutomationCard]
}>()

const listRef = ref<ComponentPublicInstance | HTMLElement | null>(null)

const resolveListDragTarget = (event: DragEvent): ProviderDragTarget | null => {
  const listElement = listRef.value instanceof HTMLElement
    ? listRef.value
    : ((listRef.value as ComponentPublicInstance | null)?.$el as HTMLElement | undefined) ?? null

  if (!listElement) return null

  const bounds = Array.from(listElement.querySelectorAll<HTMLElement>('.automation-card[data-provider-id]'))
    .map((cardElement) => {
      const id = Number(cardElement.dataset.providerId)
      if (!Number.isFinite(id)) return null
      const rect = cardElement.getBoundingClientRect()
      return {
        id,
        top: rect.top,
        height: rect.height,
        bottom: rect.bottom,
      }
    })
    .filter((entry): entry is NonNullable<typeof entry> => entry !== null)

  return resolveProviderDragTarget(bounds, event.clientY)
}

const handleListDragOver = (event: DragEvent) => {
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'move'
  }

  const target = resolveListDragTarget(event)
  if (target) {
    emit('dragover-card', target)
  }
}

const handleListDragLeave = (event: DragEvent) => {
  const listElement = listRef.value instanceof HTMLElement
    ? listRef.value
    : ((listRef.value as ComponentPublicInstance | null)?.$el as HTMLElement | undefined) ?? null

  const nextTarget = event.relatedTarget as Node | null
  if (listElement && nextTarget && listElement.contains(nextTarget)) {
    return
  }

  emit('dragleave-list')
}

const handleListDrop = (event: DragEvent) => {
  const target = resolveListDragTarget(event)
  if (target) {
    emit('drop', target)
  }
}
</script>
