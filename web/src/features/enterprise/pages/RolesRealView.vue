<script setup lang="ts">
import { UiButton, UiOption, UiSelect } from '@/ui/base'

import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppIcon from '@/ui/common/AppIcon.vue'
import PageHeading from '@/ui/common/PageHeading.vue'
import SearchField from '@/ui/common/SearchField.vue'
import RoleServerTable from '@/features/enterprise/components/server/RoleServerTable.vue'
import RoleServerEditor from '@/features/enterprise/components/server/RoleServerEditor.vue'
import { loginUrl, logoutSession, type PermissionGrant, type TrustedSession } from '@/services/runtime/api'
import {
  listEnterpriseMembers,
  sameTrustedSession,
  type EnterpriseTenantMember,
} from '@/services/enterprise/memberRuntime'
import {
  assignEnterpriseRoleMember,
  createEnterpriseRole,
  disableEnterpriseRole,
  enableEnterpriseRole,
  getEnterpriseRole,
  listEnterpriseRoles,
  readEnterpriseRoleSession,
  revokeEnterpriseRoleMember,
  roleRequestId,
  roleRuntimeError,
  setEnterpriseRolePermissions,
  switchEnterpriseRoleTenant,
  updateEnterpriseRole,
  type EnterpriseRoleDraft,
  type EnterpriseTenantRole,
  type RoleMutation,
} from '@/services/enterprise/roleRuntime'

const route = useRoute()
const router = useRouter()
const session = ref<TrustedSession>({ authenticated: false })
const roles = ref<EnterpriseTenantRole[]>([])
const members = ref<EnterpriseTenantMember[]>([])
const busy = ref(false)
const error = ref('')
const notice = ref('')
const query = ref('')
const editorOpen = ref(false)
const target = ref<EnterpriseTenantRole | null>(null)
const mutationKeys = ref<Record<string, string>>({})
const draftFingerprint = ref('')
let epoch = 0

const canRead = computed(() => Boolean(session.value.authenticated && session.value.active_tenant_id))
const filteredRoles = computed(() => {
  const value = query.value.trim().toLowerCase()
  return value
    ? roles.value.filter((role) => `${role.name} ${role.id}`.toLowerCase().includes(value))
    : roles.value
})
const activeCount = computed(() => roles.value.filter((role) => role.status === 'TENANT_ROLE_STATUS_ACTIVE').length)
const disabledCount = computed(() => roles.value.filter((role) => role.status === 'TENANT_ROLE_STATUS_DISABLED').length)
const assignedMemberCount = computed(() => members.value.filter((member) => member.roles.length > 0).length)
const memberCounts = computed<Record<string, number>>(() => {
  const counts: Record<string, number> = {}
  for (const member of members.value) {
    if (member.status === 'TENANT_MEMBER_STATUS_REMOVED') continue
    for (const role of member.roles) counts[role.roleId] = (counts[role.roleId] ?? 0) + 1
  }
  return counts
})

function sortedGrants(grants: PermissionGrant[]) {
  return grants
    .map((grant) => ({ permission: grant.permission, scope: grant.scope }))
    .sort((a, b) => a.permission.localeCompare(b.permission))
}

function grantsEqual(a: PermissionGrant[], b: PermissionGrant[]) {
  return JSON.stringify(sortedGrants(a)) === JSON.stringify(sortedGrants(b))
}

function currentMemberIds(roleId: string) {
  return members.value
    .filter((member) => member.roles.some((role) => role.roleId === roleId))
    .map((member) => member.userId)
    .sort()
}

function fingerprint(draft: EnterpriseRoleDraft) {
  return JSON.stringify({
    target: target.value?.id ?? '',
    name: draft.name.trim(),
    enabled: draft.enabled,
    permissions: sortedGrants(draft.permissions),
    memberIds: [...draft.memberIds].sort(),
  })
}

function key(operation: RoleMutation, suffix = '') {
  const id = `${operation}:${suffix}`
  if (!mutationKeys.value[id]) mutationKeys.value[id] = roleRequestId(operation, suffix)
  return mutationKeys.value[id]!
}

function prepareKeys(draft: EnterpriseRoleDraft) {
  const next = fingerprint(draft)
  if (draftFingerprint.value !== next) {
    mutationKeys.value = {}
    draftFingerprint.value = next
  }
}

function closeEditor() {
  if (busy.value) return
  editorOpen.value = false
  target.value = null
  mutationKeys.value = {}
  draftFingerprint.value = ''
  error.value = ''
}

function openEditor(role: EnterpriseTenantRole | null) {
  target.value = role ? { ...role, permissions: role.permissions.map((grant) => ({ ...grant })) } : null
  editorOpen.value = true
  mutationKeys.value = {}
  draftFingerprint.value = ''
  error.value = ''
  notice.value = ''
}

