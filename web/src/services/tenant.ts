import { defineStore } from "pinia";
import { ref } from "vue";
export const tenants = [
  { id: "demo-shanghai", name: "上海咖啡科技有限公司", kind: "运营商" },
  { id: "demo-hangzhou", name: "杭州云啡运营有限公司", kind: "服务商" },
] as const;
export const useTenantStore = defineStore("tenant", () => {
  const tenantId = ref<string>(tenants[0].id);
  function switchTenant(id: string) {
    if (tenants.some((t) => t.id === id)) tenantId.value = id;
  }
  return { tenantId, switchTenant };
});
