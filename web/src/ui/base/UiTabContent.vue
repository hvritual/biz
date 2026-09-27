<script setup lang="ts">
import { TabsContent } from 'reka-ui'

defineOptions({ inheritAttrs: false })
defineProps<{
  value: string
}>()
</script>
<template>
  <TabsContent
    :value="value"
    data-slot="tab-content"
    class="tab-content"
  >
    <slot />
  </TabsContent>
</template>
<style scoped>
.tab-content {
  min-width: 0;
}
.tab-content:focus-visible {
  outline: var(--focus-ring-width) solid var(--focus-ring-color);
  outline-offset: var(--focus-ring-offset);
}
</style>
