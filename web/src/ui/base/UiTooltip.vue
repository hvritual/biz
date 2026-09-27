<script setup lang="ts">
import { TooltipContent, TooltipPortal, TooltipProvider, TooltipRoot, TooltipTrigger } from 'reka-ui'

defineOptions({ inheritAttrs: false })
withDefaults(
  defineProps<{
    content?: string
    side?: 'top' | 'right' | 'bottom' | 'left'
    align?: 'start' | 'center' | 'end'
    delay?: number
  }>(),
  { content: '', side: 'top', align: 'center', delay: 200 },
)
</script>
<template>
  <TooltipProvider :delay-duration="delay">
    <TooltipRoot>
      <TooltipTrigger as-child>
        <slot />
      </TooltipTrigger>
      <TooltipPortal>
        <TooltipContent
          :side="side"
          :align="align"
          data-slot="tooltip"
          class="tooltip-content"
          :side-offset="6"
        >
          <slot name="content">{{ content }}</slot>
          <TooltipArrow class="tooltip-arrow" />
        </TooltipContent>
      </TooltipPortal>
    </TooltipRoot>
  </TooltipProvider>
</template>
<style scoped>
.tooltip-content {
  z-index: var(--z-dialog);
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm);
  background: var(--color-text);
  color: var(--color-surface);
  font-size: var(--text-xs);
  line-height: var(--leading-normal);
  box-shadow: var(--shadow-sm);
  max-width: 240px;
  animation: tooltip-in var(--duration-fast) var(--ease-standard);
}
.tooltip-arrow {
  fill: var(--color-text);
}
@keyframes tooltip-in {
  from { opacity: 0; transform: scale(0.96); }
  to { opacity: 1; transform: scale(1); }
}
@media (prefers-reduced-motion: reduce) {
  .tooltip-content {
    animation: none;
  }
}
</style>
