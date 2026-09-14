<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })
type TextareaModelValue = string | number | null | undefined
const props = defineProps<{ modelValue?: TextareaModelValue; value?: TextareaModelValue }>()
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
function handleInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLTextAreaElement).value)
  emit('input', event)
}
function handleChange(event: Event) {
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
