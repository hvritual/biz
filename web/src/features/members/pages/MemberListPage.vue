<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import PageHeader from "@/shared/ui/PageHeader.vue";
import MetricCard from "@/shared/ui/MetricCard.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import StatusBadge from "@/shared/ui/StatusBadge.vue";
import EmptyState from "@/shared/ui/EmptyState.vue";
import TablePagination from "@/shared/ui/TablePagination.vue";
import MemberEditor from "../components/MemberEditor.vue";
import MemberActionMenu from "../components/MemberActionMenu.vue";
import MemberOperationDialog, {
  type OperationKind,
} from "../components/MemberOperationDialog.vue";
import { useMemberStore } from "../model/memberStore";
import { filterMembers } from "../model/rules";
import { type Member, type MemberQuery } from "../model/types";
import { useTenantStore } from "@/services/tenant";
import { useAuditStore } from "@/services/audit";
import { downloadText, csvCell } from "@/shared/lib/download";
import { useRoleStore } from "@/features/enterprise/model/roleStore";
import { useOrganizationStore } from "@/features/enterprise/model/organization";
const catalogRoles = useRoleStore();
const catalogOrganization = useOrganizationStore();
const roleNames = computed(() =>
  catalogRoles.roles.filter((r) => r.enabled).map((r) => r.name),
);
const departments = computed(() => catalogOrganization.items.map((d) => d.name));
const store = useMemberStore();
const tenant = useTenantStore();
const audit = useAuditStore();
const route = useRoute();
const router = useRouter();
const query = reactive<MemberQuery>({
  keyword: "",
  role: "",
  department: "",
  status: "",
});
const page = ref(1);
const pageSize = ref(8);
const selected = ref<string[]>([]);
const filtered = computed(() => filterMembers(store.members, query));
const rows = computed(() =>
  filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
);
const allSelected = computed(
  () => rows.value.length > 0 && rows.value.every((m) => selected.value.includes(m.id)),
);
const active = computed(() => store.members.filter((m) => m.status !== "removed"));
const online = computed(() => active.value.filter((m) => m.online).length);
const invited = computed(() => active.value.filter((m) => m.status === "invited").length);
const editorOpen = ref(false);
const editMember = ref<Member>();
const invite = ref(false);
const actionMember = ref<Member>();
const operationOpen = ref(false);
const operationMember = ref<Member>();
const kind = ref<OperationKind>("role");
function reset() {
  Object.assign(query, { keyword: "", role: "", department: "", status: "" });
  page.value = 1;
  selected.value = [];
}
watch(query, () => {
  page.value = 1;
  selected.value = [];
});
watch(pageSize, () => {
  page.value = 1;
  selected.value = [];
});
watch(page, () => (selected.value = []));
watch(
  () => tenant.tenantId,
  () => {
    reset();
    editorOpen.value = false;
    operationOpen.value = false;
    actionMember.value = undefined;
  },
);
watch(
  () => route.query,
  (q) => {
    query.keyword = String(q.q ?? "");
    if (q.action === "create" || q.action === "invite")
      openEditor(undefined, q.action === "invite");
  },
  { immediate: true },
);
function openEditor(member?: Member, isInvite = false) {
  editMember.value = member;
  invite.value = isInvite;
  editorOpen.value = true;
}
function closeEditor() {
  editorOpen.value = false;
  if (route.query.action) void router.replace({ path: route.path });
}
function toggleAll() {
  selected.value = allSelected.value ? [] : rows.value.map((m) => m.id);
}
function openOperation(value: OperationKind) {
  kind.value = value;
  operationMember.value = actionMember.value;
  actionMember.value = undefined;
  operationOpen.value = true;
}
function detail() {
  if (actionMember.value)
    void router.push(`/enterprise/members/${actionMember.value.id}`);
  actionMember.value = undefined;
}
function exportSelected() {
  const data = selected.value.length
    ? filtered.value.filter((m) => selected.value.includes(m.id))
    : filtered.value;
  const csv = [
    ["姓名", "邮箱", "部门", "角色", "账号状态"],
    ...data.map((m) => [m.name, m.email, m.department, m.role, m.status]),
  ]
    .map((row) => row.map(csvCell).join(","))
    .join("\r\n");
  downloadText(
    "coffeelink-members-preview.csv",
    "\uFEFF" + csv,
    "text/csv;charset=utf-8",
  );
  audit.record("导出成员列表", `${data.length}条演示记录`);
}
</script>
<template>
  <PageHeader
    title="成员管理"
    description="管理企业成员，分配角色权限，助力团队高效协作"
    banner
  />
  <div class="metrics-grid">
    <MetricCard
      title="成员总数"
      :value="active.length"
      icon="UserRound"
      note="企业全部成员 · 含待激活"
    /><MetricCard
      title="在线成员"
      :value="online"
      icon="Users"
      tone="green"
      :note="`占比 ${active.length ? ((online / active.length) * 100).toFixed(1) : '0'}%`"
    /><MetricCard
      title="角色数量"
      :value="catalogRoles.roles.length"
      icon="Layers"
      tone="purple"
      note="内置角色与自定义角色"
    /><MetricCard
      title="待激活成员"
      :value="invited"
      icon="UserPlus"
      tone="orange"
      note="等待成员接受企业邀请"
    />
  </div>
  <section class="panel member-panel" aria-label="成员列表">
    <form class="query-bar" role="search" @submit.prevent="page = 1">
      <div class="search-field">
        <AppIcon name="Search" :size="17" /><input
          v-model="query.keyword"
          aria-label="搜索成员"
          placeholder="搜索成员姓名、手机号、邮箱…"
        />
      </div>
      <select v-model="query.role" aria-label="筛选角色">
        <option value="">全部角色</option>
        <option v-for="r in roleNames" :key="r">{{ r }}</option></select
      ><select v-model="query.department" aria-label="筛选部门">
        <option value="">全部部门</option>
        <option v-for="d in departments" :key="d">{{ d }}</option></select
      ><select v-model="query.status" aria-label="筛选账号状态">
        <option value="">全部状态</option>
        <option value="active">已启用</option>
        <option value="invited">待激活</option>
        <option value="suspended">已禁用</option></select
      ><BaseButton variant="quiet" icon="RefreshCw" @click="reset">重置</BaseButton
      ><BaseButton
        variant="quiet"
        icon="Download"
        aria-label="导出成员"
        @click="exportSelected"
      /><BaseButton variant="primary" icon="Plus" @click="openEditor()"
        >新增成员</BaseButton
      >
    </form>
    <div v-if="selected.length" class="selection-bar">
      已选择 {{ selected.length }} 位成员
      <button class="text-button" @click="exportSelected">导出所选</button
      ><button class="text-button" @click="selected = []">清空选择</button>
    </div>
    <div class="table-scroll">
      <table v-if="rows.length" class="member-table">
        <thead>
          <tr>
            <th class="checkbox-cell">
              <input
                type="checkbox"
                :checked="allSelected"
                aria-label="选择当前页全部成员"
                @change="toggleAll"
              />
            </th>
            <th>成员信息</th>
            <th>所属部门</th>
            <th>角色</th>
            <th>账号 / 在线状态</th>
            <th>最后登录时间</th>
            <th class="row-actions-heading">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="member in rows"
            :key="member.id"
            :class="{ selected: selected.includes(member.id) }"
          >
            <td class="checkbox-cell">
              <input
                v-model="selected"
                type="checkbox"
                :value="member.id"
                :aria-label="`选择${member.name}`"
              />
            </td>
            <td>
              <RouterLink :to="`/enterprise/members/${member.id}`" class="member-identity"
                ><span class="person-avatar">{{ member.name.slice(0, 1) }}</span>
                <div>
                  <strong>{{ member.name }}</strong
                  ><small>{{ member.email }}</small>
                </div></RouterLink
              >
            </td>
            <td>{{ member.department }}</td>
            <td>
              <StatusBadge :tone="member.role === '成员' ? 'gray' : 'blue'">{{
                member.role
              }}</StatusBadge>
            </td>
            <td>
              <span
                v-if="member.status === 'active'"
                :class="['online-label', member.online ? 'is-online' : 'is-offline']"
                ><i />{{ member.online ? "在线" : "离线" }}</span
              ><StatusBadge
                v-else
                :tone="member.status === 'invited' ? 'orange' : 'red'"
                dot
                >{{ member.status === "invited" ? "待激活" : "已禁用" }}</StatusBadge
              >
            </td>
            <td class="muted numeric">{{ member.lastLogin }}</td>
            <td>
              <div class="row-actions">
                <button
                  class="text-button"
                  :aria-label="`编辑${member.name}`"
                  @click="openEditor(member)"
                >
                  编辑</button
                ><button
                  class="icon-button blue-text"
                  :aria-label="`更多操作 ${member.name}`"
                  @click="actionMember = member"
                >
                  <AppIcon name="Ellipsis" :size="19" />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <EmptyState v-else><BaseButton @click="reset">清空筛选条件</BaseButton></EmptyState>
    </div>
    <TablePagination
      v-model:page="page"
      v-model:page-size="pageSize"
      :total="filtered.length"
    />
  </section>
  <div class="page-footnote">
    <AppIcon name="ShieldCheck" :size="14" />仅展示当前企业授权范围内的数据<span
      >演示数据 · 不连接生产环境</span
    >
  </div>
  <MemberEditor
    :open="editorOpen"
    :member="editMember"
    :invite="invite"
    @close="closeEditor"
  /><MemberActionMenu
    :member="actionMember"
    @close="actionMember = undefined"
    @operation="openOperation"
    @detail="detail"
  /><MemberOperationDialog
    :open="operationOpen"
    :member="operationMember"
    :kind="kind"
    @close="operationOpen = false"
  />
</template>
