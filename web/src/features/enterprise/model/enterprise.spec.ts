import { beforeEach, expect, it } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { descendantIds, useOrganizationStore } from "./organization";
import { useRoleStore } from "./roleStore";
import { usePlanStore } from "./planStore";
import { useTenantStore } from "@/services/tenant";
import {
  defaultPreferences,
  usePreferencesStore,
} from "@/features/settings/model/preferences";
import { csvCell } from "@/shared/lib/download";
beforeEach(() => setActivePinia(createPinia()));
it("creates custom roles but never modifies built-in roles", () => {
  const s = useRoleStore();
  s.add("访客角色", "只读");
  expect(s.roles.at(-1)?.builtin).toBe(false);
  expect(() => s.save("role-0", {}, "全部数据")).toThrow("内置角色");
});
it("rejects duplicate roles", () => {
  const s = useRoleStore();
  expect(() => s.add("成员", "重复")).toThrow("已存在");
});
it("resolves descendant departments and prevents runaway traversal", () => {
  const s = useOrganizationStore();
  expect(descendantIds(s.items, "dept-0")).toHaveLength(3);
  expect(
    descendantIds(
      [
        { id: "a", parentId: "b", name: "a", manager: "", description: "" },
        { id: "b", parentId: "a", name: "b", manager: "", description: "" },
      ],
      "a",
    ),
  ).toEqual(["a", "b"]);
});
it("creates a department with a valid parent only", () => {
  const s = useOrganizationStore();
  expect(() => s.add("试验部", "absent")).toThrow("上级");
  s.add("试验部", "dept-0");
  expect(s.items.at(-1)?.name).toBe("试验部");
});
it("keeps plan state isolated across tenants", () => {
  const s = usePlanStore();
  s.change("企业版");
  expect(s.memberLimit).toBe(1000);
  useTenantStore().switchTenant("demo-hangzhou");
  expect(s.name).toBe("专业版");
  useTenantStore().switchTenant("demo-shanghai");
  expect(s.name).toBe("企业版");
});
it("validates secure webhook intent without making network requests", () => {
  const s = usePreferencesStore();
  expect(() =>
    s.save({
      ...defaultPreferences(),
      webhookEnabled: true,
      webhook: "http://example.com",
    }),
  ).toThrow("HTTPS");
  s.save({
    ...defaultPreferences(),
    webhookEnabled: true,
    webhook: "https://example.com",
  });
  expect(s.current.webhookEnabled).toBe(true);
});
it("settings remain tenant local", () => {
  const s = usePreferencesStore();
  s.save({ ...defaultPreferences(), name: "隔离测试" });
  useTenantStore().switchTenant("demo-hangzhou");
  expect(s.current.name).not.toBe("隔离测试");
});
it("escapes formulas and quotes in exported CSV cells", () => {
  expect(csvCell("=1+1")).toContain("'=1+1");
  expect(csvCell('a"b')).toContain('a""b');
});
