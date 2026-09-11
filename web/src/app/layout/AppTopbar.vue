<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { tenants, useTenantStore } from "@/services/tenant";
import AppIcon from "@/shared/ui/AppIcon.vue";
import BaseDialog from "@/shared/ui/BaseDialog.vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import { useMemberStore } from "@/features/members/model/memberStore";
import { usePlanStore } from "@/features/enterprise/model/planStore";
import { useNavigationStore } from "../stores/navigation";
import { downloadText } from "@/shared/lib/download";
const tenant = useTenantStore();
const members = useMemberStore();
const plans = usePlanStore();
const pending = computed(
  () => members.members.filter((m) => m.status === "invited").length,
);
const nav = useNavigationStore();
const router = useRouter();
const query = ref("");
const help = ref(false);
const notifications = ref(false);
watch(
  () => tenant.tenantId,
  () => {
    nav.close();
    query.value = "";
    notifications.value = false;
    help.value = false;
    void router.push("/enterprise/members");
  },
);
function search() {
  if (query.value.trim())
    void router.push({
      path: "/enterprise/members",
      query: { q: query.value },
    });
}
</script>
<template>
  <header class="app-topbar" data-testid="topbar">
    <div class="tenant-selector">
      <AppIcon name="Building2" :size="20" /><select
        :value="tenant.tenantId"
        aria-label="切换企业"
        @change="tenant.switchTenant(($event.target as HTMLSelectElement).value)"
      >
        <option v-for="item in tenants" :key="item.id" :value="item.id">
          {{ item.name }}
        </option>
      </select>
    </div>
    <span class="plan-pill">{{ plans.name }}</span>
    <form class="global-search" role="search" @submit.prevent="search">
      <AppIcon name="Search" :size="18" /><input
        v-model="query"
        aria-label="全局搜索"
        placeholder="搜索设备、点位、客户、订单…"
      /><kbd>⌘ K</kbd>
    </form>
    <div class="topbar-tools">
      <span class="preview-pill" title="全部数据为前端演示数据，未连接生产接口"
        >界面演示</span
      ><button
        class="icon-button notification-trigger"
        aria-label="查看通知"
        @click="notifications = !notifications"
      >
        <AppIcon name="Bell" :size="21" /><span>3</span></button
      ><button class="topbar-text-button" @click="help = true">
        <AppIcon name="CircleHelp" :size="18" /><span>帮助中心</span></button
      ><button
        class="topbar-text-button"
        @click="
          downloadText(
            'coffeelink-preview-info.json',
            JSON.stringify(
              { mode: 'preview', tenant: tenant.tenantId, version: '0.1.0' },
              null,
              2,
            ),
            'application/json',
          )
        "
      >
        <AppIcon name="Download" :size="18" /><span>下载中心</span>
      </button>
      <details class="profile-menu">
        <summary>
          <span class="profile-avatar">张</span
          ><span class="profile-copy"><strong>张三</strong><small>超级管理员</small></span
          ><AppIcon name="ChevronDown" :size="14" />
        </summary>
        <div class="profile-popover">
          <b>当前为界面演示</b>
          <p>张三 · 演示管理员</p>
          <p>登录与权限由正式后端接入后提供。</p>
        </div>
      </details>
    </div>
    <div v-if="notifications" class="notification-popover">
      <h3>通知中心 <small>演示</small></h3>
      <p>有 {{ pending }} 位成员等待激活</p>
      <p>企业资料与设置支持交互预览</p>
      <p>本轮前端预览已就绪</p>
      <button class="text-button" @click="notifications = false">全部已读</button>
    </div>
    <BaseDialog :open="help" title="CoffeeLink 帮助中心" @close="help = false"
      ><h3>前端交互预览</h3>
      <p>
        已实现成员管理、角色权限、组织架构、套餐信息、企业信息、操作日志与系统设置。界面数据与操作均为本地演示，不发送邮件、不修改真实账号。
      </p>
      <p>点击企业中心展开 480 px 双列浮层；按 Esc 关闭；左下角可收起主菜单。</p>
      <template #footer
        ><BaseButton variant="primary" @click="help = false">知道了</BaseButton></template
      ></BaseDialog
    >
  </header>
</template>
