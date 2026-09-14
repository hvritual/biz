<script setup lang="ts">
import { ChevronDown } from 'lucide-vue-next'
import { UiCollapsible } from '@/ui/base'
</script>
<template>
  <UiCollapsible data-ui-pattern="Disclosure">
    <template #trigger>
      <span class="flex w-full cursor-pointer items-center justify-between gap-2 rounded-[var(--radius-sm)] px-2 py-2 text-left text-[var(--text-sm)] font-medium text-foreground hover:bg-accent">
        <span><slot name="summary" /></span><ChevronDown class="size-4 shrink-0 text-muted-foreground" />
      </span>
    </template>
    <div class="pt-2"><slot /></div>
  </UiCollapsible>
</template>
