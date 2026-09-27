<script setup lang="ts">
import { cva } from 'class-variance-authority'
import { computed, ref, useAttrs } from 'vue'
import { cn } from '@/lib/utils'
import UiSpinner from './UiSpinner.vue'

defineOptions({ inheritAttrs: false })
const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'secondary' | 'outline' | 'ghost' | 'destructive' | 'link'
    size?: 'default' | 'sm' | 'lg' | 'icon'
    loading?: boolean
    disabled?: boolean
  }>(),
  { variant: 'default', size: 'default', loading: false, disabled: false },
)
const attrs = useAttrs()
const element = ref<HTMLButtonElement | null>(null)
const variants = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[var(--radius-sm)] text-[var(--text-sm)] font-medium transition-colors outline-none disabled:pointer-events-none disabled:opacity-45 focus-visible:outline focus-visible:outline-[var(--focus-ring-width)] focus-visible:outline-[var(--focus-ring-color)] focus-visible:outline-offset-[var(--focus-ring-offset)] active:scale-[0.98]',
  {
    variants: {
      variant: {
        default: 'border border-primary bg-primary text-primary-foreground hover:bg-[var(--color-primary-hover)]',
        secondary: 'border border-border bg-secondary text-secondary-foreground hover:bg-accent hover:text-accent-foreground',
        outline: 'border border-input bg-background text-foreground hover:border-primary hover:bg-accent hover:text-accent-foreground',
        ghost: 'border border-transparent bg-transparent text-foreground hover:bg-accent hover:text-accent-foreground',
        destructive: 'border border-destructive bg-destructive text-destructive-foreground hover:bg-[var(--color-danger)]',
        link: 'h-auto border-0 bg-transparent p-0 text-primary underline-offset-4 hover:underline',
      },
      size: {
        default: 'h-[var(--control-height)] px-3.5',
        sm: 'h-8 px-3 text-xs',
        lg: 'h-10 px-5',
        icon: 'size-[var(--control-height)] p-0',
      },
    },
  },
)
const isDisabled = computed(() => props.disabled || props.loading)
const spinnerSize = computed(() => (props.size === 'sm' ? 14 : props.size === 'lg' ? 18 : 16))
const classes = computed(() =>
  cn(
    variants({ variant: props.variant, size: props.size }),
    props.loading && 'cursor-wait',
    attrs.class,
  ),
)
function focus(options?: FocusOptions) {
  element.value?.focus(options)
}
defineExpose({ focus })
</script>
<template>
  <button
    ref="element"
    data-slot="button"
    v-bind="$attrs"
    :class="classes"
    :disabled="isDisabled"
    :aria-busy="loading || undefined"
  >
    <UiSpinner v-if="loading" :size="spinnerSize" />
    <slot />
  </button>
</template>
