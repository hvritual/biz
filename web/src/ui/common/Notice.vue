<script setup lang="ts">
import { CircleAlert, CircleCheck, CircleX, Info, X } from 'lucide-vue-next'
import { computed } from 'vue'
import { UiButton } from '@/ui/base'

defineOptions({ name: 'UiNotice' })
const props = withDefaults(
  defineProps<{
    tone?: 'info' | 'success' | 'warning' | 'danger'
    title?: string
    dismissible?: boolean
  }>(),
  { tone: 'info', title: '', dismissible: false },
)
const emit = defineEmits<{ dismiss: [] }>()
const icons = {
  info: Info,
  success: CircleCheck,
  warning: CircleAlert,
  danger: CircleX,
}
const iconComponent = computed(() => icons[props.tone])
const toneClass = computed(() => {
  if (props.tone === 'success') return 'success'
  if (props.tone === 'warning') return 'warning'
  if (props.tone === 'danger') return 'danger'
  return ''
})
</script>
<template>
  <div
    :class="['notice-box', toneClass]"
    role="status"
    data-ui-pattern="Notice"
  >
    <component :is="iconComponent" class="notice-icon" :size="18" aria-hidden="true" />
    <div class="notice-content">
      <p v-if="title" class="notice-title">{{ title }}</p>
      <div class="notice-body"><slot /></div>
    </div>
    <UiButton
      v-if="dismissible"
      variant="ghost"
      size="icon"
      class="notice-dismiss"
      aria-label="关闭"
      @click="emit('dismiss')"
    >
      <X :size="16" />
    </UiButton>
  </div>
</template>
<style scoped>
.notice-box {
  display: flex;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  color: var(--color-text-secondary);
  font-size: var(--text-sm);
  border-radius: var(--radius-md);
  background: var(--color-primary-soft);
  line-height: 1.7;
}
.notice-box.success {
  background: var(--color-success-soft);
  color: var(--color-success);
}
.notice-box.warning {
  background: var(--color-warning-soft);
  color: var(--color-warning);
}
.notice-box.danger {
  background: var(--color-danger-soft);
  color: var(--color-danger);
}
.notice-icon {
  flex-shrink: 0;
  margin-top: 1px;
  color: currentColor;
}
.notice-content {
  flex: 1;
  min-width: 0;
}
.notice-title {
  font-weight: 600;
  color: var(--color-text);
  margin-bottom: var(--space-1);
}
.notice-box.success .notice-title {
  color: var(--color-success);
}
.notice-box.warning .notice-title {
  color: var(--color-warning);
}
.notice-box.danger .notice-title {
  color: var(--color-danger);
}
.notice-body {
  color: var(--color-text-secondary);
}
.notice-dismiss {
  width: 20px !important;
  height: 20px !important;
  flex-shrink: 0;
  color: currentColor;
  opacity: 0.6;
}
.notice-dismiss:hover {
  opacity: 1;
  background: transparent !important;
  color: currentColor !important;
}
</style>
