<script setup lang="ts">
import { X } from 'lucide-vue-next'

defineOptions({ inheritAttrs: false })
withDefaults(
  defineProps<{
    tone?: 'default' | 'primary' | 'success' | 'warning' | 'danger' | 'info'
    size?: 'sm' | 'md'
    removable?: boolean
  }>(),
  { tone: 'default', size: 'sm', removable: false },
)
const emit = defineEmits<{ remove: [] }>()
</script>
<template>
  <span
    class="tag"
    :data-tone="tone"
    :data-size="size"
    data-slot="tag"
  >
    <span class="tag-text"><slot /></span>
    <button
      v-if="removable"
      type="button"
      class="tag-remove"
      aria-label="移除"
      @click="emit('remove')"
    >
      <X :size="12" />
    </button>
  </span>
</template>
<style scoped>
.tag {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  white-space: nowrap;
  border-radius: var(--radius-sm);
  font-weight: var(--font-weight-medium);
  flex-shrink: 0;
}
.tag[data-size='sm'] {
  font-size: var(--text-xs);
  padding: 2px 7px;
}
.tag[data-size='md'] {
  font-size: var(--text-sm);
  padding: var(--space-1) var(--space-2);
}
.tag[data-tone='default'] {
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
}
.tag[data-tone='primary'] {
  background: var(--color-primary-soft);
  color: var(--color-primary);
}
.tag[data-tone='success'] {
  background: var(--color-success-soft);
  color: var(--color-success);
}
.tag[data-tone='warning'] {
  background: var(--color-warning-soft);
  color: var(--color-warning);
}
.tag[data-tone='danger'] {
  background: var(--color-danger-soft);
  color: var(--color-danger);
}
.tag[data-tone='info'] {
  background: var(--color-info-soft);
  color: var(--color-info);
}
.tag-remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  background: transparent;
  padding: 0;
  width: 14px;
  height: 14px;
  color: currentColor;
  opacity: 0.6;
  cursor: pointer;
  transition: opacity var(--duration-base) var(--ease-standard);
}
.tag-remove:hover {
  opacity: 1;
}
.tag-remove:focus-visible {
  outline: var(--focus-ring-width) solid var(--focus-ring-color);
  outline-offset: var(--focus-ring-offset);
}
</style>
