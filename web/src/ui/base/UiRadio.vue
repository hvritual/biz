<script setup lang="ts">
import { RadioGroupIndicator, RadioGroupItem } from 'reka-ui'

defineOptions({ inheritAttrs: false })
withDefaults(
  defineProps<{
    value: string | number
    disabled?: boolean
    label?: string
    description?: string
  }>(),
  { disabled: false, label: '', description: '' },
)
</script>
<template>
  <label class="radio-item" :class="{ disabled }">
    <RadioGroupItem
      :value="value"
      :disabled="disabled"
      data-slot="radio"
      class="radio-control"
    >
      <RadioGroupIndicator class="radio-indicator" :aria-hidden="true" />
    </RadioGroupItem>
    <span v-if="label || description" class="radio-text">
      <span class="radio-label">{{ label }}</span>
      <span v-if="description" class="radio-description">{{ description }}</span>
    </span>
  </label>
</template>
<style scoped>
.radio-item {
  display: inline-flex;
  align-items: flex-start;
  gap: var(--space-2);
  cursor: pointer;
}
.radio-item.disabled {
  cursor: not-allowed;
  opacity: 0.5;
}
.radio-control {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  border: 1.5px solid var(--color-border-strong);
  border-radius: 50%;
  background: var(--color-surface);
  transition: border-color var(--duration-base) var(--ease-standard);
}
.radio-control[data-state='checked'] {
  border-color: var(--color-primary);
}
.radio-indicator {
  display: block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-primary);
}
.radio-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.radio-label {
  font-size: var(--text-sm);
  color: var(--color-text);
  line-height: 1.5;
}
.radio-description {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  line-height: 1.5;
}
.radio-control:focus-visible {
  outline: var(--focus-ring-width) solid var(--focus-ring-color);
  outline-offset: var(--focus-ring-offset);
}
</style>
