<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })
const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'outline' | 'soft'
    interactive?: boolean
    padding?: 'default' | 'compact' | 'none'
  }>(),
  { variant: 'default', interactive: false, padding: 'default' },
)
const attrs = useAttrs()
const classes = computed(() =>
  cn(
    'flex flex-col min-w-0 rounded-[var(--radius-lg)] transition-colors',
    props.interactive && 'cursor-pointer hover:border-[var(--color-border-strong)]',
    attrs.class,
  ),
)
</script>
<template>
  <section
    data-slot="card"
    :data-variant="variant"
    :class="classes"
    :tabindex="interactive ? 0 : undefined"
    :role="interactive ? 'button' : undefined"
  >
    <header v-if="$slots.header" class="card-header" data-slot="card-header">
      <slot name="header" />
    </header>
    <div
      class="card-body"
      :class="{ 'card-pad': padding === 'default', 'card-pad-compact': padding === 'compact' }"
      data-slot="card-body"
    >
      <slot />
    </div>
    <footer v-if="$slots.footer" class="card-footer" data-slot="card-footer">
      <slot name="footer" />
    </footer>
  </section>
</template>
<style scoped>
[data-slot='card'] {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  box-shadow: var(--shadow-panel);
}
[data-slot='card'][data-variant='outline'] {
  box-shadow: none;
}
[data-slot='card'][data-variant='soft'] {
  background: var(--color-surface-soft);
  box-shadow: none;
}
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--color-border);
  flex-shrink: 0;
}
.card-body {
  flex: 1;
  min-width: 0;
}
.card-pad {
  padding: var(--space-5);
}
.card-pad-compact {
  padding: var(--space-3);
}
.card-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-2);
  padding: var(--space-4) var(--space-5);
  border-top: 1px solid var(--color-border);
  flex-shrink: 0;
}
[data-slot='card']:focus-visible {
  outline: var(--focus-ring-width) solid var(--focus-ring-color);
  outline-offset: var(--focus-ring-offset);
}
</style>
