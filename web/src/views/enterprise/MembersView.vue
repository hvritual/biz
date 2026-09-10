<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import type { Member, MemberAction, MemberStatus } from '@/types/enterprise'
import { statusLabels } from '@/types/enterprise'
import { downloadCsv } from '@/utils/format'
import PageHeading from '@/components/ui/PageHeading.vue'
import MetricCard from '@/components/ui/MetricCard.vue'
import SearchField from '@/components/ui/SearchField.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import MemberTable from '@/components/members/MemberTable.vue'
import MemberActionDialog from '@/components/members/MemberActionDialog.vue'
import MemberDetailDrawer from '@/components/members/MemberDetailDrawer.vue'
const store = useEnterpriseStore(),
  ui = useUiStore(),
  route = useRoute(),
  router = useRouter()
const query = ref(''),
  role = ref(''),
  department = ref(''),
  status = ref(''),
  joinedAfter = ref(''),
  page = ref(1),
  pageSize = ref(10),
  selected = ref<string[]>([])
const action = ref<MemberAction>('create'),
  actionOpen = ref(false),
  target = ref<Member | null>(null),
  detailId = ref<string | null>(null),
  more = ref<Member | null>(null)
const detail = computed(() => store.members.find((m) => m.id === detailId.value) ?? null)
const filtered = computed(() =>
  store.members.filter(
    (m) =>
      (!query.value ||
        `${m.name} ${m.email} ${m.phone} ${m.employeeId}`
          .toLowerCase()
          .includes(query.value.trim().toLowerCase())) &&
      (!role.value || m.roleIds.includes(role.value)) &&
      (!department.value || m.departmentId === department.value) &&
      (!status.value || m.status === status.value) &&
      (!joinedAfter.value || m.joinedAt >= joinedAfter.value),
  ),
)
const paged = computed(() =>
  filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
)
const inUse = computed(() => store.members.filter((m) => m.status !== 'removed').length)
watch([query, role, department, status, joinedAfter], () => {
  page.value = 1
  selected.value = []
})
watch([page, pageSize], () => {
  selected.value = []
})
watch(
  () => route.query,
  (q) => {
    if (typeof q.q === 'string') query.value = q.q
    if (typeof q.department === 'string') department.value = q.department
    if (q.action === 'create' || q.action === 'invite') {
      openAction(q.action)
      void router.replace({ path: route.path, query: {} })
    }
  },
  { immediate: true },
)
function openAction(kind: MemberAction, member: Member | null = null) {
  action.value = kind
  target.value = member ? (JSON.parse(JSON.stringify(member)) as Member) : null
  more.value = null
  detailId.value = null
  actionOpen.value = true
}
function clear() {
  query.value = ''
  role.value = ''
  department.value = ''
  status.value = ''
  joinedAfter.value = ''
}
function exportMembers() {
  const output = selected.value.length
    ? filtered.value.filter((m) => selected.value.includes(m.id))
    : filtered.value
  downloadCsv('成员列表-界面预览.csv', [
    ['姓名', '邮箱', '部门', '角色', '账号状态'],
    ...output.map((m) => [
      m.name,
      m.email,
      store.departmentName(m.departmentId),
      m.roleIds.map(store.roleName).join('、'),
      statusLabels[m.status],
    ]),
  ])
  store.audit('成员管理', '导出成员列表', `${output.length} 条预览数据`)
  ui.toast(`已导出 ${output.length} 条预览记录。`)
}
</script>
<template>
  <div class="page-stack">
    <PageHeading title="成员管理" description="管理企业成员，分配角色权限，助力团队高效协作" banner />
    <div class="metric-grid">
      <MetricCard label="成员总数" :value="inUse" icon="user" caption="当前企业有效成员关系" /><MetricCard
        label="在线成员"
        :value="store.members.filter((m) => m.online && m.status === 'active').length"
        icon="users"
        tone="green"
        caption="预览快照 · 非实时在线统计"
      /><MetricCard
        label="角色数量"
        :value="store.roles.length"
        icon="layers"
        tone="purple"
        caption="内置角色与自定义角色"
      /><MetricCard
        label="待激活成员"
        :value="store.members.filter((m) => m.status === 'invited').length"
        icon="clock"
        tone="orange"
        caption="邀请已创建，等待激活"
      />
    </div>
    <section class="card data-panel">
      <div class="query-bar">
        <SearchField v-model="query" label="搜索成员" placeholder="搜索成员姓名、手机号、邮箱…" /><select
          v-model="role"
          class="select"
          aria-label="筛选角色"
        >
          <option value="">全部角色</option>
          <option v-for="r in store.roles" :key="r.id" :value="r.id">{{ r.name }}</option></select
        ><select v-model="department" class="select" aria-label="筛选部门">
          <option value="">全部部门</option>
          <option v-for="d in store.departments" :key="d.id" :value="d.id">{{ d.name }}</option></select
        ><select v-model="status" class="select" aria-label="筛选账号状态">
          <option value="">全部状态</option>
          <option v-for="(label, key) in statusLabels" :key="key" :value="key">{{ label }}</option></select
        ><input
          v-model="joinedAfter"
          class="input date-filter"
          type="date"
          aria-label="加入时间起始日期"
        /><button class="btn btn-primary" @click="openAction('create')">
          <AppIcon name="plus" :size="17" />新增成员
        </button>
      </div>
      <div class="row-between table-toolbar">
        <div class="row">
          <button class="btn-link" @click="openAction('invite')">
            <AppIcon name="invite" :size="15" />邀请成员</button
          ><button class="btn-link" @click="exportMembers">
            <AppIcon name="download" :size="15" />{{ selected.length ? '导出已选' : '导出列表' }}</button
          ><button
            v-if="query || role || department || status || joinedAfter"
            class="btn-link"
            @click="clear"
          >
            清空筛选
          </button>
        </div>
        <span class="muted selection-label">{{
          selected.length ? '当前页已选 ' + selected.length + ' 项' : '共 ' + filtered.length + ' 位成员'
        }}</span>
      </div>
      <MemberTable
        v-if="paged.length"
        v-model:selected="selected"
        :members="paged"
        @detail="detailId = $event.id"
        @edit="openAction('edit', $event)"
        @more="more = $event"
      /><EmptyState v-else><button class="btn" @click="clear">清空筛选</button></EmptyState
      ><AppPagination v-model:page="page" v-model:page-size="pageSize" :total="filtered.length" />
    </section>
    <MemberActionDialog
      :open="actionOpen"
      :action="action"
      :member="target"
      @close="actionOpen = false"
    /><MemberDetailDrawer :member="detail" @close="detailId = null" @action="openAction" /><UiDialog
      :open="Boolean(more)"
      :title="(more?.name ?? '') + ' · 成员操作'"
      width="420px"
      @close="more = null"
      ><div v-if="more" class="member-operation-list">
        <button
          @click="
            () => {
              detailId = more!.id
              more = null
            }
          "
        >
          <AppIcon name="eye" />查看完整资料</button
        ><button @click="openAction('edit', more)"><AppIcon name="edit" />修改成员信息</button
        ><button @click="openAction('role', more)"><AppIcon name="shield" />角色与数据权限变更</button
        ><button @click="openAction('reset', more)"><AppIcon name="key" />密码重置</button
        ><button v-if="more.status === 'active'" class="text-danger" @click="openAction('suspend', more)">
          <AppIcon name="lock" />禁用当前企业访问</button
        ><button
          v-if="(['suspended', 'invited'] as MemberStatus[]).includes(more.status)"
          @click="openAction('activate', more)"
        >
          <AppIcon name="success" />重新启用成员</button
        ><button v-if="more.status !== 'removed'" class="text-danger" @click="openAction('remove', more)">
          <AppIcon name="logout" />移除成员与交接
        </button>
      </div></UiDialog
    >
  </div>
</template>
<style scoped>
.date-filter {
  max-width: 155px;
}
.selection-label {
  font-size: 12px;
}
.table-toolbar {
  padding: 0 0 2px;
}
.member-operation-list {
  display: flex;
  flex-direction: column;
  gap: 5px;
}
.member-operation-list button {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 13px;
  border-radius: 7px;
  text-align: left;
}
.member-operation-list button:hover {
  background: var(--color-surface-soft);
}
.member-operation-list .icon {
  color: var(--color-text-secondary);
}
.member-operation-list .text-danger .icon {
  color: var(--color-danger);
}
@media (max-width: 1100px) {
  .date-filter {
    display: none;
  }
}
</style>