async function refresh() {
  const token = ++epoch
  busy.value = true
  error.value = ''
  notice.value = ''
  roles.value = []
  members.value = []
  try {
    const current = await readEnterpriseRoleSession()
    if (token !== epoch) return
    session.value = current
    if (!current.authenticated || !current.active_tenant_id) return
    const [roleRows, memberRows] = await Promise.all([
      listEnterpriseRoles(current),
      listEnterpriseMembers(current),
    ])
    if (token === epoch) {
      roles.value = roleRows
      members.value = memberRows
    }
  } catch (e) {
    if (token === epoch) error.value = roleRuntimeError(e)
  } finally {
    if (token === epoch) busy.value = false
  }
}

async function changeTenant(event: Event) {
  const tenantId = (event.target as HTMLSelectElement).value
  ++epoch
  busy.value = true
  roles.value = []
  members.value = []
  error.value = ''
  notice.value = ''
  closeEditor()
  try {
    await switchEnterpriseRoleTenant(tenantId)
    await refresh()
  } catch (e) {
    error.value = roleRuntimeError(e)
    busy.value = false
  }
}

async function logout() {
  ++epoch
  busy.value = true
  roles.value = []
  members.value = []
  closeEditor()
  try {
    await logoutSession()
    session.value = { authenticated: false }
  } catch (e) {
    error.value = roleRuntimeError(e)
  } finally {
    busy.value = false
  }
}

async function verifiedRole(sessionValue: TrustedSession, receipt: EnterpriseTenantRole) {
  const readback = await getEnterpriseRole(sessionValue, receipt.id)
  if (
    String(readback.version) !== String(receipt.version) ||
    readback.name !== receipt.name ||
    readback.status !== receipt.status ||
    !grantsEqual(readback.permissions, receipt.permissions)
  ) {
    throw new Error('角色写操作已返回回执，但服务端角色回读与回执不一致。')
  }
  return readback
}

async function saveRole(draft: EnterpriseRoleDraft) {
  if (!draft.name.trim()) {
    error.value = '角色名称不能为空。'
    return
  }
  prepareKeys(draft)
  const token = epoch
  const expectedSession = session.value
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const currentSession = await readEnterpriseRoleSession()
    if (token !== epoch) return
    if (!sameTrustedSession(expectedSession, currentSession)) {
      await refresh()
      error.value = '会话或当前租户已变化，请重新打开角色后再操作。'
      return
    }

    let role = target.value
    const originalRole = target.value
    if (!role) {
      role = await createEnterpriseRole(currentSession, draft.name, key('create'))
      role = await verifiedRole(currentSession, role)
    }

    if (!role.protectedOwner && role.status === 'TENANT_ROLE_STATUS_DISABLED' && draft.enabled) {
      role = await enableEnterpriseRole(currentSession, role, key('enable'))
      role = await verifiedRole(currentSession, role)
    }

    if (!role.protectedOwner && role.name !== draft.name.trim()) {
      role = await updateEnterpriseRole(currentSession, role, draft.name, key('update'))
      role = await verifiedRole(currentSession, role)
    }

    if (!role.protectedOwner && !grantsEqual(role.permissions, draft.permissions)) {
      role = await setEnterpriseRolePermissions(currentSession, role, draft.permissions, key('permissions'))
      role = await verifiedRole(currentSession, role)
    }

    const beforeIds = originalRole ? currentMemberIds(originalRole.id) : []
    const wantedIds = [...draft.memberIds].sort()
    const assigned = wantedIds.filter((userId) => !beforeIds.includes(userId))
    const revoked = beforeIds.filter((userId) => !wantedIds.includes(userId))

    if (assigned.length && role.status !== 'TENANT_ROLE_STATUS_ACTIVE') {
      throw new Error('停用角色不能新增成员绑定；请先启用角色。')
    }
    for (const userId of assigned) {
      await assignEnterpriseRoleMember(currentSession, role.id, userId, key('assign', userId))
    }
    for (const userId of revoked) {
      await revokeEnterpriseRoleMember(currentSession, role.id, userId, key('revoke', userId))
    }

    if (!role.protectedOwner && role.status === 'TENANT_ROLE_STATUS_ACTIVE' && !draft.enabled) {
      role = await disableEnterpriseRole(currentSession, role, key('disable'))
      role = await verifiedRole(currentSession, role)
    }

    const [finalRole, roleRows, memberRows] = await Promise.all([
      getEnterpriseRole(currentSession, role.id),
      listEnterpriseRoles(currentSession),
      listEnterpriseMembers(currentSession),
    ])
    const confirmedMemberIds = memberRows
      .filter((member) => member.roles.some((item) => item.roleId === role!.id))
      .map((member) => member.userId)
      .sort()
    if (JSON.stringify(confirmedMemberIds) !== JSON.stringify(wantedIds)) {
      throw new Error('角色成员写操作已返回回执，但服务端成员回读未确认最终绑定关系。')
    }
    if (token !== epoch) return
    roles.value = roleRows
    members.value = memberRows
    editorOpen.value = false
    target.value = null
    mutationKeys.value = {}
    draftFingerprint.value = ''
    notice.value = `角色配置已由服务端确认：${finalRole.protectedOwner ? '企业所有者' : finalRole.name} · v${finalRole.version}。`
  } catch (e) {
    if (token === epoch) error.value = roleRuntimeError(e)
  } finally {
    if (token === epoch) busy.value = false
  }
}

