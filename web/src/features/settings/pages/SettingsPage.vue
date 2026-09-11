<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import PageHeader from "@/shared/ui/PageHeader.vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
import { useToast } from "@/shared/ui/useToast";
import { useTenantStore } from "@/services/tenant";
import { useAuditStore } from "@/services/audit";
import GeneralSettings from "../components/GeneralSettings.vue";
import NotificationSettings from "../components/NotificationSettings.vue";
import SecuritySettings from "../components/SecuritySettings.vue";
import IntegrationSettings from "../components/IntegrationSettings.vue";
import { usePreferencesStore } from "../model/preferences";
const route = useRoute();
const preferences = usePreferencesStore();
const tenant = useTenantStore();
const audit = useAuditStore();
const toast = useToast();
const draft = ref({ ...preferences.current });
const error = ref("");
const saved = ref("尚无保存记录");
const sections = [
  { id: "general", name: "基础设置", component: GeneralSettings },
  { id: "notifications", name: "通知设置", component: NotificationSettings },
  { id: "security", name: "安全设置", component: SecuritySettings },
  { id: "integrations", name: "接口管理", component: IntegrationSettings },
];
const active = computed(
  () => sections.find((s) => s.id === route.params.tab) ?? sections[0]!,
);
const records = computed(() =>
  audit.records.filter((r) => r.action.includes("设置")).slice(0, 4),
);
watch(
  () => tenant.tenantId,
  () => {
    draft.value = { ...preferences.current };
    error.value = "";
    saved.value = "尚无保存记录";
  },
);
function save() {
  try {
    preferences.save({ ...draft.value });
    error.value = "";
    saved.value = new Date().toLocaleTimeString("zh-CN", { hour12: false });
    toast.show("已保存演示配置；未修改真实系统或身份服务。");
  } catch (e) {
    error.value = e instanceof Error ? e.message : "保存失败";
  }
}
function restore() {
  draft.value = { ...preferences.current };
  error.value = "";
  toast.show("已撤销尚未保存的修改。");
}
</script>
<template>
  <PageHeader
    :title="`系统设置 / ${active.name}`"
    description="配置企业的基础信息、通知偏好与安全策略，让协作有序进行"
    section="系统设置"
  />
  <nav class="tabs settings-tabs" aria-label="系统设置分类">
    <RouterLink
      v-for="section in sections"
      :key="section.id"
      :to="`/settings/${section.id}`"
      :class="{ active: active.id === section.id }"
      >{{ section.name }}</RouterLink
    >
  </nav>
  <div class="settings-layout">
    <section class="panel">
      <form @submit.prevent="save">
        <component :is="active.component" v-model="draft" />
        <p v-if="error" class="form-error panel-content" role="alert">
          {{ error }}
        </p>
        <footer class="sticky-save">
          <div class="form-actions">
            <BaseButton type="submit" variant="primary" icon="Check">保存更改</BaseButton
            ><BaseButton icon="RefreshCw" @click="restore">撤销修改</BaseButton>
          </div>
          <small>上次保存：{{ saved }}</small>
        </footer>
      </form>
    </section>
    <aside class="settings-sidebar">
      <section class="panel">
        <h3 class="panel-heading">配置状态</h3>
        <div class="panel-content">
          <div class="safety-title">
            <span class="metric-icon tone-blue"
              ><AppIcon name="ShieldCheck" :size="27"
            /></span>
            <div>
              <h3>界面演示模式</h3>
              <p>尚未连接真实服务</p>
            </div>
          </div>
          <ul class="cert-list">
            <li><AppIcon name="CheckCircle2" />配置表单<span>可交互</span></li>
            <li><AppIcon name="Circle" />登录与认证<span>未联调</span></li>
            <li><AppIcon name="Circle" />通知发送<span>未联调</span></li>
            <li><AppIcon name="Circle" />操作审计持久化<span>未联调</span></li>
          </ul>
          <p class="permission-note">
            这里展示的是配置意图，不是安全检测结果。真实策略是否生效需要服务端回读验证。
          </p>
        </div>
      </section>
      <section class="panel">
        <h3 class="panel-heading">最近变更记录</h3>
        <div class="panel-content">
          <div v-for="r in records" :key="r.id" class="timeline-item">
            <span />
            <div>
              <strong>{{ r.action }}</strong>
              <p>{{ r.actor }} · {{ r.time }}</p>
            </div>
          </div>
          <p v-if="!records.length" class="muted">本次会话尚未修改系统设置</p>
          <RouterLink class="text-link" to="/enterprise/audit">查看操作日志</RouterLink>
        </div>
      </section>
      <section class="panel">
        <h3 class="panel-heading">配置提示</h3>
        <div class="panel-content">
          <p class="permission-note" style="margin: 0">
            企业配置按租户独立存放。切换企业后，当前未保存的表单内容不会带入新企业。
          </p>
        </div>
      </section>
    </aside>
  </div>
</template>
