<script setup lang="ts">
import { TabsTrigger } from 'reka-ui'

defineOptions({ inheritAttrs: false })
defineProps<{
  value: string
  disabled?: boolean
}>()
</script>
<template>
  <TabsTrigger
    :value="value"
    :disabled="disabled"
    data-slot="tab-trigger"
    class="tab-trigger"
  >
    <slot />
  </TabsTrigger>
</template>
<style scoped>
.tab-trigger {
  height: 45px;
  white-space: nowrap;
  border: 0;
  border-bottom: 2px solid transparent;
  padding: 0 4px;
  background: transparent;
  color: var(--color-text-secondary);
  font-size: var(--text-sm);
  font-weight: 500;
  cursor: pointer;
  transition: color var(--duration-base) var(--ease-standard), border-color var(--duration-base) var(--ease-standard);
}
.tab-trigger[data-state='active'] {
  color: var(--color-primary);
  border-color: var(--color-primary);
  font-weight: 600;
}
.tab-trigger:hover:not([data-state='active']):not(:disabled) {
  color: var(--color-text);
}
.tab-trigger:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}
.tab-trigger:focus-visible {
  outline: var(--focus-ring-width) solid var(--focus-ring-color);
  outline-offset: var(--focus-ring-offset);
}
@media (max-width: 767px) {
  .tab-trigger {
    height: var(--touch-target-min);
  }
}
</style>
