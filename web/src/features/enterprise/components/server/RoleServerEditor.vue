<script setup lang="ts">
import { UiButton, UiInput, UiOption, UiSelect } from '@/ui/base'

import { computed, ref, watch } from 'vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import type { PermissionGrant } from '@/services/runtime/api'
import type { EnterpriseTenantMember } from '@/services/enterprise/memberRuntime'
import {
  roleGrantScopeOptions,
  rolePermissionCatalog,
  rolePermissionGroups,
  rolePermissionLabel,
} from '@/services/enterprise/rolePermissionCatalog'
import type { EnterpriseRoleDraft, EnterpriseTenantRole } from '@/services/enterprise/roleRuntime'

const props = defineProps<{
  open: boolean
  role: EnterpriseTenantRole | null
  members: EnterpriseTenantMember[]
  busy: boolean
  error: string
}>()
const emit = defineEmits<{ close: []; submit: [EnterpriseRoleDraft] }>()

const name = ref('')
const enabled = ref(true)
const permissions = ref<PermissionGrant[]>([])
const memberIds = ref<string[]>([])
const catalogGroups = rolePermissionGroups()
const protectedOwner = computed(() => Boolean(props.role?.protectedOwner))
const knownPermissions = new Set(rolePermissionCatalog.map((item) => item.permission))
const unknownGrants = computed(() => permissions.value.filter((grant) => !knownPermissions.has(grant.permission)))

function assignedMemberIds(role: EnterpriseTenantRole | null) {
  if (!role) return []
  return props.members
    .filter((member) => member.roles.some((item) => item.roleId === role.id))
    .map((member) => member.userId)
}

watch(
  () => [props.open, props.role, props.members],
  () => {
    if (!props.open) return
    name.value = props.role?.name ?? ''
    enabled.value = props.role ? props.role.status === 'TENANT_ROLE_STATUS_ACTIVE' : true
    permissions.value = (props.role?.permissions ?? []).map((grant) => ({ ...grant }))
    memberIds.value = assignedMemberIds(props.role)
  },
  { immediate: true, deep: true },
)

function grant(permission: string) {
  return permissions.value.find((item) => item.permission === permission)
}

function togglePermission(permission: string) {
  if (protectedOwner.value || props.busy) return
  const current = grant(permission)
  permissions.value = current
    ? permissions.value.filter((item) => item.permission !== permission)
    : [...permissions.value, { permission, scope: 'self' }]
}

function changeScope(permission: string, scope: string) {
  if (protectedOwner.value || props.busy) return
  permissions.value = permissions.value.map((item) =>
    item.permission === permission ? { ...item, scope } : item,
  )
}

function toggleMember(userId: string) {
  if (props.busy) return
  memberIds.value = memberIds.value.includes(userId)
    ? memberIds.value.filter((value) => value !== userId)
    : [...memberIds.value, userId]
}

function memberCanAssign(member: EnterpriseTenantMember) {
  return memberIds.value.includes(member.userId) || (enabled.value && member.status === 'TENANT_MEMBER_STATUS_ACTIVE')
}

function submit() {
  emit('submit', {
    name: name.value.trim(),
    enabled: enabled.value,
    permissions: permissions.value.map((item) => ({ ...item })),
    memberIds: [...memberIds.value],
  })
}
</script>

