<script setup lang="ts">
import { UiButton, UiOption, UiSelect } from '@/ui/base'

import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useEnterpriseStore } from '@/stores/enterprise'
import type { Role } from '@/types/enterprise'
import { scopeLabels } from '@/types/enterprise'
import PageHeading from '@/ui/common/PageHeading.vue'
import MetricCard from '@/ui/common/MetricCard.vue'
import SearchField from '@/ui/common/SearchField.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import EmptyState from '@/ui/common/EmptyState.vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import RoleEditor from '@/features/enterprise/components/roles/RoleEditor.vue'
import { currentAuthorizationAllows } from '@/services/runtime/authorization'
const store = useEnterpriseStore(),
  route = useRoute(),
  router = useRouter()
const query = ref(''),
  kind = ref(''),
  status = ref<'' | 'active' | 'disabled'>(''),
  editorOpen = ref(false),
  target = ref<Role | null>(null),
  deleteTarget = ref<Role | null>(null),
  deleting = ref(false),
  deleteError = ref('')
const roleActionsAllowed=(codes:string[])=>store.previewMode||codes.every(currentAuthorizationAllows)
const canCreateRole = computed(() => roleActionsAllowed([
  'tenant.role.create',
  'tenant.role.set_permissions',
  'tenant.role.enable',
  'tenant.role.disable',
]))
const canEditRole = computed(() => roleActionsAllowed([
  'tenant.role.update',
  'tenant.role.set_permissions',
  'tenant.role.enable',
  'tenant.role.disable',
]))
const canDeleteRole = computed(() => roleActionsAllowed(['tenant.role.delete']))
const filtered = computed(() =>
  store.roles.filter(
    (r) =>
      (r.name + r.description).includes(query.value) &&
      (!kind.value || (kind.value === 'builtin') === r.builtin) &&
      (!status.value || r.enabled === (status.value === 'active')),
  ),
)
const memberCount = (role: Role) =>
  store.previewMode
    ? store.members.filter((m) => m.roleIds.includes(role.id) && m.status !== 'removed').length
    : (role.memberCount ?? 0)
function edit(role: Role | null) {
  if (!store.previewMode && !canEditRole.value && role) return
  if (!store.previewMode && !canCreateRole.value && !role) return
  target.value = role
  editorOpen.value = true
}
function copy(role: Role) {
  if (!canCreateRole.value) return
  target.value = {
    ...JSON.parse(JSON.stringify(role)),
    id: crypto.randomUUID(),
    name: role.name + '（副本）',
    roleCode: '',
    builtin: false,
    memberCount: 0,
  }
  editorOpen.value = true
}
async function searchRoles() {
  if (!store.previewMode) await store.queryRoles({ query: query.value, status: status.value })
}
async function resetFilters() {
  query.value = ''
  kind.value = ''
  status.value = ''
  if (!store.previewMode) await store.queryRoles({ query: '', status: '' })
}
function requestDelete(role: Role) {
  if (!canDeleteRole.value || role.builtin) return
  deleteError.value = ''
  deleteTarget.value = role
}
async function confirmDelete() {
  const role = deleteTarget.value
  if (!role) return
  deleting.value = true
  deleteError.value = ''
  try {
    await store.deleteRole(role)
    deleteTarget.value = null
  } catch (cause) {
    deleteError.value = cause instanceof Error ? cause.message : '删除角色失败。'
  } finally {
    deleting.value = false
  }
}
watch(
  () => route.query.action,
  (a) => {
    if (a === 'create') {
      if (canCreateRole.value) edit(null)
      void router.replace({ path: route.path, query: {} })
    }
  },
  { immediate: true },
)
watch(canEditRole,(allowed)=>{if(!allowed&&target.value&&!target.value.builtin)editorOpen.value=false})
onMounted(() => void store.ensureDomains(['roles', 'members']).catch(() => undefined))
</script>
<template>
  <div class="page-stack" data-enterprise-page="roles" data-ui-template="ListPage">
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
    <section class="card data-panel" data-ui-region="data">
      <div class="query-bar" data-ui-region="query">
        <SearchField v-model="query" placeholder="搜索角色名称、说明…" /><UiSelect
          v-model="status"
          class="select"
          aria-label="角色状态"
        >
          <UiOption value="">全部状态</UiOption>
          <UiOption value="active">已启用</UiOption>
          <UiOption value="disabled">已停用</UiOption>
        </UiSelect><UiSelect
          v-model="kind"
          class="select"
          aria-label="角色类型"
        >
          <UiOption value="">全部类型</UiOption>
          <UiOption value="builtin">内置角色</UiOption>
          <UiOption value="custom">自定义角色</UiOption>
        </UiSelect><UiButton class="btn" @click="searchRoles">查询</UiButton><UiButton class="btn" @click="resetFilters">重置</UiButton><UiButton v-if="canCreateRole" class="btn btn-primary" @click="edit(null)">
          <AppIcon name="plus" :size="16" />新建角色
        </UiButton>
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
                    <UiButton v-if="canEditRole || r.builtin" class="role-name" @click="edit(r)">{{ r.name }}</UiButton><strong v-else class="role-name">{{ r.name }}</strong><small class="muted role-description">{{ r.description }}</small>
                  </div>
                </div>
              </td>
              <td>
                <span class="pill">{{ r.builtin ? '内置角色' : '自定义角色' }}</span>
              </td>
              <td class="numeric">{{ memberCount(r) }} 人</td>
              <td>{{ scopeLabels[r.scope] }}</td>
              <td>
                <StatusBadge :text="r.enabled ? '启用' : '禁用'" :tone="r.enabled ? 'success' : 'neutral'" />
              </td>
              <td class="muted numeric">{{ r.updatedAt }}</td>
              <td>
                <div class="table-actions">
                  <UiButton v-if="r.builtin || canEditRole" class="btn-link" @click="edit(r)">{{ r.builtin ? '查看' : '编辑' }}</UiButton><UiButton v-if="canCreateRole && !r.builtin" class="btn-link" :aria-label="'复制 ' + r.name" @click="copy(r)">复制</UiButton><UiButton
                    v-if="canDeleteRole && !r.builtin"
                    class="btn-link"
                    :disabled="memberCount(r) > 0"
                    :title="memberCount(r) > 0 ? '仍有关联成员，请先移除成员绑定' : '删除角色'"
                    :aria-label="'删除 ' + r.name"
                    @click="requestDelete(r)"
                  >删除</UiButton>
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
    <UiDialog
      :open="Boolean(deleteTarget)"
      title="删除角色"
      width="480px"
      @close="deleteTarget = null"
    >
      <p v-if="deleteTarget">确认删除角色“{{ deleteTarget.name }}”？删除后该角色的授权关系将一并清理；如仍有关联成员，系统会阻止删除。</p>
      <p v-if="deleteError" class="form-error" role="alert">{{ deleteError }}</p>
      <template #footer>
        <UiButton class="btn" :disabled="deleting" @click="deleteTarget = null">取消</UiButton>
        <UiButton class="btn btn-primary" :disabled="deleting" @click="confirmDelete">
          {{ deleting ? '正在删除…' : '确认删除' }}
        </UiButton>
      </template>
    </UiDialog>
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
