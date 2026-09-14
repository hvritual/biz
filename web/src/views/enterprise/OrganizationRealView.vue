<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import PageHeading from '@/components/ui/PageHeading.vue'
import SearchField from '@/components/ui/SearchField.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import { loginUrl, logoutSession, type TrustedSession } from '@/services/runtime/api'
import {
  listEnterpriseMembers,
  sameTrustedSession,
  type EnterpriseTenantMember,
} from '@/services/enterprise/memberRuntime'
import {
  createEnterpriseDepartment,
  departmentRequestId,
  departmentRuntimeError,
  departmentStatusLabel,
  disableEnterpriseDepartment,
  enableEnterpriseDepartment,
  getEnterpriseDepartment,
  listEnterpriseDepartments,
  readEnterpriseDepartmentSession,
  switchEnterpriseDepartmentTenant,
  updateEnterpriseDepartment,
  type DepartmentMutation,
  type EnterpriseDepartment,
  type EnterpriseDepartmentDraft,
} from '@/services/enterprise/departmentRuntime'

const session = ref<TrustedSession>({ authenticated: false })
const departments = ref<EnterpriseDepartment[]>([])
const members = ref<EnterpriseTenantMember[]>([])
const busy = ref(false)
const error = ref('')
const notice = ref('')
const query = ref('')
const selectedId = ref('')
const editorOpen = ref(false)
const target = ref<EnterpriseDepartment | null>(null)
const draft = ref<EnterpriseDepartmentDraft>(emptyDraft())
const mutationKeys = ref<Record<string, string>>({})
let epoch = 0

