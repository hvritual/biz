<script setup lang="ts">
import AppIcon from '@/components/ui/AppIcon.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import { roleGrantScopeLabel, rolePermissionLabel } from '@/services/enterprise/rolePermissionCatalog'
import { roleStatusLabel, type EnterpriseTenantRole } from '@/services/enterprise/roleRuntime'

const props = defineProps<{
  roles: EnterpriseTenantRole[]
  memberCounts: Record<string, number>
  busy: boolean
}>()
const emit = defineEmits<{ edit: [EnterpriseTenantRole] }>()

function scopeSummary(role: EnterpriseTenantRole) {
  const values = [...new Set(role.permissions.map((grant) => roleGrantScopeLabel(grant.scope)))]
  return values.length ? values.join(' / ') : '无授权'
}

function permissionSummary(role: EnterpriseTenantRole) {
  if (!role.permissions.length) return '暂无权限'
  const first = role.permissions.slice(0, 2).map((grant) => rolePermissionLabel(grant.permission)).join('、')
  return role.permissions.length > 2 ? `${first} 等 ${role.permissions.length} 项` : first
}
</script>

<template>
  <section class="card data-panel role-server-panel" data-role-source="server">
    <div v-if="roles.length" class="table-scroll">
      <table class="data-table role-server-table">
        <thead>
          <tr>
            <th>角色</th>
            <th>成员</th>
            <th>权限</th>
            <th>数据范围</th>
            <th>状态 / 版本</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="role in roles" :key="role.id">
            <td>
              <div class="role-name-cell">
                <span :class="['role-icon', { owner: role.protectedOwner }]">
                  <AppIcon :name="role.protectedOwner ? 'crown' : 'shield'" :size="17" />
                </span>
                <div>
                  <strong>{{ role.protectedOwner ? '企业所有者' : role.name }}</strong>
                  <small>{{ role.id }}</small>
                </div>
              </div>
            </td>
            <td class="numeric">{{ props.memberCounts[role.id] ?? 0 }} 人</td>
            <td><span class="permission-summary">{{ permissionSummary(role) }}</span></td>
            <td>{{ scopeSummary(role) }}</td>
            <td>
              <div class="status-version">
                <StatusBadge
                  :text="roleStatusLabel(role.status)"
                  :tone="role.status === 'TENANT_ROLE_STATUS_ACTIVE' ? 'success' : 'neutral'"
                />
                <small>v{{ role.version }}</small>
              </div>
            </td>
            <td>
              <button class="btn-link" :disabled="busy" @click="emit('edit', role)">
                {{ role.protectedOwner ? '查看与成员' : '管理' }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <EmptyState v-else />
  </section>
</template>

<style scoped>
.role-server-table { min-width: 1040px; }
.role-name-cell { display: flex; align-items: center; gap: 10px; min-width: 210px; }
.role-name-cell small { display: block; margin-top: 3px; color: var(--color-text-muted); max-width: 210px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.role-icon { width: 32px; height: 32px; border-radius: 50%; display: grid; place-items: center; background: var(--color-primary-soft); color: var(--color-primary); flex: 0 0 auto; }
.role-icon.owner { background: var(--color-primary); color: var(--color-on-primary); }
.permission-summary { display: block; max-width: 230px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.status-version { display: flex; align-items: center; gap: 8px; }
.status-version small { color: var(--color-text-muted); }
@media (max-width: 720px) { .role-server-table { min-width: 930px; } }
</style>
