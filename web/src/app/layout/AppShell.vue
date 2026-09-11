<script setup lang="ts">
import { onBeforeUnmount, onMounted, watch } from "vue";
import { useRoute } from "vue-router";
import PrimaryNavigation from "./PrimaryNavigation.vue";
import NavigationFlyout from "./NavigationFlyout.vue";
import AppTopbar from "./AppTopbar.vue";
import { useNavigationStore } from "../stores/navigation";
import { useToast } from "@/shared/ui/useToast";
import AppIcon from "@/shared/ui/AppIcon.vue";
const nav = useNavigationStore();
const toast = useToast();
const route = useRoute();
watch(
  () => route.fullPath,
  () => nav.close(),
);
function closeAndFocus() {
  const id = nav.openModule;
  nav.close();
  if (id) document.querySelector<HTMLButtonElement>(`[data-nav-id="${id}"]`)?.focus();
}
function keydown(event: KeyboardEvent) {
  if (event.key === "Escape" && nav.openModule) {
    event.preventDefault();
    closeAndFocus();
  }
  if ((event.metaKey || event.ctrlKey) && event.key === "\\") {
    event.preventDefault();
    nav.toggleCollapse();
  }
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
    event.preventDefault();
    nav.close();
    document.querySelector<HTMLInputElement>('[aria-label="全局搜索"]')?.focus();
  }
  if (event.key === "Tab" && nav.openModule) {
    const controls = Array.from(
      document.querySelectorAll<HTMLElement>(
        "[data-navigation-region] button:not([disabled]),[data-navigation-region] a[href]",
      ),
    ).filter((e) => e.getClientRects().length);
    const first = controls[0],
      last = controls.at(-1);
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last?.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first?.focus();
    }
  }
}
onMounted(() => {
  if (matchMedia("(max-width: 760px)").matches) nav.collapsed = true;
  window.addEventListener("keydown", keydown);
});
onBeforeUnmount(() => window.removeEventListener("keydown", keydown));
</script>
<template>
  <div
    class="app-shell"
    :style="{ '--rail-size': nav.collapsed ? '72px' : '208px' }"
    :class="{
      'is-collapsed': nav.collapsed,
      'is-drawer-open': !!nav.openModule,
    }"
  >
    <a class="skip-link" href="#main-content">跳转至内容</a><PrimaryNavigation />
    <div :inert="!!nav.openModule"><AppTopbar /></div>
    <main
      id="main-content"
      class="main-content"
      :inert="!!nav.openModule"
      data-testid="main-content"
    >
      <RouterView />
    </main>
    <button
      v-if="nav.openModule"
      tabindex="-1"
      class="navigation-backdrop"
      aria-label="关闭菜单浮层"
      @click="closeAndFocus"
    /><NavigationFlyout />
    <div v-if="toast.message" class="toast" role="status">
      <AppIcon name="CircleHelp" :size="18" />{{ toast.message }}
    </div>
  </div>
</template>
