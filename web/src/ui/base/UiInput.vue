<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })
const model = defineModel<any>()
const attrs = useAttrs()
const classes = computed(() => cn('flex h-[var(--control-height)] w-full min-w-0 rounded-[var(--radius-sm)] border border-input bg-background px-3 text-[var(--text-sm)] text-foreground shadow-none outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-ring/20 disabled:cursor-not-allowed disabled:opacity-50 file:border-0 file:bg-transparent file:text-sm file:font-medium', attrs.class))
</script>
<template><input v-model="model" data-slot="input" v-bind="$attrs" :class="classes" /></template>
