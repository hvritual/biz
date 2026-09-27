<script setup lang="ts">
import { Check, Minus } from 'lucide-vue-next'
import { CheckboxIndicator, CheckboxRoot } from 'reka-ui'

defineOptions({ inheritAttrs: false })
withDefaults(
  defineProps<{
    modelValue?: boolean | string[]
    value?: string | number | null
    disabled?: boolean
    indeterminate?: boolean
    label?: string
    description?: string
  }>(),
  {
    modelValue: undefined,
    value: null,
    disabled: false,
    indeterminate: false,
    label: '',
    description: '',
  },
)
const emit = defineEmits<{
  'update:modelValue': [value: boolean | string[]]
  change: [event: Event]
}>()
function handleUpdate(value: boolean | string[] | 'indeterminate') {
  if (value === 'indeterminate') return
  emit('update:modelValue', value)
  emit('change', { target: { checked: value } } as unknown as Event)
}
</script>
<template>
  <CheckboxRoot
    :model-value="modelValue"
    :value="value ?? undefined"
    :disabled="disabled"
    data-slot="checkbox"
    class="checkbox-root"
    @update:model-value="handleUpdate"
  >
    <CheckboxIndicator class="checkbox-indicator" :aria-hidden="true">
      <Minus v-if="indeterminate" :size="14" />
      <Check v-else :size="14" />
    </CheckboxIndicator>
    <span v-if="label || description" class="checkbox-text">
      <span class="checkbox-label">{{ label }}</span>
      <span v-if="description" class="checkbox-description">{{ description }}</span>
    </span>
  </CheckboxRoot>
</template>
<style scoped>
.checkbox-root {
  display: inline-flex;
  align-items: flex-start;
  gap: var(--space-2);
  cursor: pointer;
}
.checkbox-root[data-disabled] {
  cursor: not-allowed;
  opacity: 0.5;
}
.checkbox-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  border: 1.5px solid var(--color-border-strong);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-on-primary);
  transition: background var(--duration-base) var(--ease-standard), border-color var(--duration-base) var(--ease-standard);
}
.checkbox-root[data-state='checked'] .checkbox-indicator {
  background: var(--color-primary);
  border-color: var(--color-primary);
}
.checkbox-root[data-state='indeterminate'] .checkbox-indicator {
  background: var(--color-primary);
  border-color: var(--color-primary);
}
.checkbox-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.checkbox-label {
  font-size: var(--text-sm);
  color: var(--color-text);
  line-height: 1.5;
}
.checkbox-description {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  line-height: 1.5;
}
.checkbox-root:focus-visible {
  outline: var(--focus-ring-width) solid var(--focus-ring-color);
  outline-offset: var(--focus-ring-offset);
}
</style>
