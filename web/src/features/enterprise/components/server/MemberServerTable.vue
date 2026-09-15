<script setup lang="ts">
import { UiButton } from '@/ui/base'

import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import {
  memberDataScopeLabel,
  memberStatusLabel,
  type EnterpriseTenantMember,
  type MemberMutation,
} from '@/services/enterprise/memberRuntime'

const props = defineProps<{
  members: EnterpriseTenantMember[]
  busy: boolean
}>()

const emit = defineEmits<{
  begin: [kind: MemberMutation, member: EnterpriseTenantMember | null]
}>()

function canActivate(member: EnterpriseTenantMember) {
  return ['TENANT_MEMBER_STATUS_INVITED', 'TENANT_MEMBER_STATUS_SUSPENDED'].includes(member.status)
}

function canSuspend(member: EnterpriseTenantMember) {
  return member.status === 'TENANT_MEMBER_STATUS_ACTIVE'
}

function canRemove(member: EnterpriseTenantMember) {
  return member.status !== 'TENANT_MEMBER_STATUS_REMOVED'
}

function canEdit(member: EnterpriseTenantMember) {
  return member.status !== 'TENANT_MEMBER_STATUS_REMOVED'
}
</script>

<template>
  <section class="card data-panel member-real-panel" aria-label="真实成员列表">
    <div class="row-between member-real-toolbar">
      <div>
        <h2>企业成员</h2>
        <p>档案、部门引用、角色和数据范围均来自服务端。部门名称与层级将在 EC-RI-04 组织域接入后解析。</p>
      </div>
      <UiButton class="btn btn-primary" :disabled="busy" @click="emit('begin', 'invite', null)">
        <AppIcon name="invite" :size="16" />邀请成员
      </UiButton>
    </div>
    <p v-if="busy" class="muted member-loading" role="status">正在读取服务端成员数据…</p>
    <div class="table-scroll">
      <table class="data-table member-real-table">
        <thead>
          <tr>
            <th>姓名 / 邮箱</th><th>手机号</th><th>工号 / 岗位</th><th>部门引用</th><th>角色</th><th>数据范围</th><th>状态 / 版本</th><th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="member in props.members" :key="member.userId">
            <td>
              <strong>{{ member.name || '未填写姓名' }}</strong>
              <small>{{ member.email }}</small>
            </td>
            <td>{{ member.phone || '—' }}</td>
            <td>
              <strong class="mono">{{ member.employeeId || '—' }}</strong>
              <small>{{ member.position || '未填写岗位' }}</small>
            </td>
            <td class="mono">{{ member.departmentId || '未分配' }}</td>
            <td>
              <div v-if="member.roles.length" class="role-pills">
                <span
                  v-for="role in member.roles"
                  :key="role.roleId"
                  :class="['pill', { disabled: role.roleStatus !== 'TENANT_ROLE_STATUS_ACTIVE' }]"
                >
                  {{ role.roleName }}<small v-if="role.roleStatus !== 'TENANT_ROLE_STATUS_ACTIVE'">停用</small>
                </span>
              </div>
              <span v-else>—</span>
            </td>
            <td>{{ memberDataScopeLabel(member.derivedDataScope) }}</td>
            <td>
              <StatusBadge
                :text="memberStatusLabel(member.status)"
                :tone="member.status === 'TENANT_MEMBER_STATUS_ACTIVE' ? 'success' : member.status === 'TENANT_MEMBER_STATUS_SUSPENDED' ? 'warning' : 'primary'"
              />
              <small>v{{ member.version }}</small>
            </td>
            <td>
              <div class="table-actions">
                <UiButton v-if="canEdit(member)" class="btn-link" :disabled="busy" @click="emit('begin', 'profile', member)">档案</UiButton>
                <UiButton v-if="canEdit(member)" class="btn-link" :disabled="busy" @click="emit('begin', 'roles', member)">角色</UiButton>
                <UiButton v-if="canActivate(member)" class="btn-link" :disabled="busy" @click="emit('begin', 'activate', member)">启用</UiButton>
                <UiButton v-if="canSuspend(member)" class="btn-link" :disabled="busy" @click="emit('begin', 'suspend', member)">停用</UiButton>
                <UiButton v-if="canRemove(member)" class="btn-link text-danger" :disabled="busy" @click="emit('begin', 'remove', member)">移除</UiButton>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-if="!busy && !props.members.length" class="member-empty">当前租户暂无可见成员。</div>
  </section>
</template>

<style scoped>
.member-real-panel { padding: 0 12px 12px; }
.member-real-toolbar { min-height: 76px; padding: 16px 4px; gap: 16px; }
.member-real-toolbar h2 { font-size: 16px; margin-bottom: 5px; }
.member-real-toolbar p { color: var(--color-text-muted); font-size: 12px; line-height: 1.6; max-width: 760px; }
.member-real-table { min-width: 1080px; }
.member-real-table th:first-child { min-width: 165px; }
.member-real-table th:nth-child(2) { min-width: 120px; }
.member-real-table th:nth-child(3) { min-width: 135px; }
.member-real-table th:nth-child(4) { min-width: 115px; }
.member-real-table th:nth-child(5) { min-width: 120px; }
.member-real-table td { height: 66px; vertical-align: middle; }
.member-real-table td > strong { display: block; font-size: 12px; font-weight: 600; }
.member-real-table td > small { display: block; margin-top: 4px; color: var(--color-text-muted); font-size: 10px; }
.member-real-table th:last-child,
.member-real-table td:last-child {
  min-width: 175px;
}
.member-loading, .member-empty { padding: 18px 4px; }
.text-danger { color: var(--color-danger); }
.role-pills { display: flex; flex-wrap: wrap; gap: 4px; max-width: 170px; }
.role-pills .pill { display: inline-flex; gap: 4px; align-items: center; }
.role-pills .pill.disabled { opacity: 0.62; }
.role-pills .pill small { font-size: 9px; }
@media (max-width: 900px) {
  .member-real-toolbar { align-items: flex-start; flex-direction: column; }
  .member-real-table th:last-child,
  .member-real-table td:last-child {
    position: sticky;
    right: 0;
    background: var(--color-surface);
    border-left: 1px solid var(--color-border);
    z-index: 1;
  }
  .member-real-table th:last-child { z-index: 2; }
}
</style>
