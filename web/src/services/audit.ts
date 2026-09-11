import { createPreviewId } from "@/shared/lib/previewId";
import { computed, reactive } from "vue";
import { defineStore } from "pinia";
import { useTenantStore } from "./tenant";
export interface AuditRecord {
  id: string;
  time: string;
  actor: string;
  module: string;
  action: string;
  target: string;
  result: "成功" | "失败";
  risk: "低风险" | "中风险" | "高风险";
  before: string;
  after: string;
  requestId: string;
}
function seed(): AuditRecord[] {
  return [
    "为成员分配角色",
    "修改企业资料",
    "禁用成员账号",
    "发送成员邀请",
    "修改数据权限",
    "导出成员列表",
    "调整部门层级",
    "启用成员账号",
    "查看套餐信息",
    "修改通知偏好",
    "重置成员密码",
    "新建角色",
  ].map((action, i) => ({
    id: `log-${i}`,
    time: `2026-09-08 ${String(14 - Math.floor(i / 3)).padStart(2, "0")}:${String(5 + i * 2).padStart(2, "0")}:15`,
    actor: ["张三", "李四", "王五"][i % 3]!,
    module: ["成员管理", "企业信息", "成员管理"][i % 3]!,
    action,
    target: ["李四", "上海咖啡科技有限公司", "陈晨"][i % 3]!,
    result: "成功",
    risk: i % 4 === 0 ? "中风险" : "低风险",
    before: "成员 · 所属部门数据",
    after: "管理员 · 所属部门数据",
    requestId: `preview_req_${1001 + i}`,
  }));
}
export const useAuditStore = defineStore("audit", () => {
  const tenant = useTenantStore();
  const recordsByTenant = reactive<Record<string, AuditRecord[]>>({});
  const records = computed(
    () => recordsByTenant[tenant.tenantId] ?? (recordsByTenant[tenant.tenantId] = seed()),
  );
  function record(action: string, target: string, before = "—", after = "—") {
    records.value.unshift({
      id: createPreviewId(),
      time: new Date().toLocaleString("sv-SE", { timeZone: "Asia/Shanghai" }),
      actor: "张三（演示）",
      module: "企业中心",
      action,
      target,
      result: "成功",
      risk: /禁用|权限|密码/.test(action) ? "中风险" : "低风险",
      before,
      after,
      requestId: `preview_${createPreviewId().slice(0, 8)}`,
    });
  }
  return { records, record };
});
