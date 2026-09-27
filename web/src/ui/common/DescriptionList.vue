<script setup lang="ts">
import { computed } from 'vue'

export type DescriptionEntry = { label: string; value: string | number | null | undefined }

const props = withDefaults(
  defineProps<{
    items?: DescriptionEntry[]
    columns?: number
  }>(),
  { items: () => [], columns: 2 },
)
const gridTemplate = computed(() => `repeat(${props.columns}, minmax(0, 1fr))`)
</script>
<template>
  <dl class="description-list" :style="{ '--dl-columns': gridTemplate }" data-ui-pattern="DescriptionList">
    <template v-if="items.length">
      <dt v-for="(item, i) in items" :key="`dt-${i}`">{{ item.label }}</dt>
      <dd v-for="(item, i) in items" :key="`dd-${i}`">{{ item.value ?? '—' }}</dd>
    </template>
    <template v-else>
      <slot />
    </template>
  </dl>
</template>
<style scoped>
.description-list {
  display: grid;
  grid-template-columns: var(--dl-columns, repeat(2, minmax(0, 1fr)));
  gap: var(--space-3) var(--space-3);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}
.description-list :deep(dt) {
  color: var(--color-text-muted);
}
.description-list :deep(dd) {
  margin: 0;
  overflow-wrap: anywhere;
  color: var(--color-text);
}
@media (max-width: 767px) {
  .description-list {
    grid-template-columns: 1fr;
  }
}
</style>
