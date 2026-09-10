<script setup lang="ts">
import AppIcon from './AppIcon.vue'
withDefaults(defineProps<{ title?: string; description?: string; icon?: string }>(), {
  title: '没有找到相关记录',
  description: '尝试调整筛选条件，或清空条件重新查询。',
  icon: 'search',
})
</script>
<template>
  <div class="empty-state">
    <span><AppIcon :name="icon" :size="30" /></span>
    <h3>{{ title }}</h3>
    <p>{{ description }}</p>
    <slot />
  </div>
</template>
<style scoped>
.empty-state {
  text-align: center;
  padding: 60px 20px;
}
.empty-state > span {
  display: grid;
  place-items: center;
  margin: 0 auto 16px;
  width: 64px;
  height: 64px;
  border-radius: 20px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}
.empty-state p {
  margin: 8px auto 18px;
  max-width: 430px;
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
}
</style>
