import type { Member, MemberInput, MemberQuery } from "./types";
export function filterMembers(members: Member[], query: MemberQuery): Member[] {
  const keyword = query.keyword.trim().toLocaleLowerCase();
  return members.filter(
    (m) =>
      m.status !== "removed" &&
      (!keyword ||
        [m.name, m.email, m.phone, m.employeeNo].some((v) =>
          v.toLocaleLowerCase().includes(keyword),
        )) &&
      (!query.role || m.role === query.role) &&
      (!query.department || m.department === query.department) &&
      (!query.status || m.status === query.status),
  );
}
export function validateMember(input: MemberInput): string | null {
  if (!input.name.trim()) return "请填写成员姓名";
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(input.email)) return "请输入有效的邮箱地址";
  if (!input.department || !input.role) return "请选择所属部门与角色";
  return null;
}
export function assertOwnerProtected(
  members: Member[],
  member: Member,
  nextRole: string,
  nextStatus: string,
): void {
  const remaining = members.filter(
    (m) => m.id !== member.id && m.role === "超级管理员" && m.status === "active",
  );
  if (
    member.role === "超级管理员" &&
    member.status === "active" &&
    !remaining.length &&
    (nextRole !== "超级管理员" || nextStatus !== "active")
  )
    throw new Error("不能禁用或移除企业最后一位超级管理员，请先完成所有者交接。");
}
export function assertVersion(member: Member, expected: number): void {
  if (member.version !== expected) throw new Error("成员信息已变更，请刷新后重试。");
}
