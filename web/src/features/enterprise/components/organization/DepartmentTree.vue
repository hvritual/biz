<script setup lang="ts">
import { computed, ref } from 'vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { descendantIds, flattenDepartments } from '@/utils/organization'
import AppIcon from '@/components/ui/AppIcon.vue'
import SearchField from '@/components/ui/SearchField.vue'
defineProps<{ selected: string }>()
const emit = defineEmits<{ select: [id: string] }>()
const store = useEnterpriseStore(),
  query = ref('')
const rows = computed(() =>
  flattenDepartments(store.departments).filter((d) => !query.value || d.name.includes(query.value)),
)
const count = (id: string) =>
  store.members.filter(
    (m) => descendantIds(store.departments, id).includes(m.departmentId) && m.status !== 'removed',
  ).length
</script>
<template>
  <section class="card department-tree">
    <h3>组织架构</h3>
    <SearchField v-model="query" placeholder="搜索部门名称" /><button
      :class="['tree-node root-node', { chosen: selected === '' }]"
      @click="emit('select', '')"
    >
      <AppIcon name="company" :size="17" /><span
        >{{ store.company.shortName
        }}<small>{{ store.members.filter((m) => m.status !== 'removed').length }}</small></span
      ></button
    ><button
      v-for="d in rows"
      :key="d.id"
      :class="['tree-node', { chosen: selected === d.id }]"
      :style="{ paddingLeft: 12 + d.depth * 18 + 'px' }"
      @click="emit('select', d.id)"
    >
      <AppIcon :name="d.depth ? 'folder' : 'organization'" :size="16" /><span
        >{{ d.name }}<small>{{ count(d.id) }}</small></span
      >
    </button>
  </section>
</template>
<style scoped>
.department-tree {
  padding: 20px 14px;
  align-self: stretch;
}
.department-tree h3 {
  margin: 0 5px 17px;
}
.department-tree .search-field {
  margin-bottom: 16px;
}
.tree-node {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  text-align: left;
  padding: 11px 12px;
  font-size: 12px;
  border-radius: 6px;
  white-space: nowrap;
}
.tree-node .icon {
  color: var(--color-text-muted);
}
.tree-node > span {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 5px;
  width: 100%;
}
.tree-node small {
  color: var(--color-text-muted);
  font-size: 10px;
}
.tree-node.chosen {
  color: var(--color-primary);
  background: var(--color-primary-soft);
}
.tree-node.chosen .icon {
  color: var(--color-primary);
}
.tree-node:hover {
  background: var(--color-surface-soft);
}
.root-node {
  font-weight: 600;
  margin-bottom: 4px;
}
</style>
