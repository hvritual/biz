<script setup lang="ts">
import { computed } from "vue";
import PageHeader from "@/shared/ui/PageHeader.vue";
import MetricCard from "@/shared/ui/MetricCard.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
import { useMemberStore } from "@/features/members/model/memberStore";
import { useRoleStore } from "@/features/enterprise/model/roleStore";
import { useAuditStore } from "@/services/audit";
import type { IconName } from "@/shared/lib/icons";
const members = useMemberStore();
const roles = useRoleStore();
const audit = useAuditStore();
const online = computed(() => members.members.filter((m) => m.online).length);
const invited = computed(
  () => members.members.filter((m) => m.status === "invited").length,
);
const offline = computed(() => members.members.length - online.value - invited.value);
const ring = computed(() => {
  const all = members.members.length || 1;
  const a = (online.value / all) * 100;
  const b = ((online.value + offline.value) / all) * 100;
  return {
    background: `conic-gradient(var(--green) 0 ${a}%,var(--gray) ${a}% ${b}%,var(--orange) ${b}%)`,
  };
});
const entries: { label: string; icon: IconName; to: string }[] = [
  {
    label: "邀请成员",
    icon: "UserPlus",
    to: "/enterprise/members?action=invite",
  },
  { label: "角色权限", icon: "ShieldCheck", to: "/enterprise/roles" },
  { label: "组织架构", icon: "Network", to: "/enterprise/organization" },
  { label: "套餐信息", icon: "Crown", to: "/enterprise/plan" },
  { label: "企业信息", icon: "Building2", to: "/enterprise/profile" },
  { label: "操作日志", icon: "FileText", to: "/enterprise/audit" },
];
</script>
<template>
  <div class="workbench-title">
    <PageHeader
      title="你好，张三"
      description="企业成员、组织与权限，一处掌握团队协作动态"
      section="工作台"
      banner
    />
  </div>
  <div class="metrics-grid">
    <MetricCard
      title="企业成员"
      :value="members.members.length"
      icon="Users"
      note="当前企业全部成员"
    /><MetricCard
      title="在线成员"
      :value="online"
      icon="UserRound"
      tone="green"
      :note="`在线占比 ${((online / members.members.length) * 100).toFixed(1)}%`"
    /><MetricCard
      title="角色数量"
      :value="roles.roles.length"
      icon="Layers"
      tone="purple"
      note="按业务职责分配授权"
    /><MetricCard
      title="待激活成员"
      :value="invited"
      icon="Clock"
      tone="orange"
      note="等待成员接受邀请"
    />
  </div>
  <div class="workbench-charts">
    <section class="panel">
      <h3 class="panel-heading">
        成员活跃趋势
        <span class="muted" style="font-size: 11px; font-weight: 400"
          >近 7 天 · 图表样例</span
        >
      </h3>
      <div class="panel-content">
        <svg
          class="trend-svg"
          viewBox="0 0 640 240"
          role="img"
          aria-label="近七天成员活跃趋势设计示例，不代表真实业务数据"
        >
          <defs>
            <linearGradient id="trend-fill" x1="0" x2="0" y1="0" y2="1">
              <stop offset="0%" stop-color="#1879ff" stop-opacity=".2" />
              <stop offset="100%" stop-color="#1879ff" stop-opacity="0" />
            </linearGradient>
          </defs>
          <g v-for="(label, i) in [400, 300, 200, 100, 0]" :key="label">
            <line
              class="chart-gridline"
              x1="42"
              x2="620"
              :y1="16 + i * 46"
              :y2="16 + i * 46"
            />
            <text class="chart-axis" x="5" :y="20 + i * 46">{{ label }}</text>
          </g>
          <path
            class="chart-area"
            d="M42 132 L80 122 L115 127 L154 97 L193 108 L232 91 L270 92 L310 65 L348 72 L387 49 L425 61 L464 40 L502 47 L543 38 L582 56 L620 48 L620 200 L42 200 Z"
          />
          <path
            class="chart-series"
            d="M42 132 L80 122 L115 127 L154 97 L193 108 L232 91 L270 92 L310 65 L348 72 L387 49 L425 61 L464 40 L502 47 L543 38 L582 56 L620 48"
          />
          <g v-for="i in 7" :key="i">
            <text class="chart-axis" :x="35 + (i - 1) * 95" y="226">09/0{{ i + 1 }}</text>
          </g>
        </svg>
      </div>
    </section>
    <section class="panel">
      <h3 class="panel-heading">
        成员在线状态
        <span class="muted" style="font-size: 11px; font-weight: 400">当前演示数据</span>
      </h3>
      <div class="donut-wrap">
        <div class="donut" :style="ring">
          <div>
            <strong>{{ members.members.length }}</strong
            ><small>成员总数</small>
          </div>
        </div>
        <div class="chart-legend">
          <p>
            <i class="legend-dot" />在线 <b>{{ online }}</b>
          </p>
          <p>
            <i class="legend-dot offline" />其他成员 <b>{{ offline }}</b>
          </p>
          <p>
            <i class="legend-dot invited" />待激活 <b>{{ invited }}</b>
          </p>
        </div>
      </div>
    </section>
  </div>
  <div class="workbench-bottom">
    <section class="panel">
      <h3 class="panel-heading">
        最近操作
        <RouterLink class="text-link" to="/enterprise/audit">查看全部</RouterLink>
      </h3>
      <div class="panel-content">
        <div v-for="r in audit.records.slice(0, 4)" :key="r.id" class="timeline-item">
          <span />
          <div>
            <strong>{{ r.action }} · {{ r.target }}</strong>
            <p>{{ r.actor }} · {{ r.time }}</p>
          </div>
        </div>
      </div>
    </section>
    <section class="panel">
      <h3 class="panel-heading">常用功能</h3>
      <div class="quick-grid">
        <RouterLink v-for="item in entries" :key="item.to" :to="item.to"
          ><span class="metric-icon tone-blue"
            ><AppIcon :name="item.icon" :size="23" /></span
          >{{ item.label }}</RouterLink
        >
      </div>
    </section>
  </div>
  <p class="preview-note">
    界面演示 · 本轮实现企业中心与系统设置；设备等业务模块保留导航入口，未接入真实服务。
  </p>
</template>
