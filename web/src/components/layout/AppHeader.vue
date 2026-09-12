<script setup lang="ts">
import { computed, ref } from 'vue'
import { useUiStore } from '@/stores/ui'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useRouter, useRoute } from 'vue-router'
import AppIcon from '@/components/ui/AppIcon.vue'
import AvatarMark from '@/components/ui/AvatarMark.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import brand from '@/assets/brand-mark.png'
const ui = useUiStore(),
  store = useEnterpriseStore(),
  router = useRouter(),
  route = useRoute()
const live = computed(() => route.meta.surface === 'platform' || route.meta.surface === 'runtime')
const search = ref(''),
  panel = ref('')
function changeTenant(e: Event) {
  ui.closeMenu()
  search.value = ''
  panel.value = ''
  void router.replace({ path: route.path, query: {} })
  store.switchTenant((e.target as HTMLSelectElement).value)
  ui.toast('已切换企业，数据与操作状态已隔离。', 'info')
}
function globalSearch() {
  if (search.value.trim()) {
    void router.push({ path: '/enterprise/members', query: { q: search.value.trim() } })
    ui.closeMenu()
  }
}
</script>
<template>
  <header class="app-header">
    <a class="brand" href="#/enterprise/members" aria-label="CoffeeLink 企业中心"
      ><img :src="brand" alt="CoffeeLink 标识" /><span
        ><strong>CoffeeLink</strong><small>咖啡机物联云平台</small></span
      ></a
    ><button
      class="icon-button mobile-toggle"
      aria-label="打开主导航"
      @click="ui.mobileOpen = !ui.mobileOpen"
    >
      <AppIcon name="menu" />
    </button>
    <div v-if="!live" class="header-company">
      <AppIcon name="company" :size="19" /><select
        :value="store.tenantId"
        aria-label="切换企业"
        @change="changeTenant"
      >
        <option value="shanghai">上海咖啡科技有限公司</option>
        <option value="hangzhou">杭州咖啡运营有限公司</option></select
      ><span class="edition">标准版</span>
    </div>
    <div v-else class="header-company">
      {{ route.meta.surface === 'platform' ? '平台管理' : '业务工作区' }}
    </div>
    <form v-if="!live" class="global-search" role="search" @submit.prevent="globalSearch">
      <AppIcon name="search" :size="16" /><input
        v-model="search"
        aria-label="全局搜索成员"
        placeholder="搜索设备、点位、客户、订单…"
      /><span>⌘ K</span>
    </form>
    <div v-if="!live" class="header-actions">
      <button class="icon-button notification" aria-label="通知中心" @click="panel = '通知中心'">
        <AppIcon name="bell" :size="21" /><b>12</b></button
      ><button class="header-link" @click="panel = '帮助中心'"><AppIcon name="help" />帮助中心</button
      ><button class="header-link" @click="panel = '下载中心'"><AppIcon name="download" />下载中心</button
      ><button class="profile" @click="panel = '当前账号'">
        <AvatarMark name="张" :size="36" tone="solid" /><span>张三<small>超级管理员</small></span>
        <AppIcon name="down" :size="14" />
      </button>
    </div>
  </header>
  <UiDialog :open="Boolean(panel)" :title="panel" @close="panel = ''">
    <div class="page-stack">
      <div class="notice-box">
        <AppIcon name="help" />当前为独立前端预览环境，未连接生产账号、通知或下载服务。
      </div>
      <template v-if="panel === '帮助中心'">
        <h3>企业中心使用指南</h3>
        <p class="secondary">
          通过一级菜单打开悬浮导航；右侧快捷入口可直接邀请成员、配置角色或查看审计记录。菜单支持 Esc
          关闭及键盘操作。
        </p> </template
      ><template v-else-if="panel === '通知中心'">
        <p>暂无已连接的通知源。</p>
        <button
          class="btn"
          @click="
            () => {
              router.push('/system/notifications')
              panel = ''
            }
          "
        >
          前往通知设置
        </button> </template
      ><template v-else-if="panel === '下载中心'">
        <p>列表导出文件由浏览器直接下载，不会上传到远程服务。</p> </template
      ><template v-else>
        <p>预览身份：张三 · 企业所有者</p>
        <p class="muted">真实登录、会话和退出由后续认证集成提供。</p>
      </template>
    </div>
  </UiDialog>
