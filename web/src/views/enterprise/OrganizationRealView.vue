<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import OrganizationServerDetail from '@/components/enterprise/OrganizationServerDetail.vue'
import OrganizationServerEditor from '@/components/enterprise/OrganizationServerEditor.vue'
import OrganizationServerTree from '@/components/enterprise/OrganizationServerTree.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import PageHeading from '@/components/ui/PageHeading.vue'
import SearchField from '@/components/ui/SearchField.vue'
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
const mutationKeys = ref<Record<string, string>>({})
let epoch = 0

const canRead = computed(() => Boolean(session.value.authenticated && session.value.active_tenant_id))
const activeDepartments = computed(() => departments.value.filter((item) => item.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE'))
const activeMembers = computed(() => members.value.filter((item) => item.status === 'TENANT_MEMBER_STATUS_ACTIVE'))
const activeLeaderCount = computed(() => new Set(activeDepartments.value.map((item) => item.leaderUserId).filter(Boolean)).size)
const rootCount = computed(() => departments.value.filter((item) => !item.parentId).length)
const selected = computed(() => departments.value.find((item) => item.departmentId === selectedId.value) ?? null)

function requestKey(action: DepartmentMutation) {
  if (!mutationKeys.value[action]) mutationKeys.value[action] = departmentRequestId(action, target.value?.departmentId ?? 'new')
  return mutationKeys.value[action]!
}

function resetEditor() {
  editorOpen.value = false
  target.value = null
  mutationKeys.value = {}
}

function openEditor(department: EnterpriseDepartment | null) {
  target.value = department ? { ...department } : null
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

function draftMatches(department: EnterpriseDepartment, draft: EnterpriseDepartmentDraft) {
  return department.name === draft.name.trim()
    && department.parentId === draft.parentId.trim()
    && department.leaderUserId === draft.leaderUserId.trim()
    && department.email === draft.email.trim().toLowerCase()
    && department.phone === draft.phone.trim()
    && department.sort === (Number(draft.sort) || 0)
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

async function saveDepartment(draft: EnterpriseDepartmentDraft) {
  if (!draft.name.trim()) {
    error.value = '部门名称不能为空。'
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
      department = await createEnterpriseDepartment(current, draft, requestKey('create'))
      department = await verifiedDepartment(current, department)
    } else if (!draftMatches(department, draft)) {
      department = await updateEnterpriseDepartment(current, department, draft, requestKey('update'))
      department = await verifiedDepartment(current, department)
    }

    const enabled = department.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE'
    if (draft.enabled !== enabled) {
      department = draft.enabled
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
        <div class="authority-main"><strong>服务端身份</strong><span>{{ session.user_id || session.platform_subject || '已认证账号' }}</span></div>
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
        <div class="authority-main"><strong>尚未登录</strong><span>真实组织数据不会回退到本地预览。</span></div>
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
        <OrganizationServerTree :departments="departments" :members="members" :selected-id="selectedId" :query="query" @select="selectedId = $event" />
        <OrganizationServerDetail :department="selected" :departments="departments" :members="members" :busy="busy" @edit="openEditor" />
      </div>
    </template>
    <section v-else-if="session.authenticated" class="card panel-pad">请选择可访问租户。组织架构页面不会展示示例部门作为替代。</section>

    <OrganizationServerEditor
      :open="editorOpen"
      :department="target"
      :departments="departments"
      :members="members"
      :busy="busy"
      :error="editorOpen ? error : ''"
      @close="resetEditor"
      @submit="saveDepartment"
    />
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
@media (max-width: 1000px) {
  .organization-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .organization-layout { grid-template-columns: 1fr; }
}
@media (max-width: 600px) {
  .organization-metrics { grid-template-columns: 1fr; }
  .organization-toolbar { align-items: stretch; flex-direction: column; }
  .organization-toolbar :deep(.search-field) { max-width: none; }
  .tenant-select, .tenant-select select { width: 100%; }
}
</style>
