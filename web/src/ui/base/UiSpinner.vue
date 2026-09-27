<script setup lang="ts">
withDefaults(
  defineProps<{ size?: number; label?: string }>(),
  { size: 18, label: '' },
)
</script>
<template>
  <span class="spinner" role="status" :aria-label="label || undefined" data-slot="spinner">
    <svg
      :width="size"
      :height="size"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" opacity="0.25" />
      <path
        d="M22 12a10 10 0 0 1-10 10"
        stroke="currentColor"
        stroke-width="3"
        stroke-linecap="round"
      />
    </svg>
    <span v-if="label" class="spinner-label">{{ label }}</span>
  </span>
</template>
<style scoped>
.spinner {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  color: var(--color-primary);
}
.spinner svg {
  animation: spinner-rotate var(--duration-slow) linear infinite;
}
.spinner-label {
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
}
@keyframes spinner-rotate {
  to {
    transform: rotate(360deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .spinner svg {
    animation: none;
  }
}
</style>
