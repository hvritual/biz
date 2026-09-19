<script setup lang="ts">
import { UiButton } from '@/ui/base'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useUiStore } from '@/stores/ui'
import { isPrimaryNavigationActive, primaryNavigation } from '@/router/navigation'
import AppIcon from '@/ui/common/AppIcon.vue'
import {
  authorizationApiMode,
  currentAuthorizationAllowsAny,
  currentAuthorizationModuleAllowed,
  currentAuthorizationState,
} from '@/services/runtime/authorization'
const ui = useUiStore(), route = useRoute(), router = useRouter()
const { t } = useI18n()
const hoverOpened = ref<string | null>(null)
const previewLabel = computed(() => route.meta.surface === 'platform' || route.meta.surface === 'runtime' ? t('shell.trustedRuntime') : t('shell.previewData'))
const visiblePrimaryNavigation = computed(() =>
  primaryNavigation.filter((item) => {
    if (!authorizationApiMode()) return true
    if (item.id === 'dashboard') return true
    if (item.id === 'platform-commercial') return currentAuthorizationState.session?.actor_kind === 'platform'
    if (!item.authorizationActions?.length) return false
    if (currentAuthorizationState.status !== 'ready') return false
    if (item.authorizationModule && !currentAuthorizationModuleAllowed(item.authorizationModule)) return false
    return currentAuthorizationAllowsAny(item.authorizationActions)
  }),
)
watch(visiblePrimaryNavigation, (items) => {
  if (ui.module && !items.some((item) => item.id === ui.module)) ui.closeMenu()
})
const label = (item: (typeof primaryNavigation)[number]) => t(`navigation.primary.${item.id}`)
function activate(item: (typeof primaryNavigation)[number]) {
  if (item.path) { hoverOpened.value = null; ui.closeMenu(); void router.push(item.path); return }
  if (hoverOpened.value === item.id && ui.module === item.id) { hoverOpened.value = null; return }
  ui.module = ui.module === item.id ? null : item.id
  hoverOpened.value = null
}
function previewModule(item: (typeof primaryNavigation)[number]) {
  if (item.path || ui.module === item.id) return
  ui.module = item.id
  hoverOpened.value = item.id
}
function toggleCollapsed() { hoverOpened.value = null; ui.closeMenu(); ui.collapsed = !ui.collapsed }
</script>
<template>
  <nav class="primary-nav" :aria-label="t('shell.primaryNav')" :class="{ collapsed: ui.collapsed }">
    <div class="nav-items">
      <UiButton v-for="item in visiblePrimaryNavigation" :key="item.id" :class="['primary-item',{active:isPrimaryNavigationActive(item,route.meta.module),expanded:ui.module===item.id&&!isPrimaryNavigationActive(item,route.meta.module)}]" :title="label(item)" :aria-label="label(item)" :aria-expanded="item.path?undefined:ui.module===item.id" :aria-controls="item.path?undefined:'module-drawer'" :data-module="item.selectorId??item.id" :data-module-id="item.id" @click="activate(item)" @mouseenter="previewModule(item)">
        <AppIcon :name="item.icon" :size="20"/><span>{{ label(item) }}</span>
      </UiButton>
    </div>
    <footer><span v-if="!ui.collapsed" class="preview-label">{{ previewLabel }}</span><UiButton class="icon-button collapse-button" :aria-label="ui.collapsed?t('shell.expandPrimary'):t('shell.collapsePrimary')" @click="toggleCollapsed"><AppIcon :name="ui.collapsed?'expand':'collapse'" :size="17"/></UiButton></footer>
  </nav>
</template>
<style scoped>
.primary-nav { width:100%; flex-shrink:0; display:flex; flex-direction:column; padding:12px 8px; height:100%; overflow:hidden; }
.primary-nav.collapsed { padding-left:7px; padding-right:7px; }
.nav-items { flex:1; overflow:auto; scrollbar-width:none; }
.primary-item { width:100%; height:48px; display:flex; align-items:center; gap:14px; text-align:left; padding:0 14px; margin:3px 0; border-radius:var(--radius-md); white-space:nowrap; font-size:13px; font-weight:500; }
.primary-item > .icon { color:var(--color-text-muted); }
.primary-item:hover,.primary-item.expanded { background:var(--color-primary-soft); }
.primary-item.expanded { color:var(--color-primary); }
.primary-item.expanded > .icon { color:var(--color-primary); }
.primary-item.active { background:linear-gradient(105deg,var(--color-primary),var(--color-gradient-end)); color:var(--color-on-primary); box-shadow:0 4px 12px var(--color-fixed-77a43b18); }
.primary-item.active > .icon { color:var(--color-on-primary); }
.collapsed .primary-item { padding:0; justify-content:center; }
.collapsed .primary-item span { display:none; }
footer { display:flex; justify-content:space-between; align-items:center; padding:12px 7px 0; gap:5px; border-top:1px solid var(--color-border); min-height:46px; }
.preview-label { font-size:10px; color:var(--color-text-muted); white-space:nowrap; }
.collapse-button { border:1px solid var(--color-border); background:var(--color-surface); border-radius:50%; width:29px; height:29px; flex-shrink:0; }
.collapsed footer { padding:10px 0 0; justify-content:center; }
@media (max-height:830px){.primary-item{height:42px;margin:1px 0;font-size:12px;gap:12px}.primary-item>.icon{width:18px;height:18px}.primary-nav{padding-top:8px}}
@media (max-height:710px){.primary-item{height:37px}}
</style>
