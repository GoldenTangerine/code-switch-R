<template>
  <input
    v-bind="$attrs"
    :type="type"
    class="base-input"
    :value="modelValue"
    autocorrect="off"
    autocapitalize="none"
    spellcheck="false"
    autocomplete="off"
    @input="onInput"
  />
</template>

<script setup lang="ts">
import { useAttrs } from 'vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    modelValue?: string | number
    type?: string
  }>(),
  {
    modelValue: '',
    type: 'text',
  },
)

const emit = defineEmits<{ (e: 'update:modelValue', value: string): void }>()

useAttrs()

const onInput = (event: Event) => {
  const target = event.target as HTMLInputElement
  emit('update:modelValue', target.value)
}
</script>
