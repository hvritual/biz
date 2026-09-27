<script setup lang="ts">
import { AccordionContent, AccordionHeader, AccordionItem, AccordionTrigger } from 'reka-ui'
import { ChevronDown } from 'lucide-vue-next'

defineProps<{
  value: string
  disabled?: boolean
}>()
</script>
<template>
  <AccordionItem :value="value" :disabled="disabled" class="accordion-item">
    <AccordionHeader class="accordion-header">
      <AccordionTrigger class="accordion-trigger">
        <span class="accordion-label"><slot name="header" /></span>
        <ChevronDown class="accordion-chevron" :size="16" aria-hidden="true" />
      </AccordionTrigger>
    </AccordionHeader>
    <AccordionContent class="accordion-content">
      <div class="accordion-inner"><slot /></div>
    </AccordionContent>
  </AccordionItem>
</template>
<style scoped>
.accordion-item {
  border-bottom: 1px solid var(--color-border);
}
.accordion-item:last-child {
  border-bottom: 0;
}
.accordion-header {
  margin: 0;
}
.accordion-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  width: 100%;
  padding: var(--space-3) var(--space-4);
  border: 0;
  background: transparent;
  font-size: var(--text-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
  text-align: left;
  cursor: pointer;
  transition: background var(--duration-base) var(--ease-standard);
}
.accordion-trigger:hover:not(:disabled) {
  background: var(--color-surface-soft);
}
.accordion-trigger:focus-visible {
  outline: var(--focus-ring-width) solid var(--focus-ring-color);
  outline-offset: var(--focus-ring-offset);
}
.accordion-trigger:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}
.accordion-label {
  flex: 1;
  min-width: 0;
}
.accordion-chevron {
  flex-shrink: 0;
  color: var(--color-text-muted);
  transition: transform var(--duration-base) var(--ease-standard);
}
.accordion-trigger[data-state='open'] .accordion-chevron {
  transform: rotate(180deg);
}
.accordion-content {
  overflow: hidden;
}
.accordion-inner {
  padding: 0 var(--space-4) var(--space-3);
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  line-height: var(--leading-relaxed);
}
@media (prefers-reduced-motion: reduce) {
  .accordion-chevron,
  .accordion-trigger {
    transition: none;
  }
}
</style>
