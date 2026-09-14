<script setup lang="ts">
import { CollapsibleContent, CollapsibleRoot, CollapsibleTrigger } from 'reka-ui'
const open = defineModel<boolean>({ default: false })
</script>
<template>
  <CollapsibleRoot v-model:open="open" data-slot="collapsible">
    <CollapsibleTrigger data-slot="collapsible-trigger" class="contents">
      <slot name="trigger" />
    </CollapsibleTrigger>
    <CollapsibleContent data-slot="collapsible-content"><slot /></CollapsibleContent>
  </CollapsibleRoot>
</template>
