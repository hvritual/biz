<script setup lang="ts">
import CoffeeBanner from "./CoffeeBanner.vue";
withDefaults(
  defineProps<{
    title: string;
    description: string;
    section?: string;
    banner?: boolean;
  }>(),
  { section: "企业中心", banner: false },
);
</script>
<template>
  <header class="page-heading">
    <div>
      <div class="breadcrumb">
        {{ section }}<span>/</span><b>{{ title }}</b>
      </div>
      <h1>{{ title }}</h1>
      <p>{{ description }}</p>
    </div>
    <CoffeeBanner v-if="banner" />
    <div v-else class="page-heading-actions"><slot /></div>
  </header>
</template>
