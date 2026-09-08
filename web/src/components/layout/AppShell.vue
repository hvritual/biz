<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useUiStore } from '@/stores/ui'
import { useEnterpriseStore } from '@/stores/enterprise'
import AppHeader from './AppHeader.vue'
import PrimaryNavigation from './PrimaryNavigation.vue'
import ModulePanel from './ModulePanel.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
const ui = useUiStore(),
  store = useEnterpriseStore()
const frame = ref<HTMLElement>(),
  isMobile = ref(false)
const expanded = computed(() => Boolean(ui.module))
function viewport() {
  isMobile.value = window.innerWidth < 768
  if (isMobile.value) ui.collapsed = true
}
function keydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && (ui.module || ui.mobileOpen)) {
    e.preventDefault()
    ui.closeMenu()
  }
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    document.querySelector<HTMLInputElement>('.global-search input')?.focus()
  }
  if (e.key === 'Tab' && ui.module) {
    const items = frame.value?.querySelectorAll<HTMLElement>('button:not(:disabled),a[href]')
    if (!items?.length) return
    const first = items[0]!,
      last = items[items.length - 1]!
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault()
      last.focus()
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault()
      first.focus()
    }
  }
}
watch(
  () => ui.module,
  async (value, old) => {
    await nextTick()
    if (value) {
      frame.value?.querySelector<HTMLElement>('.module-panel .icon-button')?.focus()
    } else if (old) {
      frame.value?.querySelector<HTMLElement>(`[data-module="${old}"]`)?.focus()
    }
  },
)
onMounted(() => {
  viewport()
  window.addEventListener('resize', viewport)
  document.addEventListener('keydown', keydown)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', viewport)
  document.removeEventListener('keydown', keydown)
})
</script>
<template>
  <div
    class="app-shell"
    :class="{ collapsed: ui.collapsed, 'module-open': expanded, 'mobile-nav-open': ui.mobileOpen }"
    data-business-ui
  >
    <AppHeader /><button
      v-if="expanded || ui.mobileOpen"
      class="navigation-scrim"
      aria-label="关闭悬浮菜单"
      tabindex="-1"
      @click="ui.closeMenu()"
    />
    <aside ref="frame" class="side-frame" :class="{ joined: expanded }">
      <PrimaryNavigation /><ModulePanel v-if="ui.module" />
    </aside>
    <main class="main-content" :inert="expanded || ui.mobileOpen" data-testid="main-content">
      <div v-if="!store.previewMode" class="card panel-pad">
        <h1>真实 API 模式尚未接入</h1>
        <p class="secondary">
          本轮交付为界面复刻。系统不会将接口失败伪装成演示数据；请配置并完成认证与契约适配后启用 API 模式。
        </p>
      </div>
      <RouterView v-else :key="store.tenantId" />
    </main>
    <Teleport to="body"
      ><div v-if="ui.notice" role="status" :class="['toast', ui.noticeTone]">
        <AppIcon
          :name="ui.noticeTone === 'success' ? 'success' : ui.noticeTone === 'error' ? 'error' : 'help'"
        />{{ ui.notice
        }}<button class="icon-button" aria-label="关闭提示" @click="ui.notice = ''">
          <AppIcon name="close" :size="15" />
        </button></div
    ></Teleport>
  </div>
</template>
<style scoped>
.app-shell {
  --current-rail: var(--rail-width);
}
.app-shell.collapsed {
  --current-rail: var(--rail-collapsed-width);
}
.side-frame {
  position: fixed;
  left: 16px;
  top: 76px;
  bottom: 16px;
  z-index: var(--z-nav);
  display: flex;
  align-items: stretch;
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-panel);
  width: calc(var(--current-rail) - 16px);
}
.side-frame.joined {
  width: calc(var(--current-rail) - 16px + var(--module-width));
  background: var(--color-surface);
  box-shadow: var(--shadow-menu);
}
.side-frame.joined :deep(.primary-nav) {
  border-radius: var(--radius-lg) 0 0 var(--radius-lg);
}
.main-content {
  margin-left: var(--current-rail);
  padding: calc(var(--header-height) + 20px) var(--content-padding) 24px;
  min-width: 0;
  min-height: 100vh;
}
.navigation-scrim {
  position: fixed;
  inset: var(--header-height) 0 0 var(--current-rail);
  z-index: var(--z-scrim);
  background: var(--color-overlay);
  cursor: default;
}
.toast {
  position: fixed;
  top: 80px;
  left: 50%;
  transform: translateX(-50%);
  z-index: var(--z-toast);
  display: flex;
  align-items: center;
  gap: 10px;
  max-width: calc(100vw - 32px);
  padding: 10px 14px;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-menu);
  font-size: var(--text-sm);
}
.toast.success > .icon {
  color: var(--color-success);
}
.toast.error > .icon {
  color: var(--color-danger);
}
.toast.info > .icon {
  color: var(--color-primary);
}
@media (max-width: 767px) {
  .main-content {
    margin-left: 0;
    padding-top: calc(var(--header-height) + 14px);
  }
  .side-frame {
    display: none;
    top: var(--header-height);
    left: 0;
    bottom: 0;
    border-radius: 0;
  }
  .mobile-nav-open .side-frame,
  .module-open .side-frame {
    display: flex;
  }
  .side-frame.joined {
    width: 100vw;
  }
  .side-frame :deep(.primary-nav) {
    width: 80px;
    border-radius: 0 !important;
  }
  .navigation-scrim {
    left: 0;
  }
  .side-frame :deep(.module-panel) {
    border-radius: 0;
  }
  .toast {
    top: 74px;
  }
  .app-shell {
    --current-rail: 80px;
  }
}
</style>
