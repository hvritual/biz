<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUiStore } from '@/stores/ui'
import { enterpriseNavigation, systemNavigation, quickActions, primaryNavigation } from '@/router/navigation'
import AppIcon from '@/components/ui/AppIcon.vue'
import coffee from '@/assets/coffee-menu.webp'
import { customerDomains } from '@/router/customerNavigation'
const ui = useUiStore(),
  route = useRoute(),
  router = useRouter()
const title = computed(() => primaryNavigation.find((p) => p.id === ui.module)?.label ?? '企业中心')
const links = computed(() =>
  ui.module === 'enterprise'
    ? enterpriseNavigation
    : ui.module === 'system'
      ? systemNavigation
      : (customerDomains[ui.module ?? '']?.links ?? []),
)
const actions = computed(() =>
  ui.module === 'enterprise'
    ? quickActions
    : ui.module === 'system'
      ? [
        { label: '修改基础设置', icon: 'settings', path: '/system/general' },
        { label: '配置通知', icon: 'bell', path: '/system/notifications' },
        { label: '查看安全策略', icon: 'shield', path: '/system/security' },
        { label: '查看操作日志', icon: 'file', path: '/enterprise/logs' },
      ]
      : (customerDomains[ui.module ?? '']?.actions ?? []),
)
function navigate(path?: string) {
  if (path) {
    ui.closeMenu()
    void router.push(path)
  }
}
</script>
<template>
  <section id="module-drawer" class="module-panel" role="dialog" aria-modal="true" :aria-label="title + '导航'">
    <header class="module-heading">
      <div>
        <h2>{{ title }}</h2>
        <p>
          {{
            ui.module === 'enterprise'
              ? '管理企业组织、成员、权限与套餐'
              : ui.module === 'system'
                ? '配置企业偏好、通知与访问策略'
                : (customerDomains[ui.module ?? '']?.description ?? '该业务模块尚未纳入本轮前端交付')
          }}
        </p>
      </div>
      <button class="icon-button" aria-label="关闭模块菜单" @click="ui.closeMenu()">
        <AppIcon name="close" :size="18" />
      </button>
    </header>
    <div v-if="links.length" class="module-columns">
      <nav class="sub-navigation" aria-label="功能菜单">
        <h3>功能菜单</h3>
        <button v-for="item in links" :key="item.id" :class="['sub-link', { active: route.path === item.path }]"
          :aria-current="route.path === item.path ? 'page' : undefined" @click="navigate(item.path)">
          <AppIcon :name="item.icon" :size="20" /><span>{{ item.label }}</span>
        </button>
      </nav>
      <nav class="quick-navigation" aria-label="快捷操作">
        <h3>快捷操作</h3>
        <button v-for="action in actions" :key="action.label" class="quick-link" @click="navigate(action.path)">
          <span :class="['quick-icon', { orange: action.icon === 'crown' }]">
            <AppIcon :name="action.icon" :size="19" />
          </span><span>{{ action.label }}</span>
        </button>
      </nav>
    </div>
    <div v-else class="module-unavailable">
      <AppIcon name="lock" :size="32" />
      <h3>页面尚未接入</h3>
      <p>本轮完成企业中心、成员生命周期与系统设置。这里保留产品导航位置，不展示虚构业务数据。</p>
      <button class="btn" @click="ui.module = 'enterprise'">进入企业中心</button>
    </div>
    <footer class="menu-art">
      <div>
        <h3>让每一杯咖啡更智能</h3>
        <p>连接设备 · 激活数据 · 创造更多价值</p>
      </div>
      <img :src="coffee" alt="浅蓝色咖啡主题插画" />
    </footer>
  </section>
</template>
<style scoped>
.module-panel {
  width: var(--module-width);
  min-width: var(--module-width);
  max-height: 100%;
  display: flex;
  flex-direction: column;
  padding: 24px;
  background: var(--color-surface);
  border-radius: 0 var(--radius-lg) var(--radius-lg) 0;
  box-shadow: var(--shadow-menu);
  overflow-y: auto;
}

.module-heading {
  position: relative;
  display: flex;
  gap: 12px;
  justify-content: space-between;
  border-bottom: 1px solid var(--color-border);
  padding-bottom: 19px;
  flex-shrink: 0;
}

