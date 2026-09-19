<script setup lang="ts">
import { UiButton } from '@/ui/base'

import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUiStore } from '@/stores/ui'
import { useEnterpriseStore } from '@/stores/enterprise'
import AppHeader from './AppHeader.vue'
import PrimaryNavigation from './PrimaryNavigation.vue'
import ModulePanel from './ModulePanel.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import { applyUiTheme, resolveTenantUiTheme } from '@/ui/base/theme'
import {
  authorizationApiMode,
  currentAuthorizationAllowsAny,
  currentAuthorizationState,
  redirectToTrustedLogin,
} from '@/services/runtime/authorization'

const ui = useUiStore()
const store = useEnterpriseStore()
const route = useRoute()
const router = useRouter()
const frame = ref<HTMLElement>()
const expanded = computed(() => Boolean(ui.module))
const platformSurface = computed(() => route.meta.surface === 'platform')
const routeKey = computed(() => `${store.tenantId || 'no-tenant'}:${route.path}`)
const protectedActions = computed(() =>
  Array.isArray(route.meta.authorizationActions)
    ? route.meta.authorizationActions.filter((value): value is string => typeof value === 'string' && value.length > 0)
    : [],
)
const protectedRoute = computed(() => authorizationApiMode() && protectedActions.value.length > 0)
const authorizationRenderable = computed(() => !protectedRoute.value || currentAuthorizationState.status === 'ready')

watch(
  [() => currentAuthorizationState.status, protectedActions] as const,
  ([status, required]) => {
    if (!authorizationApiMode() || !required.length) return
    if (status === 'unauthenticated') {
      redirectToTrustedLogin()
      return
    }
    if (status === 'error') {
      void router.replace({ path: '/authorization-state', query: { reason: 'unavailable', from: route.fullPath } })
      return
    }
    if (status === 'forbidden' || (status === 'ready' && !currentAuthorizationAllowsAny(required))) {
      void router.replace({ path: '/authorization-state', query: { reason: 'forbidden', from: route.fullPath } })
    }
  },
  { immediate: true },
)

function viewport() {
  if (window.innerWidth < 768) ui.collapsed = true
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
    const first = items[0]!
    const last = items[items.length - 1]!
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
      frame.value?.querySelector<HTMLElement>(`[data-module-id="${old}"]`)?.focus()
    }
  },
)

watch(
  [() => store.tenantId, () => store.branding] as const,
  ([tenantId, branding]) => {
    if (!branding || branding.tenantId !== tenantId) {
      applyUiTheme('blue')
      return
    }
    try {
      applyUiTheme(resolveTenantUiTheme({ preset: branding.preset, primary: branding.primary || undefined }))
    } catch {
      applyUiTheme('blue')
    }
  },
  { immediate: true },
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
    :class="{
      collapsed: ui.collapsed,
      'module-open': expanded,
      'mobile-nav-open': ui.mobileOpen,
      'platform-surface': platformSurface,
    }"
    data-business-ui
  >
    <AppHeader />
    <UiButton
      v-if="expanded || ui.mobileOpen"
      class="navigation-scrim"
      aria-label="关闭悬浮菜单"
      tabindex="-1"
      @click="ui.closeMenu()"
    />
    <aside ref="frame" class="side-frame" :class="{ joined: expanded }">
      <PrimaryNavigation />
      <ModulePanel v-if="ui.module" />
    </aside>
    <main class="main-content" :inert="expanded || ui.mobileOpen" data-testid="main-content">
      <RouterView v-if="authorizationRenderable" :key="routeKey" />
      <div v-else class="authorization-loading" role="status">正在确认当前租户授权…</div>
    </main>
    <Teleport to="body">
      <div v-if="ui.notice" role="status" :class="['toast', ui.noticeTone]">
        <AppIcon :name="ui.noticeTone === 'success' ? 'success' : ui.noticeTone === 'error' ? 'error' : 'help'" />{{
          ui.notice
        }}<UiButton class="icon-button" aria-label="关闭提示" @click="ui.notice = ''">
          <AppIcon name="close" :size="15" />
        </UiButton>
      </div>
    </Teleport>
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
  left: 8px;
  top: var(--header-height);
  bottom: 0;
  z-index: var(--z-nav);
  display: flex;
  align-items: stretch;
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-panel);
  width: var(--current-rail);
}
.side-frame :deep(.primary-nav) {
  width: var(--current-rail);
  flex: 0 0 var(--current-rail);
}
.side-frame.joined {
  width: calc(var(--current-rail) + var(--module-width));
  background: var(--color-surface);
  box-shadow: var(--shadow-menu);
}
.side-frame.joined :deep(.primary-nav) {
  border-radius: var(--radius-lg) 0 0 var(--radius-lg);
}
.main-content {
  margin-left: calc(var(--current-rail) + 8px);
  margin-top: var(--header-height);
  padding: 20px var(--content-padding) 24px;
  background: var(--color-surface);
  border-radius: var(--radius-sm) 0 0 0;
  min-width: 0;
  min-height: 100vh;
}
.authorization-loading {
  min-height: 240px;
  display: grid;
  place-items: center;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}
.platform-surface .main-content {
  background: var(--color-canvas);
  border-radius: 0;
}
.platform-surface .main-content :deep(.page-stack) {
  gap: 14px;
}
.platform-surface .main-content :deep(.card) {
  border-color: var(--color-border);
  border-radius: var(--radius-md);
  box-shadow: none;
}
.platform-surface .main-content :deep([data-ui-region='data'].card),
.platform-surface .main-content :deep([data-ui-region='history'].card),
.platform-surface .main-content :deep(.tenant-data-panel),
.platform-surface .main-content :deep(.versions-card) {
  box-shadow: var(--shadow-panel);
}
.platform-surface .main-content :deep([data-ui-region='metrics'] .card) {
  min-height: 96px;
}
.platform-surface .main-content :deep([data-ui-region='query'].card) {
  background: var(--color-surface);
}
.navigation-scrim {
  position: fixed;
  inset: var(--header-height) 0 0 calc(var(--current-rail) + 8px);
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
    width: var(--rail-collapsed-width);
    flex-basis: var(--rail-collapsed-width);
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
    --current-rail: var(--rail-collapsed-width);
  }
}
</style>
