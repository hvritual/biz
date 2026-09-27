<script setup lang="ts">
import { CollapsibleContent, CollapsibleRoot, CollapsibleTrigger } from 'reka-ui'
const open = defineModel<boolean>({ default: false })
</script>
<template>
  <CollapsibleRoot v-model:open="open" data-slot="collapsible">
    <CollapsibleTrigger data-slot="collapsible-trigger" class="contents">
      <slot name="trigger" />
    </CollapsibleTrigger>
    <CollapsibleContent
      data-slot="collapsible-content"
      force-mount
      class="collapsible-content"
    >
      <div class="collapsible-inner">
        <slot />
      </div>
    </CollapsibleContent>
  </CollapsibleRoot>
</template>
<style scoped>
.collapsible-content {
  display: grid;
  grid-template-rows: 0fr;
  opacity: 0;
  transition:
    grid-template-rows var(--duration-base) var(--ease-standard),
    opacity var(--duration-base) var(--ease-standard);
}
.collapsible-content[data-state='open'] {
  grid-template-rows: 1fr;
  opacity: 1;
}
.collapsible-inner {
  overflow: hidden;
  min-height: 0;
}
@media (prefers-reduced-motion: reduce) {
  .collapsible-content {
    transition: none;
  }
}
</style>
