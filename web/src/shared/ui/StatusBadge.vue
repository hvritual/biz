<script setup lang="ts">
withDefaults(
  defineProps<{
    tone?: "blue" | "green" | "orange" | "red" | "purple" | "gray";
    dot?: boolean;
  }>(),
  { tone: "gray", dot: false },
);
</script>
<template>
  <span :class="['badge', `tone-${tone}`]"
    ><span v-if="dot" class="status-dot" /><slot
  /></span>
</template>
