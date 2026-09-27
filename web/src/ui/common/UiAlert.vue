<script setup lang="ts">
import {
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogOverlay,
  AlertDialogPortal,
  AlertDialogRoot,
  AlertDialogTitle,
} from 'reka-ui'
import { UiButton } from '@/ui/base'

withDefaults(
  defineProps<{
    open: boolean
    title: string
    description?: string
    confirmText?: string
    cancelText?: string
    tone?: 'default' | 'danger'
  }>(),
  { confirmText: '确认', cancelText: '取消', tone: 'default', description: '' },
)
const emit = defineEmits<{
  'update:open': [value: boolean]
  confirm: []
  cancel: []
}>()
function handleConfirm() {
  emit('confirm')
  emit('update:open', false)
}
function handleCancel() {
  emit('cancel')
  emit('update:open', false)
}
</script>
<template>
  <AlertDialogRoot :open="open" @update:open="(v: boolean) => emit('update:open', v)">
    <AlertDialogPortal>
      <AlertDialogOverlay class="alert-overlay" />
      <AlertDialogContent
        class="alert-content"
        :data-tone="tone"
        role="alertdialog"
        :aria-describedby="description ? 'alert-desc' : undefined"
      >
        <AlertDialogTitle class="alert-title">{{ title }}</AlertDialogTitle>
        <AlertDialogDescription v-if="description" id="alert-desc" class="alert-desc">
          {{ description }}
        </AlertDialogDescription>
        <div class="alert-body"><slot /></div>
        <footer class="alert-footer">
          <AlertDialogCancel as-child>
            <UiButton variant="outline" @click="handleCancel">{{ cancelText }}</UiButton>
          </AlertDialogCancel>
          <AlertDialogAction as-child>
            <UiButton :variant="tone === 'danger' ? 'destructive' : 'default'" @click="handleConfirm">
              {{ confirmText }}
            </UiButton>
          </AlertDialogAction>
        </footer>
      </AlertDialogContent>
    </AlertDialogPortal>
  </AlertDialogRoot>
</template>
<style scoped>
.alert-overlay {
  position: fixed;
  inset: 0;
  z-index: var(--z-dialog);
  background: var(--color-overlay);
  backdrop-filter: blur(2px);
  animation: overlay-in var(--duration-base) var(--ease-standard);
}
.alert-content {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: var(--z-dialog);
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  width: min(440px, calc(100vw - 32px));
  max-height: calc(100dvh - 48px);
  padding: var(--space-5);
  background: var(--color-surface);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
  animation: alert-in var(--duration-base) var(--ease-emphasized);
}
.alert-title {
  font-size: var(--text-lg);
  font-weight: 600;
  color: var(--color-text);
}
.alert-content[data-tone='danger'] .alert-title {
  color: var(--color-danger);
}
.alert-desc {
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  line-height: 1.6;
}
.alert-body {
  min-width: 0;
}
.alert-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  margin-top: var(--space-2);
}
@keyframes overlay-in {
  from { opacity: 0; }
  to { opacity: 1; }
}
@keyframes alert-in {
  from { opacity: 0; transform: translate(-50%, -48%) scale(0.96); }
  to { opacity: 1; transform: translate(-50%, -50%) scale(1); }
}
@media (prefers-reduced-motion: reduce) {
  .alert-overlay,
  .alert-content {
    animation: none;
  }
}
</style>
