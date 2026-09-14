<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import type { Member, MemberAction, MemberStatus } from '@/types/enterprise'
import { scopeLabels, statusLabels } from '@/types/enterprise'
import { downloadCsv } from '@/utils/format'
import PageHeading from '@/components/ui/PageHeading.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import MemberOverview from '@/components/members/MemberOverview.vue'
import MemberFilters, { type MemberFilterValue } from '@/components/members/MemberFilters.vue'
import MemberTable, { type MemberSortKey } from '@/components/members/MemberTable.vue'
import MemberActionDialog from '@/components/members/MemberActionDialog.vue'
import MemberDetailDrawer from '@/components/members/MemberDetailDrawer.vue'
import MemberBulkDialog from '@/components/members/MemberBulkDialog.vue'
const store = useEnterpriseStore(),
  ui = useUiStore(),
  route = useRoute(),
  router = useRouter()
const defaultFilters = (): MemberFilterValue => ({ query: '', role: '', department: '', status: '' })
const filters = ref(defaultFilters()),
  page = ref(1),
  pageSize = ref(10),
  selected = ref<string[]>([])
const action = ref<MemberAction>('create'),
  actionOpen = ref(false),
  target = ref<Member | null>(null)
const detailId = ref<string | null>(null),
  more = ref<Member | null>(null)
const sortKey = ref<MemberSortKey>(),
  sortDirection = ref<'asc' | 'desc'>('asc')
const batchOpen = ref(false),
  batchAction = ref<'activate' | 'suspend'>('suspend')
const batchTargets = ref<{ id: string; version: number }[]>([])
const detail = computed(() => store.members.find((m) => m.id === detailId.value) ?? null)
const filtered = computed(() => {
  const { query, role, department, status } = filters.value
  const result = store.members.filter(
    (m) =>
      (!query ||
        `${m.name} ${m.email} ${m.phone} ${m.employeeId}`
          .toLowerCase()
          .includes(query.trim().toLowerCase())) &&
      (!role || m.roleIds.includes(role)) &&
      (!department || m.departmentId === department) &&
      (!status || m.status === status),
  )
  if (!sortKey.value) return result
  const key = sortKey.value
  return [...result].sort((a, b) => {
    const value = (m: Member) =>
      key === 'departmentId' ? store.departmentName(m.departmentId) : String(m[key] ?? '')
    return (
      value(a).localeCompare(value(b), 'zh-CN', { numeric: true }) * (sortDirection.value === 'asc' ? 1 : -1)
    )
  })
})
const paged = computed(() =>
  filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
)
const selection = computed(() => store.members.filter((m) => selected.value.includes(m.id)))
const canActivate = computed(
  () =>
    selection.value.length > 0 && selection.value.every((m) => ['invited', 'suspended'].includes(m.status)),
)
const canSuspend = computed(
  () => selection.value.length > 0 && selection.value.every((m) => m.status === 'active'),
)
watch([page, pageSize], () => {
  selected.value = []
})
watch(
  () => filtered.value.length,
  () => {
    page.value = Math.min(page.value, Math.max(1, Math.ceil(filtered.value.length / pageSize.value)))
  },
)
watch(
  () => ui.module,
  (module) => {
    if (module) detailId.value = null
  },
)
watch(
  () => route.query,
  (q) => {
    if (typeof q.q === 'string' || typeof q.department === 'string') {
      applyFilters({
        ...defaultFilters(),
        query: typeof q.q === 'string' ? q.q : '',
        department: typeof q.department === 'string' ? q.department : '',
      })
    }
    if (q.action === 'create' || q.action === 'invite') {
      openAction(q.action)
      void router.replace({ path: route.path, query: {} })
    }
  },
  { immediate: true },
)
function applyFilters(value: MemberFilterValue) {
  filters.value = { ...value }
  page.value = 1
  selected.value = []
  detailId.value = null
}
function clear() {
  applyFilters(defaultFilters())
}
function sort(key: MemberSortKey) {
  sortDirection.value = sortKey.value === key && sortDirection.value === 'asc' ? 'desc' : 'asc'
  sortKey.value = key
  page.value = 1
  selected.value = []
}
function openAction(kind: MemberAction, member: Member | null = null) {
  action.value = kind
  target.value = member ? (JSON.parse(JSON.stringify(member)) as Member) : null
  more.value = null
  detailId.value = null
  actionOpen.value = true
}
function showMore(member: Member) {
  detailId.value = null
  more.value = member
}
function openBatch(kind: 'activate' | 'suspend') {
  batchAction.value = kind
  batchTargets.value = selection.value.map((m) => ({ id: m.id, version: m.version }))
  batchOpen.value = true
}
function finishBatch() {
  batchOpen.value = false
  selected.value = []
}
function exportMembers() {
  try {
    const output = selected.value.length
      ? filtered.value.filter((m) => selected.value.includes(m.id))
      : filtered.value
    downloadCsv('成员列表-界面预览.csv', [
      ['姓名', '手机号', '邮箱', '部门', '角色', '数据权限', '账号状态', '最后登录', '加入时间'],
      ...output.map((m) => [
        m.name,
        m.phone,
        m.email,
        store.departmentName(m.departmentId),
        m.roleIds.map(store.roleName).join('、'),
        scopeLabels[m.scope],
        statusLabels[m.status],
        m.lastLogin || '',
        m.joinedAt,
      ]),
    ])
    store.audit('成员管理', '导出成员列表', `${output.length} 条预览数据`)
    ui.toast(`已导出 ${output.length} 条预览记录。`)
  } catch (e) {
    ui.toast((e as Error).message, 'error')
  }
}
</script>
<template>
  <div class="members-view" :class="{ 'has-detail': Boolean(detail) }">
    <div class="member-main">
      <div class="page-stack members-stack">
        <PageHeading
          title="成员管理"
          description="管理企业成员账号，分配角色权限，控制数据访问范围，保障企业数据安全。"
          banner
          compact
        />
        <MemberOverview />
        <MemberFilters :value="filters" @apply="applyFilters" @reset="clear" />
        <section class="card member-data-panel" aria-label="成员管理列表">
          <div class="row-between member-toolbar">
            <div class="row wrap member-tools">
              <button class="btn btn-primary" @click="openAction('create')">
                <AppIcon name="plus" :size="16" />添加成员
              </button>
              <button class="btn invite-button" @click="openAction('invite')">
                <AppIcon name="invite" :size="16" />邀请成员
              </button>
              <button class="btn" :disabled="!canActivate" @click="openBatch('activate')">
                <AppIcon name="checks" :size="15" />批量启用
              </button>
              <button class="btn" :disabled="!canSuspend" @click="openBatch('suspend')">
                <AppIcon name="lock" :size="14" />批量停用
              </button>
              <button
                class="btn"
                :aria-label="selected.length ? '导出已选' : '导出列表'"
                @click="exportMembers"
              >
                <AppIcon name="download" :size="15" />导出
              </button>
            </div>
            <span class="selection-label"
              >已选择 <strong>{{ selected.length }}</strong> 项<span v-if="selected.length">
                · 当前页</span
              ></span
            >
          </div>
          <MemberTable
            v-if="paged.length"
            v-model:selected="selected"
            :members="paged"
            :detail-id="detailId"
            :sort-key="sortKey"
            :sort-direction="sortDirection"
            @sort="sort"
            @detail="detailId = $event.id"
            @edit="openAction('edit', $event)"
            @more="showMore"
          />
          <EmptyState v-else><button class="btn" @click="clear">清空筛选</button></EmptyState>
          <AppPagination v-model:page="page" v-model:page-size="pageSize" :total="filtered.length" />
        </section>
      </div>
    </div>
    <MemberActionDialog :open="actionOpen" :action="action" :member="target" @close="actionOpen = false" />
    <MemberDetailDrawer :member="detail" @close="detailId = null" @action="openAction" @more="showMore" />
    <MemberBulkDialog
      :open="batchOpen"
      :action="batchAction"
      :targets="batchTargets"
      @close="batchOpen = false"
      @saved="finishBatch"
    />
    <UiDialog
      :open="Boolean(more)"
      :title="(more?.name ?? '') + ' · 成员操作'"
      width="420px"
      @close="more = null"
    >
      <div v-if="more" class="member-operation-list">
        <button
          @click="
            () => {
              detailId = more!.id
              more = null
            }
          "
        >
          <AppIcon name="eye" />查看完整资料
        </button>
        <button @click="openAction('edit', more)"><AppIcon name="edit" />修改成员信息</button>
        <button @click="openAction('role', more)"><AppIcon name="shield" />角色与数据权限变更</button>
        <button @click="openAction('reset', more)"><AppIcon name="key" />密码重置</button>
        <button v-if="more.status === 'active'" class="text-danger" @click="openAction('suspend', more)">
          <AppIcon name="lock" />禁用当前企业访问
        </button>
        <button
          v-if="(['suspended', 'invited'] as MemberStatus[]).includes(more.status)"
          @click="openAction('activate', more)"
        >
          <AppIcon name="success" />重新启用成员
        </button>
        <button v-if="more.status !== 'removed'" class="text-danger" @click="openAction('remove', more)">
          <AppIcon name="logout" />移除成员与交接
        </button>
      </div>
    </UiDialog>
  </div>
