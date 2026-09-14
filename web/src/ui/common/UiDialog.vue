<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import AppIcon from './AppIcon.vue'
const props = withDefaults(
  defineProps<{ open: boolean; title: string; width?: string; drawer?: boolean }>(),
  { width: '620px', drawer: false },
)
const emit = defineEmits<{ close: [] }>()
const panel = ref<HTMLElement>()
let previous: HTMLElement | null = null
function onKey(e: KeyboardEvent) {
  if (!props.open) return
  if (e.key === 'Escape') {
    e.stopPropagation()
    emit('close')
  }
  if (e.key === 'Tab') {
    const items = panel.value?.querySelectorAll<HTMLElement>(
      'button:not(:disabled),a[href],input:not(:disabled),select:not(:disabled),textarea:not(:disabled),[tabindex="0"]',
    )
    if (!items?.length) return
    const first = items[0]!,
      last = items[items.length - 1]!
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault()
      last.focus()
    }
    if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault()
      first.focus()
    }
  }
}
watch(
  () => props.open,
  async (open) => {
    if (open) {
      previous = document.activeElement as HTMLElement
      await nextTick()
      panel.value?.querySelector<HTMLElement>('input,button')?.focus()
      document.addEventListener('keydown', onKey)
    } else {
      document.removeEventListener('keydown', onKey)
      previous?.focus()
    }
  },
  { immediate: true },
)
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))
</script>
<template>
  <Teleport to="body"
    ><div v-if="open" class="dialog-layer" :class="{ drawer }" @mousedown.self="emit('close')">
      <section
        ref="panel"
        class="dialog-panel"
        :style="{ width }"
        role="dialog"
        aria-modal="true"
        :aria-label="title"
      >
        <header>
          <h2>{{ title }}</h2>
          <button class="icon-button" aria-label="关闭弹窗" @click="emit('close')">
            <AppIcon name="close" />
          </button>
        </header>
        <div class="dialog-body"><slot /></div>
        <footer v-if="$slots.footer"><slot name="footer" /></footer>
      </section></div
  ></Teleport>
</template>
<style scoped>
.dialog-layer {
  position: fixed;
  inset: 0;
  z-index: var(--z-dialog);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--color-overlay);
  backdrop-filter: blur(2px);
}
.dialog-panel {
  display: flex;
  flex-direction: column;
  max-width: calc(100vw - 32px);
  max-height: calc(100dvh - 48px);
  background: var(--color-surface);
  border-radius: var(--radius-lg);
  box-shadow: 0 24px 80px var(--color-shadow);
}
.dialog-panel > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  border-bottom: 1px solid var(--color-border);
  flex-shrink: 0;
}
.dialog-panel > header h2 {
  font-size: 18px;
}
.dialog-body {
  padding: 24px;
  overflow-y: auto;
}
.dialog-panel > footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid var(--color-border);
  flex-shrink: 0;
}
.drawer {
  justify-content: flex-end;
  padding: 0;
}
.drawer .dialog-panel {
  height: 100dvh;
  max-height: 100dvh;
  max-width: 100vw;
  border-radius: var(--radius-lg) 0 0 var(--radius-lg);
}
.drawer .dialog-body {
  flex: 1;
}
@media (max-width: 767px) {
  .dialog-layer {
    padding: 12px;
  }
  .dialog-panel {
    max-height: calc(100dvh - 24px);
  }
  .dialog-body {
    padding: 18px;
  }
  .dialog-panel > header,
  .dialog-panel > footer {
    padding: 14px 18px;
  }
  .drawer {
    padding: 0;
  }
}
</style>
