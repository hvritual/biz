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

withDefaults(
  defineProps<{
    open: boolean
    title: string
    description?: string
    width?: string
    drawer?: boolean
  }>(),
  { description: '', width: '620px', drawer: false },
)
const emit = defineEmits<{ close: [] }>()
function handleOpenChange(open: boolean) {
  if (!open) emit('close')
}
</script>
<template>
  <DialogRoot :open="open" @update:open="handleOpenChange">
    <DialogPortal>
      <DialogOverlay class="dialog-overlay" />
      <DialogContent
        class="dialog-panel"
        :class="{ drawer }"
        :style="{ width }"
        :aria-describedby="description ? 'dialog-desc' : undefined"
        @interact-outside="emit('close')"
        @escape-keydown="emit('close')"
      >
        <header class="dialog-header">
          <DialogTitle class="dialog-title">{{ title }}</DialogTitle>
          <DialogDescription v-if="description" id="dialog-desc" class="dialog-desc">
            {{ description }}
          </DialogDescription>
          <DialogClose as-child>
            <UiButton variant="ghost" size="icon" class="dialog-close" aria-label="关闭弹窗">
              <AppIcon name="close" />
            </UiButton>
          </DialogClose>
        </header>
        <div class="dialog-body"><slot /></div>
        <footer v-if="$slots.footer" class="dialog-footer"><slot name="footer" /></footer>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
<style scoped>
.dialog-overlay {
  position: fixed;
  inset: 0;
  z-index: var(--z-dialog);
  background: var(--color-overlay);
  backdrop-filter: blur(2px);
  animation: overlay-in var(--duration-base) var(--ease-standard);
}
.dialog-panel {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: var(--z-dialog);
  display: flex;
  flex-direction: column;
  max-width: calc(100vw - 32px);
  max-height: calc(100dvh - 48px);
  background: var(--color-surface);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
  animation: dialog-in var(--duration-base) var(--ease-emphasized);
}
.dialog-panel.drawer {
  top: 0;
  bottom: 0;
  right: 0;
  left: auto;
  transform: none;
  height: 100dvh;
  max-height: 100dvh;
  max-width: 100vw;
  border-radius: var(--radius-lg) 0 0 var(--radius-lg);
  animation: drawer-in var(--duration-slow) var(--ease-emphasized);
}
.dialog-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-4) var(--space-6);
  border-bottom: 1px solid var(--color-border);
  flex-shrink: 0;
}
.dialog-title {
  font-size: var(--text-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--color-text);
  flex: 1;
  min-width: 0;
}
.dialog-desc {
  margin-top: var(--space-1);
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
}
.dialog-close {
  flex-shrink: 0;
}
.dialog-body {
  padding: var(--space-6);
  overflow-y: auto;
  flex: 1;
  min-width: 0;
}
.dialog-panel.drawer .dialog-body {
  flex: 1;
}
.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  padding: var(--space-4) var(--space-6);
  border-top: 1px solid var(--color-border);
  flex-shrink: 0;
}
@keyframes overlay-in {
  from { opacity: 0; }
  to { opacity: 1; }
}
@keyframes dialog-in {
  from { opacity: 0; transform: translate(-50%, -48%) scale(0.96); }
  to { opacity: 1; transform: translate(-50%, -50%) scale(1); }
}
@keyframes drawer-in {
  from { transform: translateX(100%); }
  to { transform: translateX(0); }
}
@media (prefers-reduced-motion: reduce) {
  .dialog-overlay,
  .dialog-panel {
    animation: none;
  }
}
@media (max-width: 767px) {
  .dialog-panel {
    max-height: calc(100dvh - 24px);
  }
  .dialog-body {
    padding: var(--space-4);
  }
  .dialog-header,
  .dialog-footer {
    padding: var(--space-3) var(--space-4);
  }
  .dialog-panel.drawer {
    width: 100vw !important;
    border-radius: 0;
  }
}
</style>
