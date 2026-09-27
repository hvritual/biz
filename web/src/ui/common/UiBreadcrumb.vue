<script setup lang="ts">
import { computed } from 'vue'

export type BreadcrumbItem = {
  label: string
  href?: string
}

const props = defineProps<{
  items: BreadcrumbItem[]
}>()
const trail = computed(() => props.items)
const lastIndex = computed(() => trail.value.length - 1)
</script>
<template>
  <nav class="breadcrumb" aria-label="breadcrumb" data-ui-pattern="Breadcrumb">
    <ol>
      <li v-for="(item, index) in trail" :key="index">
        <a v-if="item.href && index !== lastIndex" :href="item.href">{{ item.label }}</a>
        <strong v-else :aria-current="index === lastIndex ? 'page' : undefined">{{ item.label }}</strong>
        <span v-if="index !== lastIndex" class="separator" aria-hidden="true">/</span>
      </li>
    </ol>
  </nav>
</template>
<style scoped>
.breadcrumb ol {
  display: flex;
  gap: var(--space-2);
  align-items: center;
  margin: 0;
  padding: 0;
  list-style: none;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}
.breadcrumb li {
  display: inline-flex;
  gap: var(--space-2);
  align-items: center;
}
.breadcrumb a {
  color: var(--color-text-muted);
  text-decoration: none;
  transition: color var(--duration-base) var(--ease-standard);
}
.breadcrumb a:hover {
  color: var(--color-primary);
}
.breadcrumb a:focus-visible {
  outline: var(--focus-ring-width) solid var(--focus-ring-color);
  outline-offset: var(--focus-ring-offset);
}
.breadcrumb strong {
  font-weight: 500;
  color: var(--color-text);
}
.breadcrumb .separator {
  color: var(--color-text-muted);
}
</style>
