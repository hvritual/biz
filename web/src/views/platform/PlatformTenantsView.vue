<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, toRaw } from 'vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import PageHeading from '@/components/ui/PageHeading.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import {
  loginUrl,
  logoutSession,
  readSession,
  type Tenant,
  type TrustedSession,
} from '@/services/runtime/api'
import { actions, executeCommand, fields, loadRows } from '@/services/runtime/commands'

const session = ref<TrustedSession>({ authenticated: false })
const tenants = ref<Tenant[]>([])
const busy = ref(false)
const error = ref('')
const notice = ref('')
const keyword = ref('')
const statusFilter = ref('all')
const page = ref(1)
const pageSize = 10
const action = ref('')
const selected = ref<Tenant | null>(null)
const input = ref<Record<string, string>>({})
const draftKey = ref('')
const submitted = ref(false)
let epoch = 0

const canRead = computed(() => session.value.authenticated && session.value.actor_kind === 'platform')
const platformIdentity = computed(() => session.value.platform_subject || '平台操作员')
const summary = computed(() => ({
  total: tenants.value.length,
  active: tenants.value.filter((item) => item.status.includes('ACTIVE')).length,
  suspended: tenants.value.filter((item) => item.status.includes('SUSPEND')).length,
  closed: tenants.value.filter((item) => item.status.includes('CLOSED')).length,
}))
const filtered = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  return tenants.value.filter((item) => {
    const matchesKeyword = !q || item.name.toLowerCase().includes(q) || item.id.toLowerCase().includes(q)
    const matchesStatus = statusFilter.value === 'all' || statusKey(item.status) === statusFilter.value
    return matchesKeyword && matchesStatus
  })
})
const totalPages = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize)))
const visibleTenants = computed(() => {
  const safePage = Math.min(page.value, totalPages.value)
  const start = (safePage - 1) * pageSize
  return filtered.value.slice(start, start + pageSize)
})
const firstVisible = computed(() => (filtered.value.length ? (Math.min(page.value, totalPages.value) - 1) * pageSize + 1 : 0))
const lastVisible = computed(() => Math.min(filtered.value.length, firstVisible.value + visibleTenants.value.length - 1))

function identity(value: TrustedSession) {
  return `${value.actor_kind}/${value.platform_subject}/${value.user_id}/${value.active_tenant_id}`
}
function statusKey(status: string) {
  if (status.includes('ACTIVE')) return 'active'
  if (status.includes('SUSPEND')) return 'suspended'
  if (status.includes('CLOSED')) return 'closed'
  return 'other'
}
function statusLabel(status: string) {
  return {
    active: '已启用',
    suspended: '已停用',
    closed: '已关闭',
    other: '未声明',
  }[statusKey(status)]
}
function statusTone(status: string): 'success' | 'warning' | 'danger' | 'neutral' {
  return { active: 'success', suspended: 'warning', closed: 'danger', other: 'neutral' }[statusKey(status)] as
    | 'success'
    | 'warning'
    | 'danger'
    | 'neutral'
}
function clearDraft() {
  action.value = ''
  selected.value = null
  input.value = {}
  submitted.value = false
  draftKey.value = ''
}
function resetFilters() {
  keyword.value = ''
  statusFilter.value = 'all'
  page.value = 1
}
function begin(id: string, tenant: Tenant | null = null) {
  clearDraft()
  action.value = id
  selected.value = tenant ? structuredClone(toRaw(tenant)) : null
  input.value = tenant ? { name: tenant.name } : {}
  draftKey.value = crypto.randomUUID()
  error.value = ''
  notice.value = ''
}
async function refresh() {
  const token = ++epoch
  busy.value = true
  error.value = ''
  notice.value = ''
  tenants.value = []
  clearDraft()
  try {
    const current = await readSession()
    if (token !== epoch) return false
    session.value = current
    if (current.authenticated && current.actor_kind === 'platform') {
      tenants.value = (await loadRows('tenants', current)) as Tenant[]
      page.value = Math.min(page.value, Math.max(1, Math.ceil(filtered.value.length / pageSize)))
    }
    return token === epoch
  } catch (caught) {
    if (token === epoch) error.value = caught instanceof Error ? caught.message : '租户目录读取失败'
    return false
  } finally {
    if (token === epoch) busy.value = false
  }
}
async function logout() {
  ++epoch
  busy.value = true
  tenants.value = []
  clearDraft()
  try {
    await logoutSession()
    session.value = { authenticated: false }
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : '退出失败'
  } finally {
    busy.value = false
  }
}
async function submit() {
  const token = epoch
  const originalIdentity = identity(session.value)
  busy.value = true
  error.value = ''
  submitted.value = true
  try {
    const current = await readSession()
    if (token !== epoch) return
    if (!current.authenticated || identity(current) !== originalIdentity) {
      await refresh()
      error.value = '会话已变化，请使用当前平台身份重新选择租户。'
      return
    }
    await executeCommand('tenants', action.value, selected.value, input.value, [], draftKey.value, session.value)
    if (token !== epoch) return
    const refreshed = await refresh()
    notice.value = refreshed
      ? '操作已提交，租户列表已从服务端重新读取。'
      : '操作已提交，但列表回读失败，请刷新确认。'
  } catch (caught) {
    if (token === epoch) error.value = caught instanceof Error ? caught.message : '操作失败'
  } finally {
    if (token === epoch) busy.value = false
  }
}

