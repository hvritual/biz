<script setup lang="ts">
withDefaults(
  defineProps<{ width?: string; height?: string; rounded?: 'sm' | 'md' | 'lg' | 'full' }>(),
  { width: '100%', height: '14px', rounded: 'sm' },
)
</script>
<template>
  <span
    class="skeleton"
    :class="`rounded-${rounded}`"
    :style="{ width, height }"
    aria-hidden="true"
    data-slot="skeleton"
  />
</template>
<style scoped>
.skeleton {
  display: inline-block;
  background: var(--color-surface-soft);
  position: relative;
  overflow: hidden;
}
.skeleton.rounded-sm { border-radius: var(--radius-sm); }
.skeleton.rounded-md { border-radius: var(--radius-md); }
.skeleton.rounded-lg { border-radius: var(--radius-lg); }
.skeleton.rounded-full { border-radius: 9999px; }
.skeleton::after {
  content: '';
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(90deg, transparent, var(--color-border), transparent);
  animation: skeleton-shimmer var(--duration-slow) var(--ease-standard) infinite;
}
@keyframes skeleton-shimmer {
  to {
    transform: translateX(100%);
  }
}
@media (prefers-reduced-motion: reduce) {
  .skeleton::after {
    animation: none;
  }
}
</style>
