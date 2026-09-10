<script setup lang="ts">
withDefaults(defineProps<{ name: string; size?: number; tone?: string }>(), { size: 36, tone: 'blue' })
</script>
<template>
  <span
    class="avatar-mark"
    :class="tone"
    :style="{ width: size + 'px', height: size + 'px', fontSize: Math.max(14, size * 0.4) + 'px' }"
    aria-hidden="true"
    >{{ name.slice(0, 1) }}</span
  >
</template>
<style scoped>
.avatar-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--color-primary-soft), var(--color-border));
  color: var(--color-primary);
  font-weight: 500;
}
.avatar-mark.solid {
  background: linear-gradient(135deg, var(--color-gradient-end), var(--color-primary));
  color: var(--color-on-primary);
}
.avatar-mark.violet {
  background: var(--color-violet-soft);
  color: var(--color-violet);
}
</style>
