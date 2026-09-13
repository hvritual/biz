<script setup lang="ts">
import AppIcon from '@/components/ui/AppIcon.vue'
import PageHeading from '@/components/ui/PageHeading.vue'

const domains = [
  {
    title: '租户管理',
    description: '管理 SaaS 租户创建、启停、关闭与平台身份下的生命周期操作。',
    icon: 'company',
    path: '/platform/tenants',
    capability: '租户生命周期',
  },
  {
    title: '模块目录',
    description: '管理平台功能模块、技术状态、销售状态和服务端声明的依赖关系。',
    icon: 'database',
    path: '/platform/commercial/modules',
    capability: '能力目录',
  },
  {
    title: '套餐版本',
    description: '管理套餐版本、适用范围、模块组合与发布状态。',
    icon: 'crown',
    path: '/platform/commercial/plans',
    capability: '套餐治理',
  },
  {
    title: '租户权益',
    description: '查看租户最终权益决策，并在已有权限与契约范围内处理专项授权。',
    icon: 'shield',
    path: '/platform/commercial/tenant-entitlements',
    capability: '权益控制',
  },
]

const guardrails = [
  ['身份边界', '平台管理只接受平台可信会话；租户身份不能读取平台租户目录。'],
  ['数据边界', '平台商业事实以服务端回读为准，不使用前端 seed 模拟真实开通结果。'],
  ['操作边界', '套餐、权益、租户生命周期分别维护，不用页面状态替代后端授权。'],
]
</script>

<template>
  <div class="page-stack platform-overview" data-ui-template="WorkbenchPage" data-testid="platform-overview">
    <div data-ui-region="page-heading">
      <PageHeading
        title="平台管理"
        breadcrumb="管理系统"
        description="统一管理租户、功能模块、套餐版本与租户权益；平台管理与企业运行配置保持独立作用域。"
      />
    </div>

    <section class="scope-banner card" aria-label="平台管理作用域">
      <div class="scope-icon"><AppIcon name="shield" :size="24" /></div>
      <div class="flex-1">
        <strong>平台级管理作用域</strong>
        <p>这里管理所有租户及其商业能力，不修改单个租户的组织、成员和日常业务数据。</p>
      </div>
      <span class="scope-tag">Platform Scope</span>
    </section>

    <section class="management-grid" data-ui-region="management-domains" aria-label="平台管理功能">
      <RouterLink v-for="item in domains" :key="item.title" :to="item.path" class="management-card card">
        <div class="management-icon"><AppIcon :name="item.icon" :size="23" /></div>
        <div class="management-copy">
          <div class="row-between">
            <h2>{{ item.title }}</h2>
            <span class="capability-tag">{{ item.capability }}</span>
          </div>
          <p>{{ item.description }}</p>
          <span class="management-link">进入管理 <AppIcon name="right" :size="15" /></span>
        </div>
      </RouterLink>
    </section>

    <section class="card governance-panel" data-ui-region="governance">
      <div class="governance-heading">
        <div>
          <h2>平台治理边界</h2>
          <p>把“能看到页面”与“有权执行操作”分开，所有真实写操作都需要服务端回执与回读。</p>
        </div>
        <RouterLink class="btn" to="/platform/commercial/tenant-entitlements"><AppIcon name="shield" :size="15" />查看租户权益</RouterLink>
      </div>
      <div class="guardrail-grid">
        <article v-for="item in guardrails" :key="item[0]" class="guardrail-item">
          <span>{{ item[0] }}</span>
          <p>{{ item[1] }}</p>
        </article>
      </div>
    </section>
  </div>
</template>

<style scoped>
.platform-overview {
  gap: 18px;
}
.scope-banner {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 18px;
  border-color: rgb(37 99 235 / 16%);
  background: linear-gradient(110deg, var(--color-primary-soft), var(--color-surface));
}
.scope-icon {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  color: var(--color-primary);
  background: var(--color-surface);
  box-shadow: var(--shadow-panel);
}
.scope-banner strong {
  font-size: 15px;
}
.scope-banner p {
  margin-top: 3px;
  color: var(--color-text-secondary);
  font-size: var(--text-sm);
}
.scope-tag,
.capability-tag {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  padding: 0 9px;
  border-radius: 999px;
  background: var(--color-surface);
  color: var(--color-primary);
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}
.management-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}
.management-card {
  display: flex;
  gap: 16px;
  min-height: 174px;
  padding: 22px;
  color: inherit;
  transition: transform 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease;
}
.management-card:hover {
  transform: translateY(-2px);
  border-color: rgb(37 99 235 / 24%);
  box-shadow: 0 14px 30px rgb(31 41 55 / 9%);
}
.management-icon {
  display: grid;
  place-items: center;
  flex: 0 0 48px;
  height: 48px;
  border-radius: 13px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}
.management-copy {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
}
.management-copy h2 {
  font-size: 17px;
}
.management-copy p {
  margin-top: 12px;
  color: var(--color-text-secondary);
  font-size: var(--text-sm);
  line-height: 1.8;
}
.management-link {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  margin-top: auto;
  padding-top: 16px;
  color: var(--color-primary);
  font-size: var(--text-sm);
  font-weight: 600;
}
.governance-panel {
  padding: 22px;
}
.governance-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
}
.governance-heading p {
  margin-top: 5px;
  color: var(--color-text-secondary);
  font-size: var(--text-sm);
}
.guardrail-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-top: 20px;
}
.guardrail-item {
  min-height: 112px;
  padding: 16px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface-soft);
}
.guardrail-item span {
  font-size: 12px;
  font-weight: 650;
  color: var(--color-primary);
}
.guardrail-item p {
  margin-top: 8px;
  color: var(--color-text-secondary);
  font-size: 12px;
  line-height: 1.75;
}
@media (max-width: 900px) {
  .management-grid,
  .guardrail-grid {
    grid-template-columns: 1fr;
  }
  .governance-heading,
  .scope-banner {
    align-items: flex-start;
  }
}
@media (max-width: 600px) {
  .scope-banner,
  .governance-heading {
    flex-wrap: wrap;
  }
  .management-card {
    padding: 18px;
  }
  .capability-tag {
    display: none;
  }
}
</style>
