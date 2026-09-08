import { createPreviewId } from "@/shared/lib/previewId";
import { computed, reactive } from "vue";
import { defineStore } from "pinia";
import { roleNames } from "@/features/members/model/types";
import { useTenantStore } from "@/services/tenant";
import { useAuditStore } from "@/services/audit";
export const permissionModules = [
  "工作台",
  "设备管理",
  "点位管理",
  "客户管理",
  "订单管理",
  "饮品管理",
  "远程运维",
  "故障工单",
  "数据分析",
  "企业中心",
  "系统设置",
];
export interface Role {
  id: string;
  name: string;
  builtin: boolean;
  enabled: boolean;
  description: string;
  scope: string;
  permissions: Record<string, boolean[]>;
}
export const useRoleStore = defineStore("roles", () => {
  const tenant = useTenantStore();
  const audit = useAuditStore();
  const data = reactive<Record<string, Role[]>>({});
  const roles = computed(
    () =>
      data[tenant.tenantId] ??
      (data[tenant.tenantId] = roleNames.map((name, i) => ({
        id: `role-${i}`,
        name,
        builtin: [0, 1, 2, 4, 10].includes(i),
        enabled: i !== 8,
        description:
          i === 0
            ? "企业所有者角色，拥有当前企业的全部授权范围。"
            : "负责日常业务管理与协作，按职责授予最小必要权限。",
        scope: i === 0 ? "全部数据" : "所属部门数据",
        permissions: Object.fromEntries(
          permissionModules.map((module, j) => [
            module,
            [
              i === 0 || j < 9,
              i === 0 || (i !== 2 && j < 8),
              i === 0 || (i !== 2 && j < 6),
            ],
          ]),
        ),
      }))),
  );
  function add(name: string, description: string) {
    if (!name.trim()) throw new Error("请填写角色名称");
    if (roles.value.some((r) => r.name === name.trim()))
      throw new Error("角色名称已存在");
    roles.value.push({
      id: createPreviewId(),
      name: name.trim(),
      description,
      builtin: false,
      enabled: true,
      scope: "本人负责的数据",
      permissions: Object.fromEntries(
        permissionModules.map((m) => [m, [m === "工作台", false, false]]),
      ),
    });
    audit.record("新建角色", name);
  }
  function save(id: string, permissions: Record<string, boolean[]>, scope: string) {
    const role = roles.value.find((r) => r.id === id);
    if (!role) throw new Error("角色不存在");
    if (role.builtin) throw new Error("内置角色不允许修改，请新建自定义角色");
    role.permissions = structuredClone(permissions);
    role.scope = scope;
    audit.record("修改角色权限", role.name, "原权限配置", scope);
  }
  return { roles, add, save };
});
