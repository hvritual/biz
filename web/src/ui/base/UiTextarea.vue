<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })
const model = defineModel<any>()
const attrs = useAttrs()
const classes = computed(() => cn('flex min-h-22 w-full rounded-[var(--radius-sm)] border border-input bg-background px-3 py-2 text-[var(--text-sm)] text-foreground outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-ring/20 disabled:cursor-not-allowed disabled:opacity-50', attrs.class))
</script>
<template><textarea v-model="model" data-slot="textarea" v-bind="$attrs" :class="classes" /></template>
