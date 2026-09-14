<script setup lang="ts">
import { cva } from 'class-variance-authority'
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })
const props = withDefaults(defineProps<{ variant?: 'default' | 'secondary' | 'outline' | 'ghost' | 'destructive' | 'link'; size?: 'default' | 'sm' | 'lg' | 'icon' }>(), { variant: 'default', size: 'default' })
const attrs = useAttrs()
const variants = cva('inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[var(--radius-sm)] text-[var(--text-sm)] font-medium transition-colors outline-none disabled:pointer-events-none disabled:opacity-45 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2', {
  variants: {
    variant: {
      default: 'border border-primary bg-primary text-primary-foreground hover:bg-[var(--color-primary-hover)]',
      secondary: 'border border-border bg-secondary text-secondary-foreground hover:bg-accent hover:text-accent-foreground',
      outline: 'border border-input bg-background text-foreground hover:border-primary hover:bg-accent hover:text-accent-foreground',
      ghost: 'border border-transparent bg-transparent text-foreground hover:bg-accent hover:text-accent-foreground',
      destructive: 'border border-destructive bg-destructive text-destructive-foreground hover:opacity-90',
      link: 'h-auto border-0 bg-transparent p-0 text-primary underline-offset-4 hover:underline',
    },
    size: {
      default: 'h-[var(--control-height)] px-3.5',
      sm: 'h-8 px-3 text-xs',
      lg: 'h-10 px-5',
      icon: 'size-[var(--control-height)] p-0',
    },
  },
})
const classes = computed(() => cn(variants({ variant: props.variant, size: props.size }), attrs.class))
</script>
<template><button data-slot="button" v-bind="$attrs" :class="classes"><slot /></button></template>
