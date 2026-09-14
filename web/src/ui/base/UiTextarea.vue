<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })
type TextareaModelValue = string | number | null | undefined
type ModelModifiers = { number?: boolean; trim?: boolean; lazy?: boolean }
const props = defineProps<{
  modelValue?: TextareaModelValue
  modelModifiers?: ModelModifiers
  value?: TextareaModelValue
}>()
const emit = defineEmits<{
  'update:modelValue': [value: TextareaModelValue]
  input: [event: Event]
  change: [event: Event]
}>()
const attrs = useAttrs()
const displayValue = computed(() => (props.modelValue !== undefined ? props.modelValue : props.value))
const classes = computed(() =>
  cn(
    'flex min-h-22 w-full rounded-[var(--radius-sm)] border border-input bg-background px-3 py-2 text-[var(--text-sm)] text-foreground outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-ring/20 disabled:cursor-not-allowed disabled:opacity-50',
    attrs.class,
  ),
)
function normalizeText(value: string): string | number {
  const trimmed = props.modelModifiers?.trim ? value.trim() : value
  if (!props.modelModifiers?.number) return trimmed
  const parsed = Number.parseFloat(trimmed)
  return Number.isNaN(parsed) ? trimmed : parsed
}
function handleInput(event: Event) {
  if (!props.modelModifiers?.lazy) {
    emit('update:modelValue', normalizeText((event.target as HTMLTextAreaElement).value))
  }
  emit('input', event)
}
function handleChange(event: Event) {
  if (props.modelModifiers?.lazy) {
    emit('update:modelValue', normalizeText((event.target as HTMLTextAreaElement).value))
  }
  emit('change', event)
}
</script>
<template>
  <textarea
    data-slot="textarea"
    v-bind="$attrs"
    :class="classes"
    :value="displayValue"
    @input="handleInput"
    @change="handleChange"
  />
</template>
