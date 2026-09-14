<script setup lang="ts">
import AppIcon from './AppIcon.vue'
defineProps<{ modelValue: string; placeholder?: string; label?: string }>()
defineEmits<{ 'update:modelValue': [value: string] }>()
</script>
<template>
  <label class="search-field"
    ><AppIcon name="search" :size="16" /><span class="sr-only">{{ label || placeholder || '搜索' }}</span
    ><input
      :value="modelValue"
      :placeholder="placeholder"
      @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
  /></label>
</template>
<style scoped>
.search-field {
  display: flex;
  gap: 9px;
  align-items: center;
  border: 1px solid var(--color-border-strong);
  border-radius: var(--radius-sm);
  height: var(--control-height);
  padding: 0 12px;
  color: var(--color-text-muted);
  background: var(--color-surface);
}
.search-field input {
  font-size: var(--text-sm);
  outline: 0;
  border: 0;
  min-width: 0;
  width: 100%;
  background: transparent;
}
.search-field:focus-within {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
.search-field input::placeholder {
  color: var(--color-text-muted);
}
</style>