</template>
<style scoped>
.member-main {
  min-width: 0;
  container-type: inline-size;
  container-name: member-main;
}
.members-view {
  min-width: 0;
}
.members-stack {
  gap: 16px;
}
.member-data-panel {
  padding: 0 12px 12px;
  border-radius: var(--radius-md);
}
.member-toolbar {
  min-height: 68px;
  padding: 14px 0;
  flex-wrap: wrap;
  gap: 10px;
}
.member-tools {
  gap: 8px;
}
.member-tools .btn {
  border-color: var(--color-border);
}
.member-tools .btn-primary {
  border-color: var(--color-primary);
}
.member-tools .invite-button {
  color: var(--color-primary);
}
.selection-label {
  color: var(--color-text-muted);
  font-size: 11px;
  white-space: nowrap;
  margin-left: auto;
}
.selection-label strong {
  color: var(--color-primary);
  font-weight: 500;
}
.member-data-panel :deep(.pagination) {
  padding: 20px 2px 5px;
  font-size: 12px;
}
.member-data-panel :deep(.pagination-controls) {
  gap: 5px;
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
@media (min-width: 1280px) {
  .members-view {
    transition: margin-right 0.24s cubic-bezier(0.2, 0.8, 0.2, 1);
  }
  .members-view.has-detail {
    margin-right: var(--member-detail-width);
  }
}
@container member-main (max-width: 790px) {
  .member-tools {
    gap: 6px;
  }
  .member-tools .btn {
    padding: 0 10px;
    font-size: 12px;
  }
  .member-data-panel :deep(.page-jump) {
    display: none;
  }
}
@media (prefers-reduced-motion: reduce) {
  .members-view {
    transition: none;
  }
}
</style>
