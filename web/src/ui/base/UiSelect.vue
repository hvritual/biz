<script setup lang="ts">
import { ChevronDown } from 'lucide-vue-next'
import { computed, useAttrs } from 'vue'
import { SelectContent, SelectIcon, SelectPortal, SelectRoot, SelectTrigger, SelectValue, SelectViewport } from 'reka-ui'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })
type SelectModelValue = string | number | boolean | null | undefined
const props = withDefaults(
  defineProps<{ placeholder?: string; disabled?: boolean; value?: SelectModelValue }>(),
  { placeholder: '请选择', disabled: false, value: undefined },
)
const emit = defineEmits<{ change: [event: Event] }>()
const model = defineModel<SelectModelValue>({ default: undefined })
const attrs = useAttrs()
const emptyValue = '__coffeelink_empty__'
function toLegacyChangeEvent(value: string): Event {
  const target = { value }
  return { target, currentTarget: target } as unknown as Event
}
const internalValue = computed({
  get: () => {
    const value = model.value ?? props.value
    return value === '' || value == null ? emptyValue : String(value)
  },
  set: (value: string) => {
    const next = value === emptyValue ? '' : value
    model.value = next
    emit('change', toLegacyChangeEvent(next))
  },
})
const triggerClass = computed(() =>
  cn(
    'flex h-[var(--control-height)] w-full items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-input bg-background px-3 text-[var(--text-sm)] text-foreground outline-none focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-ring/20 disabled:cursor-not-allowed disabled:opacity-50',
    attrs.class,
  ),
)
</script>
<template>
  <SelectRoot v-model="internalValue" :disabled="disabled">
    <SelectTrigger
      data-slot="select-trigger"
      :class="triggerClass"
      :aria-label="String($attrs['aria-label'] ?? placeholder)"
    >
      <SelectValue :placeholder="placeholder" />
      <SelectIcon><ChevronDown class="size-4 opacity-60" /></SelectIcon>
    </SelectTrigger>
    <SelectPortal>
      <SelectContent
        data-slot="select-content"
        position="popper"
        class="z-[var(--z-dialog)] min-w-[var(--reka-select-trigger-width)] overflow-hidden rounded-[var(--radius-md)] border border-border bg-popover text-popover-foreground shadow-[var(--shadow-menu)]"
      >
        <SelectViewport class="p-1"><slot :empty-value="emptyValue" /></SelectViewport>
      </SelectContent>
    </SelectPortal>
  </SelectRoot>
</template>
