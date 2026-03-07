<template>
  <section class="logs-summary" v-if="statsCards.length">
    <article
      v-for="card in statsCards"
      :key="card.key"
      :class="['summary-card', { 'summary-card--clickable': isClickable(card.key) }]"
      @click="emit('card-click', card.key)"
    >
      <div class="summary-card__label">{{ card.label }}</div>
      <div class="summary-card__value">
        {{ card.value }}
        <span v-if="card.subValue" class="summary-card__sub-value">({{ card.subValue }})</span>
      </div>
      <div class="summary-card__hint">{{ card.hint }}</div>
    </article>
  </section>
</template>

<script setup lang="ts">
import type { LogsSummaryCard } from '../types'

defineProps<{
  statsCards: LogsSummaryCard[]
}>()

const emit = defineEmits<{
  (event: 'card-click', key: string): void
}>()

const isClickable = (key: string) => key === 'cost' || key === 'tokens'
</script>

<style scoped>
.logs-summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 1rem;
  margin-bottom: 0.75rem;
}

.summary-card {
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 16px;
  padding: 1rem 1.25rem;
  background: radial-gradient(circle at top, rgba(148, 163, 184, 0.1), rgba(15, 23, 42, 0));
  backdrop-filter: blur(6px);
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.summary-card__label {
  font-size: 0.85rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: #475569;
}

.summary-card__value {
  font-size: 1.85rem;
  font-weight: 600;
  color: #0f172a;
}

.summary-card__hint {
  font-size: 0.85rem;
  color: #94a3b8;
}

.summary-card__sub-value {
  font-size: 0.65em;
  font-weight: 400;
  color: #64748b;
  margin-left: 0.25rem;
}

html.dark .summary-card {
  border-color: rgba(255, 255, 255, 0.12);
  background: radial-gradient(circle at top, rgba(148, 163, 184, 0.2), rgba(15, 23, 42, 0.35));
}

html.dark .summary-card__label {
  color: rgba(248, 250, 252, 0.75);
}

html.dark .summary-card__value {
  color: rgba(248, 250, 252, 0.95);
}

html.dark .summary-card__hint {
  color: rgba(186, 194, 210, 0.8);
}

html.dark .summary-card__sub-value {
  color: #94a3b8;
}

@media (max-width: 768px) {
  .logs-summary {
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  }
}

.summary-card--clickable {
  cursor: pointer;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.summary-card--clickable:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(249, 115, 22, 0.15);
}

.summary-card--clickable:active {
  transform: translateY(0);
}

html.dark .summary-card--clickable:hover {
  box-shadow: 0 4px 12px rgba(249, 115, 22, 0.25);
}
</style>
