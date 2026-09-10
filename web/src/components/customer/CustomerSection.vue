<script setup lang="ts">
import AppIcon from '@/components/ui/AppIcon.vue'
defineProps<{ title: string; icon?: string }>()
</script>
<template>
  <section class="card customer-section">
    <header class="customer-section-header">
      <h2><AppIcon v-if="icon" :name="icon" :size="18" />{{ title }}</h2>
      <slot name="action" />
    </header>
    <div class="customer-section-body"><slot /></div>
  </section>
</template>
