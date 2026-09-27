<script setup lang="ts">
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
} from 'reka-ui'
import { UiButton } from '@/ui/base'
import AppIcon from './AppIcon.vue'

defineOptions({ name: 'UiDrawer' })

withDefaults(
  defineProps<{
    open: boolean
    title?: string
    description?: string
    side?: 'right' | 'left'
    width?: string
  }>(),
  { title: '', description: '', side: 'right', width: '360px' },
)
const emit = defineEmits<{ 'update:open': [value: boolean]; close: [] }>()
function handleClose() {
  emit('update:open', false)
  emit('close')
}
</script>
<template>
  <DialogRoot :open="open" @update:open="(v: boolean) => emit('update:open', v)">
    <DialogPortal>
      <DialogOverlay class="drawer-overlay" />
      <DialogContent
        class="drawer-content"
        :data-side="side"
        :style="{ width }"
      >
        <header class="drawer-header">
          <div class="drawer-titles">
          <DialogTitle v-if="title" class="drawer-title">{{ title }}</DialogTitle>
          <DialogDescription v-if="description" class="drawer-desc">{{ description }}</DialogDescription>
          </div>
          <DialogClose as-child>
            <UiButton variant="ghost" size="icon" class="drawer-close" aria-label="关闭" @click="handleClose">
              <AppIcon name="close" :size="18" />
            </UiButton>
          </DialogClose>
        </header>
        <div class="drawer-body">
          <slot />
        </div>
        <footer v-if="$slots.footer" class="drawer-footer">
          <slot name="footer" />
        </footer>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
<style scoped>
.drawer-overlay {
  position: fixed;
  inset: 0;
  z-index: var(--z-dialog);
  background: var(--color-overlay);
  backdrop-filter: blur(2px);
  animation: overlay-in var(--duration-base) var(--ease-standard);
}
.drawer-content {
  position: fixed;
  top: 0;
  bottom: 0;
  z-index: var(--z-dialog);
  display: flex;
  flex-direction: column;
  max-width: 100vw;
  background: var(--color-surface);
  box-shadow: var(--shadow-lg);
  animation: drawer-in var(--duration-slow) var(--ease-emphasized);
}
.drawer-content[data-side='right'] {
  right: 0;
  border-radius: var(--radius-lg) 0 0 var(--radius-lg);
}
.drawer-content[data-side='left'] {
  left: 0;
  border-radius: 0 var(--radius-lg) var(--radius-lg) 0;
}
.drawer-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--color-border);
  flex-shrink: 0;
}
.drawer-titles {
  min-width: 0;
  flex: 1;
}
.drawer-title {
  font-size: var(--text-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--color-text);
}
.drawer-desc {
  margin-top: var(--space-1);
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
}
.drawer-close {
  flex-shrink: 0;
}
.drawer-body {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-5);
  min-width: 0;
}
.drawer-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-2);
  padding: var(--space-4) var(--space-5);
  border-top: 1px solid var(--color-border);
  flex-shrink: 0;
}
@keyframes overlay-in {
  from { opacity: 0; }
  to { opacity: 1; }
}
@keyframes drawer-in {
  from { transform: translateX(100%); }
  to { transform: translateX(0); }
}
.drawer-content[data-side='left'] {
  animation-name: drawer-in-left;
}
@keyframes drawer-in-left {
  from { transform: translateX(-100%); }
  to { transform: translateX(0); }
}
@media (prefers-reduced-motion: reduce) {
  .drawer-overlay,
  .drawer-content {
    animation: none;
  }
}
@media (max-width: 767px) {
  .drawer-content {
    width: 100vw !important;
    border-radius: 0;
  }
  .drawer-body {
    padding: var(--space-4);
  }
  .drawer-header,
  .drawer-footer {
    padding: var(--space-3) var(--space-4);
  }
}
</style>
