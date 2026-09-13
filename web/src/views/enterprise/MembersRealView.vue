<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import MemberServerTable from '@/components/enterprise/MemberServerTable.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import PageHeading from '@/components/ui/PageHeading.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import { loginUrl, logoutSession, type TenantRole, type TrustedSession } from '@/services/runtime/api'
import {
  activateEnterpriseMember,
  assignEnterpriseMemberRole,
  getEnterpriseMember,
  inviteEnterpriseMember,
  listEnterpriseMembers,
  listEnterpriseRoles,
  memberRequestId,
  memberRoleRequestId,
  memberRuntimeError,
  memberStatusLabel,
  readEnterpriseMemberSession,
  removeEnterpriseMember,
  revokeEnterpriseMemberRole,
  sameTrustedSession,
  suspendEnterpriseMember,
  switchEnterpriseMemberTenant,
  updateEnterpriseMemberProfile,
  type EnterpriseMemberProfileInput,
  type EnterpriseTenantMember,
  type MemberMutation,
  type MemberRoleMutation,
} from '@/services/enterprise/memberRuntime'

const session = ref<TrustedSession>({ authenticated: false })
const members = ref<EnterpriseTenantMember[]>([])
const roles = ref<TenantRole[]>([])
const busy = ref(false)
const error = ref('')
const notice = ref('')
const action = ref<MemberMutation | ''>('')
const selected = ref<EnterpriseTenantMember | null>(null)
const email = ref('')
const requestId = ref('')
const submitted = ref(false)
const profile = ref<EnterpriseMemberProfileInput>(emptyProfile())
const roleBusyId = ref('')
const roleError = ref('')
const roleRetry = ref<{ roleId: string; operation: MemberRoleMutation; key: string } | null>(null)
let epoch = 0

const canRead = computed(() => Boolean(session.value.authenticated && session.value.active_tenant_id))
const activeCount = computed(
  () => members.value.filter((member) => member.status === 'TENANT_MEMBER_STATUS_ACTIVE').length,
)
const pendingCount = computed(
  () => members.value.filter((member) => member.status === 'TENANT_MEMBER_STATUS_INVITED').length,
)
const suspendedCount = computed(
  () => members.value.filter((member) => member.status === 'TENANT_MEMBER_STATUS_SUSPENDED').length,
)

function emptyProfile(): EnterpriseMemberProfileInput {
  return { name: '', phone: '', employeeId: '', position: '', departmentId: '' }
}

function copyMember(member: EnterpriseTenantMember): EnterpriseTenantMember {
  return { ...member, roles: member.roles.map((role) => ({ ...role })) }
}

function clearAction() {
  action.value = ''
  selected.value = null
  email.value = ''
  requestId.value = ''
  submitted.value = false
  profile.value = emptyProfile()
  roleBusyId.value = ''
  roleError.value = ''
  roleRetry.value = null
}

async function refresh() {
  const token = ++epoch
  busy.value = true
  error.value = ''
  notice.value = ''
  members.value = []
  roles.value = []
  clearAction()
  try {
    const current = await readEnterpriseMemberSession()
    if (token !== epoch) return
    session.value = current
    if (!canRead.value) return
    const [rows, roleRows] = await Promise.all([
      listEnterpriseMembers(current),
      listEnterpriseRoles(current),
    ])
    if (token === epoch) {
      members.value = rows
      roles.value = roleRows
    }
  } catch (e) {
    if (token === epoch) error.value = memberRuntimeError(e)
  } finally {
    if (token === epoch) busy.value = false
  }
}

async function changeTenant(event: Event) {
  const tenantId = (event.target as HTMLSelectElement).value
  ++epoch
  busy.value = true
  error.value = ''
  notice.value = ''
  members.value = []
  roles.value = []
  clearAction()
  try {
    await switchEnterpriseMemberTenant(tenantId)
    await refresh()
  } catch (e) {
    error.value = memberRuntimeError(e)
    busy.value = false
  }
}

async function logout() {
  ++epoch
  busy.value = true
  error.value = ''
  notice.value = ''
  members.value = []
  roles.value = []
  clearAction()
  try {
    await logoutSession()
    session.value = { authenticated: false }
  } catch (e) {
    error.value = memberRuntimeError(e)
  } finally {
    busy.value = false
  }
}

