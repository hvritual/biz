<script setup lang="ts">
import { useRoute, useRouter } from "vue-router";
import { modules } from "../navigation/menu";
import { useNavigationStore } from "../stores/navigation";
import BrandLogo from "@/shared/ui/BrandLogo.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
import { useToast } from "@/shared/ui/useToast";
const nav = useNavigationStore();
const route = useRoute();
const router = useRouter();
const toast = useToast();
function activate(id: string) {
  const module = modules.find((m) => m.id === id);
  if (module?.items) nav.toggle(id);
  else if (module?.to) {
    nav.close();
    void router.push(module.to);
  } else toast.show("此模块不在本轮复刻范围；本轮已实现企业中心、工作台和系统设置。");
}
</script>
<template>
  <aside
    :class="[
      'primary-nav',
      {
        'primary-nav--joined': nav.openModule,
        'primary-nav--collapsed': nav.collapsed,
      },
    ]"
    data-testid="primary-nav"
    data-navigation-region
  >
    <RouterLink
      to="/enterprise/members"
      class="brand-link"
      aria-label="CoffeeLink 成员管理"
      ><BrandLogo :compact="nav.collapsed"
    /></RouterLink>
    <nav aria-label="主导航" class="primary-nav-links">
      <button
        v-for="module in modules"
        :key="module.id"
        :class="[
          'primary-link',
          {
            selected: route.meta.section === module.id,
            expanded: nav.openModule === module.id,
          },
        ]"
        :title="nav.collapsed ? module.label : undefined"
        :aria-label="module.label"
        :aria-expanded="module.items ? nav.openModule === module.id : undefined"
        :aria-controls="module.items ? 'module-drawer' : undefined"
        :data-nav-id="module.id"
        @click="activate(module.id)"
      >
        <AppIcon :name="module.icon" :size="21" /><span v-if="!nav.collapsed">{{
          module.label
        }}</span>
      </button>
    </nav>
    <div class="nav-bottom">
      <button
        class="collapse-control"
        :aria-label="nav.collapsed ? '展开主菜单' : '收起主菜单'"
        @click="nav.toggleCollapse"
      >
        <AppIcon
          :name="nav.collapsed ? 'PanelLeftOpen' : 'PanelLeftClose'"
          :size="19"
        /><span v-if="!nav.collapsed">收起菜单</span
        ><kbd v-if="!nav.collapsed">⌘ \</kbd></button
      ><span v-if="!nav.collapsed" class="nav-version"
        >CoffeeLink Console <b>v0.1.0</b></span
      >
    </div>
  </aside>
</template>
