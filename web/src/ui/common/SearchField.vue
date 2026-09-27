<script setup lang="ts">
import { X } from 'lucide-vue-next'
import { ref, watch } from 'vue'
import { UiButton, UiInput } from '@/ui/base'
import AppIcon from './AppIcon.vue'

const props = withDefaults(
  defineProps<{
    modelValue: string
    placeholder?: string
    label?: string
    debounce?: number
    clearable?: boolean
  }>(),
  { placeholder: '', label: '', debounce: 0, clearable: true },
)
const emit = defineEmits<{
  'update:modelValue': [value: string]
  search: [value: string]
}>()
const inner = ref(props.modelValue)
let timer: ReturnType<typeof setTimeout> | null = null
watch(() => props.modelValue, (v) => { inner.value = v })
function onInput(event: Event) {
  const value = (event.target as HTMLInputElement).value
  inner.value = value
  emit('update:modelValue', value)
  if (props.debounce > 0) {
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => emit('search', value), props.debounce)
  } else {
    emit('search', value)
  }
}
function clear() {
  inner.value = ''
  emit('update:modelValue', '')
  emit('search', '')
}
</script>
<template>
  <label class="search-field">
    <AppIcon name="search" :size="16" />
    <span class="sr-only">{{ label || placeholder || '搜索' }}</span>
    <UiInput
      :value="inner"
      :placeholder="placeholder"
      @input="onInput"
    />
    <UiButton
      v-if="clearable && inner"
      variant="ghost"
      size="icon"
      class="search-clear"
      aria-label="清除搜索"
      @click="clear"
    >
      <X :size="14" />
    </UiButton>
  </label>
</template>
<style scoped>
.search-field {
  display: flex;
  gap: var(--space-2);
  align-items: center;
  border: 1px solid var(--color-border-strong);
  border-radius: var(--radius-sm);
  height: var(--control-height);
  padding: 0 var(--space-3);
  color: var(--color-text-muted);
  background: var(--color-surface);
}
.search-field :deep([data-slot='input-wrapper']) {
  border: 0;
  background: transparent;
  height: auto;
  padding: 0;
  gap: 0;
  box-shadow: none;
}
.search-field :deep([data-slot='input-wrapper']:focus-within) {
  outline: none;
  border: 0;
}
.search-field :deep([data-slot='input']) {
  font-size: var(--text-sm);
  outline: 0;
  border: 0;
  min-width: 0;
  width: 100%;
  height: auto;
  background: transparent;
}
.search-field :deep([data-slot='input']:focus-visible) {
  outline: none;
}
.search-field:focus-within {
  outline: var(--focus-ring-width) solid var(--focus-ring-color);
  outline-offset: var(--focus-ring-offset);
}
.search-clear {
  width: 20px !important;
  height: 20px !important;
  flex-shrink: 0;
  color: var(--color-text-muted);
}
.search-clear:hover {
  color: var(--color-text) !important;
  background: transparent !important;
}
</style>
