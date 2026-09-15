<script setup lang="ts">
defineProps<{ steps: Array<{ key: string; label: string; description?: string }>; current: string }>()
</script>
<template>
  <ol class="grid gap-3" data-ui-pattern="StepFlow">
    <li v-for="(step, index) in steps" :key="step.key" class="flex items-start gap-3 rounded-[var(--radius-md)] border border-border bg-card p-3" :aria-current="step.key === current ? 'step' : undefined">
      <span class="flex size-7 shrink-0 items-center justify-center rounded-full bg-secondary text-xs font-semibold text-secondary-foreground">{{ index + 1 }}</span>
      <span class="min-w-0"><strong class="block text-sm">{{ step.label }}</strong><small v-if="step.description" class="text-muted-foreground">{{ step.description }}</small></span>
    </li>
  </ol>
</template>
