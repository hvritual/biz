<script setup lang="ts">
import AppIcon from "./AppIcon.vue";
withDefaults(defineProps<{ title?: string; description?: string }>(), {
  title: "没有找到匹配的记录",
  description: "请调整筛选条件后重试。",
});
</script>
<template>
  <div class="empty-state" role="status">
    <span><AppIcon name="Search" :size="30" /></span>
    <h3>{{ title }}</h3>
    <p>{{ description }}</p>
    <slot />
  </div>
</template>
