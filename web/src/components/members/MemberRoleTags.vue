<script setup lang="ts">
import { useEnterpriseStore } from '@/stores/enterprise'
defineProps<{ ids: string[] }>()
const store = useEnterpriseStore()
function tone(id: string) {
  return id === 'owner'
    ? 'owner'
    : ['role-3', 'role-8'].includes(id)
      ? 'green'
      : ['role-5', 'role-6'].includes(id)
        ? 'orange'
        : ['role-7', 'role-10'].includes(id)
          ? 'violet'
          : ['role-2', 'role-11'].includes(id)
            ? 'neutral'
            : 'blue'
}
</script>
<template>
  <span class="member-role-tags"
    ><span v-for="id in ids" :key="id" :class="['member-role-tag', tone(id)]">{{
      store.roleName(id)
    }}</span></span
  >
</template>
<style scoped>
.member-role-tags {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
}
.member-role-tag {
  display: inline-flex;
  padding: 3px 6px;
  border-radius: 4px;
  color: var(--color-primary);
  background: var(--color-primary-soft);
  font-size: 11px;
  line-height: 1.45;
  white-space: nowrap;
}
.owner {
  color: var(--color-danger);
  background: var(--color-danger-soft);
}
.green {
  color: var(--color-success);
  background: var(--color-success-soft);
}
.orange {
  color: var(--color-warning);
  background: var(--color-warning-soft);
}
.violet {
  color: var(--color-violet);
  background: var(--color-violet-soft);
}
.neutral {
  color: var(--color-text-secondary);
  background: var(--color-surface-soft);
}
</style>
