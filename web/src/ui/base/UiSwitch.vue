<script setup lang="ts">
import { SwitchRoot, SwitchThumb } from 'reka-ui'

defineOptions({ inheritAttrs: false })
withDefaults(
  defineProps<{
    modelValue?: boolean
    disabled?: boolean
    label?: string
  }>(),
  { modelValue: undefined, disabled: false, label: '' },
)
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  change: [event: Event]
}>()
function handleUpdate(value: boolean) {
  emit('update:modelValue', value)
  emit('change', { target: { checked: value } } as unknown as Event)
}
</script>
<template>
  <SwitchRoot
    :model-value="modelValue"
    :disabled="disabled"
    data-slot="switch"
    class="switch-root"
    :aria-label="label || undefined"
    @update:model-value="handleUpdate"
  >
    <SwitchThumb class="switch-thumb" />
  </SwitchRoot>
</template>
<style scoped>
.switch-root {
  display: inline-flex;
  align-items: center;
  justify-content: flex-start;
  width: 38px;
  height: 22px;
  background: var(--color-border-strong);
  border-radius: 20px;
  padding: 3px;
  flex-shrink: 0;
  cursor: pointer;
  transition: background var(--duration-base) var(--ease-standard);
}
.switch-root[data-disabled] {
  cursor: not-allowed;
  opacity: 0.5;
}
.switch-root[data-state='checked'] {
  background: var(--color-primary);
}
.switch-thumb {
  height: 16px;
  width: 16px;
  border-radius: 50%;
  background: var(--color-surface);
  box-shadow: var(--shadow-sm);
  transition: transform var(--duration-base) var(--ease-standard);
}
.switch-root[data-state='checked'] .switch-thumb {
  transform: translateX(16px);
}
.switch-root:focus-visible {
  outline: var(--focus-ring-width) solid var(--focus-ring-color);
  outline-offset: var(--focus-ring-offset);
}
@media (prefers-reduced-motion: reduce) {
  .switch-root,
  .switch-thumb {
    transition: none;
  }
}
</style>
