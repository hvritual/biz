<script setup lang="ts">
import { UiSpinner } from '@/ui/base'
defineProps<{ loading?: boolean; error?: string }>()
</script>
<template>
  <section class="flex min-w-0 flex-col gap-4" data-ui-pattern="ListPage">
    <header v-if="$slots.heading" data-ui-region="heading"><slot name="heading" /></header>
    <div v-if="$slots.metrics" data-ui-region="metrics"><slot name="metrics" /></div>
    <div v-if="$slots.filters" data-ui-region="filters"><slot name="filters" /></div>
    <section class="card data-panel" data-ui-region="data-panel">
      <div v-if="$slots.toolbar" class="table-toolbar" data-ui-region="toolbar"><slot name="toolbar" /></div>
      <div v-if="loading" class="list-loading" data-ui-region="loading" role="status" aria-live="polite" aria-busy="true">
        <UiSpinner :size="24" />
      </div>
      <div v-else-if="error" class="list-error" data-ui-region="error" role="alert">
        <span>{{ error }}</span>
      </div>
      <slot v-else />
      <footer v-if="$slots.pagination" data-ui-region="pagination"><slot name="pagination" /></footer>
    </section>
  </section>
</template>
<style scoped>
.list-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-8);
}
.list-error {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  padding: var(--space-6);
  color: var(--color-danger);
  font-size: var(--text-sm);
  background: var(--color-danger-soft);
  border-radius: var(--radius-sm);
}
</style>
