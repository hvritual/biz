<script setup lang="ts">
import AppIcon from './AppIcon.vue'
withDefaults(
  defineProps<{
    title?: string
    description?: string
    icon?: string
    size?: 'default' | 'compact'
  }>(),
  { title: '没有找到相关记录', description: '尝试调整筛选条件，或清空条件重新查询。', icon: 'search', size: 'default' },
)
</script>
<template>
  <div class="empty-state" :data-size="size">
    <span class="empty-icon"><AppIcon :name="icon" :size="size === 'compact' ? 22 : 30" /></span>
    <h3 class="empty-title">{{ title }}</h3>
    <p class="empty-desc">{{ description }}</p>
    <div v-if="$slots.action" class="empty-action"><slot name="action" /></div>
    <slot />
  </div>
</template>
<style scoped>
.empty-state {
  text-align: center;
  padding: var(--space-8) var(--space-5);
}
.empty-state[data-size='compact'] {
  padding: var(--space-5) var(--space-4);
}
.empty-icon {
  display: grid;
  place-items: center;
  margin: 0 auto var(--space-4);
  width: 64px;
  height: 64px;
  border-radius: var(--radius-lg);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}
.empty-state[data-size='compact'] .empty-icon {
  width: 44px;
  height: 44px;
  border-radius: var(--radius-md);
}
.empty-title {
  font-size: var(--text-base);
  font-weight: var(--font-weight-semibold);
  color: var(--color-text);
}
.empty-desc {
  margin: var(--space-2) auto var(--space-4);
  max-width: 430px;
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  line-height: var(--leading-relaxed);
}
.empty-action {
  display: flex;
  justify-content: center;
  gap: var(--space-2);
}
</style>
