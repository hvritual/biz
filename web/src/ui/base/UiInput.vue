<script setup lang="ts">
import { X } from 'lucide-vue-next'
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
    indeterminate?: boolean
    error?: boolean
    clearable?: boolean
  }>(),
  {
    modelValue: undefined,
    modelModifiers: undefined,
    value: undefined,
    checked: undefined,
    indeterminate: false,
    error: false,
    clearable: false,
  },
)
const emit = defineEmits<{
  'update:modelValue': [value: InputModelValue]
  input: [event: Event]
  change: [event: Event]
  clear: []
}>()
const attrs = useAttrs()
const inputType = computed(() => String(attrs.type ?? 'text'))
const checkable = computed(() => inputType.value === 'checkbox' || inputType.value === 'radio')
const isFile = computed(() => inputType.value === 'file')
const useWrapper = computed(() => !checkable.value && !isFile.value)
const innerAttrs = computed(() =>
  Object.fromEntries(Object.entries(attrs).filter(([key]) => key !== 'class')),
)
const displayValue = computed(() => {
  if (isFile.value) return undefined
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
const canClear = computed(() => {
  if (!props.clearable || checkable.value || isFile.value) return false
  const val = props.modelValue !== undefined ? props.modelValue : props.value
  return val !== '' && val != null && val !== false
})
const wrapperClasses = computed(() =>
  cn(
    'flex h-[var(--control-height)] w-full min-w-0 items-center gap-2 rounded-[var(--radius-sm)] border border-input bg-background px-3 text-[var(--text-sm)] text-foreground shadow-none outline-none transition-colors placeholder:text-muted-foreground focus-within:border-primary focus-within:outline-[var(--focus-ring-width)] focus-within:outline-[var(--focus-ring-color)] focus-within:outline-offset-[var(--focus-ring-offset)] disabled:cursor-not-allowed disabled:opacity-50',
    props.error && 'border-[var(--color-danger)] focus-within:border-[var(--color-danger)] focus-within:outline-[var(--color-danger)]',
    attrs.class,
  ),
)
const bareInputClasses = computed(() =>
  cn(
    'flex h-[var(--control-height)] w-full min-w-0 rounded-[var(--radius-sm)] border border-input bg-background px-3 text-[var(--text-sm)] text-foreground shadow-none outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-primary focus-visible:outline-[var(--focus-ring-width)] focus-visible:outline-offset-[var(--focus-ring-offset)] disabled:cursor-not-allowed disabled:opacity-50 file:border-0 file:bg-transparent file:text-sm file:font-medium',
    props.error && 'border-[var(--color-danger)] focus-visible:border-[var(--color-danger)]',
    attrs.class,
  ),
)
const innerInputClasses = 'min-w-0 flex-1 border-0 bg-transparent p-0 text-[var(--text-sm)] text-foreground outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50 file:border-0 file:bg-transparent file:text-sm file:font-medium'
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
function clear() {
  emit('update:modelValue', '')
  emit('clear')
}
</script>
<template>
  <!-- Checkable / file: single input, no wrapper -->
  <input
    v-if="!useWrapper"
    data-slot="input"
    v-bind="$attrs"
    :class="bareInputClasses"
    :value="displayValue"
    :checked="displayChecked"
    :indeterminate="inputType === 'checkbox' && Boolean(props.indeterminate)"
    :aria-checked="props.indeterminate ? 'mixed' : undefined"
    :aria-invalid="error || undefined"
    @input="handleInput"
    @change="handleChange"
  />
  <!-- Text-like: wrapper with prefix/suffix/clear support -->
  <div
    v-else
    data-slot="input-wrapper"
    :class="wrapperClasses"
    :aria-invalid="error || undefined"
  >
    <slot name="prefix" />
    <input
      data-slot="input"
      v-bind="innerAttrs"
      :class="innerInputClasses"
      :value="displayValue"
      @input="handleInput"
      @change="handleChange"
    />
    <button
      v-if="canClear"
      type="button"
      class="clear-button"
      aria-label="清除"
      tabindex="-1"
      @click="clear"
    >
      <X :size="14" />
    </button>
    <slot name="suffix" />
  </div>
</template>
<style scoped>
.clear-button {
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
.clear-button:hover {
  color: var(--color-text);
}
</style>
