export type MemberStatus = "active" | "invited" | "suspended" | "removed";
export interface Member {
  id: string;
  name: string;
  email: string;
  phone: string;
  department: string;
  role: string;
  status: MemberStatus;
  online: boolean;
  lastLogin: string;
  employeeNo: string;
  joined: string;
  dataScope: string;
  version: number;
}
export type MemberInput = Pick<
  Member,
  "name" | "email" | "phone" | "department" | "role"
>;
export interface MemberQuery {
  keyword: string;
  role: string;
  department: string;
  status: string;
}
export const departments = [
  "产品研发部",
  "运营管理部",
  "市场部",
  "客户成功部",
  "技术支持部",
  "财务部",
  "人事部",
  "售后服务部",
  "销售部",
  "研发一组",
  "研发二组",
  "行政部",
];
export const roleNames = [
  "超级管理员",
  "管理员",
  "成员",
  "运营管理员",
  "运维工程师",
  "销售经理",
  "财务专员",
  "数据分析师",
  "门店管理员",
  "区域经理",
  "审计专员",
  "客户经理",
];
export const statusLabels: Record<MemberStatus, string> = {
  active: "已启用",
  invited: "待激活",
  suspended: "已禁用",
  removed: "已移除",
};
