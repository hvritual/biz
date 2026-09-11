<script setup lang="ts">
import photo from "@/assets/coffee-scene.webp";
withDefaults(defineProps<{ title?: string; subtitle?: string; compact?: boolean }>(), {
  title: "优秀的团队，成就更好的咖啡体验",
  subtitle: "让每一杯咖啡更智能、更高效",
  compact: false,
});
</script>
<template>
  <div :class="['coffee-banner', { 'coffee-banner--compact': compact }]">
    <img :src="photo" alt="咖啡杯与咖啡制作场景" />
    <div>
      <strong>{{ title }}</strong>
      <p>{{ subtitle }}</p>
    </div>
  </div>
</template>
