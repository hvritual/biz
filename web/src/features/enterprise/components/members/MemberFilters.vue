<script setup lang="ts">
import { reactive, watch } from 'vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { statusLabels } from '@/types/enterprise'
import SearchField from '@/components/ui/SearchField.vue'
export interface MemberFilterValue {
  query: string
  department: string
  role: string
  status: string
}
const props = defineProps<{ value: MemberFilterValue }>()
const emit = defineEmits<{ apply: [value: MemberFilterValue]; reset: [] }>()
const store = useEnterpriseStore()
const draft = reactive({ ...props.value })
watch(
  () => props.value,
  (value) => Object.assign(draft, value),
  { deep: true },
)
</script>
<template>
  <form class="card member-filters" aria-label="成员筛选" @submit.prevent="emit('apply', { ...draft })">
    <SearchField v-model="draft.query" label="搜索成员" placeholder="搜索姓名、手机号、邮箱…" />
    <label class="filter-item"
      ><span>所属部门</span
      ><select v-model="draft.department" class="select" aria-label="筛选部门">
        <option value="">全部部门</option>
        <option v-for="item in store.departments" :key="item.id" :value="item.id">{{ item.name }}</option>
      </select></label
    >
    <label class="filter-item"
      ><span>角色</span
      ><select v-model="draft.role" class="select" aria-label="筛选角色">
        <option value="">全部角色</option>
        <option v-for="item in store.roles" :key="item.id" :value="item.id">{{ item.name }}</option>
      </select></label
    >
    <label class="filter-item"
      ><span>状态</span
      ><select v-model="draft.status" class="select" aria-label="筛选账号状态">
        <option value="">全部状态</option>
        <option v-for="(text, statusKey) in statusLabels" :key="statusKey" :value="statusKey">
          {{ text }}
        </option>
      </select></label
    >
    <div class="filter-buttons">
      <button class="btn" type="button" @click="emit('reset')">重置</button
      ><button class="btn btn-primary" type="submit">查询</button>
    </div>
  </form>
</template>
<style scoped>
.member-filters {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
  padding: 15px 16px;
  border-radius: var(--radius-md);
}
.member-filters .search-field {
  flex: 1 1 220px;
  min-width: 190px;
}
.filter-item {
  display: flex;
  align-items: center;
  gap: 9px;
  font-size: 12px;
}
.filter-item > span {
  white-space: nowrap;
}
.filter-item .select {
  width: 110px;
  font-size: 12px;
  padding-left: 10px;
}
.filter-buttons {
  display: flex;
  gap: 8px;
}
.filter-buttons .btn {
  min-width: 62px;
}
@container member-main (max-width: 1010px) {
  .member-filters {
    gap: 10px;
    padding: 14px;
  }
  .member-filters .search-field {
    flex-basis: 190px;
  }
  .filter-item {
    gap: 7px;
  }
  .filter-item .select {
    width: 102px;
  }
  .filter-buttons .btn {
    min-width: 55px;
    padding: 0 12px;
  }
}
@container member-main (max-width: 790px) {
  .member-filters .search-field {
    flex-basis: calc(100% - 140px);
  }
  .filter-buttons {
    margin-left: auto;
  }
  .filter-item {
    flex: 1;
  }
  .filter-item .select {
    flex: 1;
  }
}
@media (max-width: 767px) {
  .member-filters .search-field {
    flex-basis: 100%;
  }
  .filter-item {
    flex: 1 1 120px;
  }
}
</style>
