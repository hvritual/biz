<script setup lang="ts">
import { ref, watch, nextTick, onBeforeUnmount } from "vue";
import AppIcon from "./AppIcon.vue";
const props = withDefaults(
  defineProps<{
    open: boolean;
    title: string;
    description?: string;
    wide?: boolean;
  }>(),
  { description: "", wide: false },
);
const emit = defineEmits<{ close: [] }>();
const dialog = ref<HTMLDialogElement>();
watch(
  () => props.open,
  async (value) => {
    await nextTick();
    if (value && !dialog.value?.open) dialog.value?.showModal();
    else if (!value && dialog.value?.open) dialog.value?.close();
  },
  { immediate: true },
);
onBeforeUnmount(() => dialog.value?.close());
</script>
<template>
  <Teleport to="body"
    ><dialog
      ref="dialog"
      :class="['base-dialog', { 'base-dialog--wide': wide }]"
      :aria-label="title"
      @cancel.prevent="emit('close')"
      @click="$event.target === dialog && emit('close')"
    >
      <div class="dialog-inner">
        <header class="dialog-heading">
          <div>
            <h2>{{ title }}</h2>
            <p v-if="description">{{ description }}</p>
          </div>
          <button class="icon-button" aria-label="关闭对话框" @click="emit('close')">
            <AppIcon name="X" />
          </button>
        </header>
        <div class="dialog-body"><slot /></div>
        <footer v-if="$slots.footer" class="dialog-footer">
          <slot name="footer" />
        </footer>
      </div></dialog
  ></Teleport>
</template>
