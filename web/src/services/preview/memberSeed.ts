import type { Member } from "@/features/members/model/types";
import { departments } from "@/features/members/model/types";
const names = ["张三", "李四", "王五", "刘佳", "陈晨", "赵六", "孙七", "周八"];
const emails = [
  "zhangsan",
  "lisi",
  "wangwu",
  "liujia",
  "chenchen",
  "zhaoliu",
  "sunqi",
  "zhouba",
];
export function createMemberSeed(tenant: string): Member[] {
  const count = tenant === "demo-shanghai" ? 368 : 24;
  return Array.from({ length: count }, (_, index) => {
    const status =
      index >= count - 18 && count > 24
        ? "invited"
        : index >= count - 26 && count > 24
          ? "suspended"
          : "active";
    return {
      id: `${tenant}-member-${index + 1}`,
      name:
        tenant === "demo-shanghai"
          ? (names[index] ?? `演示成员${String(index + 1).padStart(3, "0")}`)
          : `杭州成员${index + 1}`,
      email: `${emails[index] ?? `member${index + 1}`}@example.com`,
      phone: `138****${String(1000 + index).slice(-4)}`,
      department: departments[index % departments.length]!,
      role:
        index === 0 ? "超级管理员" : index % 3 === 0 || index === 1 ? "管理员" : "成员",
      status,
      online:
        status === "active" && index < Math.min(315, count) && ![2, 4, 6].includes(index),
      lastLogin:
        status === "invited"
          ? "—"
          : `2026-09-${String(8 - (index % 3)).padStart(2, "0")} ${["14:05", "11:23", "18:40", "09:12", "16:30"][index % 5]}`,
      employeeNo: `CL2026${String(index + 1).padStart(5, "0")}`,
      joined: "2026-02-18",
      dataScope: index === 0 ? "全部数据" : "所属部门数据",
      version: 1,
    };
  });
}
