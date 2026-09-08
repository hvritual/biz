<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useEnterpriseStore } from '@/stores/enterprise'
import { systemNavigation } from '@/router/navigation'
import PageHeading from '@/components/ui/PageHeading.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import GeneralSettings from '@/components/settings/GeneralSettings.vue'
import NotificationSettings from '@/components/settings/NotificationSettings.vue'
import SecuritySettings from '@/components/settings/SecuritySettings.vue'
import IntegrationSettings from '@/components/settings/IntegrationSettings.vue'
import DictionarySettings from '@/components/settings/DictionarySettings.vue'
const route = useRoute(),
  store = useEnterpriseStore()
const section = computed(() => String(route.params.section))
const component = computed(
  () =>
    ({
      general: GeneralSettings,
      notifications: NotificationSettings,
      security: SecuritySettings,
      integrations: IntegrationSettings,
      dictionary: DictionarySettings,
    })[section.value] ?? GeneralSettings,
)
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="系统设置"
      breadcrumb="企业运行系统"
      description="配置当前企业的基础信息、通知偏好与访问策略"
    />
    <div class="tabs">
      <RouterLink
        v-for="item in systemNavigation"
        :key="item.id"
        :to="item.path!"
        :class="['tab', { active: section === item.id }]"
        >{{ item.label }}</RouterLink
      >
    </div>
    <div class="split-layout">
      <section class="card panel-pad settings-main"><component :is="component" :key="section" /></section>
      <aside class="side-summary">
        <section class="card panel-pad">
          <h2>配置范围</h2>
          <div class="scope-symbol"><AppIcon name="shield" :size="33" /></div>
          <h3 class="scope-title">仅作用于当前企业</h3>
          <p class="scope-description">{{ store.company.name }}</p>
          <dl class="detail-list">
            <dt>编辑身份</dt>
            <dd>企业所有者</dd>
            <dt>环境</dt>
            <dd>前端界面预览</dd>
            <dt>生效方式</dt>
            <dd>本地保存</dd>
          </dl>
          <div class="divider" />
          <p class="scope-tip">平台级租户管理、全局套餐规则与运行参数属于独立管理系统，不在此处开放。</p>
        </section>
        <section class="card panel-pad">
          <div class="row-between block-title">
            <h2>最近变更记录</h2>
            <RouterLink class="btn-link" to="/enterprise/logs">更多</RouterLink>
          </div>
          <div class="timeline">
            <div
              v-for="log in store.logs.filter((l) => l.module === '系统设置').slice(0, 4)"
              :key="log.id"
              class="timeline-item"
            >
              <strong>{{ log.action }}</strong
              ><small>{{ log.time }} · {{ log.actor }}</small>
            </div>
          </div>
        </section>
      </aside>
    </div>
  </div>
</template>
<style scoped>
.tabs > .tab {
  display: flex;
  align-items: center;
  text-decoration: none;
}
.settings-main {
  padding: 28px;
}
.scope-symbol {
  display: grid;
  place-items: center;
  width: 74px;
  height: 74px;
  margin: 22px auto 15px;
  border-radius: 50%;
  background: var(--color-success-soft);
  color: var(--color-success);
}
.scope-title {
  text-align: center;
  color: var(--color-success);
}
.scope-description {
  text-align: center;
  font-size: 12px;
  color: var(--color-text-muted);
  margin: 8px 0 24px;
}
.detail-list {
  grid-template-columns: 70px 1fr;
  font-size: 12px;
}
.scope-tip {
  font-size: 12px;
  line-height: 1.9;
  color: var(--color-text-secondary);
}
@media (max-width: 767px) {
  .settings-main {
    padding: 20px;
  }
}
</style>
