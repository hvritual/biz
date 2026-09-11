import { ref } from "vue";
import { defineStore } from "pinia";
export const useNavigationStore = defineStore("navigation", () => {
  const collapsed = ref(false);
  const openModule = ref<string | null>(null);
  function close() {
    openModule.value = null;
  }
  function toggle(id: string) {
    openModule.value = openModule.value === id ? null : id;
  }
  function toggleCollapse() {
    collapsed.value = !collapsed.value;
  }
  return { collapsed, openModule, close, toggle, toggleCollapse };
});
