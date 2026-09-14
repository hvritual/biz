<script setup lang="ts">
import { computed, onBeforeUnmount, ref, toRaw, watch } from 'vue'
import { useRoute } from 'vue-router'
import PageHeading from '@/components/ui/PageHeading.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import {
  loginUrl,
  logoutSession,
  readSession,
  selectSessionTenant,
  type PermissionGrant,
  type TrustedSession,
} from '@/services/runtime/api'
import {
  actions,
  executeCommand,
  fields,
  labels,
  loadRows,
  rowId,
  rowName,
  rowStatus,
  type Resource,
  type RuntimeRow,
} from '@/services/runtime/commands'
const route = useRoute()
const resource = computed<Resource>(() =>
  route.path === '/platform/tenants' ? 'tenants' : (route.params.resource as Resource),
)
const session = ref<TrustedSession>({ authenticated: false }),
  rows = ref<RuntimeRow[]>([]),
  busy = ref(false),
  error = ref(''),
  notice = ref('')
const action = ref(''),
  selected = ref<RuntimeRow | null>(null),
  input = ref<Record<string, string>>({}),
  permissions = ref<PermissionGrant[]>([]),
  draftKey = ref(''),
  submitted = ref(false)
let epoch = 0
const rowActions = computed(() => Object.entries(actions[resource.value]).filter(([id]) => id !== 'create'))
const canRead = computed(
  () =>
    session.value.authenticated &&
    (resource.value === 'tenants'
      ? session.value.actor_kind === 'platform'
      : !!session.value.active_tenant_id),
)
const identity = (s: TrustedSession) =>
  `${s.actor_kind}/${s.platform_subject}/${s.user_id}/${s.active_tenant_id}`
