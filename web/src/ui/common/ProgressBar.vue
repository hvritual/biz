<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    value?: number
    max?: number
    tone?: 'primary' | 'success' | 'warning' | 'danger'
    size?: 'sm' | 'md'
    indeterminate?: boolean
    label?: string
  }>(),
  { value: 0, max: 100, tone: 'primary', size: 'md', indeterminate: false, label: '' },
)
const percentage = computed(() => {
  if (props.indeterminate) return 100
  return Math.max(0, Math.min(100, (props.value / props.max) * 100))
})
</script>
<template>
  <div class="progress-wrapper" :data-size="size">
    <progress
      v-if="!indeterminate"
      class="progress-native"
      :value="value"
      :max="max"
      :aria-label="label || undefined"
    />
    <div v-else class="progress-track" role="progressbar" :aria-label="label || undefined" aria-valuenow="0" aria-valuemin="0" aria-valuemax="100">
      <div class="progress-fill progress-indeterminate" />
    </div>
    <div v-if="!indeterminate" class="progress-track" role="progressbar" :aria-label="label || undefined" :aria-valuenow="value" :aria-valuemin="0" :aria-valuemax="max">
      <div class="progress-fill" :data-tone="tone" :style="{ width: percentage + '%' }" />
    </div>
  </div>
</template>
<style scoped>
.progress-wrapper {
  width: 100%;
}
.progress-native {
  display: none;
}
.progress-track {
  height: 7px;
  border-radius: 999px;
  background: var(--color-border);
  overflow: hidden;
}
.progress-wrapper[data-size='sm'] .progress-track {
  height: 4px;
}
.progress-fill {
  height: 100%;
  border-radius: inherit;
  background: var(--color-primary);
  transition: width var(--duration-slow) var(--ease-standard);
}
.progress-fill[data-tone='primary'] { background: var(--color-primary); }
.progress-fill[data-tone='success'] { background: var(--color-success); }
.progress-fill[data-tone='warning'] { background: var(--color-warning); }
.progress-fill[data-tone='danger'] { background: var(--color-danger); }
.progress-indeterminate {
  width: 40% !important;
  animation: progress-slide var(--duration-slow) var(--ease-in-out) infinite;
}
@keyframes progress-slide {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(350%); }
}
@media (prefers-reduced-motion: reduce) {
  .progress-fill {
    transition: none;
  }
  .progress-indeterminate {
    animation: none;
  }
}
</style>
