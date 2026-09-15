<script setup lang="ts">
withDefaults(
  defineProps<{
    text: string
    tone?: 'success' | 'warning' | 'danger' | 'neutral' | 'primary'
    dot?: boolean
  }>(),
  { tone: 'success', dot: true },
)
</script>
<template>
  <span class="status-badge" :class="tone"><i v-if="dot" />{{ text }}</span>
</template>
<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: var(--text-xs);
  padding: 2px 7px;
  border-radius: 5px;
  white-space: nowrap;
}
.status-badge i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}
.success {
  color: var(--color-success);
  background: var(--color-success-soft);
}
.warning {
  color: var(--color-warning);
  background: var(--color-warning-soft);
}
.danger {
  color: var(--color-danger);
  background: var(--color-danger-soft);
}
.neutral {
  color: var(--color-text-muted);
  background: var(--color-surface-soft);
}
.primary {
  color: var(--color-primary);
  background: var(--color-primary-soft);
}
</style>