</template>
<style scoped>
.app-header {
  height: var(--header-height);
  position: fixed;
  inset: 0 0 auto;
  z-index: var(--z-header);
  display: flex;
  align-items: center;
  gap: 20px;
  /* background: rgb(255 255 255 / 90%); */
  /* border-bottom: 1px solid var(--color-border); */
  padding: 0 24px 0 20px;
  backdrop-filter: blur(10px);
}

.brand {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 176px;
  flex-shrink: 0;
  color: var(--color-text);
}

.brand img {
  width: 38px;
  height: 43px;
  object-fit: contain;
  mix-blend-mode: multiply;
}

.brand strong {
  display: block;
  font-size: 21px;
  line-height: 1.2;
  letter-spacing: -0.6px;
  font-weight: 750;
}

.brand small {
  display: block;
  font-size: 11px;
  margin-top: 2px;
}

.header-company {
  display: flex;
  align-items: center;
  gap: 9px;
  white-space: nowrap;
}

.header-company > .icon {
  color: var(--color-success);
}

.header-company select {
  border: 0;
  background: transparent;
  font-size: 12px;
  font-weight: 600;
  max-width: 190px;
  outline-offset: 4px;
}

.edition {
  font-size: 12px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  border-radius: 7px;
  padding: 8px 11px;
}

.global-search {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  height: 36px;
  border: 1px solid var(--color-border);
  border-radius: 9px;
  background: var(--color-surface-soft);
  margin: 0 auto;
  max-width: 470px;
  flex: 1;
  min-width: 100px;
  color: var(--color-text-muted);
}

.global-search input {
  border: 0;
  background: none;
  outline: 0;
  min-width: 0;
  flex: 1;
  font-size: 12px;
}

.global-search input::placeholder {
  color: var(--color-text-muted);
}

.global-search > span {
  font-size: 11px;
  white-space: nowrap;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-link {
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
  padding: 0;
  font-size: 12px;
}

.notification {
  position: relative;
}

.notification b {
  position: absolute;
  right: -1px;
  top: 0;
  color: var(--color-on-primary);
  background: var(--color-danger);
  font-size: 9px;
  line-height: 14px;
  min-width: 15px;
  border-radius: 9px;
  border: 1px solid var(--color-surface);
}

.profile {
  display: flex;
  align-items: center;
  gap: 10px;
  text-align: left;
  padding: 0 0 0 12px;
  border-left: 1px solid var(--color-border);
  font-size: 13px;
}

.profile small {
  display: block;
  color: var(--color-text-muted);
  font-size: 11px;
}

.mobile-toggle {
  display: none;
}

@media (max-width: 1250px) {
  .header-link {
    display: none;
  }

  .header-company select {
    max-width: 155px;
  }

  .header-actions {
    gap: 9px;
  }

  .app-header {
    gap: 14px;
  }

  .edition {
    display: none;
  }
}

@media (max-width: 767px) {
  .app-header {
    padding: 0 14px;
    gap: 8px;
  }

  .brand {
    width: auto;
    flex: 1;
  }

  .brand strong {
    font-size: 19px;
  }

  .brand img {
    height: 37px;
    width: 31px;
  }

  .brand small {
    font-size: 10px;
  }

  .header-company,
  .global-search,
  .profile > span:not(.avatar-mark),
  .profile > .icon {
    display: none;
  }

  .profile {
    padding-left: 5px;
    border: 0;
  }

  .mobile-toggle {
    display: flex;
    order: -1;
  }

  .header-actions {
    gap: 2px;
  }
}
</style>