const canRead = computed(() => Boolean(session.value.authenticated && session.value.active_tenant_id))
const activeDepartments = computed(() => departments.value.filter((item) => item.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE'))
const activeMembers = computed(() => members.value.filter((item) => item.status === 'TENANT_MEMBER_STATUS_ACTIVE'))
const activeLeaderCount = computed(() => new Set(activeDepartments.value.map((item) => item.leaderUserId).filter(Boolean)).size)
const rootCount = computed(() => departments.value.filter((item) => !item.parentId).length)
const selected = computed(() => departments.value.find((item) => item.departmentId === selectedId.value) ?? null)
const selectedMembers = computed(() => members.value.filter((item) => item.departmentId === selectedId.value && item.status !== 'TENANT_MEMBER_STATUS_REMOVED'))
const parentOptions = computed(() => {
  const forbidden = target.value ? new Set([target.value.departmentId, ...descendantIds(target.value.departmentId)]) : new Set<string>()
  return activeDepartments.value.filter((item) => !forbidden.has(item.departmentId))
})
const treeRows = computed(() => flattenDepartments(departments.value))
const visibleRows = computed(() => {
  const value = query.value.trim().toLowerCase()
  if (!value) return treeRows.value
  return treeRows.value.filter(({ department }) => {
    const leader = memberLabel(department.leaderUserId)
    return `${department.name} ${department.email} ${department.phone} ${leader} ${department.departmentId}`.toLowerCase().includes(value)
  })
})

function emptyDraft(): EnterpriseDepartmentDraft {
  return { name: '', parentId: '', leaderUserId: '', email: '', phone: '', sort: 0, enabled: true }
}

function copyDepartment(value: EnterpriseDepartment): EnterpriseDepartment {
  return { ...value }
}

function memberLabel(userId: string) {
  if (!userId) return '未设置'
  const member = members.value.find((item) => item.userId === userId)
  return member ? (member.name || member.email || member.userId) : userId
}

function parentLabel(parentId: string) {
  if (!parentId) return '顶级部门'
  return departments.value.find((item) => item.departmentId === parentId)?.name ?? parentId
}

function descendantIds(departmentId: string) {
  const result: string[] = []
  const queue = [departmentId]
  const seen = new Set(queue)
  while (queue.length) {
    const current = queue.shift()!
    for (const item of departments.value) {
      if (item.parentId !== current || seen.has(item.departmentId)) continue
      seen.add(item.departmentId)
      result.push(item.departmentId)
      queue.push(item.departmentId)
    }
  }
  return result
}

function flattenDepartments(values: EnterpriseDepartment[]) {
  const ordered = [...values].sort((a, b) => a.sort - b.sort || a.name.localeCompare(b.name) || a.departmentId.localeCompare(b.departmentId))
  const ids = new Set(ordered.map((item) => item.departmentId))
  const children = new Map<string, EnterpriseDepartment[]>()
  for (const item of ordered) {
    const parent = item.parentId && ids.has(item.parentId) ? item.parentId : ''
    const rows = children.get(parent) ?? []
    rows.push(item)
    children.set(parent, rows)
  }
  const result: Array<{ department: EnterpriseDepartment; depth: number }> = []
  const seen = new Set<string>()
  const walk = (parentId: string, depth: number) => {
    for (const item of children.get(parentId) ?? []) {
      if (seen.has(item.departmentId)) continue
      seen.add(item.departmentId)
      result.push({ department: item, depth })
      walk(item.departmentId, depth + 1)
    }
  }
  walk('', 0)
  for (const item of ordered) {
    if (!seen.has(item.departmentId)) result.push({ department: item, depth: 0 })
  }
  return result
}

function requestKey(action: DepartmentMutation) {
  if (!mutationKeys.value[action]) mutationKeys.value[action] = departmentRequestId(action, target.value?.departmentId ?? 'new')
  return mutationKeys.value[action]!
}

function resetEditor() {
  editorOpen.value = false
  target.value = null
  draft.value = emptyDraft()
  mutationKeys.value = {}
}

function openEditor(department: EnterpriseDepartment | null) {
  target.value = department ? copyDepartment(department) : null
  draft.value = department
    ? {
        name: department.name,
        parentId: department.parentId,
        leaderUserId: department.leaderUserId,
        email: department.email,
        phone: department.phone,
        sort: department.sort,
        enabled: department.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE',
      }
    : emptyDraft()
  mutationKeys.value = {}
  error.value = ''
  notice.value = ''
  editorOpen.value = true
}

async function refresh() {
  const token = ++epoch
  busy.value = true
  error.value = ''
  notice.value = ''
  departments.value = []
  members.value = []
  resetEditor()
  try {
    const current = await readEnterpriseDepartmentSession()
    if (token !== epoch) return
    session.value = current
    if (!current.authenticated || !current.active_tenant_id) return
    const [departmentRows, memberRows] = await Promise.all([
      listEnterpriseDepartments(current),
      listEnterpriseMembers(current),
    ])
    if (token !== epoch) return
    departments.value = departmentRows
    members.value = memberRows
    if (!departmentRows.some((item) => item.departmentId === selectedId.value)) {
      selectedId.value = departmentRows[0]?.departmentId ?? ''
    }
  } catch (e) {
    if (token === epoch) error.value = departmentRuntimeError(e)
  } finally {
    if (token === epoch) busy.value = false
  }
}

async function changeTenant(event: Event) {
  const tenantId = (event.target as HTMLSelectElement).value
  ++epoch
  busy.value = true
  departments.value = []
  members.value = []
  selectedId.value = ''
  error.value = ''
  notice.value = ''
  resetEditor()
  try {
    await switchEnterpriseDepartmentTenant(tenantId)
    await refresh()
  } catch (e) {
    error.value = departmentRuntimeError(e)
    busy.value = false
  }
}

async function logout() {
  ++epoch
  busy.value = true
  departments.value = []
  members.value = []
  selectedId.value = ''
  resetEditor()
  try {
    await logoutSession()
    session.value = { authenticated: false }
  } catch (e) {
    error.value = departmentRuntimeError(e)
  } finally {
    busy.value = false
  }
}

function draftMatches(department: EnterpriseDepartment, value: EnterpriseDepartmentDraft) {
  return department.name === value.name.trim()
    && department.parentId === value.parentId.trim()
    && department.leaderUserId === value.leaderUserId.trim()
    && department.email === value.email.trim().toLowerCase()
    && department.phone === value.phone.trim()
    && department.sort === (Number(value.sort) || 0)
}

async function verifiedDepartment(current: TrustedSession, receipt: EnterpriseDepartment) {
  const readback = await getEnterpriseDepartment(current, receipt.departmentId)
  if (
    String(readback.version) !== String(receipt.version)
    || readback.status !== receipt.status
    || readback.name !== receipt.name
    || readback.parentId !== receipt.parentId
    || readback.leaderUserId !== receipt.leaderUserId
  ) {
    throw new Error('部门写操作已返回回执，但服务端部门回读与回执不一致。')
  }
  return readback
}

async function saveDepartment() {
  if (!draft.value.name.trim()) {
    error.value = '部门名称不能为空。'
    return
  }
  if (draft.value.parentId && draft.value.parentId === target.value?.departmentId) {
    error.value = '部门不能选择自身作为上级部门。'
    return
  }
  const token = epoch
  const expectedSession = session.value
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const current = await readEnterpriseDepartmentSession()
    if (token !== epoch) return
    if (!sameTrustedSession(expectedSession, current)) {
      await refresh()
      error.value = '会话或当前租户已变化，请重新打开部门后再操作。'
      return
    }

    let department = target.value
    if (!department) {
      department = await createEnterpriseDepartment(current, draft.value, requestKey('create'))
      department = await verifiedDepartment(current, department)
    } else if (!draftMatches(department, draft.value)) {
      department = await updateEnterpriseDepartment(current, department, draft.value, requestKey('update'))
      department = await verifiedDepartment(current, department)
    }

    const enabled = department.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE'
    if (draft.value.enabled !== enabled) {
      department = draft.value.enabled
        ? await enableEnterpriseDepartment(current, department, requestKey('enable'))
        : await disableEnterpriseDepartment(current, department, requestKey('disable'))
      department = await verifiedDepartment(current, department)
    }

    const [finalDepartment, departmentRows, memberRows] = await Promise.all([
      getEnterpriseDepartment(current, department.departmentId),
      listEnterpriseDepartments(current),
      listEnterpriseMembers(current),
    ])
    if (token !== epoch) return
    departments.value = departmentRows
    members.value = memberRows
    selectedId.value = finalDepartment.departmentId
    resetEditor()
    notice.value = `部门配置已由服务端确认：${finalDepartment.name} · ${departmentStatusLabel(finalDepartment.status)} · v${finalDepartment.version}。`
  } catch (e) {
    if (token === epoch) error.value = departmentRuntimeError(e)
  } finally {
    if (token === epoch) busy.value = false
  }
}

void refresh()
onBeforeUnmount(() => { epoch++ })
</script>

<template>
  <div class="page-stack organization-real" data-enterprise-organization-source="server">
    <PageHeading
      title="组织架构"
      breadcrumb="企业中心"
      description="部门层级、负责人、状态与成员归属均以 Access 服务端为准；层级移动、负责人和成员转入由服务端组织规则校验。"
    />

    <section class="card panel-pad organization-authority" aria-label="组织架构服务端身份上下文">
      <template v-if="session.authenticated">
        <div class="authority-main">
          <strong>服务端身份</strong>
          <span>{{ session.user_id || session.platform_subject || '已认证账号' }}</span>
        </div>
        <label v-if="session.tenants?.length" class="tenant-select">
          <span>当前租户</span>
          <select :value="session.active_tenant_id" :disabled="busy" @change="changeTenant">
            <option value="" disabled>请选择租户</option>
            <option v-for="tenant in session.tenants" :key="tenant.id" :value="tenant.id">{{ tenant.name }}</option>
          </select>
        </label>
        <button class="btn" :disabled="busy" @click="refresh"><AppIcon name="refresh" :size="15" />刷新</button>
        <button class="btn" :disabled="busy" @click="logout">退出登录</button>
      </template>
      <template v-else>
        <div class="authority-main">
          <strong>尚未登录</strong>
          <span>真实组织数据不会回退到本地预览。</span>
        </div>
        <a class="btn btn-primary" :href="loginUrl()">登录业务账号</a>
      </template>
    </section>

    <p v-if="error && !editorOpen" class="notice-box organization-error" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice-box" role="status">{{ notice }}</p>

    <template v-if="canRead">
      <div class="organization-metrics">
        <article class="card"><span>部门总数</span><strong>{{ departments.length }}</strong><small>服务端部门事实</small></article>
        <article class="card"><span>已启用部门</span><strong>{{ activeDepartments.length }}</strong><small>允许新成员转入</small></article>
        <article class="card"><span>活跃成员</span><strong>{{ activeMembers.length }}</strong><small>成员服务端事实</small></article>
        <article class="card"><span>部门负责人</span><strong>{{ activeLeaderCount }}</strong><small>{{ rootCount }} 个顶级部门</small></article>
      </div>

      <section class="card panel-pad organization-toolbar">
        <SearchField v-model="query" placeholder="搜索部门、负责人或服务端 ID…" />
        <button class="btn btn-primary" :disabled="busy" @click="openEditor(null)"><AppIcon name="plus" :size="15" />新建部门</button>
      </section>

      <div class="organization-layout">
        <section class="card organization-tree" aria-label="服务端部门树">
          <header class="section-head">
            <div><strong>部门树</strong><span>按服务端 sort 与层级排列</span></div>
          </header>
          <div v-if="visibleRows.length" class="tree-list">
            <button
              v-for="row in visibleRows"
              :key="row.department.departmentId"
              class="tree-row"
              :class="{ selected: row.department.departmentId === selectedId }"
              :style="{ '--depth': String(row.depth) }"
              @click="selectedId = row.department.departmentId"
            >
              <span class="tree-line"><AppIcon name="organization" :size="15" />{{ row.department.name }}</span>
              <span class="tree-meta">
                <i :class="['status-dot', row.department.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE' ? 'success' : 'muted']" />
                {{ departmentStatusLabel(row.department.status) }} · {{ members.filter((item) => item.departmentId === row.department.departmentId && item.status !== 'TENANT_MEMBER_STATUS_REMOVED').length }} 人
              </span>
            </button>
          </div>
          <div v-else class="empty-state">暂无匹配的服务端部门。</div>
        </section>

        <section class="card organization-detail" aria-label="部门服务端详情">
          <template v-if="selected">
            <header class="detail-head">
              <div>
                <span class="eyebrow">{{ parentLabel(selected.parentId) }}</span>
                <h2>{{ selected.name }}</h2>
                <p>{{ selected.departmentId }} · v{{ selected.version }}</p>
              </div>
              <button class="btn" :disabled="busy" @click="openEditor(selected)"><AppIcon name="edit" :size="15" />编辑部门</button>
            </header>
            <div class="detail-grid">
              <div><span>状态</span><strong>{{ departmentStatusLabel(selected.status) }}</strong></div>
              <div><span>负责人</span><strong>{{ memberLabel(selected.leaderUserId) }}</strong></div>
              <div><span>直属成员</span><strong>{{ selectedMembers.length }}</strong></div>
              <div><span>排序</span><strong>{{ selected.sort }}</strong></div>
              <div><span>联系邮箱</span><strong>{{ selected.email || '未设置' }}</strong></div>
              <div><span>联系电话</span><strong>{{ selected.phone || '未设置' }}</strong></div>
            </div>
            <div v-if="selected.status === 'TENANT_DEPARTMENT_STATUS_DISABLED'" class="organization-rule-note">
              已停用部门保留既有成员归属；服务端会拒绝新的成员转入或从其他部门转入。
            </div>
            <div class="member-section">
              <div class="section-head"><div><strong>直属成员</strong><span>来自成员服务端读模型</span></div></div>
              <div class="table-scroll">
                <table>
                  <thead><tr><th>成员</th><th>职位</th><th>状态</th><th>角色</th></tr></thead>
                  <tbody>
                    <tr v-for="member in selectedMembers" :key="member.userId">
                      <td><strong>{{ member.name || member.email }}</strong><small>{{ member.userId }}</small></td>
                      <td>{{ member.position || '—' }}</td>
                      <td>{{ member.status === 'TENANT_MEMBER_STATUS_ACTIVE' ? '已启用' : member.status }}</td>
                      <td>{{ member.roles.map((role) => role.roleName).join('、') || '未分配角色' }}</td>
                    </tr>
                    <tr v-if="!selectedMembers.length"><td colspan="4" class="empty-cell">当前部门暂无直属成员。</td></tr>
                  </tbody>
                </table>
              </div>
            </div>
          </template>
          <div v-else class="empty-state detail-empty">选择一个部门查看服务端详情，或新建第一个部门。</div>
        </section>
      </div>
    </template>
    <section v-else-if="session.authenticated" class="card panel-pad">请选择可访问租户。组织架构页面不会展示示例部门作为替代。</section>

    <UiDialog :open="editorOpen" :title="target ? '编辑部门' : '新建部门'" width="660px" @close="!busy && resetEditor()">
      <div class="editor-form">
        <label class="field"><span>部门名称 *</span><input v-model="draft.name" maxlength="100" placeholder="例如：客户成功部" /></label>
        <label class="field"><span>上级部门</span><select v-model="draft.parentId"><option value="">顶级部门</option><option v-for="item in parentOptions" :key="item.departmentId" :value="item.departmentId">{{ item.name }}</option></select></label>
        <label class="field"><span>负责人</span><select v-model="draft.leaderUserId"><option value="">未设置</option><option v-for="member in activeMembers" :key="member.userId" :value="member.userId">{{ member.name || member.email || member.userId }}</option></select></label>
        <label class="field"><span>排序</span><input v-model.number="draft.sort" type="number" min="0" step="1" /></label>
        <label class="field"><span>联系邮箱</span><input v-model="draft.email" type="email" maxlength="320" placeholder="department@example.com" /></label>
        <label class="field"><span>联系电话</span><input v-model="draft.phone" maxlength="40" placeholder="可选" /></label>
        <label class="switch-field"><input v-model="draft.enabled" type="checkbox" /><span><strong>启用部门</strong><small>停用后保留历史归属，但禁止新的成员转入。</small></span></label>
        <p v-if="error" class="notice-box organization-error" role="alert">{{ error }}</p>
      </div>
      <template #footer>
        <button class="btn" :disabled="busy" @click="resetEditor">取消</button>
        <button class="btn btn-primary" :disabled="busy" @click="saveDepartment">{{ busy ? '提交中…' : '提交并回读确认' }}</button>
      </template>
    </UiDialog>
  </div>
</template>

<style scoped>
.organization-authority { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.authority-main { display: grid; gap: 2px; margin-right: auto; }
.authority-main span, .tenant-select span { color: var(--color-text-muted); font-size: 12px; }
.tenant-select { display: grid; gap: 3px; }
.tenant-select select { min-width: 190px; }
.organization-metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.organization-metrics article { padding: 16px; display: grid; gap: 4px; }
.organization-metrics span, .organization-metrics small { color: var(--color-text-muted); }
.organization-metrics strong { font-size: 24px; }
.organization-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.organization-toolbar :deep(.search-field) { flex: 1; max-width: 460px; }
.organization-layout { display: grid; grid-template-columns: minmax(270px, 0.8fr) minmax(0, 2fr); gap: 16px; align-items: start; }
.organization-tree, .organization-detail { min-width: 0; overflow: hidden; }
.section-head { padding: 16px 18px; border-bottom: 1px solid var(--color-border); display: flex; justify-content: space-between; gap: 12px; }
.section-head div { display: grid; gap: 2px; }
.section-head span { color: var(--color-text-muted); font-size: 12px; font-weight: 400; }
.tree-list { padding: 8px; display: grid; gap: 3px; }
.tree-row { width: 100%; border: 0; background: transparent; border-radius: var(--radius-md); padding: 10px 10px 10px calc(10px + var(--depth) * 18px); text-align: left; cursor: pointer; color: inherit; display: grid; gap: 4px; }
.tree-row:hover, .tree-row.selected { background: var(--color-primary-soft); }
.tree-line { display: flex; align-items: center; gap: 7px; font-weight: 650; }
.tree-meta { display: flex; align-items: center; gap: 6px; padding-left: 22px; color: var(--color-text-muted); font-size: 12px; }
.status-dot { width: 7px; height: 7px; border-radius: 999px; background: var(--color-text-muted); }
.status-dot.success { background: var(--color-success); }
.status-dot.muted { opacity: .45; }
.empty-state { padding: 28px 18px; color: var(--color-text-muted); text-align: center; }
.detail-empty { min-height: 280px; display: grid; place-items: center; }
.detail-head { padding: 20px; display: flex; justify-content: space-between; gap: 16px; border-bottom: 1px solid var(--color-border); }
.detail-head h2 { margin: 4px 0; font-size: 21px; }
.detail-head p, .eyebrow { margin: 0; color: var(--color-text-muted); font-size: 12px; }
.detail-grid { padding: 18px 20px; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
.detail-grid div { min-width: 0; display: grid; gap: 4px; }
.detail-grid span { color: var(--color-text-muted); font-size: 12px; }
.detail-grid strong { overflow-wrap: anywhere; }
.organization-rule-note { margin: 0 20px 18px; padding: 12px 14px; border-radius: var(--radius-md); background: var(--color-primary-soft); color: var(--color-text-secondary); font-size: 13px; line-height: 1.6; }
.member-section { border-top: 1px solid var(--color-border); }
.table-scroll { overflow-x: auto; }
table { width: 100%; border-collapse: collapse; min-width: 620px; }
th, td { text-align: left; padding: 12px 16px; border-bottom: 1px solid var(--color-border); font-size: 13px; vertical-align: top; }
th { color: var(--color-text-muted); font-weight: 600; }
td strong, td small { display: block; }
td small { margin-top: 3px; color: var(--color-text-muted); }
.empty-cell { text-align: center; color: var(--color-text-muted); padding: 24px; }
.editor-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.field { display: grid; gap: 6px; min-width: 0; }
.field span { font-size: 13px; font-weight: 600; }
.field input, .field select { width: 100%; min-width: 0; }
.switch-field { grid-column: 1 / -1; display: flex; align-items: flex-start; gap: 10px; padding: 12px; border: 1px solid var(--color-border); border-radius: var(--radius-md); }
.switch-field input { margin-top: 3px; }
.switch-field span { display: grid; gap: 3px; }
.switch-field small { color: var(--color-text-muted); line-height: 1.5; }
.editor-form .notice-box { grid-column: 1 / -1; }
@media (max-width: 1000px) {
  .organization-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .organization-layout { grid-template-columns: 1fr; }
}
@media (max-width: 600px) {
  .organization-metrics { grid-template-columns: 1fr; }
  .organization-toolbar { align-items: stretch; flex-direction: column; }
  .organization-toolbar :deep(.search-field) { max-width: none; }
  .tenant-select, .tenant-select select { width: 100%; }
  .detail-head { flex-direction: column; align-items: stretch; }
  .detail-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .editor-form { grid-template-columns: 1fr; }
}
</style>