<template>
  <UiDialog
    :open="open"
    :title="role ? (protectedOwner ? '企业所有者角色' : '管理角色权限') : '新建角色'"
    width="920px"
    @close="!busy && emit('close')"
  >
    <div class="page-stack role-real-editor" data-role-editor="server">
      <div class="notice-box">
        <AppIcon name="shield" :size="16" />
        API 模式直接修改 Access 角色事实。权限、状态与成员关系只有在服务端回读确认后才视为完成。
      </div>

      <div v-if="protectedOwner" class="notice-box owner-protection">
        企业所有者的名称、启用状态和必需权限受服务端保护；本页仅允许调整所有者成员，最后一位所有者仍由服务端拒绝撤销。
      </div>

      <div class="form-grid">
        <label class="field">
          <span class="required">角色名称</span>
          <UiInput
            v-model="name"
            class="input"
            maxlength="80"
            :readonly="protectedOwner"
            :disabled="busy"
            data-role-name
          />
        </label>
        <label class="option-line role-enabled">
          <UiInput v-model="enabled" type="checkbox" :disabled="busy || protectedOwner" data-role-enabled />
          启用此角色
        </label>
      </div>

      <div class="section-heading">
        <div><h3>功能权限与数据范围</h3><p>每条授权独立保存 scope，不再使用一个角色级范围覆盖全部权限。</p></div>
        <span class="pill">{{ permissions.length }} 项</span>
      </div>

      <div class="permission-groups">
        <section v-for="group in catalogGroups" :key="group.name" class="permission-group">
          <h4>{{ group.name }}</h4>
          <div v-for="item in group.permissions" :key="item.permission" class="permission-row">
            <label class="permission-main">
              <UiInput
                type="checkbox"
                :checked="Boolean(grant(item.permission))"
                :disabled="busy || protectedOwner"
                :aria-label="item.label"
                @change="togglePermission(item.permission)"
              />
              <span><strong>{{ item.label }}</strong><small>{{ item.permission }} · {{ item.description }}</small></span>
            </label>
            <UiSelect
              class="select scope-select"
              :value="grant(item.permission)?.scope ?? 'none'"
              :disabled="busy || protectedOwner || !grant(item.permission)"
              :aria-label="item.label + ' 数据范围'"
              @change="changeScope(item.permission, ($event.target as HTMLSelectElement).value)"
            >
              <UiOption v-for="scope in roleGrantScopeOptions" :key="scope.value" :value="scope.value">{{ scope.label }}</UiOption>
            </UiSelect>
          </div>
        </section>
      </div>

      <section v-if="unknownGrants.length" class="unknown-grants">
        <h4>服务端其它授权</h4>
        <p>这些权限不在当前 UI 契约目录中；为避免误删，保存时原样保留。</p>
        <div v-for="item in unknownGrants" :key="item.permission" class="unknown-grant">
          <code>{{ item.permission }}</code><span>{{ rolePermissionLabel(item.permission) }} · {{ item.scope }}</span>
        </div>
      </section>

      <div class="section-heading">
        <div><h3>关联成员</h3><p>绑定/解除分别使用独立幂等键；停用角色不能新增绑定，非启用成员也不能新增绑定。</p></div>
        <span class="pill">{{ memberIds.length }} 人</span>
      </div>
      <div class="member-grid">
        <label v-for="member in members" :key="member.userId" class="member-option">
          <UiInput
            type="checkbox"
            :checked="memberIds.includes(member.userId)"
            :disabled="busy || !memberCanAssign(member)"
            :aria-label="'角色成员 ' + (member.name || member.email)"
            @change="toggleMember(member.userId)"
          />
          <span><strong>{{ member.name || member.email }}</strong><small>{{ member.email }} · {{ member.status }}</small></span>
        </label>
      </div>

      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    </div>
    <template #footer>
      <UiButton class="btn" :disabled="busy" @click="emit('close')">取消</UiButton>
      <UiButton class="btn btn-primary" :disabled="busy || !name.trim()" @click="submit">
        {{ busy ? '服务端处理中…' : '保存并回读确认' }}
      </UiButton>
    </template>
  </UiDialog>
</template>

<style scoped>
.role-enabled { align-self: end; min-height: 38px; }
.section-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; }
.section-heading h3, .permission-group h4, .unknown-grants h4 { margin: 0; }
.section-heading p, .unknown-grants p { margin: 4px 0 0; color: var(--color-text-muted); font-size: 12px; }
.permission-groups { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.permission-group { border: 1px solid var(--color-border); border-radius: 12px; padding: 12px; }
.permission-group h4 { margin-bottom: 8px; }
.permission-row { display: grid; grid-template-columns: minmax(0, 1fr) 128px; gap: 10px; align-items: center; padding: 8px 0; border-top: 1px solid var(--color-border); }
.permission-main { display: flex; gap: 8px; align-items: flex-start; min-width: 0; }
.permission-main span, .member-option span { min-width: 0; }
.permission-main small, .member-option small { display: block; color: var(--color-text-muted); margin-top: 2px; overflow-wrap: anywhere; }
.scope-select { min-width: 0; }
.member-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; max-height: 250px; overflow: auto; }
.member-option { display: flex; gap: 9px; align-items: flex-start; padding: 9px 10px; border: 1px solid var(--color-border); border-radius: 10px; }
.unknown-grants { border: 1px dashed var(--color-border); border-radius: 12px; padding: 12px; }
.unknown-grant { display: flex; justify-content: space-between; gap: 12px; padding-top: 7px; font-size: 12px; }
.owner-protection { border-style: solid; }
@media (max-width: 720px) {
  .permission-groups, .member-grid { grid-template-columns: 1fr; }
  .permission-row { grid-template-columns: 1fr; }
}
</style>
