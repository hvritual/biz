<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { usePlanStore, type PlanName } from "../model/planStore";
import PageHeader from "@/shared/ui/PageHeader.vue";
import MetricCard from "@/shared/ui/MetricCard.vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import BaseDialog from "@/shared/ui/BaseDialog.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
import StatusBadge from "@/shared/ui/StatusBadge.vue";
import { useMemberStore } from "@/features/members/model/memberStore";
import { useTenantStore } from "@/services/tenant";
import { useAuditStore } from "@/services/audit";
import { useToast } from "@/shared/ui/useToast";
import type { IconName } from "@/shared/lib/icons";
const members = useMemberStore();
const tenant = useTenantStore();
const audit = useAuditStore();
const toast = useToast();
const route = useRoute();
const router = useRouter();
const planStore = usePlanStore();
const current = computed(() => planStore.name);
const limit = computed(() => planStore.memberLimit);
const planOptions: PlanName[] = ["基础版", "专业版", "企业版"];
const choose = ref<PlanName>("企业版");
const upgrade = ref(false);
const tab = ref("使用额度");
const resources = computed(() => [
  {
    name: "成员账号",
    used: members.members.filter((m) => m.status !== "removed").length,
    total: limit.value,
    unit: "人",
  },
  {
    name: "设备数量",
    used: 320,
    total: current.value === "企业版" ? 2000 : 500,
    unit: "台",
  },
  { name: "点位数量", used: 86, total: 150, unit: "个" },
  { name: "客户数量", used: 42, total: 200, unit: "个" },
  { name: "数据存储", used: 128, total: 500, unit: "GB" },
]);
const rights: {
  name: string;
  description: string;
  icon: IconName;
  enabled: boolean;
}[] = [
  {
    name: "设备管理",
    description: "设备接入、监控与分组",
    icon: "Boxes",
    enabled: true,
  },
  {
    name: "工单服务",
    description: "故障报修与服务闭环",
    icon: "ClipboardList",
    enabled: true,
  },
  {
    name: "OTA 升级",
    description: "固件管理与批量升级",
    icon: "Upload",
    enabled: true,
  },
  {
    name: "成员与权限",
    description: "企业成员和访问授权",
    icon: "Users",
    enabled: true,
  },
  {
    name: "数据分析",
    description: "运营数据与趋势分析",
    icon: "ChartNoAxesCombined",
    enabled: true,
  },
  {
    name: "点位管理",
    description: "点位归属与运行概况",
    icon: "MapPin",
    enabled: true,
  },
  {
    name: "API 接口",
    description: "第三方系统数据对接",
    icon: "Link",
    enabled: false,
  },
  {
    name: "定制化报表",
    description: "按业务需求扩展报表",
    icon: "FileText",
    enabled: false,
  },
];
watch(
  () => route.query.action,
  (v) => (upgrade.value = v === "upgrade"),
  { immediate: true },
);
watch(
  () => tenant.tenantId,
  () => {
    upgrade.value = false;
    choose.value = "企业版";
  },
);
function close() {
  upgrade.value = false;
  if (route.query.action) void router.replace(route.path);
}
function apply() {
  planStore.change(choose.value);
  audit.record("模拟套餐变更", choose.value);
  toast.show("模拟套餐已变更；未产生真实订单或扣费。");
  close();
}
</script>
<template>
  <PageHeader
    title="套餐信息管理"
    description="掌握企业套餐权益与资源使用情况，让业务增长始终从容有序"
    ><BaseButton variant="primary" icon="Crown" @click="upgrade = true"
      >升级套餐</BaseButton
    ></PageHeader
  >
  <div class="metrics-grid">
    <MetricCard
      title="当前套餐"
      :value="current"
      icon="Crown"
      tone="green"
      note="演示订阅 · 使用中"
    /><MetricCard
      title="成员额度"
      :value="`${members.members.length} / ${limit}`"
      icon="Users"
      note="已使用 / 总额度"
    /><MetricCard
      title="设备额度"
      :value="`320 / ${planStore.deviceLimit}`"
      icon="Boxes"
      tone="purple"
      note="演示资源用量"
    /><MetricCard
      title="已开通模块"
      :value="`${current === '企业版' ? 8 : 6} / 8`"
      icon="Layers"
      tone="orange"
      note="功能与字段权限以套餐为准"
    />
  </div>
  <div class="plan-summary-grid">
    <section class="panel current-plan">
      <div class="plan-title">
        <span class="metric-icon tone-blue"><AppIcon name="Crown" :size="29" /></span>
        <div>
          <h2>{{ current }} <StatusBadge tone="green" dot>使用中</StatusBadge></h2>
          <p>适合规模化运营团队的咖啡机物联协作方案</p>
        </div>
      </div>
      <dl class="description-grid">
        <div>
          <dt>生效时间</dt>
          <dd>2026-09-08</dd>
        </div>
        <div>
          <dt>到期时间</dt>
          <dd>2027-09-08</dd>
        </div>
        <div>
          <dt>计费周期</dt>
          <dd>按年 · 价格为设计示例</dd>
        </div>
        <div>
          <dt>自动续费</dt>
          <dd>未开通</dd>
        </div>
      </dl>
      <div class="plan-price">
        {{ current === "企业版" ? "联系商务" : "¥ 29,800" }}
        <small v-if="current !== '企业版'">/ 年</small>
      </div>
      <div class="plan-actions">
        <BaseButton variant="primary" @click="upgrade = true">升级套餐</BaseButton
        ><BaseButton @click="upgrade = true">调整额度</BaseButton
        ><BaseButton @click="toast.show('当前为界面演示，没有创建续费订单。')"
          >续费</BaseButton
        >
      </div>
    </section>
    <section class="panel">
      <h3 class="panel-heading">
        套餐权益概览
        <span class="muted" style="font-size: 11px; font-weight: 400">当前企业</span>
      </h3>
      <div class="entitlements">
        <article v-for="right in rights" :key="right.name" class="entitlement">
          <AppIcon :name="right.icon" />
          <div>
            <h3>{{ right.name }}</h3>
            <p>{{ right.description }}</p>
          </div>
          <StatusBadge
            :tone="right.enabled || current === '企业版' ? 'green' : 'orange'"
            >{{
              right.enabled || current === "企业版" ? "已开通" : "未开通"
            }}</StatusBadge
          >
        </article>
      </div>
    </section>
  </div>
  <section class="panel">
    <div class="tabs">
      <button
        v-for="item in ['使用额度', '功能对比', '变更记录']"
        :key="item"
        :class="{ active: tab === item }"
        @click="tab = item"
      >
        {{ item }}
      </button>
    </div>
    <div class="panel-content plan-content">
      <table v-if="tab === '使用额度'" class="quota-table">
        <thead>
          <tr>
            <th>资源类型</th>
            <th>已使用</th>
            <th>总额度</th>
            <th style="width: 32%">使用率</th>
            <th>状态</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="resource in resources" :key="resource.name">
            <td>{{ resource.name }}</td>
            <td class="numeric">{{ resource.used }} {{ resource.unit }}</td>
            <td class="numeric">{{ resource.total }} {{ resource.unit }}</td>
            <td>
              <div class="quota-progress">
                <div class="progress-track">
                  <span
                    :style="{
                      width: `${Math.min(100, (resource.used / resource.total) * 100)}%`,
                    }"
                  />
                </div>
                <small>{{ ((resource.used / resource.total) * 100).toFixed(1) }}%</small>
              </div>
            </td>
            <td><StatusBadge tone="green">正常</StatusBadge></td>
          </tr>
        </tbody>
      </table>
      <table v-else-if="tab === '功能对比'">
        <thead>
          <tr>
            <th>功能模块</th>
            <th>基础版</th>
            <th>专业版</th>
            <th>企业版</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(right, i) in rights" :key="right.name">
            <td>{{ right.name }}</td>
            <td>{{ i < 2 ? "支持" : "—" }}</td>
            <td>{{ right.enabled ? "支持" : "—" }}</td>
            <td>支持</td>
          </tr>
        </tbody>
      </table>
      <div v-else>
        <div
          v-for="r in audit.records.filter((r) => r.action.includes('套餐'))"
          :key="r.id"
          class="timeline-item"
        >
          <span />
          <div>
            <strong>{{ r.action }} · {{ r.target }}</strong>
            <p>{{ r.time }}</p>
          </div>
        </div>
        <p class="muted">仅记录本次演示会话中的套餐变更，不对应真实账单。</p>
      </div>
    </div>
  </section>
  <BaseDialog
    :open="upgrade"
    title="调整企业套餐"
    description="选择目标方案并预览新的资源额度"
    wide
    @close="close"
    ><div class="plan-choice-grid">
      <button
        v-for="plan in planOptions"
        :key="plan"
        :class="['plan-choice', { active: choose === plan }]"
        @click="choose = plan"
      >
        <AppIcon name="Crown" :size="27" /><strong>{{ plan }}</strong
        ><small>{{
          plan === "企业版"
            ? "1,000 个成员 · 完整权益"
            : plan === "专业版"
              ? "500 个成员 · 专业运营"
              : "400 个成员 · 基础管理"
        }}</small>
      </button>
    </div>
    <div class="info-banner">
      仅更新前端演示套餐，不发起支付，不生成合同或真实续费订单。
    </div>
    <template #footer
      ><BaseButton @click="close">取消</BaseButton
      ><BaseButton variant="primary" @click="apply">应用模拟方案</BaseButton></template
    ></BaseDialog
  >
</template>
