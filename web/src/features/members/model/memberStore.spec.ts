import { beforeEach, describe, expect, it } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { useMemberStore } from "./memberStore";
import { filterMembers, validateMember } from "./rules";
import { useTenantStore } from "@/services/tenant";
import { useAuditStore } from "@/services/audit";
const input = {
  name: "测试成员",
  email: "test@example.com",
  phone: "",
  department: "运营管理部",
  role: "成员",
};
beforeEach(() => setActivePinia(createPinia()));
describe("member preview domain", () => {
  it("has internally consistent metrics", () => {
    const s = useMemberStore();
    expect(s.members).toHaveLength(368);
    expect(s.members.filter((m) => m.online)).toHaveLength(312);
    expect(s.members.filter((m) => m.status === "invited")).toHaveLength(18);
  });
  it("filters text, role, department and status together", () => {
    const s = useMemberStore();
    const found = filterMembers(s.members, {
      keyword: "李四",
      role: "管理员",
      department: "运营管理部",
      status: "active",
    });
    expect(found).toHaveLength(1);
  });
  it("rejects blank name and invalid email", () => {
    expect(validateMember({ ...input, name: " " })).not.toBeNull();
    expect(validateMember({ ...input, email: "bad" })).not.toBeNull();
  });
  it("creates an invitation without a real email service", () => {
    const s = useMemberStore();
    const m = s.add(input, true);
    expect(m.status).toBe("invited");
    expect(s.members).toHaveLength(369);
    expect(useAuditStore().records[0]?.action).toContain("邀请");
  });
  it("rejects duplicate emails case insensitively", () => {
    const s = useMemberStore();
    s.add(input);
    expect(() => s.add({ ...input, email: "TEST@example.com" })).toThrow("邮箱");
  });
  it("protects last owner against suspend and demotion", () => {
    const s = useMemberStore();
    const owner = s.members.find((m) => m.role === "超级管理员")!;
    expect(() => s.setStatus(owner.id, "suspended", "离岗", owner.version)).toThrow(
      "最后一位",
    );
    expect(() => s.changeRole(owner.id, "成员", "本人负责的数据", owner.version)).toThrow(
      "最后一位",
    );
    expect(owner.status).toBe("active");
  });
  it("changes member state and prevents stale updates", () => {
    const s = useMemberStore();
    const m = s.add(input);
    const v = m.version;
    s.setStatus(m.id, "suspended", "暂离", v);
    expect(m.online).toBe(false);
    expect(() => s.setStatus(m.id, "active", "返回", v)).toThrow("已变更");
    s.setStatus(m.id, "active", "返回", m.version);
    expect(m.status).toBe("active");
  });
  it("requires an operational reason", () => {
    const s = useMemberStore();
    const m = s.add(input);
    expect(() => s.setStatus(m.id, "suspended", " ", m.version)).toThrow("原因");
  });
  it("changes role and records before/after", () => {
    const s = useMemberStore();
    const m = s.add(input);
    s.changeRole(m.id, "运营管理员", "本人负责的数据", m.version);
    expect(m.role).toBe("运营管理员");
    expect(useAuditStore().records[0]?.after).toContain("本人负责");
  });
  it("does not log secret material during password reset", () => {
    const s = useMemberStore();
    const m = s.add(input);
    s.resetPassword(m.id, m.version);
    const r = useAuditStore().records[0]!;
    expect(r.action).toContain("密码重置");
    expect(Object.keys(m)).not.toContain("password");
    expect(Object.keys(r)).not.toContain("token");
  });
  it("keeps tenant data and audit isolated", () => {
    const s = useMemberStore();
    const t = useTenantStore();
    const m = s.add(input);
    t.switchTenant("demo-hangzhou");
    expect(s.members).toHaveLength(24);
    expect(() => s.find(m.id)).toThrow("当前企业");
    expect(useAuditStore().records.some((r) => r.target === input.name)).toBe(false);
    t.switchTenant("demo-shanghai");
    expect(s.find(m.id).email).toBe(input.email);
  });
});
