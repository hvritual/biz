<script setup lang="ts">
import { TabsList, TabsRoot } from 'reka-ui'

defineOptions({ inheritAttrs: false })
withDefaults(
  defineProps<{
    modelValue?: string
    label?: string
  }>(),
  { modelValue: undefined, label: '' },
)
const emit = defineEmits<{
  'update:modelValue': [value: string]
  change: [event: Event]
}>()
function handleUpdate(value: string) {
  emit('update:modelValue', value)
  emit('change', { target: { value } } as unknown as Event)
}
</script>
<template>
  <TabsRoot
    :model-value="modelValue"
    data-slot="tabs"
    class="tabs-root"
    :aria-label="label || undefined"
    @update:model-value="handleUpdate"
  >
    <TabsList class="tabs-list" :aria-label="label || undefined">
      <slot name="list" />
    </TabsList>
    <slot />
  </TabsRoot>
</template>
<style scoped>
.tabs-root {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}
.tabs-list {
  display: flex;
  gap: 28px;
  border-bottom: 1px solid var(--color-border);
  overflow-x: auto;
}
</style>