function begin(kind: MemberMutation, member: EnterpriseTenantMember | null = null) {
  action.value = kind
  selected.value = member ? copyMember(member) : null
  email.value = ''
  requestId.value = memberRequestId(kind)
  submitted.value = false
  error.value = ''
  notice.value = ''
  roleBusyId.value = ''
  roleError.value = ''
  roleRetry.value = null
  profile.value = member
    ? {
        name: member.name,
        phone: member.phone,
        employeeId: member.employeeId,
        position: member.position,
        departmentId: member.departmentId,
      }
    : emptyProfile()
}

function hasRole(member: EnterpriseTenantMember | null, roleId: string) {
  return Boolean(member?.roles.some((role) => role.roleId === roleId))
}

function actionTitle() {
  switch (action.value) {
    case 'invite': return '邀请成员'
    case 'activate': return '启用成员'
    case 'suspend': return '停用成员'
    case 'remove': return '移除成员'
    case 'profile': return '编辑成员档案'
    case 'roles': return '管理成员角色'
    default: return '成员操作'
  }
}

async function confirmReadback(current: TrustedSession, receipt: EnterpriseTenantMember) {
  const readback = await getEnterpriseMember(current, receipt.userId)
  const refreshed = await listEnterpriseMembers(current)
  if (String(readback.version) !== String(receipt.version) || readback.status !== receipt.status) {
    throw new Error('写操作已返回回执，但服务端回读版本不一致；当前状态未确认。')
  }
  members.value = refreshed
  return readback
}

async function submit() {
  if (!action.value || action.value === 'roles' || !requestId.value) return
  const token = epoch
  const expectedSession = session.value
  busy.value = true
  error.value = ''
  notice.value = ''
  submitted.value = true
  try {
    const current = await readEnterpriseMemberSession()
    if (token !== epoch) return
    if (!sameTrustedSession(expectedSession, current)) {
      await refresh()
      error.value = '会话或当前租户已变化，请重新选择成员后再操作。'
      return
    }

    let receipt: EnterpriseTenantMember
    if (action.value === 'invite') {
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.value.trim())) {
        error.value = '请填写有效邮箱。'
        return
      }
      receipt = await inviteEnterpriseMember(current, email.value, requestId.value)
    } else {
      if (!selected.value) throw new Error('请选择成员。')
      if (action.value === 'profile') {
        receipt = await updateEnterpriseMemberProfile(current, selected.value, profile.value, requestId.value)
      } else {
        receipt =
          action.value === 'activate'
            ? await activateEnterpriseMember(current, selected.value, requestId.value)
            : action.value === 'suspend'
              ? await suspendEnterpriseMember(current, selected.value, requestId.value)
              : await removeEnterpriseMember(current, selected.value, requestId.value)
      }
    }

    const readback = await confirmReadback(current, receipt)
    if (token !== epoch) return
    clearAction()
    notice.value = `成员操作已由服务端确认：${readback.name || readback.email} · ${memberStatusLabel(readback.status)}。`
  } catch (e) {
    if (token === epoch) error.value = memberRuntimeError(e)
  } finally {
    if (token === epoch) busy.value = false
  }
}

async function toggleRole(role: TenantRole) {
  if (!selected.value || roleBusyId.value) return
  const member = selected.value
  const assigned = hasRole(member, role.id)
  const operation: MemberRoleMutation = assigned ? 'revoke' : 'assign'
  if (!assigned && role.status !== 'TENANT_ROLE_STATUS_ACTIVE') return
  const retry = roleRetry.value
  const key = retry && retry.roleId === role.id && retry.operation === operation
    ? retry.key
    : memberRoleRequestId(operation)
  roleRetry.value = { roleId: role.id, operation, key }
  roleBusyId.value = role.id
  roleError.value = ''
  notice.value = ''
  const token = epoch
  const expectedSession = session.value
  try {
    const current = await readEnterpriseMemberSession()
    if (token !== epoch) return
    if (!sameTrustedSession(expectedSession, current)) {
      await refresh()
      roleError.value = '会话或当前租户已变化，请重新选择成员后再操作。'
      return
    }
    if (operation === 'assign') {
      await assignEnterpriseMemberRole(current, member.userId, role.id, key)
    } else {
      await revokeEnterpriseMemberRole(current, member.userId, role.id, key)
    }
    const readback = await getEnterpriseMember(current, member.userId)
    const refreshed = await listEnterpriseMembers(current)
    if (token !== epoch) return
    const nowAssigned = hasRole(readback, role.id)
    if ((operation === 'assign' && !nowAssigned) || (operation === 'revoke' && nowAssigned)) {
      throw new Error('角色写操作已返回回执，但成员回读未确认该角色关系。')
    }
    members.value = refreshed
    selected.value = copyMember(readback)
    roleRetry.value = null
    roleError.value = ''
    notice.value = `角色关系已由服务端确认：${role.name} · ${operation === 'assign' ? '已绑定' : '已解除'}。`
  } catch (e) {
    if (token === epoch) roleError.value = memberRuntimeError(e)
  } finally {
    if (token === epoch) roleBusyId.value = ''
  }
}

