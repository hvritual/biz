import type { IconName } from "@/shared/lib/icons";
export interface NavLink {
  label: string;
  icon: IconName;
  to: string;
}
export interface NavModule {
  id: string;
  label: string;
  icon: IconName;
  description?: string;
  items?: NavLink[];
  shortcuts?: NavLink[];
  to?: string;
}
export const modules: NavModule[] = [
  { id: "home", label: "工作台", icon: "Home", to: "/workbench" },
  { id: "devices", label: "设备管理", icon: "Boxes" },
  { id: "sites", label: "点位管理", icon: "MapPin" },
  { id: "customers", label: "客户管理", icon: "Users" },
  { id: "orders", label: "订单管理", icon: "ClipboardList" },
  { id: "drinks", label: "饮品管理", icon: "Coffee" },
  { id: "remote", label: "远程运维", icon: "Orbit" },
  { id: "tickets", label: "故障工单", icon: "Wrench" },
  { id: "analytics", label: "数据分析", icon: "ChartNoAxesCombined" },
  { id: "success", label: "客户运营", icon: "UserRound" },
  { id: "leasing", label: "租赁管理", icon: "HardDrive" },
  {
    id: "enterprise",
    label: "企业中心",
    icon: "Building2",
    description: "管理企业组织、成员、权限与套餐",
    items: [
      { label: "成员管理", icon: "Users", to: "/enterprise/members" },
      { label: "角色权限", icon: "ShieldCheck", to: "/enterprise/roles" },
      { label: "组织架构", icon: "Network", to: "/enterprise/organization" },
      { label: "套餐信息", icon: "Crown", to: "/enterprise/plan" },
      { label: "企业信息", icon: "Building2", to: "/enterprise/profile" },
      { label: "操作日志", icon: "FileText", to: "/enterprise/audit" },
    ],
    shortcuts: [
      {
        label: "新增成员",
        icon: "Plus",
        to: "/enterprise/members?action=create",
      },
      {
        label: "邀请成员",
        icon: "UserPlus",
        to: "/enterprise/members?action=invite",
      },
      {
        label: "新建角色",
        icon: "ShieldCheck",
        to: "/enterprise/roles?action=create",
      },
      {
        label: "调整套餐",
        icon: "Crown",
        to: "/enterprise/plan?action=upgrade",
      },
      { label: "编辑企业信息", icon: "Pencil", to: "/enterprise/profile" },
      { label: "查看操作日志", icon: "FileText", to: "/enterprise/audit" },
    ],
  },
  {
    id: "settings",
    label: "系统设置",
    icon: "Settings",
    description: "管理企业配置、通知与访问安全",
    items: [
      { label: "基础设置", icon: "Settings", to: "/settings/general" },
      { label: "通知设置", icon: "Bell", to: "/settings/notifications" },
      { label: "安全设置", icon: "ShieldCheck", to: "/settings/security" },
      { label: "接口管理", icon: "Link", to: "/settings/integrations" },
    ],
    shortcuts: [
      { label: "修改品牌信息", icon: "Pencil", to: "/settings/general" },
      { label: "配置通知", icon: "Bell", to: "/settings/notifications" },
      { label: "检查安全策略", icon: "Shield", to: "/settings/security" },
    ],
  },
];
