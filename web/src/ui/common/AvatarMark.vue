<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    name: string
    src?: string
    size?: number
    tone?: string
    assetRef?: string
    status?: 'online' | 'offline' | 'busy' | 'away' | ''
  }>(),
  { src: '', size: 36, tone: 'blue', assetRef: '', status: '' },
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
const showImage = computed(() => Boolean(props.src))
</script>
<template>
  <span
    class="avatar-mark"
    :class="[tone, assetTone, status ? `status-${status}` : '']"
    :data-avatar-ref="assetRef || undefined"
    :style="{ width: size + 'px', height: size + 'px', fontSize: Math.max(14, size * 0.4) + 'px' }"
    :role="name ? 'img' : undefined"
    :aria-label="name || undefined"
  >
    <img v-if="showImage" :src="src" :alt="name" class="avatar-image" />
    <template v-else>{{ initial }}</template>
    <span v-if="status" class="avatar-status" aria-hidden="true" />
  </span>
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
  font-weight: var(--font-weight-semibold);
  position: relative;
  overflow: visible;
}
.avatar-image {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  object-fit: cover;
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
.avatar-status {
  position: absolute;
  bottom: 0;
  right: 0;
  width: 30%;
  height: 30%;
  min-width: 8px;
  min-height: 8px;
  border-radius: 50%;
  border: 2px solid var(--color-surface);
}
.avatar-mark.status-online .avatar-status { background: var(--color-success); }
.avatar-mark.status-offline .avatar-status { background: var(--color-text-muted); }
.avatar-mark.status-busy .avatar-status { background: var(--color-danger); }
.avatar-mark.status-away .avatar-status { background: var(--color-warning); }
</style>
