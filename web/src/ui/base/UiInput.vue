<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })
type InputModelValue = string | number | boolean | string[] | null | undefined
type ModelModifiers = { number?: boolean; trim?: boolean; lazy?: boolean }
const props = withDefaults(
  defineProps<{
    modelValue?: InputModelValue
    modelModifiers?: ModelModifiers
    value?: string | number | null
    checked?: boolean
  }>(),
  { modelValue: undefined },
)
const emit = defineEmits<{
  'update:modelValue': [value: InputModelValue]
  input: [event: Event]
  change: [event: Event]
}>()
const attrs = useAttrs()
const inputType = computed(() => String(attrs.type ?? 'text'))
const checkable = computed(() => inputType.value === 'checkbox' || inputType.value === 'radio')
const displayValue = computed(() => {
  if (inputType.value === 'file') return undefined
  if (checkable.value) return props.value ?? undefined
  return props.modelValue !== undefined ? props.modelValue : props.value
})
const displayChecked = computed(() => {
  if (!checkable.value) return undefined
  if (props.modelValue === undefined) return props.checked
  if (inputType.value === 'radio') return String(props.modelValue ?? '') === String(props.value ?? '')
  if (Array.isArray(props.modelValue)) return props.modelValue.includes(String(props.value ?? ''))
  return Boolean(props.modelValue)
})
const classes = computed(() =>
  cn(
    'flex h-[var(--control-height)] w-full min-w-0 rounded-[var(--radius-sm)] border border-input bg-background px-3 text-[var(--text-sm)] text-foreground shadow-none outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-ring/20 disabled:cursor-not-allowed disabled:opacity-50 file:border-0 file:bg-transparent file:text-sm file:font-medium',
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
  const target = event.target as HTMLInputElement
  if (!checkable.value && !props.modelModifiers?.lazy) emit('update:modelValue', normalizeText(target.value))
  emit('input', event)
}
function handleChange(event: Event) {
  const target = event.target as HTMLInputElement
  if (!checkable.value && props.modelModifiers?.lazy) {
    emit('update:modelValue', normalizeText(target.value))
  } else if (inputType.value === 'checkbox') {
    if (Array.isArray(props.modelValue)) {
      const item = String(props.value ?? '')
      const next = target.checked
        ? [...new Set([...props.modelValue, item])]
        : props.modelValue.filter((value) => value !== item)
      emit('update:modelValue', next)
    } else {
      emit('update:modelValue', target.checked)
    }
  } else if (inputType.value === 'radio' && target.checked) {
    emit('update:modelValue', props.value ?? target.value)
  }
  emit('change', event)
}
</script>
<template>
  <input
    data-slot="input"
    v-bind="$attrs"
    :class="classes"
    :value="displayValue"
    :checked="displayChecked"
    @input="handleInput"
    @change="handleChange"
  />
</template>
