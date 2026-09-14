<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useEnterpriseStore } from '@/stores/enterprise'
import type { Role } from '@/types/enterprise'
import { scopeLabels } from '@/types/enterprise'
import PageHeading from '@/components/ui/PageHeading.vue'
import MetricCard from '@/components/ui/MetricCard.vue'
import SearchField from '@/components/ui/SearchField.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import RoleEditor from '@/components/roles/RoleEditor.vue'
const store = useEnterpriseStore(),
  route = useRoute(),
  router = useRouter()
const query = ref(''),
  kind = ref(''),
  editorOpen = ref(false),
  target = ref<Role | null>(null)
const filtered = computed(() =>
  store.roles.filter(
    (r) =>
      (r.name + r.description).includes(query.value) &&
      (!kind.value || (kind.value === 'builtin') === r.builtin),
  ),
)
const memberCount = (id: string) =>
  store.members.filter((m) => m.roleIds.includes(id) && m.status !== 'removed').length
function edit(role: Role | null) {
  target.value = role
  editorOpen.value = true
}
function copy(role: Role) {
  target.value = {
    ...JSON.parse(JSON.stringify(role)),
    id: crypto.randomUUID(),
    name: role.name + '（副本）',
    builtin: false,
  }
  editorOpen.value = true
}
watch(
  () => route.query.action,
  (a) => {
    if (a === 'create') {
      edit(null)
      void router.replace({ path: route.path, query: {} })
    }
  },
  { immediate: true },
)
</script>
<template>
  <div class="page-stack">
    <PageHeading title="角色权限" description="以最小必要权限分配职责，独立控制功能权限与数据范围" />
    <div class="metric-grid">
      <MetricCard
        label="角色总数"
        :value="store.roles.length"
        icon="shield"
        caption="统一的角色授权管理"
      /><MetricCard
        label="内置角色"
        :value="store.roles.filter((r) => r.builtin).length"
        icon="layers"
        caption="内置角色受保护，不可直接修改"
      /><MetricCard
        label="自定义角色"
        :value="store.roles.filter((r) => !r.builtin).length"
        icon="file"
        tone="purple"
        caption="按业务岗位灵活配置"
      /><MetricCard
        label="已分配成员"
        :value="store.members.filter((m) => m.roleIds.length && m.status !== 'removed').length"
        icon="users"
        tone="green"
        caption="按成员去重统计"
      />
    </div>
    <section class="card data-panel">
      <div class="query-bar">
        <SearchField v-model="query" placeholder="搜索角色名称、说明…" /><select
          v-model="kind"
          class="select"
          aria-label="角色类型"
        >
          <option value="">全部类型</option>
          <option value="builtin">内置角色</option>
          <option value="custom">自定义角色</option></select
        ><button class="btn btn-primary" @click="edit(null)">
          <AppIcon name="plus" :size="16" />新建角色
        </button>
      </div>
      <div v-if="filtered.length" class="table-scroll">
        <table class="data-table role-table">
          <thead>
            <tr>
              <th>角色名称</th>
              <th>角色类型</th>
              <th>关联成员</th>
              <th>数据范围</th>
              <th>状态</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in filtered" :key="r.id">
              <td>
                <div class="row">
                  <span :class="['role-icon', { owner: r.id === 'owner' }]"
                    ><AppIcon :name="r.id === 'owner' ? 'crown' : 'shield'" :size="18"
                  /></span>
                  <div>
                    <button class="role-name" @click="edit(r)">{{ r.name }}</button
                    ><small class="muted role-description">{{ r.description }}</small>
                  </div>
                </div>
              </td>
              <td>
                <span class="pill">{{ r.builtin ? '内置角色' : '自定义角色' }}</span>
              </td>
              <td class="numeric">{{ memberCount(r.id) }} 人</td>
              <td>{{ scopeLabels[r.scope] }}</td>
              <td>
                <StatusBadge :text="r.enabled ? '启用' : '禁用'" :tone="r.enabled ? 'success' : 'neutral'" />
              </td>
              <td class="muted numeric">{{ r.updatedAt }}</td>
              <td>
                <div class="table-actions">
                  <button class="btn-link" @click="edit(r)">{{ r.builtin ? '查看' : '编辑' }}</button
                  ><button class="btn-link" :aria-label="'复制 ' + r.name" @click="copy(r)">复制</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState v-else />
      <div class="role-footer muted">共 {{ filtered.length }} 个角色 · 角色变更将在操作日志中保留记录</div>
    </section>
    <RoleEditor :open="editorOpen" :role="target" @close="editorOpen = false" />
  </div>
</template>
<style scoped>
.role-table {
  min-width: 980px;
}
.role-icon {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}
.role-icon.owner {
  background: var(--color-primary);
  color: var(--color-on-primary);
}
.role-name {
  padding: 0;
  font-size: 13px;
  font-weight: 600;
}
.role-name:hover {
  color: var(--color-primary);
}
.role-description {
  display: block;
  font-size: 10px;
  max-width: 245px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-top: 3px;
}
.role-footer {
  padding: 20px 3px 5px;
  font-size: 12px;
}
</style>
