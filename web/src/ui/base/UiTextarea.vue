<script setup lang="ts">
import { computed, nextTick, ref, useAttrs, watch } from 'vue'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })
type TextareaModelValue = string | number | null | undefined
type ModelModifiers = { number?: boolean; trim?: boolean; lazy?: boolean }
const props = withDefaults(
  defineProps<{
    modelValue?: TextareaModelValue
    modelModifiers?: ModelModifiers
    value?: TextareaModelValue
    error?: boolean
    maxlength?: string | number
    showCount?: boolean
    autoResize?: boolean
  }>(),
  { error: false, maxlength: undefined, showCount: false, autoResize: false },
)
const emit = defineEmits<{
  'update:modelValue': [value: TextareaModelValue]
  input: [event: Event]
  change: [event: Event]
}>()
const attrs = useAttrs()
const textarea = ref<HTMLTextAreaElement | null>(null)
const displayValue = computed(() => (props.modelValue !== undefined ? props.modelValue : props.value))
const currentLength = computed(() => String(displayValue.value ?? '').length)
const useWrapper = computed(() => props.showCount || props.autoResize)
const wrapperClasses = computed(() =>
  cn(
    'flex flex-col min-h-22 w-full rounded-[var(--radius-sm)] border border-input bg-background transition-colors focus-within:border-primary focus-within:outline-[var(--focus-ring-width)] focus-within:outline-[var(--focus-ring-color)] focus-within:outline-offset-[var(--focus-ring-offset)] disabled:opacity-50',
    props.error && 'border-[var(--color-danger)] focus-within:border-[var(--color-danger)]',
    attrs.class,
  ),
)
const bareClasses = computed(() =>
  cn(
    'flex min-h-22 w-full rounded-[var(--radius-sm)] border border-input bg-background px-3 py-2 text-[var(--text-sm)] text-foreground outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-primary focus-visible:outline-[var(--focus-ring-width)] focus-visible:outline-offset-[var(--focus-ring-offset)] disabled:cursor-not-allowed disabled:opacity-50',
    props.error && 'border-[var(--color-danger)] focus-visible:border-[var(--color-danger)]',
    attrs.class,
  ),
)
const innerClasses = 'min-w-0 flex-1 border-0 bg-transparent px-3 py-2 text-[var(--text-sm)] text-foreground outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed resize-none'
const innerAttrs = computed(() =>
  Object.fromEntries(Object.entries(attrs).filter(([key]) => key !== 'class')),
)
function normalizeText(value: string): string | number {
  const trimmed = props.modelModifiers?.trim ? value.trim() : value
  if (!props.modelModifiers?.number) return trimmed
  const parsed = Number.parseFloat(trimmed)
  return Number.isNaN(parsed) ? trimmed : parsed
}
function doAutoResize() {
  if (!props.autoResize || !textarea.value) return
  const el = textarea.value
  el.style.height = 'auto'
  el.style.height = `${el.scrollHeight}px`
}
function handleInput(event: Event) {
  if (!props.modelModifiers?.lazy) {
    emit('update:modelValue', normalizeText((event.target as HTMLTextAreaElement).value))
  }
  emit('input', event)
  if (props.autoResize) nextTick(doAutoResize)
}
function handleChange(event: Event) {
  if (props.modelModifiers?.lazy) {
    emit('update:modelValue', normalizeText((event.target as HTMLTextAreaElement).value))
  }
  emit('change', event)
}
watch(() => props.modelValue, () => {
  if (props.autoResize) nextTick(doAutoResize)
})
</script>
<template>
  <!-- Bare textarea (no wrapper) -->
  <textarea
    v-if="!useWrapper"
    ref="textarea"
    data-slot="textarea"
    v-bind="$attrs"
    :class="bareClasses"
    :value="displayValue"
    :aria-invalid="error || undefined"
    @input="handleInput"
    @change="handleChange"
  />
  <!-- Wrapped: supports char count + auto-resize -->
  <div
    v-else
    data-slot="textarea-wrapper"
    :class="wrapperClasses"
    :aria-invalid="error || undefined"
  >
    <textarea
      ref="textarea"
      data-slot="textarea"
      v-bind="innerAttrs"
      :class="innerClasses"
      :value="displayValue"
      :maxlength="maxlength"
      @input="handleInput"
      @change="handleChange"
    />
    <div v-if="showCount" class="textarea-count">
      <span>{{ currentLength }}</span>
      <span v-if="maxlength"> / {{ maxlength }}</span>
    </div>
  </div>
</template>
<style scoped>
.textarea-count {
  display: flex;
  justify-content: flex-end;
  gap: 2px;
  padding: 0 var(--space-3) var(--space-2);
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  font-variant-numeric: tabular-nums;
}
</style>
