<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUiStore } from '@/stores/ui'
import { primaryNavigation } from '@/router/navigation'
import AppIcon from '@/components/ui/AppIcon.vue'
const ui = useUiStore(),
  route = useRoute(),
  router = useRouter()
const hoverOpened = ref<string | null>(null)
const previewLabel = computed(() => route.meta.surface === 'platform' || route.meta.surface === 'runtime' ? '可信运行会话' : '界面预览 · 示例数据')
function activate(item: (typeof primaryNavigation)[number]) {
  if (item.path) {
    hoverOpened.value = null
    ui.closeMenu()
    void router.push(item.path)
    return
  }
  if (hoverOpened.value === item.id && ui.module === item.id) {
    hoverOpened.value = null
    return
  }
  ui.module = ui.module === item.id ? null : item.id
  hoverOpened.value = null
}
function previewModule(item: (typeof primaryNavigation)[number]) {
  if (item.path || ui.module === item.id) return
  ui.module = item.id
  hoverOpened.value = item.id
}
function toggleCollapsed() {
  hoverOpened.value = null
  ui.closeMenu()
  ui.collapsed = !ui.collapsed
}
</script>
<template>
  <nav class="primary-nav" aria-label="一级导航" :class="{ collapsed: ui.collapsed }">
    <div class="nav-items">
      <button
        v-for="item in primaryNavigation"
        :key="item.id"
        :class="[
          'primary-item',
          {
            active: route.meta.module === item.id,
            expanded: ui.module === item.id && route.meta.module !== item.id,
          },
        ]"
        :title="item.label"
        :aria-label="item.label"
        :aria-expanded="item.path ? undefined : ui.module === item.id"
        :aria-controls="item.path ? undefined : 'module-drawer'"
        :data-module="item.id"
        @click="activate(item)"
        @mouseenter="previewModule(item)"
      >
        <AppIcon :name="item.icon" :size="20" /><span>{{ item.label }}</span>
      </button>
    </div>
    <footer>
      <span v-if="!ui.collapsed" class="preview-label">{{ previewLabel }}</span
      ><button
        class="icon-button collapse-button"
        :aria-label="ui.collapsed ? '展开一级菜单' : '收起一级菜单'"
        @click="toggleCollapsed"
      >
        <AppIcon :name="ui.collapsed ? 'expand' : 'collapse'" :size="17" />
      </button>
    </footer>
  </nav>
</template>
<style scoped>
.primary-nav {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  padding: 12px 8px;
  height: 100%;
  overflow: hidden;
}
.primary-nav.collapsed {
  width: 64px;
  padding-left: 7px;
  padding-right: 7px;
}
.nav-items {
  flex: 1;
  overflow: auto;
  scrollbar-width: none;
}
.primary-item {
  width: 100%;
  height: 47px;
  display: flex;
  align-items: center;
  gap: 14px;
  text-align: left;
  padding: 0 14px;
  margin: 3px 0;
  border-radius: var(--radius-md);
  white-space: nowrap;
  font-size: 13px;
  font-weight: 500;
}
.primary-item > .icon {
  color: var(--color-text-muted);
}
.primary-item:hover,
.primary-item.expanded {
  background: var(--color-primary-soft);
}
.primary-item.expanded {
  color: var(--color-primary);
}
.primary-item.expanded > .icon {
  color: var(--color-primary);
}
.primary-item.active {
  background: linear-gradient(105deg, var(--color-primary), var(--color-gradient-end));
  color: var(--color-on-primary);
  box-shadow: 0 4px 12px rgb(8 123 255 / 12%);
}
.primary-item.active > .icon {
  color: var(--color-on-primary);
}
.collapsed .primary-item {
  padding: 0;
  justify-content: center;
}
.collapsed .primary-item span {
  display: none;
}
footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 7px 0;
  gap: 5px;
  border-top: 1px solid var(--color-border);
  min-height: 46px;
}
.preview-label {
  font-size: 10px;
  color: var(--color-text-muted);
  white-space: nowrap;
}
.collapse-button {
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  border-radius: 50%;
  width: 29px;
  height: 29px;
  flex-shrink: 0;
}
.collapsed footer {
  padding: 10px 0 0;
  justify-content: center;
}
@media (max-height: 830px) {
  .primary-item {
    height: 42px;
    margin: 1px 0;
    font-size: 12px;
    gap: 12px;
  }
  .primary-item > .icon {
    width: 18px;
    height: 18px;
  }
  .primary-nav {
    padding-top: 8px;
  }
}
@media (max-height: 710px) {
  .primary-item {
    height: 37px;
  }
}
</style>
