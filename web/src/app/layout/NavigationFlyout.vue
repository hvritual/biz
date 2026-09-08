<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { modules } from "../navigation/menu";
import { useNavigationStore } from "../stores/navigation";
import AppIcon from "@/shared/ui/AppIcon.vue";
import CoffeeBanner from "@/shared/ui/CoffeeBanner.vue";
const nav = useNavigationStore();
const route = useRoute();
const panel = ref<HTMLElement>();
const module = computed(() => modules.find((m) => m.id === nav.openModule));
watch(
  () => nav.openModule,
  async (value) => {
    if (value) {
      await nextTick();
      panel.value?.focus();
    }
  },
);
</script>
<template>
  <section
    v-if="module"
    id="module-drawer"
    ref="panel"
    tabindex="-1"
    class="navigation-flyout"
    aria-label="模块子菜单与快捷操作"
    data-testid="module-drawer"
    data-navigation-region
  >
    <header class="flyout-heading">
      <div>
        <h2>{{ module.label }}</h2>
        <p>{{ module.description }}</p>
      </div>
      <button class="icon-button" aria-label="关闭子菜单" @click="nav.close">
        <AppIcon name="X" :size="19" />
      </button>
    </header>
    <div class="flyout-columns">
      <nav aria-label="功能菜单" class="flyout-menu">
        <h3>功能菜单</h3>
        <RouterLink
          v-for="item in module.items"
          :key="item.to"
          :to="item.to"
          :class="[
            'flyout-link',
            {
              active:
                route.path === item.to ||
                (item.to === '/enterprise/members' && route.path.startsWith(item.to)),
            },
          ]"
          :aria-current="route.path === item.to ? 'page' : undefined"
          @click="nav.close"
          ><AppIcon :name="item.icon" :size="22" /><span>{{
            item.label
          }}</span></RouterLink
        >
      </nav>
      <nav aria-label="快捷操作" class="flyout-shortcuts">
        <h3>快捷操作</h3>
        <RouterLink
          v-for="item in module.shortcuts"
          :key="item.to"
          :to="item.to"
          class="shortcut-link"
          @click="nav.close"
          ><span
            :class="['shortcut-icon', { 'shortcut-icon--orange': item.icon === 'Crown' }]"
            ><AppIcon :name="item.icon" :size="20" /></span
          >{{ item.label }}</RouterLink
        >
      </nav>
    </div>
    <CoffeeBanner
      compact
      title="让每一杯咖啡更智能"
      subtitle="连接设备 · 激活数据 · 创造更多价值"
    />
  </section>
</template>
