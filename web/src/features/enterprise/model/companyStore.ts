import { computed, reactive } from "vue";
import { defineStore } from "pinia";
import { tenants, useTenantStore } from "@/services/tenant";
import { useAuditStore } from "@/services/audit";
export interface CompanyProfile {
  name: string;
  shortName: string;
  industry: string;
  size: string;
  timezone: string;
  description: string;
  contact: string;
  email: string;
  phone: string;
  city: string;
}
export const useCompanyStore = defineStore("company", () => {
  const tenant = useTenantStore();
  const audit = useAuditStore();
  const data = reactive<Record<string, CompanyProfile>>({});
  const profile = computed(
    () =>
      data[tenant.tenantId] ??
      (data[tenant.tenantId] = {
        name: tenants.find((t) => t.id === tenant.tenantId)?.name ?? "演示企业",
        shortName: "CoffeeLink",
        industry: "咖啡设备与运营服务",
        size: "50–200 人",
        timezone: "Asia/Shanghai",
        description: "连接设备、激活数据，为咖啡经营提供专业、智能的数字化协作体验。",
        contact: "张三",
        email: "service@example.com",
        phone: "021-****1234",
        city: "上海",
      }),
  );
  function save(input: CompanyProfile) {
    if (!input.name.trim()) throw new Error("企业名称不能为空");
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(input.email))
      throw new Error("请填写有效的联系邮箱");
    Object.assign(profile.value, input);
    audit.record("修改企业信息", input.name);
  }
  return { profile, save };
});
