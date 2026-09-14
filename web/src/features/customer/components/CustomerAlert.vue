<script setup lang="ts">
import AppIcon from '@/components/ui/AppIcon.vue'
withDefaults(defineProps<{ title: string; description?: string; tone?: string }>(), {
  tone: 'primary',
  description: '',
})
</script>
<template>
  <div class="customer-alert" :class="tone" :role="tone === 'danger' ? 'alert' : 'status'">
    <AppIcon
      :name="
        tone === 'success' ? 'success' : tone === 'danger' ? 'error' : tone === 'warning' ? 'warning' : 'help'
      "
      :size="18"
    />
    <div class="alert-main">
      <strong>{{ title }}</strong>
      <p v-if="description">{{ description }}</p>
      <slot />
    </div>
  </div>
</template>