onMounted(refresh)
onBeforeUnmount(() => {
  epoch++
})
</script>

<template>
  <div class="page-stack tenant-page" data-ui-template="ListPage" data-testid="platform-tenant-management">
    <div data-ui-region="page-heading">
      <PageHeading
        title="租户管理"
        breadcrumb="平台管理"
        description="管理 SaaS 租户生命周期。套餐版本、模块开通与专项权益在平台管理对应功能中独立维护。"
      />
    </div>

    <section class="identity-strip card" aria-label="当前平台身份">
      <div class="identity-icon"><AppIcon name="shield" :size="20" /></div>
      <div class="flex-1">
        <strong>{{ session.authenticated ? `当前身份：${platformIdentity}` : '尚未建立平台可信会话' }}</strong>
        <p>{{ canRead ? '租户读取和写操作均使用平台可信会话，并在提交后从服务端重新回读。' : '租户目录只对平台身份开放，不向租户身份降级展示。' }}</p>
      </div>
      <template v-if="session.authenticated">
        <StatusBadge :text="canRead ? '平台身份' : '非平台身份'" :tone="canRead ? 'primary' : 'warning'" />
        <button class="btn" type="button" :disabled="busy" @click="logout">退出登录</button>
      </template>
      <a v-else class="btn btn-primary" :href="loginUrl()">登录平台账号</a>
    </section>

    <p v-if="error && !action" class="notice-box danger" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice-box" role="status">{{ notice }}</p>

    <section v-if="!session.authenticated" class="card empty-state">
      <AppIcon name="lock" :size="28" />
      <div><strong>请先登录平台账号</strong><p>这里不展示示例租户数据，也不会在认证失败时回退到本地模拟记录。</p></div>
    </section>
    <section v-else-if="!canRead" class="card empty-state warning" role="alert">
      <AppIcon name="shield" :size="28" />
      <div><strong>租户管理需要平台身份。</strong><p>当前会话属于租户侧身份，平台租户目录没有发起读取请求。</p></div>
    </section>

    <template v-else>
      <section class="metric-grid" data-ui-region="metrics" aria-label="租户概览">
        <article class="card metric-card"><span>租户总数</span><strong>{{ summary.total }}</strong><small>当前平台可见租户</small></article>
        <article class="card metric-card"><span>已启用</span><strong>{{ summary.active }}</strong><small>ACTIVE</small></article>
        <article class="card metric-card"><span>已停用</span><strong>{{ summary.suspended }}</strong><small>SUSPENDED</small></article>
        <article class="card metric-card"><span>已关闭</span><strong>{{ summary.closed }}</strong><small>CLOSED</small></article>
      </section>

      <section class="card query-panel" data-ui-region="query" aria-label="租户筛选">
        <label class="field search-field">
          <span>租户名称 / 编号</span>
          <div class="input-with-icon"><AppIcon name="search" :size="16" /><input v-model="keyword" class="input" placeholder="输入名称或租户编号" @input="page = 1" /></div>
        </label>
        <label class="field status-field">
          <span>租户状态</span>
          <select v-model="statusFilter" class="select" @change="page = 1">
            <option value="all">全部状态</option>
            <option value="active">已启用</option>
            <option value="suspended">已停用</option>
            <option value="closed">已关闭</option>
          </select>
        </label>
        <div class="query-actions">
          <button class="btn" type="button" @click="resetFilters">重置</button>
          <button class="btn" type="button" :disabled="busy" @click="refresh"><AppIcon name="refresh" :size="15" />刷新</button>
        </div>
      </section>

      <section class="card data-panel tenant-data-panel" data-ui-region="data">
        <div class="table-toolbar row-between wrap">
          <div>
            <h2>租户列表</h2>
            <p>生命周期操作与商业权益保持解耦；关闭租户不会在前端隐式修改套餐或模块状态。</p>
          </div>
          <div class="table-actions">
            <span class="result-count">共 {{ filtered.length }} 条</span>
            <button class="btn btn-primary" type="button" :disabled="busy" @click="begin('create')"><AppIcon name="plus" :size="15" />创建租户</button>
          </div>
        </div>

        <p v-if="busy && !tenants.length" role="status" class="table-state">正在从服务端读取租户目录…</p>
        <div v-else class="table-scroll">
          <table class="data-table">
            <thead>
              <tr><th>租户</th><th>状态</th><th>版本</th><th>商业管理</th><th>生命周期操作</th></tr>
            </thead>
            <tbody>
              <tr v-for="tenant in visibleTenants" :key="tenant.id">
                <td><strong class="tenant-name">{{ tenant.name }}</strong><small class="tenant-id">{{ tenant.id }}</small></td>
                <td><StatusBadge :text="statusLabel(tenant.status)" :tone="statusTone(tenant.status)" /></td>
                <td class="numeric">{{ tenant.version }}</td>
                <td><RouterLink class="btn-link" to="/platform/commercial/tenant-entitlements">管理权益 <AppIcon name="right" :size="14" /></RouterLink></td>
                <td>
                  <div class="row-actions">
                    <button class="btn-link" type="button" :disabled="busy" @click="begin('rename', tenant)">修改名称</button>
                    <button class="btn-link" type="button" :disabled="busy" @click="begin('activate', tenant)">启用</button>
                    <button class="btn-link" type="button" :disabled="busy" @click="begin('suspend', tenant)">停用</button>
                    <button class="btn-link danger-link" type="button" :disabled="busy" @click="begin('close', tenant)">关闭租户</button>
                  </div>
                </td>
              </tr>
              <tr v-if="!visibleTenants.length"><td colspan="5" class="no-result">没有符合当前筛选条件的租户。</td></tr>
            </tbody>
          </table>
        </div>

        <div class="pagination" data-ui-region="pagination" aria-label="分页">
          <span>显示 {{ firstVisible }}–{{ lastVisible }} / {{ filtered.length }}</span>
          <div class="pagination-actions">
            <button class="icon-button" type="button" aria-label="上一页" :disabled="page <= 1" @click="page--"><AppIcon name="left" :size="16" /></button>
            <strong>{{ Math.min(page, totalPages) }} / {{ totalPages }}</strong>
            <button class="icon-button" type="button" aria-label="下一页" :disabled="page >= totalPages" @click="page++"><AppIcon name="right" :size="16" /></button>
          </div>
        </div>
      </section>
    </template>

    <UiDialog :open="Boolean(action)" :title="actions.tenants[action] ?? '租户操作'" @close="!busy && clearDraft()">
      <form class="tenant-action-form" @submit.prevent="submit">
        <div v-if="selected" class="action-object">
          <span>操作对象</span><strong>{{ selected.name }}</strong><small>{{ selected.id }} · 版本 {{ selected.version }}</small>
        </div>
        <label v-for="field in fields('tenants', action)" :key="field.key" class="field">
          <span>{{ field.label }}</span>
          <input v-model="input[field.key]" class="input" :required="field.required" :disabled="busy || submitted" :type="field.key.toLowerCase().includes('email') ? 'email' : 'text'" />
        </label>
        <p v-if="action === 'suspend'" class="notice-box warning">停用会影响该租户的运行访问；套餐与模块商业事实不会由前端自动改写。</p>
        <p v-if="action === 'close'" class="notice-box danger">关闭租户属于生命周期终止操作，请确认后提交。商业权益的后续处理仍以服务端规则为准。</p>
        <p v-if="submitted && error" class="notice-box warning">本次请求内容已锁定，可重试相同操作。若需修改，请关闭后重新选择。</p>
        <p v-if="error" role="alert" class="notice-box danger">{{ error }}</p>
        <div class="dialog-actions">
          <button type="button" class="btn" :disabled="busy" @click="clearDraft">取消</button>
          <button class="btn btn-primary" :disabled="busy" type="submit">{{ submitted && error ? '重试相同操作' : '确认操作' }}</button>
        </div>
      </form>
    </UiDialog>
  </div>
