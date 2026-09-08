<script setup lang="ts">
import { computed, ref } from 'vue'
import { statusLabels, scopeLabels } from '@/types/enterprise'
import SearchField from '@/components/ui/SearchField.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
const query = ref(''),
  group = ref('status')
const source = computed(() => (group.value === 'status' ? statusLabels : scopeLabels))
const entries = computed(() =>
  Object.entries(source.value).filter(([key, label]) => (key + label).includes(query.value)),
)
</script>
<template>
  <div class="page-stack">
    <h2>系统数据字典</h2>
    <div class="notice-box">
      <AppIcon name="database" />成员状态与数据范围是业务契约的一部分，不能通过展示字典改变状态机或授权规则。
    </div>
    <div class="query-bar">
      <SearchField v-model="query" placeholder="搜索字典键或名称…" /><select
        v-model="group"
        class="select"
        aria-label="字典类型"
      >
        <option value="status">成员状态</option>
        <option value="scope">数据范围</option>
      </select>
    </div>
    <div class="table-scroll">
      <table class="data-table">
        <thead>
          <tr>
            <th>字典键</th>
            <th>显示名称</th>
            <th>来源</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="[key, label] in entries" :key="key">
            <td class="mono">{{ key }}</td>
            <td>{{ label }}</td>
            <td>业务契约</td>
            <td class="muted">只读</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
