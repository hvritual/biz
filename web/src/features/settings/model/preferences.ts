import { computed, reactive } from "vue";
import { defineStore } from "pinia";
import { useTenantStore } from "@/services/tenant";
import { useAuditStore } from "@/services/audit";
export interface Preferences {
  name: string;
  shortName: string;
  description: string;
  language: string;
  timezone: string;
  workOrder: boolean;
  deviceAlert: boolean;
  memberChange: boolean;
  newsletter: boolean;
  email: boolean;
  siteMessage: boolean;
  mfa: boolean;
  loginLock: boolean;
  sessionMinutes: string;
  sso: boolean;
  webhook: string;
  webhookEnabled: boolean;
}
export function defaultPreferences(): Preferences {
  return {
    name: "CoffeeLink 咖啡机物联云平台",
    shortName: "CoffeeLink",
    description: "连接每一台咖啡机，让每一杯咖啡更智能。",
    language: "简体中文",
    timezone: "Asia/Shanghai",
    workOrder: true,
    deviceAlert: true,
    memberChange: true,
    newsletter: false,
    email: false,
    siteMessage: true,
    mfa: true,
    loginLock: true,
    sessionMinutes: "30",
    sso: false,
    webhook: "",
    webhookEnabled: false,
  };
}
export const usePreferencesStore = defineStore("preferences", () => {
  const tenant = useTenantStore();
  const audit = useAuditStore();
  const values = reactive<Record<string, Preferences>>({});
  const current = computed(
    () => values[tenant.tenantId] ?? (values[tenant.tenantId] = defaultPreferences()),
  );
  function save(value: Preferences) {
    if (!value.name.trim()) throw new Error("请填写平台名称。");
    if (value.webhookEnabled) {
      let url: URL;
      try {
        url = new URL(value.webhook);
      } catch {
        throw new Error("请输入有效的 HTTPS 回调地址。");
      }
      if (url.protocol !== "https:") throw new Error("回调地址必须使用 HTTPS。");
    }
    values[tenant.tenantId] = { ...value };
    audit.record("修改系统设置（演示）", value.shortName);
  }
  return { current, save };
});