</template>

<style scoped>
.tenant-page { gap: 16px; }
.identity-strip { display: flex; align-items: center; gap: 12px; padding: 13px 16px; }
.identity-icon { display: grid; place-items: center; width: 38px; height: 38px; border-radius: 10px; background: var(--color-primary-soft); color: var(--color-primary); }
.identity-strip strong { font-size: var(--text-sm); }
.identity-strip p { margin-top: 2px; color: var(--color-text-muted); font-size: 11px; }
.empty-state { display: flex; align-items: center; gap: 16px; min-height: 128px; padding: 24px; color: var(--color-primary); }
.empty-state.warning { color: var(--color-warning); }
.empty-state p { margin-top: 5px; color: var(--color-text-secondary); font-size: var(--text-sm); }
.metric-card { min-height: 112px; padding: 18px; display: flex; flex-direction: column; }
.metric-card span { color: var(--color-text-secondary); font-size: 12px; }
.metric-card strong { margin-top: 8px; font-size: 28px; line-height: 1.15; font-variant-numeric: tabular-nums; }
.metric-card small { margin-top: auto; padding-top: 9px; color: var(--color-text-muted); }
.query-panel { display: grid; grid-template-columns: minmax(260px, 1fr) 180px auto; gap: 14px; align-items: end; padding: 16px 18px; }
.input-with-icon { position: relative; }
.input-with-icon > .icon { position: absolute; left: 11px; top: 50%; transform: translateY(-50%); color: var(--color-text-muted); pointer-events: none; }
.input-with-icon .input { padding-left: 34px; }
.query-actions { display: flex; gap: 8px; align-items: center; }
.tenant-data-panel { padding: 18px; }
.table-toolbar { margin-bottom: 16px; }
.table-toolbar h2 { font-size: 16px; }
.table-toolbar p { margin-top: 4px; color: var(--color-text-muted); font-size: 11px; }
.result-count { color: var(--color-text-muted); font-size: 12px; }
.tenant-name, .tenant-id { display: block; }
.tenant-name { font-weight: 600; }
.tenant-id { margin-top: 3px; color: var(--color-text-muted); }
.row-actions { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.danger-link { color: var(--color-danger); }
.no-result, .table-state { padding: 30px 14px !important; text-align: center !important; color: var(--color-text-muted); }
.pagination { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding-top: 14px; color: var(--color-text-muted); font-size: 12px; }
.pagination-actions { display: flex; align-items: center; gap: 8px; }
.pagination-actions strong { min-width: 52px; text-align: center; color: var(--color-text-secondary); }
.tenant-action-form { display: flex; flex-direction: column; gap: 16px; }
.action-object { display: flex; flex-direction: column; gap: 3px; padding: 12px 14px; border-radius: var(--radius-md); background: var(--color-surface-soft); }
.action-object span, .action-object small { color: var(--color-text-muted); font-size: 11px; }
.dialog-actions { display: flex; justify-content: flex-end; gap: 8px; padding-top: 4px; }
@media (max-width: 900px) {
  .query-panel { grid-template-columns: 1fr 160px; }
  .query-actions { grid-column: 1 / -1; justify-content: flex-end; }
}
@media (max-width: 767px) {
  .identity-strip { align-items: flex-start; flex-wrap: wrap; }
  .metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .query-panel { grid-template-columns: 1fr; }
  .query-actions { grid-column: auto; justify-content: flex-start; }
  .table-toolbar { align-items: flex-start; }
  .pagination { align-items: flex-start; }
}
@media (max-width: 460px) {
  .metric-grid { grid-template-columns: 1fr; }
  .table-actions { width: 100%; justify-content: space-between; }
}
</style>
