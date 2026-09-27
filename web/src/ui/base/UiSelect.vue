<script setup lang="ts">
import { ChevronDown, X } from 'lucide-vue-next'
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
    clearable?: boolean
    error?: boolean
    value?: SelectModelValue
  }>(),
  { placeholder: '请选择', disabled: false, clearable: false, error: false, modelValue: undefined, modelModifiers: undefined, value: undefined },
)
const emit = defineEmits<{
  'update:modelValue': [value: SelectModelValue]
  change: [event: Event]
  clear: []
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
const hasValue = computed(() => internalValue.value !== emptyValue)
const canClear = computed(() => props.clearable && hasValue.value && !props.disabled)
const explicitAriaLabel = computed(() =>
  attrs['aria-label'] == null ? undefined : String(attrs['aria-label']),
)
const triggerAttrs = computed(() =>
  Object.fromEntries(Object.entries(attrs).filter(([key]) => key !== 'class')),
)
const triggerClass = computed(() =>
  cn(
    'flex h-[var(--control-height)] w-full items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-input bg-background px-3 text-[var(--text-sm)] text-foreground outline-none focus-visible:border-primary focus-visible:outline-[var(--focus-ring-width)] focus-visible:outline-offset-[var(--focus-ring-offset)] disabled:cursor-not-allowed disabled:opacity-50',
    props.error && 'border-[var(--color-danger)] focus-visible:border-[var(--color-danger)]',
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
function clear(event: Event) {
  event.preventDefault()
  event.stopPropagation()
  emit('update:modelValue', '')
  emit('clear')
  emit('change', toLegacyChangeEvent(''))
}
</script>
<template>
  <SelectRoot :model-value="internalValue" :disabled="disabled" @update:model-value="handleUpdate">
    <SelectTrigger
      v-bind="triggerAttrs"
      data-slot="select-trigger"
      :class="triggerClass"
      :aria-label="explicitAriaLabel"
      :aria-invalid="error || undefined"
    >
      <SelectValue :placeholder="placeholder" />
      <span class="flex items-center gap-1.5">
        <button
          v-if="canClear"
          type="button"
          class="select-clear"
          aria-label="清除选择"
          tabindex="-1"
          @click="clear"
        >
          <X :size="14" />
        </button>
        <SelectIcon as-child><ChevronDown class="size-4 opacity-60" /></SelectIcon>
      </span>
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
<style scoped>
.select-clear {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  background: transparent;
  padding: 0;
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  color: var(--color-text-muted);
  cursor: pointer;
  transition: color var(--duration-base) var(--ease-standard);
}
.select-clear:hover {
  color: var(--color-text);
}
</style>
