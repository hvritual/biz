<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from './AppIcon.vue'
const props = defineProps<{ total: number; page: number; pageSize: number }>()
const emit = defineEmits<{ 'update:page': [value: number]; 'update:pageSize': [value: number] }>()
const pages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))
const visible = computed(() => {
  const start = Math.max(1, Math.min(props.page - 2, pages.value - 4))
  return Array.from({ length: Math.min(5, pages.value) }, (_, i) => start + i)
})
function change(value: number) {
  emit('update:page', Math.max(1, Math.min(pages.value, value)))
}
</script>
<template>
  <footer class="pagination">
    <span class="muted">共 {{ total.toLocaleString('zh-CN') }} 条</span>
    <div class="pagination-controls">
      <button class="icon-button" aria-label="上一页" :disabled="page <= 1" @click="change(page - 1)">
        <AppIcon name="left" :size="16" /></button
      ><button
        v-for="n in visible"
        :key="n"
        :class="['page-number', { active: page === n }]"
        :aria-label="'第 ' + n + ' 页'"
        :aria-current="page === n ? 'page' : undefined"
        @click="change(n)"
      >
        {{ n }}</button
      ><span v-if="visible.at(-1) !== pages" class="muted">…</span
      ><button v-if="visible.at(-1) !== pages" class="page-number" @click="change(pages)">{{ pages }}</button
      ><button class="icon-button" aria-label="下一页" :disabled="page >= pages" @click="change(page + 1)">
        <AppIcon name="right" :size="16" /></button
      ><select
        class="select page-size"
        :value="pageSize"
        aria-label="每页条数"
        @change="
          ($event: Event) => {
            emit('update:pageSize', Number(($event.target as HTMLSelectElement).value))
            change(1)
          }
        "
      >
        <option :value="10">10 条/页</option>
        <option :value="20">20 条/页</option>
        <option :value="50">50 条/页</option></select
      ><span class="page-jump muted"
        >前往
        <input
          class="input"
          type="number"
          :value="page"
          min="1"
          :max="pages"
          aria-label="跳转页码"
          @change="change(Number(($event.target as HTMLInputElement).value) || 1)"
        />
        页</span
      >
    </div>
  </footer>
</template>
<style scoped>
.pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 22px 2px 4px;
  font-size: var(--text-sm);
  flex-wrap: wrap;
}
.pagination-controls {
  display: flex;
  gap: 7px;
  align-items: center;
}
.page-number {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  font-size: var(--text-sm);
}
.page-number.active {
  color: var(--color-primary);
  background: var(--color-primary-soft);
  font-weight: 600;
}
.page-number:hover {
  background: var(--color-surface-soft);
}
.page-size {
  width: 108px;
  margin-left: 14px;
}
.page-jump {
  display: flex;
  gap: 9px;
  align-items: center;
}
.page-jump .input {
  width: 52px;
  text-align: center;
  padding: 0 3px;
}
@media (max-width: 900px) {
  .page-jump {
    display: none;
  }
}
@media (max-width: 600px) {
  .pagination-controls {
    gap: 2px;
    flex-wrap: wrap;
    max-width: 100%;
  }
  .page-size {
    margin-left: 3px;
    width: 99px;
  }
  .page-number {
    width: 28px;
  }
  .pagination > span {
    width: 100%;
  }
}
</style>