function clearDraft() {
  action.value = ''
  selected.value = null
  input.value = {}
  permissions.value = []
  submitted.value = false
  draftKey.value = ''
}
async function refresh() {
  const token = ++epoch
  busy.value = true
  error.value = ''
  notice.value = ''
  rows.value = []
  clearDraft()
  try {
    const current = await readSession()
    if (token !== epoch) return
    session.value = current
    if (canRead.value) {
      const result = await loadRows(resource.value, current)
      if (token === epoch) rows.value = result
      return token === epoch
    }
  } catch (e) {
    if (token === epoch) error.value = e instanceof Error ? e.message : '读取失败'
    return false
  } finally {
    if (token === epoch) busy.value = false
  }
}
async function changeTenant(event: Event) {
  const id = (event.target as HTMLSelectElement).value
  ++epoch
  rows.value = []
  clearDraft()
  busy.value = true
  error.value = ''
  try {
    await selectSessionTenant(id)
    await refresh()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '切换失败'
    busy.value = false
  }
}
async function logout() {
  ++epoch
  rows.value = []
  clearDraft()
  busy.value = true
  try {
    await logoutSession()
    session.value = { authenticated: false }
  } catch (e) {
    error.value = e instanceof Error ? e.message : '退出失败'
  } finally {
    busy.value = false
  }
}
function begin(id: string, row: RuntimeRow | null = null) {
  clearDraft()
  action.value = id
  selected.value = row ? structuredClone(toRaw(row)) : null
  input.value = row
    ? { ...('name' in row ? { name: row.name } : {}), ...('siteId' in row ? { siteId: row.siteId } : {}) }
    : {}
  permissions.value = row && 'permissions' in row ? structuredClone(toRaw(row.permissions ?? [])) : []
  draftKey.value = crypto.randomUUID()
  error.value = ''
  notice.value = ''
}
async function submit() {
  const token = epoch,
    originalIdentity = identity(session.value)
  busy.value = true
  error.value = ''
  submitted.value = true
  try {
    const current = await readSession()
    if (token !== epoch) return
    if (!current.authenticated || identity(current) !== originalIdentity) {
      await refresh()
      error.value = '会话已变化，请在当前租户重新选择记录。'
      return
    }
    await executeCommand(
      resource.value,
      action.value,
      selected.value,
      input.value,
      permissions.value,
      draftKey.value,
      session.value,
    )
    if (token !== epoch) return
    const refreshed = await refresh()
    notice.value = refreshed
      ? '操作已提交，列表已从服务端重新读取。'
      : '操作已提交，但列表回读失败，请刷新确认。'
  } catch (e) {
    if (token === epoch) error.value = e instanceof Error ? e.message : '操作失败'
  } finally {
    if (token === epoch) busy.value = false
  }
}
watch(resource, () => void refresh(), { immediate: true })
onBeforeUnmount(() => {
  epoch++
})
</script>
<template>
  <div class="page-stack">
    <PageHeading
      :title="labels[resource]"
      :breadcrumb="resource === 'tenants' ? '平台管理' : '业务工作区'"
      description="管理实际业务记录，权限和操作结果以服务端回读为准。"
    />
    <section class="card panel-pad session-bar">
      <template v-if="session.authenticated"
        ><span>当前身份：{{ session.actor_kind === 'platform' ? '平台操作员' : session.user_id }}</span>
        <label v-if="session.actor_kind !== 'platform'"
          >当前租户
          <select :value="session.active_tenant_id" :disabled="busy" @change="changeTenant">
            <option value="" disabled>选择租户</option>
            <option v-for="tenant in session.tenants" :key="tenant.id" :value="tenant.id">
              {{ tenant.name }}
            </option>
          </select></label
        >
        <button class="btn" :disabled="busy" @click="logout">退出登录</button> </template
      ><a v-else class="btn primary" :href="loginUrl()">登录业务账号</a>
      <button class="btn" :disabled="busy" @click="refresh">刷新</button>
    </section>
    <p v-if="error" role="alert" class="notice-box">{{ error }}</p>
    <p v-if="notice" role="status" class="notice-box">{{ notice }}</p>
    <p v-if="!session.authenticated" class="card panel-pad">请先登录。这里不展示示例业务数据。</p>
    <p v-else-if="!canRead" class="card panel-pad">
      {{ resource === 'tenants' ? '租户管理需要平台身份。' : '请选择可访问的租户。' }}
    </p>
    <section v-else class="card panel-pad">
      <div class="session-bar">
        <h2>{{ labels[resource] }}</h2>
        <button class="btn primary" :disabled="busy" @click="begin('create')">
          {{ actions[resource].create }}
        </button>
      </div>
      <p v-if="busy" role="status">正在读取或提交…</p>
      <div class="table-scroll">
        <table>
          <thead>
            <tr>
              <th>编号</th>
              <th>名称 / 邮箱</th>
              <th>状态 / 点位</th>
              <th>版本</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="rowId(row)">
              <td>{{ rowId(row) }}</td>
              <td>{{ rowName(row) }}</td>
              <td>{{ rowStatus(row) }}</td>
              <td>{{ row.version }}</td>
              <td class="row-actions">
                <button
                  v-for="[id, label] in rowActions"
                  :key="id"
                  class="btn"
                  :disabled="busy"
                  @click="begin(id, row)"
                >
                  {{ label }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-if="!busy && !rows.length">暂无可见记录。</p>
    </section>
    <UiDialog
      :open="Boolean(action)"
      :title="actions[resource][action] ?? '业务操作'"
      @close="!busy && clearDraft()"
    >
      <form class="page-stack" @submit.prevent="submit">
        <p v-if="selected">
          对象：{{ rowName(selected) }}（{{ rowId(selected) }}），版本 {{ selected.version }}
        </p>
        <label v-for="field in fields(resource, action)" :key="field.key"
          >{{ field.label
          }}<input
            v-model="input[field.key]"
            :required="field.required"
            :disabled="busy || submitted"
            :type="field.key.toLowerCase().includes('email') ? 'email' : 'text'"
        /></label>
        <template v-if="action === 'permissions'">
          <p>此操作替换整组权限。空列表会移除角色全部权限。</p>
          <div v-for="(grant, index) in permissions" :key="index" class="session-bar">
            <label>权限代码<input v-model="grant.permission" required :disabled="busy || submitted" /></label
            ><label
              >数据范围<select v-model="grant.scope" :disabled="busy || submitted">
                <option value="DATA_SCOPE_NONE">无</option>
                <option value="DATA_SCOPE_SELF">本人</option>
                <option value="DATA_SCOPE_SITES">授权点位</option>
                <option value="DATA_SCOPE_ALL">全部</option>
              </select></label
            ><button
              type="button"
              class="btn"
              :disabled="busy || submitted"
              @click="permissions.splice(index, 1)"
            >
              移除权限
            </button>
          </div>
          <button
            type="button"
            class="btn"
            :disabled="busy || submitted"
            @click="permissions.push({ permission: '', scope: 'DATA_SCOPE_NONE' })"
          >
            添加权限
          </button>
        </template>
        <p v-if="['suspend', 'close', 'remove', 'disable', 'delete', 'revoke'].includes(action)">
          请确认此操作，可能影响访问权限或移除业务记录。
        </p>
        <p v-if="submitted && error">本次请求内容已锁定，可重试相同操作。若需修改，请关闭后重新选择。</p>
        <p v-if="error" role="alert">{{ error }}</p>
        <div class="session-bar">
          <button type="button" class="btn" :disabled="busy" @click="clearDraft">取消</button
          ><button class="btn primary" :disabled="busy" type="submit">
            {{ submitted && error ? '重试相同操作' : '确认操作' }}
          </button>
        </div>
      </form>
    </UiDialog>
  </div>
</template>
<style scoped>
.session-bar {
  display: flex;
  gap: 16px;
  align-items: center;
  flex-wrap: wrap;
}
.session-bar h2 {
  flex: 1;
}
.table-scroll {
  overflow-x: auto;
}
table {
  width: 100%;
  border-collapse: collapse;
}
td,
th {
  padding: 12px;
  text-align: left;
  border-bottom: 1px solid var(--border-color, #e5e7eb);
}
.row-actions {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
label {
  display: flex;
  gap: 8px;
  flex-direction: column;
}
input,
select {
  min-height: 36px;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 6px;
  padding: 6px 10px;
}
</style>
