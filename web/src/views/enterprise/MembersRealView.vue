<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import PageHeading from '@/components/ui/PageHeading.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import { loginUrl, logoutSession, type TenantMember, type TrustedSession } from '@/services/runtime/api'
import {
  activateEnterpriseMember,
  getEnterpriseMember,
  inviteEnterpriseMember,
  listEnterpriseMembers,
  memberRequestId,
  memberRuntimeError,
  memberStatusLabel,
  readEnterpriseMemberSession,
  removeEnterpriseMember,
  sameTrustedSession,
  suspendEnterpriseMember,
  switchEnterpriseMemberTenant,
  type MemberMutation,
} from '@/services/enterprise/memberRuntime'

const session = ref<TrustedSession>({ authenticated: false })
const members = ref<TenantMember[]>([])
const busy = ref(false)
const error = ref('')
const notice = ref('')
const action = ref<MemberMutation | ''>('')
const selected = ref<TenantMember | null>(null)
const email = ref('')
const requestId = ref('')
const submitted = ref(false)
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

function clearAction() {
  action.value = ''
  selected.value = null
  email.value = ''
  requestId.value = ''
  submitted.value = false
}

async function refresh() {
  const token = ++epoch
  busy.value = true
  error.value = ''
  notice.value = ''
  members.value = []
  clearAction()
  try {
    const current = await readEnterpriseMemberSession()
    if (token !== epoch) return
    session.value = current
    if (!canRead.value) return
    const rows = await listEnterpriseMembers(current)
    if (token === epoch) members.value = rows
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

function begin(kind: MemberMutation, member: TenantMember | null = null) {
  action.value = kind
  selected.value = member ? structuredClone(member) : null
  email.value = ''
  requestId.value = memberRequestId(kind)
  submitted.value = false
  error.value = ''
  notice.value = ''
}

function canActivate(member: TenantMember) {
  return ['TENANT_MEMBER_STATUS_INVITED', 'TENANT_MEMBER_STATUS_SUSPENDED'].includes(member.status)
}

function canSuspend(member: TenantMember) {
  return member.status === 'TENANT_MEMBER_STATUS_ACTIVE'
}

function canRemove(member: TenantMember) {
  return member.status !== 'TENANT_MEMBER_STATUS_REMOVED'
}

async function submit() {
  if (!action.value || !requestId.value) return
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

    let receipt: TenantMember
    if (action.value === 'invite') {
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.value.trim())) {
        error.value = '请填写有效邮箱。'
        return
      }
      receipt = await inviteEnterpriseMember(current, email.value, requestId.value)
    } else {
      if (!selected.value) throw new Error('请选择成员。')
      receipt =
        action.value === 'activate'
          ? await activateEnterpriseMember(current, selected.value, requestId.value)
          : action.value === 'suspend'
            ? await suspendEnterpriseMember(current, selected.value, requestId.value)
            : await removeEnterpriseMember(current, selected.value, requestId.value)
    }

    // A write receipt is not enough. Confirm the authoritative resource and list again.
    const readback = await getEnterpriseMember(current, receipt.userId)
    const refreshed = await listEnterpriseMembers(current)
    if (token !== epoch) return
    if (String(readback.version) !== String(receipt.version) || readback.status !== receipt.status) {
      throw new Error('写操作已返回回执，但服务端回读版本不一致；当前状态未确认。')
    }
    members.value = refreshed
    clearAction()
    notice.value = `成员操作已由服务端确认：${readback.email} · ${memberStatusLabel(readback.status)}。`
  } catch (e) {
    if (token === epoch) error.value = memberRuntimeError(e)
  } finally {
    if (token === epoch) busy.value = false
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
      description="成员生命周期以服务端 Access 数据为准；写操作必须经过可信会话、幂等执行和回读确认。"
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

      <section class="card data-panel member-real-panel" aria-label="真实成员列表">
        <div class="row-between member-real-toolbar">
          <div>
            <h2>企业成员</h2>
            <p>当前阶段只展示后端成员契约中的权威字段；姓名、电话、工号、部门、岗位与角色档案将在 EC-RI-02 接入。</p>
          </div>
          <button class="btn btn-primary" :disabled="busy" @click="begin('invite')">
            <AppIcon name="invite" :size="16" />邀请成员
          </button>
        </div>
        <p v-if="busy" class="muted member-loading" role="status">正在读取服务端成员数据…</p>
        <div class="table-scroll">
          <table class="data-table member-real-table">
            <thead>
              <tr><th>用户 ID</th><th>邮箱</th><th>状态</th><th>版本</th><th>操作</th></tr>
            </thead>
            <tbody>
              <tr v-for="member in members" :key="member.userId">
                <td class="mono">{{ member.userId }}</td>
                <td>{{ member.email }}</td>
                <td>
                  <StatusBadge
                    :text="memberStatusLabel(member.status)"
                    :tone="member.status === 'TENANT_MEMBER_STATUS_ACTIVE' ? 'success' : member.status === 'TENANT_MEMBER_STATUS_SUSPENDED' ? 'warning' : 'primary'"
                  />
                </td>
                <td class="numeric">v{{ member.version }}</td>
                <td>
                  <div class="table-actions">
                    <button v-if="canActivate(member)" class="btn-link" :disabled="busy" @click="begin('activate', member)">启用</button>
                    <button v-if="canSuspend(member)" class="btn-link" :disabled="busy" @click="begin('suspend', member)">停用</button>
                    <button v-if="canRemove(member)" class="btn-link text-danger" :disabled="busy" @click="begin('remove', member)">移除</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="!busy && !members.length" class="member-empty">当前租户暂无可见成员。</div>
      </section>
    </template>
    <section v-else-if="session.authenticated" class="card panel-pad">
      请选择可访问租户。成员页面不会展示示例业务数据作为替代。
    </section>

    <UiDialog
      :open="Boolean(action)"
      :title="action === 'invite' ? '邀请成员' : action === 'activate' ? '启用成员' : action === 'suspend' ? '停用成员' : '移除成员'"
      width="520px"
      @close="!busy && clearAction()"
    >
      <div class="page-stack">
        <div class="notice-box">
          <AppIcon name="shield" :size="16" />本次操作直接提交到 Access 服务；失败不会写入本地成员快照。
        </div>
        <label v-if="action === 'invite'" class="field">
          <span class="required">成员邮箱</span>
          <input v-model="email" class="input" type="email" :disabled="busy || submitted" autocomplete="off" />
        </label>
        <dl v-else-if="selected" class="detail-list">
          <dt>用户 ID</dt><dd class="mono">{{ selected.userId }}</dd>
          <dt>邮箱</dt><dd>{{ selected.email }}</dd>
          <dt>当前状态</dt><dd>{{ memberStatusLabel(selected.status) }}</dd>
          <dt>当前版本</dt><dd>v{{ selected.version }}</dd>
        </dl>
        <p v-if="submitted && error" class="muted">重试会复用同一个 Idempotency-Key；如需修改操作内容，请关闭后重新发起。</p>
        <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      </div>
      <template #footer>
        <button class="btn" :disabled="busy" @click="clearAction">取消</button>
        <button class="btn btn-primary" :disabled="busy" @click="submit">{{ submitted && error ? '重试相同操作' : '确认操作' }}</button>
      </template>
    </UiDialog>
  </div>
</template>

<style scoped>
.member-authority { display: flex; align-items: center; gap: 14px; flex-wrap: wrap; }
.authority-main { display: grid; gap: 4px; margin-right: auto; }
.authority-main span, .tenant-select span, .member-real-toolbar p, .member-real-metrics small { color: var(--color-text-muted); font-size: 12px; }
.tenant-select { display: flex; align-items: center; gap: 8px; }
.tenant-select select { min-height: 36px; border: 1px solid var(--color-border); border-radius: 7px; padding: 0 10px; background: var(--color-surface); }
.member-real-metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.member-real-metrics > div { padding: 18px; display: grid; gap: 8px; }
.member-real-metrics span { color: var(--color-text-secondary); font-size: 12px; }
.member-real-metrics strong { font-size: 28px; font-variant-numeric: tabular-nums; }
.member-real-panel { padding: 0 12px 12px; }
.member-real-toolbar { min-height: 76px; padding: 16px 4px; gap: 16px; }
.member-real-toolbar h2 { font-size: 16px; margin-bottom: 5px; }
.member-real-toolbar p { line-height: 1.6; max-width: 720px; }
.member-real-table { min-width: 760px; }
.member-real-table td { height: 58px; }
.member-loading, .member-empty { padding: 18px 4px; }
.member-error { border-color: var(--color-danger); }
.text-danger { color: var(--color-danger); }
@media (max-width: 900px) {
  .member-real-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .member-real-toolbar { align-items: flex-start; flex-direction: column; }
}
@media (max-width: 480px) {
  .member-real-metrics { grid-template-columns: 1fr 1fr; gap: 8px; }
  .member-real-metrics > div { padding: 14px; }
  .member-real-metrics strong { font-size: 23px; }
  .member-authority { align-items: stretch; }
  .tenant-select { width: 100%; align-items: stretch; flex-direction: column; }
  .tenant-select select { width: 100%; }
}
</style>
