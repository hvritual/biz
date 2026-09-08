<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import PageHeader from "@/shared/ui/PageHeader.vue";
import MetricCard from "@/shared/ui/MetricCard.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import StatusBadge from "@/shared/ui/StatusBadge.vue";
import EmptyState from "@/shared/ui/EmptyState.vue";
import RoleCreateDialog from "../components/RoleCreateDialog.vue";
import RolePermissionsPanel from "../components/RolePermissionsPanel.vue";
import { useRoleStore } from "../model/roleStore";
import { useMemberStore } from "@/features/members/model/memberStore";
const store = useRoleStore();
const members = useMemberStore();
const route = useRoute();
const router = useRouter();
const keyword = ref("");
const type = ref("");
const selectedId = ref("role-3");
const create = ref(false);
const roles = computed(() =>
  store.roles.filter(
    (r) =>
      r.name.includes(keyword.value) &&
      (!type.value || (type.value === "builtin" ? r.builtin : !r.builtin)),
  ),
);
const selected = computed(
  () => store.roles.find((r) => r.id === selectedId.value) ?? store.roles[0],
);
const builtin = computed(() => store.roles.filter((r) => r.builtin).length);
watch(
  () => route.query.action,
  (value) => {
    create.value = value === "create";
  },
  { immediate: true },
);
function close() {
  create.value = false;
  if (route.query.action) void router.replace(route.path);
}
</script>
<template>
  <PageHeader
    title="角色权限管理"
    description="通过角色配置功能权限与数据范围，构建清晰、安全的企业访问体系"
    ><BaseButton variant="primary" icon="Plus" @click="create = true"
      >新建角色</BaseButton
    ></PageHeader
  >
  <div class="metrics-grid">
    <MetricCard
      title="角色总数"
      :value="store.roles.length"
      icon="Users"
      note="当前企业全部角色"
    /><MetricCard
      title="内置角色"
      :value="builtin"
      icon="Layers"
      note="系统预设 · 不可修改"
    /><MetricCard
      title="自定义角色"
      :value="store.roles.length - builtin"
      icon="FileText"
      note="按实际业务职责配置"
    /><MetricCard
      title="已分配成员"
      :value="members.members.filter((m) => m.status === 'active').length"
      icon="Users"
      tone="green"
      note="当前已启用成员"
    />
  </div>
  <div class="master-detail-grid">
    <section class="panel">
      <div class="query-bar">
        <div class="search-field">
          <AppIcon name="Search" :size="17" /><input
            v-model="keyword"
            aria-label="搜索角色"
            placeholder="搜索角色名称…"
          />
        </div>
        <select v-model="type" aria-label="角色类型">
          <option value="">全部类型</option>
          <option value="builtin">内置角色</option>
          <option value="custom">自定义角色</option></select
        ><BaseButton
          variant="quiet"
          icon="RefreshCw"
          @click="
            keyword = '';
            type = '';
          "
          >重置</BaseButton
        >
      </div>
      <div class="table-scroll">
        <table class="role-table">
          <thead>
            <tr>
              <th>角色名称</th>
              <th>角色类型</th>
              <th>关联成员</th>
              <th>数据范围</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="role in roles"
              :key="role.id"
              :class="{ selected: selectedId === role.id }"
              @click="selectedId = role.id"
            >
              <td>
                <button class="text-button role-name" @click="selectedId = role.id">
                  <AppIcon :name="role.builtin ? 'ShieldCheck' : 'Users'" :size="17" />{{
                    role.name
                  }}
                </button>
              </td>
              <td>
                <StatusBadge :tone="role.builtin ? 'blue' : 'green'">{{
                  role.builtin ? "内置角色" : "自定义角色"
                }}</StatusBadge>
              </td>
              <td class="numeric">
                {{ members.members.filter((m) => m.role === role.name).length }}
                人
              </td>
              <td class="muted">{{ role.scope }}</td>
              <td>
                <StatusBadge :tone="role.enabled ? 'green' : 'red'" dot>{{
                  role.enabled ? "启用" : "停用"
                }}</StatusBadge>
              </td>
              <td>
                <button class="text-button" @click="selectedId = role.id">配置</button>
              </td>
            </tr>
          </tbody>
        </table>
        <EmptyState v-if="!roles.length" />
      </div>
      <footer class="pagination">
        <span>共 {{ roles.length }} 个角色</span><small>权限变更仅作用于当前企业</small>
      </footer>
    </section>
    <RolePermissionsPanel v-if="selected" :key="selected.id" :role="selected" />
  </div>
  <RoleCreateDialog :open="create" @close="close" />
</template>
