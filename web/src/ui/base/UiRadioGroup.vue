<script setup lang="ts">
import { RadioGroupRoot } from 'reka-ui'

defineOptions({ inheritAttrs: false })
withDefaults(
  defineProps<{
    modelValue?: string | number | null
    disabled?: boolean
    label?: string
  }>(),
  { modelValue: undefined, disabled: false, label: '' },
)
const emit = defineEmits<{
  'update:modelValue': [value: string | number]
  change: [event: Event]
}>()
function handleUpdate(value: unknown) {
  const next = value as string | number
  emit('update:modelValue', next)
  emit('change', { target: { value: next } } as unknown as Event)
}
</script>
<template>
  <RadioGroupRoot
    :model-value="modelValue"
    :disabled="disabled"
    data-slot="radio-group"
    class="radio-group"
    :aria-label="label || undefined"
    @update:model-value="handleUpdate"
  >
    <slot />
  </RadioGroupRoot>
</template>
<style scoped>
.radio-group {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
</style>
