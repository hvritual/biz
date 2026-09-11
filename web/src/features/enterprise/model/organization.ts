import { createPreviewId } from "@/shared/lib/previewId";
import { computed, reactive } from "vue";
import { defineStore } from "pinia";
import { departments } from "@/features/members/model/types";
import { useTenantStore } from "@/services/tenant";
import { useAuditStore } from "@/services/audit";
export interface Department {
  id: string;
  name: string;
  parentId: string | null;
  manager: string;
  description: string;
}
export const useOrganizationStore = defineStore("organization", () => {
  const tenant = useTenantStore();
  const audit = useAuditStore();
  const data = reactive<Record<string, Department[]>>({});
  const items = computed(
    () =>
      data[tenant.tenantId] ??
      (data[tenant.tenantId] = departments.map((name, i) => ({
        id: `dept-${i}`,
        name,
        parentId: [9, 10].includes(i) ? "dept-0" : null,
        manager: i === 1 ? "李四" : "张三",
        description: "负责部门日常业务与跨团队协作，保障企业运营效率。",
      }))),
  );
  function add(name: string, parentId: string | null) {
    if (!name.trim()) throw new Error("请填写部门名称");
    if (items.value.some((d) => d.name === name.trim()))
      throw new Error("部门名称已存在");
    if (parentId && !items.value.some((d) => d.id === parentId))
      throw new Error("上级部门不存在");
    items.value.push({
      id: createPreviewId(),
      name: name.trim(),
      parentId,
      manager: "待分配",
      description: "新建部门",
    });
    audit.record("新增部门", name);
  }
  return { items, add };
});
export function descendantIds(items: Department[], root: string): string[] {
  const result = [root];
  for (let i = 0; i < result.length; i++) {
    for (const child of items.filter((d) => d.parentId === result[i]))
      if (!result.includes(child.id)) result.push(child.id);
  }
  return result;
}
