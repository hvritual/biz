import { computed, reactive } from "vue";
import { defineStore } from "pinia";
import { useTenantStore } from "@/services/tenant";
export type PlanName = "基础版" | "专业版" | "企业版";
export const usePlanStore = defineStore("plans", () => {
  const tenant = useTenantStore();
  const plans = reactive<Record<string, PlanName>>({});
  const name = computed(() => plans[tenant.tenantId] ?? "专业版");
  const memberLimit = computed(() =>
    name.value === "企业版" ? 1000 : name.value === "基础版" ? 400 : 500,
  );
  const deviceLimit = computed(() => (name.value === "企业版" ? 2000 : 500));
  function change(value: PlanName) {
    plans[tenant.tenantId] = value;
  }
  return { name, memberLimit, deviceLimit, change };
});
