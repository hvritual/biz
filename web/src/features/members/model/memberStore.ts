import { createPreviewId } from "@/shared/lib/previewId";
import { computed, reactive } from "vue";
import { defineStore } from "pinia";
import { useTenantStore } from "@/services/tenant";
import { useAuditStore } from "@/services/audit";
import { createMemberSeed } from "@/services/preview/memberSeed";
import { assertOwnerProtected, assertVersion, validateMember } from "./rules";
import type { Member, MemberInput, MemberStatus } from "./types";
export const useMemberStore = defineStore("members", () => {
  const tenant = useTenantStore();
  const audit = useAuditStore();
  const byTenant = reactive<Record<string, Member[]>>({});
  const members = computed(
    () =>
      byTenant[tenant.tenantId] ??
      (byTenant[tenant.tenantId] = createMemberSeed(tenant.tenantId)),
  );
  function find(id: string): Member {
    const m = members.value.find((item) => item.id === id);
    if (!m) throw new Error("当前企业下未找到该成员。");
    return m;
  }
  function add(input: MemberInput, invited = false) {
    const error = validateMember(input);
    if (error) throw new Error(error);
    if (
      members.value.some(
        (m) =>
          m.email.toLowerCase() === input.email.toLowerCase() && m.status !== "removed",
      )
    )
      throw new Error("该邮箱已是当前企业的成员。");
    const member: Member = {
      ...input,
      id: `${tenant.tenantId}-${createPreviewId()}`,
      status: invited ? "invited" : "active",
      online: false,
      lastLogin: "—",
      employeeNo: `CL${Date.now().toString().slice(-9)}`,
      joined: new Date().toISOString().slice(0, 10),
      dataScope: "所属部门数据",
      version: 1,
    };
    members.value.unshift(member);
    audit.record(invited ? "创建邀请（演示）" : "新增成员", member.name);
    return member;
  }
  function edit(id: string, input: MemberInput, version: number) {
    const m = find(id);
    assertVersion(m, version);
    const error = validateMember(input);
    if (error) throw new Error(error);
    if (
      members.value.some(
        (other) =>
          other.id !== id &&
          other.email.toLowerCase() === input.email.toLowerCase() &&
          other.status !== "removed",
      )
    )
      throw new Error("该邮箱已被其他成员使用。");
    assertOwnerProtected(members.value, m, input.role, m.status);
    const before = `${m.name} / ${m.department}`;
    Object.assign(m, input, { version: m.version + 1 });
    audit.record("修改成员信息", m.name, before, `${m.name} / ${m.department}`);
  }
  function changeRole(id: string, role: string, scope: string, version: number) {
    const m = find(id);
    assertVersion(m, version);
    assertOwnerProtected(members.value, m, role, m.status);
    const before = `${m.role} / ${m.dataScope}`;
    Object.assign(m, { role, dataScope: scope, version: m.version + 1 });
    audit.record("变更角色权限", m.name, before, `${role} / ${scope}`);
  }
  function setStatus(id: string, status: MemberStatus, reason: string, version: number) {
    const m = find(id);
    assertVersion(m, version);
    if (!reason.trim()) throw new Error("请填写操作原因。");
    if (!(
      (m.status === "active" && status === "suspended") ||
      (m.status === "suspended" && status === "active")
    ))
      throw new Error("当前成员状态不支持此操作。");
    assertOwnerProtected(members.value, m, m.role, status);
    const before = m.status;
    Object.assign(m, { status, online: false, version: m.version + 1 });
    audit.record(
      status === "suspended" ? "禁用成员" : "启用成员",
      m.name,
      before,
      `${status} / ${reason}`,
    );
  }
  function resetPassword(id: string, version: number) {
    const m = find(id);
    assertVersion(m, version);
    if (m.status === "invited" || m.status === "removed")
      throw new Error("此成员尚未激活，不能重置密码。");
    m.online = false;
    m.version++;
    audit.record(
      "发起密码重置（演示）",
      m.name,
      "—",
      "仅生成演示回执，不发送邮件、不修改真实凭据",
    );
  }
  return { members, find, add, edit, changeRole, setStatus, resetPassword };
});
