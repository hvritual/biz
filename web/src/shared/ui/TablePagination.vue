<script setup lang="ts">
import { computed } from "vue";
import AppIcon from "./AppIcon.vue";
const props = defineProps<{ total: number; page: number; pageSize: number }>();
const emit = defineEmits<{
  "update:page": [value: number];
  "update:pageSize": [value: number];
}>();
const count = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)));
const pages = computed(() =>
  [
    ...new Set([
      1,
      Math.max(1, props.page - 1),
      props.page,
      Math.min(count.value, props.page + 1),
      count.value,
    ]),
  ].sort((a, b) => a - b),
);
</script>
<template>
  <footer class="pagination">
    <span
      >共 <b>{{ total.toLocaleString() }}</b> 条</span
    >
    <div class="pagination-controls">
      <button
        class="icon-button"
        aria-label="上一页"
        :disabled="page <= 1"
        @click="emit('update:page', page - 1)"
      >
        <AppIcon name="ChevronLeft" :size="16" /></button
      ><button
        v-for="p in pages"
        :key="p"
        :class="['page-number', { active: p === page }]"
        :aria-label="`第${p}页`"
        :aria-current="p === page ? 'page' : undefined"
        @click="emit('update:page', p)"
      >
        {{ p }}</button
      ><button
        class="icon-button"
        aria-label="下一页"
        :disabled="page >= count"
        @click="emit('update:page', page + 1)"
      >
        <AppIcon name="ChevronRight" :size="16" /></button
      ><select
        :value="pageSize"
        aria-label="每页条数"
        @change="
          emit('update:pageSize', Number(($event.target as HTMLSelectElement).value))
        "
      >
        <option :value="8">8 条/页</option>
        <option :value="16">16 条/页</option>
        <option :value="32">32 条/页</option></select
      ><span class="page-count">第 {{ page }} / {{ count }} 页</span>
    </div>
  </footer>
</template>
