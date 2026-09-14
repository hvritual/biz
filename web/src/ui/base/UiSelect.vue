<script setup lang="ts">
import { ChevronDown } from 'lucide-vue-next'
import { computed, useAttrs } from 'vue'
import { SelectContent, SelectIcon, SelectPortal, SelectRoot, SelectTrigger, SelectValue, SelectViewport } from 'reka-ui'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })
type SelectModelValue = string | number | boolean | null | undefined
type ModelModifiers = { number?: boolean; trim?: boolean }
const props = withDefaults(
  defineProps<{
    modelValue?: SelectModelValue
    modelModifiers?: ModelModifiers
    placeholder?: string
    disabled?: boolean
    value?: SelectModelValue
  }>(),
  { placeholder: '请选择', disabled: false, modelValue: undefined, modelModifiers: undefined, value: undefined },
)
const emit = defineEmits<{
  'update:modelValue': [value: SelectModelValue]
  change: [event: Event]
}>()
const attrs = useAttrs()
const emptyValue = '__coffeelink_empty__'
function toLegacyChangeEvent(value: SelectModelValue): Event {
  const target = { value }
  return { target, currentTarget: target } as unknown as Event
}
const internalValue = computed(() => {
  const value = props.modelValue !== undefined ? props.modelValue : props.value
  return value === '' || value == null ? emptyValue : String(value)
})
const explicitAriaLabel = computed(() =>
  attrs['aria-label'] == null ? undefined : String(attrs['aria-label']),
)
const triggerAttrs = computed(() => {
  const { class: _class, ...rest } = attrs
  return rest
})
const triggerClass = computed(() =>
  cn(
    'flex h-[var(--control-height)] w-full items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-input bg-background px-3 text-[var(--text-sm)] text-foreground outline-none focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-ring/20 disabled:cursor-not-allowed disabled:opacity-50',
    attrs.class,
  ),
)
function normalizeModelValue(value: string): string | number {
  const trimmed = props.modelModifiers?.trim ? value.trim() : value
  if (!props.modelModifiers?.number) return trimmed
  const parsed = Number.parseFloat(trimmed)
  return Number.isNaN(parsed) ? trimmed : parsed
}
function handleUpdate(value: unknown) {
  const normalized = String(value ?? emptyValue)
  const next = normalized === emptyValue ? '' : normalizeModelValue(normalized)
  emit('update:modelValue', next)
  emit('change', toLegacyChangeEvent(next))
}
</script>
<template>
  <SelectRoot :model-value="internalValue" :disabled="disabled" @update:model-value="handleUpdate">
    <SelectTrigger
      v-bind="triggerAttrs"
      data-slot="select-trigger"
      :class="triggerClass"
      :aria-label="explicitAriaLabel"
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
