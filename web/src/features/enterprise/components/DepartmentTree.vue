<script setup lang="ts">
import AppIcon from "@/shared/ui/AppIcon.vue";
import type { Department } from "../model/organization";
defineProps<{
  items: Department[];
  parentId: string | null;
  selected: string;
  counts: Record<string, number>;
  keyword: string;
}>();
const emit = defineEmits<{ select: [id: string] }>();
</script>
<template>
  <ul class="department-tree">
    <li
      v-for="item in items.filter(
        (d) =>
          d.parentId === parentId &&
          (!keyword ||
            d.name.includes(keyword) ||
            items.some((c) => c.parentId === d.id && c.name.includes(keyword))),
      )"
      :key="item.id"
    >
      <button :class="{ active: selected === item.id }" @click="emit('select', item.id)">
        <AppIcon name="Network" :size="16" /><span>{{ item.name }}</span
        ><small>{{ counts[item.name] ?? 0 }}</small></button
      ><DepartmentTree
        v-if="items.some((d) => d.parentId === item.id)"
        :items="items"
        :parent-id="item.id"
        :selected="selected"
        :counts="counts"
        :keyword="keyword"
        @select="emit('select', $event)"
      />
    </li>
  </ul>
</template>