void refresh()
onBeforeUnmount(() => {
  epoch++
})
</script>

<template>
  <div class="page-stack real-members" data-enterprise-member-source="server">
    <PageHeading
      title="成员管理"
      breadcrumb="企业中心"
      description="成员档案、角色关系与生命周期均以服务端 Access 数据为准；写操作必须经过可信会话、幂等执行和回读确认。"
    />

    <section class="card panel-pad member-authority" aria-label="成员服务端身份上下文">
      <template v-if="session.authenticated">
        <div class="authority-main">
          <strong>服务端身份</strong>
          <span>{{ session.user_id || session.platform_subject || '已认证账号' }}</span>
        </div>
        <label v-if="session.tenants?.length" class="tenant-select">
          <span>当前租户</span>
          <select :value="session.active_tenant_id" :disabled="busy" @change="changeTenant">
            <option value="" disabled>请选择租户</option>
            <option v-for="tenant in session.tenants" :key="tenant.id" :value="tenant.id">
              {{ tenant.name }}
            </option>
          </select>
        </label>
        <button class="btn" :disabled="busy" @click="refresh"><AppIcon name="refresh" :size="15" />刷新</button>
        <button class="btn" :disabled="busy" @click="logout">退出登录</button>
      </template>
      <template v-else>
        <div class="authority-main">
          <strong>尚未登录</strong>
          <span>真实成员数据不会回退到本地预览。</span>
        </div>
        <a class="btn btn-primary" :href="loginUrl()">登录业务账号</a>
      </template>
    </section>

    <p v-if="error" class="notice-box member-error" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice-box" role="status">{{ notice }}</p>

    <template v-if="canRead">
      <div class="member-real-metrics">
        <div class="card"><span>成员总数</span><strong>{{ members.length }}</strong><small>服务端列表回读</small></div>
        <div class="card"><span>已启用</span><strong>{{ activeCount }}</strong><small>Access ACTIVE</small></div>
        <div class="card"><span>待激活</span><strong>{{ pendingCount }}</strong><small>Access INVITED</small></div>
        <div class="card"><span>已停用</span><strong>{{ suspendedCount }}</strong><small>Access SUSPENDED</small></div>
      </div>
      <MemberServerTable :members="members" :busy="busy" @begin="begin" />
    </template>
    <section v-else-if="session.authenticated" class="card panel-pad">
      请选择可访问租户。成员页面不会展示示例业务数据作为替代。
    </section>

    <UiDialog
      :open="Boolean(action)"
      :title="actionTitle()"
      :width="action === 'roles' ? '620px' : '560px'"
      @close="!busy && !roleBusyId && clearAction()"
    >
      <div class="page-stack">
        <div class="notice-box">
          <AppIcon name="shield" :size="16" />本次操作直接提交到 Access 服务；失败不会写入本地成员快照。
        </div>
        <label v-if="action === 'invite'" class="field">
          <span class="required">成员邮箱</span>
          <input v-model="email" class="input" type="email" :disabled="busy || submitted" autocomplete="off" />
        </label>
        <div v-else-if="action === 'profile' && selected" class="form-grid profile-form">
          <label class="field"><span>姓名</span><input v-model="profile.name" class="input" maxlength="100" :disabled="busy || submitted" /></label>
          <label class="field"><span>手机号</span><input v-model="profile.phone" class="input" maxlength="40" :disabled="busy || submitted" /></label>
          <label class="field"><span>工号</span><input v-model="profile.employeeId" class="input" maxlength="64" :disabled="busy || submitted" /></label>
          <label class="field"><span>岗位</span><input v-model="profile.position" class="input" maxlength="100" :disabled="busy || submitted" /></label>
          <label class="field full-width"><span>部门引用</span><input v-model="profile.departmentId" class="input" maxlength="64" :disabled="busy || submitted" /><small>当前保存后端 department_id；部门树语义由 EC-RI-04 提供。</small></label>
        </div>
        <div v-else-if="action === 'roles' && selected" class="role-manager">
          <div class="member-role-context">
            <strong>{{ selected.name || selected.email }}</strong>
            <span>{{ selected.userId }}</span>
          </div>
          <div v-for="role in roles" :key="role.id" class="role-row">
            <div>
              <strong>{{ role.name }}</strong>
              <small>{{ role.status === 'TENANT_ROLE_STATUS_ACTIVE' ? '角色启用' : '角色已停用' }}</small>
            </div>
            <span v-if="hasRole(selected, role.id)" class="pill">已绑定</span>
            <button
              class="btn"
              :class="{ 'text-danger': hasRole(selected, role.id) }"
              :disabled="Boolean(roleBusyId) || (!hasRole(selected, role.id) && role.status !== 'TENANT_ROLE_STATUS_ACTIVE')"
              @click="toggleRole(role)"
            >
              {{ roleBusyId === role.id ? '处理中…' : roleRetry?.roleId === role.id ? '重试相同操作' : hasRole(selected, role.id) ? '解除' : '绑定' }}
            </button>
          </div>
          <p v-if="!roles.length" class="muted">当前租户没有可配置角色。</p>
          <p v-if="roleError" class="form-error" role="alert">{{ roleError }}</p>
        </div>
        <dl v-else-if="selected" class="detail-list">
          <dt>用户 ID</dt><dd class="mono">{{ selected.userId }}</dd>
          <dt>邮箱</dt><dd>{{ selected.email }}</dd>
          <dt>当前状态</dt><dd>{{ memberStatusLabel(selected.status) }}</dd>
          <dt>当前版本</dt><dd>v{{ selected.version }}</dd>
        </dl>
        <p v-if="action !== 'roles' && submitted && error" class="muted">重试会复用同一个 Idempotency-Key；如需修改操作内容，请关闭后重新发起。</p>
        <p v-if="action !== 'roles' && error" class="form-error" role="alert">{{ error }}</p>
      </div>
      <template #footer>
        <button class="btn" :disabled="busy || Boolean(roleBusyId)" @click="clearAction">{{ action === 'roles' ? '完成' : '取消' }}</button>
        <button v-if="action !== 'roles'" class="btn btn-primary" :disabled="busy" @click="submit">{{ submitted && error ? '重试相同操作' : '确认操作' }}</button>
      </template>
    </UiDialog>
  </div>