watch(
  () => route.query.action,
  (action) => {
    if (action === 'create' && canRead.value) {
      openEditor(null)
      void router.replace({ path: route.path, query: {} })
    }
  },
  { immediate: true },
)

void refresh()
onBeforeUnmount(() => { epoch++ })
</script>

<template>
  <div class="page-stack real-roles" data-enterprise-role-source="server">
    <PageHeading
      title="角色权限"
      breadcrumb="企业中心"
      description="角色、权限 grant、状态与成员绑定均以 Access 服务端为准；每次写入都要求幂等执行和回读确认。"
    />

    <section class="card panel-pad role-authority" aria-label="角色服务端身份上下文">
      <template v-if="session.authenticated">
        <div class="authority-main"><strong>服务端身份</strong><span>{{ session.user_id || session.platform_subject || '已认证账号' }}</span></div>
        <label v-if="session.tenants?.length" class="tenant-select">
          <span>当前租户</span>
          <UiSelect :value="session.active_tenant_id" :disabled="busy" @change="changeTenant">
            <UiOption value="" disabled>请选择租户</UiOption>
            <UiOption v-for="tenant in session.tenants" :key="tenant.id" :value="tenant.id">{{ tenant.name }}</UiOption>
          </UiSelect>
        </label>
        <UiButton class="btn" :disabled="busy" @click="refresh"><AppIcon name="refresh" :size="15" />刷新</UiButton>
        <UiButton class="btn" :disabled="busy" @click="logout">退出登录</UiButton>
      </template>
      <template v-else>
        <div class="authority-main"><strong>尚未登录</strong><span>真实角色数据不会回退到本地预览。</span></div>
        <a class="btn btn-primary" :href="loginUrl()">登录业务账号</a>
      </template>
    </section>

    <p v-if="error && !editorOpen" class="notice-box role-error" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice-box" role="status">{{ notice }}</p>

    <template v-if="canRead">
      <div class="role-real-metrics">
        <div class="card"><span>角色总数</span><strong>{{ roles.length }}</strong><small>服务端角色列表</small></div>
        <div class="card"><span>已启用</span><strong>{{ activeCount }}</strong><small>ACTIVE</small></div>
        <div class="card"><span>已停用</span><strong>{{ disabledCount }}</strong><small>DISABLED</small></div>
        <div class="card"><span>已分配成员</span><strong>{{ assignedMemberCount }}</strong><small>成员事实回读</small></div>
      </div>
      <section class="card panel-pad role-toolbar">
        <SearchField v-model="query" placeholder="搜索角色名称或服务端 ID…" />
        <UiButton class="btn btn-primary" :disabled="busy" @click="openEditor(null)"><AppIcon name="plus" :size="15" />新建角色</UiButton>
      </section>
      <RoleServerTable :roles="filteredRoles" :member-counts="memberCounts" :busy="busy" @edit="openEditor" />
    </template>
    <section v-else-if="session.authenticated" class="card panel-pad">请选择可访问租户。角色页面不会展示示例授权作为替代。</section>

    <RoleServerEditor
      :open="editorOpen"
      :role="target"
      :members="members"
      :busy="busy"
      :error="editorOpen ? error : ''"
      @close="closeEditor"
      @submit="saveRole"
    />
  </div>
</template>

<style scoped>
.role-authority { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.authority-main { display: grid; gap: 2px; margin-right: auto; }
.authority-main span, .tenant-select span { color: var(--color-text-muted); font-size: 12px; }
.tenant-select { display: grid; gap: 3px; }
.tenant-select select { min-width: 190px; }
.role-real-metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.role-real-metrics .card { padding: 16px; display: grid; gap: 4px; }
.role-real-metrics span, .role-real-metrics small { color: var(--color-text-muted); }
.role-real-metrics strong { font-size: 24px; }
.role-toolbar { display: flex; justify-content: space-between; gap: 12px; align-items: center; }
.role-toolbar :deep(.search-field) { max-width: 440px; flex: 1; }
@media (max-width: 900px) { .role-real-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 560px) {
  .role-real-metrics { grid-template-columns: 1fr; }
  .role-toolbar { align-items: stretch; flex-direction: column; }
  .tenant-select, .tenant-select select { width: 100%; }
}
</style>