.module-heading h2 {
  font-size: 23px;
  font-weight: 700;
  letter-spacing: -0.5px;
}

.module-heading p {
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  margin-top: 6px;
}

.module-heading>.icon-button {
  margin-top: -5px;
  margin-right: -8px;
  flex-shrink: 0;
}

.module-columns {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 22px;
  flex: 1;
  padding-top: 20px;
  min-height: 420px;
}

.module-columns h3 {
  font-size: var(--text-sm);
  font-weight: 650;
  margin-bottom: 17px;
}

.quick-navigation {
  border-left: 1px solid var(--color-border);
  padding-left: 20px;
}

.sub-link {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  width: fit-content;
  max-width: 100%;
  height: 44px;
  margin: 8px 0 10px -6px;
  border-radius: 8px;
  font-size: 14px;
  white-space: nowrap;
}

.sub-link>.icon {
  color: var(--color-text-secondary);
}

.sub-link.active {
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.sub-link.active>.icon {
  color: var(--color-primary);
}

.sub-link:hover {
  background: var(--color-surface-soft);
}

.sub-link.active:hover {
  background: var(--color-primary-soft);
}

.quick-link {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 48px;
  border: 1px solid var(--color-border);
  background: linear-gradient(115deg, var(--color-surface), var(--color-surface-soft));
  border-radius: 8px;
  padding: 8px 10px;
  margin-bottom: 10px;
  text-align: left;
  font-size: 12px;
  white-space: nowrap;
}

.quick-link:hover {
  border-color: var(--color-primary);
  background: var(--color-primary-soft);
}

.quick-icon {
  display: flex;
  color: var(--color-primary);
}

.quick-icon.orange {
  color: var(--color-warning);
}

.menu-art {
  position: relative;
  display: flex;
  align-items: center;
  min-height: 125px;
  border-radius: 10px;
  background: linear-gradient(115deg, var(--color-primary-soft), var(--color-canvas));
  isolation: isolate;
  overflow: hidden;
  flex-shrink: 0;
  margin-top: 24px;
  padding: 22px 20px;
}

.menu-art>div {
  z-index: 1;
  max-width: 280px;
}

.menu-art h3 {
  font-size: 16px;
  letter-spacing: -0.3px;
}

.menu-art p {
  font-size: 11px;
  margin-top: 8px;
  color: var(--color-text-secondary);
}

.menu-art img {
  position: absolute;
  right: 0;
  bottom: 0;
  height: 126px;
  width: 140px;
  object-fit: cover;
  mask-image: linear-gradient(to right, transparent, black 35%);
  z-index: 0;
}

.module-unavailable {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
  align-items: flex-start;
  justify-content: center;
  color: var(--color-text-secondary);
  font-size: var(--text-sm);
  padding-bottom: 20px;
}

.module-unavailable>.icon {
  color: var(--color-primary);
}

@media (max-height: 800px) {
  .module-panel {
    padding: 20px 24px;
  }

  .module-columns {
    min-height: 360px;
    padding-top: 15px;
  }

  .sub-link {
    height: 40px;
    margin-bottom: 8px;
  }

  .quick-link {
    min-height: 43px;
    margin-bottom: 8px;
  }

  .module-columns h3 {
    margin-bottom: 12px;
  }

  .menu-art {
    min-height: 96px;
    margin-top: 15px;
  }

  .menu-art img {
    height: 103px;
  }

  .menu-art p {
    max-width: 245px;
  }
}

@media (max-width: 767px) {
  .module-panel {
    width: calc(100vw - 80px);
    min-width: 0;
    padding: 20px 16px;
  }

  .module-columns {
    gap: 12px;
    grid-template-columns: 1fr 1fr;
  }

  .quick-navigation {
    padding-left: 12px;
  }

  .sub-link {
    font-size: 12px;
    gap: 7px;
    padding-left: 6px;
    padding-right: 7px;
  }

  .quick-link {
    font-size: 11px;
    padding: 7px;
    gap: 6px;
  }

  .quick-icon>.icon {
    width: 16px;
  }

  .menu-art {
    padding: 15px;
  }

  .menu-art h3 {
    font-size: 14px;
  }

  .menu-art p {
    max-width: 145px;
  }

  .module-heading h2 {
    font-size: 20px;
  }

  .module-heading p {
    font-size: 11px;
  }
}
</style>
