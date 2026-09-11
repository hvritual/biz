<script setup lang="ts">
import mark from "@/assets/brand-mark.png";
withDefaults(defineProps<{ compact?: boolean }>(), { compact: false });
</script>
<template>
  <div class="brand">
    <img :src="mark" alt="" width="37" height="44" />
    <div v-if="!compact"><strong>CoffeeLink</strong><span>咖啡机物联云平台</span></div>
  </div>
</template>