</template>

<style scoped>
.member-authority { display: flex; align-items: center; gap: 14px; flex-wrap: wrap; }
.authority-main { display: grid; gap: 4px; margin-right: auto; }
.authority-main span, .tenant-select span, .member-real-metrics small { color: var(--color-text-muted); font-size: 12px; }
.tenant-select { display: flex; align-items: center; gap: 8px; }
.tenant-select select { min-height: 36px; border: 1px solid var(--color-border); border-radius: 7px; padding: 0 10px; background: var(--color-surface); }
.member-real-metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.member-real-metrics > div { padding: 18px; display: grid; gap: 8px; }
.member-real-metrics span { color: var(--color-text-secondary); font-size: 12px; }
.member-real-metrics strong { font-size: 28px; font-variant-numeric: tabular-nums; }
.member-error { border-color: var(--color-danger); }
.text-danger { color: var(--color-danger); }
.profile-form { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.profile-form .field small { color: var(--color-text-muted); font-size: 10px; line-height: 1.5; }
.role-manager { display: grid; gap: 8px; }
.member-role-context { display: grid; gap: 4px; padding-bottom: 12px; border-bottom: 1px solid var(--color-border); }
.member-role-context span { color: var(--color-text-muted); font-family: var(--font-mono); font-size: 11px; }
.role-row { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: 10px; padding: 12px 0; border-bottom: 1px solid var(--color-border); }
.role-row > div { display: grid; gap: 4px; }
.role-row small { color: var(--color-text-muted); font-size: 10px; }
@media (max-width: 900px) {
  .member-real-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 480px) {
  .member-real-metrics { grid-template-columns: 1fr 1fr; gap: 8px; }
  .member-real-metrics > div { padding: 14px; }
  .member-real-metrics strong { font-size: 23px; }
  .member-authority { align-items: stretch; }
  .tenant-select { width: 100%; align-items: stretch; flex-direction: column; }
  .tenant-select select { width: 100%; }
  .profile-form { grid-template-columns: 1fr; }
  .role-row { grid-template-columns: minmax(0, 1fr) auto; }
  .role-row > .pill { grid-column: 1 / -1; width: fit-content; }
}
</style>
