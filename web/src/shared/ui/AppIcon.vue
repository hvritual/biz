<script setup lang="ts">
import { icons, type IconName } from "../lib/icons";
withDefaults(defineProps<{ name: IconName; size?: number }>(), { size: 18 });
</script>
<template>
  <component
    :is="icons[name]"
    :size="size"
    :stroke-width="1.85"
    aria-hidden="true"
    class="app-icon"
  />
</template>
