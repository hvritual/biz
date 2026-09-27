<script setup lang="ts">
import { AccordionRoot } from 'reka-ui'

defineOptions({ inheritAttrs: false })
withDefaults(
  defineProps<{
    modelValue?: string | string[]
    type?: 'single' | 'multiple'
    collapsible?: boolean
  }>(),
  { modelValue: undefined, type: 'single', collapsible: true },
)
const emit = defineEmits<{ 'update:modelValue': [value: string | string[]] }>()
function handleUpdate(value: string | string[] | undefined) {
  emit('update:modelValue', value ?? '')
}
</script>
<template>
  <AccordionRoot
    :type="type"
    :model-value="modelValue"
    :collapsible="collapsible"
    data-slot="accordion"
    class="accordion-root"
    @update:model-value="handleUpdate"
  >
    <slot />
  </AccordionRoot>
</template>
<style scoped>
.accordion-root {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
}
</style>
