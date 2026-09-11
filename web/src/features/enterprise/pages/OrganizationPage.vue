<script setup lang="ts">
import { computed, ref, watch } from "vue";
import PageHeader from "@/shared/ui/PageHeader.vue";
import MetricCard from "@/shared/ui/MetricCard.vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import BaseDialog from "@/shared/ui/BaseDialog.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
import StatusBadge from "@/shared/ui/StatusBadge.vue";
import EmptyState from "@/shared/ui/EmptyState.vue";
import TablePagination from "@/shared/ui/TablePagination.vue";
import DepartmentTree from "../components/DepartmentTree.vue";
import { useOrganizationStore, descendantIds } from "../model/organization";
import { useMemberStore } from "@/features/members/model/memberStore";
import { useToast } from "@/shared/ui/useToast";
import { tenants, useTenantStore } from "@/services/tenant";
const org = useOrganizationStore();
const members = useMemberStore();
const tenant = useTenantStore();
const toast = useToast();
const selectedId = ref("dept-1");
const keyword = ref("");
const memberKeyword = ref("");
const create = ref(false);
const name = ref("");
const parent = ref("");
const error = ref("");
const page = ref(1);
const pageSize = ref(8);
const selected = computed(() => org.items.find((d) => d.id === selectedId.value));
const counts = computed(() =>
  Object.fromEntries(
    org.items.map((d) => [
      d.name,
      members.members.filter((m) => m.department === d.name).length,
    ]),
  ),
);
const scoped = computed(() => {
  const ids = selected.value
    ? descendantIds(org.items, selected.value.id)
    : org.items.map((d) => d.id);
  const names = org.items.filter((d) => ids.includes(d.id)).map((d) => d.name);
  return members.members.filter(
    (m) =>
      names.includes(m.department) &&
      (!memberKeyword.value || m.name.includes(memberKeyword.value)),
  );
});
const rows = computed(() =>
  scoped.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
);
watch([selectedId, memberKeyword, pageSize], () => (page.value = 1));
function add() {
  try {
    org.add(name.value, parent.value || null);
    create.value = false;
    name.value = "";
    toast.show("部门已新增（本地演示）。");
  } catch (e) {
    error.value = (e as Error).message;
  }
}
</script>
<template>
  <PageHeader
    title="组织架构管理"
    description="管理企业部门与组织关系，让团队协作更清晰、数据归属更明确"
    ><BaseButton
      variant="primary"
      icon="Plus"
      @click="
        create = true;
        error = '';
      "
      >新建部门</BaseButton
    ></PageHeader
  >
  <div class="metrics-grid">
    <MetricCard
      title="部门数量"
      :value="org.items.length"
      icon="Network"
      note="支持多层级组织结构"
    /><MetricCard
      title="成员总数"
      :value="members.members.length"
      icon="Users"
      note="包含待激活成员"
    /><MetricCard
      title="已启用成员"
      :value="members.members.filter((m) => m.status === 'active').length"
      icon="UserRound"
      tone="green"
      note="当前企业有效成员"
    /><MetricCard
      title="管理员人数"
      :value="
        members.members.filter((m) => m.role.includes('管理员') && m.status === 'active')
          .length
      "
      icon="Crown"
      tone="purple"
      note="依据成员当前角色统计"
    />
  </div>
  <div class="organization-grid">
    <aside class="panel org-tree-panel">
      <h3>组织架构</h3>
      <input
        v-model="keyword"
        class="org-search"
        aria-label="搜索部门"
        placeholder="搜索部门名称"
      /><button class="org-root" @click="selectedId = ''">
        <AppIcon name="Building2" :size="17" />{{
          tenants.find((t) => t.id === tenant.tenantId)?.name
        }}</button
      ><DepartmentTree
        :items="org.items"
        :parent-id="null"
        :selected="selectedId"
        :counts="counts"
        :keyword="keyword"
        @select="selectedId = $event"
      />
    </aside>
    <section class="panel">
      <header class="org-detail-header">
        <div>
          <h2>{{ selected?.name ?? "全部部门" }}</h2>
          <p>{{ selected?.description ?? "查看当前企业全部成员及组织归属" }}</p>
        </div>
        <div>
          <BaseButton
            icon="Plus"
            @click="
              parent = selected?.id ?? '';
              create = true;
              error = '';
            "
            >新增下级部门</BaseButton
          >
        </div>
      </header>
      <div class="org-summary">
        <div>
          <small>部门负责人</small><strong>{{ selected?.manager ?? "张三" }}</strong>
          <p>负责团队日常管理</p>
        </div>
        <div>
          <small>部门成员</small
          ><strong>{{ scoped.length }} <small style="display: inline">人</small></strong>
          <p>包含下级部门</p>
        </div>
        <div>
          <small>部门状态</small
          ><strong><StatusBadge tone="green" dot>正常</StatusBadge></strong>
          <p>组织关系已生效</p>
        </div>
      </div>
      <div class="tabs"><button class="active">成员列表</button></div>
      <div class="query-bar">
        <div class="search-field">
          <AppIcon name="Search" :size="17" /><input
            v-model="memberKeyword"
            aria-label="搜索部门成员"
            placeholder="搜索成员姓名…"
          />
        </div>
        <BaseButton variant="quiet" @click="memberKeyword = ''">重置</BaseButton>
      </div>
      <div class="table-scroll">
        <table v-if="rows.length">
          <thead>
            <tr>
              <th>姓名</th>
              <th>角色</th>
              <th>所属部门</th>
              <th>账号状态</th>
              <th>加入时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="m in rows" :key="m.id">
              <td>
                <RouterLink :to="`/enterprise/members/${m.id}`" class="member-identity"
                  ><span class="person-avatar">{{ m.name.slice(0, 1) }}</span>
                  <div>
                    <strong>{{ m.name }}</strong
                    ><small>{{ m.email }}</small>
                  </div></RouterLink
                >
              </td>
              <td>
                <StatusBadge tone="blue">{{ m.role }}</StatusBadge>
              </td>
              <td>{{ m.department }}</td>
              <td>
                <StatusBadge
                  :tone="
                    m.status === 'active'
                      ? 'green'
                      : m.status === 'invited'
                        ? 'orange'
                        : 'red'
                  "
                  dot
                  >{{
                    m.status === "active"
                      ? "已启用"
                      : m.status === "invited"
                        ? "待激活"
                        : "已禁用"
                  }}</StatusBadge
                >
              </td>
              <td class="muted">{{ m.joined }}</td>
              <td>
                <RouterLink class="text-button" :to="`/enterprise/members/${m.id}`"
                  >查看</RouterLink
                >
              </td>
            </tr>
          </tbody>
        </table>
        <EmptyState v-else title="当前部门暂无匹配成员" />
      </div>
      <TablePagination
        v-model:page="page"
        v-model:page-size="pageSize"
        :total="scoped.length"
      />
    </section>
  </div>
  <BaseDialog :open="create" title="新建部门" @close="create = false"
    ><form id="department-create-form" @submit.prevent="add">
      <label class="field"
        ><span>部门名称 <em>*</em></span
        ><input
          v-model="name"
          required
          maxlength="40"
          placeholder="例如：华东运营中心" /></label
      ><label class="field separated"
        ><span>上级部门</span
        ><select v-model="parent">
          <option value="">企业根部门</option>
          <option v-for="d in org.items" :key="d.id" :value="d.id">
            {{ d.name }}
          </option>
        </select></label
      >
      <p v-if="error" role="alert" class="form-error">{{ error }}</p>
    </form>
    <template #footer
      ><BaseButton @click="create = false">取消</BaseButton
      ><BaseButton variant="primary" type="submit" form="department-create-form"
        >创建部门</BaseButton
      ></template
    ></BaseDialog
  >
</template>
