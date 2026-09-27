<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{ name: string; size?: number; tone?: string; assetRef?: string }>(),
  { size: 36, tone: 'blue', assetRef: '' },
)

const assetTone = computed(() => {
  switch (props.assetRef) {
    case 'avatar:coffee-violet': return 'asset-violet'
    case 'avatar:coffee-emerald': return 'asset-emerald'
    case 'avatar:coffee-amber': return 'asset-amber'
    case 'avatar:coffee-blue': return 'asset-blue'
    default: return ''
  }
})
const initial = computed(() => props.name.trim().slice(0, 1) || 'U')
</script>
<template>
  <span
    class="avatar-mark"
    :class="[tone, assetTone]"
    :data-avatar-ref="assetRef || undefined"
    :style="{ width: size + 'px', height: size + 'px', fontSize: Math.max(14, size * 0.4) + 'px' }"
    aria-hidden="true"
    >{{ initial }}</span
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
  font-weight: 600;
}
.avatar-mark.solid {
  background: linear-gradient(135deg, var(--color-gradient-end), var(--color-primary));
  color: var(--color-on-primary);
}
.avatar-mark.violet,
.avatar-mark.asset-violet {
  background: var(--color-violet-soft);
  color: var(--color-violet);
}
.avatar-mark.asset-blue {
  background: var(--color-primary-soft);
  color: var(--color-primary);
}
.avatar-mark.asset-emerald {
  background: var(--color-success-soft);
  color: var(--color-success);
}
.avatar-mark.asset-amber {
  background: var(--color-warning-soft);
  color: var(--color-warning);
}
</style>
